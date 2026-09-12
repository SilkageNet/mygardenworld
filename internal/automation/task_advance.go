package automation

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	taskAdvanceDemandPriority int32 = 58

	taskAdvanceProgressConsume         int32 = 4
	taskAdvanceProgressCultivateShopN  int32 = 1003
	taskAdvanceProgressVideoNum        int32 = 1005
	taskAdvanceProgressGuildShare      int32 = 2009
	taskAdvanceProgressGuildBuild      int32 = 2010
	taskAdvanceProgressPlantAny        int32 = 3001
	taskAdvanceProgressHarvestAny      int32 = 3002
	taskAdvanceProgressResidentOrder   int32 = 3006
	taskAdvanceProgressWater           int32 = 3014
	taskAdvanceProgressFlowerArtSell   int32 = 3015
	taskAdvanceProgressCustomerOrder   int32 = 3016
	taskAdvanceProgressCultivateShop   int32 = 3017
	taskAdvanceProgressPalaceOrder     int32 = 3018
	taskAdvanceProgressPearlHire       int32 = 3023
	taskAdvanceProgressFriendSteal     int32 = 3024
	taskAdvanceProgressVideo           int32 = 3025
	taskAdvanceProgressFlowerUpgrade   int32 = 3035
	taskAdvanceProgressElvesHarvest    int32 = 3037
	taskAdvanceProgressElvesAid        int32 = 3040
	taskAdvanceProgressElvesSteal      int32 = 3041
	taskAdvanceProgressCultivate       int32 = 3044
	taskAdvanceProgressElvesBook       int32 = 3045
	taskAdvanceProgressElvesDispatch   int32 = 3047
	taskAdvanceProgressMapEvent        int32 = 3052
)

type taskAdvanceSource string

const (
	taskAdvanceSourceDaily      taskAdvanceSource = "daily"
	taskAdvanceSourceFlowerPass taskAdvanceSource = "flower_pass"
	taskAdvanceSourceElvesPass  taskAdvanceSource = "elves_pass"
)

// taskAdvanceAction is one incomplete daily/pass progress type selected for drive.
type taskAdvanceAction struct {
	Source       taskAdvanceSource
	ProgressType int32
	FeatureID    string
	Demand       Demand
}

type taskAdvanceRep struct {
	taskID   int32
	progress int32
	target   int32
	title    string
}

func taskAdvanceActionDemands(s *state.State, policy *pb.Policy, now time.Time) []taskAdvanceAction {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() {
		return nil
	}
	task := policy.GetBasic().GetTask()
	if task == nil {
		return nil
	}
	var out []taskAdvanceAction
	if task.GetDailyAutoAdvance() {
		out = append(out, dailyTaskAdvanceActions(s, policy)...)
	}
	if task.GetFlowerPassAutoAdvance() {
		out = append(out, passBoardAdvanceActions(
			s.FlowerPassView(), policy,
			taskAdvanceSourceFlowerPass, "basic.flower_pass_advance", "basic.flower_pass",
		)...)
	}
	if task.GetElvesPassAutoAdvance() {
		out = append(out, passBoardAdvanceActions(
			s.FlowerElvesPassView(), policy,
			taskAdvanceSourceElvesPass, "basic.elves_pass_advance", "basic.elves_pass",
		)...)
	}
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].ProgressType < out[j].ProgressType
	})
	return out
}

func dailyTaskAdvanceActions(s *state.State, policy *pb.Policy) []taskAdvanceAction {
	byType := map[int32]taskAdvanceRep{}
	for id, task := range s.DailyTasks() {
		if task.Receipted != 0 || task.Target <= 0 || task.Finished >= task.Target {
			continue
		}
		progressType := task.ProgressType
		if progressType <= 0 {
			if pt, ok := state.DailyTaskProgressType(task.TaskID); ok {
				progressType = pt
			}
		}
		if !taskAdvanceProgressSupported(progressType) || !taskAdvanceModuleEnabled(policy, progressType) {
			continue
		}
		remaining := task.Target - task.Finished
		if current, exists := byType[progressType]; exists {
			curRemain := current.target - current.progress
			if curRemain < remaining || (curRemain == remaining && current.taskID <= id) {
				continue
			}
		}
		title := state.DailyTaskTitle(task.TaskID, task.Target)
		if title == "" {
			title = fmt.Sprintf("日常任务 #%d", task.TaskID)
		}
		byType[progressType] = taskAdvanceRep{taskID: id, progress: task.Finished, target: task.Target, title: title}
	}
	return advanceActionsFromReps(byType, taskAdvanceSourceDaily, "basic.task_daily_advance", "basic.task.daily", false, 0)
}

func passBoardAdvanceActions(
	view state.PassBoardView,
	policy *pb.Policy,
	source taskAdvanceSource,
	featureID, goalID string,
) []taskAdvanceAction {
	if !view.Found || view.Bid <= 0 {
		return nil
	}
	byType := map[int32]taskAdvanceRep{}
	for _, task := range view.Tasks {
		if task.Received || task.Target <= 0 || task.Progress >= task.Target || !task.CatalogKnown {
			continue
		}
		if !taskAdvanceProgressSupported(task.ProgressType) || !taskAdvanceModuleEnabled(policy, task.ProgressType) {
			continue
		}
		remaining := task.Target - task.Progress
		if current, exists := byType[task.ProgressType]; exists {
			curRemain := current.target - current.progress
			if curRemain < remaining || (curRemain == remaining && current.taskID <= task.TaskID) {
				continue
			}
		}
		title := task.Title
		if title == "" {
			title = fmt.Sprintf("密令任务 #%d", task.TaskID)
		}
		byType[task.ProgressType] = taskAdvanceRep{
			taskID: task.TaskID, progress: task.Progress, target: task.Target, title: title,
		}
	}
	return advanceActionsFromReps(byType, source, featureID, goalID, true, view.Bid)
}

func advanceActionsFromReps(
	byType map[int32]taskAdvanceRep,
	source taskAdvanceSource,
	featureID, goalID string,
	includeBid bool,
	bid int32,
) []taskAdvanceAction {
	if len(byType) == 0 {
		return nil
	}
	types := make([]int32, 0, len(byType))
	for pt := range byType {
		types = append(types, pt)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	out := make([]taskAdvanceAction, 0, len(types))
	for _, pt := range types {
		rep := byType[pt]
		missing := rep.target - rep.progress
		if missing < 0 {
			missing = 0
		}
		entityID := strconv.FormatInt(int64(rep.taskID), 10)
		if includeBid {
			entityID = strconv.FormatInt(int64(bid), 10) + ":" + entityID
		}
		out = append(out, taskAdvanceAction{
			Source:       source,
			ProgressType: pt,
			FeatureID:    featureID,
			Demand: Demand{
				ID:        goalID + ":" + entityID,
				GoalID:    goalID,
				Category:  CategoryBasic,
				Domain:    goalID,
				EntityID:  entityID,
				Source:    "progress_type:" + strconv.FormatInt(int64(pt), 10),
				Label:     rep.title,
				Kind:      DemandKindAction,
				Count:     rep.target,
				Have:      rep.progress,
				Available: rep.progress,
				Missing:   missing,
				Priority:  taskAdvanceDemandPriority,
			},
		})
	}
	return out
}

func taskAdvanceProgressSupported(progressType int32) bool {
	switch progressType {
	case taskAdvanceProgressPlantAny, taskAdvanceProgressHarvestAny, taskAdvanceProgressWater,
		taskAdvanceProgressResidentOrder, taskAdvanceProgressFlowerArtSell, taskAdvanceProgressCustomerOrder,
		taskAdvanceProgressCultivateShop, taskAdvanceProgressCultivateShopN, taskAdvanceProgressPalaceOrder,
		taskAdvanceProgressPearlHire, taskAdvanceProgressFriendSteal, taskAdvanceProgressMapEvent,
		taskAdvanceProgressGuildBuild, taskAdvanceProgressFlowerUpgrade, taskAdvanceProgressCultivate,
		taskAdvanceProgressElvesHarvest, taskAdvanceProgressElvesAid, taskAdvanceProgressElvesSteal:
		return true
	case taskAdvanceProgressVideo, taskAdvanceProgressVideoNum, taskAdvanceProgressGuildShare,
		taskAdvanceProgressElvesBook, taskAdvanceProgressElvesDispatch, taskAdvanceProgressConsume:
		return false
	default:
		return false
	}
}

func taskAdvanceModuleEnabled(policy *pb.Policy, progressType int32) bool {
	if policy == nil {
		return false
	}
	switch progressType {
	case taskAdvanceProgressPlantAny, taskAdvanceProgressHarvestAny, taskAdvanceProgressWater,
		taskAdvanceProgressElvesHarvest:
		// Self-drive farm cycle / plant without requiring plant.auto.
		return true
	case taskAdvanceProgressFlowerArtSell:
		return true // self-drive rack sell/claim like cyclic note
	case taskAdvanceProgressResidentOrder:
		return policy.GetOrder().GetResident().GetNormalEnabled()
	case taskAdvanceProgressCustomerOrder:
		return policy.GetOrder().GetCustomer().GetEnabled()
	case taskAdvanceProgressPalaceOrder:
		return policy.GetOrder().GetPalace().GetEnabled()
	case taskAdvanceProgressPearlHire:
		return policy.GetBasic().GetPearl().GetAutoHireEnabled()
	case taskAdvanceProgressFriendSteal:
		return policy.GetPlant().GetFriendSteal().GetEnabled()
	case taskAdvanceProgressElvesSteal:
		return policy.GetPlant().GetFriendSteal().GetEnabled() &&
			(policy.GetPlant().GetFriendSteal().GetStealElves() || policy.GetPlant().GetElvesPlant().GetStealFriendElvesEnabled())
	case taskAdvanceProgressElvesAid:
		return policy.GetPlant().GetElves().GetHelpFriend()
	case taskAdvanceProgressMapEvent:
		return policy.GetBasic().GetMapEventEnabled()
	case taskAdvanceProgressCultivateShop, taskAdvanceProgressCultivateShopN:
		return policy.GetBasic().GetShop().GetCultivateShop().GetAutoBuy()
	case taskAdvanceProgressGuildBuild:
		return policy.GetUnion().GetBuild().GetGoldEnabled()
	case taskAdvanceProgressFlowerUpgrade:
		return policy.GetPlant().GetCultivate().GetUpgradeEnabled()
	case taskAdvanceProgressCultivate:
		return policy.GetPlant().GetCultivate().GetEnabled()
	default:
		return false
	}
}

func taskAdvanceForceFarmCycle(actions []taskAdvanceAction) bool {
	for _, action := range actions {
		switch action.ProgressType {
		case taskAdvanceProgressPlantAny, taskAdvanceProgressHarvestAny, taskAdvanceProgressWater,
			taskAdvanceProgressElvesHarvest:
			return true
		}
	}
	return false
}

func taskAdvanceForceFarmPlant(actions []taskAdvanceAction) bool {
	for _, action := range actions {
		if action.ProgressType == taskAdvanceProgressPlantAny && action.Demand.Missing > 0 {
			return true
		}
	}
	return false
}

func driveTaskAdvanceOperations(
	s *state.State,
	policy *pb.Policy,
	actions []taskAdvanceAction,
	ledger *InventoryLedger,
	ops []PlannedOp,
	now time.Time,
) []PlannedOp {
	if policy == nil || len(actions) == 0 {
		return ops
	}
	for _, action := range actions {
		if action.Demand.Missing <= 0 && action.ProgressType != taskAdvanceProgressFlowerArtSell {
			continue
		}
		switch action.ProgressType {
		case taskAdvanceProgressPlantAny:
			ops = driveTaskAdvancePlant(action, ops)
		case taskAdvanceProgressFlowerArtSell:
			ops = driveCyclicNoteFlowerRackOperations(s, action.Demand, ledger, ops, now)
			tagTaskAdvanceFeature(ops, action)
		case taskAdvanceProgressCustomerOrder:
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) && op.Kind == clientproto.RPCOrderCustomerFinishOrder.String()
			})
		case taskAdvanceProgressResidentOrder:
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) &&
					(op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() ||
						op.Kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() ||
						op.Kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String())
			})
		case taskAdvanceProgressPalaceOrder:
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) && op.Kind == clientproto.RPCOrderPalaceFinishOrder.String()
			})
		case taskAdvanceProgressPearlHire:
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) && op.FeatureID == "basic.pearl_hire" &&
					cyclicNotePearlPlannerKind(op.Kind)
			})
		case taskAdvanceProgressFriendSteal:
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) && op.FeatureID == "plant.friend_steal"
			})
		case taskAdvanceProgressElvesSteal:
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) &&
					(op.FeatureID == "plant.friend_steal_elves" || op.FeatureID == "plant.elves_plant")
			})
		case taskAdvanceProgressElvesAid:
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) && op.FeatureID == "plant.elves_aid_help"
			})
		case taskAdvanceProgressMapEvent:
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) &&
					(op.Kind == clientproto.RPCRandomEventDoAffair.String() || op.FeatureID == "basic.map_event")
			})
		case taskAdvanceProgressCultivateShop, taskAdvanceProgressCultivateShopN:
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) && op.Kind == clientproto.RPCShopCultivateBuy.String()
			})
		case taskAdvanceProgressGuildBuild:
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) && op.Kind == clientproto.RPCFmlBld.String() &&
					op.FeatureID != "union.build_video"
			})
		case taskAdvanceProgressFlowerUpgrade:
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) &&
					(op.FeatureID == "plant.upgrade" || op.Domain == "farm.upgrade")
			})
		case taskAdvanceProgressCultivate:
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) &&
					(op.Kind == clientproto.RPCCultivateCultivate.String() || op.FeatureID == "plant.cultivate")
			})
		case taskAdvanceProgressHarvestAny, taskAdvanceProgressWater, taskAdvanceProgressElvesHarvest:
			// Force-farm already enables water/harvest; tag a runnable farm op when present.
			linkTaskAdvanceOperation(action, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) &&
					(op.Kind == clientproto.RPCUsrLandHarvest.String() ||
						op.Kind == clientproto.RPCUsrLandHarvestOneKey.String() ||
						op.Kind == clientproto.RPCUsrLandWater.String() ||
						op.Kind == clientproto.RPCUsrLandWaterBatch.String())
			})
		}
	}
	return ops
}

func driveTaskAdvancePlant(action taskAdvanceAction, ops []PlannedOp) []PlannedOp {
	ops = driveCyclicNotePlant(action.Demand, ops)
	tagTaskAdvanceFeature(ops, action)
	return ops
}

func linkTaskAdvanceOperation(action taskAdvanceAction, ops []PlannedOp, match func(PlannedOp) bool) {
	idx := deterministicOperationIndex(ops, match)
	if idx < 0 {
		return
	}
	ops[idx].DemandID = action.Demand.ID
	ops[idx].FeatureID = action.FeatureID
	ops[idx].Reason = taskAdvanceDriveReason(action, ops[idx].Reason)
}

func tagTaskAdvanceFeature(ops []PlannedOp, action taskAdvanceAction) {
	for i := range ops {
		if ops[i].DemandID == action.Demand.ID {
			ops[i].FeatureID = action.FeatureID
			if ops[i].Reason == "" || !strings.Contains(ops[i].Reason, "剩余") {
				ops[i].Reason = taskAdvanceDriveReason(action, ops[i].Reason)
			}
		}
	}
}

func taskAdvanceDriveReason(action taskAdvanceAction, existing string) string {
	label := "任务推进"
	switch action.Source {
	case taskAdvanceSourceDaily:
		label = "每日任务"
	case taskAdvanceSourceFlowerPass:
		label = "花之密令"
	case taskAdvanceSourceElvesPass:
		label = "花灵密令"
	}
	prefix := fmt.Sprintf("%s剩余 %d 次", label, action.Demand.Missing)
	if existing == "" {
		return prefix
	}
	return prefix + "；" + existing
}
