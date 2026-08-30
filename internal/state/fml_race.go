package state

import (
	"bytes"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func applyFmlRaceBatchLocked(view *FmlRaceView, raw json.RawMessage) {
	if isJSONNull(raw) {
		view.Observed = false
		view.BatchActive = false
		view.BatchID = 0
		view.BatchStatus = 0
		view.BatchStartMs = 0
		view.BatchEndMs = 0
		view.TakeQuotaExhausted = false
		resetFmlRaceTaskPoolForBatch(view)
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
	prevBatchID := view.BatchID
	view.Observed = true
	view.BatchID = batch.BatchId
	view.BatchStatus = batch.Status
	view.BatchStartMs = batch.StartTime
	view.BatchEndMs = batch.EndTime
	view.BatchActive = fmlRaceBatchActive(batch.Status, batch.StartTime, batch.EndTime, time.Now())
	if batch.BatchId != prevBatchID {
		view.TakeQuotaExhausted = false
		resetFmlRaceTaskPoolForBatch(view)
		// Quota counters are per-batch. A new batch's sparse 110 row omits
		// fTaskNum/buyTaskNum while zero, so presence-merge alone would keep
		// the previous batch's counts and AutoStopOnQuotaDone could block all
		// takes of the new batch. Reset and let 110/116 re-observe.
		view.TaskQuotaObserved = false
		view.FinishedTaskNum = 0
		view.BuyTaskNum = 0
		view.ScoreObserved = false
		view.Score = 0
		view.ScoreTimeMs = 0
		view.RankObserved = false
		view.Rank = 0
		view.RaceQuotaSyncAtMs = 0
	}
}

func resetFmlRaceTaskPoolForBatch(view *FmlRaceView) {
	view.TasksObserved = false
	view.TaskPoolStale = false
	view.TaskPoolSyncAttemptAtMs = 0
	view.TasksSyncedAtMs = 0
	view.Tasks = nil
	view.Taken = FmlRaceTakenView{}
	view.MissingParamRefreshFP = ""
	view.LocalFinishCnt = 0
	view.LocalFinishTaskMsId = 0
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
			MsId:           t.MsId,
			TaskId:         t.TaskId,
			TaskType:       FmlRaceTaskTypeByID(t.TaskId),
			Score:          t.Score,
			IsUpgrade:      t.IsUpgrade,
			UpgradeUid:     t.UpgradeUid,
			UID:            t.UID,
			ParamID:        paramID,
			TargetLabel:    ItemLabel(paramID),
			AppearTime:     t.AppearTime,
			TargetCnt:      t.TargetCnt,
			FinishCnt:      t.FinishCnt,
			TakeLimitMin:   t.TakeLimitMin,
			TakeExpireTime: t.TakeExpireTime,
		})
	}
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
	updateFmlRaceMissingParamRefreshFP(view, fullPool)
}

// FmlRacePoolMissingParam reports whether any pool task whose target controls
// safe execution lacks a resolved ParamID. Plant-harvest tasks use a flower
// target and flower-art-craft tasks use a vase target.
func FmlRacePoolMissingParam(tasks []FmlRaceTaskView) bool {
	for _, t := range tasks {
		taskType := t.TaskType
		if taskType == 0 {
			taskType = t.TaskId
		}
		switch taskType {
		case 3036, 3034:
			if t.ParamID <= 0 {
				return true
			}
		}
	}
	return false
}

// FmlRaceMissingParamFingerprint is a stable key for only the pool rows whose
// safe execution target is missing. Including the task instance and catalog id
// lets a newly incomplete row trigger its own refresh even if the overall pool
// still contains the same task ids.
func FmlRaceMissingParamFingerprint(tasks []FmlRaceTaskView) string {
	missing := make([]FmlRaceTaskView, 0, len(tasks))
	for _, task := range tasks {
		taskType := task.TaskType
		if taskType == 0 {
			taskType = task.TaskId
		}
		if (taskType == 3036 || taskType == 3034) && task.ParamID <= 0 {
			missing = append(missing, task)
		}
	}
	if len(missing) == 0 {
		return ""
	}
	sort.Slice(missing, func(i, j int) bool {
		if missing[i].MsId != missing[j].MsId {
			return missing[i].MsId < missing[j].MsId
		}
		return missing[i].TaskId < missing[j].TaskId
	})
	parts := make([]string, len(missing))
	for i, task := range missing {
		parts[i] = strconv.FormatInt(task.MsId, 10) + ":" + strconv.FormatInt(int64(task.TaskId), 10)
	}
	return strings.Join(parts, ",")
}

func updateFmlRaceMissingParamRefreshFP(view *FmlRaceView, refreshAttempt bool) {
	if !FmlRacePoolMissingParam(view.Tasks) {
		view.MissingParamRefreshFP = ""
		return
	}
	// Ordinary namespace pushes can introduce a newly incomplete row and must
	// not masquerade as the explicit getTaskList refresh intended to fill it.
	// Full-pool apply (and NoteFmlRaceTaskPoolSync for empty deltas) records the
	// attempt; a different incomplete fingerprint remains eligible for refresh.
	if refreshAttempt {
		view.MissingParamRefreshFP = FmlRaceMissingParamFingerprint(view.Tasks)
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
	if next.TakeLimitMin == 0 && prev.TakeLimitMin != 0 {
		next.TakeLimitMin = prev.TakeLimitMin
	}
	if next.TakeExpireTime == 0 && prev.TakeExpireTime != 0 {
		next.TakeExpireTime = prev.TakeExpireTime
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
			TaskMsId:     t.MsId,
			TaskId:       t.TaskId,
			TaskType:     taskType,
			Score:        t.Score,
			TargetCnt:    t.TargetCnt,
			FinishCnt:    t.FinishCnt,
			ParamID:      t.ParamID,
			TargetLabel:  t.TargetLabel,
			TakeLimitMin: t.TakeLimitMin,
			ExpireTime:   t.TakeExpireTime,
			HasTask:      true,
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
		if taken.TakeLimitMin == 0 && t.TakeLimitMin > 0 {
			taken.TakeLimitMin = t.TakeLimitMin
		}
		if taken.ExpireTime == 0 && t.TakeExpireTime > 0 {
			taken.ExpireTime = t.TakeExpireTime
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
	if view.Taken.TakeLimitMin == 0 && poolTaken.TakeLimitMin > 0 {
		view.Taken.TakeLimitMin = poolTaken.TakeLimitMin
	}
	if view.Taken.ExpireTime == 0 && poolTaken.ExpireTime > 0 {
		view.Taken.ExpireTime = poolTaken.ExpireTime
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

// reconcileFmlRaceLocalFinishAfterFullPool drops an inflated LocalFinishCnt
// after getTaskList left authoritative FinishCnt short of TargetCnt.
func reconcileFmlRaceLocalFinishAfterFullPool(view *FmlRaceView) {
	if view == nil {
		return
	}
	taken := view.Taken
	if !taken.HasTask || taken.TargetCnt <= 0 || taken.FinishCnt >= taken.TargetCnt {
		return
	}
	if view.LocalFinishTaskMsId != taken.TaskMsId || view.LocalFinishCnt < taken.TargetCnt {
		return
	}
	view.LocalFinishCnt = taken.FinishCnt
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
		ExpireTime:  tt.ExpireTime,
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
	if src.TakeLimitMin > 0 && dst.TakeLimitMin == 0 {
		dst.TakeLimitMin = src.TakeLimitMin
	}
	if src.TakenAtMs > 0 && dst.TakenAtMs == 0 {
		dst.TakenAtMs = src.TakenAtMs
	}
	if src.ExpireTime > 0 {
		// Protocol expire wins over a locally computed deadline.
		dst.ExpireTime = src.ExpireTime
	}
}

// finalizeFmlRaceTakenDeadline stamps TakenAtMs on a new hold and fills
// ExpireTime from TakenAtMs + TakeLimitMin when the server omitted expireTime.
func finalizeFmlRaceTakenDeadline(view *FmlRaceView, prev FmlRaceTakenView, nowMs int64) {
	if view == nil || !view.Taken.HasTask || view.Taken.TaskMsId == 0 {
		return
	}
	enrichFmlRaceTakenFromTask(&view.Taken, view.Tasks)
	if prev.HasTask && prev.TaskMsId == view.Taken.TaskMsId {
		if view.Taken.TakenAtMs == 0 && prev.TakenAtMs > 0 {
			view.Taken.TakenAtMs = prev.TakenAtMs
		}
		if view.Taken.TakeLimitMin == 0 && prev.TakeLimitMin > 0 {
			view.Taken.TakeLimitMin = prev.TakeLimitMin
		}
		// Keep a previously computed deadline unless protocol already set one.
		if view.Taken.ExpireTime == 0 && prev.ExpireTime > 0 {
			view.Taken.ExpireTime = prev.ExpireTime
		}
	}
	if view.Taken.TakenAtMs == 0 {
		if nowMs > 0 {
			view.Taken.TakenAtMs = nowMs
		} else {
			view.Taken.TakenAtMs = time.Now().UnixMilli()
		}
	}
	if view.Taken.ExpireTime == 0 && view.Taken.TakenAtMs > 0 && view.Taken.TakeLimitMin > 0 {
		view.Taken.ExpireTime = view.Taken.TakenAtMs + int64(view.Taken.TakeLimitMin)*int64(time.Minute/time.Millisecond)
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
func fmlRaceBatchActive(status int32, startMs, endMs int64, now time.Time) bool {
	if status == 2 {
		return false
	}
	if status == 1 {
		return true
	}
	if startMs <= 0 || endMs <= startMs {
		return false
	}
	ms := now.UnixMilli()
	return ms >= startMs && ms < endMs
}

// ActiveAt re-evaluates BatchActive at planner/UI time so a published
// status==0 window can open without waiting for another field-111 apply.
func (v FmlRaceView) ActiveAt(now time.Time) bool {
	return fmlRaceBatchActive(v.BatchStatus, v.BatchStartMs, v.BatchEndMs, now)
}

// Observed guild-race session: Tuesday 09:00 to Sunday 21:00 Asia/Shanghai.
// Production batch 1783990800000–1784466000000 is 2026-07-14 09:00 – 2026-07-19 21:00.
const (
	fmlRaceCalendarStartWeekday = time.Tuesday
	fmlRaceCalendarStartHour    = 9
	fmlRaceCalendarEndWeekday   = time.Sunday
	fmlRaceCalendarEndHour      = 21
)

// FmlRaceCalendarSessionStart is the Tuesday 09:00 Asia/Shanghai that opened
// the current race week (last Tuesday when called Mon–Sun).
func FmlRaceCalendarSessionStart(now time.Time) time.Time {
	loc := gameDayLocation()
	local := now.In(loc)
	daysSinceStart := (int(local.Weekday()) - int(fmlRaceCalendarStartWeekday) + 7) % 7
	y, m, d := local.Date()
	return time.Date(y, m, d, fmlRaceCalendarStartHour, 0, 0, 0, loc).AddDate(0, 0, -daysSinceStart)
}

// FmlRaceCalendarSessionEnd is Sunday 21:00 Asia/Shanghai of the same race week.
func FmlRaceCalendarSessionEnd(now time.Time) time.Time {
	start := FmlRaceCalendarSessionStart(now)
	days := int(fmlRaceCalendarEndWeekday - fmlRaceCalendarStartWeekday)
	if days < 0 {
		days += 7
	}
	return time.Date(start.Year(), start.Month(), start.Day(), fmlRaceCalendarEndHour, 0, 0, 0, start.Location()).AddDate(0, 0, days)
}

// FmlRaceCalendarInSession reports the observed weekly open window.
func FmlRaceCalendarInSession(now time.Time) bool {
	start := FmlRaceCalendarSessionStart(now)
	end := FmlRaceCalendarSessionEnd(now)
	return !now.Before(start) && now.Before(end)
}

// applyFmlRaceUsrRankListLocked updates FinishedTaskNum/BuyTaskNum/Score from
// FmlRaceUsrRankList (ns25 field 116) and derives personal Rank by sorting the
// list (score desc, scoreTime asc). takeTaskData is ignored so a rank snapshot
// cannot clobber live Taken.
func applyFmlRaceUsrRankListLocked(view *FmlRaceView, raw json.RawMessage, uid int64) {
	if len(raw) == 0 || isJSONNull(raw) || uid <= 0 {
		return
	}
	var rows []json.RawMessage
	if json.Unmarshal(raw, &rows) != nil || len(rows) == 0 {
		return
	}
	type rankRow struct {
		uid       int64
		score     int32
		scoreTime int64
	}
	ranked := make([]rankRow, 0, len(rows))
	var selfFound bool
	for _, row := range rows {
		var fields map[string]json.RawMessage
		if json.Unmarshal(row, &fields) != nil {
			continue
		}
		var rcd clientproto.IFmlRaceUsrRcd
		if json.Unmarshal(row, &rcd) != nil || rcd.UID == 0 {
			continue
		}
		if view.BatchID > 0 && rcd.BatchId > 0 && rcd.BatchId != view.BatchID {
			continue
		}
		_, hasScore := fields["4"]
		ranked = append(ranked, rankRow{
			uid:       rcd.UID,
			score:     rcd.Score,
			scoreTime: rcd.ScoreTime,
		})
		if rcd.UID != uid {
			continue
		}
		selfFound = true
		if _, ok := fields["3"]; ok {
			view.FinishedTaskNum = rcd.FTaskNum
			view.TaskQuotaObserved = true
		}
		if _, ok := fields["6"]; ok {
			view.BuyTaskNum = rcd.BuyTaskNum
			view.TaskQuotaObserved = true
		}
		if hasScore {
			view.Score = rcd.Score
			view.ScoreObserved = true
			if _, ok := fields["5"]; ok {
				view.ScoreTimeMs = rcd.ScoreTime
			}
		}
	}
	if !selfFound || len(ranked) == 0 {
		return
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].scoreTime != ranked[j].scoreTime {
			// Earlier scoreTime ranks higher on a tie (reached the score first).
			if ranked[i].scoreTime == 0 {
				return false
			}
			if ranked[j].scoreTime == 0 {
				return true
			}
			return ranked[i].scoreTime < ranked[j].scoreTime
		}
		return ranked[i].uid < ranked[j].uid
	})
	for i, row := range ranked {
		if row.uid != uid {
			continue
		}
		view.Rank = int32(i + 1)
		view.RankObserved = true
		return
	}
}

// parseFmlRaceUsrRcd extracts taken-task progress, task-quota counters, and
// personal race score from FmlRaceUsrRcdMap (namespace 25, field 110). Observed
// payloads key the map by batchId (not uid). Prefer batchId, then uid, then any
// entry with TakeTaskData (for taken) / any entry (for quota/score fields that
// are actually present).
//
// finishedOK / buyOK / scoreOK are true only when JSON keys "3" / "6" / "4"
// appear on the chosen row. giveUpTask often returns {"8":giveUpTime,"9":uTime}
// without fTaskNum; callers must keep prior FinishedTaskNum in that case.
func parseFmlRaceUsrRcd(raw json.RawMessage, uid, batchID int64) (
	taken FmlRaceTakenView, finished, buy, score int32, scoreTime int64, finishedOK, buyOK, scoreOK bool,
) {
	if len(raw) == 0 {
		return FmlRaceTakenView{}, 0, 0, 0, 0, false, false, false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil || len(m) == 0 {
		return FmlRaceTakenView{}, 0, 0, 0, 0, false, false, false
	}
	tryKeys := make([]string, 0, 2)
	if batchID > 0 {
		tryKeys = append(tryKeys, strconv.FormatInt(batchID, 10))
	}
	if uid > 0 {
		tryKeys = append(tryKeys, strconv.FormatInt(uid, 10))
	}
	type usrRcdFields struct {
		taken     FmlRaceTakenView
		finished  int32
		buy       int32
		score     int32
		scoreTime int64
		fOK       bool
		bOK       bool
		sOK       bool
	}
	read := func(rcdRaw json.RawMessage) usrRcdFields {
		var fields map[string]json.RawMessage
		if json.Unmarshal(rcdRaw, &fields) != nil {
			return usrRcdFields{}
		}
		var rcd clientproto.IFmlRaceUsrRcd
		if json.Unmarshal(rcdRaw, &rcd) != nil {
			return usrRcdFields{}
		}
		_, fOK := fields["3"]
		_, bOK := fields["6"]
		_, sOK := fields["4"]
		scoreTime := int64(0)
		if _, ok := fields["5"]; ok {
			scoreTime = rcd.ScoreTime
		}
		return usrRcdFields{
			taken:     takenFromUsrRcd(rcd),
			finished:  rcd.FTaskNum,
			buy:       rcd.BuyTaskNum,
			score:     rcd.Score,
			scoreTime: scoreTime,
			fOK:       fOK,
			bOK:       bOK,
			sOK:       sOK,
		}
	}
	pack := func(r usrRcdFields) (FmlRaceTakenView, int32, int32, int32, int64, bool, bool, bool) {
		return r.taken, r.finished, r.buy, r.score, r.scoreTime, r.fOK, r.bOK, r.sOK
	}
	var preferredRaw json.RawMessage
	for _, key := range tryKeys {
		if rcdRaw, ok := m[key]; ok {
			preferredRaw = rcdRaw
			break
		}
	}
	if preferredRaw != nil {
		r := read(preferredRaw)
		taken, finished, buy, score, scoreTime, finishedOK, buyOK, scoreOK = pack(r)
		if r.taken.HasTask {
			return taken, finished, buy, score, scoreTime, finishedOK, buyOK, scoreOK
		}
	}
	for _, rcdRaw := range m {
		r := read(rcdRaw)
		if !r.taken.HasTask {
			continue
		}
		if preferredRaw == nil {
			finished, buy, score, scoreTime, finishedOK, buyOK, scoreOK = r.finished, r.buy, r.score, r.scoreTime, r.fOK, r.bOK, r.sOK
		}
		return r.taken, finished, buy, score, scoreTime, finishedOK, buyOK, scoreOK
	}
	if preferredRaw == nil {
		for _, rcdRaw := range m {
			return pack(read(rcdRaw))
		}
	}
	return taken, finished, buy, score, scoreTime, finishedOK, buyOK, scoreOK
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

// MarkFmlRaceTaskPoolStale forces the next race tick to re-fetch getTaskList
// without lying about whether field 114 was previously observed or wiping the
// last snapshot used for display.
func (s *State) MarkFmlRaceTaskPoolStale() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlRace.TaskPoolStale = true
	s.fmlRace.TaskPoolSyncAttemptAtMs = 0
}

// MarkFmlRaceSessionStale clears enter + task-pool observation so the planner
// runs enter → getTaskList before take after transient server rejects (e.g. 221).
func (s *State) MarkFmlRaceSessionStale() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlRace.Observed = false
	s.fmlRace.TaskPoolStale = true
	s.fmlRace.TaskPoolSyncAttemptAtMs = 0
}

// NoteFmlRaceTaskPoolSync records a successful getTaskList round-trip. The
// server uses delta responses, so an omitted field 114 confirms an existing
// observed snapshot but must not pretend a never-observed pool was received.
// In both cases the attempt and missing-target fingerprint are recorded so a
// successful empty delta cannot cause a decision-interval sync loop.
func (s *State) NoteFmlRaceTaskPoolSync(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if at.IsZero() {
		at = time.Now()
	}
	nowMs := at.UnixMilli()
	s.fmlRace.TaskPoolSyncAttemptAtMs = nowMs
	s.fmlRace.TaskPoolStale = false
	if s.fmlRace.TasksObserved {
		s.fmlRace.TasksSyncedAtMs = nowMs
	}
	if FmlRacePoolMissingParam(s.fmlRace.Tasks) {
		s.fmlRace.MissingParamRefreshFP = FmlRaceMissingParamFingerprint(s.fmlRace.Tasks)
	} else {
		s.fmlRace.MissingParamRefreshFP = ""
	}
}

// MarkFmlRaceLvlSyncAttempt records that enter was used to seek raceLvl, so the
// planner waits before retrying when the payload still omitted the tier.
func (s *State) MarkFmlRaceLvlSyncAttempt() {
	s.MarkFmlRaceLvlSyncAttemptAt(time.Now())
}

// MarkFmlRaceLvlSyncAttemptAt records an enter attempt at a specific time so
// inactive-batch re-enter and raceLvl retry share one clock.
func (s *State) MarkFmlRaceLvlSyncAttemptAt(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlRace.RaceLvlSyncAtMs = at.UnixMilli()
}

// MarkFmlRaceQuotaSyncAttempt records that getFmlRaceUsrRankList was used to
// seek fTaskNum / personal score / rank, so the planner backs off when the
// payload still omitted them.
func (s *State) MarkFmlRaceQuotaSyncAttempt() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlRace.RaceQuotaSyncAtMs = time.Now().UnixMilli()
}

// MarkFmlRacePoolTaskClaimed marks a pool row as already taken so automation
// will not re-select it before the next getTaskList refresh. Used when
// takeTask reports the task was claimed by another member while local state
// still showed UID==0.
func (s *State) MarkFmlRacePoolTaskClaimed(msID int64) {
	if msID == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.fmlRace.Tasks {
		if s.fmlRace.Tasks[i].MsId != msID || s.fmlRace.Tasks[i].UID != 0 {
			continue
		}
		// Placeholder non-self UID so RaceTakeSkipReason returns「已被接取」.
		s.fmlRace.Tasks[i].UID = -1
		return
	}
}

// MarkFmlRaceTakeQuotaExhausted stops further takeTask planning for the current
// race batch after the server reports the take-count limit.
func (s *State) MarkFmlRaceTakeQuotaExhausted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlRace.TakeQuotaExhausted = true
}
