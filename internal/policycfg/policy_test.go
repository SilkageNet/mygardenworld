package policycfg

import (
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
)

func TestNormalizeFillsNewPlantDefaults(t *testing.T) {
	p := Normalize(&pb.Policy{})
	flower := p.GetPlant().GetFlower()
	if flower.GetPlantMaxBatch() != 8 || flower.GetMaxPerFlowerPerCycle() != 4 {
		t.Fatalf("plant defaults mismatch: %+v", p.GetPlant())
	}
	if flower.GetGoalPriority()[automation.GoalCustomerOrder] == 0 {
		t.Fatalf("goal priorities not populated: %+v", flower.GetGoalPriority())
	}
}

func TestPolicyJSONRoundTripUsesFullParityTree(t *testing.T) {
	raw := `{
	  "automation_enabled": true,
	  "plant": {
	    "flower": {
	      "planting_mode": "PLANTING_MODE_SPECIFIC",
	      "plant_max_batch": 3,
	      "max_per_flower_per_cycle": 2,
	      "goal_priority": {"order.customer": 99}
	    }
	  }
	}`
	p, err := FromJSON(raw)
	if err != nil {
		t.Fatalf("FromJSON returned error: %v", err)
	}
	flower := p.GetPlant().GetFlower()
	if !p.GetAutomationEnabled() || flower.GetPlantMaxBatch() != 3 || flower.GetMaxPerFlowerPerCycle() != 2 {
		t.Fatalf("policy values not preserved: %+v", p)
	}
	if flower.GetGoalPriority()[automation.GoalCustomerOrder] != 99 {
		t.Fatalf("custom priority not preserved: %+v", flower.GetGoalPriority())
	}
}
