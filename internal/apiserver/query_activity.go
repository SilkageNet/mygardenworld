package apiserver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
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
		Observed:        view.Observed,
		BatchActive:     view.ActiveAt(now),
		BatchStartMs:    view.BatchStartMs,
		BatchEndMs:      view.BatchEndMs,
		BatchStatus:     view.BatchStatus,
		TasksSyncedAtMs: view.TasksSyncedAtMs,
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
		out.Tasks = append(out.Tasks, &pb.FmlRaceTask{
			MsId:           t.MsId,
			TaskId:         t.TaskId,
			TaskType:       taskType,
			TaskLabel:      fmlRaceTaskLabels[taskType],
			Score:          t.Score,
			IsUpgrade:      t.IsUpgrade != 0,
			UpgradeUid:     t.UpgradeUid,
			TargetLabel:    t.TargetLabel,
			AppearTimeMs:   t.AppearTime,
			TakeSkipReason: automation.RaceTakeSkipReason(s, t, racePolicy, uid, now, gates),
		})
	}
	return out
}

func dessertProto(view state.DessertView) *pb.DessertView {
	out := &pb.DessertView{
		Observed:                  view.Observed,
		Found:                     view.Found,
		Valid:                     view.Valid,
		BatchId:                   view.BatchID,
		TemplateId:                view.TmpID,
		TemplateType:              view.TmpType,
		Status:                    view.Status,
		Phase:                     view.Phase,
		PhaseEndMs:                view.PhaseEndMs,
		BeginMs:                   view.BeginMs,
		VisibleStartMs:            view.VisibleStartMs,
		EndMs:                     view.EndMs,
		GraceEndMs:                view.GraceEndMs,
		Name:                      view.Name,
		Description:               view.Description,
		DropCount:                 view.DropCount,
		TotalScore:                view.TotalScore,
		EnergyItemId:              view.EnergyItemID,
		EnergyBalance:             view.EnergyBalance,
		CurrencyItemId:            view.CurrencyItemID,
		CurrencyBalance:           view.CurrencyBalance,
		PointItemId:               view.PointItemID,
		RewardBoxItemId:           view.RewardBoxItemID,
		RewardBoxBalance:          view.RewardBoxBalance,
		DropCountObserved:         view.DropCountObserved,
		TotalScoreObserved:        view.TotalScoreObserved,
		BagObserved:               view.BagObserved,
		ExtensionObserved:         view.ExtensionObserved,
		ExtensionValid:            view.ExtensionValid,
		ModeMapObserved:           view.ModeMapObserved,
		ModeMapValid:              view.ModeMapValid,
		TaskGroupsObserved:        view.TaskGroupsObserved,
		TaskGroupsValid:           view.TaskGroupsValid,
		TaskRecordObserved:        view.TaskRecordObserved,
		MilestoneReceiptsObserved: view.MilestoneReceiptsObserved,
	}

	itemIDs := make([]int32, 0, len(view.Bag))
	for itemID := range view.Bag {
		itemIDs = append(itemIDs, itemID)
	}
	sort.Slice(itemIDs, func(i, j int) bool { return itemIDs[i] < itemIDs[j] })
	for _, itemID := range itemIDs {
		out.Items = append(out.Items, activityItemProto(state.ItemCount{ItemID: itemID, Count: view.Bag[itemID]}))
	}

	modes := append([]state.DessertModeView(nil), view.Modes...)
	sort.SliceStable(modes, func(i, j int) bool { return modes[i].Mode < modes[j].Mode })
	for _, mode := range modes {
		modeView := &pb.DessertModeView{
			Mode:                mode.Mode,
			Multiplier:          mode.Multiplier,
			UnlockScore:         mode.UnlockScore,
			Unlocked:            mode.Mode == 1 || (view.TotalScoreObserved && view.TotalScore >= mode.UnlockScore),
			Observed:            mode.Observed,
			Valid:               mode.Valid,
			Step:                mode.Step,
			Score:               mode.Score,
			IsRunning:           mode.IsRunning,
			RawGameStatus:       mode.GameStatus,
			EffectiveGameStatus: dessertEffectiveGameStatus(mode.GameStatus),
			CurrentId:           mode.CurID,
			ObjectCount:         mode.ObjectCount,
		}
		levels := make([]int32, 0, len(mode.LevelCounts))
		for level := range mode.LevelCounts {
			levels = append(levels, level)
		}
		sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })
		for _, level := range levels {
			modeView.LevelCounts = append(modeView.LevelCounts, &pb.DessertLevelCountView{
				Level: level,
				Count: mode.LevelCounts[level],
			})
		}
		out.Modes = append(out.Modes, modeView)
	}

	tasks := append([]state.DessertTaskView(nil), view.Tasks...)
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].TaskIndex != tasks[j].TaskIndex {
			return tasks[i].TaskIndex < tasks[j].TaskIndex
		}
		if tasks[i].Position != tasks[j].Position {
			return tasks[i].Position < tasks[j].Position
		}
		return tasks[i].TaskID < tasks[j].TaskID
	})
	for _, task := range tasks {
		out.Tasks = append(out.Tasks, &pb.DessertTaskView{
			TaskIndex:        task.TaskIndex,
			Position:         task.Position,
			TaskId:           task.TaskID,
			TaskType:         task.TaskType,
			Param:            task.Param,
			HasParam:         task.HasParam,
			Title:            task.Title,
			Target:           task.Target,
			Progress:         clampProgress(task.Progress, task.Target),
			RawProgress:      task.Progress,
			ProgressObserved: task.ProgressObserved,
			Received:         task.Received,
			ReceiptObserved:  task.ReceiptObserved,
			CatalogKnown:     task.CatalogKnown,
			Reward:           activityItemsProto(task.Reward),
			Status:           dessertTaskStatus(view, task),
		})
	}

	milestones := append([]state.DessertMilestoneView(nil), view.Milestones...)
	sort.SliceStable(milestones, func(i, j int) bool { return milestones[i].Index < milestones[j].Index })
	for _, milestone := range milestones {
		ready := dessertMilestoneReady(view, milestone)
		out.Milestones = append(out.Milestones, &pb.DessertMilestoneView{
			Index:       milestone.Index,
			Target:      milestone.Target,
			Received:    milestone.Received,
			Reward:      activityItemsProto(milestone.Reward),
			Status:      dessertMilestoneStatus(view, milestone),
			Progress:    clampProgress(view.TotalScore, milestone.Target),
			RawProgress: view.TotalScore,
			Ready:       ready,
		})
	}

	celebrityReward, rewardValid := dessertCelebrityReward()
	out.Celebrity = &pb.DessertCelebrityLikeView{
		Observed:         view.Celebrity.Observed,
		Valid:            view.Celebrity.Valid,
		TypesObserved:    view.Celebrity.TypesObserved,
		RankingsObserved: view.Celebrity.RankingsObserved,
		LikesObserved:    view.Celebrity.LikesObserved,
		TypeListed:       view.Celebrity.TypeListed,
		RankingObserved:  view.Celebrity.RankingObserved,
		RankingCount:     view.Celebrity.RankingCount,
		LikedThisBatch:   view.Celebrity.LikedThisBatch,
		LastLikeTimeMs:   view.Celebrity.LastLikeTimeMs,
		CreateTimeMs:     view.Celebrity.CreateTimeMs,
		Reward:           activityItemsProto(celebrityReward),
		Status:           dessertCelebrityStatus(view, rewardValid),
	}
	return out
}

func dessertRuntimeProto(snapshot runner.DessertRuntimeSnapshot) *pb.DessertRuntimeView {
	return &pb.DessertRuntimeView{
		Observed:             snapshot.Observed,
		ShadowOnly:           snapshot.ShadowOnly,
		PolicyEnabled:        snapshot.PolicyEnabled,
		SessionEpoch:         snapshot.SessionEpoch,
		BatchId:              snapshot.BatchID,
		Mode:                 snapshot.Mode,
		AuthorityRevision:    snapshot.AuthorityRevision,
		BoardHash:            sanitizeDessertBoardHash(snapshot.BoardHash),
		BoardOwned:           snapshot.BoardOwned,
		TakeoverRequested:    snapshot.TakeoverRequested,
		Waiting:              snapshot.Waiting,
		WaitingRemainingMs:   snapshot.WaitingRemainingMS,
		FrozenWaitingLevel:   snapshot.FrozenWaitingLevel,
		SessionEnergyUsed:    snapshot.SessionEnergyUsed,
		Suggestion:           sanitizeDessertRuntimeText(snapshot.Suggestion, 160),
		BlockedReason:        sanitizeDessertRuntimeText(snapshot.BlockedReason, 240),
		FailureLocked:        snapshot.FailureLocked,
		LiveEvidenceReady:    snapshot.LiveEvidenceReady,
		LiveExecutionAllowed: snapshot.LiveExecutionAllowed,
		MaxSessionEnergy:     snapshot.MaxSessionEnergy,
		MinEnergyReserve:     snapshot.MinEnergyReserve,
	}
}

func sanitizeDessertBoardHash(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 64 {
		return ""
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return ""
		}
	}
	return value[:16]
}

func sanitizeDessertRuntimeText(value string, maxRunes int) string {
	value = strings.Join(strings.Fields(value), " ")
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}
	return string(runes)
}

func dessertEffectiveGameStatus(status int32) int32 {
	// The client treats a persisted stopped board as playable when it resumes.
	if status == 2 {
		return 1
	}
	return status
}

func dessertTaskStatus(view state.DessertView, task state.DessertTaskView) pb.PlanStatus {
	if !view.Valid || !task.CatalogKnown || task.TaskIndex != 0 || task.TaskType != 18 || task.Target <= 0 || !dessertTaskRewardValid(task.Reward) {
		return pb.PlanStatus_PLAN_STATUS_BLOCKED
	}
	if !task.ReceiptObserved {
		return pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
	}
	// A claimed task is removed from the progress map, so receipt must be
	// checked before requiring a still-present progress value.
	if task.Received {
		return pb.PlanStatus_PLAN_STATUS_SKIPPED
	}
	if !task.ProgressObserved || (view.Phase != 2 && view.Phase != 3) {
		return pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
	}
	if task.Progress >= task.Target {
		return pb.PlanStatus_PLAN_STATUS_READY
	}
	return pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
}

func dessertTaskRewardValid(reward []state.ItemCount) bool {
	config, ok := state.DessertCatalogConfig()
	return ok && len(reward) == 1 && reward[0].ItemID == config.EnergyItemID && reward[0].Count == 100
}

func dessertMilestoneReady(view state.DessertView, milestone state.DessertMilestoneView) bool {
	return view.Valid && (view.Phase == 2 || view.Phase == 3) && view.TotalScoreObserved &&
		view.MilestoneReceiptsObserved && milestone.Target > 0 && view.TotalScore >= milestone.Target && !milestone.Received
}

func dessertMilestoneStatus(view state.DessertView, milestone state.DessertMilestoneView) pb.PlanStatus {
	switch {
	case !view.Valid || milestone.Target <= 0:
		return pb.PlanStatus_PLAN_STATUS_BLOCKED
	case !view.TotalScoreObserved || !view.MilestoneReceiptsObserved:
		return pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
	case milestone.Received:
		return pb.PlanStatus_PLAN_STATUS_SKIPPED
	default:
		// Reward boxes remain monitoring-only until a sanitized successful
		// act.recvBoxes fixture confirms the exact request and response shape.
		return pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
	}
}

func dessertCelebrityReward() ([]state.ItemCount, bool) {
	config, ok := state.DessertCatalogConfig()
	if !ok || config.CelebrityReward.ItemID <= 0 || config.CelebrityReward.Count <= 0 {
		return nil, false
	}
	return []state.ItemCount{config.CelebrityReward}, true
}

func dessertCelebrityStatus(view state.DessertView, rewardValid bool) pb.PlanStatus {
	celebrity := view.Celebrity
	if !view.Valid || !rewardValid {
		return pb.PlanStatus_PLAN_STATUS_BLOCKED
	}
	if view.Phase != 2 || !celebrity.Observed || !celebrity.TypesObserved || !celebrity.RankingsObserved || !celebrity.LikesObserved {
		return pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
	}
	if !celebrity.Valid || !celebrity.TypeListed || !celebrity.RankingObserved || celebrity.RankingCount <= 0 {
		return pb.PlanStatus_PLAN_STATUS_BLOCKED
	}
	if celebrity.LikedThisBatch {
		return pb.PlanStatus_PLAN_STATUS_SKIPPED
	}
	return pb.PlanStatus_PLAN_STATUS_READY
}

func activityItemsProto(items []state.ItemCount) []*pb.ActivityItem {
	out := make([]*pb.ActivityItem, 0, len(items))
	for _, item := range items {
		out = append(out, activityItemProto(item))
	}
	return out
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
	return buildPendingTasksAtPolicy(st, now, true)
}

func buildPendingTasksAtPolicy(st *state.State, now time.Time, mapEventEnabled bool) []*pb.PendingTaskView {
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
		if order == nil || len(order.Requires) == 0 {
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
			Category:        "居民订单",
			Id:              strconv.FormatInt(int64(boxID), 10),
			Title:           fmt.Sprintf("居民订单 #%d", boxID),
			Status:          status,
			Requirements:    reqs,
			CooldownUntilMs: cooldownUntil,
			CooldownReason:  cooldownReason,
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
			Category:     "顾客订单",
			Id:           strconv.FormatInt(int64(npcID), 10),
			Title:        fmt.Sprintf("顾客订单 NPC=%d", npcID),
			Status:       requirementsStatus(reqs),
			Requirements: reqs,
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
			view.Requirements = []*pb.RequirementView{requirementView(flowerID, target, inventory[flowerID])}
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
			Category:     "主线剧情",
			Id:           strconv.FormatInt(int64(story.SectionID), 10),
			Title:        title,
			Status:       status,
			Requirements: reqs,
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
		out = append(out, &pb.PendingTaskView{
			Category: "日常任务",
			Id:       strconv.FormatInt(int64(id), 10),
			Title:    title,
			Finished: task.Finished,
			Target:   task.Target,
			Status:   status,
		})
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
			Category: "周常任务",
			Id:       strconv.FormatInt(int64(id), 10),
			Title:    title,
			Finished: task.Finished,
			Target:   task.Target,
			Status:   status,
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
			Category: "成就任务",
			Id:       strconv.FormatInt(int64(id), 10),
			Title:    title,
			Finished: task.Finished,
			Target:   task.Target,
			Status:   status,
		})
	}

	observed, mapValid, mapError := st.RandomEventMapStatus()
	disabledSuffix := ""
	if !mapEventEnabled {
		disabledSuffix = "（地图随机事件自动处理已关闭）"
	}
	if !observed {
		status := pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
		if !mapEventEnabled {
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
			} else if !mapEventEnabled {
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
		out = append(out, &pb.PendingTaskView{
			Category: "宠物事件",
			Id:       fmt.Sprintf("%d:%d", evt.PetID, evt.TableID),
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
	out = append(out, dessertPendingTasks(st, now)...)

	return out
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

func dessertPendingTasks(st *state.State, now time.Time) []*pb.PendingTaskView {
	view, _ := st.DessertView(now)
	return dessertPendingTasksFromView(view)
}

func dessertPendingTasksFromView(view state.DessertView) []*pb.PendingTaskView {
	if !view.Found || (view.Phase != 2 && view.Phase != 3) {
		return nil
	}
	tasks := append([]state.DessertTaskView(nil), view.Tasks...)
	sort.SliceStable(tasks, func(i, j int) bool {
		if tasks[i].TaskIndex != tasks[j].TaskIndex {
			return tasks[i].TaskIndex < tasks[j].TaskIndex
		}
		if tasks[i].Position != tasks[j].Position {
			return tasks[i].Position < tasks[j].Position
		}
		return tasks[i].TaskID < tasks[j].TaskID
	})
	out := make([]*pb.PendingTaskView, 0, len(tasks))
	for _, task := range tasks {
		if task.Received {
			continue
		}
		title := task.Title
		if title == "" {
			title = fmt.Sprintf("香卉甜糕任务 #%d", task.TaskID)
		}
		out = append(out, &pb.PendingTaskView{
			Category: "activity",
			Id:       fmt.Sprintf("%d:%d:%d", view.BatchID, task.TaskIndex, task.TaskID),
			Title:    title,
			Finished: clampProgress(task.Progress, task.Target),
			Target:   task.Target,
			Status:   dessertTaskStatus(view, task),
		})
	}
	return out
}
