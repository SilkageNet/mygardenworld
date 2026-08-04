package state

import (
	"encoding/json"
	"sort"
	"time"
)

func (s *State) applyInventoryLocked(ns7 map[string]json.RawMessage) {
	if raw0, ok := ns7["0"]; ok {
		var s0 map[string]json.RawMessage
		if err := json.Unmarshal(raw0, &s0); err == nil {
			if rawRoleID, ok := s0["0"]; ok {
				if uid, valid := readExactInt64Raw(rawRoleID); valid && uid > 0 && (s.roleID == 0 || s.roleID == uid) {
					s.roleID = uid
				}
			}
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

// MarkWaterDropsExhausted reconciles local state after the server rejects a
// water RPC for item 7. The next authoritative namespace 7 update can still
// correct the count or recovery clock.
func (s *State) MarkWaterDropsExhausted(now time.Time) {
	s.mu.Lock()
	prevWaterDrops := s.currentWaterDropsLocked()
	prevWaterNextMs := s.waterDropsNextMs
	prevInventory := cloneInt32Map(s.inventory)
	s.hasWaterDropsItem = true
	s.inventory[7] = 0
	s.waterDropsInFlight = 0
	if s.waterDropsTotal > 0 {
		next := now.UnixMilli() + waterDropRestoreIntervalMs()
		if s.waterDropsNextMs <= now.UnixMilli() || s.waterDropsNextMs == 0 {
			s.waterDropsNextMs = next
		}
	}
	resourceSnap := s.resourceSnapshotLocked()
	invChanges := inventoryChanges(prevInventory, s.inventory)
	var inventorySnap InventorySnapshot
	resourceCb := s.onResourceChange
	inventoryCb := s.onInventoryChange
	waterChanged := resourceSnap.WaterDrops != prevWaterDrops || resourceSnap.WaterDropsNextMs != prevWaterNextMs
	if len(invChanges) > 0 {
		inventorySnap = InventorySnapshot{Inventory: cloneInt32Map(s.inventory), Changes: invChanges}
	}
	s.mu.Unlock()

	if waterChanged && resourceCb != nil {
		resourceCb(resourceSnap)
	}
	if inventoryCb != nil && len(inventorySnap.Changes) > 0 {
		inventoryCb(inventorySnap)
	}
}

// MarkInventoryItemExhausted reconciles local inventory after the server rejects
// an RPC for material shortage (code 301 + param.iid). Zeroing the stale local
// count prevents the planner from reissuing the same craft until an
// authoritative namespace-7 update restores stock.
func (s *State) MarkInventoryItemExhausted(itemID int32) {
	if itemID <= 0 {
		return
	}
	s.mu.Lock()
	prevInventory := cloneInt32Map(s.inventory)
	if prevInventory[itemID] <= 0 {
		s.mu.Unlock()
		return
	}
	s.inventory[itemID] = 0
	if itemID == 7 {
		s.hasWaterDropsItem = true
		s.waterDropsInFlight = 0
	}
	invChanges := inventoryChanges(prevInventory, s.inventory)
	inventorySnap := InventorySnapshot{Inventory: cloneInt32Map(s.inventory), Changes: invChanges}
	inventoryCb := s.onInventoryChange
	s.mu.Unlock()

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
			Lvl:        cv.Lvl,
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

// RoleID returns the cached own UID observed at `7.0.0`, `100.0.0`, or
// `115.1.0`.
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
// spend. When the next recovery timestamp has elapsed but the server has not
// pushed namespace 7 yet, advance the local recovery clock with the
// c_item.restore interval (projected recovery never exceeds capacity). Inventory
// that already exceeds waterDropsTotal — e.g. from packs — remains fully
// spendable; only projected recovery is capacity-capped.
func (s *State) AvailableWaterDrops(now time.Time) (int32, int32, int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current, nextMs, _ := s.projectedWaterDropsLocked(now)
	current -= s.waterDropsInFlight
	if current < 0 {
		current = 0
	}
	return current, s.waterDropsTotal, nextMs
}

// LockWaterDrops marks drops as committed to an in-flight water RPC. This
// keeps concurrent planners from spending them again before the server response
// updates namespace 7.
func (s *State) LockWaterDrops(n int32, now time.Time) bool {
	if n <= 0 {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, _, _ := s.projectedWaterDropsLocked(now)
	if current-s.waterDropsInFlight < n {
		return false
	}
	s.waterDropsInFlight += n
	return true
}

// ReleaseWaterDropsLock releases a previous in-flight lock after the RPC
// fails or after the response has been reconciled into state.
func (s *State) ReleaseWaterDropsLock(n int32) {
	if n <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if n >= s.waterDropsInFlight {
		s.waterDropsInFlight = 0
		return
	}
	s.waterDropsInFlight -= n
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

// Diamonds returns visible and secondary diamond balances (7.0.41, 7.0.42).
func (s *State) Diamonds() (visible int32, paid int32) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.diamondsFree, s.diamondsPaid
}

// SpendableDiamonds returns the 元宝 balance shown by the game client.
// Observed clients display namespace 7.0.41; namespace 7.0.42 is tracked
// separately and must not be added to the visible/spendable balance.
func (s *State) SpendableDiamonds() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.diamondsFree
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
