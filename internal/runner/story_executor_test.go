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

func validStoryUnlockOperation() *automation.PlannedOp {
	return &automation.PlannedOp{
		Kind: clientproto.RPCStoryMainUnlock.String(), Lane: automation.LaneSide,
		OperationID: clientproto.RPCStoryMainUnlock.String(), TargetID: 4101,
		ItemCost: map[int32]int32{56: 85},
	}
}

func TestStoryRequestsAreEmptyAndRejectUnexpectedMetadata(t *testing.T) {
	if _, err := storyEnterRequest(&automation.PlannedOp{Kind: clientproto.RPCStoryMainEnter.String()}); err != nil {
		t.Fatal(err)
	}
	for _, op := range []*automation.PlannedOp{
		{TargetID: 1}, {TargetUID: 1}, {TargetUIDs: []int64{1}}, {Count: 1},
		{ItemID: 1}, {GoldCost: 1}, {DiamondCost: 1}, {ItemCost: map[int32]int32{56: 1}},
	} {
		if _, err := storyEnterRequest(op); err == nil {
			t.Fatalf("unsafe story enter accepted: %+v", op)
		}
	}

	valid := validStoryUnlockOperation()
	request, err := storyUnlockRequest(valid)
	if err != nil {
		t.Fatal(err)
	}
	if raw, _ := json.Marshal(request); string(raw) != "{}" {
		t.Fatalf("story unlock wire request=%s, want {}", raw)
	}
	mutations := []func(*automation.PlannedOp){
		func(op *automation.PlannedOp) { op.TargetID = 0 },
		func(op *automation.PlannedOp) { op.TargetUID = 1 },
		func(op *automation.PlannedOp) { op.TargetUIDs = []int64{1} },
		func(op *automation.PlannedOp) { op.Count = 1 },
		func(op *automation.PlannedOp) { op.ItemID = 56 },
		func(op *automation.PlannedOp) { op.GoldCost = 1 },
		func(op *automation.PlannedOp) { op.DiamondCost = 1 },
		func(op *automation.PlannedOp) { op.ItemCost = nil },
		func(op *automation.PlannedOp) { op.ItemCost = map[int32]int32{56: 0} },
	}
	for _, mutate := range mutations {
		op := validStoryUnlockOperation()
		mutate(op)
		if _, err := storyUnlockRequest(op); err == nil {
			t.Fatalf("unsafe story unlock accepted: %+v", op)
		}
	}
}

func TestValidateStoryUnlockMetadataRejectsStaleTargetAndCost(t *testing.T) {
	snapshot := state.StoryUnlockSnapshot{
		Chapter: 32, SectionIdx: 0, SectionID: 4101,
		Cost:      []state.ItemCount{{ItemID: 56, Count: 85}},
		Inventory: map[int32]int32{56: 100},
	}
	if err := validateStoryUnlockMetadata(validStoryUnlockOperation(), snapshot); err != nil {
		t.Fatal(err)
	}
	wrongTarget := validStoryUnlockOperation()
	wrongTarget.TargetID = 4102
	if err := validateStoryUnlockMetadata(wrongTarget, snapshot); err == nil {
		t.Fatal("stale story target accepted")
	}
	wrongCost := validStoryUnlockOperation()
	wrongCost.ItemCost[56] = 84
	if err := validateStoryUnlockMetadata(wrongCost, snapshot); err == nil {
		t.Fatal("stale story cost accepted")
	}
}

func TestExecuteStoryEnterRequiresPayloadAndManualApply(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s := state.New()
		applyCount := 0
		raw := json.RawMessage(`{"7":{"101":{"1":32,"2":0}}}`)
		got, err := executeStoryEnter(context.Background(), clientproto.StoryMainEnterRequest{}, storyEnterExecution{
			preflight: func() error { return nil },
			enter:     func(context.Context, clientproto.StoryMainEnterRequest) (json.RawMessage, error) { return raw, nil },
			apply:     func(payload json.RawMessage) { applyCount++; s.ApplyV(payload) },
			ready:     s.StoryMainReady,
		})
		if err != nil || string(got) != string(raw) || applyCount != 1 {
			t.Fatalf("got=%s err=%v applyCount=%d", got, err, applyCount)
		}
	})

	t.Run("preflight prevents rpc", func(t *testing.T) {
		called := false
		_, err := executeStoryEnter(context.Background(), clientproto.StoryMainEnterRequest{}, storyEnterExecution{
			preflight: func() error { return errors.New("stale") },
			enter: func(context.Context, clientproto.StoryMainEnterRequest) (json.RawMessage, error) {
				called = true
				return nil, nil
			},
			apply: func(json.RawMessage) {}, ready: func() bool { return true },
		})
		if err == nil || called {
			t.Fatalf("err=%v called=%t", err, called)
		}
	})

	t.Run("empty payload and missing apply fail", func(t *testing.T) {
		base := storyEnterExecution{
			preflight: func() error { return nil },
			enter:     func(context.Context, clientproto.StoryMainEnterRequest) (json.RawMessage, error) { return nil, nil },
			apply:     func(json.RawMessage) {}, ready: func() bool { return true },
		}
		if _, err := executeStoryEnter(context.Background(), clientproto.StoryMainEnterRequest{}, base); err == nil || !strings.Contains(err.Error(), "payload is empty") {
			t.Fatalf("empty payload err=%v", err)
		}
		base.enter = func(context.Context, clientproto.StoryMainEnterRequest) (json.RawMessage, error) {
			return json.RawMessage(`{"7":{}}`), nil
		}
		base.apply = nil
		if _, err := executeStoryEnter(context.Background(), clientproto.StoryMainEnterRequest{}, base); err == nil || !strings.Contains(err.Error(), "incomplete") {
			t.Fatalf("missing apply err=%v", err)
		}
	})

	t.Run("rpc error does not apply", func(t *testing.T) {
		applied := false
		_, err := executeStoryEnter(context.Background(), clientproto.StoryMainEnterRequest{}, storyEnterExecution{
			preflight: func() error { return nil },
			enter: func(context.Context, clientproto.StoryMainEnterRequest) (json.RawMessage, error) {
				return nil, errors.New("transport")
			},
			apply: func(json.RawMessage) { applied = true }, ready: func() bool { return true },
		})
		if err == nil || applied {
			t.Fatalf("err=%v applied=%t", err, applied)
		}
	})
}

func TestExecuteStoryUnlockExactPostcondition(t *testing.T) {
	tests := []struct {
		name        string
		response    json.RawMessage
		wantSuccess bool
	}{
		{name: "success", response: json.RawMessage(`{"7":{"2":{"2":{"56":115}},"101":{"1":32,"2":1}}}`), wantSuccess: true},
		{name: "same progress", response: json.RawMessage(`{"7":{"2":{"2":{"56":115}},"101":{"1":32,"2":0}}}`)},
		{name: "jumped progress", response: json.RawMessage(`{"7":{"2":{"2":{"56":115}},"101":{"1":32,"2":2}}}`)},
		{name: "wrong decrement", response: json.RawMessage(`{"7":{"2":{"2":{"56":116}},"101":{"1":32,"2":1}}}`)},
		{name: "empty payload", response: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := state.New()
			s.ApplyV(json.RawMessage(`{"7":{"0":{"32":{"56":200}},"101":{"1":32,"2":0}}}`))
			applyCount, callCount := 0, 0
			_, err := executeStoryUnlock(context.Background(), clientproto.StoryMainUnlockRequest{}, storyUnlockExecution{
				preflight: func() (state.StoryUnlockSnapshot, error) {
					snapshot, ok := s.StoryUnlockSnapshot()
					if !ok {
						return state.StoryUnlockSnapshot{}, errors.New("snapshot unavailable")
					}
					return snapshot, nil
				},
				unlock: func(context.Context, clientproto.StoryMainUnlockRequest) (json.RawMessage, error) {
					callCount++
					return tc.response, nil
				},
				apply:   func(raw json.RawMessage) { applyCount++; s.ApplyV(raw) },
				applied: s.StoryUnlockApplied,
			})
			if tc.wantSuccess {
				if err != nil || callCount != 1 || applyCount != 1 {
					t.Fatalf("success err=%v calls=%d apply=%d", err, callCount, applyCount)
				}
				return
			}
			if err == nil || callCount != 1 {
				t.Fatalf("failure err=%v calls=%d", err, callCount)
			}
			if tc.response == nil && applyCount != 0 {
				t.Fatalf("empty payload applyCount=%d", applyCount)
			}
			if tc.response != nil && applyCount != 1 {
				t.Fatalf("response applyCount=%d", applyCount)
			}
		})
	}
}

func TestExecuteStoryUnlockPreflightFailureDoesNotCallRPCAndCanCooldown(t *testing.T) {
	called := false
	_, err := executeStoryUnlock(context.Background(), clientproto.StoryMainUnlockRequest{}, storyUnlockExecution{
		preflight: func() (state.StoryUnlockSnapshot, error) {
			return state.StoryUnlockSnapshot{}, errors.New("insufficient")
		},
		unlock: func(context.Context, clientproto.StoryMainUnlockRequest) (json.RawMessage, error) {
			called = true
			return nil, nil
		},
		apply: func(json.RawMessage) {}, applied: func(state.StoryUnlockSnapshot) bool { return true },
	})
	if err == nil || called {
		t.Fatalf("err=%v called=%t", err, called)
	}

	op := validStoryUnlockOperation()
	r := &Runner{operationCooldowns: make(map[string]operationCooldown)}
	now := time.Now()
	r.setSideOperationCooldown(op, now, errors.New("postcondition"), "", 0)
	if _, cooling := r.operationCoolingDown(op, now.Add(time.Second)); !cooling {
		t.Fatal("story postcondition failure did not enter side-operation cooldown")
	}
	nextSection := *op
	nextSection.TargetID = 4102
	if _, cooling := r.operationCoolingDown(&nextSection, now.Add(time.Second)); !cooling {
		t.Fatal("ambiguous story response did not cool down the linear next section")
	}
}

func TestExecuteStoryUnlockAcceptsExactTerminalTransition(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"32":{"56":200}},"101":{"1":156,"2":5}}}`))
	raw := json.RawMessage(`{"7":{"2":{"2":{"56":5}},"101":{"1":157,"2":0}}}`)
	applyCount := 0
	_, err := executeStoryUnlock(context.Background(), clientproto.StoryMainUnlockRequest{}, storyUnlockExecution{
		preflight: func() (state.StoryUnlockSnapshot, error) {
			snapshot, ok := s.StoryUnlockSnapshot()
			if !ok {
				return state.StoryUnlockSnapshot{}, errors.New("snapshot unavailable")
			}
			return snapshot, nil
		},
		unlock:  func(context.Context, clientproto.StoryMainUnlockRequest) (json.RawMessage, error) { return raw, nil },
		apply:   func(payload json.RawMessage) { applyCount++; s.ApplyV(payload) },
		applied: s.StoryUnlockApplied,
	})
	story, _ := s.StoryMain()
	if err != nil || applyCount != 1 || !story.Valid || !story.Complete || story.Chapter != 157 || story.SectionIdx != 0 {
		t.Fatalf("err=%v apply=%d story=%+v", err, applyCount, story)
	}
}
