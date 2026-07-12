package runner

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestDessertRequestsSerializeOnlyCaptureConfirmedFields(t *testing.T) {
	enter, err := dessertEnterRequest(&automation.PlannedOp{Kind: clientproto.RPCActDessertEnter.String(), BatchID: 9101})
	if err != nil || jsonString(t, enter) != `{"batchId":9101}` {
		t.Fatalf("enter=%s err=%v", jsonString(t, enter), err)
	}
	claim, err := dessertTaskClaimRequest(&automation.PlannedOp{Kind: clientproto.RPCActRecv.String(), BatchID: 9101, TaskID: 1})
	if err != nil || jsonString(t, claim) != `{"batchId":9101,"taskIdx":0,"taskId":1}` {
		t.Fatalf("claim=%s err=%v", jsonString(t, claim), err)
	}
	syncReq, err := dessertCelebritySyncRequest(&automation.PlannedOp{Kind: clientproto.RPCCelebrityGetAllTypesInfo.String(), BatchID: 9101})
	if err != nil || jsonString(t, syncReq) != `{}` {
		t.Fatalf("sync=%s err=%v", jsonString(t, syncReq), err)
	}
	like, err := dessertCelebrityLikeRequest(&automation.PlannedOp{Kind: clientproto.RPCCelebrityLikeCelebrity.String(), BatchID: 9101})
	if err != nil || jsonString(t, like) != `{"type":5601}` {
		t.Fatalf("like=%s err=%v", jsonString(t, like), err)
	}

	for name, mutate := range map[string]func(*automation.PlannedOp){
		"diamond":    func(op *automation.PlannedOp) { op.DiamondCost = 1 },
		"item cost":  func(op *automation.PlannedOp) { op.ItemCost = map[int32]int32{1342: 1} },
		"target uid": func(op *automation.PlannedOp) { op.TargetUID = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			for _, base := range []automation.PlannedOp{
				{Kind: clientproto.RPCActDessertEnter.String(), BatchID: 9101},
				{Kind: clientproto.RPCActRecv.String(), BatchID: 9101, TaskID: 1},
				{Kind: clientproto.RPCCelebrityGetAllTypesInfo.String(), BatchID: 9101},
				{Kind: clientproto.RPCCelebrityLikeCelebrity.String(), BatchID: 9101},
			} {
				op := base
				mutate(&op)
				var err error
				switch op.Kind {
				case clientproto.RPCActDessertEnter.String():
					_, err = dessertEnterRequest(&op)
				case clientproto.RPCActRecv.String():
					_, err = dessertTaskClaimRequest(&op)
				case clientproto.RPCCelebrityGetAllTypesInfo.String():
					_, err = dessertCelebritySyncRequest(&op)
				case clientproto.RPCCelebrityLikeCelebrity.String():
					_, err = dessertCelebrityLikeRequest(&op)
				}
				if err == nil {
					t.Fatalf("%s accepted unsafe metadata: %+v", op.Kind, op)
				}
			}
		})
	}
}

func TestDessertOperationRegistryStrictAllowlist(t *testing.T) {
	for _, rpc := range []clientproto.RPCName{
		clientproto.RPCActDessertEnter,
		clientproto.RPCActRecv,
		clientproto.RPCCelebrityGetAllTypesInfo,
		clientproto.RPCCelebrityLikeCelebrity,
	} {
		spec, ok := operationSpecFor(rpc.String())
		if !ok || spec.args == nil || spec.run == nil {
			t.Fatalf("safe dessert RPC %s is not fully registered", rpc)
		}
	}
	for _, rpc := range []clientproto.RPCName{
		clientproto.RPCActRecvBoxes,
		clientproto.RPCActDessertOpenBox,
		clientproto.RPCActDessertGameStart,
		clientproto.RPCActDessertGameSync,
		clientproto.RPCActDessertGameOver,
		clientproto.RPCActDessertGiftBuy,
	} {
		if _, ok := operationSpecFor(rpc.String()); ok {
			t.Fatalf("unconfirmed or unsafe dessert RPC %s was registered", rpc)
		}
	}
}

func TestDessertExecutorsApplyExactlyOnceAndEnforcePostconditions(t *testing.T) {
	before := state.DessertTaskClaimSnapshot{BatchID: 9101, TaskIndex: 0, TaskID: 1}
	applyCount := 0
	_, err := executeDessertTaskClaim(context.Background(), clientproto.ActRecvRequest{}, dessertTaskClaimExecution{
		preflight: func() (state.DessertTaskClaimSnapshot, error) { return before, nil },
		recv: func(context.Context, clientproto.ActRecvRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"23":{"3":{}}}`), nil
		},
		apply: func(json.RawMessage) { applyCount++ },
		applied: func(got state.DessertTaskClaimSnapshot) bool {
			return got.BatchID == before.BatchID && got.TaskID == before.TaskID
		},
	})
	if err != nil || applyCount != 1 {
		t.Fatalf("task executor err=%v applyCount=%d", err, applyCount)
	}

	applyCount = 0
	_, err = executeDessertCelebritySync(context.Background(), clientproto.CelebrityGetAllTypesInfoRequest{}, dessertCelebritySyncExecution{
		preflight: func() (state.DessertCelebritySyncSnapshot, error) {
			return state.DessertCelebritySyncSnapshot{BatchID: 9101}, nil
		},
		sync: func(context.Context, clientproto.CelebrityGetAllTypesInfoRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"166":{"2":{}}}`), nil
		},
		apply:   func(json.RawMessage) { applyCount++ },
		applied: func(state.DessertCelebritySyncSnapshot) bool { return true },
		mark:    func(int32) {},
	})
	if err == nil || !strings.Contains(err.Error(), "full type and ranking") || applyCount != 0 {
		t.Fatalf("incomplete full-sync err=%v applyCount=%d", err, applyCount)
	}

	marked := int32(0)
	_, err = executeDessertCelebritySync(context.Background(), clientproto.CelebrityGetAllTypesInfoRequest{}, dessertCelebritySyncExecution{
		preflight: func() (state.DessertCelebritySyncSnapshot, error) {
			return state.DessertCelebritySyncSnapshot{BatchID: 9101}, nil
		},
		sync: func(context.Context, clientproto.CelebrityGetAllTypesInfoRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"165":{"0":[5601],"1":{"5601":[]}}}`), nil
		},
		apply:   func(json.RawMessage) { applyCount++ },
		applied: func(state.DessertCelebritySyncSnapshot) bool { return true },
		mark:    func(batchID int32) { marked = batchID },
	})
	if err != nil || applyCount != 1 || marked != 9101 {
		t.Fatalf("full-sync err=%v applyCount=%d marked=%d", err, applyCount, marked)
	}
}

func TestDessertPolicyRequiresEveryEnableLayer(t *testing.T) {
	policy := automation.DefaultPolicy()
	r := &Runner{policy: policy}
	if r.dessertEnterAutomationEnabled() || r.dessertTaskClaimAutomationEnabled() || r.dessertCelebrityLikeAutomationEnabled() {
		t.Fatal("default policy enabled dessert execution")
	}

	policy.AutomationEnabled = true
	policy.Activity.Enabled = true
	policy.Activity.Modules = map[string]*pb.ActivityModulePolicy{
		dessertModuleID: {Enabled: true, BoolParams: map[string]bool{}},
	}
	if r.dessertEnterAutomationEnabled() {
		t.Fatal("missing action bool enabled dessert enter")
	}
	policy.Activity.Modules[dessertModuleID].BoolParams[dessertAutoClaimTaskRewardsPolicy] = true
	if !r.dessertEnterAutomationEnabled() || !r.dessertTaskClaimAutomationEnabled() || r.dessertCelebrityLikeAutomationEnabled() {
		t.Fatal("task flag did not gate exact dessert actions")
	}
	policy.Activity.Modules[dessertModuleID].BoolParams[dessertAutoLikeCelebrityPolicy] = true
	if !r.dessertCelebrityLikeAutomationEnabled() {
		t.Fatal("like flag did not enable controlled sync/like chain")
	}
	policy.Activity.Enabled = false
	if r.dessertEnterAutomationEnabled() || r.dessertTaskClaimAutomationEnabled() || r.dessertCelebrityLikeAutomationEnabled() {
		t.Fatal("disabled Activity parent allowed dessert actions")
	}
}

func TestDessertCelebrityFullSyncDeltaPrefersCanonicalNamespace(t *testing.T) {
	for _, raw := range []json.RawMessage{
		json.RawMessage(`{"166":{"0":[5601],"1":{"5601":[]}}}`),
		json.RawMessage(`{"165":{"0":[5601],"1":{"5601":[]}}}`),
	} {
		if !dessertCelebrityFullSyncDelta(raw) {
			t.Fatalf("full sync rejected: %s", raw)
		}
	}
	if dessertCelebrityFullSyncDelta(json.RawMessage(`{"166":{"2":{}},"165":{"0":[5601],"1":{"5601":[]}}}`)) {
		t.Fatal("sparse canonical namespace incorrectly fell back to legacy full state")
	}
}
