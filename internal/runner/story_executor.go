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

type storyEnterExecution struct {
	preflight func() error
	enter     func(context.Context, clientproto.StoryMainEnterRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	ready     func() bool
}

type storyUnlockExecution struct {
	preflight func() (state.StoryUnlockSnapshot, error)
	unlock    func(context.Context, clientproto.StoryMainUnlockRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.StoryUnlockSnapshot) bool
}

func storyEnterRequest(op *automation.PlannedOp) (clientproto.StoryMainEnterRequest, error) {
	if op == nil {
		return clientproto.StoryMainEnterRequest{}, fmt.Errorf("storyMain.enter operation is nil")
	}
	if storyOperationHasRequestFields(op) || op.TargetID != 0 || len(op.ItemCost) != 0 || op.GoldCost != 0 || op.DiamondCost != 0 {
		return clientproto.StoryMainEnterRequest{}, fmt.Errorf("storyMain.enter requires an empty request and no cost metadata")
	}
	return clientproto.StoryMainEnterRequest{}, nil
}

func storyUnlockRequest(op *automation.PlannedOp) (clientproto.StoryMainUnlockRequest, error) {
	if op == nil || op.TargetID <= 0 {
		return clientproto.StoryMainUnlockRequest{}, fmt.Errorf("storyMain.unlock requires a catalog section target")
	}
	if storyOperationHasRequestFields(op) || op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) == 0 {
		return clientproto.StoryMainUnlockRequest{}, fmt.Errorf("storyMain.unlock requires empty wire args and positive item-cost metadata")
	}
	for itemID, count := range op.ItemCost {
		if itemID <= 0 || count <= 0 {
			return clientproto.StoryMainUnlockRequest{}, fmt.Errorf("storyMain.unlock contains invalid item-cost metadata")
		}
	}
	return clientproto.StoryMainUnlockRequest{}, nil
}

func storyOperationHasRequestFields(op *automation.PlannedOp) bool {
	return op.TargetUID != 0 || len(op.TargetUIDs) != 0 || op.ItemID != 0 || op.Count != 0 ||
		op.FlowerID != 0 || op.VaseID != 0 || len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0
}

func validateStoryUnlockMetadata(op *automation.PlannedOp, snapshot state.StoryUnlockSnapshot) error {
	if op == nil || op.TargetID != snapshot.SectionID {
		var planned int32
		if op != nil {
			planned = op.TargetID
		}
		return fmt.Errorf("story unlock target changed: planned=%d live=%d", planned, snapshot.SectionID)
	}
	expected := make(map[int32]int32, len(snapshot.Cost))
	for _, cost := range snapshot.Cost {
		if cost.ItemID <= 0 || cost.Count <= 0 {
			return fmt.Errorf("story unlock snapshot contains invalid cost")
		}
		expected[cost.ItemID] += cost.Count
	}
	if len(op.ItemCost) != len(expected) {
		return fmt.Errorf("story unlock item cost changed")
	}
	for itemID, count := range expected {
		if op.ItemCost[itemID] != count {
			return fmt.Errorf("story unlock item cost changed for item %d", itemID)
		}
	}
	return nil
}

func runStoryEnter(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := storyEnterRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("story enter runner state or RPC unavailable")
	}
	exec := storyEnterExecution{
		preflight: func() error {
			if !rt.runner.storyAutomationEnabled() {
				return fmt.Errorf("story automation is disabled")
			}
			if rt.runner.state.StoryMainObserved() {
				return fmt.Errorf("story enter preflight rejected: state is already observed")
			}
			return nil
		},
		enter: func(ctx context.Context, request clientproto.StoryMainEnterRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.StoryMain().Enter(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply: rt.runner.state.ApplyV,
		ready: rt.runner.state.StoryMainReady,
	}
	return executeStoryEnter(ctx, req, exec)
}

func runStoryUnlock(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := storyUnlockRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("story unlock runner state or RPC unavailable")
	}
	exec := storyUnlockExecution{
		preflight: func() (state.StoryUnlockSnapshot, error) {
			if !rt.runner.storyAutomationEnabled() {
				return state.StoryUnlockSnapshot{}, fmt.Errorf("story automation is disabled")
			}
			snapshot, ok := rt.runner.state.StoryUnlockSnapshot()
			if !ok {
				return state.StoryUnlockSnapshot{}, fmt.Errorf("story unlock preflight rejected: invalid target, completed story, or insufficient inventory")
			}
			if err := validateStoryUnlockMetadata(op, snapshot); err != nil {
				return state.StoryUnlockSnapshot{}, err
			}
			return snapshot, nil
		},
		unlock: func(ctx context.Context, request clientproto.StoryMainUnlockRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.StoryMain().Unlock(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:   rt.runner.state.ApplyV,
		applied: rt.runner.state.StoryUnlockApplied,
	}
	return executeStoryUnlock(ctx, req, exec)
}

func executeStoryEnter(ctx context.Context, req clientproto.StoryMainEnterRequest, exec storyEnterExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.enter == nil || exec.apply == nil || exec.ready == nil {
		return nil, fmt.Errorf("story enter execution is incomplete")
	}
	if err := exec.preflight(); err != nil {
		return nil, err
	}
	raw, err := exec.enter(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("storyMain.enter: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("storyMain.enter postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.ready() {
		return nil, fmt.Errorf("storyMain.enter postcondition failed: response did not produce valid or completed story state")
	}
	return raw, nil
}

func executeStoryUnlock(ctx context.Context, req clientproto.StoryMainUnlockRequest, exec storyUnlockExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.unlock == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("story unlock execution is incomplete")
	}
	snapshot, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.unlock(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("storyMain.unlock: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("storyMain.unlock postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(snapshot) {
		return nil, fmt.Errorf("storyMain.unlock postcondition failed: progress or exact item consumption did not match")
	}
	return raw, nil
}

func (r *Runner) storyAutomationEnabled() bool {
	policy := r.Policy()
	return policy.GetAutomationEnabled() && policy.GetBasic().GetTask().GetStoryEnabled()
}
