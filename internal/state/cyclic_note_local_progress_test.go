package state

import (
	"testing"
	"time"
)

func TestCyclicNoteLocalProgressResetsOnServerRegression(t *testing.T) {
	s := New()
	s.BumpCyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack, 123, 12) // local 135
	if got := s.CyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack); got != 135 {
		t.Fatalf("local=%d, want 135", got)
	}

	// Same task type reassigned: server counter back at 0.
	s.ReconcileCyclicNoteLocalProgressFromTasks(1390, []CyclicNoteTaskSlotView{{
		TaskType:         CyclicNoteTaskTypeFlowerRack,
		Progress:         0,
		ProgressObserved: true,
	}})
	if got := s.CyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack); got != 0 {
		t.Fatalf("after regression local=%d, want 0", got)
	}
	if s.CyclicNoteEffectiveProgress(1390, CyclicNoteTaskTypeFlowerRack, 0) != 0 {
		t.Fatalf("effective progress should follow reset server counter")
	}
}

func TestCyclicNoteLocalProgressLowersOnCancel(t *testing.T) {
	s := New()
	s.BumpCyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack, 123, 12) // local 135
	s.LowerCyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack, 123, 12)
	if got := s.CyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack); got != 0 {
		t.Fatalf("after cancel lower local=%d, want 0", got)
	}
	s.BumpCyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack, 123, 12)
	s.LowerCyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack, 123, 5)
	if got := s.CyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack); got != 130 {
		t.Fatalf("partial cancel lower local=%d, want 130", got)
	}
}

func TestCyclicNoteFlowerRackRelistSlowResetsOnServerAdvance(t *testing.T) {
	s := New()
	s.BumpCyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack, 123, 1)
	s.MarkCyclicNoteFlowerRackRelistSlow()
	s.ReconcileCyclicNoteLocalProgressFromTasks(1390, []CyclicNoteTaskSlotView{{
		TaskType:         CyclicNoteTaskTypeFlowerRack,
		Progress:         124,
		ProgressObserved: true,
	}})
	if s.CyclicNoteFlowerRackRelistSlow() {
		t.Fatal("slow relist should reset when server progress advances")
	}
}

func TestCyclicNoteFlowerRackPostCompleteClearsOnServerRegression(t *testing.T) {
	s := New()
	now := time.UnixMilli(1_700_000)
	s.BeginCyclicNoteFlowerRackPostCompleteCancel(1390, now)
	if !s.CyclicNoteFlowerRackPostCompleteCancelReady(1390, now.Add(7*time.Minute+time.Second)) {
		t.Fatal("expected post-complete ready after 7 minutes")
	}
	s.BumpCyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack, 135, 1)
	s.ReconcileCyclicNoteLocalProgressFromTasks(1390, []CyclicNoteTaskSlotView{{
		TaskType:         CyclicNoteTaskTypeFlowerRack,
		Progress:         0,
		ProgressObserved: true,
	}})
	if s.CyclicNoteFlowerRackPostCompleteCancelReady(1390, now.Add(time.Hour)) {
		t.Fatal("post-complete timer must clear when 3015 progress regresses")
	}
}

func TestCyclicNoteLocalProgressKeepsHighWaterWhileServerLags(t *testing.T) {
	s := New()
	s.BumpCyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack, 80, 20) // local 100
	s.ReconcileCyclicNoteLocalProgressFromTasks(1390, []CyclicNoteTaskSlotView{{
		TaskType:         CyclicNoteTaskTypeFlowerRack,
		Progress:         80,
		ProgressObserved: true,
	}})
	if got := s.CyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack); got != 100 {
		t.Fatalf("lagging server must not clear local=%d", got)
	}
	s.ReconcileCyclicNoteLocalProgressFromTasks(1390, []CyclicNoteTaskSlotView{{
		TaskType:         CyclicNoteTaskTypeFlowerRack,
		Progress:         100,
		ProgressObserved: true,
	}})
	if got := s.CyclicNoteLocalProgress(1390, CyclicNoteTaskTypeFlowerRack); got != 0 {
		t.Fatalf("catch-up should clear local, got %d", got)
	}
}
