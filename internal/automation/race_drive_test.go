package automation

import (
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// raceTakenPlantState seeds an active race batch with an unfinished plant-harvest
// task targeting flower 23001, plus empty lands and plantable cultivate unlock.
func raceTakenPlantState(t *testing.T, finish, target int32) *state.State {
	t.Helper()
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"0": 999}},
		"25": map[string]any{
			"111": map[string]any{"0": 1783872000000, "1": 1, "2": 1783990800000, "3": 1784466000000},
			"114": []any{
				map[string]any{"0": 99, "4": 4001, "6": []any{23001}, "10": 30, "12": 999, "14": 0, "15": 0},
			},
			"110": map[string]any{
				"1783872000000": map[string]any{
					"7": map[string]any{"0": 99, "1": 4001, "2": target, "3": finish, "4": []any{23001}},
				},
			},
		},
		"100": map[string]any{"1": emptyLands(3)},
		"101": map[string]any{"0": cultivate(23001)},
	})
	return s
}

func racePlantPolicy(useRaceSpeedup bool) *pb.Policy {
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.Planting.AutoEnabled = true
	policy.Plant.Planting.AutoHarvestEnabled = true
	policy.Plant.Planting.UseSpeedUpTicket = false
	policy.Union.Race.Enabled = true
	policy.Union.Race.AutoEnableModules = true
	policy.Union.Race.UseSpeedupTicketInTask = useRaceSpeedup
	policy.Union.Race.MaxTaskScore = 0 // accept any score for drive tests
	return policy
}

func TestRaceTakenPlantHarvestDrivesPlantOp(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 2, 10)
	policy := racePlantPolicy(false)

	result := BuildPlan(s, policy, now)
	var plant *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if isPlantOperation(op.Kind) && op.FlowerID == 23001 && op.Executable {
			plant = op
			break
		}
	}
	if plant == nil {
		t.Fatalf("expected plant op for race flower 23001, ops=%+v demands=%+v", result.Operations, result.Demands)
	}
	if plant.GoalID != "union.race" {
		t.Fatalf("plant GoalID=%q, want union.race", plant.GoalID)
	}
	if !strings.HasPrefix(plant.DemandID, "union.race:") {
		t.Fatalf("plant DemandID=%q, want union.race…", plant.DemandID)
	}
}

func TestRaceTakenPlantHarvestEmitsFlowerDemand(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 2, 10)
	policy := racePlantPolicy(false)

	result := BuildPlan(s, policy, now)
	demand, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001")
	if !ok {
		t.Fatalf("missing race flower demand, demands=%+v", result.Demands)
	}
	if demand.Kind != DemandKindFlower || demand.ItemID != 23001 || demand.Missing != 8 {
		t.Fatalf("demand=%+v, want flower 23001 missing=8", demand)
	}
}

func TestRaceNoPlantDemandWhenTaskComplete(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 10, 10)
	policy := racePlantPolicy(false)

	result := BuildPlan(s, policy, now)
	if _, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001"); ok {
		t.Fatalf("completed race task must not emit plant demand, demands=%+v", result.Demands)
	}
}

func TestRaceUseSpeedupTicketInTaskEnablesSpeedup(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 2, 10)
	// Replace empty lands with growing lands that need speedup.
	// Land schema: 0=flowerId, 1=state, 5=nextTimeMs.
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0":  999,
			"32": map[string]any{"1001": 5}, // speedup tickets
		}},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 2, "5": now.Add(2 * time.Hour).UnixMilli()},
			"1002": map[string]any{"0": 23001, "1": 2, "5": now.Add(2 * time.Hour).UnixMilli()},
		}},
	})
	policy := racePlantPolicy(true)

	result := BuildPlan(s, policy, now)
	var hasSpeedup bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCUsrLandSpeedUpBatch.String() && op.Executable {
			hasSpeedup = true
			break
		}
	}
	if !hasSpeedup {
		t.Fatalf("expected speedup when useSpeedupTicketInTask + plant race task, ops=%+v", result.Operations)
	}
}

func TestRaceNoSpeedupWhenFlagOff(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 2, 10)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0":  999,
			"32": map[string]any{"1001": 5},
		}},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 2, "5": now.Add(2 * time.Hour).UnixMilli()},
		}},
	})
	policy := racePlantPolicy(false)

	result := BuildPlan(s, policy, now)
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCUsrLandSpeedUpBatch.String() && op.Executable {
			t.Fatalf("speedup must stay off when useSpeedupTicketInTask=false, op=%+v", op)
		}
	}
}
