package runner

import (
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"google.golang.org/protobuf/proto"
)

func TestCustomerRewardFilterSendGate(t *testing.T) {
	r := newOperationEventTestRunner()
	r.policy = automation.DefaultPolicy()
	r.policy.Order.Customer.ExactFloralCoin = proto.Int64(3)
	r.state.ApplyVMap(map[string]any{"109": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 2, "1": 300207, "2": 2, "3": 1}}}}})
	for _, kind := range []string{clientproto.RPCOrderCustomerFinishOrder.String(), clientproto.RPCOrderCustomerRejectOrder.String(), clientproto.RPCFlowerArtMakeFlowerArt.String()} {
		t.Run(kind, func(t *testing.T) {
			op := &automation.PlannedOp{Kind: kind, GoalID: automation.GoalCustomerOrder, TargetID: 1}
			if err := r.checkOperationResources(op, time.Now()); err == nil {
				t.Fatal("stale planned operation bypassed reward filter")
			}
			r.policy.Order.Customer.ExactFloralCoin = proto.Int64(2)
			if err := r.checkOperationResources(op, time.Now()); err != nil {
				t.Fatal(err)
			}
			r.policy.Order.Customer.ExactFloralCoin = proto.Int64(3)
		})
	}
	// Rack crafts sharing an art are not customer orders.
	if err := r.checkOperationResources(&automation.PlannedOp{Kind: clientproto.RPCFlowerArtMakeFlowerArt.String(), GoalID: automation.GoalFlowerArt}, time.Now()); err != nil {
		t.Fatal(err)
	}
}
