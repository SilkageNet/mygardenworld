package automation

import (
	"fmt"
	"strconv"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	raceTaskTypePlantHarvest int32 = 3036

	raceDemandPriority int32 = 85
	raceActionGoal           = "union.race"
)

// raceTaskProgressDemands converts the currently taken unfinished guild-race
// task into planner demands that drive regular farm modules.
//
// Progress itself is server-side (FinishCnt); the planner only ensures the
// underlying business actions happen so FinishCnt can advance, after which
// unionRaceOperations emits finishTask to claim the score reward.
func raceTaskProgressDemands(s *state.State, policy *pb.Policy) []Demand {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() {
		return nil
	}
	race := policy.GetUnion().GetRace()
	if race == nil || !race.GetEnabled() || !race.GetAutoEnableModules() {
		return nil
	}
	taken := s.FmlRace().Taken
	if !taken.HasTask || taken.TargetCnt <= 0 || taken.FinishCnt >= taken.TargetCnt {
		return nil
	}
	// Do not plant/harvest for a task we are about to give up (low score,
	// missing from pool, uncompletable, priority 0), and do not progress while
	// score is still unresolved under a min_task_score gate. Farm-lane plant
	// ops otherwise outrank side-lane giveUp and can fill the whole farm first.
	if raceTakenBlocksProgress(s, race, s.FmlRace()) {
		return nil
	}
	remaining := taken.TargetCnt - taken.FinishCnt
	switch taken.TaskType {
	case raceTaskTypePlantHarvest:
		if taken.ParamID <= 0 {
			return nil
		}
		// TargetCnt/FinishCnt are flower counts ("收获N朵"). Convert the
		// remaining flowers into empty-land plant slots using c_flowerLvl
		// cropGets × frequencys at the current cultivate level. Already-
		// planted lands of the same flower each count as one full slot so
		// mid-harvest top-ups cannot overshoot when FinishCnt lags.
		plantsMissing := racePlantHarvestPlantMissing(s, taken.ParamID, remaining)
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
	default:
		return nil
	}
}

// racePlantHarvestPlantMissing converts remaining race flower-counts into how
// many empty lands still need planting.
//
// slotsNeeded = ceil(remaining / (cropGets*frequencys)) at the cultivate level.
// Each land already planted with the target flower counts as one committed
// slot for its full lifetime — including mid-harvest — so a lagging FinishCnt
// after the first harvest round cannot trigger extra top-up planting.
func racePlantHarvestPlantMissing(s *state.State, flowerID, remainingFlowers int32) int32 {
	if remainingFlowers <= 0 || flowerID <= 0 {
		return 0
	}
	perPlant := raceFlowerFlowersPerPlant(s, flowerID)
	slotsNeeded := (remainingFlowers + perPlant - 1) / perPlant
	committed := racePlantedSlotCount(s, flowerID)
	if committed >= slotsNeeded {
		return 0
	}
	return slotsNeeded - committed
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

func racePlantedSlotCount(s *state.State, flowerID int32) int32 {
	if s == nil || flowerID <= 0 {
		return 0
	}
	var n int32
	for _, land := range s.Lands() {
		if int32(land.FlowerID) == flowerID {
			n++
		}
	}
	return n
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
// task is being progressed. While true, farm planning plants only the
// yield-calculated race slots and leaves remaining empty lands alone.
func raceSuppressesAutoReplant(s *state.State, policy *pb.Policy) bool {
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
	return !raceTakenBlocksProgress(s, race, s.FmlRace())
}

// raceTaskTypeLabel returns the Chinese label for a guild-race task type id.
func raceTaskTypeLabel(taskType int32) string {
	switch taskType {
	case 2004:
		return "VIP商店购买"
	case 3006:
		return "居民订单"
	case 3016:
		return "顾客订单"
	case 3017:
		return "材料商店购买"
	case 3018:
		return "宫廷订单"
	case 3023:
		return "珍珠采集雇佣"
	case 3024:
		return "好友偷花"
	case 3030:
		return "花艺售卖"
	case 3034:
		return "花艺制作"
	case 3035:
		return "鲜花升级"
	case raceTaskTypePlantHarvest:
		return "种植收获"
	case 3044:
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

// raceSpeedupEnabled reports whether guild-race policy should unlock farm
// speedup while an unfinished plant-harvest race task is held.
func raceSpeedupEnabled(s *state.State, race *pb.UnionRacePolicy) bool {
	if s == nil || race == nil || !race.GetEnabled() || !race.GetAutoEnableModules() || !race.GetUseSpeedupTicketInTask() {
		return false
	}
	taken := s.FmlRace().Taken
	return taken.HasTask && taken.TaskType == raceTaskTypePlantHarvest &&
		taken.FinishCnt < taken.TargetCnt && !raceTakenBlocksProgress(s, race, s.FmlRace())
}
