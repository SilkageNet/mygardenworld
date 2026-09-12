package automation

import (
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestTaskAdvanceFlowerPassPlantForcesFarm(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": emptyLands(3)},
		"101": map[string]any{"0": cultivate(23001)},
		"131": map[string]any{
			"0": map[string]any{"15": map[string]any{"1": 15, "2": 1, "3": 0, "6": map[string]any{"1": []any{}}}},
			"1": map[string]any{"15": map[string]any{
				"1": 15,
				"5": []any{1008},
				"6": map[string]any{"3001_0": 1},
				"8": map[string]any{},
			}},
		},
	})

	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Task.FlowerPassAutoAdvance = true
	policy.Plant.Planting.AutoEnabled = false
	policy.Plant.Planting.AutoHarvestEnabled = false
	policy.Union.Race.Enabled = false

	actions := taskAdvanceActionDemands(s, policy, time.Now())
	if len(actions) != 1 || actions[0].ProgressType != taskAdvanceProgressPlantAny || actions[0].Demand.Missing != 59 {
		t.Fatalf("actions=%+v", actions)
	}
	if !taskAdvanceForceFarmPlant(actions) || !taskAdvanceForceFarmCycle(actions) {
		t.Fatal("expected force farm plant/cycle")
	}

	plan := BuildPlan(s, policy, time.Now())
	for _, op := range plan.Operations {
		if isPlantOperation(op.Kind) && op.Executable {
			return
		}
	}
	t.Fatalf("expected plant ops under auto-advance, got %+v", summarizeOps(plan.Operations))
}

func TestTaskAdvanceLinksCustomerOrderOp(t *testing.T) {
	action := taskAdvanceAction{
		Source:       taskAdvanceSourceFlowerPass,
		ProgressType: taskAdvanceProgressCustomerOrder,
		FeatureID:    "basic.flower_pass_advance",
		Demand: Demand{
			ID:      "basic.flower_pass:15:1004",
			Missing: 15,
			Label:   "顾客订单",
		},
	}
	ops := []PlannedOp{{
		Kind:       clientproto.RPCOrderCustomerFinishOrder.String(),
		Executable: true,
		Status:     PlanStatusManaged,
		FeatureID:  "order.customer",
		Reason:     "顾客订单可交付",
	}}
	ops = driveTaskAdvanceOperations(state.New(), DefaultPolicy(), []taskAdvanceAction{action}, NewInventoryLedger(nil), ops, time.Now())
	if ops[0].DemandID != action.Demand.ID || ops[0].FeatureID != "basic.flower_pass_advance" {
		t.Fatalf("link failed: %+v", ops[0])
	}
	if ops[0].Reason == "" || ops[0].Reason == "顾客订单可交付" {
		t.Fatalf("reason not annotated: %q", ops[0].Reason)
	}
}

func TestTaskAdvanceSkipsUnsupportedTypes(t *testing.T) {
	for _, pt := range []int32{
		taskAdvanceProgressVideo, taskAdvanceProgressVideoNum,
		taskAdvanceProgressGuildShare, taskAdvanceProgressElvesBook,
		taskAdvanceProgressElvesDispatch, taskAdvanceProgressConsume,
	} {
		if taskAdvanceProgressSupported(pt) {
			t.Fatalf("type %d must be unsupported", pt)
		}
	}
}

func TestTaskAdvanceDisabledEmitsNoActions(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"131": map[string]any{
			"0": map[string]any{"15": map[string]any{"1": 15, "2": 1, "6": map[string]any{"1": []any{}}}},
			"1": map[string]any{"15": map[string]any{
				"5": []any{1008},
				"6": map[string]any{"3001_0": 0},
				"8": map[string]any{},
			}},
		},
	})
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	if actions := taskAdvanceActionDemands(s, policy, time.Now()); len(actions) != 0 {
		t.Fatalf("got %+v", actions)
	}
}

func TestTaskAdvanceModuleGates(t *testing.T) {
	policy := DefaultPolicy()
	if !taskAdvanceModuleEnabled(policy, taskAdvanceProgressWater) {
		t.Fatal("water advance should self-drive")
	}
	if taskAdvanceModuleEnabled(policy, taskAdvanceProgressCustomerOrder) {
		t.Fatal("customer advance requires customer module")
	}
	policy.Order.Customer.Enabled = true
	if !taskAdvanceModuleEnabled(policy, taskAdvanceProgressCustomerOrder) {
		t.Fatal("customer advance should enable with module")
	}
}

func summarizeOps(ops []PlannedOp) []string {
	out := make([]string, 0, len(ops))
	for _, op := range ops {
		out = append(out, op.Kind+":"+op.FeatureID+":"+op.DemandID)
	}
	return out
}
