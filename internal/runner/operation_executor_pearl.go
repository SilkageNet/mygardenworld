package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func pearlRecvOneKeyRequest(op *automation.PlannedOp) (clientproto.PearlPlaceRecvOneKeyRequest, error) {
	if op == nil {
		return clientproto.PearlPlaceRecvOneKeyRequest{}, fmt.Errorf("recvOneKey operation is nil")
	}
	if op.TargetID != 0 || op.ItemID != 0 || op.Count != 0 || op.FlowerID != 0 ||
		op.VaseID != 0 || op.TargetUID != 0 || len(op.TargetUIDs) != 0 || len(op.LandIDs) != 0 ||
		len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0 || plannedOpHasCyclicNoteTargets(op) {
		return clientproto.PearlPlaceRecvOneKeyRequest{}, fmt.Errorf("pearlPlace.recvOneKey requires an empty request")
	}
	if op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
		return clientproto.PearlPlaceRecvOneKeyRequest{}, fmt.Errorf("pearlPlace.recvOneKey must be cost-free")
	}
	return clientproto.PearlPlaceRecvOneKeyRequest{}, nil
}

func pearlFriendSyncRequest(op *automation.PlannedOp) (clientproto.FrdEnterRequest, error) {
	if err := validatePearlCandidateSyncOperation(op, false); err != nil {
		return clientproto.FrdEnterRequest{}, err
	}
	return clientproto.FrdEnterRequest{NeedFriendList: 1}, nil
}

func pearlCandidateDetailRequest(op *automation.PlannedOp) (clientproto.OpptGetDetailOpptsRequest, error) {
	if err := validatePearlCandidateSyncOperation(op, true); err != nil {
		return clientproto.OpptGetDetailOpptsRequest{}, err
	}
	return clientproto.OpptGetDetailOpptsRequest{
		UIDs:    append(clientproto.RPCUIDList(nil), op.TargetUIDs...),
		ExtKeys: clientproto.RPCIDList{1},
	}, nil
}

func pearlCandidateHireStateRequest(op *automation.PlannedOp) (clientproto.PearlGetHireStateByUidsRequest, error) {
	if err := validatePearlCandidateSyncOperation(op, true); err != nil {
		return clientproto.PearlGetHireStateByUidsRequest{}, err
	}
	return clientproto.PearlGetHireStateByUidsRequest{UIDs: append(clientproto.RPCUIDList(nil), op.TargetUIDs...)}, nil
}

func pearlRecommendRequest(op *automation.PlannedOp) (clientproto.PearlGetRecommendListRequest, error) {
	if err := validatePearlCandidateSyncOperation(op, false); err != nil {
		return clientproto.PearlGetRecommendListRequest{}, err
	}
	return clientproto.PearlGetRecommendListRequest{}, nil
}

func validatePearlCandidateSyncOperation(op *automation.PlannedOp, requireUIDs bool) error {
	if op == nil {
		return fmt.Errorf("pearl candidate sync operation is nil")
	}
	if op.TargetID != 0 || op.TargetUID != 0 || op.ItemID != 0 || op.Count != 0 ||
		op.FlowerID != 0 || op.VaseID != 0 || len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0 ||
		plannedOpHasCyclicNoteTargets(op) {
		return fmt.Errorf("pearl candidate sync carries unexpected target fields")
	}
	if op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
		return fmt.Errorf("pearl candidate sync must be cost-free")
	}
	if requireUIDs && len(op.TargetUIDs) == 0 {
		return fmt.Errorf("pearl candidate sync requires at least one UID")
	}
	if !requireUIDs && len(op.TargetUIDs) != 0 {
		return fmt.Errorf("pearl candidate sync requires an empty UID list")
	}
	seen := make(map[int64]struct{}, len(op.TargetUIDs))
	for _, uid := range op.TargetUIDs {
		if uid <= 0 {
			return fmt.Errorf("pearl candidate sync contains an invalid UID")
		}
		if _, exists := seen[uid]; exists {
			return fmt.Errorf("pearl candidate sync contains a duplicate UID")
		}
		seen[uid] = struct{}{}
	}
	return nil
}

func pearlHireRequest(op *automation.PlannedOp) (clientproto.PearlPlaceHireRequest, error) {
	if op == nil || op.TargetID <= 0 || op.TargetUID <= 0 {
		return clientproto.PearlPlaceHireRequest{}, fmt.Errorf("pearlPlace.hire requires placeId and dstUid")
	}
	if op.Count != 1 || op.ItemID != 0 || op.FlowerID != 0 || op.VaseID != 0 ||
		len(op.TargetUIDs) != 0 || len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0 ||
		plannedOpHasCyclicNoteTargets(op) {
		return clientproto.PearlPlaceHireRequest{}, fmt.Errorf("pearlPlace.hire carries unexpected request fields")
	}
	if op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 1 || op.ItemCost[1003] != 1 {
		return clientproto.PearlPlaceHireRequest{}, fmt.Errorf("pearlPlace.hire requires exact ItemCost{1003:1} and no currency cost")
	}
	return clientproto.PearlPlaceHireRequest{PlaceId: op.TargetID, DstUid: op.TargetUID}, nil
}

func runPearlHire(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := pearlHireRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("pearl hire runner state or RPC unavailable")
	}
	if err := preparePearlHireCandidate(ctx, rt, op); err != nil {
		return nil, err // Only read requests ran; never lock the hire session.
	}
	exec := pearlHireExecution{
		preflight: func(at time.Time) (state.PearlHireAttemptSnapshot, error) {
			policy := rt.runner.Policy().GetBasic().GetPearl()
			if err := automation.ValidateSafePearlHire(rt.runner.state, policy, op, at); err != nil {
				return state.PearlHireAttemptSnapshot{}, err
			}
			snapshot, ok := rt.runner.state.PearlHireAttemptSnapshot(op.TargetID, op.TargetUID, at)
			if !ok {
				return state.PearlHireAttemptSnapshot{}, fmt.Errorf("pearl hire slot snapshot unavailable")
			}
			return snapshot, nil
		},
		hire: func(ctx context.Context, request clientproto.PearlPlaceHireRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.PearlPlace().Hire(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:         rt.runner.state.ApplyV,
		outcome:       rt.runner.state.PearlHireAttemptApplied,
		ticketSpent:   rt.runner.state.PearlHireTicketDecreased,
		markFailed:    rt.runner.state.MarkPearlHireFailed,
		skipCandidate: rt.runner.state.SkipPearlHireCandidate,
		noteUsed:      rt.runner.notePearlHireTicketUsed,
		lockSession: func(reason string) {
			rt.runner.state.LockPearlHireSession(reason)
			rt.runner.emit(Event{Kind: "pearl_hire_locked", Category: "basic", Domain: "basic.pearl", Action: "blocked", Level: "error", Label: "珍珠雇佣保护", Message: reason})
		},
		now: time.Now,
	}
	return executePearlHire(ctx, req, exec)
}

// Refresh only the selected UID under the existing operation lock. Keeping
// these two bounded reads with the mutation avoids restarting discovery each
// time a separate scheduled step exhausts the 30-second spend-time window.
func preparePearlHireCandidate(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) error {
	if err := automation.ValidatePearlHireCandidate(rt.runner.state, rt.runner.Policy().GetBasic().GetPearl(), op, time.Now()); err != nil {
		return err
	}
	if automation.PearlHireCandidateFresh(rt.runner.state, op.TargetUID, time.Now()) {
		return nil
	}
	uids := clientproto.RPCUIDList{op.TargetUID}
	if _, err := checkedStateDelta(rt.rpc.Oppt().GetDetailOppts(ctx, clientproto.OpptGetDetailOpptsRequest{UIDs: uids, ExtKeys: clientproto.RPCIDList{1}})); err != nil {
		return fmt.Errorf("雇佣前核验候选等级: %w", err)
	}
	if _, err := checkedStateDelta(rt.rpc.Pearl().GetHireStateByUids(ctx, clientproto.PearlGetHireStateByUidsRequest{UIDs: uids})); err != nil {
		return fmt.Errorf("雇佣前核验候选保护状态: %w", err)
	}
	return nil
}

type pearlHireSendGuardKey struct{}

// A local veto is proof that hire was not sent, unlike an ambiguous transport
// error. It must not permanently lock the session or mark the UID contested.
type pearlHireNotSentError struct{ err error }

func (e *pearlHireNotSentError) Error() string { return "珍珠雇佣未发送: " + e.err.Error() }
func (e *pearlHireNotSentError) Unwrap() error { return e.err }

func executePearlHire(ctx context.Context, req clientproto.PearlPlaceHireRequest, exec pearlHireExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.hire == nil || exec.outcome == nil || exec.markFailed == nil || exec.skipCandidate == nil || exec.lockSession == nil {
		return nil, fmt.Errorf("pearl hire execution is incomplete")
	}
	clock := exec.now
	if clock == nil {
		clock = time.Now
	}
	startedAt := clock()
	snapshot, err := exec.preflight(startedAt)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, pearlHireSendGuardKey{}, func() error {
		var err error
		snapshot, err = exec.preflight(clock())
		return err
	})
	raw, err := exec.hire(ctx, req)
	if err != nil {
		var notSent *pearlHireNotSentError
		if errors.As(err, &notSent) {
			return nil, err
		}
		exec.lockSession("珍珠雇佣请求结果不明确，当前会话已锁定以避免重复扣券")
		exec.markFailed(snapshot.TargetUID, clock())
		return nil, fmt.Errorf("pearlPlace.hire: %w", err)
	}
	fallback, fallbackErr := pearlHireGoldFallback(raw)
	if babigame.HasPayload(raw) && exec.apply != nil {
		exec.apply(raw)
	}
	ticketSpent := exec.ticketSpent != nil && exec.ticketSpent(snapshot)
	if ticketSpent && exec.noteUsed != nil {
		exec.noteUsed(ctx, clock())
	}
	success, failCount, known := exec.outcome(snapshot)
	// Match the official client's result precedence: an authoritative
	// hireFailCnt belongs to the contested-candidate path even if $ext also
	// happens to be present in the same namespace delta.
	if known && failCount > 0 {
		exec.markFailed(snapshot.TargetUID, clock())
		return nil, fmt.Errorf("pearlPlace.hire candidate was contested (hireFailCnt=%d)", failCount)
	}
	if fallbackErr != nil {
		reason := "珍珠雇佣响应中的 3.0 金币回退字段格式异常，当前会话已锁定"
		exec.lockSession(reason)
		exec.markFailed(snapshot.TargetUID, clock())
		return nil, fmt.Errorf("%s: %w", reason, fallbackErr)
	}
	if fallback {
		exec.skipCandidate(snapshot.TargetUID)
		return raw, &pearlHireCandidateFallbackError{TicketSpent: ticketSpent}
	}
	if !success {
		exec.lockSession("珍珠雇佣响应未满足票券与槽位后置条件，当前会话已锁定")
		exec.markFailed(snapshot.TargetUID, clock())
		return nil, fmt.Errorf("pearlPlace.hire postcondition failed: slot, UID, end time, failure count, or ticket decrement did not match")
	}
	return raw, nil
}

// pearlHireGoldFallback inspects only namespace 3 field 0, the wire field
// exposed to the official client as $ext.iv for this RPC. Missing or exact
// integer zero is safe; any nonzero value is a candidate-level fallback and
// any present malformed value is an error that must lock the session.
func pearlHireGoldFallback(raw json.RawMessage) (bool, error) {
	if !babigame.HasPayload(raw) {
		return false, nil
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return false, fmt.Errorf("payload is not an object")
	}
	rawNS3, exists := top["3"]
	if !exists {
		return false, nil
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(rawNS3, &fields) != nil {
		return false, fmt.Errorf("namespace 3 is not an object")
	}
	rawIV, exists := fields["0"]
	if !exists {
		return false, nil
	}
	if isJSONNullRunner(rawIV) {
		return false, fmt.Errorf("3.0 is null")
	}
	value, ok := strictInt64Runner(rawIV)
	if !ok {
		return false, fmt.Errorf("3.0 is not an exact integer")
	}
	return value != 0, nil
}

func isJSONNullRunner(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

func strictInt64Runner(raw json.RawMessage) (int64, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] == '"' {
		return 0, false
	}
	var value json.Number
	if json.Unmarshal(raw, &value) != nil {
		return 0, false
	}
	n, err := strconv.ParseInt(value.String(), 10, 64)
	return n, err == nil
}

func runPearlRecvOneKey(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := pearlRecvOneKeyRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("pearl recvOneKey runner state or RPC unavailable")
	}
	exec := pearlRecvOneKeyExecution{
		preflight: func() (state.PearlClaimSnapshot, error) {
			snapshot, ok := rt.runner.state.PearlClaimSnapshot(time.Now())
			if !ok {
				return state.PearlClaimSnapshot{}, fmt.Errorf("pearl recvOneKey preflight rejected: no time-matured production")
			}
			return snapshot, nil
		},
		recv: func(ctx context.Context, request clientproto.PearlPlaceRecvOneKeyRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.PearlPlace().RecvOneKey(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:   rt.runner.state.ApplyV,
		claimed: rt.runner.state.PearlClaimApplied,
	}
	return executePearlRecvOneKey(ctx, req, exec)
}

func executePearlRecvOneKey(ctx context.Context, req clientproto.PearlPlaceRecvOneKeyRequest, exec pearlRecvOneKeyExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.recv == nil || exec.apply == nil || exec.claimed == nil {
		return nil, fmt.Errorf("pearl recvOneKey execution is incomplete")
	}
	snapshot, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.recv(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("pearlPlace.recvOneKey: %w", err)
	}
	if babigame.HasPayload(raw) && exec.apply != nil {
		exec.apply(raw)
	}
	if !exec.claimed(snapshot) {
		return nil, fmt.Errorf("pearlPlace.recvOneKey postcondition failed: response did not clear all preflight-ready slots")
	}
	return raw, nil
}
