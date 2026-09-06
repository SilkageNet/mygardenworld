package automation

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"google.golang.org/protobuf/proto"
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
	sort.SliceStable(ops, func(i, j int) bool { return operationComesBefore(ops[i], ops[j]) })
}

func operationComesBefore(left, right PlannedOp) bool {
	if operationLaneRank(left) != operationLaneRank(right) {
		return operationLaneRank(left) < operationLaneRank(right)
	}
	if left.Priority != right.Priority {
		return left.Priority > right.Priority
	}
	if left.Category != right.Category {
		return categoryRank(left.Category) < categoryRank(right.Category)
	}
	if left.Domain != right.Domain {
		return left.Domain < right.Domain
	}
	// Auto-replant batches encode flower IDs in OperationID. Preserve their
	// stock-ranked insertion order so high IDs are not starved. Other operation
	// families still need a stable, input-order-independent tie-break.
	if left.GoalID == GoalAutoReplant && right.GoalID == GoalAutoReplant &&
		isPlantOperation(left.Kind) && isPlantOperation(right.Kind) {
		return false
	}
	return left.OperationID < right.OperationID
}

// A pure cyclic-note auto-replant remains a farm-lane operation for runner
// locking/cooldown semantics, but ranks with side operations so activity
// priority 50 cannot jump ahead of main tasks or major orders merely because
// usrLand RPCs normally own the farm lane.
func operationLaneRank(op PlannedOp) int {
	// Login/contest bootstrap and contested take/finish stay on the side lane
	// for locking, but sort ahead of farm/order work.
	if IsUrgentRaceOp(op) {
		return -1
	}
	if op.Lane == LaneFarm && op.GoalID == GoalAutoReplant && strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") {
		return laneRank(LaneSide)
	}
	return laneRank(op.Lane)
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
	case CategoryRace:
		return 0
	case CategoryPlant:
		return 1
	case CategoryOrder, CategoryFlowerArt, CategoryWater:
		return 2
	case CategoryBasic:
		return 3
	case CategoryUnion:
		return 4
	case CategoryActivity:
		return 5
	case CategoryAccount:
		return 6
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
		SchemaVersion:     1,
		AutomationEnabled: false,
		Basic: &pb.BasicPolicy{
			Reputation:                     &pb.ReputationPolicy{Enabled: true, Threshold: 80},
			ReconnectIntervalSeconds:       300,
			DisplacedSessionReloginEnabled: false,
			RedeemConnectMode:              pb.RedeemConnectMode_REDEEM_CONNECT_MODE_AUTO,
			Task:                           &pb.BasicTaskPolicy{},
			Benefit:                        &pb.BenefitPolicy{},
			Sign:                           &pb.SignPolicy{},
			Pearl:                          &pb.PearlPolicy{},
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
				AutoEnabled:           true,
				AutoHarvestEnabled:    true,
				DemandPriority:        defaultDemandPriority(),
				DemandPriorityEnabled: false,
				MinWaterDrops:         5,
				AutoReplantMode:       pb.SelectionMode_SELECTION_MODE_ALL,
			},
			FriendSteal: &pb.FriendStealPolicy{
				Mode:         pb.SelectionMode_SELECTION_MODE_ALL,
				FriendMode:   pb.SelectionMode_SELECTION_MODE_ALL,
				FriendCounts: map[int64]int32{},
			},
			Elves: &pb.FlowerElvesPolicy{},
			Market: &pb.FlowerMarketPolicy{
				PutMode:    pb.MarketPutMode_MARKET_PUT_MODE_INVENTORY,
				BuyMode:    pb.MarketBuyMode_MARKET_BUY_MODE_ALL,
				PriceIndex: 2,
				MaxSell:    25,
			},
		},
		Order: &pb.OrderPolicy{
			Customer:  &pb.CustomerOrderPolicy{},
			Resident:  &pb.ResidentOrderPolicy{NormalDailyLimit: 1200, DecorateDailyLimit: 120, SatinDailyLimit: 120},
			Palace:    &pb.PalaceOrderPolicy{},
			Team:      &pb.TeamOrderPolicy{},
			FlowerArt: &pb.FlowerArtPolicy{},
		},
		Union: &pb.UnionPolicy{
			Build:  &pb.UnionBuildPolicy{},
			Flower: &pb.UnionFlowerPolicy{},
			Race: &pb.UnionRacePolicy{
				// Enabled keeps enter/getTaskList (and TTL refresh) so the
				// competition task pool is visible by default. Auto-complete
				// of take/finish/upgrade stays off until the operator turns on
				// AutoEnableModules. Low-score deletion has its own switch and
				// requires an observed guild position with delete permission.
				Enabled:                  true,
				AutoEnableModules:        false,
				AutoGiveUpTask:           false,
				AutoStopOnQuotaDone:      true,
				ExcludeOthersUpgradeTask: true,
				AvoidProgressedTasks:     proto.Bool(true),
				MinTaskScore:             28,
				DeleteIntervalSeconds:    120,
				TaskTypePriority:         defaultUnionRacePriority(),
			},
			Land: &pb.UnionLandPolicy{},
		},
		Activity: &pb.ActivityPolicy{
			CyclicNote:  &pb.CyclicNotePolicy{},
			CyclicStory: &pb.CyclicStoryPolicy{},
		},
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
		3006: 0,
		3016: 0,
		3017: 0,
		3018: 0,
		3023: 0,
		3024: 0,
		3030: 0,
		3034: 0,
		3035: 0,
		3036: 5,
		3044: 0,
		3052: 0,
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
