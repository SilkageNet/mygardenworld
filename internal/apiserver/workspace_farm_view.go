package apiserver

import (
	"fmt"
	"sort"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func vasesProto(vases map[int32]state.VaseView) []*pb.VaseView {
	ids := make([]int32, 0, len(vases))
	for id := range vases {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]*pb.VaseView, 0, len(ids))
	for _, id := range ids {
		vase := vases[id]
		out = append(out, &pb.VaseView{VaseId: vase.VaseID, UTimeMs: vase.UTimeMs, CTimeMs: vase.CTimeMs})
	}
	return out
}

func flowerArtAvailabilityProto(st *state.State, plan automation.PlanResult) []*pb.FlowerArtAvailabilityView {
	countByArt := map[int32]int32{}
	for _, demand := range plan.Demands {
		if demand.Kind == automation.DemandKindFlowerArt && demand.ItemID > 0 {
			count := demand.Missing
			if count <= 0 {
				count = demand.Count
			}
			if count > countByArt[demand.ItemID] {
				countByArt[demand.ItemID] = count
			}
		}
	}
	for _, op := range plan.Operations {
		if op.ItemID > 0 {
			if _, ok := state.FlowerArtRecipeByID(op.ItemID); ok {
				count := op.Count
				if count <= 0 {
					count = 1
				}
				if count > countByArt[op.ItemID] {
					countByArt[op.ItemID] = count
				}
			}
		}
	}
	ids := make([]int32, 0, len(countByArt))
	for id := range countByArt {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]*pb.FlowerArtAvailabilityView, 0, len(ids))
	for _, id := range ids {
		count := countByArt[id]
		if count <= 0 {
			count = 1
		}
		availability := automation.FlowerArtAvailability(st, id, count, plan.Ledger)
		if availability.Recipe.ArtID == 0 {
			continue
		}
		view := &pb.FlowerArtAvailabilityView{
			ArtId:          availability.Recipe.ArtID,
			ArtName:        itemNameOrID(availability.Recipe.ArtID),
			VaseId:         availability.Recipe.VaseID,
			Level:          availability.Recipe.Level,
			SaleValue:      availability.Recipe.SaleValue,
			LevelOk:        true,
			VaseUnlocked:   availability.VaseUnlocked,
			Craftable:      availability.Craftable,
			BlockedReasons: append([]string(nil), availability.BlockedReasons...),
		}
		for _, req := range availability.Requirements {
			view.Requirements = append(view.Requirements, requirementView(req.ItemID, req.Count, req.Have))
		}
		out = append(out, view)
	}
	return out
}

func orderStatisticsProto(st *state.State, now time.Time) *pb.OrderStatisticsView {
	stats := st.Statistics()
	out := &pb.OrderStatisticsView{
		Observed:                 stats.Observed,
		DayId:                    stats.DayID,
		ResidentNormalFinished:   st.ResidentOrderFinishNum(now),
		PalaceFinished:           stats.OrderPalaceFinishNum,
		CustomerFinished:         st.CustomerOrderFinishNum(now),
		ResidentSatinFinished:    st.ResidentSatinOrderFinishNum(now),
		ResidentDecorateFinished: st.ResidentDecorateOrderFinishNum(now),
		FlowerArtSold:            stats.FlowerArtSellNum,
		UpdatedAtMs:              stats.UTimeMs,
		CreatedAtMs:              stats.CTimeMs,
	}
	if !stats.Observed {
		out.BlockedReasons = []string{"未观察到订单统计 namespace 124"}
	}
	return out
}

func businessStatisticsProto(st *state.State) *pb.BusinessStatisticsView {
	days := st.StatisticsDays()
	out := &pb.BusinessStatisticsView{Observed: st.Statistics().Observed}
	if len(days) == 0 {
		return out
	}
	out.Days = make([]*pb.DailyBusinessStatisticsView, 0, len(days))
	for _, day := range days {
		out.Days = append(out.Days, dailyBusinessStatisticsProto(day))
	}
	out.Today = out.Days[0]
	return out
}

func dailyBusinessStatisticsProto(stats state.StatisticsView) *pb.DailyBusinessStatisticsView {
	return &pb.DailyBusinessStatisticsView{
		DayId:                    stats.DayID,
		Gold:                     stats.Gold,
		Experience:               stats.Experience,
		Diamonds:                 stats.Diamonds,
		SpeedUpCard:              stats.SpeedUpCard,
		FlowerShopCoin:           stats.FlowerShopCoin,
		FlowerHarvestNum:         stats.FlowerHarvestNum,
		FlowerArtSold:            stats.FlowerArtSellNum,
		ResidentNormalFinished:   stats.OrderFlowerFinishNum,
		PalaceFinished:           stats.OrderPalaceFinishNum,
		CustomerFinished:         stats.OrderCustomerFinishNum,
		ResidentSatinFinished:    stats.OrderSatinFinishNum,
		Satin:                    stats.Satin,
		ResidentDecorateFinished: stats.OrderDecorateFinishNum,
		Wood:                     stats.Wood,
		UpdatedAtMs:              stats.UTimeMs,
		CreatedAtMs:              stats.CTimeMs,
	}
}

func inventoryLedgerProto(inventory map[int32]int32, ledger *automation.InventoryLedger) *pb.InventoryLedgerView {
	ids := make([]int32, 0, len(inventory))
	seen := map[int32]struct{}{}
	for itemID, count := range inventory {
		if count <= 0 {
			continue
		}
		ids = append(ids, itemID)
		seen[itemID] = struct{}{}
	}
	for itemID := range ledger.AllocatedItems() {
		if _, ok := seen[itemID]; ok {
			continue
		}
		ids = append(ids, itemID)
		seen[itemID] = struct{}{}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := &pb.InventoryLedgerView{Items: make([]*pb.InventoryLedgerItem, 0, len(ids))}
	for _, itemID := range ids {
		owned := inventory[itemID]
		allocated := int32(0)
		available := owned
		if ledger != nil {
			owned = ledger.Owned(itemID)
			allocated = ledger.Allocated(itemID)
			available = ledger.Available(itemID)
		}
		out.Items = append(out.Items, &pb.InventoryLedgerItem{
			ItemId:    itemID,
			ItemName:  itemNameOrID(itemID),
			Owned:     owned,
			Allocated: allocated,
			Available: available,
		})
	}
	return out
}

func plantableFlowersProto(flowers []state.PlantableFlower) []*pb.PlantableFlowerView {
	if len(flowers) == 0 {
		return nil
	}
	out := make([]*pb.PlantableFlowerView, 0, len(flowers))
	for _, flower := range flowers {
		var cdSeconds int32
		if cd, ok := state.FlowerLvlCDSeconds(flower.FlowerID, flower.Lvl); ok {
			cdSeconds = cd
		}
		out = append(out, &pb.PlantableFlowerView{
			FlowerId:   flower.FlowerID,
			FlowerName: itemNameOrID(flower.FlowerID),
			Stock:      flower.Stock,
			Gold:       flower.Gold,
			Experience: flower.Experience,
			Lvl:        flower.Lvl,
			CdSeconds:  cdSeconds,
		})
	}
	return out
}

func sellableFlowerArtsProto(st *state.State) []*pb.SellableFlowerArtView {
	if st == nil {
		return nil
	}
	inventory := st.Inventory()
	out := make([]*pb.SellableFlowerArtView, 0)
	for _, recipe := range state.AllFlowerArtRecipes() {
		if st.VaseObserved() && !st.HasVase(recipe.VaseID) {
			continue
		}
		stock := inventory[recipe.ArtID]
		out = append(out, &pb.SellableFlowerArtView{
			ArtId:     recipe.ArtID,
			ArtName:   itemNameOrID(recipe.ArtID),
			VaseId:    recipe.VaseID,
			VaseName:  itemNameOrID(recipe.VaseID),
			Stock:     stock,
			SaleValue: recipe.SaleValue,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func friendTouchFriendsProto(friends []state.FriendTouchFriendView) []*pb.FriendTouchFriendView {
	if len(friends) == 0 {
		return nil
	}
	out := make([]*pb.FriendTouchFriendView, 0, len(friends))
	for _, friend := range friends {
		out = append(out, &pb.FriendTouchFriendView{
			Uid:                  friend.UID,
			Name:                 friend.Name,
			StolenCount:          friend.StolenCount,
			StealMax:             friend.StealMax,
			StealLeft:            friend.StealLeft,
			CanSteal:             friend.CanSteal,
			ProfileObserved:      friend.ProfileObserved,
			BaseStealMax:         friend.BaseStealMax,
			BoughtCount:          friend.BoughtCount,
			QuotaObserved:        friend.QuotaObserved,
			AvailabilityObserved: friend.AvailabilityObserved,
		})
	}
	return out
}

func blockingSummaryProto(domainStatuses []*pb.DomainStatus, plan automation.PlanResult) *pb.BlockingSummary {
	type groupKey struct {
		category string
		domain   string
		stage    string
		status   pb.PlanStatus
	}
	groups := map[groupKey]*pb.BlockingGroup{}
	add := func(category, domain, stage string, status pb.PlanStatus, reasons []string) {
		if len(reasons) == 0 {
			return
		}
		if stage == "" {
			stage = "unknown"
		}
		key := groupKey{category: category, domain: domain, stage: stage, status: status}
		group := groups[key]
		if group == nil {
			group = &pb.BlockingGroup{Category: category, Domain: domain, Stage: stage, Status: status}
			groups[key] = group
		}
		group.Count++
		for _, reason := range reasons {
			if reason == "" || containsString(group.Reasons, reason) {
				continue
			}
			group.Reasons = append(group.Reasons, reason)
		}
	}
	for _, domain := range domainStatuses {
		add(domain.GetCategory(), domain.GetDomain(), "domain", pb.PlanStatus_PLAN_STATUS_BLOCKED, domain.GetBlockedReasons())
	}
	for _, demand := range plan.Demands {
		add(demand.Category, demand.Domain, demand.BlockingStage, planStatusProto(demand.Status), demand.BlockedReasons)
	}
	for _, op := range plan.Operations {
		add(op.Category, op.Domain, op.BlockingStage, planStatusProto(op.Status), op.BlockedReasons)
		for _, gate := range op.CostGates {
			add(op.Category, op.Domain, gate.Source, planStatusProto(gate.Status), gate.BlockedReasons)
		}
	}
	keys := make([]groupKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].category != keys[j].category {
			return keys[i].category < keys[j].category
		}
		if keys[i].domain != keys[j].domain {
			return keys[i].domain < keys[j].domain
		}
		if keys[i].stage != keys[j].stage {
			return keys[i].stage < keys[j].stage
		}
		return keys[i].status < keys[j].status
	})
	out := &pb.BlockingSummary{}
	for _, key := range keys {
		group := groups[key]
		out.Total += group.Count
		out.Groups = append(out.Groups, group)
	}
	return out
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func itemNameOrID(itemID int32) string {
	name := state.ItemName(itemID)
	if name != "" {
		return name
	}
	return fmt.Sprintf("#%d", itemID)
}
