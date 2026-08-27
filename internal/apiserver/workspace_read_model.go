package apiserver

import (
	"context"
	"sort"
	"sync"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (svc *Services) accountStatuses(ctx context.Context) ([]*pb.AccountStatus, error) {
	var userID int64
	if !auth.IsAdmin(ctx) {
		userID = auth.UserIDFromContext(ctx)
	}
	accs, err := svc.DB.ListAccounts(ctx, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	statuses := make([]*pb.AccountStatus, 0, len(accs))
	for _, a := range accs {
		status, err := svc.statusFor(ctx, a)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (svc *Services) statusFor(ctx context.Context, acc *store.Account) (*pb.AccountStatus, error) {
	out := &pb.AccountStatus{
		AccountId:   acc.ID,
		AccountName: acc.Name,
		GsIdx:       acc.GsIdx,
	}
	var r *runner.Runner
	if svc.Manager != nil {
		r = svc.Manager.Get(acc.ID)
	}
	if r == nil {
		policy, err := svc.policyFor(ctx, acc.ID)
		if err != nil {
			return nil, err
		}
		out.AutomationEnabled = policy.GetAutomationEnabled()
		var diag runner.Diagnostics
		if svc.Manager != nil {
			diag, _ = svc.Manager.LastDiagnostics(acc.ID)
		}
		applyStoppedRunnerDiagnostics(out, policy, diag)
		if svc.Manager != nil {
			stats, ok := svc.Manager.RuntimeStats(acc.ID)
			if ok {
				out.RuntimeStatistics = runtimeStatisticsProto(stats)
			}
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

type accountReadModel struct {
	account *store.Account
	runner  *runner.Runner
	state   *state.State
	policy  *pb.Policy
	now     time.Time
	diag    runner.Diagnostics
	plan    automation.PlanResult
}

func newAccountReadModel(acc *store.Account, r *runner.Runner) *accountReadModel {
	now := time.Now()
	st := r.State()
	policy := r.Policy()
	return &accountReadModel{
		account: acc,
		runner:  r,
		state:   st,
		policy:  policy,
		now:     now,
		diag:    r.Diagnostics(now),
		plan:    automation.BuildPlan(st, policy, now),
	}
}

const workspaceReadModelAttempts = 4

type workspaceProjectionCache struct {
	mu          sync.Mutex
	runner      *runner.Runner
	revision    uint64
	lastEventAt time.Time
	accountName string
	gsIdx       int32
	policy      *pb.Policy
	state       *pb.WorkspaceState
}

// workspaceStateForAccount is the hot push path. A workspace session retains
// its already-authorized account, so live updates never re-read account rows or
// statistics from SQLite.
func (svc *Services) workspaceStateForAccount(ctx context.Context, acc *store.Account) (*pb.WorkspaceState, error) {
	var r *runner.Runner
	if svc.Manager != nil {
		r = svc.Manager.Get(acc.ID)
	}
	if r == nil {
		status, statusErr := svc.statusFor(ctx, acc)
		if statusErr != nil {
			return nil, statusErr
		}
		policy, policyErr := svc.policyFor(ctx, acc.ID)
		if policyErr != nil {
			return nil, policyErr
		}
		return &pb.WorkspaceState{
			AccountId:     acc.ID,
			AccountStatus: status,
			Policy:        policy,
			Statistics:    &pb.WorkspaceStatistics{RuntimeStatistics: status.GetRuntimeStatistics()},
		}, nil
	}
	return svc.cachedLiveWorkspaceState(ctx, acc, r)
}

func (svc *Services) cachedLiveWorkspaceState(ctx context.Context, acc *store.Account, r *runner.Runner) (*pb.WorkspaceState, error) {
	svc.workspaceProjectionMu.Lock()
	if svc.workspaceProjections == nil {
		svc.workspaceProjections = make(map[int64]*workspaceProjectionCache)
	}
	cache := svc.workspaceProjections[acc.ID]
	if cache == nil {
		cache = new(workspaceProjectionCache)
		svc.workspaceProjections[acc.ID] = cache
	}
	svc.workspaceProjectionMu.Unlock()

	cache.mu.Lock()
	defer cache.mu.Unlock()
	revision := r.State().Revision()
	lastEventAt := r.LastEventAt()
	policy := r.Policy()
	if cache.state != nil && cache.runner == r && cache.revision == revision && cache.lastEventAt.Equal(lastEventAt) &&
		cache.accountName == acc.Name && cache.gsIdx == acc.GsIdx && proto.Equal(cache.policy, policy) {
		return cache.state, nil
	}
	state, err := svc.buildLiveWorkspaceState(ctx, acc, r)
	if err != nil {
		return nil, err
	}
	cache.runner = r
	cache.revision = revision
	cache.lastEventAt = lastEventAt
	cache.accountName = acc.Name
	cache.gsIdx = acc.GsIdx
	cache.policy = proto.Clone(policy).(*pb.Policy)
	cache.state = state
	return state, nil
}

func (svc *Services) buildLiveWorkspaceState(ctx context.Context, acc *store.Account, r *runner.Runner) (*pb.WorkspaceState, error) {

	// State getters are individually synchronized. Retrying around the whole
	// projection prevents one pushed frame from mixing two runner revisions
	// when a game response lands during read-model construction.
	var out *pb.WorkspaceState
	for attempt := 0; attempt < workspaceReadModelAttempts; attempt++ {
		revision := r.State().Revision()
		status, statusErr := svc.statusFor(ctx, acc)
		if statusErr != nil {
			return nil, statusErr
		}
		model := newAccountReadModel(acc, r)
		candidate := &pb.WorkspaceState{
			Revision:      revision,
			AccountId:     acc.ID,
			AccountStatus: status,
			Policy:        model.policy,
			Basic:         buildBasicView(model),
			Garden:        buildGardenView(model),
			Orders:        buildOrdersView(model),
			Union:         buildUnionView(model),
			Activities:    buildActivitiesView(model),
			Warehouse:     buildWarehouseView(model),
		}
		out = candidate
		if r.State().Revision() == revision {
			break
		}
	}
	out.Statistics = &pb.WorkspaceStatistics{
		RuntimeStatistics:  out.Basic.GetRuntimeStatistics(),
		BusinessStatistics: out.Orders.GetBusinessStatistics(),
	}
	return out, nil
}

func buildBasicView(model *accountReadModel) *pb.BasicView {
	st, now := model.state, model.now
	waterDrops, waterDropsTotal, waterDropsNextMs := st.WaterDrops()
	diamondsFree, diamondsPaid := st.Diamonds()
	vip, vipExp := st.Vip()
	expToNext, nextLevelExp, levelMaxed := st.ExperienceToNextLevel()
	domainStatuses := buildDomainStatuses(model.policy, model.diag, model.runner.Connected())
	resp := &pb.BasicView{
		AccountId:             model.account.ID,
		AccountName:           model.account.Name,
		RoleId:                st.RoleID(),
		CapturedAt:            timestamppb.New(now),
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
		Vip:                   vip,
		VipExp:                vipExp,
		NobleEligible:         st.NobleEligible(),
		ObservedNamespaces:    append([]string(nil), model.diag.ObservedNamespaces...),
		UnknownRpcCount:       model.diag.UnknownRPCCount,
		UnknownNamespaceCount: model.diag.UnknownNamespaceCount,
		Diagnostics:           runnerDiagnosticsProto(model.diag),
		DomainStatuses:        domainStatuses,
		PlannedOperations:     plannedOperationsProto(model.plan.Operations, model.diag),
		BlockingSummary:       blockingSummaryProto(domainStatuses, model.plan),
		RuntimeStatistics:     runtimeStatisticsProto(model.runner.RuntimeStats()),
		PendingTasks:          buildPendingTasksAtPolicy(st, now, model.policy),
	}
	if rep, ok := st.Reputation(); ok {
		resp.ReputationObserved = true
		resp.ReputationScore = rep.Score
		resp.ReputationLastSyncTimeMs = rep.LastSyncTimeMs
		resp.ReputationLastViewTimeMs = rep.LastViewTimeMs
	}
	return resp
}

func buildGardenView(model *accountReadModel) *pb.GardenView {
	st, now := model.state, model.now
	resp := &pb.GardenView{
		AccountId:                  model.account.ID,
		AccountName:                model.account.Name,
		CapturedAt:                 timestamppb.New(now),
		PlantableFlowers:           plantableFlowersProto(st.PlantableFlowers(nil, nil)),
		FriendTouchFriends:         friendTouchFriendsProto(st.FriendTouchFriends(now)),
		FriendTouchFriendsObserved: st.FriendTouch(now).FriendsObserved,
	}
	resp.Lands = buildLandViews(st.Lands(), st.FarmLands(), st.LandRosterObserved(), st.FarmLandConfigObserved(), st.Level(), now, time.Duration(model.policy.GetPlant().GetPlanting().GetHarvestDelaySeconds())*time.Second)
	return resp
}

func buildOrdersView(model *accountReadModel) *pb.OrdersView {
	st, now := model.state, model.now
	resp := &pb.OrdersView{
		AccountId:             model.account.ID,
		AccountName:           model.account.Name,
		CapturedAt:            timestamppb.New(now),
		PendingTasks:          buildPendingTasksAtPolicy(st, now, model.policy),
		Demands:               demandsProto(model.plan.Demands),
		Vases:                 vasesProto(st.Vases()),
		FlowerArtAvailability: flowerArtAvailabilityProto(st, model.plan),
		OrderStatistics:       orderStatisticsProto(st, now),
		BusinessStatistics:    businessStatisticsProto(st),
		SellableFlowerArts:    sellableFlowerArtsProto(st),
	}
	return resp
}

func buildUnionView(model *accountReadModel) *pb.UnionView {
	st, now := model.state, model.now
	membership := st.FmlBuild()
	resp := &pb.UnionView{
		AccountId:          model.account.ID,
		AccountName:        model.account.Name,
		CapturedAt:         timestamppb.New(now),
		MembershipObserved: membership.MembershipObserved,
		InUnion:            membership.MembershipObserved && membership.MemberFmlID > 0,
		UnionId:            membership.MemberFmlID,
		LandsObserved:      st.FmlLandObserved(),
		Lands:              buildFmlLandViews(st.FmlLands(), st.Cultivations(), now),
		Race: fmlRaceProto(
			st.FmlRace(), st, model.policy.GetUnion().GetRace(), st.RoleID(), now,
			automation.RaceModuleGates{
				Customer:  model.policy.GetOrder().GetCustomer().GetEnabled(),
				Pearl:     model.policy.GetBasic().GetPearl().GetAutoHireEnabled(),
				Cultivate: model.policy.GetPlant().GetCultivate().GetEnabled(),
			},
		),
	}
	return resp
}

func buildActivitiesView(model *accountReadModel) *pb.ActivitiesView {
	cyclicNote, _ := model.state.CyclicNoteView(model.now)
	cyclicStory, _ := model.state.CyclicStoryView(model.now)
	return &pb.ActivitiesView{
		AccountId:   model.account.ID,
		AccountName: model.account.Name,
		CapturedAt:  timestamppb.New(model.now),
		CyclicNote:  cyclicNoteProto(cyclicNote),
		CyclicStory: cyclicStoryProto(cyclicStory),
	}
}

func buildWarehouseView(model *accountReadModel) *pb.WarehouseView {
	view := &pb.WarehouseView{
		AccountId:       model.account.ID,
		AccountName:     model.account.Name,
		CapturedAt:      timestamppb.New(model.now),
		Inventory:       model.state.Inventory(),
		InventoryLedger: inventoryLedgerProto(model.state.Inventory(), model.plan.Ledger),
	}
	return view
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
