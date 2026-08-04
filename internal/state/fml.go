package state

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	if rawUsrRcd, ok := ns25["110"]; ok {
		if isJSONNull(rawUsrRcd) {
			s.fmlRace.Taken = FmlRaceTakenView{}
			s.fmlRace.TaskQuotaObserved = false
			s.fmlRace.FinishedTaskNum = 0
			s.fmlRace.BuyTaskNum = 0
		} else {
			taken, finished, buy, quotaOK := parseFmlRaceUsrRcd(rawUsrRcd, s.roleID, s.fmlRace.BatchID)
			s.fmlRace.Taken = taken
			if quotaOK {
				s.fmlRace.TaskQuotaObserved = true
				s.fmlRace.FinishedTaskNum = finished
				s.fmlRace.BuyTaskNum = buy
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

func applyFmlRaceCurRcdLocked(view *FmlRaceView, raw json.RawMessage) {
	if isJSONNull(raw) {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if n, ok := readInt32JSONField(fields, "5"); ok && n > 0 {
		view.RaceLvl = n
		view.RaceLvlObserved = true
		return
	}
	var rcd clientproto.IFmlRaceRcd
	if err := json.Unmarshal(raw, &rcd); err == nil && rcd.RaceLvl > 0 {
		view.RaceLvl = rcd.RaceLvl
		view.RaceLvlObserved = true
	}
}

func applyFmlRaceGroupRcdLocked(view *FmlRaceView, raw json.RawMessage, fmlID int32) {
	if isJSONNull(raw) {
		return
	}
	var list []clientproto.IFmlRaceRcd
	if err := json.Unmarshal(raw, &list); err != nil || len(list) == 0 {
		return
	}
	var fallback int32
	for _, rcd := range list {
		if rcd.RaceLvl <= 0 {
			continue
		}
		if view.BatchID > 0 && rcd.BatchId > 0 && rcd.BatchId != view.BatchID {
			continue
		}
		if fmlID > 0 && rcd.Fid == fmlID {
			view.RaceLvl = rcd.RaceLvl
			view.RaceLvlObserved = true
			return
		}
		if fallback == 0 {
			fallback = rcd.RaceLvl
		}
	}
	if fallback > 0 {
		if view.RaceLvl <= 0 {
			view.RaceLvl = fallback
		}
		view.RaceLvlObserved = true
	}
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

// enrichFmlRaceTakenFromTask copies score/param/type gaps and monotonic
// TargetCnt/FinishCnt from the pool row with the same msId (UID may be 0 on
// some shards while progress still advances on field 7/8).
func enrichFmlRaceTakenFromTask(taken *FmlRaceTakenView, tasks []FmlRaceTaskView) {
	if taken == nil || !taken.HasTask || taken.TaskMsId == 0 {
		return
	}
	for _, t := range tasks {
		if t.MsId != taken.TaskMsId {
			continue
		}
		if taken.Score == 0 {
			taken.Score = t.Score
		}
		if taken.ParamID == 0 && t.ParamID != 0 {
			taken.ParamID = t.ParamID
			taken.TargetLabel = t.TargetLabel
		}
		if t.TargetLabel != "" && taken.TargetLabel == "" {
			taken.TargetLabel = t.TargetLabel
		}
		if t.TargetCnt > taken.TargetCnt {
			taken.TargetCnt = t.TargetCnt
		}
		if t.FinishCnt > taken.FinishCnt {
			taken.FinishCnt = t.FinishCnt
		}
		if taken.TaskType == 0 && t.TaskType > 0 {
			taken.TaskType = t.TaskType
		}
		if taken.TaskId == 0 && t.TaskId > 0 {
			taken.TaskId = t.TaskId
		}
		return
	}
}

// reconcileFmlRaceTakenWithPool aligns Taken with the task pool.
//
// Pool UID==self is the live holder. When it points at a different TaskMsId than
// 110 (stale 鹤望兰 / score-0 ghost), replace Taken entirely. When it matches the
// current Taken msId, merge gaps and advance FinishCnt monotonically.
//
// Authoritative getTaskList (fullPool) with no UID==self clears Taken only when
// the current msId is also gone from the pool (true orphan). Some shards keep
// the holder's pool row at UID=0 while 110 still carries takeTaskData — clearing
// those would drop a live task, skip finishTask, and allow a duplicate take.
func reconcileFmlRaceTakenWithPool(view *FmlRaceView, roleID int64, fullPool bool) {
	poolTaken, ok := synthesizeFmlRaceTakenFromPool(view.Tasks, roleID)
	if !ok {
		if fullPool && view.Taken.HasTask {
			if racePoolHasMsID(view.Tasks, view.Taken.TaskMsId) {
				enrichFmlRaceTakenFromTask(&view.Taken, view.Tasks)
			} else {
				view.Taken = FmlRaceTakenView{}
			}
		}
		return
	}
	if !view.Taken.HasTask || view.Taken.TaskMsId != poolTaken.TaskMsId {
		view.Taken = poolTaken
		return
	}
	if view.Taken.Score == 0 {
		view.Taken.Score = poolTaken.Score
	}
	if view.Taken.ParamID == 0 && poolTaken.ParamID != 0 {
		view.Taken.ParamID = poolTaken.ParamID
		view.Taken.TargetLabel = poolTaken.TargetLabel
	}
	if view.Taken.TargetLabel == "" && poolTaken.TargetLabel != "" {
		view.Taken.TargetLabel = poolTaken.TargetLabel
	}
	if poolTaken.TargetCnt > view.Taken.TargetCnt {
		view.Taken.TargetCnt = poolTaken.TargetCnt
	}
	if poolTaken.FinishCnt > view.Taken.FinishCnt {
		view.Taken.FinishCnt = poolTaken.FinishCnt
	}
	if view.Taken.TaskType == 0 {
		view.Taken.TaskType = poolTaken.TaskType
	}
	if view.Taken.TaskId == 0 {
		view.Taken.TaskId = poolTaken.TaskId
	}
}

func racePoolHasMsID(tasks []FmlRaceTaskView, msID int64) bool {
	if msID == 0 {
		return false
	}
	for _, t := range tasks {
		if t.MsId == msID {
			return true
		}
	}
	return false
}

// applyFmlRaceTakenProgressLocked merges NS25 field 134 into Taken.
//
// Harvest / plant-harvest responses push:
//
//	{"<batchId>":{"3":IFmlRaceTakeTask,"4":uTimeMs}}
//
// Field 3 empty/null clears Taken when the held msId matches (finish/give-up).
// Non-empty takeTaskData advances FinishCnt/TargetCnt immediately so the
// planner can emit finishTask without waiting for the next getTaskList.
func applyFmlRaceTakenProgressLocked(view *FmlRaceView, raw json.RawMessage) {
	if view == nil || isJSONNull(raw) {
		return
	}
	var byBatch map[string]json.RawMessage
	if err := json.Unmarshal(raw, &byBatch); err != nil || len(byBatch) == 0 {
		return
	}
	rawEntry, ok := pickFmlRaceTakenProgressEntry(byBatch, view.BatchID)
	if !ok {
		return
	}
	if isJSONNull(rawEntry) {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawEntry, &fields); err != nil {
		return
	}
	rawTT, hasTT := fields["3"]
	if !hasTT {
		return
	}
	if isJSONNull(rawTT) || isJSONEmptyObject(rawTT) {
		// Finished / abandoned: clear only when we were holding a task.
		if view.Taken.HasTask {
			view.Taken = FmlRaceTakenView{}
			clearFmlRaceLocalFinish(view)
		}
		return
	}
	var tt clientproto.IFmlRaceTakeTask
	if err := json.Unmarshal(rawTT, &tt); err != nil || tt.TaskMsId == 0 {
		return
	}
	incoming := takenFromTakeTask(tt)
	if !view.Taken.HasTask || view.Taken.TaskMsId != incoming.TaskMsId {
		view.Taken = incoming
		resetFmlRaceLocalFinish(view, incoming.TaskMsId, incoming.FinishCnt)
	} else {
		mergeFmlRaceTakenProgress(&view.Taken, incoming)
		bumpFmlRaceLocalFinish(view, incoming.FinishCnt)
	}
	bumpFmlRacePoolProgress(view.Tasks, incoming.TaskMsId, incoming.TargetCnt, incoming.FinishCnt)
}

func clearFmlRaceLocalFinish(view *FmlRaceView) {
	if view == nil {
		return
	}
	view.LocalFinishCnt = 0
	view.LocalFinishTaskMsId = 0
}

func resetFmlRaceLocalFinish(view *FmlRaceView, taskMsId int64, finish int32) {
	if view == nil {
		return
	}
	view.LocalFinishTaskMsId = taskMsId
	if finish < 0 {
		finish = 0
	}
	view.LocalFinishCnt = finish
}

func bumpFmlRaceLocalFinish(view *FmlRaceView, finish int32) {
	if view == nil || !view.Taken.HasTask {
		return
	}
	if view.LocalFinishTaskMsId != view.Taken.TaskMsId {
		resetFmlRaceLocalFinish(view, view.Taken.TaskMsId, view.Taken.FinishCnt)
	}
	if finish > view.LocalFinishCnt {
		view.LocalFinishCnt = finish
	}
	if view.Taken.FinishCnt > view.LocalFinishCnt {
		view.LocalFinishCnt = view.Taken.FinishCnt
	}
}

// syncFmlRaceLocalFinishLocked keeps LocalFinishCnt ahead of lagging server
// FinishCnt using land HarvestCnt deltas for the held plant-harvest flower.
//
// Mid-cycle harvests bump HarvestCnt while the flower stays planted. The final
// harvest often clears the plot in the same delta (`100.1.<id>={}`), so we also
// credit remaining frequencys-HarvestCnt rounds when a race flower disappears.
func (s *State) syncFmlRaceLocalFinishLocked(landChanges []LandChange) {
	view := &s.fmlRace
	if !view.Taken.HasTask {
		clearFmlRaceLocalFinish(view)
		return
	}
	bumpFmlRaceLocalFinish(view, view.Taken.FinishCnt)
	taskType := view.Taken.TaskType
	if taskType == 0 {
		taskType = FmlRaceTaskTypeByID(view.Taken.TaskId)
	}
	if taskType != 3036 || view.Taken.ParamID <= 0 || len(landChanges) == 0 {
		return
	}
	flowerID := view.Taken.ParamID
	fallbackLvl := int32(0)
	if cv, ok := s.cultivations[flowerID]; ok && cv.Lvl > 0 {
		fallbackLvl = cv.Lvl
	}
	for _, ch := range landChanges {
		before := ch.Before
		after := ch.After
		if int32(before.FlowerID) != flowerID {
			continue
		}
		lvl := int32(before.Lvl)
		if lvl <= 0 {
			lvl = int32(after.Lvl)
		}
		if lvl <= 0 {
			lvl = fallbackLvl
		}
		yield, ok := FlowerLvlYieldByID(flowerID, lvl)
		if !ok || yield.CropGets <= 0 {
			continue
		}
		deltaRounds := int32(0)
		switch {
		case int32(after.FlowerID) == flowerID && after.HarvestCnt > before.HarvestCnt:
			deltaRounds = int32(after.HarvestCnt - before.HarvestCnt)
		case after.FlowerID == 0:
			// Final harvest cleared the land — credit unfinished rounds.
			if yield.Frequencys > 0 {
				remaining := yield.Frequencys - int32(before.HarvestCnt)
				if remaining > 0 {
					deltaRounds = remaining
				}
			} else {
				deltaRounds = 1
			}
		}
		if deltaRounds <= 0 {
			continue
		}
		view.LocalFinishCnt += deltaRounds * yield.CropGets
	}
}

func pickFmlRaceTakenProgressEntry(byBatch map[string]json.RawMessage, batchID int64) (json.RawMessage, bool) {
	if batchID > 0 {
		if raw, ok := byBatch[strconv.FormatInt(batchID, 10)]; ok {
			return raw, true
		}
	}
	for _, raw := range byBatch {
		return raw, true
	}
	return nil, false
}

func isJSONEmptyObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 2 && trimmed[0] == '{' && trimmed[1] == '}'
}

func takenFromTakeTask(tt clientproto.IFmlRaceTakeTask) FmlRaceTakenView {
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

func mergeFmlRaceTakenProgress(dst *FmlRaceTakenView, src FmlRaceTakenView) {
	if dst == nil || !src.HasTask {
		return
	}
	if src.Score > 0 && dst.Score == 0 {
		dst.Score = src.Score
	}
	if src.ParamID > 0 && dst.ParamID == 0 {
		dst.ParamID = src.ParamID
		dst.TargetLabel = src.TargetLabel
	}
	if src.TargetLabel != "" && dst.TargetLabel == "" {
		dst.TargetLabel = src.TargetLabel
	}
	if src.TargetCnt > dst.TargetCnt {
		dst.TargetCnt = src.TargetCnt
	}
	if src.FinishCnt > dst.FinishCnt {
		dst.FinishCnt = src.FinishCnt
	}
	if src.TaskType > 0 && dst.TaskType == 0 {
		dst.TaskType = src.TaskType
	}
	if src.TaskId > 0 && dst.TaskId == 0 {
		dst.TaskId = src.TaskId
	}
}

func bumpFmlRacePoolProgress(tasks []FmlRaceTaskView, msID int64, target, finish int32) {
	if msID == 0 {
		return
	}
	for i := range tasks {
		if tasks[i].MsId != msID {
			continue
		}
		if target > tasks[i].TargetCnt {
			tasks[i].TargetCnt = target
		}
		if finish > tasks[i].FinishCnt {
			tasks[i].FinishCnt = finish
		}
		return
	}
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

// parseFmlRaceUsrRcd extracts taken-task progress and task-quota counters from
// FmlRaceUsrRcdMap (namespace 25, field 110). Observed payloads key the map by
// batchId (not uid). Prefer batchId, then uid, then any entry with TakeTaskData
// (for taken) / any entry (for quota).
func parseFmlRaceUsrRcd(raw json.RawMessage, uid, batchID int64) (taken FmlRaceTakenView, finished, buy int32, quotaOK bool) {
	if len(raw) == 0 {
		return FmlRaceTakenView{}, 0, 0, false
	}
	var m map[string]clientproto.IFmlRaceUsrRcd
	if err := json.Unmarshal(raw, &m); err != nil || len(m) == 0 {
		return FmlRaceTakenView{}, 0, 0, false
	}
	tryKeys := make([]string, 0, 2)
	if batchID > 0 {
		tryKeys = append(tryKeys, strconv.FormatInt(batchID, 10))
	}
	if uid > 0 {
		tryKeys = append(tryKeys, strconv.FormatInt(uid, 10))
	}
	var preferred *clientproto.IFmlRaceUsrRcd
	for _, key := range tryKeys {
		if rcd, ok := m[key]; ok {
			preferred = &rcd
			break
		}
	}
	if preferred != nil {
		quotaOK = true
		finished = preferred.FTaskNum
		buy = preferred.BuyTaskNum
		taken = takenFromUsrRcd(*preferred)
		if taken.HasTask {
			return taken, finished, buy, true
		}
	}
	for _, rcd := range m {
		if view := takenFromUsrRcd(rcd); view.HasTask {
			if !quotaOK {
				finished = rcd.FTaskNum
				buy = rcd.BuyTaskNum
				quotaOK = true
			}
			return view, finished, buy, quotaOK
		}
	}
	if !quotaOK {
		for _, rcd := range m {
			finished = rcd.FTaskNum
			buy = rcd.BuyTaskNum
			quotaOK = true
			break
		}
	}
	return taken, finished, buy, quotaOK
}

func takenFromUsrRcd(rcd clientproto.IFmlRaceUsrRcd) FmlRaceTakenView {
	if rcd.TakeTaskData.TaskMsId == 0 {
		return FmlRaceTakenView{}
	}
	return takenFromTakeTask(rcd.TakeTaskData)
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

// MarkFmlRaceTasksSynced records a successful getTaskList round-trip even when
// the payload omitted field 114, so the planner does not re-sync every tick.
func (s *State) MarkFmlRaceTasksSynced() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlRace.TasksObserved = true
	s.fmlRace.TasksSyncedAtMs = time.Now().UnixMilli()
}

// MarkFmlRaceLvlSyncAttempt records that enter was used to seek raceLvl, so the
// planner waits before retrying when the payload still omitted the tier.
func (s *State) MarkFmlRaceLvlSyncAttempt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlRace.RaceLvlSyncAtMs = time.Now().UnixMilli()
}
