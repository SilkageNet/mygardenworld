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
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
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
		resp.Accounts = append(resp.Accounts, svc.statusFor(ctx, acc))
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
		resp.Accounts = append(resp.Accounts, svc.statusFor(ctx, a))
	}
	return connect.NewResponse(resp), nil
}

func (svc *Services) statusFor(ctx context.Context, acc *store.Account) *pb.AccountStatus {
	out := &pb.AccountStatus{
		AccountId:   fmt.Sprintf("%d", acc.ID),
		AccountName: acc.Name,
	}
	r := svc.Manager.Get(acc.ID)
	if r == nil {
		entries, _ := svc.DB.LoadPolicyValues(ctx, acc.ID)
		policy := policycfg.FromEntries(entries)
		out.AutomationEnabled = policy.GetAutomationEnabled()
		return out
	}
	out.Connected = r.Connected()
	if last := r.LastEventAt(); !last.IsZero() {
		out.LastEventAt = timestamppb.New(last)
	}
	out.AutomationEnabled = r.Policy().GetAutomationEnabled()
	lands := r.State().Lands()
	out.KnownLands = int32(len(lands))
	byKind := map[string]int32{}
	for _, l := range lands {
		kind, _ := automation.Recommend(l, time.Now())
		byKind[kind]++
	}
	out.ByKind = byKind
	for _, count := range r.State().FlowerInventory() {
		out.FlowerStockTotal += count
	}
	return out
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
	resp := &pb.GetSnapshotResponse{
		AccountId:        fmt.Sprintf("%d", acc.ID),
		AccountName:      acc.Name,
		Inventory:        st.Inventory(),
		RoleId:           st.RoleID(),
		CapturedAt:       timestamppb.Now(),
		Gold:             st.Gold(),
		WaterDrops:       waterDrops,
		WaterDropsTotal:  waterDropsTotal,
		WaterDropsNextMs: waterDropsNextMs,
		Level:            st.Level(),
		Experience:       st.Experience(),
		DiamondsFree:     diamondsFree,
		DiamondsPaid:     diamondsPaid,
		PendingTasks:     buildPendingTasks(st),
	}
	resp.Lands = buildLandViews(lands, st.FarmLands(), st.LandRosterObserved(), st.FarmLandConfigObserved(), st.Level(), now)
	return connect.NewResponse(resp), nil
}

func (svc *Services) GetHarvestStats(ctx context.Context, req *connect.Request[pb.GetHarvestStatsRequest]) (*connect.Response[pb.GetHarvestStatsResponse], error) {
	var accountIDs []int64
	if req.Msg.GetAccountId() != "" || req.Msg.GetAccountName() != "" {
		acc, err := svc.resolveAccount(ctx, req.Msg.GetAccountId(), req.Msg.GetAccountName())
		if err != nil {
			return nil, mapErr(err)
		}
		accountIDs = []int64{acc.ID}
	}
	var userID int64
	if len(accountIDs) == 0 && !auth.IsAdmin(ctx) {
		userID = auth.UserIDFromContext(ctx)
	}
	runGap := time.Duration(req.Msg.GetRunGapSeconds()) * time.Second
	stats, err := svc.DB.LatestHarvestStats(ctx, store.HarvestStatsOptions{
		AccountIDs: accountIDs,
		UserID:     userID,
		RunGap:     runGap,
	})
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(harvestStatsProto(stats, req.Msg.GetLimitItems())), nil
}

func harvestStatsProto(stats *store.HarvestStats, limitItems int32) *pb.GetHarvestStatsResponse {
	resp := &pb.GetHarvestStatsResponse{
		RunGapSeconds: int32(stats.RunGap.Seconds()),
		HarvestOps:    int32(stats.HarvestOps),
	}
	if !stats.WindowStart.IsZero() {
		resp.WindowStart = timestamppb.New(stats.WindowStart)
	}
	if !stats.WindowEnd.IsZero() {
		resp.WindowEnd = timestamppb.New(stats.WindowEnd)
	}
	for _, account := range stats.Accounts {
		out := &pb.AccountHarvestStats{
			AccountId:   fmt.Sprintf("%d", account.AccountID),
			AccountName: account.AccountName,
			HarvestOps:  int32(account.HarvestOps),
		}
		if !account.FirstHarvestAt.IsZero() {
			out.FirstHarvestAt = timestamppb.New(account.FirstHarvestAt)
		}
		if !account.LastHarvestAt.IsZero() {
			out.LastHarvestAt = timestamppb.New(account.LastHarvestAt)
		}
		items := append([]store.HarvestItemTotal(nil), account.Items...)
		for _, item := range items {
			switch harvestItemCategory(item.ItemID) {
			case "experience":
				out.ExperienceTotal += item.Count
			case "flower":
				out.FlowerTotal += item.Count
			case "essence":
				out.EssenceTotal += item.Count
			default:
				out.OtherTotal += item.Count
			}
		}
		sort.Slice(items, func(i, j int) bool {
			ci, cj := harvestItemCategory(items[i].ItemID), harvestItemCategory(items[j].ItemID)
			if rankHarvestCategory(ci) != rankHarvestCategory(cj) {
				return rankHarvestCategory(ci) < rankHarvestCategory(cj)
			}
			if items[i].Count != items[j].Count {
				return items[i].Count > items[j].Count
			}
			return items[i].ItemID < items[j].ItemID
		})
		if limitItems > 0 && len(items) > int(limitItems) {
			items = items[:limitItems]
		}
		for _, item := range items {
			category := harvestItemCategory(item.ItemID)
			name := state.ItemName(item.ItemID)
			if name == "" {
				name = fmt.Sprintf("#%d", item.ItemID)
			}
			out.Items = append(out.Items, &pb.HarvestItemTotal{
				ItemId:   item.ItemID,
				ItemName: name,
				Count:    item.Count,
				Category: category,
			})
		}
		resp.Accounts = append(resp.Accounts, out)
	}
	return resp
}

func harvestItemCategory(itemID int32) string {
	switch {
	case itemID == 2:
		return "experience"
	case state.IsFlowerItemID(itemID):
		return "flower"
	case itemID >= 22000 && itemID < 23000:
		return "essence"
	default:
		return "item"
	}
}

func rankHarvestCategory(category string) int {
	switch category {
	case "experience":
		return 0
	case "flower":
		return 1
	case "essence":
		return 2
	default:
		return 3
	}
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

func buildLandViews(lands map[int32]state.LandView, farmLands []state.FarmLandInfo, rosterObserved bool, farmLandObserved bool, level int32, now time.Time) []*pb.LandView {
	out := make([]*pb.LandView, 0, len(lands))
	seen := make(map[int32]struct{}, len(lands))
	for _, info := range farmLands {
		l, observed := lands[info.ID]
		out = append(out, landViewProto(info.ID, l, info, observed, rosterObserved, farmLandObserved, level, now))
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
		out = append(out, landViewProto(id, lands[id], state.FarmLandInfo{}, true, rosterObserved, farmLandObserved, level, now))
	}
	return out
}

func landViewProto(id int32, l state.LandView, info state.FarmLandInfo, observed bool, rosterObserved bool, farmLandObserved bool, level int32, now time.Time) *pb.LandView {
	kind, reason := "unknown", "等待服务端土地清单"
	status := "locked"
	if observed {
		kind, reason = automation.Recommend(l, now)
		status = "opened"
	} else if !farmLandObserved {
		kind, reason = "unknown", "等待当前客户端土地配置"
	} else if rosterObserved {
		status = "unopened"
		if info.OpenLevel > 0 && level < info.OpenLevel {
			kind, reason = "locked", "等级未达到开放条件"
			status = "locked"
		} else if len(info.Cost) >= 2 {
			kind, reason = "unlock", "可开垦"
		} else {
			kind, reason = "unknown", "缺少开垦消耗配置"
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
		UnlockCost:     append([]int32(nil), info.Cost...),
		Wasteland:      append([]int32(nil), info.Wasteland...),
	}
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
