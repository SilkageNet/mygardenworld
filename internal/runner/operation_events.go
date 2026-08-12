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
	op               *automation.PlannedOp
	args             any
	startedAt        time.Time
	goldBefore       int32
	levelBefore      int32
	waterDropsBefore int32
	scoreBefore      int32
	scoreBeforeSet   bool
}

type operationResult struct {
	operationAttempt
	raw        json.RawMessage
	err        error
	finishedAt time.Time
}

type operationErrorKind string

const (
	operationErrorOrdinary                  operationErrorKind = "ordinary"
	operationErrorHarvestNotMature          operationErrorKind = "harvest_not_mature"
	operationErrorResidentOrderCooldown     operationErrorKind = "resident_order_cooldown"
	operationErrorResidentOrderDailyLimit   operationErrorKind = "resident_order_daily_limit"
	operationErrorWaterwheelInvalidData     operationErrorKind = "waterwheel_invalid_data"
	operationErrorWaterwheelDailyLimit      operationErrorKind = "waterwheel_daily_limit"
	operationErrorWaterDropRejected         operationErrorKind = "water_drop_rejected"
	operationErrorFlowerArtMaterialRejected operationErrorKind = "flower_art_material_rejected"
	operationErrorTaskGroupFinished         operationErrorKind = "task_group_finished"
	operationErrorRaceTakeAlreadyTaken      operationErrorKind = "race_take_already_taken"
	operationErrorRaceTakeClaimedByOther    operationErrorKind = "race_take_claimed_by_other"
	operationErrorRaceTakeQuotaExceeded     operationErrorKind = "race_take_quota_exceeded"
	operationErrorRaceTakeOnCooldown         operationErrorKind = "race_take_on_cooldown"
	operationErrorFmlFlowerTakeDailyLimit   operationErrorKind = "fml_flower_take_daily_limit"
	operationErrorCyclicStoryOrderNotReady  operationErrorKind = "cyclic_story_order_not_ready"
	operationErrorMailAlreadyPicked         operationErrorKind = "mail_already_picked"
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
	r.emit(Event{
		Kind:        "operation_planned",
		Category:    attempt.op.Category,
		Domain:      attempt.op.Domain,
		Action:      attempt.op.Action,
		Label:       operationEventLabel(attempt.op),
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
	case operationErrorRaceTakeClaimedByOther:
		if op.TaskMsID != 0 {
			r.state.MarkFmlRacePoolTaskClaimed(op.TaskMsID)
		}
		r.state.MarkFmlRaceTasksUnobserved()
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
		r.state.MarkFmlRaceTasksUnobserved()
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
		// resync the pool so mid-wait upgrades are visible before retry.
		now := result.finishedAt
		r.state.MarkFmlRaceTasksUnobserved()
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
		r.state.NoteFmlFlowerShareTake(op.TargetUID, op.TargetID)
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
	case clientproto.RPCCultivateUpgrade.String():
		kind = "flower_upgrade"
		label = "鲜花升级"
		category = automation.CategoryPlant
		message = flowerUpgradeSuccessMessage(op, result.levelBefore, r.state)
	case clientproto.RPCWaterwheelRecv.String():
		kind = "waterwheel"
		label = "水车水滴"
		category = automation.CategoryWater
		message = waterwheelClaimSuccessMessage(result.waterDropsBefore, r.state)
	case clientproto.RPCFreeWaterRecv.String():
		kind = "free_water"
		label = "限时水滴"
		category = automation.CategoryWater
		message = freeWaterClaimSuccessMessage(op, result.waterDropsBefore, r.state)
	case clientproto.RPCActCyclicStoryRecvOrderRwd.String():
		kind = "activity_cyclic_story_order"
		label = "莳花纪闻"
		category = automation.CategoryActivity
		message = cyclicStoryOrderClaimSuccessMessage(op, result.scoreBefore, result.scoreBeforeSet, r.state, result.finishedAt)
	case clientproto.RPCActCyclicStoryEnter.String():
		kind = "activity_cyclic_story_enter"
		label = "莳花纪闻"
		category = automation.CategoryActivity
		message = fmt.Sprintf("同步莳花纪闻订单%s", r.opSuffix(op))
	case clientproto.RPCActCyclicStoryRecv.String():
		kind = "activity_cyclic_story_progress"
		label = "莳花纪闻"
		category = automation.CategoryActivity
		message = cyclicStoryMilestoneClaimSuccessMessage(op, r.state, result.finishedAt)
	case clientproto.RPCFmlRaceGetTaskList.String():
		kind = "race_task_sync"
		label = "同步竞赛任务"
		category = automation.CategoryRace
		message = "完成"
	case clientproto.RPCFmlRaceEnter.String():
		kind = "race_enter"
		label = "进入公会竞赛"
		category = automation.CategoryRace
		message = "完成"
	case clientproto.RPCFmlRaceTakeTask.String():
		kind = "race_task_taken"
		label = "接取竞赛任务"
		category = automation.CategoryRace
		message = raceTaskSuccessMessage(op)
	case clientproto.RPCFmlRaceFinishTask.String():
		kind = "race_task_finished"
		label = "完成竞赛任务"
		category = automation.CategoryRace
		message = raceTaskSuccessMessage(op)
	case clientproto.RPCFmlRaceUpgradeTask.String():
		kind = "race_task_upgraded"
		label = "升级竞赛任务"
		category = automation.CategoryRace
		message = raceTaskSuccessMessage(op)
	case clientproto.RPCFmlRaceDelTask.String():
		kind = "race_task_deleted"
		label = "删除竞赛任务"
		category = automation.CategoryRace
		message = raceTaskSuccessMessage(op)
	case clientproto.RPCFmlRaceGiveUpTask.String():
		kind = "race_task_given_up"
		label = "放弃竞赛任务"
		category = automation.CategoryRace
		message = raceTaskSuccessMessage(op)
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
	if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() &&
		automation.RaceHoldsUnfinishedCustomerOrder(r.state.FmlRace()) {
		// Customer-order race FinishCnt advances via getTaskList, not harvest
		// field 134. Force a pool refresh on the next tick.
		r.state.MarkFmlRaceTasksUnobserved()
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
	case op.Kind == clientproto.RPCWaterwheelRecv.String() || op.Domain == "basic.waterwheel":
		return "水车水滴"
	case op.Kind == clientproto.RPCFreeWaterRecv.String() || op.Domain == "basic.free_water":
		return "限时水滴"
	case op.Kind == clientproto.RPCActCyclicStoryEnter.String(),
		op.Kind == clientproto.RPCActCyclicStoryRecvOrderRwd.String(),
		op.Kind == clientproto.RPCActCyclicStoryRecv.String(),
		op.Domain == "activity.actCyclicStory":
		return "莳花纪闻"
	case op.Kind == clientproto.RPCFmlRaceGetTaskList.String():
		return "同步竞赛任务"
	case op.Kind == clientproto.RPCFmlRaceEnter.String():
		return "进入公会竞赛"
	case op.Kind == clientproto.RPCFmlRaceTakeTask.String():
		return "接取竞赛任务"
	case op.Kind == clientproto.RPCFmlRaceFinishTask.String():
		return "完成竞赛任务"
	case op.Kind == clientproto.RPCFmlRaceUpgradeTask.String():
		return "升级竞赛任务"
	case op.Kind == clientproto.RPCFmlRaceDelTask.String():
		return "删除竞赛任务"
	case op.Kind == clientproto.RPCFmlRaceGiveUpTask.String():
		return "放弃竞赛任务"
	}
	return ""
}

func raceTaskSuccessMessage(op *automation.PlannedOp) string {
	if op == nil {
		return "完成"
	}
	if desc := automation.FormatRaceTaskOpDesc(op.TaskID, op.FlowerID); desc != "" {
		return desc
	}
	return "完成"
}

func cyclicStoryOrderClaimSuccessMessage(op *automation.PlannedOp, scoreBefore int32, scoreBeforeSet bool, st *state.State, at time.Time) string {
	count := int32(0)
	flowerID := int32(0)
	if op != nil {
		flowerID = op.FlowerID
		if flowerID > 0 {
			count = op.ItemCost[flowerID]
		}
		if count <= 0 && len(op.ItemCost) == 1 {
			for id, n := range op.ItemCost {
				flowerID = id
				count = n
			}
		}
	}
	flower := "鲜花"
	if flowerID > 0 {
		if name := flowerName(int(flowerID)); name != "" {
			flower = name
		} else {
			flower = fmt.Sprintf("#%d", flowerID)
		}
	}

	gained := cyclicStoryOrderScoreGain(op)
	scoreAfter := int32(0)
	scoreAfterOK := false
	if st != nil {
		if view, ok := st.CyclicStoryView(at); ok && view.Valid {
			scoreAfter = view.Score
			scoreAfterOK = true
			if scoreBeforeSet && scoreAfter > scoreBefore {
				gained = scoreAfter - scoreBefore
			}
		}
	}

	parts := make([]string, 0, 3)
	switch {
	case count > 0:
		parts = append(parts, fmt.Sprintf("提交了%d朵%s", count, flower))
	case flowerID > 0:
		parts = append(parts, fmt.Sprintf("提交了%s", flower))
	default:
		parts = append(parts, "提交订单")
	}
	if gained > 0 {
		parts = append(parts, fmt.Sprintf("获得了%d分", gained))
	}
	if scoreAfterOK {
		parts = append(parts, fmt.Sprintf("累计%d分", scoreAfter))
	}
	return strings.Join(parts, "，")
}

func cyclicStoryOrderScoreGain(op *automation.PlannedOp) int32 {
	if op == nil || op.TaskID <= 0 {
		return 0
	}
	info := state.CyclicStoryOrderInfoByID(op.TaskID)
	if !info.CatalogKnown {
		return 0
	}
	currencyID := int32(1108)
	if cfg, ok := state.CyclicStoryCatalogConfig(); ok && cfg.CurrencyItemID > 0 {
		currencyID = cfg.CurrencyItemID
	}
	var gained int32
	for _, reward := range info.Reward {
		if reward.ItemID == currencyID && reward.Count > 0 {
			gained += reward.Count
		}
	}
	if gained > 0 {
		return gained
	}
	for _, reward := range info.Reward {
		if reward.Count > 0 {
			gained += reward.Count
		}
	}
	return gained
}

func cyclicStoryMilestoneClaimSuccessMessage(op *automation.PlannedOp, st *state.State, at time.Time) string {
	idx := int32(0)
	if op != nil {
		idx = op.MilestoneIndex
	}
	if st != nil {
		if view, ok := st.CyclicStoryView(at); ok && view.Valid {
			if idx > 0 {
				return fmt.Sprintf("领取积分里程碑 #%d，累计%d分", idx, view.Score)
			}
			return fmt.Sprintf("领取积分里程碑，累计%d分", view.Score)
		}
	}
	if idx > 0 {
		return fmt.Sprintf("领取积分里程碑 #%d", idx)
	}
	return "领取积分里程碑"
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

func waterwheelClaimSuccessMessage(waterBefore int32, st *state.State) string {
	after, total, _ := st.WaterDrops()
	parts := []string{"水车水滴领取成功"}
	if gain := after - waterBefore; gain > 0 {
		parts = append(parts, fmt.Sprintf("+%d", gain))
	}
	if total > 0 {
		parts = append(parts, fmt.Sprintf("当前 %d/%d", after, total))
	} else {
		parts = append(parts, fmt.Sprintf("当前 %d", after))
	}
	claimed := st.WaterwheelClaimedCount()
	if max := state.WaterwheelBucketDailyMax(); max > 0 {
		parts = append(parts, fmt.Sprintf("今日 %d/%d", claimed, max))
	} else if claimed > 0 {
		parts = append(parts, fmt.Sprintf("今日已领 %d", claimed))
	}
	return strings.Join(parts, " ")
}

func freeWaterClaimSuccessMessage(op *automation.PlannedOp, waterBefore int32, st *state.State) string {
	after, total, _ := st.WaterDrops()
	parts := []string{"限时水滴领取成功"}
	if op != nil {
		parts = append(parts, fmt.Sprintf("时段#%d", op.TargetID))
	}
	if gain := after - waterBefore; gain > 0 {
		parts = append(parts, fmt.Sprintf("+%d", gain))
	}
	if total > 0 {
		parts = append(parts, fmt.Sprintf("当前 %d/%d", after, total))
	} else {
		parts = append(parts, fmt.Sprintf("当前 %d", after))
	}
	return strings.Join(parts, " ")
}

// flowerUpgradeSuccessMessage formats "花名 lvN-lvM" for cultivate.upgrade logs.
func flowerUpgradeSuccessMessage(op *automation.PlannedOp, fromLevel int32, st *state.State) string {
	name := "花朵"
	if op != nil && op.FlowerID > 0 {
		name = flowerName(int(op.FlowerID))
	}
	if fromLevel <= 0 && op != nil && op.Count > 0 {
		fromLevel = op.Count
	}
	toLevel := int32(0)
	if st != nil && op != nil && op.FlowerID > 0 {
		if cv, ok := st.Cultivations()[op.FlowerID]; ok && cv.Lvl > 0 {
			toLevel = cv.Lvl
		}
	}
	if fromLevel <= 0 && toLevel > 1 {
		fromLevel = toLevel - 1
	}
	if toLevel <= 0 && fromLevel > 0 {
		toLevel = fromLevel + 1
	}
	if fromLevel > 0 && toLevel > fromLevel {
		return fmt.Sprintf("鲜花升级: %s lv%d-lv%d", name, fromLevel, toLevel)
	}
	if fromLevel > 0 {
		return fmt.Sprintf("鲜花升级: %s lv%d", name, fromLevel)
	}
	return fmt.Sprintf("鲜花升级: %s", name)
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
	case clientproto.RPCActCyclicStoryEnter.String():
		if op.BatchID > 0 {
			return fmt.Sprintf(" (活动批次=%d)", op.BatchID)
		}
	case clientproto.RPCActCyclicStoryRecvOrderRwd.String():
		if op.BatchID > 0 && op.TaskID > 0 {
			return fmt.Sprintf(" (活动批次=%d 订单槽=%d 订单=%d)", op.BatchID, op.SlotID, op.TaskID)
		}
	case clientproto.RPCActCyclicStoryRecv.String():
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
	case clientproto.RPCCultivateUpgrade.String():
		if op.Count > 0 {
			return fmt.Sprintf(" lv%d-lv%d", op.Count, op.Count+1)
		}
	case clientproto.RPCBenefitBoxDraw.String():
		if op.Count > 0 {
			return fmt.Sprintf(" ×%d", op.Count)
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
