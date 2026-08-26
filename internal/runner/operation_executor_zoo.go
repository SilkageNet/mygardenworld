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
