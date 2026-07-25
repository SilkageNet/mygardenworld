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

func TestRandomEventRequestsRejectUnexpectedMetadata(t *testing.T) {
	enter := &automation.PlannedOp{Kind: clientproto.RPCRandomEventEnter.String(), CooldownKey: "basic.map_event:sync"}
	if request, err := randomEventEnterRequest(enter); err != nil {
		t.Fatal(err)
	} else if raw, _ := json.Marshal(request); string(raw) != "{}" {
		t.Fatalf("enter request=%s", raw)
	}
	claim := &automation.PlannedOp{Kind: clientproto.RPCRandomEventDoAffair.String(), TargetID: 6004, CooldownKey: "basic.map_event:claim"}
	if request, err := randomEventClaimRequest(claim); err != nil {
		t.Fatal(err)
	} else if request.EventId != 6004 {
		t.Fatalf("claim request=%+v", request)
	}

	for _, mutate := range []func(*automation.PlannedOp){
		func(op *automation.PlannedOp) { op.TargetUID = 1 },
		func(op *automation.PlannedOp) { op.TargetUIDs = []int64{1} },
		func(op *automation.PlannedOp) { op.BatchID = 1 },
		func(op *automation.PlannedOp) { op.ItemID = 1 },
		func(op *automation.PlannedOp) { op.Count = 1 },
		func(op *automation.PlannedOp) { op.LandIDs = []int32{1} },
		func(op *automation.PlannedOp) { op.GoldCost = 1 },
		func(op *automation.PlannedOp) { op.DiamondCost = 1 },
		func(op *automation.PlannedOp) { op.ItemCost = map[int32]int32{1: 1} },
		func(op *automation.PlannedOp) {
			op.CostGates = []automation.CostGate{{ResourceKind: automation.GateResourceItem, Required: 1}}
		},
	} {
		op := *claim
		mutate(&op)
		if _, err := randomEventClaimRequest(&op); err == nil {
			t.Fatalf("unsafe claim metadata accepted: %+v", op)
		}
	}
	unsafeEnter := *enter
	unsafeEnter.TargetID = 6004
	if _, err := randomEventEnterRequest(&unsafeEnter); err == nil {
		t.Fatal("targeted randomEvent.enter metadata accepted")
	}
}

func TestExecuteRandomEventEnterAppliesOnceAndAcceptsValidEmptyTable(t *testing.T) {
	s := state.New()
	applyCount := 0
	var order []string
	raw := json.RawMessage(`{"129":{"0":{"1":{}}}}`)
	got, err := executeRandomEventEnter(context.Background(), clientproto.RandomEventEnterRequest{}, randomEventEnterExecution{
		preflight: func() error { order = append(order, "preflight"); return nil },
		enter: func(context.Context, clientproto.RandomEventEnterRequest) (json.RawMessage, error) {
			order = append(order, "enter")
			return raw, nil
		},
		apply: func(payload json.RawMessage) {
			order = append(order, "apply")
			applyCount++
			s.ApplyV(payload)
		},
		ready: func() bool { order = append(order, "postcondition"); return s.RandomEventTableReady() },
	})
	if err != nil || string(got) != string(raw) || applyCount != 1 {
		t.Fatalf("got=%s err=%v applyCount=%d", got, err, applyCount)
	}
	if joined := strings.Join(order, ","); joined != "preflight,enter,apply,postcondition" {
		t.Fatalf("execution order=%q", joined)
	}
}

func TestExecuteRandomEventEnterRejectsStaleOrInvalidResponse(t *testing.T) {
	t.Run("preflight prevents rpc", func(t *testing.T) {
		called := false
		_, err := executeRandomEventEnter(context.Background(), clientproto.RandomEventEnterRequest{}, randomEventEnterExecution{
			preflight: func() error { return errors.New("stale") },
			enter: func(context.Context, clientproto.RandomEventEnterRequest) (json.RawMessage, error) {
				called = true
				return nil, nil
			},
			apply: func(json.RawMessage) {}, ready: func() bool { return true },
		})
		if err == nil || called {
			t.Fatalf("err=%v called=%t", err, called)
		}
	})

	t.Run("malformed table", func(t *testing.T) {
		s := state.New()
		applyCount := 0
		_, err := executeRandomEventEnter(context.Background(), clientproto.RandomEventEnterRequest{}, randomEventEnterExecution{
			preflight: func() error { return nil },
			enter: func(context.Context, clientproto.RandomEventEnterRequest) (json.RawMessage, error) {
				return json.RawMessage(`{"129":{"0":{"1":[]}}}`), nil
			},
			apply: func(raw json.RawMessage) { applyCount++; s.ApplyV(raw) }, ready: s.RandomEventTableReady,
		})
		if err == nil || !strings.Contains(err.Error(), "postcondition failed") || applyCount != 1 {
			t.Fatalf("err=%v applyCount=%d", err, applyCount)
		}
	})
}

func TestExecuteRandomEventClaimAppliesOnceAndRequiresRemoval(t *testing.T) {
	newState := func() *state.State {
		s := state.New()
		s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{"6004":{"0":6004,"1":2,"2":60040901},"6008":{"0":6008,"1":0,"2":60080101}}}}}`))
		return s
	}

	t.Run("target removed", func(t *testing.T) {
		s := newState()
		applyCount := 0
		req := clientproto.RandomEventDoAffairRequest{EventId: 6004}
		got, err := executeRandomEventClaim(context.Background(), req, randomEventClaimExecution{
			preflight: func() (state.RandomEventClaimSnapshot, error) {
				snapshot, ok := s.RandomEventClaimSnapshot(6004)
				if !ok {
					return state.RandomEventClaimSnapshot{}, errors.New("missing")
				}
				return snapshot, nil
			},
			claim: func(_ context.Context, got clientproto.RandomEventDoAffairRequest) (json.RawMessage, error) {
				if got.EventId != 6004 {
					t.Fatalf("request=%+v", got)
				}
				return json.RawMessage(`{"129":{"0":{"1":{"6008":{"0":6008,"1":0,"2":60080101}}}}}`), nil
			},
			apply:   func(raw json.RawMessage) { applyCount++; s.ApplyV(raw) },
			applied: s.RandomEventClaimApplied,
		})
		if err != nil || !json.Valid(got) || applyCount != 1 {
			t.Fatalf("got=%s err=%v applyCount=%d", got, err, applyCount)
		}
		if ids := s.ReadyRandomEventIDs(); len(ids) != 1 || ids[0] != 6008 {
			t.Fatalf("remaining events=%v", ids)
		}
	})

	t.Run("target removed via null entry", func(t *testing.T) {
		s := newState()
		applyCount := 0
		got, err := executeRandomEventClaim(context.Background(), clientproto.RandomEventDoAffairRequest{EventId: 6004}, randomEventClaimExecution{
			preflight: func() (state.RandomEventClaimSnapshot, error) {
				snapshot, ok := s.RandomEventClaimSnapshot(6004)
				if !ok {
					return state.RandomEventClaimSnapshot{}, errors.New("missing")
				}
				return snapshot, nil
			},
			claim: func(context.Context, clientproto.RandomEventDoAffairRequest) (json.RawMessage, error) {
				return json.RawMessage(`{"129":{"0":{"1":{"6004":null,"6008":{"0":6008,"1":0,"2":60080101}}}}}`), nil
			},
			apply:   func(raw json.RawMessage) { applyCount++; s.ApplyV(raw) },
			applied: s.RandomEventClaimApplied,
		})
		if err != nil || !json.Valid(got) || applyCount != 1 {
			t.Fatalf("got=%s err=%v applyCount=%d", got, err, applyCount)
		}
		if ids := s.ReadyRandomEventIDs(); len(ids) != 1 || ids[0] != 6008 {
			t.Fatalf("remaining events=%v", ids)
		}
	})

	for name, response := range map[string]json.RawMessage{
		"sparse unchanged": json.RawMessage(`{"129":{"0":{"2":1234}}}`),
		"malformed":        json.RawMessage(`{"129":{"0":{"1":[]}}}`),
	} {
		t.Run(name, func(t *testing.T) {
			s := newState()
			applyCount := 0
			_, err := executeRandomEventClaim(context.Background(), clientproto.RandomEventDoAffairRequest{EventId: 6004}, randomEventClaimExecution{
				preflight: func() (state.RandomEventClaimSnapshot, error) {
					snapshot, ok := s.RandomEventClaimSnapshot(6004)
					if !ok {
						return state.RandomEventClaimSnapshot{}, errors.New("missing")
					}
					return snapshot, nil
				},
				claim: func(context.Context, clientproto.RandomEventDoAffairRequest) (json.RawMessage, error) {
					return response, nil
				},
				apply: func(raw json.RawMessage) { applyCount++; s.ApplyV(raw) }, applied: s.RandomEventClaimApplied,
			})
			if err == nil || !strings.Contains(err.Error(), "postcondition failed") || applyCount != 1 {
				t.Fatalf("err=%v applyCount=%d", err, applyCount)
			}
		})
	}
}

func TestRandomEventTargetsShareCooldownScope(t *testing.T) {
	r := newOperationEventTestRunner()
	first := &automation.PlannedOp{
		OperationID: "randomEvent.doAffair|6004", CooldownKey: "basic.map_event:claim",
		Kind: clientproto.RPCRandomEventDoAffair.String(), Lane: automation.LaneSide,
	}
	second := &automation.PlannedOp{
		OperationID: "randomEvent.doAffair|6008", CooldownKey: "basic.map_event:claim",
		Kind: clientproto.RPCRandomEventDoAffair.String(), Lane: automation.LaneSide,
	}
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	r.setSideOperationCooldown(first, now, errors.New("postcondition failed"), "", 0)
	if cd, cooling := r.operationCoolingDown(second, now.Add(time.Second)); !cooling || cd.OperationID != "basic.map_event:claim" {
		t.Fatalf("second event bypassed shared cooldown: cd=%+v cooling=%t", cd, cooling)
	}
}
