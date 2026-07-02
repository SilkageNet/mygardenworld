// Package state tracks per-account land + inventory state. This is the Go
// port of GardenState from scripts/tools/garden_bot.py.
//
// The tracker is fed v-namespace fragments (typically `100` and `7`) from
// either index.reLogin responses (initial bulk) or per-RPC responses (delta
// after plant/water/harvest). It maintains a coherent in-memory view that
// the automation engine queries.
package state

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

// FlowerSeedLow is the inclusive lower itemId for flower seeds.
const FlowerSeedLow = 23000

// FlowerSeedHigh is the exclusive upper itemId for flower seeds.
const FlowerSeedHigh = 24000

// LandView mirrors the G.ILand schema: per-land state observed on the wire.
//
//	field 0 = flowerId
//	field 1 = state (1=just planted, 3=initial bloom ready, 2=regrowing)
//	field 2 = lvl (land level)
//	field 3 = harvestCnt (times this plant has been harvested)
//	field 5 = nextTime (ms; next state transition - regrow ready)
//	field 7 = plantTime (ms; last plant/state-change tick)
type LandView struct {
	FlowerID    int   `json:"flower_id,omitempty"`
	State       int   `json:"state,omitempty"`
	Lvl         int   `json:"lvl,omitempty"`
	HarvestCnt  int   `json:"harvest_cnt,omitempty"`
	NextTimeMs  int64 `json:"next_time_ms,omitempty"`
	PlantTimeMs int64 `json:"plant_time_ms,omitempty"`

	// Observed = the server has confirmed this land's state at least once
	// (including the empty-after-harvest state). Distinguishes "land we have
	// never seen" from "land we know is empty and ready to plant".
	Observed bool `json:"observed,omitempty"`
}

// IsPlanted returns true when a flower id is set on the land.
func (l LandView) IsPlanted() bool { return l.FlowerID != 0 }

// FromPrimary builds a LandView from the raw `100.1.<id>` JSON dict. Server
// responses use numeric-string keys ("0".."7"), per the G.ILand schema.
func FromPrimary(raw map[string]any) LandView {
	v := LandView{Observed: true}
	v.FlowerID = readInt(raw, "0")
	v.State = readInt(raw, "1")
	v.Lvl = readInt(raw, "2")
	v.HarvestCnt = readInt(raw, "3")
	v.NextTimeMs = readInt64(raw, "5")
	v.PlantTimeMs = readInt64(raw, "7")
	return v
}

// EmptyObserved is what we record after a harvest clears the land
// (server sends `100.1.<id> = {}`) - we still mark it observed so the
// automation engine knows the slot is plant-ready, not unknown.
func EmptyObserved() LandView { return LandView{Observed: true} }

// ToJSON returns the LandView as JSON for event emission.
func (l LandView) ToJSON() map[string]any {
	return map[string]any{
		"flowerId":   l.FlowerID,
		"state":      l.State,
		"lvl":        l.Lvl,
		"harvestCnt": l.HarvestCnt,
		"nextTime":   l.NextTimeMs,
		"plantTime":  l.PlantTimeMs,
		"observed":   l.Observed,
	}
}

// CultivateView mirrors the G.ICultivate schema from namespace 101.
//
//	field 1 = flowerId
//	field 2 = lvl (cultivation level, 0-5)
//	field 3 = culTime (ms; cultivation completion timestamp)
//	field 4 = status (0=idle, 1=cultivating, 2=received/ready for upgrade)
//	field 5 = uTime (ms; last update)
type CultivateView struct {
	FlowerID  int32 `json:"flower_id"`
	Lvl       int32 `json:"lvl"`
	CulTimeMs int64 `json:"cul_time_ms"`
	Status    int32 `json:"status"`
	UTimeMs   int64 `json:"u_time_ms"`
}

// FlowerOrder represents a resident order box from namespace 105 (orderFlower).
type FlowerOrder struct {
	BoxID    int32           `json:"box_id"`
	Mode     int32           `json:"mode,omitempty"`
	Requires []FlowerRequire `json:"requires"`
}

// CustomerOrder represents a customer order from namespace 109 (orderCustomer).
type CustomerOrder struct {
	NPCID        int32           `json:"npc_id"`
	Requires     []FlowerRequire `json:"requires,omitempty"`
	ItemRequires []ItemRequire   `json:"item_requires,omitempty"`
	FinishCnt    int32           `json:"finish_cnt,omitempty"`
}

// FlowerRackSlot represents one shelf slot from namespace 104 (flowerRack).
type FlowerRackSlot struct {
	RackID      int32 `json:"rack_id"`
	ItemID      int32 `json:"item_id,omitempty"`
	Count       int32 `json:"count,omitempty"`
	ListedAtMs  int64 `json:"listed_at_ms,omitempty"`
	UpdatedAtMs int64 `json:"updated_at_ms,omitempty"`
}

// FlowerRequire is a single flower requirement in an order.
type FlowerRequire struct {
	FlowerID int32 `json:"flower_id"`
	Count    int32 `json:"count"`
}

// ItemRequire is a generic inventory item requirement in an order.
type ItemRequire struct {
	ItemID int32 `json:"item_id"`
	Count  int32 `json:"count"`
}

// PlantableFlower describes a cultivated flower currently available for
// planting.
type PlantableFlower struct {
	FlowerID   int32 `json:"flower_id"`
	Stock      int32 `json:"stock"`
	Gold       int32 `json:"gold,omitempty"`
	Experience int32 `json:"experience,omitempty"`
}

// DailyTaskView is the tracked subset of G.ITaskItem from namespace 22.
type DailyTaskView struct {
	TaskID    int32 `json:"task_id"`
	Target    int32 `json:"target"`
	Finished  int32 `json:"finished"`
	Status    int32 `json:"status"`
	Receipted int32 `json:"receipted"`
}

// WeeklyTaskView is the tracked subset of c_task_week evaluated against
// namespace 22.100 progress and receipt maps.
type WeeklyTaskView struct {
	TaskID    int32 `json:"task_id"`
	Target    int32 `json:"target"`
	Finished  int32 `json:"finished"`
	Status    int32 `json:"status"`
	Receipted int32 `json:"receipted"`
}

// MainTaskView is the tracked subset of G.ITaskMain from namespace 22.0.
type MainTaskView struct {
	TaskID   int32 `json:"task_id"`
	Finished int32 `json:"finished"`
}

// RandomEventView is the tracked subset of namespace 129 map events. Static
// client schema names 129.IRandomEventInfo.1/2 as posIdx/dialogId; current
// status/affair semantics are capture-derived and pending revalidation.
type RandomEventView struct {
	EventID int32 `json:"event_id"`
	Status  int32 `json:"status"`
	Affair  int32 `json:"affair"`
}

// State is the per-account in-memory tracker.
type State struct {
	mu sync.RWMutex

	lands              map[int32]LandView
	landRosterObserved bool
	farmLands          map[int32]FarmLandInfo
	farmLandObserved   bool
	inventory          map[int32]int32 // 7.0.32 sub-map: itemId -> count
	gold               int32           // 7.0.44 金币
	level              int32           // 7.0.34 等级
	experience         int32           // 7.0.35 经验
	vip                int32           // 7.0.36 VIP level
	vipExp             int32           // 7.0.37 VIP exp
	diamondsFree       int32           // 7.0.41 免费钻石
	diamondsPaid       int32           // 7.0.42 付费钻石
	roleID             int64

	rawNamespaces   map[string]json.RawMessage
	namespaceCounts map[string]int32
	unknownNSCounts map[string]int32

	hasWaterDropsItem  bool  // whether namespace 7 has carried itemId=7 at least once
	waterDropsTotal    int32 // 7.0.33.7.1 observed water-drop capacity / total
	waterDropsNextMs   int64 // 7.0.33.7.5 下次恢复时间 ms
	waterDropsReserved int32 // drops committed to in-flight water RPCs

	wwClaimedCount int32 // 114.1 水车已领取总次数
	wwLastRecvTs   int64 // 114.4 uTime; used as the latest observed claim/update timestamp

	cultivations map[int32]*CultivateView // 101.0.<flowerId>

	customerOrders map[int32]*CustomerOrder  // 109.0.1.<npcId> 当前活跃顾客订单
	flowerRack     map[int32]*FlowerRackSlot // 104.0.<rackId> 花艺货架

	flowerOrders               map[int32]*FlowerOrder // 105.0.1.<boxId> 当前活跃居民订单
	flowerOrderRewardsReceived map[int32]bool         // 105.0.2 已领取的居民订单阶段奖励 target

	mainTask    *MainTaskView             // 22.0 当前主线任务
	dailyTasks  map[int32]*DailyTaskView  // 22.1.100.<taskId>
	weeklyTasks map[int32]*WeeklyTaskView // 22.100 + c_task_week

	roadGrowReceived map[int32]bool             // 119.3.<taskId> 成长之路已领取
	randomEvents     map[int32]*RandomEventView // 129.0.1.<eventId> 地图随机事件

	freeWaterObserved bool  // namespace 117 has been observed at least once
	freeWaterRecvIdx  int32 // 117.1 last observed free-water receive index
	freeWaterResetMs  int64 // 117.2 reset timestamp

	benefitBoxDrawCnt    int32 // 116.0.1 drawCnt
	benefitBoxResetCntMs int64 // 116.0.2 resetCntTime
	benefitBoxUTimeMs    int64 // 116.0.3 uTime
	benefitBoxObserved   bool  // namespace 116 has been observed at least once

	// Recent server-side activity timestamp; updated on every apply.
	lastApplyMs int64

	// onChange (optional) is invoked on every accepted apply, with a
	// snapshot of changed-land ids and the new view. Useful for the runner
	// to push events to subscribers.
	onChange func(changed []LandChange)

	// onResourceChange (optional) is invoked when any resource field changes.
	onResourceChange func(ResourceSnapshot)

	// onInventoryChange (optional) is invoked when the tracked inventory map changes.
	onInventoryChange func(InventorySnapshot)
}

// LandChange is the diff produced by apply.
type LandChange struct {
	LandID int32
	Before LandView
	After  LandView
}

// ResourceSnapshot is the current state of player resources, emitted on change.
type ResourceSnapshot struct {
	Gold             int32 `json:"gold"`
	WaterDrops       int32 `json:"water_drops"`
	WaterDropsTotal  int32 `json:"water_drops_total"`
	WaterDropsNextMs int64 `json:"water_drops_next_ms"`
	Level            int32 `json:"level"`
	Experience       int32 `json:"experience"`
	Vip              int32 `json:"vip"`
	VipExp           int32 `json:"vip_exp"`
	NobleEligible    bool  `json:"noble_eligible"`
	DiamondsFree     int32 `json:"diamonds_free"`
	DiamondsPaid     int32 `json:"diamonds_paid"`
}

// InventorySnapshot is emitted when the tracked item inventory changes.
type InventorySnapshot struct {
	Inventory map[int32]int32      `json:"inventory"`
	Changes   []InventoryItemDelta `json:"changes,omitempty"`
}

// InventoryItemDelta describes one changed inventory entry.
type InventoryItemDelta struct {
	ItemID int32 `json:"item_id"`
	Before int32 `json:"before"`
	After  int32 `json:"after"`
}

// New creates an empty tracker.
func New() *State {
	return &State{
		lands:                      make(map[int32]LandView),
		farmLands:                  make(map[int32]FarmLandInfo),
		inventory:                  make(map[int32]int32),
		rawNamespaces:              make(map[string]json.RawMessage),
		namespaceCounts:            make(map[string]int32),
		unknownNSCounts:            make(map[string]int32),
		cultivations:               make(map[int32]*CultivateView),
		customerOrders:             make(map[int32]*CustomerOrder),
		flowerRack:                 make(map[int32]*FlowerRackSlot),
		flowerOrders:               make(map[int32]*FlowerOrder),
		flowerOrderRewardsReceived: make(map[int32]bool),
		dailyTasks:                 make(map[int32]*DailyTaskView),
		weeklyTasks:                make(map[int32]*WeeklyTaskView),
		roadGrowReceived:           make(map[int32]bool),
		randomEvents:               make(map[int32]*RandomEventView),
	}
}

// SetOnChange installs a callback fired whenever lands change. Called with
// the lock released.
func (s *State) SetOnChange(fn func(changed []LandChange)) {
	s.mu.Lock()
	s.onChange = fn
	s.mu.Unlock()
}

// SetOnResourceChange installs a callback fired whenever resource fields change.
// Called with the lock released.
func (s *State) SetOnResourceChange(fn func(ResourceSnapshot)) {
	s.mu.Lock()
	s.onResourceChange = fn
	s.mu.Unlock()
}

// SetOnInventoryChange installs a callback fired whenever tracked item counts change.
// Called with the lock released.
func (s *State) SetOnInventoryChange(fn func(InventorySnapshot)) {
	s.mu.Lock()
	s.onInventoryChange = fn
	s.mu.Unlock()
}

// ApplyV merges a v-fragment from the server (full or partial) into state.
// Recognised top-level keys include inventory/resources, lands, cultivation,
// orders, tasks, waterwheel, and free-water reward state. Other keys are
// silently ignored - they're outside this tracker's scope.
//
// When the input is not a JSON object (e.g. some legacy responses serialize
// v as a JSON-stringified blob), ApplyV is a no-op.
func (s *State) ApplyV(rawV json.RawMessage) {
	if len(rawV) == 0 {
		return
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(rawV, &top); err != nil {
		return
	}
	s.applyTop(top)
}

// ApplyVMap is the post-decoded counterpart of ApplyV: pass an already-parsed
// `map[string]any`. The runner uses this when subscribing via
// Client.OnNamespace, where the fragment arrives pre-extracted.
func (s *State) ApplyVMap(top map[string]any) {
	conv := make(map[string]json.RawMessage, len(top))
	for k, v := range top {
		raw, _ := json.Marshal(v)
		conv[k] = raw
	}
	s.applyTop(conv)
}

func (s *State) applyTop(top map[string]json.RawMessage) {
	s.mu.Lock()
	now := time.Now().UnixMilli()
	s.lastApplyMs = now

	var changes []LandChange
	if s.rawNamespaces == nil {
		s.rawNamespaces = make(map[string]json.RawMessage)
	}
	if s.namespaceCounts == nil {
		s.namespaceCounts = make(map[string]int32)
	}
	if s.unknownNSCounts == nil {
		s.unknownNSCounts = make(map[string]int32)
	}
	for ns, raw := range top {
		s.rawNamespaces[ns] = append(json.RawMessage(nil), raw...)
		s.namespaceCounts[ns]++
		if !babigame.IsModeledNamespace(ns) {
			s.unknownNSCounts[ns]++
		}
	}

	// Capture resource state before NS7 apply for change detection.
	prevGold := s.gold
	prevWaterDrops, prevWaterDropsTotal, prevWaterDropsNext := s.currentWaterDropsLocked(), s.waterDropsTotal, s.waterDropsNextMs
	prevLevel, prevExp, prevVip, prevVipExp := s.level, s.experience, s.vip, s.vipExp
	prevDFree, prevDPaid := s.diamondsFree, s.diamondsPaid
	var prevInventory map[int32]int32
	if _, ok := top["7"]; ok {
		prevInventory = cloneInt32Map(s.inventory)
	}

	if rawNS100, ok := top["100"]; ok {
		var ns map[string]json.RawMessage
		if err := json.Unmarshal(rawNS100, &ns); err == nil {
			changes = append(changes, s.applyLandsLocked(ns)...)
		}
	}
	if rawNS7, ok := top["7"]; ok {
		var ns map[string]json.RawMessage
		if err := json.Unmarshal(rawNS7, &ns); err == nil {
			s.applyInventoryLocked(ns)
		}
	}
	if rawNS114, ok := top["114"]; ok {
		s.applyWaterwheelLocked(rawNS114)
	}
	if rawNS101, ok := top["101"]; ok {
		s.applyCultivationsLocked(rawNS101)
	}
	if rawNS109, ok := top["109"]; ok {
		s.applyCustomerOrdersLocked(rawNS109)
	}
	if rawNS104, ok := top["104"]; ok {
		s.applyFlowerRackLocked(rawNS104)
	}
	if rawNS105, ok := top["105"]; ok {
		s.applyFlowerOrdersLocked(rawNS105)
	}
	if rawNS22, ok := top["22"]; ok {
		s.applyTasksLocked(rawNS22)
	}
	if rawNS117, ok := top["117"]; ok {
		s.applyFreeWaterLocked(rawNS117)
	}
	if rawNS116, ok := top["116"]; ok {
		s.applyBenefitBoxLocked(rawNS116)
	}
	if rawNS119, ok := top["119"]; ok {
		s.applyRoadGrowLocked(rawNS119)
	}
	if rawNS129, ok := top["129"]; ok {
		s.applyRandomEventsLocked(rawNS129)
	}

	resourcesChanged := s.gold != prevGold || s.currentWaterDropsLocked() != prevWaterDrops ||
		s.waterDropsTotal != prevWaterDropsTotal || s.waterDropsNextMs != prevWaterDropsNext || s.level != prevLevel ||
		s.experience != prevExp || s.vip != prevVip || s.vipExp != prevVipExp ||
		s.diamondsFree != prevDFree || s.diamondsPaid != prevDPaid
	var resourceSnap ResourceSnapshot
	var resourceCb func(ResourceSnapshot)
	if resourcesChanged {
		resourceSnap = ResourceSnapshot{
			Gold: s.gold, WaterDrops: s.currentWaterDropsLocked(), WaterDropsTotal: s.waterDropsTotal, WaterDropsNextMs: s.waterDropsNextMs,
			Level: s.level, Experience: s.experience, Vip: s.vip, VipExp: s.vipExp, NobleEligible: s.nobleEligibleLocked(),
			DiamondsFree: s.diamondsFree, DiamondsPaid: s.diamondsPaid,
		}
		resourceCb = s.onResourceChange
	}
	var inventorySnap InventorySnapshot
	var inventoryCb func(InventorySnapshot)
	if prevInventory != nil {
		changes := inventoryChanges(prevInventory, s.inventory)
		if len(changes) > 0 {
			inventorySnap = InventorySnapshot{
				Inventory: cloneInt32Map(s.inventory),
				Changes:   changes,
			}
			inventoryCb = s.onInventoryChange
		}
	}

	cb := s.onChange
	s.mu.Unlock()

	if cb != nil && len(changes) > 0 {
		cb(changes)
	}
	if resourceCb != nil {
		resourceCb(resourceSnap)
	}
	if inventoryCb != nil {
		inventoryCb(inventorySnap)
	}
}

func (s *State) applyLandsLocked(ns100 map[string]json.RawMessage) []LandChange {
	var changes []LandChange
	if raw0, ok := ns100["0"]; ok {
		var s0 map[string]json.RawMessage
		if err := json.Unmarshal(raw0, &s0); err == nil {
			if rawRole, ok := s0["0"]; ok {
				_ = json.Unmarshal(rawRole, &s.roleID)
			}
			if raw1, ok := s0["1"]; ok {
				var roster map[string]json.RawMessage
				if err := json.Unmarshal(raw1, &roster); err == nil {
					s.landRosterObserved = true
					next := make(map[int32]LandView, len(roster))
					for lidStr, rawEntry := range roster {
						lid := atoi32(lidStr)
						if lid < 1000 {
							continue
						}
						var entry map[string]any
						if len(rawEntry) > 0 && string(rawEntry) != "{}" {
							if err := json.Unmarshal(rawEntry, &entry); err != nil {
								continue
							}
						}
						var view LandView
						if len(entry) > 0 {
							view = FromPrimary(entry)
						} else {
							view = EmptyObserved()
						}
						next[lid] = view
						before, existed := s.lands[lid]
						if !existed || before != view {
							changes = append(changes, LandChange{LandID: lid, Before: before, After: view})
						}
					}
					for lid, before := range s.lands {
						if _, ok := next[lid]; !ok {
							changes = append(changes, LandChange{LandID: lid, Before: before, After: LandView{}})
						}
					}
					s.lands = next
				}
			}
		}
	}
	if raw1, ok := ns100["1"]; ok {
		var sub1 map[string]json.RawMessage
		if err := json.Unmarshal(raw1, &sub1); err == nil {
			for lidStr, rawEntry := range sub1 {
				lid := atoi32(lidStr)
				if lid < 1000 {
					continue
				}
				var entry map[string]any
				view := EmptyObserved()
				if len(rawEntry) > 0 && string(rawEntry) != "{}" {
					if err := json.Unmarshal(rawEntry, &entry); err == nil {
						view = FromPrimary(entry)
					}
				}
				if change, ok := s.upsertLandLocked(lid, view, "primary"); ok {
					changes = append(changes, change)
				}
			}
		}
	}
	return changes
}

func (s *State) upsertLandLocked(lid int32, next LandView, _ string) (LandChange, bool) {
	prev, existed := s.lands[lid]
	if existed && prev == next {
		return LandChange{}, false
	}
	s.lands[lid] = next
	return LandChange{LandID: lid, Before: prev, After: next}, true
}

func (s *State) applyInventoryLocked(ns7 map[string]json.RawMessage) {
	if raw0, ok := ns7["0"]; ok {
		var s0 map[string]json.RawMessage
		if err := json.Unmarshal(raw0, &s0); err == nil {
			if cell32, ok := s0["32"]; ok {
				s.applyInventoryCountsLocked(cell32, true)
			}
			if raw33, ok := s0["33"]; ok {
				s.applyWaterDropsLocked(raw33)
			}
			if raw44, ok := s0["44"]; ok {
				var n int32
				if json.Unmarshal(raw44, &n) == nil {
					s.gold = n
				}
			}
			if raw34, ok := s0["34"]; ok {
				var n int32
				if json.Unmarshal(raw34, &n) == nil {
					s.level = n
				}
			}
			if raw35, ok := s0["35"]; ok {
				var n int32
				if json.Unmarshal(raw35, &n) == nil {
					s.experience = n
				}
			}
			if raw36, ok := s0["36"]; ok {
				var n int32
				if json.Unmarshal(raw36, &n) == nil {
					s.vip = n
				}
			}
			if raw37, ok := s0["37"]; ok {
				var n int32
				if json.Unmarshal(raw37, &n) == nil {
					s.vipExp = n
				}
			}
			if raw41, ok := s0["41"]; ok {
				var n int32
				if json.Unmarshal(raw41, &n) == nil {
					s.diamondsFree = n
				}
			}
			if raw42, ok := s0["42"]; ok {
				var n int32
				if json.Unmarshal(raw42, &n) == nil {
					s.diamondsPaid = n
				}
			}
		}
	}

	if raw1, ok := ns7["1"]; ok && !s.hasWaterDropsItem {
		s.applyWaterDropsColdFallbackLocked(raw1)
	}

	if raw2, ok := ns7["2"]; ok {
		s.applyInventoryDeltaLocked(raw2)
	}
}

func (s *State) applyInventoryCountsLocked(raw json.RawMessage, absolute bool) {
	var inv map[string]any
	if err := json.Unmarshal(raw, &inv); err != nil {
		return
	}
	for k, v := range inv {
		id := atoi32(k)
		count := readInt32Any(v)
		if id == 0 {
			continue
		}
		if absolute {
			s.inventory[id] = count
		} else {
			s.inventory[id] += count
		}
		if id == 7 {
			s.hasWaterDropsItem = true
		}
	}
}

func (s *State) applyWaterDropsLocked(raw33 json.RawMessage) {
	var cell33 map[string]json.RawMessage
	if err := json.Unmarshal(raw33, &cell33); err != nil {
		return
	}
	raw7, ok := cell33["7"]
	if !ok {
		return
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(raw7, &inner); err != nil {
		return
	}
	if v, ok := inner["1"]; ok {
		var n int32
		if json.Unmarshal(v, &n) == nil {
			s.waterDropsTotal = n
		}
	}
	if v, ok := inner["5"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.waterDropsNextMs = n
		}
	}
}

func (s *State) applyWaterDropsColdFallbackLocked(raw1 json.RawMessage) {
	var cell1 map[string]json.RawMessage
	if err := json.Unmarshal(raw1, &cell1); err != nil {
		return
	}
	rawCurrent, ok := cell1["13"]
	if !ok {
		return
	}
	var n int32
	if json.Unmarshal(rawCurrent, &n) != nil {
		return
	}
	if s.waterDropsTotal > 0 && n > s.waterDropsTotal {
		return
	}
	s.inventory[7] = n
	s.hasWaterDropsItem = true
}

func (s *State) applyInventoryDeltaLocked(raw2 json.RawMessage) {
	var cell2 map[string]json.RawMessage
	if err := json.Unmarshal(raw2, &cell2); err != nil {
		return
	}
	if rawTotals, ok := cell2["2"]; ok {
		s.applyInventoryCountsLocked(rawTotals, true)
		return
	}
	if rawDelta, ok := cell2["0"]; ok {
		s.applyInventoryCountsLocked(rawDelta, false)
	}
}

func cloneInt32Map(src map[int32]int32) map[int32]int32 {
	dst := make(map[int32]int32, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func inventoryChanges(before, after map[int32]int32) []InventoryItemDelta {
	seen := make(map[int32]struct{}, len(before)+len(after))
	var out []InventoryItemDelta
	for id, prev := range before {
		seen[id] = struct{}{}
		if next := after[id]; next != prev {
			out = append(out, InventoryItemDelta{ItemID: id, Before: prev, After: next})
		}
	}
	for id, next := range after {
		if _, ok := seen[id]; ok {
			continue
		}
		if next != 0 {
			out = append(out, InventoryItemDelta{ItemID: id, After: next})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemID < out[j].ItemID })
	return out
}

func (s *State) applyWaterwheelLocked(raw json.RawMessage) {
	var ns114 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns114); err != nil {
		return
	}
	if v, ok := ns114["1"]; ok {
		var n int32
		if json.Unmarshal(v, &n) == nil {
			s.wwClaimedCount = n
		}
	}
	if v, ok := ns114["4"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.wwLastRecvTs = n
		}
	}
}

func (s *State) applyCultivationsLocked(raw json.RawMessage) {
	var ns101 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns101); err != nil {
		return
	}
	raw0, ok := ns101["0"]
	if !ok {
		return
	}
	var flowers map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &flowers); err != nil {
		return
	}
	for fid, rawEntry := range flowers {
		id := atoi32(fid)
		if id == 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawEntry, &fields); err != nil {
			continue
		}
		cv := s.cultivations[id]
		if cv == nil {
			cv = &CultivateView{FlowerID: id}
			s.cultivations[id] = cv
		}
		if v, ok := fields["2"]; ok {
			var n int32
			if json.Unmarshal(v, &n) == nil {
				cv.Lvl = n
			}
		}
		if v, ok := fields["3"]; ok {
			var n int64
			if json.Unmarshal(v, &n) == nil {
				cv.CulTimeMs = n
			}
		}
		if v, ok := fields["4"]; ok {
			var n int32
			if json.Unmarshal(v, &n) == nil {
				cv.Status = n
			}
		}
		if v, ok := fields["5"]; ok {
			var n int64
			if json.Unmarshal(v, &n) == nil {
				cv.UTimeMs = n
			}
		}
	}
}

func (s *State) applyCustomerOrdersLocked(raw json.RawMessage) {
	var ns109 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns109); err != nil {
		return
	}
	raw0, ok := ns109["0"]
	if !ok {
		return
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &outer); err != nil {
		return
	}
	raw1, ok := outer["1"]
	if !ok {
		return
	}
	var orders map[string]json.RawMessage
	if err := json.Unmarshal(raw1, &orders); err != nil {
		return
	}
	// Replace the full order set.
	// Older captures used fields 0=[[flowerId,count],...], 1=npcId, 3=finishCnt.
	// Current captures use fields 0=dialogId, 1=artId, 2=num, 3=pathId.
	s.customerOrders = make(map[int32]*CustomerOrder, len(orders))
	for npcID, rawOrder := range orders {
		id := atoi32(npcID)
		if id <= 0 {
			continue
		}
		order := &CustomerOrder{NPCID: id}
		storeID := id
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawOrder, &fields); err == nil {
			oldShape := false
			if rawReqs, ok := fields["0"]; ok {
				flowers, items := parseOrderRequires(rawReqs)
				order.Requires = append(order.Requires, flowers...)
				order.ItemRequires = append(order.ItemRequires, items...)
				oldShape = len(flowers) > 0 || len(items) > 0
			}
			if oldShape {
				if rawNPCID, ok := fields["1"]; ok {
					var n int32
					if json.Unmarshal(rawNPCID, &n) == nil && n > 0 {
						order.NPCID = n
						storeID = n
					}
				}
				if rawFinishCnt, ok := fields["3"]; ok {
					var n int32
					if json.Unmarshal(rawFinishCnt, &n) == nil {
						order.FinishCnt = n
					}
				}
				if rawItemID, rawItemOK := fields["1"]; rawItemOK {
					var itemID int32
					var count int32
					_ = json.Unmarshal(rawItemID, &itemID)
					if rawCount, ok := fields["2"]; ok {
						_ = json.Unmarshal(rawCount, &count)
					}
					if itemID > 0 && count > 0 && itemID != order.NPCID {
						order.ItemRequires = append(order.ItemRequires, ItemRequire{ItemID: itemID, Count: count})
					}
				}
			} else if rawItemID, ok := fields["1"]; ok {
				var itemID int32
				var count int32
				_ = json.Unmarshal(rawItemID, &itemID)
				if rawCount, ok := fields["2"]; ok {
					_ = json.Unmarshal(rawCount, &count)
				}
				if itemID > 0 && count > 0 {
					order.ItemRequires = []ItemRequire{{ItemID: itemID, Count: count}}
				}
			}
		}
		s.customerOrders[storeID] = order
	}
}

func (s *State) applyFlowerRackLocked(raw json.RawMessage) {
	var ns104 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns104); err != nil {
		return
	}
	raw0, ok := ns104["0"]
	if !ok {
		return
	}
	var slots map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &slots); err != nil {
		return
	}
	for rackIDStr, rawSlot := range slots {
		rackID := atoi32(rackIDStr)
		if rackID <= 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawSlot, &fields); err != nil {
			continue
		}
		slot := s.flowerRack[rackID]
		if slot == nil {
			slot = &FlowerRackSlot{RackID: rackID}
			s.flowerRack[rackID] = slot
		}
		if rawRackID, ok := fields["1"]; ok {
			var n int32
			if json.Unmarshal(rawRackID, &n) == nil && n > 0 {
				slot.RackID = n
			}
		}
		if rawItemID, ok := fields["2"]; ok {
			var n int32
			if json.Unmarshal(rawItemID, &n) == nil {
				slot.ItemID = n
			}
		}
		if rawCount, ok := fields["3"]; ok {
			var n int32
			if json.Unmarshal(rawCount, &n) == nil {
				slot.Count = n
			}
		}
		if rawListedAt, ok := fields["4"]; ok {
			var n int64
			if json.Unmarshal(rawListedAt, &n) == nil {
				slot.ListedAtMs = n
			}
		}
		if rawUpdatedAt, ok := fields["5"]; ok {
			var n int64
			if json.Unmarshal(rawUpdatedAt, &n) == nil {
				slot.UpdatedAtMs = n
			}
		}
		if slot.ItemID == 0 || slot.Count == 0 {
			slot.ItemID = 0
			slot.Count = 0
		}
	}
}

func (s *State) applyFlowerOrdersLocked(raw json.RawMessage) {
	// NS105 structure: {"0": {"1": {boxId: {order...}}, ...}}
	var ns105 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns105); err != nil {
		return
	}
	raw0, ok := ns105["0"]
	if !ok {
		return
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &inner); err != nil {
		return
	}
	if raw1, ok := inner["1"]; ok {
		var boxes map[string]json.RawMessage
		if err := json.Unmarshal(raw1, &boxes); err == nil {
			s.flowerOrders = make(map[int32]*FlowerOrder, len(boxes))
			for boxIDStr, rawBox := range boxes {
				boxID := atoi32(boxIDStr)
				if boxID == 0 {
					continue
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(rawBox, &fields); err != nil {
					continue
				}
				order := &FlowerOrder{BoxID: boxID}
				if rawMode, ok := fields["0"]; ok {
					_ = json.Unmarshal(rawMode, &order.Mode)
				}
				// field "2" = [[flowerId, count], ...]
				if rawReqs, ok := fields["2"]; ok {
					order.Requires = parseFlowerRequires(rawReqs)
				}
				s.flowerOrders[boxID] = order
			}
		}
	}
	if rawReceived, ok := inner["2"]; ok {
		var ids []int32
		if json.Unmarshal(rawReceived, &ids) == nil {
			s.flowerOrderRewardsReceived = make(map[int32]bool, len(ids))
			for _, id := range ids {
				if id > 0 {
					s.flowerOrderRewardsReceived[id] = true
				}
			}
		}
	}
}

func (s *State) applyRoadGrowLocked(raw json.RawMessage) {
	var ns119 map[string]json.RawMessage
	if json.Unmarshal(raw, &ns119) != nil {
		return
	}
	rawRecv, ok := ns119["3"]
	if !ok {
		return
	}
	var recv map[string]int32
	if json.Unmarshal(rawRecv, &recv) != nil {
		return
	}
	s.roadGrowReceived = make(map[int32]bool, len(recv))
	for id, v := range recv {
		if v != 0 {
			s.roadGrowReceived[atoi32(id)] = true
		}
	}
}

func (s *State) applyRandomEventsLocked(raw json.RawMessage) {
	var ns129 map[string]json.RawMessage
	if json.Unmarshal(raw, &ns129) != nil {
		return
	}
	raw0, ok := ns129["0"]
	if !ok {
		return
	}
	var inner map[string]json.RawMessage
	if json.Unmarshal(raw0, &inner) != nil {
		return
	}
	rawEvents, ok := inner["1"]
	if !ok {
		return
	}
	var events map[string]json.RawMessage
	if json.Unmarshal(rawEvents, &events) != nil {
		return
	}
	s.randomEvents = make(map[int32]*RandomEventView, len(events))
	for idStr, rawEvent := range events {
		id := atoi32(idStr)
		if id == 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(rawEvent, &fields) != nil {
			continue
		}
		event := &RandomEventView{EventID: id}
		if rawID, ok := fields["0"]; ok {
			_ = json.Unmarshal(rawID, &event.EventID)
		}
		if rawStatus, ok := fields["1"]; ok {
			_ = json.Unmarshal(rawStatus, &event.Status)
		}
		if rawAffair, ok := fields["2"]; ok {
			_ = json.Unmarshal(rawAffair, &event.Affair)
		}
		s.randomEvents[id] = event
	}
}

func parseFlowerRequires(raw json.RawMessage) []FlowerRequire {
	flowers, _ := parseOrderRequires(raw)
	return flowers
}

func parseOrderRequires(raw json.RawMessage) ([]FlowerRequire, []ItemRequire) {
	var reqs [][]int32
	if json.Unmarshal(raw, &reqs) != nil {
		return nil, nil
	}
	flowers := make([]FlowerRequire, 0, len(reqs))
	items := make([]ItemRequire, 0, len(reqs))
	for _, req := range reqs {
		if len(req) >= 2 && req[0] > 0 && req[1] > 0 {
			if isFlowerItemID(req[0]) {
				flowers = append(flowers, FlowerRequire{FlowerID: req[0], Count: req[1]})
			} else {
				items = append(items, ItemRequire{ItemID: req[0], Count: req[1]})
			}
		}
	}
	return flowers, items
}

func (s *State) applyTasksLocked(raw json.RawMessage) {
	var ns22 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns22); err != nil {
		return
	}
	if rawMain, ok := ns22["0"]; ok {
		var main map[string]json.RawMessage
		if err := json.Unmarshal(rawMain, &main); err == nil {
			task := &MainTaskView{}
			if rawTaskID, ok := main["1"]; ok {
				var n int32
				if json.Unmarshal(rawTaskID, &n) == nil {
					task.TaskID = n
				}
			}
			if rawFinished, ok := main["2"]; ok {
				var n int32
				if json.Unmarshal(rawFinished, &n) == nil {
					task.Finished = n
				}
			}
			if task.TaskID > 0 {
				s.mainTask = task
			}
		}
	}
	if rawDaily, ok := ns22["1"]; ok {
		s.applyDailyTasksLocked(rawDaily)
	}
	if rawWeekly, ok := ns22["100"]; ok {
		s.applyWeeklyTasksLocked(rawWeekly)
	}
}

func (s *State) applyDailyTasksLocked(rawDaily json.RawMessage) {
	var daily map[string]json.RawMessage
	if err := json.Unmarshal(rawDaily, &daily); err != nil {
		return
	}

	progressMap := readInt32RawMap(daily["1"])
	recvMap := readInt32RawMap(daily["3"])

	rawTaskMap, ok := daily["100"]
	if !ok {
		return
	}
	var tasks map[string]json.RawMessage
	if err := json.Unmarshal(rawTaskMap, &tasks); err != nil {
		return
	}
	s.dailyTasks = make(map[int32]*DailyTaskView, len(tasks))
	for idStr, rawTask := range tasks {
		id := atoi32(idStr)
		if id == 0 {
			continue
		}
		var fields map[string]any
		if err := json.Unmarshal(rawTask, &fields); err != nil {
			continue
		}
		taskID := int32(readInt(fields, "0"))
		if taskID == 0 {
			taskID = id
		}
		finished := readInt32Any(fields["2"])
		if progressType, ok := DailyTaskProgressType(taskID); ok {
			if progress := progressMap[progressType]; progress > finished {
				finished = progress
			}
		}
		s.dailyTasks[id] = &DailyTaskView{
			TaskID:    taskID,
			Target:    readInt32Any(fields["1"]),
			Finished:  finished,
			Status:    readInt32Any(fields["4"]),
			Receipted: recvMap[id],
		}
	}
}

func (s *State) applyWeeklyTasksLocked(rawWeekly json.RawMessage) {
	var weekly map[string]json.RawMessage
	if err := json.Unmarshal(rawWeekly, &weekly); err != nil {
		return
	}
	rawProgress, progressObserved := weekly["1"]
	rawRecv, recvObserved := weekly["3"]
	progressMap := readInt32RawMap(rawProgress)
	recvMap := readInt32RawMap(rawRecv)
	defs := WeeklyTaskDefinitions()
	prev := s.weeklyTasks
	s.weeklyTasks = make(map[int32]*WeeklyTaskView, len(defs))
	for _, def := range defs {
		finished := progressMap[def.ProgressType]
		receipted := recvMap[def.TaskID]
		if old := prev[def.TaskID]; old != nil {
			if !progressObserved {
				finished = old.Finished
			}
			if !recvObserved {
				receipted = old.Receipted
			}
		}
		status := int32(2)
		if receipted != 0 {
			status = 3
		} else if def.Target > 0 && finished >= def.Target {
			status = 1
		}
		s.weeklyTasks[def.TaskID] = &WeeklyTaskView{
			TaskID:    def.TaskID,
			Target:    def.Target,
			Finished:  finished,
			Status:    status,
			Receipted: receipted,
		}
	}
}

func (s *State) applyFreeWaterLocked(raw json.RawMessage) {
	var ns117 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns117); err != nil {
		return
	}
	s.freeWaterObserved = true
	if v, ok := ns117["1"]; ok {
		var n int32
		if json.Unmarshal(v, &n) == nil {
			s.freeWaterRecvIdx = n
		}
	}
	if v, ok := ns117["2"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.freeWaterResetMs = n
		}
	}
}

func (s *State) applyBenefitBoxLocked(raw json.RawMessage) {
	// NS 116 client schema: {"0": {"1": drawCnt, "2": resetCntTime, "3": uTime, "4": cTime}}
	var ns116 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns116); err != nil {
		return
	}
	s.benefitBoxObserved = true
	raw0, ok := ns116["0"]
	if !ok {
		return
	}
	var sub map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &sub); err != nil {
		return
	}
	if v, ok := sub["1"]; ok {
		var n int32
		if json.Unmarshal(v, &n) == nil {
			s.benefitBoxDrawCnt = n
		}
	}
	if v, ok := sub["2"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.benefitBoxResetCntMs = n
		}
	}
	if v, ok := sub["3"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.benefitBoxUTimeMs = n
		}
	}
}

// BenefitBoxDrawsRemaining returns the number of free draws available.
func (s *State) BenefitBoxDrawsRemaining() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.benefitBoxDrawCnt
}

// BenefitBoxReady returns true if there are draws available.
func (s *State) BenefitBoxReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.benefitBoxObserved && s.benefitBoxDrawCnt > 0
}

// Lands returns a copy of the land map.
func (s *State) Lands() map[int32]LandView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]LandView, len(s.lands))
	for k, v := range s.lands {
		out[k] = v
	}
	return out
}

// LandRosterObserved reports whether the cold-start `100.0.1` land roster has
// arrived. Once true, absence from Lands means the server did not include that
// land in the player's opened/owned land list.
func (s *State) LandRosterObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.landRosterObserved
}

// SetFarmLands replaces the per-account runtime c_farmLand view loaded from
// the current client resource pack. This intentionally does not fall back to
// embedded static data, because stale land tables cause wrong unlock decisions.
func (s *State) SetFarmLands(lands []FarmLandInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.farmLands = make(map[int32]FarmLandInfo, len(lands))
	for _, land := range lands {
		if land.ID <= 0 {
			continue
		}
		s.farmLands[land.ID] = cloneFarmLandInfo(land)
	}
	s.farmLandObserved = len(s.farmLands) > 0
}

// FarmLandConfigObserved reports whether the current client-side land table has
// been loaded for this running account session.
func (s *State) FarmLandConfigObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.farmLandObserved
}

// FarmLands returns the runtime c_farmLand rows loaded for this account,
// sorted by id. It returns nil until SetFarmLands succeeds.
func (s *State) FarmLands() []FarmLandInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.farmLandObserved {
		return nil
	}
	out := make([]FarmLandInfo, 0, len(s.farmLands))
	for _, land := range s.farmLands {
		if land.ID > 0 {
			out = append(out, cloneFarmLandInfo(land))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// FarmLand returns one runtime c_farmLand row for this account.
func (s *State) FarmLand(id int32) (FarmLandInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.farmLandObserved {
		return FarmLandInfo{}, false
	}
	land, ok := s.farmLands[id]
	if !ok {
		return FarmLandInfo{}, false
	}
	return cloneFarmLandInfo(land), true
}

// MarkLandsWatered forces the given lands to state=2 (growing) locally and
// spends one local water drop per land when item 7 is tracked. Some successful
// water RPC responses omit inventory deltas, so this keeps the next plan from
// reusing a stale water balance.
func (s *State) MarkLandsWatered(landIDs []int32) {
	now := time.Now()
	s.mu.Lock()
	prevWaterDrops := s.currentWaterDropsLocked()
	prevWaterNextMs := s.waterDropsNextMs
	prevInventory := cloneInt32Map(s.inventory)
	var changes []LandChange
	if nextDrops, nextMs, recovered := s.projectedWaterDropsLocked(now); recovered > 0 {
		s.inventory[7] = nextDrops
		s.waterDropsNextMs = nextMs
	}
	beforeSpend := s.currentWaterDropsLocked()
	for _, id := range landIDs {
		if l, ok := s.lands[id]; ok && l.State == 1 {
			before := l
			l.State = 2
			l.PlantTimeMs = now.UnixMilli()
			s.lands[id] = l
			changes = append(changes, LandChange{LandID: id, Before: before, After: l})
		}
	}
	if s.hasWaterDropsItem && len(landIDs) > 0 {
		spend := int32(len(landIDs))
		if spend >= s.inventory[7] {
			s.inventory[7] = 0
		} else {
			s.inventory[7] -= spend
		}
		if s.waterDropsTotal > 0 && s.inventory[7] < s.waterDropsTotal && (beforeSpend >= s.waterDropsTotal || s.waterDropsNextMs <= 0) {
			s.waterDropsNextMs = now.UnixMilli() + waterDropRestoreIntervalMs()
		}
	}
	resourceSnap := ResourceSnapshot{
		Gold: s.gold, WaterDrops: s.currentWaterDropsLocked(), WaterDropsTotal: s.waterDropsTotal, WaterDropsNextMs: s.waterDropsNextMs,
		Level: s.level, Experience: s.experience, Vip: s.vip, VipExp: s.vipExp, NobleEligible: s.nobleEligibleLocked(),
		DiamondsFree: s.diamondsFree, DiamondsPaid: s.diamondsPaid,
	}
	invChanges := inventoryChanges(prevInventory, s.inventory)
	var inventorySnap InventorySnapshot
	resourceCb := s.onResourceChange
	inventoryCb := s.onInventoryChange
	landCb := s.onChange
	waterChanged := resourceSnap.WaterDrops != prevWaterDrops || resourceSnap.WaterDropsNextMs != prevWaterNextMs
	if len(invChanges) > 0 {
		inventorySnap = InventorySnapshot{Inventory: cloneInt32Map(s.inventory), Changes: invChanges}
	}
	s.mu.Unlock()

	if landCb != nil && len(changes) > 0 {
		landCb(changes)
	}
	if waterChanged && resourceCb != nil {
		resourceCb(resourceSnap)
	}
	if inventoryCb != nil && len(inventorySnap.Changes) > 0 {
		inventoryCb(inventorySnap)
	}
}

// Inventory returns a copy of the inventory map.
func (s *State) Inventory() map[int32]int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]int32, len(s.inventory))
	for k, v := range s.inventory {
		out[k] = v
	}
	return out
}

// FlowerInventory returns only the flower-seed slice of the inventory.
func (s *State) FlowerInventory() map[int32]int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]int32)
	for k, v := range s.inventory {
		if int(k) >= FlowerSeedLow && int(k) < FlowerSeedHigh && v > 0 {
			out[k] = v
		}
	}
	return out
}

// LeastInventoryFlower returns the (id, count) of the lowest-stock flower
// among allowed ids (or all flower seeds if allowed is empty). Returns
// id=0 if the inventory has no flower with positive stock.
func (s *State) LeastInventoryFlower(allowed []int32, blocked []int32) (int32, int32) {
	return s.leastInventoryFlower(allowed, blocked, false)
}

// LeastPlantableFlower returns the lowest-stock flower that is both in
// inventory and has completed cultivation. The server rejects planting flowers
// that still have no successful cultivation record in namespace 101.
func (s *State) LeastPlantableFlower(allowed []int32, blocked []int32) (int32, int32) {
	return s.leastInventoryFlower(allowed, blocked, true)
}

// PlantableFlowers returns cultivated flowers that the server should accept
// for planting, filtered by allow/block lists. Planting does not consume 230xx
// flower inventory, so a cultivated flower with zero stock is still plantable.
func (s *State) PlantableFlowers(allowed []int32, blocked []int32) []PlantableFlower {
	s.mu.RLock()
	defer s.mu.RUnlock()
	allowedSet := setOf(allowed)
	blockedSet := setOf(blocked)
	out := make([]PlantableFlower, 0)
	for id, cv := range s.cultivations {
		if !isPlantableCultivation(cv) || !isFlowerItemID(id) {
			continue
		}
		if len(allowedSet) > 0 {
			if _, ok := allowedSet[id]; !ok {
				continue
			}
		}
		if _, ok := blockedSet[id]; ok {
			continue
		}
		info := catalog.Flowers[id]
		out = append(out, PlantableFlower{
			FlowerID:   id,
			Stock:      s.inventory[id],
			Gold:       info.Gold,
			Experience: info.Experience,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FlowerID < out[j].FlowerID })
	return out
}

func (s *State) leastInventoryFlower(allowed []int32, blocked []int32, requireCultivated bool) (int32, int32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	allowedSet := setOf(allowed)
	blockedSet := setOf(blocked)
	type entry struct {
		id    int32
		count int32
	}
	var candidates []entry
	for id, count := range s.inventory {
		if int(id) < FlowerSeedLow || int(id) >= FlowerSeedHigh {
			continue
		}
		if count <= 0 {
			continue
		}
		if len(allowedSet) > 0 {
			if _, ok := allowedSet[id]; !ok {
				continue
			}
		}
		if _, ok := blockedSet[id]; ok {
			continue
		}
		if requireCultivated && !isPlantableCultivation(s.cultivations[id]) {
			continue
		}
		candidates = append(candidates, entry{id, count})
	}
	if len(candidates) == 0 {
		return 0, 0
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].count != candidates[j].count {
			return candidates[i].count < candidates[j].count
		}
		return candidates[i].id < candidates[j].id
	})
	return candidates[0].id, candidates[0].count
}

func isPlantableCultivation(cv *CultivateView) bool {
	return cv != nil && cv.Status == 2 && cv.Lvl > 0
}

// RoleID returns the cached role id (`100.0.0`).
func (s *State) RoleID() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.roleID
}

func (s *State) currentWaterDropsLocked() int32 {
	if s.hasWaterDropsItem {
		return s.inventory[7]
	}
	return 0
}

func waterDropRestoreIntervalMs() int64 {
	if item, ok := catalog.Items[7]; ok && len(item.Restore) > 0 && item.Restore[0].Count > 0 {
		return int64(item.Restore[0].Count)
	}
	return 120001
}

func (s *State) projectedWaterDropsLocked(now time.Time) (drops int32, nextMs int64, recovered int32) {
	current := s.currentWaterDropsLocked()
	next := s.waterDropsNextMs
	if !s.hasWaterDropsItem || next <= 0 {
		return current, next, 0
	}
	if s.waterDropsTotal > 0 && current >= s.waterDropsTotal {
		return current, 0, 0
	}
	nowMs := now.UnixMilli()
	if nowMs < next {
		return current, next, 0
	}
	interval := waterDropRestoreIntervalMs()
	if interval <= 0 {
		return current, next, 0
	}

	recover := int32((nowMs-next)/interval) + 1
	if s.waterDropsTotal > 0 {
		remaining := s.waterDropsTotal - current
		if remaining <= 0 {
			return current, 0, 0
		}
		if recover > remaining {
			recover = remaining
		}
	}
	current += recover
	next += int64(recover) * interval
	if s.waterDropsTotal > 0 && current >= s.waterDropsTotal {
		next = 0
	}
	return current, next, recover
}

func (s *State) resourceSnapshotLocked() ResourceSnapshot {
	return ResourceSnapshot{
		Gold: s.gold, WaterDrops: s.currentWaterDropsLocked(), WaterDropsTotal: s.waterDropsTotal, WaterDropsNextMs: s.waterDropsNextMs,
		Level: s.level, Experience: s.experience, Vip: s.vip, VipExp: s.vipExp, NobleEligible: s.nobleEligibleLocked(),
		DiamondsFree: s.diamondsFree, DiamondsPaid: s.diamondsPaid,
	}
}

// WaterDrops returns current water drops, total capacity, and the next recovery
// timestamp. Current drops come from inventory item 7 in either the cold
// snapshot (7.0.32["7"]) or absolute inventory deltas (7.2.2["7"]). Some
// cold snapshots omit item 7; in that case 7.1.13 is used only as a bounded
// fallback. 7.0.33.7.1 is the total/capacity, not the current value.
func (s *State) WaterDrops() (int32, int32, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentWaterDropsLocked(), s.waterDropsTotal, s.waterDropsNextMs
}

// AvailableWaterDrops returns the drops that automation may safely attempt to
// spend. It is conservative: when the next recovery timestamp has elapsed but
// the server has not pushed namespace 7 yet, advance the local recovery clock
// with the c_item.restore interval and cap at the server-reported total.
func (s *State) AvailableWaterDrops(now time.Time) (int32, int32, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current, nextMs, _ := s.projectedWaterDropsLocked(now)
	if s.waterDropsTotal > 0 && current > s.waterDropsTotal {
		current = s.waterDropsTotal
	}
	current -= s.waterDropsReserved
	if current < 0 {
		current = 0
	}
	return current, s.waterDropsTotal, nextMs
}

// ReserveWaterDrops marks drops as committed to an in-flight water RPC. This
// keeps concurrent planners from spending them again before the server response
// updates namespace 7.
func (s *State) ReserveWaterDrops(n int32, now time.Time) bool {
	if n <= 0 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, _, _ := s.projectedWaterDropsLocked(now)
	if s.waterDropsTotal > 0 && current > s.waterDropsTotal {
		current = s.waterDropsTotal
	}
	if current-s.waterDropsReserved < n {
		return false
	}
	s.waterDropsReserved += n
	return true
}

// ReleaseWaterDropsReservation releases a previous reservation after the RPC
// fails or after the response has been reconciled into state.
func (s *State) ReleaseWaterDropsReservation(n int32) {
	if n <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n >= s.waterDropsReserved {
		s.waterDropsReserved = 0
		return
	}
	s.waterDropsReserved -= n
}

// RefreshWaterDrops materializes elapsed natural water-drop recovery into
// local state and emits normal resource/inventory callbacks. The next server
// namespace 7 update remains authoritative and can correct the local clock.
func (s *State) RefreshWaterDrops(now time.Time) bool {
	s.mu.Lock()
	next, nextMs, recovered := s.projectedWaterDropsLocked(now)
	if recovered <= 0 {
		s.mu.Unlock()
		return false
	}
	prevInventory := cloneInt32Map(s.inventory)
	s.inventory[7] = next
	s.waterDropsNextMs = nextMs
	resourceSnap := s.resourceSnapshotLocked()
	resourceCb := s.onResourceChange
	var inventorySnap InventorySnapshot
	var inventoryCb func(InventorySnapshot)
	changes := inventoryChanges(prevInventory, s.inventory)
	if len(changes) > 0 {
		inventorySnap = InventorySnapshot{
			Inventory: cloneInt32Map(s.inventory),
			Changes:   changes,
		}
		inventoryCb = s.onInventoryChange
	}
	s.mu.Unlock()

	if resourceCb != nil {
		resourceCb(resourceSnap)
	}
	if inventoryCb != nil {
		inventoryCb(inventorySnap)
	}
	return true
}

// Gold returns the current gold balance (itemId 44).
func (s *State) Gold() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.gold
}

// Level returns the current player level (7.0.34).
func (s *State) Level() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.level
}

// Experience returns the current experience points (7.0.35).
func (s *State) Experience() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.experience
}

// Diamonds returns free and paid diamond balances (7.0.41, 7.0.42).
func (s *State) Diamonds() (free int32, paid int32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.diamondsFree, s.diamondsPaid
}

// Resources returns a snapshot of all tracked resource fields.
func (s *State) Resources() ResourceSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resourceSnapshotLocked()
}

func (s *State) nobleEligibleLocked() bool {
	return s.vip > 0
}

// Vip returns the observed VIP level and experience from namespace 7.
func (s *State) Vip() (level int32, exp int32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vip, s.vipExp
}

// NobleEligible reports whether the account has the observed client-side
// privilege gate needed for noble-only actions such as usrLand.waterOneKey.
func (s *State) NobleEligible() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.nobleEligibleLocked()
}

// ObservedNamespaces returns every v namespace key observed by this state.
func (s *State) ObservedNamespaces() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.namespaceCounts))
	for k := range s.namespaceCounts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// UnknownNamespaceCount returns how many namespace keys have been observed
// without a typed state model.
func (s *State) UnknownNamespaceCount() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int32(len(s.unknownNSCounts))
}

// NamespaceCounts returns a copy of namespace observation counts.
func (s *State) NamespaceCounts() map[string]int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]int32, len(s.namespaceCounts))
	for k, v := range s.namespaceCounts {
		out[k] = v
	}
	return out
}

// WaterwheelClaimedCount returns the total number of waterwheel claims made.
func (s *State) WaterwheelClaimedCount() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.wwClaimedCount
}

// WaterwheelReady is a compatibility accessor used by older diagnostics. It
// returns 1 when the local cooldown view says a claim can be attempted, else 0.
func (s *State) WaterwheelReady() int32 {
	if s.WaterwheelCooldownReady() {
		return 1
	}
	return 0
}

func waterwheelBucketCreateInterval() time.Duration {
	raw, ok := catalog.Tables["c_waterwheel"].Rows["-1"]
	if !ok {
		return time.Hour
	}
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil {
		return time.Hour
	}
	seconds := readInt32Any(row["$bucketCreateCd"])
	if seconds <= 0 {
		return time.Hour
	}
	return time.Duration(seconds) * time.Second
}

func waterwheelBucketDailyMax() int32 {
	raw, ok := catalog.Tables["c_waterwheel"].Rows["-1"]
	if !ok {
		return 0
	}
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil {
		return 0
	}
	return readInt32Any(row["$bucketGetMax"])
}

// WaterwheelCooldownReady returns true if the local bucket-generation clock
// says a waterwheel claim can be attempted. The client config uses
// c_waterwheel.$bucketCreateCd seconds between generated buckets.
func (s *State) WaterwheelCooldownReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if max := waterwheelBucketDailyMax(); max > 0 && s.wwClaimedCount >= max {
		return false
	}
	if s.wwLastRecvTs == 0 {
		return true
	}
	return time.Duration(time.Now().UnixMilli()-s.wwLastRecvTs)*time.Millisecond >= waterwheelBucketCreateInterval()
}

// MaxLandID returns the highest land ID currently tracked.
func (s *State) MaxLandID() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var max int32
	for id := range s.lands {
		if id > max {
			max = id
		}
	}
	return max
}

// FlowerOrders returns the current resident order requirements.
func (s *State) FlowerOrders() map[int32]*FlowerOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]*FlowerOrder, len(s.flowerOrders))
	for k, v := range s.flowerOrders {
		cp := *v
		cp.Requires = append([]FlowerRequire(nil), v.Requires...)
		out[k] = &cp
	}
	return out
}

// FlowerRackSlots returns the current flower-art shelf slots.
func (s *State) FlowerRackSlots() map[int32]FlowerRackSlot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]FlowerRackSlot, len(s.flowerRack))
	for k, v := range s.flowerRack {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}

// EmptyFlowerRackSlotIDs returns observed rack slots with no listed art.
func (s *State) EmptyFlowerRackSlotIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0)
	for rackID, slot := range s.flowerRack {
		if slot == nil || slot.ItemID != 0 || slot.Count != 0 {
			continue
		}
		out = append(out, rackID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyFlowerOrderAdBoxIDs returns resident-order boxes that currently present
// the client as a video/share reward before a concrete order is generated.
func (s *State) ReadyFlowerOrderAdBoxIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0)
	for id, order := range s.flowerOrders {
		if order != nil && order.Mode == 8 && len(order.Requires) == 0 {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyFlowerOrderRewardTargets returns resident-order milestone rewards that
// are claimable from observed daily progress.
func (s *State) ReadyFlowerOrderRewardTargets() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var finished int32
	for _, task := range s.dailyTasks {
		if task != nil && task.TaskID == 30060001 && task.Finished > finished {
			finished = task.Finished
		}
	}
	if finished <= 0 {
		return nil
	}
	thresholds := []int32{15, 30, 45, 60}
	out := make([]int32, 0, len(thresholds))
	for idx, threshold := range thresholds {
		target := int32(idx + 1)
		if finished >= threshold && !s.flowerOrderRewardsReceived[target] {
			out = append(out, target)
		}
	}
	return out
}

// FlowerOrderDeficits returns flower ids whose long-lived requirements are not
// yet covered by current inventory. Customer orders are intentionally excluded:
// they should be completed from current stock/craft capacity or refreshed.
func (s *State) FlowerOrderDeficits() map[int32]int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	needed := make(map[int32]int32)
	addRequires := func(reqs []FlowerRequire) {
		for _, req := range reqs {
			if req.FlowerID == 0 || req.Count <= 0 {
				continue
			}
			needed[req.FlowerID] += req.Count
		}
	}
	for _, order := range s.flowerOrders {
		if order != nil {
			addRequires(order.Requires)
		}
	}
	if s.mainTask != nil {
		if flowerID, missing, ok := MainTaskFlowerRequirement(s.mainTask.TaskID, s.mainTask.Finished); ok {
			needed[flowerID] += missing
		}
	}
	out := make(map[int32]int32)
	for flowerID, count := range needed {
		if have := s.inventory[flowerID]; have < count {
			out[flowerID] = count - have
		}
	}
	return out
}

// Cultivations returns a copy of the cultivation state map.
func (s *State) Cultivations() map[int32]CultivateView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]CultivateView, len(s.cultivations))
	for k, v := range s.cultivations {
		out[k] = *v
	}
	return out
}

// CustomerOrders returns the set of active customer order npcIds.
func (s *State) CustomerOrders() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.customerOrders))
	for id := range s.customerOrders {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// CustomerOrderDetails returns the active customer order requirements.
func (s *State) CustomerOrderDetails() map[int32]*CustomerOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]*CustomerOrder, len(s.customerOrders))
	for k, v := range s.customerOrders {
		if v == nil {
			continue
		}
		cp := *v
		cp.Requires = append([]FlowerRequire(nil), v.Requires...)
		cp.ItemRequires = append([]ItemRequire(nil), v.ItemRequires...)
		out[k] = &cp
	}
	return out
}

// MainTask returns the current main task progress when namespace 22.0 has
// been observed.
func (s *State) MainTask() (MainTaskView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.mainTask == nil {
		return MainTaskView{}, false
	}
	return *s.mainTask, true
}

// ReadyDailyTaskIDs returns daily task ids that look claimable from namespace
// 22. A status of 1 is treated as the client's explicit "ready" marker; when
// status is absent, completed target progress with no receipt is accepted.
func (s *State) ReadyDailyTaskIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.dailyTasks))
	for id, task := range s.dailyTasks {
		if task != nil && taskClaimable(task.Status, task.Target, task.Finished, task.Receipted) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyWeeklyTaskIDs returns weekly task ids that look claimable from namespace
// 22.100 and the current c_task_week table.
func (s *State) ReadyWeeklyTaskIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.weeklyTasks))
	for id, task := range s.weeklyTasks {
		if task != nil && taskClaimable(task.Status, task.Target, task.Finished, task.Receipted) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyRoadGrowTaskIDs returns growth-road rewards that can be claimed from
// the observed player state and client task table.
func (s *State) ReadyRoadGrowTaskIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tasks := RoadGrowLevelTasks()
	out := make([]int32, 0, len(tasks))
	for _, task := range tasks {
		if task.TaskID == 0 || s.roadGrowReceived[task.TaskID] {
			continue
		}
		if task.TargetLevel > 0 && s.level >= task.TargetLevel {
			out = append(out, task.TaskID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// RoadGrowReceived returns a copy of the growth-road receipt map.
func (s *State) RoadGrowReceived() map[int32]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]bool, len(s.roadGrowReceived))
	for id, v := range s.roadGrowReceived {
		out[id] = v
	}
	return out
}

// ReadyRandomEventIDs returns map random events whose status is actionable.
func (s *State) ReadyRandomEventIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.randomEvents))
	for id, event := range s.randomEvents {
		if event != nil && (event.Status == 0 || event.Status == 1) {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// RandomEvents returns the current map-random-event state.
func (s *State) RandomEvents() map[int32]RandomEventView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]RandomEventView, len(s.randomEvents))
	for id, event := range s.randomEvents {
		if event != nil {
			out[id] = *event
		}
	}
	return out
}

// DailyTasks returns a copy of tracked daily task progress.
func (s *State) DailyTasks() map[int32]DailyTaskView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]DailyTaskView, len(s.dailyTasks))
	for id, task := range s.dailyTasks {
		if task != nil {
			out[id] = *task
		}
	}
	return out
}

// WeeklyTasks returns a copy of tracked weekly task progress.
func (s *State) WeeklyTasks() map[int32]WeeklyTaskView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]WeeklyTaskView, len(s.weeklyTasks))
	for id, task := range s.weeklyTasks {
		if task != nil {
			out[id] = *task
		}
	}
	return out
}

// NextFreeWaterIndex returns the next candidate idx for freeWater.recv.
// The static client schema exposes IFreeWater.recvIdx and the RPC argument
// is also named idx, so use the observed index directly and let the server
// response advance it.
func (s *State) NextFreeWaterIndex() (int32, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.freeWaterObserved {
		return 0, false
	}
	return s.freeWaterRecvIdx, true
}

func setOf(ids []int32) map[int32]struct{} {
	if len(ids) == 0 {
		return nil
	}
	m := make(map[int32]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}

func taskClaimable(status, target, finished, receipted int32) bool {
	if receipted != 0 {
		return false
	}
	return status == 1 || (status == 0 && target > 0 && finished >= target)
}

func readInt32RawMap(raw json.RawMessage) map[int32]int32 {
	out := map[int32]int32{}
	if len(raw) == 0 {
		return out
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return out
	}
	for key, rawValue := range values {
		id := atoi32(key)
		if id == 0 {
			continue
		}
		var n int32
		if json.Unmarshal(rawValue, &n) == nil {
			out[id] = n
		}
	}
	return out
}

func readInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if i := readInt32Any(v); i != 0 {
				return int(i)
			}
		}
	}
	return 0
}

func readInt64(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case float64:
				return int64(x)
			case int:
				return int64(x)
			case int64:
				return x
			case json.Number:
				i, _ := x.Int64()
				return i
			}
		}
	}
	return 0
}

func readInt32Any(v any) int32 {
	switch x := v.(type) {
	case float64:
		return int32(x)
	case int:
		return int32(x)
	case int32:
		return x
	case int64:
		return int32(x)
	case json.Number:
		i, _ := x.Int64()
		return int32(i)
	}
	return 0
}

func atoi32(s string) int32 {
	var n int32
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int32(c-'0')
	}
	return n
}
