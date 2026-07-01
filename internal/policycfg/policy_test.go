package policycfg

import (
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"google.golang.org/protobuf/proto"
)

func TestFlattenApplyRoundTripIncludesMisc(t *testing.T) {
	p := Normalize(&pb.Policy{
		AutomationEnabled:       true,
		DecisionIntervalSeconds: 9,
		Harvest:                 &pb.HarvestPolicy{Enabled: true, PreferOneKey: false},
		Plant: &pb.PlantPolicy{
			Enabled:             true,
			Mode:                "selected",
			TaskPriorityEnabled: proto.Bool(false),
			MinStock:            3,
			MaxBatch:            4,
			AllowedFlowerIds:    []int32{23001, 23002},
			BlockedFlowerIds:    []int32{23009},
		},
		Water: &pb.WaterPolicy{Enabled: false, MaxBatch: 2, MinDrops: 7, PreferOneKeyIfNoble: true},
		Misc: &pb.MiscPolicy{
			LandUnlockEnabled:          true,
			StoryUnlockEnabled:         true,
			WaterwheelEnabled:          true,
			CultivateEnabled:           false,
			FlowerUpgradeEnabled:       true,
			ResidentOrderEnabled:       true,
			CustomerOrderEnabled:       false,
			CustomerOrderCraftEnabled:  true,
			CustomerOrderRejectEnabled: false,
			ResidentOrderRewardEnabled: true,
			ResidentOrderAdEnabled:     false,
			FlowerRackEnabled:          true,
			FlowerRackCraftEnabled:     false,
			FreeWaterEnabled:           false,
			TaskMainRewardEnabled:      true,
			TaskDailyRewardEnabled:     false,
			RoadGrowRewardEnabled:      true,
			RandomEventEnabled:         false,
			FlowerArtRewardEnabled:     true,
			OrderPalaceEnabled:         true,
			SignEnabled:                false,
			FlowerPassEnabled:          true,
			FlowerElvesPassEnabled:     false,
			PlayerBackEnabled:          true,
			ActivityRewardEnabled:      false,
			ZooSyncEnabled:             true,
			ZooFeedEnabled:             false,
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
	if got.GetPlant().GetTaskPriorityEnabled() {
		t.Fatalf("task priority did not round-trip as disabled: %+v", got.GetPlant())
	}
	if len(got.GetPlant().GetAllowedFlowerIds()) != 2 || got.GetPlant().GetAllowedFlowerIds()[1] != 23002 {
		t.Fatalf("allowed flower ids did not round-trip: %+v", got.GetPlant().GetAllowedFlowerIds())
	}
	if got.GetWater().GetMinDrops() != 7 || !got.GetWater().GetPreferOneKeyIfNoble() {
		t.Fatalf("water threshold did not round-trip: %+v", got.GetWater())
	}
	if !got.GetMisc().GetLandUnlockEnabled() || !got.GetMisc().GetStoryUnlockEnabled() || !got.GetMisc().GetWaterwheelEnabled() {
		t.Fatalf("enabled misc fields did not round-trip: %+v", got.GetMisc())
	}
	if got.GetMisc().GetCultivateEnabled() {
		t.Fatalf("disabled misc fields did not round-trip: %+v", got.GetMisc())
	}
	if !got.GetMisc().GetFlowerUpgradeEnabled() {
		t.Fatalf("flower upgrade field did not round-trip: %+v", got.GetMisc())
	}
	if !got.GetMisc().GetResidentOrderEnabled() || got.GetMisc().GetCustomerOrderEnabled() ||
		!got.GetMisc().GetCustomerOrderCraftEnabled() || got.GetMisc().GetCustomerOrderRejectEnabled() ||
		!got.GetMisc().GetResidentOrderRewardEnabled() || got.GetMisc().GetResidentOrderAdEnabled() ||
		!got.GetMisc().GetFlowerRackEnabled() || got.GetMisc().GetFlowerRackCraftEnabled() ||
		got.GetMisc().GetFreeWaterEnabled() || !got.GetMisc().GetTaskMainRewardEnabled() ||
		got.GetMisc().GetTaskDailyRewardEnabled() || !got.GetMisc().GetRoadGrowRewardEnabled() ||
		got.GetMisc().GetRandomEventEnabled() || !got.GetMisc().GetFlowerArtRewardEnabled() ||
		!got.GetMisc().GetOrderPalaceEnabled() || got.GetMisc().GetSignEnabled() ||
		!got.GetMisc().GetFlowerPassEnabled() || got.GetMisc().GetFlowerElvesPassEnabled() ||
		!got.GetMisc().GetPlayerBackEnabled() || got.GetMisc().GetActivityRewardEnabled() ||
		!got.GetMisc().GetZooSyncEnabled() || got.GetMisc().GetZooFeedEnabled() {
		t.Fatalf("fine-grained misc fields did not round-trip: %+v", got.GetMisc())
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
	if p.GetMisc().GetResidentOrderEnabled() || p.GetMisc().GetFreeWaterEnabled() || p.GetMisc().GetRandomEventEnabled() ||
		p.GetMisc().GetOrderPalaceEnabled() || p.GetMisc().GetSignEnabled() || p.GetMisc().GetFlowerPassEnabled() ||
		p.GetMisc().GetFlowerElvesPassEnabled() || p.GetMisc().GetPlayerBackEnabled() ||
		p.GetMisc().GetActivityRewardEnabled() || p.GetMisc().GetZooSyncEnabled() || p.GetMisc().GetZooFeedEnabled() {
		t.Fatalf("fine-grained misc defaults should be opt-in: %+v", p.GetMisc())
	}
	if p.GetPlant().GetMode() != "high_value" || !p.GetPlant().GetTaskPriorityEnabled() || p.GetWater().GetMinDrops() != 5 {
		t.Fatalf("strategy defaults were not filled: plant=%+v water=%+v", p.GetPlant(), p.GetWater())
	}
}

func TestSetKeyRejectsLegacyAutoPlantMode(t *testing.T) {
	p := &pb.Policy{}
	if err := SetKey(p, KeyPlantMode, "auto"); err == nil {
		t.Fatal("expected legacy auto plant mode to be rejected")
	}
}
