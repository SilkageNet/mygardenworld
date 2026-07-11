package state

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

const cyclicNoteFixtureNowMs int64 = 1783696000000

func TestCyclicNoteCaptureFixture(t *testing.T) {
	s := applyCyclicNoteCaptureFixture(t)
	view, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
	if !ok || !view.Observed || !view.Found || !view.Valid {
		t.Fatalf("CyclicNoteView=(%+v,%t), want observed valid activity", view, ok)
	}
	if view.BatchID != 9001 || view.TmpID != 40020007 || view.TmpType != 4002 || view.Status != 1 || view.Phase != 2 {
		t.Fatalf("activity identity/phase=%+v", view)
	}
	if view.VisibleStartMs != 1783353600000 || view.BeginMs != 1783353600000 || view.EndMs != 1784563140000 ||
		view.GraceEndMs != 1784649540000 || view.PhaseEndMs != 1784563140000 {
		t.Fatalf("activity timing=%+v", view)
	}
	if view.Name != "花笺集芳7期" || view.Description != "苔绿贝壳花(7.7-7.20)" || view.Score != 81 ||
		view.CurrencyItemID != 1107 || view.CurrencyBalance != 5 || view.FinishCount != 2 ||
		view.LastRefreshTimeMs != 1783695955911 {
		t.Fatalf("activity summary=%+v", view)
	}
	if !view.TaskListObserved || !view.TaskRecordObserved || !view.MilestoneReceiptsObserved {
		t.Fatalf("observed flags=%+v", view)
	}
	if !reflect.DeepEqual(view.ClaimedMilestoneIndexes, []int32{1}) || view.Bag[1107] != 5 {
		t.Fatalf("bag/claimed=%v/%v", view.Bag, view.ClaimedMilestoneIndexes)
	}
	if len(view.Tasks) != 3 {
		t.Fatalf("tasks=%+v, want 3 slots", view.Tasks)
	}
	assertCyclicNoteTask(t, view.Tasks[0], 1, 4003, 3001, 81, 80, false)
	assertCyclicNoteTask(t, view.Tasks[1], 2, 2001, 3015, 70, 135, false)
	assertCyclicNoteTask(t, view.Tasks[2], 3, 1006, 1010, 3, 3, true)
	if len(view.Milestones) != 3 || view.Milestones[0].Index != 1 || view.Milestones[0].Target != 60 ||
		!view.Milestones[0].Received || view.Milestones[1].Target != 120 || view.Milestones[1].Received ||
		view.Milestones[2].Target != 265 || view.Milestones[2].Received {
		t.Fatalf("milestones=%+v", view.Milestones)
	}
}

func TestCyclicNoteEntrySparseMergePreservesAbsentFields(t *testing.T) {
	s := applyCyclicNoteCaptureFixture(t)
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"11":82,"14":{"105":{"1":3}}}},"1":{"40020007":{"2":"更新后的活动说明"}},"3":{"9001|0":{"6":1783697000000}}}}`))

	view, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
	if !ok || !view.Valid {
		t.Fatalf("CyclicNoteView=(%+v,%t), want valid after sparse delta", view, ok)
	}
	if view.BatchID != 9001 || view.BeginMs != 1783353600000 || view.EndMs != 1784563140000 ||
		view.Score != 82 || view.FinishCount != 3 || view.LastRefreshTimeMs != 1783695955911 ||
		view.Name != "花笺集芳7期" || view.Description != "更新后的活动说明" {
		t.Fatalf("sparse batch/template merge lost fields: %+v", view)
	}
	if len(view.Tasks) != 3 || view.Tasks[0].TaskID != 4003 || view.Tasks[0].Progress != 81 ||
		view.Tasks[2].TaskID != 1006 || !view.Tasks[2].Received {
		t.Fatalf("sparse record delta lost authoritative maps: %+v", view.Tasks)
	}
}

func TestCyclicNoteTaskListReplacementPreservesMalformedSlot(t *testing.T) {
	s := applyCyclicNoteCaptureFixture(t)
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"0":[2007,null,1005]}}}}}}`))
	locked, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
	if !ok || !locked.Valid || len(locked.Tasks) != 3 || locked.Tasks[1].Unlocked || locked.Tasks[1].SlotID != 2 {
		t.Fatalf("null task-list element must remain a valid locked slot: (%+v,%t)", locked.Tasks, ok)
	}

	s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"0":[2007,"2001",1005]}}}}}}`))

	batch := s.activityBatches[9001]
	if batch == nil || !batch.TaskListObserved || batch.TaskListValid || !reflect.DeepEqual(batch.TaskList, []int32{2007, 0, 1005}) {
		t.Fatalf("replacement task list=%+v", batch)
	}
	view, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
	if !ok || view.Valid {
		t.Fatalf("CyclicNoteView=(%+v,%t), malformed replacement must fail closed", view, ok)
	}
	if len(view.Tasks) != 3 || view.Tasks[0].SlotID != 1 || view.Tasks[0].TaskID != 2007 ||
		view.Tasks[1].SlotID != 2 || view.Tasks[1].Unlocked || view.Tasks[1].TaskID != 0 ||
		view.Tasks[2].SlotID != 3 || view.Tasks[2].TaskID != 1005 {
		t.Fatalf("malformed element collapsed slot positions: %+v", view.Tasks)
	}
}

func TestCyclicNoteTaskRecordMapsReplaceRatherThanMerge(t *testing.T) {
	s := applyCyclicNoteCaptureFixture(t)
	s.ApplyV(json.RawMessage(`{"23":{"3":{"9001|0":{"3":{"2001":71},"5":{"2001":0}}}}}`))

	record := s.activityTaskRecords["9001|0"]
	if record == nil || !record.ProgressObserved || !record.ProgressValid || !record.ReceiptsObserved || !record.ReceiptsValid ||
		!reflect.DeepEqual(record.Progress, map[int32]int32{2001: 71}) ||
		!reflect.DeepEqual(record.Receipts, map[int32]int32{2001: 0}) {
		t.Fatalf("task record maps were merged instead of replaced: %+v", record)
	}
	view, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
	if !ok || !view.Valid || view.Tasks[0].Progress != 0 || view.Tasks[0].Received ||
		view.Tasks[1].Progress != 71 || !view.Tasks[1].Received || view.Tasks[2].Progress != 0 || view.Tasks[2].Received {
		t.Fatalf("replacement maps not reflected in task views: %+v", view.Tasks)
	}

	s.ApplyV(json.RawMessage(`{"23":{"3":{"9001|0":{"3":{},"4":{},"5":{}}}}}`))
	record = s.activityTaskRecords["9001|0"]
	if len(record.Progress) != 0 || len(record.Receipts) != 0 || !record.ProgressObserved || !record.ReceiptsObserved {
		t.Fatalf("observed empty task maps did not clear old values: %+v", record)
	}
}

func TestCyclicNoteTopEntryNullDeletesState(t *testing.T) {
	s := applyCyclicNoteCaptureFixture(t)
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":null},"1":{"40020007":null},"3":{"9001|0":null}}}`))

	if _, exists := s.activityBatches[9001]; exists {
		t.Fatal("null batch entry did not delete state")
	}
	if _, exists := s.activityTemplates[40020007]; exists {
		t.Fatal("null template entry did not delete state")
	}
	if _, exists := s.activityTaskRecords["9001|0"]; exists {
		t.Fatal("null task-record entry did not delete state")
	}
	view, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
	if ok || !view.Observed || view.Found {
		t.Fatalf("CyclicNoteView after deletes=(%+v,%t)", view, ok)
	}
}

func TestCyclicNotePhaseExactBoundaries(t *testing.T) {
	batch := &activityBatchState{BeginMs: 1000, EndMs: 2000, DurationBeforeMs: 100, DurationAfterMs: 100}
	tests := []struct {
		now      int64
		phase    int32
		phaseEnd int64
	}{
		{now: 899, phase: 0, phaseEnd: 900},
		{now: 900, phase: 1, phaseEnd: 1000},
		{now: 999, phase: 1, phaseEnd: 1000},
		{now: 1000, phase: 2, phaseEnd: 2000},
		{now: 1999, phase: 2, phaseEnd: 2000},
		{now: 2000, phase: 3, phaseEnd: 2100},
		{now: 2099, phase: 3, phaseEnd: 2100},
		{now: 2100, phase: 4, phaseEnd: 2100},
	}
	for _, tc := range tests {
		phase, visibleStart, graceEnd, phaseEnd, ok := cyclicNotePhase(batch, tc.now)
		if !ok || phase != tc.phase || visibleStart != 900 || graceEnd != 2100 || phaseEnd != tc.phaseEnd {
			t.Errorf("cyclicNotePhase(now=%d)=(%d,%d,%d,%d,%t), want phase=%d end=%d", tc.now, phase, visibleStart, graceEnd, phaseEnd, ok, tc.phase, tc.phaseEnd)
		}
	}
}

func TestCyclicNoteCandidatePriorityAndNewestBegin(t *testing.T) {
	const nowMs int64 = 10000
	batch := func(id int32, begin, end, before, after int64) *activityBatchState {
		return &activityBatchState{BatchID: id, TmpType: 4002, Status: 1, BeginMs: begin, EndMs: end, DurationBeforeMs: before, DurationAfterMs: after}
	}
	s := New()
	s.activityBatches = map[int32]*activityBatchState{
		1: batch(1, 11000, 13000, 2000, 0), // phase 1
		2: batch(2, 7000, 9000, 0, 2000),   // phase 3
		3: batch(3, 8000, 11000, 0, 0),     // phase 2, older begin
		4: batch(4, 9000, 12000, 0, 0),     // phase 2, newer begin
	}
	selected, phase, _, _, _ := s.preferredCyclicNoteBatchLocked(nowMs)
	if selected == nil || selected.BatchID != 4 || phase != 2 {
		t.Fatalf("selected=(%+v, phase %d), want newest phase-2 batch", selected, phase)
	}

	delete(s.activityBatches, 3)
	delete(s.activityBatches, 4)
	selected, phase, _, _, _ = s.preferredCyclicNoteBatchLocked(nowMs)
	if selected == nil || selected.BatchID != 2 || phase != 3 {
		t.Fatalf("selected=(%+v, phase %d), want phase-3 before phase-1", selected, phase)
	}

	delete(s.activityBatches, 2)
	selected, phase, _, _, _ = s.preferredCyclicNoteBatchLocked(nowMs)
	if selected == nil || selected.BatchID != 1 || phase != 1 {
		t.Fatalf("selected=(%+v, phase %d), want remaining phase-1", selected, phase)
	}
}

func TestCyclicNoteViewIsDefensiveCopy(t *testing.T) {
	s := applyCyclicNoteCaptureFixture(t)
	first, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
	if !ok || !first.Valid {
		t.Fatalf("first view=(%+v,%t)", first, ok)
	}
	first.Bag[1107] = 999
	first.ClaimedMilestoneIndexes[0] = 999
	first.Tasks[0].TaskID = 999
	first.Tasks[0].Reward[0].Count = 999
	first.Tasks[0].FinishCost[0].Count = 999
	first.Milestones[0].Index = 999
	first.Milestones[0].Reward[0].Count = 999

	again, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
	if !ok || again.Bag[1107] != 5 || again.ClaimedMilestoneIndexes[0] != 1 || again.Tasks[0].TaskID != 4003 ||
		again.Tasks[0].Reward[0].Count != 4 || again.Tasks[0].FinishCost[0].Count != 36 ||
		again.Milestones[0].Index != 1 || again.Milestones[0].Reward[0].Count != 80 {
		t.Fatalf("view mutation leaked into state/catalog: %+v", again)
	}
}

func TestCyclicNoteInvalidAndUnknownFailClosed(t *testing.T) {
	t.Run("malformed namespace", func(t *testing.T) {
		s := New()
		s.ApplyV(json.RawMessage(`{"23":[]}`))
		view, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
		if ok || !view.Observed || view.Found || view.Valid {
			t.Fatalf("CyclicNoteView=(%+v,%t)", view, ok)
		}
	})

	t.Run("incomplete current batch", func(t *testing.T) {
		s := New()
		s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"0":9001,"1":40020007,"2":4002,"3":1,"5":1783353600000,"7":1784563140000,"8":0,"9":86400000}}}}`))
		view, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
		if !ok || !view.Found || view.Valid {
			t.Fatalf("CyclicNoteView=(%+v,%t), incomplete batch must remain visible but invalid", view, ok)
		}
	})

	t.Run("unknown task catalog row", func(t *testing.T) {
		s := applyCyclicNoteCaptureFixture(t)
		s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"0":[999999,2001,1006]}}}}}}`))
		view, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
		if !ok || !view.Valid || len(view.Tasks) != 3 || view.Tasks[0].CatalogKnown ||
			view.Tasks[0].Target != 0 || view.Tasks[0].Title != "" || len(view.Tasks[0].Reward) != 0 {
			t.Fatalf("CyclicNoteView=(%+v,%t), unknown slot must stay blocked without invalidating known slots", view, ok)
		}
	})

	malformedMaps := []struct {
		name     string
		delta    json.RawMessage
		progress bool
	}{
		{name: "quoted progress", delta: json.RawMessage(`{"23":{"3":{"9001|0":{"3":{"4003":"81","2001":71}}}}}`), progress: true},
		{name: "fractional progress", delta: json.RawMessage(`{"23":{"3":{"9001|0":{"3":{"4003":81.5}}}}}`), progress: true},
		{name: "overflow progress", delta: json.RawMessage(`{"23":{"3":{"9001|0":{"3":{"4003":2147483648}}}}}`), progress: true},
		{name: "noncanonical progress key", delta: json.RawMessage(`{"23":{"3":{"9001|0":{"3":{"04003":81}}}}}`), progress: true},
		{name: "quoted receipt", delta: json.RawMessage(`{"23":{"3":{"9001|0":{"5":{"4003":"1"}}}}}`)},
	}
	for _, tc := range malformedMaps {
		t.Run(tc.name, func(t *testing.T) {
			s := applyCyclicNoteCaptureFixture(t)
			s.ApplyV(tc.delta)
			view, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
			record := s.activityTaskRecords["9001|0"]
			mapValid := record.ReceiptsValid
			if tc.progress {
				mapValid = record.ProgressValid
			}
			if !ok || view.Valid || mapValid {
				t.Fatalf("CyclicNoteView=(%+v,%t), malformed authoritative map must fail closed; record=%+v", view, ok, record)
			}
		})
	}

	malformedAuthoritativeFields := []struct {
		name  string
		delta json.RawMessage
	}{
		{name: "score wrong type", delta: json.RawMessage(`{"23":{"0":{"9001":{"11":"81"}}}}`)},
		{name: "bag wrong container", delta: json.RawMessage(`{"23":{"0":{"9001":{"12":[]}}}}`)},
		{name: "claimed boxes wrong container", delta: json.RawMessage(`{"23":{"0":{"9001":{"13":{}}}}}`)},
		{name: "task list wrong container", delta: json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"0":{}}}}}}}`)},
		{name: "template boxes wrong container", delta: json.RawMessage(`{"23":{"1":{"40020007":{"9":{}}}}}`)},
		{name: "progress wrong container", delta: json.RawMessage(`{"23":{"3":{"9001|0":{"3":[]}}}}`)},
		{name: "receipts wrong container", delta: json.RawMessage(`{"23":{"3":{"9001|0":{"5":[]}}}}`)},
	}
	for _, tc := range malformedAuthoritativeFields {
		t.Run(tc.name, func(t *testing.T) {
			s := applyCyclicNoteCaptureFixture(t)
			s.ApplyV(tc.delta)
			view, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
			if !ok || view.Valid {
				t.Fatalf("CyclicNoteView=(%+v,%t), malformed present field must replace stale state and fail closed", view, ok)
			}
		})
	}

	t.Run("wrong status and type are not candidates", func(t *testing.T) {
		s := applyCyclicNoteCaptureFixture(t)
		s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"3":0},"9002":{"0":9002,"1":40020007,"2":4003,"3":1,"5":1783353600000,"7":1784563140000,"8":0,"9":0}}}}`))
		view, ok := s.CyclicNoteView(time.UnixMilli(cyclicNoteFixtureNowMs))
		if ok || view.Found || view.Valid {
			t.Fatalf("CyclicNoteView=(%+v,%t), no eligible candidate expected", view, ok)
		}
	})
}

func applyCyclicNoteCaptureFixture(t *testing.T) *State {
	t.Helper()
	raw, err := os.ReadFile("testdata/cyclic_note_activity.json")
	if err != nil {
		t.Fatalf("read cyclic-note fixture: %v", err)
	}
	s := New()
	s.ApplyV(raw)
	return s
}

func assertCyclicNoteTask(t *testing.T, task CyclicNoteTaskSlotView, slotID, taskID, taskType, progress, target int32, received bool) {
	t.Helper()
	if task.SlotID != slotID || !task.Unlocked || task.TaskID != taskID || task.TaskType != taskType ||
		!task.CatalogKnown || !task.ProgressObserved || !task.ReceiptObserved || task.Progress != progress ||
		task.Target != target || task.Received != received || len(task.Reward) == 0 || len(task.FinishCost) == 0 {
		t.Fatalf("task slot=%+v", task)
	}
}
