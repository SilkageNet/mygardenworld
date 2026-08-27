package automation

import (
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestDailyWaterTaskLinksEnabledFarmOperation(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 10},
			"33": map[string]any{"7": map[string]any{"1": 130, "5": now.UnixMilli()}},
		}},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 1, "2": 1, "3": 0},
		}},
		"22": map[string]any{"1": map[string]any{
			"1": map[string]any{"3014": 0},
			"3": map[string]any{},
			"100": map[string]any{
				"30140001": map[string]any{"0": 30140001, "1": 2, "2": 0},
			},
		}},
	})
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Task.DailyEnabled = true
	policy.Plant.Planting.AutoEnabled = true
	policy.Union.Race.Enabled = false

	result := BuildPlan(s, policy, now)
	const demandID = GoalDailyTask + ":30140001"
	for _, planned := range result.Operations {
		if planned.Kind != clientproto.RPCUsrLandWater.String() && planned.Kind != clientproto.RPCUsrLandWaterBatch.String() {
			continue
		}
		if planned.DemandID != demandID || planned.Priority < dailyTaskDrivePriorityFloor || !strings.Contains(planned.Reason, "日常任务剩余 2 次") {
			t.Fatalf("daily-driven water operation=%+v", planned)
		}
		return
	}
	t.Fatalf("missing daily-driven water operation: %+v", result.Operations)
}

func TestDailyTaskDoesNotBypassDisabledOrUnsupportedModule(t *testing.T) {
	for _, test := range []struct {
		name         string
		taskID       int32
		progressType int32
	}{
		{name: "watering module disabled", taskID: 30140001, progressType: 3014},
		{name: "video unsupported", taskID: 30250001, progressType: 3025},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := state.New()
			applyMap(t, s, map[string]any{"22": map[string]any{"1": map[string]any{
				"1":   map[string]any{itoa32(test.progressType): 0},
				"3":   map[string]any{},
				"100": map[string]any{itoa32(test.taskID): map[string]any{"0": test.taskID, "1": 2, "2": 0}},
			}}})
			policy := DefaultPolicy()
			policy.AutomationEnabled = true
			policy.Basic.Task.DailyEnabled = true
			policy.Plant.Planting.AutoEnabled = false

			result := BuildPlan(s, policy, time.Now())
			for _, demand := range result.Demands {
				if demand.ID == GoalDailyTask+":"+itoa32(test.taskID) {
					t.Fatalf("task bypassed module/support gate: %+v", demand)
				}
			}
		})
	}
}
