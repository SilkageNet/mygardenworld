package automation

import (
	"fmt"
	"strconv"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
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
					if flowerOrder == nil || flowerOrder.IsVideo != 0 {
						continue
					}
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
						reason = withOrderReason(reason, FormatFlowerRequires(flowerOrder.Requires))
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
			satin := s.ResidentSatinOrder()
			reason, limited := residentSatinDailyLimitReached(s, resident, now)
			switch {
			case limited:
				blocked := markerOp(CategoryOrder, "order.resident.satin", "finish", "绸缎居民订单今日上限已达", goal.Priority*100+695)
				blocked.GoalID = goal.ID
				blocked.Status = PlanStatusBlocked
				blocked.Executable = false
				blocked.BlockedReasons = []string{reason}
				ops = append(ops, blocked)
			case !satin.Observed:
				blocked := markerOp(CategoryOrder, "order.resident.satin", "finish", "绸缎居民订单未同步", goal.Priority*100+125)
				blocked.GoalID = goal.ID
				blocked.Status = PlanStatusBlocked
				blocked.Executable = false
				blocked.BlockedReasons = []string{"绸缎居民订单状态未观察到"}
				ops = append(ops, blocked)
			case satin.IsVideo != 0:
				blocked := markerOp(CategoryOrder, "order.resident.satin", "finish", "绸缎居民订单需看广告", goal.Priority*100+125)
				blocked.GoalID = goal.ID
				blocked.Status = PlanStatusBlocked
				blocked.Executable = false
				blocked.BlockedReasons = []string{"当前绸缎订单为广告订单，暂不自动提交；请自行看广告，看完或重启后会重新同步并继续"}
				ops = append(ops, blocked)
			case len(satin.Requires) == 0:
				ops = append(ops, blockedResidentSpecialOrderOp(clientproto.RPCOrderFlowerFinishSatinOrder.String(), "order.resident.satin", "绸缎居民订单", satin, goal, "绸缎居民订单缺少可识别需求"))
			case !satin.CooldownReady(now):
				// Wait for cooldown; no blocked marker needed.
			case !residentSpecialOrderAllowed(satin, resident):
				ops = append(ops, blockedResidentSpecialOrderOp(clientproto.RPCOrderFlowerFinishSatinOrder.String(), "order.resident.satin", "绸缎居民订单", satin, goal, "绸缎居民订单品质不符合策略"))
			case canFulfillResidentSpecialOrder(satin, "satin", goal, ledger):
				finish := op(clientproto.RPCOrderFlowerFinishSatinOrder.String(), goal, "finish", withOrderReason("绸缎居民订单可交付", FormatFlowerRequires(satin.Requires)), goal.Priority*100+710, 0, 0, 0)
				finish.Domain = "order.resident.satin"
				ops = append(ops, finish)
			}
		}
		if resident.GetDecorateEnabled() {
			decorate := s.ResidentDecorateOrder()
			reason, limited := residentDecorateDailyLimitReached(s, resident, now)
			switch {
			case limited:
				blocked := markerOp(CategoryOrder, "order.resident.decorate", "finish", "建材居民订单今日上限已达", goal.Priority*100+694)
				blocked.GoalID = goal.ID
				blocked.Status = PlanStatusBlocked
				blocked.Executable = false
				blocked.BlockedReasons = []string{reason}
				ops = append(ops, blocked)
			case !decorate.Observed:
				blocked := markerOp(CategoryOrder, "order.resident.decorate", "finish", "建材居民订单未同步", goal.Priority*100+124)
				blocked.GoalID = goal.ID
				blocked.Status = PlanStatusBlocked
				blocked.Executable = false
				blocked.BlockedReasons = []string{"建材居民订单状态未观察到"}
				ops = append(ops, blocked)
			case decorate.IsVideo != 0:
				blocked := markerOp(CategoryOrder, "order.resident.decorate", "finish", "建材居民订单需看广告", goal.Priority*100+124)
				blocked.GoalID = goal.ID
				blocked.Status = PlanStatusBlocked
				blocked.Executable = false
				blocked.BlockedReasons = []string{"当前建材订单为广告订单，暂不自动提交；请自行看广告，看完或重启后会重新同步并继续"}
				ops = append(ops, blocked)
			case len(decorate.Requires) == 0:
				ops = append(ops, blockedResidentSpecialOrderOp(clientproto.RPCOrderFlowerFinishDecorateOrder.String(), "order.resident.decorate", "建材居民订单", decorate, goal, "建材居民订单缺少可识别需求"))
			case !decorate.CooldownReady(now):
				// Wait for cooldown; no blocked marker needed.
			case !residentSpecialOrderAllowed(decorate, resident):
				ops = append(ops, blockedResidentSpecialOrderOp(clientproto.RPCOrderFlowerFinishDecorateOrder.String(), "order.resident.decorate", "建材居民订单", decorate, goal, "建材居民订单品质不符合策略"))
			case canFulfillResidentSpecialOrder(decorate, "decorate", goal, ledger):
				finish := op(clientproto.RPCOrderFlowerFinishDecorateOrder.String(), goal, "finish", withOrderReason("建材居民订单可交付", FormatFlowerRequires(decorate.Requires)), goal.Priority*100+705, 0, 0, 0)
				finish.Domain = "order.resident.decorate"
				ops = append(ops, finish)
			}
		}
	}
	if goal, ok := goalByID(goals, GoalCustomerOrder); ok {
		customer := order.GetCustomer()
		if blocked, ok := customerOrderLimitBlock(s, customer, goal, now); ok {
			ops = append(ops, blocked)
		} else {
			bypassMinArt := RaceHoldsUnfinishedCustomerOrder(s.FmlRace())
			for npcID, customerOrder := range s.CustomerOrderDetails() {
				reqSummary := FormatCustomerOrderRequires(s, customerOrder)
				if !customerOrderMeetsMinFlowerArt(customerOrder, customer, bypassMinArt) {
					artCount := customerOrderFlowerArtCount(customerOrder)
					minArt := customer.GetMinFlowerArtCount()
					reject := op(clientproto.RPCOrderCustomerRejectOrder.String(), goal, "reject",
						withOrderReason(fmt.Sprintf("顾客订单花艺件数 %d < 最低要求 %d，执行拒绝", artCount, minArt), reqSummary),
						orderSchedulePriority(orderStageCustomerReject), npcID, 0, 0)
					ops = append(ops, reject)
					continue
				}
				// Priority: inventory finish → craft when materials ready → reject.
				if canFulfillCustomerOrder(customerOrder, npcID, goal, ledger) {
					ops = append(ops, op(clientproto.RPCOrderCustomerFinishOrder.String(), goal, "finish", withOrderReason("顾客订单可交付", reqSummary), orderSchedulePriority(orderStageCustomerFinish), npcID, 0, 0))
					continue
				}
				if craft, ok := craftOperationForCustomerOrder(s, customerOrder, npcID, goal, demands, ledger); ok && craft.Executable {
					craft.Reason = withOrderReason(craft.Reason, reqSummary)
					ops = append(ops, craft)
					continue
				}
				if customer.GetRejectUnavailableEnabled() {
					reject := op(clientproto.RPCOrderCustomerRejectOrder.String(), goal, "reject", withOrderReason("顾客订单库存不足且无法制作，执行暂时无货", reqSummary), orderSchedulePriority(orderStageCustomerReject), npcID, 0, 0)
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
				ops = append(ops, op(clientproto.RPCOrderCustomerGenOrder.String(), goal, "generate", "顾客订单为空且刷新时间已到，生成顾客订单", orderSchedulePriority(orderStageCustomerGenerate), 0, 0, 0))
			}
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
			claimable := s.FlowerRackClaimableSlotIDs(now)
			if len(claimable) > 0 {
				rackID := claimable[0]
				claim := op(clientproto.RPCFlowerRackRecvSellMoney.String(), goal, "claim", "花架售卖时间已到，可领取收益", orderSchedulePriority(orderStageFlowerRackClaim), rackID, 0, 0)
				if slot, ok := slots[rackID]; ok {
					claim.ItemID = slot.ItemID
					claim.Count = slot.Count
				}
				claim.Category = CategoryOrder
				ops = append(ops, claim)
			}
			if flowerArtAutoListActive(order.GetFlowerArt(), now) {
				for _, rackID := range s.EmptyFlowerRackSlotIDs() {
					if artID, count, ok := bestRackArt(s, order.GetFlowerArt(), ledger); ok {
						sell := op(clientproto.RPCFlowerRackSell.String(), goal, "sell", "花架空位可上架未预留花艺", orderSchedulePriority(orderStageFlowerRackSell), rackID, artID, count)
						sell.Category = CategoryOrder
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
		if req.ItemID == 0 || req.Count <= 0 {
			continue
		}
		have := int32(0)
		if ledger != nil {
			have = ledger.Owned(req.ItemID)
		}
		missing := req.Count - have
		if missing <= 0 {
			continue
		}
		allocated := allocatedCraftFlowerCounts(goal, entityID, req.ItemID, demands)
		availability := FlowerArtAvailabilityWithAllocated(s, req.ItemID, missing, ledger, allocated)
		if !availability.Craftable {
			blocked := op(clientproto.RPCFlowerArtMakeFlowerArt.String(), goal, "craft", "顾客订单花艺暂不可制作", goal.Priority*100+500, npcID, req.ItemID, missing)
			blocked.Status = PlanStatusBlocked
			blocked.Executable = false
			blocked.VaseID = availability.Recipe.VaseID
			blocked.FlowerIDs = append([]int32(nil), availability.Recipe.Flowers...)
			blocked.BlockedReasons = append([]string(nil), availability.BlockedReasons...)
			return blocked, true
		}
		craft := op(clientproto.RPCFlowerArtMakeFlowerArt.String(), goal, "craft", "顾客订单缺少花艺成品，材料已满足", orderSchedulePriority(orderStageCustomerCraft), npcID, req.ItemID, missing)
		craft.VaseID = availability.Recipe.VaseID
		craft.FlowerIDs = append([]int32(nil), availability.Recipe.Flowers...)
		craft.ItemCost = flowerArtCraftItemCost(craft.FlowerIDs, craft.Count)
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

// orderScheduleStage models product ordering between customer and flower-rack
// operations. It replaces cross-goal arithmetic: customer delivery remains
// first, while a ready rack can make progress before recurring generation,
// rejection, or craft work. Rack craft consumes only ledger-unreserved stock.
type orderScheduleStage int32

const (
	orderStageCustomerCraft orderScheduleStage = iota + 1
	orderStageCustomerReject
	orderStageCustomerGenerate
	orderStageFlowerRackCraft
	orderStageFlowerRackSell
	orderStageFlowerRackClaim
	orderStageCustomerFinish
)

func orderSchedulePriority(stage orderScheduleStage) int32 {
	const (
		orderSchedulePriorityBase int32 = 11000
		orderScheduleStageWidth   int32 = 100
	)
	return orderSchedulePriorityBase + int32(stage)*orderScheduleStageWidth
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
	craft := op(clientproto.RPCFlowerArtMakeFlowerArt.String(), goal, "craft", "花架缺少花艺成品，材料已满足", orderSchedulePriority(orderStageFlowerRackCraft), 0, artID, count)
	craft.VaseID = availability.Recipe.VaseID
	craft.FlowerIDs = append([]int32(nil), availability.Recipe.Flowers...)
	craft.ItemCost = flowerArtCraftItemCost(craft.FlowerIDs, craft.Count)
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
