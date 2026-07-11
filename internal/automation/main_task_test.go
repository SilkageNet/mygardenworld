package automation

import (
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestMainTaskPlannerClaimsOnlyObservedReadyUnclaimedTask(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"22": map[string]any{"0": map[string]any{
		"1": 910001, "2": 14, "4": map[string]any{},
	}}})
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Task.MainEnabled = true
	ops := mainTaskPlanOperations(BuildPlan(s, policy, time.Now()).Operations)
	if len(ops) != 1 {
		t.Fatalf("main task operations=%+v", ops)
	}
	op := ops[0]
	if op.Kind != clientproto.RPCTaskMainRecv.String() || op.TargetID != 910001 ||
		op.OperationID != clientproto.RPCTaskMainRecv.String() || !op.Executable ||
		op.Count != 0 || op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
		t.Fatalf("main task claim=%+v", op)
	}
}

func TestMainTaskPlannerFailsClosed(t *testing.T) {
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Task.MainEnabled = true
	tests := []struct {
		name        string
		main        map[string]any
		wantBlocked bool
	}{
		{name: "not finished", main: map[string]any{"1": 910001, "2": 13, "4": map[string]any{}}},
		{name: "receipt key even zero", main: map[string]any{"1": 910001, "2": 14, "4": map[string]any{"910001": 0}}},
		{name: "progress unknown", main: map[string]any{"1": 910001, "4": map[string]any{}}, wantBlocked: true},
		{name: "receipts unknown", main: map[string]any{"1": 910001, "2": 14}, wantBlocked: true},
		{name: "missing catalog definition", main: map[string]any{"1": 12345, "2": 999, "4": map[string]any{}}, wantBlocked: true},
		{name: "terminal despite later row", main: map[string]any{"1": 6950001}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := state.New()
			applyMap(t, s, map[string]any{"22": map[string]any{"0": tc.main}})
			ops := mainTaskPlanOperations(BuildPlan(s, policy, time.Now()).Operations)
			if tc.wantBlocked {
				if len(ops) != 1 || ops[0].Status != PlanStatusBlocked || ops[0].Executable ||
					ops[0].Kind == clientproto.RPCTaskMainRecv.String() {
					t.Fatalf("blocked main task operations=%+v", ops)
				}
				return
			}
			if len(ops) != 0 {
				t.Fatalf("main task operations=%+v, want none", ops)
			}
		})
	}
}

func TestMainTaskPlannerRequiresPolicy(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{"22": map[string]any{"0": map[string]any{
		"1": 910001, "2": 14, "4": map[string]any{},
	}}})
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	if ops := mainTaskPlanOperations(BuildPlan(s, policy, time.Now()).Operations); len(ops) != 0 {
		t.Fatalf("disabled main task planned=%+v", ops)
	}
}

func TestMainTaskPlannerBlocksUnobservedWithoutProbe(t *testing.T) {
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Task.MainEnabled = true
	ops := mainTaskPlanOperations(BuildPlan(state.New(), policy, time.Now()).Operations)
	if len(ops) != 1 || ops[0].Status != PlanStatusBlocked || ops[0].Executable ||
		ops[0].Kind == clientproto.RPCTaskMainRecv.String() {
		t.Fatalf("unobserved main task operations=%+v", ops)
	}
}

func mainTaskPlanOperations(ops []PlannedOp) []PlannedOp {
	out := make([]PlannedOp, 0, 1)
	for _, op := range ops {
		if op.Domain == "basic.task.main" {
			out = append(out, op)
		}
	}
	return out
}
