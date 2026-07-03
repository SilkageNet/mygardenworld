package policycfg

import (
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
)

func TestNormalizeFillsNewPlantDefaults(t *testing.T) {
	p := Normalize(&pb.Policy{})
	flower := p.GetPlant().GetFlower()
	if flower.GetGoalPriority()[automation.GoalCustomerOrder] == 0 {
		t.Fatalf("goal priorities not populated: %+v", flower.GetGoalPriority())
	}
}

func TestPolicyJSONRoundTripUsesFullParityTree(t *testing.T) {
	raw := `{
	  "automation_enabled": true,
	  "plant": {
	    "flower": {
	      "goal_priority": {"order.customer": 99}
	    }
	  }
	}`
	p, err := FromJSON(raw)
	if err != nil {
		t.Fatalf("FromJSON returned error: %v", err)
	}
	flower := p.GetPlant().GetFlower()
	if !p.GetAutomationEnabled() {
		t.Fatalf("policy values not kept: %+v", p)
	}
	if flower.GetGoalPriority()[automation.GoalCustomerOrder] != 99 {
		t.Fatalf("custom priority not kept: %+v", flower.GetGoalPriority())
	}
}

func TestFromJSONMigratesLegacyFlowerAuto(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{
			name: "legacy land cycle enabled",
			raw: `{
			  "plant": {
			    "flower": {
			      "harvest_enabled": false,
			      "plant_enabled": true,
			      "water_enabled": false
			    }
			  }
			}`,
			want: true,
		},
		{
			name: "legacy land cycle disabled",
			raw: `{
			  "plant": {
			    "flower": {
			      "harvest_enabled": false,
			      "plant_enabled": false,
			      "water_enabled": false
			    }
			  }
			}`,
			want: false,
		},
		{
			name: "new field wins",
			raw: `{
			  "plant": {
			    "flower": {
			      "auto_enabled": false,
			      "harvest_enabled": true,
			      "plant_enabled": true,
			      "water_enabled": true
			    }
			  }
			}`,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := FromJSON(tt.raw)
			if err != nil {
				t.Fatalf("FromJSON returned error: %v", err)
			}
			if got := p.GetPlant().GetFlower().GetAutoEnabled(); got != tt.want {
				t.Fatalf("auto enabled = %v, want %v", got, tt.want)
			}
		})
	}
}
