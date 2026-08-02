package automation

import (
	"reflect"
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestCyclicNoteActionDemandUsesMaxRemainingAndServerProgressOnly(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{1003, 2003, nil}, map[string]any{
		"1003": 79,
		"2003": 20,
	}, map[string]any{}, 0, []any{})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})

	first := BuildPlan(s, policy, now)
	demand := requireCyclicNoteActionDemand(t, first, cyclicNoteTaskTypePlantAny)
	if demand.ID != "activity.cyclicNote:9001:3001" || demand.Kind != DemandKindAction || demand.ItemID != 0 ||
		demand.Count != 80 || demand.Have != 20 || demand.Available != 20 || demand.Missing != 60 ||
		demand.Priority != 50 || demand.Category != CategoryActivity {
		t.Fatalf("max-remaining action demand=%+v", demand)
	}
	if got := cyclicNoteActionDemands(first); len(got) != 1 {
		t.Fatalf("same task type was summed or duplicated: %+v", got)
	}

	second := BuildPlan(s, policy, now)
	if got := requireCyclicNoteActionDemand(t, second, cyclicNoteTaskTypePlantAny); !reflect.DeepEqual(got, demand) {
		t.Fatalf("planning without delta changed demand: first=%+v second=%+v", demand, got)
	}

	// A business-state update must not optimistically advance namespace 23.
	applyMap(t, s, map[string]any{"100": map[string]any{"1": map[string]any{"1001": map[string]any{"0": 23001, "1": 1}}}})
	third := BuildPlan(s, policy, now)
	if got := requireCyclicNoteActionDemand(t, third, cyclicNoteTaskTypePlantAny); !reflect.DeepEqual(got, demand) {
		t.Fatalf("business delta changed server-owned task progress: before=%+v after=%+v", demand, got)
	}

	// Only the authoritative task-record replacement changes remaining.
	applyMap(t, s, map[string]any{"23": map[string]any{"3": map[string]any{"9001|0": map[string]any{
		"3": map[string]any{"1003": 79, "2003": 30}, "5": map[string]any{},
	}}}})
	fourth := BuildPlan(s, policy, now)
	updated := requireCyclicNoteActionDemand(t, fourth, cyclicNoteTaskTypePlantAny)
	if updated.Have != 30 || updated.Missing != 50 || updated.Count != 80 {
		t.Fatalf("authoritative progress not reflected: %+v", updated)
	}
}

func TestCyclicNoteActionDemandRequiresCorrespondingEnabledModule(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	tests := []struct {
		name     string
		taskID   int32
		taskType int32
		enable   func(*pb.Policy)
		disable  func(*pb.Policy)
	}{
		{
			name: "plant", taskID: 4003, taskType: cyclicNoteTaskTypePlantAny,
			enable:  func(p *pb.Policy) { p.Plant.Planting.AutoEnabled = true },
			disable: func(p *pb.Policy) { p.Plant.Planting.AutoEnabled = false },
		},
		{
			name: "rack", taskID: 2001, taskType: cyclicNoteTaskTypeFlowerRack,
			enable:  func(p *pb.Policy) { p.Order.FlowerArt.SellEnabled = true },
			disable: func(p *pb.Policy) { p.Order.FlowerArt.SellEnabled = false },
		},
		{
			name: "customer", taskID: 2007, taskType: cyclicNoteTaskTypeCustomerOrder,
			enable:  func(p *pb.Policy) { p.Order.Customer.Enabled = true },
			disable: func(p *pb.Policy) { p.Order.Customer.Enabled = false },
		},
		{
			name: "resident", taskID: 1005, taskType: cyclicNoteTaskTypeResidentOrder,
			enable:  func(p *pb.Policy) { p.Order.Resident.NormalEnabled = true },
			disable: func(p *pb.Policy) { p.Order.Resident.NormalEnabled = false },
		},
		{
			name: "pearl", taskID: 1006, taskType: cyclicNoteTaskTypePearlHire,
			enable:  func(p *pb.Policy) { p.Basic.Pearl.AutoHireEnabled = true; p.Basic.Pearl.MaxHireTicketUsage = 1 },
			disable: func(p *pb.Policy) { p.Basic.Pearl.AutoHireEnabled = false },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := cyclicNotePlannerState(t, now, 2, []any{tc.taskID, nil, nil}, map[string]any{itoa32(tc.taskID): 0}, map[string]any{}, 0, []any{})
			policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
			tc.enable(policy)
			if _, ok := findCyclicNoteActionDemand(BuildPlan(s, policy, now), tc.taskType); !ok {
				t.Fatal("enabled business module did not expose action demand")
			}
			tc.disable(policy)
			result := BuildPlan(s, policy, now)
			if _, ok := findCyclicNoteActionDemand(result, tc.taskType); ok {
				t.Fatalf("disabled business module retained managed action demand: %+v", result.Demands)
			}
			if ops := cyclicNoteDrivenBusinessOps(result.Operations); len(ops) != 0 {
				t.Fatalf("disabled business module was driven: %+v", ops)
			}
		})
	}
}

func TestCyclicNoteActionDemandRequiresAllActivityGates(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{4003, nil, nil}, map[string]any{"4003": 1}, map[string]any{}, 0, []any{})
	tests := []struct {
		name   string
		mutate func(*pb.Policy)
	}{
		{name: "global automation", mutate: func(p *pb.Policy) { p.AutomationEnabled = false }},
		{name: "activity", mutate: func(p *pb.Policy) { p.Activity.Enabled = false }},
		{name: "cyclic note module", mutate: func(p *pb.Policy) { p.Activity.Modules[cyclicNoteModuleKey].Enabled = false }},
		{name: "satisfy tasks", mutate: func(p *pb.Policy) {
			p.Activity.Modules[cyclicNoteModuleKey].BoolParams[cyclicNoteSatisfyTasksKey] = false
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
			tc.mutate(policy)
			if got := cyclicNoteActionDemands(BuildPlan(s, policy, now)); len(got) != 0 {
				t.Fatalf("disabled %s gate produced action demand: %+v", tc.name, got)
			}
		})
	}
}

func TestCyclicNoteActionDemandFailsClosedOnPhaseAndTaskState(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})

	tests := []struct {
		name     string
		phase    int32
		tasks    []any
		progress map[string]any
		receipts map[string]any
	}{
		{name: "grace phase", phase: 3, tasks: []any{4003, nil, nil}, progress: map[string]any{"4003": 1}, receipts: map[string]any{}},
		{name: "unknown catalog task", phase: 2, tasks: []any{999999, nil, nil}, progress: map[string]any{"999999": 1}, receipts: map[string]any{}},
		{name: "unsupported known type", phase: 2, tasks: []any{1002, nil, nil}, progress: map[string]any{"1002": 1}, receipts: map[string]any{}},
		{name: "completed", phase: 2, tasks: []any{4003, nil, nil}, progress: map[string]any{"4003": 80}, receipts: map[string]any{}},
		{name: "received", phase: 2, tasks: []any{4003, nil, nil}, progress: map[string]any{"4003": 1}, receipts: map[string]any{"4003": 1}},
		{name: "negative progress", phase: 2, tasks: []any{4003, nil, nil}, progress: map[string]any{"4003": -1}, receipts: map[string]any{}},
		{name: "task list unobserved", phase: 2, progress: map[string]any{"4003": 1}, receipts: map[string]any{}},
		{name: "task record unobserved", phase: 2, tasks: []any{4003, nil, nil}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := cyclicNotePlannerState(t, now, tc.phase, tc.tasks, tc.progress, tc.receipts, 0, []any{})
			if got := cyclicNoteActionDemands(BuildPlan(s, policy, now)); len(got) != 0 {
				t.Fatalf("unsafe task state produced action demand: %+v", got)
			}
		})
	}
}

func TestCyclicNoteAnyPlantCapsAndRebuildsAutoReplant(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{4003, nil, nil}, map[string]any{"4003": 78}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": emptyLands(3)},
		"101": map[string]any{"0": cultivate(23001)},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})

	first := BuildPlan(s, policy, now)
	op := requireSingleCyclicNoteBusinessOp(t, first.Operations, cyclicNoteTaskTypePlantAny)
	if op.Kind != clientproto.RPCUsrLandPlantBatch.String() || !reflect.DeepEqual(op.LandIDs, []int32{1001, 1002}) ||
		op.GoalID != GoalAutoReplant || op.Priority != cyclicNotePlantOpFloor ||
		op.OperationID != "usrLand.plantBatch|flower=23001|lands=1001,1002" || operationLaneRank(op) != laneRank(LaneSide) {
		t.Fatalf("activity plant op=%+v laneRank=%d", op, operationLaneRank(op))
	}
	assertNoCyclicNoteWireTargets(t, op)

	second := BuildPlan(s, policy, now)
	again := requireSingleCyclicNoteBusinessOp(t, second.Operations, cyclicNoteTaskTypePlantAny)
	if !reflect.DeepEqual(op, again) {
		t.Fatalf("repeat planning changed activity plant without delta:\nfirst=%+v\nsecond=%+v", op, again)
	}
}

func TestCyclicNoteSpecificFlowerDemandPreventsPureActivityReplant(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{4003, nil, nil}, map[string]any{"4003": 0}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 0}}},
		"100": map[string]any{"1": emptyLands(2)},
		"101": map[string]any{"0": cultivate(23005)},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": [][]int32{{23005, 1}}, "1": 7},
		}}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Plant.Planting.DemandPriorityEnabled = true
	policy.Order.Customer.Enabled = true

	result := BuildPlan(s, policy, now)
	var specific bool
	for _, op := range result.Operations {
		if isPlantOperation(op.Kind) && op.GoalID == GoalCustomerOrder && op.DemandID != "" {
			specific = true
			if strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") {
				t.Fatalf("specific flower operation was overwritten by activity: %+v", op)
			}
		}
		if isPlantOperation(op.Kind) && strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") {
			t.Fatalf("activity created a second plant operation beside concrete demand: %+v", op)
		}
	}
	if !specific {
		t.Fatalf("missing concrete customer flower plant: ops=%+v demands=%+v", result.Operations, result.Demands)
	}
}

func TestCyclicNoteActivityPlantRanksBelowReadyCustomerOrder(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{4003, nil, nil}, map[string]any{"4003": 1}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"23005": 5}}},
		"100": map[string]any{"1": emptyLands(2)},
		"101": map[string]any{"0": cultivate(23005)},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": [][]int32{{23005, 1}}, "1": 7},
		}}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.Customer.Enabled = true

	op := Plan(s, policy, now)
	if op == nil || op.Kind != clientproto.RPCOrderCustomerFinishOrder.String() || op.TargetID != 7 {
		t.Fatalf("Plan()=%+v, ready major order must precede pure activity replant", op)
	}
}

func TestCyclicNoteFlowerRackUsesOnlyUnreservedInventoryAndExactCost(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 133}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"300208": 5}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 300208, "2": 4, "3": 1},
		}}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.Customer.Enabled = true
	policy.Order.FlowerArt.SellEnabled = true

	result := BuildPlan(s, policy, now)
	op := requireSingleCyclicNoteBusinessOp(t, result.Operations, cyclicNoteTaskTypeFlowerRack)
	if op.Kind != clientproto.RPCFlowerRackSell.String() || op.ItemID != 300208 || op.Count != 1 ||
		op.Priority != cyclicNoteRackOpFloor || len(op.ItemCost) != 1 || op.ItemCost[300208] != 1 ||
		result.Ledger.Available(300208) != 1 {
		t.Fatalf("activity rack op=%+v ledger available=%d", op, result.Ledger.Available(300208))
	}
	assertNoCyclicNoteWireTargets(t, op)
}

func TestCyclicNoteFlowerRackCapsToActivityRemaining(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 132}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"300208": 20}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = true

	result := BuildPlan(s, policy, now)
	op := requireSingleCyclicNoteBusinessOp(t, result.Operations, cyclicNoteTaskTypeFlowerRack)
	if op.Count != 3 || op.ItemCost[300208] != 3 {
		t.Fatalf("rack op did not cap to remaining=3: %+v", op)
	}
}

func TestCyclicNoteFlowerRackNeverPromotesCraft(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 130}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"23005": 4, "23007": 4, "23008": 4}, "34": 12,
		}},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3002": map[string]any{"1": 3002}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = true
	policy.Order.FlowerArt.CraftEnabled = true

	result := BuildPlan(s, policy, now)
	foundCraft := false
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() {
			foundCraft = true
			if strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") || op.Priority >= cyclicNoteRackOpFloor {
				t.Fatalf("activity task promoted flower-art craft: %+v", op)
			}
		}
	}
	if !foundCraft {
		t.Fatalf("fixture did not produce ordinary rack craft: %+v", result.Operations)
	}
}

func TestCyclicNoteOrdersLinkOneDeterministicSafeFinish(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{2007, 1005, nil}, map[string]any{"2007": 20, "1005": 60}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"23005": 10}}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"2":  map[string]any{"0": [][]int32{{23005, 1}}, "1": 2},
			"10": map[string]any{"0": [][]int32{{23005, 1}}, "1": 10},
		}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{
			"2":  map[string]any{"0": 2, "2": [][]int32{{23005, 1}}},
			"10": map[string]any{"0": 10, "2": [][]int32{{23005, 1}}},
		}}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.Customer.Enabled = true
	policy.Order.Resident.NormalEnabled = true

	result := BuildPlan(s, policy, now)
	linked := cyclicNoteDrivenBusinessOps(result.Operations)
	if len(linked) != 2 {
		t.Fatalf("linked business operations=%+v", linked)
	}
	seen := map[string]PlannedOp{}
	for _, op := range linked {
		seen[op.Kind] = op
		assertNoCyclicNoteWireTargets(t, op)
	}
	if op := seen[clientproto.RPCOrderCustomerFinishOrder.String()]; op.TargetID != 10 || op.Priority != 11290 {
		t.Fatalf("customer link not deterministic/safe: %+v", op)
	}
	if op := seen[clientproto.RPCOrderFlowerFinishOrder.String()]; op.TargetID != 10 || op.Priority != 8700 {
		t.Fatalf("resident link not deterministic/safe: %+v", op)
	}
}

func TestCyclicNoteCustomerTaskDoesNotPromoteGenerateOrReject(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{2007, nil, nil}, map[string]any{"2007": 1}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{"109": map[string]any{"0": map[string]any{
		"1": map[string]any{}, "2": now.Add(-time.Second).UnixMilli(),
	}}})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.Customer.Enabled = true

	result := BuildPlan(s, policy, now)
	for _, op := range result.Operations {
		if (op.Kind == clientproto.RPCOrderCustomerGenOrder.String() || op.Kind == clientproto.RPCOrderCustomerRejectOrder.String()) &&
			strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") {
			t.Fatalf("non-finish customer operation was promoted: %+v", op)
		}
	}
	if ops := cyclicNoteDrivenBusinessOps(result.Operations); len(ops) != 0 {
		t.Fatalf("customer task linked without a finishable order: %+v", ops)
	}
}

func TestCyclicNotePearlHireReusesSafePlannerWithExactTicketCost(t *testing.T) {
	now := time.Now().Add(2 * time.Second)
	s := cyclicNotePlannerState(t, now, 2, []any{1006, nil, nil}, map[string]any{"1006": 0}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"0": int64(9001), "32": map[string]any{"1003": 3}}},
		"115": map[string]any{
			"0": map[string]any{"1": map[string]any{"2": int64(0), "3": nil, "4": 0, "9": int64(1)}},
			"1": map[string]any{"5": map[string]any{}},
		},
		"24": map[string]any{
			"0": map[string]any{"0": int64(9001)},
			"1": []any{map[string]any{"0": int64(9001), "1": int64(2001)}},
		},
		"28": map[string]any{"5": []any{map[string]any{"0": int64(2001), "1": "safe", "4": 12}}},
	})
	applyMap(t, s, map[string]any{"115": map[string]any{"5": map[string]any{"2001": int64(0)}}})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Basic.Pearl.AutoHireEnabled = true
	policy.Basic.Pearl.MaxHireTicketUsage = 2

	result := BuildPlan(s, policy, now)
	op := requireSingleCyclicNoteBusinessOp(t, result.Operations, cyclicNoteTaskTypePearlHire)
	if op.Kind != clientproto.RPCPearlPlaceHire.String() || op.TargetID != 1 || op.TargetUID != 2001 || op.Count != 1 ||
		op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 1 || op.ItemCost[1003] != 1 {
		t.Fatalf("activity pearl hire=%+v", op)
	}
	assertNoCyclicNoteWireTargets(t, op)
	if err := ValidateSafePearlHire(s, policy.Basic.Pearl, &op, now); err != nil {
		t.Fatalf("linked activity hire no longer passes safe helper: %v", err)
	}
	count := 0
	for _, candidate := range result.Operations {
		if candidate.FeatureID == "basic.pearl_hire" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("activity duplicated pearl planner operation: %+v", result.Operations)
	}

	policy.Basic.Pearl.MaxHireTicketUsage = 0
	blocked := BuildPlan(s, policy, now)
	for _, candidate := range blocked.Operations {
		if candidate.Kind == clientproto.RPCPearlPlaceHire.String() && candidate.Executable {
			t.Fatalf("activity bypassed max_hire_ticket_usage=0: %+v", candidate)
		}
	}
	blockedOp := requireSingleCyclicNoteBusinessOp(t, blocked.Operations, cyclicNoteTaskTypePearlHire)
	if blockedOp.Executable || blockedOp.Status != PlanStatusBlocked || len(blockedOp.BlockedReasons) == 0 {
		t.Fatalf("activity did not preserve safe-hire blocked result: %+v", blockedOp)
	}
}

func cyclicNoteActionDemands(result PlanResult) []Demand {
	var out []Demand
	for _, demand := range result.Demands {
		if demand.Kind == DemandKindAction && demand.Domain == cyclicNoteActionGoal {
			out = append(out, demand)
		}
	}
	return out
}

func findCyclicNoteActionDemand(result PlanResult, taskType int32) (Demand, bool) {
	suffix := ":" + itoa32(taskType)
	for _, demand := range cyclicNoteActionDemands(result) {
		if strings.HasSuffix(demand.ID, suffix) {
			return demand, true
		}
	}
	return Demand{}, false
}

func requireCyclicNoteActionDemand(t *testing.T, result PlanResult, taskType int32) Demand {
	t.Helper()
	demand, ok := findCyclicNoteActionDemand(result, taskType)
	if !ok {
		t.Fatalf("missing cyclic-note action demand type %d: %+v", taskType, result.Demands)
	}
	return demand
}

func cyclicNoteDrivenBusinessOps(ops []PlannedOp) []PlannedOp {
	var out []PlannedOp
	for _, op := range ops {
		if strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") {
			out = append(out, op)
		}
	}
	return out
}

func requireSingleCyclicNoteBusinessOp(t *testing.T, ops []PlannedOp, taskType int32) PlannedOp {
	t.Helper()
	suffix := ":" + itoa32(taskType)
	var found []PlannedOp
	for _, op := range cyclicNoteDrivenBusinessOps(ops) {
		if strings.HasSuffix(op.DemandID, suffix) {
			found = append(found, op)
		}
	}
	if len(found) != 1 {
		t.Fatalf("cyclic-note business ops type %d=%+v", taskType, found)
	}
	return found[0]
}

func assertNoCyclicNoteWireTargets(t *testing.T, op PlannedOp) {
	t.Helper()
	if op.BatchID != 0 || op.SlotID != 0 || op.TaskID != 0 || op.MilestoneIndex != 0 {
		t.Fatalf("business operation leaked activity wire targets: %+v", op)
	}
}
