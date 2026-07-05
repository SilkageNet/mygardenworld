package policycfg

import (
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
)

func TestNormalizeFillsNewPlantDefaults(t *testing.T) {
	p := Normalize(&pb.Policy{})
	planting := p.GetPlant().GetPlanting()
	if planting.GetDemandPriority()[automation.GoalCustomerOrder] == 0 {
		t.Fatalf("demand priorities not populated: %+v", planting.GetDemandPriority())
	}
	if planting.GetAutoReplantMode() != pb.SelectionMode_SELECTION_MODE_ALL {
		t.Fatalf("auto replant mode=%v, want ALL", planting.GetAutoReplantMode())
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
