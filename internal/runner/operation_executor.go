package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientrpc"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

type operationRuntime struct {
	runner *Runner
	rpc    *clientrpc.Client
}

type harvestCallResult struct {
	LandID int32           `json:"landId"`
	Raw    json.RawMessage `json:"raw,omitempty"`
}

type harvestLandError struct {
	LandID int32
	Err    error
}

type zooHandleEventExecution struct {
	preflight func() error
	handle    func(context.Context, clientproto.ZooHandleEventRequest) (json.RawMessage, error)
	read      func(context.Context, clientproto.ZooReadLogRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	handled   func() bool
}

type zooHandleEventResult struct {
	Handle json.RawMessage `json:"handle,omitempty"`
	Read   json.RawMessage `json:"read,omitempty"`
}

type zooReadLogExecution struct {
	preflight func() (int64, error)
	read      func(context.Context, clientproto.ZooReadLogRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	readDone  func(int64) bool
}

type pearlRecvOneKeyExecution struct {
	preflight func() (state.PearlClaimSnapshot, error)
	recv      func(context.Context, clientproto.PearlPlaceRecvOneKeyRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	claimed   func(state.PearlClaimSnapshot) bool
}

type pearlHireExecution struct {
	preflight   func(time.Time) (state.PearlHireAttemptSnapshot, error)
	hire        func(context.Context, clientproto.PearlPlaceHireRequest) (json.RawMessage, error)
	apply       func(json.RawMessage)
	outcome     func(state.PearlHireAttemptSnapshot) (bool, int32, bool)
	markFailed  func(int64, time.Time)
	lockSession func(string)
	now         func() time.Time
}

type zooRecvSouvenirRewardExecution struct {
	preflight func() error
	recv      func(context.Context, clientproto.ZooRecvSouvenirRwdRequest) (json.RawMessage, error)
	apply     func(json.RawMessage)
	claimed   func() bool
}

type zooReadSouvenirExecution struct {
	preflight    func() error
	read         func(context.Context, clientproto.ZooReadSouvenirRequest) (json.RawMessage, error)
	apply        func(json.RawMessage)
	acknowledged func() bool
}

func (e *harvestLandError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return fmt.Sprintf("land %d: %v", e.LandID, e.Err)
}

func (e *harvestLandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func runUsrLandHarvest(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	reqs, err := harvestRequests(op)
	if err != nil {
		return nil, err
	}
	results := make([]harvestCallResult, 0, len(reqs))
	for i, req := range reqs {
		raw, err := checkedStateDelta(rt.rpc.UsrLand().Harvest(ctx, req))
		if err != nil {
			return nil, &harvestLandError{LandID: req.LandId, Err: err}
		}
		results = append(results, harvestCallResult{LandID: req.LandId, Raw: raw})
		if i == len(reqs)-1 || harvestRPCInterval <= 0 {
			continue
		}
		timer := time.NewTimer(harvestRPCInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	raw, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func zooHandleEventRequest(op *automation.PlannedOp) (clientproto.ZooHandleEventRequest, error) {
	if op == nil || op.TargetID <= 0 || op.ItemID <= 0 {
		return clientproto.ZooHandleEventRequest{}, fmt.Errorf("handleEvent missing pet id or log index")
	}
	if op.Count != 1 {
		return clientproto.ZooHandleEventRequest{}, fmt.Errorf("handleEvent requires the unique agree result")
	}
	if plannedOpHasCyclicNoteTargets(op) || op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
		return clientproto.ZooHandleEventRequest{}, fmt.Errorf("handleEvent automation only permits cost-free logs")
	}
	return clientproto.ZooHandleEventRequest{
		PetId:        op.TargetID,
		TableId:      op.ItemID,
		Agree:        true,
		IsShareVideo: 0,
	}, nil
}

func zooHandleEventPreflight(st *state.State, req clientproto.ZooHandleEventRequest) error {
	if st == nil {
		return fmt.Errorf("handleEvent state unavailable")
	}
	action, ok := st.ZooHandleEventAction(req.PetId, req.TableId)
	if !ok {
		return fmt.Errorf("pet %d log %d no longer exists", req.PetId, req.TableId)
	}
	if action.Blocked || action.Action != "handle_event" || !action.Agree {
		reason := action.BlockedReason
		if reason == "" {
			reason = "log is no longer safe to handle"
		}
		return fmt.Errorf("pet %d log %d preflight rejected: %s", req.PetId, req.TableId, reason)
	}
	return nil
}

func runZooHandleEvent(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := zooHandleEventRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("handleEvent runner state or RPC unavailable")
	}
	exec := zooHandleEventExecution{
		preflight: func() error { return zooHandleEventPreflight(rt.runner.state, req) },
		handle: func(ctx context.Context, request clientproto.ZooHandleEventRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.Zoo().HandleEvent(ctx, request, babigame.WithPayloadApply(false)))
		},
		read: func(ctx context.Context, request clientproto.ZooReadLogRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.Zoo().ReadLog(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:   rt.runner.state.ApplyV,
		handled: func() bool { return rt.runner.state.ZooLogHandled(req.PetId, req.TableId) },
	}
	return executeZooHandleEvent(ctx, req, exec)
}

func executeZooHandleEvent(ctx context.Context, req clientproto.ZooHandleEventRequest, exec zooHandleEventExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.handle == nil || exec.read == nil || exec.handled == nil {
		return nil, fmt.Errorf("handleEvent execution is incomplete")
	}
	if err := exec.preflight(); err != nil {
		return nil, err
	}
	handleRaw, err := exec.handle(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("handleEvent: %w", err)
	}
	if babigame.HasPayload(handleRaw) && exec.apply != nil {
		exec.apply(handleRaw)
	}
	readRaw, err := exec.read(ctx, clientproto.ZooReadLogRequest{PetId: req.PetId})
	if err != nil {
		return nil, fmt.Errorf("readLog after handleEvent: %w", err)
	}
	if babigame.HasPayload(readRaw) && exec.apply != nil {
		exec.apply(readRaw)
	}
	if !exec.handled() {
		return nil, fmt.Errorf("handleEvent postcondition failed for pet %d log %d: response left log pending", req.PetId, req.TableId)
	}
	return json.Marshal(zooHandleEventResult{Handle: handleRaw, Read: readRaw})
}

func zooReadLogRequest(op *automation.PlannedOp) (clientproto.ZooReadLogRequest, error) {
	if op == nil || op.TargetID <= 0 || op.ItemID <= 0 {
		return clientproto.ZooReadLogRequest{}, fmt.Errorf("readLog missing pet id or log index")
	}
	if plannedOpHasCyclicNoteTargets(op) {
		return clientproto.ZooReadLogRequest{}, fmt.Errorf("readLog carries unexpected activity targets")
	}
	return clientproto.ZooReadLogRequest{PetId: op.TargetID}, nil
}

func runZooReadLog(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := zooReadLogRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("readLog runner state or RPC unavailable")
	}
	index := op.ItemID
	exec := zooReadLogExecution{
		preflight: func() (int64, error) {
			action, ok := rt.runner.state.ZooReadLogAction(req.PetId, index)
			if !ok {
				return 0, fmt.Errorf("pet %d log %d no longer exists", req.PetId, index)
			}
			if action.Blocked || action.Action != "read_log" || action.CreatedAtMs <= 0 {
				reason := action.BlockedReason
				if reason == "" {
					reason = "log is no longer completed and unread"
				}
				return 0, fmt.Errorf("pet %d log %d read preflight rejected: %s", req.PetId, index, reason)
			}
			return action.CreatedAtMs, nil
		},
		read: func(ctx context.Context, request clientproto.ZooReadLogRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.Zoo().ReadLog(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:    rt.runner.state.ApplyV,
		readDone: func(createdAtMs int64) bool { return rt.runner.state.ZooLogRead(req.PetId, index, createdAtMs) },
	}
	return executeZooReadLog(ctx, req, exec)
}

func executeZooReadLog(ctx context.Context, req clientproto.ZooReadLogRequest, exec zooReadLogExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.read == nil || exec.readDone == nil {
		return nil, fmt.Errorf("readLog execution is incomplete")
	}
	createdAtMs, err := exec.preflight()
	if err != nil {
		return nil, err
	}
	raw, err := exec.read(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("readLog: %w", err)
	}
	if babigame.HasPayload(raw) && exec.apply != nil {
		exec.apply(raw)
	}
	if !exec.readDone(createdAtMs) {
		return nil, fmt.Errorf("readLog postcondition failed for pet %d: response did not advance read time past log %d", req.PetId, createdAtMs)
	}
	return raw, nil
}

func zooRecvSouvenirRewardRequest(op *automation.PlannedOp) (clientproto.ZooRecvSouvenirRwdRequest, error) {
	ids, err := zooSouvenirBatchIDs(op, "recvSouvenirRwd")
	if err != nil {
		return clientproto.ZooRecvSouvenirRwdRequest{}, err
	}
	return clientproto.ZooRecvSouvenirRwdRequest{IdxList: clientproto.RPCIDList(ids)}, nil
}

func zooReadSouvenirRequest(op *automation.PlannedOp) (clientproto.ZooReadSouvenirRequest, error) {
	ids, err := zooSouvenirBatchIDs(op, "readSouvenir")
	if err != nil {
		return clientproto.ZooReadSouvenirRequest{}, err
	}
	return clientproto.ZooReadSouvenirRequest{SouvenirIds: clientproto.RPCIDList(ids)}, nil
}

func zooSouvenirBatchIDs(op *automation.PlannedOp, action string) ([]int32, error) {
	if op == nil || len(op.SlotIDs) == 0 {
		return nil, fmt.Errorf("%s requires a non-empty id list", action)
	}
	if op.Count != int32(len(op.SlotIDs)) {
		return nil, fmt.Errorf("%s count %d does not match id list length %d", action, op.Count, len(op.SlotIDs))
	}
	if plannedOpHasCyclicNoteTargets(op) || op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
		return nil, fmt.Errorf("%s automation only permits cost-free requests", action)
	}
	ids := append([]int32(nil), op.SlotIDs...)
	for i, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("%s contains invalid id %d", action, id)
		}
		if i > 0 && ids[i-1] >= id {
			return nil, fmt.Errorf("%s ids must be strictly increasing and unique", action)
		}
	}
	return ids, nil
}

func runZooRecvSouvenirReward(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := zooRecvSouvenirRewardRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("recvSouvenirRwd runner state or RPC unavailable")
	}
	indices := append([]int32(nil), req.IdxList...)
	exec := zooRecvSouvenirRewardExecution{
		preflight: func() error {
			if !rt.runner.state.ZooSouvenirRewardsReady(indices) {
				return fmt.Errorf("recvSouvenirRwd preflight rejected: one or more milestones are no longer ready")
			}
			return nil
		},
		recv: func(ctx context.Context, request clientproto.ZooRecvSouvenirRwdRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.Zoo().RecvSouvenirRwd(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:   rt.runner.state.ApplyV,
		claimed: func() bool { return rt.runner.state.ZooSouvenirRewardsClaimed(indices) },
	}
	return executeZooRecvSouvenirReward(ctx, req, exec)
}

func executeZooRecvSouvenirReward(ctx context.Context, req clientproto.ZooRecvSouvenirRwdRequest, exec zooRecvSouvenirRewardExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.recv == nil || exec.claimed == nil {
		return nil, fmt.Errorf("recvSouvenirRwd execution is incomplete")
	}
	if err := exec.preflight(); err != nil {
		return nil, err
	}
	raw, err := exec.recv(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("recvSouvenirRwd: %w", err)
	}
	if babigame.HasPayload(raw) && exec.apply != nil {
		exec.apply(raw)
	}
	if !exec.claimed() {
		return nil, fmt.Errorf("recvSouvenirRwd postcondition failed: response did not claim every requested milestone")
	}
	return raw, nil
}

func runZooReadSouvenir(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	req, err := zooReadSouvenirRequest(op)
	if err != nil {
		return nil, err
	}
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("readSouvenir runner state or RPC unavailable")
	}
	ids := append([]int32(nil), req.SouvenirIds...)
	exec := zooReadSouvenirExecution{
		preflight: func() error {
			if !rt.runner.state.ZooSouvenirsReadyToAcknowledge(ids) {
				return fmt.Errorf("readSouvenir preflight rejected: rewards became ready, source state is unknown, or a souvenir is no longer explicitly unread")
			}
			return nil
		},
		read: func(ctx context.Context, request clientproto.ZooReadSouvenirRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.Zoo().ReadSouvenir(ctx, request, babigame.WithPayloadApply(false)))
		},
		apply:        rt.runner.state.ApplyV,
		acknowledged: func() bool { return rt.runner.state.ZooSouvenirsAcknowledged(ids) },
	}
	return executeZooReadSouvenir(ctx, req, exec)
}

func executeZooReadSouvenir(ctx context.Context, req clientproto.ZooReadSouvenirRequest, exec zooReadSouvenirExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.read == nil || exec.acknowledged == nil {
		return nil, fmt.Errorf("readSouvenir execution is incomplete")
	}
	if err := exec.preflight(); err != nil {
		return nil, err
	}
	raw, err := exec.read(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("readSouvenir: %w", err)
	}
	if babigame.HasPayload(raw) && exec.apply != nil {
		exec.apply(raw)
	}
	if !exec.acknowledged() {
		return nil, fmt.Errorf("readSouvenir postcondition failed: response left one or more souvenirs unread")
	}
	return raw, nil
}

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
		apply:       rt.runner.state.ApplyV,
		outcome:     rt.runner.state.PearlHireAttemptApplied,
		markFailed:  rt.runner.state.MarkPearlHireFailed,
		lockSession: rt.runner.state.LockPearlHireSession,
		now:         time.Now,
	}
	return executePearlHire(ctx, req, exec)
}

func executePearlHire(ctx context.Context, req clientproto.PearlPlaceHireRequest, exec pearlHireExecution) (json.RawMessage, error) {
	if exec.preflight == nil || exec.hire == nil || exec.outcome == nil || exec.markFailed == nil || exec.lockSession == nil {
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
	raw, err := exec.hire(ctx, req)
	if err != nil {
		exec.lockSession("珍珠雇佣请求结果不明确，当前会话已锁定以避免重复扣券")
		exec.markFailed(snapshot.TargetUID, clock())
		return nil, fmt.Errorf("pearlPlace.hire: %w", err)
	}
	fallback, fallbackErr := pearlHireGoldFallback(raw)
	if babigame.HasPayload(raw) && exec.apply != nil {
		exec.apply(raw)
	}
	if fallbackErr != nil {
		reason := "珍珠雇佣响应中的 3.0 金币回退字段格式异常，当前会话已锁定"
		exec.lockSession(reason)
		exec.markFailed(snapshot.TargetUID, clock())
		return nil, fmt.Errorf("%s: %w", reason, fallbackErr)
	}
	if fallback {
		reason := "珍珠雇佣触发金币回退，当前会话已锁定；不会自动消耗金币"
		exec.lockSession(reason)
		exec.markFailed(snapshot.TargetUID, clock())
		return nil, fmt.Errorf("%s", reason)
	}
	success, failCount, known := exec.outcome(snapshot)
	if known && failCount > 0 {
		exec.markFailed(snapshot.TargetUID, clock())
		return nil, fmt.Errorf("pearlPlace.hire candidate was contested (hireFailCnt=%d)", failCount)
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
// integer zero is safe; any nonzero value is fallback and any present malformed value is an
// error that must lock the session.
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

func stateDeltaOperation[Req any](
	build func(*automation.PlannedOp) (Req, error),
	call func(context.Context, *clientrpc.Client, Req) (babigame.RPCResponse[clientproto.StateDelta], error),
) operationSpec {
	return operationSpec{
		args: func(op *automation.PlannedOp) (any, error) {
			return build(op)
		},
		run: func(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
			req, err := build(op)
			if err != nil {
				return nil, err
			}
			return checkedStateDelta(call(ctx, rt.rpc, req))
		},
	}
}

func checkedStateDelta(resp babigame.RPCResponse[clientproto.StateDelta], err error) (json.RawMessage, error) {
	v, d, err := rpcResult(resp, err)
	return checkedPayload(v, d, err)
}

func runFmlRaceEnter(ctx context.Context, rt operationRuntime, _ *automation.PlannedOp) (json.RawMessage, error) {
	if rt.runner == nil || rt.runner.state == nil {
		return nil, fmt.Errorf("fmlRace.enter requires runner state")
	}
	v, d, err := rpcResult(rt.rpc.FmlRace().Enter(
		ctx,
		clientproto.FmlRaceEnterRequest{},
		babigame.WithPayloadApply(false),
	))
	v, err = checkedPayload(v, d, err)
	if err != nil {
		return nil, err
	}
	if babigame.HasPayload(v) {
		v = normalizeFmlRaceEnterV(v)
		rt.runner.state.ApplyV(v)
		rt.runner.state.MarkFmlRaceLvlSyncAttempt()
		// Enter may push sparse 114/110. Force the next tick to getTaskList so
		// full-pool reconcile can replace stale Taken (e.g. 鹤望兰 score 0).
		rt.runner.state.MarkFmlRaceTasksUnobserved()
	}
	return v, nil
}

// normalizeFmlRaceEnterV wraps a bare IFmlTot-shaped payload under namespace 25.
// Some enter/getTaskList/getFmlRaceUsrRankList responses place fields like
// 111/117/116 at the top level of v; ApplyV expects them under "25". A bare
// top-level 116 here is the race member rank list (25.116), never the benefit
// box namespace — this helper is only called on race RPC payloads.
func normalizeFmlRaceEnterV(v json.RawMessage) json.RawMessage {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(v, &top); err != nil || len(top) == 0 {
		return v
	}
	if _, ok := top["25"]; ok {
		return v
	}
	_, hasBatch := top["111"]
	_, hasCurRcd := top["117"]
	_, hasGroup := top["112"]
	_, hasTasks := top["114"]
	_, hasUsr := top["110"]
	_, hasRank := top["116"]
	if !hasBatch && !hasCurRcd && !hasGroup && !hasTasks && !hasUsr && !hasRank {
		return v
	}
	wrapped, err := json.Marshal(map[string]json.RawMessage{"25": v})
	if err != nil {
		return v
	}
	return wrapped
}

func runFmlRaceGetTaskList(ctx context.Context, rt operationRuntime, _ *automation.PlannedOp) (json.RawMessage, error) {
	if rt.runner == nil || rt.runner.state == nil {
		return nil, fmt.Errorf("fmlRace.getTaskList requires runner state")
	}
	v, d, err := rpcResult(rt.rpc.FmlRace().GetTaskList(
		ctx,
		clientproto.FmlRaceGetTaskListRequest{},
		babigame.WithPayloadApply(false),
	))
	v, err = checkedPayload(v, d, err)
	if err != nil {
		return nil, err
	}
	if babigame.HasPayload(v) {
		// Same bare IFmlTot shape as enter: top-level 114/110 must sit under
		// namespace 25, or ApplyV treats 114 as waterwheel and race
		// TasksObserved never sticks — planner then re-syncs every tick.
		v = normalizeFmlRaceEnterV(v)
		rt.runner.state.ApplyVFullFmlRaceTaskPool(v)
	}
	// Successful RPC with empty/no-114 payload must still clear the
	// !TasksObserved early-exit, or sync loops on the decision interval.
	if !rt.runner.state.FmlRace().TasksObserved {
		rt.runner.state.MarkFmlRaceTasksSynced()
	}
	// Piggyback member rank list so personal score/rank stay fresh whenever
	// the task pool syncs (enter bootstrap, TTL refresh, progress catch-up).
	if batchID := rt.runner.state.FmlRace().BatchID; batchID > 0 {
		// Soft: pool sync already succeeded; rank can retry on the next tick.
		_, _ = runFmlRaceGetUsrRankList(ctx, rt, &automation.PlannedOp{TaskMsID: batchID})
	}
	return v, nil
}

func runFmlRaceGetUsrRankList(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	if rt.runner == nil || rt.runner.state == nil {
		return nil, fmt.Errorf("fmlRace.getFmlRaceUsrRankList requires runner state")
	}
	batchID := op.TaskMsID
	if batchID <= 0 {
		batchID = rt.runner.state.FmlRace().BatchID
	}
	if batchID <= 0 {
		return nil, fmt.Errorf("fmlRace.getFmlRaceUsrRankList requires batchId")
	}
	// Generated FmlRaceGetFmlRaceUsrRankListRequest.BatchId is int32; race
	// batchIds are millisecond timestamps, so send an int64 map value.
	v, d, err := rpcResult(rt.rpc.CallStateDelta(
		ctx,
		clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String(),
		map[string]any{"batchId": batchID},
		babigame.WithPayloadApply(false),
	))
	// Record the attempt even when the call fails: the planner emits this sync
	// as an early return, and an unmarked failure would starve every other
	// race op (finish/giveUp/take) behind endless retries.
	rt.runner.state.MarkFmlRaceQuotaSyncAttempt()
	v, err = checkedPayload(v, d, err)
	if err != nil {
		return nil, err
	}
	if babigame.HasPayload(v) {
		v = normalizeFmlRaceEnterV(v)
		rt.runner.state.ApplyV(v)
	}
	return v, nil
}

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

func runWaterwheelRecv(ctx context.Context, rt operationRuntime, _ *automation.PlannedOp) (json.RawMessage, error) {
	if rt.runner != nil && rt.runner.state.WaterwheelNextClaimRequiresSkip() {
		if v, d, err := rpcResult(rt.rpc.Waterwheel().Skip(ctx, clientproto.WaterwheelSkipRequest{})); err != nil || d.IsError() {
			return checkedPayload(v, d, err)
		}
	}
	return checkedStateDelta(rt.rpc.Waterwheel().Recv(ctx, clientproto.WaterwheelRecvRequest{}))
}

func (r *Runner) executePlannedOp(ctx context.Context, client *babigame.Client, session *babigame.Session, op *automation.PlannedOp) (json.RawMessage, error) {
	if op == nil {
		return nil, fmt.Errorf("nil planned operation")
	}
	// This transport-level denylist is intentionally evaluated before the
	// operation registry. Even an accidental future registry entry cannot send
	// a dessert game RPC while live replay/lifecycle evidence is incomplete.
	if isHardBlockedDessertGameOperation(op.Kind) {
		return nil, fmt.Errorf("dessert live game RPC %s is compile-time blocked", op.Kind)
	}
	spec, ok := operationSpecFor(op.Kind)
	if !ok {
		return nil, fmt.Errorf("unsupported planned operation %s", op.Kind)
	}
	rawRPC := babigame.NewRPCClient(
		client,
		session,
		babigame.WithDefaultTimeout(30*time.Second),
		babigame.WithApplyV(r.state.ApplyV),
	)
	rt := operationRuntime{runner: r, rpc: clientrpc.NewClient(rawRPC)}
	return spec.run(ctx, rt, op)
}

func isHardBlockedDessertGameOperation(kind string) bool {
	switch kind {
	case clientproto.RPCActDessertGameStart.String(),
		clientproto.RPCActDessertGameSync.String(),
		clientproto.RPCActDessertGameOver.String():
		return true
	default:
		return false
	}
}

func checkedPayload(v json.RawMessage, d babigame.WSResponseD, err error) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	if d.IsError() {
		msg := d.ErrorMsg()
		if msg == "" {
			msg = "server returned error"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return v, nil
}
