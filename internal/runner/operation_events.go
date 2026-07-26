package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

type operationAttempt struct {
	op         *automation.PlannedOp
	args       any
	startedAt  time.Time
	goldBefore int32
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
	operationErrorRaceTakeAlreadyTaken    operationErrorKind = "race_take_already_taken"
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
	case isRaceTakeAlreadyTakenError(kind, err):
		return operationErrorRaceTakeAlreadyTaken
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
			Label:       operationEventLabel(op),
			Message:     fmt.Sprintf("%s 暂缓: 服务端提示订单冷却中，稍后重试", opDesc(op)),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "cooldown"})
		return nil
	case operationErrorResidentOrderDailyLimit:
		now := result.finishedAt
		isSpecial := op.Kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() ||
			op.Kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String()
		if !isSpecial {
			r.state.MarkResidentOrderDailyLimitReached(now)
		}
		until, ok := r.state.ResidentOrderDailyLimitReached(now)
		cooldown := state.NextGameDayReset(now).Sub(now)
		if !isSpecial && ok {
			cooldown = until.Sub(now)
		}
		payloadOp := r.cooldownSideOperation(op, now, err, "服务端提示今日完成订单次数已达上限", cooldown)
		label := operationEventLabel(op)
		if label == "" {
			label = "普通居民订单"
		}
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       label,
			Message:     fmt.Sprintf("%s 暂停: %s已达服务端今日上限，已跳过以继续执行其他流程", opDesc(op), label),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "daily_limit"})
		if !isSpecial {
			r.lastResidentOrderLimitReason = "server_daily_limit"
		}
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
	case operationErrorRaceTakeAlreadyTaken:
		r.state.MarkFmlRaceTasksUnobserved()
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 暂缓: 服务端提示已接取其他任务，将重新同步任务池后继续", opDesc(op)),
			PayloadJSON: operationPayload(op, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "race_taken_resync"})
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
	kind := "operation_ack"
	label := operationEventLabel(op)
	message := fmt.Sprintf("%s 完成%s", opDesc(op), r.opSuffix(op))
	category := op.Category
	switch op.Kind {
	case clientproto.RPCOrderFlowerFinishOrder.String():
		kind = "order_finish"
		label = "普通居民订单"
		category = automation.CategoryOrder
		message = residentOrderFinishSuccessMessage("普通居民订单", op)
	case clientproto.RPCOrderFlowerFinishSatinOrder.String():
		kind = "order_satin_finish"
		label = "绸缎订单"
		category = automation.CategoryOrder
		message = residentOrderFinishSuccessMessage("绸缎订单", op)
	case clientproto.RPCOrderFlowerFinishDecorateOrder.String():
		kind = "order_decorate_finish"
		label = "建材订单"
		category = automation.CategoryOrder
		message = residentOrderFinishSuccessMessage("建材订单", op)
	case clientproto.RPCOrderFlowerRecvOrderRwd.String():
		kind = "order_reward"
		label = "居民订单领奖"
		category = automation.CategoryOrder
		message = fmt.Sprintf("领取居民订单阶段奖励 target=%d", op.TargetID)
	case clientproto.RPCFmlFlowerShareTake.String():
		kind = "union_flower_take"
		label = "公会摸花"
		message = fmt.Sprintf("公会摸花成功%s", unionFlowerTakeMessageSuffix(op))
	case clientproto.RPCFlowerRackSell.String():
		kind = "flower_rack_sell"
		label = "花艺上架"
		category = automation.CategoryOrder
		message = flowerRackSellSuccessMessage(op)
	case clientproto.RPCFlowerRackRecvSellMoney.String():
		kind = "flower_rack_claim"
		label = "花艺售出"
		category = automation.CategoryOrder
		message = flowerRackClaimSuccessMessage(op, result.goldBefore, r.state.Gold())
	}
	r.emit(Event{
		Kind:        kind,
		Category:    category,
		Domain:      op.Domain,
		Action:      op.Action,
		Label:       label,
		Message:     message,
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
	switch op.Kind {
	case clientproto.RPCOrderFlowerFinishOrder.String():
		r.state.NoteResidentOrderFinished(result.finishedAt, result.raw)
		r.emitResidentOrderLimitInfo(r.Policy(), result.finishedAt)
	case clientproto.RPCOrderFlowerFinishSatinOrder.String():
		r.state.NoteResidentSatinOrderFinished(result.finishedAt, result.raw)
	case clientproto.RPCOrderFlowerFinishDecorateOrder.String():
		r.state.NoteResidentDecorateOrderFinished(result.finishedAt, result.raw)
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
		"rpc":            op.Kind,
		"lane":           op.Lane,
		"category":       op.Category,
		"domain":         op.Domain,
		"action":         op.Action,
		"priority":       op.Priority,
		"reason":         op.Reason,
		"label":          opDesc(op),
		"landIds":        op.LandIDs,
		"slotIds":        op.SlotIDs,
		"args":           args,
		"flowerId":       op.FlowerID,
		"targetUid":      op.TargetUID,
		"targetUids":     op.TargetUIDs,
		"batchId":        op.BatchID,
		"slotId":         op.SlotID,
		"taskId":         op.TaskID,
		"milestoneIndex": op.MilestoneIndex,
		"targetId":       op.TargetID,
		"itemId":         op.ItemID,
		"count":          op.Count,
		"vaseId":         op.VaseID,
		"flowerIds":      op.FlowerIDs,
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

// emitResidentOrderLimitInfo writes a clear log line when ordinary resident
// orders are paused by the policy/server daily limit. Deduped by reason so
// decision ticks do not spam the event stream.
func (r *Runner) emitResidentOrderLimitInfo(policy *pb.Policy, now time.Time) {
	if policy == nil || r.state == nil {
		return
	}
	resident := policy.GetOrder().GetResident()
	if !resident.GetNormalEnabled() {
		r.lastResidentOrderLimitReason = ""
		return
	}
	reason, reached := automation.ResidentNormalDailyLimitReached(r.state, resident, now)
	if !reached {
		r.lastResidentOrderLimitReason = ""
		return
	}
	if reason == r.lastResidentOrderLimitReason {
		return
	}
	r.lastResidentOrderLimitReason = reason
	r.emit(Event{
		Kind:     "operation_deferred",
		Category: "order",
		Domain:   "order.resident",
		Action:   "blocked",
		Label:    "普通居民订单",
		Message:  fmt.Sprintf("普通居民订单暂停: %s，已跳过提交以继续执行其他流程", reason),
		Level:    "warn",
	})
}

func operationEventLabel(op *automation.PlannedOp) string {
	if op == nil {
		return ""
	}
	switch {
	case op.Kind == clientproto.RPCOrderFlowerFinishOrder.String():
		return "普通居民订单"
	case op.Kind == clientproto.RPCOrderFlowerFinishSatinOrder.String():
		return "绸缎订单"
	case op.Kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String():
		return "建材订单"
	case op.Kind == clientproto.RPCOrderFlowerRecvOrderRwd.String():
		return "居民订单领奖"
	case op.Kind == clientproto.RPCFmlFlowerShareTake.String() || op.Domain == "union.flower.take":
		return "公会摸花"
	case op.Kind == clientproto.RPCFlowerRackSell.String():
		return "花艺上架"
	case op.Kind == clientproto.RPCFlowerRackRecvSellMoney.String():
		return "花艺售出"
	}
	return ""
}

func residentOrderFinishSuccessMessage(label string, op *automation.PlannedOp) string {
	if label == "" {
		label = "居民订单"
	}
	parts := make([]string, 0, 2)
	if op != nil && op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() && op.TargetID > 0 {
		parts = append(parts, fmt.Sprintf("格子=%d", op.TargetID))
	}
	if summary := orderRequireSummaryFromReason(op); summary != "" {
		parts = append(parts, summary)
	}
	if len(parts) == 0 {
		return "完成" + label
	}
	return "完成" + label + ": " + strings.Join(parts, " ")
}

func orderRequireSummaryFromReason(op *automation.PlannedOp) string {
	if op == nil {
		return ""
	}
	reason := strings.TrimSpace(op.Reason)
	start := strings.LastIndex(reason, "(")
	end := strings.LastIndex(reason, ")")
	if start < 0 || end <= start+1 {
		return ""
	}
	return strings.TrimSpace(reason[start+1 : end])
}

func unionFlowerTakeMessageSuffix(op *automation.PlannedOp) string {
	if op == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if op.FlowerID > 0 {
		parts = append(parts, fmt.Sprintf("%s(#%d)", flowerName(int(op.FlowerID)), op.FlowerID))
	}
	if op.TargetUID > 0 {
		parts = append(parts, fmt.Sprintf("成员=%d", op.TargetUID))
	}
	if op.TargetID > 0 {
		parts = append(parts, fmt.Sprintf("槽位=%d", op.TargetID))
	}
	if len(parts) == 0 {
		return ""
	}
	return ": " + strings.Join(parts, " ")
}

func flowerRackSellSuccessMessage(op *automation.PlannedOp) string {
	if op == nil {
		return "花艺上架成功"
	}
	parts := make([]string, 0, 2)
	if desc := automation.FormatFlowerArtOpDesc(op.ItemID, op.Count); desc != "" {
		parts = append(parts, desc)
	} else if op.ItemID > 0 {
		parts = append(parts, fmt.Sprintf("花艺#%d×%d", op.ItemID, op.Count))
	}
	if op.TargetID > 0 {
		parts = append(parts, fmt.Sprintf("花架=%d", op.TargetID))
	}
	if len(parts) == 0 {
		return "花艺上架成功"
	}
	return "花艺上架成功: " + strings.Join(parts, " ")
}

func flowerRackClaimSuccessMessage(op *automation.PlannedOp, goldBefore, goldAfter int32) string {
	if op == nil {
		return "花艺售出领取成功"
	}
	parts := make([]string, 0, 3)
	if op.ItemID > 0 {
		if desc := automation.FormatFlowerArtOpDesc(op.ItemID, op.Count); desc != "" {
			parts = append(parts, desc)
		} else {
			parts = append(parts, fmt.Sprintf("花艺#%d×%d", op.ItemID, op.Count))
		}
	}
	if goldGain := goldAfter - goldBefore; goldGain > 0 {
		parts = append(parts, fmt.Sprintf("金币+%d", goldGain))
	} else if expected := flowerRackExpectedGold(op.ItemID, op.Count); expected > 0 {
		parts = append(parts, fmt.Sprintf("金币+%d", expected))
	}
	if op.TargetID > 0 {
		parts = append(parts, fmt.Sprintf("花架=%d", op.TargetID))
	}
	if len(parts) == 0 {
		return "花艺售出领取成功"
	}
	return "花艺售出领取成功: " + strings.Join(parts, " ")
}

func flowerRackExpectedGold(artID, count int32) int32 {
	if artID <= 0 || count <= 0 {
		return 0
	}
	recipe, ok := state.FlowerArtRecipeByID(artID)
	if !ok || recipe.SaleValue <= 0 {
		return 0
	}
	return recipe.SaleValue * count
}

func opDesc(op *automation.PlannedOp) string {
	desc := opKindDesc(op.Kind)
	if op.FlowerID == 0 || isRaceOpKind(op.Kind) {
		return desc
	}
	return fmt.Sprintf("%s %s(#%d)", desc, flowerName(int(op.FlowerID)), op.FlowerID)
}

func isRaceOpKind(kind string) bool {
	switch kind {
	case clientproto.RPCFmlRaceTakeTask.String(),
		clientproto.RPCFmlRaceFinishTask.String(),
		clientproto.RPCFmlRaceUpgradeTask.String(),
		clientproto.RPCFmlRaceDelTask.String(),
		clientproto.RPCFmlRaceGiveUpTask.String():
		return true
	default:
		return false
	}
}

func operationTargetSuffix(op *automation.PlannedOp) string {
	if op == nil {
		return ""
	}
	switch op.Kind {
	case clientproto.RPCFmlLandHarvest.String(),
		clientproto.RPCFmlLandHarvestAll.String():
		if op.Reason != "" {
			return " " + op.Reason
		}
	case clientproto.RPCFmlFlowerShareTake.String():
		parts := make([]string, 0, 2)
		if op.TargetUID > 0 {
			parts = append(parts, fmt.Sprintf("成员=%d", op.TargetUID))
		}
		if op.TargetID > 0 {
			parts = append(parts, fmt.Sprintf("槽位=%d", op.TargetID))
		}
		if len(parts) > 0 {
			return " (" + strings.Join(parts, " ") + ")"
		}
	}
	if suffix := landSuffix(op.LandIDs); suffix != "" {
		return suffix
	}
	switch op.Kind {
	case clientproto.RPCFmlRaceTakeTask.String(),
		clientproto.RPCFmlRaceFinishTask.String(),
		clientproto.RPCFmlRaceUpgradeTask.String(),
		clientproto.RPCFmlRaceDelTask.String(),
		clientproto.RPCFmlRaceGiveUpTask.String():
		if desc := automation.FormatRaceTaskOpDesc(op.TaskID, op.FlowerID); desc != "" {
			return " " + desc
		}
	case clientproto.RPCActCyclicNoteEnter.String():
		if op.BatchID > 0 {
			return fmt.Sprintf(" (活动批次=%d)", op.BatchID)
		}
	case clientproto.RPCActCyclicNoteRecvTaskRwd.String():
		if op.BatchID > 0 && op.SlotID > 0 && op.TaskID > 0 {
			return fmt.Sprintf(" (活动批次=%d 槽位=%d 任务=%d)", op.BatchID, op.SlotID, op.TaskID)
		}
	case clientproto.RPCActCyclicNoteRecv.String():
		if op.BatchID > 0 && op.MilestoneIndex > 0 {
			return fmt.Sprintf(" (活动批次=%d 里程碑=%d)", op.BatchID, op.MilestoneIndex)
		}
	case clientproto.RPCStoryMainUnlock.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (剧情小节=%d)", op.TargetID)
		}
	case clientproto.RPCTaskMainRecv.String(), clientproto.RPCTaskAchRecv.String(), clientproto.RPCTaskDlyRecv.String(), clientproto.RPCTaskWeekRecv.String(), clientproto.RPCRoadGrowRecv.String():
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
	case clientproto.RPCFlowerRackSell.String():
		parts := make([]string, 0, 2)
		if desc := automation.FormatFlowerArtOpDesc(op.ItemID, op.Count); desc != "" {
			parts = append(parts, desc)
		} else if op.ItemID > 0 {
			parts = append(parts, fmt.Sprintf("花艺#%d×%d", op.ItemID, op.Count))
		}
		if op.TargetID > 0 {
			parts = append(parts, fmt.Sprintf("花架=%d", op.TargetID))
		}
		if len(parts) > 0 {
			return " " + strings.Join(parts, " ")
		}
	case clientproto.RPCFlowerRackRecvSellMoney.String():
		parts := make([]string, 0, 2)
		if op.ItemID > 0 {
			if desc := automation.FormatFlowerArtOpDesc(op.ItemID, op.Count); desc != "" {
				parts = append(parts, desc)
			} else {
				parts = append(parts, fmt.Sprintf("花艺#%d×%d", op.ItemID, op.Count))
			}
		}
		if op.TargetID > 0 {
			parts = append(parts, fmt.Sprintf("花架=%d", op.TargetID))
		}
		if len(parts) > 0 {
			return " " + strings.Join(parts, " ")
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
