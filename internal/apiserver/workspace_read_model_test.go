package apiserver

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestWorkspaceReadySourcesCapabilitiesAndStatuses(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	svc := &Services{DB: db}

	if len(featureCapabilitiesProto()) == 0 {
		t.Fatal("feature capabilities are missing")
	}
	statuses, err := svc.accountStatuses(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 0 {
		t.Fatal("empty database unexpectedly returned account statuses")
	}
}

func TestWorkspaceContractUsesBusinessDomainReadModels(t *testing.T) {
	if services := pb.File_mygardenworld_v1_workspace_proto.Services(); services.Len() != 0 {
		t.Fatalf("workspace protobuf unexpectedly declares %d RPC services", services.Len())
	}
	stateFields := (&pb.WorkspaceState{}).ProtoReflect().Descriptor().Fields()
	for _, name := range []protoreflect.Name{"basic", "garden", "orders", "union", "activities", "warehouse", "statistics"} {
		if stateFields.ByName(name) == nil {
			t.Fatalf("WorkspaceState domain field %s is missing", name)
		}
	}

	orderFields := (&pb.OrdersView{}).ProtoReflect().Descriptor().Fields()
	if orderFields.ByName("inventory_ledger") != nil {
		t.Fatal("OrdersView still owns the asset inventory ledger")
	}
	assetLedger := (&pb.WarehouseView{}).ProtoReflect().Descriptor().Fields().ByName("inventory_ledger")
	if assetLedger == nil || assetLedger.Number() != 5 {
		t.Fatalf("WarehouseView inventory_ledger=%v, want field 5", assetLedger)
	}
	unionFields := (&pb.UnionView{}).ProtoReflect().Descriptor().Fields()
	for _, name := range []protoreflect.Name{"membership_observed", "in_union", "union_id", "member_position_observed", "member_position", "member_position_label", "race_delete_allowed", "race", "lands"} {
		if unionFields.ByName(name) == nil {
			t.Fatalf("UnionView membership-gated field %s is missing", name)
		}
	}
}

func TestBuildPearlHireStatusViewExposesDailyUsageAndSlots(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	st := state.New()
	st.ApplyV([]byte(`{"7":{"0":{"32":{"1003":4}}},"115":{"0":{"1":{"2":2001,"3":1800000060000,"4":0,"9":1800000000001},"2":{"2":0,"3":null,"4":0,"9":1800000000001}}}}`))
	st.ApplyV([]byte(`{"28":{"5":[{"0":2001,"1":"劳工甲","4":12}]}}`))
	st.SetPearlHireTicketUsed(state.PearlHireTicketDayID(now), 2)
	got := buildPearlHireStatusView(st, &pb.PearlPolicy{DailyHireTicketLimit: 3}, now)
	if got.GetTicketCount() != 4 || got.GetTicketUsedToday() != 2 || got.GetDailyTicketLimit() != 3 {
		t.Fatalf("pearl status counts=%+v", got)
	}
	if len(got.GetSlots()) != 2 || got.GetSlots()[0].GetPlaceId() != 1 || !got.GetSlots()[0].GetActive() || got.GetSlots()[0].GetLaborName() != "劳工甲" {
		t.Fatalf("pearl slots=%+v", got.GetSlots())
	}
	if got.GetSlots()[1].GetPlaceId() != 2 || got.GetSlots()[1].GetActive() || got.GetSlots()[1].GetLaborUid() != 0 {
		t.Fatalf("empty pearl slot=%+v", got.GetSlots()[1])
	}
}

func TestVideoActionReadModelsKeepDomainOwnershipAndState(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, loc)
	st := state.New()
	st.ApplyVMap(map[string]any{
		"31": map[string]any{"0": map[string]any{
			"10": map[string]any{"1": 10, "2": 1, "6": now.UnixMilli()},
			"14": map[string]any{"1": 14, "2": 1, "6": now.UnixMilli()},
		}},
		"112": map[string]any{
			"1": map[string]any{"1": 1},
			"5": map[string]any{"1": now.Add(-time.Hour).UnixMilli()},
			"6": now.UnixMilli(),
		},
		"118": map[string]any{"1": 2, "2": now.Add(90 * time.Minute).UnixMilli()},
		"105": map[string]any{"0": map[string]any{
			"1": map[string]any{
				"2": map[string]any{"0": 81, "1": 9, "3": 1, "7": map[string]any{"11": 1200}},
			},
			"6": map[string]any{"4": 1, "5": map[string]any{"7": 30}},
		}},
		"25": map[string]any{"133": map[string]any{"4": now.UnixMilli(), "5": map[string]any{"1": 1}}},
	})

	basic := buildBasicVideoActions(st, now)
	if len(basic) != 3 || basic[0].GetState() != pb.VideoActionState_VIDEO_ACTION_STATE_ACTIVE || basic[0].GetExpiresAtMs() != now.Add(90*time.Minute).UnixMilli() {
		t.Fatalf("basic video actions=%+v", basic)
	}
	if basic[1].GetState() != pb.VideoActionState_VIDEO_ACTION_STATE_COOLDOWN || basic[1].GetUsed() != 1 || len(basic[1].GetRewards()) != 1 || basic[1].GetRewards()[0].GetCount() != 6 {
		t.Fatalf("giftbag action=%+v", basic[1])
	}
	if basic[2].GetState() != pb.VideoActionState_VIDEO_ACTION_STATE_READY || basic[2].GetUsed() != 1 || basic[2].GetLimit() != 2 {
		t.Fatalf("pearl action=%+v", basic[2])
	}

	orders := buildResidentVideoOrders(st, now)
	if len(orders) != 2 || orders[0].GetLabel() != "居民订单 #2" || orders[1].GetLabel() != "绸缎订单" || orders[0].GetRewards()[0].GetItemId() != 11 {
		t.Fatalf("resident video actions=%+v", orders)
	}
	build := buildFmlVideoBuildStatus(st, now)
	if build.GetState() != pb.VideoActionState_VIDEO_ACTION_STATE_EXHAUSTED || build.GetUsed() != 1 || build.GetLimit() != 1 || len(build.GetRewards()) != 1 || build.GetAvailableAtMs() != state.NextCalendarDayReset(now).UnixMilli() {
		t.Fatalf("guild video build=%+v", build)
	}
}

func TestFeatureCapabilitiesExposeRaceUpgradeAsExecutable(t *testing.T) {
	capabilities := featureCapabilitiesProto()
	seen := make(map[string]struct{}, len(capabilities))
	foundRaceUpgrade := false
	for _, capability := range capabilities {
		if _, exists := seen[capability.GetId()]; exists {
			t.Fatalf("duplicate feature capability id %q", capability.GetId())
		}
		seen[capability.GetId()] = struct{}{}
		if capability.GetId() != "union.race.upgrade" {
			continue
		}
		if capability.GetStatus() != pb.PlanStatus_PLAN_STATUS_MANAGED ||
			!capability.GetExecutable() ||
			capability.GetSyncOnly() ||
			len(capability.GetBlockedReasons()) != 0 {
			t.Fatalf("race upgrade capability=%+v, want managed executable", capability)
		}
		foundRaceUpgrade = true
	}
	if !foundRaceUpgrade {
		t.Fatal("race upgrade capability is missing")
	}
}

func TestBuildFmlLandViewsExposesPlantingInfo(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	lands := map[int32]state.FmlLandView{
		2: {LandID: 2, Level: 0, FlowerID: 0},
		1: {
			LandID:      1,
			Level:       0,
			FlowerID:    23001,
			StartTimeMs: now.Add(-45 * time.Minute).UnixMilli(), // 2700s / 900s = 3
		},
	}
	cultivations := map[int32]state.CultivateView{
		23001: {FlowerID: 23001, Lvl: 7},
	}
	got := buildFmlLandViews(lands, cultivations, now)
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
	if got[0].GetLandId() != 1 || got[1].GetLandId() != 2 {
		t.Fatalf("order=%d,%d want 1,2", got[0].GetLandId(), got[1].GetLandId())
	}
	if got[0].GetRecommendation() != "harvest" || got[0].GetPendingHarvest() != 3 {
		t.Fatalf("land 1 = %q pending=%d, want harvest pending=3", got[0].GetRecommendation(), got[0].GetPendingHarvest())
	}
	if got[0].GetTimeSec() <= 0 || got[0].GetStockCap() <= 0 {
		t.Fatalf("land 1 missing c_fmlLandLvl metadata time=%d stock=%d", got[0].GetTimeSec(), got[0].GetStockCap())
	}
	if got[0].GetFlowerLvl() != 7 {
		t.Fatalf("land 1 flower_lvl=%d, want 7", got[0].GetFlowerLvl())
	}
	if got[1].GetRecommendation() != "plant" || got[1].GetFlowerId() != 0 || got[1].GetFlowerLvl() != 0 {
		t.Fatalf("land 2 = %q flower=%d lvl=%d, want plant empty", got[1].GetRecommendation(), got[1].GetFlowerId(), got[1].GetFlowerLvl())
	}
}

func TestBuildLandViewsUsesServerRosterForOpenedStatus(t *testing.T) {
	lands := map[int32]state.LandView{
		1001: {Observed: true, FlowerID: 23001, State: 1},
	}
	farmLands := []state.FarmLandInfo{
		{ID: 1001, OpenLevel: 1, Cost: []int32{33, 800}},
		{ID: 1002, OpenLevel: 1, Cost: []int32{34, 800}},
		{ID: 1058, OpenLevel: 30, Cost: []int32{35, 800}},
	}

	got := buildLandViews(lands, farmLands, true, true, 1, time.Unix(0, 0), 0)
	if len(got) != 3 {
		t.Fatalf("expected runtime land roster, got %d", len(got))
	}

	byID := make(map[int32]string, len(got))
	observed := make(map[int32]bool, len(got))
	for _, land := range got {
		byID[land.GetLandId()] = land.GetLandStatus()
		observed[land.GetLandId()] = land.GetObserved()
	}

	if byID[1001] != "opened" || !observed[1001] {
		t.Fatalf("land 1001 = status %q observed %v, want opened true", byID[1001], observed[1001])
	}
	if byID[1002] != "unopened" || observed[1002] {
		t.Fatalf("land 1002 = status %q observed %v, want unopened false", byID[1002], observed[1002])
	}
	if byID[1058] != "unopened" || observed[1058] {
		t.Fatalf("land 1058 = status %q observed %v, want unopened false", byID[1058], observed[1058])
	}
}

func TestBuildLandViewsDoesNotInventNextFourWastelands(t *testing.T) {
	lands := map[int32]state.LandView{}
	for id := int32(1001); id <= 1024; id++ {
		lands[id] = state.LandView{Observed: true}
	}

	farmLands := []state.FarmLandInfo{{ID: 1025, OpenLevel: 42, Cost: []int32{37, 1526}}}
	got := buildLandViews(lands, farmLands, true, true, 13, time.Unix(0, 0), 0)
	for _, land := range got {
		if land.GetLandId() == 1025 {
			if land.GetLandStatus() != "unopened" {
				t.Fatalf("land 1025 status=%q, want unopened (openLevel is not a hard lock)", land.GetLandStatus())
			}
			return
		}
	}
	t.Fatal("land 1025 not found")
}

func TestBuildLandViewsDoesNotMarkStaticRowsBeforeRoster(t *testing.T) {
	got := buildLandViews(map[int32]state.LandView{}, []state.FarmLandInfo{{ID: 1001, OpenLevel: 1, Cost: []int32{33, 800}}}, false, true, 1, time.Unix(0, 0), 0)
	for _, land := range got {
		if land.GetLandId() == 1001 {
			if land.GetLandStatus() != "locked" || land.GetReason() != "等待服务端土地清单" {
				t.Fatalf("land 1001=(%q,%q), want locked waiting for roster", land.GetLandStatus(), land.GetReason())
			}
			return
		}
	}
	t.Fatal("land 1001 not found")
}

func TestBuildLandViewsDoesNotUseStaticLandConfigUntilRuntimeObserved(t *testing.T) {
	got := buildLandViews(map[int32]state.LandView{}, nil, true, false, 13, time.Unix(0, 0), 0)
	if len(got) != 0 {
		t.Fatalf("got %d land rows, want no synthetic static rows before runtime config", len(got))
	}
}

func TestBuildPendingTasksGroupsTrackedTaskSources(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001":  2,
			"300208": 1,
		}, "34": 14}},
		"105": map[string]any{
			"0": map[string]any{"1": map[string]any{
				"2": map[string]any{"2": [][]int32{{23001, 5}}},
			}},
		},
		"109": map[string]any{
			"0": map[string]any{"1": map[string]any{
				"1": map[string]any{"0": 2, "1": 300208, "2": 3, "3": 1},
			}},
		},
		"22": map[string]any{
			"0": map[string]any{"1": 10001, "2": 1},
			"1": map[string]any{
				"3": map[string]any{"102": 1},
				"100": map[string]any{
					"101": map[string]any{"0": 101, "1": 5, "2": 2, "4": 0},
					"102": map[string]any{"0": 102, "1": 1, "2": 1, "4": 1},
				},
			},
			"100": map[string]any{
				"1": map[string]any{"3026": 163},
				"3": map[string]any{},
			},
		},
		"119": map[string]any{"3": map[string]any{"20001": 1, "20002": 1, "20003": 1}},
		"129": map[string]any{"0": map[string]any{"1": map[string]any{
			"6002": map[string]any{"0": 6002, "1": 0, "2": 60020601},
		}}},
	})

	tasks := buildPendingTasks(st)
	byCategory := map[string]int{}
	statusByCategory := map[string]pb.PlanStatus{}
	for _, task := range tasks {
		byCategory[task.GetCategory()]++
		statusByCategory[task.GetCategory()] = task.GetStatus()
	}
	if byCategory["居民订单"] != 1 || byCategory["顾客订单"] != 1 || byCategory["主线任务"] != 1 || byCategory["日常任务"] != 1 || byCategory["周常任务"] == 0 || byCategory["地图随机事件"] != 1 {
		t.Fatalf("task categories = %+v, want tracked task/order categories", byCategory)
	}
	if statusByCategory["居民订单"] != pb.PlanStatus_PLAN_STATUS_MANAGED {
		t.Fatalf("resident task status=%s, want MANAGED", statusByCategory["居民订单"])
	}
	if statusByCategory["地图随机事件"] != pb.PlanStatus_PLAN_STATUS_READY {
		t.Fatalf("random event status=%s, want READY", statusByCategory["地图随机事件"])
	}

	var flowerReq, artReq, recipeReq *pb.RequirementView
	for _, task := range tasks {
		for _, req := range task.GetRequirements() {
			switch req.GetItemId() {
			case 23001:
				flowerReq = req
			case 300208:
				artReq = req
			case 23005:
				recipeReq = req
			}
		}
	}
	if flowerReq == nil || !flowerReq.GetPlantingRelevant() || flowerReq.GetMissing() != 2 {
		t.Fatalf("flower requirement = %+v, want planting-relevant missing 2", flowerReq)
	}
	if artReq != nil {
		t.Fatalf("art requirement = %+v, want recipe output hidden while crafting materials are missing", artReq)
	}
	if recipeReq == nil || !recipeReq.GetPlantingRelevant() || recipeReq.GetMissing() != 2 {
		t.Fatalf("recipe requirement = %+v, want planting-relevant missing 2", recipeReq)
	}
}

func TestBuildPendingTasksUsesMainTaskCatalogTargetAndHidesTerminal(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{"22": map[string]any{"0": map[string]any{
		"1": 920001, "2": 12, "4": map[string]any{},
	}}})
	var main *pb.PendingTaskView
	for _, task := range buildPendingTasks(st) {
		if task.GetCategory() == "主线任务" {
			main = task
			break
		}
	}
	if main == nil || main.GetId() != "920001" || main.GetFinished() != 12 || main.GetTarget() != 24 {
		t.Fatalf("main pending task=%+v", main)
	}

	st.ApplyVMap(map[string]any{"22": map[string]any{"0": map[string]any{"1": 6950001}}})
	for _, task := range buildPendingTasks(st) {
		if task.GetCategory() == "主线任务" {
			t.Fatalf("terminal main task still visible: %+v", task)
		}
	}
}

func TestBuildPendingTasksMainTaskReadinessUsesServerProgress(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"23001": 0}}},
		"22": map[string]any{"0": map[string]any{
			"1": 10001, "2": 4, "4": map[string]any{},
		}},
	})
	var main *pb.PendingTaskView
	for _, task := range buildPendingTasks(st) {
		if task.GetCategory() == "主线任务" {
			main = task
			break
		}
	}
	if main == nil || main.GetStatus() != pb.PlanStatus_PLAN_STATUS_READY ||
		main.GetFinished() != 4 || main.GetTarget() != 4 {
		t.Fatalf("ready main pending task=%+v", main)
	}
	if len(main.GetRequirements()) != 1 || main.GetRequirements()[0].GetMissing() != 4 {
		t.Fatalf("main requirements=%+v, want display-only flower shortage", main.GetRequirements())
	}

	unknown := state.New()
	unknown.ApplyVMap(map[string]any{"22": map[string]any{"0": map[string]any{
		"1": 10001, "4": map[string]any{},
	}}})
	for _, task := range buildPendingTasks(unknown) {
		if task.GetCategory() == "主线任务" && task.GetStatus() != pb.PlanStatus_PLAN_STATUS_BLOCKED {
			t.Fatalf("unknown-progress main status=%s", task.GetStatus())
		}
	}
}

func TestBuildPendingTasksCultivateMainTaskUsesMaterialRequirements(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"1401": 3, "1410": 3, "1422": 2, "1437": 1,
		}}},
		"22": map[string]any{"0": map[string]any{"1": 40001, "2": 0, "4": map[string]any{}}},
	})
	var main *pb.PendingTaskView
	for _, task := range buildPendingTasks(st) {
		if task.GetCategory() == "主线任务" {
			main = task
			break
		}
	}
	if main == nil || main.GetId() != "40001" ||
		main.GetExecutionFeature() != pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_CULTIVATION ||
		!main.GetAutoCompletionSupported() || len(main.GetRequirements()) != 4 {
		t.Fatalf("cultivate main task=%+v", main)
	}
	for _, requirement := range main.GetRequirements() {
		if requirement.GetItemId() == 23003 || requirement.GetPlantingRelevant() {
			t.Fatalf("cultivation task leaked flower inventory demand: %+v", requirement)
		}
	}
}

func TestBuildPendingTasksCultivateMainTaskShowsActiveCultivation(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{
		"22": map[string]any{"0": map[string]any{"1": 40001, "2": 0, "4": map[string]any{}}},
		"101": map[string]any{"0": map[string]any{"23003": map[string]any{
			"2": 0, "3": int64(1787814000000), "4": 1,
		}}},
	})
	var main *pb.PendingTaskView
	for _, task := range buildPendingTasks(st) {
		if task.GetCategory() == "主线任务" {
			main = task
			break
		}
	}
	if main == nil || main.GetCooldownUntilMs() != 1787814000000 || main.GetCooldownReason() != "铃兰培育中" || len(main.GetRequirements()) != 0 {
		t.Fatalf("active cultivate main task=%+v", main)
	}
}

func TestBuildPendingTasksCultivateMainTaskShowsCompletedCultivationWaitingForSync(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{
		"22": map[string]any{"0": map[string]any{"1": 40001, "2": 0, "4": map[string]any{}}},
		"101": map[string]any{"0": map[string]any{"23003": map[string]any{
			"2": 1, "4": 2,
		}}},
	})
	var main *pb.PendingTaskView
	for _, task := range buildPendingTasks(st) {
		if task.GetCategory() == "主线任务" {
			main = task
			break
		}
	}
	if main == nil || main.GetCooldownReason() != "铃兰已培育，等待主线任务进度同步" || len(main.GetRequirements()) != 0 {
		t.Fatalf("completed cultivate main task=%+v", main)
	}
}

func TestBuildPendingTasksExposesDailyExecutionCapability(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{"22": map[string]any{"1": map[string]any{
		"1": map[string]any{"3014": 1, "3025": 0},
		"3": map[string]any{},
		"100": map[string]any{
			"30140001": map[string]any{"0": 30140001, "1": 2, "2": 1},
			"30250001": map[string]any{"0": 30250001, "1": 4, "2": 0},
		},
	}}})
	byID := map[string]*pb.PendingTaskView{}
	for _, task := range buildPendingTasks(st) {
		byID[task.GetId()] = task
	}
	water := byID["30140001"]
	if water == nil || water.GetExecutionFeature() != pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_PLANTING || !water.GetAutoCompletionSupported() {
		t.Fatalf("water daily task=%+v", water)
	}
	video := byID["30250001"]
	if video == nil || video.GetExecutionFeature() != pb.TaskExecutionFeature_TASK_EXECUTION_FEATURE_VIDEO || video.GetAutoCompletionSupported() || video.GetCooldownReason() == "" {
		t.Fatalf("video daily task=%+v", video)
	}
}

func TestBuildPendingTasksMarksResidentOrderCooling(t *testing.T) {
	st := state.New()
	now := time.Date(2026, 7, 5, 16, 0, 0, 0, time.UTC)
	cooldownUntil := now.Add(42 * time.Second).UnixMilli()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23014": 144,
			"23003": 157,
		}}},
		"105": map[string]any{
			"0": map[string]any{"1": map[string]any{
				"3": map[string]any{"2": [][]int32{{23014, 1}, {23003, 8}}, "4": cooldownUntil, "5": now.UnixMilli()},
			}},
		},
	})

	task := residentOrderTask(t, buildPendingTasksAt(st, now))
	if task.GetStatus() != pb.PlanStatus_PLAN_STATUS_MANAGED {
		t.Fatalf("status before cooldown=%s, want MANAGED", task.GetStatus())
	}
	if task.GetCooldownUntilMs() != cooldownUntil || task.GetCooldownReason() == "" {
		t.Fatalf("cooldown=(%d,%q), want populated", task.GetCooldownUntilMs(), task.GetCooldownReason())
	}

	task = residentOrderTask(t, buildPendingTasksAt(st, now.Add(42*time.Second)))
	if task.GetStatus() != pb.PlanStatus_PLAN_STATUS_READY {
		t.Fatalf("status after cooldown=%s, want READY", task.GetStatus())
	}
	if task.GetCooldownUntilMs() != 0 || task.GetCooldownReason() != "" {
		t.Fatalf("cooldown after ready=(%d,%q), want empty", task.GetCooldownUntilMs(), task.GetCooldownReason())
	}
}

func TestBuildPendingTasksDoesNotInferZooEventsFromPetFields(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{"33": map[string]any{
		"0": map[string]any{"0": 1, "3": []int32{1}},
		"1": map[string]any{"1": map[string]any{"1": 1, "9": 4001, "10": []int32{2096}}},
	}})

	for _, task := range buildPendingTasks(st) {
		if task.GetCategory() == "宠物事件" {
			t.Fatalf("old pet-field event leaked into pending tasks: %+v", task)
		}
	}
}

func TestBuildPendingTasksUsesZooLogPetAndIndexIDs(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{"33": map[string]any{
		"1": map[string]any{"7": map[string]any{"1": 7, "19": int64(1000)}},
		"2": map[string]any{
			"7|41": map[string]any{"1": 7, "2": 41, "5": 2001, "7": 1, "13": int64(1500)},
			"7|42": map[string]any{"1": 7, "2": 42, "5": 2096, "6": 2, "7": 0, "8": map[string]any{}, "9": map[string]any{}, "10": map[string]any{}, "11": map[string]any{}, "13": int64(2000)},
			"7|43": map[string]any{"1": 7, "2": 43, "5": 2096, "6": 2, "7": 0, "8": map[string]any{}, "9": map[string]any{"11": 1}, "10": map[string]any{}, "11": map[string]any{}, "13": int64(1900)},
		},
	}})

	statuses := map[string]pb.PlanStatus{}
	for _, task := range buildPendingTasks(st) {
		if task.GetCategory() == "宠物事件" {
			statuses[task.GetId()] = task.GetStatus()
		}
	}
	if statuses["7:42"] != pb.PlanStatus_PLAN_STATUS_BLOCKED || statuses["7:41"] != pb.PlanStatus_PLAN_STATUS_READY || statuses["7:43"] != pb.PlanStatus_PLAN_STATUS_BLOCKED {
		t.Fatalf("zoo pending task statuses=%+v", statuses)
	}
	if _, exists := statuses["7:2096"]; exists {
		t.Fatalf("pending task used eventId instead of log idx: %+v", statuses)
	}
}

func TestBuildPendingTasksAggregatesZooCollectionFailure(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{"33": map[string]any{"2": map[string]any{
		"first":  map[string]any{"1": 7, "2": 41, "5": 2096, "7": 1, "13": int64(1500)},
		"second": map[string]any{"1": 7, "2": 42, "5": 2096, "7": 1, "13": int64(1600)},
	}}})
	st.ApplyVMap(map[string]any{"33": map[string]any{"2": []any{"invalid"}}})

	var zooTasks []*pb.PendingTaskView
	for _, task := range buildPendingTasks(st) {
		if task.GetCategory() == "宠物事件" {
			zooTasks = append(zooTasks, task)
		}
	}
	if len(zooTasks) != 1 || zooTasks[0].GetId() != "sync" || zooTasks[0].GetStatus() != pb.PlanStatus_PLAN_STATUS_BLOCKED || !strings.Contains(zooTasks[0].GetTitle(), "日志集合") {
		t.Fatalf("zoo collection tasks=%+v", zooTasks)
	}
}

func TestBuildPendingTasksShowsZooSouvenirRewardAndUnreadState(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{"33": map[string]any{
		"0": map[string]any{"13": []int32{1}},
		"4": map[string]any{
			"30201": map[string]any{"1": 30201, "2": 1},
			"32901": map[string]any{"1": 32901, "2": 0},
		},
	}})

	tasks := map[string]*pb.PendingTaskView{}
	for _, task := range buildPendingTasks(st) {
		if task.GetCategory() == "宠物纪念品" {
			tasks[task.GetId()] = task
		}
	}
	reward := tasks["reward:2"]
	if reward == nil || reward.GetStatus() != pb.PlanStatus_PLAN_STATUS_READY || reward.GetFinished() != 2 || reward.GetTarget() != 2 {
		t.Fatalf("souvenir reward pending task=%+v", reward)
	}
	unread := tasks["unread:32901"]
	if unread == nil || unread.GetStatus() != pb.PlanStatus_PLAN_STATUS_BLOCKED || unread.GetTitle() == "" {
		t.Fatalf("souvenir unread pending task=%+v", unread)
	}

	st.ApplyVMap(map[string]any{"33": map[string]any{"0": map[string]any{"13": []int32{1, 2}}}})
	foundUnread := false
	for _, task := range buildPendingTasks(st) {
		if task.GetCategory() == "宠物纪念品" && task.GetId() == "unread:32901" {
			foundUnread = true
			if task.GetStatus() != pb.PlanStatus_PLAN_STATUS_READY {
				t.Fatalf("souvenir unread task stayed blocked after rewards were claimed: %+v", task)
			}
		}
	}
	if !foundUnread {
		t.Fatal("souvenir unread task disappeared after rewards were claimed")
	}
}

func TestCyclicNoteProtoClampsDisplayButUsesRawProgressForReadiness(t *testing.T) {
	view := state.CyclicNoteView{
		Observed:                  true,
		Found:                     true,
		Valid:                     true,
		BatchID:                   1309,
		TmpID:                     40020007,
		TmpType:                   4002,
		Phase:                     2,
		Score:                     130,
		Bag:                       map[int32]int32{1107: 9, 11: 2000},
		CurrencyItemID:            1107,
		CurrencyBalance:           9,
		TaskListObserved:          true,
		MilestoneReceiptsObserved: true,
		Tasks: []state.CyclicNoteTaskSlotView{
			{SlotID: 1, Unlocked: true, TaskID: 4003, TaskType: 3001, Target: 80, Progress: 81, ProgressObserved: true, ReceiptObserved: true, CatalogKnown: true},
			{SlotID: 2, Unlocked: true, TaskID: 9999, Target: 20, Progress: 20, ProgressObserved: true, ReceiptObserved: true},
			{SlotID: 3, Unlocked: true, TaskID: 2001, TaskType: 3015, Target: 135, Progress: 70, ProgressObserved: true, ReceiptObserved: true, CatalogKnown: true},
		},
		Milestones: []state.CyclicNoteMilestoneView{
			{Index: 1, Target: 120, Reward: []state.ItemCount{{ItemID: 1, Count: 200}}},
			{Index: 2, Target: 60, Received: true},
		},
	}

	got := cyclicNoteProto(view)
	if !got.GetObserved() || !got.GetFound() || !got.GetValid() || got.GetBatchId() != 1309 {
		t.Fatalf("activity identity=%+v", got)
	}
	if len(got.GetItems()) != 2 || got.GetItems()[0].GetItemId() != 11 || got.GetItems()[1].GetItemId() != 1107 {
		t.Fatalf("activity items=%+v, want stable item-id order", got.GetItems())
	}
	ready := got.GetTasks()[0]
	if ready.GetProgress() != 80 || ready.GetRawProgress() != 81 || ready.GetStatus() != pb.PlanStatus_PLAN_STATUS_READY {
		t.Fatalf("overshot task=%+v, want display 80 raw 81 READY", ready)
	}
	if got.GetTasks()[1].GetStatus() != pb.PlanStatus_PLAN_STATUS_BLOCKED {
		t.Fatalf("unknown task status=%s, want BLOCKED", got.GetTasks()[1].GetStatus())
	}
	if got.GetTasks()[2].GetStatus() != pb.PlanStatus_PLAN_STATUS_SYNC_ONLY {
		t.Fatalf("unfinished task status=%s, want SYNC_ONLY", got.GetTasks()[2].GetStatus())
	}
	milestone := got.GetMilestones()[0]
	if milestone.GetProgress() != 120 || milestone.GetRawProgress() != 130 || !milestone.GetReady() || milestone.GetStatus() != pb.PlanStatus_PLAN_STATUS_READY {
		t.Fatalf("ready milestone=%+v, want display 120 raw 130 READY", milestone)
	}
	if got.GetMilestones()[1].GetReady() || got.GetMilestones()[1].GetStatus() != pb.PlanStatus_PLAN_STATUS_SKIPPED {
		t.Fatalf("received milestone=%+v, want not-ready SKIPPED", got.GetMilestones()[1])
	}

	view.Valid = false
	invalid := cyclicNoteProto(view)
	if invalid.GetTasks()[0].GetStatus() != pb.PlanStatus_PLAN_STATUS_BLOCKED || invalid.GetMilestones()[0].GetReady() {
		t.Fatalf("invalid activity exposed ready work: tasks=%+v milestones=%+v", invalid.GetTasks(), invalid.GetMilestones())
	}

	view.Valid = true
	view.Phase = 3
	grace := cyclicNoteProto(view)
	if grace.GetTasks()[0].GetStatus() == pb.PlanStatus_PLAN_STATUS_READY || !grace.GetMilestones()[0].GetReady() || grace.GetMilestones()[0].GetStatus() != pb.PlanStatus_PLAN_STATUS_READY {
		t.Fatalf("reward phase readiness mismatch: tasks=%+v milestones=%+v", grace.GetTasks(), grace.GetMilestones())
	}

	view.Phase = 4
	ended := cyclicNoteProto(view)
	if ended.GetTasks()[0].GetStatus() == pb.PlanStatus_PLAN_STATUS_READY || ended.GetMilestones()[0].GetReady() || ended.GetMilestones()[0].GetStatus() == pb.PlanStatus_PLAN_STATUS_READY {
		t.Fatalf("ended activity exposed ready work: tasks=%+v milestones=%+v", ended.GetTasks(), ended.GetMilestones())
	}

	view.Phase = 2
	view.MilestoneReceiptsObserved = false
	unknownReceipts := cyclicNoteProto(view)
	if unknownReceipts.GetMilestones()[0].GetReady() || unknownReceipts.GetMilestones()[0].GetStatus() == pb.PlanStatus_PLAN_STATUS_READY {
		t.Fatalf("unobserved milestone receipts exposed ready work: %+v", unknownReceipts.GetMilestones()[0])
	}
}

func TestCyclicNotePendingTasksUseMachineCategoryCompositeIDAndActivePhase(t *testing.T) {
	view := state.CyclicNoteView{
		Found:   true,
		Valid:   true,
		BatchID: 1309,
		Phase:   2,
		Tasks: []state.CyclicNoteTaskSlotView{
			{SlotID: 1, Unlocked: true, TaskID: 2007, Title: "完成顾客订单25次", Target: 25, Progress: 29, ProgressObserved: true, ReceiptObserved: true, CatalogKnown: true},
			{SlotID: 2, Unlocked: true, TaskID: 9999, Title: "未知任务", Target: 5, ProgressObserved: true, ReceiptObserved: true},
			{SlotID: 3, Unlocked: false},
			{SlotID: 4, Unlocked: true, TaskID: 1006, Received: true, Target: 3, Progress: 3, ProgressObserved: true, ReceiptObserved: true, CatalogKnown: true},
		},
	}

	got := cyclicNotePendingTasksFromView(view)
	if len(got) != 2 {
		t.Fatalf("pending tasks=%+v, want unlocked unreceived slots only", got)
	}
	if got[0].GetCategory() != "activity" || got[0].GetId() != "1309:1:2007" || got[0].GetFinished() != 25 || got[0].GetStatus() != pb.PlanStatus_PLAN_STATUS_READY {
		t.Fatalf("ready pending task=%+v", got[0])
	}
	if got[1].GetId() != "1309:2:9999" || got[1].GetStatus() != pb.PlanStatus_PLAN_STATUS_BLOCKED {
		t.Fatalf("unknown pending task=%+v, want BLOCKED", got[1])
	}

	view.Phase = 3
	if grace := cyclicNotePendingTasksFromView(view); len(grace) != 0 {
		t.Fatalf("phase 3 pending tasks=%+v, want none", grace)
	}
}

func TestBusinessStatisticsProtoExposesDailyHistory(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{
		"124": map[string]any{"0": map[string]any{
			"20260817": map[string]any{"1": 20260817, "2": 100, "7": 12, "9": 3},
			"20260818": map[string]any{"1": 20260818, "2": 1540590, "3": 623894, "7": 410, "8": 9, "9": 846},
		}},
	})

	got := businessStatisticsProto(st)
	if !got.GetObserved() || got.GetToday().GetDayId() != 20260818 || got.GetToday().GetGold() != 1540590 {
		t.Fatalf("today business statistics=%+v", got)
	}
	if got.GetToday().GetExperience() != 623894 || got.GetToday().GetFlowerHarvestNum() != 410 || got.GetToday().GetResidentNormalFinished() != 846 {
		t.Fatalf("today counters=%+v", got.GetToday())
	}
	if len(got.GetDays()) != 2 || got.GetDays()[1].GetDayId() != 20260817 || got.GetDays()[1].GetGold() != 100 {
		t.Fatalf("business history=%+v", got.GetDays())
	}
	field := (&pb.OrdersView{}).ProtoReflect().Descriptor().Fields().ByName("business_statistics")
	if field == nil || field.Number() != 9 {
		t.Fatalf("OrdersView business_statistics field=%v, want field 9", field)
	}
}

func TestCyclicNoteApplyVFlowsToSnapshotAndPending(t *testing.T) {
	now := time.Date(2026, 7, 11, 3, 0, 0, 0, time.UTC)
	beginMs := now.Add(-time.Hour).UnixMilli()
	endMs := now.Add(24 * time.Hour).UnixMilli()
	st := state.New()
	st.ApplyVMap(map[string]any{
		"23": map[string]any{
			"0": map[string]any{
				"1309": map[string]any{
					"0": 1309, "1": 40020007, "2": 4002, "3": 1,
					"5": beginMs, "6": beginMs, "7": endMs, "8": int64(0), "9": int64(86_400_000),
					"11": 130,
					"12": map[string]any{"1107": 9},
					"14": map[string]any{"105": map[string]any{
						"0": []int32{4003, 0, 1006}, "1": 2, "2": now.Add(-time.Minute).UnixMilli(),
					}},
				},
			},
			"1": map[string]any{
				"40020007": map[string]any{
					"0": 40020007, "1": "花笺集芳7期", "2": "苔绿贝壳花测试批次", "3": 4002,
					"9": []any{
						[]any{1, 60, "1,80;1002,350"},
						[]any{2, 120, "1,200;1001,600"},
						[]any{3, 265, "1,600;21541,1"},
					},
				},
			},
			"3": map[string]any{
				"1309|0": map[string]any{
					"1": 1309, "2": 0,
					"3": map[string]any{"4003": 81, "1006": 2},
					"4": map[string]any{"4003": map[string]any{"2": now.UnixMilli()}},
					"5": map[string]any{},
					"7": now.UnixMilli(),
				},
			},
		},
	})

	view, ok := st.CyclicNoteView(now)
	if !ok || !view.Observed || !view.Found || !view.Valid || view.Phase != 2 {
		t.Fatalf("cyclic-note view identity=%+v ok=%t", view, ok)
	}
	if view.BatchID != 1309 || view.TmpID != 40020007 || view.CurrencyItemID != 1107 || view.CurrencyBalance != 9 || view.FinishCount != 2 {
		t.Fatalf("cyclic-note batch summary=%+v", view)
	}
	if !view.TaskListObserved || !view.TaskRecordObserved || view.MilestoneReceiptsObserved {
		t.Fatalf("observation flags taskList=%t taskRecord=%t milestoneReceipts=%t", view.TaskListObserved, view.TaskRecordObserved, view.MilestoneReceiptsObserved)
	}
	if len(view.Tasks) != 3 || view.Tasks[0].TaskID != 4003 || view.Tasks[0].Progress != 81 || !view.Tasks[0].ProgressObserved || !view.Tasks[0].ReceiptObserved ||
		view.Tasks[1].Unlocked || view.Tasks[2].TaskID != 1006 || view.Tasks[2].Progress != 2 {
		t.Fatalf("cyclic-note task slots=%+v", view.Tasks)
	}
	if len(view.Milestones) != 3 || view.Milestones[0].Index != 1 || view.Milestones[0].Target != 60 || view.Milestones[2].Target != 265 {
		t.Fatalf("cyclic-note milestones=%+v", view.Milestones)
	}

	snapshot := cyclicNoteProto(view)
	if snapshot.GetTasks()[0].GetProgress() != 80 || snapshot.GetTasks()[0].GetRawProgress() != 81 || snapshot.GetTasks()[0].GetStatus() != pb.PlanStatus_PLAN_STATUS_READY {
		t.Fatalf("snapshot overshot task=%+v", snapshot.GetTasks()[0])
	}
	for _, milestone := range snapshot.GetMilestones() {
		if milestone.GetReady() || milestone.GetStatus() == pb.PlanStatus_PLAN_STATUS_READY {
			t.Fatalf("milestone became ready before boxes receipt state was observed: %+v", milestone)
		}
	}

	pending := cyclicNotePendingTasks(st, now)
	if len(pending) != 2 || pending[0].GetCategory() != "activity" || pending[0].GetId() != "1309:1:4003" || pending[0].GetFinished() != 80 || pending[0].GetTarget() != 80 || pending[0].GetStatus() != pb.PlanStatus_PLAN_STATUS_READY ||
		pending[1].GetId() != "1309:3:1006" || pending[1].GetStatus() != pb.PlanStatus_PLAN_STATUS_SYNC_ONLY {
		t.Fatalf("cyclic-note pending tasks=%+v", pending)
	}

	// An observed empty boxes list is authoritative, unlike an absent field.
	st.ApplyVMap(map[string]any{"23": map[string]any{"0": map[string]any{"1309": map[string]any{"13": []int32{}}}}})
	view, ok = st.CyclicNoteView(now)
	if !ok || !view.MilestoneReceiptsObserved {
		t.Fatalf("empty claimed-box list not observed: view=%+v ok=%t", view, ok)
	}
	snapshot = cyclicNoteProto(view)
	if !snapshot.GetMilestoneReceiptsObserved() || !snapshot.GetMilestones()[0].GetReady() || !snapshot.GetMilestones()[1].GetReady() || snapshot.GetMilestones()[2].GetReady() ||
		snapshot.GetMilestones()[0].GetProgress() != 60 || snapshot.GetMilestones()[0].GetRawProgress() != 130 {
		t.Fatalf("milestone snapshot after receipt sync=%+v", snapshot.GetMilestones())
	}

	// recvTaskRwd replaces the entire task list and task-record maps. The old
	// task must disappear from both the domain view and pending-task projection.
	st.ApplyVMap(map[string]any{
		"23": map[string]any{
			"0": map[string]any{"1309": map[string]any{
				"11": 134,
				"12": map[string]any{"1107": 13},
				"14": map[string]any{"105": map[string]any{"0": []int32{2007, 0, 1006}, "1": 3}},
			}},
			"3": map[string]any{"1309|0": map[string]any{
				"3": map[string]any{"1006": 2}, "4": map[string]any{}, "5": map[string]any{}, "7": now.Add(time.Second).UnixMilli(),
			}},
		},
	})
	view, ok = st.CyclicNoteView(now)
	if !ok || len(view.Tasks) != 3 || view.Tasks[0].TaskID != 2007 || view.Tasks[0].Progress != 0 || view.Tasks[2].TaskID != 1006 || view.Tasks[2].Progress != 2 || view.FinishCount != 3 {
		t.Fatalf("replacement cyclic-note view=%+v ok=%t", view, ok)
	}
	for _, task := range view.Tasks {
		if task.TaskID == 4003 {
			t.Fatalf("old task survived task-list replacement: %+v", view.Tasks)
		}
	}
	pending = cyclicNotePendingTasks(st, now)
	if len(pending) != 2 || pending[0].GetId() != "1309:1:2007" || pending[1].GetId() != "1309:3:1006" {
		t.Fatalf("replacement pending tasks=%+v", pending)
	}
	for _, task := range pending {
		if task.GetId() == "1309:1:4003" {
			t.Fatalf("old pending task survived replacement: %+v", pending)
		}
	}
}

func residentOrderTask(t *testing.T, tasks []*pb.PendingTaskView) *pb.PendingTaskView {
	t.Helper()
	for _, task := range tasks {
		if task.GetCategory() == "居民订单" {
			return task
		}
	}
	t.Fatalf("resident order task not found in %+v", tasks)
	return nil
}

func TestDomainStatusUsesPlanStatus(t *testing.T) {
	cases := []struct {
		name      string
		enabled   bool
		observed  bool
		connected bool
		blocked   []string
		want      pb.PlanStatus
	}{
		{name: "ready", enabled: true, observed: true, connected: true, want: pb.PlanStatus_PLAN_STATUS_READY},
		{name: "syncing", enabled: true, observed: false, connected: true, want: pb.PlanStatus_PLAN_STATUS_SYNC_ONLY},
		{name: "blocked", enabled: true, observed: true, connected: true, blocked: []string{"limited"}, want: pb.PlanStatus_PLAN_STATUS_BLOCKED},
		{name: "disabled", enabled: false, observed: true, connected: true, want: pb.PlanStatus_PLAN_STATUS_SKIPPED},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domainStatus("order", "order", tc.enabled, tc.observed, tc.blocked, "", tc.connected)
			if got.GetStatus() != tc.want {
				t.Fatalf("status=%s, want %s", got.GetStatus(), tc.want)
			}
		})
	}
}

func TestPlannedOperationsExposeLaneAndCooldown(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	ops := []automation.PlannedOp{{
		OperationID:    "taskDly.recv|target=40001",
		Kind:           "taskDly.recv",
		Lane:           automation.LaneSide,
		Category:       automation.CategoryBasic,
		Domain:         "basic.task.daily",
		Action:         "claim",
		Executable:     true,
		Status:         automation.PlanStatusReady,
		TargetID:       40001,
		TargetUID:      9002001,
		TargetUIDs:     []int64{9002001, 9002002},
		BatchID:        1309,
		SlotID:         2,
		TaskID:         2007,
		MilestoneIndex: 3,
	}}
	diag := runner.Diagnostics{OperationCooldowns: []runner.OperationCooldownSnapshot{{
		OperationID: "taskDly.recv|target=40001",
		Category:    automation.CategoryBasic,
		Domain:      "basic.task.daily",
		Lane:        automation.LaneSide,
		Reason:      "服务端提示本组任务已经完结",
		Until:       now,
	}}}
	got := plannedOperationsProto(ops, diag)
	if len(got) != 1 {
		t.Fatalf("plannedOperationsProto len=%d, want 1", len(got))
	}
	if got[0].GetLane() != pb.ExecutionLane_EXECUTION_LANE_SIDE {
		t.Fatalf("lane=%s, want SIDE", got[0].GetLane())
	}
	if got[0].GetCooldownUntilMs() != now.UnixMilli() || got[0].GetCooldownReason() == "" {
		t.Fatalf("cooldown=(%d,%q), want populated", got[0].GetCooldownUntilMs(), got[0].GetCooldownReason())
	}
	if got[0].GetTargetUid() != 9002001 || !reflect.DeepEqual(got[0].GetTargetUids(), []int64{9002001, 9002002}) {
		t.Fatalf("target uid(s)=(%d,%v), want mapped", got[0].GetTargetUid(), got[0].GetTargetUids())
	}
	if got[0].GetBatchId() != 1309 || got[0].GetSlotId() != 2 || got[0].GetTaskId() != 2007 || got[0].GetMilestoneIndex() != 3 {
		t.Fatalf("activity identifiers=(%d,%d,%d,%d), want mapped", got[0].GetBatchId(), got[0].GetSlotId(), got[0].GetTaskId(), got[0].GetMilestoneIndex())
	}
}

func TestDomainStatusesExposeCooldownSummary(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 5, 0, 0, time.UTC)
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Task.DailyEnabled = true
	diag := runner.Diagnostics{
		ObservedNamespaces: []string{"7", "22"},
		OperationCooldowns: []runner.OperationCooldownSnapshot{{
			OperationID: "taskDly.recv|target=40001",
			Category:    automation.CategoryBasic,
			Domain:      "basic.task.daily",
			Lane:        automation.LaneSide,
			Reason:      "服务端提示本组任务已经完结",
			Until:       now,
		}},
	}
	statuses := buildDomainStatuses(policy, diag, true)
	for _, status := range statuses {
		if status.GetCategory() != automation.CategoryBasic {
			continue
		}
		if status.GetLane() != pb.ExecutionLane_EXECUTION_LANE_SIDE {
			t.Fatalf("basic lane=%s, want SIDE", status.GetLane())
		}
		if status.GetCooldownUntilMs() != now.UnixMilli() || status.GetCooldownReason() == "" {
			t.Fatalf("basic cooldown=(%d,%q), want populated", status.GetCooldownUntilMs(), status.GetCooldownReason())
		}
		if status.GetStatus() != pb.PlanStatus_PLAN_STATUS_BLOCKED {
			t.Fatalf("basic status=%s, want BLOCKED while cooldown is active", status.GetStatus())
		}
		return
	}
	t.Fatal("missing basic domain status")
}

func TestApplyStoppedRunnerDiagnosticsKeepsPhoneKickAbnormal(t *testing.T) {
	policy := automation.DefaultPolicy()
	out := &pb.AccountStatus{AccountId: 1, AccountName: "main"}
	reason := "账号已在其他设备登录，当前会话被替换"
	applyStoppedRunnerDiagnostics(out, policy, runner.Diagnostics{
		SessionInvalidatedReason: reason,
		BlockedReasons:           []string{"会话已失效"},
	})
	if out.GetHealth() != pb.AccountHealth_ACCOUNT_HEALTH_SESSION_EXPIRED {
		t.Fatalf("health=%q, want session_expired", out.GetHealth())
	}
	if out.GetLastError() != reason {
		t.Fatalf("last_error=%q, want kick reason", out.GetLastError())
	}
	if out.GetDiagnostics().GetSessionInvalidatedReason() != reason {
		t.Fatalf("diagnostics reason=%q, want kick reason", out.GetDiagnostics().GetSessionInvalidatedReason())
	}
}

func TestApplyStoppedRunnerDiagnosticsPlainOfflineWithoutCache(t *testing.T) {
	policy := automation.DefaultPolicy()
	out := &pb.AccountStatus{AccountId: 1, AccountName: "main"}
	applyStoppedRunnerDiagnostics(out, policy, runner.Diagnostics{})
	if out.GetHealth() != pb.AccountHealth_ACCOUNT_HEALTH_OFFLINE {
		t.Fatalf("health=%q, want offline", out.GetHealth())
	}
	if out.GetLastError() != "" {
		t.Fatalf("last_error=%q, want empty", out.GetLastError())
	}
}
