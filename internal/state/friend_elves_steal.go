package state

import "time"

func (s *State) MarkFriendElvesSkipEnter(uid int64, until time.Time) {
	if s == nil || uid <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.frdStealElvesSkipEnterUntil == nil {
		s.frdStealElvesSkipEnterUntil = make(map[int64]int64)
	}
	s.frdStealElvesSkipEnterUntil[uid] = until.UnixMilli()
}

func (s *State) FriendElvesSkipEnter(uid int64, now time.Time) bool {
	if s == nil || uid <= 0 {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.frdStealElvesSkipEnterUntil[uid] > now.UnixMilli()
}

func (s *State) ClearFriendElvesSkipEnter(uid int64) {
	if s == nil || uid <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.frdStealElvesSkipEnterUntil, uid)
}

// NoteFriendStealElvesSuccess reconciles a stealElves=1 success that may omit
// IFrdSteal.stealElvesCnt / land.elvesStealUids. Client still consumes the
// per-friend flower-steal quota (getStealCntLeftNumByFrdUid).
func (s *State) NoteFriendStealElvesSuccess(uid int64, landID int32, usedBefore int32, usedBeforeObserved bool, elvesCntBefore int32, elvesCntBeforeObserved bool, now time.Time) {
	if s == nil || uid <= 0 || landID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.noteFriendStealUsedLocked(uid, usedBefore, usedBeforeObserved, now)
	if elvesCntBeforeObserved && s.frdStealObserved && frdStealMapFresh(s.frdStealRTimeMs, now) {
		if minimum := elvesCntBefore + 1; s.frdStealElvesCnt < minimum {
			s.frdStealElvesCnt = minimum
		}
	}
	if s.frdVisitUID != uid {
		return
	}
	land, exists := s.frdVisitLands[landID]
	if !exists || s.roleID <= 0 || int64SliceContains(land.ElvesStealUIDs, s.roleID) {
		return
	}
	land.ElvesStealUIDs = append(append([]int64(nil), land.ElvesStealUIDs...), s.roleID)
	s.frdVisitLands[landID] = land
}

// MarkFriendStealElvesLandUnavailable sticky-skips a friend land for elf steals
// until that plot's plantTime advances (replant). frdHome refreshes that still
// show elvesId with empty elvesStealUids must not revive a server-rejected plot.
func (s *State) MarkFriendStealElvesLandUnavailable(friendUID int64, landID int32) {
	if s == nil || friendUID <= 0 || landID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	plantTime := int64(0)
	if s.frdVisitUID == friendUID {
		if land, ok := s.frdVisitLands[landID]; ok {
			plantTime = land.PlantTimeMs
			// Also hide it on the current visit snapshot so the next plan tick
			// can advance without waiting for another enter.
			if s.roleID > 0 && !int64SliceContains(land.ElvesStealUIDs, s.roleID) {
				land.ElvesStealUIDs = append(append([]int64(nil), land.ElvesStealUIDs...), s.roleID)
				s.frdVisitLands[landID] = land
			} else if s.roleID <= 0 {
				land.ElvesID = 0
				s.frdVisitLands[landID] = land
			}
		}
	}
	if s.frdStealElvesSkipPlantTime == nil {
		s.frdStealElvesSkipPlantTime = make(map[int64]map[int32]int64)
	}
	byLand := s.frdStealElvesSkipPlantTime[friendUID]
	if byLand == nil {
		byLand = make(map[int32]int64)
		s.frdStealElvesSkipPlantTime[friendUID] = byLand
	}
	byLand[landID] = plantTime
}

// FriendStealElvesLandSkipped reports a sticky elf-steal skip for friend+land
// that still matches the observed plantTime (0 matches an unknown plantTime).
func (s *State) FriendStealElvesLandSkipped(friendUID int64, landID int32, plantTimeMs int64) bool {
	if s == nil || friendUID <= 0 || landID <= 0 {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	byLand := s.frdStealElvesSkipPlantTime[friendUID]
	if byLand == nil {
		return false
	}
	skippedAt, ok := byLand[landID]
	if !ok {
		return false
	}
	return skippedAt == plantTimeMs
}

// FriendStealElvesActuallyStolen reports whether a stealElves=1 response landed
// an elf (cnt bump, elvesStealUids, or elves inventory gain) rather than an
// ordinary flower steal that still returned ok.
func (s *State) FriendStealElvesActuallyStolen(friendUID int64, landID, elvesItemID, elvesCntBefore int32, elvesCntBeforeObserved bool, inventoryBefore int32, now time.Time) bool {
	if s == nil {
		return false
	}
	if elvesCntBeforeObserved && s.StealElvesCntAt(now) > elvesCntBefore {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.frdVisitUID == friendUID {
		if land, ok := s.frdVisitLands[landID]; ok {
			if s.roleID > 0 && int64SliceContains(land.ElvesStealUIDs, s.roleID) {
				return true
			}
			if len(land.ElvesStealUIDs) > 0 && land.ElvesID == 0 {
				return true
			}
		}
	}
	if elvesItemID > 0 && s.inventory[elvesItemID] > inventoryBefore {
		return true
	}
	return false
}

// NoteFriendStealUsed reconciles per-friend flower-steal quota after a steal
// that consumed an attempt without confirming an elf gain (false flower steal).
func (s *State) NoteFriendStealUsed(uid int64, usedBefore int32, usedBeforeObserved bool, now time.Time) {
	if s == nil || uid <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.noteFriendStealUsedLocked(uid, usedBefore, usedBeforeObserved, now)
}

func (s *State) noteFriendStealUsedLocked(uid int64, usedBefore int32, usedBeforeObserved bool, now time.Time) {
	if !usedBeforeObserved || !s.frdStealObserved || !frdStealMapFresh(s.frdStealRTimeMs, now) {
		return
	}
	if s.frdStealMap == nil {
		s.frdStealMap = make(map[int64]int32)
	}
	if minimum := usedBefore + 1; s.frdStealMap[uid] < minimum {
		s.frdStealMap[uid] = minimum
	}
}
