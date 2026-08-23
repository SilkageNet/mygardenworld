package automation

import (
	"fmt"
	"sort"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// raceTakeLeadWindow is how early automation may emit takeTask for a CD pool
// row that already meets filter rules. The decision loop wakes at
// AppearTime-lead so the default 4s tick cannot miss this window.
const raceTakeLeadWindow = 300 * time.Millisecond

// raceTaskPoolRefreshInterval is how often automation re-fetches the task pool
// when idle of giveUp/finish/take.
const raceTaskPoolRefreshInterval = 10 * time.Minute

// raceNearCDSyncSuppressWindow is how close a filter-passing CD task must be
// before periodic getTaskList is deferred. Keeping this much shorter than
// raceTaskPoolRefreshInterval lets long CD waits still refresh upgrade/claim
// state; only the final approach skips sync to favor take timing.
const raceNearCDSyncSuppressWindow = 45 * time.Second

// raceFinishProgressSyncInterval caps getTaskList retries when LocalFinishCnt
// already meets the target but server FinishCnt still lags. A successful
// getTaskList that still leaves FinishCnt short clamps LocalFinishCnt (see
// state.reconcileFmlRaceLocalFinishAfterFullPool), so this is a short nudge
// rather than an unbounded poll.
const raceFinishProgressSyncInterval = 30 * time.Second

// raceInactiveEnterRetryInterval is how often enter may re-probe after an
// inactive batch once the weekly session (or a published start window) is open.
const raceInactiveEnterRetryInterval = 30 * time.Second

// fmlFlowerTakeListRefreshInterval is how often automation re-fetches the
// guild other-share list while take quota remains.
const fmlFlowerTakeListRefreshInterval = time.Hour

func shopOperations(s *state.State, policy *pb.Policy, now time.Time) []PlannedOp {
	shop := policy.GetBasic().GetShop()
	var ops []PlannedOp
	ops = append(ops, giftbagOperations(s, shop)...)
	ops = append(ops, cultivateShopOperations(s, shop, now)...)
	vipShop := shop.GetVipShop()
	if vipShop.GetAutoBuy() {
		blocked := markerOp(CategoryBasic, "basic.shop.vip", "buy", "VIP 商店购买需要成本和状态确认", 115)
		blocked.Label = "VIP 商店"
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"VIP 商店商品状态、花坊币/元宝成本尚未完成协议确认"}
		ops = append(ops, blocked)
	}
	return ops
}

func giftbagOperations(s *state.State, shop *pb.ShopPolicy) []PlannedOp {
	if !shop.GetVideoFreeGiftEnabled() {
		return nil
	}
	goal := Goal{ID: "basic.shop.giftbag", Category: CategoryBasic, Domain: "basic.shop.giftbag", Label: "视频礼包", Priority: 54}
	if !s.ShopGiftbagObserved() {
		return []PlannedOp{domainOp(clientproto.RPCShopGiftbagEnter.String(), goal, "basic.shop.giftbag", "sync", "礼包商店状态未同步，先进入商店获取购买记录", 5480, 0, 0, 0)}
	}
	for _, offer := range s.ShopGiftbagOffers() {
		if !freeVideoGiftbag(offer) || offer.Remaining <= 0 {
			continue
		}
		buy := domainOp(clientproto.RPCShopGiftbagBuy.String(), goal, "basic.shop.video_gift", "claim", "视频免费礼包可领取", 5470, offer.ShopID, 0, 1)
		return []PlannedOp{buy}
	}
	return nil
}

func freeVideoGiftbag(offer state.ShopGiftbagOfferView) bool {
	return offer.Type == 1 && offer.ShareID > 0 && offer.RchgID == 0 &&
		offer.MoneyID == 0 && offer.Price == 0 && offer.PriceMax == 0
}

func cultivateShopOperations(s *state.State, shop *pb.ShopPolicy, now time.Time) []PlannedOp {
	cultivateShop := shop.GetCultivateShop()
	if !cultivateShop.GetAutoBuy() {
		return nil
	}
	goal := Goal{ID: "basic.shop.cultivate", Category: CategoryBasic, Domain: "basic.shop.cultivate", Label: "材料商店", Priority: 54}
	if s.ShopCultivateNeedsEnter(now) {
		reason := "材料商店状态未同步，先进入商店获取价格"
		if s.ShopCultivateObserved() {
			reason = "材料商店已跨日重置，先进入商店刷新购买记录"
		}
		return []PlannedOp{domainOp(clientproto.RPCShopCultivateEnter.String(), goal, "basic.shop.cultivate", "sync", reason, 5450, 0, 0, 0)}
	}
	if s.ShopCultivateAutoRefreshReady(now) {
		return []PlannedOp{domainOp(clientproto.RPCShopCultivateRefresh.String(), goal, "basic.shop.cultivate", "refresh", "材料商店免费刷新可用，先刷新货架", 5420, 0, 0, 0)}
	}
	allowed := int32Set(cultivateShop.GetItemIds())
	inventory := s.Inventory()
	var firstBlocked *PlannedOp
	for _, offer := range s.ShopCultivateOffers() {
		if offer.ShopID <= 0 || offer.Remaining <= 0 {
			continue
		}
		if len(allowed) > 0 && !allowed[offer.ShopID] && !allowed[offer.ItemID] {
			continue
		}
		buy := domainOp(clientproto.RPCShopCultivateBuy.String(), goal, "basic.shop.cultivate", "buy", "材料商店白名单商品可购买", 5400, offer.ShopID, offer.ItemID, offer.ItemCount)
		buy.ItemCost = map[int32]int32{}
		if blocked := applyShopCultivateCostGate(&buy, offer, cultivateShop, s, inventory); len(blocked) > 0 {
			buy.Status = PlanStatusAdapterMissing
			buy.Executable = false
			buy.BlockedReasons = blocked
			if firstBlocked == nil {
				cp := buy
				firstBlocked = &cp
			}
			continue
		}
		if len(buy.ItemCost) == 0 {
			buy.ItemCost = nil
		}
		return []PlannedOp{buy}
	}
	if firstBlocked != nil {
		return []PlannedOp{*firstBlocked}
	}
	return nil
}

func applyShopCultivateCostGate(op *PlannedOp, offer state.ShopCultivateOfferView, policy *pb.ShopBuyPolicy, s *state.State, inventory map[int32]int32) []string {
	if offer.CostItemID <= 0 || offer.CostCount <= 0 {
		return []string{"材料商店价格未观测"}
	}
	switch offer.CostItemID {
	case 11:
		if policy.GetMaxSpendGold() <= 0 {
			return []string{"材料商店金币预算未设置"}
		}
		if int64(offer.CostCount) > policy.GetMaxSpendGold() {
			return []string{"材料商店金币成本超过策略上限"}
		}
		if s.Gold() < offer.CostCount {
			return []string{"金币不足"}
		}
		op.GoldCost = offer.CostCount
	case 1:
		op.DiamondCost = offer.CostCount
		if policy.GetMaxSpendDiamond() <= 0 {
			return []string{"材料商店元宝预算未设置"}
		}
		if int64(offer.CostCount) > policy.GetMaxSpendDiamond() {
			return []string{"材料商店元宝成本超过策略上限"}
		}
		return []string{"元宝成本操作尚未放开自动执行"}
	default:
		if inventory[offer.CostItemID] < offer.CostCount {
			return []string{"成本物品不足或未观测"}
		}
		op.ItemCost[offer.CostItemID] = offer.CostCount
	}
	return nil
}

func unionOperations(s *state.State, policy *pb.Policy, now time.Time) []PlannedOp {
	if policy == nil {
		return nil
	}
	union := policy.GetUnion()
	if union == nil {
		return nil
	}
	uid := s.RoleID()
	gates := raceModuleGatesFromPolicy(policy)
	var ops []PlannedOp
	ops = append(ops, unionBuildOperations(s, union.GetBuild())...)
	ops = append(ops, unionFlowerOperations(s, union.GetFlower(), now)...)
	ops = append(ops, unionLandOperations(s, union.GetLand(), now)...)
	ops = append(ops, unionRaceOperations(s, union.GetRace(), uid, now, gates)...)
	ops = append(ops, unionForestOperations(s, union.GetForestEnabled())...)
	return ops
}

func unionFlowerOperations(s *state.State, policy *pb.UnionFlowerPolicy, now time.Time) []PlannedOp {
	if policy == nil || (!policy.GetShareEnabled() && !policy.GetTakeEnabled()) {
		return nil
	}
	goal := Goal{ID: "union.flower", Category: CategoryUnion, Domain: "union.flower", Label: "公会鲜花共享", Priority: 44}
	var ops []PlannedOp
	if policy.GetShareEnabled() {
		if !s.FmlFlowerShareObserved() {
			sync := domainOp(clientproto.RPCFmlFlowerShareRefresh.String(), goal, "union.flower.reward", "claim", "公会分享状态未观测，先刷新", 4470, 0, 0, 0)
			ops = append(ops, sync)
		} else if slotIDs := s.ReadyFmlFlowerShareRewardSlotIDs(); len(slotIDs) > 0 {
			claim := domainOp(clientproto.RPCFmlFlowerShareRecvRwd.String(), goal, "union.flower.reward", "claim", "公会分享槽位有可领取奖励", 4470, 0, 0, int32(len(slotIDs)))
			claim.SlotIDs = slotIDs
			ops = append(ops, claim)
		}
	}
	if policy.GetTakeEnabled() {
		ops = append(ops, unionFlowerTakeOperations(s, policy, goal, now)...)
	}
	return ops
}

func unionFlowerTakeOperations(s *state.State, policy *pb.UnionFlowerPolicy, goal Goal, now time.Time) []PlannedOp {
	// Daily window: only run at/after 00:01 Asia/Shanghai.
	if !state.FmlFlowerTakeWindowOpen(now) {
		return nil
	}
	// Quota exhausted: do not take and do not refresh the share list.
	if _, ok := s.FmlFlowerTakeDailyLimitReached(now); ok {
		return nil
	}
	if s.FmlFlowerTakeExhausted(now) {
		return nil
	}
	// Need own share counters before deciding takes when possible.
	if !s.FmlFlowerShareObserved() {
		sync := domainOp(clientproto.RPCFmlFlowerShareRefresh.String(), goal, "union.flower.take", "take", "公会摸花次数未观测，先刷新分享状态", 4465, 0, 0, 0)
		sync.CooldownKey = "union.flower.take"
		return []PlannedOp{sync}
	}
	if !s.OtherFmlFlowerSharesObserved() || fmlFlowerTakeListStale(s, now) {
		sync := domainOp(clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String(), goal, "union.flower.take", "take", "公会摸花列表未观测或超过1小时，重新拉取", 4460, 0, 0, 0)
		sync.CooldownKey = "union.flower.take"
		return []PlannedOp{sync}
	}
	// Prefer the allowed share whose flower has the lowest personal inventory
	// stock, so multi-flower take lists refill scarcest flowers first instead
	// of always taking the first configured / lowest FlowerID match.
	inventory := s.Inventory()
	var best state.FmlFlowerTakeCandidate
	found := false
	bestStock := int32(0)
	for _, candidate := range s.FmlFlowerTakeCandidates() {
		if !unionFlowerTakeAllowed(candidate, policy) {
			continue
		}
		stock := inventory[candidate.FlowerID]
		if !found || stock < bestStock {
			best = candidate
			bestStock = stock
			found = true
		}
	}
	if !found {
		return nil
	}
	take := domainOp(clientproto.RPCFmlFlowerShareTake.String(), goal, "union.flower.take", "take", "公会成员分享鲜花可摸取", 4460, best.SlotID, 0, 1)
	take.TargetUID = best.UID
	take.FlowerID = best.FlowerID
	take.CooldownKey = "union.flower.take"
	return []PlannedOp{take}
}

func fmlFlowerTakeListStale(s *state.State, now time.Time) bool {
	syncedAt := s.OtherFmlFlowerSharesSyncedAtMs()
	if syncedAt <= 0 {
		return true
	}
	return !now.Before(time.UnixMilli(syncedAt).Add(fmlFlowerTakeListRefreshInterval))
}

func unionFlowerTakeAllowed(candidate state.FmlFlowerTakeCandidate, policy *pb.UnionFlowerPolicy) bool {
	if candidate.FlowerID <= 0 {
		return false
	}
	mode := policy.GetTakeMode()
	flowers := int32Set(policy.GetTakeFlowerIds())
	qualities := int32Set(policy.GetTakeQualities())
	switch mode {
	case pb.SelectionMode_SELECTION_MODE_SPECIFIC:
		return len(flowers) == 0 || flowers[candidate.FlowerID]
	case pb.SelectionMode_SELECTION_MODE_EXCLUDE:
		return !flowers[candidate.FlowerID]
	case pb.SelectionMode_SELECTION_MODE_QUALITY:
		return len(qualities) == 0 || qualities[flowerQuality(candidate.FlowerID)]
	case pb.SelectionMode_SELECTION_MODE_ALL:
		return true
	default:
		if len(flowers) > 0 && !flowers[candidate.FlowerID] {
			return false
		}
		if len(qualities) > 0 && !qualities[flowerQuality(candidate.FlowerID)] {
			return false
		}
		return true
	}
}

func flowerQuality(flowerID int32) int32 {
	item, ok := state.ItemInfoByID(flowerID)
	if !ok {
		return 0
	}
	return item.Color
}

func unionBuildOperations(s *state.State, policy *pb.UnionBuildPolicy) []PlannedOp {
	if policy == nil || !unionBuildPolicyEnabled(policy) {
		return nil
	}
	goal := Goal{ID: "union.build", Category: CategoryUnion, Domain: "union.build", Label: "公会建设", Priority: 45}
	if !s.FmlBuildObserved() {
		blocked := domainOp(clientproto.RPCFmlBld.String(), goal, "union.build", "build", "公会建设状态未观测", 4590, 0, 0, 0)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"未观察到公会 namespace 25，需先进入公会或补充 fml.enter 同步链路"}
		return []PlannedOp{blocked}
	}
	build := s.FmlBuild()
	if !build.BuildCountsObserved {
		blocked := domainOp(clientproto.RPCFmlBld.String(), goal, "union.build", "build", "公会建设次数未观测", 4590, 0, 0, 0)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"未观察到 bldCountMap，无法确认今日建设次数"}
		return []PlannedOp{blocked}
	}
	inventory := s.Inventory()
	var firstBlocked *PlannedOp
	for _, id := range unionBuildOptionIDs(policy) {
		option, ok := state.FmlBuildOptionByID(id)
		if !ok {
			blocked := domainOp(clientproto.RPCFmlBld.String(), goal, "union.build", "build", "公会建设档位配置缺失", 4500-id, id, 0, 0)
			blocked.Status = PlanStatusAdapterMissing
			blocked.Executable = false
			blocked.BlockedReasons = []string{"缺少 c_fmlBld 静态配置"}
			if firstBlocked == nil {
				firstBlocked = &blocked
			}
			continue
		}
		if option.DailyLimit > 0 && build.BuildCounts[id] >= option.DailyLimit {
			continue
		}
		reason := strings.TrimSpace(option.Name)
		if reason == "" {
			reason = fmt.Sprintf("公会建设 #%d", id)
		}
		buildOp := domainOp(clientproto.RPCFmlBld.String(), goal, "union.build", "build", reason+"可执行", 4500-id, id, 0, 1)
		if blocked := applyUnionBuildCostGate(&buildOp, option, policy, s, inventory); len(blocked) > 0 {
			buildOp.Status = PlanStatusAdapterMissing
			buildOp.Executable = false
			buildOp.BlockedReasons = blocked
			if firstBlocked == nil {
				cp := buildOp
				firstBlocked = &cp
			}
			continue
		}
		if len(buildOp.ItemCost) == 0 {
			buildOp.ItemCost = nil
		}
		return []PlannedOp{buildOp}
	}
	if firstBlocked != nil {
		return []PlannedOp{*firstBlocked}
	}
	return nil
}

func unionBuildPolicyEnabled(policy *pb.UnionBuildPolicy) bool {
	return policy.GetFreeEnabled() || policy.GetGoldEnabled() || policy.GetDiamondEnabled()
}

func unionBuildOptionIDs(policy *pb.UnionBuildPolicy) []int32 {
	var ids []int32
	if policy.GetFreeEnabled() {
		ids = append(ids, 1)
	}
	if policy.GetGoldEnabled() {
		ids = append(ids, 2)
	}
	if policy.GetDiamondEnabled() {
		ids = append(ids, 3)
	}
	return ids
}

func applyUnionBuildCostGate(op *PlannedOp, option state.FmlBuildOption, policy *pb.UnionBuildPolicy, s *state.State, inventory map[int32]int32) []string {
	// c_fmlBld id=1 is 视频捐献 (shareId→c_share hasVideo). Bare fml.bld({id:1})
	// is rejected without the SDK video/share flow; runner does not forge that.
	if option.ShareID > 0 {
		return []string{"依赖客户端 SDK 广告 token，本地 runner 不伪造视频完成"}
	}
	if option.ItemID <= 0 || option.Cost <= 0 {
		return []string{"公会建设档位缺少可执行成本配置"}
	}
	switch option.ItemID {
	case 11:
		if policy.GetMaxSpendGold() <= 0 {
			return []string{"公会金币建设预算未设置"}
		}
		if int64(option.Cost) > policy.GetMaxSpendGold() {
			return []string{"公会金币建设成本超过策略上限"}
		}
		if s.Gold() < option.Cost {
			return []string{"金币不足"}
		}
		op.GoldCost = option.Cost
	case 1:
		op.DiamondCost = option.Cost
		if policy.GetMaxSpendDiamond() <= 0 {
			return []string{"公会元宝建设预算未设置"}
		}
		if int64(option.Cost) > policy.GetMaxSpendDiamond() {
			return []string{"公会元宝建设成本超过策略上限"}
		}
		if s.SpendableDiamonds() < option.Cost {
			return []string{"元宝不足"}
		}
		return []string{"元宝成本操作尚未放开自动执行"}
	default:
		if inventory[option.ItemID] < option.Cost {
			return []string{"公会建设成本物品不足或未观测"}
		}
		op.ItemCost = map[int32]int32{option.ItemID: option.Cost}
	}
	return nil
}

func unionLandOperations(s *state.State, policy *pb.UnionLandPolicy, now time.Time) []PlannedOp {
	if policy == nil {
		policy = &pb.UnionLandPolicy{}
	}
	goal := Goal{ID: "union.land", Category: CategoryUnion, Domain: "union.land", Label: "公会土地", Priority: 44}
	// Sync is independent of harvest/plant toggles so the land monitor can
	// observe 25.102 even when auto actions stay off (same pattern as race
	// enter/getTaskList while AutoEnableModules is false).
	if !s.FmlLandObserved() {
		sync := domainOp(clientproto.RPCFmlEnter.String(), goal, "union.land", "sync", "公会土地状态未观测，先进入公会同步", 4485, 0, 0, 0)
		return []PlannedOp{sync}
	}
	harvestEnabled := policy.GetHarvestEnabled()
	plantEnabled := policy.GetAutoPlantEnabled()
	if !harvestEnabled && !plantEnabled {
		return nil
	}
	var ops []PlannedOp
	if plantEnabled {
		if plant, ok := unionLandPlantOperation(s, policy, goal, now); ok {
			ops = append(ops, plant)
		}
	}
	if harvestEnabled {
		landIDs := s.ReadyFmlLandHarvestIDs(now)
		if len(landIDs) > 0 {
			reason := state.FormatFmlLandHarvestReason(s.FmlLands(), landIDs, now)
			harvest := domainOp(clientproto.RPCFmlLandHarvest.String(), goal, "union.land.harvest", "harvest", reason, 4475, 0, 0, int32(len(landIDs)))
			harvest.LandIDs = landIDs
			ops = append(ops, harvest)
		}
	}
	return ops
}

const (
	unionLandPreferBelowLevel   int32 = 11
	unionLandDefaultMaturityMin int32 = 20
	// When leveling flowers below 11, skip force-replace if the current crop
	// matures within this grace window so harvest can finish first.
	unionLandNearMatureGrace = 2 * time.Minute
)

func unionLandPlantOperation(s *state.State, policy *pb.UnionLandPolicy, goal Goal, now time.Time) (PlannedOp, bool) {
	candidates := filterUnionLandPlantCandidates(s.PlantableFlowers(nil, nil), policy)
	flowerID, reason := selectUnionLandPlantFlowerFrom(candidates, policy)
	if flowerID <= 0 {
		return PlannedOp{}, false
	}
	leveling := unionLandHasBelowLevel(candidates)
	landIDs := unionLandPlantableIDs(s, flowerID, now, leveling)
	if len(landIDs) == 0 {
		return PlannedOp{}, false
	}
	name := state.FlowerName(flowerID)
	if name == "" {
		name = fmt.Sprintf("花卉#%d", flowerID)
	}
	if leveling {
		reason += "；未满11级强制换种练级"
	}
	plantReason := fmt.Sprintf("公会土地自动种植 %s×%d: %s", name, len(landIDs), reason)
	// Plant above harvest so continuous mature-land harvest cannot starve empty
	// or replace planting when many guild slots produce in rotation.
	plant := domainOp(clientproto.RPCFmlLandPlant.String(), goal, "union.land.plant", "plant", plantReason, 4480, 0, flowerID, int32(len(landIDs)))
	plant.LandIDs = landIDs
	plant.FlowerID = flowerID
	return plant, true
}

// selectUnionLandPlantFlowerFrom picks one flower for guild-land auto-plant
// from already policy-filtered candidates: while any candidate is below level
// 11, plant the highest-quality flower first (华>珍>普>凡), then lowest level
// and lowest stock so every flower can reach 11; maturity minutes are ignored
// in that phase. Once every candidate is at/above 11, prefer long-maturity and
// break ties by lowest stock.
func selectUnionLandPlantFlowerFrom(candidates []state.PlantableFlower, policy *pb.UnionLandPolicy) (flowerID int32, reason string) {
	if len(candidates) == 0 {
		return 0, ""
	}
	minMinutes := policy.GetMinMaturityMinutes()
	if minMinutes <= 0 {
		minMinutes = unionLandDefaultMaturityMin
	}
	minCD := minMinutes * 60
	preferBelow := unionLandPreferBelowLevel
	lowLevel := make([]state.PlantableFlower, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Lvl > 0 && candidate.Lvl < preferBelow {
			lowLevel = append(lowLevel, candidate)
		}
	}
	if len(lowLevel) > 0 {
		best := pickHighestQualityThenLevelStock(lowLevel)
		return best.FlowerID, fmt.Sprintf("优先未满%d级（品阶高，其次等级低、库存少），确保全部升到%d", preferBelow, preferBelow)
	}
	if longMature := filterPlantableByMinCD(candidates, minCD); len(longMature) > 0 {
		best := pickLowestStockPlantable(longMature)
		return best.FlowerID, fmt.Sprintf("全部≥%d级，改种成熟≥%d分钟且库存少", preferBelow, minMinutes)
	}
	best := pickLowestStockPlantable(candidates)
	return best.FlowerID, fmt.Sprintf("全部≥%d级且无长成熟候选，改种库存最少", preferBelow)
}

func unionLandHasBelowLevel(candidates []state.PlantableFlower) bool {
	for _, candidate := range candidates {
		if candidate.Lvl > 0 && candidate.Lvl < unionLandPreferBelowLevel {
			return true
		}
	}
	return false
}

// unionLandNearMature reports whether the next flower matures within the grace
// window. Leveling force-replace waits for harvest in that case.
func unionLandNearMature(land state.FmlLandView, now time.Time) bool {
	next := state.FmlLandNextMatureMs(land, now)
	if next <= 0 {
		return false
	}
	remaining := time.UnixMilli(next).Sub(now)
	return remaining > 0 && remaining <= unionLandNearMatureGrace
}

func filterPlantableByMinCD(candidates []state.PlantableFlower, minCD int32) []state.PlantableFlower {
	out := make([]state.PlantableFlower, 0, len(candidates))
	for _, candidate := range candidates {
		cd, ok := state.FlowerLvlCDSeconds(candidate.FlowerID, candidate.Lvl)
		if !ok || cd < minCD {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func filterUnionLandPlantCandidates(candidates []state.PlantableFlower, policy *pb.UnionLandPolicy) []state.PlantableFlower {
	if policy == nil || len(candidates) == 0 {
		return candidates
	}
	flowers := int32Set(policy.GetFlowerIds())
	qualities := int32Set(policy.GetQualities())
	maxLvl := policy.GetMaxFlowerLevel()
	if len(flowers) == 0 && len(qualities) == 0 && maxLvl <= 0 {
		return candidates
	}
	out := make([]state.PlantableFlower, 0, len(candidates))
	for _, candidate := range candidates {
		if len(flowers) > 0 && !flowers[candidate.FlowerID] {
			continue
		}
		if len(qualities) > 0 && !qualities[flowerQuality(candidate.FlowerID)] {
			continue
		}
		if maxLvl > 0 && candidate.Lvl > maxLvl {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func pickLowestStockPlantable(candidates []state.PlantableFlower) state.PlantableFlower {
	best := candidates[0]
	for i := 1; i < len(candidates); i++ {
		candidate := candidates[i]
		if candidate.Stock < best.Stock ||
			(candidate.Stock == best.Stock && candidate.FlowerID < best.FlowerID) {
			best = candidate
		}
	}
	return best
}

// pickHighestQualityThenLevelStock prefers higher item.color (仙>华>珍>普>凡),
// then lowest cultivate level, then lowest stock, then lower flower id.
func pickHighestQualityThenLevelStock(candidates []state.PlantableFlower) state.PlantableFlower {
	best := candidates[0]
	bestQuality := flowerQuality(best.FlowerID)
	for i := 1; i < len(candidates); i++ {
		candidate := candidates[i]
		quality := flowerQuality(candidate.FlowerID)
		switch {
		case quality > bestQuality:
			best = candidate
			bestQuality = quality
		case quality == bestQuality && candidate.Lvl < best.Lvl:
			best = candidate
		case quality == bestQuality && candidate.Lvl == best.Lvl && candidate.Stock < best.Stock:
			best = candidate
		case quality == bestQuality && candidate.Lvl == best.Lvl && candidate.Stock == best.Stock &&
			candidate.FlowerID < best.FlowerID:
			best = candidate
		}
	}
	return best
}

// unionLandPlantableIDs returns empty slots and replace targets.
// While any filtered flower is below level 11, occupied lands with a different
// flower are force-replaced unless harvest is pending or the next mature is
// within 2 minutes (wait for harvest, then switch). After every flower reaches
// 11, occupied lands may be replaced freely for long-maturity selection.
func unionLandPlantableIDs(s *state.State, flowerID int32, now time.Time, leveling bool) []int32 {
	lands := s.FmlLands()
	ids := make([]int32, 0, len(lands))
	for id := range lands {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]int32, 0, len(ids))
	for _, id := range ids {
		land := lands[id]
		if state.FmlLandPendingHarvest(land, now) > 0 {
			continue
		}
		if land.FlowerID <= 0 {
			out = append(out, id)
			continue
		}
		if land.FlowerID == flowerID {
			continue
		}
		if leveling && unionLandNearMature(land, now) {
			// Current crop matures within 2 minutes: harvest first, then switch.
			continue
		}
		// Leveling: force replace when farther than the grace window.
		// Post-11: replace freely for maturity/stock selection.
		out = append(out, id)
	}
	return out
}

func unionForestOperations(s *state.State, enabled bool) []PlannedOp {
	if !enabled {
		return nil
	}
	goal := Goal{ID: "union.forest", Category: CategoryUnion, Domain: "union.forest", Label: "能量森林", Priority: 43}
	if !s.FmlForestEnergyObserved() {
		sync := domainOp(clientproto.RPCFmlForestRefresh.String(), goal, "union.forest", "collect", "能量森林状态未观测，先刷新并自动收集", 4430, 1, 0, 0)
		return []PlannedOp{sync}
	}
	energy := s.FmlForestEnergy()
	types := s.ReadyFmlForestEnergyTypes()
	if len(types) == 0 {
		return nil
	}
	collect := domainOp(clientproto.RPCFmlForestRefresh.String(), goal, "union.forest", "collect", "能量森林有临时能量可收集", 4430, 1, 0, energy.PendingTempEnergyTotal)
	return []PlannedOp{collect}
}

// unionRaceOperations emits PlannedOps for the guild race task pool.
// Lifecycle:
//  1. enter + getTaskList (sync) — runs when Enabled, even if AutoEnableModules is off
//  2. takeTask (接取) — requires AutoEnableModules; supports 种植收获、顾客订单、
//     珍珠采集雇佣、花艺制作/售卖、花种培育
//  3. progress — raceTaskProgressDemands drives plant/harvest for 种植收获;
//     顾客订单 / 珍珠雇佣 / 花艺 reuse ordinary (or race-owned flower-art) ops.
//     花种培育 is take/finish only: no progress demand; FinishCnt advances
//     outside race automation (manual / ordinary cultivate) and is synced via
//     getTaskList before finishTask
//  4. finishTask when TargetCnt > 0 && FinishCnt >= TargetCnt (完成并领取积分;
//     TargetCnt<=0 means unknown progress and must not auto-finish)
//
// useSpeedupTicketInTask is honored by maintenanceOperations via
// raceSpeedupEnabled while an unfinished plant-harvest task is held.
// Near ExpireTime (last 10 minutes), speedup tickets are always allowed as a
// forced completion guarantee even when the regular toggle is off.
func unionRaceOperations(s *state.State, policy *pb.UnionRacePolicy, uid int64, now time.Time, gates RaceModuleGates) []PlannedOp {
	if policy == nil || !policy.GetEnabled() {
		return nil
	}
	view := s.FmlRace()

	goal := Goal{ID: "union.race", Category: CategoryRace, Domain: "union.race", Label: "公会竞赛", Priority: 43}

	// Enter pushes CurFmlRaceBatch (111) and CurFmlRaceRcd (117, raceLvl).
	// Login may already carry 111 without 117; re-enter once while raceLvl is
	// unknown so task-quota totals use the correct guild tier (甲=18/乙=15/…).
	// Task pool / taken-task (114/110) require a follow-up getTaskList.
	// Neither sync step is gated by autoEnableModules.
	// Also re-fetch when a plant-harvest row is missing flower ParamID (once per
	// pool msId set — state.MissingParamRefreshFP prevents tight loops).
	if !view.Observed {
		// Outside an active/calendar contest, enter stays a normal side op so
		// farm/order work is not delayed. During the weekly window, login with
		// no batch yet must still enter before farm so the pool can be claimed.
		op := domainOp(clientproto.RPCFmlRaceEnter.String(), goal, "union.race.enter", "enter", "公会竞赛进入同步", 4400, 0, 0, 0)
		if state.FmlRaceCalendarInSession(now) {
			op.PreemptFarm = true
		}
		return []PlannedOp{op}
	}
	// Re-evaluate the published start/end window at planner time. Apply-time
	// BatchActive stays false if enter ran before Tuesday 09:00.
	view.BatchActive = view.ActiveAt(now)
	if view.BatchStatus != 1 {
		if raceShouldEnterInactiveBatch(view, now) {
			op := domainOp(clientproto.RPCFmlRaceEnter.String(), goal, "union.race.enter", "enter", "公会竞赛开赛同步批次", 4400, 0, 0, 0)
			op.CooldownKey = "union.race.enter.batch"
			op.PreemptFarm = true
			return []PlannedOp{op}
		}
		if !view.BatchActive {
			return nil
		}
	}
	if view.BatchActive && view.RaceLvl <= 0 && s.FmlBuild().RaceLvl <= 0 {
		const raceLvlSyncInterval = 10 * time.Minute
		synced := view.RaceLvlSyncAtMs
		if synced == 0 || !now.Before(time.UnixMilli(synced).Add(raceLvlSyncInterval)) {
			op := domainOp(clientproto.RPCFmlRaceEnter.String(), goal, "union.race.enter", "enter", "公会竞赛同步段位与任务配额", 4399, 0, 0, 0)
			op.CooldownKey = "union.race.enter.race_lvl"
			op.PreemptFarm = true
			return []PlannedOp{op}
		}
	}
	if !view.BatchActive {
		return nil
	}
	// enter/getTaskList may omit field 110; recover fTaskNum from member rank list
	// so UI「已做」and AutoStopOnQuotaDone work after restart.
	if !view.TaskQuotaObserved && view.BatchID > 0 {
		const raceQuotaSyncInterval = 10 * time.Minute
		synced := view.RaceQuotaSyncAtMs
		if synced == 0 || !now.Before(time.UnixMilli(synced).Add(raceQuotaSyncInterval)) {
			op := domainOp(
				clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String(), goal, "union.race.sync", "sync",
				"公会竞赛同步已做次数", 4398, 0, 0, 0,
			)
			op.TaskMsID = view.BatchID
			op.CooldownKey = "union.race.usr_rank"
			return []PlannedOp{op}
		}
	}
	if view.Taken.HasTask && raceTakenExpired(view.Taken, now) {
		if !view.TasksObserved || view.TasksSyncedAtMs <= 0 ||
			!now.Before(time.UnixMilli(view.TasksSyncedAtMs).Add(raceExpiredTaskSyncInterval)) {
			op := domainOp(
				clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
				"公会竞赛已接任务过期，重新同步任务池", 4399, 0, 0, 0,
			)
			op.PreemptFarm = true
			return []PlannedOp{op}
		}
		return nil
	}

	// Finish a completed held task before pool sync. Harvest ACKs update
	// FinishCnt via field 134; waiting for getTaskList here risks expire.
	if policy.GetAutoEnableModules() &&
		view.Taken.HasTask && view.Taken.TargetCnt > 0 && view.Taken.FinishCnt >= view.Taken.TargetCnt {
		return []PlannedOp{raceFinishOperation(goal, view.Taken)}
	}

	// Local harvest high-water already covers the target but server FinishCnt
	// still lags (missing/delayed field 134). Force getTaskList so pool field 8
	// can advance FinishCnt and finishTask can fire on the next tick.
	if policy.GetAutoEnableModules() && raceNeedsFinishProgressSync(view, now) {
		op := domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			"公会竞赛本地收获已达标，同步进度以便提交", 4397, 0, 0, 0,
		)
		op.PreemptFarm = true
		return []PlannedOp{op}
	}

	syncPrio := int32(4398)
	switch {
	case RaceHoldsUnfinishedCustomerOrder(view) && gates.Customer:
		syncPrio = raceCustomerSyncPriority
	case RaceHoldsUnfinishedPearlHire(view) && gates.Pearl:
		syncPrio = racePearlSyncPriority
	case RaceHoldsUnfinishedFlowerArtSell(view) || RaceHoldsUnfinishedFlowerArtCraft(view):
		syncPrio = raceFlowerArtSyncPriority
	// Cultivate holds (especially sticky score-36) may advance manually while
	// plant.cultivate is off; still elevate getTaskList so FinishCnt can catch up.
	case RaceHoldsUnfinishedFlowerCultivate(view):
		syncPrio = raceCultivateSyncPriority
	}
	if view.BatchActive && (!view.TasksObserved || raceTaskPoolNeedsParamRefresh(view)) {
		op := domainOp(clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync", "公会竞赛拉取任务池与已接任务", syncPrio, 0, 0, 0)
		op.PreemptFarm = true
		return []PlannedOp{op}
	}

	// autoEnableModules gates take/finish/upgrade/delete. When off, the race
	// module still syncs (enter/getTaskList + TTL refresh) so the task pool
	// remains visible, but does not auto-execute tasks.
	if !policy.GetAutoEnableModules() {
		if op, ok := raceUsrRankScoreSyncOp(view, goal, now); ok {
			return []PlannedOp{op}
		}
		if raceTaskPoolTTLStale(view, now) && !raceHasNearTakeableCD(s, view.Tasks, policy, uid, now, gates) {
			return []PlannedOp{domainOp(
				clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
				"公会竞赛定时刷新任务池", 4398, 0, 0, 0,
			)}
		}
		return nil
	}
	var ops []PlannedOp

	// 1a. Abandon a taken task that cannot or should not be kept.
	// TargetCnt<=0 means progress unknown (e.g. synthesized from pool UID) — treat as unfinished.
	// Tasks with FinishCnt>0 are kept (do not auto-cancel mid-progress).
	// Run before module progress sync so a disabled module can giveUp instead
	// of spinning on getTaskList.
	if view.Taken.HasTask && (view.Taken.TargetCnt <= 0 || view.Taken.FinishCnt < view.Taken.TargetCnt) {
		if reason := raceTakenAbandonReason(s, policy, view, gates); reason != "" {
			op := domainOp(clientproto.RPCFmlRaceGiveUpTask.String(), goal, "union.race.giveUp", "giveUp", reason, 4395, 0, 0, 0)
			op.TaskMsID = view.Taken.TaskMsId
			op.TaskID = view.Taken.TaskType
			if op.TaskID == 0 {
				op.TaskID = view.Taken.TaskId
			}
			op.FlowerID = view.Taken.ParamID
			op.PreemptFarm = true
			ops = append(ops, op)
		}
	}

	// Module-backed race progress needs getTaskList after ordinary finishes.
	// Customer/pearl sync only while those modules can still advance counters
	// (module-off holds are abandoned). Flower-art sync has no ordinary toggle
	// gate (race owns those ops). Flower-cultivate is take/finish only: sync
	// FinishCnt from manual / ordinary cultivate without driving progress.
	if len(ops) == 0 && gates.Customer && raceNeedsCustomerProgressSync(view, now) {
		return []PlannedOp{domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			"公会竞赛顾客订单进度同步", raceCustomerSyncPriority, 0, 0, 0,
		)}
	}
	if len(ops) == 0 && gates.Pearl && raceNeedsPearlProgressSync(view, now) {
		return []PlannedOp{domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			"公会竞赛珍珠雇佣进度同步", racePearlSyncPriority, 0, 0, 0,
		)}
	}
	if len(ops) == 0 && raceNeedsFlowerArtProgressSync(view, now) {
		reason := "公会竞赛花艺制作进度同步"
		if RaceHoldsUnfinishedFlowerArtSell(view) {
			reason = "公会竞赛花艺售卖进度同步"
		}
		return []PlannedOp{domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			reason, raceFlowerArtSyncPriority, 0, 0, 0,
		)}
	}
	if len(ops) == 0 && raceNeedsCultivateProgressSync(view, now) {
		return []PlannedOp{domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			"公会竞赛花种培育进度同步", raceCultivateSyncPriority, 0, 0, 0,
		)}
	}

	// 1b. Finish the current taken task if complete.
	// Require TargetCnt>0 so unknown progress (0/0) is never auto-finished.
	if view.Taken.HasTask && view.Taken.TargetCnt > 0 && view.Taken.FinishCnt >= view.Taken.TargetCnt {
		ops = append(ops, raceFinishOperation(goal, view.Taken))
	}

	// 2. Select a task to take (only if not currently holding one).
	// TakeQuotaExhausted is sticky for this batch after the server reports
	// 「任务接取次数已达上限」— do not keep retrying exhausted takes.
	// AutoStopOnQuotaDone also stops take when free-task quota is already used
	// (finished >= total), without waiting for a take rejection.
	if !view.Taken.HasTask && !view.TakeQuotaExhausted && !raceFreeTaskQuotaDone(s, view, policy) {
		selected := selectRaceTasks(s, view.Tasks, policy, uid, now, gates)
		if len(selected) > 0 {
			best := selected[0]
			op := domainOp(clientproto.RPCFmlRaceTakeTask.String(), goal, "union.race.take", "take", "公会竞赛选择最优任务接取", 4380, 0, 0, 0)
			op.TaskMsID = best.MsId
			op.TaskID = best.TaskType
			if op.TaskID == 0 {
				op.TaskID = best.TaskId
			}
			op.FlowerID = best.ParamID
			op.PreemptFarm = true
			ops = append(ops, op)
		}
	}

	hasPrimary := false
	for _, op := range ops {
		if isRacePrimaryMutatingOp(op) {
			hasPrimary = true
			break
		}
	}

	if !hasPrimary && raceTaskPoolTTLStale(view, now) && !raceHasNearTakeableCD(s, view.Tasks, policy, uid, now, gates) {
		return []PlannedOp{domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			"公会竞赛定时刷新任务池", 4398, 0, 0, 0,
		)}
	}

	// 3. Optional: upgrade the currently held task. The observed client sends
	// an empty upgradeTask request, so this RPC cannot target an arbitrary pool
	// row. Its diamond cost must be known and pass the configured budget; the
	// global diamond gate still blocks automatic execution by default.
	if policy.GetUpgradeTask() && view.Taken.HasTask {
		if task, ok := raceTaskByMsID(view.Tasks, view.Taken.TaskMsId); ok && task.IsUpgrade == 0 {
			op := domainOp(clientproto.RPCFmlRaceUpgradeTask.String(), goal, "union.race.upgrade", "upgrade", "公会竞赛当前任务可升级", 4370, 0, 0, 0)
			op.TaskMsID = task.MsId
			op.TaskID = task.TaskType
			if op.TaskID == 0 {
				op.TaskID = task.TaskId
			}
			op.FlowerID = task.ParamID
			cost, costKnown := state.FmlRaceTaskUpgradeCost(task.TaskId, task.Score)
			switch {
			case !costKnown:
				op.Status = PlanStatusAdapterMissing
				op.Executable = false
				op.BlockedReasons = []string{"公会竞赛任务升级成本无法从客户端配置确认"}
			case policy.GetMaxSpendDiamond() <= 0:
				op.DiamondCost = cost
				op.Status = PlanStatusBlocked
				op.Executable = false
				op.BlockedReasons = []string{"公会竞赛任务升级元宝预算未设置"}
			case int64(cost) > policy.GetMaxSpendDiamond():
				op.DiamondCost = cost
				op.Status = PlanStatusBlocked
				op.Executable = false
				op.BlockedReasons = []string{"公会竞赛任务升级元宝成本超过策略上限"}
			default:
				op.DiamondCost = cost
			}
			ops = append(ops, op)
		}
	}

	// 4. Optional: delete low-score tasks.
	if policy.GetDeleteLowScoreTask() {
		maxDel := policy.GetDeleteTaskMaxScore()
		if maxDel > 0 {
			for _, task := range view.Tasks {
				if task.UID == 0 && task.Score <= maxDel {
					op := domainOp(clientproto.RPCFmlRaceDelTask.String(), goal, "union.race.delete", "delete", "公会竞赛低分任务清理", 4360, 0, 0, 0)
					op.TaskMsID = task.MsId
					op.TaskID = task.TaskType
					if op.TaskID == 0 {
						op.TaskID = task.TaskId
					}
					op.FlowerID = task.ParamID
					ops = append(ops, op)
					break // one delete per cycle
				}
			}
		}
	}

	// Idle: sync personal score/rank without preempting take/finish/giveUp.
	// getTaskList also piggybacks a member-rank fetch for the common path.
	if len(ops) == 0 {
		if op, ok := raceUsrRankScoreSyncOp(view, goal, now); ok {
			return []PlannedOp{op}
		}
	}

	return ops
}

// raceShouldEnterInactiveBatch re-probes fmlRace.enter when the stored batch is
// not in-progress. Without this, an ended last-week snapshot (status==2) or a
// published future window (status==0) never discovers Tuesday 09:00 opening.
func raceShouldEnterInactiveBatch(view state.FmlRaceView, now time.Time) bool {
	if view.BatchStatus != 2 && view.BatchStartMs > 0 {
		start := time.UnixMilli(view.BatchStartMs)
		if now.Before(start) {
			return false
		}
		if view.BatchEndMs <= 0 || now.Before(time.UnixMilli(view.BatchEndMs)) {
			return raceInactiveEnterRetryDue(view, now, start)
		}
	}
	if !state.FmlRaceCalendarInSession(now) {
		return false
	}
	return raceInactiveEnterRetryDue(view, now, state.FmlRaceCalendarSessionStart(now))
}

func raceInactiveEnterRetryDue(view state.FmlRaceView, now, sessionStart time.Time) bool {
	if view.RaceLvlSyncAtMs <= 0 {
		return true
	}
	last := time.UnixMilli(view.RaceLvlSyncAtMs)
	if !sessionStart.IsZero() && last.Before(sessionStart) {
		return true
	}
	return !now.Before(last.Add(raceInactiveEnterRetryInterval))
}

func raceTaskPoolTTLStale(view state.FmlRaceView, now time.Time) bool {
	if !view.BatchActive || !view.TasksObserved {
		return false
	}
	if view.TasksSyncedAtMs <= 0 {
		return true
	}
	return !now.Before(time.UnixMilli(view.TasksSyncedAtMs).Add(raceTaskPoolRefreshInterval))
}

// raceNeedsFinishProgressSync reports that plant-harvest LocalFinishCnt already
// meets TargetCnt while authoritative FinishCnt has not, so the planner must
// refresh the task pool instead of idling until the 10m TTL.
// Recent successful getTaskList (within raceFinishProgressSyncInterval) is
// respected so a lagging server FinishCnt cannot re-plan sync every decision tick.
func raceNeedsFinishProgressSync(view state.FmlRaceView, now time.Time) bool {
	taken := view.Taken
	if !taken.HasTask || taken.TargetCnt <= 0 || taken.FinishCnt >= taken.TargetCnt {
		return false
	}
	if view.LocalFinishTaskMsId != taken.TaskMsId || view.LocalFinishCnt < taken.TargetCnt {
		return false
	}
	if view.TasksObserved && view.TasksSyncedAtMs > 0 &&
		now.Before(time.UnixMilli(view.TasksSyncedAtMs).Add(raceFinishProgressSyncInterval)) {
		return false
	}
	return true
}

func raceHasNearTakeableCD(s *state.State, tasks []state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64, now time.Time, gates RaceModuleGates) bool {
	nowMs := now.UnixMilli()
	for _, t := range tasks {
		if t.AppearTime <= 0 || t.AppearTime <= nowMs {
			continue
		}
		if t.UID != 0 {
			continue
		}
		rem := time.Duration(t.AppearTime-nowMs) * time.Millisecond
		if rem >= raceNearCDSyncSuppressWindow {
			continue
		}
		if raceTakeNonCDSkipReason(s, t, policy, uid, gates) == "" {
			return true
		}
	}
	return false
}

// RaceTakeWakeAt is when the decision loop should next tick to emit takeTask
// for a filter-passing CD pool row: AppearTime minus raceTakeLeadWindow.
// Zero means there is nothing to wait for (already holding, already takeable
// this tick, auto-complete off, or no eligible CD task).
func RaceTakeWakeAt(s *state.State, policy *pb.Policy, now time.Time) time.Time {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() {
		return time.Time{}
	}
	race := policy.GetUnion().GetRace()
	if race == nil || !race.GetEnabled() || !race.GetAutoEnableModules() {
		return time.Time{}
	}
	view := s.FmlRace()
	view.BatchActive = view.ActiveAt(now)
	if !view.BatchActive || !view.TasksObserved || view.Taken.HasTask ||
		view.TakeQuotaExhausted || raceFreeTaskQuotaDone(s, view, race) {
		return time.Time{}
	}
	uid := s.RoleID()
	gates := raceModuleGatesFromPolicy(policy)
	nowMs := now.UnixMilli()
	leadMs := raceTakeLeadWindow.Milliseconds()
	var bestAppear int64
	anyTakeableNow := false
	for _, t := range view.Tasks {
		if t.UID != 0 {
			continue
		}
		if raceTakeNonCDSkipReason(s, t, race, uid, gates) != "" {
			continue
		}
		if t.AppearTime <= 0 || t.AppearTime <= nowMs || t.AppearTime-nowMs <= leadMs {
			anyTakeableNow = true
			continue
		}
		if bestAppear == 0 || t.AppearTime < bestAppear {
			bestAppear = t.AppearTime
		}
	}
	if anyTakeableNow || bestAppear == 0 {
		return time.Time{}
	}
	return time.UnixMilli(bestAppear - leadMs)
}

// RaceTakeDue reports that takeTask should fire on this tick (ready now or
// inside raceTakeLeadWindow). The decision loop uses this to skip water /
// resident / reputation preamble so the 300ms lead is not eaten before RPC.
func RaceTakeDue(s *state.State, policy *pb.Policy, now time.Time) bool {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() {
		return false
	}
	race := policy.GetUnion().GetRace()
	if race == nil || !race.GetEnabled() || !race.GetAutoEnableModules() {
		return false
	}
	view := s.FmlRace()
	view.BatchActive = view.ActiveAt(now)
	if !view.BatchActive || !view.TasksObserved || view.Taken.HasTask ||
		view.TakeQuotaExhausted || raceFreeTaskQuotaDone(s, view, race) {
		return false
	}
	return len(selectRaceTasks(s, view.Tasks, race, s.RoleID(), now, raceModuleGatesFromPolicy(policy))) > 0
}

// RaceBootstrapDue reports that race enter / getTaskList / take / finish must
// run before farm or order work: unobserved pool after login, takeable rows, or
// a held task ready to submit.
func RaceBootstrapDue(s *state.State, policy *pb.Policy, now time.Time) bool {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() {
		return false
	}
	race := policy.GetUnion().GetRace()
	if race == nil || !race.GetEnabled() {
		return false
	}
	view := s.FmlRace()
	if !view.Observed {
		// First enter only preempts during the weekly contest window.
		return state.FmlRaceCalendarInSession(now)
	}
	view.BatchActive = view.ActiveAt(now)
	if !view.BatchActive {
		// Still allow calendar-window enter probes (status may still be 0).
		return raceShouldEnterInactiveBatch(view, now)
	}
	if !view.TasksObserved {
		return true
	}
	if view.Taken.HasTask && view.Taken.TargetCnt > 0 && view.Taken.FinishCnt >= view.Taken.TargetCnt {
		return race.GetAutoEnableModules()
	}
	if !race.GetAutoEnableModules() {
		return false
	}
	return RaceTakeDue(s, policy, now)
}

// IsUrgentRaceOp reports ops that must preempt farm/order lanes (login sync,
// take/giveUp/finish). Routine TTL refresh and bare off-week enter stay normal.
func IsUrgentRaceOp(op PlannedOp) bool {
	return op.PreemptFarm
}

// IsUrgentRaceDomain reports domains that may preempt farm/order when the
// planned op also carries PreemptFarm (see IsUrgentRaceOp).
func IsUrgentRaceDomain(domain string) bool {
	switch domain {
	case "union.race.enter", "union.race.sync", "union.race.take",
		"union.race.giveUp", "union.race.finish":
		return true
	default:
		return false
	}
}

func isRacePrimaryMutatingOp(op PlannedOp) bool {
	switch op.Kind {
	case clientproto.RPCFmlRaceGiveUpTask.String(),
		clientproto.RPCFmlRaceFinishTask.String(),
		clientproto.RPCFmlRaceTakeTask.String():
		return true
	default:
		return false
	}
}

// raceUsrRankScoreSyncOp plans getFmlRaceUsrRankList for personal score/rank
// when missing, or periodically after a dedicated sync. Callers must only use
// this when it would not preempt primary race take/finish/giveUp work.
func raceUsrRankScoreSyncOp(view state.FmlRaceView, goal Goal, now time.Time) (PlannedOp, bool) {
	if view.BatchID <= 0 {
		return PlannedOp{}, false
	}
	needScoreRank := !view.ScoreObserved || !view.RankObserved
	const raceScoreRankSyncInterval = 10 * time.Minute
	synced := view.RaceQuotaSyncAtMs
	backoffOK := synced == 0 || !now.Before(time.UnixMilli(synced).Add(raceScoreRankSyncInterval))
	periodic := !needScoreRank && synced > 0 && !now.Before(time.UnixMilli(synced).Add(raceScoreRankSyncInterval))
	if (!needScoreRank || !backoffOK) && !periodic {
		return PlannedOp{}, false
	}
	op := domainOp(
		clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String(), goal, "union.race.sync", "sync",
		"公会竞赛同步个人得分与排名", 4398, 0, 0, 0,
	)
	op.TaskMsID = view.BatchID
	op.CooldownKey = "union.race.usr_rank"
	return op, true
}

// raceFreeTaskQuotaDone reports that AutoStopOnQuotaDone should block further
// takeTask planning: usr-rcd quota was observed and finished_task_num already
// covers the free tier total (c_fmlRace(raceLvl).taskNum). Purchased extras
// (buyTaskNum) are intentionally ignored so automation stops at the UI「已做」
// free quota. Unknown raceLvl / unobserved quota returns false.
func raceFreeTaskQuotaDone(s *state.State, view state.FmlRaceView, policy *pb.UnionRacePolicy) bool {
	if policy == nil || !policy.GetAutoStopOnQuotaDone() {
		return false
	}
	if !view.TaskQuotaObserved {
		return false
	}
	raceLvl := view.RaceLvl
	if raceLvl <= 0 && s != nil {
		raceLvl = s.FmlBuild().RaceLvl
	}
	total := state.FmlRaceTotalTaskNum(raceLvl, view.BuyTaskNum)
	if total <= 0 {
		return false
	}
	return view.FinishedTaskNum >= total
}

func raceFinishOperation(goal Goal, taken state.FmlRaceTakenView) PlannedOp {
	prio := int32(4390)
	taskType := taken.TaskType
	if taskType == 0 {
		taskType = taken.TaskId
	}
	switch taskType {
	case raceTaskTypeCustomerOrder:
		prio = raceCustomerFinishPriority
	case raceTaskTypePearlHire:
		prio = racePearlFinishPriority
	case raceTaskTypeFlowerArtSell, raceTaskTypeFlowerArtCraft:
		prio = raceFlowerArtFinishPriority
	case raceTaskTypeFlowerCultivate:
		prio = raceCultivateFinishPriority
	}
	op := domainOp(clientproto.RPCFmlRaceFinishTask.String(), goal, "union.race.finish", "finish", "公会竞赛任务已完成，提交领取积分", prio, 0, 0, 0)
	op.TaskMsID = taken.TaskMsId
	op.TaskID = taskType
	if op.TaskID == 0 {
		op.TaskID = taken.TaskId
	}
	op.FlowerID = taken.ParamID
	op.PreemptFarm = true
	return op
}

// RaceTakeSkipReason returns the primary reason automation will not take this
// pool task, or "" if it is takeable (including preemptive CD within raceTakeLeadWindow).
// Priority and CD copy branching are documented in docs/guild-race.md.
func RaceTakeSkipReason(s *state.State, t state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64, now time.Time, gates RaceModuleGates) string {
	if t.UID != 0 {
		return "已被接取"
	}
	leadUntil := now.Add(raceTakeLeadWindow).UnixMilli()
	if t.AppearTime > 0 && t.AppearTime > leadUntil {
		hhmmss := time.UnixMilli(t.AppearTime).Local().Format("15:04:05")
		if raceTakeNonCDSkipReason(s, t, policy, uid, gates) != "" {
			return hhmmss + " 后刷新"
		}
		return "冷却中，" + hhmmss + " 后可接"
	}
	return raceTakeNonCDSkipReason(s, t, policy, uid, gates)
}

// raceTakeNonCDSkipReason evaluates take filters other than far-CD AppearTime.
// Empty means those filters would allow take (ready / within-lead still apply outside).
func raceTakeNonCDSkipReason(s *state.State, t state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64, gates RaceModuleGates) string {
	if policy.GetMinTaskScore() > 0 && t.Score <= policy.GetMinTaskScore() {
		return fmt.Sprintf("分数不足（≤%d）", policy.GetMinTaskScore())
	}
	if policy.GetOnlyUpgradeTask() && t.IsUpgrade == 0 {
		return "仅接已升级任务"
	}
	// Only member upgrades carry UpgradeUid. UpgradeUid==0 is system upgrade
	// and stays takeable when exclude-others is on.
	if policy.GetExcludeOthersUpgradeTask() && t.UpgradeUid != 0 && t.UpgradeUid != uid {
		return "他人已升级"
	}
	taskType := t.TaskType
	if taskType == 0 {
		taskType = t.TaskId
	}
	if raceTaskTypePriority(policy, taskType) <= 0 {
		return "优先级为0"
	}
	if !raceTaskTypeAutoCompletable(taskType) {
		return "暂不支持自动完成"
	}
	switch taskType {
	case raceTaskTypePlantHarvest:
		if t.ParamID <= 0 || !flowerCultivated(s, t.ParamID) {
			return "目标花卉未培养"
		}
	case raceTaskTypeCustomerOrder:
		if !gates.Customer {
			return "顾客订单模块未开启"
		}
	case raceTaskTypePearlHire:
		if !gates.Pearl {
			return "珍珠雇佣模块未开启"
		}
	case raceTaskTypeFlowerCultivate:
		// Take does not require plant.cultivate. Race does not drive cultivate
		// ops — only take / sync FinishCnt / finishTask.
		if t.Score != raceFlowerCultivateRequiredScore {
			return fmt.Sprintf("仅接%d分花种培育", raceFlowerCultivateRequiredScore)
		}
		if t.FinishCnt > 0 {
			return "仅接进度为0的花种培育"
		}
	}
	return ""
}

func raceTaskByMsID(tasks []state.FmlRaceTaskView, msID int64) (state.FmlRaceTaskView, bool) {
	for _, task := range tasks {
		if task.MsId == msID {
			return task, true
		}
	}
	return state.FmlRaceTaskView{}, false
}

// raceTaskPoolNeedsParamRefresh reports whether getTaskList should run again
// because at least one plant-harvest pool task has no flower ParamID and this
// incomplete pool identity has not yet been refresh-attempted.
func raceTaskPoolNeedsParamRefresh(view state.FmlRaceView) bool {
	if !state.FmlRacePlantHarvestMissingParam(view.Tasks) {
		return false
	}
	return state.FmlRaceTaskPoolMsFingerprint(view.Tasks) != view.MissingParamRefreshFP
}

// raceTaskTypePriority returns the configured priority for a race task type.
// Missing map entries fall back to defaultUnionRacePriority (0 = do not take).
func raceTaskTypePriority(policy *pb.UnionRacePolicy, taskType int32) int32 {
	if m := policy.GetTaskTypePriority(); m != nil {
		if p, ok := m[taskType]; ok {
			return p
		}
	}
	return defaultUnionRacePriority()[taskType]
}

// selectRaceTasks filters the available task pool via RaceTakeSkipReason, then
// sorts by configured priority.
//
// min_task_score is a lower bound: tasks with Score <= minScore are skipped.
// 0 means no score filtering. Combined with only_upgrade_task, only upgraded tasks
// above the threshold are eligible.
//
// task_type_priority: 0 (or missing → default 0) means do not take that type.
// Positive values rank candidates (higher first), then Score descending.
//
// Plant-harvest (3036): skip when ParamID is missing or the flower is not yet
// cultivated (Status==2 && Lvl>0). Seed stock / empty land are not required.
//
// Flower-art sell (3030) / craft (3034): race auto-complete drives
// flowerRack.sell / makeFlowerArt itself; order.flower_art toggles are not
// required for take or progress.
//
// Flower-cultivate (3044): only Score==36 and FinishCnt==0; plant.cultivate is
// not required. Race does not drive cultivate ops — only take, progress sync,
// and finishTask once FinishCnt catches up.
//
// AppearTime gating: ready tasks (appearTime already due) are preferred. CD tasks
// within raceTakeLeadWindow may be selected preemptively when no ready candidate
// remains; farther CD tasks are skipped.
func selectRaceTasks(s *state.State, tasks []state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64, now time.Time, gates RaceModuleGates) []state.FmlRaceTaskView {
	nowMs := now.UnixMilli()

	var ready, upcoming []state.FmlRaceTaskView
	for _, t := range tasks {
		if RaceTakeSkipReason(s, t, policy, uid, now, gates) != "" {
			continue
		}
		if t.AppearTime > 0 && t.AppearTime > nowMs {
			// Within lead (otherwise skip reason would be non-empty).
			upcoming = append(upcoming, t)
			continue
		}
		ready = append(ready, t)
	}

	sortRaceTasks := func(list []state.FmlRaceTaskView) {
		sort.SliceStable(list, func(i, j int) bool {
			pi := int(raceTaskTypePriority(policy, list[i].TaskType))
			pj := int(raceTaskTypePriority(policy, list[j].TaskType))
			if pi != pj {
				return pi > pj
			}
			return list[i].Score > list[j].Score
		})
	}
	sortRaceTasks(ready)
	sortRaceTasks(upcoming)
	if len(ready) > 0 {
		return ready
	}
	return upcoming
}

// raceTakenAbandonReason returns a non-empty give-up reason when a held
// unfinished task should not be kept. Callers must only invoke this for
// unfinished taken tasks (unknown progress TargetCnt<=0 counts as unfinished).
//
// Order (auto-complete on):
//  1. Flower-cultivate at the required score (36): never give up once held
//     (manual take, priority 0, module off, min_task_score, missing pool row).
//     Race does not drive cultivate progress — only sync + finishTask.
//  2. Score <= min_task_score (pool score fills Taken.Score==0; still-unresolved
//     Score==0 → do not give up for score alone). Fires even when FinishCnt>0 so
//     a sub-threshold hold is dropped instead of planted to completion.
//  3. Flower-cultivate with known score other than 36 → give up.
//  4. Plant-harvest uncompletable / customer / pearl module off / priority 0
//     (also with progress). Flower-cultivate is never abandoned for a missing
//     plant.cultivate toggle. Flower-art sell/craft never abandons for a
//     missing ordinary sell/craft toggle.
//  5. Pool observed and TaskMsId missing from pool → give up only when FinishCnt==0
//     (mid-progress keep avoids dropping a live task on a transient pool gap)
func raceTakenAbandonReason(s *state.State, policy *pb.UnionRacePolicy, view state.FmlRaceView, gates RaceModuleGates) string {
	taken := view.Taken
	if policy == nil || !taken.HasTask {
		return ""
	}
	score := raceTakenScore(view)
	minScore := policy.GetMinTaskScore()
	taskType := taken.TaskType
	if taskType == 0 {
		taskType = taken.TaskId
	}
	// 36-point flower-cultivate: never give up once held (including manually
	// taken holds and priority 0). Race only takes/finishes; progress is external.
	if taskType == raceTaskTypeFlowerCultivate && score == raceFlowerCultivateRequiredScore {
		return ""
	}
	switch {
	case minScore > 0 && score > 0 && score <= minScore:
		return "公会竞赛放弃不符合分数要求的已接任务"
	case taskType == raceTaskTypeFlowerCultivate && score > 0 && score != raceFlowerCultivateRequiredScore:
		return fmt.Sprintf("公会竞赛放弃非%d分花种培育任务", raceFlowerCultivateRequiredScore)
	case raceTakenUncompletable(s, taken, gates):
		switch taskType {
		case raceTaskTypeCustomerOrder:
			return "公会竞赛放弃无法完成的顾客订单任务"
		case raceTaskTypePearlHire:
			return "公会竞赛放弃无法完成的珍珠雇佣任务"
		default:
			return "公会竞赛放弃无法完成的种植收获任务"
		}
	case raceTakenPriorityZero(policy, taken):
		return "公会竞赛放弃优先级为0的已接任务"
	}
	if taken.FinishCnt > 0 {
		return ""
	}
	if view.TasksObserved {
		if _, ok := raceTaskByMsID(view.Tasks, taken.TaskMsId); !ok {
			return "公会竞赛放弃不在任务池中的已接任务"
		}
	}
	return ""
}

// raceTakenScore prefers the held-task score, then the matching pool row when
// field 110 omitted score and enrichment has not filled it yet.
func raceTakenScore(view state.FmlRaceView) int32 {
	if view.Taken.Score > 0 {
		return view.Taken.Score
	}
	if task, ok := raceTaskByMsID(view.Tasks, view.Taken.TaskMsId); ok && task.Score > 0 {
		return task.Score
	}
	return 0
}

// raceTakenBlocksProgress reports whether farm modules must not advance a held
// race task — either it is about to be given up, or its score is still unknown
// while a min_task_score gate is active (planting before score resolves caused
// full-field race plants of sub-threshold tasks). Started tasks (FinishCnt>0)
// are never blocked by the score-unresolved gate alone.
func raceTakenBlocksProgress(s *state.State, policy *pb.UnionRacePolicy, view state.FmlRaceView, gates RaceModuleGates) bool {
	taken := view.Taken
	if !taken.HasTask {
		return false
	}
	if raceTakenAbandonReason(s, policy, view, gates) != "" {
		return true
	}
	if taken.FinishCnt > 0 {
		return false
	}
	return policy != nil && policy.GetMinTaskScore() > 0 && raceTakenScore(view) == 0
}

// raceTakenUncompletable reports whether a held unfinished task can never be
// progressed by automation — plant-harvest with a missing/unplantable target, or
// customer/pearl while the ordinary module is off. Flower-art sell/craft are
// race-driven. Flower-cultivate is take/finish only and is never treated as
// uncompletable for a missing plant.cultivate toggle.
func raceTakenUncompletable(s *state.State, taken state.FmlRaceTakenView, gates RaceModuleGates) bool {
	taskType := taken.TaskType
	if taskType == 0 {
		taskType = taken.TaskId
	}
	switch taskType {
	case raceTaskTypePlantHarvest:
		return taken.ParamID <= 0 || !flowerCultivated(s, taken.ParamID)
	case raceTaskTypeCustomerOrder:
		return !gates.Customer
	case raceTaskTypePearlHire:
		return !gates.Pearl
	default:
		return false
	}
}

// raceTakenPriorityZero reports whether a held task's type is configured at
// priority 0 (do not take / should give up).
func raceTakenPriorityZero(policy *pb.UnionRacePolicy, taken state.FmlRaceTakenView) bool {
	taskType := taken.TaskType
	if taskType == 0 {
		taskType = taken.TaskId
	}
	return raceTaskTypePriority(policy, taskType) <= 0
}
