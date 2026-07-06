package automation

import (
	"fmt"
	"strings"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func shopOperations(s *state.State, policy *pb.Policy) []PlannedOp {
	shop := policy.GetBasic().GetShop()
	var ops []PlannedOp
	ops = append(ops, giftbagOperations(s, shop)...)
	ops = append(ops, cultivateShopOperations(s, shop)...)
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

func cultivateShopOperations(s *state.State, shop *pb.ShopPolicy) []PlannedOp {
	cultivateShop := shop.GetCultivateShop()
	if !cultivateShop.GetAutoBuy() {
		return nil
	}
	goal := Goal{ID: "basic.shop.cultivate", Category: CategoryBasic, Domain: "basic.shop.cultivate", Label: "材料商店", Priority: 54}
	if !s.ShopCultivateObserved() {
		return []PlannedOp{domainOp(clientproto.RPCShopCultivateEnter.String(), goal, "basic.shop.cultivate", "sync", "材料商店状态未同步，先进入商店获取价格", 5450, 0, 0, 0)}
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

func unionOperations(s *state.State, union *pb.UnionPolicy) []PlannedOp {
	if union == nil {
		return nil
	}
	var ops []PlannedOp
	ops = append(ops, unionBuildOperations(s, union.GetBuild())...)
	ops = append(ops, unionFlowerOperations(s, union.GetFlower())...)
	ops = append(ops, unionLandOperations(s, union.GetLand())...)
	ops = append(ops, unionForestOperations(s, union.GetForestEnabled())...)
	return ops
}

func unionFlowerOperations(s *state.State, policy *pb.UnionFlowerPolicy) []PlannedOp {
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
		if !s.OtherFmlFlowerSharesObserved() {
			sync := domainOp(clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String(), goal, "union.flower.take", "take", "公会摸花列表未观测，先同步", 4460, 0, 0, 0)
			ops = append(ops, sync)
		} else {
			for _, candidate := range s.FmlFlowerTakeCandidates() {
				if !unionFlowerTakeAllowed(candidate, policy) {
					continue
				}
				take := domainOp(clientproto.RPCFmlFlowerShareTake.String(), goal, "union.flower.take", "take", "公会成员分享鲜花可摸取", 4460, candidate.SlotID, 0, 1)
				take.TargetUID = candidate.UID
				take.FlowerID = candidate.FlowerID
				ops = append(ops, take)
				break
			}
		}
	}
	return ops
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
		blocked := domainOp(clientproto.RPCFmlBuild.String(), goal, "union.build", "build", "公会建设状态未观测", 4590, 0, 0, 0)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"未观察到公会 namespace 25，需先进入公会或补充 fml.enter 同步链路"}
		return []PlannedOp{blocked}
	}
	build := s.FmlBuild()
	if !build.BuildCountsObserved {
		blocked := domainOp(clientproto.RPCFmlBuild.String(), goal, "union.build", "build", "公会建设次数未观测", 4590, 0, 0, 0)
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
			blocked := domainOp(clientproto.RPCFmlBuild.String(), goal, "union.build", "build", "公会建设档位配置缺失", 4500-id, id, 0, 0)
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
		buildOp := domainOp(clientproto.RPCFmlBuild.String(), goal, "union.build", "build", reason+"可执行", 4500-id, id, 0, 1)
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
	if option.ItemID <= 0 || option.Cost <= 0 {
		return nil
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

func unionLandOperations(s *state.State, policy *pb.UnionLandPolicy) []PlannedOp {
	if policy == nil || !policy.GetHarvestEnabled() {
		return nil
	}
	goal := Goal{ID: "union.land", Category: CategoryUnion, Domain: "union.land", Label: "公会土地", Priority: 44}
	if !s.FmlLandObserved() {
		blocked := domainOp(clientproto.RPCFmlLandHarvest.String(), goal, "union.land.harvest", "harvest", "公会土地状态未观测", 4480, 0, 0, 0)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"未观察到 25.102.fmlLand，需先进入公会土地或等待同步"}
		return []PlannedOp{blocked}
	}
	landIDs := s.ReadyFmlLandHarvestIDs()
	if len(landIDs) == 0 {
		return nil
	}
	harvest := domainOp(clientproto.RPCFmlLandHarvest.String(), goal, "union.land.harvest", "harvest", "公会土地有成熟鲜花可收获", 4480, 0, 0, int32(len(landIDs)))
	harvest.LandIDs = landIDs
	return []PlannedOp{harvest}
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
