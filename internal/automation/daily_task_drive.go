package automation

import (
	"fmt"
	"sort"
	"strconv"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const dailyTaskDrivePriorityFloor int32 = 6260

type dailyTaskActionDemand struct {
	Feature state.TaskExecutionFeature
	Demand  Demand
}

// dailyTaskActionDemands represents incomplete daily tasks whose owning
// business module is also enabled. The task switch never bypasses a module's
// resource, selection, cost, cooldown, or preflight policy.
func dailyTaskActionDemands(s *state.State, policy *pb.Policy) []dailyTaskActionDemand {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() || !policy.GetBasic().GetTask().GetDailyEnabled() {
		return nil
	}
	tasks := s.DailyTasks()
	ids := make([]int32, 0, len(tasks))
	for id := range tasks {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]dailyTaskActionDemand, 0, len(ids))
	for _, id := range ids {
		task := tasks[id]
		if task.Receipted != 0 || task.Target <= 0 || task.Finished < 0 || task.Finished >= task.Target {
			continue
		}
		feature, supported := state.DailyTaskExecutionFeature(task.ProgressType)
		if !supported || !dailyTaskBusinessModuleEnabled(policy, feature) {
			continue
		}
		missing := task.Target - task.Finished
		entityID := strconv.FormatInt(int64(task.TaskID), 10)
		out = append(out, dailyTaskActionDemand{
			Feature: feature,
			Demand: Demand{
				ID:        GoalDailyTask + ":" + entityID,
				GoalID:    GoalDailyTask,
				Category:  CategoryBasic,
				Domain:    "basic.task.daily",
				EntityID:  entityID,
				Source:    "task_type:" + strconv.FormatInt(int64(task.ProgressType), 10),
				Label:     state.DailyTaskTitle(task.TaskID, task.Target),
				Kind:      DemandKindAction,
				Count:     task.Target,
				Have:      task.Finished,
				Available: task.Finished,
				Missing:   missing,
				Priority:  priorityFor(policy.GetPlant().GetPlanting().GetDemandPriority(), GoalDailyTask),
			},
		})
	}
	return out
}

func dailyTaskBusinessModuleEnabled(policy *pb.Policy, feature state.TaskExecutionFeature) bool {
	if policy == nil {
		return false
	}
	switch feature {
	case state.TaskExecutionFeatureStory:
		return policy.GetBasic().GetTask().GetStoryEnabled()
	case state.TaskExecutionFeaturePlanting:
		return policy.GetPlant().GetPlanting().GetAutoEnabled()
	case state.TaskExecutionFeatureResident:
		return policy.GetOrder().GetResident().GetNormalEnabled()
	case state.TaskExecutionFeatureFlowerRack:
		return policy.GetOrder().GetFlowerArt().GetSellEnabled()
	case state.TaskExecutionFeatureCustomer:
		return policy.GetOrder().GetCustomer().GetEnabled()
	case state.TaskExecutionFeatureCultivateShop:
		return policy.GetBasic().GetShop().GetCultivateShop().GetAutoBuy()
	case state.TaskExecutionFeaturePearlHire:
		return policy.GetBasic().GetPearl().GetAutoHireEnabled()
	case state.TaskExecutionFeatureFriendTouch:
		return policy.GetPlant().GetFriendSteal().GetEnabled()
	case state.TaskExecutionFeatureZooStroke:
		zoo := policy.GetBasic().GetZoo()
		return zoo.GetEnabled() && zoo.GetAutoStroke()
	default:
		return false
	}
}

// driveDailyTaskOperations links one deterministic operation from the owning
// module to each task. It does not synthesize RPCs or turn disabled modules on.
func driveDailyTaskOperations(actions []dailyTaskActionDemand, ops []PlannedOp) []PlannedOp {
	for _, action := range actions {
		idx := deterministicOperationIndex(ops, func(op PlannedOp) bool {
			return dailyTaskOperationMatches(action.Feature, op)
		})
		if idx < 0 {
			continue
		}
		op := &ops[idx]
		op.DemandID = action.Demand.ID
		if op.Priority < dailyTaskDrivePriorityFloor {
			op.Priority = dailyTaskDrivePriorityFloor
		}
		prefix := fmt.Sprintf("日常任务剩余 %d 次", action.Demand.Missing)
		if op.Reason == "" {
			op.Reason = prefix
		} else {
			op.Reason = prefix + "；" + op.Reason
		}
	}
	return ops
}

func dailyTaskOperationMatches(feature state.TaskExecutionFeature, op PlannedOp) bool {
	if !runnableBusinessOperation(op) {
		return false
	}
	switch feature {
	case state.TaskExecutionFeatureStory:
		return op.Domain == "basic.story"
	case state.TaskExecutionFeaturePlanting:
		return op.Kind == clientproto.RPCUsrLandWater.String() || op.Kind == clientproto.RPCUsrLandWaterBatch.String()
	case state.TaskExecutionFeatureResident:
		return op.Kind == clientproto.RPCOrderFlowerFinishOrder.String()
	case state.TaskExecutionFeatureFlowerRack:
		return op.Kind == clientproto.RPCFlowerRackSell.String()
	case state.TaskExecutionFeatureCustomer:
		return op.GoalID == GoalCustomerOrder && (op.Action == "finish" || op.Action == "craft" || op.Action == "generate")
	case state.TaskExecutionFeatureCultivateShop:
		return op.Domain == "basic.shop.cultivate"
	case state.TaskExecutionFeaturePearlHire:
		return op.FeatureID == "basic.pearl_hire" && cyclicNotePearlPlannerKind(op.Kind)
	case state.TaskExecutionFeatureFriendTouch:
		return op.FeatureID == "plant.friend_steal"
	case state.TaskExecutionFeatureZooStroke:
		return op.Domain == "basic.zoo.stroke" || op.Domain == "basic.zoo"
	default:
		return false
	}
}
