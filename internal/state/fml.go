package state

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *State) applyFmlLocked(raw json.RawMessage, fullRaceTaskPool bool) {
	var ns25 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns25); err != nil {
		return
	}
	prevTaken := s.fmlRace.Taken
	s.fmlBuild.Observed = true
	if s.fmlBuild.BuildCounts == nil {
		s.fmlBuild.BuildCounts = make(map[int32]int32)
	}
	if rawFml, ok := ns25["0"]; ok {
		s.applyFmlObjectLocked(rawFml)
	}
	rawMember, memberPresent := ns25["1"]
	if memberPresent {
		s.applyFmlMemberObjectLocked(rawMember)
	}
	// fml.enter may return the current user only inside mbL (25.2), even when
	// mb=1 was requested. Use the member list strictly as a fallback matched by
	// the authenticated role UID; an explicit null 25.1 remains authoritative.
	if rawMembers, ok := ns25["2"]; ok &&
		(!memberPresent || (s.fmlBuild.MemberFmlID > 0 && !s.fmlBuild.MemberPositionObserved)) {
		s.applyFmlMemberListForCurrentRoleLocked(rawMembers)
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
			// Sparse deltas often omit tdyTakeCnt (field 2). Full-replace would
			// zero the counter and incorrectly reopen takes after tips8.
			s.fmlFlowerShare = mergeFmlFlowerShareView(s.fmlFlowerShare, view, rawShare)
		}
	}
	if rawOtherShares, ok := ns25["108"]; ok {
		s.applyOtherFmlFlowerSharesObjectLocked(rawOtherShares)
	}

	// Race batch + task pool + user record (fields 111, 114, 110).
	// Sparse merge: missing keys preserve prior race state. Only a meaningful
	// CurFmlRaceBatch marks Observed — empty/null stubs must not block enter.
	if rawBatch, ok := ns25["111"]; ok {
		applyFmlRaceBatchLocked(&s.fmlRace, rawBatch)
	}
	if rawRcd, ok := ns25["117"]; ok {
		applyFmlRaceCurRcdLocked(&s.fmlRace, rawRcd)
	}
	if rawGroup, ok := ns25["112"]; ok {
		applyFmlRaceGroupRcdLocked(&s.fmlRace, rawGroup, s.fmlBuild.FmlID)
	}
	if rawTasks, ok := ns25["114"]; ok {
		applyFmlRaceTasksLocked(&s.fmlRace, rawTasks, s.lastApplyMs, fullRaceTaskPool)
	}
	// Field 116 under ns25 is FmlRaceUsrRankList ([]IFmlRaceUsrRcd), distinct
	// from top-level namespace 116 (benefit box). Used to recover fTaskNum after
	// restart when enter/getTaskList omit field 110.
	if rawRank, ok := ns25["116"]; ok {
		applyFmlRaceUsrRankListLocked(&s.fmlRace, rawRank, s.roleID)
	}
	if rawUsrRcd, ok := ns25["110"]; ok {
		if isJSONNull(rawUsrRcd) {
			s.fmlRace.Taken = FmlRaceTakenView{}
			s.fmlRace.TaskQuotaObserved = false
			s.fmlRace.FinishedTaskNum = 0
			s.fmlRace.BuyTaskNum = 0
			s.fmlRace.ScoreObserved = false
			s.fmlRace.Score = 0
			s.fmlRace.ScoreTimeMs = 0
		} else {
			// Sparse 110 (e.g. giveUpTask only sends giveUpTime/uTime) must not
			// treat omitted fTaskNum/buyTaskNum as zero — that wipes UI「已做」.
			taken, finished, buy, score, scoreTime, finishedOK, buyOK, scoreOK := parseFmlRaceUsrRcd(rawUsrRcd, s.roleID, s.fmlRace.BatchID)
			s.fmlRace.Taken = taken
			if finishedOK {
				s.fmlRace.TaskQuotaObserved = true
				s.fmlRace.FinishedTaskNum = finished
			}
			if buyOK {
				s.fmlRace.TaskQuotaObserved = true
				s.fmlRace.BuyTaskNum = buy
			}
			if scoreOK {
				s.fmlRace.ScoreObserved = true
				s.fmlRace.Score = score
				s.fmlRace.ScoreTimeMs = scoreTime
			}
		}
	}
	// Enrich taken task from the pool (score / param / label / progress / type).
	// takeTask / 110 often lag finishCnt while the matching pool row (field 8)
	// advances; always take the higher FinishCnt/TargetCnt so finishTask can
	// fire and plant demand shrinks with real progress.
	if s.fmlRace.Taken.HasTask {
		enrichFmlRaceTakenFromTask(&s.fmlRace.Taken, s.fmlRace.Tasks)
		if s.fmlRace.Taken.TargetLabel == "" && s.fmlRace.Taken.ParamID > 0 {
			s.fmlRace.Taken.TargetLabel = ItemLabel(s.fmlRace.Taken.ParamID)
		}
	}
	if !s.fmlRace.Taken.HasTask {
		if taken, ok := synthesizeFmlRaceTakenFromPool(s.fmlRace.Tasks, s.roleID); ok {
			s.fmlRace.Taken = taken
		}
	}
	// Pool UID==self is the live holder. Prefer it over 110 takeTaskData whenever
	// present — stale 110 (e.g. 鹤望兰 score 0) otherwise survives enter/sparse
	// syncs and blocks take/giveUp. Full-pool getTaskList with no UID==self also
	// clears orphans so UI does not keep a ghost task.
	reconcileFmlRaceTakenWithPool(&s.fmlRace, s.roleID, fullRaceTaskPool)
	// Field 134 carries live takeTaskData on plant-harvest (and finish) deltas.
	// Apply last so harvest progress is not overwritten by a lagging 110 stub,
	// and finishTask can fire on the next plan tick without waiting for getTaskList.
	if rawTakenProg, ok := ns25["134"]; ok {
		applyFmlRaceTakenProgressLocked(&s.fmlRace, rawTakenProg)
	}
	// Full-pool getTaskList is authoritative for FinishCnt. If LocalFinishCnt
	// already claims the target but the synced FinishCnt is still short, the
	// local high-water overcounted (or the lag will not resolve via re-fetch).
	// Clamp so the planner resumes planting instead of getTaskList every 30s.
	if fullRaceTaskPool {
		reconcileFmlRaceLocalFinishAfterFullPool(&s.fmlRace)
	}
	// Stamp take time / fill ExpireTime = TakenAtMs + TakeLimitMin when the
	// server omits expireTime (common on 110/pool until harvest progress).
	finalizeFmlRaceTakenDeadline(&s.fmlRace, prevTaken, s.lastApplyMs)
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
	if n, ok := readInt32JSONField(fields, "102"); ok {
		s.fmlBuild.FlowerTakeCnt = n
	}
	if n, ok := readInt32JSONField(fields, "103"); ok {
		s.fmlBuild.RaceLvl = n
		if n > 0 && s.fmlRace.RaceLvl <= 0 {
			s.fmlRace.RaceLvl = n
			s.fmlRace.RaceLvlObserved = true
		}
	}
	if rawCounts, ok := fields["30"]; ok {
		s.setFmlBuildCountsLocked(rawCounts)
	}
}

// applyFmlMemberObjectLocked tracks IFmlTot.mb (25.1), the authoritative
// current-user guild membership record. IFmlTot.fml (25.0) can survive as
// cached guild/race data after the user has left and is therefore not proof of
// current membership.
func (s *State) applyFmlMemberObjectLocked(raw json.RawMessage) {
	s.fmlBuild.MembershipObserved = true
	s.fmlBuild.MemberFmlID = 0
	s.fmlBuild.MemberPositionObserved = false
	s.fmlBuild.MemberPosition = 0
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if id, ok := readInt32JSONField(fields, "1"); ok {
		s.fmlBuild.MemberFmlID = id
		if s.fmlBuild.FmlID <= 0 {
			s.fmlBuild.FmlID = id
		}
	}
	if position, ok := readInt32JSONField(fields, "2"); ok {
		s.fmlBuild.MemberPositionObserved = true
		s.fmlBuild.MemberPosition = position
	}
}

func (s *State) applyFmlMemberListForCurrentRoleLocked(raw json.RawMessage) {
	if s.roleID <= 0 || len(raw) == 0 || string(raw) == "null" {
		return
	}
	var members []json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return
	}
	for _, rawMember := range members {
		var fields map[string]json.RawMessage
		if json.Unmarshal(rawMember, &fields) != nil {
			continue
		}
		uid, ok := readInt64JSONField(fields, "0")
		if !ok || uid != s.roleID {
			continue
		}
		s.applyFmlMemberObjectLocked(rawMember)
		return
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
		// Sparse 25.102 stubs may omit landMap. Treat the namespace as observed
		// without wiping any previously synced slots.
		s.fmlLandObserved = true
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
	syncedAt := s.lastApplyMs
	if syncedAt <= 0 {
		syncedAt = time.Now().UnixMilli()
	}
	if len(raw) == 0 || string(raw) == "null" {
		s.fmlOtherFlowerShares = next
		s.fmlOtherShareObserved = true
		s.fmlOtherShareSyncedAtMs = syncedAt
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
		s.fmlOtherShareSyncedAtMs = syncedAt
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
	s.fmlOtherShareSyncedAtMs = syncedAt
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

// mergeFmlFlowerShareView keeps prior scalar fields when a sparse delta omits them.
func mergeFmlFlowerShareView(prev, incoming FmlFlowerShareView, raw json.RawMessage) FmlFlowerShareView {
	out := prev
	out.Observed = true
	if out.Slots == nil {
		out.Slots = make(map[int32]FmlFlowerShareSlotView)
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return cloneFmlFlowerShareView(incoming)
	}
	if _, ok := fields["0"]; ok {
		out.UID = incoming.UID
	}
	if _, ok := fields["1"]; ok {
		out.Slots = incoming.Slots
		if out.Slots == nil {
			out.Slots = make(map[int32]FmlFlowerShareSlotView)
		}
	}
	if _, ok := fields["2"]; ok {
		out.TdyTakeCnt = incoming.TdyTakeCnt
	}
	if _, ok := fields["3"]; ok {
		out.LastTakeTimeMs = incoming.LastTakeTimeMs
	}
	if _, ok := fields["4"]; ok {
		out.UpdatedAtMs = incoming.UpdatedAtMs
	}
	if _, ok := fields["5"]; ok {
		out.CreatedAtMs = incoming.CreatedAtMs
	}
	return out
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

// FmlBuildOptionUsageAt returns the current calendar-day counters for one
// c_fmlBld option and its option group. A stale bldTime makes prior counters
// zero without mutating the preserved server snapshot.
func (s *State) FmlBuildOptionUsageAt(optionID int32, now time.Time) FmlBuildOptionUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	usage := FmlBuildOptionUsage{Observed: s.fmlBuild.BuildCountsObserved}
	if !usage.Observed || optionID <= 0 {
		return usage
	}
	if s.fmlBuild.LastBuildTimeMs > 0 && calendarDayID(time.UnixMilli(s.fmlBuild.LastBuildTimeMs)) < calendarDayID(now) {
		return usage
	}
	usage.Count = s.fmlBuild.BuildCounts[optionID]
	option, ok := FmlBuildOptionByID(optionID)
	if !ok || option.Type <= 0 {
		return usage
	}
	for id, count := range s.fmlBuild.BuildCounts {
		row, known := FmlBuildOptionByID(id)
		if known && row.Type == option.Type {
			usage.GroupCount += count
		}
	}
	return usage
}

// BeginFmlMembershipSnapshot starts a new connection epoch. Guild and race
// snapshots may remain available for diagnostics, but their previous guild ID
// must not count as current membership until login + lazySync provide evidence
// for this epoch.
func (s *State) BeginFmlMembershipSnapshot() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlBuild.FmlID = 0
	s.fmlBuild.MembershipObserved = false
	s.fmlBuild.MemberFmlID = 0
	s.fmlBuild.MemberPositionObserved = false
	s.fmlBuild.MemberPosition = 0
	s.fmlBuild.MemberPositionSyncAtMs = 0
	s.fmlForestRefreshAttemptAtMs = 0
	s.bumpRevisionLocked()
}

// MarkNoFmlMembership records an authoritative server response that the
// account is not currently a guild member. It lets every guild planner stop
// immediately even if stale IFml/race records remain in the login snapshot.
func (s *State) MarkNoFmlMembership() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlBuild.MembershipObserved = true
	s.fmlBuild.MemberFmlID = 0
	s.fmlBuild.MemberPositionObserved = false
	s.fmlBuild.MemberPosition = 0
	s.fmlBuild.MemberPositionSyncAtMs = 0
	s.fmlForestRefreshAttemptAtMs = 0
	s.bumpRevisionLocked()
}

// MarkFmlMemberPositionSyncAttempt records an explicit fml.enter round-trip
// used to request IFmlTot.mb. Some login/lazySync responses omit field 25.1;
// the timestamp prevents an empty member response from causing a tight loop.
func (s *State) MarkFmlMemberPositionSyncAttempt() {
	s.MarkFmlMemberPositionSyncAttemptAt(time.Now())
}

// MarkFmlMemberPositionSyncAttemptAt is the deterministic-time variant used by
// the planner and tests.
func (s *State) MarkFmlMemberPositionSyncAttemptAt(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlBuild.MemberPositionSyncAtMs = at.UnixMilli()
	s.bumpRevisionLocked()
}

// MarkFmlForestRefreshAttempt records a successful or failed refresh attempt.
// Some channel fronts acknowledge fmlForest.refresh without a usable 25.127
// delta; the planner uses this timestamp to avoid a tight success loop.
func (s *State) MarkFmlForestRefreshAttempt() {
	s.MarkFmlForestRefreshAttemptAt(time.Now())
}

// MarkFmlForestRefreshAttemptAt is the deterministic-time variant used by
// planner tests.
func (s *State) MarkFmlForestRefreshAttemptAt(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlForestRefreshAttemptAtMs = at.UnixMilli()
	s.bumpRevisionLocked()
}

// FmlForestRefreshAttemptAtMs returns the last explicit forest refresh time.
func (s *State) FmlForestRefreshAttemptAtMs() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlForestRefreshAttemptAtMs
}

// FinalizeFmlMembershipSnapshot closes the startup index.login + lazySync
// baseline. Some channel fronts omit IFmlTot.mb (25.1) for joined accounts but
// still return IFmlTot.fml (25.0). In that shape the guild ID is positive
// membership evidence; only a baseline with neither record is treated as no
// guild. This must only be called after the full startup sync, not for sparse
// operation deltas.
func (s *State) FinalizeFmlMembershipSnapshot() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.fmlBuild.MembershipObserved {
		s.fmlBuild.MembershipObserved = true
		if s.fmlBuild.FmlID > 0 {
			s.fmlBuild.MemberFmlID = s.fmlBuild.FmlID
		}
		s.bumpRevisionLocked()
	}
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

// FormatFmlLandHarvestReason builds a human-readable harvest summary for logs.
func FormatFmlLandHarvestReason(lands map[int32]FmlLandView, landIDs []int32, now time.Time) string {
	if len(landIDs) == 0 {
		return "公会土地有成熟鲜花可收获"
	}
	parts := make([]string, 0, len(landIDs))
	total := int32(0)
	for _, id := range landIDs {
		land, ok := lands[id]
		if !ok {
			parts = append(parts, fmt.Sprintf("土地#%d", id))
			continue
		}
		pending := FmlLandPendingHarvest(land, now)
		total += pending
		name := FlowerName(land.FlowerID)
		if name == "" {
			name = fmt.Sprintf("花卉#%d", land.FlowerID)
		}
		parts = append(parts, fmt.Sprintf("%s×%d(土地#%d)", name, pending, id))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("公会土地可收获 %d 块", len(landIDs))
	}
	return fmt.Sprintf("公会土地可收获 %d 朵: %s", total, strings.Join(parts, "、"))
}

// FmlLandPendingHarvest returns unclaimed mature flowers on one guild land.
// When protocol matureFlwCnt is stale (often 0 until the client UI recalculates),
// maturity is derived from startTime and c_fmlLandLvl time/stock.
func FmlLandPendingHarvest(land FmlLandView, now time.Time) int32 {
	if land.FlowerID <= 0 {
		return 0
	}
	stored := land.MatureFlowerCnt - land.HarvestedCnt
	if stored < 0 {
		stored = 0
	}
	computed := int32(0)
	if land.StartTimeMs > 0 {
		if cfg, ok := FmlLandLvlByID(land.Level); ok && cfg.TimeSec > 0 {
			elapsedSec := (now.UnixMilli() - land.StartTimeMs) / 1000
			if elapsedSec > 0 {
				produced := elapsedSec / int64(cfg.TimeSec)
				if cfg.Stock > 0 && produced > int64(cfg.Stock) {
					produced = int64(cfg.Stock)
				}
				computed = int32(produced) - land.HarvestedCnt
				if computed < 0 {
					computed = 0
				}
			}
		}
	}
	if computed > stored {
		return computed
	}
	return stored
}

// FmlLandNextMatureMs returns when the next flower becomes harvestable.
// Zero means empty, already pending, stock-full, or catalog/timing unknown.
func FmlLandNextMatureMs(land FmlLandView, now time.Time) int64 {
	if land.FlowerID <= 0 || land.StartTimeMs <= 0 {
		return 0
	}
	if FmlLandPendingHarvest(land, now) > 0 {
		return 0
	}
	cfg, ok := FmlLandLvlByID(land.Level)
	if !ok || cfg.TimeSec <= 0 {
		return 0
	}
	elapsedSec := (now.UnixMilli() - land.StartTimeMs) / 1000
	if elapsedSec < 0 {
		elapsedSec = 0
	}
	produced := elapsedSec / int64(cfg.TimeSec)
	if cfg.Stock > 0 && produced >= int64(cfg.Stock) {
		return 0
	}
	nextIndex := produced + 1
	return land.StartTimeMs + nextIndex*int64(cfg.TimeSec)*1000
}

// ReadyFmlLandHarvestIDs returns guild lands with unclaimed mature flowers.
func (s *State) ReadyFmlLandHarvestIDs(now time.Time) []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.fmlLands))
	for id, land := range s.fmlLands {
		if land == nil {
			continue
		}
		if FmlLandPendingHarvest(*land, now) <= 0 {
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

// OtherFmlFlowerSharesSyncedAtMs is local wall time (ms) when 25.108 was last applied.
func (s *State) OtherFmlFlowerSharesSyncedAtMs() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlOtherShareSyncedAtMs
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

// FmlFlowerTakeLimit is the current daily take allowance for this guild
// (IFml.flowerTakeCnt). When the guild field is unobserved it falls back to
// $takeMax (not $initTakeNum) so callers that only need an upper bound do not
// under-count upgraded guilds; exhaustion gating uses FlowerTakeCnt directly.
func (s *State) FmlFlowerTakeLimit() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlFlowerTakeLimitLocked()
}

func (s *State) fmlFlowerTakeLimitLocked() int32 {
	limit := s.fmlBuild.FlowerTakeCnt
	if limit <= 0 {
		// Prefer $takeMax over $initTakeNum when unobserved: init is only the
		// pre-upgrade baseline (1) and would leave unused daily takes.
		limit = fmlFlowerShareTakeMax()
		if limit <= 0 {
			limit = fmlFlowerShareInitTakeNum()
		}
	}
	if max := fmlFlowerShareTakeMax(); max > 0 && limit > max {
		limit = max
	}
	if limit <= 0 {
		return 1
	}
	return limit
}

// FmlFlowerTakeExhausted reports whether today's take quota is already used up
// from observed share state (tdyTakeCnt >= guild FlowerTakeCnt) or a server
// tips8 mark.
//
// When IFml.flowerTakeCnt (25.0.102) has not been observed, this must NOT fall
// back to c_fmlFlowerShare.$initTakeNum (1): that under-counts upgraded guilds
// and stops automation after a single take while daily quota remains.
func (s *State) FmlFlowerTakeExhausted(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fmlFlowerTakeLimitUntilMs > 0 {
		until := time.UnixMilli(s.fmlFlowerTakeLimitUntilMs)
		if until.After(now) {
			return true
		}
		s.fmlFlowerTakeLimitUntilMs = 0
	}
	if !s.fmlFlowerShare.Observed {
		return false
	}
	// Across the 00:00 boundary the local counter is stale until 107 refreshes.
	if s.fmlFlowerShare.LastTakeTimeMs > 0 &&
		calendarDayID(time.UnixMilli(s.fmlFlowerShare.LastTakeTimeMs)) < calendarDayID(now) {
		return false
	}
	limit := s.fmlBuild.FlowerTakeCnt
	if limit <= 0 {
		return false
	}
	if max := fmlFlowerShareTakeMax(); max > 0 && limit > max {
		limit = max
	}
	return s.fmlFlowerShare.TdyTakeCnt >= limit
}

// NoteFmlFlowerShareTake bumps local other-share TakeNum after a successful
// take when the response omitted a 25.108 delta, so the planner advances to
// the next candidate instead of retrying a depleted slot under shared cooldown.
// Own tdyTakeCnt is left to ApplyV / tips8 — do not guess it here.
func (s *State) NoteFmlFlowerShareTake(dstUID int64, slotID int32) {
	if dstUID == 0 || slotID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, share := range s.fmlOtherFlowerShares {
		if share == nil {
			continue
		}
		actual := share.UID
		if actual == 0 {
			actual = key
		}
		if actual != dstUID {
			continue
		}
		slot, ok := share.Slots[slotID]
		if !ok {
			continue
		}
		// ApplyV may already have installed the authoritative TakeNum; only
		// fill in a missing local increment while the slot still looks free.
		if slot.ShareNum-slot.TakeNum <= 0 {
			continue
		}
		slot.TakeNum++
		share.Slots[slotID] = slot
	}
}

// MarkFmlFlowerTakeDailyLimitReached records the server-side daily take cap so
// automation stops selecting fmlFlowerShare.take until the next 00:00 reset.
func (s *State) MarkFmlFlowerTakeDailyLimitReached(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlFlowerTakeLimitUntilMs = NextCalendarDayReset(now).UnixMilli()
	limit := s.fmlFlowerTakeLimitLocked()
	// Force local exhausted state even when 25.107 was never observed, so the
	// planner does not keep selecting take after a short side-op cooldown.
	s.fmlFlowerShare.Observed = true
	if s.fmlFlowerShare.Slots == nil {
		s.fmlFlowerShare.Slots = make(map[int32]FmlFlowerShareSlotView)
	}
	if s.fmlFlowerShare.TdyTakeCnt < limit {
		s.fmlFlowerShare.TdyTakeCnt = limit
	}
	s.fmlFlowerShare.LastTakeTimeMs = now.UnixMilli()
}

// FmlFlowerTakeDailyLimitReached reports a locally recorded server-side daily
// take cap (fmlShare_tips8).
func (s *State) FmlFlowerTakeDailyLimitReached(now time.Time) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fmlFlowerTakeLimitUntilMs <= 0 {
		return time.Time{}, false
	}
	until := time.UnixMilli(s.fmlFlowerTakeLimitUntilMs)
	if !until.After(now) {
		s.fmlFlowerTakeLimitUntilMs = 0
		return time.Time{}, false
	}
	return until, true
}

// FmlFlowerTakeWindowStart is today's 00:01 Asia/Shanghai; take/sync only run at/after this.
func FmlFlowerTakeWindowStart(now time.Time) time.Time {
	local := now.In(gameDayLocation())
	y, m, d := local.Date()
	return time.Date(y, m, d, 0, 1, 0, 0, local.Location())
}

// FmlFlowerTakeWindowOpen reports whether flower-take automation may run now.
func FmlFlowerTakeWindowOpen(now time.Time) bool {
	return !now.In(gameDayLocation()).Before(FmlFlowerTakeWindowStart(now))
}

func fmlFlowerShareInitTakeNum() int32 {
	raw, ok := catalog.Tables["c_fmlFlowerShare"].Rows["-1"]
	if !ok {
		return 1
	}
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil {
		return 1
	}
	if n := readInt32Any(row["$initTakeNum"]); n > 0 {
		return n
	}
	return 1
}

func fmlFlowerShareTakeMax() int32 {
	raw, ok := catalog.Tables["c_fmlFlowerShare"].Rows["-1"]
	if !ok {
		return 4
	}
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil {
		return 4
	}
	if n := readInt32Any(row["$takeMax"]); n > 0 {
		return n
	}
	return 4
}
