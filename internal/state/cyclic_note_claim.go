package state

import "time"

// CyclicNoteEnterSnapshot returns the exact dynamically selected batch only
// when an enter request is safe and still needed. It never guesses a batch ID.
func (s *State) CyclicNoteEnterSnapshot(now time.Time) (CyclicNoteEnterSnapshot, bool) {
	view, ok := s.CyclicNoteView(now)
	if !ok || !view.Valid || view.BatchID <= 0 || (view.Phase != 2 && view.Phase != 3) || view.TaskListObserved {
		return CyclicNoteEnterSnapshot{}, false
	}
	return CyclicNoteEnterSnapshot{At: now, BatchID: view.BatchID, Phase: view.Phase}, true
}

// CyclicNoteEnterApplied confirms that the same batch now carries a valid,
// authoritative task list. Re-selecting at the preflight instant makes a
// concurrent/newer batch fail closed instead of accepting the wrong response.
func (s *State) CyclicNoteEnterApplied(snapshot CyclicNoteEnterSnapshot) bool {
	if snapshot.BatchID <= 0 || (snapshot.Phase != 2 && snapshot.Phase != 3) {
		return false
	}
	view, ok := s.CyclicNoteView(snapshot.At)
	return ok && view.Valid && view.BatchID == snapshot.BatchID && view.TaskListObserved
}

// CyclicNoteTaskClaimSnapshot validates an exact unique slot/task pair. Both
// progress and receipt maps are authoritative replacements, so both must have
// been observed before readiness can be inferred.
func (s *State) CyclicNoteTaskClaimSnapshot(now time.Time, batchID, slotID, taskID int32) (CyclicNoteTaskClaimSnapshot, bool) {
	if batchID <= 0 || slotID <= 0 || taskID <= 0 {
		return CyclicNoteTaskClaimSnapshot{}, false
	}
	view, ok := s.CyclicNoteView(now)
	if !ok || !view.Valid || view.BatchID != batchID || view.Phase != 2 ||
		!view.TaskListObserved || !view.TaskRecordObserved {
		return CyclicNoteTaskClaimSnapshot{}, false
	}

	var selected *CyclicNoteTaskSlotView
	occurrences := 0
	for i := range view.Tasks {
		task := &view.Tasks[i]
		if task.Unlocked && task.TaskID == taskID {
			occurrences++
		}
		if task.SlotID == slotID {
			selected = task
		}
	}
	if occurrences != 1 || selected == nil || !selected.Unlocked || selected.TaskID != taskID ||
		!selected.CatalogKnown || selected.Target <= 0 || !selected.ProgressObserved ||
		!selected.ReceiptObserved || selected.Received || selected.Progress < selected.Target {
		return CyclicNoteTaskClaimSnapshot{}, false
	}
	return CyclicNoteTaskClaimSnapshot{
		At: now, BatchID: batchID, SlotID: slotID, TaskID: taskID,
		Target: selected.Target, Progress: selected.Progress, FinishCount: view.FinishCount,
		FinishCountObserved: view.FinishCountObserved,
	}, true
}

// CyclicNoteTaskClaimApplied accepts only an exact receipt, an authoritative
// replacement of the exact slot, or a finish-count increase accompanied by
// authoritative evidence that the old task is no longer ready.
func (s *State) CyclicNoteTaskClaimApplied(snapshot CyclicNoteTaskClaimSnapshot) bool {
	if snapshot.BatchID <= 0 || snapshot.SlotID <= 0 || snapshot.TaskID <= 0 || snapshot.Target <= 0 ||
		snapshot.Progress < snapshot.Target {
		return false
	}
	view, ok := s.CyclicNoteView(snapshot.At)
	if !ok || !view.Valid || view.BatchID != snapshot.BatchID || !view.TaskListObserved {
		return false
	}
	var current *CyclicNoteTaskSlotView
	for i := range view.Tasks {
		if view.Tasks[i].SlotID == snapshot.SlotID {
			current = &view.Tasks[i]
			break
		}
	}
	if current == nil {
		return false
	}
	if current.TaskID != snapshot.TaskID {
		return true
	}
	if current.ReceiptObserved && current.Received {
		return true
	}
	return snapshot.FinishCountObserved && view.FinishCountObserved && view.FinishCount > snapshot.FinishCount &&
		current.CatalogKnown && current.Target == snapshot.Target &&
		current.ProgressObserved && current.ReceiptObserved && !current.Received && current.Progress < current.Target
}

// CyclicNoteMilestoneClaimSnapshot validates a server-configured milestone idx
// against the raw score. Reward-grace phase (3) remains claimable.
func (s *State) CyclicNoteMilestoneClaimSnapshot(now time.Time, batchID, index int32) (CyclicNoteMilestoneClaimSnapshot, bool) {
	if batchID <= 0 || index <= 0 {
		return CyclicNoteMilestoneClaimSnapshot{}, false
	}
	view, ok := s.CyclicNoteView(now)
	if !ok || !view.Valid || view.BatchID != batchID || (view.Phase != 2 && view.Phase != 3) ||
		!view.MilestoneReceiptsObserved {
		return CyclicNoteMilestoneClaimSnapshot{}, false
	}
	var selected *CyclicNoteMilestoneView
	occurrences := 0
	for i := range view.Milestones {
		milestone := &view.Milestones[i]
		if milestone.Index == index {
			selected = milestone
			occurrences++
		}
	}
	if occurrences != 1 || selected == nil || selected.Target <= 0 || selected.Received || view.Score < selected.Target {
		return CyclicNoteMilestoneClaimSnapshot{}, false
	}
	return CyclicNoteMilestoneClaimSnapshot{
		At: now, BatchID: batchID, MilestoneIndex: index, Target: selected.Target, Score: view.Score,
	}, true
}

// CyclicNoteMilestoneClaimApplied requires the exact milestone idx to appear
// in the authoritative claimed-box list of the same batch.
func (s *State) CyclicNoteMilestoneClaimApplied(snapshot CyclicNoteMilestoneClaimSnapshot) bool {
	if snapshot.BatchID <= 0 || snapshot.MilestoneIndex <= 0 || snapshot.Target <= 0 || snapshot.Score < snapshot.Target {
		return false
	}
	view, ok := s.CyclicNoteView(snapshot.At)
	if !ok || !view.Valid || view.BatchID != snapshot.BatchID || !view.MilestoneReceiptsObserved {
		return false
	}
	for _, milestone := range view.Milestones {
		if milestone.Index == snapshot.MilestoneIndex {
			return milestone.Target == snapshot.Target && milestone.Received
		}
	}
	return false
}
