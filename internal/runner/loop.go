package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

const (
	waterRetryWait        = 5 * time.Minute
	harvestRetryWait      = 30 * time.Second
	waterSourceSyncPeriod = 60 * time.Second
	freeWaterRetryWait    = time.Hour
	dailyTaskRetryWait    = 30 * time.Minute
)

// CallRaw forwards an arbitrary RPC over the WebSocket. Returns the v field.
func (r *Runner) CallRaw(ctx context.Context, name string, args map[string]any, timeout time.Duration) (json.RawMessage, error) {
	r.mu.RLock()
	c := r.client
	s := r.session
	sessionInvalidated := r.sessionInvalidated
	sessionInvalidatedReason := r.sessionInvalidatedReason
	r.mu.RUnlock()
	if sessionInvalidated {
		return nil, fmt.Errorf("session invalidated: %s", sessionInvalidatedReason)
	}
	if c == nil {
		return nil, errors.New("not connected")
	}
	v, d, err := c.RPC(ctx, name, args, s.RouteArg(), timeout)
	if err != nil {
		return nil, err
	}
	if d.IsError() {
		return nil, fmt.Errorf("server: %s", d.ErrorMsg())
	}
	if babigame.HasPayload(v) {
		r.state.ApplyV(v)
	}
	return v, nil
}

func (r *Runner) decisionLoop(ctx context.Context) {
	for {
		timer := time.NewTimer(r.tickInterval())
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

	// Anti-cheat verification: send once per session before the first
	// harvest or plant batch.
	if isHarvestOp(op.Kind) {
		if err := r.ensureHarvestRqst(ctx); err != nil {
			r.log.Debug("harvest rqst failed", "err", err)
		}
	} else if op.Kind == "usrLand.plant" || op.Kind == "usrLand.plantBatch" {
		if err := r.ensurePlantRqst(ctx); err != nil {
			r.log.Debug("plant rqst failed", "err", err)
		}
	}

	// Skip water operations if blocked (water drops exhausted).
	r.mu.RLock()
	waterBlocked := r.waterBlocked
	waterBlockedUntil := r.waterBlockedUntil
	r.mu.RUnlock()
	if waterBlocked && (op.Kind == "usrLand.water" || op.Kind == "usrLand.waterBatch") {
		if time.Now().Before(waterBlockedUntil) {
			return
		}
		r.setWaterBlocked(false)
	}
	reservedWaterDrops := int32(0)
	if op.Kind == "usrLand.water" || op.Kind == "usrLand.waterBatch" {
		reservedWaterDrops = int32(len(op.LandIDs))
		if !r.state.ReserveWaterDrops(reservedWaterDrops, now) {
			return
		}
		defer r.state.ReleaseWaterDropsReservation(reservedWaterDrops)
	}
	r.emit(Event{
		Kind:        "operation_planned",
		Message:     fmt.Sprintf("计划执行 %s (田地=%v)", opDesc(op), op.LandIDs),
		PayloadJSON: operationPayload(op, nil, nil),
	})
	v, err := r.CallRaw(ctx, op.Kind, op.Args, 30*time.Second)
	if err != nil {
		if isHarvestOp(op.Kind) && isFlowerNotMatureError(err) {
			r.setHarvestBlockedUntil(op.LandIDs, time.Now().Add(harvestRetryWait))
			r.emit(Event{
				Kind:        "operation_failed",
				Message:     fmt.Sprintf("%s 暂缓: 服务端提示鲜花尚未成熟，稍后重试 (田地=%v)", opDesc(op), op.LandIDs),
				PayloadJSON: operationPayload(op, nil, err),
				Level:       "warn",
			})
			_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, op.Args, map[string]any{"error": err.Error(), "retryAfterSeconds": int(harvestRetryWait.Seconds())})
			return
		}
		r.emit(Event{
			Kind:        "operation_failed",
			Message:     fmt.Sprintf("%s 失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(op, nil, err),
		})
		_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, op.Args, map[string]any{"error": err.Error()})
		// Block water operations on server error (likely water drops exhausted).
		if op.Kind == "usrLand.water" || op.Kind == "usrLand.waterBatch" {
			r.setWaterBlockedUntil(time.Now().Add(waterRetryWait))
		}
		return
	}
	r.emit(Event{
		Kind:        "operation_ack",
		Message:     fmt.Sprintf("%s 完成 (田地=%v)", opDesc(op), op.LandIDs),
		PayloadJSON: operationPayload(op, v, nil),
	})
	_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, op.Args, json.RawMessage(v))

	// Some successful water RPC responses omit inventory deltas. When the
	// server did include item 7, ApplyV has already installed the authoritative
	// remaining drop count, so do not spend it locally again.
	if (op.Kind == "usrLand.water" || op.Kind == "usrLand.waterBatch") && !waterResponseIncludesDrops(v) {
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
			Args:    map[string]any{"landId": int(id)},
		}
	}
	return nil
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

	v, d, err := client.RPC(ctx, "waterwheel.enter", map[string]any{}, session.RouteArg(), 10*time.Second)
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

func operationPayload(op *automation.PlannedOp, raw json.RawMessage, err error) string {
	payload := map[string]any{
		"rpc":      op.Kind,
		"label":    opDesc(op),
		"landIds":  op.LandIDs,
		"args":     op.Args,
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
