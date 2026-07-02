package apiserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (svc *Services) GetStatus(ctx context.Context, req *connect.Request[pb.GetStatusRequest]) (*connect.Response[pb.GetStatusResponse], error) {
	resp := &pb.GetStatusResponse{}
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
	}
	r := svc.Manager.Get(acc.ID)
	if r == nil {
		policy, err := svc.policyFor(ctx, acc.ID)
		if err != nil {
			return nil, err
		}
		out.AutomationEnabled = policy.GetAutomationEnabled()
		out.DomainStatuses = buildDomainStatuses(policy, runner.Diagnostics{}, false)
		out.Health = "offline"
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
	lands := r.State().Lands()
	out.KnownLands = int32(len(lands))
	byKind := map[string]int32{}
	for _, l := range lands {
		kind, _ := automation.Recommend(l, now)
		byKind[kind]++
	}
	out.ByKind = byKind
	for _, count := range r.State().FlowerInventory() {
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
	diag := r.Diagnostics(now)
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
		DiamondsFree:          diamondsFree,
		DiamondsPaid:          diamondsPaid,
		PendingTasks:          buildPendingTasks(st),
		Vip:                   vip,
		VipExp:                vipExp,
		NobleEligible:         st.NobleEligible(),
		ObservedNamespaces:    diag.ObservedNamespaces,
		UnknownRpcCount:       diag.UnknownRPCCount,
		UnknownNamespaceCount: diag.UnknownNamespaceCount,
		Diagnostics:           runnerDiagnosticsProto(diag),
	}
	resp.Lands = buildLandViews(lands, st.FarmLands(), st.LandRosterObserved(), st.FarmLandConfigObserved(), st.Level(), now)
	policy := r.Policy()
	plan := automation.BuildPlan(st, policy, now)
	resp.DomainStatuses = buildDomainStatuses(policy, diag, r.Connected())
	resp.PlannedOperations = plannedOperationsProto(plan.Operations)
	resp.Demands = demandsProto(plan.Demands)
	resp.Vases = vasesProto(st.Vases())
	resp.FlowerArtAvailability = flowerArtAvailabilityProto(st, plan)
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

func timestampOrNil(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func buildPendingTasks(st *state.State) []*pb.PendingTaskView {
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
		out = append(out, &pb.PendingTaskView{
			Category:     "居民订单",
			Id:           strconv.FormatInt(int64(boxID), 10),
			Title:        fmt.Sprintf("居民订单 #%d", boxID),
			Status:       requirementsStatus(reqs),
			Requirements: reqs,
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

	if task, ok := st.MainTask(); ok {
		title := state.MainTaskTitle(task.TaskID)
		if title == "" {
			title = fmt.Sprintf("主线任务 #%d", task.TaskID)
		}
		view := &pb.PendingTaskView{
			Category: "主线任务",
			Id:       strconv.FormatInt(int64(task.TaskID), 10),
			Title:    title,
			Finished: task.Finished,
			Status:   "progress",
		}
		if flowerID, target, ok := state.MainTaskFlowerTarget(task.TaskID); ok {
			view.Target = target
			view.Requirements = []*pb.RequirementView{requirementView(flowerID, target, inventory[flowerID])}
			view.Status = requirementsStatus(view.Requirements)
		}
		out = append(out, view)
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
		status := "progress"
		if task.Status == 1 || (task.Status == 0 && task.Target > 0 && task.Finished >= task.Target) {
			status = "ready"
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

	for _, id := range st.ReadyRandomEventIDs() {
		out = append(out, &pb.PendingTaskView{
			Category: "地图事件",
			Id:       strconv.FormatInt(int64(id), 10),
			Title:    fmt.Sprintf("地图事件 #%d", id),
			Status:   "ready",
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

func requirementsStatus(reqs []*pb.RequirementView) string {
	for _, req := range reqs {
		if req.GetMissing() > 0 {
			return "missing"
		}
	}
	return "ready"
}

func plannedOperationsProto(ops []automation.PlannedOp) []*pb.PlannedOperation {
	out := make([]*pb.PlannedOperation, 0, len(ops))
	for _, op := range ops {
		out = append(out, &pb.PlannedOperation{
			Category:       op.Category,
			Domain:         op.Domain,
			Action:         op.Action,
			Rpc:            op.Kind,
			Reason:         op.Reason,
			LandIds:        append([]int32(nil), op.LandIDs...),
			FlowerId:       op.FlowerID,
			Priority:       op.Priority,
			GoldCost:       op.GoldCost,
			DiamondCost:    op.DiamondCost,
			ItemCost:       cloneInt32Map(op.ItemCost),
			FeatureId:      op.FeatureID,
			Label:          op.Label,
			Status:         op.Status,
			Executable:     op.Executable,
			SyncOnly:       op.SyncOnly,
			BlockedReasons: append([]string(nil), op.BlockedReasons...),
			OperationId:    op.OperationID,
			GoalId:         op.GoalID,
			DemandId:       op.DemandID,
			TargetId:       op.TargetID,
			ItemId:         op.ItemID,
			Count:          op.Count,
			VaseId:         op.VaseID,
			FlowerIds:      append([]int32(nil), op.FlowerIDs...),
		})
	}
	return out
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
		})
	}
	return out
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
	seen := map[int32]struct{}{}
	for _, demand := range plan.Demands {
		if demand.Kind == automation.DemandKindFlowerArt && demand.ItemID > 0 {
			seen[demand.ItemID] = struct{}{}
		}
	}
	for _, op := range plan.Operations {
		if op.ItemID > 0 {
			if _, ok := state.FlowerArtRecipeByID(op.ItemID); ok {
				seen[op.ItemID] = struct{}{}
			}
		}
	}
	ids := make([]int32, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]*pb.FlowerArtAvailabilityView, 0, len(ids))
	for _, id := range ids {
		availability := automation.FlowerArtAvailability(st, id, 1, plan.Ledger)
		if availability.Recipe.ArtID == 0 {
			continue
		}
		view := &pb.FlowerArtAvailabilityView{
			ArtId:          availability.Recipe.ArtID,
			ArtName:        itemNameOrID(availability.Recipe.ArtID),
			VaseId:         availability.Recipe.VaseID,
			Level:          availability.Recipe.Level,
			SaleValue:      availability.Recipe.SaleValue,
			LevelOk:        availability.LevelOK,
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
	return []*pb.DomainStatus{
		domainStatus("basic", "basic", basicEnabled(policy.GetBasic()), observedAny(observed, "7", "22", "116", "117", "119", "129"), blocked, diag.LastOperationError, connected),
		domainStatus("plant", "plant", plantEnabled(policy.GetPlant()), observedAny(observed, "100", "101", "104", "105", "109", "114"), blocked, diag.LastOperationError, connected),
		domainStatus("order", "order", orderEnabled(policy.GetOrder()), observedAny(observed, "104", "105", "107", "108", "109"), blocked, diag.LastOperationError, connected),
		domainStatus("union", "union", unionEnabled(policy.GetUnion()), observedAny(observed, "25", "152"), blocked, diag.LastOperationError, connected),
		domainStatus("activity", "activity", activityEnabled(policy.GetActivity()), observedActivity(observed), blocked, diag.LastOperationError, connected),
	}
}

func domainStatus(category, domain string, enabled bool, observed bool, blocked []string, lastErr string, connected bool) *pb.DomainStatus {
	status := "disabled"
	var reasons []string
	if enabled {
		status = "ready"
		if !connected {
			status = "blocked"
			reasons = append(reasons, "WebSocket 未连接")
		}
		if len(blocked) > 0 {
			status = "blocked"
			reasons = append(reasons, blocked...)
		}
		if !observed && connected {
			status = "syncing"
		}
	}
	return &pb.DomainStatus{
		Category:       category,
		Domain:         domain,
		Observed:       observed,
		Status:         status,
		BlockedReasons: reasons,
		LastError:      lastErr,
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

func basicEnabled(p *pb.BasicPolicy) bool {
	task := p.GetTask()
	benefit := p.GetBenefit()
	sign := p.GetSign()
	return p != nil && (task.GetMainEnabled() || task.GetDailyEnabled() || task.GetWeeklyEnabled() ||
		task.GetStoryEnabled() || task.GetAchievementEnabled() || p.GetMailEnabled() ||
		sign.GetDailyEnabled() || sign.GetPatchEnabled() || p.GetFreeWaterEnabled() ||
		p.GetWaterwheelEnabled() || benefit.GetBoxEnabled() || benefit.GetDoubleCoinEnabled() ||
		benefit.GetShareRewardEnabled() || benefit.GetAntiScamBoxEnabled() ||
		p.GetRandomEventEnabled() || p.GetRoadGrowRewardEnabled() ||
		p.GetPearl().GetFreeEnabled() || p.GetShop().GetVideoFreeGiftEnabled() ||
		p.GetFeedCat().GetEnabled())
}

func plantEnabled(p *pb.PlantPolicy) bool {
	flower := p.GetFlower()
	cultivate := p.GetCultivate()
	return p != nil && (flower.GetHarvestEnabled() || flower.GetPlantEnabled() || flower.GetWaterEnabled() ||
		flower.GetUseSpeedUpTicket() || flower.GetVideoSpeedUpEnabled() || flower.GetAutoUnlockLand() ||
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
	if p == nil || !p.GetEnabled() {
		return false
	}
	for _, module := range p.GetModules() {
		if module != nil && module.GetEnabled() {
			return true
		}
	}
	return false
}

func observedActivity(observed map[string]struct{}) bool {
	return observedAny(observed, "138", "139", "140", "152", "155", "160", "161", "162", "165")
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

func buildLandViews(lands map[int32]state.LandView, farmLands []state.FarmLandInfo, rosterObserved bool, farmLandObserved bool, level int32, now time.Time) []*pb.LandView {
	out := make([]*pb.LandView, 0, len(lands))
	seen := make(map[int32]struct{}, len(lands))
	unopenedCount := 0
	for _, info := range farmLands {
		l, observed := lands[info.ID]
		isUnopened := !observed && rosterObserved && farmLandObserved
		if isUnopened {
			unopenedCount++
		}
		out = append(out, landViewProtoWithLimit(info.ID, l, info, observed, rosterObserved, farmLandObserved, level, now, unopenedCount))
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
		out = append(out, landViewProtoWithLimit(id, lands[id], state.FarmLandInfo{}, true, rosterObserved, farmLandObserved, level, now, 0))
	}
	return out
}

const maxReclaimableLands = 6

func landViewProtoWithLimit(id int32, l state.LandView, info state.FarmLandInfo, observed bool, rosterObserved bool, farmLandObserved bool, level int32, now time.Time, unopenedIdx int) *pb.LandView {
	kind, reason := "unknown", "等待服务端土地清单"
	status := "locked"
	switch {
	case observed:
		kind, reason = automation.Recommend(l, now)
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

// StreamEvents is a server-streaming RPC. We subscribe to the in-process bus
// for the lifetime of the request and forward filtered events to the wire.
func (svc *Services) StreamEvents(ctx context.Context, req *connect.Request[pb.StreamEventsRequest], stream *connect.ServerStream[pb.Event]) error {
	ch, cancel := svc.Manager.Bus().Subscribe(256)
	defer cancel()

	wantedKinds := map[string]struct{}{}
	for _, k := range req.Msg.GetKinds() {
		wantedKinds[k] = struct{}{}
	}
	wantedID := req.Msg.GetAccountId()
	wantedName := req.Msg.GetAccountName()
	allowedIDs := map[string]struct{}{}
	if wantedID != "" || wantedName != "" {
		acc, err := svc.resolveAccount(ctx, wantedID, wantedName)
		if err != nil {
			return mapErr(err)
		}
		wantedID = fmt.Sprintf("%d", acc.ID)
		wantedName = acc.Name
	} else if !auth.IsAdmin(ctx) {
		userID := auth.UserIDFromContext(ctx)
		accounts, err := svc.DB.ListAccounts(ctx, userID)
		if err != nil {
			return mapErr(err)
		}
		for _, acc := range accounts {
			allowedIDs[fmt.Sprintf("%d", acc.ID)] = struct{}{}
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
			if len(wantedKinds) > 0 {
				if _, has := wantedKinds[e.Kind]; !has {
					continue
				}
			}
			if wantedID != "" && e.AccountID != wantedID {
				continue
			}
			if wantedName != "" && e.AccountName != wantedName {
				continue
			}
			if len(allowedIDs) > 0 {
				if _, ok := allowedIDs[e.AccountID]; !ok {
					continue
				}
			}
			if err := stream.Send(e.ToProto()); err != nil {
				return err
			}
		}
	}
}
