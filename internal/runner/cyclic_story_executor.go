package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

type cyclicStoryEnterExecution struct {
	preflight func() (state.CyclicStoryEnterSnapshot, error)
	enter     func(context.Context, clientproto.ActCyclicStoryEnterRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.CyclicStoryEnterSnapshot) bool
}

type cyclicStoryOrderClaimExecution struct {
	preflight func() (state.CyclicStoryOrderClaimSnapshot, error)
	recv      func(context.Context, clientproto.ActCyclicStoryRecvOrderRwdRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.CyclicStoryOrderClaimSnapshot) bool
}

type cyclicStoryMilestoneClaimExecution struct {
	preflight func() (state.CyclicStoryMilestoneClaimSnapshot, error)
	recv      func(context.Context, clientproto.ActCyclicStoryRecvRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.CyclicStoryMilestoneClaimSnapshot) bool
}

func cyclicStoryEnterRequest(op *automation.PlannedOp) (clientproto.ActCyclicStoryEnterRequest, error) {
	if op == nil || op.Kind != clientproto.RPCActCyclicStoryEnter.String() || op.BatchID <= 0 {
		return clientproto.ActCyclicStoryEnterRequest{}, fmt.Errorf("actCyclicStory.enter requires a positive batch target")
	}
	if cyclicStoryOperationHasExtraFields(op) || op.SlotID != 0 || op.TaskID != 0 || op.MilestoneIndex != 0 ||
		op.FlowerID != 0 || len(op.ItemCost) != 0 {
		return clientproto.ActCyclicStoryEnterRequest{}, fmt.Errorf("actCyclicStory.enter requires batch-only, cost-free metadata")
	}
	return clientproto.ActCyclicStoryEnterRequest{BatchId: op.BatchID}, nil
}

func cyclicStoryOrderClaimRequest(op *automation.PlannedOp) (clientproto.ActCyclicStoryRecvOrderRwdRequest, error) {
	if op == nil || op.Kind != clientproto.RPCActCyclicStoryRecvOrderRwd.String() || op.BatchID <= 0 || op.TaskID <= 0 {
		return clientproto.ActCyclicStoryRecvOrderRwdRequest{}, fmt.Errorf("actCyclicStory.recvOrderRwd requires positive batch and order targets")
	}
	if op.SlotID < 0 || op.MilestoneIndex != 0 || op.FlowerID <= 0 || len(op.ItemCost) != 1 || op.ItemCost[op.FlowerID] <= 0 {
		return clientproto.ActCyclicStoryRecvOrderRwdRequest{}, fmt.Errorf("actCyclicStory.recvOrderRwd requires exact flower cost metadata")
	}
	if cyclicStoryOperationHasExtraFields(op) {
		return clientproto.ActCyclicStoryRecvOrderRwdRequest{}, fmt.Errorf("actCyclicStory.recvOrderRwd carries unexpected request metadata")
	}
	return clientproto.ActCyclicStoryRecvOrderRwdRequest{BatchId: op.BatchID, OrderIdx: op.SlotID}, nil
}

func cyclicStoryMilestoneClaimRequest(op *automation.PlannedOp) (clientproto.ActCyclicStoryRecvRequest, error) {
	if op == nil || op.Kind != clientproto.RPCActCyclicStoryRecv.String() || op.BatchID <= 0 || op.MilestoneIndex <= 0 {
		return clientproto.ActCyclicStoryRecvRequest{}, fmt.Errorf("actCyclicStory.recv requires positive batch and milestone targets")
	}
	if cyclicStoryOperationHasExtraFields(op) || op.SlotID != 0 || op.TaskID != 0 || op.FlowerID != 0 || len(op.ItemCost) != 0 {
		return clientproto.ActCyclicStoryRecvRequest{}, fmt.Errorf("actCyclicStory.recv requires exact milestone-only, cost-free metadata")
	}
	return clientproto.ActCyclicStoryRecvRequest{BatchId: op.BatchID, Idx: op.MilestoneIndex}, nil
}

func cyclicStoryOperationHasExtraFields(op *automation.PlannedOp) bool {
	return op.TargetUID != 0 || len(op.TargetUIDs) != 0 || op.TargetID != 0 || op.ItemID != 0 || op.Count != 0 ||
		op.VaseID != 0 || len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0 ||
		op.GoldCost != 0 || op.DiamondCost != 0
}

func validateCyclicStoryEnterMetadata(op *automation.PlannedOp, snapshot state.CyclicStoryEnterSnapshot) error {
	if op == nil || op.BatchID != snapshot.BatchID {
		var planned int32
		if op != nil {
			planned = op.BatchID
		}
		return fmt.Errorf("cyclic story enter batch changed: planned=%d live=%d", planned, snapshot.BatchID)
	}
	if snapshot.Phase != 2 && snapshot.Phase != 3 {
		return fmt.Errorf("cyclic story enter snapshot is outside the active or grace phase")
	}
	return nil
}

func validateCyclicStoryOrderClaimMetadata(op *automation.PlannedOp, snapshot state.CyclicStoryOrderClaimSnapshot) error {
	if op == nil || op.BatchID != snapshot.BatchID || op.SlotID != snapshot.OrderIdx || op.TaskID != snapshot.OrderID {
		return fmt.Errorf("cyclic story order target changed")
	}
	if snapshot.FlowerID <= 0 || snapshot.Cost <= 0 || op.FlowerID != snapshot.FlowerID ||
		op.ItemCost[snapshot.FlowerID] != snapshot.Cost {
		return fmt.Errorf("cyclic story order cost changed")
	}
	return nil
}

func validateCyclicStoryMilestoneClaimMetadata(op *automation.PlannedOp, snapshot state.CyclicStoryMilestoneClaimSnapshot) error {
	if op == nil || op.BatchID != snapshot.BatchID || op.MilestoneIndex != snapshot.MilestoneIndex {
		return fmt.Errorf("cyclic story milestone target changed")
	}
	if snapshot.Target <= 0 || snapshot.Score < snapshot.Target {
		return fmt.Errorf("cyclic story milestone snapshot is not ready")
	}
	return nil
}

func runCyclicStoryEnter(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := cyclicStoryEnterRequest(op)
	if err != nil {
		return nil, err
	}
	exec := cyclicStoryEnterExecution{
		preflight: func() (state.CyclicStoryEnterSnapshot, error) {
			if !rt.runner.cyclicStoryEnterAutomationEnabled() {
				return state.CyclicStoryEnterSnapshot{}, fmt.Errorf("cyclic story enter automation is disabled")
			}
			snapshot, ok := rt.runner.state.CyclicStoryEnterSnapshot(time.Now())
			if !ok {
				return state.CyclicStoryEnterSnapshot{}, fmt.Errorf("actCyclicStory.enter preflight rejected: activity is invalid, initialized, or outside an executable phase")
			}
			if err := validateCyclicStoryEnterMetadata(op, snapshot); err != nil {
				return state.CyclicStoryEnterSnapshot{}, err
			}
			return snapshot, nil
		},
		enter: func(ctx context.Context, request clientproto.ActCyclicStoryEnterRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.ActCyclicStory().Enter(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:   rt.runner.state.ApplyV,
		applied: rt.runner.state.CyclicStoryEnterApplied,
	}
	return executeCyclicStoryEnter(ctx, req, exec)
}

func runCyclicStoryOrderClaim(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := cyclicStoryOrderClaimRequest(op)
	if err != nil {
		return nil, err
	}
	exec := cyclicStoryOrderClaimExecution{
		preflight: func() (state.CyclicStoryOrderClaimSnapshot, error) {
			if !rt.runner.cyclicStoryOrderClaimAutomationEnabled() {
				return state.CyclicStoryOrderClaimSnapshot{}, fmt.Errorf("cyclic story order claim automation is disabled")
			}
			now := time.Now()
			if !rt.runner.cyclicStoryUnderScoreCap(now) {
				return state.CyclicStoryOrderClaimSnapshot{}, fmt.Errorf("actCyclicStory.recvOrderRwd blocked: score reached configured max_score")
			}
			snapshot, ok := rt.runner.state.CyclicStoryOrderClaimSnapshot(now, op.BatchID, op.SlotID)
			if !ok {
				return state.CyclicStoryOrderClaimSnapshot{}, fmt.Errorf("actCyclicStory.recvOrderRwd preflight rejected: order is not claimable")
			}
			if err := validateCyclicStoryOrderClaimMetadata(op, snapshot); err != nil {
				return state.CyclicStoryOrderClaimSnapshot{}, err
			}
			return snapshot, nil
		},
		recv: func(ctx context.Context, request clientproto.ActCyclicStoryRecvOrderRwdRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.ActCyclicStory().RecvOrderRwd(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:   rt.runner.state.ApplyV,
		applied: rt.runner.state.CyclicStoryOrderClaimApplied,
	}
	return executeCyclicStoryOrderClaim(ctx, req, exec)
}

func runCyclicStoryMilestoneClaim(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := cyclicStoryMilestoneClaimRequest(op)
	if err != nil {
		return nil, err
	}
	exec := cyclicStoryMilestoneClaimExecution{
		preflight: func() (state.CyclicStoryMilestoneClaimSnapshot, error) {
			if !rt.runner.cyclicStoryMilestoneClaimAutomationEnabled() {
				return state.CyclicStoryMilestoneClaimSnapshot{}, fmt.Errorf("cyclic story milestone claim automation is disabled")
			}
			snapshot, ok := rt.runner.state.CyclicStoryMilestoneClaimSnapshot(time.Now(), op.BatchID, op.MilestoneIndex)
			if !ok {
				return state.CyclicStoryMilestoneClaimSnapshot{}, fmt.Errorf("actCyclicStory.recv preflight rejected: milestone is not claimable")
			}
			if err := validateCyclicStoryMilestoneClaimMetadata(op, snapshot); err != nil {
				return state.CyclicStoryMilestoneClaimSnapshot{}, err
			}
			return snapshot, nil
		},
		recv: func(ctx context.Context, request clientproto.ActCyclicStoryRecvRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.ActCyclicStory().Recv(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:   rt.runner.state.ApplyV,
		applied: rt.runner.state.CyclicStoryMilestoneClaimApplied,
	}
	return executeCyclicStoryMilestoneClaim(ctx, req, exec)
}

func executeCyclicStoryEnter(ctx context.Context, req clientproto.ActCyclicStoryEnterRequest, exec cyclicStoryEnterExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.enter == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("cyclic story enter execution is incomplete")
	}
	snapshot, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.enter(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("actCyclicStory.enter: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("actCyclicStory.enter postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(snapshot) {
		return nil, fmt.Errorf("actCyclicStory.enter postcondition failed: exact batch order info was not initialized")
	}
	return raw, nil
}

func executeCyclicStoryOrderClaim(ctx context.Context, req clientproto.ActCyclicStoryRecvOrderRwdRequest, exec cyclicStoryOrderClaimExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.recv == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("cyclic story order claim execution is incomplete")
	}
	snapshot, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.recv(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("actCyclicStory.recvOrderRwd: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("actCyclicStory.recvOrderRwd postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(snapshot) {
		return nil, fmt.Errorf("actCyclicStory.recvOrderRwd postcondition failed: exact order replacement was not observed")
	}
	return raw, nil
}

func executeCyclicStoryMilestoneClaim(ctx context.Context, req clientproto.ActCyclicStoryRecvRequest, exec cyclicStoryMilestoneClaimExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.recv == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("cyclic story milestone claim execution is incomplete")
	}
	snapshot, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.recv(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("actCyclicStory.recv: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("actCyclicStory.recv postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(snapshot) {
		return nil, fmt.Errorf("actCyclicStory.recv postcondition failed: exact milestone receipt was not observed")
	}
	return raw, nil
}

func (r *Runner) cyclicStoryPolicy() *pb.CyclicStoryPolicy {
	policy := r.Policy()
	if !policy.GetAutomationEnabled() {
		return nil
	}
	module := policy.GetActivity().GetCyclicStory()
	if module == nil || !module.GetEnabled() {
		return nil
	}
	return module
}

func (r *Runner) cyclicStoryEnterAutomationEnabled() bool {
	module := r.cyclicStoryPolicy()
	return module != nil && (module.GetAutoClaimOrderRewards() || module.GetAutoClaimProgressBoxes())
}

func (r *Runner) cyclicStoryOrderClaimAutomationEnabled() bool {
	module := r.cyclicStoryPolicy()
	return module != nil && module.GetAutoClaimOrderRewards()
}

func (r *Runner) cyclicStoryMilestoneClaimAutomationEnabled() bool {
	module := r.cyclicStoryPolicy()
	return module != nil && module.GetAutoClaimProgressBoxes()
}

func (r *Runner) cyclicStoryUnderScoreCap(now time.Time) bool {
	module := r.cyclicStoryPolicy()
	if module == nil {
		return true
	}
	maxScore := module.GetMaxScore()
	if maxScore <= 0 {
		return true
	}
	view, ok := r.state.CyclicStoryView(now)
	return ok && view.Valid && int64(view.Score) < maxScore
}
