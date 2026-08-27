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
	// Operation priorities use goalPriority*100 scale. Activity base 50 must
	// stay below main/major orders and above ordinary flower-rack work.
	cyclicNotePriority int32 = 50 * 100

	cyclicStoryPriority int32 = 50 * 100
)

// activityOperations combines independently gated activity modules. Each
// module contributes at most one operation per planning cycle.
func activityOperations(s *state.State, policy *pb.ActivityPolicy, now time.Time) []PlannedOp {
	if s == nil || policy == nil {
		return nil
	}
	operations := cyclicNoteOperations(s, policy, now)
	operations = append(operations, cyclicStoryOperations(s, policy, now)...)
	return operations
}

// cyclicNoteOperations returns at most one safe 花笺集芳 side operation per
// planning cycle.
func cyclicNoteOperations(s *state.State, policy *pb.ActivityPolicy, now time.Time) []PlannedOp {
	if s == nil || policy == nil {
		return nil
	}
	module := policy.GetCyclicNote()
	if module == nil || !module.GetEnabled() {
		return nil
	}
	claimTasks := module.GetAutoClaimTaskRewards()
	claimMilestones := module.GetAutoClaimProgressBoxes()
	satisfyTasks := module.GetSatisfyTasks()
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

// cyclicStoryOperations returns at most one safe 莳花纪闻 side operation per
// planning cycle. max_score=0 means unlimited; reaching the cap blocks order
// claims that would raise score while milestones remain claimable.
func cyclicStoryOperations(s *state.State, policy *pb.ActivityPolicy, now time.Time) []PlannedOp {
	if s == nil || policy == nil {
		return nil
	}
	module := policy.GetCyclicStory()
	if module == nil || !module.GetEnabled() {
		return nil
	}
	claimOrders := module.GetAutoClaimOrderRewards()
	claimMilestones := module.GetAutoClaimProgressBoxes()
	if !claimOrders && !claimMilestones {
		return nil
	}
	maxScore := module.GetMaxScore()

	view, ok := s.CyclicStoryView(now)
	if !ok || !view.Found || view.BatchID <= 0 {
		return nil
	}
	// Enter bootstraps fresh batches (and resyncs invalid order payloads)
	// before score/bag make Valid=true.
	if claimOrders || claimMilestones {
		if snapshot, ready := s.CyclicStoryEnterSnapshot(now); ready && snapshot.BatchID == view.BatchID {
			reason := "活动订单尚未初始化，进入莳花纪闻同步状态"
			if view.OrdersObserved && !view.OrdersValid {
				reason = "莳花纪闻订单状态异常，重新进入同步"
			}
			return []PlannedOp{cyclicStoryPlannedOp(
				clientproto.RPCActCyclicStoryEnter.String(), "enter", reason,
				snapshot.BatchID, 0, 0, 0, 0, nil,
			)}
		}
	}
	if !view.Valid {
		return nil
	}

	underScoreCap := maxScore <= 0 || int64(view.Score) < maxScore
	if claimOrders && view.Phase == 2 && underScoreCap {
		for _, order := range view.Orders {
			snapshot, ready := s.CyclicStoryOrderClaimSnapshot(now, view.BatchID, order.OrderIdx)
			if !ready {
				continue
			}
			return []PlannedOp{cyclicStoryPlannedOp(
				clientproto.RPCActCyclicStoryRecvOrderRwd.String(), "claim_order", "莳花纪闻订单材料已齐，领取订单奖励",
				snapshot.BatchID, snapshot.OrderIdx, snapshot.OrderID, 0, snapshot.FlowerID,
				map[int32]int32{snapshot.FlowerID: snapshot.Cost},
			)}
		}
	}

	if claimMilestones && (view.Phase == 2 || view.Phase == 3) {
		for _, milestone := range view.Milestones {
			snapshot, ready := s.CyclicStoryMilestoneClaimSnapshot(now, view.BatchID, milestone.Index)
			if !ready {
				continue
			}
			return []PlannedOp{cyclicStoryPlannedOp(
				clientproto.RPCActCyclicStoryRecv.String(), "claim_progress", "莳花纪闻积分达到里程碑，领取进度奖励",
				snapshot.BatchID, 0, 0, snapshot.MilestoneIndex, 0, nil,
			)}
		}
	}
	return nil
}

func cyclicStoryPlannedOp(kind, action, reason string, batchID, orderIdx, orderID, milestoneIndex, flowerID int32, itemCost map[int32]int32) PlannedOp {
	operationParts := []string{kind, strconv.FormatInt(int64(batchID), 10)}
	if orderID > 0 {
		operationParts = append(operationParts, strconv.FormatInt(int64(orderIdx), 10), strconv.FormatInt(int64(orderID), 10))
	} else if milestoneIndex > 0 {
		operationParts = append(operationParts, strconv.FormatInt(int64(milestoneIndex), 10))
	}
	return enrichPlannedOp(PlannedOp{
		OperationID:    strings.Join(operationParts, ":"),
		Kind:           kind,
		Lane:           LaneSide,
		FeatureID:      "activity.actCyclicStory." + action,
		Category:       CategoryActivity,
		Label:          "莳花纪闻",
		Domain:         "activity.actCyclicStory",
		Action:         action,
		Status:         PlanStatusManaged,
		Executable:     true,
		Reason:         reason,
		Priority:       cyclicStoryPriority,
		BatchID:        batchID,
		SlotID:         orderIdx,
		TaskID:         orderID,
		MilestoneIndex: milestoneIndex,
		FlowerID:       flowerID,
		ItemCost:       itemCost,
	})
}
