package runner

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func validMainTaskClaimOperation() *automation.PlannedOp {
	return &automation.PlannedOp{
		Kind:        clientproto.RPCTaskMainRecv.String(),
		OperationID: clientproto.RPCTaskMainRecv.String(),
		TargetID:    910001,
	}
}

func TestMainTaskClaimRequestIsEmptyAndStrict(t *testing.T) {
	req, err := mainTaskClaimRequest(validMainTaskClaimOperation())
	if err != nil {
		t.Fatal(err)
	}
	if raw, _ := json.Marshal(req); string(raw) != `{}` {
		t.Fatalf("taskMain.recv request=%s", raw)
	}
	mutations := []func(*automation.PlannedOp){
		func(op *automation.PlannedOp) { op.TargetID = 0 },
		func(op *automation.PlannedOp) { op.TargetUID = 1 },
		func(op *automation.PlannedOp) { op.TargetUIDs = []int64{1} },
		func(op *automation.PlannedOp) { op.ItemID = 1 },
		func(op *automation.PlannedOp) { op.Count = 1 },
		func(op *automation.PlannedOp) { op.FlowerID = 23001 },
		func(op *automation.PlannedOp) { op.VaseID = 1 },
		func(op *automation.PlannedOp) { op.LandIDs = []int32{1} },
		func(op *automation.PlannedOp) { op.SlotIDs = []int32{1} },
		func(op *automation.PlannedOp) { op.FlowerIDs = []int32{23001} },
		func(op *automation.PlannedOp) { op.GoldCost = 1 },
		func(op *automation.PlannedOp) { op.DiamondCost = 1 },
		func(op *automation.PlannedOp) { op.ItemCost = map[int32]int32{11: 1} },
	}
	for _, mutate := range mutations {
		op := validMainTaskClaimOperation()
		mutate(op)
		if _, err := mainTaskClaimRequest(op); err == nil {
			t.Fatalf("unsafe taskMain.recv accepted: %+v", op)
		}
	}
	if _, err := mainTaskClaimRequest(nil); err == nil {
		t.Fatal("nil taskMain.recv accepted")
	}
}

func TestValidateMainTaskClaimMetadataRejectsStaleTarget(t *testing.T) {
	snapshot := state.MainTaskClaimSnapshot{TaskID: 910001, Target: 14, NextTaskID: 920001, Finished: 14}
	if err := validateMainTaskClaimMetadata(validMainTaskClaimOperation(), snapshot); err != nil {
		t.Fatal(err)
	}
	stale := validMainTaskClaimOperation()
	stale.TargetID = 920001
	if err := validateMainTaskClaimMetadata(stale, snapshot); err == nil {
		t.Fatal("stale task target accepted")
	}
	for _, invalid := range []state.MainTaskClaimSnapshot{
		{TaskID: 910001, Target: 0, NextTaskID: 920001, Finished: 14},
		{TaskID: 910001, Target: 14, NextTaskID: 0, Finished: 14},
		{TaskID: 910001, Target: 14, NextTaskID: 920001, Finished: 13},
	} {
		if err := validateMainTaskClaimMetadata(validMainTaskClaimOperation(), invalid); err == nil {
			t.Fatalf("invalid snapshot accepted: %+v", invalid)
		}
	}
}

func TestExecuteMainTaskClaimManualApplyAndPostcondition(t *testing.T) {
	tests := []struct {
		name        string
		response    json.RawMessage
		wantSuccess bool
	}{
		{name: "success", response: json.RawMessage(`{"22":{"0":{"1":920001,"2":0,"4":{"910001":1}}}}`), wantSuccess: true},
		{name: "same task with receipt", response: json.RawMessage(`{"22":{"0":{"1":910001,"2":14,"4":{"910001":1}}}}`)},
		{name: "next without receipt", response: json.RawMessage(`{"22":{"0":{"1":920001,"2":0,"4":{}}}}`)},
		{name: "wrong next", response: json.RawMessage(`{"22":{"0":{"1":930001,"2":0,"4":{"910001":1}}}}`)},
		{name: "next progress missing", response: json.RawMessage(`{"22":{"0":{"1":920001,"4":{"910001":1}}}}`)},
		{name: "empty payload", response: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := readyMainTaskState()
			applyCount, callCount := 0, 0
			got, err := executeMainTaskClaim(context.Background(), clientproto.TaskMainRecvRequest{}, mainTaskClaimExecution{
				preflight: func() (state.MainTaskClaimSnapshot, error) {
					snapshot, ok := s.MainTaskClaimSnapshot()
					if !ok {
						return state.MainTaskClaimSnapshot{}, errors.New("snapshot unavailable")
					}
					return snapshot, nil
				},
				recv: func(context.Context, clientproto.TaskMainRecvRequest) (json.RawMessage, error) {
					callCount++
					return tc.response, nil
				},
				apply:   func(raw json.RawMessage) { applyCount++; s.ApplyV(raw) },
				applied: s.MainTaskClaimApplied,
			})
			if tc.wantSuccess {
				if err != nil || string(got) != string(tc.response) || callCount != 1 || applyCount != 1 {
					t.Fatalf("got=%s err=%v calls=%d apply=%d", got, err, callCount, applyCount)
				}
				return
			}
			if err == nil || callCount != 1 {
				t.Fatalf("failure err=%v calls=%d", err, callCount)
			}
			if tc.response == nil && applyCount != 0 {
				t.Fatalf("empty response apply=%d", applyCount)
			}
			if tc.response != nil && applyCount != 1 {
				t.Fatalf("response apply=%d", applyCount)
			}
		})
	}
}

func TestExecuteMainTaskClaimPreflightRPCAndCompletenessFailures(t *testing.T) {
	t.Run("preflight prevents rpc", func(t *testing.T) {
		called := false
		_, err := executeMainTaskClaim(context.Background(), clientproto.TaskMainRecvRequest{}, mainTaskClaimExecution{
			preflight: func() (state.MainTaskClaimSnapshot, error) {
				return state.MainTaskClaimSnapshot{}, errors.New("stale")
			},
			recv: func(context.Context, clientproto.TaskMainRecvRequest) (json.RawMessage, error) {
				called = true
				return nil, nil
			},
			apply: func(json.RawMessage) {}, applied: func(state.MainTaskClaimSnapshot) bool { return true },
		})
		if err == nil || called {
			t.Fatalf("err=%v called=%t", err, called)
		}
	})

	t.Run("rpc error does not apply", func(t *testing.T) {
		applied := false
		_, err := executeMainTaskClaim(context.Background(), clientproto.TaskMainRecvRequest{}, mainTaskClaimExecution{
			preflight: func() (state.MainTaskClaimSnapshot, error) {
				return state.MainTaskClaimSnapshot{TaskID: 910001, Target: 14, NextTaskID: 920001, Finished: 14}, nil
			},
			recv: func(context.Context, clientproto.TaskMainRecvRequest) (json.RawMessage, error) {
				return nil, errors.New("transport")
			},
			apply: func(json.RawMessage) { applied = true }, applied: func(state.MainTaskClaimSnapshot) bool { return true },
		})
		if err == nil || !strings.Contains(err.Error(), "transport") || applied {
			t.Fatalf("err=%v applied=%t", err, applied)
		}
	})

	if _, err := executeMainTaskClaim(context.Background(), clientproto.TaskMainRecvRequest{}, mainTaskClaimExecution{}); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete execution err=%v", err)
	}
}

func TestExecuteMainTaskClaimAcceptsExactTerminal(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"22":{"0":{"1":6940001,"2":144,"4":{}}}}`))
	raw := json.RawMessage(`{"22":{"0":{"1":6950001,"4":{"6940001":1}}}}`)
	_, err := executeMainTaskClaim(context.Background(), clientproto.TaskMainRecvRequest{}, mainTaskClaimExecution{
		preflight: func() (state.MainTaskClaimSnapshot, error) {
			snapshot, ok := s.MainTaskClaimSnapshot()
			if !ok {
				return state.MainTaskClaimSnapshot{}, errors.New("snapshot unavailable")
			}
			return snapshot, nil
		},
		recv:    func(context.Context, clientproto.TaskMainRecvRequest) (json.RawMessage, error) { return raw, nil },
		apply:   s.ApplyV,
		applied: s.MainTaskClaimApplied,
	})
	task, _ := s.MainTask()
	if err != nil || !task.Valid || !task.Complete || task.TaskID != 6950001 {
		t.Fatalf("err=%v task=%+v", err, task)
	}
}

func TestMainTaskClaimStableCooldownAcrossTaskSwitch(t *testing.T) {
	op := validMainTaskClaimOperation()
	r := &Runner{operationCooldowns: make(map[string]operationCooldown)}
	now := time.Now()
	r.setSideOperationCooldown(op, now, errors.New("ambiguous response"), "", 0)
	next := *op
	next.TargetID = 920001
	if _, cooling := r.operationCoolingDown(&next, now.Add(time.Second)); !cooling {
		t.Fatal("next main task bypassed stable side-operation cooldown")
	}
}

func TestMainTaskAutomationEnabledRequiresBothPolicyGates(t *testing.T) {
	policy := automation.DefaultPolicy()
	r := &Runner{policy: policy}
	if r.mainTaskAutomationEnabled() {
		t.Fatal("main task automation enabled by default")
	}
	policy.AutomationEnabled = true
	policy.Basic.Task.MainEnabled = true
	if !r.mainTaskAutomationEnabled() {
		t.Fatal("main task automation not enabled with both gates")
	}
	policy.Basic.Task.MainEnabled = false
	if r.mainTaskAutomationEnabled() {
		t.Fatal("main task automation ignored module gate")
	}
}

func readyMainTaskState() *state.State {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"22":{"0":{"1":910001,"2":14,"4":{}}}}`))
	return s
}
