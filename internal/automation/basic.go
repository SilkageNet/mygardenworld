package automation

import (
	"fmt"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"time"
)

func basicOperations(s *state.State, policy *pb.Policy, goals []Goal, now time.Time) []PlannedOp {
	var ops []PlannedOp
	basic := policy.GetBasic()
	task := basic.GetTask()
	benefit := basic.GetBenefit()
	sign := basic.GetSign()
	add := func(enabled bool, kind, domain, action, reason string, priority int32, targetID int32) {
		if !enabled {
			return
		}
		goal := Goal{ID: domain, Category: CategoryBasic, Domain: domain, Label: domain, Priority: priority / 100}
		ops = append(ops, op(kind, goal, action, reason, priority, targetID, 0, 0))
	}
	if basic.GetWaterwheelEnabled() && waterClaimAllowed(s, basic, now) && s.WaterwheelCooldownReady() {
		add(true, clientproto.RPCWaterwheelRecv.String(), "basic.waterwheel", "claim", "水车水滴可领取", 6500, 0)
	}
	if basic.GetFreeWaterEnabled() && waterClaimAllowed(s, basic, now) {
		if idx, ok := s.NextFreeWaterIndex(now); ok {
			add(true, clientproto.RPCFreeWaterRecv.String(), "basic.free_water", "claim", "限时水滴可领取", 6450, idx)
		}
	}
	if benefit.GetBoxEnabled() && s.BenefitBoxReady() {
		add(true, clientproto.RPCBenefitBoxDraw.String(), "basic.benefit", "claim", "福利宝箱可领取", 6400, 0)
	}
	if benefit.GetDoubleCoinEnabled() && !s.VideoDoubleActive(now) {
		reason := "双倍金币未生效，看视频奖励需要客户端 SDK token"
		if !s.VideoDoubleObserved() {
			reason = "双倍金币状态未同步，看视频奖励需要客户端 SDK token"
		}
		blocked := markerOp(CategoryBasic, "basic.benefit.double_coin", "claim", reason, 6385)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"客户端通过 UT.share(11,{opType:1}) 完成激励视频后调用 usr.share；本地 runner 不伪造视频完成或 SDK token"}
		ops = append(ops, blocked)
	}
	if benefit.GetAntiScamBoxEnabled() {
		if status, ok := s.AntiFraudQAStatus(); ok && status != state.AntiFraudQAStatusClaimed {
			if status == 1 {
				add(true, clientproto.RPCUsrExtraRecvAntiFraudQARwd.String(), "basic.benefit.anti_scam", "claim", "防骗宝箱问答奖励可领取", 6370, 0)
			} else {
				add(true, clientproto.RPCUsrExtraUpdateAntiFraudQAStatus.String(), "basic.benefit.anti_scam", "answer", "防骗宝箱问答未完成，更新问答状态", 6375, 0)
			}
		}
	}
	if task.GetDailyEnabled() {
		for _, id := range s.ReadyDailyTaskIDs() {
			add(true, clientproto.RPCTaskDlyRecv.String(), "basic.task.daily", "claim", "每日任务奖励可领取", 6250, id)
			break
		}
	}
	if task.GetWeeklyEnabled() {
		for _, id := range s.ReadyWeeklyTaskIDs() {
			add(true, clientproto.RPCTaskWeekRecv.String(), "basic.task.weekly", "claim", "每周任务奖励可领取", 6200, id)
			break
		}
	}
	if task.GetStoryEnabled() {
		ops = append(ops, storyOperations(s)...)
	}
	if task.GetAchievementEnabled() {
		for _, id := range s.ReadyAchievementTaskIDs() {
			add(true, clientproto.RPCTaskAchRecv.String(), "basic.task.achievement", "claim", "成就任务奖励可领取", 6120, id)
			break
		}
	}
	if basic.GetRoadGrowRewardEnabled() {
		for _, id := range s.ReadyRoadGrowTaskIDs() {
			add(true, clientproto.RPCRoadGrowRecv.String(), "basic.road_grow", "claim", "成长之路奖励可领取", 5980, id)
			break
		}
	}
	if basic.GetMapEventEnabled() {
		if !s.RandomEventObserved() {
			add(true, clientproto.RPCRandomEventEnter.String(), "basic.map_event", "sync", "地图随机事件未同步，先进入事件模块", 5970, 0)
		} else {
			for _, id := range s.ReadyRandomEventIDs() {
				add(true, clientproto.RPCRandomEventDoAffair.String(), "basic.map_event", "claim", "地图随机事件可处理", 5960, id)
				break
			}
		}
	}
	ops = append(ops, zooOperations(s, basic.GetZoo(), now)...)
	if basic.GetMailEnabled() {
		if !s.MailObserved() {
			add(true, clientproto.RPCMailGetList.String(), "basic.mail", "sync", "邮件列表未同步，先获取列表", 5700, 0)
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
		add(true, clientproto.RPCSignTypeSign.String(), "basic.sign", "claim", "签到由调度退避控制", 5600, 1)
	}
	ops = append(ops, pearlOperations(s, basic.GetPearl(), now)...)
	return ops
}

func storyOperations(s *state.State) []PlannedOp {
	goal := Goal{ID: "basic.story", Category: CategoryBasic, Domain: "basic.story", Label: "剧情", Priority: 61}
	if !s.StoryMainObserved() {
		return []PlannedOp{domainOp(clientproto.RPCStoryMainEnter.String(), goal, "basic.story", "sync", "主线剧情未同步，先进入剧情模块", 6140, 0, 0, 0)}
	}
	story, ok := s.StoryMain()
	if !ok || story.SectionID <= 0 {
		blocked := markerOp(CategoryBasic, "basic.story", "unlock", "主线剧情当前小节未识别，暂不自动解锁", 6130)
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.BlockedReasons = []string{"无法从 c_storyMainChapter/c_storyMainSection 匹配当前章节小节"}
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
	planned.ItemCost = itemCost
	return []PlannedOp{planned}
}

func waterClaimAllowed(s *state.State, basic *pb.BasicPolicy, now time.Time) bool {
	if s == nil {
		return false
	}
	waterDrops, total, _ := s.AvailableWaterDrops(now)
	if total > 0 && waterDrops >= total {
		return false
	}
	if threshold := basic.GetWaterClaimThreshold(); threshold > 0 && waterDrops >= threshold {
		return false
	}
	return true
}

func zooOperations(s *state.State, policy *pb.ZooPolicy, now time.Time) []PlannedOp {
	if policy == nil || !policy.GetEnabled() {
		return nil
	}
	goal := Goal{ID: "basic.zoo", Category: CategoryBasic, Domain: "basic.zoo", Label: "宠物", Priority: 57}
	var ops []PlannedOp
	if !s.ZooObserved() {
		return []PlannedOp{domainOp(clientproto.RPCZooEnterZoo.String(), goal, "basic.zoo", "sync", "宠物状态未同步，先进入宠物模块", 5690, 0, 0, 0)}
	}
	for _, petID := range s.ReadyZooStatusRefreshPetIDs(now) {
		return []PlannedOp{domainOp(clientproto.RPCZooRefreshPetStatus.String(), goal, "basic.zoo", "refresh", "宠物状态冷却已到期，先刷新服务端状态", 5685, petID, 0, 0)}
	}
	if policy.GetAutoFeed() {
		if food, ok := s.NextZooFoodstuffPlan(); ok {
			stock := domainOp(clientproto.RPCZooAddFoodstuff.String(), goal, "basic.zoo.feed", "stock", "使用已有库存自动补充宠物食盆", 5680, food.PetID, food.FoodstuffID, food.Count)
			stock.ItemCost = map[int32]int32{food.FoodstuffID: food.Count}
			ops = append(ops, stock)
		}
	}
	if policy.GetAutoStroke() {
		for _, petID := range s.ReadyZooStrokePetIDs(now) {
			ops = append(ops, domainOp(clientproto.RPCZooStrokePet.String(), goal, "basic.zoo.stroke", "stroke", "宠物当前可互动且心情未满", 5670, petID, 0, 0))
			break
		}
	}
	if policy.GetAutoEventEnabled() {
		if !s.ZooLogsObserved() {
			reason := "宠物服务端日志尚未同步，不使用宠物字段猜测事件"
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
	if policy.GetAutoBuyFood() {
		blocked := markerOp(CategoryBasic, "basic.zoo.buy_food", "buy", "购买猫粮涉及成本和商品选择，暂不自动执行", 5660)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"猫粮购买成本和商品选择尚未放开自动执行"}
		ops = append(ops, blocked)
	}
	return ops
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
		hire := markerOp(CategoryBasic, "basic.pearl.hire", "hire", "珍珠雇佣需要候选用户与成本确认", 120)
		hire.Label = "雇佣劳工"
		hire.Status = PlanStatusAdapterMissing
		hire.Executable = false
		hire.BlockedReasons = []string{"自动雇佣需要好友/推荐 UID、雇佣券消耗与等级过滤的协议确认"}
		ops = append(ops, hire)
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
