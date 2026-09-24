package state

import "testing"

func TestElvesSpawnTrackingFromLandDeltas(t *testing.T) {
	s := New()
	s.ApplyV([]byte(`{"100":{"1":{"1001":{"0":23331,"1":3,"6":110132}}}}`))
	if got := s.ElvesProducedCount(); got != 1 {
		t.Fatalf("spawn count=%d, want 1", got)
	}
	s.ApplyV([]byte(`{"100":{"1":{"1001":{"0":23331,"1":3,"6":110132}}}}`))
	if got := s.ElvesProducedCount(); got != 1 {
		t.Fatalf("duplicate delta count=%d", got)
	}
	s.ApplyV([]byte(`{"100":{"1":{"1002":{"0":23332,"1":3,"6":110133}}}}`))
	if got := s.ElvesProducedCount(); got != 2 {
		t.Fatalf("second spawn count=%d", got)
	}
	s.ClearElvesRound()
	if got := s.ElvesProducedCount(); got != 0 {
		t.Fatalf("cleared count=%d", got)
	}
}
