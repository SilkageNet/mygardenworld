package state

import (
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func (s *State) applyFmlLocked(raw json.RawMessage, fullRaceTaskPool bool) {
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

	// Race batch + task pool + user record (fields 111, 114, 110).
	// Sparse merge: missing keys preserve prior race state. Only a meaningful
	// CurFmlRaceBatch marks Observed — empty/null stubs must not block enter.
	if rawBatch, ok := ns25["111"]; ok {
		applyFmlRaceBatchLocked(&s.fmlRace, rawBatch)
	}
	if rawTasks, ok := ns25["114"]; ok {
		applyFmlRaceTasksLocked(&s.fmlRace, rawTasks, s.lastApplyMs, fullRaceTaskPool)
	}
	if rawUsrRcd, ok := ns25["110"]; ok {
		if isJSONNull(rawUsrRcd) {
			s.fmlRace.Taken = FmlRaceTakenView{}
		} else {
			s.fmlRace.Taken = parseFmlRaceTaken(rawUsrRcd, s.roleID, s.fmlRace.BatchID)
		}
	}
	// Enrich taken task from the pool (score / param / label / progress / type).
	// takeTask responses sometimes omit targetCnt (field 2) / finishCnt (field 3)
	// in field 110.7; backfill from the matching pool row so that race progress
	// demands can fire.
	if s.fmlRace.Taken.HasTask {
		for _, t := range s.fmlRace.Tasks {
			if t.MsId != s.fmlRace.Taken.TaskMsId {
				continue
			}
			if s.fmlRace.Taken.Score == 0 {
				s.fmlRace.Taken.Score = t.Score
			}
			if s.fmlRace.Taken.ParamID == 0 && t.ParamID != 0 {
				s.fmlRace.Taken.ParamID = t.ParamID
				s.fmlRace.Taken.TargetLabel = t.TargetLabel
			}
			if s.fmlRace.Taken.TargetCnt == 0 && t.TargetCnt > 0 {
				s.fmlRace.Taken.TargetCnt = t.TargetCnt
			}
			if s.fmlRace.Taken.FinishCnt == 0 && t.FinishCnt > 0 {
				s.fmlRace.Taken.FinishCnt = t.FinishCnt
			}
			if s.fmlRace.Taken.TaskType == 0 && t.TaskType > 0 {
				s.fmlRace.Taken.TaskType = t.TaskType
			}
			break
		}
		if s.fmlRace.Taken.TargetLabel == "" && s.fmlRace.Taken.ParamID > 0 {
			s.fmlRace.Taken.TargetLabel = ItemLabel(s.fmlRace.Taken.ParamID)
		}
	}
	if !s.fmlRace.Taken.HasTask {
		if taken, ok := synthesizeFmlRaceTakenFromPool(s.fmlRace.Tasks, s.roleID); ok {
			s.fmlRace.Taken = taken
		}
	}
}

func applyFmlRaceBatchLocked(view *FmlRaceView, raw json.RawMessage) {
	if isJSONNull(raw) {
		view.Observed = false
		view.BatchActive = false
		view.BatchID = 0
		view.BatchStatus = 0
		view.BatchStartMs = 0
		view.BatchEndMs = 0
		return
	}
	var batch clientproto.IFmlRaceBatch
	if err := json.Unmarshal(raw, &batch); err != nil {
		return
	}
	// Empty {} stubs from login/lazy sync are not a real batch sync.
	if batch.BatchId == 0 && batch.Status == 0 && batch.StartTime == 0 && batch.EndTime == 0 {
		view.Observed = false
		view.BatchActive = false
		view.BatchID = 0
		view.BatchStatus = 0
		view.BatchStartMs = 0
		view.BatchEndMs = 0
		return
	}
	view.Observed = true
	view.BatchID = batch.BatchId
	view.BatchStatus = batch.Status
	view.BatchStartMs = batch.StartTime
	view.BatchEndMs = batch.EndTime
	view.BatchActive = fmlRaceBatchActive(batch.Status, batch.StartTime, batch.EndTime)
}

func applyFmlRaceTasksLocked(view *FmlRaceView, raw json.RawMessage, nowMs int64, fullPool bool) {
	if isJSONNull(raw) {
		view.TasksObserved = true
		view.Tasks = nil
		view.MissingParamRefreshFP = ""
		view.TasksSyncedAtMs = nowMs
		return
	}
	var tasks []clientproto.IFmlRaceTask
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return
	}
	incoming := make([]FmlRaceTaskView, 0, len(tasks))
	for _, t := range tasks {
		paramID := firstInt32FromRaw(t.Param)
		incoming = append(incoming, FmlRaceTaskView{
			MsId:        t.MsId,
			TaskId:      t.TaskId,
			TaskType:    FmlRaceTaskTypeByID(t.TaskId),
			Score:       t.Score,
			IsUpgrade:   t.IsUpgrade,
			UpgradeUid:  t.UpgradeUid,
			UID:         t.UID,
			ParamID:     paramID,
			TargetLabel: ItemLabel(paramID),
			AppearTime:  t.AppearTime,
			TargetCnt:   t.TargetCnt,
			FinishCnt:   t.FinishCnt,
		})
	}
	wasObserved := view.TasksObserved
	// getTaskList returns the full pool; take/finish responses often carry only
	// the changed rows. Replace on first sync or when the payload is clearly a
	// full list; otherwise merge by MsId.
	if fullPool || !view.TasksObserved || len(incoming) >= len(view.Tasks) {
		if view.TasksObserved {
			// Full-list replace can still omit param on some rows; keep known detail.
			byID := make(map[int64]FmlRaceTaskView, len(view.Tasks))
			for _, prev := range view.Tasks {
				byID[prev.MsId] = prev
			}
			for i := range incoming {
				if prev, ok := byID[incoming[i].MsId]; ok {
					incoming[i] = preserveFmlRaceTaskDetail(incoming[i], prev)
				}
			}
		}
		view.Tasks = incoming
	} else {
		byID := make(map[int64]int, len(view.Tasks))
		for i, t := range view.Tasks {
			byID[t.MsId] = i
		}
		for _, t := range incoming {
			if i, ok := byID[t.MsId]; ok {
				view.Tasks[i] = preserveFmlRaceTaskDetail(t, view.Tasks[i])
			} else {
				view.Tasks = append(view.Tasks, t)
			}
		}
	}
	view.TasksObserved = true
	view.TasksSyncedAtMs = nowMs
	updateFmlRaceMissingParamRefreshFP(view, wasObserved)
}

// FmlRacePlantHarvestMissingParam reports whether any pool plant-harvest task
// lacks a resolved ParamID (flower target). Used to trigger getTaskList refresh.
func FmlRacePlantHarvestMissingParam(tasks []FmlRaceTaskView) bool {
	for _, t := range tasks {
		taskType := t.TaskType
		if taskType == 0 {
			taskType = t.TaskId
		}
		if taskType == 3036 && t.ParamID <= 0 {
			return true
		}
	}
	return false
}

// FmlRaceTaskPoolMsFingerprint is a stable key for the current pool identity
// (msIds only), used to avoid re-requesting getTaskList for the same incomplete set.
func FmlRaceTaskPoolMsFingerprint(tasks []FmlRaceTaskView) string {
	if len(tasks) == 0 {
		return ""
	}
	ids := make([]int64, len(tasks))
	for i, t := range tasks {
		ids[i] = t.MsId
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}

func updateFmlRaceMissingParamRefreshFP(view *FmlRaceView, wasObserved bool) {
	if !FmlRacePlantHarvestMissingParam(view.Tasks) {
		view.MissingParamRefreshFP = ""
		return
	}
	// First observation of an incomplete pool still allows one getTaskList refresh.
	// A subsequent apply of the same incomplete pool records the fingerprint so
	// automation does not loop forever when the server omits param.
	if wasObserved {
		view.MissingParamRefreshFP = FmlRaceTaskPoolMsFingerprint(view.Tasks)
	}
}

func preserveFmlRaceTaskDetail(next, prev FmlRaceTaskView) FmlRaceTaskView {
	if next.ParamID == 0 && prev.ParamID != 0 {
		next.ParamID = prev.ParamID
		next.TargetLabel = prev.TargetLabel
	}
	if next.AppearTime == 0 && prev.AppearTime != 0 {
		next.AppearTime = prev.AppearTime
	}
	return next
}

// synthesizeFmlRaceTakenFromPool builds a Taken view from the task pool when the
// user's row (UID == roleID) is present but field 110 lacked takeTaskData.
// Progress (TargetCnt/FinishCnt) is filled when the pool row carries it; 0 means
// unknown and callers must treat it as an unfinished task.
func synthesizeFmlRaceTakenFromPool(tasks []FmlRaceTaskView, roleID int64) (FmlRaceTakenView, bool) {
	if roleID <= 0 {
		return FmlRaceTakenView{}, false
	}
	for _, t := range tasks {
		if t.UID != roleID || t.MsId == 0 {
			continue
		}
		taskType := t.TaskType
		if taskType == 0 {
			taskType = FmlRaceTaskTypeByID(t.TaskId)
		}
		return FmlRaceTakenView{
			TaskMsId:    t.MsId,
			TaskId:      t.TaskId,
			TaskType:    taskType,
			Score:       t.Score,
			TargetCnt:   t.TargetCnt,
			FinishCnt:   t.FinishCnt,
			ParamID:     t.ParamID,
			TargetLabel: t.TargetLabel,
			HasTask:     true,
		}, true
	}
	return FmlRaceTakenView{}, false
}

// firstInt32FromRaw returns the first numeric entry from a JSON array/number
// param payload (e.g. [23001]). Empty/null arrays yield 0.
func firstInt32FromRaw(raw json.RawMessage) int32 {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var nums []int64
	if err := json.Unmarshal(raw, &nums); err == nil {
		if len(nums) == 0 || nums[0] <= 0 || nums[0] > math.MaxInt32 {
			return 0
		}
		return int32(nums[0])
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err != nil || n <= 0 || n > math.MaxInt32 {
		return 0
	}
	return int32(n)
}

// fmlRaceBatchActive reports whether the current guild race batch is playable.
// status==1 is the primary in-progress signal; when status is still 0 but the
// server already published a start/end window, treat the open window as active.
// status==2 (ended) always stays inactive.
func fmlRaceBatchActive(status int32, startMs, endMs int64) bool {
	if status == 2 {
		return false
	}
	if status == 1 {
		return true
	}
	if startMs <= 0 || endMs <= startMs {
		return false
	}
	now := time.Now().UnixMilli()
	return now >= startMs && now < endMs
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

// parseFmlRaceTaken extracts the current user's taken-task progress from the
// FmlRaceUsrRcdMap raw JSON (namespace 25, field 110). Observed payloads key
// the map by batchId (not uid). Prefer batchId, then uid, then any entry that
// carries TakeTaskData.
func parseFmlRaceTaken(raw json.RawMessage, uid, batchID int64) FmlRaceTakenView {
	if len(raw) == 0 {
		return FmlRaceTakenView{}
	}
	var m map[string]clientproto.IFmlRaceUsrRcd
	if err := json.Unmarshal(raw, &m); err != nil {
		return FmlRaceTakenView{}
	}
	tryKeys := make([]string, 0, 2)
	if batchID > 0 {
		tryKeys = append(tryKeys, strconv.FormatInt(batchID, 10))
	}
	if uid > 0 {
		tryKeys = append(tryKeys, strconv.FormatInt(uid, 10))
	}
	for _, key := range tryKeys {
		if rcd, ok := m[key]; ok {
			if view := takenFromUsrRcd(rcd); view.HasTask {
				return view
			}
		}
	}
	for _, rcd := range m {
		if view := takenFromUsrRcd(rcd); view.HasTask {
			return view
		}
	}
	return FmlRaceTakenView{}
}

func takenFromUsrRcd(rcd clientproto.IFmlRaceUsrRcd) FmlRaceTakenView {
	tt := rcd.TakeTaskData
	if tt.TaskMsId == 0 {
		return FmlRaceTakenView{}
	}
	paramID := firstInt32FromRaw(tt.Param)
	return FmlRaceTakenView{
		TaskMsId:    tt.TaskMsId,
		TaskId:      tt.TaskId,
		TaskType:    FmlRaceTaskTypeByID(tt.TaskId),
		TargetCnt:   tt.TargetCnt,
		FinishCnt:   tt.FinishCnt,
		ParamID:     paramID,
		TargetLabel: ItemLabel(paramID),
		HasTask:     true,
	}
}

// FmlRace returns the guild race view parsed from namespace 25.
func (s *State) FmlRace() FmlRaceView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fmlRace
}

// MarkFmlRaceTasksUnobserved forces the next race tick to re-fetch getTaskList
// without wiping the last observed pool snapshot.
func (s *State) MarkFmlRaceTasksUnobserved() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlRace.TasksObserved = false
}
