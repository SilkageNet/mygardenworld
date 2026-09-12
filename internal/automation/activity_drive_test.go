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
		// selfDriven tasks stay active under satisfy_tasks without the ordinary
		// business-module toggle (plant/rack; plant flower filters still apply).
		selfDriven bool
	}{
		{
			name: "plant", taskID: 4003, taskType: cyclicNoteTaskTypePlantAny, selfDriven: true,
			enable:  func(p *pb.Policy) { p.Plant.Planting.AutoEnabled = true },
			disable: func(p *pb.Policy) { p.Plant.Planting.AutoEnabled = false },
		},
		{
			name: "rack", taskID: 2001, taskType: cyclicNoteTaskTypeFlowerRack, selfDriven: true,
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
		{
			name: "resident via activity switch", taskID: 1005, taskType: cyclicNoteTaskTypeResidentOrder,
			enable: func(p *pb.Policy) {
				p.Order.Resident.NormalEnabled = false
				p.Activity.Modules[cyclicNoteModuleKey].BoolParams[cyclicNoteAutoCompleteResidentOrdersKey] = true
			},
			disable: func(p *pb.Policy) {
				p.Activity.Modules[cyclicNoteModuleKey].BoolParams[cyclicNoteAutoCompleteResidentOrdersKey] = false
			},
		},
		{
			name: "pearl via activity switch", taskID: 1006, taskType: cyclicNoteTaskTypePearlHire,
			enable: func(p *pb.Policy) {
				p.Basic.Pearl.AutoHireEnabled = false
				p.Basic.Pearl.MaxHireTicketUsage = 1
				p.Activity.Modules[cyclicNoteModuleKey].BoolParams[cyclicNoteAutoHireKey] = true
			},
			disable: func(p *pb.Policy) {
				p.Activity.Modules[cyclicNoteModuleKey].BoolParams[cyclicNoteAutoHireKey] = false
			},
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
			_, ok := findCyclicNoteActionDemand(result, tc.taskType)
			if tc.selfDriven {
				if !ok {
					t.Fatalf("self-driven task lost action demand with module off: %+v", result.Demands)
				}
				return
			}
			if ok {
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
		{name: "cyclic note module", mutate: func(p *pb.Policy) { p.Activity.Modules[cyclicNoteModuleKey].Enabled = false }},
		{name: "satisfy tasks", mutate: func(p *pb.Policy) {
			p.Activity.Modules[cyclicNoteModuleKey].BoolParams[cyclicNoteSatisfyTasksKey] = false
		}},
		{name: "auto plant any", mutate: func(p *pb.Policy) {
			p.Activity.Modules[cyclicNoteModuleKey].BoolParams[cyclicNoteAutoPlantAnyKey] = false
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

	rack := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 1}, map[string]any{}, 0, []any{})
	t.Run("auto sell flower art", func(t *testing.T) {
		policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
		policy.Activity.Modules[cyclicNoteModuleKey].BoolParams[cyclicNoteAutoSellFlowerArtKey] = false
		if got := cyclicNoteActionDemands(BuildPlan(rack, policy, now)); len(got) != 0 {
			t.Fatalf("disabled auto sell flower art gate produced action demand: %+v", got)
		}
	})
}

func TestCyclicNoteAutoPlantAnyDefaultsOnAndCanDisable(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{4003, nil, nil}, map[string]any{"4003": 0}, map[string]any{}, 0, []any{})

	// Missing auto_plant_any key keeps historical default: drive plant-any.
	absent := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	if _, ok := findCyclicNoteActionDemand(BuildPlan(s, absent, now), cyclicNoteTaskTypePlantAny); !ok {
		t.Fatal("missing auto_plant_any should still expose plant-any demand")
	}

	explicitOn := cyclicNotePlannerPolicy(true, true, map[string]bool{
		cyclicNoteSatisfyTasksKey: true,
		cyclicNoteAutoPlantAnyKey: true,
	})
	if _, ok := findCyclicNoteActionDemand(BuildPlan(s, explicitOn, now), cyclicNoteTaskTypePlantAny); !ok {
		t.Fatal("explicit auto_plant_any=true should expose plant-any demand")
	}

	off := cyclicNotePlannerPolicy(true, true, map[string]bool{
		cyclicNoteSatisfyTasksKey: true,
		cyclicNoteAutoPlantAnyKey: false,
	})
	result := BuildPlan(s, off, now)
	if _, ok := findCyclicNoteActionDemand(result, cyclicNoteTaskTypePlantAny); ok {
		t.Fatalf("auto_plant_any=false retained plant-any demand: %+v", result.Demands)
	}
	if ops := cyclicNoteDrivenBusinessOps(result.Operations); len(ops) != 0 {
		t.Fatalf("auto_plant_any=false still drove business ops: %+v", ops)
	}

	// Disabling plant-any must not block other self-driven tasks under satisfy_tasks.
	mixed := cyclicNotePlannerState(t, now, 2, []any{4003, 2001, nil}, map[string]any{"4003": 0, "2001": 0}, map[string]any{}, 0, []any{})
	mixedPolicy := cyclicNotePlannerPolicy(true, true, map[string]bool{
		cyclicNoteSatisfyTasksKey: true,
		cyclicNoteAutoPlantAnyKey: false,
	})
	mixedResult := BuildPlan(mixed, mixedPolicy, now)
	if _, ok := findCyclicNoteActionDemand(mixedResult, cyclicNoteTaskTypePlantAny); ok {
		t.Fatal("auto_plant_any=false should hide plant-any even with rack task present")
	}
	if _, ok := findCyclicNoteActionDemand(mixedResult, cyclicNoteTaskTypeFlowerRack); !ok {
		t.Fatal("auto_plant_any=false must not suppress flower-rack demand")
	}
}

func TestCyclicNoteAutoSellFlowerArtDefaultsOnAndCanDisable(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 0}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"300208": 5}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})

	// Missing auto_sell_flower_art key keeps historical default: drive flower-rack.
	absent := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	if _, ok := findCyclicNoteActionDemand(BuildPlan(s, absent, now), cyclicNoteTaskTypeFlowerRack); !ok {
		t.Fatal("missing auto_sell_flower_art should still expose flower-rack demand")
	}

	explicitOn := cyclicNotePlannerPolicy(true, true, map[string]bool{
		cyclicNoteSatisfyTasksKey:      true,
		cyclicNoteAutoSellFlowerArtKey: true,
	})
	if _, ok := findCyclicNoteActionDemand(BuildPlan(s, explicitOn, now), cyclicNoteTaskTypeFlowerRack); !ok {
		t.Fatal("explicit auto_sell_flower_art=true should expose flower-rack demand")
	}

	off := cyclicNotePlannerPolicy(true, true, map[string]bool{
		cyclicNoteSatisfyTasksKey:      true,
		cyclicNoteAutoSellFlowerArtKey: false,
	})
	result := BuildPlan(s, off, now)
	if _, ok := findCyclicNoteActionDemand(result, cyclicNoteTaskTypeFlowerRack); ok {
		t.Fatalf("auto_sell_flower_art=false retained flower-rack demand: %+v", result.Demands)
	}
	if ops := cyclicNoteDrivenBusinessOps(result.Operations); len(ops) != 0 {
		t.Fatalf("auto_sell_flower_art=false still drove business ops: %+v", ops)
	}

	// Disabling flower-rack must not block other self-driven tasks under satisfy_tasks.
	mixed := cyclicNotePlannerState(t, now, 2, []any{4003, 2001, nil}, map[string]any{"4003": 0, "2001": 0}, map[string]any{}, 0, []any{})
	mixedPolicy := cyclicNotePlannerPolicy(true, true, map[string]bool{
		cyclicNoteSatisfyTasksKey:      true,
		cyclicNoteAutoSellFlowerArtKey: false,
	})
	mixedResult := BuildPlan(mixed, mixedPolicy, now)
	if _, ok := findCyclicNoteActionDemand(mixedResult, cyclicNoteTaskTypeFlowerRack); ok {
		t.Fatal("auto_sell_flower_art=false should hide flower-rack even with plant task present")
	}
	if _, ok := findCyclicNoteActionDemand(mixedResult, cyclicNoteTaskTypePlantAny); !ok {
		t.Fatal("auto_sell_flower_art=false must not suppress plant-any demand")
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
	policy.Plant.Planting.AutoEnabled = true

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
	policy.Union.Race.Enabled = false

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
		op.Priority < cyclicNoteRackOpFloor || len(op.ItemCost) != 1 || op.ItemCost[300208] != 1 ||
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
	// Finished art already in stock — activity must list, not craft.
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 130}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"300208": 4, "23005": 4, "23007": 4, "23008": 4}, "34": 12,
		}},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3002": map[string]any{"1": 3002}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false
	policy.Order.FlowerArt.CraftEnabled = false

	result := BuildPlan(s, policy, now)
	op := requireSingleCyclicNoteBusinessOp(t, result.Operations, cyclicNoteTaskTypeFlowerRack)
	if op.Kind != clientproto.RPCFlowerRackSell.String() {
		t.Fatalf("with finished stock, activity should sell not craft: %+v", op)
	}
	for _, candidate := range result.Operations {
		if candidate.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() &&
			strings.HasPrefix(candidate.DemandID, cyclicNoteActionGoal+":") {
			t.Fatalf("activity promoted craft while finished art is in stock: %+v", candidate)
		}
	}
}

func TestCyclicNoteFlowerRackEmitsWithoutSellEnabled(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 130}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"300208": 4}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false

	result := BuildPlan(s, policy, now)
	op := requireSingleCyclicNoteBusinessOp(t, result.Operations, cyclicNoteTaskTypeFlowerRack)
	if op.Kind != clientproto.RPCFlowerRackSell.String() || op.ItemID != 300208 || !op.Executable {
		t.Fatalf("expected self-driven flower-rack sell: %+v", op)
	}
}

func TestCyclicNoteFlowerRackCancelsAfterFiveMinutes(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	listedAt := now.Add(-cyclicNoteFlowerRackRelistAfter - time.Second).UnixMilli()
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 130}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"300208": 4}}},
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 2, "4": listedAt},
			"2": map[string]any{"1": 2, "2": 300208, "3": 2, "4": listedAt},
			"3": map[string]any{"1": 3, "2": 0, "3": 0},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false

	result := BuildPlan(s, policy, now)
	var cancels []PlannedOp
	for _, candidate := range result.Operations {
		if candidate.Kind == clientproto.RPCFlowerRackCancelSell.String() &&
			strings.HasPrefix(candidate.DemandID, cyclicNoteActionGoal+":") {
			cancels = append(cancels, candidate)
		}
	}
	if len(cancels) != 2 {
		t.Fatalf("expected cancel of both stale racks, got %d ops=%+v", len(cancels), result.Operations)
	}
	for _, cancel := range cancels {
		if !cancel.Executable || cancel.Priority != cyclicNoteRackCancelFloor ||
			cancel.FeatureID != "activity.cyclicNote.flower_art_cancel" {
			t.Fatalf("cancel not executable at cyclic-note priority: %+v", cancel)
		}
		if !strings.Contains(cancel.Reason, "5分钟") {
			t.Fatalf("cancel reason = %q", cancel.Reason)
		}
	}
}

func TestCyclicNoteFlowerRackCancelsAfterSevenMinutesWhenEscalated(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	listedAt := now.Add(-cyclicNoteFlowerRackRelistSlowAfter - time.Second).UnixMilli()
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 130}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"300208": 4}}},
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 2, "4": listedAt},
			"2": map[string]any{"1": 2, "2": 0, "3": 0},
		}},
	})
	s.MarkCyclicNoteFlowerRackRelistSlow()
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false

	result := BuildPlan(s, policy, now)
	op := requireSingleCyclicNoteBusinessOp(t, result.Operations, cyclicNoteTaskTypeFlowerRack)
	if op.Kind != clientproto.RPCFlowerRackCancelSell.String() || op.TargetID != 1 {
		t.Fatalf("expected 7-minute escalated cancel: %+v", op)
	}
	if !strings.Contains(op.Reason, "7分钟") {
		t.Fatalf("cancel reason = %q, want 7-minute threshold", op.Reason)
	}
}

func TestCyclicNoteFlowerRackPostCompleteCancelsAllAfterSevenMinutes(t *testing.T) {
	completeAt := time.UnixMilli(1_700_000)
	// Target for catalog task 2001 is 135 — progress 135 means 花艺上架 done.
	s := cyclicNotePlannerState(t, completeAt, 2, []any{2001, nil, nil}, map[string]any{"2001": 135}, map[string]any{}, 0, []any{})
	listedAt := completeAt.Add(-time.Minute).UnixMilli() // freshly listed relative to 7-minute wait
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"300208": 4}}},
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 2, "4": listedAt},
			"2": map[string]any{"1": 2, "2": 300208, "3": 3, "4": listedAt},
			"3": map[string]any{"1": 3, "2": 0, "3": 0},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false

	// First observation arms the timer; shelves stay listed.
	if _, ok := findCyclicNoteActionDemand(BuildPlan(s, policy, completeAt), cyclicNoteTaskTypeFlowerRack); ok {
		t.Fatal("completed flower-rack must not expose unfinished demand")
	}
	early := BuildPlan(s, policy, completeAt.Add(6*time.Minute))
	for _, op := range early.Operations {
		if op.Kind == clientproto.RPCFlowerRackCancelSell.String() &&
			strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") {
			t.Fatalf("must not cancel before 7 minutes after complete: %+v", op)
		}
	}

	due := completeAt.Add(cyclicNoteFlowerRackRelistSlowAfter + time.Second)
	result := BuildPlan(s, policy, due)
	var cancels []PlannedOp
	for _, candidate := range result.Operations {
		if candidate.Kind == clientproto.RPCFlowerRackCancelSell.String() &&
			strings.HasPrefix(candidate.DemandID, cyclicNoteActionGoal+":") {
			cancels = append(cancels, candidate)
		}
	}
	if len(cancels) != 2 {
		t.Fatalf("expected cancel of all occupied racks after post-complete 7 minutes, got %d ops=%+v",
			len(cancels), result.Operations)
	}
	for _, cancel := range cancels {
		if !cancel.Executable || cancel.Priority != cyclicNoteRackCancelFloor ||
			cancel.FeatureID != "activity.cyclicNote.flower_art_post_complete_cancel" {
			t.Fatalf("post-complete cancel malformed: %+v", cancel)
		}
		if !strings.Contains(cancel.Reason, "已完成") || !strings.Contains(cancel.Reason, "7分钟") {
			t.Fatalf("cancel reason = %q", cancel.Reason)
		}
	}

	// Empty shelves mark the one-shot clear done; sell_enabled may list again.
	applyMap(t, s, map[string]any{
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 0, "3": 0},
			"2": map[string]any{"1": 2, "2": 0, "3": 0},
		}},
	})
	cleared := BuildPlan(s, policy, due.Add(time.Minute))
	for _, op := range cleared.Operations {
		if op.Kind == clientproto.RPCFlowerRackCancelSell.String() &&
			strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") {
			t.Fatalf("must not keep canceling after post-complete clear: %+v", op)
		}
	}
	if !s.CyclicNoteFlowerRackPostCompleteCancelDone(9001) {
		t.Fatal("expected post-complete cancel marked done after empty shelves")
	}
}

func TestCyclicNoteFlowerRackPostCompleteSurvivesTaskClaimReplacement(t *testing.T) {
	completeAt := time.UnixMilli(1_700_000)
	s := cyclicNotePlannerState(t, completeAt, 2, []any{2001, nil, nil}, map[string]any{"2001": 135}, map[string]any{}, 0, []any{})
	listedAt := completeAt.UnixMilli()
	applyMap(t, s, map[string]any{
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 2, "4": listedAt},
			"2": map[string]any{"1": 2, "2": 300208, "3": 3, "4": listedAt},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false

	_ = BuildPlan(s, policy, completeAt) // arm while 3015 still visible
	if _, armed := s.CyclicNoteFlowerRackPostCompleteArmed(); !armed {
		t.Fatal("expected post-complete timer armed before claim")
	}

	// recvTaskRwd replaces the task list — 3015 disappears (顾依萱 reproduction).
	applyMap(t, s, map[string]any{
		"23": map[string]any{
			"0": map[string]any{"9001": map[string]any{
				"14": map[string]any{"105": map[string]any{"0": []any{1003, nil, 5005}, "1": 3}},
			}},
			"3": map[string]any{"9001|0": map[string]any{
				"3": map[string]any{"1003": 60, "5005": 52},
				"5": map[string]any{},
			}},
		},
	})
	view, ok := s.CyclicNoteView(completeAt)
	if !ok || len(view.Tasks) < 1 || view.Tasks[0].TaskType == cyclicNoteTaskTypeFlowerRack {
		t.Fatalf("expected 3015 replaced after claim, view=%+v ok=%t", view, ok)
	}

	due := completeAt.Add(cyclicNoteFlowerRackRelistSlowAfter + time.Second)
	result := BuildPlan(s, policy, due)
	var cancels []PlannedOp
	for _, candidate := range result.Operations {
		if candidate.FeatureID == "activity.cyclicNote.flower_art_post_complete_cancel" {
			cancels = append(cancels, candidate)
		}
	}
	if len(cancels) != 2 {
		t.Fatalf("armed timer must survive claim replacement, got %d ops=%+v", len(cancels), result.Operations)
	}
}

func TestCyclicNoteFlowerRackPostCompleteOrphanListedAtAfterRestart(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	listedAt := now.Add(-cyclicNoteFlowerRackRelistSlowAfter - time.Second).UnixMilli()
	// No 3015 in task list (already claimed); no in-memory timer (restart).
	s := cyclicNotePlannerState(t, now, 2, []any{1003, nil, 5005}, map[string]any{"1003": 60, "5005": 52}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 2, "4": listedAt},
			"2": map[string]any{"1": 2, "2": 300208, "3": 3, "4": listedAt},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false

	result := BuildPlan(s, policy, now)
	var cancels []PlannedOp
	for _, candidate := range result.Operations {
		if candidate.FeatureID == "activity.cyclicNote.flower_art_post_complete_cancel" {
			cancels = append(cancels, candidate)
		}
	}
	if len(cancels) != 2 {
		t.Fatalf("orphan ListedAt recovery must cancel after 7 minutes, got %d ops=%+v",
			len(cancels), result.Operations)
	}
}

func TestCyclicNoteFlowerRackPostCompleteCancelsMissingListedAt(t *testing.T) {
	completeAt := time.UnixMilli(1_700_000)
	s := cyclicNotePlannerState(t, completeAt, 2, []any{2001, nil, nil}, map[string]any{"2001": 135}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 2}, // no field 4
			"2": map[string]any{"1": 2, "2": 300208, "3": 3, "4": 0},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false
	_ = BuildPlan(s, policy, completeAt) // arm

	due := completeAt.Add(cyclicNoteFlowerRackRelistSlowAfter + time.Second)
	result := BuildPlan(s, policy, due)
	var cancels []PlannedOp
	for _, candidate := range result.Operations {
		if candidate.Kind == clientproto.RPCFlowerRackCancelSell.String() &&
			candidate.FeatureID == "activity.cyclicNote.flower_art_post_complete_cancel" {
			cancels = append(cancels, candidate)
		}
	}
	if len(cancels) != 2 {
		t.Fatalf("post-complete must cancel occupied racks even without ListedAt, got %d ops=%+v",
			len(cancels), result.Operations)
	}
	if s.CyclicNoteFlowerRackPostCompleteCancelDone(9001) {
		t.Fatal("must not mark post-complete done while racks still occupied")
	}
}

func TestCyclicNoteFlowerRackCancelOutranksCustomerOrders(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	listedAt := now.Add(-cyclicNoteFlowerRackRelistAfter - time.Second).UnixMilli()
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 134}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"300208": 4}}},
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 2, "4": listedAt},
		}},
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 300208, "2": 1, "3": 1},
		}}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false
	policy.Order.Customer.Enabled = true
	policy.Union.Race.Enabled = false

	result := BuildPlan(s, policy, now)
	var cancel, customer *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCFlowerRackCancelSell.String() &&
			strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") {
			cancel = op
		}
		if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() {
			customer = op
		}
	}
	if cancel == nil {
		t.Fatalf("expected cyclic-note cancel: %+v", result.Operations)
	}
	if cancel.Priority != cyclicNoteRackCancelFloor {
		t.Fatalf("cancel priority=%d, want %d", cancel.Priority, cyclicNoteRackCancelFloor)
	}
	if customer != nil && !operationComesBefore(*cancel, *customer) {
		t.Fatalf("cancel %+v must outrank customer %+v", cancel, customer)
	}
	picked := Plan(s, policy, now)
	if picked == nil || picked.Kind != clientproto.RPCFlowerRackCancelSell.String() {
		t.Fatalf("expected cancel selected over customer work, plan=%+v", picked)
	}
}

func TestCyclicNoteFlowerRackPostCompleteSkipsWhenTaskStillOpen(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	listedAt := now.Add(-time.Hour).UnixMilli()
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 130}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 2, "4": listedAt},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false

	result := BuildPlan(s, policy, now.Add(10*time.Minute))
	for _, op := range result.Operations {
		if op.FeatureID == "activity.cyclicNote.flower_art_post_complete_cancel" {
			t.Fatalf("post-complete path must not run while 3015 still open: %+v", op)
		}
	}
}

func TestCyclicNoteFlowerRackRecoversStrandedLocalProgress(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// Server 123/135, local high-water already 135, every rack empty — the
	// cancel/relist bug that froze accounts at missing=0.
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 123}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"300208": 20}}},
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 0, "3": 0},
			"2": map[string]any{"1": 2, "2": 0, "3": 0},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false
	s.BumpCyclicNoteLocalProgress(9001, cyclicNoteTaskTypeFlowerRack, 123, 12) // local 135

	result := BuildPlan(s, policy, now)
	demand, ok := findCyclicNoteActionDemand(result, cyclicNoteTaskTypeFlowerRack)
	if !ok {
		t.Fatal("expected rack demand after stranded recovery")
	}
	op := requireSingleCyclicNoteBusinessOp(t, result.Operations, cyclicNoteTaskTypeFlowerRack)
	if op.Kind != clientproto.RPCFlowerRackSell.String() || op.Count != 12 {
		t.Fatalf("expected resume sell for remaining 12, demand=%+v op=%+v", demand, op)
	}
	if s.CyclicNoteLocalProgress(9001, cyclicNoteTaskTypeFlowerRack) != 0 {
		t.Fatalf("stranded local high-water should be cleared before re-list")
	}
}

func TestCyclicNoteFlowerRackRecoversWhenEmptyRackRemains(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// Near end of quota: local already 135, server still 123, one rack holds the
	// last listing and another is empty — previously froze until restart.
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 123}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"300208": 20}}},
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 3, "4": now.UnixMilli()},
			"2": map[string]any{"1": 2, "2": 0, "3": 0},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false
	s.BumpCyclicNoteLocalProgress(9001, cyclicNoteTaskTypeFlowerRack, 123, 12) // local 135

	result := BuildPlan(s, policy, now)
	op := requireSingleCyclicNoteBusinessOp(t, result.Operations, cyclicNoteTaskTypeFlowerRack)
	if op.Kind != clientproto.RPCFlowerRackSell.String() || op.TargetID != 2 || op.Count != 12 {
		t.Fatalf("expected sell onto empty rack for server remaining, got %+v", op)
	}
	if s.CyclicNoteLocalProgress(9001, cyclicNoteTaskTypeFlowerRack) != 0 {
		t.Fatalf("local high-water should clear when an empty rack remains")
	}
}

func TestCyclicNoteFlowerRackDoesNotCraftWhenNoStock(t *testing.T) {
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
	policy.Order.FlowerArt.SellEnabled = false
	policy.Order.FlowerArt.CraftEnabled = false

	result := BuildPlan(s, policy, now)
	for _, candidate := range result.Operations {
		if candidate.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() &&
			strings.HasPrefix(candidate.DemandID, cyclicNoteActionGoal+":") {
			t.Fatalf("activity must not craft flower art: %+v", candidate)
		}
	}
	if ops := cyclicNoteDrivenBusinessOps(result.Operations); len(ops) != 0 {
		t.Fatalf("expected no cyclic-note business ops without finished stock: %+v", ops)
	}
}

func TestCyclicNoteFlowerRackListsHighestStockFinishedArt(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 130}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"300208": 3,
			"301612": 40,
		}}},
		"102": map[string]any{"0": map[string]any{
			"3002": map[string]any{"1": 3002},
			"3016": map[string]any{"1": 3016},
		}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = true
	policy.Order.FlowerArt.SellArtIds = []int32{300208} // preferred low-stock must not win

	result := BuildPlan(s, policy, now)
	op := requireSingleCyclicNoteBusinessOp(t, result.Operations, cyclicNoteTaskTypeFlowerRack)
	if op.Kind != clientproto.RPCFlowerRackSell.String() || op.ItemID != 301612 || op.Count != 5 {
		// remaining=5 (135-130); listCount capped by remaining, not per-slot 12
		t.Fatalf("expected highest-stock finished art 301612: %+v", op)
	}
}

func TestCyclicNoteFlowerRackDoesNotCraftWhenFinishedStockExists(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	listedAt := now.Add(-time.Minute).UnixMilli()
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 130}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"300208": 20, "23005": 40, "23007": 40, "23008": 40}, "34": 12,
		}},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3002": map[string]any{"1": 3002}}},
		// All racks occupied — sell cannot run, but finished stock exists so do not craft.
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 2, "4": listedAt},
			"2": map[string]any{"1": 2, "2": 300208, "3": 2, "4": listedAt},
			"3": map[string]any{"1": 3, "2": 300208, "3": 2, "4": listedAt},
			"4": map[string]any{"1": 4, "2": 300208, "3": 2, "4": listedAt},
			"5": map[string]any{"1": 5, "2": 300208, "3": 2, "4": listedAt},
			"6": map[string]any{"1": 6, "2": 300208, "3": 2, "4": listedAt},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false
	policy.Order.FlowerArt.CraftEnabled = false

	result := BuildPlan(s, policy, now)
	for _, op := range result.Operations {
		if strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") &&
			op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() {
			t.Fatalf("must not craft while finished stock exists: %+v", op)
		}
	}
}

func TestCyclicNotePlantWorksWithoutAutoEnabledAndAutoHarvest(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{4003, nil, nil}, map[string]any{"4003": 78}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": emptyLands(3)},
		"101": map[string]any{"0": cultivate(23001)},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Plant.Planting.AutoEnabled = false
	policy.Plant.Planting.AutoHarvestEnabled = false

	result := BuildPlan(s, policy, now)
	op := requireSingleCyclicNoteBusinessOp(t, result.Operations, cyclicNoteTaskTypePlantAny)
	if op.Kind != clientproto.RPCUsrLandPlantBatch.String() || op.FlowerID != 23001 || !op.Executable {
		t.Fatalf("expected plant-any without auto plant/harvest: %+v", op)
	}

	// Ready lands must still harvest so slots turn over.
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 3, "7": now.UnixMilli()},
		}},
	})
	harvestPlan := BuildPlan(s, policy, now)
	var harvested bool
	for _, candidate := range harvestPlan.Operations {
		if candidate.Kind == clientproto.RPCUsrLandHarvest.String() && runnableBusinessOperation(candidate) {
			harvested = true
			break
		}
	}
	if !harvested {
		t.Fatalf("plant-any must harvest without auto_harvest_enabled: %+v", harvestPlan.Operations)
	}
}

func TestCyclicNotePlantFollowsAutoReplantFilters(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{4003, nil, nil}, map[string]any{"4003": 78}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": emptyLands(3)},
		"101": map[string]any{"0": cultivate(23001, 23002)},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Plant.Planting.AutoEnabled = false
	policy.Plant.Planting.AutoHarvestEnabled = false
	policy.Plant.Planting.AutoReplantMode = pb.SelectionMode_SELECTION_MODE_SPECIFIC
	policy.Plant.Planting.AutoReplantFlowerIds = []int32{23002}

	op := requireSingleCyclicNoteBusinessOp(t, BuildPlan(s, policy, now).Operations, cyclicNoteTaskTypePlantAny)
	if op.FlowerID != 23002 || !op.Executable {
		t.Fatalf("plant-any must follow planting auto_replant filters: %+v", op)
	}
}

func TestCyclicNoteLocalProgressStopsPlantAndFlowerRack(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{4003, 2001, nil}, map[string]any{"4003": 78, "2001": 134}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"32": map[string]any{"300208": 4}}},
		"100": map[string]any{"1": emptyLands(3)},
		"101": map[string]any{"0": cultivate(23001)},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Plant.Planting.AutoEnabled = false
	policy.Plant.Planting.AutoHarvestEnabled = false
	policy.Order.FlowerArt.SellEnabled = false

	first := BuildPlan(s, policy, now)
	if _, ok := findCyclicNoteActionDemand(first, cyclicNoteTaskTypePlantAny); !ok {
		t.Fatalf("missing plant demand before local progress: %+v", first.Demands)
	}
	if _, ok := findCyclicNoteActionDemand(first, cyclicNoteTaskTypeFlowerRack); !ok {
		t.Fatalf("missing rack demand before local progress: %+v", first.Demands)
	}

	// Plant remaining=2 (target 80, progress 78) and rack remaining=1 (target 135, progress 134).
	s.BumpCyclicNoteLocalProgress(9001, cyclicNoteTaskTypePlantAny, 78, 2)
	s.BumpCyclicNoteLocalProgress(9001, cyclicNoteTaskTypeFlowerRack, 134, 1)
	// Keep one rack listed and no empty observed slots so local high-water is
	// "in flight" on a full observed shelf (recovery only runs when an empty
	// rack remains).
	applyMap(t, s, map[string]any{
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 1, "4": now.UnixMilli()},
		}},
	})

	second := BuildPlan(s, policy, now)
	// Server counters are still short of target, so demands remain with Missing=0
	// (no more plant/list). Flower-rack can still cancel/relist occupied racks.
	plantDemand, ok := findCyclicNoteActionDemand(second, cyclicNoteTaskTypePlantAny)
	if !ok || plantDemand.Missing != 0 {
		t.Fatalf("plant demand after local progress=%+v", plantDemand)
	}
	rackDemand, ok := findCyclicNoteActionDemand(second, cyclicNoteTaskTypeFlowerRack)
	if !ok || rackDemand.Missing != 0 {
		t.Fatalf("rack demand after local progress=%+v", rackDemand)
	}
	for _, op := range second.Operations {
		if strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") &&
			(isPlantOperation(op.Kind) || op.Kind == clientproto.RPCFlowerRackSell.String()) {
			t.Fatalf("local-complete tasks must not plant/list more: %+v", op)
		}
	}
}

func TestCyclicNoteLocalProgressStillWatersAndHarvests(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// Server 79/80; local high-water already 80 → Missing=0, but task still open.
	s := cyclicNotePlannerState(t, now, 2, []any{4003, nil, nil}, map[string]any{"4003": 79}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 50},
			"33": map[string]any{"7": map[string]any{"1": 130, "5": now.Add(time.Hour).UnixMilli()}},
		}},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 1, "2": 1, "3": 0},
			"1002": map[string]any{"0": 23001, "1": 3, "7": now.UnixMilli()},
			"1003": map[string]any{"1": 0},
		}},
		"101": map[string]any{"0": cultivate(23001)},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Plant.Planting.AutoEnabled = false
	policy.Plant.Planting.AutoHarvestEnabled = false
	policy.Plant.Planting.MinWaterDrops = 5
	s.BumpCyclicNoteLocalProgress(9001, cyclicNoteTaskTypePlantAny, 79, 1)

	result := BuildPlan(s, policy, now)
	demand, ok := findCyclicNoteActionDemand(result, cyclicNoteTaskTypePlantAny)
	if !ok || demand.Missing != 0 {
		t.Fatalf("expected open plant-any with Missing=0: %+v", demand)
	}
	var watered, harvested, planted bool
	for _, op := range result.Operations {
		if !runnableBusinessOperation(op) {
			continue
		}
		switch {
		case op.Kind == clientproto.RPCUsrLandWater.String() || op.Kind == clientproto.RPCUsrLandWaterBatch.String():
			watered = true
		case op.Kind == clientproto.RPCUsrLandHarvest.String():
			harvested = true
		case isPlantOperation(op.Kind):
			planted = true
		}
	}
	if !watered {
		t.Fatalf("plant-any must keep watering while server lags local high-water: %+v", result.Operations)
	}
	if !harvested {
		t.Fatalf("plant-any must keep harvesting while server lags local high-water: %+v", result.Operations)
	}
	if planted {
		t.Fatalf("Missing=0 must not plant more when auto_enabled is off: %+v", result.Operations)
	}
}

func TestCyclicNoteFlowerRackCancelsClaimableWhenLocalAheadOfServer(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	// Claimable but listed >5m: cyclic-note relists instead of recvSellMoney.
	listedAt := now.Add(-cyclicNoteFlowerRackRelistAfter - time.Second).UnixMilli()
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 123}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"300208": 20}}},
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 1, "4": listedAt},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	policy.Order.FlowerArt.SellEnabled = false
	s.BumpCyclicNoteLocalProgress(9001, cyclicNoteTaskTypeFlowerRack, 123, 12) // local 135, server 123

	result := BuildPlan(s, policy, now)
	demand, ok := findCyclicNoteActionDemand(result, cyclicNoteTaskTypeFlowerRack)
	if !ok || demand.Missing != 0 {
		t.Fatalf("expected Missing=0 rack demand while server lags: %+v", demand)
	}
	op := requireSingleCyclicNoteBusinessOp(t, result.Operations, cyclicNoteTaskTypeFlowerRack)
	if op.Kind != clientproto.RPCFlowerRackCancelSell.String() || op.TargetID != 1 {
		t.Fatalf("expected cancel/relist while local ahead, not claim: %+v", op)
	}
}

func TestCyclicNoteFlowerRackCapsAllSellsToRemaining(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{2001, nil, nil}, map[string]any{"2001": 123}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"300208": 100}}},
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 0, "3": 0},
			"2": map[string]any{"1": 2, "2": 0, "3": 0},
			"3": map[string]any{"1": 3, "2": 0, "3": 0},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
	// Force multiple ordinary sells into the plan before cyclic-note linking.
	policy.Order.FlowerArt.SellEnabled = true

	result := BuildPlan(s, policy, now)
	var linked []PlannedOp
	var total int32
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFlowerRackSell.String() && strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") {
			linked = append(linked, op)
			total += op.Count
		}
		if op.Kind == clientproto.RPCFlowerRackSell.String() && !strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") &&
			!strings.HasPrefix(op.DemandID, raceActionGoal+":") && runnableBusinessOperation(op) {
			t.Fatalf("unlinked ordinary sell must be suppressed: %+v", op)
		}
	}
	if total != 12 { // target 135 - progress 123
		t.Fatalf("linked sell total=%d ops=%+v, want 12", total, linked)
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

func TestCyclicNoteAutoHireBypassesBasicSwitchButKeepsLimits(t *testing.T) {
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
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{
		cyclicNoteSatisfyTasksKey: true,
		cyclicNoteAutoHireKey:     true,
	})
	policy.Basic.Pearl.AutoHireEnabled = false
	policy.Basic.Pearl.MaxHireTicketUsage = 2

	result := BuildPlan(s, policy, now)
	op := requireSingleCyclicNoteBusinessOp(t, result.Operations, cyclicNoteTaskTypePearlHire)
	if op.Kind != clientproto.RPCPearlPlaceHire.String() || op.TargetID != 1 || op.TargetUID != 2001 ||
		op.ItemCost[1003] != 1 || op.GoldCost != 0 {
		t.Fatalf("activity auto_hire=%+v", op)
	}
	if err := ValidateSafePearlHire(s, policy.Basic.Pearl, &op, now); err != nil {
		t.Fatalf("auto_hire preflight failed while basic switch off: %v", err)
	}

	// No pearl-hire task → no hire while basic switch stays off.
	idle := cyclicNotePlannerState(t, now, 2, []any{4003, nil, nil}, map[string]any{"4003": 0}, map[string]any{}, 0, []any{})
	applyMap(t, idle, map[string]any{
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
	applyMap(t, idle, map[string]any{"115": map[string]any{"5": map[string]any{"2001": int64(0)}}})
	idlePlan := BuildPlan(idle, policy, now)
	for _, candidate := range idlePlan.Operations {
		if candidate.Kind == clientproto.RPCPearlPlaceHire.String() {
			t.Fatalf("auto_hire must not run without pearl-hire task: %+v", candidate)
		}
	}

	policy.Basic.Pearl.MaxHireTicketUsage = 0
	blocked := BuildPlan(s, policy, now)
	blockedOp := requireSingleCyclicNoteBusinessOp(t, blocked.Operations, cyclicNoteTaskTypePearlHire)
	if blockedOp.Executable || blockedOp.Status != PlanStatusBlocked {
		t.Fatalf("auto_hire must still honor max_hire_ticket_usage: %+v", blockedOp)
	}
}

func TestCyclicNoteAutoCompleteResidentOrdersBypassesOrderModule(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{1005, nil, nil}, map[string]any{"1005": 60}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"23005": 10, "23001": 5, "23003": 5}}},
		"105": map[string]any{"0": map[string]any{
			"1": map[string]any{
				"2":  map[string]any{"0": 2, "2": [][]int32{{23005, 1}}},
				"10": map[string]any{"0": 10, "2": [][]int32{{23005, 1}}},
			},
			"6": map[string]any{"0": []any{[]any{23001, 2}}},
			"7": map[string]any{"0": []any{[]any{23003, 2}}},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{
		cyclicNoteSatisfyTasksKey:               true,
		cyclicNoteAutoCompleteResidentOrdersKey: true,
	})
	policy.Order.Resident.NormalEnabled = false
	policy.Order.Resident.SatinEnabled = false
	policy.Order.Resident.DecorateEnabled = false
	policy.Order.Resident.Qualities = []int32{9} // would block quality-1 flowers under order module
	policy.Order.Resident.NormalDailyLimit = 1
	policy.Order.Resident.SatinDailyLimit = 1
	policy.Order.Resident.DecorateDailyLimit = 1

	result := BuildPlan(s, policy, now)
	var normalFinishes, satinFinishes, decorateFinishes []PlannedOp
	for _, candidate := range result.Operations {
		if !strings.HasPrefix(candidate.DemandID, cyclicNoteActionGoal+":") || !runnableBusinessOperation(candidate) {
			continue
		}
		switch candidate.Kind {
		case clientproto.RPCOrderFlowerFinishOrder.String():
			normalFinishes = append(normalFinishes, candidate)
		case clientproto.RPCOrderFlowerFinishSatinOrder.String():
			satinFinishes = append(satinFinishes, candidate)
		case clientproto.RPCOrderFlowerFinishDecorateOrder.String():
			decorateFinishes = append(decorateFinishes, candidate)
		}
	}
	if len(normalFinishes) != 2 {
		t.Fatalf("expected both resident finishes under activity switch: %+v", normalFinishes)
	}
	if len(satinFinishes) != 1 || len(decorateFinishes) != 1 {
		t.Fatalf("expected satin and decorate finishes under activity switch: satin=%+v decorate=%+v", satinFinishes, decorateFinishes)
	}

	// Mixed ad + normal boxes: finish the normal one; do not pause.
	applyMap(t, s, map[string]any{
		"105": map[string]any{"0": map[string]any{"1": map[string]any{
			"1": map[string]any{"0": 8},
			"2": map[string]any{"0": 2, "2": [][]int32{{23005, 1}}},
		}}},
	})
	mixed := BuildPlan(s, policy, now)
	var mixedFinishes []PlannedOp
	for _, candidate := range mixed.Operations {
		if candidate.Kind == clientproto.RPCOrderFlowerFinishOrder.String() &&
			strings.HasPrefix(candidate.DemandID, cyclicNoteActionGoal+":") &&
			runnableBusinessOperation(candidate) {
			mixedFinishes = append(mixedFinishes, candidate)
		}
		if candidate.FeatureID == "activity.cyclicNote.resident_ad" {
			t.Fatalf("must not pause while a normal resident order is ready: %+v", candidate)
		}
	}
	if len(mixedFinishes) != 1 || mixedFinishes[0].TargetID != 2 {
		t.Fatalf("expected finish for normal box 2 only: %+v", mixedFinishes)
	}

	// All normal boxes are ad slots but satin is ready: finish satin, do not pause.
	applyMap(t, s, map[string]any{
		"105": map[string]any{"0": map[string]any{
			"1": map[string]any{
				"1": map[string]any{"0": 8},
				"2": map[string]any{"0": 8},
			},
			"6": map[string]any{"0": []any{[]any{23001, 2}}},
		}},
	})
	satinOnly := BuildPlan(s, policy, now)
	var satinOnlyFinishes []PlannedOp
	for _, candidate := range satinOnly.Operations {
		if candidate.Kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() &&
			strings.HasPrefix(candidate.DemandID, cyclicNoteActionGoal+":") &&
			runnableBusinessOperation(candidate) {
			satinOnlyFinishes = append(satinOnlyFinishes, candidate)
		}
		if candidate.FeatureID == "activity.cyclicNote.resident_ad" {
			t.Fatalf("must not pause while satin order is ready: %+v", candidate)
		}
	}
	if len(satinOnlyFinishes) != 1 {
		t.Fatalf("expected satin finish while normal boxes are ads: %+v", satinOnlyFinishes)
	}

	// All cooldown-ready orders are ad-only: pause instead of finishing.
	applyMap(t, s, map[string]any{
		"105": map[string]any{"0": map[string]any{
			"1": map[string]any{
				"1": map[string]any{"0": 8},
				"2": map[string]any{"0": 8},
			},
			"6": map[string]any{"0": []any{}, "4": 1},
			"7": map[string]any{"0": []any{}, "4": 1},
		}},
	})
	paused := BuildPlan(s, policy, now)
	var sawPause bool
	for _, candidate := range paused.Operations {
		if strings.HasPrefix(candidate.DemandID, cyclicNoteActionGoal+":") &&
			runnableBusinessOperation(candidate) &&
			cyclicNoteResidentOrderKind(candidate.Kind) {
			t.Fatalf("must not finish while all orders are ad-only: %+v", candidate)
		}
		if candidate.FeatureID == "activity.cyclicNote.resident_ad" && candidate.Status == PlanStatusBlocked {
			sawPause = true
		}
	}
	if !sawPause {
		t.Fatalf("expected resident ad pause marker: %+v", paused.Operations)
	}
}

func TestCyclicNoteRespectResidentOrderDailyLimit(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{1005, nil, nil}, map[string]any{"1005": 60}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"23005": 10, "23001": 5, "23003": 5}}},
		"105": map[string]any{"0": map[string]any{
			"1": map[string]any{
				"2":  map[string]any{"0": 2, "2": [][]int32{{23005, 1}}},
				"10": map[string]any{"0": 10, "2": [][]int32{{23005, 1}}},
			},
			"6": map[string]any{"0": []any{[]any{23001, 2}}},
			"7": map[string]any{"0": []any{[]any{23003, 2}}},
		}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{
		cyclicNoteSatisfyTasksKey:                     true,
		cyclicNoteAutoCompleteResidentOrdersKey:       true,
		cyclicNoteRespectResidentOrderDailyLimitKey:   true,
	})
	policy.Order.Resident.NormalDailyLimit = 1
	policy.Order.Resident.SatinDailyLimit = 1
	policy.Order.Resident.DecorateDailyLimit = 1
	s.NoteResidentOrderFinished(now, nil)
	s.NoteResidentSatinOrderFinished(now, nil)
	s.NoteResidentDecorateOrderFinished(now, nil)

	result := BuildPlan(s, policy, now)
	var normalFinishes, satinFinishes, decorateFinishes int
	for _, candidate := range result.Operations {
		if !strings.HasPrefix(candidate.DemandID, cyclicNoteActionGoal+":") || !runnableBusinessOperation(candidate) {
			continue
		}
		switch candidate.Kind {
		case clientproto.RPCOrderFlowerFinishOrder.String():
			normalFinishes++
		case clientproto.RPCOrderFlowerFinishSatinOrder.String():
			satinFinishes++
		case clientproto.RPCOrderFlowerFinishDecorateOrder.String():
			decorateFinishes++
		}
	}
	if normalFinishes != 0 || satinFinishes != 0 || decorateFinishes != 0 {
		t.Fatalf("expected no finishes when daily limits reached: normal=%d satin=%d decorate=%d ops=%+v",
			normalFinishes, satinFinishes, decorateFinishes, result.Operations)
	}

	policy.Activity.Modules[cyclicNoteModuleKey].BoolParams[cyclicNoteRespectResidentOrderDailyLimitKey] = false
	bypass := BuildPlan(s, policy, now)
	normalFinishes = 0
	satinFinishes = 0
	decorateFinishes = 0
	for _, candidate := range bypass.Operations {
		if !strings.HasPrefix(candidate.DemandID, cyclicNoteActionGoal+":") || !runnableBusinessOperation(candidate) {
			continue
		}
		switch candidate.Kind {
		case clientproto.RPCOrderFlowerFinishOrder.String():
			normalFinishes++
		case clientproto.RPCOrderFlowerFinishSatinOrder.String():
			satinFinishes++
		case clientproto.RPCOrderFlowerFinishDecorateOrder.String():
			decorateFinishes++
		}
	}
	if normalFinishes != 2 || satinFinishes != 1 || decorateFinishes != 1 {
		t.Fatalf("expected all finishes when daily limit switch off: normal=%d satin=%d decorate=%d",
			normalFinishes, satinFinishes, decorateFinishes)
	}
}

func TestCyclicNoteRespectResidentOrderDailyLimitDefersOrderModule(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{4003, nil, nil}, map[string]any{"4003": 0}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"23005": 10}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{
			"2": map[string]any{"0": 2, "2": [][]int32{{23005, 1}}},
		}}},
	})
	policy := cyclicNotePlannerPolicy(true, true, map[string]bool{
		cyclicNoteSatisfyTasksKey:                   true,
		cyclicNoteRespectResidentOrderDailyLimitKey: true,
	})
	policy.Order.Resident.NormalEnabled = true

	deferred := BuildPlan(s, policy, now)
	for _, candidate := range deferred.Operations {
		if candidate.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && runnableBusinessOperation(candidate) {
			t.Fatalf("order module must defer resident finish before resident task appears: %+v", candidate)
		}
	}

	s = cyclicNotePlannerState(t, now, 2, []any{1005, nil, nil}, map[string]any{"1005": 60}, map[string]any{}, 0, []any{})
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"23005": 10}}},
		"105": map[string]any{"0": map[string]any{"1": map[string]any{
			"2":  map[string]any{"0": 2, "2": [][]int32{{23005, 1}}},
			"10": map[string]any{"0": 10, "2": [][]int32{{23005, 1}}},
		}}},
	})
	active := BuildPlan(s, policy, now)
	var linked []PlannedOp
	for _, candidate := range active.Operations {
		if candidate.Kind == clientproto.RPCOrderFlowerFinishOrder.String() &&
			strings.HasPrefix(candidate.DemandID, cyclicNoteActionGoal+":") &&
			runnableBusinessOperation(candidate) {
			linked = append(linked, candidate)
		}
	}
	if len(linked) != 1 {
		t.Fatalf("expected one linked resident finish when task active: %+v", linked)
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
