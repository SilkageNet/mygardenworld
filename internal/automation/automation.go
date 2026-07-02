// Package automation is the pure decision engine. It turns observed state and
// the domain policy into an ordered, categorized plan. Runners execute only
// operations they have a concrete RPC implementation for; the full plan is
// exposed to the UI for transparency.
package automation

import (
	"fmt"
	"sort"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	CategoryAccount  = "account"
	CategoryBasic    = "basic"
	CategoryPlant    = "plant"
	CategoryOrder    = "order"
	CategoryUnion    = "union"
	CategoryActivity = "activity"
	CategorySystem   = "system"
)

const (
	KindHarvest = "harvest"
	KindPlant   = "plant"
	KindWater   = "water"
	KindWait    = "wait"
	KindUnknown = "unknown"
)

const (
	PlantModeLowStock  = "low_stock"
	PlantModeHighValue = "high_value"
	PlantModeSelected  = "selected"
)

// PlannedOp is one categorized operation candidate.
type PlannedOp struct {
	Kind           string
	FeatureID      string
	Category       string
	Label          string
	Domain         string
	Action         string
	Status         string
	Executable     bool
	SyncOnly       bool
	Reason         string
	BlockedReasons []string
	Priority       int32
	LandIDs        []int32
	FlowerID       int32
	GoldCost       int32
	DiamondCost    int32
	ItemCost       map[int32]int32
}

func Recommend(land state.LandView, now time.Time) (kind, reason string) {
	if !land.Observed {
		return KindUnknown, "no observed primary state"
	}
	if !land.IsPlanted() {
		return KindPlant, "land is empty"
	}
	if land.State == 3 {
		return KindHarvest, "state=3 (initial bloom ready)"
	}
	if land.State == 2 {
		if land.NextTimeMs > 0 && land.NextTimeMs <= now.UnixMilli() {
			return KindHarvest, fmt.Sprintf("state=2, nextTime(%d) elapsed", land.NextTimeMs)
		}
		return KindWait, fmt.Sprintf("state=2 regrowing; nextTime=%d", land.NextTimeMs)
	}
	if land.State == 1 {
		return KindWater, "state=1, awaiting first water"
	}
	return KindWait, fmt.Sprintf("state=%d not actionable", land.State)
}

// Plan returns the highest-priority directly executable farm RPC operation.
func Plan(s *state.State, policy *pb.Policy, now time.Time) *PlannedOp {
	for _, op := range PlanOperations(s, policy, now) {
		if strings.HasPrefix(op.Kind, "usrLand.") {
			cp := op
			return &cp
		}
	}
	return nil
}

// PlanOperations returns the categorized operation list in execution order.
func PlanOperations(s *state.State, policy *pb.Policy, now time.Time) []PlannedOp {
	if policy == nil || !policy.GetAutomationEnabled() {
		return nil
	}
	var ops []PlannedOp
	ops = append(ops, farmOps(s, policy.GetPlant(), now)...)
	ops = append(ops, orderOps(policy.GetOrder())...)
	ops = append(ops, plantMaintenanceOps(policy.GetPlant())...)
	ops = append(ops, basicOps(policy.GetBasic())...)
	ops = append(ops, unionOps(policy.GetUnion())...)
	ops = append(ops, activityOps(policy.GetActivity())...)
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Priority != ops[j].Priority {
			return ops[i].Priority > ops[j].Priority
		}
		if ops[i].Category != ops[j].Category {
			return categoryRank(ops[i].Category) < categoryRank(ops[j].Category)
		}
		return ops[i].Domain < ops[j].Domain
	})
	return ops
}

func farmOps(s *state.State, policy *pb.PlantPolicy, now time.Time) []PlannedOp {
	if policy == nil {
		return nil
	}
	lands := s.Lands()
	type bucket struct {
		harvest []int32
		water   []int32
		plant   []int32
	}
	var b bucket
	ids := make([]int32, 0, len(lands))
	for id := range lands {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		kind, _ := Recommend(lands[id], now)
		switch kind {
		case KindHarvest:
			b.harvest = append(b.harvest, id)
		case KindWater:
			b.water = append(b.water, id)
		case KindPlant:
			b.plant = append(b.plant, id)
		}
	}

	var ops []PlannedOp
	if len(b.harvest) > 0 && policy.GetHarvestEnabled() {
		if len(b.harvest) > 1 && policy.GetHarvestPreferOneKey() {
			ops = append(ops, landOp("usrLand.harvestOneKey", "farm.harvest", "harvest", "ready lands", 1000, b.harvest, 0))
		} else {
			ops = append(ops, landOp("usrLand.harvest", "farm.harvest", "harvest", "ready land", 1000, []int32{b.harvest[0]}, 0))
		}
	}

	if len(b.plant) > 0 && policy.GetPlantEnabled() {
		flowerID, _, plantLimit := selectPlantFlower(s, policy)
		if flowerID != 0 {
			maxBatch := policy.GetPlantMaxBatch()
			if maxBatch <= 0 {
				maxBatch = 8
			}
			want := int32(len(b.plant))
			if want > maxBatch {
				want = maxBatch
			}
			if plantLimit > 0 && want > plantLimit {
				want = plantLimit
			}
			if want > 0 {
				picks := b.plant[:want]
				if len(picks) > 1 {
					ops = append(ops, landOp("usrLand.plantBatch", "farm.plant", "plant", "empty lands", 900, picks, flowerID))
				} else {
					ops = append(ops, landOp("usrLand.plant", "farm.plant", "plant", "empty land", 900, picks, flowerID))
				}
			}
		}
	}

	if len(b.water) > 0 && policy.GetWaterEnabled() {
		waterDrops, _, _ := s.AvailableWaterDrops(now)
		minDrops := policy.GetMinWaterDrops()
		if minDrops < 0 {
			minDrops = 0
		}
		usableDrops := waterDrops - minDrops
		if usableDrops > 0 {
			maxBatch := policy.GetWaterMaxBatch()
			if maxBatch <= 0 {
				maxBatch = 8
			}
			want := int32(len(b.water))
			if want > maxBatch {
				want = maxBatch
			}
			if want > usableDrops {
				want = usableDrops
			}
			if want > 0 {
				picks := b.water[:want]
				switch {
				case policy.GetWaterPreferOneKeyIfNoble() && s.NobleEligible() && int32(len(b.water)) == want:
					ops = append(ops, landOp("usrLand.waterOneKey", "farm.water", "water", "noble one-key water", 800, picks, 0))
				case len(picks) > 1:
					ops = append(ops, landOp("usrLand.waterBatch", "farm.water", "water", "lands need water", 800, picks, 0))
				default:
					ops = append(ops, landOp("usrLand.water", "farm.water", "water", "land needs water", 800, picks, 0))
				}
			}
		}
	}
	return ops
}

func plantMaintenanceOps(policy *pb.PlantPolicy) []PlannedOp {
	if policy == nil {
		return nil
	}
	var ops []PlannedOp
	if policy.GetLandUnlockEnabled() {
		ops = append(ops, markerOp(CategoryPlant, "farm.land", "unlock", "land unlock enabled", 760))
	}
	if policy.GetSpeedUpEnabled() {
		ops = append(ops, markerOp(CategoryPlant, "farm.speed_up", "speed_up", "speed-up enabled", 740))
	}
	if policy.GetCultivateEnabled() {
		ops = append(ops, markerOp(CategoryPlant, "farm.cultivate", "cultivate", "cultivation enabled", 720))
	}
	if policy.GetFlowerUpgradeEnabled() {
		ops = append(ops, markerOp(CategoryPlant, "farm.upgrade", "upgrade", "flower upgrade enabled", 710))
	}
	return ops
}

func basicOps(policy *pb.BasicPolicy) []PlannedOp {
	if policy == nil {
		return nil
	}
	var ops []PlannedOp
	add := func(enabled bool, domain, action, reason string, priority int32) {
		if enabled {
			ops = append(ops, markerOp(CategoryBasic, domain, action, reason, priority))
		}
	}
	add(policy.GetWaterwheelEnabled(), "basic.waterwheel", "claim", "waterwheel enabled", 650)
	add(policy.GetFreeWaterEnabled(), "basic.free_water", "claim", "free water enabled", 645)
	add(policy.GetBenefitBoxEnabled(), "basic.benefit", "claim", "benefit box enabled", 640)
	add(policy.GetMailEnabled(), "basic.mail", "claim", "mail enabled", 635)
	add(policy.GetWelfareEnabled(), "basic.welfare", "claim", "welfare enabled", 632)
	add(policy.GetMainTaskEnabled(), "basic.task.main", "claim", "main task rewards enabled", 630)
	add(policy.GetDailyTaskEnabled(), "basic.task.daily", "claim", "daily task rewards enabled", 625)
	add(policy.GetWeeklyTaskEnabled(), "basic.task.weekly", "claim", "weekly task rewards enabled", 620)
	add(policy.GetAchievementTaskEnabled(), "basic.task.achievement", "claim", "achievement rewards enabled", 615)
	add(policy.GetStoryEnabled(), "basic.story", "unlock", "story unlock enabled", 610)
	add(policy.GetSignEnabled(), "basic.sign", "claim", "sign enabled", 600)
	add(policy.GetRoadGrowRewardEnabled(), "basic.road_grow", "claim", "road grow rewards enabled", 598)
	add(policy.GetRandomEventEnabled(), "basic.random_event", "claim", "random events enabled", 596)
	if policy.GetPearl().GetEnabled() {
		ops = append(ops, markerOp(CategoryBasic, "basic.pearl", "run", "pearl enabled", 590))
	}
	if policy.GetShop().GetVideoGiftEnabled() || policy.GetShop().GetCultivateShopEnabled() || policy.GetShop().GetVipShopEnabled() {
		ops = append(ops, markerOp(CategoryBasic, "basic.shop", "buy", "shop automation enabled", 580))
	}
	if policy.GetZoo().GetEnabled() || policy.GetZoo().GetSyncEnabled() {
		ops = append(ops, markerOp(CategoryBasic, "basic.zoo", "run", "zoo automation enabled", 570))
	}
	return ops
}

func orderOps(policy *pb.OrderPolicy) []PlannedOp {
	if policy == nil {
		return nil
	}
	var ops []PlannedOp
	if policy.GetCustomer().GetEnabled() {
		ops = append(ops, markerOp(CategoryOrder, "order.customer", "finish", "customer orders enabled", 780))
	}
	if policy.GetResident().GetNormalEnabled() || policy.GetResident().GetDecorateEnabled() || policy.GetResident().GetSatinEnabled() {
		ops = append(ops, markerOp(CategoryOrder, "order.resident", "finish", "resident orders enabled", 775))
	}
	if policy.GetFlowerArt().GetSellEnabled() || policy.GetFlowerArt().GetCraftEnabled() {
		ops = append(ops, markerOp(CategoryOrder, "order.flower_art", "sell", "flower art enabled", 770))
	}
	if policy.GetPalace().GetEnabled() {
		ops = append(ops, markerOp(CategoryOrder, "order.palace", "finish", "palace orders enabled", 760))
	}
	if policy.GetTeam().GetEnabled() {
		ops = append(ops, markerOp(CategoryOrder, "order.team", "submit", "team orders enabled", 755))
	}
	return ops
}

func unionOps(policy *pb.UnionPolicy) []PlannedOp {
	if policy == nil {
		return nil
	}
	var ops []PlannedOp
	if policy.GetBuildFreeEnabled() || policy.GetBuildGoldEnabled() || policy.GetBuildDiamondEnabled() {
		ops = append(ops, markerOp(CategoryUnion, "union.build", "build", "union build enabled", 500))
	}
	if policy.GetFlowerShareEnabled() || policy.GetFlowerTakeEnabled() {
		ops = append(ops, markerOp(CategoryUnion, "union.flower", "share_take", "union flower enabled", 490))
	}
	if policy.GetRaceEnabled() {
		ops = append(ops, markerOp(CategoryUnion, "union.race", "race", "union race enabled", 480))
	}
	if policy.GetLandAutoPlant() || policy.GetLandHarvest() {
		ops = append(ops, markerOp(CategoryUnion, "union.land", "run", "union land enabled", 470))
	}
	if policy.GetRedPacketEnabled() {
		ops = append(ops, markerOp(CategoryUnion, "union.red_packet", "claim", "union red packet enabled", 460))
	}
	if policy.GetForestEnabled() {
		ops = append(ops, markerOp(CategoryUnion, "union.forest", "run", "union forest enabled", 450))
	}
	return ops
}

func activityOps(policy *pb.ActivityPolicy) []PlannedOp {
	if policy == nil || !policy.GetEnabled() {
		return nil
	}
	keys := make([]string, 0, len(policy.Modules))
	for name, module := range policy.Modules {
		if module != nil && module.GetEnabled() {
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	ops := make([]PlannedOp, 0, len(keys))
	for _, name := range keys {
		ops = append(ops, markerOp(CategoryActivity, "activity."+name, "run", "activity module enabled", 300))
	}
	return ops
}

func landOp(kind, domain, action, reason string, priority int32, landIDs []int32, flowerID int32) PlannedOp {
	op := PlannedOp{
		Kind:     kind,
		Category: CategoryPlant,
		Domain:   domain,
		Action:   action,
		Reason:   reason,
		Priority: priority,
		LandIDs:  append([]int32(nil), landIDs...),
		FlowerID: flowerID,
	}
	return enrichPlannedOp(op)
}

func markerOp(category, domain, action, reason string, priority int32) PlannedOp {
	op := PlannedOp{
		Kind:     domain + "." + action,
		Category: category,
		Domain:   domain,
		Action:   action,
		Reason:   reason,
		Priority: priority,
	}
	return enrichPlannedOp(op)
}

func selectPlantFlower(s *state.State, policy *pb.PlantPolicy) (int32, int32, int32) {
	if policy == nil {
		return 0, 0, 0
	}
	mode := normalizePlantMode(policy.GetPlantingMode())
	if mode == PlantModeSelected && len(policy.GetAllowedFlowerIds()) == 0 {
		return 0, 0, 0
	}
	candidates := s.PlantableFlowers(policy.GetAllowedFlowerIds(), policy.GetBlockedFlowerIds())
	if len(candidates) == 0 {
		return 0, 0, 0
	}
	if policy.GetTaskPriorityEnabled() {
		deficits := s.FlowerOrderDeficits()
		if candidate, deficit, ok := bestDeficitCandidate(candidates, deficits); ok {
			return candidate.FlowerID, candidate.Stock, deficit
		}
	}
	if mode == PlantModeHighValue || mode == PlantModeSelected {
		candidate := bestValueCandidate(candidates)
		return candidate.FlowerID, candidate.Stock, 0
	}
	candidate := lowestStockCandidate(candidates)
	return candidate.FlowerID, candidate.Stock, 0
}

func bestDeficitCandidate(candidates []state.PlantableFlower, deficits map[int32]int32) (state.PlantableFlower, int32, bool) {
	var best state.PlantableFlower
	var bestDeficit int32
	for _, candidate := range candidates {
		deficit := deficits[candidate.FlowerID]
		if deficit <= 0 {
			continue
		}
		if best.FlowerID == 0 ||
			deficit > bestDeficit ||
			(deficit == bestDeficit && candidate.Gold > best.Gold) ||
			(deficit == bestDeficit && candidate.Gold == best.Gold && candidate.FlowerID < best.FlowerID) {
			best = candidate
			bestDeficit = deficit
		}
	}
	return best, bestDeficit, best.FlowerID != 0
}

func bestValueCandidate(candidates []state.PlantableFlower) state.PlantableFlower {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Gold > best.Gold ||
			(candidate.Gold == best.Gold && candidate.Experience > best.Experience) ||
			(candidate.Gold == best.Gold && candidate.Experience == best.Experience && candidate.FlowerID < best.FlowerID) {
			best = candidate
		}
	}
	return best
}

func lowestStockCandidate(candidates []state.PlantableFlower) state.PlantableFlower {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Stock < best.Stock ||
			(candidate.Stock == best.Stock && candidate.Gold > best.Gold) ||
			(candidate.Stock == best.Stock && candidate.Gold == best.Gold && candidate.FlowerID < best.FlowerID) {
			best = candidate
		}
	}
	return best
}

func normalizePlantMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case PlantModeLowStock:
		return PlantModeLowStock
	case PlantModeHighValue:
		return PlantModeHighValue
	case PlantModeSelected:
		return PlantModeSelected
	case "":
		return PlantModeLowStock
	default:
		return PlantModeLowStock
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

func DefaultPolicy() *pb.Policy {
	return &pb.Policy{
		AutomationEnabled: false,
		Basic: &pb.BasicPolicy{
			ReputationEnabled:   true,
			ReputationThreshold: 80,
			Pearl:               &pb.PearlPolicy{},
			Shop:                &pb.ShopPolicy{},
			Zoo:                 &pb.ZooPolicy{},
		},
		Plant: &pb.PlantPolicy{
			HarvestEnabled:           true,
			HarvestPreferOneKey:      true,
			PlantEnabled:             true,
			PlantingMode:             PlantModeLowStock,
			TaskPriorityEnabled:      true,
			TaskPriority:             defaultTaskPriority(),
			PlantMaxBatch:            8,
			WaterEnabled:             true,
			WaterMaxBatch:            8,
			MinWaterDrops:            5,
			WaterPreferOneKeyIfNoble: false,
		},
		Order: &pb.OrderPolicy{
			Customer:  &pb.CustomerOrderPolicy{},
			Resident:  &pb.ResidentOrderPolicy{},
			Palace:    &pb.PalaceOrderPolicy{},
			Team:      &pb.TeamOrderPolicy{},
			FlowerArt: &pb.FlowerArtPolicy{},
		},
		Union: &pb.UnionPolicy{
			FlowerShareMode:      "quality",
			FlowerTakeMode:       "quality",
			LandPlantMode:        PlantModeLowStock,
			RaceTaskTypePriority: defaultUnionTaskPriority(),
		},
		Activity: &pb.ActivityPolicy{
			Modules: defaultActivityModules(),
		},
		Safety: &pb.SafetyPolicy{
			RequireObservedState:     true,
			StopOnSessionInvalidated: true,
			MaxConsecutiveErrors:     3,
			DomainBackoffSeconds:     1800,
			BlockOnDailyLimit:        true,
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

func defaultTaskPriority() map[string]int32 {
	return map[string]int32{
		"公会竞赛": 90,
		"宫廷订单": 80,
		"居民订单": 70,
		"顾客订单": 60,
		"花艺售卖": 50,
		"莳花纪闻": 40,
	}
}

func defaultUnionTaskPriority() map[string]int32 {
	return map[string]int32{
		"2004": 50, "3006": 50, "3016": 50, "3017": 50, "3018": 50,
		"3023": 50, "3024": 50, "3030": 50, "3034": 50, "3035": 50,
		"3036": 50, "3044": 50, "3052": 50,
	}
}

func defaultActivityModules() map[string]*pb.ActivityModulePolicy {
	names := []string{
		"actCyclicStory", "actDessert", "actDuanWu", "actElim", "actMerge2",
		"actSpool", "cyclicNote", "fishFun", "fishMerge", "lanternFestival",
		"magicBubble", "moneyTree", "recvLuck", "redPacket", "yzCall", "zooGameElim",
	}
	out := make(map[string]*pb.ActivityModulePolicy, len(names))
	for _, name := range names {
		out[name] = &pb.ActivityModulePolicy{Speed: 1}
	}
	return out
}
