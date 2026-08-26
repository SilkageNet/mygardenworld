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

func unionOperations(s *state.State, policy *pb.Policy, now time.Time) []PlannedOp {
	if policy == nil {
		return nil
	}
	union := policy.GetUnion()
	if union == nil {
		return nil
	}
	// IFmlTot.mb (25.1) is the authoritative current-user membership record.
	// Guild automation is closed until that record has been observed and proves
	// membership. Stale guild/race fragments are never sufficient on their own.
	build := s.FmlBuild()
	if !build.MembershipObserved || build.MemberFmlID <= 0 {
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
		return []string{SDKAdUnsupportedReason}
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
	unionLandDefaultReplantMin  int32 = 60
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
	landIDs := unionLandPlantableIDs(s, flowerID, now, leveling, policy)
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

func unionLandMinReplantMinutes(policy *pb.UnionLandPolicy) int32 {
	if policy == nil {
		return unionLandDefaultReplantMin
	}
	min := policy.GetMinReplantMinutes()
	if min <= 0 {
		return unionLandDefaultReplantMin
	}
	return min
}

// unionLandReplantCooldownElapsed reports whether an occupied land's current
// crop has been planted long enough to allow post-level-11 replacement.
func unionLandReplantCooldownElapsed(land state.FmlLandView, now time.Time, minMinutes int32) bool {
	if land.StartTimeMs <= 0 {
		return false
	}
	elapsed := now.Sub(time.UnixMilli(land.StartTimeMs))
	return elapsed >= time.Duration(minMinutes)*time.Minute
}

// unionLandPlantableIDs returns empty slots and replace targets.
// While any filtered flower is below level 11, occupied lands with a different
// flower are force-replaced unless harvest is pending or the next mature is
// within 2 minutes (wait for harvest, then switch). After every flower reaches
// 11, empty slots are always filled; occupied lands with a different flower
// are replaced only after min_replant_minutes (default 60), so multiple flower
// types can coexist across guild lands.
func unionLandPlantableIDs(s *state.State, flowerID int32, now time.Time, leveling bool, policy *pb.UnionLandPolicy) []int32 {
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
		if leveling {
			if unionLandNearMature(land, now) {
				// Current crop matures within 2 minutes: harvest first, then switch.
				continue
			}
			out = append(out, id)
			continue
		}
		if unionLandReplantCooldownElapsed(land, now, unionLandMinReplantMinutes(policy)) {
			out = append(out, id)
		}
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
