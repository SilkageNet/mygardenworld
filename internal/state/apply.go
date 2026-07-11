package state

import (
	"encoding/json"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

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
			s.applyUsrExtraLocked(ns)
			s.applyReputationLocked(ns)
			s.applyStoryMainLocked(ns)
		}
	}
	if rawNS19, ok := top["19"]; ok {
		s.applyMailLocked(rawNS19)
	}
	if rawNS114, ok := top["114"]; ok {
		s.applyWaterwheelLocked(rawNS114)
	}
	if rawNS115, ok := top["115"]; ok {
		s.applyPearlLocked(rawNS115)
	}
	if rawNS24, ok := top["24"]; ok {
		s.applyPearlFriendsLocked(rawNS24)
	}
	if rawNS28, ok := top["28"]; ok {
		s.applyPearlProfilesLocked(rawNS28)
	}
	if rawNS101, ok := top["101"]; ok {
		s.applyCultivationsLocked(rawNS101)
	}
	if rawNS102, ok := top["102"]; ok {
		s.applyVasesLocked(rawNS102)
	}
	if rawNS103, ok := top["103"]; ok {
		s.applyCollectRewardsLocked(rawNS103)
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
	if rawNS106, ok := top["106"]; ok {
		s.applyFlowerArtLocked(rawNS106)
	}
	if rawNS107, ok := top["107"]; ok {
		s.applyTeamOrderLocked(rawNS107)
	}
	if rawNS108, ok := top["108"]; ok {
		s.applyPalaceOrderLocked(rawNS108)
	}
	if rawNS25, ok := top["25"]; ok {
		s.applyFmlLocked(rawNS25)
	}
	if rawNS112, ok := top["112"]; ok {
		s.applyShopGiftbagLocked(rawNS112)
	}
	if rawNS113, ok := top["113"]; ok {
		s.applyShopCultivateLocked(rawNS113)
	}
	if rawNS22, ok := top["22"]; ok {
		s.applyTasksLocked(rawNS22)
	}
	if rawNS23, ok := top["23"]; ok {
		s.applyActivitiesLocked(rawNS23)
	}
	if rawNS117, ok := top["117"]; ok {
		s.applyFreeWaterLocked(rawNS117)
	}
	if rawNS116, ok := top["116"]; ok {
		s.applyBenefitBoxLocked(rawNS116)
	}
	if rawNS118, ok := top["118"]; ok {
		s.applyVideoDoubleLocked(rawNS118)
	}
	if rawNS119, ok := top["119"]; ok {
		s.applyRoadGrowLocked(rawNS119)
	}
	if rawNS124, ok := top["124"]; ok {
		s.applyStatisticsLocked(rawNS124)
	}
	if rawNS129, ok := top["129"]; ok {
		s.applyRandomEventsLocked(rawNS129)
	}
	if rawNS33, ok := top["33"]; ok {
		s.applyZooLocked(rawNS33)
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
