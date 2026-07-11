package state

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCyclicNoteEnterSnapshotAndExactPostcondition(t *testing.T) {
	s := applyCyclicNoteCaptureFixture(t)
	s.mu.Lock()
	s.activityBatches[9001].TaskList = nil
	s.activityBatches[9001].TaskListObserved = false
	s.activityBatches[9001].TaskListValid = true
	s.mu.Unlock()
	now := time.UnixMilli(cyclicNoteFixtureNowMs)

	snapshot, ok := s.CyclicNoteEnterSnapshot(now)
	if !ok || snapshot.BatchID != 9001 || snapshot.Phase != 2 || !snapshot.At.Equal(now) {
		t.Fatalf("CyclicNoteEnterSnapshot=(%+v,%t)", snapshot, ok)
	}
	if s.CyclicNoteEnterApplied(snapshot) {
		t.Fatal("enter postcondition accepted before task-list observation")
	}
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"0":[4003,2001,1006]}}}}}}`))
	if !s.CyclicNoteEnterApplied(snapshot) {
		t.Fatal("enter postcondition rejected exact observed task list")
	}

	wrong := snapshot
	wrong.BatchID++
	if s.CyclicNoteEnterApplied(wrong) {
		t.Fatal("enter postcondition accepted a different batch")
	}
}

func TestCyclicNoteTaskClaimSnapshotIsStrictAndUnique(t *testing.T) {
	s := applyCyclicNoteCaptureFixture(t)
	now := time.UnixMilli(cyclicNoteFixtureNowMs)
	snapshot, ok := s.CyclicNoteTaskClaimSnapshot(now, 9001, 1, 4003)
	if !ok || snapshot.BatchID != 9001 || snapshot.SlotID != 1 || snapshot.TaskID != 4003 ||
		snapshot.Target != 80 || snapshot.Progress != 81 || snapshot.FinishCount != 2 || !snapshot.FinishCountObserved ||
		!snapshot.At.Equal(now) {
		t.Fatalf("CyclicNoteTaskClaimSnapshot=(%+v,%t)", snapshot, ok)
	}

	for _, tc := range []struct {
		name              string
		batchID, slot, id int32
	}{
		{name: "wrong batch", batchID: 9002, slot: 1, id: 4003},
		{name: "wrong slot", batchID: 9001, slot: 2, id: 4003},
		{name: "incomplete", batchID: 9001, slot: 2, id: 2001},
		{name: "received", batchID: 9001, slot: 3, id: 1006},
		{name: "unknown", batchID: 9001, slot: 1, id: 999999},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, ready := s.CyclicNoteTaskClaimSnapshot(now, tc.batchID, tc.slot, tc.id); ready {
				t.Fatalf("snapshot=%+v, want rejected", got)
			}
		})
	}

	s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"0":[4003,4003,1006]}}}}}}`))
	if got, ready := s.CyclicNoteTaskClaimSnapshot(now, 9001, 1, 4003); ready {
		t.Fatalf("duplicate task snapshot=%+v, want rejected", got)
	}

	phaseThree := time.UnixMilli(s.activityBatches[9001].EndMs)
	if got, ready := s.CyclicNoteTaskClaimSnapshot(phaseThree, 9001, 1, 4003); ready {
		t.Fatalf("phase-3 task snapshot=%+v, want rejected", got)
	}
}

func TestCyclicNoteTaskClaimFinishCountFallbackRequiresKnownBaseline(t *testing.T) {
	s := applyCyclicNoteCaptureFixture(t)
	s.mu.Lock()
	s.activityBatches[9001].FinishCountObserved = false
	s.activityBatches[9001].FinishCountValid = false
	s.mu.Unlock()
	now := time.UnixMilli(cyclicNoteFixtureNowMs)
	snapshot, ok := s.CyclicNoteTaskClaimSnapshot(now, 9001, 1, 4003)
	if !ok || snapshot.FinishCountObserved {
		t.Fatalf("snapshot=(%+v,%t), want ready with unknown optional finish-count baseline", snapshot, ok)
	}
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"1":3}}}}}}`))
	s.ApplyV(json.RawMessage(`{"23":{"3":{"9001|0":{"3":{},"5":{}}}}}`))
	if s.CyclicNoteTaskClaimApplied(snapshot) {
		t.Fatal("postcondition inferred finish-count increase from an unknown baseline")
	}

	malformed := applyCyclicNoteCaptureFixture(t)
	malformed.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"1":"3"}}}}}}`))
	if got, ready := malformed.CyclicNoteTaskClaimSnapshot(now, 9001, 1, 4003); ready {
		t.Fatalf("malformed finish-count snapshot=%+v, want rejected", got)
	}
}

func TestCyclicNoteTaskClaimAppliedAcceptedTransitions(t *testing.T) {
	tests := []struct {
		name   string
		deltas []json.RawMessage
	}{
		{
			name:   "exact receipt",
			deltas: []json.RawMessage{json.RawMessage(`{"23":{"3":{"9001|0":{"5":{"4003":1}}}}}`)},
		},
		{
			name:   "exact slot changed",
			deltas: []json.RawMessage{json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"0":[2007,2001,1006]}}}}}}`)},
		},
		{
			name: "finish count and old task no longer ready",
			deltas: []json.RawMessage{
				json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"1":3}}}}}}`),
				json.RawMessage(`{"23":{"3":{"9001|0":{"3":{},"5":{}}}}}`),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := applyCyclicNoteCaptureFixture(t)
			snapshot, ok := s.CyclicNoteTaskClaimSnapshot(time.UnixMilli(cyclicNoteFixtureNowMs), 9001, 1, 4003)
			if !ok {
				t.Fatal("preflight snapshot unavailable")
			}
			for _, delta := range tc.deltas {
				s.ApplyV(delta)
			}
			if !s.CyclicNoteTaskClaimApplied(snapshot) {
				view, _ := s.CyclicNoteView(snapshot.At)
				t.Fatalf("postcondition rejected state after %s: %+v", tc.deltas, view)
			}
		})
	}
}

func TestCyclicNoteTaskClaimAppliedRejectsAmbiguousTransitions(t *testing.T) {
	tests := []struct {
		name  string
		delta json.RawMessage
	}{
		{name: "no state change", delta: json.RawMessage(`{"23":{"0":{"9001":{"11":81}}}}`)},
		{name: "finish only while still ready", delta: json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"1":3}}}}}}`)},
		{name: "progress cleared without finish", delta: json.RawMessage(`{"23":{"3":{"9001|0":{"3":{},"5":{}}}}}`)},
		{name: "malformed receipt map", delta: json.RawMessage(`{"23":{"3":{"9001|0":{"5":[]}}}}`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := applyCyclicNoteCaptureFixture(t)
			snapshot, ok := s.CyclicNoteTaskClaimSnapshot(time.UnixMilli(cyclicNoteFixtureNowMs), 9001, 1, 4003)
			if !ok {
				t.Fatal("preflight snapshot unavailable")
			}
			s.ApplyV(tc.delta)
			if s.CyclicNoteTaskClaimApplied(snapshot) {
				t.Fatalf("postcondition accepted ambiguous state after %s", tc.delta)
			}
		})
	}
}

func TestCyclicNoteMilestoneSnapshotAllowsGraceAndRequiresExactReceipt(t *testing.T) {
	s := applyCyclicNoteCaptureFixture(t)
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"11":120}}}}`))
	active := time.UnixMilli(cyclicNoteFixtureNowMs)
	snapshot, ok := s.CyclicNoteMilestoneClaimSnapshot(active, 9001, 2)
	if !ok || snapshot.BatchID != 9001 || snapshot.MilestoneIndex != 2 || snapshot.Target != 120 || snapshot.Score != 120 {
		t.Fatalf("active milestone snapshot=(%+v,%t)", snapshot, ok)
	}
	grace := time.UnixMilli(s.activityBatches[9001].EndMs)
	if graceSnapshot, ready := s.CyclicNoteMilestoneClaimSnapshot(grace, 9001, 2); !ready || graceSnapshot.MilestoneIndex != 2 {
		t.Fatalf("grace milestone snapshot=(%+v,%t)", graceSnapshot, ready)
	}
	if task, ready := s.CyclicNoteTaskClaimSnapshot(grace, 9001, 1, 4003); ready {
		t.Fatalf("grace task claim=%+v, want rejected", task)
	}

	wrong := applyCyclicNoteCaptureFixture(t)
	wrong.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"11":120,"13":[1,3]}}}}`))
	if wrong.CyclicNoteMilestoneClaimApplied(snapshot) {
		t.Fatal("milestone postcondition accepted a different claimed index")
	}
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"13":[1,2]}}}}`))
	if !s.CyclicNoteMilestoneClaimApplied(snapshot) {
		t.Fatal("milestone postcondition rejected exact claimed index")
	}

	for _, index := range []int32{1, 3, 99} {
		if got, ready := s.CyclicNoteMilestoneClaimSnapshot(active, 9001, index); ready {
			t.Fatalf("milestone %d snapshot=%+v, want rejected", index, got)
		}
	}
}
