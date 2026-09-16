package apiserver

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"google.golang.org/protobuf/proto"
)

func TestCustomerRewardFilterView(t *testing.T) {
	for _, tt := range []struct {
		art   int
		known bool
	}{{300207, true}, {99999999, false}} {
		s := state.New()
		s.ApplyVMap(map[string]any{"109": map[string]any{"0": map[string]any{"1": map[string]any{"1": map[string]any{"0": 2, "1": tt.art, "2": 2, "3": 1}}}}})
		p := automation.DefaultPolicy()
		p.Order.Customer.ExactFloralCoin = proto.Int64(3)
		found := false
		for _, view := range buildPendingTasksAtPolicy(s, time.Now(), p) {
			if view.Category != "顾客订单" {
				continue
			}
			found = true
			if (view.FloralCoinReward != nil) != tt.known || (tt.known && view.GetFloralCoinReward() != 2) {
				t.Fatalf("reward=%+v", view)
			}
			if view.AutomationSkipReason == "" || view.Status != pb.PlanStatus_PLAN_STATUS_SKIPPED {
				t.Fatalf("filtered view=%+v", view)
			}
		}
		if !found {
			t.Fatal("order disappeared from monitor")
		}
	}
}
