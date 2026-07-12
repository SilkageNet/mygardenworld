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

const dessertPlannerNowMs int64 = 1783819000000

func TestDessertPolicyDefaultsAndFutureSwitchesFailClosed(t *testing.T) {
	now := time.UnixMilli(dessertPlannerNowMs)
	s := dessertPlannerState(t, false)
	for _, tc := range []struct {
		name     string
		activity bool
		module   bool
		bools    map[string]bool
	}{
		{name: "missing bools", activity: true, module: true},
		{name: "activity disabled", module: true, bools: map[string]bool{dessertAutoClaimTaskRewardsKey: true}},
		{name: "module disabled", activity: true, bools: map[string]bool{dessertAutoClaimTaskRewardsKey: true}},
		{name: "future switches cannot execute", activity: true, module: true, bools: map[string]bool{
			dessertAutoClaimProgressBoxesKey: true, dessertAutoOpenRewardBoxesKey: true, "auto_play": true,
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			policy := dessertPlannerPolicy(tc.activity, tc.module, tc.bools)
			if got := dessertPlanOperations(BuildPlan(s, policy, now).Operations); len(got) != 0 {
				t.Fatalf("dessert operations=%+v, want none", got)
			}
		})
	}
}

func TestDessertAutoplayCannotProduceLiveGameOperations(t *testing.T) {
	now := time.UnixMilli(dessertPlannerNowMs)
	s := dessertPlannerState(t, false)
	policy := dessertPlannerPolicy(true, true, map[string]bool{"auto_play": true, "resume_existing_round": true})
	policy.Activity.Modules[dessertModuleKey].IntParams = map[string]int64{
		"mode": 1, "max_energy_per_session": 100, "min_energy_reserve": 0,
	}
	blocked := map[string]struct{}{
		clientproto.RPCActDessertGameStart.String(): {},
		clientproto.RPCActDessertGameSync.String():  {},
		clientproto.RPCActDessertGameOver.String():  {},
	}
	for _, op := range BuildPlan(s, policy, now).Operations {
		if _, forbidden := blocked[op.Kind]; forbidden {
			t.Fatalf("autoplay policy produced hard-blocked live game operation: %+v", op)
		}
	}
}

func TestDessertPlannerStrictOrderAndSharedCooldown(t *testing.T) {
	now := time.UnixMilli(dessertPlannerNowMs)
	s := dessertPlannerState(t, false)
	policy := dessertPlannerPolicy(true, true, map[string]bool{
		dessertAutoClaimTaskRewardsKey: true,
		dessertAutoLikeCelebrityKey:    true,
	})
	ops := dessertPlanOperations(BuildPlan(s, policy, now).Operations)
	if len(ops) != 1 {
		t.Fatalf("dessert operations=%+v, want one", ops)
	}
	claim := ops[0]
	if claim.Kind != clientproto.RPCActRecv.String() || claim.BatchID != 9101 || claim.TaskID != 1 || claim.SlotID != 0 ||
		claim.Action != "claim_task" || claim.Lane != LaneSide || claim.Priority != dessertPriority ||
		claim.OperationID != clientproto.RPCActRecv.String()+":9101:0:1" || claim.CooldownKey != dessertCooldownKey {
		t.Fatalf("first dessert task claim=%+v", claim)
	}

	likeOnly := dessertPlannerPolicy(true, true, map[string]bool{dessertAutoLikeCelebrityKey: true})
	syncOps := dessertPlanOperations(BuildPlan(s, likeOnly, now).Operations)
	if len(syncOps) != 1 || syncOps[0].Kind != clientproto.RPCCelebrityGetAllTypesInfo.String() ||
		syncOps[0].Action != "sync_celebrity" || syncOps[0].CooldownKey != dessertCooldownKey {
		t.Fatalf("controlled celebrity sync=%+v", syncOps)
	}
	s.MarkDessertCelebritySynced(9101)
	likeOps := dessertPlanOperations(BuildPlan(s, likeOnly, now).Operations)
	if len(likeOps) != 1 || likeOps[0].Kind != clientproto.RPCCelebrityLikeCelebrity.String() ||
		likeOps[0].Action != "like_celebrity" || likeOps[0].CooldownKey != dessertCooldownKey {
		t.Fatalf("free celebrity like=%+v", likeOps)
	}
}

func TestDessertPlannerEnterDependsOnlyOnMissingBagOrExtension(t *testing.T) {
	now := time.UnixMilli(dessertPlannerNowMs)
	s := dessertPlannerState(t, true)
	policy := dessertPlannerPolicy(true, true, map[string]bool{dessertAutoClaimTaskRewardsKey: true})
	ops := dessertPlanOperations(BuildPlan(s, policy, now).Operations)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCActDessertEnter.String() || ops[0].BatchID != 9101 ||
		ops[0].Action != "enter" || ops[0].CooldownKey != dessertCooldownKey {
		t.Fatalf("dessert enter=%+v", ops)
	}
}

func TestDessertFeatureCatalogUsesActionLevelSafety(t *testing.T) {
	want := map[string]string{
		"activity.actDessert.monitor":        PlanStatusManaged,
		"activity.actDessert.enter":          PlanStatusManaged,
		"activity.actDessert.claim_task":     PlanStatusManaged,
		"activity.actDessert.sync_celebrity": PlanStatusManaged,
		"activity.actDessert.like_celebrity": PlanStatusManaged,
		"activity.actDessert.progress_boxes": PlanStatusBlocked,
		"activity.actDessert.reward_boxes":   PlanStatusBlocked,
		"activity.actDessert.game":           PlanStatusBlocked,
	}
	for _, spec := range featureSpecs {
		status, exists := want[spec.ID]
		if !exists {
			continue
		}
		if spec.Status != status || (status == PlanStatusManaged && spec.Action != "observe" && !spec.Executable) ||
			(status == PlanStatusBlocked && (spec.Executable || len(spec.BlockedReasons) == 0)) {
			t.Fatalf("dessert feature=%+v", spec)
		}
		delete(want, spec.ID)
	}
	if len(want) != 0 {
		t.Fatalf("missing dessert action features: %v", want)
	}
}

func dessertPlannerPolicy(activityEnabled, moduleEnabled bool, bools map[string]bool) *pb.Policy {
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Activity.Enabled = activityEnabled
	policy.Activity.Modules = map[string]*pb.ActivityModulePolicy{
		dessertModuleKey: {Enabled: moduleEnabled, BoolParams: bools},
	}
	return policy
}

func dessertPlanOperations(ops []PlannedOp) []PlannedOp {
	out := make([]PlannedOp, 0, 1)
	for _, op := range ops {
		if op.Domain == "activity.actDessert" {
			out = append(out, op)
		}
	}
	return out
}

func dessertPlannerState(t *testing.T, removeBag bool) *state.State {
	t.Helper()
	raw, err := os.ReadFile("../state/testdata/dessert_activity.json")
	if err != nil {
		t.Fatalf("read dessert fixture: %v", err)
	}
	if removeBag {
		var root map[string]any
		if err := json.Unmarshal(raw, &root); err != nil {
			t.Fatalf("decode dessert fixture: %v", err)
		}
		ns23 := root["23"].(map[string]any)
		batches := ns23["0"].(map[string]any)
		batch := batches["9101"].(map[string]any)
		delete(batch, "12")
		raw, err = json.Marshal(root)
		if err != nil {
			t.Fatalf("encode dessert fixture: %v", err)
		}
	}
	s := state.New()
	s.ApplyV(raw)
	return s
}
