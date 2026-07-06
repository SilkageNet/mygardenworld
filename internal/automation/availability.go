package automation

import (
	"fmt"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"strconv"
	"time"
)

func FlowerArtAvailability(s *state.State, artID, count int32, ledger *InventoryLedger) ArtAvailability {
	return FlowerArtAvailabilityWithAllocated(s, artID, count, ledger, nil)
}

func FlowerArtAvailabilityWithAllocated(s *state.State, artID, count int32, ledger *InventoryLedger, allocated map[int32]int32) ArtAvailability {
	recipe, ok := state.FlowerArtRecipeByID(artID)
	if !ok {
		return ArtAvailability{BlockedReasons: []string{"缺少花艺配方"}}
	}
	availability := ArtAvailability{Recipe: recipe}
	availability.VaseUnlocked = s.HasVase(recipe.VaseID)
	if !s.VaseObserved() {
		availability.BlockedReasons = append(availability.BlockedReasons, "未观察到花瓶状态 namespace 102")
	} else if !availability.VaseUnlocked {
		availability.BlockedReasons = append(availability.BlockedReasons, fmt.Sprintf("花瓶 #%d 未解锁", recipe.VaseID))
	}
	if ledger == nil {
		ledger = NewInventoryLedger(s.Inventory())
	}
	for flowerID, needEach := range recipeFlowerCounts(recipe) {
		required := needEach * count
		have := ledger.Owned(flowerID)
		available := ledger.Available(flowerID) + allocated[flowerID]
		missing := required - available
		if missing < 0 {
			missing = 0
		}
		demand := Demand{
			ID:        demandID("flower_art.availability", strconv.FormatInt(int64(artID), 10), "availability", DemandKindFlower, flowerID),
			Kind:      DemandKindFlower,
			ItemID:    flowerID,
			Count:     required,
			Have:      have,
			Available: available,
			Missing:   missing,
			Label:     fmt.Sprintf("制作 %s", itemLabel(artID)),
			Priority:  0,
		}
		if missing > 0 {
			demand.BlockedReasons = append(demand.BlockedReasons, "花朵库存不足")
			availability.BlockedReasons = append(availability.BlockedReasons, fmt.Sprintf("%s 缺少 %d", itemLabel(flowerID), missing))
		}
		if !flowerCultivated(s, flowerID) {
			demand.BlockedReasons = append(demand.BlockedReasons, "花朵尚未培育/解锁")
			availability.BlockedReasons = append(availability.BlockedReasons, fmt.Sprintf("%s 尚未培育/解锁", itemLabel(flowerID)))
		}
		availability.Requirements = append(availability.Requirements, demand)
	}
	availability.Craftable = len(availability.BlockedReasons) == 0
	return availability
}

func artBlockedReasons(s *state.State, recipe state.FlowerArtRecipe) []string {
	var blocked []string
	if !s.VaseObserved() {
		blocked = append(blocked, "未观察到花瓶状态 namespace 102")
	} else if !s.HasVase(recipe.VaseID) {
		blocked = append(blocked, fmt.Sprintf("花瓶 #%d 未解锁", recipe.VaseID))
	}
	for flowerID := range recipeFlowerCounts(recipe) {
		if !flowerCultivated(s, flowerID) {
			blocked = append(blocked, fmt.Sprintf("%s 尚未培育/解锁", itemLabel(flowerID)))
		}
	}
	return blocked
}

func residentFlowerOrderAllowed(order *state.FlowerOrder, policy *pb.ResidentOrderPolicy) bool {
	if order == nil || len(order.Requires) == 0 {
		return false
	}
	qualities := int32Set(policy.GetQualities())
	if len(qualities) == 0 {
		return true
	}
	for _, req := range order.Requires {
		if req.FlowerID <= 0 {
			continue
		}
		if !qualities[flowerQuality(req.FlowerID)] {
			return false
		}
	}
	return true
}

func palaceOrderAllowed(order state.PalaceOrderView, policy *pb.PalaceOrderPolicy) bool {
	if !order.Observed || order.IsFinish != 0 || order.FlowerID <= 0 || order.Num <= 0 {
		return false
	}
	qualities := int32Set(policy.GetQualities())
	return len(qualities) == 0 || qualities[flowerQuality(order.FlowerID)]
}

func teamOrderAllowed(order state.TeamOrderView, policy *pb.TeamOrderPolicy, s *state.State) bool {
	if !order.Observed || order.FlowerID <= 0 || teamOrderNeedCount(order) <= 0 {
		return false
	}
	qualities := int32Set(policy.GetQualities())
	if len(qualities) > 0 && !qualities[flowerQuality(order.FlowerID)] {
		return false
	}
	if policy.GetSubmitOnlyCultivated() && !flowerCultivated(s, order.FlowerID) {
		return false
	}
	return true
}

func teamOrderNeedCount(order state.TeamOrderView) int32 {
	if order.RemainingNum > 0 {
		return order.RemainingNum
	}
	return order.OrderNum
}

func flowerCultivated(s *state.State, flowerID int32) bool {
	for id, cv := range s.Cultivations() {
		if id == flowerID && cv.Status == 2 && cv.Lvl > 0 {
			return true
		}
	}
	return false
}

func customerOrderUnavailableReasons(s *state.State, order *state.CustomerOrder) (rejectable, blocked []string) {
	if order == nil {
		return nil, []string{"顾客订单状态缺失"}
	}
	hasRequirements := false
	addReject := func(reason string) {
		if reason != "" && !containsString(rejectable, reason) {
			rejectable = append(rejectable, reason)
		}
	}
	addBlocked := func(reason string) {
		if reason != "" && !containsString(blocked, reason) {
			blocked = append(blocked, reason)
		}
	}
	for _, req := range order.Requires {
		if req.FlowerID <= 0 || req.Count <= 0 {
			addBlocked("顾客订单缺少可识别花朵需求")
			continue
		}
		hasRequirements = true
		if !flowerCultivated(s, req.FlowerID) {
			addReject(fmt.Sprintf("%s 尚未培育/解锁", itemLabel(req.FlowerID)))
		}
	}
	for _, req := range order.ItemRequires {
		if req.ItemID <= 0 || req.Count <= 0 {
			addBlocked("顾客订单缺少可识别花艺需求")
			continue
		}
		hasRequirements = true
		recipe, ok := state.FlowerArtRecipeByID(req.ItemID)
		if !ok {
			addBlocked(fmt.Sprintf("%s 缺少花艺配方", itemLabel(req.ItemID)))
			continue
		}
		if !s.VaseObserved() {
			addBlocked("未观察到花瓶状态 namespace 102")
		} else if !s.HasVase(recipe.VaseID) {
			addReject(fmt.Sprintf("花瓶 #%d 未解锁", recipe.VaseID))
		}
		for flowerID := range recipeFlowerCounts(recipe) {
			if !flowerCultivated(s, flowerID) {
				addReject(fmt.Sprintf("%s 尚未培育/解锁", itemLabel(flowerID)))
			}
		}
	}
	if !hasRequirements {
		addBlocked("顾客订单缺少可识别需求")
	}
	return rejectable, blocked
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func syncOnlyOperation(op PlannedOp, reasons ...string) PlannedOp {
	op.Status = PlanStatusSyncOnly
	op.SyncOnly = true
	op.Executable = false
	op.BlockedReasons = append(op.BlockedReasons, reasons...)
	op.BlockingStage = inferBlockingStage(op.BlockedReasons)
	return op
}

func residentOrderLimitBlock(s *state.State, policy *pb.ResidentOrderPolicy, goal Goal, now time.Time) (PlannedOp, bool) {
	if until, ok := s.ResidentOrderDailyLimitReached(now); ok {
		blocked := markerOp(CategoryOrder, "order.resident", "finish", "居民订单今日上限已达", goal.Priority*100+690)
		blocked.GoalID = goal.ID
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.BlockedReasons = []string{fmt.Sprintf("服务端提示今日完成订单次数已达上限，预计 %s 后再试", until.Format("01/02 15:04"))}
		return blocked, true
	}
	limit := policy.GetNormalDailyLimit()
	if limit <= 0 {
		blocked := markerOp(CategoryOrder, "order.resident", "finish", "居民订单普通订单上限未设置", goal.Priority*100+690)
		blocked.GoalID = goal.ID
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.BlockedReasons = []string{"普通居民订单上限必须大于 0"}
		return blocked, true
	}
	if hardLimit := state.ResidentOrderNormalDailyMax(); hardLimit > 0 && hardLimit < limit {
		limit = hardLimit
	}
	stats := s.Statistics()
	if stats.Observed && stats.OrderFlowerFinishNum >= limit {
		blocked := markerOp(CategoryOrder, "order.resident", "finish", "居民订单今日上限已达", goal.Priority*100+690)
		blocked.GoalID = goal.ID
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.BlockedReasons = []string{fmt.Sprintf("普通居民订单今日已完成 %d/%d", stats.OrderFlowerFinishNum, limit)}
		return blocked, true
	}
	return PlannedOp{}, false
}

func residentSpecialOrderBlockedOp(domain, label string, order state.ResidentSpecialOrder, finished, limit int32, goal Goal) PlannedOp {
	blocked := markerOp(CategoryOrder, domain, "finish", label+"暂不执行", goal.Priority*100+130)
	blocked.GoalID = goal.ID
	blocked.Status = PlanStatusAdapterMissing
	blocked.Executable = false
	switch {
	case limit <= 0:
		blocked.BlockedReasons = []string{label + "日上限必须大于 0"}
		blocked.Status = PlanStatusBlocked
	case finished >= limit:
		blocked.BlockedReasons = []string{fmt.Sprintf("%s今日已完成 %d/%d", label, finished, limit)}
		blocked.Status = PlanStatusBlocked
	case !order.Observed:
		blocked.BlockedReasons = []string{label + "状态未观察到，无法确认需求和次数"}
	default:
		blocked.BlockedReasons = []string{label + "已观察到状态，但协议仅暴露聚合 flowers 字段，缺少可安全提交的花朵需求列表"}
	}
	return blocked
}

func blockedResidentOrderOp(order *state.FlowerOrder, boxID int32, goal Goal, reason string) PlannedOp {
	blocked := op(clientproto.RPCOrderFlowerFinishOrder.String(), goal, "finish", reason, goal.Priority*100+120, boxID, 0, 0)
	blocked.Status = PlanStatusBlocked
	blocked.Executable = false
	if order == nil || len(order.Requires) == 0 {
		blocked.BlockedReasons = []string{"居民订单缺少可识别需求"}
		return blocked
	}
	var reasons []string
	for _, req := range order.Requires {
		quality := flowerQuality(req.FlowerID)
		reasons = append(reasons, fmt.Sprintf("%s 品质 %d 不在策略范围内", itemLabel(req.FlowerID), quality))
	}
	blocked.BlockedReasons = reasons
	return blocked
}

func blockedPalaceOrderOp(order state.PalaceOrderView, goal Goal, policy *pb.PalaceOrderPolicy) PlannedOp {
	blocked := op(clientproto.RPCOrderPalaceFinishOrder.String(), goal, "finish", "宫廷订单暂不可提交", goal.Priority*100+120, 0, order.FlowerID, order.Num)
	blocked.Status = PlanStatusBlocked
	blocked.Executable = false
	blocked.BlockedReasons = orderPolicyBlockedReasons(order.Observed, order.FlowerID, order.Num, policy.GetQualities(), false)
	if len(blocked.BlockedReasons) == 0 {
		blocked.BlockedReasons = []string{"宫廷订单状态显示已完成或缺少可识别需求"}
	}
	return blocked
}

func blockedTeamOrderOp(order state.TeamOrderView, goal Goal, policy *pb.TeamOrderPolicy, s *state.State) PlannedOp {
	count := teamOrderNeedCount(order)
	blocked := op(clientproto.RPCOrderTeamSubmitOrder.String(), goal, "submit", "组团订单暂不可提交", goal.Priority*100+120, 0, order.FlowerID, count)
	blocked.Status = PlanStatusBlocked
	blocked.Executable = false
	blocked.BlockedReasons = orderPolicyBlockedReasons(order.Observed, order.FlowerID, count, policy.GetQualities(), policy.GetSubmitOnlyCultivated() && !flowerCultivated(s, order.FlowerID))
	if len(blocked.BlockedReasons) == 0 {
		blocked.BlockedReasons = []string{"组团订单缺少可识别需求"}
	}
	return blocked
}

func orderPolicyBlockedReasons(observed bool, flowerID, count int32, qualities []int32, notCultivated bool) []string {
	var reasons []string
	if !observed {
		reasons = append(reasons, "订单状态未观察到")
	}
	if flowerID <= 0 || count <= 0 {
		reasons = append(reasons, "订单缺少可识别花朵需求")
	}
	if len(qualities) > 0 && flowerID > 0 && !int32Set(qualities)[flowerQuality(flowerID)] {
		reasons = append(reasons, fmt.Sprintf("%s 品质 %d 不在策略范围内", itemLabel(flowerID), flowerQuality(flowerID)))
	}
	if notCultivated {
		reasons = append(reasons, fmt.Sprintf("%s 未达到已培育可提交条件", itemLabel(flowerID)))
	}
	return reasons
}

func canFulfillSingleFlowerDemand(goal Goal, entityID string, flowerID, count int32, ledger *InventoryLedger) bool {
	if flowerID <= 0 || count <= 0 {
		return false
	}
	id := demandID(goal.ID, entityID, "direct", DemandKindFlower, flowerID)
	return ledger.AllocatedForDemand(id, flowerID) >= count
}

func recipeFlowerCounts(recipe state.FlowerArtRecipe) map[int32]int32 {
	out := make(map[int32]int32)
	for _, flowerID := range recipe.Flowers {
		if flowerID > 0 {
			out[flowerID]++
		}
	}
	return out
}

func maxCraftableCount(recipe state.FlowerArtRecipe, ledger *InventoryLedger) int32 {
	if recipe.ArtID <= 0 || ledger == nil {
		return 0
	}
	var max int32
	for flowerID, needEach := range recipeFlowerCounts(recipe) {
		if needEach <= 0 {
			continue
		}
		available := ledger.Available(flowerID) / needEach
		if max == 0 || available < max {
			max = available
		}
	}
	return max
}

func bestRackArt(ledger *InventoryLedger) (int32, int32, bool) {
	if ledger == nil {
		return 0, 0, false
	}
	for _, recipe := range rackCandidateRecipes() {
		available := ledger.Available(recipe.ArtID)
		if available <= 0 {
			continue
		}
		if available > flowerRackPerSlotCount {
			available = flowerRackPerSlotCount
		}
		return recipe.ArtID, available, true
	}
	return 0, 0, false
}

func rackCraftTarget(s *state.State, policy *pb.FlowerArtPolicy, ledger *InventoryLedger) (int32, int32, bool) {
	if policy == nil || !policy.GetSellEnabled() || !policy.GetCraftEnabled() {
		return 0, 0, false
	}
	if len(s.EmptyFlowerRackSlotIDs()) == 0 {
		return 0, 0, false
	}
	if ledger == nil {
		ledger = NewInventoryLedger(s.Inventory())
	}
	for _, recipe := range rackCandidateRecipes() {
		if ledger.Available(recipe.ArtID) > 0 {
			return 0, 0, false
		}
	}
	for _, recipe := range rackCandidateRecipes() {
		if len(artBlockedReasons(s, recipe)) > 0 {
			continue
		}
		count := maxCraftableCount(recipe, ledger)
		if count <= 0 {
			continue
		}
		if count > flowerRackPerSlotCount {
			count = flowerRackPerSlotCount
		}
		if count > 0 {
			return recipe.ArtID, count, true
		}
	}
	return 0, 0, false
}

func rackCandidateRecipes() []state.FlowerArtRecipe {
	return state.AllFlowerArtRecipes()
}

func canFulfillFlowerOrder(order *state.FlowerOrder, boxID int32, goal Goal, ledger *InventoryLedger) bool {
	if order == nil || len(order.Requires) == 0 {
		return false
	}
	entityID := strconv.FormatInt(int64(boxID), 10)
	for _, req := range order.Requires {
		id := demandID(goal.ID, entityID, "direct", DemandKindFlower, req.FlowerID)
		if req.FlowerID == 0 || req.Count <= 0 || ledger.AllocatedForDemand(id, req.FlowerID) < req.Count {
			return false
		}
	}
	return true
}

func canFulfillCustomerOrder(order *state.CustomerOrder, npcID int32, goal Goal, ledger *InventoryLedger) bool {
	if order == nil {
		return false
	}
	hasRequirements := false
	entityID := strconv.FormatInt(int64(npcID), 10)
	for _, req := range order.Requires {
		if req.FlowerID == 0 || req.Count <= 0 {
			continue
		}
		hasRequirements = true
		id := demandID(goal.ID, entityID, "direct", DemandKindFlower, req.FlowerID)
		if ledger.AllocatedForDemand(id, req.FlowerID) < req.Count {
			return false
		}
	}
	for _, req := range order.ItemRequires {
		if req.ItemID == 0 || req.Count <= 0 {
			continue
		}
		hasRequirements = true
		id := demandID(goal.ID, entityID, "direct", DemandKindFlowerArt, req.ItemID)
		if ledger.AllocatedForDemand(id, req.ItemID) < req.Count {
			return false
		}
	}
	return hasRequirements
}
