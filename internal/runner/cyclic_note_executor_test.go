package runner

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const cyclicNoteRunnerFixtureNowMs int64 = 1783696000000

func TestCyclicNoteRequestsUseOnlyObservedWireFields(t *testing.T) {
	enterOp := &automation.PlannedOp{Kind: clientproto.RPCActCyclicNoteEnter.String(), BatchID: 9001}
	enter, err := cyclicNoteEnterRequest(enterOp)
	if err != nil || jsonString(t, enter) != `{"batchId":9001}` {
		t.Fatalf("enter request=%s err=%v", jsonString(t, enter), err)
	}

	taskOp := &automation.PlannedOp{
		Kind: clientproto.RPCActCyclicNoteRecvTaskRwd.String(), BatchID: 9001, SlotID: 1, TaskID: 4003,
	}
	task, err := cyclicNoteTaskClaimRequest(taskOp)
	if err != nil || jsonString(t, task) != `{"batchId":9001,"taskId":4003}` {
		t.Fatalf("task request=%s err=%v", jsonString(t, task), err)
	}

	milestoneOp := &automation.PlannedOp{
		Kind: clientproto.RPCActCyclicNoteRecv.String(), BatchID: 9001, MilestoneIndex: 2,
	}
	milestone, err := cyclicNoteMilestoneClaimRequest(milestoneOp)
	if err != nil || jsonString(t, milestone) != `{"batchId":9001,"idx":2}` {
		t.Fatalf("milestone request=%s err=%v", jsonString(t, milestone), err)
	}

	commonPollutants := map[string]func(*automation.PlannedOp){
		"target uid":   func(op *automation.PlannedOp) { op.TargetUID = 1 },
		"target uids":  func(op *automation.PlannedOp) { op.TargetUIDs = []int64{1} },
		"target id":    func(op *automation.PlannedOp) { op.TargetID = 1 },
		"item id":      func(op *automation.PlannedOp) { op.ItemID = 1 },
		"count":        func(op *automation.PlannedOp) { op.Count = 1 },
		"flower id":    func(op *automation.PlannedOp) { op.FlowerID = 1 },
		"vase id":      func(op *automation.PlannedOp) { op.VaseID = 1 },
		"land ids":     func(op *automation.PlannedOp) { op.LandIDs = []int32{1} },
		"slot ids":     func(op *automation.PlannedOp) { op.SlotIDs = []int32{1} },
		"flower ids":   func(op *automation.PlannedOp) { op.FlowerIDs = []int32{1} },
		"gold cost":    func(op *automation.PlannedOp) { op.GoldCost = 1 },
		"diamond cost": func(op *automation.PlannedOp) { op.DiamondCost = 1 },
		"item cost":    func(op *automation.PlannedOp) { op.ItemCost = map[int32]int32{1107: 1} },
	}
	for name, pollute := range commonPollutants {
		t.Run("reject common metadata "+name, func(t *testing.T) {
			for _, base := range []*automation.PlannedOp{enterOp, taskOp, milestoneOp} {
				op := *base
				pollute(&op)
				var err error
				switch op.Kind {
				case clientproto.RPCActCyclicNoteEnter.String():
					_, err = cyclicNoteEnterRequest(&op)
				case clientproto.RPCActCyclicNoteRecvTaskRwd.String():
					_, err = cyclicNoteTaskClaimRequest(&op)
				case clientproto.RPCActCyclicNoteRecv.String():
					_, err = cyclicNoteMilestoneClaimRequest(&op)
				}
				if err == nil {
					t.Fatalf("%s accepted unsafe metadata: %+v", op.Kind, op)
				}
			}
		})
	}

	invalid := []struct {
		name string
		run  func() error
	}{
		{name: "nil enter", run: func() error { _, err := cyclicNoteEnterRequest(nil); return err }},
		{name: "zero batch", run: func() error {
			_, err := cyclicNoteEnterRequest(&automation.PlannedOp{Kind: clientproto.RPCActCyclicNoteEnter.String()})
			return err
		}},
		{name: "enter task", run: func() error { op := *enterOp; op.TaskID = 1; _, err := cyclicNoteEnterRequest(&op); return err }},
		{name: "task zero slot", run: func() error { op := *taskOp; op.SlotID = 0; _, err := cyclicNoteTaskClaimRequest(&op); return err }},
		{name: "task milestone", run: func() error {
			op := *taskOp
			op.MilestoneIndex = 1
			_, err := cyclicNoteTaskClaimRequest(&op)
			return err
		}},
		{name: "milestone zero index", run: func() error {
			op := *milestoneOp
			op.MilestoneIndex = 0
			_, err := cyclicNoteMilestoneClaimRequest(&op)
			return err
		}},
		{name: "milestone task", run: func() error {
			op := *milestoneOp
			op.TaskID = 1
			_, err := cyclicNoteMilestoneClaimRequest(&op)
			return err
		}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil {
				t.Fatal("unsafe request metadata accepted")
			}
		})
	}
}

func TestCyclicNoteOperationRegistryAllowsOnlyFreeRPCs(t *testing.T) {
	for _, rpc := range []clientproto.RPCName{
		clientproto.RPCActCyclicNoteEnter,
		clientproto.RPCActCyclicNoteRecvTaskRwd,
		clientproto.RPCActCyclicNoteRecv,
	} {
		spec, ok := operationSpecFor(rpc.String())
		if !ok || spec.args == nil || spec.run == nil {
			t.Fatalf("safe cyclic-note RPC %s is not fully registered", rpc)
		}
	}
	for _, rpc := range []clientproto.RPCName{
		clientproto.RPCActCyclicNoteDirectRecvTaskRwd,
		clientproto.RPCActCyclicNoteReRandomTask,
		clientproto.RPCActCyclicNoteUnlockTaskSlot,
		clientproto.RPCActCyclicNoteGiftBuy,
		clientproto.RPCActCyclicNoteResetGiftCd,
	} {
		if _, ok := operationSpecFor(rpc.String()); ok {
			t.Fatalf("unsafe cyclic-note RPC %s was registered", rpc)
		}
	}
}

func TestExistingStrictRequestsRejectCyclicNoteTargets(t *testing.T) {
	type strictRequest struct {
		name  string
		valid func() *automation.PlannedOp
		build func(*automation.PlannedOp) error
	}
	requests := []strictRequest{
		{
			name:  "main task",
			valid: func() *automation.PlannedOp { return &automation.PlannedOp{TargetID: 910001} },
			build: func(op *automation.PlannedOp) error { _, err := mainTaskClaimRequest(op); return err },
		},
		{
			name:  "story enter",
			valid: func() *automation.PlannedOp { return &automation.PlannedOp{} },
			build: func(op *automation.PlannedOp) error { _, err := storyEnterRequest(op); return err },
		},
		{
			name: "story unlock",
			valid: func() *automation.PlannedOp {
				return &automation.PlannedOp{TargetID: 4101, ItemCost: map[int32]int32{56: 85}}
			},
			build: func(op *automation.PlannedOp) error { _, err := storyUnlockRequest(op); return err },
		},
		{
			name:  "pearl recv one key",
			valid: func() *automation.PlannedOp { return &automation.PlannedOp{} },
			build: func(op *automation.PlannedOp) error { _, err := pearlRecvOneKeyRequest(op); return err },
		},
		{
			name:  "pearl candidate details",
			valid: func() *automation.PlannedOp { return &automation.PlannedOp{TargetUIDs: []int64{2001}} },
			build: func(op *automation.PlannedOp) error { _, err := pearlCandidateDetailRequest(op); return err },
		},
		{
			name: "pearl hire",
			valid: func() *automation.PlannedOp {
				return &automation.PlannedOp{TargetID: 1, TargetUID: 2001, Count: 1, ItemCost: map[int32]int32{1003: 1}}
			},
			build: func(op *automation.PlannedOp) error { _, err := pearlHireRequest(op); return err },
		},
		{
			name:  "zoo handle event",
			valid: func() *automation.PlannedOp { return &automation.PlannedOp{TargetID: 7, ItemID: 42, Count: 1} },
			build: func(op *automation.PlannedOp) error { _, err := zooHandleEventRequest(op); return err },
		},
		{
			name:  "zoo read log",
			valid: func() *automation.PlannedOp { return &automation.PlannedOp{TargetID: 7, ItemID: 42} },
			build: func(op *automation.PlannedOp) error { _, err := zooReadLogRequest(op); return err },
		},
		{
			name:  "zoo souvenir reward",
			valid: func() *automation.PlannedOp { return &automation.PlannedOp{SlotIDs: []int32{1}, Count: 1} },
			build: func(op *automation.PlannedOp) error { _, err := zooRecvSouvenirRewardRequest(op); return err },
		},
		{
			name: "zoo food bowl",
			valid: func() *automation.PlannedOp {
				return &automation.PlannedOp{Kind: clientproto.RPCZooAddFoodstuff.String(), TargetID: 7, ItemID: 1501, Count: 1, ItemCost: map[int32]int32{1501: 1}}
			},
			build: func(op *automation.PlannedOp) error { _, err := operationArgs(op); return err },
		},
	}
	mutations := []struct {
		name   string
		mutate func(*automation.PlannedOp)
	}{
		{name: "batch", mutate: func(op *automation.PlannedOp) { op.BatchID = 9001 }},
		{name: "slot", mutate: func(op *automation.PlannedOp) { op.SlotID = 1 }},
		{name: "task", mutate: func(op *automation.PlannedOp) { op.TaskID = 4003 }},
		{name: "milestone", mutate: func(op *automation.PlannedOp) { op.MilestoneIndex = 2 }},
	}
	for _, request := range requests {
		t.Run(request.name+" accepts clean metadata", func(t *testing.T) {
			if err := request.build(request.valid()); err != nil {
				t.Fatalf("clean request rejected: %v", err)
			}
		})
		for _, mutation := range mutations {
			t.Run(request.name+" rejects "+mutation.name, func(t *testing.T) {
				op := request.valid()
				mutation.mutate(op)
				if err := request.build(op); err == nil {
					t.Fatalf("activity target accepted: %+v", op)
				}
			})
		}
	}
}

func TestCyclicNoteExecutionsApplyPayloadExactlyOnce(t *testing.T) {
	now := time.UnixMilli(cyclicNoteRunnerFixtureNowMs)

	t.Run("enter", func(t *testing.T) {
		s := cyclicNoteRunnerState(t, false)
		snapshot, ok := s.CyclicNoteEnterSnapshot(now)
		if !ok {
			t.Fatal("enter snapshot unavailable")
		}
		response := json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"0":[4003,2001,1006]}}}}}}`)
		applyCount := 0
		requestCount := 0
		_, err := executeCyclicNoteEnter(context.Background(), clientproto.ActCyclicNoteEnterRequest{BatchId: 9001}, cyclicNoteEnterExecution{
			preflight: func() (state.CyclicNoteEnterSnapshot, error) { return snapshot, nil },
			enter: func(_ context.Context, req clientproto.ActCyclicNoteEnterRequest) (json.RawMessage, error) {
				requestCount++
				if req.BatchId != 9001 {
					t.Fatalf("enter request=%+v", req)
				}
				return response, nil
			},
			apply:   func(raw json.RawMessage) { applyCount++; s.ApplyV(raw) },
			applied: s.CyclicNoteEnterApplied,
		})
		if err != nil || requestCount != 1 || applyCount != 1 {
			view, _ := s.CyclicNoteView(now)
			t.Fatalf("enter err=%v requests=%d apply=%d view=%+v", err, requestCount, applyCount, view)
		}
	})

	t.Run("task reward", func(t *testing.T) {
		s := cyclicNoteRunnerState(t, true)
		snapshot, ok := s.CyclicNoteTaskClaimSnapshot(now, 9001, 1, 4003)
		if !ok {
			t.Fatal("task claim snapshot unavailable")
		}
		response := json.RawMessage(`{"23":{"3":{"9001|0":{"5":{"4003":1}}}}}`)
		applyCount := 0
		requestCount := 0
		_, err := executeCyclicNoteTaskClaim(context.Background(), clientproto.ActCyclicNoteRecvTaskRwdRequest{BatchId: 9001, TaskId: 4003}, cyclicNoteTaskClaimExecution{
			preflight: func() (state.CyclicNoteTaskClaimSnapshot, error) { return snapshot, nil },
			recv: func(_ context.Context, req clientproto.ActCyclicNoteRecvTaskRwdRequest) (json.RawMessage, error) {
				requestCount++
				if req.BatchId != 9001 || req.TaskId != 4003 {
					t.Fatalf("task request=%+v", req)
				}
				return response, nil
			},
			apply:   func(raw json.RawMessage) { applyCount++; s.ApplyV(raw) },
			applied: s.CyclicNoteTaskClaimApplied,
		})
		if err != nil || requestCount != 1 || applyCount != 1 {
			t.Fatalf("task err=%v requests=%d apply=%d", err, requestCount, applyCount)
		}
	})

	t.Run("milestone reward", func(t *testing.T) {
		s := cyclicNoteRunnerState(t, true)
		s.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"11":120,"13":[]}}}}`))
		snapshot, ok := s.CyclicNoteMilestoneClaimSnapshot(now, 9001, 2)
		if !ok {
			t.Fatal("milestone claim snapshot unavailable")
		}
		response := json.RawMessage(`{"23":{"0":{"9001":{"13":[2]}}}}`)
		applyCount := 0
		requestCount := 0
		_, err := executeCyclicNoteMilestoneClaim(context.Background(), clientproto.ActCyclicNoteRecvRequest{BatchId: 9001, Idx: 2}, cyclicNoteMilestoneClaimExecution{
			preflight: func() (state.CyclicNoteMilestoneClaimSnapshot, error) { return snapshot, nil },
			recv: func(_ context.Context, req clientproto.ActCyclicNoteRecvRequest) (json.RawMessage, error) {
				requestCount++
				if req.BatchId != 9001 || req.Idx != 2 {
					t.Fatalf("milestone request=%+v", req)
				}
				return response, nil
			},
			apply:   func(raw json.RawMessage) { applyCount++; s.ApplyV(raw) },
			applied: s.CyclicNoteMilestoneClaimApplied,
		})
		if err != nil || requestCount != 1 || applyCount != 1 {
			t.Fatalf("milestone err=%v requests=%d apply=%d", err, requestCount, applyCount)
		}
	})
}

func TestCyclicNoteExecutionsFailClosedBeforeAndAfterRPC(t *testing.T) {
	t.Run("preflight prevents RPC", func(t *testing.T) {
		called := false
		_, err := executeCyclicNoteTaskClaim(context.Background(), clientproto.ActCyclicNoteRecvTaskRwdRequest{}, cyclicNoteTaskClaimExecution{
			preflight: func() (state.CyclicNoteTaskClaimSnapshot, error) {
				return state.CyclicNoteTaskClaimSnapshot{}, errors.New("stale")
			},
			recv: func(context.Context, clientproto.ActCyclicNoteRecvTaskRwdRequest) (json.RawMessage, error) {
				called = true
				return nil, nil
			},
			apply: func(json.RawMessage) {}, applied: func(state.CyclicNoteTaskClaimSnapshot) bool { return true },
		})
		if err == nil || called {
			t.Fatalf("err=%v called=%t", err, called)
		}
	})

	t.Run("RPC error does not apply", func(t *testing.T) {
		applied := false
		_, err := executeCyclicNoteTaskClaim(context.Background(), clientproto.ActCyclicNoteRecvTaskRwdRequest{}, cyclicNoteTaskClaimExecution{
			preflight: func() (state.CyclicNoteTaskClaimSnapshot, error) {
				return state.CyclicNoteTaskClaimSnapshot{BatchID: 1, SlotID: 1, TaskID: 1, Target: 1, Progress: 1}, nil
			},
			recv: func(context.Context, clientproto.ActCyclicNoteRecvTaskRwdRequest) (json.RawMessage, error) {
				return nil, errors.New("transport")
			},
			apply: func(json.RawMessage) { applied = true }, applied: func(state.CyclicNoteTaskClaimSnapshot) bool { return true },
		})
		if err == nil || !strings.Contains(err.Error(), "transport") || applied {
			t.Fatalf("err=%v applied=%t", err, applied)
		}
	})

	tests := []struct {
		name string
		run  func(json.RawMessage, *int) error
	}{
		{
			name: "enter",
			run: func(raw json.RawMessage, applies *int) error {
				_, err := executeCyclicNoteEnter(context.Background(), clientproto.ActCyclicNoteEnterRequest{}, cyclicNoteEnterExecution{
					preflight: func() (state.CyclicNoteEnterSnapshot, error) {
						return state.CyclicNoteEnterSnapshot{BatchID: 1, Phase: 2}, nil
					},
					enter: func(context.Context, clientproto.ActCyclicNoteEnterRequest) (json.RawMessage, error) { return raw, nil },
					apply: func(json.RawMessage) { *applies++ }, applied: func(state.CyclicNoteEnterSnapshot) bool { return false },
				})
				return err
			},
		},
		{
			name: "task",
			run: func(raw json.RawMessage, applies *int) error {
				_, err := executeCyclicNoteTaskClaim(context.Background(), clientproto.ActCyclicNoteRecvTaskRwdRequest{}, cyclicNoteTaskClaimExecution{
					preflight: func() (state.CyclicNoteTaskClaimSnapshot, error) {
						return state.CyclicNoteTaskClaimSnapshot{BatchID: 1, SlotID: 1, TaskID: 1, Target: 1, Progress: 1}, nil
					},
					recv: func(context.Context, clientproto.ActCyclicNoteRecvTaskRwdRequest) (json.RawMessage, error) {
						return raw, nil
					},
					apply: func(json.RawMessage) { *applies++ }, applied: func(state.CyclicNoteTaskClaimSnapshot) bool { return false },
				})
				return err
			},
		},
		{
			name: "milestone",
			run: func(raw json.RawMessage, applies *int) error {
				_, err := executeCyclicNoteMilestoneClaim(context.Background(), clientproto.ActCyclicNoteRecvRequest{}, cyclicNoteMilestoneClaimExecution{
					preflight: func() (state.CyclicNoteMilestoneClaimSnapshot, error) {
						return state.CyclicNoteMilestoneClaimSnapshot{BatchID: 1, MilestoneIndex: 1, Target: 1, Score: 1}, nil
					},
					recv:  func(context.Context, clientproto.ActCyclicNoteRecvRequest) (json.RawMessage, error) { return raw, nil },
					apply: func(json.RawMessage) { *applies++ }, applied: func(state.CyclicNoteMilestoneClaimSnapshot) bool { return false },
				})
				return err
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name+" empty response", func(t *testing.T) {
			applies := 0
			err := tc.run(json.RawMessage(`{}`), &applies)
			if err == nil || !strings.Contains(err.Error(), "payload is empty") || applies != 0 {
				t.Fatalf("err=%v applies=%d", err, applies)
			}
		})
		t.Run(tc.name+" unchanged response", func(t *testing.T) {
			applies := 0
			err := tc.run(json.RawMessage(`{"23":{"0":{}}}`), &applies)
			if err == nil || !strings.Contains(err.Error(), "postcondition failed") || applies != 1 {
				t.Fatalf("err=%v applies=%d", err, applies)
			}
			assertCyclicNoteErrorEntersCooldown(t, err)
		})
	}

	if _, err := executeCyclicNoteEnter(context.Background(), clientproto.ActCyclicNoteEnterRequest{}, cyclicNoteEnterExecution{}); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete execution err=%v", err)
	}
}

func TestCyclicNotePolicyFlagsAreExplicitAndIndependent(t *testing.T) {
	policy := automation.DefaultPolicy()
	r := &Runner{policy: policy}
	if r.cyclicNoteEnterAutomationEnabled() || r.cyclicNoteTaskClaimAutomationEnabled() || r.cyclicNoteMilestoneClaimAutomationEnabled() {
		t.Fatal("cyclic note automation enabled by default")
	}

	policy.AutomationEnabled = true
	policy.Activity.CyclicNote = &pb.CyclicNotePolicy{Enabled: true}
	if r.cyclicNoteEnterAutomationEnabled() || r.cyclicNoteTaskClaimAutomationEnabled() || r.cyclicNoteMilestoneClaimAutomationEnabled() {
		t.Fatal("missing bool keys did not default to false")
	}

	module := policy.Activity.CyclicNote
	module.SatisfyTasks = true
	if !r.cyclicNoteEnterAutomationEnabled() || r.cyclicNoteTaskClaimAutomationEnabled() || r.cyclicNoteMilestoneClaimAutomationEnabled() {
		t.Fatal("satisfy_tasks must permit only initialization")
	}
	module.AutoClaimTaskRewards = true
	if !r.cyclicNoteEnterAutomationEnabled() || !r.cyclicNoteTaskClaimAutomationEnabled() || r.cyclicNoteMilestoneClaimAutomationEnabled() {
		t.Fatal("task reward flag leaked into milestone claims")
	}
	module.AutoClaimProgressBoxes = true
	if !r.cyclicNoteMilestoneClaimAutomationEnabled() {
		t.Fatal("milestone flag did not enable milestone claim")
	}
	module.Enabled = false
	if r.cyclicNoteEnterAutomationEnabled() || r.cyclicNoteTaskClaimAutomationEnabled() || r.cyclicNoteMilestoneClaimAutomationEnabled() {
		t.Fatal("disabled module did not block cyclic note RPCs")
	}
}

func TestCyclicNoteOperationDiagnosticsAndRuntimeClassification(t *testing.T) {
	op := &automation.PlannedOp{
		Kind: clientproto.RPCActCyclicNoteRecvTaskRwd.String(), BatchID: 9001, SlotID: 2, TaskID: 2001,
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(operationPayload(op, nil, nil, nil)), &payload); err != nil {
		t.Fatalf("decode operation payload: %v", err)
	}
	if payload["batchId"] != float64(9001) || payload["slotId"] != float64(2) || payload["taskId"] != float64(2001) || payload["milestoneIndex"] != float64(0) {
		t.Fatalf("cyclic note payload=%v", payload)
	}
	if got := opKindDesc(op.Kind); got != "领取花笺集芳任务奖励" {
		t.Fatalf("opKindDesc=%q", got)
	}
	if got := operationTargetSuffix(op); got != " (活动批次=9001 槽位=2 任务=2001)" {
		t.Fatalf("operationTargetSuffix=%q", got)
	}
	if key, label, ok := runtimeTaskCompletion(op.Kind); !ok || key != "cyclic_note" || label != "花笺集芳任务" {
		t.Fatalf("runtime task classification=(%q,%q,%t)", key, label, ok)
	}
	for _, kind := range []string{clientproto.RPCActCyclicNoteEnter.String(), clientproto.RPCActCyclicNoteRecv.String()} {
		if _, _, ok := runtimeTaskCompletion(kind); ok {
			t.Fatalf("%s incorrectly counted as a completed task", kind)
		}
	}
}

func cyclicNoteRunnerState(t *testing.T, initialized bool) *state.State {
	t.Helper()
	raw, err := os.ReadFile("../state/testdata/cyclic_note_activity.json")
	if err != nil {
		t.Fatalf("read cyclic-note fixture: %v", err)
	}
	if !initialized {
		var top map[string]any
		if err := json.Unmarshal(raw, &top); err != nil {
			t.Fatalf("decode cyclic-note fixture: %v", err)
		}
		ns23 := top["23"].(map[string]any)
		batches := ns23["0"].(map[string]any)
		batch := batches["9001"].(map[string]any)
		ext := batch["14"].(map[string]any)
		cyclic := ext["105"].(map[string]any)
		delete(cyclic, "0")
		raw, err = json.Marshal(top)
		if err != nil {
			t.Fatalf("encode uninitialized cyclic-note fixture: %v", err)
		}
	}
	s := state.New()
	s.ApplyV(raw)
	return s
}

func assertCyclicNoteErrorEntersCooldown(t *testing.T, err error) {
	t.Helper()
	r := newOperationEventTestRunner()
	op := &automation.PlannedOp{
		OperationID: "cyclic-note-task:9001:1:4003",
		Kind:        clientproto.RPCActCyclicNoteRecvTaskRwd.String(),
		Lane:        automation.LaneSide,
		Category:    automation.CategoryActivity,
		Domain:      "activity.cyclicNote",
		Action:      "claim",
		BatchID:     9001,
		SlotID:      1,
		TaskID:      4003,
	}
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	got := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op}, err: err, finishedAt: now,
	})
	if !errors.Is(got, err) {
		t.Fatalf("handleOperationError=%v, want original error", got)
	}
	if _, cooling := r.operationCoolingDown(op, now.Add(time.Second)); !cooling {
		t.Fatal("cyclic note postcondition failure did not enter side-operation cooldown")
	}
}
