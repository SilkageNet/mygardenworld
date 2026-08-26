package automation

import (
	"fmt"
	"sort"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func Recommend(land state.LandView, now time.Time, harvestDelay time.Duration) (kind, reason string) {
	if harvestDelay < 0 {
		harvestDelay = 0
	}
	if !land.Observed {
		return KindUnknown, "no observed primary state"
	}
	if !land.IsPlanted() {
		return KindPlant, "land is empty"
	}
	if land.State == 3 {
		if readyAt, ok := harvestReadyAt(land, harvestDelay); ok && now.Before(readyAt) {
			return KindWait, fmt.Sprintf("state=3 harvest delay; readyAt=%d", readyAt.UnixMilli())
		}
		return KindHarvest, "state=3 (initial bloom ready)"
	}
	if land.State == 2 {
		if land.NextTimeMs > 0 {
			readyAt, _ := harvestReadyAt(land, harvestDelay)
			if !now.Before(readyAt) {
				return KindHarvest, fmt.Sprintf("state=2, nextTime(%d)+delay elapsed", land.NextTimeMs)
			}
			return KindWait, fmt.Sprintf("state=2 regrowing; nextTime=%d readyAt=%d", land.NextTimeMs, readyAt.UnixMilli())
		}
		return KindWait, fmt.Sprintf("state=2 regrowing; nextTime=%d", land.NextTimeMs)
	}
	if land.State == 1 {
		return KindWater, "state=1, awaiting first water"
	}
	return KindWait, fmt.Sprintf("state=%d not actionable", land.State)
}

// harvestReadyAt is when auto-harvest may run. State 3 uses plantTime (last
// state change into harvestable) plus the configured delay. State 2 uses
// nextTime plus max(protocol grace, configured delay) so short delays still
// avoid premature harvest races.
func harvestReadyAt(land state.LandView, harvestDelay time.Duration) (time.Time, bool) {
	if harvestDelay < 0 {
		harvestDelay = 0
	}
	switch land.State {
	case 3:
		if harvestDelay <= 0 {
			return time.Time{}, false
		}
		matureMs := land.PlantTimeMs
		if matureMs <= 0 {
			// plantTime (field 7) missing: no reliable "became ready" tick.
			// Fall back to immediate harvest — nextTime (field 5) is a future
			// regrow timestamp on state=3 rows and would stall harvests.
			return time.Time{}, false
		}
		return time.UnixMilli(matureMs).Add(harvestDelay), true
	case 2:
		if land.NextTimeMs <= 0 {
			return time.Time{}, false
		}
		wait := harvestReadyGrace
		if harvestDelay > wait {
			wait = harvestDelay
		}
		return time.UnixMilli(land.NextTimeMs).Add(wait), true
	default:
		return time.Time{}, false
	}
}

func farmOps(s *state.State, policy *pb.PlantPolicy, demands []Demand, now time.Time, suppressAutoReplant bool) []PlannedOp {
	if policy == nil {
		return nil
	}
	plantingPolicy := policy.GetPlanting()
	harvestDelay := time.Duration(plantingPolicy.GetHarvestDelaySeconds()) * time.Second
	// Race plant-harvest must keep driving farm while the task is unfinished:
	// after planting, pending yield clears the plant demand, but lands still
	// need watering (and later harvest). raceProgress is true for that whole
	// window. Race plant slots still claim first; leftover empties may
	// auto-replant when ordinary AutoEnabled is on.
	raceProgress := suppressAutoReplant
	raceDriven := hasRacePlantDemand(demands) || raceProgress
	raceFlowerID := racePlantHarvestFlowerID(s, demands)
	lands := s.Lands()
	var harvest, water, plant []int32
	ids := make([]int32, 0, len(lands))
	for id := range lands {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		land := lands[id]
		delay := harvestDelay
		// Competition crop: ignore configured harvest_delay_seconds and pick
		// as soon as ready (state=2 still keeps the short protocol grace).
		if raceFlowerID > 0 && int32(land.FlowerID) == raceFlowerID {
			delay = 0
		}
		kind, _ := Recommend(land, now, delay)
		switch kind {
		case KindHarvest:
			harvest = append(harvest, id)
		case KindWater:
			water = append(water, id)
		case KindPlant:
			plant = append(plant, id)
		}
	}
	var ops []PlannedOp
	if (plantingPolicy.GetAutoHarvestEnabled() || raceDriven) && len(harvest) > 0 {
		// Without ordinary auto-harvest, only race flowers are forced; other
		// ready lands stay untouched until AutoHarvestEnabled is on.
		if raceDriven && !plantingPolicy.GetAutoHarvestEnabled() && raceFlowerID > 0 {
			harvest = filterLandIDsByFlower(s, harvest, raceFlowerID)
		}
		if len(harvest) > 0 {
			ops = append(ops, landOp(clientproto.RPCUsrLandHarvest.String(), "farm.harvest", "harvest", fmt.Sprintf("%d ready lands", len(harvest)), 10000, harvest, 0, "", ""))
		}
	}
	if !plantingPolicy.GetAutoEnabled() && !raceDriven {
		return ops
	}
	if len(plant) > 0 {
		plantDemands := demands
		// Race-only drive (AutoEnabled off) must not fill leftover empties with
		// 自主补种. Active race still assigns first; leftover empties auto-replant
		// when AutoEnabled is on. Expired plant-harvest holds keep freezing
		// replant until getTaskList clears the stale task.
		suppressFallback := !plantingPolicy.GetAutoEnabled() ||
			(raceProgress && raceTakenExpired(s.FmlRace().Taken, now))
		plan := planPlantAssignments(s, policy, plantDemands, int32(len(plant)), suppressFallback)
		cursor := 0
		for _, assignment := range plan.executable {
			if cursor >= len(plant) {
				break
			}
			count := int(assignment.Count)
			if count > len(plant)-cursor {
				count = len(plant) - cursor
			}
			picks := append([]int32(nil), plant[cursor:cursor+count]...)
			cursor += count
			kind := clientproto.RPCUsrLandPlant.String()
			if len(picks) > 1 {
				kind = clientproto.RPCUsrLandPlantBatch.String()
			}
			ops = append(ops, landOp(kind, "farm.plant", "plant", assignment.Reason, assignment.Priority, picks, assignment.FlowerID, assignment.GoalID, assignment.DemandID))
		}
		for _, diagnostic := range plan.blockedDiagnostic {
			ops = append(ops, blockedPlantDiagnosticOp(diagnostic))
		}
	}
	if len(water) > 0 {
		if flowerID := racePlantHarvestFlowerID(s, demands); flowerID > 0 && raceProgress {
			if !plantingPolicy.GetAutoEnabled() {
				// Race-only farm drive: water only the race flower.
				water = filterLandIDsByFlower(s, water, flowerID)
			} else {
				// Auto planting on: still water race lands first so limited
				// water drops are not spent on unrelated crops.
				water = prioritizeLandIDsByFlower(s, water, flowerID)
			}
		}
	}
	if len(water) > 0 {
		waterDrops, _, _ := s.AvailableWaterDrops(now)
		minDrops := plantingPolicy.GetMinWaterDrops()
		if minDrops < 0 {
			minDrops = 0
		}
		usableDrops := waterDrops - minDrops
		if usableDrops > 0 {
			want := int32(len(water))
			if want > usableDrops {
				want = usableDrops
			}
			if want > 0 {
				picks := water[:want]
				switch {
				case len(picks) > 1:
					ops = append(ops, landOp(clientproto.RPCUsrLandWaterBatch.String(), "farm.water", "water", "lands need water", 8000, picks, 0, "", ""))
				default:
					ops = append(ops, landOp(clientproto.RPCUsrLandWater.String(), "farm.water", "water", "land needs water", 8000, picks, 0, "", ""))
				}
			}
		}
	}
	return ops
}

func blockedPlantDiagnosticOp(assignment plantAssignment) PlannedOp {
	op := landOp(clientproto.RPCUsrLandPlant.String(), "farm.plant", "plant", assignment.Reason, assignment.Priority, nil, assignment.FlowerID, assignment.GoalID, assignment.DemandID)
	op.OperationID += "|demand=" + assignment.DemandID
	op.Status = PlanStatusBlocked
	op.Executable = false
	op.LandIDs = nil
	op.BlockedReasons = []string{assignment.Reason}
	op.BlockingStage = "state"
	return op
}

type plantAssignment struct {
	FlowerID int32
	Count    int32
	Priority int32
	GoalID   string
	DemandID string
	Reason   string
}

type plantAssignmentPlan struct {
	executable        []plantAssignment
	blockedDiagnostic []plantAssignment
}

const blockedPlantDiagnosticPriority int32 = 1000

const blockedPlantDiagnosticReason = "需求花朵尚未培育，无法种植"

func plantAssignments(s *state.State, policy *pb.PlantPolicy, demands []Demand, emptyCount int32) []plantAssignment {
	return planPlantAssignments(s, policy, demands, emptyCount, false).executable
}

func planPlantAssignments(s *state.State, policy *pb.PlantPolicy, demands []Demand, emptyCount int32, suppressAutoReplant bool) plantAssignmentPlan {
	if emptyCount <= 0 {
		return plantAssignmentPlan{}
	}
	plantingPolicy := policy.GetPlanting()
	allowed, blocked := autoReplantFlowerFilters(plantingPolicy)
	candidates := filterPlantableByAutoReplantMinLevel(
		filterPlantableByAutoReplantQualities(s.PlantableFlowers(allowed, blocked), plantingPolicy),
		plantingPolicy,
	)
	plantable := map[int32]state.PlantableFlower{}
	for _, candidate := range s.PlantableFlowers(nil, nil) {
		plantable[candidate.FlowerID] = candidate
	}
	demandPlanting := plantingPolicy.GetDemandPriorityEnabled()
	var out []plantAssignment
	var diagnostics []plantAssignment
	remaining := emptyCount
	// Race plant-harvest first, then other flower demands. sortDemands already
	// ranks by Priority, but an explicit race pass keeps guild competition
	// first even if another demand's Priority is misconfigured higher.
	assignFlowerDemand := func(demand Demand) {
		if demand.Kind != DemandKindFlower || demand.Missing <= 0 || len(demand.BlockedReasons) > 0 {
			return
		}
		if !demandPlanting && !isRaceDrivenFlowerDemand(demand) {
			return
		}
		if _, ok := plantable[demand.ItemID]; !ok {
			diagnostics = append(diagnostics, plantAssignment{
				FlowerID: demand.ItemID,
				Priority: blockedPlantDiagnosticPriority,
				GoalID:   demand.GoalID,
				DemandID: demand.ID,
				Reason:   blockedPlantDiagnosticReason,
			})
			return
		}
		if remaining <= 0 {
			return
		}
		count := demand.Missing
		if count > remaining {
			count = remaining
		}
		if count <= 0 {
			return
		}
		out = append(out, plantAssignment{
			FlowerID: demand.ItemID,
			Count:    count,
			Priority: demand.Priority*100 + 500,
			GoalID:   demand.GoalID,
			DemandID: demand.ID,
			Reason:   demand.Label,
		})
		remaining -= count
	}
	for _, demand := range demands {
		if isRaceDrivenFlowerDemand(demand) {
			assignFlowerDemand(demand)
		}
	}
	for _, demand := range demands {
		if !isRaceDrivenFlowerDemand(demand) {
			assignFlowerDemand(demand)
		}
	}
	executable := executableAssignments(out)
	if remaining <= 0 || suppressAutoReplant {
		return plantAssignmentPlan{executable: executable, blockedDiagnostic: diagnostics}
	}
	mode := plantingPolicy.GetAutoReplantMode()
	if autoReplantUsesLowestStockBatch(mode) {
		applyPlantedLandStock(s, candidates)
		applyAssignmentStock(candidates, executable)
	}
	fallback := autoReplantAssignments(candidates, remaining, mode)
	return plantAssignmentPlan{
		executable:        append(executable, fallback...),
		blockedDiagnostic: diagnostics,
	}
}

func isRaceDrivenFlowerDemand(demand Demand) bool {
	return demand.GoalID == raceActionGoal && demand.Source == "race_task"
}

func racePlantHarvestFlowerID(s *state.State, demands []Demand) int32 {
	for _, demand := range demands {
		if isRaceDrivenFlowerDemand(demand) && demand.ItemID > 0 {
			return demand.ItemID
		}
	}
	if s == nil {
		return 0
	}
	taken := s.FmlRace().Taken
	if taken.HasTask && taken.TaskType == raceTaskTypePlantHarvest && taken.ParamID > 0 {
		return taken.ParamID
	}
	return 0
}

func filterLandIDsByFlower(s *state.State, landIDs []int32, flowerID int32) []int32 {
	if s == nil || flowerID <= 0 || len(landIDs) == 0 {
		return landIDs
	}
	lands := s.Lands()
	out := make([]int32, 0, len(landIDs))
	for _, id := range landIDs {
		if int32(lands[id].FlowerID) == flowerID {
			out = append(out, id)
		}
	}
	return out
}

// prioritizeLandIDsByFlower keeps all landIDs but moves matching flower lands
// ahead so scarce resources (water drops) serve guild-race crops first.
func prioritizeLandIDsByFlower(s *state.State, landIDs []int32, flowerID int32) []int32 {
	if s == nil || flowerID <= 0 || len(landIDs) <= 1 {
		return landIDs
	}
	lands := s.Lands()
	matched := make([]int32, 0, len(landIDs))
	rest := make([]int32, 0, len(landIDs))
	for _, id := range landIDs {
		if int32(lands[id].FlowerID) == flowerID {
			matched = append(matched, id)
		} else {
			rest = append(rest, id)
		}
	}
	if len(matched) == 0 || len(rest) == 0 {
		return landIDs
	}
	return append(matched, rest...)
}

func applyAssignmentStock(candidates []state.PlantableFlower, assignments []plantAssignment) {
	if len(candidates) == 0 || len(assignments) == 0 {
		return
	}
	added := map[int32]int32{}
	for _, assignment := range assignments {
		if assignment.FlowerID <= 0 || assignment.Count <= 0 {
			continue
		}
		added[assignment.FlowerID] += assignment.Count
	}
	for i := range candidates {
		candidates[i].Stock += added[candidates[i].FlowerID]
	}
}

func executableAssignments(in []plantAssignment) []plantAssignment {
	out := in[:0]
	for _, assignment := range in {
		if assignment.Count > 0 {
			out = append(out, assignment)
		}
	}
	return out
}

// autoReplantBatchSize is how many empty lands one ALL/EXCLUDE auto-replant
// step claims before re-ranking by effective stock.
const autoReplantBatchSize int32 = 4

func autoReplantUsesLowestStockBatch(mode pb.SelectionMode) bool {
	switch mode {
	case pb.SelectionMode_SELECTION_MODE_SPECIFIC:
		return false
	default:
		// ALL, EXCLUDE, and unset modes share the lowest-stock batch path.
		return true
	}
}

func applyPlantedLandStock(s *state.State, candidates []state.PlantableFlower) {
	if s == nil || len(candidates) == 0 {
		return
	}
	planted := map[int32]int32{}
	for _, land := range s.Lands() {
		if !land.IsPlanted() {
			continue
		}
		planted[int32(land.FlowerID)]++
	}
	for i := range candidates {
		candidates[i].Stock += planted[candidates[i].FlowerID]
	}
}

func autoReplantAssignments(candidates []state.PlantableFlower, limit int32, mode pb.SelectionMode) []plantAssignment {
	if len(candidates) == 0 || limit <= 0 {
		return nil
	}
	if autoReplantUsesLowestStockBatch(mode) {
		return autoReplantLowestStockBatchAssignments(candidates, limit)
	}
	return autoReplantBalanceAssignments(candidates, limit)
}

func sortPlantableByStock(candidates []state.PlantableFlower) {
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Stock != candidates[j].Stock {
			return candidates[i].Stock < candidates[j].Stock
		}
		return candidates[i].FlowerID < candidates[j].FlowerID
	})
}

func autoReplantAssignment(flowerID, count int32) plantAssignment {
	return plantAssignment{
		FlowerID: flowerID,
		Count:    count,
		Priority: priorityFor(defaultDemandPriority(), GoalAutoReplant)*100 + 100,
		GoalID:   GoalAutoReplant,
		Reason:   "自主补种",
	}
}

// autoReplantLowestStockBatchAssignments plants up to autoReplantBatchSize lands
// of the current lowest-stock flower, then re-ranks and repeats. Used for ALL
// and EXCLUDE auto-replant scopes.
func autoReplantLowestStockBatchAssignments(candidates []state.PlantableFlower, limit int32) []plantAssignment {
	var out []plantAssignment
	remaining := limit
	for remaining > 0 {
		sortPlantableByStock(candidates)
		count := autoReplantBatchSize
		if count > remaining {
			count = remaining
		}
		out = append(out, autoReplantAssignment(candidates[0].FlowerID, count))
		candidates[0].Stock += count
		remaining -= count
	}
	return out
}

// autoReplantBalanceAssignments keeps SPECIFIC-mode water-level balancing among
// the selected flowers: raise every current minimum-stock flower toward the
// next stock tier before moving on.
func autoReplantBalanceAssignments(candidates []state.PlantableFlower, limit int32) []plantAssignment {
	sortPlantableByStock(candidates)
	var out []plantAssignment
	remaining := limit
	for remaining > 0 {
		minStock := candidates[0].Stock
		nextStock := minStock + 1
		for _, candidate := range candidates {
			if candidate.Stock > minStock {
				nextStock = candidate.Stock
				break
			}
		}
		advanced := false
		for i := range candidates {
			if remaining <= 0 {
				break
			}
			if candidates[i].Stock > minStock {
				break
			}
			count := nextStock - candidates[i].Stock
			if count <= 0 {
				count = 1
			}
			if count > remaining {
				count = remaining
			}
			out = append(out, autoReplantAssignment(candidates[i].FlowerID, count))
			candidates[i].Stock += count
			remaining -= count
			advanced = true
		}
		if !advanced {
			break
		}
		sortPlantableByStock(candidates)
	}
	return out
}

func autoReplantFlowerFilters(policy *pb.PlantingPolicy) (allowed []int32, blocked []int32) {
	if policy == nil {
		return nil, nil
	}
	switch policy.GetAutoReplantMode() {
	case pb.SelectionMode_SELECTION_MODE_SPECIFIC:
		return uniquePositiveInt32s(policy.GetAutoReplantFlowerIds()), nil
	case pb.SelectionMode_SELECTION_MODE_EXCLUDE:
		return nil, uniquePositiveInt32s(policy.GetAutoReplantExcludeFlowerIds())
	default:
		return nil, nil
	}
}

// filterPlantableByAutoReplantQualities applies quality limits for ALL-mode
// autonomous replanting. Empty qualities means every quality is allowed.
func filterPlantableByAutoReplantQualities(candidates []state.PlantableFlower, policy *pb.PlantingPolicy) []state.PlantableFlower {
	if policy == nil {
		return candidates
	}
	switch policy.GetAutoReplantMode() {
	case pb.SelectionMode_SELECTION_MODE_SPECIFIC, pb.SelectionMode_SELECTION_MODE_EXCLUDE:
		return candidates
	}
	allowed := int32Set(policy.GetAutoReplantQualities())
	if len(allowed) == 0 {
		return candidates
	}
	out := make([]state.PlantableFlower, 0, len(candidates))
	for _, candidate := range candidates {
		if allowed[flowerQuality(candidate.FlowerID)] {
			out = append(out, candidate)
		}
	}
	return out
}

// filterPlantableByAutoReplantMinLevel keeps flowers whose cultivate level is
// at least the configured minimum. 0 (or negative) means no level filter.
func filterPlantableByAutoReplantMinLevel(candidates []state.PlantableFlower, policy *pb.PlantingPolicy) []state.PlantableFlower {
	if policy == nil {
		return candidates
	}
	minLevel := policy.GetAutoReplantMinLevel()
	if minLevel <= 0 {
		return candidates
	}
	out := make([]state.PlantableFlower, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Lvl >= minLevel {
			out = append(out, candidate)
		}
	}
	return out
}

func uniquePositiveInt32s(values []int32) []int32 {
	seen := map[int32]bool{}
	var out []int32
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
