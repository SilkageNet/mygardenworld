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
					"3": 0, // fTaskNum — mark TaskQuotaObserved so usr-rank sync does not preempt
					"4": 0, // score — mark ScoreObserved
					"7": map[string]any{"0": 99, "1": 4001, "2": target, "3": finish, "4": []any{23001}},
				},
			},
			"116": []any{
				map[string]any{"0": 999, "1": 1783872000000, "3": 0, "4": 0},
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
	policy.Union.Race.AutoGiveUpTask = true
	policy.Union.Race.UseSpeedupTicketInTask = useRaceSpeedup
	policy.Union.Race.MinTaskScore = 0 // accept any score for drive tests
	return policy
}

func TestRaceHeldLowScoreContinuesWhenAutoGiveUpOff(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 0, 10)
	policy := racePlantPolicy(false)
	policy.Union.Race.AutoGiveUpTask = false
	policy.Union.Race.MinTaskScore = 30

	result := BuildPlan(s, policy, now)
	if _, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001"); !ok {
		t.Fatalf("held task should keep progressing when auto give-up is off, demands=%+v", result.Demands)
	}
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("held task must not be released without explicit opt-in: %+v", op)
		}
	}
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

func TestRacePlantDemandOverridesRegularFarmToggles(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 2, 10)
	policy := racePlantPolicy(false)
	policy.Plant.Planting.AutoEnabled = false
	policy.Plant.Planting.AutoHarvestEnabled = false

	result := BuildPlan(s, policy, now)
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.FlowerID == 23001 && op.Executable {
			return
		}
	}
	t.Fatalf("active race task must still drive its farm module, ops=%+v demands=%+v", result.Operations, result.Demands)
}

func TestRacePlantBeatsHigherPriorityOrderDemand(t *testing.T) {
	// Customer-order demand priority defaults to 90; race must still own empty
	// lands first when DemandPriorityEnabled is on.
	s := state.New()
	applyMap(t, s, map[string]any{
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 1, "4": 2},
			"23002": map[string]any{"1": 23002, "2": 1, "4": 2},
		}},
	})
	p := DefaultPolicy()
	p.Plant.Planting.DemandPriorityEnabled = true

	assignments := plantAssignments(s, p.Plant, []Demand{
		{
			ID:       "order-23002",
			GoalID:   GoalCustomerOrder,
			Kind:     DemandKindFlower,
			ItemID:   23002,
			Missing:  6,
			Priority: 90,
			Label:    "顾客订单",
		},
		{
			ID:       "union.race:99:race_task:flower:23001",
			GoalID:   raceActionGoal,
			Source:   "race_task",
			Kind:     DemandKindFlower,
			ItemID:   23001,
			Missing:  4,
			Priority: raceDemandPriority,
			Label:    "公会竞赛种植",
		},
	}, 3)
	if len(assignments) == 0 {
		t.Fatal("expected race plant assignment")
	}
	if assignments[0].GoalID != raceActionGoal || assignments[0].FlowerID != 23001 {
		t.Fatalf("race must claim lands before customer order, got %+v", assignments)
	}
	if assignments[0].Count != 3 {
		t.Fatalf("race should take all 3 empty lands, count=%d assignments=%+v", assignments[0].Count, assignments)
	}
}

func TestRacePlantBeatsCustomerOrderInBuildPlan(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 0, 10)
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": emptyLands(3)},
		"101": map[string]any{"0": cultivate(23001, 23002)},
	})
	policy := racePlantPolicy(false)
	policy.Plant.Planting.DemandPriorityEnabled = true
	policy.Order.Customer.Enabled = true

	// Inject a higher-looking customer flower demand via direct plantAssignments
	// path is covered above; here ensure BuildPlan race demand still plants 23001
	// first. With only 3 empties and race needing 5, race claims every slot so
	// auto-replant has nothing left.
	result := BuildPlan(s, policy, now)
	var plantedRace, plantedOther int32
	for _, op := range result.Operations {
		if !isPlantOperation(op.Kind) || !op.Executable {
			continue
		}
		n := int32(len(op.LandIDs))
		switch {
		case op.FlowerID == 23001 && op.GoalID == raceActionGoal:
			plantedRace += n
		default:
			plantedOther += n
		}
	}
	if plantedRace == 0 {
		t.Fatalf("expected race plant, ops=%+v demands=%+v", result.Operations, result.Demands)
	}
	if plantedOther > 0 {
		t.Fatalf("race must claim all scarce empties before other planting, race=%d other=%d ops=%+v", plantedRace, plantedOther, result.Operations)
	}
}

func TestRaceWatersBeforeOtherCropsWhenAutoOn(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 0, 10)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0": 999,
			// 7 drops on hand; MinWaterDrops=5 → only 2 usable. nextMs in the
			// future so projection does not recover an extra drop.
			"32": map[string]any{"7": 7},
			"33": map[string]any{"7": map[string]any{"1": 130, "5": now.Add(time.Hour).UnixMilli()}},
		}},
		"100": map[string]any{"1": map[string]any{
			// Lower land IDs are non-race; without prioritization they would
			// consume the scarce water drops first.
			"1001": map[string]any{"0": 23002, "1": 1, "2": 1, "3": 0},
			"1002": map[string]any{"0": 23002, "1": 1, "2": 1, "3": 0},
			"1003": map[string]any{"0": 23002, "1": 1, "2": 1, "3": 0},
			"1004": map[string]any{"0": 23001, "1": 1, "2": 1, "3": 0},
			"1005": map[string]any{"0": 23001, "1": 1, "2": 1, "3": 0},
		}},
		"101": map[string]any{"0": cultivate(23001, 23002)},
	})
	policy := racePlantPolicy(false)
	policy.Plant.Planting.AutoEnabled = true
	policy.Plant.Planting.MinWaterDrops = 5

	result := BuildPlan(s, policy, now)
	var water *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if (op.Kind == clientproto.RPCUsrLandWater.String() || op.Kind == clientproto.RPCUsrLandWaterBatch.String()) && op.Executable {
			water = op
			break
		}
	}
	if water == nil {
		t.Fatalf("expected water op, ops=%+v", result.Operations)
	}
	// usable = 7-5 = 2 drops → must be exactly the two race lands.
	if len(water.LandIDs) != 2 {
		t.Fatalf("water lands=%v, want exactly [1004 1005]", water.LandIDs)
	}
	for _, id := range water.LandIDs {
		if id != 1004 && id != 1005 {
			t.Fatalf("scarce water must hit race lands first, got %v", water.LandIDs)
		}
	}
}

func TestRaceSpeedupPreferredWhenGlobalSpeedupOn(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 2, 10)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0":  999,
			"32": map[string]any{"1001": 1}, // one ticket only
		}},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23002, "1": 2, "5": now.Add(2 * time.Hour).UnixMilli()},
			"1002": map[string]any{"0": 23001, "1": 2, "5": now.Add(2 * time.Hour).UnixMilli()},
		}},
	})
	policy := racePlantPolicy(true)
	policy.Plant.Planting.UseSpeedUpTicket = true

	result := BuildPlan(s, policy, now)
	var speed *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCUsrLandSpeedUpBatch.String() && op.Executable {
			speed = op
			break
		}
	}
	if speed == nil {
		t.Fatalf("expected speedup, ops=%+v", result.Operations)
	}
	if len(speed.LandIDs) != 1 || speed.LandIDs[0] != 1002 {
		t.Fatalf("global+race speedup must prefer race land 1002, got %v", speed.LandIDs)
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
	// cultivate helper seeds lvl=1 → cropGets=2 × frequencys=1 = 2 flowers/plant.
	// remaining flowers 8 → ceil(8/2) = 4 plant slots.
	if demand.Kind != DemandKindFlower || demand.ItemID != 23001 || demand.Missing != 4 {
		t.Fatalf("demand=%+v, want flower 23001 missing=4", demand)
	}
}

func TestRaceNoPlantDemandWhenTakenBelowMinScore(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := state.New()
	// Score 9 below min=24, FinishCnt=0 → give up, do not plant.
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"0": 999}},
		"25": map[string]any{
			"111": map[string]any{"0": 1783872000000, "1": 1, "2": 1783990800000, "3": 1784466000000},
			"117": map[string]any{"5": 4},
			"114": []any{
				map[string]any{"0": 99, "4": 4001, "6": []any{23001}, "10": 9, "12": 999, "14": 0, "15": 0},
			},
			"110": map[string]any{
				"1783872000000": map[string]any{
					"3": 0,
					"4": 0,
					"7": map[string]any{"0": 99, "1": 4001, "2": 10, "3": 0, "4": []any{23001}},
				},
			},
			"116": []any{
				map[string]any{"0": 999, "1": 1783872000000, "3": 0, "4": 0},
			},
		},
		"100": map[string]any{"1": emptyLands(3)},
		"101": map[string]any{"0": cultivate(23001)},
	})
	policy := racePlantPolicy(false)
	policy.Union.Race.MinTaskScore = 24

	result := BuildPlan(s, policy, now)
	if _, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001"); ok {
		t.Fatalf("sub-threshold race task must not emit plant demand, demands=%+v", result.Demands)
	}
	var hasGiveUp bool
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.Executable && op.GoalID == "union.race" {
			t.Fatalf("sub-threshold race task must not race-plant, op=%+v", op)
		}
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() && op.Executable {
			hasGiveUp = true
		}
	}
	if !hasGiveUp {
		t.Fatalf("expected giveUp for sub-threshold taken task, ops=%+v", result.Operations)
	}
}

func TestRaceNoPlantDemandWhenTakenScoreUnresolvedUnderMinGate(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := state.New()
	// In pool with Score=0, FinishCnt=0, min_task_score set → wait, do not plant.
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"0": 999}},
		"25": map[string]any{
			"111": map[string]any{"0": 1783872000000, "1": 1, "2": 1783990800000, "3": 1784466000000},
			"114": []any{
				map[string]any{"0": 99, "4": 4001, "6": []any{23001}, "10": 0, "12": 999, "14": 0, "15": 0},
			},
			"110": map[string]any{
				"1783872000000": map[string]any{
					"7": map[string]any{"0": 99, "1": 4001, "2": 10, "3": 0, "4": []any{23001}},
				},
			},
		},
		"100": map[string]any{"1": emptyLands(3)},
		"101": map[string]any{"0": cultivate(23001)},
	})
	policy := racePlantPolicy(false)
	policy.Union.Race.MinTaskScore = 24

	result := BuildPlan(s, policy, now)
	if _, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001"); ok {
		t.Fatalf("unresolved score under min gate must not plant, demands=%+v", result.Demands)
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

// TestRaceSpeedupOnlyTargetsRaceFlower ensures race-only accel does not burn
// tickets on unrelated growing crops sharing the field (Siri-style mixed plot).
func TestRaceSpeedupOnlyTargetsRaceFlower(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 2, 10)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0":  999,
			"32": map[string]any{"1001": 5},
		}},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 2, "5": now.Add(2 * time.Hour).UnixMilli()}, // race flower
			"1002": map[string]any{"0": 23002, "1": 2, "5": now.Add(2 * time.Hour).UnixMilli()}, // other crop
			"1003": map[string]any{"0": 23002, "1": 2, "5": now.Add(2 * time.Hour).UnixMilli()},
		}},
	})
	policy := racePlantPolicy(true)

	result := BuildPlan(s, policy, now)
	var speed *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCUsrLandSpeedUpBatch.String() && op.Executable {
			speed = op
			break
		}
	}
	if speed == nil {
		t.Fatalf("expected race flower speedup, ops=%+v", result.Operations)
	}
	if len(speed.LandIDs) != 1 || speed.LandIDs[0] != 1001 {
		t.Fatalf("race speedup LandIDs=%v, want only [1001]", speed.LandIDs)
	}
	if speed.ItemCost[1001] != 1 {
		t.Fatalf("ItemCost=%v, want 1 ticket", speed.ItemCost)
	}
}

// TestRaceExpireUrgentSpeedupForcedFallback ensures that within 10 minutes of
// ExpireTime, unfinished plant-harvest tasks always use speedup tickets even
// when the regular race speedup toggle is off.
func TestRaceExpireUrgentSpeedupForcedFallback(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	expire := now.Add(5 * time.Minute).UnixMilli()
	s := raceTakenPlantState(t, 2, 10)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0":  999,
			"32": map[string]any{"1001": 5},
		}},
		"25": map[string]any{
			"110": map[string]any{
				"1783872000000": map[string]any{
					"7": map[string]any{"0": 99, "1": 4001, "2": 10, "3": 2, "4": []any{23001}, "5": expire},
				},
			},
		},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 2, "5": now.Add(2 * time.Hour).UnixMilli()},
		}},
	})
	if got := s.FmlRace().Taken.ExpireTime; got != expire {
		t.Fatalf("ExpireTime=%d, want %d", got, expire)
	}
	policy := racePlantPolicy(false) // UseSpeedupTicketInTask off

	result := BuildPlan(s, policy, now)
	var speed *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCUsrLandSpeedUpBatch.String() && op.Executable {
			speed = op
			break
		}
	}
	if speed == nil {
		t.Fatalf("expected forced urgent expire speedup, ops=%+v", result.Operations)
	}
	if speed.Reason != "公会竞赛任务即将过期，使用加速卡" {
		t.Fatalf("Reason=%q, want expire urgency reason", speed.Reason)
	}
	if len(speed.LandIDs) != 1 || speed.LandIDs[0] != 1001 {
		t.Fatalf("LandIDs=%v, want [1001]", speed.LandIDs)
	}
}

func TestRaceNoSpeedupFarFromExpireWithoutToggle(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	expire := now.Add(30 * time.Minute).UnixMilli() // outside 10m lead
	s := raceTakenPlantState(t, 2, 10)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0":  999,
			"32": map[string]any{"1001": 5},
		}},
		"25": map[string]any{
			"110": map[string]any{
				"1783872000000": map[string]any{
					"7": map[string]any{"0": 99, "1": 4001, "2": 10, "3": 2, "4": []any{23001}, "5": expire},
				},
			},
		},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 2, "5": now.Add(2 * time.Hour).UnixMilli()},
		}},
	})
	policy := racePlantPolicy(false)

	result := BuildPlan(s, policy, now)
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCUsrLandSpeedUpBatch.String() && op.Executable {
			t.Fatalf("must not speedup far from expire without toggle, got %+v", op)
		}
	}
}

func TestRaceExpiredTaskStopsProgressAndRefreshes(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000)
	s := raceTakenPlantState(t, 2, 10)
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"117": map[string]any{"5": 4},
			"110": map[string]any{
				"1783872000000": map[string]any{
					"3": 0,
					"7": map[string]any{"0": 99, "1": 4001, "2": 10, "3": 2, "4": []any{23001}, "5": now.Add(-time.Second).UnixMilli()},
				},
			},
		},
	})
	s.MarkFmlRaceTaskPoolStale()
	policy := racePlantPolicy(true)

	result := BuildPlan(s, policy, now)
	foundSync := false
	for _, op := range result.Operations {
		switch {
		case op.Kind == clientproto.RPCFmlRaceGetTaskList.String() && op.Executable:
			foundSync = true
		case (isPlantOperation(op.Kind) || op.Kind == clientproto.RPCUsrLandSpeedUpBatch.String()) && op.Executable:
			t.Fatalf("expired race task must not progress or spend tickets, op=%+v", op)
		}
	}
	if !foundSync {
		t.Fatalf("expired race task must refresh server state, ops=%+v", result.Operations)
	}
}

func TestGlobalSpeedUpTicketStillCoversAllGrowingLands(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 2, 10)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0":  999,
			"32": map[string]any{"1001": 5},
		}},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 2, "5": now.Add(2 * time.Hour).UnixMilli()},
			"1002": map[string]any{"0": 23002, "1": 2, "5": now.Add(2 * time.Hour).UnixMilli()},
		}},
	})
	policy := racePlantPolicy(false)
	policy.Plant.Planting.UseSpeedUpTicket = true

	result := BuildPlan(s, policy, now)
	var speed *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCUsrLandSpeedUpBatch.String() && op.Executable {
			speed = op
			break
		}
	}
	if speed == nil {
		t.Fatalf("expected global speedup, ops=%+v", result.Operations)
	}
	if len(speed.LandIDs) != 2 {
		t.Fatalf("global speedup LandIDs=%v, want both growing lands", speed.LandIDs)
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

// TestRaceTakenEnrichedTargetCntFromPoolDrivesPlant covers the case where the
// takeTask server response populates field 110 without targetCnt/finishCnt
// (fields 2/3) but the pool row (field 114) carries the correct progress.
// Enrichment must backfill TargetCnt/FinishCnt from the pool so the race plant
// demand fires.
func TestRaceTakenEnrichedTargetCntFromPoolDrivesPlant(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"0": 999}},
		"25": map[string]any{
			"111": map[string]any{"0": 1783872000000, "1": 1, "2": 1783990800000, "3": 1784466000000},
			"114": []any{
				// Pool row with UID=self, TargetCnt=10, FinishCnt=2.
				map[string]any{"0": 99, "4": 4001, "6": []any{23001}, "7": 10, "8": 2, "10": 30, "12": 999},
			},
			// 110 has takeTaskData WITHOUT targetCnt (2) or finishCnt (3);
			// also without param (4). Enrichment must backfill from pool.
			"110": map[string]any{
				"1783872000000": map[string]any{
					"7": map[string]any{"0": 99, "1": 4001},
				},
			},
		},
		"100": map[string]any{"1": emptyLands(3)},
		"101": map[string]any{"0": cultivate(23001)},
	})
	policy := racePlantPolicy(false)

	taken := s.FmlRace().Taken
	if !taken.HasTask {
		t.Fatalf("expected HasTask, got %+v", taken)
	}
	if taken.TargetCnt != 10 {
		t.Fatalf("TargetCnt = %d, want 10 (enriched from pool)", taken.TargetCnt)
	}
	if taken.FinishCnt != 2 {
		t.Fatalf("FinishCnt = %d, want 2 (enriched from pool)", taken.FinishCnt)
	}
	if taken.ParamID != 23001 {
		t.Fatalf("ParamID = %d, want 23001 (enriched from pool)", taken.ParamID)
	}

	result := BuildPlan(s, policy, now)
	demand, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001")
	if !ok {
		t.Fatalf("expected race plant demand after enrichment, demands=%+v", result.Demands)
	}
	if demand.Missing != 4 {
		t.Fatalf("demand Missing=%d, want 4 (8 flowers / 2 per plant at lvl 1)", demand.Missing)
	}
	var foundPlant bool
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.FlowerID == 23001 && op.Executable {
			foundPlant = true
			break
		}
	}
	if !foundPlant {
		t.Fatalf("expected plant op for race flower 23001 after enrichment, ops=%+v", result.Operations)
	}
}

func TestRacePlantMissingUsesFlowerLvlYield(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 2, 10)
	// Raise cultivate level to 10: cropGets=3 × frequencys=3 = 9 flowers/plant.
	applyMap(t, s, map[string]any{
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 10, "4": 2},
		}},
	})
	policy := racePlantPolicy(false)

	result := BuildPlan(s, policy, now)
	demand, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001")
	if !ok {
		t.Fatalf("missing race flower demand, demands=%+v", result.Demands)
	}
	// remaining 8 → ceil(8/9) = 1 plant slot.
	if demand.Missing != 1 {
		t.Fatalf("demand Missing=%d, want 1 at lvl 10", demand.Missing)
	}
}

func TestRacePlantMissingCreditsPlantedLands(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 2, 10)
	// Four planted lands at lvl 1 (2 flowers each, 0 harvests) cover remaining 8.
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 2, "2": 1, "3": 0},
			"1002": map[string]any{"0": 23001, "1": 2, "2": 1, "3": 0},
			"1003": map[string]any{"0": 23001, "1": 2, "2": 1, "3": 0},
			"1004": map[string]any{"0": 23001, "1": 2, "2": 1, "3": 0},
		}},
	})
	policy := racePlantPolicy(false)

	result := BuildPlan(s, policy, now)
	if _, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001"); ok {
		t.Fatalf("planted yield covering remaining must not emit plant demand, demands=%+v", result.Demands)
	}
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.Executable && op.GoalID == raceActionGoal {
			t.Fatalf("must not plant when pending yield covers race remaining, op=%+v", op)
		}
	}
}

func TestRacePlantDoesNotTopUpWhenPendingCoversSyncedProgress(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// Target 280, lvl 11 → 3×4=12/plant. 24 lands already took first harvest
	// round; FinishCnt synced to 72 via field 134. Pending roundsLeft=3 → 216
	// covers remaining 208 — no race top-up onto empty lands (自主补种 may still
	// fill leftovers when AutoEnabled).
	s := raceTakenPlantState(t, 72, 280)
	lands := map[string]any{}
	for i := 0; i < 24; i++ {
		lands[itoa(1001+i)] = map[string]any{"0": 23001, "1": 2, "2": 11, "3": 1}
	}
	for i := 24; i < 32; i++ {
		lands[itoa(1001+i)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": lands},
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 11, "4": 2},
		}},
	})
	policy := racePlantPolicy(false)

	result := BuildPlan(s, policy, now)
	if _, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001"); ok {
		t.Fatalf("pending yield covering synced remaining must not top-up, demands=%+v", result.Demands)
	}
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.Executable && op.GoalID == raceActionGoal {
			t.Fatalf("must not top-up when pending covers progress: %+v", op)
		}
	}
}

func TestRacePlantTopsUpWhenPendingCannotCoverTarget(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// Remaining 280, only 10 lands planted (pending 10*12=120). After verifying
	// progress+pending cannot finish the task, top-up ceil((280-120)/12)=14.
	s := raceTakenPlantState(t, 0, 280)
	lands := map[string]any{}
	for i := 0; i < 10; i++ {
		lands[itoa(1001+i)] = map[string]any{"0": 23001, "1": 1, "2": 11, "3": 0}
	}
	for i := 10; i < 40; i++ {
		lands[itoa(1001+i)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": lands},
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 11, "4": 2},
		}},
	})
	policy := racePlantPolicy(false)

	result := BuildPlan(s, policy, now)
	demand, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001")
	if !ok {
		t.Fatalf("missing race top-up demand, demands=%+v", result.Demands)
	}
	if demand.Missing != 14 {
		t.Fatalf("demand Missing=%d, want 14 top-up", demand.Missing)
	}
	var plantCount int32
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.Executable && op.GoalID == raceActionGoal && op.FlowerID == 23001 {
			plantCount += int32(len(op.LandIDs))
		}
	}
	if plantCount != 14 {
		t.Fatalf("planted %d lands, want 14 top-up while 10 still growing", plantCount)
	}
}

func TestRacePlantReplantsAfterAllRaceLandsClearIfIncomplete(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// Target 280, lvl11 → 12/plant. First round planted only 10 lands and fully
	// harvested them (LocalFinish=120). Lands cleared, task still short → plant
	// ceil((280-120)/12)=14 more.
	s := raceTakenPlantState(t, 0, 280)
	applyMap(t, s, map[string]any{
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 11, "4": 2},
		}},
	})
	lands := map[string]any{}
	for i := 0; i < 10; i++ {
		lands[itoa(1001+i)] = map[string]any{"0": 23001, "1": 2, "2": 11, "3": 0}
	}
	for i := 10; i < 40; i++ {
		lands[itoa(1001+i)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{"100": map[string]any{"1": lands}})
	harvested := map[string]any{}
	for i := 0; i < 10; i++ {
		harvested[itoa(1001+i)] = map[string]any{"0": 23001, "1": 2, "2": 11, "3": 4}
	}
	for i := 10; i < 40; i++ {
		harvested[itoa(1001+i)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{"100": map[string]any{"1": harvested}})
	applyMap(t, s, map[string]any{"100": map[string]any{"1": emptyLands(40)}})
	got := s.FmlRace()
	if got.LocalFinishCnt < 120 {
		t.Fatalf("LocalFinishCnt=%d, want >=120 after 10×4×3 harvests", got.LocalFinishCnt)
	}
	policy := racePlantPolicy(false)
	result := BuildPlan(s, policy, now)
	demand, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001")
	if !ok {
		t.Fatalf("missing race demand after clear, demands=%+v", result.Demands)
	}
	if demand.Missing != 14 {
		t.Fatalf("demand Missing=%d, want 14 after first round cleared", demand.Missing)
	}
	var plantCount int32
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.Executable && op.GoalID == raceActionGoal && op.FlowerID == 23001 {
			plantCount += int32(len(op.LandIDs))
		}
	}
	if plantCount != 14 {
		t.Fatalf("planted %d lands, want 14 replant after clear", plantCount)
	}
}

func TestRacePlantNoReplantDuringPartialSpeedupHarvest(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// 24 lands planted for target 280. First 5 fully harvested and emptied after
	// speedup; 19 still growing. progress 60 + pending 19*12=228 covers 280 —
	// must not replant into the 5 empty slots.
	s := raceTakenPlantState(t, 0, 280)
	lands := map[string]any{}
	for i := 0; i < 5; i++ {
		lands[itoa(1001+i)] = map[string]any{}
	}
	for i := 5; i < 24; i++ {
		lands[itoa(1001+i)] = map[string]any{
			"0": 23001, "1": 2, "2": 11, "3": 0,
			"5": now.Add(2 * time.Hour).UnixMilli(),
		}
	}
	for i := 24; i < 40; i++ {
		lands[itoa(1001+i)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0":  999,
			"32": map[string]any{"1001": 20},
		}},
		"100": map[string]any{"1": lands},
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 11, "4": 2},
		}},
	})
	// Simulate LocalFinish from the 5 cleared lands (5×12=60).
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"110": map[string]any{
				"1783872000000": map[string]any{
					"7": map[string]any{"0": 99, "1": 4001, "2": 280, "3": 60, "4": []any{23001}},
				},
			},
		},
	})
	policy := racePlantPolicy(true)
	result := BuildPlan(s, policy, now)
	if _, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001"); ok {
		t.Fatalf("partial speedup harvest must not emit replant demand, demands=%+v", result.Demands)
	}
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.Executable && op.GoalID == raceActionGoal {
			t.Fatalf("must not replant while remaining race flowers still growing: %+v", op)
		}
	}
	var hasSpeedup bool
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCUsrLandSpeedUpBatch.String() && op.Executable {
			hasSpeedup = true
			break
		}
	}
	if !hasSpeedup {
		t.Fatal("expected speedup on remaining growing race lands")
	}
}

func TestRacePlantTopsUpAfterHarvestRoundWhenShort(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// Target 280, lvl11 → 12/plant. First round: 10 lands harvested once
	// (HarvestCnt=1 → local harvested 30, pending 10*9=90). Remaining need
	// 280-30-90=160 → ceil(160/12)=14 top-up onto empty lands.
	s := raceTakenPlantState(t, 0, 280)
	lands := map[string]any{}
	for i := 0; i < 10; i++ {
		lands[itoa(1001+i)] = map[string]any{"0": 23001, "1": 2, "2": 11, "3": 1}
	}
	for i := 10; i < 40; i++ {
		lands[itoa(1001+i)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": lands},
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 11, "4": 2},
		}},
	})
	if got := racePlantHarvestPlantMissing(s, 23001, 280, 0); got != 14 {
		t.Fatalf("racePlantHarvestPlantMissing after first round=%d, want 14", got)
	}
	policy := racePlantPolicy(false)
	result := BuildPlan(s, policy, now)
	demand, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001")
	if !ok {
		t.Fatalf("missing top-up demand after harvest round, demands=%+v", result.Demands)
	}
	if demand.Missing != 14 {
		t.Fatalf("demand Missing=%d, want 14", demand.Missing)
	}
	var plantCount int32
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.Executable && op.GoalID == raceActionGoal && op.FlowerID == 23001 {
			plantCount += int32(len(op.LandIDs))
		}
	}
	if plantCount != 14 {
		t.Fatalf("planted %d lands, want 14 after harvest-round shortfall", plantCount)
	}
}

func TestRacePlantHarvestPlantMissingUnit(t *testing.T) {
	s := raceTakenPlantState(t, 0, 10)
	// lvl 1 empty farm: 8 flowers → 4 plants.
	if got := racePlantHarvestPlantMissing(s, 23001, 8, 0); got != 4 {
		t.Fatalf("racePlantHarvestPlantMissing(empty,8)=%d, want 4", got)
	}
	applyMap(t, s, map[string]any{
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 20, "4": 2},
		}},
	})
	// lvl 20: cropGets=5 × frequencys=5 = 25 → ceil(600/25)=24.
	if got := racePlantHarvestPlantMissing(s, 23001, 600, 0); got != 24 {
		t.Fatalf("racePlantHarvestPlantMissing(lvl20,600)=%d, want 24", got)
	}
	// 2 planted lands: pending 2*25=50, shortfall 550 → ceil(550/25)=22 top-up.
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 1, "2": 20, "3": 0},
			"1002": map[string]any{"0": 23001, "1": 1, "2": 20, "3": 0},
		}},
	})
	if got := racePlantHarvestPlantMissing(s, 23001, 600, 0); got != 22 {
		t.Fatalf("racePlantHarvestPlantMissing(2 planted,600)=%d, want 22", got)
	}
}

func TestRacePlantNoTopUpWhenFinishCntLagsHarvestCnt(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// 榴莲千层: target 600, lvl16 → 4×4=16/plant → first plant 38 lands.
	// After speedup first-round harvest, HarvestCnt=1 on all 38 but FinishCnt
	// still 0 (field 134 lag). Pending 38*12=456 looks short of 600 and would
	// top-up 9 lands without local-harvest credit; local 38*4=152 covers the gap.
	s := raceTakenPlantState(t, 0, 600)
	lands := map[string]any{}
	for i := 0; i < 38; i++ {
		lands[itoa(1001+i)] = map[string]any{"0": 23001, "1": 2, "2": 16, "3": 1}
	}
	for i := 38; i < 64; i++ {
		lands[itoa(1001+i)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": lands},
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 16, "4": 2},
		}},
	})
	if got := racePlantHarvestPlantMissing(s, 23001, 600, 0); got != 0 {
		t.Fatalf("racePlantHarvestPlantMissing(lag FinishCnt)=%d, want 0", got)
	}
	policy := racePlantPolicy(false)
	result := BuildPlan(s, policy, now)
	if _, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001"); ok {
		t.Fatalf("FinishCnt lag must not top-up, demands=%+v", result.Demands)
	}
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.Executable && op.GoalID == raceActionGoal {
			t.Fatalf("must not top-up while FinishCnt lags harvest: %+v", op)
		}
	}
}

func TestRacePlantNoTopUpWhenLocalFinishCoversAfterLandsClear(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// Target 300, lvl11 → 12/plant → max 25 slots. After full harvest of 25
	// lands, lands are empty and FinishCnt still 0, but LocalFinishCnt=300 from
	// HarvestCnt deltas — must not replant another 25.
	s := raceTakenPlantState(t, 0, 300)
	applyMap(t, s, map[string]any{
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 11, "4": 2},
		}},
	})
	// Plant then harvest one round on 25 lands (LocalFinish rises, FinishCnt=0).
	lands := map[string]any{}
	for i := 0; i < 25; i++ {
		lands[itoa(1001+i)] = map[string]any{"0": 23001, "1": 2, "2": 11, "3": 0}
	}
	for i := 25; i < 64; i++ {
		lands[itoa(1001+i)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{"100": map[string]any{"1": lands}})
	harvested := map[string]any{}
	for i := 0; i < 25; i++ {
		harvested[itoa(1001+i)] = map[string]any{"0": 23001, "1": 2, "2": 11, "3": 4}
	}
	for i := 25; i < 64; i++ {
		harvested[itoa(1001+i)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{"100": map[string]any{"1": harvested}})
	// Clear all race lands (emptied after final harvest).
	applyMap(t, s, map[string]any{"100": map[string]any{"1": emptyLands(64)}})
	got := s.FmlRace()
	if got.LocalFinishCnt < 300 {
		t.Fatalf("LocalFinishCnt=%d, want >=300 after 25×4×3 harvests", got.LocalFinishCnt)
	}
	if got := racePlantHarvestPlantMissing(s, 23001, 300, got.LocalFinishCnt); got != 0 {
		t.Fatalf("racePlantHarvestPlantMissing after clear=%d, want 0", got)
	}
	policy := racePlantPolicy(false)
	result := BuildPlan(s, policy, now)
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.Executable && op.GoalID == raceActionGoal {
			t.Fatalf("must not replant after local progress covers target: %+v", op)
		}
	}
}

func TestRacePlantSlotCapBlocksCascadeTopUp(t *testing.T) {
	// 25 planted lands at lvl11 cover 25*12=300 == target — no top-up.
	s := raceTakenPlantState(t, 0, 300)
	lands := map[string]any{}
	for i := 0; i < 25; i++ {
		lands[itoa(1001+i)] = map[string]any{"0": 23001, "1": 1, "2": 11, "3": 0}
	}
	for i := 25; i < 64; i++ {
		lands[itoa(1001+i)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": lands},
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 11, "4": 2},
		}},
	})
	if got := racePlantHarvestPlantMissing(s, 23001, 300, 0); got != 0 {
		t.Fatalf("racePlantHarvestPlantMissing(25 planted)=%d, want 0", got)
	}
	// 24 planted covers only 288 — shortfall 12 → top-up 1.
	for i := 24; i < 25; i++ {
		lands[itoa(1001+i)] = map[string]any{}
	}
	applyMap(t, s, map[string]any{"100": map[string]any{"1": lands}})
	if got := racePlantHarvestPlantMissing(s, 23001, 300, 0); got != 1 {
		t.Fatalf("racePlantHarvestPlantMissing(24 planted)=%d, want 1", got)
	}
}

func TestRacePlantTopsUpWhileRaceHarvestReadyIfShort(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 0, 300)
	// One land ready to harvest (pending 12) leaves a large shortfall — harvest
	// and top-up plant in the same plan.
	lands := emptyLands(64)
	lands["1001"] = map[string]any{"0": 23001, "1": 3, "2": 11, "3": 0}
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": lands},
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 11, "4": 2},
		}},
	})
	policy := racePlantPolicy(false)
	result := BuildPlan(s, policy, now)
	var sawHarvest bool
	var plantCount int32
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCUsrLandHarvest.String() && op.Executable {
			sawHarvest = true
		}
		if isPlantOperation(op.Kind) && op.Executable && op.FlowerID == 23001 && op.GoalID == "union.race" {
			plantCount += int32(len(op.LandIDs))
		}
	}
	if !sawHarvest {
		t.Fatal("expected harvest op for ready race land")
	}
	// pending 12 → need 288 → ceil(288/12)=24
	if plantCount != 24 {
		t.Fatalf("planted %d lands, want 24 top-up while harvest-ready", plantCount)
	}
}

func TestRacePlantClaimsYieldSlotsThenAutoReplantsLeftover(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// Target 280 like 叶小楠; lvl 11 → 3×4=12 flowers/plant → ceil(280/12)=24.
	// Auto planting on: race takes 24, leftover empties go to 自主补种.
	s := raceTakenPlantState(t, 0, 280)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 50,
			"23002": 0,
		}}},
		"100": map[string]any{"1": emptyLands(64)},
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 11, "4": 2},
			"23002": map[string]any{"1": 23002, "2": 1, "4": 2},
		}},
	})
	policy := racePlantPolicy(false)

	result := BuildPlan(s, policy, now)
	demand, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001")
	if !ok {
		t.Fatalf("missing race demand, demands=%+v", result.Demands)
	}
	if demand.Missing != 24 {
		t.Fatalf("demand Missing=%d, want 24", demand.Missing)
	}
	var raceCount, fallbackCount, fallbackFlower int32
	for _, op := range result.Operations {
		if !isPlantOperation(op.Kind) || !op.Executable {
			continue
		}
		n := int32(len(op.LandIDs))
		switch op.GoalID {
		case raceActionGoal:
			if op.FlowerID != 23001 {
				t.Fatalf("race plant flower=%d, want 23001: %+v", op.FlowerID, op)
			}
			raceCount += n
		case GoalAutoReplant:
			fallbackCount += n
			fallbackFlower = op.FlowerID
		default:
			t.Fatalf("unexpected plant goal %q: %+v", op.GoalID, op)
		}
	}
	if raceCount != 24 {
		t.Fatalf("race planted %d lands, want 24", raceCount)
	}
	if fallbackCount != 40 {
		t.Fatalf("auto-replant leftover=%d, want 40 ops=%+v", fallbackCount, result.Operations)
	}
	if fallbackFlower != 23002 {
		t.Fatalf("leftover should prefer low-stock 23002, got flower=%d", fallbackFlower)
	}
}

func TestRacePlantLeavesLeftoverEmptyWhenAutoOff(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 0, 280)
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": emptyLands(64)},
		"101": map[string]any{"0": map[string]any{
			"23001": map[string]any{"1": 23001, "2": 11, "4": 2},
			"23002": map[string]any{"1": 23002, "2": 1, "4": 2},
		}},
	})
	policy := racePlantPolicy(false)
	policy.Plant.Planting.AutoEnabled = false
	policy.Plant.Planting.AutoHarvestEnabled = false

	result := BuildPlan(s, policy, now)
	var raceCount, otherCount int32
	for _, op := range result.Operations {
		if !isPlantOperation(op.Kind) || !op.Executable {
			continue
		}
		n := int32(len(op.LandIDs))
		if op.GoalID == raceActionGoal && op.FlowerID == 23001 {
			raceCount += n
			continue
		}
		otherCount += n
	}
	if raceCount != 24 {
		t.Fatalf("race planted %d lands, want 24", raceCount)
	}
	if otherCount != 0 {
		t.Fatalf("auto off must leave leftover empty, other=%d ops=%+v", otherCount, result.Operations)
	}
}

func TestRacePlantUsesFlowerLvlCfgWhenFlowerRowMissing(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// 梦紫郁金香 (23436): no per-flower c_flowerLvl yield rows. Without
	// c_flowerLvlCfg fallback, Missing collapses to remaining flowers and
	// fills all 64 lands as race demand.
	s := raceTakenPlantState(t, 0, 280)
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"111": map[string]any{"0": 1783872000000, "1": 1, "2": 1783990800000, "3": 1784466000000},
			"117": map[string]any{"5": 4},
			"110": map[string]any{
				"1783872000000": map[string]any{
					"7": map[string]any{"0": 99, "1": 4001, "2": 280, "3": 0, "4": []any{23436}},
				},
			},
		},
		"100": map[string]any{"1": emptyLands(64)},
		"101": map[string]any{"0": map[string]any{
			"23436": map[string]any{"1": 23436, "2": 11, "4": 2},
		}},
	})
	policy := racePlantPolicy(false)
	policy.Plant.Planting.AutoEnabled = false

	result := BuildPlan(s, policy, now)
	demand, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23436")
	if !ok {
		t.Fatalf("missing race demand, demands=%+v", result.Demands)
	}
	if demand.Missing != 24 {
		t.Fatalf("demand Missing=%d, want 24 via c_flowerLvlCfg", demand.Missing)
	}
	var plantCount int32
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.Executable {
			if op.FlowerID != 23436 || op.GoalID != raceActionGoal {
				t.Fatalf("unexpected plant while measuring race yield slots: %+v", op)
			}
			plantCount += int32(len(op.LandIDs))
		}
	}
	if plantCount != 24 {
		t.Fatalf("planted %d lands, want 24 race slots (not full 64)", plantCount)
	}
}

func TestRaceWatersPlantedFlowerWhenPlantingAutoOff(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// Planted race flower already covers remaining yield → no plant demand, but
	// lands are state=1 awaiting water. Race auto-complete must still water.
	s := raceTakenPlantState(t, 0, 10)
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0":  999,
			"32": map[string]any{"7": 50},
			"33": map[string]any{"7": map[string]any{"1": 130, "5": now.UnixMilli()}},
		}},
		"100": map[string]any{"1": map[string]any{
			// lvl1 yield 2/plant × 5 lands = 10, covers target; all need water.
			"1001": map[string]any{"0": 23001, "1": 1, "2": 1, "3": 0},
			"1002": map[string]any{"0": 23001, "1": 1, "2": 1, "3": 0},
			"1003": map[string]any{"0": 23001, "1": 1, "2": 1, "3": 0},
			"1004": map[string]any{"0": 23001, "1": 1, "2": 1, "3": 0},
			"1005": map[string]any{"0": 23001, "1": 1, "2": 1, "3": 0},
			"1006": map[string]any{"0": 23002, "1": 1, "2": 1, "3": 0}, // other flower
		}},
	})
	policy := racePlantPolicy(false)
	policy.Plant.Planting.AutoEnabled = false
	policy.Plant.Planting.AutoHarvestEnabled = false

	result := BuildPlan(s, policy, now)
	if _, ok := demandByID(result.Demands, "union.race:99:race_task:flower:23001"); ok {
		t.Fatalf("pending planted yield should clear plant demand, demands=%+v", result.Demands)
	}
	var water *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if (op.Kind == clientproto.RPCUsrLandWater.String() || op.Kind == clientproto.RPCUsrLandWaterBatch.String()) && op.Executable {
			water = op
			break
		}
	}
	if water == nil {
		t.Fatalf("expected race to drive watering, ops=%+v", result.Operations)
	}
	for _, id := range water.LandIDs {
		if id == 1006 {
			t.Fatalf("race-only water must not include non-race flower land 1006: %+v", water.LandIDs)
		}
	}
	if len(water.LandIDs) != 5 {
		t.Fatalf("water lands=%v, want 5 race flower lands", water.LandIDs)
	}
}

func TestRaceHarvestsWhenAutoHarvestOff(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := raceTakenPlantState(t, 0, 10)
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 3, "2": 1},
			"1002": map[string]any{"0": 23002, "1": 3, "2": 1}, // non-race ready
		}},
	})
	policy := racePlantPolicy(false)
	policy.Plant.Planting.AutoEnabled = false
	policy.Plant.Planting.AutoHarvestEnabled = false

	result := BuildPlan(s, policy, now)
	var harvest *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCUsrLandHarvest.String() && op.Executable {
			harvest = op
			break
		}
	}
	if harvest == nil {
		t.Fatalf("race must force-harvest competition flower when auto harvest is off, ops=%+v", result.Operations)
	}
	if len(harvest.LandIDs) != 1 || harvest.LandIDs[0] != 1001 {
		t.Fatalf("race-only harvest LandIDs=%v, want [1001]", harvest.LandIDs)
	}
}

func TestRaceFlowerIgnoresHarvestDelay(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	s := raceTakenPlantState(t, 0, 10)
	// Race flower matured 10s ago; non-race flower same. Configured delay=30s.
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 3, "2": 1, "7": now.Add(-10 * time.Second).UnixMilli()},
			"1002": map[string]any{"0": 23002, "1": 3, "2": 1, "7": now.Add(-10 * time.Second).UnixMilli()},
		}},
	})
	policy := racePlantPolicy(false)
	policy.Plant.Planting.AutoHarvestEnabled = true
	policy.Plant.Planting.HarvestDelaySeconds = 30

	result := BuildPlan(s, policy, now)
	var harvest *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCUsrLandHarvest.String() && op.Executable {
			harvest = op
			break
		}
	}
	if harvest == nil {
		t.Fatalf("race flower must harvest immediately despite delay, ops=%+v", result.Operations)
	}
	for _, id := range harvest.LandIDs {
		if id == 1002 {
			t.Fatalf("non-race flower must still respect harvest delay, got LandIDs=%v", harvest.LandIDs)
		}
	}
	if len(harvest.LandIDs) != 1 || harvest.LandIDs[0] != 1001 {
		t.Fatalf("LandIDs=%v, want only race land [1001]", harvest.LandIDs)
	}
}
