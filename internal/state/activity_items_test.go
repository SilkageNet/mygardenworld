package state

import "testing"

func TestActivityItemCountUsesExactObservedBatchBag(t *testing.T) {
	s := applyDessertCaptureFixture(t)
	if count, ok := s.ActivityItemCount(9101, 1347); !ok || count != 13 {
		t.Fatalf("reward box count=(%d,%t), want (13,true)", count, ok)
	}
	if count, ok := s.ActivityItemCount(9101, 9999); !ok || count != 0 {
		t.Fatalf("missing activity item=(%d,%t), want observed zero", count, ok)
	}
	if _, ok := s.ActivityItemCount(9102, 1347); ok {
		t.Fatal("wrong batch reused another activity bag")
	}
	if _, ok := s.ActivityItemCount(0, 1347); ok {
		t.Fatal("zero batch was accepted")
	}

	s.mu.Lock()
	s.activityBatches[9101].BagValid = false
	s.mu.Unlock()
	if _, ok := s.ActivityItemCount(9101, 1347); ok {
		t.Fatal("malformed activity bag was accepted")
	}
}
