package automation

import (
	"fmt"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"sort"
	"strconv"
	"strings"
)

func landOp(kind, domain, action, reason string, priority int32, landIDs []int32, flowerID int32, goalID, demandID string) PlannedOp {
	op := PlannedOp{
		OperationID: operationID(kind, landIDs, flowerID, 0, 0),
		GoalID:      goalID,
		DemandID:    demandID,
		Kind:        kind,
		Lane:        laneForDomain(domain),
		Category:    CategoryPlant,
		Domain:      domain,
		Action:      action,
		Reason:      reason,
		Priority:    priority,
		LandIDs:     append([]int32(nil), landIDs...),
		FlowerID:    flowerID,
	}
	return enrichPlannedOp(op)
}

func markerOp(category, domain, action, reason string, priority int32) PlannedOp {
	op := PlannedOp{
		OperationID: operationID(domain+"."+action, nil, 0, 0, 0),
		Kind:        domain + "." + action,
		Lane:        laneForDomain(domain),
		Category:    category,
		Domain:      domain,
		Action:      action,
		Reason:      reason,
		Priority:    priority,
	}
	return enrichPlannedOp(op)
}

func op(kind string, goal Goal, action, reason string, priority, targetID, itemID, count int32) PlannedOp {
	out := PlannedOp{
		OperationID: operationID(kind, nil, 0, targetID, itemID),
		GoalID:      goal.ID,
		Kind:        kind,
		Lane:        laneForDomain(goal.Domain),
		Category:    goal.Category,
		Domain:      goal.Domain,
		Action:      action,
		Reason:      reason,
		Priority:    priority,
		TargetID:    targetID,
		ItemID:      itemID,
		Count:       count,
	}
	return enrichPlannedOp(out)
}

func domainOp(kind string, goal Goal, domain, action, reason string, priority, targetID, itemID, count int32) PlannedOp {
	out := PlannedOp{
		OperationID: operationID(kind, nil, 0, targetID, itemID),
		GoalID:      goal.ID,
		Kind:        kind,
		Lane:        laneForDomain(domain),
		Category:    goal.Category,
		Domain:      domain,
		Action:      action,
		Reason:      reason,
		Priority:    priority,
		TargetID:    targetID,
		ItemID:      itemID,
		Count:       count,
	}
	return enrichPlannedOp(out)
}

func sortOperations(ops []PlannedOp) {
	sort.SliceStable(ops, func(i, j int) bool {
		if laneRank(ops[i].Lane) != laneRank(ops[j].Lane) {
			return laneRank(ops[i].Lane) < laneRank(ops[j].Lane)
		}
		if ops[i].Priority != ops[j].Priority {
			return ops[i].Priority > ops[j].Priority
		}
		if ops[i].Category != ops[j].Category {
			return categoryRank(ops[i].Category) < categoryRank(ops[j].Category)
		}
		if ops[i].Domain != ops[j].Domain {
			return ops[i].Domain < ops[j].Domain
		}
		return ops[i].OperationID < ops[j].OperationID
	})
}

func laneForDomain(domain string) string {
	switch domain {
	case "farm.harvest", "farm.plant", "farm.water":
		return LaneFarm
	default:
		return LaneSide
	}
}

func laneRank(lane string) int {
	switch lane {
	case LaneFarm:
		return 0
	case LaneSide, "":
		return 1
	default:
		return 2
	}
}

func categoryRank(category string) int {
	switch category {
	case CategoryPlant:
		return 0
	case CategoryOrder:
		return 1
	case CategoryBasic:
		return 2
	case CategoryUnion:
		return 3
	case CategoryActivity:
		return 4
	case CategoryAccount:
		return 5
	default:
		return 9
	}
}

func int32Set(ids []int32) map[int32]bool {
	if len(ids) == 0 {
		return nil
	}
	out := make(map[int32]bool, len(ids))
	for _, id := range ids {
		if id > 0 {
			out[id] = true
		}
	}
	return out
}

func DefaultPolicy() *pb.Policy {
	return &pb.Policy{
		AutomationEnabled: false,
		Basic: &pb.BasicPolicy{
			Reputation: &pb.ReputationPolicy{Enabled: true, Threshold: 80},
			Task:       &pb.BasicTaskPolicy{},
			Benefit:    &pb.BenefitPolicy{},
			Sign:       &pb.SignPolicy{},
			Pearl:      &pb.PearlPolicy{},
			Shop: &pb.ShopPolicy{
				CultivateShop: &pb.ShopBuyPolicy{},
				VipShop:       &pb.VipShopPolicy{},
			},
			Zoo: &pb.ZooPolicy{},
		},
		Plant: &pb.PlantPolicy{
			Cultivate: &pb.CultivatePolicy{
				TargetLevel: 20,
			},
			Planting: &pb.PlantingPolicy{
				AutoEnabled:     true,
				DemandPriority:  defaultDemandPriority(),
				MinWaterDrops:   5,
				AutoReplantMode: pb.SelectionMode_SELECTION_MODE_ALL,
			},
			FriendSteal: &pb.FriendStealPolicy{},
			Elves:       &pb.FlowerElvesPolicy{},
			Market: &pb.FlowerMarketPolicy{
				PutMode:    pb.MarketPutMode_MARKET_PUT_MODE_INVENTORY,
				BuyMode:    pb.MarketBuyMode_MARKET_BUY_MODE_ALL,
				PriceIndex: 2,
				MaxSell:    25,
			},
		},
		Order: &pb.OrderPolicy{
			Customer:  &pb.CustomerOrderPolicy{},
			Resident:  &pb.ResidentOrderPolicy{NormalDailyLimit: 1260, DecorateDailyLimit: 120, SatinDailyLimit: 120},
			Palace:    &pb.PalaceOrderPolicy{},
			Team:      &pb.TeamOrderPolicy{},
			FlowerArt: &pb.FlowerArtPolicy{},
		},
		Union: &pb.UnionPolicy{
			Build:  &pb.UnionBuildPolicy{},
			Flower: &pb.UnionFlowerPolicy{},
			Race:   &pb.UnionRacePolicy{TaskTypePriority: defaultUnionRacePriority()},
			Land:   &pb.UnionLandPolicy{},
		},
		Activity:                &pb.ActivityPolicy{},
		DecisionIntervalSeconds: 4,
	}
}

func DefaultPolicyIfNil(p *pb.Policy) *pb.Policy {
	if p == nil {
		return DefaultPolicy()
	}
	return p
}

func defaultDemandPriority() map[string]int32 {
	return map[string]int32{
		GoalCustomerOrder: 90,
		GoalResidentOrder: 80,
		GoalMainTask:      70,
		GoalDailyTask:     60,
		GoalWeeklyTask:    55,
		GoalFlowerArt:     40,
		GoalAutoReplant:   10,
	}
}

func defaultUnionRacePriority() map[int32]int32 {
	return map[int32]int32{
		2004: 0,
		3006: 2,
		3016: 2,
		3017: 3,
		3018: 2,
		3023: 3,
		3024: 3,
		3030: 2,
		3034: 2,
		3035: 3,
		3036: 1,
		3044: 3,
		3052: 3,
	}
}

func demandByID(demands []Demand, id string) (Demand, bool) {
	for _, demand := range demands {
		if demand.ID == id {
			return demand, true
		}
	}
	return Demand{}, false
}

func demandID(goalID, entityID, source, kind string, itemID int32) string {
	return goalID + ":" + entityID + ":" + source + ":" + kind + ":" + strconv.FormatInt(int64(itemID), 10)
}

func operationID(kind string, landIDs []int32, flowerID, targetID, itemID int32) string {
	parts := []string{kind}
	if targetID != 0 {
		parts = append(parts, "target="+strconv.FormatInt(int64(targetID), 10))
	}
	if itemID != 0 {
		parts = append(parts, "item="+strconv.FormatInt(int64(itemID), 10))
	}
	if flowerID != 0 {
		parts = append(parts, "flower="+strconv.FormatInt(int64(flowerID), 10))
	}
	if len(landIDs) > 0 {
		ids := make([]string, 0, len(landIDs))
		for _, id := range landIDs {
			ids = append(ids, strconv.FormatInt(int64(id), 10))
		}
		parts = append(parts, "lands="+strings.Join(ids, ","))
	}
	return strings.Join(parts, "|")
}

func itemLabel(itemID int32) string {
	if name := state.ItemName(itemID); name != "" {
		return name
	}
	return fmt.Sprintf("#%d", itemID)
}
