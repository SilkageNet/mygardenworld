package apiserver

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fmlRaceTaskLabels maps race task type IDs to display labels.
var fmlRaceTaskLabels = map[int32]string{
	2004: "VIP商店购买",
	3006: "居民订单",
	3016: "顾客订单",
	3017: "材料商店购买",
	3018: "宫廷订单",
	3023: "珍珠采集雇佣",
	3024: "好友偷花",
	3030: "花艺售卖",
	3034: "花艺制作",
	3035: "鲜花升级",
	3036: "种植收获",
	3044: "花种培育",
	3052: "动物互动",
}

func fmlRaceProto(view state.FmlRaceView, s *state.State, racePolicy *pb.UnionRacePolicy, uid int64, now time.Time, gates automation.RaceModuleGates) *pb.FmlRaceView {
	out := &pb.FmlRaceView{
		AutoDeleteStatus: automation.RaceAutoDeleteStatus(s, racePolicy, now),
		Observed:         view.Observed,
		BatchActive:      view.ActiveAt(now),
		BatchStartMs:     view.BatchStartMs,
		BatchEndMs:       view.BatchEndMs,
		BatchStatus:      view.BatchStatus,
		TasksSyncedAtMs:  view.TasksSyncedAtMs,
	}
	if view.TaskQuotaObserved {
		out.TaskQuotaObserved = true
		out.FinishedTaskNum = view.FinishedTaskNum
		raceLvl := view.RaceLvl
		if raceLvl <= 0 {
			raceLvl = s.FmlBuild().RaceLvl
		}
		out.RaceLvl = raceLvl
		out.TotalTaskNum = state.FmlRaceTotalTaskNum(raceLvl, view.BuyTaskNum)
	}
	if view.ScoreObserved {
		out.ScoreObserved = true
		out.Score = view.Score
	}
	if view.RankObserved {
		out.RankObserved = true
		out.Rank = view.Rank
	}

	if view.Taken.HasTask {
		taskType := view.Taken.TaskType
		if taskType == 0 {
			taskType = view.Taken.TaskId
		}
		finishCnt := view.Taken.FinishCnt
		// Surface local harvest high-water when field 134 / pool FinishCnt lag
		// so the monitor progress bar updates as soon as race flowers are cut.
		if view.LocalFinishTaskMsId == view.Taken.TaskMsId && view.LocalFinishCnt > finishCnt {
			finishCnt = view.LocalFinishCnt
		}
		out.Taken = &pb.FmlRaceTaken{
			HasTask:      true,
			TaskMsId:     view.Taken.TaskMsId,
			TaskId:       view.Taken.TaskId,
			TaskType:     taskType,
			TaskLabel:    fmlRaceTaskLabels[taskType],
			TargetCnt:    view.Taken.TargetCnt,
			FinishCnt:    finishCnt,
			Score:        view.Taken.Score,
			TargetLabel:  view.Taken.TargetLabel,
			ExpireTimeMs: view.Taken.ExpireTime,
		}
	}

	for _, t := range view.Tasks {
		taskType := t.TaskType
		if taskType == 0 {
			taskType = t.TaskId
		}
		deleteBlockedReason := automation.RaceDeleteSkipReason(s, t, now)
		switch {
		case racePolicy == nil || !racePolicy.GetEnabled():
			deleteBlockedReason = "请先开启公会竞赛"
		case !view.Observed || !view.ActiveAt(now):
			deleteBlockedReason = "当前不在竞赛期间"
		case !view.TasksObserved || view.TaskPoolStale:
			deleteBlockedReason = "竞赛任务池尚未同步"
		}
		out.Tasks = append(out.Tasks, &pb.FmlRaceTask{
			MsId:                t.MsId,
			TaskId:              t.TaskId,
			TaskType:            taskType,
			TaskLabel:           fmlRaceTaskLabels[taskType],
			Score:               t.Score,
			IsUpgrade:           t.IsUpgrade != 0,
			UpgradeUid:          t.UpgradeUid,
			TargetLabel:         t.TargetLabel,
			AppearTimeMs:        t.AppearTime,
			TakeSkipReason:      automation.RaceTakeSkipReason(s, t, racePolicy, uid, now, gates),
			TargetCnt:           t.TargetCnt,
			FinishCnt:           t.FinishCnt,
			DeleteAllowed:       deleteBlockedReason == "",
			DeleteBlockedReason: deleteBlockedReason,
		})
	}
	return out
}

func activityItemsProto(items []state.ItemCount) []*pb.ActivityItem {
	out := make([]*pb.ActivityItem, 0, len(items))
	for _, item := range items {
		out = append(out, activityItemProto(item))
	}
	return out
}

// Project runner-owned safety into existing race status/button fields; the
// read model does not decide timing or perform game requests.
func applyRaceRequestSafety(view *pb.FmlRaceView, policy *pb.UnionRacePolicy, diag runner.Diagnostics) {
	if view == nil {
		return
	}
	cooldowns := cooldownsByOperation(diag)
	cd, paused := cooldowns["account.request"]
	if !paused {
		var ok bool
		cd, ok = cooldowns["union.race.delete.interval"]
		if !ok {
			return
		}
	}
	reason := cd.Reason
	if !paused {
		reason = fmt.Sprintf("%s · %s 后可重试", reason, cd.Until.Local().Format("15:04:05"))
	}
	if policy.GetDeleteLowScoreTask() {
		view.AutoDeleteStatus = reason
	}
	for _, task := range view.Tasks {
		task.DeleteAllowed = false
		task.DeleteBlockedReason = reason
		if paused {
			task.TakeSkipReason = cd.Reason
		}
	}
}

func activityItemProto(item state.ItemCount) *pb.ActivityItem {
	return &pb.ActivityItem{
		ItemId:   item.ItemID,
		ItemName: state.ItemName(item.ItemID),
		Count:    item.Count,
	}
}

func clampProgress(progress, target int32) int32 {
	if progress < 0 {
		return 0
	}
	if target > 0 && progress > target {
		return target
	}
	return progress
}

func timestampOrNil(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func buildPendingTasks(st *state.State) []*pb.PendingTaskView {
	return buildPendingTasksAt(st, time.Now())
}

func buildPendingTasksAt(st *state.State, now time.Time) []*pb.PendingTaskView {
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.MapEventEnabled = true
	return buildPendingTasksAtPolicy(st, now, policy)
}

func buildPendingTasksAtPolicy(st *state.State, now time.Time, policy *pb.Policy) []*pb.PendingTaskView {
	policy = automation.DefaultPolicyIfNil(policy)
	inventory := st.Inventory()
	var out []*pb.PendingTaskView

	flowerOrders := st.FlowerOrders()
	boxIDs := make([]int32, 0, len(flowerOrders))
	for boxID := range flowerOrders {
		boxIDs = append(boxIDs, boxID)
	}
	sort.Slice(boxIDs, func(i, j int) bool { return boxIDs[i] < boxIDs[j] })
	for _, boxID := range boxIDs {
		order := flowerOrders[boxID]
		if order == nil || order.IsVideo != 0 || len(order.Requires) == 0 {
			continue
		}
		reqs := flowerRequirements(order.Requires, inventory)
		status := requirementsStatus(reqs)
		var cooldownUntil int64
		var cooldownReason string
		if status == pb.PlanStatus_PLAN_STATUS_READY && !order.CooldownReady(now) {
			status = pb.PlanStatus_PLAN_STATUS_MANAGED
			cooldownUntil = order.CdTimeMs
			cooldownReason = "居民订单冷却中"
		}
		out = append(out, &pb.PendingTaskView{
			Category:                "居民订单",
			Id:                      strconv.FormatInt(int64(boxID), 10),
			Title:                   fmt.Sprintf("居民订单 #%d", boxID),
			Status:                  status,
			Requirements:            reqs,
			CooldownUntilMs:         cooldownUntil,
			CooldownReason:          cooldownReason,
			ExecutionFeature:        pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_RESIDENT_ORDER,
			AutoCompletionSupported: true,
		})
	}

	customerOrders := st.CustomerOrderDetails()
	npcIDs := make([]int32, 0, len(customerOrders))
	for npcID := range customerOrders {
		npcIDs = append(npcIDs, npcID)
	}
	sort.Slice(npcIDs, func(i, j int) bool { return npcIDs[i] < npcIDs[j] })
	for _, npcID := range npcIDs {
		order := customerOrders[npcID]
		if order == nil {
			continue
		}
		reqs := customerOrderRequirements(order, inventory)
		if len(reqs) == 0 {
			continue
		}
		out = append(out, &pb.PendingTaskView{
			Category:                "顾客订单",
			Id:                      strconv.FormatInt(int64(npcID), 10),
			Title:                   fmt.Sprintf("顾客订单 NPC=%d", npcID),
			Status:                  requirementsStatus(reqs),
			Requirements:            reqs,
			ExecutionFeature:        pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_CUSTOMER_ORDER,
			AutoCompletionSupported: true,
		})
	}

	if task, ok := st.MainTask(); ok && task.Valid && !task.Complete {
		title := state.MainTaskTitle(task.TaskID)
		if title == "" {
			title = fmt.Sprintf("主线任务 #%d", task.TaskID)
		}
		view := &pb.PendingTaskView{
			Category: "主线任务",
			Id:       strconv.FormatInt(int64(task.TaskID), 10),
			Title:    title,
			Finished: task.Finished,
			Target:   task.Target,
			Status:   pb.PlanStatus_PLAN_STATUS_MANAGED,
		}
		if !task.ProgressObserved || !task.ReceiptObserved {
			view.Status = pb.PlanStatus_PLAN_STATUS_BLOCKED
		} else if !task.Receipted && task.Target > 0 && task.Finished >= task.Target {
			view.Status = pb.PlanStatus_PLAN_STATUS_READY
		}
		if flowerID, target, ok := state.MainTaskFlowerTarget(task.TaskID); ok {
			view.ExecutionFeature = pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_PLANTING
			view.AutoCompletionSupported = true
			view.Requirements = []*pb.RequirementView{requirementView(flowerID, target, inventory[flowerID])}
		} else if flowerID, _, ok := state.MainTaskCultivateTarget(task.TaskID); ok {
			view.ExecutionFeature = pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_CULTIVATION
			view.AutoCompletionSupported = true
			cv, observed := st.Cultivations()[flowerID]
			switch {
			case observed && cv.Status == 1:
				view.CooldownUntilMs = cv.CulTimeMs
				view.CooldownReason = state.ItemName(flowerID) + "培育中"
			case observed && cv.Lvl > 0:
				view.CooldownReason = state.ItemName(flowerID) + "已培育，等待主线任务进度同步"
			default:
				if costs, known := state.CultivateCost(flowerID); known {
					view.Requirements = itemRequirements(costs, inventory)
				}
			}
		}
		out = append(out, view)
	}

	if story, ok := st.StoryMain(); ok {
		status := pb.PlanStatus_PLAN_STATUS_MANAGED
		reqs := itemRequirements(story.Cost, inventory)
		if len(reqs) > 0 {
			status = requirementsStatus(reqs)
		}
		title := story.SectionName
		if title == "" {
			title = fmt.Sprintf("剧情小节 #%d", story.SectionID)
		}
		out = append(out, &pb.PendingTaskView{
			Category:                "主线剧情",
			Id:                      strconv.FormatInt(int64(story.SectionID), 10),
			Title:                   title,
			Status:                  status,
			Requirements:            reqs,
			ExecutionFeature:        pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_STORY,
			AutoCompletionSupported: true,
		})
	}

	dailyTasks := st.DailyTasks()
	taskIDs := make([]int32, 0, len(dailyTasks))
	for id, task := range dailyTasks {
		if task.Receipted != 0 {
			continue
		}
		taskIDs = append(taskIDs, id)
	}
	sort.Slice(taskIDs, func(i, j int) bool { return taskIDs[i] < taskIDs[j] })
	for _, id := range taskIDs {
		task := dailyTasks[id]
		status := pb.PlanStatus_PLAN_STATUS_MANAGED
		if task.Status == 1 || (task.Status == 0 && task.Target > 0 && task.Finished >= task.Target) {
			status = pb.PlanStatus_PLAN_STATUS_READY
		}
		title := state.DailyTaskTitle(task.TaskID, task.Target)
		if title == "" {
			title = fmt.Sprintf("日常任务 #%d", task.TaskID)
		}
		feature, supported := state.DailyTaskExecutionFeature(task.ProgressType)
		view := &pb.PendingTaskView{
			Category:                "日常任务",
			Id:                      strconv.FormatInt(int64(id), 10),
			Title:                   title,
			Finished:                task.Finished,
			Target:                  task.Target,
			Status:                  status,
			ExecutionFeature:        taskExecutionFeatureProto(feature),
			AutoCompletionSupported: supported,
		}
		if status != pb.PlanStatus_PLAN_STATUS_READY && !supported {
			view.CooldownReason = "暂无安全自动执行协议，保持待处理"
		}
		out = append(out, view)
	}

	weeklyTasks := st.WeeklyTasks()
	weeklyIDs := make([]int32, 0, len(weeklyTasks))
	for id, task := range weeklyTasks {
		if task.Receipted != 0 {
			continue
		}
		weeklyIDs = append(weeklyIDs, id)
	}
	sort.Slice(weeklyIDs, func(i, j int) bool { return weeklyIDs[i] < weeklyIDs[j] })
	for _, id := range weeklyIDs {
		task := weeklyTasks[id]
		status := pb.PlanStatus_PLAN_STATUS_MANAGED
		if task.Status == 1 || (task.Status == 0 && task.Target > 0 && task.Finished >= task.Target) {
			status = pb.PlanStatus_PLAN_STATUS_READY
		}
		title := state.WeeklyTaskTitle(task.TaskID, task.Target)
		if title == "" {
			title = fmt.Sprintf("周常任务 #%d", task.TaskID)
		}
		out = append(out, &pb.PendingTaskView{
			Category:                "周常任务",
			Id:                      strconv.FormatInt(int64(id), 10),
			Title:                   title,
			Finished:                task.Finished,
			Target:                  task.Target,
			Status:                  status,
			ExecutionFeature:        pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_CLAIM_ONLY,
			AutoCompletionSupported: true,
		})
	}

	achievementTasks := st.AchievementTasks()
	achievementIDs := make([]int32, 0, len(achievementTasks))
	for id, task := range achievementTasks {
		if task.Receipted != 0 || !task.Current {
			continue
		}
		achievementIDs = append(achievementIDs, id)
	}
	sort.Slice(achievementIDs, func(i, j int) bool { return achievementIDs[i] < achievementIDs[j] })
	for _, id := range achievementIDs {
		task := achievementTasks[id]
		status := pb.PlanStatus_PLAN_STATUS_MANAGED
		if task.Status == 1 || (task.Status == 0 && task.Target > 0 && task.Finished >= task.Target) {
			status = pb.PlanStatus_PLAN_STATUS_READY
		}
		title := state.AchievementTaskTitle(task.TaskID)
		if title == "" {
			title = fmt.Sprintf("成就任务 #%d", task.TaskID)
		}
		out = append(out, &pb.PendingTaskView{
			Category:                "成就任务",
			Id:                      strconv.FormatInt(int64(id), 10),
			Title:                   title,
			Finished:                task.Finished,
			Target:                  task.Target,
			Status:                  status,
			ExecutionFeature:        pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_CLAIM_ONLY,
			AutoCompletionSupported: true,
		})
	}

	observed, mapValid, mapError := st.RandomEventMapStatus()
	disabledSuffix := ""
	if !policy.GetBasic().GetMapEventEnabled() {
		disabledSuffix = "（地图随机事件自动处理已关闭）"
	}
	if !observed {
		status := pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
		if !policy.GetBasic().GetMapEventEnabled() {
			status = pb.PlanStatus_PLAN_STATUS_MANAGED
		}
		out = append(out, &pb.PendingTaskView{
			Category: "地图随机事件",
			Id:       "sync",
			Title:    "地图随机事件同步" + disabledSuffix,
			Status:   status,
		})
	}
	if observed && !mapValid {
		if mapError == "" {
			mapError = "事件表格式无效"
		}
		out = append(out, &pb.PendingTaskView{
			Category: "地图随机事件",
			Id:       "invalid",
			Title:    "地图随机事件数据异常：" + mapError + disabledSuffix,
			Status:   pb.PlanStatus_PLAN_STATUS_BLOCKED,
		})
	}
	if observed && mapValid {
		events := st.RandomEvents()
		ids := make([]int32, 0, len(events))
		for id := range events {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			event := events[id]
			status := pb.PlanStatus_PLAN_STATUS_READY
			title := fmt.Sprintf("地图随机事件 #%d（位置 %d，对话 %d）", id, event.PositionIndex, event.DialogID)
			if !event.Valid {
				status = pb.PlanStatus_PLAN_STATUS_BLOCKED
				title += "：" + event.BlockedReason
			} else if !policy.GetBasic().GetMapEventEnabled() {
				status = pb.PlanStatus_PLAN_STATUS_MANAGED
			}
			title += disabledSuffix
			out = append(out, &pb.PendingTaskView{
				Category: "地图随机事件",
				Id:       strconv.FormatInt(int64(id), 10),
				Title:    title,
				Status:   status,
			})
		}
	}

	for _, evt := range st.ZooEventActions() {
		status := pb.PlanStatus_PLAN_STATUS_READY
		if evt.Blocked {
			status = pb.PlanStatus_PLAN_STATUS_BLOCKED
		}
		title := evt.Name
		if title == "" {
			title = fmt.Sprintf("宠物日志 #%d", evt.TableID)
		}
		if evt.Action == "read_log" && !evt.Blocked {
			title += "（待确认已读）"
		}
		if evt.BlockedReason != "" {
			title = fmt.Sprintf("%s：%s", title, evt.BlockedReason)
		}
		id := fmt.Sprintf("%d:%d", evt.PetID, evt.TableID)
		if evt.Action == "sync_logs" {
			id = "sync"
		}
		out = append(out, &pb.PendingTaskView{
			Category: "宠物事件",
			Id:       id,
			Title:    title,
			Status:   status,
		})
	}

	souvenirCount := st.ZooSouvenirCount()
	readySouvenirRewards := st.ReadyZooSouvenirRewardIDs()
	for _, index := range readySouvenirRewards {
		milestone, ok := state.ZooSouvenirCollectInfoByIndex(index)
		if !ok {
			continue
		}
		title := milestone.Description
		if title == "" {
			title = fmt.Sprintf("宠物纪念品收集奖励 #%d", index)
		}
		out = append(out, &pb.PendingTaskView{
			Category: "宠物纪念品",
			Id:       fmt.Sprintf("reward:%d", index),
			Title:    title,
			Finished: souvenirCount,
			Target:   milestone.Required,
			Status:   pb.PlanStatus_PLAN_STATUS_READY,
		})
	}
	rewardStateKnown := st.ZooSouvenirsObserved() && st.Zoo().SouvenirRewardIDsObserved
	for _, souvenirID := range st.UnreadZooSouvenirIDs() {
		title := state.ItemName(souvenirID)
		if title == "" {
			title = fmt.Sprintf("宠物纪念品 #%d", souvenirID)
		}
		status := pb.PlanStatus_PLAN_STATUS_READY
		if !rewardStateKnown || len(readySouvenirRewards) > 0 {
			status = pb.PlanStatus_PLAN_STATUS_BLOCKED
			title += "（待先确认收集奖励）"
		} else {
			title += "（未读）"
		}
		out = append(out, &pb.PendingTaskView{
			Category: "宠物纪念品",
			Id:       fmt.Sprintf("unread:%d", souvenirID),
			Title:    title,
			Status:   status,
		})
	}

	out = append(out, cyclicNotePendingTasks(st, now)...)
	out = append(out, cyclicStoryPendingTasks(st, now)...)

	return out
}

func taskExecutionFeatureProto(feature state.TaskExecutionFeature) pb.TaskExecutionFeature {
	switch feature {
	case state.TaskExecutionFeatureClaimOnly:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_CLAIM_ONLY
	case state.TaskExecutionFeatureStory:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_STORY
	case state.TaskExecutionFeaturePlanting:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_PLANTING
	case state.TaskExecutionFeatureResident:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_RESIDENT_ORDER
	case state.TaskExecutionFeatureFlowerRack:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_FLOWER_RACK
	case state.TaskExecutionFeatureCustomer:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_CUSTOMER_ORDER
	case state.TaskExecutionFeatureCultivateShop:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_CULTIVATE_SHOP
	case state.TaskExecutionFeaturePalace:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_PALACE_ORDER
	case state.TaskExecutionFeaturePearlHire:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_PEARL_HIRE
	case state.TaskExecutionFeatureFriendTouch:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_FRIEND_TOUCH
	case state.TaskExecutionFeatureVideo:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_VIDEO
	case state.TaskExecutionFeatureZooStroke:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_ZOO_STROKE
	case state.TaskExecutionFeatureCultivation:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_CULTIVATION
	default:
		return pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_UNSPECIFIED
	}
}

func cyclicNotePendingTasks(st *state.State, now time.Time) []*pb.PendingTaskView {
	view, _ := st.CyclicNoteView(now)
	return cyclicNotePendingTasksFromView(view)
}

func cyclicNotePendingTasksFromView(view state.CyclicNoteView) []*pb.PendingTaskView {
	if !view.Found || view.Phase != 2 {
		return nil
	}
	out := make([]*pb.PendingTaskView, 0, len(view.Tasks))
	for _, task := range view.Tasks {
		if !task.Unlocked || task.Received {
			continue
		}
		title := task.Title
		if title == "" {
			title = fmt.Sprintf("花笺集芳任务 #%d", task.TaskID)
		}
		status := cyclicNoteTaskStatus(view, task)
		out = append(out, &pb.PendingTaskView{
			Category: "activity",
			Id:       fmt.Sprintf("%d:%d:%d", view.BatchID, task.SlotID, task.TaskID),
			Title:    title,
			Finished: clampProgress(task.Progress, task.Target),
			Target:   task.Target,
			Status:   status,
		})
	}
	return out
}

func cyclicStoryPendingTasks(st *state.State, now time.Time) []*pb.PendingTaskView {
	view, _ := st.CyclicStoryView(now)
	return cyclicStoryPendingTasksFromView(view)
}

func cyclicStoryPendingTasksFromView(view state.CyclicStoryView) []*pb.PendingTaskView {
	if !view.Found || (view.Phase != 2 && view.Phase != 3) {
		return nil
	}
	out := make([]*pb.PendingTaskView, 0, len(view.Orders)+len(view.Milestones))
	if view.Phase == 2 {
		for _, order := range view.Orders {
			if order.OrderID <= 0 || order.OnCooldown {
				continue
			}
			title := fmt.Sprintf("莳花纪闻订单 #%d", order.OrderID)
			if order.FlowerID > 0 {
				title = fmt.Sprintf("莳花纪闻订单 #%d（花 %d x%d）", order.OrderID, order.FlowerID, order.Cost)
			}
			out = append(out, &pb.PendingTaskView{
				Category: "activity",
				Id:       fmt.Sprintf("story:%d:%d:%d", view.BatchID, order.OrderIdx, order.OrderID),
				Title:    title,
				Finished: 0,
				Target:   order.Cost,
				Status:   cyclicStoryOrderStatus(view, order),
			})
		}
	}
	for _, milestone := range view.Milestones {
		if milestone.Received || milestone.Target <= 0 {
			continue
		}
		ready := view.Valid && view.MilestoneReceiptsObserved && view.Score >= milestone.Target
		status := pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
		if ready {
			status = pb.PlanStatus_PLAN_STATUS_READY
		}
		out = append(out, &pb.PendingTaskView{
			Category: "activity",
			Id:       fmt.Sprintf("story-box:%d:%d", view.BatchID, milestone.Index),
			Title:    fmt.Sprintf("莳花纪闻积分奖励 #%d", milestone.Index),
			Finished: clampProgress(view.Score, milestone.Target),
			Target:   milestone.Target,
			Status:   status,
		})
	}
	return out
}
