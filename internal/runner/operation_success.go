package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func (r *Runner) handleOperationSuccess(ctx context.Context, result operationResult) {
	op, args := result.op, result.args
	r.stats.RecordOperationSuccess(op, result.finishedAt)
	kind := "operation_ack"
	label := operationEventLabel(op)
	message := fmt.Sprintf("%s 完成%s", opDesc(op), r.opSuffix(op))
	category := op.Category
	switch op.Kind {
	case clientproto.RPCFrdHomeGetFrdHomeInfo.String():
		view := r.state.FriendTouch(result.finishedAt)
		if view.VisitUID == op.TargetUID {
			if _, ok := state.PickFriendStealLandID(view.VisitLands, nil, r.state.RoleID(), result.finishedAt); !ok {
				r.state.MarkFriendTouchSkipEnter(op.TargetUID, result.finishedAt.Add(5*time.Minute))
			}
		}
	case clientproto.RPCFrdStealSteal.String():
		r.state.NoteFriendStealSuccess(op.TargetUID, op.TargetID, result.friendStealUsedBefore, result.friendStealUsedBeforeSet, result.finishedAt)
		r.state.ClearFriendTouchSkipEnter(op.TargetUID)
	case clientproto.RPCFrdExtBuyStealCnt.String():
		r.state.NoteFriendStealPurchase(op.TargetUID, result.friendStealBoughtBefore, result.friendStealBoughtBeforeSet, result.finishedAt)
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
	case clientproto.RPCFlowerRackCancelSell.String():
		kind = "flower_rack_cancel"
		label = "花艺下架"
		category = automation.CategoryOrder
		message = fmt.Sprintf("花架下架 rack=%d", op.TargetID)
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
		if strings.TrimSpace(op.Reason) != "" {
			message += " · " + strings.TrimSpace(op.Reason)
		}
	case clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String():
		kind = "race_task_sync"
		label = "同步竞赛已做次数"
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
	case clientproto.RPCMailPick.String():
		kind = "mail_claim"
		label = "邮件"
		message = fmt.Sprintf("领取邮件附件 msId=%d allId=%d", op.TargetID, op.ItemID)
		// Successful pick responses often omit ns19.isPick; mark locally so the
		// planner does not retry and hit「邮件附件已领取」.
		r.state.MarkMailPicked(op.TargetID, op.ItemID)
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
	r.clearCultivateUpgradeResourceRejection(op)
	r.deferNoopCustomerOrderGeneration(op, result.finishedAt)

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
	case clientproto.RPCOrderCustomerFinishOrder.String():
		r.state.NoteCustomerOrderFinished(result.finishedAt, result.raw)
		r.emitCustomerOrderLimitInfo(r.Policy(), result.finishedAt)
	}
	if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() &&
		automation.RaceHoldsUnfinishedCustomerOrder(r.state.FmlRace()) {
		// Customer-order race FinishCnt advances via getTaskList, not harvest
		// field 134. Force a pool refresh on the next tick.
		r.state.MarkFmlRaceTaskPoolStale()
	}
	if op.Kind == clientproto.RPCPearlPlaceHire.String() &&
		automation.RaceHoldsUnfinishedPearlHire(r.state.FmlRace()) {
		// Pearl-hire race FinishCnt advances via getTaskList. Force a pool
		// refresh on the next tick after a successful hire.
		r.state.MarkFmlRaceTaskPoolStale()
	}
	if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() &&
		automation.RaceHoldsUnfinishedFlowerArtCraft(r.state.FmlRace()) {
		// Flower-art-craft race FinishCnt advances via getTaskList. Force a
		// pool refresh on the next tick after a successful craft.
		r.state.MarkFmlRaceTaskPoolStale()
	}
	if op.Kind == clientproto.RPCFlowerRackSell.String() &&
		automation.RaceHoldsUnfinishedFlowerArtSell(r.state.FmlRace()) {
		// Flower-art-sell race FinishCnt advances via getTaskList. Force a
		// pool refresh on the next tick after a successful listing.
		r.state.MarkFmlRaceTaskPoolStale()
	}
	if (op.Kind == clientproto.RPCCultivateCultivate.String() ||
		op.Kind == clientproto.RPCCultivateRecv.String()) &&
		automation.RaceHoldsUnfinishedFlowerCultivate(r.state.FmlRace()) {
		// Flower-cultivate race FinishCnt also advances only via getTaskList;
		// without this hook submission waits on the 10-minute fallback sync.
		r.state.MarkFmlRaceTaskPoolStale()
	}
}

// deferNoopCustomerOrderGeneration prevents an accepted genOrder response
// that leaves namespace 109 empty and ready from being retried every decision
// tick. A later namespace push can still produce a real order immediately;
// only another generation attempt is held back.
func (r *Runner) deferNoopCustomerOrderGeneration(op *automation.PlannedOp, now time.Time) {
	if op == nil || op.Kind != clientproto.RPCOrderCustomerGenOrder.String() ||
		r.state == nil || !r.state.CustomerOrderGenerationReady(now) {
		return
	}
	reason := "服务端本次未生成顾客订单，10 分钟后重试"
	payloadOp := r.cooldownSideOperation(op, now, nil, reason, customerOrderGenerationNoopCooldown)
	r.emit(Event{
		Kind:        "operation_deferred",
		Category:    automation.CategoryOrder,
		Domain:      op.Domain,
		Action:      "blocked",
		Label:       operationEventLabel(op),
		Message:     fmt.Sprintf("%s 暂缓: %s", opDesc(op), reason),
		PayloadJSON: operationPayload(payloadOp, nil, nil, nil),
		Level:       "warn",
	})
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

// emitCustomerOrderLimitInfo writes a clear log line when customer orders are
// paused by the policy daily limit. Deduped by reason so decision ticks do not
// spam the event stream.
func (r *Runner) emitCustomerOrderLimitInfo(policy *pb.Policy, now time.Time) {
	if policy == nil || r.state == nil {
		return
	}
	customer := policy.GetOrder().GetCustomer()
	if !customer.GetEnabled() {
		r.lastCustomerOrderLimitReason = ""
		return
	}
	reason, reached := automation.CustomerDailyLimitReached(r.state, customer, now)
	if !reached {
		r.lastCustomerOrderLimitReason = ""
		return
	}
	if reason == r.lastCustomerOrderLimitReason {
		return
	}
	r.lastCustomerOrderLimitReason = reason
	r.emit(Event{
		Kind:     "operation_deferred",
		Category: "order",
		Domain:   "order.customer",
		Action:   "blocked",
		Label:    "顾客订单",
		Message:  fmt.Sprintf("顾客订单暂停: %s，已跳过接单/交付以继续执行其他流程", reason),
		Level:    "warn",
	})
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
