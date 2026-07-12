package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

type randomEventEnterExecution struct {
	preflight func() error
	enter     func(context.Context, clientproto.RandomEventEnterRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	ready     func() bool
}

type randomEventClaimExecution struct {
	preflight func() (state.RandomEventClaimSnapshot, error)
	claim     func(context.Context, clientproto.RandomEventDoAffairRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.RandomEventClaimSnapshot) bool
}

func randomEventEnterRequest(op *automation.PlannedOp) (clientproto.RandomEventEnterRequest, error) {
	if op == nil || op.Kind != clientproto.RPCRandomEventEnter.String() {
		return clientproto.RandomEventEnterRequest{}, fmt.Errorf("randomEvent.enter requires an enter operation")
	}
	if randomEventOperationHasWireMetadata(op) || op.TargetID != 0 {
		return clientproto.RandomEventEnterRequest{}, fmt.Errorf("randomEvent.enter requires empty, cost-free metadata")
	}
	return clientproto.RandomEventEnterRequest{}, nil
}

func randomEventClaimRequest(op *automation.PlannedOp) (clientproto.RandomEventDoAffairRequest, error) {
	if op == nil || op.Kind != clientproto.RPCRandomEventDoAffair.String() || op.TargetID <= 0 {
		return clientproto.RandomEventDoAffairRequest{}, fmt.Errorf("randomEvent.doAffair requires a positive event target")
	}
	if randomEventOperationHasWireMetadata(op) {
		return clientproto.RandomEventDoAffairRequest{}, fmt.Errorf("randomEvent.doAffair requires event-only, cost-free metadata")
	}
	return clientproto.RandomEventDoAffairRequest{EventId: op.TargetID}, nil
}

func randomEventOperationHasWireMetadata(op *automation.PlannedOp) bool {
	return op == nil || op.TargetUID != 0 || len(op.TargetUIDs) != 0 ||
		op.BatchID != 0 || op.SlotID != 0 || op.TaskID != 0 || op.MilestoneIndex != 0 ||
		op.ItemID != 0 || op.Count != 0 || op.FlowerID != 0 || op.VaseID != 0 ||
		len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0 ||
		op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 || len(op.CostGates) != 0
}

func runRandomEventEnter(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := randomEventEnterRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("random event enter runner state or RPC unavailable")
	}
	exec := randomEventEnterExecution{
		preflight: func() error {
			if !rt.runner.randomEventAutomationEnabled() {
				return fmt.Errorf("random event automation is disabled")
			}
			observed, valid, _ := rt.runner.state.RandomEventMapStatus()
			if observed && valid {
				return fmt.Errorf("randomEvent.enter preflight rejected: event table is already valid")
			}
			return nil
		},
		enter: func(ctx context.Context, request clientproto.RandomEventEnterRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.RandomEvent().Enter(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply: rt.runner.state.ApplyV,
		ready: rt.runner.state.RandomEventTableReady,
	}
	return executeRandomEventEnter(ctx, req, exec)
}

func runRandomEventClaim(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := randomEventClaimRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("random event claim runner state or RPC unavailable")
	}
	exec := randomEventClaimExecution{
		preflight: func() (state.RandomEventClaimSnapshot, error) {
			if !rt.runner.randomEventAutomationEnabled() {
				return state.RandomEventClaimSnapshot{}, fmt.Errorf("random event automation is disabled")
			}
			snapshot, ok := rt.runner.state.RandomEventClaimSnapshot(op.TargetID)
			if !ok {
				return state.RandomEventClaimSnapshot{}, fmt.Errorf("randomEvent.doAffair preflight rejected: exact event is missing or unsafe")
			}
			if snapshot.EventID != req.EventId {
				return state.RandomEventClaimSnapshot{}, fmt.Errorf("random event target changed: planned=%d live=%d", req.EventId, snapshot.EventID)
			}
			return snapshot, nil
		},
		claim: func(ctx context.Context, request clientproto.RandomEventDoAffairRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.RandomEvent().DoAffair(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:   rt.runner.state.ApplyV,
		applied: rt.runner.state.RandomEventClaimApplied,
	}
	return executeRandomEventClaim(ctx, req, exec)
}

func executeRandomEventEnter(ctx context.Context, req clientproto.RandomEventEnterRequest, exec randomEventEnterExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.enter == nil || exec.apply == nil || exec.ready == nil {
		return nil, fmt.Errorf("random event enter execution is incomplete")
	}
	if err := exec.preflight(); err != nil {
		return nil, err
	}
	raw, err := exec.enter(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("randomEvent.enter: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("randomEvent.enter postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.ready() {
		return nil, fmt.Errorf("randomEvent.enter postcondition failed: response did not produce a valid event table")
	}
	return raw, nil
}

func executeRandomEventClaim(ctx context.Context, req clientproto.RandomEventDoAffairRequest, exec randomEventClaimExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.claim == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("random event claim execution is incomplete")
	}
	snapshot, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	if snapshot.EventID != req.EventId {
		return nil, fmt.Errorf("random event claim preflight target mismatch")
	}
	raw, err := exec.claim(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("randomEvent.doAffair: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("randomEvent.doAffair postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(snapshot) {
		return nil, fmt.Errorf("randomEvent.doAffair postcondition failed: target event remains in the authoritative table")
	}
	return raw, nil
}

func (r *Runner) randomEventAutomationEnabled() bool {
	policy := r.Policy()
	return policy.GetAutomationEnabled() && policy.GetBasic().GetMapEventEnabled()
}
