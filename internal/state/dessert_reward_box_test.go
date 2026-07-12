package state

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestDessertRewardBoxOpenUsesOnlyAuthoritativeActivityBag(t *testing.T) {
	now := time.UnixMilli(dessertFixtureNowMs)
	s := applyDessertCaptureFixture(t)

	// A malformed game extension must not couple the independently observed
	// activity bag to game-board readiness.
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"14":{"121":"malformed"}}}}}`))
	if view, _ := s.DessertView(now); view.Valid {
		t.Fatal("malformed extension unexpectedly left the full dessert view valid")
	}
	before, ok := s.DessertRewardBoxOpenSnapshot(now, 9101, 1)
	if !ok || before.RewardBoxID != 1347 || before.BalanceBefore != 13 || before.Count != 1 {
		t.Fatalf("open snapshot=(%+v,%t)", before, ok)
	}
	if _, ok := s.DessertRewardBoxOpenSnapshot(now, 9101, 2); ok {
		t.Fatal("multi-box open was authorized")
	}

	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"12":{"1342":100,"1343":217,"1347":12}}}}}`))
	if !s.DessertRewardBoxOpenApplied(before) {
		t.Fatal("exact single-box decrement was rejected")
	}

	s = applyDessertCaptureFixture(t)
	before, _ = s.DessertRewardBoxOpenSnapshot(now, 9101, 1)
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"12":{"1342":100,"1343":217,"1347":11}}}}}`))
	if s.DessertRewardBoxOpenApplied(before) {
		t.Fatal("a decrement larger than one passed the postcondition")
	}
}

func TestDessertRewardBoxEnterOnlyRepairsMissingBag(t *testing.T) {
	raw, err := os.ReadFile("testdata/dessert_activity.json")
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	batches := root["23"].(map[string]any)["0"].(map[string]any)
	delete(batches["9101"].(map[string]any), "12")
	raw, err = json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}

	now := time.UnixMilli(dessertFixtureNowMs)
	s := New()
	s.ApplyV(raw)
	before, ok := s.DessertRewardBoxEnterSnapshot(now)
	if !ok || !before.BagOnly || before.BatchID != 9101 {
		t.Fatalf("bag-only enter=(%+v,%t)", before, ok)
	}
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"12":{"1342":100,"1343":217,"1347":13}}}}}`))
	if !s.DessertEnterApplied(before) {
		t.Fatal("bag-only enter postcondition rejected an observed valid bag")
	}
	if _, ready := s.DessertRewardBoxEnterSnapshot(now); ready {
		t.Fatal("observed bag still requested enter")
	}
}
