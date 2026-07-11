package runner

import (
	"context"
	"encoding/json"
	"fmt"
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
	rawRPC *babigame.RPCClient
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
	if op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
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

func rawStateDeltaOperation(
	build func(*automation.PlannedOp) (map[string]any, error),
	name clientproto.RPCName,
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
			return checkedStateDelta(babigame.CallRPC[clientproto.StateDelta](ctx, rt.rawRPC, name, req))
		},
	}
}

func checkedStateDelta(resp babigame.RPCResponse[clientproto.StateDelta], err error) (json.RawMessage, error) {
	v, d, err := rpcResult(resp, err)
	return checkedPayload(v, d, err)
}

func runSignTypeSign(ctx context.Context, rt operationRuntime, _ *automation.PlannedOp) (json.RawMessage, error) {
	if v, d, err := rpcResult(rt.rpc.SignType().Enter(ctx, clientproto.SignTypeEnterRequest{Type: 1})); err != nil || d.IsError() {
		return checkedPayload(v, d, err)
	} else if babigame.HasPayload(v) {
		rt.runner.state.ApplyV(v)
	}
	if v, d, err := rpcResult(rt.rpc.SignType().Sign(ctx, clientproto.SignTypeSignRequest{Type: 1})); err != nil || d.IsError() {
		return checkedPayload(v, d, err)
	} else if babigame.HasPayload(v) {
		rt.runner.state.ApplyV(v)
	}
	return checkedStateDelta(rt.rpc.SignType().Recv(ctx, clientproto.SignTypeRecvRequest{Type: 1}))
}

func runWaterwheelRecv(ctx context.Context, rt operationRuntime, _ *automation.PlannedOp) (json.RawMessage, error) {
	if rt.runner != nil && rt.runner.state.WaterwheelNextClaimRequiresSkip() {
		if v, d, err := rpcResult(rt.rpc.Waterwheel().Skip(ctx, clientproto.WaterwheelSkipRequest{})); err != nil || d.IsError() {
			return checkedPayload(v, d, err)
		} else if babigame.HasPayload(v) {
			rt.runner.state.ApplyV(v)
		}
	}
	return checkedStateDelta(rt.rpc.Waterwheel().Recv(ctx, clientproto.WaterwheelRecvRequest{}))
}

func (r *Runner) executePlannedOp(ctx context.Context, client *babigame.Client, session *babigame.Session, op *automation.PlannedOp) (json.RawMessage, error) {
	if op == nil {
		return nil, fmt.Errorf("nil planned operation")
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
	rt := operationRuntime{runner: r, rpc: clientrpc.NewClient(rawRPC), rawRPC: rawRPC}
	return spec.run(ctx, rt, op)
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
