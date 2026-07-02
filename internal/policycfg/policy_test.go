package policycfg

import (
	"strings"
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
)

func TestJSONRoundTripIncludesDomains(t *testing.T) {
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.DailyTaskEnabled = true
	p.Basic.FreeWaterEnabled = true
	p.Plant.PlantingMode = automation.PlantModeSelected
	p.Plant.AllowedFlowerIds = []int32{23001, 23002}
	p.Plant.BlockedFlowerIds = []int32{23009}
	p.Plant.PlantMaxBatch = 4
	p.Order.Customer = &pb.CustomerOrderPolicy{Enabled: true, CraftEnabled: true, RejectEnabled: true}
	p.Order.Resident = &pb.ResidentOrderPolicy{NormalEnabled: true, NormalMaxNum: 5, RewardEnabled: true}
	p.Union.RaceEnabled = true
	p.Activity.Enabled = true
	p.Activity.Modules["actElim"].Enabled = true
	p.Safety.MaxGoldSpendPerTick = 1000

	raw, err := ToJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := FromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !got.GetAutomationEnabled() || !got.GetBasic().GetDailyTaskEnabled() || !got.GetBasic().GetFreeWaterEnabled() {
		t.Fatalf("basic policy did not round-trip: %+v", got.GetBasic())
	}
	if got.GetPlant().GetPlantingMode() != automation.PlantModeSelected ||
		got.GetPlant().GetPlantMaxBatch() != 4 ||
		len(got.GetPlant().GetAllowedFlowerIds()) != 2 ||
		got.GetPlant().GetBlockedFlowerIds()[0] != 23009 {
		t.Fatalf("plant policy did not round-trip: %+v", got.GetPlant())
	}
	if !got.GetOrder().GetCustomer().GetEnabled() || !got.GetOrder().GetCustomer().GetCraftEnabled() ||
		!got.GetOrder().GetResident().GetNormalEnabled() || !got.GetOrder().GetResident().GetRewardEnabled() {
		t.Fatalf("order policy did not round-trip: %+v", got.GetOrder())
	}
	if !got.GetUnion().GetRaceEnabled() {
		t.Fatalf("union policy did not round-trip: %+v", got.GetUnion())
	}
	if !got.GetActivity().GetEnabled() || !got.GetActivity().GetModules()["actElim"].GetEnabled() {
		t.Fatalf("activity policy did not round-trip: %+v", got.GetActivity())
	}
	if got.GetSafety().GetMaxGoldSpendPerTick() != 1000 {
		t.Fatalf("safety policy did not round-trip: %+v", got.GetSafety())
	}
}

func TestNormalizeDefaults(t *testing.T) {
	p := Normalize(&pb.Policy{})
	if p.GetBasic() == nil || p.GetPlant() == nil || p.GetOrder() == nil ||
		p.GetUnion() == nil || p.GetActivity() == nil || p.GetSafety() == nil {
		t.Fatalf("Normalize should fill every domain: %+v", p)
	}
	if p.GetAutomationEnabled() {
		t.Fatal("automation should default off")
	}
	if !p.GetPlant().GetHarvestEnabled() || !p.GetPlant().GetPlantEnabled() || !p.GetPlant().GetWaterEnabled() {
		t.Fatalf("core farm loop should default on: %+v", p.GetPlant())
	}
	if !p.GetPlant().GetTaskPriorityEnabled() || p.GetPlant().GetPlantingMode() != automation.PlantModeLowStock {
		t.Fatalf("planting should default to task priority with low-stock fallback: %+v", p.GetPlant())
	}
	if p.GetOrder().GetCustomer().GetEnabled() || p.GetUnion().GetRaceEnabled() || p.GetActivity().GetEnabled() {
		t.Fatalf("non-core domains should default opt-in: order=%+v union=%+v activity=%+v", p.GetOrder(), p.GetUnion(), p.GetActivity())
	}
	if p.GetSafety().GetMaxConsecutiveErrors() != 3 || p.GetSafety().GetDomainBackoffSeconds() == 0 {
		t.Fatalf("safety defaults missing: %+v", p.GetSafety())
	}
}

func TestFromJSONRejectsUnknownFields(t *testing.T) {
	_, err := FromJSON(`{"automation_enabled":true,"legacy_policy":{"old":true}}`)
	if err == nil {
		t.Fatal("expected unknown legacy field to be rejected")
	}
	if !strings.Contains(err.Error(), "legacy_policy") {
		t.Fatalf("error should name the legacy field, got %v", err)
	}
}
