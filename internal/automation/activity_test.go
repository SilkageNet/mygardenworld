package automation

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	cyclicNoteAutoClaimTaskRewardsKey    = "auto_claim_task_rewards"
	cyclicNoteAutoClaimProgressBoxesKey  = "auto_claim_progress_boxes"
	cyclicNoteSatisfyTasksKey            = "satisfy_tasks"
	cyclicStoryAutoClaimOrderRewardsKey  = "auto_claim_order_rewards"
	cyclicStoryAutoClaimProgressBoxesKey = "auto_claim_progress_boxes"
	dessertAutoClaimTaskRewardsKey       = "auto_claim_task_rewards"
	dessertAutoLikeCelebrityKey          = "auto_like_celebrity"
	dessertAutoClaimProgressBoxesKey     = "auto_claim_progress_boxes"
	dessertAutoOpenRewardBoxesKey        = "auto_open_reward_boxes"
)

func TestCyclicNotePolicyDefaultsFailClosedAndRequiresModuleEnable(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{4003, 2001, nil}, map[string]any{"4003": 80}, map[string]any{}, 120, []any{})

	tests := []struct {
		name          string
		moduleEnabled bool
		bools         map[string]bool
	}{
		{name: "all bool params default false", moduleEnabled: true},
		{name: "module disabled", bools: map[string]bool{cyclicNoteAutoClaimTaskRewardsKey: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			policy := cyclicNotePlannerPolicy(tc.moduleEnabled, tc.bools)
			if got := cyclicNotePlanOperations(BuildPlan(s, policy, now).Operations); len(got) != 0 {
				t.Fatalf("cyclic-note operations=%+v, want none", got)
			}
		})
	}

	for _, spec := range featureSpecs {
		if spec.ID == "activity.cyclicNote" {
			if spec.Status != PlanStatusManaged || !spec.Executable || spec.SyncOnly {
				t.Fatalf("cyclic-note feature=%+v, want managed executable", spec)
			}
			return
		}
	}
	t.Fatal("missing cyclic-note feature")
}

func TestCyclicNotePlannerEntersOnlyForTaskConsumersInClaimablePhases(t *testing.T) {
	for _, phase := range []int32{2, 3} {
		t.Run("phase "+itoa32(phase), func(t *testing.T) {
			now := time.UnixMilli(1_500_000)
			s := cyclicNotePlannerState(t, now, phase, nil, nil, nil, 120, []any{})
			policy := cyclicNotePlannerPolicy(true, map[string]bool{cyclicNoteSatisfyTasksKey: true})
			ops := cyclicNotePlanOperations(BuildPlan(s, policy, now).Operations)
			if len(ops) != 1 || ops[0].Kind != clientproto.RPCActCyclicNoteEnter.String() ||
				ops[0].BatchID != 9001 || ops[0].Priority != cyclicNotePriority || ops[0].Lane != LaneSide ||
				ops[0].Category != CategoryActivity || !ops[0].Executable || ops[0].Status != PlanStatusManaged {
				t.Fatalf("enter operations=%+v", ops)
			}
			if ops[0].OperationID != clientproto.RPCActCyclicNoteEnter.String()+":9001" {
				t.Fatalf("enter operation id=%q", ops[0].OperationID)
			}
		})
	}

	now := time.UnixMilli(1_500_000)
	phaseOne := cyclicNotePlannerState(t, now, 1, nil, nil, nil, 120, []any{})
	policy := cyclicNotePlannerPolicy(true, map[string]bool{cyclicNoteAutoClaimTaskRewardsKey: true})
	if ops := cyclicNotePlanOperations(BuildPlan(phaseOne, policy, now).Operations); len(ops) != 0 {
		t.Fatalf("phase-1 enter operations=%+v", ops)
	}

	// Progress-box claims do not need task initialization and therefore must
	// not probe enter when task-related switches remain false.
	phaseTwo := cyclicNotePlannerState(t, now, 2, nil, nil, nil, 120, []any{})
	policy = cyclicNotePlannerPolicy(true, map[string]bool{cyclicNoteAutoClaimProgressBoxesKey: true})
	ops := cyclicNotePlanOperations(BuildPlan(phaseTwo, policy, now).Operations)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCActCyclicNoteRecv.String() {
		t.Fatalf("progress-only operations=%+v", ops)
	}
}

func TestCyclicNotePlannerClaimsOneTaskBeforeMilestoneInSlotOrder(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{2001, 4003, nil}, map[string]any{
		"2001": 134,
		"4003": 81,
	}, map[string]any{}, 120, []any{})
	policy := cyclicNotePlannerPolicy(true, map[string]bool{
		cyclicNoteAutoClaimTaskRewardsKey:   true,
		cyclicNoteAutoClaimProgressBoxesKey: true,
	})

	first := cyclicNotePlanOperations(BuildPlan(s, policy, now).Operations)
	second := cyclicNotePlanOperations(BuildPlan(s, policy, now).Operations)
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("activity op count first=%+v second=%+v", first, second)
	}
	op := first[0]
	if op.Kind != clientproto.RPCActCyclicNoteRecvTaskRwd.String() || op.BatchID != 9001 ||
		op.SlotID != 2 || op.TaskID != 4003 || op.MilestoneIndex != 0 || op.Priority != 5000 ||
		op.OperationID != clientproto.RPCActCyclicNoteRecvTaskRwd.String()+":9001:2:4003" {
		t.Fatalf("task claim=%+v", op)
	}
	if second[0].OperationID != op.OperationID {
		t.Fatalf("unstable operation IDs: %q != %q", second[0].OperationID, op.OperationID)
	}
}

func TestCyclicNotePlannerRejectsDuplicateTaskAndClaimsMilestoneByIndex(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 2, []any{4003, 4003, nil}, map[string]any{"4003": 80}, map[string]any{}, 120, []any{})
	policy := cyclicNotePlannerPolicy(true, map[string]bool{cyclicNoteAutoClaimTaskRewardsKey: true})
	if ops := cyclicNotePlanOperations(BuildPlan(s, policy, now).Operations); len(ops) != 0 {
		t.Fatalf("ambiguous duplicate task planned=%+v", ops)
	}

	policy = cyclicNotePlannerPolicy(true, map[string]bool{
		cyclicNoteAutoClaimTaskRewardsKey:   true,
		cyclicNoteAutoClaimProgressBoxesKey: true,
	})
	ops := cyclicNotePlanOperations(BuildPlan(s, policy, now).Operations)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCActCyclicNoteRecv.String() ||
		ops[0].MilestoneIndex != 1 || ops[0].BatchID != 9001 ||
		ops[0].OperationID != clientproto.RPCActCyclicNoteRecv.String()+":9001:1" {
		t.Fatalf("milestone claim=%+v", ops)
	}
}

func TestCyclicNotePlannerPhaseThreeClaimsOnlyMilestones(t *testing.T) {
	now := time.UnixMilli(1_500_000)
	s := cyclicNotePlannerState(t, now, 3, []any{4003, 2001, nil}, map[string]any{"4003": 80}, map[string]any{}, 120, []any{})
	policy := cyclicNotePlannerPolicy(true, map[string]bool{
		cyclicNoteAutoClaimTaskRewardsKey:   true,
		cyclicNoteAutoClaimProgressBoxesKey: true,
	})
	ops := cyclicNotePlanOperations(BuildPlan(s, policy, now).Operations)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCActCyclicNoteRecv.String() || ops[0].MilestoneIndex != 1 {
		t.Fatalf("phase-3 operations=%+v", ops)
	}
}

func TestCyclicNotePrioritySitsBetweenMajorGoalsAndOrdinaryFlowerRack(t *testing.T) {
	activity := cyclicNotePlannedOp(clientproto.RPCActCyclicNoteRecv.String(), "claim_progress", "ready", 9001, 0, 0, 1)
	ops := []PlannedOp{
		{OperationID: "flower-rack", Lane: LaneSide, Category: CategoryOrder, Priority: 4400},
		activity,
		{OperationID: "main-goal", Lane: LaneSide, Category: CategoryBasic, Priority: 6300},
	}
	sortOperations(ops)
	if activity.Priority != 5000 || ops[0].OperationID != "main-goal" ||
		ops[1].OperationID != activity.OperationID || ops[2].OperationID != "flower-rack" {
		t.Fatalf("priority order=%+v", ops)
	}
}

func cyclicNotePlannerPolicy(moduleEnabled bool, bools map[string]bool) *pb.Policy {
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Activity.CyclicNote = &pb.CyclicNotePolicy{
		Enabled:                moduleEnabled,
		AutoClaimTaskRewards:   bools[cyclicNoteAutoClaimTaskRewardsKey],
		AutoClaimProgressBoxes: bools[cyclicNoteAutoClaimProgressBoxesKey],
		SatisfyTasks:           bools[cyclicNoteSatisfyTasksKey],
	}
	return policy
}

func cyclicNotePlanOperations(ops []PlannedOp) []PlannedOp {
	out := make([]PlannedOp, 0, 1)
	for _, op := range ops {
		if op.Domain == "activity.cyclicNote" {
			out = append(out, op)
		}
	}
	return out
}

func cyclicNotePlannerState(t *testing.T, now time.Time, phase int32, tasks []any, progress, receipts map[string]any, score int32, boxes []any) *state.State {
	t.Helper()
	const duration int64 = 1_000_000
	nowMs := now.UnixMilli()
	beginMs := nowMs - duration/2
	endMs := nowMs + duration/2
	beforeMs := int64(0)
	afterMs := duration
	switch phase {
	case 1:
		beginMs = nowMs + duration/2
		endMs = beginMs + duration
		beforeMs = duration
	case 2:
		// Defaults above put now in the active phase.
	case 3:
		beginMs = nowMs - duration
		endMs = nowMs - duration/2
	default:
		t.Fatalf("unsupported fixture phase %d", phase)
	}

	cyclic := map[string]any{"1": 2, "2": nowMs - 1000}
	if tasks != nil {
		cyclic["0"] = tasks
	}
	batch := map[string]any{
		"0": 9001, "1": 40020007, "2": 4002, "3": 1,
		"5": beginMs, "7": endMs, "8": beforeMs, "9": afterMs,
		"11": score, "12": map[string]any{"1107": 0}, "13": boxes,
		"14": map[string]any{"105": cyclic},
	}
	ns23 := map[string]any{
		"0": map[string]any{"9001": batch},
		"1": map[string]any{"40020007": map[string]any{
			"0": 40020007, "1": "花笺集芳测试", "2": "fixture", "3": 4002,
			"9": []any{[]any{1, 60, "1,80"}, []any{2, 120, "1,200"}, []any{3, 265, "1,600"}},
		}},
	}
	if progress != nil || receipts != nil {
		if progress == nil {
			progress = map[string]any{}
		}
		if receipts == nil {
			receipts = map[string]any{}
		}
		ns23["3"] = map[string]any{"9001|0": map[string]any{
			"1": 9001, "2": 0, "3": progress, "5": receipts,
		}}
	}
	s := state.New()
	applyMap(t, s, map[string]any{"23": ns23})
	return s
}
