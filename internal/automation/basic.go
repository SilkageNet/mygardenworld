package automation

import (
	"fmt"
	"sort"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func basicOperations(s *state.State, policy *pb.Policy, goals []Goal, now time.Time) []PlannedOp {
	var ops []PlannedOp
	basic := policy.GetBasic()
	task := basic.GetTask()
	benefit := basic.GetBenefit()
	sign := basic.GetSign()
	add := func(enabled bool, kind, domain, action, reason string, priority int32, targetID int32, category string) {
		if !enabled {
			return
		}
		if category == "" {
			category = CategoryBasic
		}
		goal := Goal{ID: domain, Category: category, Domain: domain, Label: domain, Priority: priority / 100}
		ops = append(ops, op(kind, goal, action, reason, priority, targetID, 0, 0))
	}
	if basic.GetWaterwheelEnabled() && waterClaimAllowed(s, basic, now) && s.WaterwheelCooldownReady() {
		add(true, clientproto.RPCWaterwheelRecv.String(), "basic.waterwheel", "claim", "水车水滴可领取", 6500, 0, CategoryWater)
	}
	if basic.GetFreeWaterEnabled() && waterClaimAllowed(s, basic, now) {
		if idx, ok := s.NextFreeWaterIndex(now); ok {
			add(true, clientproto.RPCFreeWaterRecv.String(), "basic.free_water", "claim", "限时水滴可领取", 6450, idx, CategoryWater)
		}
	}
	if benefit.GetBoxEnabled() {
		if !s.BenefitBoxObserved() {
			reason := "福利宝箱 namespace 116 尚未同步，拒绝用领取请求试探状态"
			blocked := markerOp(CategoryBasic, "basic.benefit", "claim", reason, 6400)
			blocked.Status = PlanStatusBlocked
			blocked.Executable = false
			blocked.BlockedReasons = []string{reason}
			ops = append(ops, blocked)
		} else if remaining := s.BenefitBoxDrawsRemaining(now); remaining > 0 {
			goal := Goal{ID: "basic.benefit", Category: CategoryBasic, Domain: "basic.benefit", Label: "basic.benefit", Priority: 64}
			reason := fmt.Sprintf("福利宝箱可开启 %d 次", remaining)
			ops = append(ops, op(clientproto.RPCBenefitBoxDraw.String(), goal, "claim", reason, 6400, 0, 0, remaining))
		} else {
			waiting := markerOp(CategoryBasic, "basic.benefit", "claim", "福利宝箱暂无可领取次数，等待本地冷却累计", 6400)
			waiting.Status = PlanStatusSkipped
			waiting.Executable = false
			ops = append(ops, waiting)
		}
	}
	if benefit.GetDoubleCoinEnabled() && !s.VideoDoubleActive(now) {
		reason := "双倍金币未生效，已拒绝需要广告 SDK 回调的自动领取"
		if !s.VideoDoubleObserved() {
			reason = "双倍金币状态未同步，已拒绝需要广告 SDK 回调的自动领取"
		}
		blocked := markerOp(CategoryBasic, "basic.benefit.double_coin", "claim", reason, 6385)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{SDKAdUnsupportedReason}
		ops = append(ops, blocked)
	}
	if benefit.GetAntiScamBoxEnabled() {
		if status, ok := s.AntiFraudQAStatus(); ok && status != state.AntiFraudQAStatusClaimed {
			if status == 1 {
				add(true, clientproto.RPCUsrExtraRecvAntiFraudQARwd.String(), "basic.benefit.anti_scam", "claim", "防骗宝箱问答奖励可领取", 6370, 0, CategoryBasic)
			} else {
				add(true, clientproto.RPCUsrExtraUpdateAntiFraudQAStatus.String(), "basic.benefit.anti_scam", "answer", "防骗宝箱问答未完成，更新问答状态", 6375, 0, CategoryBasic)
			}
		}
	}
	if task.GetMainEnabled() {
		ops = append(ops, mainTaskOperations(s)...)
	}
	if task.GetDailyEnabled() {
		for _, id := range s.ReadyDailyTaskIDs() {
			add(true, clientproto.RPCTaskDlyRecv.String(), "basic.task.daily", "claim", "每日任务奖励可领取", 6250, id, CategoryBasic)
			break
		}
	}
	if task.GetWeeklyEnabled() {
		for _, id := range s.ReadyWeeklyTaskIDs() {
			add(true, clientproto.RPCTaskWeekRecv.String(), "basic.task.weekly", "claim", "每周任务奖励可领取", 6200, id, CategoryBasic)
			break
		}
	}
	if task.GetStoryEnabled() {
		ops = append(ops, storyOperations(s)...)
	}
	if task.GetAchievementEnabled() {
		for _, id := range s.ReadyAchievementTaskIDs() {
			add(true, clientproto.RPCTaskAchRecv.String(), "basic.task.achievement", "claim", "成就任务奖励可领取", 6120, id, CategoryBasic)
			break
		}
	}
	if basic.GetRoadGrowRewardEnabled() {
		for _, id := range s.ReadyRoadGrowTaskIDs() {
			add(true, clientproto.RPCRoadGrowRecv.String(), "basic.road_grow", "claim", "成长之路奖励可领取", 5980, id, CategoryBasic)
			break
		}
	}
	if basic.GetMapEventEnabled() {
		ops = append(ops, randomEventOperations(s)...)
	}
	ops = append(ops, zooOperations(s, basic.GetZoo(), now)...)
	if basic.GetMailEnabled() {
		if !s.MailObserved() {
			add(true, clientproto.RPCMailGetList.String(), "basic.mail", "sync", "邮件列表未同步，先获取列表", 5700, 0, CategoryBasic)
		} else {
			goal := Goal{ID: "basic.mail", Category: CategoryBasic, Domain: "basic.mail", Label: "邮件", Priority: 57}
			for _, target := range s.ReadyMailPickTargets() {
				claim := op(clientproto.RPCMailPick.String(), goal, "claim", "邮件奖励可领取", 5700, target.MsID, target.AllID, 0)
				ops = append(ops, claim)
				break
			}
		}
	}
	if sign.GetDailyEnabled() {
		ops = append(ops, signTypeOperations(s, now)...)
	}
	ops = append(ops, pearlOperations(s, basic.GetPearl(), now)...)
	return ops
}

func randomEventOperations(s *state.State) []PlannedOp {
	goal := Goal{ID: "basic.map_event", Category: CategoryBasic, Domain: "basic.map_event", Label: "地图随机事件", Priority: 59}
	observed, mapValid, mapError := s.RandomEventMapStatus()
	if !observed || !mapValid {
		reason := "地图随机事件未同步，先进入事件模块"
		if observed {
			reason = "地图随机事件数据异常，重新进入事件模块进行权威同步"
			if mapError != "" {
				reason += "：" + mapError
			}
		}
		planned := op(clientproto.RPCRandomEventEnter.String(), goal, "sync", reason, 5970, 0, 0, 0)
		planned.CooldownKey = "basic.map_event:sync"
		return []PlannedOp{planned}
	}

	events := s.RandomEvents()
	ids := make([]int32, 0, len(events))
	for id := range events {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	operations := make([]PlannedOp, 0, len(ids))
	for _, id := range ids {
		event := events[id]
		if event.Valid {
			planned := op(clientproto.RPCRandomEventDoAffair.String(), goal, "claim", "地图随机事件可处理", 5960, id, 0, 0)
			planned.CooldownKey = "basic.map_event:claim"
			operations = append(operations, planned)
			continue
		}
		reason := event.BlockedReason
		if reason == "" {
			reason = "事件未通过安全校验"
		}
		blocked := op(clientproto.RPCRandomEventDoAffair.String(), goal, "claim", "地图随机事件已阻塞", 5960, id, 0, 0)
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.BlockedReasons = []string{reason}
		blocked.CooldownKey = "basic.map_event:claim"
		operations = append(operations, blocked)
	}
	return operations
}

func signTypeOperations(s *state.State, now time.Time) []PlannedOp {
	const typeID = state.SignTypeAntiFraud
	view, namespaceObserved := s.SignType(typeID)
	blocked := func(reason string) []PlannedOp {
		op := markerOp(CategoryBasic, "basic.sign", "claim", reason, 5600)
		op.Status = PlanStatusBlocked
		op.Executable = false
		op.TargetID = typeID
		op.BlockedReasons = []string{reason}
		return []PlannedOp{op}
	}
	if !namespaceObserved {
		return blocked("namespace 140 防诈骗签到状态未同步，拒绝试探调用")
	}
	if !view.Observed {
		return blocked("namespace 140 未包含 type=1 防诈骗签到记录，拒绝试探调用")
	}
	if !view.Valid || !view.TypeObserved || !view.SignIDObserved || !view.StatusObserved {
		return blocked("type=1 防诈骗签到状态或 c_signReward 定义不完整，拒绝自动操作")
	}
	base, baseMapObserved := s.BaseReward(state.BaseRewardAntiFraud)
	if !baseMapObserved || !base.Observed || !base.Valid || !base.TypeObserved || !base.StatusObserved || !base.UpdatedAtObserved {
		return blocked("namespace 7.7[2] 基础防骗奖励状态不完整，拒绝进入 signType 流程")
	}
	if base.Status != state.BaseRewardStatusReceived {
		return blocked("基础防骗奖励尚未领取；rwd.setCanRecv/rwd.recv 不在本自动化安全范围内")
	}
	if base.UpdatedToday(now) {
		// The client considers today's anti-fraud reward complete at this gate
		// and does not enter signType type=1.
		return nil
	}
	if !base.UpdatedBeforeToday(now) {
		return blocked("基础防骗奖励更新时间不是今天之前的可信时间，拒绝进入 signType 流程")
	}

	goal := Goal{ID: "basic.sign", Category: CategoryBasic, Domain: "basic.sign", Label: "防诈骗签到", Priority: 56}
	switch view.Status {
	case state.SignTypeStatusCanSign:
		return []PlannedOp{op(clientproto.RPCSignTypeSign.String(), goal, "claim", "防诈骗签到条件可执行", 5600, typeID, 0, 0)}
	case state.SignTypeStatusCanReceive:
		return []PlannedOp{op(clientproto.RPCSignTypeRecv.String(), goal, "claim", "防诈骗签到已完成，奖励可领取", 5600, typeID, 0, 0)}
	case state.SignTypeStatusReceived:
		if !view.UpdatedAtObserved {
			return blocked("防诈骗签到终态缺少更新时间，无法判断是否需要跨日同步")
		}
		if view.UpdatedToday(now) {
			return nil
		}
		if !view.UpdatedBeforeToday(now) {
			return blocked("防诈骗签到终态更新时间异常，拒绝跨日试探")
		}
		if s.SignTypeEnterAttemptedToday(typeID, now) {
			return nil
		}
		sync := op(clientproto.RPCSignTypeEnter.String(), goal, "sync", "防诈骗签到终态属于前一日，执行本日一次状态同步", 5605, typeID, 0, 0)
		sync.OperationID = fmt.Sprintf("%s:%d:%s", clientproto.RPCSignTypeEnter.String(), typeID, now.Format("20060102"))
		return []PlannedOp{sync}
	default:
		return blocked("type=1 防诈骗签到状态不在实测的 0/1/2 状态机内")
	}
}

func mainTaskOperations(s *state.State) []PlannedOp {
	task, observed := s.MainTask()
	if !observed {
		blocked := markerOp(CategoryBasic, "basic.task.main", "claim", "主线任务状态尚未同步，拒绝试探领奖", 6300)
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.BlockedReasons = []string{"namespace 22.0 尚未观察，taskMain.recv 没有安全的查询或试探语义"}
		return []PlannedOp{blocked}
	}
	if !task.Valid {
		blocked := markerOp(CategoryBasic, "basic.task.main", "claim", "主线任务目录或服务端状态无效，暂不自动领奖", 6300)
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.TargetID = task.TaskID
		blocked.BlockedReasons = []string{"22.0 curTaskId 无法匹配 c_task_main 从 $initId 到 $endId 的有效任务定义"}
		return []PlannedOp{blocked}
	}
	if task.Complete || task.Receipted {
		return nil
	}
	if !task.ProgressObserved || !task.ReceiptObserved {
		blocked := markerOp(CategoryBasic, "basic.task.main", "claim", "主线任务进度或领奖记录未完整同步", 6300)
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.TargetID = task.TaskID
		blocked.BlockedReasons = []string{"必须同时观察 22.0 curValue 与完整 recvMap，才可确认任务完成且未领取"}
		return []PlannedOp{blocked}
	}
	if task.Target <= 0 {
		blocked := markerOp(CategoryBasic, "basic.task.main", "claim", "主线任务目标缺失，禁止试探领奖", 6300)
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.TargetID = task.TaskID
		blocked.BlockedReasons = []string{"c_task_main 当前任务缺少严格正整数 value"}
		return []PlannedOp{blocked}
	}
	if task.Finished < task.Target {
		if flowerID, _, cultivateTask := state.MainTaskCultivateTarget(task.TaskID); cultivateTask {
			if cultivate, observed := s.Cultivations()[flowerID]; observed && cultivate.Lvl > 0 {
				blocked := markerOp(CategoryBasic, "basic.task.main", "sync", "主线培育目标已完成，但任务进度尚未同步", 6300)
				blocked.Status = PlanStatusBlocked
				blocked.Executable = false
				blocked.TargetID = task.TaskID
				blocked.BlockedReasons = []string{"拒绝重复培育 " + state.ItemName(flowerID) + "；等待服务端刷新主线任务计数"}
				return []PlannedOp{blocked}
			}
		}
		return nil
	}
	snapshot, ready := s.MainTaskClaimSnapshot()
	if !ready || snapshot.TaskID != task.TaskID || snapshot.Target != task.Target || snapshot.NextTaskID != task.NextTaskID {
		blocked := markerOp(CategoryBasic, "basic.task.main", "claim", "主线任务领奖前置状态发生变化", 6300)
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.TargetID = task.TaskID
		blocked.BlockedReasons = []string{"当前任务、目标、后继任务或领奖状态未通过原子快照校验"}
		return []PlannedOp{blocked}
	}
	goal := Goal{ID: GoalMainTask, Category: CategoryBasic, Domain: "basic.task.main", Label: "主线任务", Priority: 70}
	planned := op(clientproto.RPCTaskMainRecv.String(), goal, "claim", "主线任务已完成且奖励未领取", 6300, task.TaskID, 0, 0)
	// The main-task chain is linear. Keep one cooldown key across task switches
	// after an ambiguous response.
	planned.OperationID = clientproto.RPCTaskMainRecv.String()
	return []PlannedOp{planned}
}

func storyOperations(s *state.State) []PlannedOp {
	goal := Goal{ID: "basic.story", Category: CategoryBasic, Domain: "basic.story", Label: "剧情", Priority: 61}
	if !s.StoryMainObserved() {
		return []PlannedOp{domainOp(clientproto.RPCStoryMainEnter.String(), goal, "basic.story", "sync", "主线剧情未同步，先进入剧情模块", 6140, 0, 0, 0)}
	}
	story, ok := s.StoryMain()
	if !ok || !story.Valid {
		blocked := markerOp(CategoryBasic, "basic.story", "unlock", "主线剧情当前进度无效，暂不自动解锁", 6130)
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.BlockedReasons = []string{"7.101 章节/小节未完整观察，或无法与精确剧情目录匹配"}
		return []PlannedOp{blocked}
	}
	if story.Complete {
		return nil
	}
	if story.SectionID <= 0 {
		blocked := markerOp(CategoryBasic, "basic.story", "unlock", "主线剧情当前小节未识别，暂不自动解锁", 6130)
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.BlockedReasons = []string{"有效剧情进度缺少目录小节 ID"}
		return []PlannedOp{blocked}
	}
	if len(story.Cost) == 0 {
		blocked := markerOp(CategoryBasic, "basic.story", "unlock", "主线剧情解锁成本未识别，暂不自动解锁", 6130)
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.TargetID = story.SectionID
		blocked.BlockedReasons = []string{"当前剧情小节没有可识别的 cost"}
		return []PlannedOp{blocked}
	}
	inventory := s.Inventory()
	itemCost := make(map[int32]int32, len(story.Cost))
	var missing []string
	for _, cost := range story.Cost {
		if cost.ItemID <= 0 || cost.Count <= 0 {
			continue
		}
		itemCost[cost.ItemID] += cost.Count
		if have := inventory[cost.ItemID]; have < cost.Count {
			missing = append(missing, fmt.Sprintf("%s不足：需要%d，当前%d", itemLabel(cost.ItemID), cost.Count, have))
		}
	}
	if len(missing) > 0 {
		blocked := markerOp(CategoryBasic, "basic.story", "unlock", "主线剧情星星不足，暂不进入执行队列", 6130)
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.TargetID = story.SectionID
		blocked.ItemCost = itemCost
		blocked.BlockedReasons = missing
		return []PlannedOp{blocked}
	}
	planned := op(clientproto.RPCStoryMainUnlock.String(), goal, "unlock", "主线剧情小节可解锁", 6130, story.SectionID, 0, 0)
	// Story progress is linear. Keep one cooldown key across sections so an
	// ambiguous response cannot immediately spend again on the next section.
	planned.OperationID = clientproto.RPCStoryMainUnlock.String()
	planned.ItemCost = itemCost
	return []PlannedOp{planned}
}

func waterClaimAllowed(s *state.State, basic *pb.BasicPolicy, now time.Time) bool {
	if s == nil {
		return false
	}
	// Regen capacity (e.g. 130) only caps natural recovery. Waterwheel / free-water
	// claims can push inventory above that, so do not gate them on capacity.
	if threshold := basic.GetWaterClaimThreshold(); threshold > 0 {
		waterDrops, _, _ := s.AvailableWaterDrops(now)
		if waterDrops >= threshold {
			return false
		}
	}
	return true
}

func zooOperations(s *state.State, policy *pb.ZooPolicy, now time.Time) []PlannedOp {
	if policy == nil {
		return nil
	}
	if !policy.GetEnabled() {
		if policy.GetAutoFeed() || policy.GetAutoBuyFood() || policy.GetAutoStroke() || policy.GetAutoEventEnabled() {
			reason := "宠物子功能已开启，但宠物模块总开关未开启"
			blocked := markerOp(CategoryBasic, "basic.zoo", "sync", reason, 5690)
			blocked.Status = PlanStatusBlocked
			blocked.Executable = false
			blocked.BlockedReasons = []string{reason}
			return []PlannedOp{blocked}
		}
		return nil
	}
	goal := Goal{ID: "basic.zoo", Category: CategoryBasic, Domain: "basic.zoo", Label: "宠物", Priority: 57}
	var ops []PlannedOp
	if !s.ZooObserved() {
		return []PlannedOp{domainOp(clientproto.RPCZooEnterZoo.String(), goal, "basic.zoo", "sync", "宠物状态未同步，先进入宠物模块", 5690, 0, 0, 0)}
	}
	need, bowlNeedsStock := s.NextZooFoodBowlNeed()
	food, foodInInventory := s.NextZooFoodstuffPlan()
	if policy.GetAutoFeed() && bowlNeedsStock {
		if foodInInventory {
			stock := domainOp(clientproto.RPCZooAddFoodstuff.String(), goal, "basic.zoo.feed", "stock", "按客户端食盆容量使用已有库存补充宠物食盆", 5688, food.PetID, food.FoodstuffID, food.Count)
			stock.ItemCost = map[int32]int32{food.FoodstuffID: food.Count}
			ops = append(ops, stock)
		} else if !policy.GetAutoBuyFood() {
			waiting := markerOp(CategoryBasic, "basic.zoo.feed", "stock", "食盆有空位，但普通/高级猫粮库存均为 0；可开启购买普通猫粮", 5688)
			waiting.Status = PlanStatusSkipped
			waiting.Executable = false
			waiting.TargetID = need.PetID
			waiting.Count = need.Count
			ops = append(ops, waiting)
		}
	} else if policy.GetAutoFeed() {
		reason := "所有已观测宠物的食盆均已装满"
		status := PlanStatusSkipped
		if !s.ZooFoodBowlsObserved() {
			reason = "宠物食盆 foodstuffArr 尚未完整同步"
			status = PlanStatusBlocked
		}
		waiting := markerOp(CategoryBasic, "basic.zoo.feed", "stock", reason, 5688)
		waiting.Status = status
		waiting.Executable = false
		if status == PlanStatusBlocked {
			waiting.BlockedReasons = []string{reason}
		}
		ops = append(ops, waiting)
	}
	if policy.GetAutoBuyFood() {
		switch {
		case !policy.GetAutoFeed():
			reason := "购买普通猫粮需要同时开启自动补充食盆，避免无需求囤积"
			blocked := markerOp(CategoryBasic, "basic.zoo.buy_food", "buy", reason, 5687)
			blocked.Status = PlanStatusBlocked
			blocked.Executable = false
			blocked.BlockedReasons = []string{reason}
			ops = append(ops, blocked)
		case bowlNeedsStock && !foodInInventory:
			ops = append(ops, zooFoodPurchaseOperation(s, policy, goal, need, now))
		}
	}
	refreshPlanned := false
	for _, petID := range s.ReadyZooStatusRefreshPetIDs(now) {
		ops = append(ops, domainOp(clientproto.RPCZooRefreshPetStatus.String(), goal, "basic.zoo", "refresh", "宠物状态冷却已到期，刷新服务端状态后再判断互动", 5685, petID, 0, 0))
		refreshPlanned = true
		break
	}
	if policy.GetAutoStroke() && !refreshPlanned {
		readyPetIDs := s.ReadyZooStrokePetIDs(now)
		if len(readyPetIDs) > 0 {
			ops = append(ops, domainOp(clientproto.RPCZooStrokePet.String(), goal, "basic.zoo.stroke", "stroke", "宠物当前可互动且心情未满", 5670, readyPetIDs[0], 0, 0))
		} else {
			reason := s.ZooStrokeWaitReason(now)
			waiting := markerOp(CategoryBasic, "basic.zoo.stroke", "stroke", reason, 5670)
			waiting.Status = PlanStatusSkipped
			waiting.Executable = false
			ops = append(ops, waiting)
		}
	}
	if policy.GetAutoEventEnabled() {
		if !s.ZooLogsObserved() {
			reason := s.ZooLogsUnavailableReason()
			if reason == "" {
				reason = "宠物服务端日志尚未同步，不使用宠物字段猜测事件"
			}
			blocked := markerOp(CategoryBasic, "basic.zoo.event", "handle_event", reason, 5665)
			blocked.Status = PlanStatusBlocked
			blocked.Executable = false
			blocked.BlockedReasons = []string{reason}
			ops = append(ops, blocked)
		} else {
			actionPlanned := false
			for _, evt := range s.ZooEventActions() {
				reason := evt.BlockedReason
				if reason == "" {
					if evt.Action == "read_log" {
						reason = "确认已完成宠物日志为已读"
					} else {
						reason = "宠物服务端日志确认事件无消耗且结果唯一"
					}
				}
				if evt.Blocked {
					blocked := markerOp(CategoryBasic, "basic.zoo.event", evt.Action, reason, 5665)
					blocked.Status = PlanStatusBlocked
					blocked.Executable = false
					blocked.TargetID = evt.PetID
					blocked.ItemID = evt.TableID
					blocked.OperationID = operationID(blocked.Kind, nil, 0, evt.PetID, evt.TableID)
					blocked.BlockedReasons = []string{reason}
					ops = append(ops, blocked)
					continue
				}
				if actionPlanned {
					continue
				}
				actionPlanned = true
				rpc := clientproto.RPCZooHandleEvent.String()
				priority := int32(5665)
				if evt.Action == "read_log" {
					rpc = clientproto.RPCZooReadLog.String()
					priority = 5664
				}
				planned := domainOp(rpc, goal, "basic.zoo.event", evt.Action, reason, priority, evt.PetID, evt.TableID, 0)
				if evt.Action == "handle_event" {
					planned.Count = 1
				}
				ops = append(ops, planned)
			}
		}
		if !s.ZooSouvenirsObserved() {
			reason := "宠物纪念品集合尚未完整观测，拒绝推断收集数量或未读状态"
			blocked := markerOp(CategoryBasic, "basic.zoo.souvenir", "claim", reason, 5663)
			blocked.Status = PlanStatusBlocked
			blocked.Executable = false
			blocked.BlockedReasons = []string{reason}
			ops = append(ops, blocked)
		} else {
			zoo := s.Zoo()
			if !zoo.SouvenirRewardIDsObserved {
				reason := "宠物纪念品奖励领取列表未观测，拒绝试探领奖"
				blocked := markerOp(CategoryBasic, "basic.zoo.souvenir", "claim", reason, 5663)
				blocked.Status = PlanStatusBlocked
				blocked.Executable = false
				blocked.BlockedReasons = []string{reason}
				ops = append(ops, blocked)
			} else if rewardIDs := s.ReadyZooSouvenirRewardIDs(); len(rewardIDs) > 0 {
				claim := domainOp(clientproto.RPCZooRecvSouvenirRwd.String(), goal, "basic.zoo.souvenir", "claim", "纪念品收集里程碑已达成且尚未领取", 5663, 0, 0, int32(len(rewardIDs)))
				claim.SlotIDs = append([]int32(nil), rewardIDs...)
				ops = append(ops, claim)
			} else if souvenirIDs := s.UnreadZooSouvenirIDs(); len(souvenirIDs) > 0 {
				read := domainOp(clientproto.RPCZooReadSouvenir.String(), goal, "basic.zoo.souvenir", "read", "纪念品奖励均已领取，清理明确观测到的未读纪念品", 5662, 0, 0, int32(len(souvenirIDs)))
				read.SlotIDs = append([]int32(nil), souvenirIDs...)
				ops = append(ops, read)
			}
		}
	}
	return ops
}

func zooFoodPurchaseOperation(s *state.State, policy *pb.ZooPolicy, goal Goal, need state.ZooFoodBowlNeed, now time.Time) PlannedOp {
	shop := s.ZooFoodShop(now)
	if shop.NeedsEnter {
		return domainOp(clientproto.RPCShopEnter.String(), goal, "basic.zoo.buy_food", "sync", "普通猫粮商店状态或今日限购记录未同步，先进入商店 9", 5687, state.ZooFoodShopTempID, 0, 0)
	}
	blocked := func(status, reason string) PlannedOp {
		op := markerOp(CategoryBasic, "basic.zoo.buy_food", "buy", reason, 5687)
		op.Status = status
		op.Executable = false
		op.TargetID = shop.ShopTempID
		op.ItemID = shop.ShopItemID
		if status == PlanStatusBlocked {
			op.BlockedReasons = []string{reason}
		}
		return op
	}
	if shop.ShopTempID != state.ZooFoodShopTempID || shop.ShopItemID != state.ZooNormalFoodShopItemID ||
		shop.FoodstuffID != 1501 || shop.FoodstuffCount <= 0 || shop.GoldCost <= 0 || shop.DailyLimit <= 0 {
		return blocked(PlanStatusBlocked, "c_shop_item_9 的普通猫粮商品、金币成本或每日限购配置不完整")
	}
	if shop.DailyRemaining <= 0 {
		return blocked(PlanStatusSkipped, fmt.Sprintf("普通猫粮今日已购买 %d/%d，等待每日重置", shop.DailyBought, shop.DailyLimit))
	}
	if policy.GetMaxSpendGold() <= 0 {
		return blocked(PlanStatusBlocked, "购买普通猫粮前必须设置单次金币上限")
	}
	count := need.Count
	if count > shop.DailyRemaining {
		count = shop.DailyRemaining
	}
	if byBudget := policy.GetMaxSpendGold() / int64(shop.GoldCost); int64(count) > byBudget {
		count = int32(byBudget)
	}
	if byBalance := s.Gold() / shop.GoldCost; count > byBalance {
		count = byBalance
	}
	if count <= 0 {
		if int64(shop.GoldCost) > policy.GetMaxSpendGold() {
			return blocked(PlanStatusBlocked, fmt.Sprintf("普通猫粮每份需 %d 金币，超过单次金币上限", shop.GoldCost))
		}
		return blocked(PlanStatusBlocked, fmt.Sprintf("金币不足：普通猫粮每份需 %d，当前 %d", shop.GoldCost, s.Gold()))
	}
	buy := domainOp(clientproto.RPCShopBuy.String(), goal, "basic.zoo.buy_food", "buy", fmt.Sprintf("食盆缺少 %d 份，购买金币普通猫粮 %d 份", need.Count, count), 5687, shop.ShopTempID, shop.ShopItemID, count)
	buy.GoldCost = shop.GoldCost * count
	return buy
}

// ValidateZooFoodPurchase rechecks the exact client shop, bowl demand, quota,
// policy and gold cost immediately before runner execution.
func ValidateZooFoodPurchase(s *state.State, policy *pb.ZooPolicy, op *PlannedOp, now time.Time) error {
	if s == nil || policy == nil || op == nil {
		return fmt.Errorf("购买普通猫粮缺少状态、策略或操作")
	}
	if !policy.GetEnabled() || !policy.GetAutoFeed() || !policy.GetAutoBuyFood() {
		return fmt.Errorf("宠物模块、自动补充食盆或购买普通猫粮已关闭")
	}
	need, ok := s.NextZooFoodBowlNeed()
	if !ok {
		return fmt.Errorf("食盆已满，无需购买猫粮")
	}
	if _, ok := s.NextZooFoodstuffPlan(); ok {
		return fmt.Errorf("已有猫粮库存，应先补充食盆")
	}
	shop := s.ZooFoodShop(now)
	if shop.NeedsEnter || !shop.Observed {
		return fmt.Errorf("普通猫粮商店今日状态未同步")
	}
	if op.TargetID != shop.ShopTempID || op.ItemID != shop.ShopItemID || op.Count <= 0 || op.Count > need.Count || op.Count > shop.DailyRemaining {
		return fmt.Errorf("普通猫粮购买参数已过期：shop=%d item=%d count=%d need=%d remaining=%d", op.TargetID, op.ItemID, op.Count, need.Count, shop.DailyRemaining)
	}
	wantGold := shop.GoldCost * op.Count
	if shop.GoldCost <= 0 || wantGold <= 0 || op.GoldCost != wantGold || int64(wantGold) > policy.GetMaxSpendGold() {
		return fmt.Errorf("普通猫粮金币成本未通过策略校验：计划=%d 当前=%d 上限=%d", op.GoldCost, wantGold, policy.GetMaxSpendGold())
	}
	if op.DiamondCost != 0 || len(op.ItemCost) != 0 {
		return fmt.Errorf("普通猫粮自动购买仅允许金币成本")
	}
	return nil
}

func pearlOperations(s *state.State, policy *pb.PearlPolicy, now time.Time) []PlannedOp {
	if policy == nil || !pearlPolicyEnabled(policy) {
		return nil
	}
	goal := Goal{ID: "basic.pearl", Category: CategoryBasic, Domain: "basic.pearl", Label: "珍珠", Priority: 55}
	if !s.PearlObserved() {
		return []PlannedOp{domainOp(clientproto.RPCPearlRefresh.String(), goal, "basic.pearl", "sync", "珍珠状态未同步，先刷新珍珠数据", 5590, 0, 0, 0)}
	}
	var ops []PlannedOp
	if policy.GetFreeEnabled() && s.PearlDailyFreeReady(now) {
		ops = append(ops, domainOp(clientproto.RPCPearlRecvDailyFree.String(), goal, "basic.pearl.free", "claim", "每日免费珍珠可领取", 5580, 0, 0, 0))
	}
	if len(s.ReadyPearlPlaceIDsAt(now)) > 0 {
		ops = append(ops, domainOp(clientproto.RPCPearlPlaceRecvOneKey.String(), goal, "basic.pearl.place", "claim", "珍珠实时产出可一键收取", 5570, 0, 0, 0))
	}
	pearl := s.Pearl()
	if policy.GetProtectEnabled() && pearl.ProtectState != 1 {
		protect := domainOp(clientproto.RPCPearlSetProtectState.String(), goal, "basic.pearl.protect", "enable", "珍珠防身未开启", 5560, 1, 0, 0)
		if pearl.ProtectNum <= 0 {
			protect.Status = PlanStatusAdapterMissing
			protect.Executable = false
			protect.BlockedReasons = []string{"防身符不足或未观测"}
		}
		ops = append(ops, protect)
	}
	if policy.GetDrawEnabled() {
		if count := s.PearlDrawCount(); count > 0 {
			draw := domainOp(clientproto.RPCPearlDraw.String(), goal, "basic.pearl.draw", "draw", "存在可开启珍珠", 5550, 0, 0, 1)
			if count < draw.Count {
				draw.Count = count
			}
			ops = append(ops, draw)
		}
	}
	if policy.GetAutoHireEnabled() {
		if hire, ok := PlanOneSafePearlHire(s, policy, now, PearlHireIntent{}); ok {
			ops = append(ops, hire)
		}
	}
	if policy.GetAutoBuyHireTicket() {
		buy := markerOp(CategoryBasic, "basic.pearl.buy_hire_ticket", "buy", "购买雇佣书涉及元宝成本", 110)
		buy.Label = "购买雇佣书"
		buy.Status = PlanStatusAdapterMissing
		buy.Executable = false
		if policy.GetMaxSpendDiamond() <= 0 {
			buy.BlockedReasons = []string{"购买雇佣书需要先设置元宝上限"}
		} else {
			buy.BlockedReasons = []string{"元宝成本操作尚未放开自动执行"}
		}
		ops = append(ops, buy)
	}
	return ops
}

func pearlPolicyEnabled(policy *pb.PearlPolicy) bool {
	return policy.GetFreeEnabled() || policy.GetAutoHireEnabled() || policy.GetDrawEnabled() ||
		policy.GetProtectEnabled() || policy.GetAutoBuyHireTicket()
}
