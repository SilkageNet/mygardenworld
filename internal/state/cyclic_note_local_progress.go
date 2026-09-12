package state

import "time"

// Cyclic note plant-any (3001) and flower-rack (3015) progress only advances in
// namespace 23.3. Business plant/sell RPCs must not rewrite that map. Local
// high-water counts let satisfy_tasks stop planting / list-unlist as soon as
// the remaining quota is exhausted, then enter reconciles with the server.

const (
	CyclicNoteTaskTypePlantAny   int32 = 3001
	CyclicNoteTaskTypeFlowerRack int32 = 3015

	// cyclicNoteProgressSyncMinInterval throttles re-enter while local
	// high-water is still ahead of observed 23.3. Empty enter payloads are
	// common once taskList exists; without a throttle the planner would
	// spin enter every decision tick, and clearing sync early freezes
	// flower-rack claim/cancel because demands disappear.
	cyclicNoteProgressSyncMinInterval = 30 * time.Second
)

// BumpCyclicNoteLocalProgress raises the absolute local high-water for one
// task type: max(currentLocal, serverProgress) + delta. delta <= 0 is ignored.
// Marks progress sync so the next enter can refresh authoritative 23.3 values.
func (s *State) BumpCyclicNoteLocalProgress(batchID, taskType, serverProgress, delta int32) {
	if s == nil || batchID <= 0 || taskType <= 0 || delta <= 0 {
		return
	}
	if serverProgress < 0 {
		serverProgress = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cyclicNoteLocalProgress == nil {
		s.cyclicNoteLocalProgress = make(map[int32]int32)
	}
	if s.cyclicNoteLocalBatchID != batchID {
		s.cyclicNoteLocalBatchID = batchID
		s.cyclicNoteLocalProgress = make(map[int32]int32)
		s.cyclicNoteLocalServerSeen = make(map[int32]int32)
		s.clearCyclicNoteFlowerRackPostCompleteLocked()
	}
	if s.cyclicNoteLocalServerSeen == nil {
		s.cyclicNoteLocalServerSeen = make(map[int32]int32)
	}
	// A lower server counter means the same task type was reassigned; drop
	// the stale high-water before applying this bump.
	if prev, ok := s.cyclicNoteLocalServerSeen[taskType]; ok && serverProgress < prev {
		delete(s.cyclicNoteLocalProgress, taskType)
	}
	s.cyclicNoteLocalServerSeen[taskType] = serverProgress
	base := serverProgress
	if cur := s.cyclicNoteLocalProgress[taskType]; cur > base {
		base = cur
	}
	s.cyclicNoteLocalProgress[taskType] = base + delta
	s.cyclicNoteProgressSyncNeeded = true
}

// CyclicNoteLocalProgress returns the local absolute high-water for taskType.
func (s *State) CyclicNoteLocalProgress(batchID, taskType int32) int32 {
	if s == nil || batchID <= 0 || taskType <= 0 {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cyclicNoteLocalBatchID != batchID {
		return 0
	}
	return s.cyclicNoteLocalProgress[taskType]
}

// MarkCyclicNoteFlowerRackRelistSlow switches flower-rack cancel from 5 to 7
// minutes when 5-minute relist is not advancing observed 23.3 progress.
func (s *State) MarkCyclicNoteFlowerRackRelistSlow() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.cyclicNoteFlowerRackRelistSlow = true
	s.mu.Unlock()
}

func (s *State) CyclicNoteFlowerRackRelistSlow() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cyclicNoteFlowerRackRelistSlow
}

func (s *State) clearCyclicNoteFlowerRackRelistSlowLocked() {
	s.cyclicNoteFlowerRackRelistSlow = false
}

func (s *State) clearCyclicNoteFlowerRackPostCompleteLocked() {
	s.cyclicNoteFlowerRackPostCompleteBatchID = 0
	s.cyclicNoteFlowerRackPostCompleteAtMs = 0
	s.cyclicNoteFlowerRackPostCompleteDone = false
}

// ClearCyclicNoteFlowerRackPostCompleteCancel drops the armed post-complete
// shelf-clear timer (unfinished 3015 owns the racks again).
func (s *State) ClearCyclicNoteFlowerRackPostCompleteCancel() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.clearCyclicNoteFlowerRackPostCompleteLocked()
	s.mu.Unlock()
}

// BeginCyclicNoteFlowerRackPostCompleteCancel arms the 7-minute all-shelf
// cancel the first time this batch's flower-rack task is observed complete.
// Re-arming the same batch keeps the original completion timestamp.
func (s *State) BeginCyclicNoteFlowerRackPostCompleteCancel(batchID int32, now time.Time) {
	if s == nil || batchID <= 0 {
		return
	}
	s.BeginCyclicNoteFlowerRackPostCompleteCancelAt(batchID, now.UnixMilli())
}

// BeginCyclicNoteFlowerRackPostCompleteCancelAt arms (or keeps) the post-complete
// timer using an explicit completion clock. Same-batch re-arms keep the earliest
// timestamp and never clear a finished one-shot.
func (s *State) BeginCyclicNoteFlowerRackPostCompleteCancelAt(batchID int32, atMs int64) {
	if s == nil || batchID <= 0 || atMs <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cyclicNoteFlowerRackPostCompleteBatchID != batchID {
		s.cyclicNoteFlowerRackPostCompleteBatchID = batchID
		s.cyclicNoteFlowerRackPostCompleteAtMs = atMs
		s.cyclicNoteFlowerRackPostCompleteDone = false
		return
	}
	if s.cyclicNoteFlowerRackPostCompleteAtMs <= 0 || atMs < s.cyclicNoteFlowerRackPostCompleteAtMs {
		s.cyclicNoteFlowerRackPostCompleteAtMs = atMs
	}
}

// CyclicNoteFlowerRackPostCompleteArmed reports an armed post-complete timer
// for batchID (including after the 3015 slot was claimed away).
func (s *State) CyclicNoteFlowerRackPostCompleteArmed() (batchID int32, ok bool) {
	if s == nil {
		return 0, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cyclicNoteFlowerRackPostCompleteBatchID <= 0 || s.cyclicNoteFlowerRackPostCompleteAtMs <= 0 {
		return 0, false
	}
	return s.cyclicNoteFlowerRackPostCompleteBatchID, true
}

// CyclicNoteFlowerRackPostCompleteCancelReady reports that the post-complete
// 7-minute wait has elapsed and the one-shot clear has not finished yet.
func (s *State) CyclicNoteFlowerRackPostCompleteCancelReady(batchID int32, now time.Time) bool {
	if s == nil || batchID <= 0 {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cyclicNoteFlowerRackPostCompleteBatchID != batchID || s.cyclicNoteFlowerRackPostCompleteDone ||
		s.cyclicNoteFlowerRackPostCompleteAtMs <= 0 {
		return false
	}
	return now.UnixMilli()-s.cyclicNoteFlowerRackPostCompleteAtMs >= int64((7 * time.Minute) / time.Millisecond)
}

// CyclicNoteFlowerRackPostCompleteCancelDone reports whether the one-shot
// post-complete shelf clear already finished for batchID.
func (s *State) CyclicNoteFlowerRackPostCompleteCancelDone(batchID int32) bool {
	if s == nil || batchID <= 0 {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cyclicNoteFlowerRackPostCompleteBatchID == batchID && s.cyclicNoteFlowerRackPostCompleteDone
}

// MarkCyclicNoteFlowerRackPostCompleteCancelDone records that occupied racks
// were cleared after the post-complete wait (sell_enabled may list again).
func (s *State) MarkCyclicNoteFlowerRackPostCompleteCancelDone(batchID int32) {
	if s == nil || batchID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cyclicNoteFlowerRackPostCompleteBatchID != batchID {
		return
	}
	s.cyclicNoteFlowerRackPostCompleteDone = true
}

// CyclicNoteFlowerRackRelistAfter returns the active cancel threshold.
func (s *State) CyclicNoteFlowerRackRelistAfter() time.Duration {
	if s != nil && s.CyclicNoteFlowerRackRelistSlow() {
		return 7 * time.Minute
	}
	return 5 * time.Minute
}

// LowerCyclicNoteLocalProgress reduces local high-water after a cancel/relist
// that never reached 23.3. Never drops below serverProgress.
func (s *State) LowerCyclicNoteLocalProgress(batchID, taskType, serverProgress, delta int32) {
	if s == nil || batchID <= 0 || taskType <= 0 || delta <= 0 {
		return
	}
	if serverProgress < 0 {
		serverProgress = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cyclicNoteLocalBatchID != batchID || s.cyclicNoteLocalProgress == nil {
		return
	}
	cur, ok := s.cyclicNoteLocalProgress[taskType]
	if !ok || cur <= serverProgress {
		return
	}
	next := cur - delta
	if next <= serverProgress {
		delete(s.cyclicNoteLocalProgress, taskType)
		if len(s.cyclicNoteLocalProgress) == 0 {
			s.clearCyclicNoteProgressSyncLocked()
		}
		return
	}
	s.cyclicNoteLocalProgress[taskType] = next
	s.cyclicNoteProgressSyncNeeded = true
}

// ClearCyclicNoteLocalProgress drops the local high-water for one task type
// (for example after cancel/relist stranded Missing at 0 while 23.3 lagged).
// Returns true when an entry was removed.
func (s *State) ClearCyclicNoteLocalProgress(batchID, taskType int32) bool {
	if s == nil || batchID <= 0 || taskType <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cyclicNoteLocalBatchID != batchID || s.cyclicNoteLocalProgress == nil {
		return false
	}
	if _, ok := s.cyclicNoteLocalProgress[taskType]; !ok {
		return false
	}
	delete(s.cyclicNoteLocalProgress, taskType)
	if len(s.cyclicNoteLocalProgress) == 0 {
		s.clearCyclicNoteProgressSyncLocked()
	}
	return true
}

// CyclicNoteEffectiveProgress is max(serverProgress, local high-water).
func (s *State) CyclicNoteEffectiveProgress(batchID, taskType, serverProgress int32) int32 {
	local := s.CyclicNoteLocalProgress(batchID, taskType)
	if local > serverProgress {
		return local
	}
	return serverProgress
}

// CyclicNoteServerProgressForType returns the best observed server progress
// among unlocked tasks of the given type on the preferred batch.
func (s *State) CyclicNoteServerProgressForType(now time.Time, taskType int32) (batchID, progress int32, ok bool) {
	if s == nil || taskType <= 0 {
		return 0, 0, false
	}
	view, found := s.CyclicNoteView(now)
	if !found || !view.Valid || view.BatchID <= 0 {
		return 0, 0, false
	}
	best := int32(-1)
	for _, task := range view.Tasks {
		if task.TaskType != taskType || !task.ProgressObserved || task.Progress < 0 {
			continue
		}
		if task.Progress > best {
			best = task.Progress
		}
	}
	if best < 0 {
		return view.BatchID, 0, true
	}
	return view.BatchID, best, true
}

// MarkCyclicNoteProgressSyncNeeded forces a follow-up actCyclicNote.enter after
// plant / flower-rack work. Enter often returns an empty delta when taskList is
// already present; reconcile keeps this flag set until observed 23.3 catches
// the local high-water (throttled by CyclicNoteEnterSnapshot).
func (s *State) MarkCyclicNoteProgressSyncNeeded() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.cyclicNoteProgressSyncNeeded = true
	s.mu.Unlock()
}

// CyclicNoteProgressSyncNeeded reports whether enter should refresh progress.
func (s *State) CyclicNoteProgressSyncNeeded() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cyclicNoteProgressSyncNeeded
}

// CyclicNoteProgressSyncDue reports whether a progress-sync enter may run now.
// Returns false while a recent enter is still within the throttle window.
func (s *State) CyclicNoteProgressSyncDue(now time.Time) bool {
	if s == nil || !s.CyclicNoteProgressSyncNeeded() {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cyclicNoteEnterAtMs <= 0 {
		return true
	}
	return now.UnixMilli()-s.cyclicNoteEnterAtMs >= int64(cyclicNoteProgressSyncMinInterval/time.Millisecond)
}

func (s *State) clearCyclicNoteProgressSyncLocked() {
	s.cyclicNoteProgressSyncNeeded = false
}

// ReconcileCyclicNoteLocalProgressFromTasks lowers local high-water once
// observed server progress catches up. Sync stays armed while any local
// high-water remains ahead of 23.3 so empty enter responses cannot strand
// flower-rack claim/cancel with a stale server counter.
//
// When 23.3 progress for a task type regresses (same catalog type reassigned
// after claim, counter back at 0), stale local high-water is discarded so
// satisfy_tasks can sell/plant for the new quota.
func (s *State) ReconcileCyclicNoteLocalProgressFromTasks(batchID int32, tasks []CyclicNoteTaskSlotView) {
	if s == nil || batchID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cyclicNoteLocalBatchID != batchID {
		s.clearCyclicNoteProgressSyncLocked()
		return
	}
	if s.cyclicNoteLocalServerSeen == nil {
		s.cyclicNoteLocalServerSeen = make(map[int32]int32)
	}
	serverByType := make(map[int32]int32)
	seenType := make(map[int32]bool)
	for _, task := range tasks {
		if task.TaskType <= 0 || !task.ProgressObserved {
			continue
		}
		seenType[task.TaskType] = true
		if task.Progress > serverByType[task.TaskType] {
			serverByType[task.TaskType] = task.Progress
		}
	}
	for taskType := range seenType {
		server := serverByType[taskType]
		prev, hadPrev := s.cyclicNoteLocalServerSeen[taskType]
		if hadPrev && server < prev {
			delete(s.cyclicNoteLocalProgress, taskType)
			if taskType == CyclicNoteTaskTypeFlowerRack {
				s.clearCyclicNoteFlowerRackPostCompleteLocked()
			}
		}
		if taskType == CyclicNoteTaskTypeFlowerRack && hadPrev && server > prev {
			s.clearCyclicNoteFlowerRackRelistSlowLocked()
		}
		s.cyclicNoteLocalServerSeen[taskType] = server
	}
	for taskType, local := range s.cyclicNoteLocalProgress {
		if server, ok := serverByType[taskType]; ok && server >= local {
			delete(s.cyclicNoteLocalProgress, taskType)
		}
	}
	if len(s.cyclicNoteLocalProgress) == 0 {
		s.clearCyclicNoteProgressSyncLocked()
		return
	}
	// Local still ahead of observed 23.3 — keep asking enter (throttled).
	s.cyclicNoteProgressSyncNeeded = true
}
