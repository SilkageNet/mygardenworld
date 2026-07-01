package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientrpc"
)

const (
	waterRetryWait        = 5 * time.Minute
	harvestRetryWait      = 30 * time.Second
	waterSourceSyncPeriod = 60 * time.Second
	freeWaterRetryWait    = time.Hour
	dailyTaskRetryWait    = 30 * time.Minute
)

func (r *Runner) decisionLoop(ctx context.Context) {
	for {
		interval := r.tickInterval()
		r.setNextDecisionAt(time.Now().Add(interval))
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			r.tick(ctx)
		}
	}
}

func (r *Runner) tickInterval() time.Duration {
	r.mu.RLock()
	p := r.policy
	r.mu.RUnlock()
	d := time.Duration(0)
	if p != nil {
		d = time.Duration(p.GetDecisionIntervalSeconds() * float64(time.Second))
	}
	if d <= 0 {
		d = 4 * time.Second
	}
	return d
}

func (r *Runner) tick(ctx context.Context) {
	r.mu.RLock()
	p := r.policy
	client := r.client
	session := r.session
	sessionInvalidated := r.sessionInvalidated
	r.mu.RUnlock()

	if sessionInvalidated {
		return
	}
	if client == nil || session == nil {
		return
	}
	now := time.Now()
	if r.state.RefreshWaterDrops(now) {
		r.setWaterBlocked(false)
	}
	r.tickWaterSourceSync(ctx, client, session)
	if r.isSessionInvalidated() {
		return
	}
	if p == nil || !p.AutomationEnabled {
		return
	}

	// 培育自动化（60 秒节流，与 misc 独立计时）
	if time.Since(r.lastCultivateTick) >= 60*time.Second {
		r.lastCultivateTick = time.Now()
		r.tickCultivate(ctx, client, session)
		if r.isSessionInvalidated() {
			return
		}
	}

	// 订单/任务/主线自动化
	r.tickMisc(ctx, client, session)
	if r.isSessionInvalidated() {
		return
	}

	op := automation.Plan(r.state, p, now)
	if op == nil {
		return
	}
	op = r.applyHarvestBlocks(op, now)
	if op == nil {
		return
	}

	var opErr error
	finishOperation := r.beginOperation(op.Kind)
	defer func() { finishOperation(opErr) }()

	if err := r.ensurePlannedOperationRqst(ctx, op); err != nil {
		opErr = fmt.Errorf("rqst: %w", err)
		r.emit(Event{
			Kind:        "operation_failed",
			Message:     fmt.Sprintf("%s 已跳过: 前置校验失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(op, nil, nil, err),
			Level:       "warn",
		})
		_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, nil, map[string]any{"error": err.Error(), "stage": "rqst"})
		return
	}

	// Skip water operations if blocked (water drops exhausted).
	r.mu.RLock()
	waterBlocked := r.waterBlocked
	waterBlockedUntil := r.waterBlockedUntil
	r.mu.RUnlock()
	if waterBlocked && isWaterOp(op.Kind) {
		if time.Now().Before(waterBlockedUntil) {
			opErr = fmt.Errorf("water blocked until %s", waterBlockedUntil.Format(time.RFC3339))
			return
		}
		r.setWaterBlocked(false)
	}
	reservedWaterDrops := int32(0)
	if isWaterOp(op.Kind) {
		reservedWaterDrops = int32(len(op.LandIDs))
		if !r.state.ReserveWaterDrops(reservedWaterDrops, now) {
			opErr = fmt.Errorf("insufficient local water drops")
			return
		}
		defer r.state.ReleaseWaterDropsReservation(reservedWaterDrops)
	}
	args, err := operationArgs(op)
	if err != nil {
		opErr = err
		r.emit(Event{
			Kind:        "operation_failed",
			Message:     fmt.Sprintf("%s 失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(op, nil, nil, err),
		})
		_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, nil, map[string]any{"error": err.Error()})
		return
	}
	r.emit(Event{
		Kind:        "operation_planned",
		Message:     fmt.Sprintf("计划执行 %s (田地=%v)", opDesc(op), op.LandIDs),
		PayloadJSON: operationPayload(op, args, nil, nil),
	})
	v, err := r.executePlannedOp(ctx, client, session, op)
	if err != nil {
		opErr = err
		if isHarvestOp(op.Kind) && isFlowerNotMatureError(err) {
			r.setHarvestBlockedUntil(op.LandIDs, time.Now().Add(harvestRetryWait))
			r.emit(Event{
				Kind:        "operation_failed",
				Message:     fmt.Sprintf("%s 暂缓: 服务端提示鲜花尚未成熟，稍后重试 (田地=%v)", opDesc(op), op.LandIDs),
				PayloadJSON: operationPayload(op, args, nil, err),
				Level:       "warn",
			})
			_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, map[string]any{"error": err.Error(), "retryAfterSeconds": int(harvestRetryWait.Seconds())})
			return
		}
		r.emit(Event{
			Kind:        "operation_failed",
			Message:     fmt.Sprintf("%s 失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(op, args, nil, err),
		})
		_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, map[string]any{"error": err.Error()})
		// Block water operations on server error (likely water drops exhausted).
		if isWaterOp(op.Kind) {
			r.setWaterBlockedUntil(time.Now().Add(waterRetryWait))
		}
		return
	}
	r.emit(Event{
		Kind:        "operation_ack",
		Message:     fmt.Sprintf("%s 完成 (田地=%v)", opDesc(op), op.LandIDs),
		PayloadJSON: operationPayload(op, args, v, nil),
	})
	_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, json.RawMessage(v))

	// Some successful water RPC responses omit inventory deltas. When the
	// server did include item 7, ApplyV has already installed the authoritative
	// remaining drop count, so do not spend it locally again.
	if isWaterOp(op.Kind) && !waterResponseIncludesDrops(v) {
		r.state.MarkLandsWatered(op.LandIDs)
	}
}

func (r *Runner) applyHarvestBlocks(op *automation.PlannedOp, now time.Time) *automation.PlannedOp {
	if op == nil || !isHarvestOp(op.Kind) {
		return op
	}
	if len(op.LandIDs) == 0 {
		return op
	}
	blocked := make(map[int32]bool, len(op.LandIDs))
	anyBlocked := false
	r.mu.RLock()
	for _, id := range op.LandIDs {
		until := r.harvestBlockedUntil[id]
		if until.IsZero() || !now.Before(until) {
			continue
		}
		blocked[id] = true
		anyBlocked = true
	}
	r.mu.RUnlock()
	if !anyBlocked {
		return op
	}
	if op.Kind == "usrLand.harvest" {
		return nil
	}

	for _, id := range op.LandIDs {
		if blocked[id] {
			continue
		}
		return &automation.PlannedOp{
			Kind:    "usrLand.harvest",
			LandIDs: []int32{id},
		}
	}
	return nil
}

func (r *Runner) executePlannedOp(ctx context.Context, client *babigame.Client, session *babigame.Session, op *automation.PlannedOp) (json.RawMessage, error) {
	if op == nil {
		return nil, fmt.Errorf("nil planned operation")
	}
	rpc := clientrpc.NewClient(babigame.NewRPCClient(
		client,
		session,
		babigame.WithDefaultTimeout(30*time.Second),
		babigame.WithApplyV(r.state.ApplyV),
	))
	switch op.Kind {
	case clientproto.RPCUsrLandHarvestOneKey.String():
		resp, err := rpc.UsrLand().HarvestOneKey(ctx, clientproto.UsrLandHarvestOneKeyRequest{})
		return resp.Payload, err
	case clientproto.RPCUsrLandHarvest.String():
		landID, err := plannedOpSingleLandID(op)
		if err != nil {
			return nil, err
		}
		resp, err := rpc.UsrLand().Harvest(ctx, clientproto.UsrLandHarvestRequest{LandId: landID})
		return resp.Payload, err
	case clientproto.RPCUsrLandPlantBatch.String():
		if op.FlowerID == 0 {
			return nil, fmt.Errorf("plantBatch missing flower id")
		}
		resp, err := rpc.UsrLand().PlantBatch(ctx, clientproto.UsrLandPlantBatchRequest{LandIds: op.LandIDs, FlowerId: op.FlowerID})
		return resp.Payload, err
	case clientproto.RPCUsrLandPlant.String():
		if op.FlowerID == 0 {
			return nil, fmt.Errorf("plant missing flower id")
		}
		landID, err := plannedOpSingleLandID(op)
		if err != nil {
			return nil, err
		}
		resp, err := rpc.UsrLand().Plant(ctx, clientproto.UsrLandPlantRequest{LandId: landID, FlowerId: op.FlowerID})
		return resp.Payload, err
	case clientproto.RPCUsrLandWaterBatch.String():
		resp, err := rpc.UsrLand().WaterBatch(ctx, clientproto.UsrLandWaterBatchRequest{LandIds: op.LandIDs})
		return resp.Payload, err
	case clientproto.RPCUsrLandWaterOneKey.String():
		resp, err := rpc.UsrLand().WaterOneKey(ctx, clientproto.UsrLandWaterOneKeyRequest{})
		return resp.Payload, err
	case clientproto.RPCUsrLandWater.String():
		landID, err := plannedOpSingleLandID(op)
		if err != nil {
			return nil, err
		}
		resp, err := rpc.UsrLand().Water(ctx, clientproto.UsrLandWaterRequest{LandId: landID})
		return resp.Payload, err
	default:
		return nil, fmt.Errorf("unsupported planned operation %s", op.Kind)
	}
}

func operationArgs(op *automation.PlannedOp) (any, error) {
	if op == nil {
		return nil, fmt.Errorf("nil planned operation")
	}
	switch op.Kind {
	case clientproto.RPCUsrLandHarvestOneKey.String():
		return clientproto.UsrLandHarvestOneKeyRequest{}, nil
	case clientproto.RPCUsrLandHarvest.String():
		landID, err := plannedOpSingleLandID(op)
		if err != nil {
			return nil, err
		}
		return clientproto.UsrLandHarvestRequest{LandId: landID}, nil
	case clientproto.RPCUsrLandPlantBatch.String():
		if op.FlowerID == 0 {
			return nil, fmt.Errorf("plantBatch missing flower id")
		}
		return clientproto.UsrLandPlantBatchRequest{LandIds: op.LandIDs, FlowerId: op.FlowerID}, nil
	case clientproto.RPCUsrLandPlant.String():
		if op.FlowerID == 0 {
			return nil, fmt.Errorf("plant missing flower id")
		}
		landID, err := plannedOpSingleLandID(op)
		if err != nil {
			return nil, err
		}
		return clientproto.UsrLandPlantRequest{LandId: landID, FlowerId: op.FlowerID}, nil
	case clientproto.RPCUsrLandWaterBatch.String():
		return clientproto.UsrLandWaterBatchRequest{LandIds: op.LandIDs}, nil
	case clientproto.RPCUsrLandWaterOneKey.String():
		return clientproto.UsrLandWaterOneKeyRequest{}, nil
	case clientproto.RPCUsrLandWater.String():
		landID, err := plannedOpSingleLandID(op)
		if err != nil {
			return nil, err
		}
		return clientproto.UsrLandWaterRequest{LandId: landID}, nil
	default:
		return nil, fmt.Errorf("unsupported planned operation %s", op.Kind)
	}
}

func isWaterOp(kind string) bool {
	return kind == clientproto.RPCUsrLandWater.String() ||
		kind == clientproto.RPCUsrLandWaterBatch.String() ||
		kind == clientproto.RPCUsrLandWaterOneKey.String()
}

func plannedOpSingleLandID(op *automation.PlannedOp) (int32, error) {
	if op == nil || len(op.LandIDs) != 1 || op.LandIDs[0] == 0 {
		return 0, fmt.Errorf("operation %s requires exactly one land id", op.Kind)
	}
	return op.LandIDs[0], nil
}

func (r *Runner) setHarvestBlockedUntil(landIDs []int32, until time.Time) {
	if len(landIDs) == 0 {
		return
	}
	r.mu.Lock()
	if r.harvestBlockedUntil == nil {
		r.harvestBlockedUntil = make(map[int32]time.Time)
	}
	for _, id := range landIDs {
		r.harvestBlockedUntil[id] = until
	}
	r.mu.Unlock()
}

func isHarvestOp(kind string) bool {
	return kind == "usrLand.harvest" || kind == "usrLand.harvestOneKey"
}

func (r *Runner) ensurePlannedOperationRqst(ctx context.Context, op *automation.PlannedOp) error {
	if op == nil {
		return nil
	}
	if isHarvestOp(op.Kind) {
		return r.ensureHarvestRqst(ctx)
	}
	if op.Kind == clientproto.RPCUsrLandPlant.String() ||
		op.Kind == clientproto.RPCUsrLandPlantBatch.String() ||
		op.Kind == clientproto.RPCUsrLandPlantOneKey.String() {
		return r.ensurePlantRqst(ctx)
	}
	if isWaterOp(op.Kind) {
		return r.ensureWaterRqst(ctx)
	}
	return nil
}

func isFlowerNotMatureError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "鲜花尚未成熟")
}

func waterResponseIncludesDrops(raw json.RawMessage) bool {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return false
	}
	raw7, ok := top["7"]
	if !ok {
		return false
	}
	var ns7 map[string]json.RawMessage
	if err := json.Unmarshal(raw7, &ns7); err != nil {
		return false
	}
	if raw0, ok := ns7["0"]; ok && nestedMapHasItem(raw0, "32", "7") {
		return true
	}
	raw2, ok := ns7["2"]
	if !ok {
		return false
	}
	return nestedMapHasItem(raw2, "0", "7") || nestedMapHasItem(raw2, "2", "7")
}

func nestedMapHasItem(raw json.RawMessage, field, itemID string) bool {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil {
		return false
	}
	innerRaw, ok := outer[field]
	if !ok {
		return false
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(innerRaw, &inner); err != nil {
		return false
	}
	_, ok = inner[itemID]
	return ok
}

func (r *Runner) tickWaterSourceSync(ctx context.Context, client *babigame.Client, session *babigame.Session) {
	if time.Since(r.lastWaterSyncTick) < waterSourceSyncPeriod {
		return
	}
	r.lastWaterSyncTick = time.Now()

	rpc := r.runnerRPC(client, session)
	v, d, err := rpcResult(rpc.Waterwheel().Enter(ctx, clientproto.WaterwheelEnterRequest{}))
	if r.isSessionInvalidated() {
		return
	}
	if err != nil {
		r.log.Debug("waterwheel sync failed", "err", err)
		return
	}
	if d.IsError() {
		return
	}
	if babigame.HasPayload(v) {
		r.state.ApplyV(v)
	}
}

func operationPayload(op *automation.PlannedOp, args any, raw json.RawMessage, err error) string {
	payload := map[string]any{
		"rpc":      op.Kind,
		"label":    opDesc(op),
		"landIds":  op.LandIDs,
		"args":     args,
		"flowerId": op.FlowerID,
	}
	if len(raw) > 0 {
		payload["raw"] = json.RawMessage(raw)
	}
	if err != nil {
		payload["error"] = err.Error()
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func opDesc(op *automation.PlannedOp) string {
	desc := opKindDesc(op.Kind)
	if op.FlowerID == 0 {
		return desc
	}
	return fmt.Sprintf("%s %s(#%d)", desc, flowerName(int(op.FlowerID)), op.FlowerID)
}
