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

func TestRedeemConnectModeDefaultsToAutoAndPreservesOnlineOnly(t *testing.T) {
	defaultPolicy, err := FromJSON(`{"schema_version":3,"basic":{}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := defaultPolicy.GetBasic().GetRedeemConnectMode(); got != pb.RedeemConnectMode_REDEEM_CONNECT_MODE_AUTO {
		t.Fatalf("missing redeem connect mode=%v, want AUTO", got)
	}

	onlineOnly, err := FromJSON(`{"schema_version":3,"basic":{"redeem_connect_mode":"REDEEM_CONNECT_MODE_ONLINE_ONLY"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := onlineOnly.GetBasic().GetRedeemConnectMode(); got != pb.RedeemConnectMode_REDEEM_CONNECT_MODE_ONLINE_ONLY {
		t.Fatalf("explicit redeem connect mode=%v, want ONLINE_ONLY", got)
	}
}

func TestNormalizeClearsUnsupportedSDKAdAutomation(t *testing.T) {
	p := automation.DefaultPolicy()
	p.Plant.Planting.VideoSpeedUpEnabled = true
	p.Plant.Cultivate.VideoSpeedUpEnabled = true
	p.Basic.Benefit.DoubleCoinEnabled = true
	p.Basic.Shop.VideoFreeGiftEnabled = true
	p.Union.Build.FreeEnabled = true

	features := UnsupportedSDKAdFeatures(p)
	if len(features) != 5 {
		t.Fatalf("UnsupportedSDKAdFeatures() = %v, want five features", features)
	}
	normalized := Normalize(p)
	if got := UnsupportedSDKAdFeatures(normalized); len(got) != 0 {
		t.Fatalf("Normalize() retained unsupported SDK ad features: %v", got)
	}
	// Normalize clones its input, so API validation can still inspect the raw
	// request and reject it explicitly.
	if got := UnsupportedSDKAdFeatures(p); len(got) != 5 {
		t.Fatalf("Normalize() mutated input features: %v", got)
	}
}

func TestFromJSONClearsUnsupportedSDKAdAutomation(t *testing.T) {
	p, err := FromJSON(`{
		"schema_version":3,
		"plant":{"planting":{"video_speed_up_enabled":true},"cultivate":{"video_speed_up_enabled":true}},
		"basic":{"benefit":{"double_coin_enabled":true},"shop":{"video_free_gift_enabled":true}},
		"union":{"build":{"free_enabled":true}}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := UnsupportedSDKAdFeatures(p); len(got) != 0 {
		t.Fatalf("unsupported SDK ad switches survived load: %v", got)
	}
}

func TestFromJSONDisplacedSessionReloginIsExplicit(t *testing.T) {
	disabledPolicy, err := FromJSON(`{"schema_version":3,"basic":{"reconnect_interval_seconds":45}}`)
	if err != nil {
		t.Fatal(err)
	}
	if disabledPolicy.GetBasic().GetDisplacedSessionReloginEnabled() {
		t.Fatal("policy without displaced-session switch enabled relogin")
	}

	enabledPolicy, err := FromJSON(`{"schema_version":3,"basic":{"displaced_session_relogin_enabled":true}}`)
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

}

func TestFromJSONRequiresExactSchema(t *testing.T) {
	tests := []string{
		`{"union":{"race":{"min_task_score":17}}}`,
		`{"schema_version":4}`,
		`{"schema_version":2,"union":{"race":{"urgent_speedup_enabled":true}}}`,
		`{"schema_version":2,"union":{"race":{"max_task_score":19}}}`,
	}
	for _, raw := range tests {
		if _, err := FromJSON(raw); err == nil {
			t.Fatalf("FromJSON(%s) unexpectedly accepted an obsolete document", raw)
		}
	}
}

func TestFromJSONPreservesCurrentRacePolicy(t *testing.T) {
	explicitOff, err := FromJSON(`{"schema_version":3,"union":{"race":{"auto_stop_on_quota_done":false,"min_task_score":17}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if explicitOff.GetUnion().GetRace().GetAutoStopOnQuotaDone() {
		t.Fatal("explicit auto_stop_on_quota_done=false must be preserved")
	}
	if explicitOff.GetUnion().GetRace().GetMinTaskScore() != 17 {
		t.Fatal("current min_task_score did not survive load")
	}

	explicitOn, err := FromJSON(`{"schema_version":3,"union":{"race":{"auto_stop_on_quota_done":true}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !explicitOn.GetUnion().GetRace().GetAutoStopOnQuotaDone() {
		t.Fatal("explicit auto_stop_on_quota_done=true must survive load")
	}
}

func TestProgressedRaceTaskPolicyDefaultsOnAndPreservesExplicitOff(t *testing.T) {
	missing, err := FromJSON(`{"schema_version":3,"union":{"race":{}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if !missing.GetUnion().GetRace().GetAvoidProgressedTasks() {
		t.Fatal("missing avoid_progressed_tasks must use the safe default")
	}

	explicitOff, err := FromJSON(`{"schema_version":3,"union":{"race":{"avoid_progressed_tasks":false}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if explicitOff.GetUnion().GetRace().GetAvoidProgressedTasks() {
		t.Fatal("explicit avoid_progressed_tasks=false must be preserved")
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
	  "schema_version": 3,
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

func TestFromJSONPreservesExplicitAutoHarvestDisabled(t *testing.T) {
	raw := `{
	  "schema_version": 3,
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

func TestFromJSONRejectsRemovedPlantFlowerFieldAndOldPriorityName(t *testing.T) {
	raw := `{
	  "schema_version": 3,
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
	if _, err := FromJSON(raw); err == nil {
		t.Fatal("removed fields unexpectedly survived strict policy decoding")
	}
}
