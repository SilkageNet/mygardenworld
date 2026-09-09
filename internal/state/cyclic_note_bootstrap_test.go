package state

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCyclicNoteBootstrapDoesNotRequireLazyState(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*State)
		want   bool
	}{
		{"fresh batch", func(s *State) {
			s.activityBatches[9001].ScoreObserved = false
			s.activityBatches[9001].BagObserved = false
		}, true},
		{"missing template", func(s *State) { delete(s.activityTemplates, 40020007) }, true},
		{"invalid tasks", func(s *State) {
			s.activityBatches[9001].TaskListObserved = true
			s.activityBatches[9001].TaskListValid = false
		}, true},
		{"wrong identity", func(s *State) { s.activityBatches[9001].IdentityValid = false }, false},
		{"inactive", func(s *State) { s.activityBatches[9001].Status = 0 }, false},
		{"unknown batch", func(s *State) { delete(s.activityBatches, 9001) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := applyCyclicNoteCaptureFixture(t)
			s.activityBatches[9001].TaskListObserved = false
			tc.change(s)
			now := time.UnixMilli(cyclicNoteFixtureNowMs)
			snapshot, ok := s.CyclicNoteEnterSnapshot(now)
			if ok != tc.want {
				t.Fatalf("snapshot=%+v ready=%t", snapshot, ok)
			}
			if tc.want && s.CyclicNoteEnterApplied(snapshot) {
				t.Fatal("incomplete bootstrap accepted")
			}
			if tc.name == "fresh batch" {
				if _, ready := s.CyclicNoteTaskClaimSnapshot(now, 9001, 1, 4003); ready {
					t.Fatal("claim before bootstrap")
				}
				s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"11":81,"12":{"1107":5},"14":{"105":{"0":[4003,2001,1006]}}}}}}`))
				if !s.CyclicNoteEnterApplied(snapshot) {
					t.Fatal("complete result rejected")
				}
			}
		})
	}
}

func TestCyclicStoryBootstrapDoesNotRequireRewardTemplate(t *testing.T) {
	s := applyCyclicStoryCaptureFixture(t)
	delete(s.activityTemplates, 40030001)
	now := time.UnixMilli(cyclicStoryFixtureNowMs)
	snapshot, ok := s.CyclicStoryEnterSnapshot(now)
	if !ok || snapshot.BatchID != 9101 {
		t.Fatalf("bootstrap=%+v ready=%t", snapshot, ok)
	}
	if s.CyclicStoryEnterApplied(snapshot) {
		t.Fatal("missing template counted as complete")
	}
	if _, ready := s.CyclicStoryMilestoneClaimSnapshot(now, 9101, 2); ready {
		t.Fatal("claim with missing template")
	}
}
