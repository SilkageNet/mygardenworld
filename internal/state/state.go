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
	"sync"
	"sync/atomic"
)

// State is the per-account in-memory tracker.
type State struct {
	mu       sync.RWMutex
	revision atomic.Uint64
	protocolState
	gardenState
	resourceState
	orderState
	unionState
	activityState
	taskState
	socialState
	assetState
	hooksState
}

// Revision returns the monotonic state generation. Callers can compare values
// around a read-only computation and retry when a concurrent mutation lands.
func (s *State) Revision() uint64 {
	if s == nil {
		return 0
	}
	return s.revision.Load()
}

func (s *State) bumpRevisionLocked() {
	s.revision.Add(1)
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
	s := &State{}
	s.lands = make(map[int32]LandView)
	s.farmLands = make(map[int32]FarmLandInfo)
	s.inventory = make(map[int32]int32)
	s.rawNamespaces = make(map[string]json.RawMessage)
	s.namespaceCounts = make(map[string]int32)
	s.unknownNSCounts = make(map[string]int32)
	s.cultivations = make(map[int32]*CultivateView)
	s.customerOrders = make(map[int32]*CustomerOrder)
	s.flowerRack = make(map[int32]*FlowerRackSlot)
	s.mails = make(map[string]*MailView)
	s.shops = make(map[int32]*shopState)
	s.vases = make(map[int32]*VaseView)
	s.collectRewards = make(map[int32]*CollectRewardView)
	s.fmlBuild = FmlBuildView{BuildCounts: make(map[int32]int32)}
	s.fmlLands = make(map[int32]*FmlLandView)
	s.fmlForestEnergy = FmlForestEnergyView{EnergyByType: make(map[int32]int32), DailyEnergyByType: make(map[int32]int32), PendingTempEnergyByType: make(map[int32]int32)}
	s.fmlFlowerShare = FmlFlowerShareView{Slots: make(map[int32]FmlFlowerShareSlotView)}
	s.fmlOtherFlowerShares = make(map[int64]*FmlFlowerShareView)
	s.shopGiftbagDRecord = make(map[int32]int32)
	s.shopGiftbagWRecord = make(map[int32]int32)
	s.shopGiftbagMRecord = make(map[int32]int32)
	s.shopGiftbagTRecord = make(map[int32]int32)
	s.shopGiftbagBuyTimeRecord = make(map[int32]int64)
	s.shareUsages = make(map[int32]ShareUsageView)
	s.shopCultivateCosts = make(map[int32]ItemCount)
	s.shopCultivateBought = make(map[int32]int32)
	s.pearlPlaces = make(map[int32]*PearlPlaceView)
	s.pearlFriendRelations = make(map[string]pearlFriendRelation)
	s.pearlProfiles = make(map[int64]*PearlCandidateProfile)
	s.pearlHireStates = make(map[int64]*PearlCandidateHireState)
	s.pearlEnemies = make(map[int64]int64)
	s.pearlHireFailedUntil = make(map[int64]int64)
	s.flowerOrders = make(map[int32]*FlowerOrder)
	s.flowerOrderRewardsReceived = make(map[int32]bool)
	s.dailyTasks = make(map[int32]*DailyTaskView)
	s.weeklyTasks = make(map[int32]*WeeklyTaskView)
	s.achievementTasks = make(map[int32]*AchievementTaskView)
	s.activityBatches = make(map[int32]*activityBatchState)
	s.activityTemplates = make(map[int32]*activityTemplateState)
	s.activityTaskRecords = make(map[string]*activityTaskRecordState)
	s.roadGrowReceived = make(map[int32]bool)
	s.randomEvents = make(map[int32]*RandomEventView)
	s.signTypes = make(map[int32]*SignTypeView)
	s.baseRewards = make(map[int32]*BaseRewardView)
	s.signTypeEnterAtMs = make(map[int32]int64)
	s.statisticsByDay = make(map[int32]StatisticsView)
	s.zooPets = make(map[int32]*ZooPetView)
	s.zooLogs = make(map[string]*ZooLogView)
	s.zooSouvenirs = make(map[int32]*ZooSouvenirView)
	return s
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
