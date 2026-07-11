package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

type operationAttempt struct {
	op        *automation.PlannedOp
	args      any
	startedAt time.Time
}

type operationResult struct {
	operationAttempt
	raw        json.RawMessage
	err        error
	finishedAt time.Time
}

type operationErrorKind string

const (
	operationErrorOrdinary                operationErrorKind = "ordinary"
	operationErrorHarvestNotMature        operationErrorKind = "harvest_not_mature"
	operationErrorResidentOrderCooldown   operationErrorKind = "resident_order_cooldown"
	operationErrorResidentOrderDailyLimit operationErrorKind = "resident_order_daily_limit"
	operationErrorWaterwheelInvalidData   operationErrorKind = "waterwheel_invalid_data"
	operationErrorWaterwheelDailyLimit    operationErrorKind = "waterwheel_daily_limit"
	operationErrorWaterDropRejected       operationErrorKind = "water_drop_rejected"
	operationErrorTaskGroupFinished       operationErrorKind = "task_group_finished"
)

func classifyOperationError(kind string, err error) operationErrorKind {
	switch {
	case isHarvestOp(kind) && isFlowerNotMatureError(err):
		return operationErrorHarvestNotMature
	case isResidentOrderCooldownError(kind, err):
		return operationErrorResidentOrderCooldown
	case isResidentOrderDailyLimitError(kind, err):
		return operationErrorResidentOrderDailyLimit
	case isWaterwheelInvalidDataError(kind, err):
		return operationErrorWaterwheelInvalidData
	case isWaterwheelDailyLimitError(kind, err):
		return operationErrorWaterwheelDailyLimit
	case isWaterDropResourceRejectedError(kind, err):
		return operationErrorWaterDropRejected
	case isTaskGroupFinishedError(kind, err):
		return operationErrorTaskGroupFinished
	default:
		return operationErrorOrdinary
	}
}

func (r *Runner) handleResourceGateFailure(ctx context.Context, op *automation.PlannedOp, err error) {
	payloadOp := r.cooldownSideOperation(op, time.Now(), err, "", 0)
	r.emit(Event{
		Kind:        "operation_failed",
		Category:    op.Category,
		Domain:      op.Domain,
		Action:      "blocked",
		Message:     fmt.Sprintf("%s 已阻塞: 资源前置校验失败: %v", opDesc(op), err),
		PayloadJSON: operationPayload(payloadOp, nil, nil, err),
		Level:       "warn",
	})
	r.logOperation(ctx, op.Kind, nil, map[string]any{"error": err.Error(), "stage": "resource_gate"})
}

func (r *Runner) handleRqstFailure(ctx context.Context, op *automation.PlannedOp, err, opErr error) {
	payloadOp := r.cooldownSideOperation(op, time.Now(), opErr, "前置校验失败，暂缓重试", 0)
	r.emit(Event{
		Kind:        "operation_failed",
		Category:    op.Category,
		Domain:      op.Domain,
		Action:      "blocked",
		Message:     fmt.Sprintf("%s 已跳过: 前置校验失败: %v", opDesc(op), err),
		PayloadJSON: operationPayload(payloadOp, nil, nil, err),
		Level:       "warn",
	})
	r.logOperation(ctx, op.Kind, nil, map[string]any{"error": err.Error(), "stage": "rqst"})
}

func (r *Runner) handleOperationArgsFailure(ctx context.Context, op *automation.PlannedOp, err error) {
	payloadOp := r.cooldownSideOperation(op, time.Now(), err, "", 0)
	r.emit(Event{
		Kind:        "operation_failed",
		Category:    op.Category,
		Domain:      op.Domain,
		Action:      "failed",
		Message:     fmt.Sprintf("%s 失败: %v", opDesc(op), err),
		PayloadJSON: operationPayload(payloadOp, nil, nil, err),
	})
	r.logOperation(ctx, op.Kind, nil, map[string]any{"error": err.Error()})
}

func (r *Runner) emitOperationPlanned(attempt operationAttempt) {
	r.emit(Event{
		Kind:        "operation_planned",
		Category:    attempt.op.Category,
		Domain:      attempt.op.Domain,
		Action:      attempt.op.Action,
		Message:     fmt.Sprintf("计划执行 %s%s", opDesc(attempt.op), r.opSuffix(attempt.op)),
		PayloadJSON: operationPayload(attempt.op, attempt.args, nil, nil),
	})
}

func (r *Runner) handleOperationError(ctx context.Context, result operationResult) error {
	op, args, err := result.op, result.args, result.err
	switch classifyOperationError(op.Kind, err) {
	case operationErrorHarvestNotMature:
		landIDs := op.LandIDs
		var landErr *harvestLandError
		if errors.As(err, &landErr) && landErr.LandID != 0 {
			landIDs = []int32{landErr.LandID}
		}
		r.setHarvestBlockedUntil(landIDs, result.finishedAt.Add(harvestRetryWait))
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 暂缓: 服务端提示鲜花尚未成熟，稍后重试 (田地=%v)", opDesc(op), landIDs),
			PayloadJSON: operationPayload(op, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "retryAfterSeconds": int(harvestRetryWait.Seconds())})
		return nil
	case operationErrorResidentOrderCooldown:
		payloadOp := r.cooldownSideOperation(op, result.finishedAt, err, "服务端提示订单冷却中", 30*time.Second)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 暂缓: 服务端提示订单冷却中，稍后重试", opDesc(op)),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "cooldown"})
		return nil
	case operationErrorResidentOrderDailyLimit:
		now := result.finishedAt
		r.state.MarkResidentOrderDailyLimitReached(now)
		until, ok := r.state.ResidentOrderDailyLimitReached(now)
		cooldown := 24 * time.Hour
		if ok {
			cooldown = until.Sub(now)
		}
		payloadOp := r.cooldownSideOperation(op, now, err, "服务端提示今日完成订单次数已达上限", cooldown)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 暂停: 服务端提示今日完成订单次数已达上限，已跳过居民订单以继续执行其他流程", opDesc(op)),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "daily_limit"})
		return nil
	case operationErrorWaterwheelInvalidData:
		r.state.MarkWaterwheelUnavailable(result.finishedAt)
		payloadOp := r.cooldownSideOperation(op, result.finishedAt, err, "服务端提示水车数据暂不可领取", time.Minute)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 暂缓: 服务端提示水车数据暂不可领取，稍后重试", opDesc(op)),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "invalid_state"})
		return nil
	case operationErrorWaterwheelDailyLimit:
		r.state.MarkWaterwheelDailyLimitReached(result.finishedAt)
		payloadOp := r.cooldownSideOperation(op, result.finishedAt, err, "服务端提示今日水车领取已达上限", 24*time.Hour)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 暂停: 服务端提示今日水车领取已达上限，已跳过水车以继续执行其他任务", opDesc(op)),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "daily_limit"})
		return nil
	case operationErrorWaterDropRejected:
		r.state.MarkWaterDropsExhausted(result.finishedAt)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 暂缓: 服务端提示水滴不足，已校正本地数量，等待恢复后重试", opDesc(op)),
			PayloadJSON: operationPayload(op, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "resource_stale"})
		return nil
	case operationErrorTaskGroupFinished:
		payloadOp := r.cooldownSideOperation(op, result.finishedAt, err, "服务端提示本组任务已经完结", sideOperationMaxCooldown)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 暂停: 服务端提示本组任务已经完结，已暂缓该任务以继续执行其他流程", opDesc(op)),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "group_finished"})
		return nil
	default:
		payloadOp := r.cooldownSideOperation(op, result.finishedAt, err, "", 0)
		r.emit(Event{
			Kind:        "operation_failed",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "failed",
			Message:     fmt.Sprintf("%s 失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error()})
		return err
	}
}

func (r *Runner) handleOperationSuccess(ctx context.Context, result operationResult) {
	op, args := result.op, result.args
	r.stats.RecordOperationSuccess(op, result.finishedAt)
	r.emit(Event{
		Kind:        "operation_ack",
		Category:    op.Category,
		Domain:      op.Domain,
		Action:      op.Action,
		Message:     fmt.Sprintf("%s 完成%s", opDesc(op), r.opSuffix(op)),
		PayloadJSON: operationPayload(op, args, result.raw, nil),
	})
	r.logOperation(ctx, op.Kind, args, json.RawMessage(result.raw))
	r.clearOperationCooldown(op)

	// Some successful water RPC responses omit inventory deltas. When the
	// server did include item 7, ApplyV has already installed the authoritative
	// remaining drop count, so do not spend it locally again.
	if isWaterOp(op.Kind) && !waterResponseIncludesDrops(result.raw) {
		r.state.MarkLandsWatered(op.LandIDs)
	}
}

func (r *Runner) logOperation(ctx context.Context, kind string, args, result any) {
	if r.db == nil || r.account == nil {
		return
	}
	_ = r.db.LogOperation(ctx, r.account.ID, kind, args, result)
}

func operationPayload(op *automation.PlannedOp, args any, raw json.RawMessage, err error) string {
	payload := map[string]any{
		"rpc":       op.Kind,
		"lane":      op.Lane,
		"category":  op.Category,
		"domain":    op.Domain,
		"action":    op.Action,
		"priority":  op.Priority,
		"reason":    op.Reason,
		"label":     opDesc(op),
		"landIds":   op.LandIDs,
		"slotIds":   op.SlotIDs,
		"args":      args,
		"flowerId":  op.FlowerID,
		"targetUid": op.TargetUID,
		"targetId":  op.TargetID,
		"itemId":    op.ItemID,
		"count":     op.Count,
		"vaseId":    op.VaseID,
		"flowerIds": op.FlowerIDs,
	}
	if len(raw) > 0 {
		payload["raw"] = json.RawMessage(raw)
	}
	if !op.CooldownUntil.IsZero() {
		payload["cooldownUntilMs"] = op.CooldownUntil.UnixMilli()
		payload["cooldownReason"] = op.CooldownReason
	}
	if err != nil {
		payload["error"] = err.Error()
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

// emitCustomerOrderInfo emits an order event whenever observed customer order
// requirements change, including flower-art recipe details for log views.
func (r *Runner) emitCustomerOrderInfo() {
	orders := r.state.CustomerOrderDetails()
	if r.lastCustomerOrderInfo == nil {
		r.lastCustomerOrderInfo = make(map[int32]string)
	}
	seen := make(map[int32]bool, len(orders))
	for npcID, order := range orders {
		seen[npcID] = true
		summary := automation.FormatCustomerOrderRequires(r.state, order)
		if summary == r.lastCustomerOrderInfo[npcID] {
			continue
		}
		r.lastCustomerOrderInfo[npcID] = summary
		if summary == "" {
			continue
		}
		r.emit(Event{
			Kind:     "order_customer_info",
			Category: "order",
			Domain:   "order.customer",
			Action:   "info",
			Message:  fmt.Sprintf("顾客订单 NPC=%d %s", npcID, summary),
			Level:    "info",
		})
	}
	for npcID := range r.lastCustomerOrderInfo {
		if !seen[npcID] {
			delete(r.lastCustomerOrderInfo, npcID)
		}
	}
}

func opDesc(op *automation.PlannedOp) string {
	desc := opKindDesc(op.Kind)
	if op.FlowerID == 0 {
		return desc
	}
	return fmt.Sprintf("%s %s(#%d)", desc, flowerName(int(op.FlowerID)), op.FlowerID)
}

func operationTargetSuffix(op *automation.PlannedOp) string {
	if op == nil {
		return ""
	}
	if suffix := landSuffix(op.LandIDs); suffix != "" {
		return suffix
	}
	switch op.Kind {
	case clientproto.RPCStoryMainUnlock.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (剧情小节=%d)", op.TargetID)
		}
	case clientproto.RPCTaskAchRecv.String(), clientproto.RPCTaskDlyRecv.String(), clientproto.RPCTaskWeekRecv.String(), clientproto.RPCRoadGrowRecv.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (任务=%d)", op.TargetID)
		}
	case clientproto.RPCRandomEventDoAffair.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (事件=%d)", op.TargetID)
		}
	case clientproto.RPCZooAddFoodstuff.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (宠物=%d 食物=%d×%d)", op.TargetID, op.ItemID, op.Count)
		}
	case clientproto.RPCZooRefreshPetStatus.String(), clientproto.RPCZooStrokePet.String(), clientproto.RPCZooFeedPets.String(), clientproto.RPCZooFindPet.String(), clientproto.RPCZooReadLog.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (宠物=%d)", op.TargetID)
		}
	case clientproto.RPCZooHandleEvent.String():
		if op.TargetID > 0 && op.ItemID > 0 {
			return fmt.Sprintf(" (宠物=%d 日志=%d)", op.TargetID, op.ItemID)
		}
		if op.TargetID > 0 {
			return fmt.Sprintf(" (宠物=%d)", op.TargetID)
		}
	case clientproto.RPCZooRecvSouvenirRwd.String():
		if len(op.SlotIDs) > 0 {
			return fmt.Sprintf(" (奖励档位=%v)", op.SlotIDs)
		}
	case clientproto.RPCZooReadSouvenir.String():
		if len(op.SlotIDs) > 0 {
			return fmt.Sprintf(" (纪念品=%v)", op.SlotIDs)
		}
	case clientproto.RPCFlowerArtMakeFlowerArt.String():
		if desc := automation.FormatFlowerArtOpDesc(op.ItemID, op.Count); desc != "" {
			return " " + desc
		}
	}
	return ""
}

func landSuffix(landIDs []int32) string {
	if len(landIDs) == 0 {
		return ""
	}
	return fmt.Sprintf(" (田地=%v)", landIDs)
}

func (r *Runner) orderCustomerSuffix(op *automation.PlannedOp) string {
	switch op.Kind {
	case clientproto.RPCOrderCustomerFinishOrder.String(), clientproto.RPCOrderCustomerRejectOrder.String():
	default:
		return ""
	}
	if op.TargetID == 0 {
		return ""
	}
	orders := r.state.CustomerOrderDetails()
	order, ok := orders[op.TargetID]
	if !ok || order == nil {
		return fmt.Sprintf(" (NPC=%d)", op.TargetID)
	}
	summary := automation.FormatCustomerOrderRequires(r.state, order)
	if summary == "" {
		return fmt.Sprintf(" (NPC=%d)", op.TargetID)
	}
	return fmt.Sprintf(" (NPC=%d %s)", op.TargetID, summary)
}

func (r *Runner) opSuffix(op *automation.PlannedOp) string {
	if suffix := operationTargetSuffix(op); suffix != "" {
		return suffix
	}
	return r.orderCustomerSuffix(op)
}
