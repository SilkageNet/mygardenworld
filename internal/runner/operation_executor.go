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
}

type harvestCallResult struct {
	LandID int32           `json:"landId"`
	Raw    json.RawMessage `json:"raw,omitempty"`
}

type harvestLandError struct {
	LandID int32
	Err    error
}

// pearlHireCandidateFallbackError is an authoritative, candidate-scoped
// outcome rather than an RPC failure. The server may consume the submitted
// hire ticket while telling the client that this candidate requires the
// configured gold alternative. Automation never follows that alternative.
type pearlHireCandidateFallbackError struct {
	TicketSpent bool
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
	preflight     func(time.Time) (state.PearlHireAttemptSnapshot, error)
	hire          func(context.Context, clientproto.PearlPlaceHireRequest) (json.RawMessage, error)
	apply         func(json.RawMessage)
	outcome       func(state.PearlHireAttemptSnapshot) (bool, int32, bool)
	ticketSpent   func(state.PearlHireAttemptSnapshot) bool
	markFailed    func(int64, time.Time)
	skipCandidate func(int64)
	noteUsed      func(context.Context, time.Time)
	lockSession   func(string)
	now           func() time.Time
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

func (e *pearlHireCandidateFallbackError) Error() string {
	if e != nil && e.TicketSpent {
		return "珍珠雇佣触发金币替代提示，本次已消耗 1 张雇佣券；已在当前会话跳过该候选，不会自动使用金币"
	}
	return "珍珠雇佣触发金币替代提示，未观察到雇佣券扣除；已在当前会话跳过该候选，不会自动使用金币"
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
	rt := operationRuntime{runner: r, rpc: clientrpc.NewClient(rawRPC)}
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
