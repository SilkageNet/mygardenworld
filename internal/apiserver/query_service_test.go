package apiserver

import (
	"reflect"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestBuildLandViewsUsesServerRosterForOpenedStatus(t *testing.T) {
	lands := map[int32]state.LandView{
		1001: {Observed: true, FlowerID: 23001, State: 1},
	}
	farmLands := []state.FarmLandInfo{
		{ID: 1001, OpenLevel: 1, Cost: []int32{33, 800}},
		{ID: 1002, OpenLevel: 1, Cost: []int32{34, 800}},
		{ID: 1058, OpenLevel: 30, Cost: []int32{35, 800}},
	}

	got := buildLandViews(lands, farmLands, true, true, 1, time.Unix(0, 0))
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
	got := buildLandViews(lands, farmLands, true, true, 13, time.Unix(0, 0))
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
	got := buildLandViews(map[int32]state.LandView{}, []state.FarmLandInfo{{ID: 1001, OpenLevel: 1, Cost: []int32{33, 800}}}, false, true, 1, time.Unix(0, 0))
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
	got := buildLandViews(map[int32]state.LandView{}, nil, true, false, 13, time.Unix(0, 0))
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
		OperationID: "taskDly.recv|target=40001",
		Kind:        "taskDly.recv",
		Lane:        automation.LaneSide,
		Category:    automation.CategoryBasic,
		Domain:      "basic.task.daily",
		Action:      "claim",
		Executable:  true,
		Status:      automation.PlanStatusReady,
		TargetID:    40001,
		TargetUID:   9002001,
		TargetUIDs:  []int64{9002001, 9002002},
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
