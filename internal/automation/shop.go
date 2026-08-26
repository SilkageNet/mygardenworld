package automation

import (
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

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
	goal := Goal{ID: "basic.shop.giftbag", Category: CategoryBasic, Domain: "basic.shop.giftbag", Label: "免费礼包", Priority: 54}
	for _, offer := range s.ShopGiftbagOffers() {
		if !zeroCostGiftbag(offer) || offer.Remaining <= 0 {
			continue
		}
		if offer.ShareID > 0 {
			blocked := markerOp(CategoryBasic, "basic.shop.video_gift", "claim", "视频礼包需要广告 SDK 回调，已拒绝自动领取", 5470)
			blocked.TargetID = offer.ShopID
			blocked.Status = PlanStatusAdapterMissing
			blocked.Executable = false
			blocked.BlockedReasons = []string{SDKAdUnsupportedReason}
			return []PlannedOp{blocked}
		}
		// A future zero-cost offer may be automated only when the observed
		// protocol explicitly has no advertising/share proof requirement.
		if !s.ShopGiftbagObserved() {
			return []PlannedOp{domainOp(clientproto.RPCShopGiftbagEnter.String(), goal, "basic.shop.giftbag", "sync", "免费礼包状态未同步，先进入商店获取领取记录", 5480, 0, 0, 0)}
		}
		buy := domainOp(clientproto.RPCShopGiftbagBuy.String(), goal, "basic.shop.giftbag", "claim", "无广告回调要求的免费礼包可领取", 5470, offer.ShopID, 0, 1)
		return []PlannedOp{buy}
	}
	return nil
}

func zeroCostGiftbag(offer state.ShopGiftbagOfferView) bool {
	return offer.Type == 1 && offer.RchgID == 0 &&
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
