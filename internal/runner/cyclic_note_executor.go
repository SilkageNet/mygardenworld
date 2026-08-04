package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	cyclicNoteModuleID                   = "cyclicNote"
	cyclicNoteAutoClaimTaskRewardsPolicy = "auto_claim_task_rewards"
	cyclicNoteAutoClaimProgressPolicy    = "auto_claim_progress_boxes"
	cyclicNoteSatisfyTasksPolicy         = "satisfy_tasks"
)

type cyclicNoteEnterExecution struct {
	preflight func() (state.CyclicNoteEnterSnapshot, error)
	enter     func(context.Context, clientproto.ActCyclicNoteEnterRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.CyclicNoteEnterSnapshot) bool
}

type cyclicNoteTaskClaimExecution struct {
	preflight func() (state.CyclicNoteTaskClaimSnapshot, error)
	recv      func(context.Context, clientproto.ActCyclicNoteRecvTaskRwdRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.CyclicNoteTaskClaimSnapshot) bool
}

type cyclicNoteMilestoneClaimExecution struct {
	preflight func() (state.CyclicNoteMilestoneClaimSnapshot, error)
	recv      func(context.Context, clientproto.ActCyclicNoteRecvRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.CyclicNoteMilestoneClaimSnapshot) bool
}

func cyclicNoteEnterRequest(op *automation.PlannedOp) (clientproto.ActCyclicNoteEnterRequest, error) {
	if op == nil || op.Kind != clientproto.RPCActCyclicNoteEnter.String() || op.BatchID <= 0 {
		return clientproto.ActCyclicNoteEnterRequest{}, fmt.Errorf("actCyclicNote.enter requires a positive batch target")
	}
	if cyclicNoteOperationHasCommonRequestFields(op) || op.SlotID != 0 || op.TaskID != 0 || op.MilestoneIndex != 0 {
		return clientproto.ActCyclicNoteEnterRequest{}, fmt.Errorf("actCyclicNote.enter requires batch-only, cost-free metadata")
	}
	return clientproto.ActCyclicNoteEnterRequest{BatchId: op.BatchID}, nil
}

func cyclicNoteTaskClaimRequest(op *automation.PlannedOp) (clientproto.ActCyclicNoteRecvTaskRwdRequest, error) {
	if op == nil || op.Kind != clientproto.RPCActCyclicNoteRecvTaskRwd.String() || op.BatchID <= 0 || op.SlotID <= 0 || op.TaskID <= 0 {
		return clientproto.ActCyclicNoteRecvTaskRwdRequest{}, fmt.Errorf("actCyclicNote.recvTaskRwd requires positive batch, slot, and task targets")
	}
	if cyclicNoteOperationHasCommonRequestFields(op) || op.MilestoneIndex != 0 {
		return clientproto.ActCyclicNoteRecvTaskRwdRequest{}, fmt.Errorf("actCyclicNote.recvTaskRwd requires exact task-only, cost-free metadata")
	}
	return clientproto.ActCyclicNoteRecvTaskRwdRequest{BatchId: op.BatchID, TaskId: op.TaskID}, nil
}

func cyclicNoteMilestoneClaimRequest(op *automation.PlannedOp) (clientproto.ActCyclicNoteRecvRequest, error) {
	if op == nil || op.Kind != clientproto.RPCActCyclicNoteRecv.String() || op.BatchID <= 0 || op.MilestoneIndex <= 0 {
		return clientproto.ActCyclicNoteRecvRequest{}, fmt.Errorf("actCyclicNote.recv requires positive batch and milestone targets")
	}
	if cyclicNoteOperationHasCommonRequestFields(op) || op.SlotID != 0 || op.TaskID != 0 {
		return clientproto.ActCyclicNoteRecvRequest{}, fmt.Errorf("actCyclicNote.recv requires exact milestone-only, cost-free metadata")
	}
	return clientproto.ActCyclicNoteRecvRequest{BatchId: op.BatchID, Idx: op.MilestoneIndex}, nil
}

func cyclicNoteOperationHasCommonRequestFields(op *automation.PlannedOp) bool {
	return op.TargetUID != 0 || len(op.TargetUIDs) != 0 || op.TargetID != 0 || op.ItemID != 0 || op.Count != 0 ||
		op.FlowerID != 0 || op.VaseID != 0 || len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0 ||
		op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0
}

func plannedOpHasCyclicNoteTargets(op *automation.PlannedOp) bool {
	return op != nil && (op.BatchID != 0 || op.SlotID != 0 || op.TaskID != 0 || op.MilestoneIndex != 0)
}

func validateCyclicNoteEnterMetadata(op *automation.PlannedOp, snapshot state.CyclicNoteEnterSnapshot) error {
	if op == nil || op.BatchID != snapshot.BatchID {
		var planned int32
		if op != nil {
			planned = op.BatchID
		}
		return fmt.Errorf("cyclic note enter batch changed: planned=%d live=%d", planned, snapshot.BatchID)
	}
	if snapshot.Phase != 2 && snapshot.Phase != 3 {
		return fmt.Errorf("cyclic note enter snapshot is outside the active or grace phase")
	}
	return nil
}

func validateCyclicNoteTaskClaimMetadata(op *automation.PlannedOp, snapshot state.CyclicNoteTaskClaimSnapshot) error {
	if op == nil || op.BatchID != snapshot.BatchID || op.SlotID != snapshot.SlotID || op.TaskID != snapshot.TaskID {
		return fmt.Errorf("cyclic note task target changed")
	}
	if snapshot.Target <= 0 || snapshot.Progress < snapshot.Target {
		return fmt.Errorf("cyclic note task snapshot is not ready")
	}
	return nil
}

func validateCyclicNoteMilestoneClaimMetadata(op *automation.PlannedOp, snapshot state.CyclicNoteMilestoneClaimSnapshot) error {
	if op == nil || op.BatchID != snapshot.BatchID || op.MilestoneIndex != snapshot.MilestoneIndex {
		return fmt.Errorf("cyclic note milestone target changed")
	}
	if snapshot.Target <= 0 || snapshot.Score < snapshot.Target {
		return fmt.Errorf("cyclic note milestone snapshot is not ready")
	}
	return nil
}

func runCyclicNoteEnter(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := cyclicNoteEnterRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("cyclic note enter runner state or RPC unavailable")
	}
	exec := cyclicNoteEnterExecution{
		preflight: func() (state.CyclicNoteEnterSnapshot, error) {
			if !rt.runner.cyclicNoteEnterAutomationEnabled() {
				return state.CyclicNoteEnterSnapshot{}, fmt.Errorf("cyclic note enter automation is disabled")
			}
			snapshot, ok := rt.runner.state.CyclicNoteEnterSnapshot(time.Now())
			if !ok {
				return state.CyclicNoteEnterSnapshot{}, fmt.Errorf("actCyclicNote.enter preflight rejected: activity is invalid, initialized, or outside an executable phase")
			}
			if err := validateCyclicNoteEnterMetadata(op, snapshot); err != nil {
				return state.CyclicNoteEnterSnapshot{}, err
			}
			return snapshot, nil
		},
		enter: func(ctx context.Context, request clientproto.ActCyclicNoteEnterRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.ActCyclicNote().Enter(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:   rt.runner.state.ApplyV,
		applied: rt.runner.state.CyclicNoteEnterApplied,
	}
	return executeCyclicNoteEnter(ctx, req, exec)
}

func runCyclicNoteTaskClaim(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := cyclicNoteTaskClaimRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("cyclic note task claim runner state or RPC unavailable")
	}
	exec := cyclicNoteTaskClaimExecution{
		preflight: func() (state.CyclicNoteTaskClaimSnapshot, error) {
			if !rt.runner.cyclicNoteTaskClaimAutomationEnabled() {
				return state.CyclicNoteTaskClaimSnapshot{}, fmt.Errorf("cyclic note task reward automation is disabled")
			}
			snapshot, ok := rt.runner.state.CyclicNoteTaskClaimSnapshot(time.Now(), op.BatchID, op.SlotID, op.TaskID)
			if !ok {
				return state.CyclicNoteTaskClaimSnapshot{}, fmt.Errorf("actCyclicNote.recvTaskRwd preflight rejected: exact task is not uniquely ready and unclaimed")
			}
			if err := validateCyclicNoteTaskClaimMetadata(op, snapshot); err != nil {
				return state.CyclicNoteTaskClaimSnapshot{}, err
			}
			return snapshot, nil
		},
		recv: func(ctx context.Context, request clientproto.ActCyclicNoteRecvTaskRwdRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.ActCyclicNote().RecvTaskRwd(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:   rt.runner.state.ApplyV,
		applied: rt.runner.state.CyclicNoteTaskClaimApplied,
	}
	return executeCyclicNoteTaskClaim(ctx, req, exec)
}

func runCyclicNoteMilestoneClaim(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := cyclicNoteMilestoneClaimRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("cyclic note milestone claim runner state or RPC unavailable")
	}
	exec := cyclicNoteMilestoneClaimExecution{
		preflight: func() (state.CyclicNoteMilestoneClaimSnapshot, error) {
			if !rt.runner.cyclicNoteMilestoneClaimAutomationEnabled() {
				return state.CyclicNoteMilestoneClaimSnapshot{}, fmt.Errorf("cyclic note milestone reward automation is disabled")
			}
			snapshot, ok := rt.runner.state.CyclicNoteMilestoneClaimSnapshot(time.Now(), op.BatchID, op.MilestoneIndex)
			if !ok {
				return state.CyclicNoteMilestoneClaimSnapshot{}, fmt.Errorf("actCyclicNote.recv preflight rejected: exact milestone is not ready and unclaimed")
			}
			if err := validateCyclicNoteMilestoneClaimMetadata(op, snapshot); err != nil {
				return state.CyclicNoteMilestoneClaimSnapshot{}, err
			}
			return snapshot, nil
		},
		recv: func(ctx context.Context, request clientproto.ActCyclicNoteRecvRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.ActCyclicNote().Recv(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:   rt.runner.state.ApplyV,
		applied: rt.runner.state.CyclicNoteMilestoneClaimApplied,
	}
	return executeCyclicNoteMilestoneClaim(ctx, req, exec)
}

func executeCyclicNoteEnter(ctx context.Context, req clientproto.ActCyclicNoteEnterRequest, exec cyclicNoteEnterExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.enter == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("cyclic note enter execution is incomplete")
	}
	snapshot, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.enter(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("actCyclicNote.enter: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("actCyclicNote.enter postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(snapshot) {
		return nil, fmt.Errorf("actCyclicNote.enter postcondition failed: exact batch task list was not initialized")
	}
	return raw, nil
}

func executeCyclicNoteTaskClaim(ctx context.Context, req clientproto.ActCyclicNoteRecvTaskRwdRequest, exec cyclicNoteTaskClaimExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.recv == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("cyclic note task claim execution is incomplete")
	}
	snapshot, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.recv(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("actCyclicNote.recvTaskRwd: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("actCyclicNote.recvTaskRwd postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(snapshot) {
		return nil, fmt.Errorf("actCyclicNote.recvTaskRwd postcondition failed: exact receipt or task replacement was not observed")
	}
	return raw, nil
}

func executeCyclicNoteMilestoneClaim(ctx context.Context, req clientproto.ActCyclicNoteRecvRequest, exec cyclicNoteMilestoneClaimExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.recv == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("cyclic note milestone claim execution is incomplete")
	}
	snapshot, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.recv(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("actCyclicNote.recv: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("actCyclicNote.recv postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(snapshot) {
		return nil, fmt.Errorf("actCyclicNote.recv postcondition failed: exact milestone receipt was not observed")
	}
	return raw, nil
}

func (r *Runner) cyclicNoteAnyPolicyFlag(keys ...string) bool {
	policy := r.Policy()
	if !policy.GetAutomationEnabled() {
		return false
	}
	module := policy.GetActivity().GetModules()[cyclicNoteModuleID]
	if module == nil || !module.GetEnabled() {
		return false
	}
	for _, key := range keys {
		if module.GetBoolParams()[key] {
			return true
		}
	}
	return false
}

func (r *Runner) cyclicNoteEnterAutomationEnabled() bool {
	return r.cyclicNoteAnyPolicyFlag(cyclicNoteAutoClaimTaskRewardsPolicy, cyclicNoteSatisfyTasksPolicy)
}

func (r *Runner) cyclicNoteTaskClaimAutomationEnabled() bool {
	return r.cyclicNoteAnyPolicyFlag(cyclicNoteAutoClaimTaskRewardsPolicy)
}

func (r *Runner) cyclicNoteMilestoneClaimAutomationEnabled() bool {
	return r.cyclicNoteAnyPolicyFlag(cyclicNoteAutoClaimProgressPolicy)
}
