package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

type operationAttempt struct {
	op                         *automation.PlannedOp
	args                       any
	startedAt                  time.Time
	goldBefore                 int32
	levelBefore                int32
	waterDropsBefore           int32
	scoreBefore                int32
	scoreBeforeSet             bool
	friendStealUsedBefore      int32
	friendStealUsedBeforeSet   bool
	friendStealBoughtBefore    int32
	friendStealBoughtBeforeSet bool
}

type operationResult struct {
	operationAttempt
	raw        json.RawMessage
	err        error
	finishedAt time.Time
}

type operationErrorKind string

const customerOrderGenerationNoopCooldown = 10 * time.Minute

const (
	operationErrorOrdinary                   operationErrorKind = "ordinary"
	operationErrorHarvestNotMature           operationErrorKind = "harvest_not_mature"
	operationErrorResidentOrderCooldown      operationErrorKind = "resident_order_cooldown"
	operationErrorResidentOrderDailyLimit    operationErrorKind = "resident_order_daily_limit"
	operationErrorWaterwheelInvalidData      operationErrorKind = "waterwheel_invalid_data"
	operationErrorWaterwheelDailyLimit       operationErrorKind = "waterwheel_daily_limit"
	operationErrorShopCultivateExhausted     operationErrorKind = "shop_cultivate_exhausted"
	operationErrorWaterDropRejected          operationErrorKind = "water_drop_rejected"
	operationErrorCultivateUpgradeRejected   operationErrorKind = "cultivate_upgrade_resource_rejected"
	operationErrorFlowerArtMaterialRejected  operationErrorKind = "flower_art_material_rejected"
	operationErrorTaskGroupFinished          operationErrorKind = "task_group_finished"
	operationErrorRaceTakeAlreadyTaken       operationErrorKind = "race_take_already_taken"
	operationErrorRaceTakeClaimedByOther     operationErrorKind = "race_take_claimed_by_other"
	operationErrorRaceTakeQuotaExceeded      operationErrorKind = "race_take_quota_exceeded"
	operationErrorRaceTakeOnCooldown         operationErrorKind = "race_take_on_cooldown"
	operationErrorRaceDeleteOnCooldown       operationErrorKind = "race_delete_on_cooldown"
	operationErrorFmlBuildDailyLimit         operationErrorKind = "fml_build_daily_limit"
	operationErrorFmlNotJoined               operationErrorKind = "fml_not_joined"
	operationErrorFmlFlowerTakeDailyLimit    operationErrorKind = "fml_flower_take_daily_limit"
	operationErrorCyclicStoryOrderNotReady   operationErrorKind = "cyclic_story_order_not_ready"
	operationErrorMailAlreadyPicked          operationErrorKind = "mail_already_picked"
	operationErrorPearlHireCandidateFallback operationErrorKind = "pearl_hire_candidate_fallback"
)

func classifyOperationError(kind string, err error) operationErrorKind {
	switch {
	case isPearlHireCandidateFallbackError(kind, err):
		return operationErrorPearlHireCandidateFallback
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
	case isShopCultivateOfferExhaustedError(kind, err):
		return operationErrorShopCultivateExhausted
	case isWaterDropResourceRejectedError(kind, err):
		return operationErrorWaterDropRejected
	case isCultivateUpgradeResourceRejectedError(kind, err):
		return operationErrorCultivateUpgradeRejected
	case isFlowerArtMaterialRejectedError(kind, err):
		return operationErrorFlowerArtMaterialRejected
	case isTaskGroupFinishedError(kind, err):
		return operationErrorTaskGroupFinished
	case isRaceTakeAlreadyTakenError(kind, err):
		return operationErrorRaceTakeAlreadyTaken
	case isRaceTakeClaimedByOtherError(kind, err):
		return operationErrorRaceTakeClaimedByOther
	case isRaceTakeQuotaExceededError(kind, err):
		return operationErrorRaceTakeQuotaExceeded
	case isRaceTakeOnCooldownError(kind, err):
		return operationErrorRaceTakeOnCooldown
	case isRaceDeleteOnCooldownError(kind, err):
		return operationErrorRaceDeleteOnCooldown
	case isFmlBuildDailyLimitError(kind, err):
		return operationErrorFmlBuildDailyLimit
	case isFmlNotJoinedError(kind, err):
		return operationErrorFmlNotJoined
	case isFmlFlowerTakeDailyLimitError(kind, err):
		return operationErrorFmlFlowerTakeDailyLimit
	case isCyclicStoryOrderNotReadyError(kind, err):
		return operationErrorCyclicStoryOrderNotReady
	case isMailAlreadyPickedError(kind, err):
		return operationErrorMailAlreadyPicked
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
	message := fmt.Sprintf("计划执行 %s%s", opDesc(attempt.op), r.opSuffix(attempt.op))
	if attempt.op.Kind == clientproto.RPCFmlRaceGetTaskList.String() && strings.TrimSpace(attempt.op.Reason) != "" {
		message += ": " + strings.TrimSpace(attempt.op.Reason)
	}
	r.emit(Event{
		Kind:        "operation_planned",
		Category:    attempt.op.Category,
		Domain:      attempt.op.Domain,
		Action:      attempt.op.Action,
		Label:       operationEventLabel(attempt.op),
		Message:     message,
		PayloadJSON: operationPayload(attempt.op, attempt.args, nil, nil),
	})
}

func (r *Runner) handleOperationError(ctx context.Context, result operationResult) error {
	op, args, err := result.op, result.args, result.err
	switch classifyOperationError(op.Kind, err) {
	case operationErrorPearlHireCandidateFallback:
		var fallbackErr *pearlHireCandidateFallbackError
		_ = errors.As(err, &fallbackErr)
		ticketResult := "未观察到雇佣券扣除"
		ticketSpent := false
		if fallbackErr != nil && fallbackErr.TicketSpent {
			ticketResult = "本次已消耗 1 张雇佣券并计入今日用量"
			ticketSpent = true
		}
		r.clearOperationCooldown(op)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       operationEventLabel(op),
			Message:     fmt.Sprintf("%s 已跳过: 服务端提示该候选需改用金币雇佣，%s；已继续检查其他候选，未自动使用金币", opDesc(op), ticketResult),
			PayloadJSON: operationPayload(op, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{
			"error":       err.Error(),
			"stage":       "candidate_gold_fallback",
			"ticketSpent": ticketSpent,
			"placeId":     op.TargetID,
			"targetUid":   op.TargetUID,
		})
		return nil
	case operationErrorFmlNotJoined:
		r.state.MarkNoFmlMembership()
		r.clearOperationCooldown(op)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       operationEventLabel(op),
			Message:     fmt.Sprintf("%s 已跳过: 账号未加入公会，已停止公会相关自动化", opDesc(op)),
			PayloadJSON: operationPayload(op, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "fml_not_joined"})
		return nil
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
		// Ordinary resident orders use $orderCd≈42s; satin/decorate use
		// $orderCd2/$orderCd3=60s and commonly surface as ~61s after ceil.
		cooldown := 30 * time.Second
		if op.Kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() ||
			op.Kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String() {
			cooldown = 61 * time.Second
		}
		payloadOp := r.cooldownSideOperation(op, result.finishedAt, err, "服务端提示订单冷却中", cooldown)
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
		var until time.Time
		var ok bool
		isSpecial := false
		switch op.Kind {
		case clientproto.RPCOrderFlowerFinishSatinOrder.String():
			isSpecial = true
			r.state.MarkResidentSatinDailyLimitReached(now)
			until, ok = r.state.ResidentSatinDailyLimitReached(now)
			// Close short retry timers; planner consults the state marker until 00:00.
			r.clearResidentSpecialOrderRetryTimers(op.Kind)
		case clientproto.RPCOrderFlowerFinishDecorateOrder.String():
			isSpecial = true
			r.state.MarkResidentDecorateDailyLimitReached(now)
			until, ok = r.state.ResidentDecorateDailyLimitReached(now)
			r.clearResidentSpecialOrderRetryTimers(op.Kind)
		default:
			r.state.MarkResidentOrderDailyLimitReached(now)
			until, ok = r.state.ResidentOrderDailyLimitReached(now)
		}
		label := operationEventLabel(op)
		if label == "" {
			label = "普通居民订单"
		}
		payloadOp := op
		message := fmt.Sprintf("%s 暂停: %s已达服务端今日上限，已跳过以继续执行其他流程", opDesc(op), label)
		if isSpecial {
			resetAt := until
			if !ok {
				resetAt = state.NextCalendarDayReset(now)
			}
			message = fmt.Sprintf("%s 暂停: %s已达服务端今日上限，已关闭重试，等待次日0点（%s）后再继续",
				opDesc(op), label, resetAt.Format("01/02 15:04"))
		} else {
			cooldown := state.NextGameDayReset(now).Sub(now)
			if ok {
				cooldown = until.Sub(now)
			}
			if cooldown <= 0 {
				cooldown = time.Minute
			}
			payloadOp = r.cooldownSideOperation(op, now, err, "服务端提示今日完成订单次数已达上限", cooldown)
		}
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       label,
			Message:     message,
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "daily_limit"})
		if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() {
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
	case operationErrorShopCultivateExhausted:
		r.state.MarkShopCultivateOfferExhausted(op.TargetID)
		r.clearOperationCooldown(op)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       operationEventLabel(op),
			Message:     fmt.Sprintf("%s 已跳过: 服务端提示当前商品无法继续购买，已校正本地购买记录并继续检查其他商品", opDesc(op)),
			PayloadJSON: operationPayload(op, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "offer_exhausted", "shopId": op.TargetID})
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
	case operationErrorCultivateUpgradeRejected:
		itemID := resourceRejectedItemID(err)
		r.markCultivateUpgradeResourceRejected(op)
		r.clearOperationCooldown(op)
		resource := state.ItemLabel(itemID)
		if itemID == 11 {
			resource = "金币"
		}
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       operationEventLabel(op),
			Message:     fmt.Sprintf("%s 已阻塞: 服务端提示%s不足；等待金币、精华或培育等级更新后重新判断", opDesc(op), resource),
			PayloadJSON: operationPayload(op, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "cultivate_upgrade_resource_rejected", "itemId": itemID})
		return nil
	case operationErrorFlowerArtMaterialRejected:
		itemID := inventoryMaterialRejectedItemID(op.Kind, err)
		if itemID > 0 {
			r.state.MarkInventoryItemExhausted(itemID)
		}
		itemName := state.ItemLabel(itemID)
		retryHint := "等待补充后重试"
		if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() {
			retryHint = "下轮将按库存→制作→拒绝重新决策"
		}
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 暂缓: 服务端提示材料不足（%s），已校正本地库存，%s", opDesc(op), itemName, retryHint),
			PayloadJSON: operationPayload(op, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "resource_stale", "missingItemId": itemID})
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
		r.state.MarkFmlRaceTaskPoolStale()
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
	case operationErrorRaceTakeClaimedByOther:
		if op.TaskMsID != 0 {
			r.state.MarkFmlRacePoolTaskClaimed(op.TaskMsID)
		}
		r.state.MarkFmlRaceTaskPoolStale()
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 暂缓: 服务端提示任务已被其他成员接取，已跳过该任务并重新同步任务池", opDesc(op)),
			PayloadJSON: operationPayload(op, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "race_claimed_by_other", "taskMsId": op.TaskMsID})
		return nil
	case operationErrorRaceTakeQuotaExceeded:
		r.state.MarkFmlRaceTakeQuotaExhausted()
		r.state.MarkFmlRaceTaskPoolStale()
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 暂停: 服务端提示任务接取次数已达上限，本轮竞赛不再自动接取", opDesc(op)),
			PayloadJSON: operationPayload(op, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "race_take_quota"})
		return nil
	case operationErrorRaceTakeOnCooldown:
		// Preemptive take (lead window) often hits server CD. Do not use the
		// ordinary 60s side-op backoff — wait until AppearTime (+pad) and
		// keep the current pool so the retry tick can take instead of syncing.
		now := result.finishedAt
		cooldown := raceTakeOnCooldownWait(r.state, op, now)
		payloadOp := r.cooldownSideOperation(op, now, err, "服务端提示任务冷却中", cooldown)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       operationEventLabel(op),
			Message:     fmt.Sprintf("%s 暂缓: 服务端提示任务冷却中，等待刷新后重试", opDesc(op)),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{
			"error":             err.Error(),
			"stage":             "race_take_cooldown",
			"taskMsId":          op.TaskMsID,
			"retryAfterSeconds": int(cooldown.Seconds()),
		})
		return nil
	case operationErrorRaceDeleteOnCooldown:
		now := result.finishedAt
		cooldown := raceTakeOnCooldownWait(r.state, op, now)
		payloadOp := r.cooldownSideOperation(op, now, err, "竞赛任务仍在刷新冷却中", cooldown)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       operationEventLabel(op),
			Message:     fmt.Sprintf("%s 暂缓: 任务仍在刷新冷却中，等待可操作后重试", opDesc(op)),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{
			"error":             err.Error(),
			"stage":             "race_delete_cooldown",
			"taskMsId":          op.TaskMsID,
			"retryAfterSeconds": int(cooldown.Seconds()),
		})
		return nil
	case operationErrorFmlBuildDailyLimit:
		now := result.finishedAt
		op.CooldownKey = "union.build"
		cooldown := state.NextCalendarDayReset(now).Sub(now)
		if cooldown <= 0 {
			cooldown = time.Minute
		}
		payloadOp := r.cooldownSideOperation(op, now, err, "服务端提示每日建设次数已达上限", cooldown)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       operationEventLabel(op),
			Message:     fmt.Sprintf("%s 暂停: 今日公会建设次数已达上限，等待次日重置", opDesc(op)),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "daily_limit"})
		return nil
	case operationErrorFmlFlowerTakeDailyLimit:
		now := result.finishedAt
		r.state.MarkFmlFlowerTakeDailyLimitReached(now)
		// Share one cooldown across all take targets (slot/uid variants).
		if strings.TrimSpace(op.CooldownKey) == "" {
			op.CooldownKey = "union.flower.take"
		}
		cooldown := state.NextCalendarDayReset(now).Sub(now)
		if cooldown <= 0 {
			cooldown = time.Minute
		}
		payloadOp := r.cooldownSideOperation(op, now, err, "服务端提示今日拿取次数已达上限", cooldown)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       operationEventLabel(op),
			Message:     fmt.Sprintf("%s 暂停: 服务端提示今日拿取次数已达上限，已跳过摸花以继续执行其他流程", opDesc(op)),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "daily_limit"})
		return nil
	case operationErrorCyclicStoryOrderNotReady:
		now := result.finishedAt
		cooldown := sideOperationBaseCooldown
		if until := cyclicStoryOrderCooldownUntil(r.state, op, now); until.After(now) {
			cooldown = until.Sub(now)
		}
		tip := state.MsgCodeText(259)
		if tip == "" {
			tip = "未达成领取奖励的条件"
		}
		payloadOp := r.cooldownSideOperation(op, now, err, tip, cooldown)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       operationEventLabel(op),
			Message:     fmt.Sprintf("%s 暂缓: 服务端提示%s，等待订单冷却后再试", opDesc(op), tip),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "order_not_ready"})
		return nil
	case operationErrorMailAlreadyPicked:
		r.state.MarkMailPicked(op.TargetID, op.ItemID)
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       operationEventLabel(op),
			Message:     fmt.Sprintf("%s 已跳过: 服务端提示邮件附件已领取，已校正本地邮件状态", opDesc(op)),
			PayloadJSON: operationPayload(op, args, nil, err),
			Level:       "warn",
		})
		r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "mail_already_picked", "msId": op.TargetID, "allId": op.ItemID})
		return nil
	default:
		if isHarvestOp(op.Kind) {
			wait := r.deferFailedHarvest(op, err, result.finishedAt)
			r.emit(Event{Kind: "operation_deferred", Category: op.Category, Domain: op.Domain,
				Action: "blocked", Label: operationEventLabel(op), Level: "warn",
				Message:     fmt.Sprintf("%s 暂缓: %v；失败田地将在 %d 秒后重试，其他操作继续", opDesc(op), err, int(wait.Seconds())),
				PayloadJSON: operationPayload(op, args, result.raw, err)})
			r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "retryAfterSeconds": int(wait.Seconds())})
			return nil
		}
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() ||
			op.Kind == clientproto.RPCFmlRaceEnter.String() {
			return r.handleRaceSyncFailure(ctx, result, "race_sync_retry")
		}
		// Contested takeTask must not sit behind the ordinary 60s side-op
		// backoff. Code 221 means the race session is stale — enter before sync.
		// Other rejects only mark the pool unobserved for an immediate getTaskList.
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			if isRaceTransientSessionError(op.Kind, err) {
				return r.handleRaceSessionStale(ctx, result, "race_take_session_stale")
			}
			r.state.MarkFmlRaceTaskPoolStale()
			r.clearOperationCooldown(op)
			r.emit(Event{
				Kind:        "operation_deferred",
				Category:    op.Category,
				Domain:      op.Domain,
				Action:      "blocked",
				Label:       operationEventLabel(op),
				Message:     fmt.Sprintf("%s 暂缓: 接取失败，将立即重新同步任务池后重试", opDesc(op)),
				PayloadJSON: operationPayload(op, args, nil, err),
				Level:       "warn",
			})
			r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": "race_take_retry"})
			return nil
		}
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

func (r *Runner) handleRaceSyncFailure(ctx context.Context, result operationResult, stage string) error {
	op, args, err := result.op, result.args, result.err
	if isRaceTransientSessionError(op.Kind, err) {
		r.state.MarkFmlRaceSessionStale()
	} else {
		r.state.MarkFmlRaceTaskPoolStale()
	}
	reason := "竞赛同步失败，稍后重试"
	message := fmt.Sprintf("%s 暂缓: 同步失败，1 秒后重试", opDesc(op))
	if isRaceTransientSessionError(op.Kind, err) {
		reason = "竞赛会话需重新进入，稍后重试"
		message = fmt.Sprintf("%s 暂缓: 竞赛会话需重新进入，1 秒后重试", opDesc(op))
	}
	payloadOp := r.cooldownSideOperation(op, result.finishedAt, err, reason, raceSyncRetryCooldown)
	r.emit(Event{
		Kind:        "operation_deferred",
		Category:    op.Category,
		Domain:      op.Domain,
		Action:      "blocked",
		Label:       operationEventLabel(op),
		Message:     message,
		PayloadJSON: operationPayload(payloadOp, args, nil, err),
		Level:       "warn",
	})
	r.logOperation(ctx, op.Kind, args, map[string]any{
		"error":             err.Error(),
		"stage":             stage,
		"retryAfterSeconds": int(raceSyncRetryCooldown.Seconds()),
	})
	return nil
}

func (r *Runner) handleRaceSessionStale(ctx context.Context, result operationResult, stage string) error {
	op, args, err := result.op, result.args, result.err
	r.state.MarkFmlRaceSessionStale()
	r.clearOperationCooldown(op)
	r.emit(Event{
		Kind:        "operation_deferred",
		Category:    op.Category,
		Domain:      op.Domain,
		Action:      "blocked",
		Label:       operationEventLabel(op),
		Message:     fmt.Sprintf("%s 暂缓: 竞赛会话需重新进入后再接取", opDesc(op)),
		PayloadJSON: operationPayload(op, args, nil, err),
		Level:       "warn",
	})
	r.logOperation(ctx, op.Kind, args, map[string]any{"error": err.Error(), "stage": stage})
	return nil
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
	if op.RaceTaskGuard != nil {
		payload["raceTaskGuard"] = op.RaceTaskGuard
	}
	if op.RaceBatchID != 0 {
		payload["raceBatchId"] = op.RaceBatchID
	}
	if op.TaskMsID != 0 {
		payload["taskMsId"] = op.TaskMsID
	}
	if op.DiamondCost > 0 {
		payload["diamondCost"] = op.DiamondCost
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
