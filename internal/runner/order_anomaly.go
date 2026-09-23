package runner

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

// serverAnomalyOrderPause is how long ordinary resident/customer order work
// stays paused after the server returns code 5000 (catalog: character data
// anomaly / re-login later). Per-target OperationIDs otherwise keep retrying
// flowerOrderRqst.showR / genOrder every few seconds across many boxes.
const serverAnomalyOrderPause = 15 * time.Minute

// isServerDataAnomalyError reports numeric code 5000. Catalog text asks the
// player to re-login later; empty args mean no absolute retry timestamp.
func isServerDataAnomalyError(err error) bool {
	if err == nil {
		return false
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil && rpcErr.Envelope.ErrorCode() == 5000 {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, `"code":5000`) || strings.Contains(msg, `"code": 5000`)
}

// isOrderServerAnomalyError covers order finish/gen/reject (and customer craft)
// rejects with code 5000. benefitBox.draw keeps its own handler.
func isOrderServerAnomalyError(kind string, err error) bool {
	if !isServerDataAnomalyError(err) {
		return false
	}
	switch kind {
	case clientproto.RPCOrderFlowerFinishOrder.String(),
		clientproto.RPCOrderFlowerFinishSatinOrder.String(),
		clientproto.RPCOrderFlowerFinishDecorateOrder.String(),
		clientproto.RPCOrderCustomerGenOrder.String(),
		clientproto.RPCOrderCustomerFinishOrder.String(),
		clientproto.RPCOrderCustomerRejectOrder.String(),
		clientproto.RPCFlowerArtMakeFlowerArt.String():
		return true
	default:
		return false
	}
}

func isOrderAnomalyPauseOp(op *automation.PlannedOp) bool {
	if op == nil {
		return false
	}
	switch op.Kind {
	case clientproto.RPCOrderFlowerFinishOrder.String(),
		clientproto.RPCOrderFlowerFinishSatinOrder.String(),
		clientproto.RPCOrderFlowerFinishDecorateOrder.String(),
		clientproto.RPCOrderCustomerGenOrder.String(),
		clientproto.RPCOrderCustomerFinishOrder.String(),
		clientproto.RPCOrderCustomerRejectOrder.String(),
		clientproto.RPCFlowerArtMakeFlowerArt.String():
		return true
	default:
		return strings.HasPrefix(op.Domain, "order.resident") ||
			strings.HasPrefix(op.Domain, "order.customer")
	}
}

func (r *Runner) orderAnomalyPauseActive(now time.Time) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.orderAnomalyUntil.After(now)
}

func (r *Runner) clearOrderAnomalyPause() {
	r.mu.Lock()
	r.orderAnomalyUntil = time.Time{}
	r.lastOrderAnomalyReason = ""
	if r.operationCooldowns != nil {
		delete(r.operationCooldowns, "order.server_anomaly")
	}
	r.mu.Unlock()
}

// markOrderServerAnomaly pauses resident/customer order automation after code
// 5000. Deduped by reason so a multi-box spam window emits one log line.
func (r *Runner) markOrderServerAnomaly(op *automation.PlannedOp, now time.Time, err error) {
	if r == nil || op == nil || !isOrderAnomalyPauseOp(op) {
		return
	}
	reason := "服务端提示角色数据异常（code 5000），暂停居民/顾客订单"
	if tip := stateMsgCode5000Hint(); tip != "" {
		reason = tip
	}
	until := now.Add(serverAnomalyOrderPause)

	r.mu.Lock()
	already := r.orderAnomalyUntil.After(now) && r.lastOrderAnomalyReason == reason
	if until.After(r.orderAnomalyUntil) {
		r.orderAnomalyUntil = until
	}
	r.lastOrderAnomalyReason = reason
	// Force rqst re-send after the pause; a prior failed showR never set the
	// sent flag, but a half-succeeded customer rqst would otherwise stick.
	r.rqst.flowerOrderSent = false
	r.rqst.customerOrderSent = false
	r.mu.Unlock()

	// Shared cooldown key so every per-target finish/gen shares one pause.
	shared := &automation.PlannedOp{
		OperationID: "order.server_anomaly",
		CooldownKey: "order.server_anomaly",
		Kind:        op.Kind,
		Lane:        automation.LaneSide,
		Category:    automation.CategoryOrder,
		Domain:      "order",
		Action:      "blocked",
	}
	_ = r.setSideOperationCooldown(shared, now, err, reason, serverAnomalyOrderPause)

	if already {
		return
	}
	r.emit(Event{
		Kind:     "operation_deferred",
		Category: automation.CategoryOrder,
		Domain:   "order",
		Action:   "blocked",
		Label:    "订单风控",
		Message:  fmt.Sprintf("%s，%s 前跳过接单/交付/生成（避免空刷校验）", reason, until.Format("15:04:05")),
		Level:    "warn",
	})
}

func stateMsgCode5000Hint() string {
	// Keep the planner message short; full catalog text is long and templated.
	return "服务端提示角色数据异常（code 5000），请稍后重试/重登"
}

// orderAnomalyBlocks reports whether this op should wait out a code-5000 pause.
func (r *Runner) orderAnomalyBlocks(op *automation.PlannedOp, now time.Time) bool {
	if !isOrderAnomalyPauseOp(op) {
		return false
	}
	if r.orderAnomalyPauseActive(now) {
		return true
	}
	// Also honor the shared cooldown key if set without the until field
	// (tests / restarts mid-window).
	shared := &automation.PlannedOp{
		CooldownKey: "order.server_anomaly",
		Lane:        automation.LaneSide,
		Kind:        op.Kind,
	}
	_, cooling := r.operationCoolingDown(shared, now)
	return cooling
}
