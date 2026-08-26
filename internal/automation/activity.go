package automation

import (
	"strconv"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
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

	dessertModuleKey                       = "actDessert"
	dessertAutoClaimTaskRewardsKey         = "auto_claim_task_rewards"
	dessertAutoLikeCelebrityKey            = "auto_like_celebrity"
	dessertAutoClaimProgressBoxesKey       = "auto_claim_progress_boxes"
	dessertAutoOpenRewardBoxesKey          = "auto_open_reward_boxes"
	dessertPriority                  int32 = 50 * 100
	dessertCooldownKey                     = "activity.actDessert:reward"

	cyclicStoryModuleKey                       = "actCyclicStory"
	cyclicStoryAutoClaimOrderRewardsKey        = "auto_claim_order_rewards"
	cyclicStoryAutoClaimProgressBoxesKey       = "auto_claim_progress_boxes"
	cyclicStoryMaxScoreKey                     = "max_score"
	cyclicStoryPriority                  int32 = 50 * 100
)

// activityOperations combines independently gated activity modules. Each
// module contributes at most one operation per planning cycle. The parent
// ActivityPolicy.enabled field is ignored; only each module's own enabled
// flag gates execution.
func activityOperations(s *state.State, policy *pb.ActivityPolicy, now time.Time) []PlannedOp {
	if s == nil || policy == nil {
		return nil
	}
	operations := cyclicNoteOperations(s, policy, now)
	operations = append(operations, cyclicStoryOperations(s, policy, now)...)
	operations = append(operations, dessertOperations(s, policy, now)...)
	return operations
}

// cyclicNoteOperations returns at most one safe 花笺集芳 side operation per
// planning cycle. Missing bool params intentionally read as false.
func cyclicNoteOperations(s *state.State, policy *pb.ActivityPolicy, now time.Time) []PlannedOp {
	if s == nil || policy == nil {
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

// dessertOperations returns at most one capture-confirmed, cost-free dessert
// operation in the strict enter -> task -> celebrity sync -> like order.
func dessertOperations(s *state.State, policy *pb.ActivityPolicy, now time.Time) []PlannedOp {
	if s == nil || policy == nil {
		return nil
	}
	module := policy.GetModules()[dessertModuleKey]
	if module == nil || !module.GetEnabled() {
		return nil
	}
	claimTasks := module.GetBoolParams()[dessertAutoClaimTaskRewardsKey]
	likeCelebrity := module.GetBoolParams()[dessertAutoLikeCelebrityKey]
	claimProgress := module.GetBoolParams()[dessertAutoClaimProgressBoxesKey]
	openRewardBoxes := module.GetBoolParams()[dessertAutoOpenRewardBoxesKey]
	claimProgressReady := claimProgress && babigame.DessertProgressBoxEvidenceGate()
	openRewardBoxesReady := openRewardBoxes && babigame.DessertOpenRewardBoxEvidenceGate()
	if !claimTasks && !likeCelebrity && !claimProgressReady && !openRewardBoxesReady {
		return nil
	}

	if claimTasks || likeCelebrity || claimProgressReady {
		if snapshot, ready := s.DessertEnterSnapshot(now); ready {
			return []PlannedOp{dessertPlannedOp(
				clientproto.RPCActDessertEnter.String(), "enter", "香卉甜糕活动数据不完整，进入活动同步状态",
				snapshot.BatchID, 0,
			)}
		}
	}
	if openRewardBoxesReady {
		if snapshot, ready := s.DessertRewardBoxEnterSnapshot(now); ready {
			return []PlannedOp{dessertPlannedOp(
				clientproto.RPCActDessertEnter.String(), "enter", "香卉甜糕活动背包未同步，进入活动读取奖励箱余额",
				snapshot.BatchID, 0,
			)}
		}
	}

	view, ok := s.DessertView(now)
	viewReady := ok && view.Valid && view.BatchID > 0
	if viewReady && claimTasks && (view.Phase == 2 || view.Phase == 3) {
		for _, task := range view.Tasks {
			if _, ready := s.DessertTaskClaimSnapshot(now, view.BatchID, task.TaskIndex, task.TaskID); !ready {
				continue
			}
			return []PlannedOp{dessertPlannedOp(
				clientproto.RPCActRecv.String(), "claim_task", "香卉甜糕固定任务已完成，领取体力奖励",
				view.BatchID, task.TaskID,
			)}
		}
	}
	if viewReady && likeCelebrity && view.Phase == 2 {
		if _, ready := s.DessertCelebritySyncSnapshot(now); ready {
			return []PlannedOp{dessertPlannedOp(
				clientproto.RPCCelebrityGetAllTypesInfo.String(), "sync_celebrity", "点赞前受控同步本期名人榜",
				view.BatchID, 0,
			)}
		}
		if _, ready := s.DessertCelebrityLikeSnapshot(now, view.BatchID); ready {
			return []PlannedOp{dessertPlannedOp(
				clientproto.RPCCelebrityLikeCelebrity.String(), "like_celebrity", "香卉甜糕本期免费点赞奖励可领取",
				view.BatchID, 0,
			)}
		}
	}
	// Cumulative progress rewards remain independently hard-blocked until a
	// successful act.recvBoxes response fixture exists.
	if claimProgressReady {
		return nil
	}
	if openRewardBoxesReady {
		if snapshot, ready := s.DessertRewardBoxOpenSnapshot(now, 0, 1); ready {
			return []PlannedOp{dessertRewardBoxOpenPlannedOp(snapshot)}
		}
	}
	return nil
}

func dessertRewardBoxOpenPlannedOp(snapshot state.DessertRewardBoxOpenSnapshot) PlannedOp {
	op := dessertPlannedOp(
		clientproto.RPCActDessertOpenBox.String(), "open_box", "香卉甜糕奖励箱可安全单次开启",
		snapshot.BatchID, 0,
	)
	op.OperationID = strings.Join([]string{op.Kind, strconv.FormatInt(int64(snapshot.BatchID), 10), "1"}, ":")
	op.Count = 1
	op.CostGates = []CostGate{resourceGate(
		"activity_item:1347", GateResourceActivityItem, "甜糕奖励箱", snapshot.RewardBoxID,
		1, int64(snapshot.BalanceBefore), "operation.activity_bag",
	)}
	return enrichPlannedOp(op)
}

func dessertPlannedOp(kind, action, reason string, batchID, taskID int32) PlannedOp {
	parts := []string{kind, strconv.FormatInt(int64(batchID), 10)}
	if taskID > 0 {
		parts = append(parts, "0", strconv.FormatInt(int64(taskID), 10))
	}
	return enrichPlannedOp(PlannedOp{
		OperationID: strings.Join(parts, ":"),
		CooldownKey: dessertCooldownKey,
		Kind:        kind, Lane: LaneSide, FeatureID: "activity.actDessert." + action, Category: CategoryActivity,
		Label: "香卉甜糕", Domain: "activity.actDessert", Action: action, Status: PlanStatusManaged,
		Executable: true, Reason: reason, Priority: dessertPriority, BatchID: batchID, TaskID: taskID,
	})
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
	module := policy.GetModules()[cyclicStoryModuleKey]
	if module == nil || !module.GetEnabled() {
		return nil
	}
	bools := module.GetBoolParams()
	claimOrders := bools[cyclicStoryAutoClaimOrderRewardsKey]
	claimMilestones := bools[cyclicStoryAutoClaimProgressBoxesKey]
	if !claimOrders && !claimMilestones {
		return nil
	}
	maxScore := module.GetIntParams()[cyclicStoryMaxScoreKey]

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
