package state

import (
	"testing"
	"time"
)

func TestLandRemainingYieldSubtractsSteals(t *testing.T) {
	// 23001 lvl 1: cropGets=2, frequencys=1 → base remaining 2.
	land := LandView{FlowerID: 23001, State: 3, Lvl: 1, HarvestCnt: 0, StealUIDs: []int64{101}}
	if got := LandRemainingYield(land); got != 1 {
		t.Fatalf("remaining=%d, want 1 after one steal", got)
	}
}

func TestLandRemainingYieldMultiRoundExample(t *testing.T) {
	// 23001 lvl 10: cropGets=3, frequencys=3.
	// harvestCnt=0, one steal → 3*3-1=8 remaining; can-touch=2.
	land := LandView{FlowerID: 23001, State: 3, Lvl: 10, HarvestCnt: 0, StealUIDs: []int64{1}}
	if got := LandRemainingYield(land); got != 8 {
		t.Fatalf("remaining=%d, want 8", got)
	}
	now := time.UnixMilli(1_000)
	if got := LandCanTouch(land, now); got != 2 {
		t.Fatalf("can_touch=%d, want 2", got)
	}
}

func TestLandCanTouchZeroWhileGrowing(t *testing.T) {
	land := LandView{FlowerID: 23001, State: 1, Lvl: 1, HarvestCnt: 0}
	now := time.UnixMilli(1_000)
	if got := LandCanTouch(land, now); got != 0 {
		t.Fatalf("can_touch=%d, want 0 while growing", got)
	}
	if got := LandRemainingYield(land); got != 2 {
		t.Fatalf("remaining=%d, want 2 while growing", got)
	}
}

func TestLandCanTouchState2WhenDue(t *testing.T) {
	land := LandView{
		FlowerID: 23001, State: 2, Lvl: 1, HarvestCnt: 0,
		NextTimeMs: 500, StealUIDs: []int64{9},
	}
	now := time.UnixMilli(1_000)
	if got := LandCanTouch(land, now); got != 1 {
		t.Fatalf("can_touch=%d, want 1 when state=2 due", got)
	}
	land.NextTimeMs = 2_000
	if got := LandCanTouch(land, now); got != 0 {
		t.Fatalf("can_touch=%d, want 0 when still regrowing", got)
	}
}

func TestLandRemainingYieldEmpty(t *testing.T) {
	if got := LandRemainingYield(LandView{Observed: true}); got != 0 {
		t.Fatalf("empty land remaining=%d, want 0", got)
	}
}
