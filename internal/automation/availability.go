package automation

import (
	"fmt"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"sort"
	"strconv"
	"strings"
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

// FormatCustomerOrderRequires builds a readable summary of direct flower needs
// and flower-art needs, including each art recipe's vase and flowers.
func FormatCustomerOrderRequires(s *state.State, order *state.CustomerOrder) string {
	if order == nil {
		return ""
	}
	var parts []string
	var flowerParts []string
	for _, req := range order.Requires {
		if req.FlowerID > 0 && req.Count > 0 {
			flowerParts = append(flowerParts, fmt.Sprintf("%s×%d", itemLabel(req.FlowerID), req.Count))
		}
	}
	if len(flowerParts) > 0 {
		parts = append(parts, "花: "+strings.Join(flowerParts, "、"))
	}
	var artParts []string
	for _, req := range order.ItemRequires {
		if req.ItemID <= 0 || req.Count <= 0 {
			continue
		}
		recipe, ok := state.FlowerArtRecipeByID(req.ItemID)
		if !ok {
			artParts = append(artParts, fmt.Sprintf("%s×%d", itemLabel(req.ItemID), req.Count))
			continue
		}
		var flowerNames []string
		for flowerID, count := range recipeFlowerCounts(recipe) {
			flowerNames = append(flowerNames, fmt.Sprintf("%s×%d", itemLabel(flowerID), count))
		}
		sort.Strings(flowerNames)
		art := fmt.Sprintf("%s×%d[花瓶:%s", itemLabel(req.ItemID), req.Count, itemLabel(recipe.VaseID))
		if len(flowerNames) > 0 {
			art += ":" + strings.Join(flowerNames, "/")
		}
		art += "]"
		artParts = append(artParts, art)
	}
	if len(artParts) > 0 {
		parts = append(parts, "花艺: "+strings.Join(artParts, "、"))
	}
	if len(parts) == 0 {
		return ""
	}
	return "需 " + strings.Join(parts, "； ")
}

// FormatFlowerRequires builds a short flower requirement summary for logs.
func FormatFlowerRequires(reqs []state.FlowerRequire) string {
	var parts []string
	for _, req := range reqs {
		if req.FlowerID > 0 && req.Count > 0 {
			parts = append(parts, fmt.Sprintf("%s×%d", itemLabel(req.FlowerID), req.Count))
		}
	}
	return strings.Join(parts, "、")
}

// FormatFlowerArtOpDesc formats a flower-art operation with art, vase, and recipe flowers.
func FormatFlowerArtOpDesc(artID, count int32) string {
	if artID <= 0 {
		return ""
	}
	recipe, ok := state.FlowerArtRecipeByID(artID)
	if !ok {
		return fmt.Sprintf("%s×%d", itemLabel(artID), count)
	}
	var flowerParts []string
	for flowerID, flowerCount := range recipeFlowerCounts(recipe) {
		flowerParts = append(flowerParts, fmt.Sprintf("%s×%d", itemLabel(flowerID), flowerCount))
	}
	sort.Strings(flowerParts)
	s := fmt.Sprintf("%s×%d[花瓶:%s", itemLabel(artID), count, itemLabel(recipe.VaseID))
	if len(flowerParts) > 0 {
		s += ":" + strings.Join(flowerParts, "/")
	}
	s += "]"
	return s
}

func withOrderReason(reason, summary string) string {
	if summary == "" {
		return reason
	}
	return reason + " (" + summary + ")"
}

func syncOnlyOperation(op PlannedOp, reasons ...string) PlannedOp {
	op.Status = PlanStatusSyncOnly
	op.SyncOnly = true
	op.Executable = false
	op.BlockedReasons = append(op.BlockedReasons, reasons...)
	op.BlockingStage = inferBlockingStage(op.BlockedReasons)
	return op
}

// residentNormalDailyLimit returns the effective ordinary resident-order cap:
// policy limit, optionally tightened by c_orderFlower.$dailyMax.
func residentNormalDailyLimit(policy *pb.ResidentOrderPolicy) int32 {
	if policy == nil {
		return 0
	}
	limit := policy.GetNormalDailyLimit()
	if limit <= 0 {
		return limit
	}
	if hardLimit := state.ResidentOrderNormalDailyMax(); hardLimit > 0 && hardLimit < limit {
		return hardLimit
	}
	return limit
}

// ResidentNormalDailyLimitReached reports whether ordinary resident finishes
// should stop for the policy daily cap (or a previously recorded server cap).
func ResidentNormalDailyLimitReached(s *state.State, policy *pb.ResidentOrderPolicy, now time.Time) (reason string, reached bool) {
	return residentNormalDailyLimitReached(s, policy, now)
}

// residentNormalDailyLimitReached reports whether ordinary resident finishes
// should stop for the policy daily cap (or a previously recorded server cap).
func residentNormalDailyLimitReached(s *state.State, policy *pb.ResidentOrderPolicy, now time.Time) (reason string, reached bool) {
	if until, ok := s.ResidentOrderDailyLimitReached(now); ok {
		return fmt.Sprintf("服务端提示今日完成订单次数已达上限，预计 %s 后再试", until.Format("01/02 15:04")), true
	}
	limit := residentNormalDailyLimit(policy)
	if limit <= 0 {
		return "普通居民订单上限必须大于 0", true
	}
	finished := s.ResidentOrderFinishNum(now)
	if finished >= limit {
		return fmt.Sprintf("普通居民订单今日已完成 %d/%d", finished, limit), true
	}
	return "", false
}

func residentOrderLimitBlock(s *state.State, policy *pb.ResidentOrderPolicy, goal Goal, now time.Time) (PlannedOp, bool) {
	reason, reached := residentNormalDailyLimitReached(s, policy, now)
	if !reached {
		return PlannedOp{}, false
	}
	label := "居民订单今日上限已达"
	if strings.Contains(reason, "必须大于 0") {
		label = "居民订单普通订单上限未设置"
	}
	blocked := markerOp(CategoryOrder, "order.resident", "finish", label, goal.Priority*100+690)
	blocked.GoalID = goal.ID
	blocked.Status = PlanStatusBlocked
	blocked.Executable = false
	blocked.BlockedReasons = []string{reason}
	return blocked, true
}

// residentSatinDailyLimit returns the effective satin resident-order cap:
// policy limit (defaulting to catalog $dailyMax2 when unset), optionally
// tightened by that same hard cap.
func residentSatinDailyLimit(policy *pb.ResidentOrderPolicy) int32 {
	if policy == nil {
		return 0
	}
	limit := policy.GetSatinDailyLimit()
	hardLimit := state.ResidentOrderSatinDailyMax()
	if limit <= 0 {
		if hardLimit > 0 {
			return hardLimit
		}
		return 120
	}
	if hardLimit > 0 && hardLimit < limit {
		return hardLimit
	}
	return limit
}

func residentSatinDailyLimitReached(s *state.State, policy *pb.ResidentOrderPolicy, now time.Time) (reason string, reached bool) {
	if until, ok := s.ResidentSatinDailyLimitReached(now); ok {
		return fmt.Sprintf("服务端提示今日完成订单次数已达上限，等待次日0点（%s）后再继续", until.Format("01/02 15:04")), true
	}
	limit := residentSatinDailyLimit(policy)
	if limit <= 0 {
		return "绸缎居民订单上限必须大于 0", true
	}
	finished := s.ResidentSatinOrderFinishNum(now)
	if finished >= limit {
		return fmt.Sprintf("绸缎居民订单今日已完成 %d/%d", finished, limit), true
	}
	return "", false
}

// residentDecorateDailyLimit returns the effective decorate resident-order cap:
// policy limit (defaulting to catalog $dailyMax3 when unset), optionally
// tightened by that same hard cap.
func residentDecorateDailyLimit(policy *pb.ResidentOrderPolicy) int32 {
	if policy == nil {
		return 0
	}
	limit := policy.GetDecorateDailyLimit()
	hardLimit := state.ResidentOrderDecorateDailyMax()
	if limit <= 0 {
		if hardLimit > 0 {
			return hardLimit
		}
		return 120
	}
	if hardLimit > 0 && hardLimit < limit {
		return hardLimit
	}
	return limit
}

func residentDecorateDailyLimitReached(s *state.State, policy *pb.ResidentOrderPolicy, now time.Time) (reason string, reached bool) {
	if until, ok := s.ResidentDecorateDailyLimitReached(now); ok {
		return fmt.Sprintf("服务端提示今日完成订单次数已达上限，等待次日0点（%s）后再继续", until.Format("01/02 15:04")), true
	}
	limit := residentDecorateDailyLimit(policy)
	if limit <= 0 {
		return "建材居民订单上限必须大于 0", true
	}
	finished := s.ResidentDecorateOrderFinishNum(now)
	if finished >= limit {
		return fmt.Sprintf("建材居民订单今日已完成 %d/%d", finished, limit), true
	}
	return "", false
}

// CustomerDailyLimitReached reports whether customer-order automation should
// stop for the configured daily finish cap. A non-positive limit means unlimited.
func CustomerDailyLimitReached(s *state.State, policy *pb.CustomerOrderPolicy, now time.Time) (reason string, reached bool) {
	return customerDailyLimitReached(s, policy, now)
}

func customerDailyLimitReached(s *state.State, policy *pb.CustomerOrderPolicy, now time.Time) (reason string, reached bool) {
	if policy == nil {
		return "", false
	}
	limit := policy.GetDailyLimit()
	if limit <= 0 {
		return "", false
	}
	finished := s.CustomerOrderFinishNum(now)
	if finished >= limit {
		return fmt.Sprintf("顾客订单今日已完成 %d/%d", finished, limit), true
	}
	return "", false
}

func customerOrderLimitBlock(s *state.State, policy *pb.CustomerOrderPolicy, goal Goal, now time.Time) (PlannedOp, bool) {
	reason, reached := customerDailyLimitReached(s, policy, now)
	if !reached {
		return PlannedOp{}, false
	}
	blocked := markerOp(CategoryOrder, "order.customer", "finish", "顾客订单今日上限已达", goal.Priority*100+690)
	blocked.GoalID = goal.ID
	blocked.Status = PlanStatusBlocked
	blocked.Executable = false
	blocked.BlockedReasons = []string{reason}
	return blocked, true
}

// customerOrderFlowerArtCount returns the total flower-art pieces required by a
// customer order (sum of ItemRequires counts).
func customerOrderFlowerArtCount(order *state.CustomerOrder) int32 {
	if order == nil {
		return 0
	}
	var total int32
	for _, req := range order.ItemRequires {
		if req.ItemID > 0 && req.Count > 0 {
			total += req.Count
		}
	}
	return total
}

// customerOrderMeetsMinFlowerArt reports whether the order satisfies the
// configured minimum flower-art piece count. A non-positive min means no filter.
// When bypass is true (held unfinished customer-order race task), the min is
// ignored so race progress can continue.
func customerOrderMeetsMinFlowerArt(order *state.CustomerOrder, policy *pb.CustomerOrderPolicy, bypass bool) bool {
	if bypass || policy == nil {
		return true
	}
	min := policy.GetMinFlowerArtCount()
	if min <= 0 {
		return true
	}
	return customerOrderFlowerArtCount(order) >= min
}

func residentSpecialOrderAllowed(order state.ResidentSpecialOrder, policy *pb.ResidentOrderPolicy) bool {
	if !order.Observed || order.IsVideo != 0 || len(order.Requires) == 0 {
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

func canFulfillResidentSpecialOrder(order state.ResidentSpecialOrder, entityID string, goal Goal, ledger *InventoryLedger) bool {
	if len(order.Requires) == 0 {
		return false
	}
	for _, req := range order.Requires {
		id := demandID(goal.ID, entityID, "direct", DemandKindFlower, req.FlowerID)
		if req.FlowerID == 0 || req.Count <= 0 || ledger.AllocatedForDemand(id, req.FlowerID) < req.Count {
			return false
		}
	}
	return true
}

func blockedResidentSpecialOrderOp(kind, domain, label string, order state.ResidentSpecialOrder, goal Goal, reason string) PlannedOp {
	blocked := op(kind, goal, "finish", reason, goal.Priority*100+125, 0, 0, 0)
	blocked.Domain = domain
	blocked.Status = PlanStatusBlocked
	blocked.Executable = false
	if len(order.Requires) == 0 {
		blocked.BlockedReasons = []string{label + "缺少可识别需求"}
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

func flowerArtCraftItemCost(flowerIDs []int32, count int32) map[int32]int32 {
	if count <= 0 {
		return nil
	}
	out := make(map[int32]int32)
	for _, flowerID := range flowerIDs {
		if flowerID > 0 {
			out[flowerID] += count
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func maxCraftableCount(recipe state.FlowerArtRecipe, ledger *InventoryLedger) int32 {
	if recipe.ArtID <= 0 || ledger == nil {
		return 0
	}
	var max int32 = -1
	for flowerID, needEach := range recipeFlowerCounts(recipe) {
		if needEach <= 0 {
			continue
		}
		available := ledger.Available(flowerID) / needEach
		if max < 0 || available < max {
			max = available
		}
	}
	if max < 0 {
		return 0
	}
	return max
}

func bestRackArt(s *state.State, policy *pb.FlowerArtPolicy, ledger *InventoryLedger) (int32, int32, bool) {
	return bestRackArtByCount(s, policy, ledger)
}

func bestRackArtForRace(s *state.State, ledger *InventoryLedger) (int32, int32, bool) {
	candidates := rackArtCandidates(s, nil)
	return bestArtByInventoryCount(candidates, ledger)
}

func rackArtCandidates(s *state.State, preferredArtIDs []int32) []state.FlowerArtRecipe {
	all := rackCandidateRecipes()
	if len(preferredArtIDs) == 0 {
		return filterRecipesByUnlockedVase(s, all)
	}
	preferred := int32Set(preferredArtIDs)
	out := make([]state.FlowerArtRecipe, 0, len(preferred))
	for _, recipe := range all {
		if preferred[recipe.ArtID] {
			out = append(out, recipe)
		}
	}
	return filterRecipesByUnlockedVase(s, out)
}

func filterRecipesByUnlockedVase(s *state.State, recipes []state.FlowerArtRecipe) []state.FlowerArtRecipe {
	if s == nil || len(recipes) == 0 {
		return recipes
	}
	if !s.VaseObserved() {
		return recipes
	}
	out := make([]state.FlowerArtRecipe, 0, len(recipes))
	for _, recipe := range recipes {
		if s.HasVase(recipe.VaseID) {
			out = append(out, recipe)
		}
	}
	return out
}

func bestRackArtByCount(s *state.State, policy *pb.FlowerArtPolicy, ledger *InventoryLedger) (int32, int32, bool) {
	if ledger == nil {
		return 0, 0, false
	}
	var preferred []int32
	if policy != nil {
		preferred = policy.GetSellArtIds()
	}
	candidates := rackArtCandidates(s, preferred)
	artID, count, ok := bestArtByInventoryCount(candidates, ledger)
	if ok {
		return artID, count, true
	}
	if len(preferred) > 0 {
		return bestArtByInventoryCount(filterRecipesByUnlockedVase(s, rackCandidateRecipes()), ledger)
	}
	return 0, 0, false
}

func bestArtByInventoryCount(recipes []state.FlowerArtRecipe, ledger *InventoryLedger) (int32, int32, bool) {
	if ledger == nil || len(recipes) == 0 {
		return 0, 0, false
	}
	var bestArtID int32
	var bestCount int32
	var bestAvailable int32
	found := false
	for _, recipe := range recipes {
		available := ledger.Available(recipe.ArtID)
		if available <= 0 {
			continue
		}
		listCount := available
		if listCount > flowerRackPerSlotCount {
			listCount = flowerRackPerSlotCount
		}
		if !found || available > bestAvailable || (available == bestAvailable && recipe.ArtID < bestArtID) {
			bestArtID = recipe.ArtID
			bestCount = listCount
			bestAvailable = available
			found = true
		}
	}
	return bestArtID, bestCount, found
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
	candidates := rackArtCandidates(s, policy.GetSellArtIds())
	for _, recipe := range candidates {
		if ledger.Available(recipe.ArtID) > 0 {
			return 0, 0, false
		}
	}
	if artID, count, ok := rackCraftRecipeTarget(candidates, s, ledger); ok {
		return artID, count, true
	}
	if len(policy.GetSellArtIds()) > 0 {
		fallback := filterRecipesByUnlockedVase(s, rackCandidateRecipes())
		for _, recipe := range fallback {
			if ledger.Available(recipe.ArtID) > 0 {
				return 0, 0, false
			}
		}
		return rackCraftRecipeTarget(fallback, s, ledger)
	}
	return 0, 0, false
}

func rackCraftRecipeTarget(recipes []state.FlowerArtRecipe, s *state.State, ledger *InventoryLedger) (int32, int32, bool) {
	for _, recipe := range recipes {
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
	if order == nil || ledger == nil {
		return false
	}
	hasRequirements := false
	entityID := strconv.FormatInt(int64(npcID), 10)
	for _, req := range order.Requires {
		if req.FlowerID == 0 || req.Count <= 0 {
			continue
		}
		hasRequirements = true
		// Prefer raw inventory so stale/over-allocated ledger rows cannot block a
		// finish when stock is actually on hand; still require this demand's
		// allocation so two NPCs cannot claim the same unit in one plan.
		if ledger.Owned(req.FlowerID) < req.Count {
			return false
		}
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
		if ledger.Owned(req.ItemID) < req.Count {
			return false
		}
		id := demandID(goal.ID, entityID, "direct", DemandKindFlowerArt, req.ItemID)
		if ledger.AllocatedForDemand(id, req.ItemID) < req.Count {
			return false
		}
	}
	return hasRequirements
}
