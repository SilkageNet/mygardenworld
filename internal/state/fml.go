package state

import (
	"encoding/json"
	"sort"
)

func (s *State) applyFmlLocked(raw json.RawMessage) {
	var ns25 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns25); err != nil {
		return
	}
	s.fmlBuild.Observed = true
	if s.fmlBuild.BuildCounts == nil {
		s.fmlBuild.BuildCounts = make(map[int32]int32)
	}
	if rawFml, ok := ns25["0"]; ok {
		s.applyFmlObjectLocked(rawFml)
	}
	if rawBuild, ok := ns25["133"]; ok {
		s.applyFmlBuildObjectLocked(rawBuild)
	}
	if rawLand, ok := ns25["102"]; ok {
		s.applyFmlLandObjectLocked(rawLand)
	}
	if rawForestEnergy, ok := ns25["127"]; ok {
		s.applyFmlForestEnergyObjectLocked(rawForestEnergy)
	}
	if rawShare, ok := ns25["107"]; ok {
		if view, ok := parseFmlFlowerShare(rawShare); ok {
			s.fmlFlowerShare = view
		}
	}
	if rawOtherShares, ok := ns25["108"]; ok {
		s.applyOtherFmlFlowerSharesObjectLocked(rawOtherShares)
	}
}

func (s *State) applyFmlObjectLocked(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if id, ok := readInt32JSONField(fields, "0"); ok {
		s.fmlBuild.FmlID = id
	}
	if count, ok := readInt32JSONField(fields, "19", "113"); ok {
		s.fmlBuild.TodayBuildNum = count
	}
	if ts, ok := readInt64JSONField(fields, "20", "29"); ok {
		s.fmlBuild.LastBuildTimeMs = ts
	}
	if rawCounts, ok := fields["30"]; ok {
		s.setFmlBuildCountsLocked(rawCounts)
	}
}

func (s *State) applyFmlBuildObjectLocked(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if id, ok := readInt32JSONField(fields, "1"); ok {
		s.fmlBuild.FmlID = id
	}
	if ts, ok := readInt64JSONField(fields, "4"); ok {
		s.fmlBuild.LastBuildTimeMs = ts
	}
	if rawCounts, ok := fields["5"]; ok {
		s.setFmlBuildCountsLocked(rawCounts)
	}
}

func (s *State) setFmlBuildCountsLocked(raw json.RawMessage) {
	counts := readInt32RawMap(raw)
	s.fmlBuild.BuildCountsObserved = true
	s.fmlBuild.BuildCounts = counts
}

func (s *State) applyFmlLandObjectLocked(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	rawLandMap, ok := fields["1"]
	if !ok {
		return
	}
	var landMap map[string]json.RawMessage
	if err := json.Unmarshal(rawLandMap, &landMap); err != nil {
		return
	}
	next := make(map[int32]*FmlLandView, len(landMap))
	for landIDStr, rawLand := range landMap {
		landID := atoi32(landIDStr)
		if landID <= 0 {
			continue
		}
		view := &FmlLandView{LandID: landID}
		if len(rawLand) > 0 && string(rawLand) != "{}" {
			var landFields map[string]json.RawMessage
			if err := json.Unmarshal(rawLand, &landFields); err == nil {
				if n, ok := readInt32JSONField(landFields, "0"); ok {
					view.Level = n
				}
				if n, ok := readInt32JSONField(landFields, "1"); ok {
					view.FlowerID = n
				}
				if n, ok := readInt64JSONField(landFields, "2"); ok {
					view.StartTimeMs = n
				}
				if n, ok := readInt32JSONField(landFields, "3"); ok {
					view.MatureFlowerCnt = n
				}
				if n, ok := readInt32JSONField(landFields, "4"); ok {
					view.HarvestedCnt = n
				}
				if n, ok := readInt64JSONField(landFields, "5"); ok {
					view.LastCalcTimeMs = n
				}
			}
		}
		next[landID] = view
	}
	s.fmlLands = next
	s.fmlLandObserved = true
}

func (s *State) applyFmlForestEnergyObjectLocked(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	view := FmlForestEnergyView{Observed: true}
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.FmlID = n
	}
	if rawEnergy, ok := fields["2"]; ok {
		view.EnergyByType = readInt32RawMap(rawEnergy)
	}
	if rawDaily, ok := fields["6"]; ok {
		view.DailyEnergyByType = readInt32RawMap(rawDaily)
	}
	if n, ok := readInt64JSONField(fields, "4"); ok {
		view.UpdatedAtMs = n
	}
	if n, ok := readInt64JSONField(fields, "7"); ok {
		view.LastDailyRefreshTimeMs = n
	}
	if rawTemp, ok := fields["8"]; ok {
		view.PendingTempEnergyByType, view.PendingTempEnergyTotal = readNestedInt32RawMapTotals(rawTemp)
	}
	if view.EnergyByType == nil {
		view.EnergyByType = map[int32]int32{}
	}
	if view.DailyEnergyByType == nil {
		view.DailyEnergyByType = map[int32]int32{}
	}
	if view.PendingTempEnergyByType == nil {
		view.PendingTempEnergyByType = map[int32]int32{}
	}
	s.fmlForestEnergy = view
}

func (s *State) applyOtherFmlFlowerSharesObjectLocked(raw json.RawMessage) {
	next := make(map[int64]*FmlFlowerShareView)
	if len(raw) == 0 || string(raw) == "null" {
		s.fmlOtherFlowerShares = next
		s.fmlOtherShareObserved = true
		return
	}
	var list []json.RawMessage
	if err := json.Unmarshal(raw, &list); err == nil {
		for _, rawShare := range list {
			view, ok := parseFmlFlowerShare(rawShare)
			if !ok || view.UID == 0 {
				continue
			}
			cp := view
			next[view.UID] = &cp
		}
		s.fmlOtherFlowerShares = next
		s.fmlOtherShareObserved = true
		return
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return
	}
	for uidStr, rawShare := range values {
		view, ok := parseFmlFlowerShare(rawShare)
		if !ok {
			continue
		}
		if view.UID == 0 {
			view.UID = atoi64(uidStr)
		}
		if view.UID == 0 {
			continue
		}
		cp := view
		next[view.UID] = &cp
	}
	s.fmlOtherFlowerShares = next
	s.fmlOtherShareObserved = true
}

func parseFmlFlowerShare(raw json.RawMessage) (FmlFlowerShareView, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return FmlFlowerShareView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return FmlFlowerShareView{}, false
	}
	view := FmlFlowerShareView{Observed: true, Slots: make(map[int32]FmlFlowerShareSlotView)}
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if rawSlots, ok := fields["1"]; ok {
		view.Slots = parseFmlFlowerShareSlots(rawSlots)
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.TdyTakeCnt = n
	}
	if n, ok := readInt64JSONField(fields, "3"); ok {
		view.LastTakeTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "4"); ok {
		view.UpdatedAtMs = n
	}
	if n, ok := readInt64JSONField(fields, "5"); ok {
		view.CreatedAtMs = n
	}
	return view, true
}

func parseFmlFlowerShareSlots(raw json.RawMessage) map[int32]FmlFlowerShareSlotView {
	out := make(map[int32]FmlFlowerShareSlotView)
	if len(raw) == 0 || string(raw) == "null" {
		return out
	}
	var slots map[string]json.RawMessage
	if err := json.Unmarshal(raw, &slots); err != nil {
		return out
	}
	for slotIDStr, rawSlot := range slots {
		slotID := atoi32(slotIDStr)
		if slotID <= 0 {
			continue
		}
		slot := FmlFlowerShareSlotView{SlotID: slotID}
		if len(rawSlot) > 0 && string(rawSlot) != "{}" {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(rawSlot, &fields); err == nil {
				if n, ok := readInt32JSONField(fields, "0"); ok {
					slot.FlowerID = n
				}
				if n, ok := readInt32JSONField(fields, "1"); ok {
					slot.ShareNum = n
				}
				if n, ok := readInt32JSONField(fields, "2"); ok {
					slot.TakeNum = n
				}
				if n, ok := readInt64JSONField(fields, "3"); ok {
					slot.ShareStartTimeMs = n
				}
			}
		}
		out[slotID] = slot
	}
	return out
}

func cloneFmlFlowerShareView(src FmlFlowerShareView) FmlFlowerShareView {
	out := src
	out.Slots = make(map[int32]FmlFlowerShareSlotView, len(src.Slots))
	for slotID, slot := range src.Slots {
		out.Slots[slotID] = slot
	}
	return out
}

// FmlBuild returns the tracked namespace 25 guild-build state.
func (s *State) FmlBuild() FmlBuildView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.fmlBuild
	out.BuildCounts = cloneInt32Map(out.BuildCounts)
	return out
}

// FmlBuildObserved reports whether namespace 25 has been observed.
func (s *State) FmlBuildObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlBuild.Observed
}

// FmlLandObserved reports whether namespace 25.102 has been observed.
func (s *State) FmlLandObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlLandObserved
}

// FmlLands returns a defensive copy of observed guild lands.
func (s *State) FmlLands() map[int32]FmlLandView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]FmlLandView, len(s.fmlLands))
	for id, land := range s.fmlLands {
		if land == nil {
			continue
		}
		out[id] = *land
	}
	return out
}

// ReadyFmlLandHarvestIDs returns guild lands with unclaimed mature flowers.
func (s *State) ReadyFmlLandHarvestIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.fmlLands))
	for id, land := range s.fmlLands {
		if land == nil || land.MatureFlowerCnt <= land.HarvestedCnt {
			continue
		}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// FmlForestEnergy returns the tracked forest-energy state.
func (s *State) FmlForestEnergy() FmlForestEnergyView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.fmlForestEnergy
	out.EnergyByType = cloneInt32Map(out.EnergyByType)
	out.DailyEnergyByType = cloneInt32Map(out.DailyEnergyByType)
	out.PendingTempEnergyByType = cloneInt32Map(out.PendingTempEnergyByType)
	return out
}

// FmlForestEnergyObserved reports whether namespace 25.127 has been observed.
func (s *State) FmlForestEnergyObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlForestEnergy.Observed
}

// ReadyFmlForestEnergyTypes returns energy types with pending temporary energy.
func (s *State) ReadyFmlForestEnergyTypes() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.fmlForestEnergy.PendingTempEnergyByType))
	for typ, count := range s.fmlForestEnergy.PendingTempEnergyByType {
		if count > 0 {
			out = append(out, typ)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// FmlFlowerShareObserved reports whether namespace 25.107 has been observed.
func (s *State) FmlFlowerShareObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlFlowerShare.Observed
}

// FmlFlowerShare returns a defensive copy of the account's own guild share.
func (s *State) FmlFlowerShare() FmlFlowerShareView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneFmlFlowerShareView(s.fmlFlowerShare)
}

// OtherFmlFlowerSharesObserved reports whether namespace 25.108 has been observed.
func (s *State) OtherFmlFlowerSharesObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlOtherShareObserved
}

// OtherFmlFlowerShares returns defensive copies of member guild shares.
func (s *State) OtherFmlFlowerShares() map[int64]FmlFlowerShareView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int64]FmlFlowerShareView, len(s.fmlOtherFlowerShares))
	for uid, share := range s.fmlOtherFlowerShares {
		if share == nil {
			continue
		}
		out[uid] = cloneFmlFlowerShareView(*share)
	}
	return out
}

// ReadyFmlFlowerShareRewardSlotIDs returns own share slots with take rewards.
func (s *State) ReadyFmlFlowerShareRewardSlotIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.fmlFlowerShare.Slots))
	for slotID, slot := range s.fmlFlowerShare.Slots {
		if slot.FlowerID > 0 && slot.TakeNum > 0 {
			out = append(out, slotID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// FmlFlowerTakeCandidates returns member share slots that still have flowers.
func (s *State) FmlFlowerTakeCandidates() []FmlFlowerTakeCandidate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]FmlFlowerTakeCandidate, 0)
	for uid, share := range s.fmlOtherFlowerShares {
		if share == nil {
			continue
		}
		actualUID := share.UID
		if actualUID == 0 {
			actualUID = uid
		}
		if actualUID == 0 {
			continue
		}
		for slotID, slot := range share.Slots {
			available := slot.ShareNum - slot.TakeNum
			if slot.FlowerID <= 0 || available <= 0 {
				continue
			}
			out = append(out, FmlFlowerTakeCandidate{
				UID:       actualUID,
				SlotID:    slotID,
				FlowerID:  slot.FlowerID,
				ShareNum:  slot.ShareNum,
				TakeNum:   slot.TakeNum,
				Available: available,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FlowerID != out[j].FlowerID {
			return out[i].FlowerID < out[j].FlowerID
		}
		if out[i].UID != out[j].UID {
			return out[i].UID < out[j].UID
		}
		return out[i].SlotID < out[j].SlotID
	})
	return out
}
