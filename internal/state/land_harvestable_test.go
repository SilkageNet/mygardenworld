package state

import "testing"

func TestStampLandHarvestableUsesApplyTimeWhenPlantTimeUnchanged(t *testing.T) {
	prev := LandView{FlowerID: 23331, State: 2, HarvestCnt: 1, PlantTimeMs: 1000, Observed: true}
	next := LandView{FlowerID: 23331, State: 3, HarvestCnt: 1, PlantTimeMs: 1000, ElvesID: 9, Observed: true}
	got := stampLandHarvestable(prev, next, 5000)
	if got.HarvestableSinceMs != 5000 {
		t.Fatalf("harvestableSince=%d want apply time 5000 (plantTime unchanged on speed-up)", got.HarvestableSinceMs)
	}
}

func TestStampLandHarvestablePrefersAdvancedPlantTime(t *testing.T) {
	prev := LandView{FlowerID: 23331, State: 2, PlantTimeMs: 1000, Observed: true}
	next := LandView{FlowerID: 23331, State: 3, PlantTimeMs: 4000, Observed: true}
	got := stampLandHarvestable(prev, next, 5000)
	if got.HarvestableSinceMs != 4000 {
		t.Fatalf("harvestableSince=%d want changed plantTime 4000", got.HarvestableSinceMs)
	}
}

func TestStampLandHarvestableAcceptsOlderPlantTimeCorrection(t *testing.T) {
	prev := LandView{FlowerID: 23001, State: 3, PlantTimeMs: 5000, HarvestableSinceMs: 5000, Observed: true}
	next := LandView{FlowerID: 23001, State: 3, PlantTimeMs: 2000, Observed: true}
	got := stampLandHarvestable(prev, next, 9000)
	if got.HarvestableSinceMs != 2000 {
		t.Fatalf("harvestableSince=%d want corrected plantTime 2000", got.HarvestableSinceMs)
	}
}

func TestStampLandHarvestablePreservesWhileStillReady(t *testing.T) {
	prev := LandView{FlowerID: 23331, State: 3, HarvestCnt: 1, PlantTimeMs: 1000, HarvestableSinceMs: 5000, Observed: true}
	next := LandView{FlowerID: 23331, State: 3, HarvestCnt: 1, PlantTimeMs: 1000, ElvesID: 9, Observed: true}
	got := stampLandHarvestable(prev, next, 9000)
	if got.HarvestableSinceMs != 5000 {
		t.Fatalf("harvestableSince=%d want preserved 5000", got.HarvestableSinceMs)
	}
}
