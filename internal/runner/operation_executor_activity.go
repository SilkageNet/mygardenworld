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

func runSignTypeEnter(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	typeID, err := plannedSignTypeID(op)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	needed, err := signTypeEnterSyncNeeded(rt, typeID, now)
	if err != nil || !needed {
		return nil, err
	}
	v, d, callErr := rpcResult(rt.rpc.SignType().Enter(ctx, clientproto.SignTypeEnterRequest{Type: typeID}))
	if stateErr := invalidateSignTypeServerStateError(rt, typeID, "signType.enter", d); stateErr != nil {
		return nil, stateErr
	}
	if callErr != nil || d.IsError() {
		return checkedPayload(v, d, callErr)
	}
	view, observed := rt.runner.state.SignType(typeID)
	if !observed || !view.Observed || !view.Valid || !view.StatusObserved {
		rt.runner.state.InvalidateSignType(typeID)
		return nil, fmt.Errorf("signType.enter postcondition failed: namespace 140 type=%d is not valid", typeID)
	}
	// Empty enter responses are observed in captures. Record only that today's
	// query succeeded; do not infer or mutate the server-owned status.
	rt.runner.state.MarkSignTypeEnterAttempt(typeID, now)
	return v, nil
}

func signTypeEnterSyncNeeded(rt operationRuntime, typeID int32, now time.Time) (bool, error) {
	if rt.runner == nil || rt.runner.state == nil {
		return false, fmt.Errorf("signType.enter type=%d preflight requires runner state", typeID)
	}
	view, observed := rt.runner.state.SignType(typeID)
	if !observed || !view.Observed || !view.Valid || !view.StatusObserved || !view.UpdatedAtObserved {
		return false, fmt.Errorf("signType.enter type=%d preflight has no valid dated namespace 140 state", typeID)
	}
	base, baseObserved := rt.runner.state.BaseReward(state.BaseRewardAntiFraud)
	if !baseObserved || !base.Observed || !base.Valid || base.Status != state.BaseRewardStatusReceived {
		return false, fmt.Errorf("signType.enter type=%d preflight has no eligible namespace 7.7[2] state", typeID)
	}
	if base.UpdatedToday(now) || view.Status != state.SignTypeStatusReceived || view.UpdatedToday(now) || rt.runner.state.SignTypeEnterAttemptedToday(typeID, now) {
		return false, nil
	}
	if !base.UpdatedBeforeToday(now) || !view.UpdatedBeforeToday(now) {
		return false, fmt.Errorf("signType.enter type=%d preflight timestamps are not before today", typeID)
	}
	return true, nil
}

func runSignTypeSign(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	typeID, err := plannedSignTypeID(op)
	if err != nil {
		return nil, err
	}
	if done, err := signTypeStagePreflight(rt, typeID, state.SignTypeStatusCanSign); err != nil || done {
		return nil, err
	}
	enterRaw, ready, err := enterAndRevalidateSignType(ctx, rt, typeID, state.SignTypeStatusCanSign)
	if err != nil {
		return nil, err
	}
	if !ready {
		return enterRaw, nil
	}

	v, d, callErr := rpcResult(rt.rpc.SignType().Sign(ctx, clientproto.SignTypeSignRequest{Type: typeID}))
	if stateErr := invalidateSignTypeServerStateError(rt, typeID, "signType.sign", d); stateErr != nil {
		return nil, stateErr
	}
	if callErr != nil || d.IsError() {
		return checkedPayload(v, d, callErr)
	}
	view, observed := rt.runner.state.SignType(typeID)
	if !observed || !view.Valid || (view.Status != state.SignTypeStatusCanReceive && view.Status != state.SignTypeStatusReceived) {
		rt.runner.state.InvalidateSignType(typeID)
		return nil, fmt.Errorf("signType.sign postcondition failed: namespace 140 type=%d status did not advance from 0", typeID)
	}
	return v, nil
}

func runSignTypeRecv(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	typeID, err := plannedSignTypeID(op)
	if err != nil {
		return nil, err
	}
	if done, err := signTypeStagePreflight(rt, typeID, state.SignTypeStatusCanReceive); err != nil || done {
		return nil, err
	}
	enterRaw, ready, err := enterAndRevalidateSignType(ctx, rt, typeID, state.SignTypeStatusCanReceive)
	if err != nil {
		return nil, err
	}
	if !ready {
		return enterRaw, nil
	}

	v, d, callErr := rpcResult(rt.rpc.SignType().Recv(ctx, clientproto.SignTypeRecvRequest{Type: typeID}))
	if stateErr := invalidateSignTypeServerStateError(rt, typeID, "signType.recv", d); stateErr != nil {
		return nil, stateErr
	}
	if callErr != nil || d.IsError() {
		return checkedPayload(v, d, callErr)
	}
	view, observed := rt.runner.state.SignType(typeID)
	if !observed || !view.Valid || view.Status != state.SignTypeStatusReceived {
		rt.runner.state.InvalidateSignType(typeID)
		return nil, fmt.Errorf("signType.recv postcondition failed: namespace 140 type=%d status did not advance from 1 to 2", typeID)
	}
	return v, nil
}

func signTypeStagePreflight(rt operationRuntime, typeID, expectedStatus int32) (bool, error) {
	view, err := signTypeActionState(rt, typeID, time.Now())
	if err != nil {
		return false, err
	}
	if view.Status == expectedStatus {
		return false, nil
	}
	if view.Status > expectedStatus && view.Status <= state.SignTypeStatusReceived {
		return true, nil
	}
	return false, fmt.Errorf("signType type=%d preflight status=%d, want %d", typeID, view.Status, expectedStatus)
}

func signTypeActionState(rt operationRuntime, typeID int32, now time.Time) (state.SignTypeView, error) {
	if rt.runner == nil || rt.runner.state == nil {
		return state.SignTypeView{}, fmt.Errorf("signType type=%d preflight requires runner state", typeID)
	}
	base, baseObserved := rt.runner.state.BaseReward(state.BaseRewardAntiFraud)
	if !baseObserved || !base.Observed || !base.Valid || base.Status != state.BaseRewardStatusReceived || !base.UpdatedBeforeToday(now) {
		return state.SignTypeView{}, fmt.Errorf("signType type=%d preflight has no eligible namespace 7.7[2] state", typeID)
	}
	view, observed := rt.runner.state.SignType(typeID)
	if !observed || !view.Observed || !view.Valid || !view.StatusObserved {
		return state.SignTypeView{}, fmt.Errorf("signType type=%d preflight has no valid namespace 140 state", typeID)
	}
	return view, nil
}

func enterAndRevalidateSignType(ctx context.Context, rt operationRuntime, typeID, expectedStatus int32) (json.RawMessage, bool, error) {
	now := time.Now()
	if rt.runner.state.SignTypeEnterAttemptedToday(typeID, now) {
		ready, err := signTypeStageReadyAfterEnter(rt, typeID, expectedStatus, now)
		return nil, ready, err
	}
	v, d, err := rpcResult(rt.rpc.SignType().Enter(ctx, clientproto.SignTypeEnterRequest{Type: typeID}))
	if stateErr := invalidateSignTypeServerStateError(rt, typeID, "signType.enter", d); stateErr != nil {
		return nil, false, stateErr
	}
	if err != nil || d.IsError() {
		_, checkedErr := checkedPayload(v, d, err)
		return nil, false, checkedErr
	}
	ready, preflightErr := signTypeStageReadyAfterEnter(rt, typeID, expectedStatus, now)
	if preflightErr != nil {
		return nil, false, preflightErr
	}
	rt.runner.state.MarkSignTypeEnterAttempt(typeID, now)
	return v, ready, nil
}

func signTypeStageReadyAfterEnter(rt operationRuntime, typeID, expectedStatus int32, now time.Time) (bool, error) {
	view, err := signTypeActionState(rt, typeID, now)
	if err != nil {
		return false, err
	}
	// enter may authoritatively reset yesterday's status=1/2 back to 0. Any
	// valid observed status is a successful sync; only the exact expected stage
	// continues in this tick, while every other stage is replanned next tick.
	return view.Status == expectedStatus, nil
}

func invalidateSignTypeServerStateError(rt operationRuntime, typeID int32, method string, d babigame.WSResponseD) error {
	description, nonRetryable := signTypeNonRetryableCode(d.ErrorCode())
	if !nonRetryable {
		return nil
	}
	if rt.runner != nil && rt.runner.state != nil {
		rt.runner.state.InvalidateSignType(typeID)
	}
	return fmt.Errorf("%s type=%d server code %d（%s），已使本地状态失效并阻止后续重试", method, typeID, d.ErrorCode(), description)
}

func signTypeNonRetryableCode(code int) (string, bool) {
	switch code {
	case 3500:
		return "条件已达成，无需重复操作", true
	case 3501:
		return "今日奖励已领取", true
	case 3502:
		return "未达成领取条件，无法获取奖励", true
	case 3503:
		return "功能暂未解锁", true
	default:
		return "", false
	}
}

func plannedSignTypeID(op *automation.PlannedOp) (int32, error) {
	typeID := state.SignTypeAntiFraud
	if op != nil && op.TargetID != 0 {
		typeID = op.TargetID
	}
	if typeID != state.SignTypeAntiFraud {
		return 0, fmt.Errorf("signType automation only supports observed type=%d, got %d", state.SignTypeAntiFraud, typeID)
	}
	return typeID, nil
}
