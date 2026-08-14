package policycfg

import (
	"math"
	"strings"
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestNormalizeClampsReconnectInterval(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{name: "zero uses default", in: 0, want: 300},
		{name: "negative uses default", in: -1, want: 300},
		{name: "nan uses default", in: math.NaN(), want: 300},
		{name: "infinity uses default", in: math.Inf(1), want: 300},
		{name: "subsecond clamps to one", in: 0.25, want: 1},
		{name: "configured value", in: 45, want: 45},
		{name: "huge clamps to one day", in: 1e30, want: 86400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(&pb.Policy{Basic: &pb.BasicPolicy{ReconnectIntervalSeconds: tt.in}}).
				GetBasic().GetReconnectIntervalSeconds()
			if got != tt.want {
				t.Fatalf("reconnect interval=%v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeDisplacedSessionReloginDefaultsOffAndPreservesChoice(t *testing.T) {
	if got := Normalize(&pb.Policy{}).GetBasic().GetDisplacedSessionReloginEnabled(); got {
		t.Fatal("displaced-session relogin default=true, want false")
	}
	if got := Normalize(&pb.Policy{Basic: &pb.BasicPolicy{
		DisplacedSessionReloginEnabled: true,
	}}).GetBasic().GetDisplacedSessionReloginEnabled(); !got {
		t.Fatal("explicit displaced-session relogin choice was not preserved")
	}
}

func TestFromJSONDisplacedSessionReloginIsBackwardCompatible(t *testing.T) {
	oldPolicy, err := FromJSON(`{"basic":{"reconnect_interval_seconds":45}}`)
	if err != nil {
		t.Fatal(err)
	}
	if oldPolicy.GetBasic().GetDisplacedSessionReloginEnabled() {
		t.Fatal("old policy without displaced-session switch enabled relogin")
	}

	enabledPolicy, err := FromJSON(`{"basic":{"displaced_session_relogin_enabled":true}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !enabledPolicy.GetBasic().GetDisplacedSessionReloginEnabled() {
		t.Fatal("explicit displaced-session switch did not survive policy JSON load")
	}
}

func TestNormalizeAutoReplantMinLevelClamp(t *testing.T) {
	if got := Normalize(&pb.Policy{Plant: &pb.PlantPolicy{
		Planting: &pb.PlantingPolicy{AutoReplantMinLevel: -3},
	}}).GetPlant().GetPlanting().GetAutoReplantMinLevel(); got != 0 {
		t.Fatalf("negative min level=%d, want 0", got)
	}
	if got := Normalize(&pb.Policy{Plant: &pb.PlantPolicy{
		Planting: &pb.PlantingPolicy{AutoReplantMinLevel: 11},
	}}).GetPlant().GetPlanting().GetAutoReplantMinLevel(); got != 11 {
		t.Fatalf("min level=%d, want 11", got)
	}
	got := Normalize(&pb.Policy{Plant: &pb.PlantPolicy{
		Planting: &pb.PlantingPolicy{AutoReplantMinLevel: 999},
	}}).GetPlant().GetPlanting().GetAutoReplantMinLevel()
	max := state.FlowerMaxLevel()
	if max > 0 {
		if got != max {
			t.Fatalf("oversize min level=%d, want clamped to FlowerMaxLevel=%d", got, max)
		}
	} else if got < 0 {
		t.Fatalf("min level=%d, want non-negative", got)
	}
}

func TestNormalizeHarvestDelaySecondsClamp(t *testing.T) {
	if got := Normalize(&pb.Policy{Plant: &pb.PlantPolicy{
		Planting: &pb.PlantingPolicy{HarvestDelaySeconds: -5},
	}}).GetPlant().GetPlanting().GetHarvestDelaySeconds(); got != 0 {
		t.Fatalf("negative harvest delay=%d, want 0", got)
	}
	if got := Normalize(&pb.Policy{Plant: &pb.PlantPolicy{
		Planting: &pb.PlantingPolicy{HarvestDelaySeconds: 45},
	}}).GetPlant().GetPlanting().GetHarvestDelaySeconds(); got != 45 {
		t.Fatalf("harvest delay=%d, want 45", got)
	}
}

func TestToJSONRoundTripPreservesHarvestDelaySeconds(t *testing.T) {
	in := automation.DefaultPolicy()
	in.Plant.Planting.HarvestDelaySeconds = 300
	raw, err := ToJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, `"harvest_delay_seconds"`) {
		t.Fatalf("ToJSON missing harvest delay: %s", raw)
	}
	out, err := FromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := out.GetPlant().GetPlanting().GetHarvestDelaySeconds(); got != 300 {
		t.Fatalf("FromJSON harvest delay=%d, want 300", got)
	}
}

func TestFlowerArtSellNightPauseRoundTrip(t *testing.T) {
	in := automation.DefaultPolicy()
	in.Order.FlowerArt.SellEnabled = true
	in.Order.FlowerArt.SellNightPauseEnabled = true

	raw, err := ToJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := FromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !out.GetOrder().GetFlowerArt().GetSellNightPauseEnabled() {
		t.Fatal("sell_night_pause_enabled did not survive policy JSON round trip")
	}

	legacy, err := FromJSON(`{"order":{"flower_art":{"sell_enabled":true}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.GetOrder().GetFlowerArt().GetSellNightPauseEnabled() {
		t.Fatal("old policy without night pause should default off")
	}
}

func TestFromJSONRaceScoreFieldCompatibility(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want int32
	}{
		{name: "stable snake case", raw: `{"union":{"race":{"min_task_score":17}}}`, want: 17},
		{name: "short lived KK snake case", raw: `{"union":{"race":{"max_task_score":19}}}`, want: 19},
		{name: "short lived KK camel case", raw: `{"union":{"race":{"maxTaskScore":21}}}`, want: 21},
		{name: "stable field wins", raw: `{"union":{"race":{"min_task_score":17,"max_task_score":19}}}`, want: 17},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy, err := FromJSON(tt.raw)
			if err != nil {
				t.Fatal(err)
			}
			if got := policy.GetUnion().GetRace().GetMinTaskScore(); got != tt.want {
				t.Fatalf("min task score=%d, want %d", got, tt.want)
			}
		})
	}
}

func TestFromJSONRaceAutoStopOnQuotaDoneBackfill(t *testing.T) {
	legacy, err := FromJSON(`{"union":{"race":{"enabled":true,"min_task_score":28}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !legacy.GetUnion().GetRace().GetAutoStopOnQuotaDone() {
		t.Fatal("legacy race policy missing auto_stop_on_quota_done should default on")
	}

	explicitOff, err := FromJSON(`{"union":{"race":{"auto_stop_on_quota_done":false}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if explicitOff.GetUnion().GetRace().GetAutoStopOnQuotaDone() {
		t.Fatal("explicit auto_stop_on_quota_done=false must be preserved")
	}

	explicitOn, err := FromJSON(`{"union":{"race":{"autoStopOnQuotaDone":true}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !explicitOn.GetUnion().GetRace().GetAutoStopOnQuotaDone() {
		t.Fatal("explicit autoStopOnQuotaDone=true must survive load")
	}
}

func TestRaceUrgentSpeedupRequiresExplicitPolicy(t *testing.T) {
	legacy, err := FromJSON(`{"union":{"race":{"enabled":true}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if legacy.GetUnion().GetRace().GetUrgentSpeedupEnabled() {
		t.Fatal("legacy policies must not opt into emergency ticket spending")
	}

	enabled, err := FromJSON(`{"union":{"race":{"urgent_speedup_enabled":true}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled.GetUnion().GetRace().GetUrgentSpeedupEnabled() {
		t.Fatal("explicit urgent_speedup_enabled=true must survive load")
	}
}

func TestNormalizeFillsNewPlantDefaults(t *testing.T) {
	p := Normalize(&pb.Policy{})
	planting := p.GetPlant().GetPlanting()
	if planting.GetDemandPriority()[automation.GoalCustomerOrder] == 0 {
		t.Fatalf("demand priorities not populated: %+v", planting.GetDemandPriority())
	}
	if planting.GetDemandPriorityEnabled() {
		t.Fatal("demand_priority_enabled should default to false")
	}
	if planting.GetAutoReplantMode() != pb.SelectionMode_SELECTION_MODE_ALL {
		t.Fatalf("auto replant mode=%v, want ALL", planting.GetAutoReplantMode())
	}
}

func TestNormalizePreservesExplicitAutoHarvestDisabled(t *testing.T) {
	p := Normalize(&pb.Policy{
		Plant: &pb.PlantPolicy{
			Planting: &pb.PlantingPolicy{
				AutoEnabled:        true,
				AutoHarvestEnabled: false,
			},
		},
	})
	if p.GetPlant().GetPlanting().GetAutoHarvestEnabled() {
		t.Fatal("explicit auto_harvest_enabled=false should be preserved")
	}
}

func TestPolicyJSONRoundTripUsesFullParityTree(t *testing.T) {
	raw := `{
	  "automation_enabled": true,
		"plant": {
	    "planting": {
	      "demand_priority": {"order.customer": 99}
	    }
	  }
	}`
	p, err := FromJSON(raw)
	if err != nil {
		t.Fatalf("FromJSON returned error: %v", err)
	}
	planting := p.GetPlant().GetPlanting()
	if !p.GetAutomationEnabled() {
		t.Fatalf("policy values not kept: %+v", p)
	}
	if planting.GetDemandPriority()[automation.GoalCustomerOrder] != 99 {
		t.Fatalf("custom priority not kept: %+v", planting.GetDemandPriority())
	}
}

func TestFromJSONBackfillsAutoHarvestForOldPlantingPolicy(t *testing.T) {
	raw := `{
	  "plant": {
	    "planting": {
	      "auto_enabled": true
	    }
	  }
	}`
	p, err := FromJSON(raw)
	if err != nil {
		t.Fatalf("FromJSON returned error: %v", err)
	}
	if !p.GetPlant().GetPlanting().GetAutoHarvestEnabled() {
		t.Fatal("old auto_enabled=true policies should keep harvesting enabled")
	}
}

func TestFromJSONPreservesExplicitAutoHarvestDisabled(t *testing.T) {
	raw := `{
	  "plant": {
	    "planting": {
	      "auto_enabled": true,
	      "auto_harvest_enabled": false
	    }
	  }
	}`
	p, err := FromJSON(raw)
	if err != nil {
		t.Fatalf("FromJSON returned error: %v", err)
	}
	if p.GetPlant().GetPlanting().GetAutoHarvestEnabled() {
		t.Fatal("explicit auto_harvest_enabled=false should survive JSON load")
	}
}

func TestFromJSONIgnoresRemovedPlantFlowerFieldAndOldPriorityName(t *testing.T) {
	raw := `{
	  "plant": {
	    "planting": {
	      "auto_enabled": true,
	      "goal_priority": {"order.customer": 99}
	    },
	    "flower": {
	      "auto_enabled": false,
	      "goal_priority": {"order.customer": 99}
	    }
	  }
	}`
	p, err := FromJSON(raw)
	if err != nil {
		t.Fatalf("FromJSON returned error: %v", err)
	}
	if !p.GetPlant().GetPlanting().GetAutoEnabled() {
		t.Fatalf("removed plant.flower field should not override planting defaults")
	}
	if got := p.GetPlant().GetPlanting().GetDemandPriority()[automation.GoalCustomerOrder]; got == 99 {
		t.Fatalf("old goal priority should be ignored")
	}
}
