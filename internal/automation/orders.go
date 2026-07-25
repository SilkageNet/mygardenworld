package automation

import (
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"strconv"
	"time"
)

func orderOperations(s *state.State, policy *pb.Policy, goals []Goal, demands []Demand, ledger *InventoryLedger, now time.Time) []PlannedOp {
	var ops []PlannedOp
	order := policy.GetOrder()
	if goal, ok := goalByID(goals, GoalResidentOrder); ok {
		resident := order.GetResident()
		if resident.GetNormalEnabled() {
			if blocked, ok := residentOrderLimitBlock(s, resident, goal, now); ok {
				ops = append(ops, blocked)
			} else {
				statsObserved := s.Statistics().Observed
				for boxID, flowerOrder := range s.FlowerOrders() {
					if !residentFlowerOrderAllowed(flowerOrder, resident) {
						ops = append(ops, blockedResidentOrderOp(flowerOrder, boxID, goal, "居民订单品质不符合策略"))
						continue
					}
					if !flowerOrder.CooldownReady(now) {
						continue
					}
					if canFulfillFlowerOrder(flowerOrder, boxID, goal, ledger) {
						reason := "居民订单可交付"
						if !statsObserved {
							reason = "居民订单可交付；未观察到今日统计 namespace 124"
						}
						ops = append(ops, op(clientproto.RPCOrderFlowerFinishOrder.String(), goal, "finish", reason, goal.Priority*100+700, boxID, 0, 0))
					}
				}
			}
		}
		if resident.GetRewardEnabled() {
			for _, target := range s.ReadyFlowerOrderRewardTargets() {
				ops = append(ops, op(clientproto.RPCOrderFlowerRecvOrderRwd.String(), goal, "reward", "居民订单阶段奖励可领取", goal.Priority*100+620, target, 0, 0))
			}
		}
		if resident.GetSatinEnabled() {
			ops = append(ops, residentSpecialOrderBlockedOp("order.resident.satin", "绸缎居民订单", s.ResidentSatinOrder(), s.Statistics().OrderSatinFinishNum, resident.GetSatinDailyLimit(), goal))
		}
		if resident.GetDecorateEnabled() {
			ops = append(ops, residentSpecialOrderBlockedOp("order.resident.decorate", "建材居民订单", s.ResidentDecorateOrder(), s.Statistics().OrderDecorateFinishNum, resident.GetDecorateDailyLimit(), goal))
		}
	}
	if goal, ok := goalByID(goals, GoalCustomerOrder); ok {
		for npcID, customerOrder := range s.CustomerOrderDetails() {
			reqSummary := FormatCustomerOrderRequires(s, customerOrder)
			if canFulfillCustomerOrder(customerOrder, npcID, goal, ledger) {
				ops = append(ops, op(clientproto.RPCOrderCustomerFinishOrder.String(), goal, "finish", withOrderReason("顾客订单可交付", reqSummary), customerOperationPriority(goal, 200), npcID, 0, 0))
				continue
			}
			if craft, ok := craftOperationForCustomerOrder(s, customerOrder, npcID, goal, demands, ledger); ok && craft.Executable {
				craft.Reason = withOrderReason(craft.Reason, reqSummary)
				ops = append(ops, craft)
				continue
			}
			if order.GetCustomer().GetRejectUnavailableEnabled() {
				reject := op(clientproto.RPCOrderCustomerRejectOrder.String(), goal, "reject", withOrderReason("顾客订单库存不足且无法制作，执行暂时无货", reqSummary), customerOperationPriority(goal, 180), npcID, 0, 0)
				ops = append(ops, reject)
				continue
			}
			blocked := op(clientproto.RPCOrderCustomerRejectOrder.String(), goal, "reject", withOrderReason("顾客订单库存不足且无法制作，等待策略允许暂时无货", reqSummary), goal.Priority*100+130, npcID, 0, 0)
			blocked.Status = PlanStatusBlocked
			blocked.Executable = false
			blocked.BlockedReasons = []string{"order.customer.reject_unavailable_enabled 未开启", "库存不足且无法制作"}
			ops = append(ops, blocked)
		}
		if s.CustomerOrderGenerationReady(now) {
			ops = append(ops, op(clientproto.RPCOrderCustomerGenOrder.String(), goal, "generate", "顾客订单为空且刷新时间已到，生成顾客订单", customerOperationPriority(goal, 190), 0, 0, 0))
		}
	}
	if goal, ok := goalByID(goals, GoalPalaceOrder); ok {
		palace := order.GetPalace()
		palaceOrder := s.PalaceOrder()
		switch {
		case !palaceOrder.Observed:
			ops = append(ops, syncOnlyOperation(domainOp(clientproto.RPCOrderPalaceEnter.String(), goal, "order.palace", "sync", "宫廷订单未同步，保留同步提示", goal.Priority*100+580, 0, 0, 0), "宫廷订单本轮保持 sync_only，不自动执行 RPC"))
		case palaceOrder.IsFinish != 0:
		case !palaceOrderAllowed(palaceOrder, palace):
			ops = append(ops, blockedPalaceOrderOp(palaceOrder, goal, palace))
		case canFulfillSingleFlowerDemand(goal, "current", palaceOrder.FlowerID, palaceOrder.Num, ledger):
			ops = append(ops, syncOnlyOperation(op(clientproto.RPCOrderPalaceFinishOrder.String(), goal, "finish", "宫廷订单可交付但执行化暂停", goal.Priority*100+570, 0, palaceOrder.FlowerID, palaceOrder.Num), "宫廷订单本轮保持 sync_only，不自动提交"))
		}
	}
	if goal, ok := goalByID(goals, GoalTeamOrder); ok {
		team := order.GetTeam()
		teamOrder := s.TeamOrder()
		switch {
		case !teamOrder.Observed:
			ops = append(ops, syncOnlyOperation(domainOp(clientproto.RPCOrderTeamRefreshOrder.String(), goal, "order.team", "sync", "组团订单未同步，保留同步提示", goal.Priority*100+560, 0, 0, 0), "组团订单本轮保持 sync_only，不自动执行 RPC"))
		case !teamOrderAllowed(teamOrder, team, s):
			ops = append(ops, blockedTeamOrderOp(teamOrder, goal, team, s))
		case canFulfillSingleFlowerDemand(goal, "current", teamOrder.FlowerID, teamOrderNeedCount(teamOrder), ledger):
			ops = append(ops, syncOnlyOperation(op(clientproto.RPCOrderTeamSubmitOrder.String(), goal, "submit", "组团订单可提交但执行化暂停", goal.Priority*100+550, 0, teamOrder.FlowerID, teamOrderNeedCount(teamOrder)), "组团订单本轮保持 sync_only，不自动提交"))
		}
	}
	if goal, ok := goalByID(goals, GoalFlowerArt); ok {
		if order.GetFlowerArt().GetCreateRewardEnabled() {
			for _, vaseID := range s.ReadyArtCreateRewardVaseIDs() {
				claim := domainOp(clientproto.RPCCollectRwdRecvArtCreateRwdByVase.String(), goal, "order.flower_art.create_reward", "claim", "花艺制作经验奖励可领取", goal.Priority*100+670, vaseID, 0, 0)
				ops = append(ops, claim)
				break
			}
		}
		if order.GetFlowerArt().GetCollectRewardEnabled() {
			for _, typeID := range s.ReadyCollectRewardTypes(11, 12, 13) {
				claim := domainOp(clientproto.RPCCollectRwdRecv.String(), goal, "order.flower_art.collect_reward", "claim", "图鉴奖励可领取", goal.Priority*100+660, typeID, 0, 0)
				ops = append(ops, claim)
				break
			}
		}
		if order.GetFlowerArt().GetSellEnabled() {
			slots := s.FlowerRackSlots()
			for _, rackID := range s.FlowerRackClaimableSlotIDs(now) {
				claim := op(clientproto.RPCFlowerRackRecvSellMoney.String(), goal, "claim", "花架售卖时间已到，可领取收益", flowerRackClaimPriority(goal), rackID, 0, 0)
				if slot, ok := slots[rackID]; ok {
					claim.ItemID = slot.ItemID
					claim.Count = slot.Count
				}
				claim.Category = CategoryFlowerArt
				ops = append(ops, claim)
				break
			}
			if flowerArtAutoListActive(order.GetFlowerArt(), now) {
				for _, rackID := range s.EmptyFlowerRackSlotIDs() {
					if artID, count, ok := bestRackArt(ledger); ok {
						sell := op(clientproto.RPCFlowerRackSell.String(), goal, "sell", "花架空位可上架未预留花艺", goal.Priority*100+400, rackID, artID, count)
						sell.Category = CategoryFlowerArt
						ops = append(ops, sell)
						break
					}
					if craft, ok := craftOperationForFlowerRack(s, order.GetFlowerArt(), goal, ledger); ok {
						ops = append(ops, craft)
						break
					}
				}
			}
		}
	}
	return ops
}

func craftOperationForCustomerOrder(s *state.State, order *state.CustomerOrder, npcID int32, goal Goal, demands []Demand, ledger *InventoryLedger) (PlannedOp, bool) {
	if order == nil {
		return PlannedOp{}, false
	}
	entityID := strconv.FormatInt(int64(npcID), 10)
	for _, req := range order.ItemRequires {
		demand, ok := demandByID(demands, demandID(goal.ID, entityID, "direct", DemandKindFlowerArt, req.ItemID))
		if !ok || demand.Missing <= 0 {
			continue
		}
		allocated := allocatedCraftFlowerCounts(goal, entityID, req.ItemID, demands)
		availability := FlowerArtAvailabilityWithAllocated(s, req.ItemID, demand.Missing, ledger, allocated)
		if !availability.Craftable {
			blocked := op(clientproto.RPCFlowerArtMakeFlowerArt.String(), goal, "craft", "顾客订单花艺暂不可制作", goal.Priority*100+500, npcID, req.ItemID, demand.Missing)
			blocked.Status = PlanStatusBlocked
			blocked.Executable = false
			blocked.VaseID = availability.Recipe.VaseID
			blocked.FlowerIDs = append([]int32(nil), availability.Recipe.Flowers...)
			blocked.BlockedReasons = append([]string(nil), availability.BlockedReasons...)
			return blocked, true
		}
		craft := op(clientproto.RPCFlowerArtMakeFlowerArt.String(), goal, "craft", "顾客订单缺少花艺成品，材料已满足", customerOperationPriority(goal, 150), npcID, req.ItemID, demand.Missing)
		craft.VaseID = availability.Recipe.VaseID
		craft.FlowerIDs = append([]int32(nil), availability.Recipe.Flowers...)
		return craft, true
	}
	return PlannedOp{}, false
}

func allocatedCraftFlowerCounts(goal Goal, entityID string, artID int32, demands []Demand) map[int32]int32 {
	source := "craft:" + strconv.FormatInt(int64(artID), 10)
	var allocated map[int32]int32
	for _, demand := range demands {
		if demand.GoalID != goal.ID || demand.EntityID != entityID || demand.Source != source || demand.Kind != DemandKindFlower || demand.Allocated <= 0 {
			continue
		}
		if allocated == nil {
			allocated = make(map[int32]int32)
		}
		allocated[demand.ItemID] += demand.Allocated
	}
	return allocated
}

func customerOperationPriority(goal Goal, offset int32) int32 {
	return 11000 + goal.Priority + offset
}

func flowerRackClaimPriority(goal Goal) int32 {
	return 10500 + goal.Priority
}

func craftOperationForFlowerRack(s *state.State, policy *pb.FlowerArtPolicy, goal Goal, ledger *InventoryLedger) (PlannedOp, bool) {
	artID, count, ok := rackCraftTarget(s, policy, ledger)
	if !ok {
		return PlannedOp{}, false
	}
	availability := FlowerArtAvailability(s, artID, count, ledger)
	if !availability.Craftable {
		return PlannedOp{}, false
	}
	craft := op(clientproto.RPCFlowerArtMakeFlowerArt.String(), goal, "craft", "花架缺少花艺成品，材料已满足", goal.Priority*100+450, 0, artID, count)
	craft.VaseID = availability.Recipe.VaseID
	craft.FlowerIDs = append([]int32(nil), availability.Recipe.Flowers...)
	return craft, true
}

// flowerArtAutoListActive reports whether automatic flower-rack listing (and
// craft-for-list) should run. Requires sell_enabled; when sell_night_pause_enabled
// is also on, listing is skipped during 00:00-08:00 Asia/Shanghai.
func flowerArtAutoListActive(policy *pb.FlowerArtPolicy, now time.Time) bool {
	if policy == nil || !policy.GetSellEnabled() {
		return false
	}
	if !policy.GetSellNightPauseEnabled() {
		return true
	}
	return now.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Hour() >= 8
}
