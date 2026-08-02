package automation

import (
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestStoryPlannerUsesExactCatalogCostAndIgnoresCounter111(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{
			"0":   map[string]any{"32": map[string]any{"56": 85}},
			"101": map[string]any{"1": 32, "2": 0},
			// 7.4.111 is usr cntMap's daily story-star statistic. It is
			// intentionally not an unlock limit, regardless of its value.
			"4": map[string]any{"111": map[string]any{"2": 999999}},
		},
	})
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Task.StoryEnabled = true
	ops := storyPlanOperations(BuildPlan(s, policy, time.Now()).Operations)
	if len(ops) != 1 {
		t.Fatalf("story operations=%+v", ops)
	}
	op := ops[0]
	if op.Kind != clientproto.RPCStoryMainUnlock.String() || op.TargetID != 4101 ||
		op.OperationID != clientproto.RPCStoryMainUnlock.String() || len(op.ItemCost) != 1 || op.ItemCost[56] != 85 || !op.Executable {
		t.Fatalf("story unlock=%+v", op)
	}
}

func TestStoryPlannerStopsAtCompleteAndBlocksInvalidObservedState(t *testing.T) {
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Task.StoryEnabled = true

	complete := state.New()
	applyMap(t, complete, map[string]any{"7": map[string]any{"101": map[string]any{"1": 165, "2": 0}}})
	if ops := storyPlanOperations(BuildPlan(complete, policy, time.Now()).Operations); len(ops) != 0 {
		t.Fatalf("completed story still planned operations: %+v", ops)
	}

	invalid := state.New()
	applyMap(t, invalid, map[string]any{"7": map[string]any{"101": map[string]any{}}})
	ops := storyPlanOperations(BuildPlan(invalid, policy, time.Now()).Operations)
	if len(ops) != 1 || ops[0].Status != PlanStatusBlocked || ops[0].Executable || ops[0].Kind == clientproto.RPCStoryMainEnter.String() {
		t.Fatalf("invalid observed story=%+v", ops)
	}
	for _, progress := range [][2]int32{{165, 1}, {166, 0}} {
		s := state.New()
		applyMap(t, s, map[string]any{"7": map[string]any{"101": map[string]any{"1": progress[0], "2": progress[1]}}})
		ops = storyPlanOperations(BuildPlan(s, policy, time.Now()).Operations)
		if len(ops) != 1 || ops[0].Status != PlanStatusBlocked || ops[0].Executable {
			t.Fatalf("invalid terminal progress %v planned=%+v", progress, ops)
		}
	}

	unobserved := state.New()
	ops = storyPlanOperations(BuildPlan(unobserved, policy, time.Now()).Operations)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCStoryMainEnter.String() || !ops[0].Executable {
		t.Fatalf("unobserved story=%+v", ops)
	}
}

func storyPlanOperations(ops []PlannedOp) []PlannedOp {
	out := make([]PlannedOp, 0, 1)
	for _, op := range ops {
		if op.Domain == "basic.story" {
			out = append(out, op)
		}
	}
	return out
}
