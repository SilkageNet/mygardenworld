package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func fmlEnterSyncRequest() clientproto.FmlEnterRequest {
	return clientproto.FmlEnterRequest{Fml: 1, Mb: 1, MbL: 1}
}

func runFmlEnter(ctx context.Context, rt operationRuntime, _ *automation.PlannedOp) (json.RawMessage, error) {
	if rt.runner == nil || rt.runner.state == nil {
		return nil, fmt.Errorf("fml.enter requires runner state")
	}
	v, d, err := rpcResult(rt.rpc.Fml().Enter(
		ctx,
		fmlEnterSyncRequest(),
		babigame.WithPayloadApply(false),
	))
	// Mark failures and empty acknowledgements too, otherwise an omitted mb
	// payload retries every decision tick and can starve ordinary operations.
	rt.runner.state.MarkFmlMemberPositionSyncAttempt()
	v, err = checkedPayload(v, d, err)
	if err != nil {
		return nil, err
	}
	if babigame.HasPayload(v) {
		v = normalizeFmlEnterV(v)
		rt.runner.state.ApplyV(v)
	}
	return v, nil
}

// normalizeFmlEnterV wraps the IFmlTot-shaped response returned by fml.enter
// under namespace 25. The RPC commonly returns bare fields such as 0/1/102;
// feeding those directly to ApplyV would interpret them as top-level namespaces
// and silently lose IFmlTot.mb.pos.
func normalizeFmlEnterV(v json.RawMessage) json.RawMessage {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(v, &top); err != nil || len(top) == 0 {
		return v
	}
	if _, ok := top["25"]; ok {
		return v
	}
	wrapped, err := json.Marshal(map[string]json.RawMessage{"25": v})
	if err != nil {
		return v
	}
	return wrapped
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
	// A successful enter is still an authoritative probe when the channel
	// returns an empty delta. Record it unconditionally so planner bootstrap
	// backs off instead of tight-looping and starving farm/order operations.
	rt.runner.state.MarkFmlRaceLvlSyncAttempt()
	if babigame.HasPayload(v) {
		v = normalizeFmlRaceEnterV(v)
		rt.runner.state.ApplyV(v)
		// Enter may push sparse 114/110. Force the next tick to getTaskList so
		// full-pool reconcile can replace stale Taken (e.g. 鹤望兰 score 0).
		rt.runner.state.MarkFmlRaceTaskPoolStale()
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
	// Record every successful round-trip, including empty/no-114 deltas. This
	// clears an explicit stale gate, backs off a never-observed response, and
	// records incomplete-target refresh attempts so they cannot live-lock the
	// decision loop.
	rt.runner.state.NoteFmlRaceTaskPoolSync(time.Now())
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
