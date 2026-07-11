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

type mainTaskClaimExecution struct {
	preflight func() (state.MainTaskClaimSnapshot, error)
	recv      func(context.Context, clientproto.TaskMainRecvRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.MainTaskClaimSnapshot) bool
}

func mainTaskClaimRequest(op *automation.PlannedOp) (clientproto.TaskMainRecvRequest, error) {
	if op == nil || op.TargetID <= 0 {
		return clientproto.TaskMainRecvRequest{}, fmt.Errorf("taskMain.recv requires a catalog task target")
	}
	if op.TargetUID != 0 || len(op.TargetUIDs) != 0 || op.ItemID != 0 || op.Count != 0 ||
		op.FlowerID != 0 || op.VaseID != 0 || len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 ||
		len(op.FlowerIDs) != 0 || plannedOpHasCyclicNoteTargets(op) ||
		op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
		return clientproto.TaskMainRecvRequest{}, fmt.Errorf("taskMain.recv requires empty wire args and target-only metadata")
	}
	return clientproto.TaskMainRecvRequest{}, nil
}

func validateMainTaskClaimMetadata(op *automation.PlannedOp, snapshot state.MainTaskClaimSnapshot) error {
	if op == nil || op.TargetID != snapshot.TaskID {
		var planned int32
		if op != nil {
			planned = op.TargetID
		}
		return fmt.Errorf("main task claim target changed: planned=%d live=%d", planned, snapshot.TaskID)
	}
	if snapshot.Target <= 0 || snapshot.NextTaskID <= 0 || snapshot.Finished < snapshot.Target {
		return fmt.Errorf("main task claim snapshot is not ready")
	}
	return nil
}

func runMainTaskClaim(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := mainTaskClaimRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("main task claim runner state or RPC unavailable")
	}
	exec := mainTaskClaimExecution{
		preflight: func() (state.MainTaskClaimSnapshot, error) {
			if !rt.runner.mainTaskAutomationEnabled() {
				return state.MainTaskClaimSnapshot{}, fmt.Errorf("main task automation is disabled")
			}
			snapshot, ok := rt.runner.state.MainTaskClaimSnapshot()
			if !ok {
				return state.MainTaskClaimSnapshot{}, fmt.Errorf("taskMain.recv preflight rejected: task is not observed, ready, or unclaimed")
			}
			if err := validateMainTaskClaimMetadata(op, snapshot); err != nil {
				return state.MainTaskClaimSnapshot{}, err
			}
			return snapshot, nil
		},
		recv: func(ctx context.Context, request clientproto.TaskMainRecvRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.TaskMain().Recv(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:   rt.runner.state.ApplyV,
		applied: rt.runner.state.MainTaskClaimApplied,
	}
	return executeMainTaskClaim(ctx, req, exec)
}

func executeMainTaskClaim(ctx context.Context, req clientproto.TaskMainRecvRequest, exec mainTaskClaimExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.recv == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("main task claim execution is incomplete")
	}
	snapshot, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.recv(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("taskMain.recv: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("taskMain.recv postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(snapshot) {
		return nil, fmt.Errorf("taskMain.recv postcondition failed: exact next task and receipt were not both observed")
	}
	return raw, nil
}

func (r *Runner) mainTaskAutomationEnabled() bool {
	policy := r.Policy()
	return policy.GetAutomationEnabled() && policy.GetBasic().GetTask().GetMainEnabled()
}
