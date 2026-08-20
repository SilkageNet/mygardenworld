package automation

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestCyclicStoryOperationsOrderClaim(t *testing.T) {
	now := time.UnixMilli(1783696000000)
	s := applyCyclicStoryPlannerFixture(t)
	// Parent ActivityPolicy.enabled is ignored; module enabled alone is enough.
	policy := cyclicStoryPlannerPolicy(false, true, map[string]bool{
		cyclicStoryAutoClaimOrderRewardsKey:  true,
		cyclicStoryAutoClaimProgressBoxesKey: true,
	}, 0)

	ops := cyclicStoryPlanOperations(BuildPlan(s, policy, now).Operations)
	for _, op := range ops {
		if op.Action == "claim_order" {
			t.Fatalf("order claim without inventory: %+v", op)
		}
	}

	s.ApplyV(json.RawMessage(`{"7":{"0":{"32":{"23001":80}}}}`))
	ops = cyclicStoryPlanOperations(BuildPlan(s, policy, now).Operations)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCActCyclicStoryRecvOrderRwd.String() ||
		ops[0].BatchID != 9101 || ops[0].SlotID != 0 || ops[0].TaskID != 1 ||
		ops[0].FlowerID != 23001 || ops[0].ItemCost[23001] != 80 {
		t.Fatalf("expected order claim, got %+v", ops)
	}

	// Active orders still inside refreshCd (future validTime) must not be claimed.
	futureValid := now.UnixMilli() + 25*60*1000
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"14":{"106":{"0":{"0":{"0":1,"1":23001,"2":1783695000000,"3":` +
		itoa64(futureValid) + `},"1":{"0":2,"1":23002,"2":1783695100000,"3":0},"2":{"0":0,"1":0,"2":0,"3":1783698000000}},"1":3,"2":1783695955911,"3":1}}}}}}`))
	for _, op := range cyclicStoryPlanOperations(BuildPlan(s, policy, now).Operations) {
		if op.Action == "claim_order" && op.SlotID == 0 {
			t.Fatalf("slot 0 still in refreshCd must not claim: %+v", op)
		}
	}
}

func itoa64(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestCyclicStoryScoreCapBlocksOrderClaim(t *testing.T) {
	now := time.UnixMilli(1783696000000)
	s := applyCyclicStoryPlannerFixture(t)
	s.ApplyV(json.RawMessage(`{"7":{"0":{"32":{"23001":80}}}}`))

	capped := cyclicStoryPlannerPolicy(false, true, map[string]bool{
		cyclicStoryAutoClaimOrderRewardsKey:  true,
		cyclicStoryAutoClaimProgressBoxesKey: true,
	}, 45)
	for _, op := range cyclicStoryPlanOperations(BuildPlan(s, capped, now).Operations) {
		if op.Action == "claim_order" {
			t.Fatalf("order claim must stop at max_score: %+v", op)
		}
	}

	open := cyclicStoryPlannerPolicy(false, true, map[string]bool{cyclicStoryAutoClaimOrderRewardsKey: true}, 46)
	ops := cyclicStoryPlanOperations(BuildPlan(s, open, now).Operations)
	if len(ops) != 1 || ops[0].Action != "claim_order" {
		t.Fatalf("expected order claim under cap, got %+v", ops)
	}

	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"11":80}}}}`))
	milestonePolicy := cyclicStoryPlannerPolicy(false, true, map[string]bool{
		cyclicStoryAutoClaimProgressBoxesKey: true,
	}, 45)
	ops = cyclicStoryPlanOperations(BuildPlan(s, milestonePolicy, now).Operations)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCActCyclicStoryRecv.String() || ops[0].MilestoneIndex != 2 {
		t.Fatalf("expected milestone claim under score cap, got %+v", ops)
	}
}

func TestCyclicStoryEnterWhenOrdersMissing(t *testing.T) {
	now := time.UnixMilli(1783696000000)
	raw, err := os.ReadFile("../state/testdata/cyclic_story_activity.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	batch := payload["23"].(map[string]any)["0"].(map[string]any)["9101"].(map[string]any)
	delete(batch, "14")
	stripped, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal stripped fixture: %v", err)
	}
	s := state.New()
	s.ApplyV(stripped)

	policy := cyclicStoryPlannerPolicy(false, true, map[string]bool{cyclicStoryAutoClaimProgressBoxesKey: true}, 0)
	ops := cyclicStoryPlanOperations(BuildPlan(s, policy, now).Operations)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCActCyclicStoryEnter.String() || ops[0].BatchID != 9101 {
		view, _ := s.CyclicStoryView(now)
		t.Fatalf("expected enter, got %+v view=%+v", ops, view)
	}
}

func TestCyclicStoryEnterWithoutScoreBag(t *testing.T) {
	now := time.UnixMilli(1783696000000)
	raw, err := os.ReadFile("../state/testdata/cyclic_story_activity.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	batch := payload["23"].(map[string]any)["0"].(map[string]any)["9101"].(map[string]any)
	delete(batch, "11")
	delete(batch, "12")
	delete(batch, "13")
	delete(batch, "14")
	stripped, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal stripped fixture: %v", err)
	}
	s := state.New()
	s.ApplyV(stripped)

	view, ok := s.CyclicStoryView(now)
	if !ok || view.Valid || !view.EnterReady || view.OrdersObserved {
		t.Fatalf("expected enter-ready invalid view, got ok=%t view=%+v", ok, view)
	}
	policy := cyclicStoryPlannerPolicy(false, true, map[string]bool{cyclicStoryAutoClaimOrderRewardsKey: true}, 0)
	ops := cyclicStoryPlanOperations(BuildPlan(s, policy, now).Operations)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCActCyclicStoryEnter.String() || ops[0].BatchID != 9101 {
		t.Fatalf("expected enter without score/bag, got %+v", ops)
	}
}

func cyclicStoryPlannerPolicy(activityEnabled, moduleEnabled bool, bools map[string]bool, maxScore int64) *pb.Policy {
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Activity = &pb.ActivityPolicy{
		//nolint:staticcheck // Verify that the deprecated parent flag is ignored in favor of module gates.
		Enabled: activityEnabled,
		Modules: map[string]*pb.ActivityModulePolicy{
			cyclicStoryModuleKey: {
				Enabled:    moduleEnabled,
				BoolParams: bools,
				IntParams:  map[string]int64{cyclicStoryMaxScoreKey: maxScore},
			},
		},
	}
	return p
}

func cyclicStoryPlanOperations(ops []PlannedOp) []PlannedOp {
	out := make([]PlannedOp, 0, len(ops))
	for _, op := range ops {
		if op.Domain == "activity.actCyclicStory" {
			out = append(out, op)
		}
	}
	return out
}

func applyCyclicStoryPlannerFixture(t *testing.T) *state.State {
	t.Helper()
	raw, err := os.ReadFile("../state/testdata/cyclic_story_activity.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	s := state.New()
	s.ApplyV(raw)
	return s
}
