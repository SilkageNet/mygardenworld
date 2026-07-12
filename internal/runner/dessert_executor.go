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
	dessertModuleID                         = "actDessert"
	dessertAutoClaimTaskRewardsPolicy       = "auto_claim_task_rewards"
	dessertAutoLikeCelebrityPolicy          = "auto_like_celebrity"
	dessertAutoOpenRewardBoxesPolicy        = "auto_open_reward_boxes"
	dessertCelebrityType              int32 = 5601
)

type dessertEnterExecution struct {
	preflight func() (state.DessertEnterSnapshot, error)
	enter     func(context.Context, clientproto.ActDessertEnterRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.DessertEnterSnapshot) bool
}

type dessertTaskClaimExecution struct {
	preflight func() (state.DessertTaskClaimSnapshot, error)
	recv      func(context.Context, clientproto.ActRecvRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.DessertTaskClaimSnapshot) bool
}

type dessertCelebritySyncExecution struct {
	preflight func() (state.DessertCelebritySyncSnapshot, error)
	sync      func(context.Context, clientproto.CelebrityGetAllTypesInfoRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.DessertCelebritySyncSnapshot) bool
	mark      func(int32)
}

type dessertCelebrityLikeExecution struct {
	preflight func() (state.DessertCelebrityLikeSnapshot, error)
	like      func(context.Context, clientproto.CelebrityLikeCelebrityRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.DessertCelebrityLikeSnapshot) bool
}

type dessertRewardBoxOpenExecution struct {
	preflight func() (state.DessertRewardBoxOpenSnapshot, error)
	open      func(context.Context, clientproto.ActDessertOpenBoxRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	applied   func(state.DessertRewardBoxOpenSnapshot) bool
}

func dessertEnterRequest(op *automation.PlannedOp) (clientproto.ActDessertEnterRequest, error) {
	if op == nil || op.Kind != clientproto.RPCActDessertEnter.String() || op.BatchID <= 0 {
		return clientproto.ActDessertEnterRequest{}, fmt.Errorf("actDessert.enter requires a positive batch target")
	}
	if op.TaskID != 0 || dessertOperationHasUnexpectedTargets(op) {
		return clientproto.ActDessertEnterRequest{}, fmt.Errorf("actDessert.enter requires batch-only, cost-free metadata")
	}
	return clientproto.ActDessertEnterRequest{BatchId: op.BatchID}, nil
}

func dessertTaskClaimRequest(op *automation.PlannedOp) (clientproto.ActRecvRequest, error) {
	if op == nil || op.Kind != clientproto.RPCActRecv.String() || op.BatchID <= 0 || op.TaskID <= 0 {
		return clientproto.ActRecvRequest{}, fmt.Errorf("act.recv requires positive dessert batch and task targets")
	}
	if op.SlotID != 0 || dessertOperationHasUnexpectedTargets(op) {
		return clientproto.ActRecvRequest{}, fmt.Errorf("act.recv requires exact index-zero task, cost-free metadata")
	}
	// TaskIdx intentionally has no omitempty tag: zero is a required wire
	// value for the only capture-confirmed dessert task group.
	return clientproto.ActRecvRequest{BatchId: op.BatchID, TaskIdx: 0, TaskId: op.TaskID}, nil
}

func dessertCelebritySyncRequest(op *automation.PlannedOp) (clientproto.CelebrityGetAllTypesInfoRequest, error) {
	if op == nil || op.Kind != clientproto.RPCCelebrityGetAllTypesInfo.String() || op.BatchID <= 0 {
		return clientproto.CelebrityGetAllTypesInfoRequest{}, fmt.Errorf("celebrity.getAllTypesInfo requires an internal positive dessert batch target")
	}
	if op.TaskID != 0 || dessertOperationHasUnexpectedTargets(op) {
		return clientproto.CelebrityGetAllTypesInfoRequest{}, fmt.Errorf("celebrity.getAllTypesInfo requires empty, cost-free wire metadata")
	}
	return clientproto.CelebrityGetAllTypesInfoRequest{}, nil
}

func dessertCelebrityLikeRequest(op *automation.PlannedOp) (clientproto.CelebrityLikeCelebrityRequest, error) {
	if op == nil || op.Kind != clientproto.RPCCelebrityLikeCelebrity.String() || op.BatchID <= 0 {
		return clientproto.CelebrityLikeCelebrityRequest{}, fmt.Errorf("celebrity.likeCelebrity requires an internal positive dessert batch target")
	}
	if op.TaskID != 0 || dessertOperationHasUnexpectedTargets(op) {
		return clientproto.CelebrityLikeCelebrityRequest{}, fmt.Errorf("celebrity.likeCelebrity requires type-only, cost-free wire metadata")
	}
	return clientproto.CelebrityLikeCelebrityRequest{Type: dessertCelebrityType}, nil
}

func dessertRewardBoxOpenRequest(op *automation.PlannedOp) (clientproto.ActDessertOpenBoxRequest, error) {
	if op == nil || op.Kind != clientproto.RPCActDessertOpenBox.String() || op.BatchID <= 0 || op.Count != 1 {
		return clientproto.ActDessertOpenBoxRequest{}, fmt.Errorf("actDessert.openBox requires a positive batch and capture-confirmed num=1")
	}
	if len(op.CostGates) != 1 {
		return clientproto.ActDessertOpenBoxRequest{}, fmt.Errorf("actDessert.openBox requires one activity-local reward-box gate")
	}
	gate := op.CostGates[0]
	if gate.ResourceKind != automation.GateResourceActivityItem || gate.ItemID != 1347 || gate.Required != 1 || gate.Available < 1 || gate.Blocking() {
		return clientproto.ActDessertOpenBoxRequest{}, fmt.Errorf("actDessert.openBox reward-box gate is missing or blocked")
	}
	metadata := *op
	metadata.Count = 0
	if metadata.TaskID != 0 || dessertOperationHasUnexpectedTargets(&metadata) {
		return clientproto.ActDessertOpenBoxRequest{}, fmt.Errorf("actDessert.openBox requires exact single-box, activity-local metadata")
	}
	return clientproto.ActDessertOpenBoxRequest{BatchId: op.BatchID, Num: 1}, nil
}

func dessertOperationHasUnexpectedTargets(op *automation.PlannedOp) bool {
	return op.TargetUID != 0 || len(op.TargetUIDs) != 0 || op.TargetID != 0 || op.ItemID != 0 || op.Count != 0 ||
		op.FlowerID != 0 || op.VaseID != 0 || len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0 ||
		op.SlotID != 0 || op.MilestoneIndex != 0 || op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0
}

func runDessertEnter(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := dessertEnterRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("dessert enter runner state or RPC unavailable")
	}
	return executeDessertEnter(ctx, req, dessertEnterExecution{
		preflight: func() (state.DessertEnterSnapshot, error) {
			if !rt.runner.dessertEnterAutomationEnabled() {
				return state.DessertEnterSnapshot{}, fmt.Errorf("dessert enter automation is disabled")
			}
			now := time.Now()
			var snapshot state.DessertEnterSnapshot
			var ok bool
			if rt.runner.dessertTaskClaimAutomationEnabled() || rt.runner.dessertCelebrityLikeAutomationEnabled() {
				snapshot, ok = rt.runner.state.DessertEnterSnapshot(now)
			}
			if !ok && rt.runner.dessertRewardBoxOpenAutomationEnabled() {
				snapshot, ok = rt.runner.state.DessertRewardBoxEnterSnapshot(now)
			}
			if !ok || snapshot.BatchID != op.BatchID {
				return state.DessertEnterSnapshot{}, fmt.Errorf("actDessert.enter preflight rejected: exact batch no longer needs safe initialization")
			}
			return snapshot, nil
		},
		enter: func(ctx context.Context, request clientproto.ActDessertEnterRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.ActDessert().Enter(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply: rt.runner.state.ApplyV, applied: rt.runner.state.DessertEnterApplied,
	})
}

func runDessertTaskClaim(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := dessertTaskClaimRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("dessert task runner state or RPC unavailable")
	}
	return executeDessertTaskClaim(ctx, req, dessertTaskClaimExecution{
		preflight: func() (state.DessertTaskClaimSnapshot, error) {
			if !rt.runner.dessertTaskClaimAutomationEnabled() {
				return state.DessertTaskClaimSnapshot{}, fmt.Errorf("dessert task reward automation is disabled")
			}
			snapshot, ok := rt.runner.state.DessertTaskClaimSnapshot(time.Now(), op.BatchID, 0, op.TaskID)
			if !ok {
				return state.DessertTaskClaimSnapshot{}, fmt.Errorf("act.recv preflight rejected: exact dessert task is not safely ready")
			}
			return snapshot, nil
		},
		recv: func(ctx context.Context, request clientproto.ActRecvRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.Act().Recv(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply: rt.runner.state.ApplyV, applied: rt.runner.state.DessertTaskClaimApplied,
	})
}

func runDessertCelebritySync(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := dessertCelebritySyncRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("dessert celebrity sync runner state or RPC unavailable")
	}
	return executeDessertCelebritySync(ctx, req, dessertCelebritySyncExecution{
		preflight: func() (state.DessertCelebritySyncSnapshot, error) {
			if !rt.runner.dessertCelebrityLikeAutomationEnabled() {
				return state.DessertCelebritySyncSnapshot{}, fmt.Errorf("dessert celebrity automation is disabled")
			}
			snapshot, ok := rt.runner.state.DessertCelebritySyncSnapshot(time.Now())
			if !ok || snapshot.BatchID != op.BatchID {
				return state.DessertCelebritySyncSnapshot{}, fmt.Errorf("celebrity.getAllTypesInfo preflight rejected: exact batch no longer needs a controlled sync")
			}
			return snapshot, nil
		},
		sync: func(ctx context.Context, request clientproto.CelebrityGetAllTypesInfoRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.Celebrity().GetAllTypesInfo(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply: rt.runner.state.ApplyV, applied: rt.runner.state.DessertCelebritySyncApplied,
		mark: rt.runner.state.MarkDessertCelebritySynced,
	})
}

func runDessertCelebrityLike(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := dessertCelebrityLikeRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("dessert celebrity like runner state or RPC unavailable")
	}
	return executeDessertCelebrityLike(ctx, req, dessertCelebrityLikeExecution{
		preflight: func() (state.DessertCelebrityLikeSnapshot, error) {
			if !rt.runner.dessertCelebrityLikeAutomationEnabled() {
				return state.DessertCelebrityLikeSnapshot{}, fmt.Errorf("dessert celebrity automation is disabled")
			}
			snapshot, ok := rt.runner.state.DessertCelebrityLikeSnapshot(time.Now(), op.BatchID)
			if !ok {
				return state.DessertCelebrityLikeSnapshot{}, fmt.Errorf("celebrity.likeCelebrity preflight rejected: exact free like is not safely ready")
			}
			return snapshot, nil
		},
		like: func(ctx context.Context, request clientproto.CelebrityLikeCelebrityRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.Celebrity().LikeCelebrity(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply: rt.runner.state.ApplyV, applied: rt.runner.state.DessertCelebrityLikeApplied,
	})
}

func runDessertRewardBoxOpen(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := dessertRewardBoxOpenRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("dessert reward-box runner state or RPC unavailable")
	}
	return executeDessertRewardBoxOpen(ctx, req, dessertRewardBoxOpenExecution{
		preflight: func() (state.DessertRewardBoxOpenSnapshot, error) {
			if !rt.runner.dessertRewardBoxOpenAutomationEnabled() {
				return state.DessertRewardBoxOpenSnapshot{}, fmt.Errorf("dessert reward-box automation is disabled or lacks evidence")
			}
			snapshot, ok := rt.runner.state.DessertRewardBoxOpenSnapshot(time.Now(), op.BatchID, 1)
			if !ok {
				return state.DessertRewardBoxOpenSnapshot{}, fmt.Errorf("actDessert.openBox preflight rejected: exact activity box is no longer available")
			}
			return snapshot, nil
		},
		open: func(ctx context.Context, request clientproto.ActDessertOpenBoxRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.ActDessert().OpenBox(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply: rt.runner.state.ApplyV, applied: rt.runner.state.DessertRewardBoxOpenApplied,
	})
}

func executeDessertEnter(ctx context.Context, req clientproto.ActDessertEnterRequest, exec dessertEnterExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.enter == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("dessert enter execution is incomplete")
	}
	before, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.enter(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("actDessert.enter: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("actDessert.enter postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(before) {
		return nil, fmt.Errorf("actDessert.enter postcondition failed: activity bag or ext121 remained incomplete")
	}
	return raw, nil
}

func executeDessertTaskClaim(ctx context.Context, req clientproto.ActRecvRequest, exec dessertTaskClaimExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.recv == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("dessert task claim execution is incomplete")
	}
	before, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.recv(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("act.recv dessert task: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("act.recv dessert task postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(before) {
		return nil, fmt.Errorf("act.recv dessert task postcondition failed: receipt or energy reward missing")
	}
	return raw, nil
}

func executeDessertCelebritySync(ctx context.Context, req clientproto.CelebrityGetAllTypesInfoRequest, exec dessertCelebritySyncExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.sync == nil || exec.apply == nil || exec.applied == nil || exec.mark == nil {
		return nil, fmt.Errorf("dessert celebrity sync execution is incomplete")
	}
	before, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.sync(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("celebrity.getAllTypesInfo: %w", err)
	}
	if !babigame.HasPayload(raw) || !dessertCelebrityFullSyncDelta(raw) {
		return nil, fmt.Errorf("celebrity.getAllTypesInfo postcondition failed: full type and ranking maps missing")
	}
	exec.apply(raw)
	if !exec.applied(before) {
		return nil, fmt.Errorf("celebrity.getAllTypesInfo postcondition failed: type 5601 ranking state incomplete")
	}
	exec.mark(before.BatchID)
	return raw, nil
}

func executeDessertCelebrityLike(ctx context.Context, req clientproto.CelebrityLikeCelebrityRequest, exec dessertCelebrityLikeExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.like == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("dessert celebrity like execution is incomplete")
	}
	before, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.like(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("celebrity.likeCelebrity: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("celebrity.likeCelebrity postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(before) {
		return nil, fmt.Errorf("celebrity.likeCelebrity postcondition failed: like timestamp or energy reward missing")
	}
	return raw, nil
}

func executeDessertRewardBoxOpen(ctx context.Context, req clientproto.ActDessertOpenBoxRequest, exec dessertRewardBoxOpenExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.open == nil || exec.apply == nil || exec.applied == nil {
		return nil, fmt.Errorf("dessert reward-box execution is incomplete")
	}
	before, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.open(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("actDessert.openBox: %w", err)
	}
	if !babigame.HasPayload(raw) {
		return nil, fmt.Errorf("actDessert.openBox postcondition failed: response payload is empty")
	}
	exec.apply(raw)
	if !exec.applied(before) {
		return nil, fmt.Errorf("actDessert.openBox postcondition failed: activity reward-box balance did not decrease by exactly one")
	}
	return raw, nil
}

func dessertCelebrityFullSyncDelta(raw json.RawMessage) bool {
	var namespaces map[string]json.RawMessage
	if json.Unmarshal(raw, &namespaces) != nil || namespaces == nil {
		return false
	}
	for _, namespace := range []string{"166", "165"} {
		rawCelebrity, present := namespaces[namespace]
		if !present {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(rawCelebrity, &fields) != nil || fields == nil {
			return false
		}
		_, hasTypes := fields["0"]
		_, hasRankings := fields["1"]
		return hasTypes && hasRankings
	}
	return false
}

func (r *Runner) dessertPolicyFlag(key string) bool {
	policy := r.Policy()
	if !policy.GetAutomationEnabled() || !policy.GetActivity().GetEnabled() {
		return false
	}
	module := policy.GetActivity().GetModules()[dessertModuleID]
	return module != nil && module.GetEnabled() && module.GetBoolParams()[key]
}

func (r *Runner) dessertEnterAutomationEnabled() bool {
	return r.dessertPolicyFlag(dessertAutoClaimTaskRewardsPolicy) || r.dessertPolicyFlag(dessertAutoLikeCelebrityPolicy) ||
		r.dessertRewardBoxOpenAutomationEnabled()
}

func (r *Runner) dessertTaskClaimAutomationEnabled() bool {
	return r.dessertPolicyFlag(dessertAutoClaimTaskRewardsPolicy)
}

func (r *Runner) dessertCelebrityLikeAutomationEnabled() bool {
	return r.dessertPolicyFlag(dessertAutoLikeCelebrityPolicy)
}

func (r *Runner) dessertRewardBoxOpenAutomationEnabled() bool {
	return babigame.DessertOpenRewardBoxEvidenceGate() && r.dessertPolicyFlag(dessertAutoOpenRewardBoxesPolicy)
}
