package automation

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	raceTaskTypePlantHarvest    int32 = 3036
	raceTaskTypeCustomerOrder   int32 = 3016
	raceTaskTypePearlHire       int32 = 3023
	raceTaskTypeFlowerArtSell   int32 = 3030
	raceTaskTypeFlowerArtCraft  int32 = 3034
	raceTaskTypeFlowerCultivate int32 = 3044

	// Catalog flower-cultivate rows upgrade to score 36 (e.g. 9→18→36).
	// Automation only takes/keeps that fully-upgraded score.
	raceFlowerCultivateRequiredScore int32 = 36

	// Above defaultDemandPriority GoalCustomerOrder (90) so race plant-harvest
	// always claims empty lands before ordinary order flower demands.
	raceDemandPriority int32 = 95
	raceActionGoal           = "union.race"

	// Customer-order / pearl-hire / flower-art / flower-cultivate race progress
	// is only authoritative after getTaskList (no live field-134 harvest
	// deltas). Successful module finishes MarkFmlRaceTaskPoolStale for an
	// immediate refresh; this interval is only a slow fallback when finishes
	// happen outside automation. Flower-cultivate is take/finish only — race
	// never emits cultivate RPCs; sync still watches FinishCnt for submit.
	raceModuleProgressSyncInterval = 10 * time.Minute

	// Beat the ordinary customer-order lane (~11000+) so race sync/finish are
	// not starved by endless gen/finish cycling. Cultivate / pearl hire /
	// flower-art ops are lower priority; the same elevated sync/finish ranks
	// keep race submission ahead of unrelated union work without blocking
	// those modules.
	raceCustomerSyncPriority    int32 = 12400
	raceCustomerFinishPriority  int32 = 12500
	racePearlSyncPriority       int32 = 12400
	racePearlFinishPriority     int32 = 12500
	raceFlowerArtSyncPriority   int32 = 12400
	raceFlowerArtFinishPriority int32 = 12500
	// Above ordinary rack sell/craft (~4k–11k) so held race flower-art tasks
	// advance before unrelated side-lane work; still below race finish/sync.
	raceFlowerArtSellOpPriority   int32 = 12300
	raceFlowerArtCraftOpPriority  int32 = 12300
	raceFlowerArtClaimOpPriority  int32 = 12350
	raceFlowerArtCancelOpPriority int32 = 12340
	raceCultivateSyncPriority     int32 = 12400
	raceCultivateFinishPriority   int32 = 12500

	// Held flower-art-sell listings older than this are cancelled and re-listed
	// so race FinishCnt can keep advancing without waiting for full sell windows.
	raceFlowerArtRelistAfter = 5 * time.Minute

	// raceExpireSpeedupLead is how long before a held plant-harvest task's
	// ExpireTime the explicit urgency policy may use speedup tickets.
	raceExpireSpeedupLead = 10 * time.Minute

	// An expired local task is refreshed promptly, but a server response that
	// still carries it must not cause a sync loop every planner tick.
	raceExpiredTaskSyncInterval = 30 * time.Second
)

// raceTaskProgressDemands converts the currently taken unfinished guild-race
// task into planner demands that drive regular farm modules.
//
// Progress itself is server-side (FinishCnt) for finishTask; planting uses
// max(FinishCnt, LocalFinishCnt) plus pending yield on still-planted race
// lands. After each harvest round, if progress+pending cannot cover the
// target, Missing is the empty-land top-up count (ceil of the flower deficit).
// LocalFinishCnt covers field-134 lag so emptied lands do not look like a
// fresh deficit.
func raceTaskProgressDemands(s *state.State, policy *pb.Policy, now time.Time) []Demand {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() {
		return nil
	}
	race := policy.GetUnion().GetRace()
	if race == nil || !race.GetEnabled() || !race.GetAutoEnableModules() {
		return nil
	}
	view := s.FmlRace()
	taken := view.Taken
	if !taken.HasTask || taken.TargetCnt <= 0 || taken.FinishCnt >= taken.TargetCnt {
		return nil
	}
	if raceTakenExpired(taken, now) {
		return nil
	}
	// Do not plant/harvest for a task we are about to give up (low score,
	// missing from pool, uncompletable, priority 0), and do not progress while
	// score is still unresolved under a min_task_score gate. Farm-lane plant
	// ops otherwise outrank side-lane giveUp and can fill the whole farm first.
	gates := raceModuleGatesFromPolicy(policy)
	if raceTakenBlocksProgress(s, race, view, gates) {
		return nil
	}
	switch taken.TaskType {
	case raceTaskTypePlantHarvest:
		if taken.ParamID <= 0 {
			return nil
		}
		// TargetCnt/FinishCnt are flower counts ("收获N朵"). After each harvest
		// round, verify progress + pending yield on planted lands; convert any
		// shortfall into empty-land plant slots via cropGets × frequencys.
		progress := taken.FinishCnt
		if view.LocalFinishTaskMsId == taken.TaskMsId && view.LocalFinishCnt > progress {
			progress = view.LocalFinishCnt
		}
		plantsMissing := racePlantHarvestPlantMissing(s, taken.ParamID, taken.TargetCnt, progress)
		if plantsMissing <= 0 {
			return nil
		}
		label := taken.TargetLabel
		if label == "" {
			label = fmt.Sprintf("公会竞赛种植 #%d", taken.ParamID)
		} else {
			label = "公会竞赛种植 " + label
		}
		entityID := strconv.FormatInt(taken.TaskMsId, 10)
		return []Demand{{
			ID:        demandID(raceActionGoal, entityID, "race_task", DemandKindFlower, taken.ParamID),
			GoalID:    raceActionGoal,
			Category:  CategoryRace,
			Domain:    raceActionGoal,
			EntityID:  entityID,
			Source:    "race_task",
			Label:     label,
			Kind:      DemandKindFlower,
			ItemID:    taken.ParamID,
			Count:     taken.TargetCnt,
			Have:      taken.FinishCnt,
			Available: taken.FinishCnt,
			Missing:   plantsMissing,
			Priority:  raceDemandPriority,
		}}
	case raceTaskTypeCustomerOrder, raceTaskTypePearlHire,
		raceTaskTypeFlowerArtSell, raceTaskTypeFlowerArtCraft:
		// Action demand only — ordinary order.customer / pearl hire /
		// flowerRack.sell / flowerArt.makeFlowerArt ops satisfy it.
		// Flower-cultivate (3044) is take/finish only: no progress demand.
		// Inventory ledger must not allocate against FinishCnt.
		missing := taken.TargetCnt - taken.FinishCnt
		if missing <= 0 {
			return nil
		}
		label := "公会竞赛顾客订单"
		switch taken.TaskType {
		case raceTaskTypePearlHire:
			label = "公会竞赛珍珠雇佣"
		case raceTaskTypeFlowerArtSell:
			label = "公会竞赛花艺售卖"
		case raceTaskTypeFlowerArtCraft:
			label = "公会竞赛花艺制作"
		}
		if taken.TargetLabel != "" {
			label = label + " " + taken.TargetLabel
		}
		entityID := strconv.FormatInt(taken.TaskMsId, 10)
		src := raceActionDemandSource(taken.TaskType)
		itemID := int32(0)
		if taken.TaskType == raceTaskTypeFlowerArtCraft {
			itemID = taken.ParamID
		}
		return []Demand{{
			ID:        demandID(raceActionGoal, entityID, src, DemandKindAction, itemID),
			GoalID:    raceActionGoal,
			Category:  CategoryRace,
			Domain:    raceActionGoal,
			EntityID:  entityID,
			Source:    src,
			Label:     label,
			Kind:      DemandKindAction,
			ItemID:    itemID,
			Count:     taken.TargetCnt,
			Have:      taken.FinishCnt,
			Available: taken.FinishCnt,
			Missing:   missing,
			Priority:  raceDemandPriority,
		}}
	default:
		return nil
	}
}

// raceActionDemandSource tags module-backed race action demands so customer /
// pearl / flower-art drivers cannot cross-link each other's ops.
func raceActionDemandSource(taskType int32) string {
	return "race_task:" + strconv.FormatInt(int64(taskType), 10)
}

// racePlantHarvestPlantMissing converts race flower-count targets into how
// many empty lands still need planting.
//
// progress = max(finishCnt/LocalFinishCnt, flowers already harvested on planted lands)
// pending  = remaining rounds on still-planted race lands × cropGets
// need     = target - progress - pending
// plants   = ceil(need / (cropGets*frequencys)) at the cultivate level
//
// Called every plan tick (including right after a harvest round). When
// progress+pending already covers the target, returns 0 even if empty slots
// exist. When a harvest round leaves the task short, returns the top-up count.
func racePlantHarvestPlantMissing(s *state.State, flowerID, targetCnt, finishCnt int32) int32 {
	if targetCnt <= 0 || flowerID <= 0 || finishCnt >= targetCnt {
		return 0
	}
	perPlant := raceFlowerFlowersPerPlant(s, flowerID)
	progress := finishCnt
	if local := racePlantedHarvestedFlowers(s, flowerID); local > progress {
		progress = local
	}
	need := targetCnt - progress - racePlantedPendingFlowers(s, flowerID)
	if need <= 0 {
		return 0
	}
	return (need + perPlant - 1) / perPlant
}

func raceFlowerCultivateLevel(s *state.State, flowerID int32) int32 {
	if s == nil || flowerID <= 0 {
		return 0
	}
	cv, ok := s.Cultivations()[flowerID]
	if !ok || cv.Lvl <= 0 {
		return 0
	}
	return cv.Lvl
}

func raceFlowerFlowersPerPlant(s *state.State, flowerID int32) int32 {
	if yield, ok := state.FlowerLvlYieldByID(flowerID, raceFlowerCultivateLevel(s, flowerID)); ok {
		if n := yield.FlowersPerPlant(); n > 0 {
			return n
		}
	}
	// Unknown yield: plant 1:1 so the task still progresses.
	return 1
}

// racePlantedPendingFlowers sums flowers still expected from lands already
// planted with flowerID: max(0, frequencys-harvestCnt) * cropGets per land.
func racePlantedPendingFlowers(s *state.State, flowerID int32) int32 {
	return racePlantedYieldSum(s, flowerID, false)
}

// racePlantedHarvestedFlowers sums flowers already taken from lands currently
// planted with flowerID: min(harvestCnt, frequencys) * cropGets per land.
func racePlantedHarvestedFlowers(s *state.State, flowerID int32) int32 {
	return racePlantedYieldSum(s, flowerID, true)
}

func racePlantedYieldSum(s *state.State, flowerID int32, harvested bool) int32 {
	if s == nil || flowerID <= 0 {
		return 0
	}
	fallbackLvl := raceFlowerCultivateLevel(s, flowerID)
	var total int32
	for _, land := range s.Lands() {
		if int32(land.FlowerID) != flowerID {
			continue
		}
		lvl := int32(land.Lvl)
		if lvl <= 0 {
			lvl = fallbackLvl
		}
		yield, ok := state.FlowerLvlYieldByID(flowerID, lvl)
		if !ok || yield.CropGets <= 0 || yield.Frequencys <= 0 {
			continue
		}
		harvestedRounds := int32(land.HarvestCnt)
		if harvestedRounds < 0 {
			harvestedRounds = 0
		}
		if harvestedRounds > yield.Frequencys {
			harvestedRounds = yield.Frequencys
		}
		rounds := harvestedRounds
		if !harvested {
			rounds = yield.Frequencys - harvestedRounds
		}
		if rounds <= 0 {
			continue
		}
		total += rounds * yield.CropGets
	}
	return total
}

func hasRacePlantDemand(demands []Demand) bool {
	for _, demand := range demands {
		if demand.GoalID == raceActionGoal && demand.Source == "race_task" &&
			demand.Kind == DemandKindFlower && demand.Missing > 0 {
			return true
		}
	}
	return false
}

// raceSuppressesAutoReplant reports whether an unfinished plant-harvest race
// task should keep driving harvest/water (and race plant slots) even when
// ordinary farm auto toggles are off. Leftover empty lands still follow
// AutoEnabled for 自主补种 after race demand slots are assigned, except while
// the held task is expired (freeze until getTaskList clears it).
func raceSuppressesAutoReplant(s *state.State, policy *pb.Policy, now time.Time) bool {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() {
		return false
	}
	race := policy.GetUnion().GetRace()
	if race == nil || !race.GetEnabled() || !race.GetAutoEnableModules() {
		return false
	}
	taken := s.FmlRace().Taken
	if !taken.HasTask || taken.TaskType != raceTaskTypePlantHarvest {
		return false
	}
	if taken.TargetCnt <= 0 || taken.FinishCnt >= taken.TargetCnt {
		return false
	}
	if raceTakenExpired(taken, now) {
		// Freeze autonomous replant until getTaskList confirms the stale hold is
		// gone; otherwise an expired race task can unexpectedly refill the farm.
		return true
	}
	return !raceTakenBlocksProgress(s, race, s.FmlRace(), raceModuleGatesFromPolicy(policy))
}

// RaceModuleGates carries ordinary business-module switches that race take /
// abandon / progress gates must honor (mirrors AutoEnableModules limits).
// Flower-art sell (3030) and craft (3034) are race-driven and do not require
// order.flower_art sell_enabled / craft_enabled. Flower-cultivate (3044) is
// take/finish only and does not require plant.cultivate.
type RaceModuleGates struct {
	Customer  bool
	Pearl     bool
	Cultivate bool
}

func raceModuleGatesFromPolicy(policy *pb.Policy) RaceModuleGates {
	if policy == nil {
		return RaceModuleGates{}
	}
	return RaceModuleGates{
		Customer:  policy.GetOrder().GetCustomer().GetEnabled(),
		Pearl:     policy.GetBasic().GetPearl().GetAutoHireEnabled(),
		Cultivate: policy.GetPlant().GetCultivate().GetEnabled(),
	}
}

// raceTaskTypeAutoCompletable reports whether automation can take and finish
// this guild-race task type (business-module gates are checked separately).
func raceTaskTypeAutoCompletable(taskType int32) bool {
	switch taskType {
	case raceTaskTypePlantHarvest, raceTaskTypeCustomerOrder, raceTaskTypePearlHire,
		raceTaskTypeFlowerArtSell, raceTaskTypeFlowerArtCraft, raceTaskTypeFlowerCultivate:
		return true
	default:
		return false
	}
}

// RaceHoldsUnfinishedCustomerOrder reports whether the account currently holds
// an incomplete customer-order race task.
func RaceHoldsUnfinishedCustomerOrder(view state.FmlRaceView) bool {
	return raceHoldsUnfinishedType(view, raceTaskTypeCustomerOrder)
}

// RaceHoldsUnfinishedPearlHire reports whether the account currently holds an
// incomplete pearl-hire race task.
func RaceHoldsUnfinishedPearlHire(view state.FmlRaceView) bool {
	return raceHoldsUnfinishedType(view, raceTaskTypePearlHire)
}

// RaceHoldsUnfinishedFlowerArtSell reports whether the account currently
// holds an incomplete flower-art-sell race task.
func RaceHoldsUnfinishedFlowerArtSell(view state.FmlRaceView) bool {
	return raceHoldsUnfinishedType(view, raceTaskTypeFlowerArtSell)
}

// RaceHoldsUnfinishedFlowerArtCraft reports whether the account currently
// holds an incomplete flower-art-craft race task.
func RaceHoldsUnfinishedFlowerArtCraft(view state.FmlRaceView) bool {
	return raceHoldsUnfinishedType(view, raceTaskTypeFlowerArtCraft)
}

// RaceHoldsUnfinishedFlowerCultivate reports whether the account currently
// holds an incomplete flower-cultivate race task.
func RaceHoldsUnfinishedFlowerCultivate(view state.FmlRaceView) bool {
	return raceHoldsUnfinishedType(view, raceTaskTypeFlowerCultivate)
}

func raceHoldsUnfinishedType(view state.FmlRaceView, wantType int32) bool {
	taken := view.Taken
	if !taken.HasTask {
		return false
	}
	taskType := taken.TaskType
	if taskType == 0 {
		taskType = taken.TaskId
	}
	return taskType == wantType &&
		(taken.TargetCnt <= 0 || taken.FinishCnt < taken.TargetCnt)
}

// raceNeedsCustomerProgressSync reports that an unfinished customer-order race
// task should re-fetch getTaskList so FinishCnt can catch up after ordinary
// customer-order finishes.
func raceNeedsCustomerProgressSync(view state.FmlRaceView, now time.Time) bool {
	return raceNeedsModuleProgressSync(view, RaceHoldsUnfinishedCustomerOrder(view), now)
}

// raceNeedsPearlProgressSync reports that an unfinished pearl-hire race task
// should re-fetch getTaskList so FinishCnt can catch up after ordinary
// pearlPlace.hire ops.
func raceNeedsPearlProgressSync(view state.FmlRaceView, now time.Time) bool {
	return raceNeedsModuleProgressSync(view, RaceHoldsUnfinishedPearlHire(view), now)
}

// raceNeedsFlowerArtProgressSync reports that an unfinished flower-art sell
// or craft race task should re-fetch getTaskList so FinishCnt can catch up.
func raceNeedsFlowerArtProgressSync(view state.FmlRaceView, now time.Time) bool {
	return raceNeedsModuleProgressSync(view, RaceHoldsUnfinishedFlowerArtSell(view) || RaceHoldsUnfinishedFlowerArtCraft(view), now)
}

// raceNeedsCultivateProgressSync reports that an unfinished flower-cultivate
// race task should re-fetch getTaskList so FinishCnt can catch up. Not gated on
// plant.cultivate: sticky score-36 holds may advance manually while the module
// is off.
func raceNeedsCultivateProgressSync(view state.FmlRaceView, now time.Time) bool {
	return raceNeedsModuleProgressSync(view, RaceHoldsUnfinishedFlowerCultivate(view), now)
}

func raceNeedsModuleProgressSync(view state.FmlRaceView, holds bool, now time.Time) bool {
	if !holds {
		return false
	}
	taken := view.Taken
	if taken.TargetCnt > 0 && taken.FinishCnt >= taken.TargetCnt {
		return false
	}
	if !view.TasksObserved || view.TasksSyncedAtMs <= 0 {
		return true
	}
	return !now.Before(time.UnixMilli(view.TasksSyncedAtMs).Add(raceModuleProgressSyncInterval))
}

// driveRaceCustomerOrderOperations links ordinary customer-order finish ops to
// the held race demand so the UI/reason shows race progress pressure. It never
// enables the customer module by itself.
func driveRaceCustomerOrderOperations(policy *pb.Policy, demands []Demand, ops []PlannedOp) []PlannedOp {
	return driveRaceActionModuleOperations(policy, demands, ops, raceTaskTypeCustomerOrder,
		func(op PlannedOp) bool {
			return runnableBusinessOperation(op) && op.Kind == clientproto.RPCOrderCustomerFinishOrder.String()
		},
		func(missing int32) string {
			return fmt.Sprintf("公会竞赛顾客订单剩余 %d 次", missing)
		},
		func(p *pb.Policy) bool { return p != nil && p.GetOrder().GetCustomer().GetEnabled() },
	)
}

// driveRacePearlHireOperations links ordinary pearl-hire planner ops to the
// held race demand. It never enables auto-hire by itself.
func driveRacePearlHireOperations(policy *pb.Policy, demands []Demand, ops []PlannedOp) []PlannedOp {
	return driveRaceActionModuleOperations(policy, demands, ops, raceTaskTypePearlHire,
		func(op PlannedOp) bool {
			return op.FeatureID == "basic.pearl_hire" && racePearlPlannerKind(op.Kind)
		},
		func(missing int32) string {
			return fmt.Sprintf("公会竞赛珍珠雇佣剩余 %d 次", missing)
		},
		func(p *pb.Policy) bool { return p != nil && p.GetBasic().GetPearl().GetAutoHireEnabled() },
	)
}

// driveRaceFlowerArtSellOperations links ordinary flowerRack.sell ops to the
// held race demand, or emits claim/sell/craft when none is planned. Does not
// require sell_enabled — guild-race auto-complete owns progression for 3030.
func driveRaceFlowerArtSellOperations(s *state.State, policy *pb.Policy, demands []Demand, ops []PlannedOp, ledger *InventoryLedger, now time.Time) []PlannedOp {
	if policy == nil {
		return ops
	}
	wantSource := raceActionDemandSource(raceTaskTypeFlowerArtSell)
	var raceDemand Demand
	found := false
	for _, demand := range demands {
		if demand.GoalID == raceActionGoal && demand.Source == wantSource &&
			demand.Kind == DemandKindAction && demand.Missing > 0 {
			raceDemand = demand
			found = true
			break
		}
	}
	if !found {
		return ops
	}
	prefix := fmt.Sprintf("公会竞赛花艺售卖剩余 %d 次", raceDemand.Missing)

	// Free claimable racks first so sell can continue without sell_enabled.
	if claim, ok := raceFlowerArtClaimOperation(s, raceDemand, prefix, now); ok {
		return append(ops, claim)
	}
	// After 5 minutes of listing, cancel every occupied rack so the next ticks
	// can re-list and keep race sell progress moving.
	if cancels := raceFlowerArtCancelStaleListings(s, raceDemand, prefix, now); len(cancels) > 0 {
		return append(ops, cancels...)
	}

	matchSell := func(op PlannedOp) bool {
		return runnableBusinessOperation(op) && op.Kind == clientproto.RPCFlowerRackSell.String() &&
			op.TargetID > 0 && op.ItemID > 0 && op.Count > 0
	}
	if idx := deterministicOperationIndex(ops, matchSell); idx >= 0 {
		ops[idx].DemandID = raceDemand.ID
		if ops[idx].Reason == "" {
			ops[idx].Reason = prefix
		} else {
			ops[idx].Reason = prefix + "；" + ops[idx].Reason
		}
		count := ops[idx].Count
		if count > raceDemand.Missing {
			count = raceDemand.Missing
		}
		if ledger != nil {
			if available := ledger.Available(ops[idx].ItemID); count > available {
				count = available
			}
		}
		if count > flowerRackPerSlotCount {
			count = flowerRackPerSlotCount
		}
		if count <= 0 {
			return ops
		}
		ops[idx].Count = count
		ops[idx].ItemCost = map[int32]int32{ops[idx].ItemID: count}
		if ops[idx].Priority < raceFlowerArtSellOpPriority {
			ops[idx].Priority = raceFlowerArtSellOpPriority
		}
		return ops
	}

	if sell, ok := raceFlowerArtSellOperation(s, raceDemand, ledger, prefix); ok {
		return append(ops, sell)
	}
	// No finished art in stock — craft one batch so the next tick can list it.
	if craft, ok := raceFlowerArtCraftOperation(s, Demand{
		ID:      raceDemand.ID,
		Missing: raceDemand.Missing,
	}, ledger); ok {
		craft.Reason = prefix + "；缺少成品，先制作"
		if craft.Priority < raceFlowerArtCraftOpPriority {
			craft.Priority = raceFlowerArtCraftOpPriority
		}
		return append(ops, craft)
	}
	return ops
}

func raceFlowerArtClaimOperation(s *state.State, demand Demand, prefix string, now time.Time) (PlannedOp, bool) {
	if s == nil || demand.Missing <= 0 {
		return PlannedOp{}, false
	}
	claimable := s.FlowerRackClaimableSlotIDs(now)
	if len(claimable) == 0 {
		return PlannedOp{}, false
	}
	rackID := claimable[0]
	goal := Goal{ID: GoalFlowerArt, Category: CategoryOrder, Domain: "order.flower_art", Label: "花艺/花架", Priority: 40}
	claim := op(clientproto.RPCFlowerRackRecvSellMoney.String(), goal, "claim",
		prefix+"；花架售卖时间已到，领取以腾空位",
		raceFlowerArtClaimOpPriority, rackID, 0, 0)
	claim.DemandID = demand.ID
	if slot, ok := s.FlowerRackSlots()[rackID]; ok {
		claim.ItemID = slot.ItemID
		claim.Count = slot.Count
	}
	return claim, true
}

// raceFlowerArtCancelStaleListings cancels every occupied rack whose listing is
// at least raceFlowerArtRelistAfter old and not yet claimable. Claimable racks
// are left for recvSellMoney so gold is not discarded.
func raceFlowerArtCancelStaleListings(s *state.State, demand Demand, prefix string, now time.Time) []PlannedOp {
	if s == nil || demand.Missing <= 0 {
		return nil
	}
	nowMs := now.UnixMilli()
	cutoff := now.Add(-raceFlowerArtRelistAfter).UnixMilli()
	type stale struct {
		rackID int32
		slot   state.FlowerRackSlot
	}
	var staleSlots []stale
	for rackID, slot := range s.FlowerRackSlots() {
		if slot.ItemID <= 0 || slot.Count <= 0 || slot.ListedAtMs <= 0 {
			continue
		}
		if slot.SellReadyAtMs > 0 && nowMs >= slot.SellReadyAtMs {
			continue
		}
		if slot.ListedAtMs > cutoff {
			continue
		}
		staleSlots = append(staleSlots, stale{rackID: rackID, slot: slot})
	}
	if len(staleSlots) == 0 {
		return nil
	}
	sort.Slice(staleSlots, func(i, j int) bool { return staleSlots[i].rackID < staleSlots[j].rackID })
	goal := Goal{ID: GoalFlowerArt, Category: CategoryOrder, Domain: "order.flower_art", Label: "花艺/花架", Priority: 40}
	out := make([]PlannedOp, 0, len(staleSlots))
	for _, item := range staleSlots {
		cancel := op(clientproto.RPCFlowerRackCancelSell.String(), goal, "cancel",
			prefix+"；上架已满5分钟，全部下架再挂",
			raceFlowerArtCancelOpPriority, item.rackID, item.slot.ItemID, item.slot.Count)
		cancel.DemandID = demand.ID
		// Ordinary early-cancel is adapter-missing; race owns this path.
		cancel.Status = PlanStatusManaged
		cancel.Executable = true
		cancel.SyncOnly = false
		cancel.BlockedReasons = nil
		cancel.FeatureID = "union.race.flower_art_cancel"
		out = append(out, cancel)
	}
	return out
}

func raceFlowerArtSellOperation(s *state.State, demand Demand, ledger *InventoryLedger, prefix string) (PlannedOp, bool) {
	if s == nil || demand.Missing <= 0 {
		return PlannedOp{}, false
	}
	empty := s.EmptyFlowerRackSlotIDs()
	if len(empty) == 0 {
		return PlannedOp{}, false
	}
	if ledger == nil {
		ledger = NewInventoryLedger(s.Inventory())
	}
	artID, count, ok := bestRackArtForRace(s, ledger)
	if !ok || count <= 0 {
		return PlannedOp{}, false
	}
	if count > demand.Missing {
		count = demand.Missing
	}
	goal := Goal{ID: GoalFlowerArt, Category: CategoryOrder, Domain: "order.flower_art", Label: "花艺/花架", Priority: 40}
	sell := op(clientproto.RPCFlowerRackSell.String(), goal, "sell", prefix,
		raceFlowerArtSellOpPriority, empty[0], artID, count)
	sell.DemandID = demand.ID
	sell.ItemCost = map[int32]int32{artID: count}
	return sell, true
}

// driveRaceFlowerArtCraftOperations links ordinary makeFlowerArt ops to the
// held race demand, or emits a race-driven craft when none is planned. Does
// not require craft_enabled / sell_enabled — guild-race auto-complete owns
// progression for task type 3034.
func driveRaceFlowerArtCraftOperations(s *state.State, policy *pb.Policy, demands []Demand, ops []PlannedOp, ledger *InventoryLedger) []PlannedOp {
	if policy == nil {
		return ops
	}
	wantSource := raceActionDemandSource(raceTaskTypeFlowerArtCraft)
	var raceDemand Demand
	found := false
	for _, demand := range demands {
		if demand.GoalID == raceActionGoal && demand.Source == wantSource &&
			demand.Kind == DemandKindAction && demand.Missing > 0 {
			raceDemand = demand
			found = true
			break
		}
	}
	if !found {
		return ops
	}
	match := func(op PlannedOp) bool {
		if !runnableBusinessOperation(op) || op.Kind != clientproto.RPCFlowerArtMakeFlowerArt.String() {
			return false
		}
		return raceDemand.ItemID <= 0 || op.VaseID == raceDemand.ItemID
	}
	idx := deterministicOperationIndex(ops, match)
	if idx >= 0 {
		ops[idx].DemandID = raceDemand.ID
		prefix := fmt.Sprintf("公会竞赛花艺制作剩余 %d 次", raceDemand.Missing)
		if ops[idx].Reason == "" {
			ops[idx].Reason = prefix
		} else {
			ops[idx].Reason = prefix + "；" + ops[idx].Reason
		}
		if ops[idx].Count > raceDemand.Missing {
			ops[idx].Count = raceDemand.Missing
		}
		ops[idx].ItemCost = flowerArtCraftItemCost(ops[idx].FlowerIDs, ops[idx].Count)
		if ops[idx].Priority < raceFlowerArtCraftOpPriority {
			ops[idx].Priority = raceFlowerArtCraftOpPriority
		}
		return ops
	}
	craft, ok := raceFlowerArtCraftOperation(s, raceDemand, ledger)
	if !ok {
		return ops
	}
	return append(ops, craft)
}

// raceFlowerArtCraftOperation picks a craftable flower-art recipe for race
// progression: every recipe flower must already have seeds (cultivated), then
// the highest SaleValue wins. A race craft task carries its required vase in
// demand.ItemID, so unrelated unlocked vases are never used for progress.
// Unlike rack craft, it does not require sell_enabled, craft_enabled, or an
// empty rack.
func raceFlowerArtCraftOperation(s *state.State, demand Demand, ledger *InventoryLedger) (PlannedOp, bool) {
	if s == nil || demand.Missing <= 0 {
		return PlannedOp{}, false
	}
	if ledger == nil {
		ledger = NewInventoryLedger(s.Inventory())
	}
	recipe, count, ok := raceBestSeededCraftableRecipe(s, ledger, demand.Missing, demand.ItemID)
	if !ok {
		return PlannedOp{}, false
	}
	goal := Goal{ID: GoalFlowerArt, Category: CategoryOrder, Domain: "order.flower_art", Label: "花艺/花架", Priority: 40}
	reason := fmt.Sprintf("公会竞赛花艺制作剩余 %d 次；选价最高有种子配方", demand.Missing)
	if demand.ItemID > 0 {
		reason = fmt.Sprintf("公会竞赛花艺制作剩余 %d 次；目标花瓶 #%d 选价最高有种子配方", demand.Missing, demand.ItemID)
	}
	craft := op(clientproto.RPCFlowerArtMakeFlowerArt.String(), goal, "craft", reason,
		raceFlowerArtCraftOpPriority, 0, recipe.ArtID, count)
	craft.DemandID = demand.ID
	craft.VaseID = recipe.VaseID
	craft.FlowerIDs = append([]int32(nil), recipe.Flowers...)
	craft.ItemCost = flowerArtCraftItemCost(craft.FlowerIDs, craft.Count)
	return craft, true
}

// raceBestSeededCraftableRecipe returns the craftable recipe whose flowers are
// all cultivated (有种子) and whose SaleValue is highest. When vaseID is
// positive, recipes for every other vase are excluded.
func raceBestSeededCraftableRecipe(s *state.State, ledger *InventoryLedger, missing, vaseID int32) (state.FlowerArtRecipe, int32, bool) {
	var best state.FlowerArtRecipe
	var bestCount int32
	found := false
	for _, recipe := range rackCandidateRecipes() {
		if vaseID > 0 && recipe.VaseID != vaseID {
			continue
		}
		if !raceRecipeAllFlowersSeeded(s, recipe) {
			continue
		}
		if len(artBlockedReasons(s, recipe)) > 0 {
			continue
		}
		count := maxCraftableCount(recipe, ledger)
		if count <= 0 {
			continue
		}
		if count > missing {
			count = missing
		}
		if count > flowerRackPerSlotCount {
			count = flowerRackPerSlotCount
		}
		if !found || recipe.SaleValue > best.SaleValue ||
			(recipe.SaleValue == best.SaleValue && recipe.ArtID < best.ArtID) {
			best = recipe
			bestCount = count
			found = true
		}
	}
	return best, bestCount, found
}

func raceRecipeAllFlowersSeeded(s *state.State, recipe state.FlowerArtRecipe) bool {
	if len(recipe.Flowers) == 0 {
		return false
	}
	seen := make(map[int32]struct{}, len(recipe.Flowers))
	for _, flowerID := range recipe.Flowers {
		if flowerID <= 0 {
			return false
		}
		if _, ok := seen[flowerID]; ok {
			continue
		}
		seen[flowerID] = struct{}{}
		if !flowerCultivated(s, flowerID) {
			return false
		}
	}
	return true
}

func racePearlPlannerKind(kind string) bool {
	return cyclicNotePearlPlannerKind(kind)
}

func driveRaceActionModuleOperations(
	policy *pb.Policy,
	demands []Demand,
	ops []PlannedOp,
	taskType int32,
	match func(PlannedOp) bool,
	reasonPrefix func(missing int32) string,
	moduleOn func(*pb.Policy) bool,
) []PlannedOp {
	if !moduleOn(policy) || len(ops) == 0 {
		return ops
	}
	wantSource := raceActionDemandSource(taskType)
	var raceDemand Demand
	found := false
	for _, demand := range demands {
		if demand.GoalID == raceActionGoal && demand.Source == wantSource &&
			demand.Kind == DemandKindAction && demand.Missing > 0 {
			raceDemand = demand
			found = true
			break
		}
	}
	if !found {
		return ops
	}
	idx := deterministicOperationIndex(ops, match)
	if idx < 0 {
		return ops
	}
	ops[idx].DemandID = raceDemand.ID
	prefix := reasonPrefix(raceDemand.Missing)
	if ops[idx].Reason == "" {
		ops[idx].Reason = prefix
	} else {
		ops[idx].Reason = prefix + "；" + ops[idx].Reason
	}
	return ops
}

// raceTaskTypeLabel returns the Chinese label for a guild-race task type id.
func raceTaskTypeLabel(taskType int32) string {
	switch taskType {
	case 2004:
		return "VIP商店购买"
	case 3006:
		return "居民订单"
	case raceTaskTypeCustomerOrder:
		return "顾客订单"
	case 3017:
		return "材料商店购买"
	case 3018:
		return "宫廷订单"
	case raceTaskTypePearlHire:
		return "珍珠采集雇佣"
	case 3024:
		return "好友偷花"
	case raceTaskTypeFlowerArtSell:
		return "花艺售卖"
	case raceTaskTypeFlowerArtCraft:
		return "花艺制作"
	case 3035:
		return "鲜花升级"
	case raceTaskTypePlantHarvest:
		return "种植收获"
	case raceTaskTypeFlowerCultivate:
		return "花种培育"
	case 3052:
		return "动物互动"
	default:
		if taskType > 0 {
			return fmt.Sprintf("任务#%d", taskType)
		}
		return ""
	}
}

// FormatRaceTaskOpDesc builds the operator-facing title used in race take/finish
// logs, matching the UI card style: "种植收获 · 欢雪颂冬".
func FormatRaceTaskOpDesc(taskType, paramID int32) string {
	typeLabel := raceTaskTypeLabel(taskType)
	target := ""
	if paramID > 0 {
		target = state.ItemLabel(paramID)
	}
	switch {
	case typeLabel != "" && target != "":
		return typeLabel + " · " + target
	case typeLabel != "":
		return typeLabel
	case target != "":
		return target
	default:
		return ""
	}
}

// raceSpeedupEnabledAt reports whether guild-race policy should unlock farm
// speedup while an unfinished plant-harvest race task is held.
//
// Normal path: UseSpeedupTicketInTask for the whole held task. Forced
// completion guarantee: within raceExpireSpeedupLead of ExpireTime, spend
// tickets on the race flower even when that toggle (and ordinary planting
// speedup) are off, so the last-10-minute completion guarantee cannot be
// accidentally left disabled.
func raceSpeedupEnabledAt(s *state.State, race *pb.UnionRacePolicy, now time.Time) bool {
	if s == nil || race == nil || !race.GetEnabled() || !race.GetAutoEnableModules() {
		return false
	}
	taken := s.FmlRace().Taken
	if !taken.HasTask || taken.TaskType != raceTaskTypePlantHarvest ||
		taken.FinishCnt >= taken.TargetCnt || raceTakenBlocksProgress(s, race, s.FmlRace(), RaceModuleGates{Customer: true, Pearl: true, Cultivate: true}) {
		return false
	}
	if race.GetUseSpeedupTicketInTask() {
		return true
	}
	return raceExpireUrgentSpeedup(taken, now)
}

// raceExpireUrgentSpeedup is true when the held task has a known ExpireTime and
// now is inside the last raceExpireSpeedupLead before that deadline.
func raceExpireUrgentSpeedup(taken state.FmlRaceTakenView, now time.Time) bool {
	if taken.ExpireTime <= 0 || taken.TargetCnt <= 0 || taken.FinishCnt >= taken.TargetCnt {
		return false
	}
	deadline := time.UnixMilli(taken.ExpireTime)
	leadStart := deadline.Add(-raceExpireSpeedupLead)
	return !now.Before(leadStart) && now.Before(deadline)
}

func raceTakenExpired(taken state.FmlRaceTakenView, now time.Time) bool {
	return taken.HasTask && taken.ExpireTime > 0 && !now.Before(time.UnixMilli(taken.ExpireTime))
}
