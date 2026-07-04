package policycfg

import (
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
)

func TestNormalizeFillsNewPlantDefaults(t *testing.T) {
	p := Normalize(&pb.Policy{})
	planting := p.GetPlant().GetPlanting()
	if planting.GetGoalPriority()[automation.GoalCustomerOrder] == 0 {
		t.Fatalf("goal priorities not populated: %+v", planting.GetGoalPriority())
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
	      "goal_priority": {"order.customer": 99}
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
	if planting.GetGoalPriority()[automation.GoalCustomerOrder] != 99 {
		t.Fatalf("custom priority not kept: %+v", planting.GetGoalPriority())
	}
}

func TestFromJSONIgnoresRemovedPlantFlowerField(t *testing.T) {
	raw := `{
	  "plant": {
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
	if got := p.GetPlant().GetPlanting().GetGoalPriority()[automation.GoalCustomerOrder]; got == 99 {
		t.Fatalf("removed plant.flower goal priority should be ignored")
	}
}
