package apiserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (svc *Services) GetStatus(ctx context.Context, req *connect.Request[pb.GetStatusRequest]) (*connect.Response[pb.GetStatusResponse], error) {
	resp := &pb.GetStatusResponse{}
	if req.Msg.GetIncludeFeatureCapabilities() {
		resp.FeatureCapabilities = featureCapabilitiesProto()
	}
	if req.Msg.GetAccountId() != "" || req.Msg.GetAccountName() != "" {
		acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
		if err != nil {
			return nil, mapErr(err)
		}
		status, err := svc.statusFor(ctx, acc)
		if err != nil {
			return nil, mapErr(err)
		}
		resp.Accounts = append(resp.Accounts, status)
		return connect.NewResponse(resp), nil
	}
	var userID int64
	if !auth.IsAdmin(ctx) {
		userID = auth.UserIDFromContext(ctx)
	}
	accs, err := svc.DB.ListAccounts(ctx, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	for _, a := range accs {
		status, err := svc.statusFor(ctx, a)
		if err != nil {
			return nil, mapErr(err)
		}
		resp.Accounts = append(resp.Accounts, status)
	}
	return connect.NewResponse(resp), nil
}

func (svc *Services) statusFor(ctx context.Context, acc *store.Account) (*pb.AccountStatus, error) {
	out := &pb.AccountStatus{
		AccountId:   fmt.Sprintf("%d", acc.ID),
		AccountName: acc.Name,
		GsIdx:       acc.GsIdx,
	}
	r := svc.Manager.Get(acc.ID)
	if r == nil {
		policy, err := svc.policyFor(ctx, acc.ID)
		if err != nil {
			return nil, err
		}
		out.AutomationEnabled = policy.GetAutomationEnabled()
		diag, _ := svc.Manager.LastDiagnostics(acc.ID)
		applyStoppedRunnerDiagnostics(out, policy, diag)
		if stats, ok := svc.Manager.RuntimeStats(acc.ID); ok {
			out.RuntimeStatistics = runtimeStatisticsProto(stats)
		}
		return out, nil
	}
	out.Connected = r.Connected()
	now := time.Now()
	if last := r.LastEventAt(); !last.IsZero() {
		out.LastEventAt = timestamppb.New(last)
	}
	out.AutomationEnabled = r.Policy().GetAutomationEnabled()
	diag := r.Diagnostics(now)
	out.Diagnostics = runnerDiagnosticsProto(diag)
	out.DomainStatuses = buildDomainStatuses(r.Policy(), diag, out.Connected)
	out.Health = accountHealth(out.Connected, diag)
	out.LastError = diag.LastOperationError
	out.RuntimeStatistics = runtimeStatisticsProto(r.RuntimeStats())
	st := r.State()
	vip, vipExp := st.Vip()
	out.Level = st.Level()
	out.Experience = st.Experience()
	expToNext, nextLevelExp, levelMaxed := st.ExperienceToNextLevel()
	out.ExperienceToNextLevel = expToNext
	out.NextLevelExperience = nextLevelExp
	out.LevelMaxed = levelMaxed
	out.Vip = vip
	out.VipExp = vipExp
	out.NobleEligible = st.NobleEligible()
	if rep, ok := st.Reputation(); ok {
		out.ReputationObserved = true
		out.ReputationScore = rep.Score
		out.ReputationLastSyncTimeMs = rep.LastSyncTimeMs
		out.ReputationLastViewTimeMs = rep.LastViewTimeMs
	}
	lands := st.Lands()
	out.KnownLands = int32(len(lands))
	harvestDelay := time.Duration(r.Policy().GetPlant().GetPlanting().GetHarvestDelaySeconds()) * time.Second
	byKind := map[string]int32{}
	for _, l := range lands {
		kind, _ := automation.Recommend(l, now, harvestDelay)
		byKind[kind]++
	}
	out.ByKind = byKind
	for _, count := range st.FlowerInventory() {
		out.FlowerStockTotal += count
	}
	return out, nil
}

func (svc *Services) GetSnapshot(ctx context.Context, req *connect.Request[pb.GetSnapshotRequest]) (*connect.Response[pb.GetSnapshotResponse], error) {
	acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
	if err != nil {
		return nil, mapErr(err)
	}
	r := svc.Manager.Get(acc.ID)
	if r == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("runner not started"))
	}
	st := r.State()
	now := time.Now()
	st.RefreshWaterDrops(now)
	lands := st.Lands()
	waterDrops, waterDropsTotal, waterDropsNextMs := st.WaterDrops()
	diamondsFree, diamondsPaid := st.Diamonds()
	vip, vipExp := st.Vip()
	expToNext, nextLevelExp, levelMaxed := st.ExperienceToNextLevel()
	diag := r.Diagnostics(now)
	cyclicNote, _ := st.CyclicNoteView(now)
	cyclicStory, _ := st.CyclicStoryView(now)
	fmlRace := st.FmlRace()
	dessert, _ := st.DessertView(now)
	dessertRuntime := r.DessertRuntimeSnapshot()
	policy := r.Policy()
	resp := &pb.GetSnapshotResponse{
		AccountId:             fmt.Sprintf("%d", acc.ID),
		AccountName:           acc.Name,
		Inventory:             st.Inventory(),
		RoleId:                st.RoleID(),
		CapturedAt:            timestamppb.Now(),
		Gold:                  st.Gold(),
		WaterDrops:            waterDrops,
		WaterDropsTotal:       waterDropsTotal,
		WaterDropsNextMs:      waterDropsNextMs,
		Level:                 st.Level(),
		Experience:            st.Experience(),
		ExperienceToNextLevel: expToNext,
		NextLevelExperience:   nextLevelExp,
		LevelMaxed:            levelMaxed,
		DiamondsFree:          diamondsFree,
		DiamondsPaid:          diamondsPaid,
		PendingTasks:          buildPendingTasksAtPolicy(st, now, policy.GetBasic().GetMapEventEnabled()),
		Vip:                   vip,
		VipExp:                vipExp,
		NobleEligible:         st.NobleEligible(),
		ObservedNamespaces:    diag.ObservedNamespaces,
		UnknownRpcCount:       diag.UnknownRPCCount,
		UnknownNamespaceCount: diag.UnknownNamespaceCount,
		Diagnostics:           runnerDiagnosticsProto(diag),
		RuntimeStatistics:     runtimeStatisticsProto(r.RuntimeStats()),
		CyclicNote:            cyclicNoteProto(cyclicNote),
		CyclicStory:           cyclicStoryProto(cyclicStory),
		FmlRace: fmlRaceProto(
			fmlRace, st, policy.GetUnion().GetRace(), st.RoleID(), now,
			automation.RaceModuleGates{
				Customer:  policy.GetOrder().GetCustomer().GetEnabled(),
				Pearl:     policy.GetBasic().GetPearl().GetAutoHireEnabled(),
				Cultivate: policy.GetPlant().GetCultivate().GetEnabled(),
			},
		),
		Dessert: dessertProto(dessert),
	}
	resp.Dessert.Runtime = dessertRuntimeProto(dessertRuntime)
	if rep, ok := st.Reputation(); ok {
		resp.ReputationObserved = true
		resp.ReputationScore = rep.Score
		resp.ReputationLastSyncTimeMs = rep.LastSyncTimeMs
		resp.ReputationLastViewTimeMs = rep.LastViewTimeMs
	}
	resp.PlantableFlowers = plantableFlowersProto(st.PlantableFlowers(nil, nil))
	resp.SellableFlowerArts = sellableFlowerArtsProto(st)
	resp.FriendTouchFriends = friendTouchFriendsProto(st.FriendTouchFriends(now))
	resp.FriendTouchFriendsObserved = st.FriendTouch(now).FriendsObserved
	resp.Lands = buildLandViews(lands, st.FarmLands(), st.LandRosterObserved(), st.FarmLandConfigObserved(), st.Level(), now, time.Duration(policy.GetPlant().GetPlanting().GetHarvestDelaySeconds())*time.Second)
	resp.FmlLandsObserved = st.FmlLandObserved()
	resp.FmlLands = buildFmlLandViews(st.FmlLands(), st.Cultivations(), now)
	plan := automation.BuildPlan(st, policy, now)
	resp.DomainStatuses = buildDomainStatuses(policy, diag, r.Connected())
	resp.PlannedOperations = plannedOperationsProto(plan.Operations, diag)
	resp.Demands = demandsProto(plan.Demands)
	resp.Vases = vasesProto(st.Vases())
	resp.FlowerArtAvailability = flowerArtAvailabilityProto(st, plan)
	resp.OrderStatistics = orderStatisticsProto(st, now)
	resp.BusinessStatistics = businessStatisticsProto(st)
	resp.InventoryLedger = inventoryLedgerProto(st.Inventory(), plan.Ledger)
	resp.BlockingSummary = blockingSummaryProto(resp.DomainStatuses, plan)
	return connect.NewResponse(resp), nil
}

func runnerDiagnosticsProto(d runner.Diagnostics) *pb.RunnerDiagnostics {
	return &pb.RunnerDiagnostics{
		CurrentOperation:          d.CurrentOperation,
		CurrentOperationStartedAt: timestampOrNil(d.CurrentOperationStartedAt),
		LastOperation:             d.LastOperation,
		LastOperationAt:           timestampOrNil(d.LastOperationAt),
		LastOperationError:        d.LastOperationError,
		LastOperationErrorAt:      timestampOrNil(d.LastOperationErrorAt),
		NextDecisionAt:            timestampOrNil(d.NextDecisionAt),
		SessionInvalidatedReason:  d.SessionInvalidatedReason,
		BlockedReasons:            append([]string(nil), d.BlockedReasons...),
		UnknownRpcCount:           d.UnknownRPCCount,
		UnknownNamespaceCount:     d.UnknownNamespaceCount,
		ObservedNamespaces:        append([]string(nil), d.ObservedNamespaces...),
	}
}

func runtimeStatisticsProto(stats runner.RuntimeStatsSnapshot) *pb.RuntimeStatisticsView {
	if stats.StartedAt.IsZero() {
		return nil
	}
	out := &pb.RuntimeStatisticsView{
		StartedAt:       timestampOrNil(stats.StartedAt),
		StoppedAt:       timestampOrNil(stats.StoppedAt),
		UpdatedAt:       timestampOrNil(stats.UpdatedAt),
		Running:         stats.Running,
		TotalOperations: stats.TotalOperations,
	}
	for _, item := range stats.ResourceGains {
		out.ResourceGains = append(out.ResourceGains, &pb.RuntimeResourceTotal{
			Key:    item.Key,
			Label:  item.Label,
			ItemId: item.ItemID,
			Gained: item.Gained,
		})
	}
	for _, item := range stats.OrderCompletions {
		out.OrderCompletions = append(out.OrderCompletions, runtimeActionTotalProto(item))
	}
	for _, item := range stats.TaskCompletions {
		out.TaskCompletions = append(out.TaskCompletions, runtimeActionTotalProto(item))
	}
	for _, item := range stats.OperationCompletions {
		out.OperationCompletions = append(out.OperationCompletions, runtimeActionTotalProto(item))
	}
	return out
}

func runtimeActionTotalProto(item runner.RuntimeActionTotal) *pb.RuntimeActionTotal {
	return &pb.RuntimeActionTotal{
		Key:   item.Key,
		Label: item.Label,
		Count: item.Count,
	}
}

func cyclicNoteProto(view state.CyclicNoteView) *pb.CyclicNoteView {
	out := &pb.CyclicNoteView{
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
		Score:                     view.Score,
		CurrencyItemId:            view.CurrencyItemID,
		CurrencyBalance:           view.CurrencyBalance,
		FinishCount:               view.FinishCount,
		LastRefreshMs:             view.LastRefreshTimeMs,
		TaskListObserved:          view.TaskListObserved,
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

	for _, task := range view.Tasks {
		out.Tasks = append(out.Tasks, &pb.CyclicNoteTaskSlot{
			SlotId:           task.SlotID,
			Unlocked:         task.Unlocked,
			TaskId:           task.TaskID,
			TaskType:         task.TaskType,
			Param:            task.Param,
			Title:            task.Title,
			Target:           task.Target,
			Progress:         clampProgress(task.Progress, task.Target),
			RawProgress:      task.Progress,
			ProgressObserved: task.ProgressObserved,
			Received:         task.Received,
			ReceiptObserved:  task.ReceiptObserved,
			CatalogKnown:     task.CatalogKnown,
			Reward:           activityItemsProto(task.Reward),
			FinishCost:       activityItemsProto(task.FinishCost),
			Status:           cyclicNoteTaskStatus(view, task),
		})
	}

	for _, milestone := range view.Milestones {
		milestoneClaimPhase := view.Phase == 2 || view.Phase == 3
		ready := view.Valid && milestoneClaimPhase && view.MilestoneReceiptsObserved && milestone.Target > 0 && view.Score >= milestone.Target && !milestone.Received
		status := pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
		switch {
		case !view.Valid || milestone.Target <= 0:
			status = pb.PlanStatus_PLAN_STATUS_BLOCKED
		case !view.MilestoneReceiptsObserved:
			status = pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
		case milestone.Received:
			status = pb.PlanStatus_PLAN_STATUS_SKIPPED
		case ready:
			status = pb.PlanStatus_PLAN_STATUS_READY
		}
		out.Milestones = append(out.Milestones, &pb.CyclicNoteMilestone{
			Index:       milestone.Index,
			Target:      milestone.Target,
			Received:    milestone.Received,
			Reward:      activityItemsProto(milestone.Reward),
			Status:      status,
			Progress:    clampProgress(view.Score, milestone.Target),
			RawProgress: view.Score,
			Ready:       ready,
		})
	}
	return out
}

func cyclicStoryProto(view state.CyclicStoryView) *pb.CyclicStoryView {
	out := &pb.CyclicStoryView{
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
		Score:                     view.Score,
		CurrencyItemId:            view.CurrencyItemID,
		CurrencyBalance:           view.CurrencyBalance,
		FinishCount:               view.FinishCount,
		ExpOrderNum:               view.ExpOrderNum,
		LastRefreshMs:             view.LastRefreshTimeMs,
		OrdersObserved:            view.OrdersObserved,
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

	for _, order := range view.Orders {
		out.Orders = append(out.Orders, &pb.CyclicStoryOrder{
			OrderIdx:     order.OrderIdx,
			OrderId:      order.OrderID,
			FlowerId:     order.FlowerID,
			OrderTime:    order.OrderTime,
			ValidTime:    order.ValidTime,
			Cost:         order.Cost,
			CatalogKnown: order.CatalogKnown,
			OnCooldown:   order.OnCooldown,
			Reward:       activityItemsProto(order.Reward),
			Status:       cyclicStoryOrderStatus(view, order),
		})
	}

	for _, milestone := range view.Milestones {
		milestoneClaimPhase := view.Phase == 2 || view.Phase == 3
		ready := view.Valid && milestoneClaimPhase && view.MilestoneReceiptsObserved && milestone.Target > 0 && view.Score >= milestone.Target && !milestone.Received
		status := pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
		switch {
		case !view.Valid || milestone.Target <= 0:
			status = pb.PlanStatus_PLAN_STATUS_BLOCKED
		case !view.MilestoneReceiptsObserved:
			status = pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
		case milestone.Received:
			status = pb.PlanStatus_PLAN_STATUS_SKIPPED
		case ready:
			status = pb.PlanStatus_PLAN_STATUS_READY
		}
		out.Milestones = append(out.Milestones, &pb.CyclicNoteMilestone{
			Index:       milestone.Index,
			Target:      milestone.Target,
			Received:    milestone.Received,
			Reward:      activityItemsProto(milestone.Reward),
			Status:      status,
			Progress:    clampProgress(view.Score, milestone.Target),
			RawProgress: view.Score,
			Ready:       ready,
		})
	}
	return out
}

func cyclicStoryOrderStatus(view state.CyclicStoryView, order state.CyclicStoryOrderView) pb.PlanStatus {
	if order.OnCooldown || order.OrderID <= 0 {
		return pb.PlanStatus_PLAN_STATUS_SKIPPED
	}
	if !view.Valid || !order.CatalogKnown || order.FlowerID <= 0 || order.Cost <= 0 {
		return pb.PlanStatus_PLAN_STATUS_BLOCKED
	}
	if !view.OrdersObserved {
		return pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
	}
	if view.Phase != 2 {
		return pb.PlanStatus_PLAN_STATUS_SKIPPED
	}
	return pb.PlanStatus_PLAN_STATUS_READY
}

func cyclicNoteTaskStatus(view state.CyclicNoteView, task state.CyclicNoteTaskSlotView) pb.PlanStatus {
	if !task.Unlocked {
		return pb.PlanStatus_PLAN_STATUS_SKIPPED
	}
	if !view.Valid || !task.CatalogKnown || task.Target <= 0 {
		return pb.PlanStatus_PLAN_STATUS_BLOCKED
	}
	if !task.ProgressObserved || !task.ReceiptObserved {
		return pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
	}
	if task.Received {
		return pb.PlanStatus_PLAN_STATUS_SKIPPED
	}
	if view.Phase != 2 {
		return pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
	}
	if task.Progress >= task.Target {
		return pb.PlanStatus_PLAN_STATUS_READY
	}
	return pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
}

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

func flowerRequirements(reqs []state.FlowerRequire, inventory map[int32]int32) []*pb.RequirementView {
	out := make([]*pb.RequirementView, 0, len(reqs))
	for _, req := range reqs {
		if req.FlowerID == 0 || req.Count <= 0 {
			continue
		}
		out = append(out, requirementView(req.FlowerID, req.Count, inventory[req.FlowerID]))
	}
	return out
}

func itemRequirements(reqs []state.ItemCount, inventory map[int32]int32) []*pb.RequirementView {
	out := make([]*pb.RequirementView, 0, len(reqs))
	for _, req := range reqs {
		if req.ItemID == 0 || req.Count <= 0 {
			continue
		}
		out = append(out, requirementView(req.ItemID, req.Count, inventory[req.ItemID]))
	}
	return out
}

func customerOrderRequirements(order *state.CustomerOrder, inventory map[int32]int32) []*pb.RequirementView {
	out := flowerRequirements(order.Requires, inventory)
	for _, req := range order.ItemRequires {
		if req.ItemID == 0 || req.Count <= 0 {
			continue
		}
		missingArt := req.Count - inventory[req.ItemID]
		recipe, ok := state.FlowerArtRecipeByID(req.ItemID)
		if !ok {
			out = append(out, requirementView(req.ItemID, req.Count, inventory[req.ItemID]))
			continue
		}
		if missingArt <= 0 {
			out = append(out, requirementView(req.ItemID, req.Count, inventory[req.ItemID]))
			continue
		}
		for _, flowerID := range recipe.Flowers {
			out = append(out, requirementView(flowerID, missingArt, inventory[flowerID]))
		}
	}
	return out
}

func requirementView(itemID, required, owned int32) *pb.RequirementView {
	missing := required - owned
	if missing < 0 {
		missing = 0
	}
	name := state.ItemName(itemID)
	if name == "" {
		name = fmt.Sprintf("#%d", itemID)
	}
	return &pb.RequirementView{
		ItemId:           itemID,
		ItemName:         name,
		Required:         required,
		Owned:            owned,
		Missing:          missing,
		PlantingRelevant: state.IsFlowerItemID(itemID),
	}
}

func requirementsStatus(reqs []*pb.RequirementView) pb.PlanStatus {
	for _, req := range reqs {
		if req.GetMissing() > 0 {
			return pb.PlanStatus_PLAN_STATUS_MANAGED
		}
	}
	return pb.PlanStatus_PLAN_STATUS_READY
}

func plannedOperationsProto(ops []automation.PlannedOp, diag runner.Diagnostics) []*pb.PlannedOperation {
	cooldowns := cooldownsByOperation(diag)
	out := make([]*pb.PlannedOperation, 0, len(ops))
	for _, op := range ops {
		cooldownUntil := op.CooldownUntil
		cooldownReason := op.CooldownReason
		if cd, ok := lookupPlannedOperationCooldown(cooldowns, op); ok {
			cooldownUntil = cd.Until
			cooldownReason = cd.Reason
		}
		out = append(out, &pb.PlannedOperation{
			Category:        op.Category,
			Domain:          op.Domain,
			Action:          op.Action,
			Rpc:             op.Kind,
			Lane:            executionLaneProto(op.Lane),
			Reason:          op.Reason,
			LandIds:         append([]int32(nil), op.LandIDs...),
			FlowerId:        op.FlowerID,
			Priority:        op.Priority,
			GoldCost:        op.GoldCost,
			DiamondCost:     op.DiamondCost,
			ItemCost:        cloneInt32Map(op.ItemCost),
			FeatureId:       op.FeatureID,
			Label:           op.Label,
			Status:          planStatusProto(op.Status),
			Executable:      op.Executable,
			SyncOnly:        op.SyncOnly,
			BlockedReasons:  append([]string(nil), op.BlockedReasons...),
			OperationId:     op.OperationID,
			GoalId:          op.GoalID,
			DemandId:        op.DemandID,
			TargetId:        op.TargetID,
			ItemId:          op.ItemID,
			Count:           op.Count,
			VaseId:          op.VaseID,
			FlowerIds:       append([]int32(nil), op.FlowerIDs...),
			CostGates:       costGatesProto(op.CostGates),
			BlockingStage:   op.BlockingStage,
			CooldownUntilMs: timeToUnixMilli(cooldownUntil),
			CooldownReason:  cooldownReason,
			TargetUid:       op.TargetUID,
			TargetUids:      append([]int64(nil), op.TargetUIDs...),
			BatchId:         op.BatchID,
			SlotId:          op.SlotID,
			TaskId:          op.TaskID,
			MilestoneIndex:  op.MilestoneIndex,
		})
	}
	return out
}

func cooldownsByOperation(diag runner.Diagnostics) map[string]runner.OperationCooldownSnapshot {
	out := make(map[string]runner.OperationCooldownSnapshot, len(diag.OperationCooldowns))
	for _, cd := range diag.OperationCooldowns {
		if cd.OperationID == "" {
			continue
		}
		out[cd.OperationID] = cd
	}
	return out
}

func lookupPlannedOperationCooldown(cooldowns map[string]runner.OperationCooldownSnapshot, op automation.PlannedOp) (runner.OperationCooldownSnapshot, bool) {
	if key := strings.TrimSpace(op.CooldownKey); key != "" {
		if cd, ok := cooldowns[key]; ok {
			return cd, true
		}
	}
	if op.OperationID != "" {
		if cd, ok := cooldowns[op.OperationID]; ok {
			return cd, true
		}
	}
	return runner.OperationCooldownSnapshot{}, false
}

func executionLaneProto(lane string) pb.ExecutionLane {
	switch lane {
	case automation.LaneFarm:
		return pb.ExecutionLane_EXECUTION_LANE_FARM
	case automation.LaneSide:
		return pb.ExecutionLane_EXECUTION_LANE_SIDE
	default:
		return pb.ExecutionLane_EXECUTION_LANE_UNSPECIFIED
	}
}

func timeToUnixMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func demandsProto(demands []automation.Demand) []*pb.DemandView {
	out := make([]*pb.DemandView, 0, len(demands))
	for _, demand := range demands {
		out = append(out, &pb.DemandView{
			Id:             demand.ID,
			GoalId:         demand.GoalID,
			Category:       demand.Category,
			Domain:         demand.Domain,
			EntityId:       demand.EntityID,
			Label:          demand.Label,
			ItemId:         demand.ItemID,
			ItemName:       itemNameOrID(demand.ItemID),
			Required:       demand.Count,
			Owned:          demand.Have,
			Allocated:      demand.Allocated,
			Available:      demand.Available,
			Missing:        demand.Missing,
			Priority:       demand.Priority,
			Kind:           demand.Kind,
			Source:         demand.Source,
			BlockedReasons: append([]string(nil), demand.BlockedReasons...),
			Status:         planStatusProto(demand.Status),
			BlockingStage:  demand.BlockingStage,
			CostGates:      costGatesProto(demand.CostGates),
		})
	}
	return out
}

func costGatesProto(gates []automation.CostGate) []*pb.CostGate {
	if len(gates) == 0 {
		return nil
	}
	out := make([]*pb.CostGate, 0, len(gates))
	for _, gate := range gates {
		out = append(out, &pb.CostGate{
			Id:             gate.ID,
			ResourceKind:   gateResourceKindProto(gate.ResourceKind),
			Label:          gate.Label,
			ItemId:         gate.ItemID,
			Required:       gate.Required,
			Available:      gate.Available,
			Status:         planStatusProto(gate.Status),
			BlockedReasons: append([]string(nil), gate.BlockedReasons...),
			Hard:           gate.Hard,
			Source:         gate.Source,
		})
	}
	return out
}

func planStatusProto(status string) pb.PlanStatus {
	switch status {
	case automation.PlanStatusReady:
		return pb.PlanStatus_PLAN_STATUS_READY
	case automation.PlanStatusManaged:
		return pb.PlanStatus_PLAN_STATUS_MANAGED
	case automation.PlanStatusSyncOnly:
		return pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
	case automation.PlanStatusAdapterMissing:
		return pb.PlanStatus_PLAN_STATUS_ADAPTER_MISSING
	case automation.PlanStatusBlocked:
		return pb.PlanStatus_PLAN_STATUS_BLOCKED
	case automation.PlanStatusSkipped:
		return pb.PlanStatus_PLAN_STATUS_SKIPPED
	default:
		return pb.PlanStatus_PLAN_STATUS_UNSPECIFIED
	}
}

func gateResourceKindProto(kind string) pb.GateResourceKind {
	switch kind {
	case automation.GateResourceGold:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_GOLD
	case automation.GateResourceDiamond:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_DIAMOND
	case automation.GateResourceItem:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_ITEM
	case automation.GateResourceActivityItem:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_ACTIVITY_ITEM
	case automation.GateResourceWaterDrop:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_WATER_DROP
	case automation.GateResourceLevel:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_LEVEL
	case automation.GateResourceVase:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_VASE
	case automation.GateResourcePolicy:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_POLICY
	case automation.GateResourceState:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_STATE
	case automation.GateResourceAdapter:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_ADAPTER
	default:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_UNSPECIFIED
	}
}

func vasesProto(vases map[int32]state.VaseView) []*pb.VaseView {
	ids := make([]int32, 0, len(vases))
	for id := range vases {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]*pb.VaseView, 0, len(ids))
	for _, id := range ids {
		vase := vases[id]
		out = append(out, &pb.VaseView{VaseId: vase.VaseID, UTimeMs: vase.UTimeMs, CTimeMs: vase.CTimeMs})
	}
	return out
}

func flowerArtAvailabilityProto(st *state.State, plan automation.PlanResult) []*pb.FlowerArtAvailabilityView {
	countByArt := map[int32]int32{}
	for _, demand := range plan.Demands {
		if demand.Kind == automation.DemandKindFlowerArt && demand.ItemID > 0 {
			count := demand.Missing
			if count <= 0 {
				count = demand.Count
			}
			if count > countByArt[demand.ItemID] {
				countByArt[demand.ItemID] = count
			}
		}
	}
	for _, op := range plan.Operations {
		if op.ItemID > 0 {
			if _, ok := state.FlowerArtRecipeByID(op.ItemID); ok {
				count := op.Count
				if count <= 0 {
					count = 1
				}
				if count > countByArt[op.ItemID] {
					countByArt[op.ItemID] = count
				}
			}
		}
	}
	ids := make([]int32, 0, len(countByArt))
	for id := range countByArt {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]*pb.FlowerArtAvailabilityView, 0, len(ids))
	for _, id := range ids {
		count := countByArt[id]
		if count <= 0 {
			count = 1
		}
		availability := automation.FlowerArtAvailability(st, id, count, plan.Ledger)
		if availability.Recipe.ArtID == 0 {
			continue
		}
		view := &pb.FlowerArtAvailabilityView{
			ArtId:          availability.Recipe.ArtID,
			ArtName:        itemNameOrID(availability.Recipe.ArtID),
			VaseId:         availability.Recipe.VaseID,
			Level:          availability.Recipe.Level,
			SaleValue:      availability.Recipe.SaleValue,
			LevelOk:        true,
			VaseUnlocked:   availability.VaseUnlocked,
			Craftable:      availability.Craftable,
			BlockedReasons: append([]string(nil), availability.BlockedReasons...),
		}
		for _, req := range availability.Requirements {
			view.Requirements = append(view.Requirements, requirementView(req.ItemID, req.Count, req.Have))
		}
		out = append(out, view)
	}
	return out
}

func orderStatisticsProto(st *state.State, now time.Time) *pb.OrderStatisticsView {
	stats := st.Statistics()
	out := &pb.OrderStatisticsView{
		Observed:                 stats.Observed,
		DayId:                    stats.DayID,
		ResidentNormalFinished:   st.ResidentOrderFinishNum(now),
		PalaceFinished:           stats.OrderPalaceFinishNum,
		CustomerFinished:         st.CustomerOrderFinishNum(now),
		ResidentSatinFinished:    st.ResidentSatinOrderFinishNum(now),
		ResidentDecorateFinished: st.ResidentDecorateOrderFinishNum(now),
		FlowerArtSold:            stats.FlowerArtSellNum,
		UpdatedAtMs:              stats.UTimeMs,
		CreatedAtMs:              stats.CTimeMs,
	}
	if !stats.Observed {
		out.BlockedReasons = []string{"未观察到订单统计 namespace 124"}
	}
	return out
}

func businessStatisticsProto(st *state.State) *pb.BusinessStatisticsView {
	days := st.StatisticsDays()
	out := &pb.BusinessStatisticsView{Observed: st.Statistics().Observed}
	if len(days) == 0 {
		return out
	}
	out.Days = make([]*pb.DailyBusinessStatisticsView, 0, len(days))
	for _, day := range days {
		out.Days = append(out.Days, dailyBusinessStatisticsProto(day))
	}
	out.Today = out.Days[0]
	return out
}

func dailyBusinessStatisticsProto(stats state.StatisticsView) *pb.DailyBusinessStatisticsView {
	return &pb.DailyBusinessStatisticsView{
		DayId:                    stats.DayID,
		Gold:                     stats.Gold,
		Experience:               stats.Experience,
		Diamonds:                 stats.Diamonds,
		SpeedUpCard:              stats.SpeedUpCard,
		FlowerShopCoin:           stats.FlowerShopCoin,
		FlowerHarvestNum:         stats.FlowerHarvestNum,
		FlowerArtSold:            stats.FlowerArtSellNum,
		ResidentNormalFinished:   stats.OrderFlowerFinishNum,
		PalaceFinished:           stats.OrderPalaceFinishNum,
		CustomerFinished:         stats.OrderCustomerFinishNum,
		ResidentSatinFinished:    stats.OrderSatinFinishNum,
		Satin:                    stats.Satin,
		ResidentDecorateFinished: stats.OrderDecorateFinishNum,
		Wood:                     stats.Wood,
		UpdatedAtMs:              stats.UTimeMs,
		CreatedAtMs:              stats.CTimeMs,
	}
}

func inventoryLedgerProto(inventory map[int32]int32, ledger *automation.InventoryLedger) *pb.InventoryLedgerView {
	ids := make([]int32, 0, len(inventory))
	seen := map[int32]struct{}{}
	for itemID, count := range inventory {
		if count <= 0 {
			continue
		}
		ids = append(ids, itemID)
		seen[itemID] = struct{}{}
	}
	for itemID := range ledger.AllocatedItems() {
		if _, ok := seen[itemID]; ok {
			continue
		}
		ids = append(ids, itemID)
		seen[itemID] = struct{}{}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := &pb.InventoryLedgerView{Items: make([]*pb.InventoryLedgerItem, 0, len(ids))}
	for _, itemID := range ids {
		owned := inventory[itemID]
		allocated := int32(0)
		available := owned
		if ledger != nil {
			owned = ledger.Owned(itemID)
			allocated = ledger.Allocated(itemID)
			available = ledger.Available(itemID)
		}
		out.Items = append(out.Items, &pb.InventoryLedgerItem{
			ItemId:    itemID,
			ItemName:  itemNameOrID(itemID),
			Owned:     owned,
			Allocated: allocated,
			Available: available,
		})
	}
	return out
}

func plantableFlowersProto(flowers []state.PlantableFlower) []*pb.PlantableFlowerView {
	if len(flowers) == 0 {
		return nil
	}
	out := make([]*pb.PlantableFlowerView, 0, len(flowers))
	for _, flower := range flowers {
		var cdSeconds int32
		if cd, ok := state.FlowerLvlCDSeconds(flower.FlowerID, flower.Lvl); ok {
			cdSeconds = cd
		}
		out = append(out, &pb.PlantableFlowerView{
			FlowerId:   flower.FlowerID,
			FlowerName: itemNameOrID(flower.FlowerID),
			Stock:      flower.Stock,
			Gold:       flower.Gold,
			Experience: flower.Experience,
			Lvl:        flower.Lvl,
			CdSeconds:  cdSeconds,
		})
	}
	return out
}

func sellableFlowerArtsProto(st *state.State) []*pb.SellableFlowerArtView {
	if st == nil {
		return nil
	}
	inventory := st.Inventory()
	out := make([]*pb.SellableFlowerArtView, 0)
	for _, recipe := range state.AllFlowerArtRecipes() {
		if st.VaseObserved() && !st.HasVase(recipe.VaseID) {
			continue
		}
		stock := inventory[recipe.ArtID]
		out = append(out, &pb.SellableFlowerArtView{
			ArtId:     recipe.ArtID,
			ArtName:   itemNameOrID(recipe.ArtID),
			VaseId:    recipe.VaseID,
			VaseName:  itemNameOrID(recipe.VaseID),
			Stock:     stock,
			SaleValue: recipe.SaleValue,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func friendTouchFriendsProto(friends []state.FriendTouchFriendView) []*pb.FriendTouchFriendView {
	if len(friends) == 0 {
		return nil
	}
	out := make([]*pb.FriendTouchFriendView, 0, len(friends))
	for _, friend := range friends {
		out = append(out, &pb.FriendTouchFriendView{
			Uid:                  friend.UID,
			Name:                 friend.Name,
			StolenCount:          friend.StolenCount,
			StealMax:             friend.StealMax,
			StealLeft:            friend.StealLeft,
			CanSteal:             friend.CanSteal,
			ProfileObserved:      friend.ProfileObserved,
			BaseStealMax:         friend.BaseStealMax,
			BoughtCount:          friend.BoughtCount,
			QuotaObserved:        friend.QuotaObserved,
			AvailabilityObserved: friend.AvailabilityObserved,
		})
	}
	return out
}

func blockingSummaryProto(domainStatuses []*pb.DomainStatus, plan automation.PlanResult) *pb.BlockingSummary {
	type groupKey struct {
		category string
		domain   string
		stage    string
		status   pb.PlanStatus
	}
	groups := map[groupKey]*pb.BlockingGroup{}
	add := func(category, domain, stage string, status pb.PlanStatus, reasons []string) {
		if len(reasons) == 0 {
			return
		}
		if stage == "" {
			stage = "unknown"
		}
		key := groupKey{category: category, domain: domain, stage: stage, status: status}
		group := groups[key]
		if group == nil {
			group = &pb.BlockingGroup{Category: category, Domain: domain, Stage: stage, Status: status}
			groups[key] = group
		}
		group.Count++
		for _, reason := range reasons {
			if reason == "" || containsString(group.Reasons, reason) {
				continue
			}
			group.Reasons = append(group.Reasons, reason)
		}
	}
	for _, domain := range domainStatuses {
		add(domain.GetCategory(), domain.GetDomain(), "domain", pb.PlanStatus_PLAN_STATUS_BLOCKED, domain.GetBlockedReasons())
	}
	for _, demand := range plan.Demands {
		add(demand.Category, demand.Domain, demand.BlockingStage, planStatusProto(demand.Status), demand.BlockedReasons)
	}
	for _, op := range plan.Operations {
		add(op.Category, op.Domain, op.BlockingStage, planStatusProto(op.Status), op.BlockedReasons)
		for _, gate := range op.CostGates {
			add(op.Category, op.Domain, gate.Source, planStatusProto(gate.Status), gate.BlockedReasons)
		}
	}
	keys := make([]groupKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].category != keys[j].category {
			return keys[i].category < keys[j].category
		}
		if keys[i].domain != keys[j].domain {
			return keys[i].domain < keys[j].domain
		}
		if keys[i].stage != keys[j].stage {
			return keys[i].stage < keys[j].stage
		}
		return keys[i].status < keys[j].status
	})
	out := &pb.BlockingSummary{}
	for _, key := range keys {
		group := groups[key]
		out.Total += group.Count
		out.Groups = append(out.Groups, group)
	}
	return out
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func itemNameOrID(itemID int32) string {
	name := state.ItemName(itemID)
	if name != "" {
		return name
	}
	return fmt.Sprintf("#%d", itemID)
}

func buildDomainStatuses(policy *pb.Policy, diag runner.Diagnostics, connected bool) []*pb.DomainStatus {
	policy = automation.DefaultPolicyIfNil(policy)
	observed := setOfStrings(diag.ObservedNamespaces)
	blocked := append([]string(nil), diag.BlockedReasons...)
	statuses := []*pb.DomainStatus{
		domainStatus("basic", "basic", basicEnabled(policy.GetBasic()), observedAny(observed, "7", "22", "33", "116", "117", "119", "129"), blocked, "", connected),
		domainStatus("plant", "plant", plantEnabled(policy.GetPlant()), observedAny(observed, "100", "101", "104", "105", "109", "114"), blocked, "", connected),
		domainStatus("order", "order", orderEnabled(policy.GetOrder()), observedAny(observed, "104", "105", "107", "108", "109"), blocked, "", connected),
		domainStatus("union", "union", unionEnabled(policy.GetUnion()), observedAny(observed, "25", "152"), blocked, "", connected),
		domainStatus("activity", "activity", activityEnabled(policy.GetActivity()), observedActivity(observed), blocked, "", connected),
	}
	applyOperationCooldownsToDomainStatuses(statuses, diag.OperationCooldowns)
	return statuses
}

func featureCapabilitiesProto() []*pb.FeatureCapability {
	specs := automation.FeatureCatalog()
	out := make([]*pb.FeatureCapability, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		if _, exists := seen[spec.ID]; exists {
			continue
		}
		seen[spec.ID] = struct{}{}
		out = append(out, &pb.FeatureCapability{
			Id:             spec.ID,
			Label:          spec.Label,
			Category:       spec.Category,
			Domain:         spec.Domain,
			Action:         spec.Action,
			Status:         planStatusProto(spec.Status),
			Executable:     spec.Executable,
			SyncOnly:       spec.SyncOnly,
			BlockedReasons: append([]string(nil), spec.BlockedReasons...),
		})
	}
	return out
}

func domainStatus(category, domain string, enabled bool, observed bool, blocked []string, lastErr string, connected bool) *pb.DomainStatus {
	status := pb.PlanStatus_PLAN_STATUS_SKIPPED
	var reasons []string
	if enabled {
		status = pb.PlanStatus_PLAN_STATUS_READY
		if !connected {
			status = pb.PlanStatus_PLAN_STATUS_BLOCKED
			reasons = append(reasons, "WebSocket 未连接")
		}
		if len(blocked) > 0 {
			status = pb.PlanStatus_PLAN_STATUS_BLOCKED
			reasons = append(reasons, blocked...)
		}
		if !observed && connected {
			status = pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
		}
	}
	return &pb.DomainStatus{
		Category:       category,
		Domain:         domain,
		Lane:           defaultDomainLane(category),
		Observed:       observed,
		Status:         status,
		BlockedReasons: reasons,
		LastError:      lastErr,
	}
}

func defaultDomainLane(category string) pb.ExecutionLane {
	if category == automation.CategoryPlant {
		return pb.ExecutionLane_EXECUTION_LANE_FARM
	}
	return pb.ExecutionLane_EXECUTION_LANE_SIDE
}

func applyOperationCooldownsToDomainStatuses(statuses []*pb.DomainStatus, cooldowns []runner.OperationCooldownSnapshot) {
	for _, status := range statuses {
		var selected runner.OperationCooldownSnapshot
		for _, cd := range cooldowns {
			if cd.Category != status.GetCategory() {
				continue
			}
			if selected.Until.IsZero() || cd.Until.Before(selected.Until) {
				selected = cd
			}
		}
		if selected.Until.IsZero() {
			continue
		}
		status.Lane = executionLaneProto(selected.Lane)
		status.CooldownUntilMs = selected.Until.UnixMilli()
		status.CooldownReason = selected.Reason
		status.Status = pb.PlanStatus_PLAN_STATUS_BLOCKED
		if selected.Reason != "" && !containsString(status.BlockedReasons, selected.Reason) {
			status.BlockedReasons = append(status.BlockedReasons, selected.Reason)
		}
	}
}

func accountHealth(connected bool, diag runner.Diagnostics) string {
	switch {
	case diag.SessionInvalidatedReason != "":
		return "session_expired"
	case len(diag.BlockedReasons) > 0:
		return "blocked"
	case connected:
		return "online"
	default:
		return "offline"
	}
}

// applyStoppedRunnerDiagnostics fills status fields when no live runner remains.
// Retained kick/expiry diagnostics keep the account in 异常 instead of bare offline.
func applyStoppedRunnerDiagnostics(out *pb.AccountStatus, policy *pb.Policy, diag runner.Diagnostics) {
	out.Diagnostics = runnerDiagnosticsProto(diag)
	out.DomainStatuses = buildDomainStatuses(policy, diag, false)
	out.Health = accountHealth(false, diag)
	if diag.LastOperationError != "" {
		out.LastError = diag.LastOperationError
		return
	}
	out.LastError = diag.SessionInvalidatedReason
}

func basicEnabled(p *pb.BasicPolicy) bool {
	task := p.GetTask()
	benefit := p.GetBenefit()
	sign := p.GetSign()
	return p != nil && (task.GetMainEnabled() || task.GetDailyEnabled() || task.GetWeeklyEnabled() ||
		task.GetStoryEnabled() || task.GetAchievementEnabled() || p.GetMailEnabled() ||
		sign.GetDailyEnabled() || sign.GetPatchEnabled() || p.GetFreeWaterEnabled() ||
		p.GetWaterwheelEnabled() || benefit.GetBoxEnabled() || benefit.GetDoubleCoinEnabled() ||
		benefit.GetShareRewardEnabled() || benefit.GetAntiScamBoxEnabled() ||
		p.GetMapEventEnabled() || p.GetRoadGrowRewardEnabled() ||
		p.GetPearl().GetFreeEnabled() || p.GetShop().GetVideoFreeGiftEnabled() ||
		p.GetZoo().GetEnabled())
}

func plantEnabled(p *pb.PlantPolicy) bool {
	planting := p.GetPlanting()
	cultivate := p.GetCultivate()
	return p != nil && (planting.GetAutoEnabled() || planting.GetAutoHarvestEnabled() ||
		planting.GetUseSpeedUpTicket() || planting.GetVideoSpeedUpEnabled() || planting.GetAutoUnlockLand() ||
		cultivate.GetEnabled() || cultivate.GetUpgradeEnabled() || cultivate.GetVideoSpeedUpEnabled() ||
		p.GetFriendSteal().GetEnabled() || p.GetElves().GetEnabled() || p.GetMarket().GetPutEnabled() ||
		p.GetMarket().GetAutoBuyFromFriend())
}

func orderEnabled(p *pb.OrderPolicy) bool {
	return p != nil && (p.GetCustomer().GetEnabled() || residentOrderEnabledProto(p.GetResident()) ||
		p.GetPalace().GetEnabled() || p.GetTeam().GetEnabled() || p.GetFlowerArt().GetSellEnabled() ||
		p.GetFlowerArt().GetCraftEnabled())
}

func residentOrderEnabledProto(p *pb.ResidentOrderPolicy) bool {
	return p != nil && (p.GetNormalEnabled() || p.GetDecorateEnabled() || p.GetSatinEnabled() ||
		p.GetRewardEnabled())
}

func unionEnabled(p *pb.UnionPolicy) bool {
	build := p.GetBuild()
	flower := p.GetFlower()
	race := p.GetRace()
	land := p.GetLand()
	return p != nil && (build.GetFreeEnabled() || build.GetGoldEnabled() || build.GetDiamondEnabled() ||
		flower.GetShareEnabled() || flower.GetTakeEnabled() || race.GetEnabled() ||
		land.GetAutoPlantEnabled() || land.GetHarvestEnabled() ||
		p.GetRedPacketEnabled() || p.GetForestEnabled())
}

func activityEnabled(p *pb.ActivityPolicy) bool {
	if p == nil {
		return false
	}
	// Parent ActivityPolicy.enabled is legacy/no-op; modules are independent.
	for _, module := range p.GetModules() {
		if module != nil && module.GetEnabled() {
			return true
		}
	}
	return false
}

func observedActivity(observed map[string]struct{}) bool {
	return observedAny(observed, "138", "139", "140", "152", "155", "160", "161", "162", babigame.CelebrityNamespaceLegacy, babigame.CelebrityNamespace)
}

func setOfStrings(xs []string) map[string]struct{} {
	out := make(map[string]struct{}, len(xs))
	for _, x := range xs {
		out[x] = struct{}{}
	}
	return out
}

func observedAny(set map[string]struct{}, keys ...string) bool {
	for _, key := range keys {
		if _, ok := set[key]; ok {
			return true
		}
	}
	return false
}

func cloneInt32Map(in map[int32]int32) map[int32]int32 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int32]int32, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func buildFmlLandViews(lands map[int32]state.FmlLandView, cultivations map[int32]state.CultivateView, now time.Time) []*pb.FmlLandView {
	if len(lands) == 0 {
		return nil
	}
	ids := make([]int32, 0, len(lands))
	for id := range lands {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]*pb.FmlLandView, 0, len(ids))
	for _, id := range ids {
		out = append(out, fmlLandViewProto(lands[id], cultivations, now))
	}
	return out
}

func fmlLandViewProto(land state.FmlLandView, cultivations map[int32]state.CultivateView, now time.Time) *pb.FmlLandView {
	pending := state.FmlLandPendingHarvest(land, now)
	kind, reason := "wait", "成长中"
	switch {
	case pending > 0:
		kind, reason = "harvest", fmt.Sprintf("可收获 %d 朵", pending)
	case land.FlowerID <= 0:
		kind, reason = "plant", "空地可种植"
	}
	var stockCap, timeSec int32
	if cfg, ok := state.FmlLandLvlByID(land.Level); ok {
		stockCap = cfg.Stock
		timeSec = cfg.TimeSec
	}
	var flowerLvl int32
	if land.FlowerID > 0 {
		if cv, ok := cultivations[land.FlowerID]; ok {
			flowerLvl = cv.Lvl
		}
	}
	return &pb.FmlLandView{
		LandId:            land.LandID,
		Level:             land.Level,
		FlowerId:          land.FlowerID,
		StartTimeMs:       land.StartTimeMs,
		MatureFlowerCount: land.MatureFlowerCnt,
		HarvestedCount:    land.HarvestedCnt,
		LastCalcTimeMs:    land.LastCalcTimeMs,
		PendingHarvest:    pending,
		StockCap:          stockCap,
		TimeSec:           timeSec,
		NextMatureMs:      state.FmlLandNextMatureMs(land, now),
		Recommendation:    kind,
		Reason:            reason,
		FlowerLvl:         flowerLvl,
	}
}

func buildLandViews(lands map[int32]state.LandView, farmLands []state.FarmLandInfo, rosterObserved bool, farmLandObserved bool, level int32, now time.Time, harvestDelay time.Duration) []*pb.LandView {
	out := make([]*pb.LandView, 0, len(lands))
	seen := make(map[int32]struct{}, len(lands))
	unopenedCount := 0
	for _, info := range farmLands {
		l, observed := lands[info.ID]
		isUnopened := !observed && rosterObserved && farmLandObserved
		if isUnopened {
			unopenedCount++
		}
		out = append(out, landViewProtoWithLimit(info.ID, l, info, observed, rosterObserved, farmLandObserved, level, now, unopenedCount, harvestDelay))
		seen[info.ID] = struct{}{}
	}
	extraIDs := make([]int32, 0)
	for id := range lands {
		if _, ok := seen[id]; ok {
			continue
		}
		extraIDs = append(extraIDs, id)
	}
	sort.Slice(extraIDs, func(i, j int) bool { return extraIDs[i] < extraIDs[j] })
	for _, id := range extraIDs {
		out = append(out, landViewProtoWithLimit(id, lands[id], state.FarmLandInfo{}, true, rosterObserved, farmLandObserved, level, now, 0, harvestDelay))
	}
	return out
}

const maxReclaimableLands = 6

func landViewProtoWithLimit(id int32, l state.LandView, info state.FarmLandInfo, observed bool, rosterObserved bool, farmLandObserved bool, level int32, now time.Time, unopenedIdx int, harvestDelay time.Duration) *pb.LandView {
	kind, reason := "unknown", "等待服务端土地清单"
	status := "locked"
	switch {
	case observed:
		kind, reason = automation.Recommend(l, now, harvestDelay)
		status = "opened"
	case !farmLandObserved:
		kind, reason = "unknown", "等待当前客户端土地配置"
	case rosterObserved:
		if unopenedIdx > 0 && unopenedIdx <= maxReclaimableLands {
			status = "unopened"
			if len(info.Cost) >= 2 {
				kind, reason = "unlock", "可开垦"
			} else {
				kind, reason = "unknown", "缺少开垦消耗配置"
			}
		} else {
			status = "locked"
			kind, reason = "locked", "未解锁"
		}
	}
	if observed && !l.Observed {
		observed = false
	}
	return &pb.LandView{
		LandId:         id,
		FlowerId:       int32(l.FlowerID),
		State:          int32(l.State),
		Lvl:            int32(l.Lvl),
		HarvestCnt:     int32(l.HarvestCnt),
		NextTimeMs:     l.NextTimeMs,
		PlantTimeMs:    l.PlantTimeMs,
		Recommendation: kind,
		Reason:         reason,
		LandStatus:     status,
		Observed:       observed,
		OpenLevel:      info.OpenLevel,
		UnlockCost:     farmLandActualCost(info.Cost),
		Wasteland:      append([]int32(nil), info.Wasteland...),
	}
}

func farmLandActualCost(cost []int32) []int32 {
	if len(cost) < 2 {
		return append([]int32(nil), cost...)
	}
	actualGold := cost[1] - cost[0] + 11
	return []int32{actualGold}
}

const defaultStreamEventReplayLimit = 500

type streamEventFilter struct {
	kinds      map[string]struct{}
	kindList   []string
	accountIDs []int64
	allowedIDs map[string]struct{}
}

// StreamEvents is a server-streaming RPC. It first replays persisted event_log
// rows, then forwards live events from the in-process bus. The subscription is
// opened before replay so events emitted during DB replay are either included
// in the replay result or delivered live.
func (svc *Services) StreamEvents(ctx context.Context, req *connect.Request[pb.StreamEventsRequest], stream *connect.ServerStream[pb.Event]) error {
	ch, cancel := svc.Manager.Bus().SubscribeLive(256)
	defer cancel()

	filter, err := svc.streamEventFilter(ctx, req.Msg)
	if err != nil {
		return err
	}

	highWater := req.Msg.GetAfterEventId()
	if replayLimit := streamEventReplayLimit(req.Msg.GetReplayLimit()); replayLimit > 0 {
		events, err := svc.DB.ListEventLogs(ctx, store.ListEventLogsOptions{
			AccountIDs: filter.accountIDs,
			Kinds:      filter.kindList,
			AfterID:    req.Msg.GetAfterEventId(),
			Limit:      replayLimit,
		})
		if err != nil {
			return mapErr(err)
		}
		for _, event := range events {
			if event.ID > highWater {
				highWater = event.ID
			}
			if err := stream.Send(eventLogToProto(event)); err != nil {
				return err
			}
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-ch:
			if !ok {
				return nil
			}
			if e.ID > 0 && e.ID <= highWater {
				continue
			}
			if !filter.matches(e) {
				continue
			}
			if err := stream.Send(e.ToProto()); err != nil {
				return err
			}
			if e.ID > highWater {
				highWater = e.ID
			}
		}
	}
}

func (svc *Services) streamEventFilter(ctx context.Context, req *pb.StreamEventsRequest) (streamEventFilter, error) {
	filter := streamEventFilter{}
	for _, kind := range req.GetKinds() {
		if kind == "" {
			continue
		}
		if filter.kinds == nil {
			filter.kinds = make(map[string]struct{}, len(req.GetKinds()))
		}
		if _, ok := filter.kinds[kind]; ok {
			continue
		}
		filter.kinds[kind] = struct{}{}
		filter.kindList = append(filter.kindList, kind)
	}

	if req.GetAccountId() != "" || req.GetAccountName() != "" {
		acc, err := svc.resolveAccount(ctx, req.GetAccountId(), req.GetAccountName())
		if err != nil {
			return filter, mapErr(err)
		}
		idStr := fmt.Sprintf("%d", acc.ID)
		filter.accountIDs = []int64{acc.ID}
		filter.allowedIDs = map[string]struct{}{idStr: {}}
		return filter, nil
	}

	if !auth.IsAdmin(ctx) {
		userID := auth.UserIDFromContext(ctx)
		accounts, err := svc.DB.ListAccounts(ctx, userID)
		if err != nil {
			return filter, mapErr(err)
		}
		filter.accountIDs = make([]int64, 0, len(accounts))
		filter.allowedIDs = make(map[string]struct{}, len(accounts))
		for _, acc := range accounts {
			filter.accountIDs = append(filter.accountIDs, acc.ID)
			filter.allowedIDs[fmt.Sprintf("%d", acc.ID)] = struct{}{}
		}
	}

	return filter, nil
}

func (f streamEventFilter) matches(e runner.Event) bool {
	if len(f.kinds) > 0 {
		if _, ok := f.kinds[e.Kind]; !ok {
			return false
		}
	}
	if f.allowedIDs != nil {
		if _, ok := f.allowedIDs[e.AccountID]; !ok {
			return false
		}
	}
	return true
}

func streamEventReplayLimit(limit int32) int {
	if limit < 0 {
		return 0
	}
	if limit == 0 {
		return defaultStreamEventReplayLimit
	}
	return int(limit)
}

func eventLogToProto(e store.EventLog) *pb.Event {
	return &pb.Event{
		Id:          e.ID,
		Ts:          timestamppb.New(e.TS),
		AccountId:   fmt.Sprintf("%d", e.AccountID),
		AccountName: e.AccountName,
		Kind:        e.Kind,
		Message:     e.Message,
		PayloadJson: e.PayloadJSON,
		Category:    e.Category,
		Domain:      e.Domain,
		Action:      e.Action,
		Label:       e.Label,
		Level:       e.Level,
	}
}
