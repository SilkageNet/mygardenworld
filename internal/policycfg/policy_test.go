package policycfg

import (
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
)

func TestFlattenApplyRoundTripIncludesMisc(t *testing.T) {
	p := Normalize(&pb.Policy{
		AutomationEnabled:       true,
		DecisionIntervalSeconds: 9,
		Harvest:                 &pb.HarvestPolicy{Enabled: true, PreferOneKey: false},
		Plant: &pb.PlantPolicy{
			Enabled:          true,
			Mode:             "selected",
			MinStock:         3,
			MaxBatch:         4,
			AllowedFlowerIds: []int32{23001, 23002},
			BlockedFlowerIds: []int32{23009},
		},
		Water: &pb.WaterPolicy{Enabled: false, MaxBatch: 2, MinDrops: 7},
		Misc: &pb.MiscPolicy{
			LandUnlockEnabled:    true,
			TaskRecvEnabled:      false,
			StoryUnlockEnabled:   true,
			OrderEnabled:         false,
			WaterwheelEnabled:    true,
			CultivateEnabled:     false,
			FlowerUpgradeEnabled: true,
		},
	})

	got := FromEntries(Flatten(p))
	if !got.AutomationEnabled || got.DecisionIntervalSeconds != 9 {
		t.Fatalf("top-level fields did not round-trip: %+v", got)
	}
	if got.GetPlant().GetMinStock() != 3 || got.GetPlant().GetMaxBatch() != 4 {
		t.Fatalf("plant fields did not round-trip: %+v", got.GetPlant())
	}
	if got.GetPlant().GetMode() != "selected" {
		t.Fatalf("plant mode did not round-trip: %+v", got.GetPlant())
	}
	if len(got.GetPlant().GetAllowedFlowerIds()) != 2 || got.GetPlant().GetAllowedFlowerIds()[1] != 23002 {
		t.Fatalf("allowed flower ids did not round-trip: %+v", got.GetPlant().GetAllowedFlowerIds())
	}
	if got.GetWater().GetMinDrops() != 7 {
		t.Fatalf("water threshold did not round-trip: %+v", got.GetWater())
	}
	if !got.GetMisc().GetLandUnlockEnabled() || !got.GetMisc().GetStoryUnlockEnabled() || !got.GetMisc().GetWaterwheelEnabled() {
		t.Fatalf("enabled misc fields did not round-trip: %+v", got.GetMisc())
	}
	if got.GetMisc().GetTaskRecvEnabled() || got.GetMisc().GetOrderEnabled() || got.GetMisc().GetCultivateEnabled() {
		t.Fatalf("disabled misc fields did not round-trip: %+v", got.GetMisc())
	}
	if !got.GetMisc().GetFlowerUpgradeEnabled() {
		t.Fatalf("flower upgrade field did not round-trip: %+v", got.GetMisc())
	}
}

func TestNormalizeFillsMissingSubPoliciesWithSafeDefaults(t *testing.T) {
	p := Normalize(&pb.Policy{AutomationEnabled: true})
	if p.GetHarvest() == nil || p.GetPlant() == nil || p.GetWater() == nil || p.GetMisc() == nil {
		t.Fatalf("missing sub-policy after normalize: %+v", p)
	}
	if p.GetMisc().GetLandUnlockEnabled() || p.GetMisc().GetCultivateEnabled() || p.GetMisc().GetFlowerUpgradeEnabled() {
		t.Fatalf("misc defaults should be opt-in: %+v", p.GetMisc())
	}
	if p.GetPlant().GetMode() != "auto" || p.GetWater().GetMinDrops() != 5 {
		t.Fatalf("strategy defaults were not filled: plant=%+v water=%+v", p.GetPlant(), p.GetWater())
	}
}
