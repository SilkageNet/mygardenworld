package automation

import (
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestBuildPlanMainTaskDrivesExactCultivation(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"1401": 3, "1410": 3, "1422": 3, "1437": 1,
		}}},
		"22": map[string]any{"0": map[string]any{"1": 40001, "2": 0, "4": map[string]any{}}},
		"101": map[string]any{"0": map[string]any{
			"23002": map[string]any{"2": 0, "4": 0},
		}},
	})
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Task.MainEnabled = true
	policy.Plant.Cultivate.Enabled = true

	for _, planned := range BuildPlan(s, policy, time.Now()).Operations {
		if planned.Kind != clientproto.RPCCultivateCultivate.String() {
			continue
		}
		if planned.FlowerID != 23003 || planned.GoalID != GoalMainTask || !planned.Executable || planned.Status == PlanStatusBlocked {
			t.Fatalf("main-task cultivate=%+v", planned)
		}
		return
	}
	t.Fatal("missing task-driven cultivation operation")
}

func TestBuildPlanCultivateUsesObservedUnstartedCandidate(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"1401": 3, "1410": 3, "1422": 3, "1437": 1,
		}}},
		"101": map[string]any{"0": map[string]any{
			"23003": map[string]any{"2": 0, "4": 0},
		}},
	})
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.Cultivate.Enabled = true

	for _, planned := range BuildPlan(s, policy, time.Now()).Operations {
		if planned.Kind == clientproto.RPCCultivateCultivate.String() && planned.FlowerID == 23003 && planned.Executable {
			return
		}
	}
	t.Fatal("observed unstarted flower was not selected for cultivation")
}

func TestBuildPlanCultivateWaitsForActiveSlot(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"1401": 3, "1410": 3, "1422": 3, "1437": 1,
		}}},
		"101": map[string]any{"0": map[string]any{
			"23002": map[string]any{"2": 0, "3": time.Now().Add(time.Hour).UnixMilli(), "4": 1},
			"23003": map[string]any{"2": 0, "4": 0},
		}},
	})
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.Cultivate.Enabled = true

	for _, planned := range BuildPlan(s, policy, time.Now()).Operations {
		if planned.Kind == clientproto.RPCCultivateCultivate.String() {
			t.Fatalf("started a second cultivation while slot active: %+v", planned)
		}
	}
}

func TestBuildPlanMainTaskRefusesToRepeatCompletedCultivation(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"22": map[string]any{"0": map[string]any{"1": 40001, "2": 0, "4": map[string]any{}}},
		"101": map[string]any{"0": map[string]any{
			"23003": map[string]any{"2": 1, "4": 2},
		}},
	})
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Task.MainEnabled = true
	policy.Plant.Cultivate.Enabled = true

	foundBlocked := false
	for _, planned := range BuildPlan(s, policy, time.Now()).Operations {
		if planned.Kind == clientproto.RPCCultivateCultivate.String() && planned.FlowerID == 23003 {
			t.Fatalf("repeated completed cultivation: %+v", planned)
		}
		if planned.Domain == "basic.task.main" && planned.Action == "sync" && planned.Status == PlanStatusBlocked {
			foundBlocked = true
		}
	}
	if !foundBlocked {
		t.Fatal("missing stale main-task progress diagnostic")
	}
}
