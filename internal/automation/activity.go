package automation

import (
	"strconv"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	cyclicNoteModuleKey                 = "cyclicNote"
	cyclicNoteAutoClaimTaskRewardsKey   = "auto_claim_task_rewards"
	cyclicNoteAutoClaimProgressBoxesKey = "auto_claim_progress_boxes"
	cyclicNoteSatisfyTasksKey           = "satisfy_tasks"
	// Operation priorities use goalPriority*100 scale. Activity base 50 must
	// stay below main/major orders and above ordinary flower-rack work.
	cyclicNotePriority int32 = 50 * 100
)

// activityOperations returns at most one safe 花笺集芳 side operation per
// planning cycle. Missing bool params intentionally read as false.
func activityOperations(s *state.State, policy *pb.ActivityPolicy, now time.Time) []PlannedOp {
	if s == nil || policy == nil || !policy.GetEnabled() {
		return nil
	}
	module := policy.GetModules()[cyclicNoteModuleKey]
	if module == nil || !module.GetEnabled() {
		return nil
	}
	bools := module.GetBoolParams()
	claimTasks := bools[cyclicNoteAutoClaimTaskRewardsKey]
	claimMilestones := bools[cyclicNoteAutoClaimProgressBoxesKey]
	satisfyTasks := bools[cyclicNoteSatisfyTasksKey]
	if !claimTasks && !claimMilestones && !satisfyTasks {
		return nil
	}

	view, ok := s.CyclicNoteView(now)
	if !ok || !view.Valid || view.BatchID <= 0 {
		return nil
	}
	if (claimTasks || satisfyTasks) && !view.TaskListObserved {
		if snapshot, ready := s.CyclicNoteEnterSnapshot(now); ready && snapshot.BatchID == view.BatchID {
			planned := cyclicNotePlannedOp(
				clientproto.RPCActCyclicNoteEnter.String(), "enter", "活动任务尚未初始化，进入花笺集芳同步任务",
				snapshot.BatchID, 0, 0, 0,
			)
			return []PlannedOp{planned}
		}
		return nil
	}

	// Completed task rewards always precede score milestones. Server slot
	// order is retained, and duplicate task IDs fail closed in the snapshot.
	if claimTasks && view.Phase == 2 {
		for _, task := range view.Tasks {
			snapshot, ready := s.CyclicNoteTaskClaimSnapshot(now, view.BatchID, task.SlotID, task.TaskID)
			if !ready {
				continue
			}
			planned := cyclicNotePlannedOp(
				clientproto.RPCActCyclicNoteRecvTaskRwd.String(), "claim_task", "花笺集芳任务已完成，领取任务奖励",
				snapshot.BatchID, snapshot.SlotID, snapshot.TaskID, 0,
			)
			return []PlannedOp{planned}
		}
	}

	if claimMilestones && (view.Phase == 2 || view.Phase == 3) {
		for _, milestone := range view.Milestones {
			snapshot, ready := s.CyclicNoteMilestoneClaimSnapshot(now, view.BatchID, milestone.Index)
			if !ready {
				continue
			}
			planned := cyclicNotePlannedOp(
				clientproto.RPCActCyclicNoteRecv.String(), "claim_progress", "花笺集芳积分达到里程碑，领取进度奖励",
				snapshot.BatchID, 0, 0, snapshot.MilestoneIndex,
			)
			return []PlannedOp{planned}
		}
	}
	return nil
}

func cyclicNotePlannedOp(kind, action, reason string, batchID, slotID, taskID, milestoneIndex int32) PlannedOp {
	operationParts := []string{kind, strconv.FormatInt(int64(batchID), 10)}
	if taskID > 0 {
		operationParts = append(operationParts, strconv.FormatInt(int64(slotID), 10), strconv.FormatInt(int64(taskID), 10))
	} else if milestoneIndex > 0 {
		operationParts = append(operationParts, strconv.FormatInt(int64(milestoneIndex), 10))
	}
	return enrichPlannedOp(PlannedOp{
		OperationID:    strings.Join(operationParts, ":"),
		Kind:           kind,
		Lane:           LaneSide,
		FeatureID:      "activity.cyclicNote",
		Category:       CategoryActivity,
		Label:          "花笺集芳",
		Domain:         "activity.cyclicNote",
		Action:         action,
		Status:         PlanStatusManaged,
		Executable:     true,
		Reason:         reason,
		Priority:       cyclicNotePriority,
		BatchID:        batchID,
		SlotID:         slotID,
		TaskID:         taskID,
		MilestoneIndex: milestoneIndex,
	})
}
