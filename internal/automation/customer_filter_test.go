package automation

import (
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"google.golang.org/protobuf/proto"
)

func TestCustomerRewardFilterPreservesMismatches(t *testing.T) {
	for _, tt := range []struct {
		name       string
		exact      *int64
		wantFinish bool
	}{
		{"disabled", nil, true}, {"matching", proto.Int64(2), true},
		{"mismatch", proto.Int64(3), false}, {"zero is enabled", proto.Int64(0), false}, {"negative fail closed", proto.Int64(-1), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := state.New()
			s.ApplyVMap(map[string]any{
				"7":   map[string]any{"0": map[string]any{"32": map[string]any{"300207": 2}, "34": 12}},
				"109": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 2, "1": 300207, "2": 2, "3": 1}}}},
			})
			p := DefaultPolicy()
			p.AutomationEnabled = true
			p.Order.Customer.Enabled = true
			p.Order.Customer.RejectUnavailableEnabled = true
			p.Order.Customer.ExactFloralCoin = tt.exact
			result := BuildPlan(s, p, time.Now())
			finished := false
			for _, op := range result.Operations {
				if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && op.Executable {
					finished = true
				}
				if !tt.wantFinish && op.Executable && op.GoalID == GoalCustomerOrder {
					t.Fatalf("filtered order executed: %+v", op)
				}
			}
			if finished != tt.wantFinish {
				t.Fatalf("finish=%v want %v", finished, tt.wantFinish)
			}
			for _, demand := range result.Demands {
				if !tt.wantFinish && demand.GoalID == GoalCustomerOrder {
					t.Fatalf("filtered demand: %+v", demand)
				}
			}
			// The reward gate precedes the old minimum-art rejection as well.
			if !tt.wantFinish {
				p.Order.Customer.MinFlowerArtCount = 3
				for _, op := range BuildPlan(s, p, time.Now()).Operations {
					if op.GoalID == GoalCustomerOrder && op.Executable {
						t.Fatalf("minimum-art bypassed preservation: %+v", op)
					}
				}
			}
		})
	}
}
