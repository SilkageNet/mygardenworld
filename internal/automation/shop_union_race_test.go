package automation

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// raceStateJSON builds a namespace-25 JSON blob with the given race task pool.
// tasks is a slice of [msId, taskId, score, isUpgrade, upgradeUid].
// Plant-harvest rows (type/id 3036) include param [23001] so take-gate tests can
// distinguish cultivated vs unknown targets.
// Fields are at the TOP LEVEL of ns25 (not nested under "0").
func raceStateJSON(tasks [][5]int32) string {
	return raceStateJSONWithParams(tasks, 23001)
}

func raceStateJSONWithParams(tasks [][5]int32, plantParam int32) string {
	pool := "[]"
	if len(tasks) > 0 {
		parts := make([]string, 0, len(tasks))
		for _, t := range tasks {
			taskID := t[1]
			if state.FmlRaceTaskTypeByID(taskID) == raceTaskTypePlantHarvest && plantParam > 0 {
				parts = append(parts, fmt.Sprintf(
					`{"0":%d,"4":%d,"6":[%d],"10":%d,"14":%d,"15":%d}`,
					t[0], taskID, plantParam, t[2], t[3], t[4],
				))
				continue
			}
			parts = append(parts, fmt.Sprintf(`{"0":%d,"4":%d,"10":%d,"14":%d,"15":%d}`, t[0], taskID, t[2], t[3], t[4]))
		}
		pool = "[" + strings.Join(parts, ",") + "]"
	}
	return `{"25":{"111":{"1":1},"114":` + pool + `}}`
}

// applyRaceState seeds cultivated flower 23001 plus an active race task pool.
func applyRaceState(s *state.State, tasks [][5]int32) {
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(raceStateJSON(tasks)))
}

// testRacePolicy returns a policy with the common defaults for race tests:
// enabled, autoEnableModules on, no score filtering.
func testRacePolicy() *pb.UnionRacePolicy {
	return &pb.UnionRacePolicy{
		Enabled:           true,
		AutoEnableModules: true,
		MaxTaskScore:      0,
	}
}

func TestUnionRaceDisabledProducesNoOps(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{{1, 3036, 10, 0, 0}})))
	policy := &pb.UnionRacePolicy{Enabled: false}
	ops := unionRaceOperations(s, policy, 0, time.Now())
	if len(ops) != 0 {
		t.Fatalf("expected 0 ops when disabled, got %d: %+v", len(ops), ops)
	}
}

func TestUnionRaceEnterIsExecutable(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"0":{}}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now())
	if len(ops) != 1 {
		t.Fatalf("expected 1 enter op, got %d: %+v", len(ops), ops)
	}
	op := ops[0]
	if op.Kind != clientproto.RPCFmlRaceEnter.String() {
		t.Fatalf("expected enter RPC, got %s", op.Kind)
	}
	if !op.Executable || op.SyncOnly {
		t.Fatalf("enter must be executable (not sync-only), got executable=%v syncOnly=%v status=%s", op.Executable, op.SyncOnly, op.Status)
	}
}

func TestUnionRaceEnterNotEmittedWhenObserved(t *testing.T) {
	s := state.New()
	// Race data present → Observed=true → no enter op.
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{{1, 3036, 10, 0, 0}})))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceEnter.String() {
			t.Fatalf("should not emit enter when race data already observed")
		}
	}
}

func TestUnionRaceAutoModulesOffProducesNoOps(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{{1, 3036, 10, 0, 0}})))
	policy := &pb.UnionRacePolicy{Enabled: true, AutoEnableModules: false, MaxTaskScore: 28}
	ops := unionRaceOperations(s, policy, 0, time.Now())
	if len(ops) != 0 {
		t.Fatalf("expected 0 ops when autoEnableModules off, got %d: %+v", len(ops), ops)
	}
}

func TestUnionRaceScoreLimitFiltersLowScoreTasks(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{
		{1, 3036, 3, 0, 0},  // score below threshold → filtered
		{2, 3036, 10, 0, 0}, // score above threshold → eligible
	})
	policy := testRacePolicy()
	policy.MaxTaskScore = 5 // lower bound: skip tasks with Score <= 5
	ops := unionRaceOperations(s, policy, 0, time.Now())
	if len(ops) != 1 {
		t.Fatalf("expected 1 take op, got %d: %+v", len(ops), ops)
	}
	if ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take RPC, got %s", ops[0].Kind)
	}
	if ops[0].TaskMsID != 2 {
		t.Fatalf("expected taskMsId 2 (score > 5), got %d", ops[0].TaskMsID)
	}
}

func TestUnionRacePrioritySorting(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{
		{1, 3044, 10, 0, 0}, // 花种培育, priority 4
		{2, 3036, 10, 0, 0}, // 种植收获, priority 5
	})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{
		3036: 5,
		3044: 4,
	}
	ops := unionRaceOperations(s, policy, 0, time.Now())
	if len(ops) != 1 {
		t.Fatalf("expected 1 take op, got %d", len(ops))
	}
	if ops[0].TaskMsID != 2 {
		t.Fatalf("expected taskMsId 2 (higher priority 3036), got %d", ops[0].TaskMsID)
	}
}

func TestUnionRaceExcludeOthersUpgraded(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{
		{1, 3036, 10, 1, 100}, // upgraded by uid 100
		{2, 3036, 10, 0, 0},   // not upgraded
	})
	policy := testRacePolicy()
	policy.ExcludeOthersUpgradeTask = true
	ops := unionRaceOperations(s, policy, 999, time.Now()) // current uid 999
	if len(ops) != 1 {
		t.Fatalf("expected 1 take op, got %d", len(ops))
	}
	if ops[0].TaskMsID != 2 {
		t.Fatalf("expected taskMsId 2 (exclude uid-100 upgraded task), got %d", ops[0].TaskMsID)
	}
}

func TestUnionRaceFinishCompletedTask(t *testing.T) {
	s := state.New()
	// User uid 999 holds task msId 5, FinishCnt 3 == TargetCnt 3 -> finish.
	// One available task in pool (msId 1, score 10).
	// Field 110 is a map keyed by UID string: {"999":{"7":{"0":5,"1":3036,"2":3,"3":3}}}
	// Field 7 is TakeTaskData with TaskMsId(0), TaskId(1), TargetCnt(2), FinishCnt(3).
	// Set roleID=999 via namespace 7 -> "0" -> "0".
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"114":[{"0":1,"4":3036,"10":10,"14":0,"15":0}],"110":{"999":{"7":{"0":5,"1":3036,"2":3,"3":3}}}}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 999, time.Now())
	var hasFinish, hasTake bool
	for _, op := range ops {
		switch op.Kind {
		case clientproto.RPCFmlRaceFinishTask.String():
			hasFinish = true
			if op.TaskMsID != 5 {
				t.Fatalf("finish taskMsId = %d, want 5", op.TaskMsID)
			}
		case clientproto.RPCFmlRaceTakeTask.String():
			hasTake = true
		}
	}
	if !hasFinish {
		t.Fatalf("expected a finish op, got %+v", ops)
	}
	if hasTake {
		t.Fatalf("should not take while holding an unfinished task")
	}
}

func TestUnionRaceOnlyUpgradeTaskFilter(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{
		{1, 3036, 3, 0, 0},  // not upgraded, below min → filtered
		{2, 3036, 3, 1, 0},  // upgraded, below min → filtered
		{3, 3036, 10, 0, 0}, // not upgraded, above min → filtered
		{4, 3036, 10, 1, 0}, // upgraded, above min → eligible ✓
	})
	policy := testRacePolicy()
	policy.MaxTaskScore = 5
	policy.OnlyUpgradeTask = true
	ops := unionRaceOperations(s, policy, 0, time.Now())
	if len(ops) != 1 {
		t.Fatalf("expected 1 take op, got %d", len(ops))
	}
	if ops[0].TaskMsID != 4 {
		t.Fatalf("expected taskMsId 4 (upgraded, score > 5), got %d", ops[0].TaskMsID)
	}
}

func TestUnionRaceBatchInactiveProducesNoOps(t *testing.T) {
	s := state.New()
	// Ended batch (status=2) with closed window → no race ops.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":9,"1":2,"2":1000,"3":2000},"114":[{"0":1,"4":3036,"10":10,"14":0,"15":0}]}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now())
	if len(ops) != 0 {
		t.Fatalf("expected 0 ops when batch inactive, got %d: %+v", len(ops), ops)
	}
}

func TestUnionRaceEnterEmittedWhenOnlyTaskStubsObserved(t *testing.T) {
	s := state.New()
	// Task pool / usr stubs without a real CurFmlRaceBatch must still trigger enter.
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":3036,"10":10,"14":0,"15":0}],"110":{}}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceEnter.String() {
		t.Fatalf("expected enter when batch not synced, got %+v", ops)
	}
}

func TestUnionRaceGetTaskListAfterActiveBatch(t *testing.T) {
	s := state.New()
	// Enter response carries batch 111 but not task pool 114.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1783872000000,"1":1,"2":1783990800000,"3":1784466000000}}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected getTaskList after enter batch, got %+v", ops)
	}
	if !ops[0].Executable || ops[0].SyncOnly {
		t.Fatalf("getTaskList must be executable, got %+v", ops[0])
	}
}

func TestUnionRaceUpgradeOpEmission(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{
		{1, 3036, 10, 0, 0}, // not upgraded, score within limit
	})))
	policy := testRacePolicy()
	policy.UpgradeTask = true
	ops := unionRaceOperations(s, policy, 0, time.Now())
	var hasUpgrade bool
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceUpgradeTask.String() {
			hasUpgrade = true
			if op.TaskMsID != 1 {
				t.Fatalf("upgrade taskMsId = %d, want 1", op.TaskMsID)
			}
		}
	}
	if !hasUpgrade {
		t.Fatalf("expected an upgrade op, got %+v", ops)
	}
}

func TestUnionRaceDeleteLowScoreOpEmission(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{
		{1, 3036, 5, 0, 0},  // low score, should be deleted
		{2, 3036, 20, 0, 0}, // higher score, taken instead
	})
	policy := testRacePolicy()
	policy.DeleteLowScoreTask = true
	policy.DeleteTaskMaxScore = 10
	ops := unionRaceOperations(s, policy, 0, time.Now())
	var hasDelete bool
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceDelTask.String() {
			hasDelete = true
			if op.TaskMsID != 1 {
				t.Fatalf("delete taskMsId = %d, want 1", op.TaskMsID)
			}
		}
	}
	if !hasDelete {
		t.Fatalf("expected a delete op, got %+v", ops)
	}
}

func TestUnionRaceGiveUpTaskBelowScoreThreshold(t *testing.T) {
	s := state.New()
	// Task pool: task msId=1, score=5. Taken task: msId=1, not completed (0/3).
	// Field 110: {"999":{"7":{"0":1,"1":3036,"2":3,"3":0}}}  — FinishCnt=0
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"114":[{"0":1,"4":3036,"10":5,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":3,"3":0}}}}}`))
	policy := testRacePolicy()
	policy.MaxTaskScore = 10 // lower bound: skip tasks with Score <= 10
	ops := unionRaceOperations(s, policy, 999, time.Now())
	var hasGiveUp bool
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			hasGiveUp = true
		}
	}
	if !hasGiveUp {
		t.Fatalf("expected a giveUp op for task below score threshold, got %+v", ops)
	}
}

func TestUnionRaceGiveUpUncultivatedTakenPlantHarvest(t *testing.T) {
	s := state.New()
	// Taken plant-harvest for uncultivated flower 23099 — impossible to complete.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"114":[{"0":1,"4":3036,"6":[23099],"10":56,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":600,"3":0,"4":[23099]}}}}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now())
	var giveUp *PlannedOp
	for i := range ops {
		if ops[i].Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			giveUp = &ops[i]
			break
		}
	}
	if giveUp == nil {
		t.Fatalf("expected giveUp for uncultivated taken plant-harvest, got %+v", ops)
	}
	if giveUp.FlowerID != 23099 || giveUp.TaskID != 3036 {
		t.Fatalf("giveUp detail taskID=%d flowerID=%d, want 3036/23099", giveUp.TaskID, giveUp.FlowerID)
	}
	if !strings.Contains(giveUp.Reason, "无法完成") {
		t.Fatalf("giveUp reason = %q, want 无法完成", giveUp.Reason)
	}
}

func TestUnionRaceNoGiveUpCultivatedTakenPlantHarvest(t *testing.T) {
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"114":[{"0":1,"4":3036,"6":[23001],"10":56,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":600,"3":0,"4":[23001]}}}}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("must not giveUp cultivated plant-harvest, got %+v", ops)
		}
	}
}

func TestUnionRaceNoGiveUpWhenTaskComplete(t *testing.T) {
	s := state.New()
	// Task pool: task msId=1, score=5. Taken task: msId=1, completed (3/3).
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"114":[{"0":1,"4":3036,"10":5,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":3,"3":3}}}}}`))
	policy := testRacePolicy()
	policy.MaxTaskScore = 10
	ops := unionRaceOperations(s, policy, 999, time.Now())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("should not giveUp a completed task, got %+v", ops)
		}
	}
}

func TestUnionRaceNoGiveUpWhenNoScoreLimit(t *testing.T) {
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	// Task pool: task msId=1, score=5. Taken plant-harvest is completable; no score limit.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"114":[{"0":1,"4":3036,"6":[23001],"10":5,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":3,"3":0,"4":[23001]}}}}}`))
	policy := testRacePolicy()
	policy.MaxTaskScore = 0 // no filtering
	ops := unionRaceOperations(s, policy, 999, time.Now())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("should not giveUp when no score limit, got %+v", ops)
		}
	}
}

func TestUnionRaceNoGiveUpWhenTakenScoreUnknown(t *testing.T) {
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	// Taken task present but missing from the pool → Score stays 0.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"114":[],"110":{"999":{"7":{"0":1,"1":3036,"2":3,"3":0,"4":[23001]}}}}}`))
	policy := testRacePolicy()
	policy.MaxTaskScore = 10
	ops := unionRaceOperations(s, policy, 999, time.Now())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("should not giveUp when taken score is unresolved, got %+v", ops)
		}
	}
}

func TestUnionRaceSkipsFarFutureAppearTime(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	appear := now.Add(time.Hour).UnixMilli()
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"111":{"1":1},"114":[{"0":1,"4":3036,"5":%d,"6":[23001],"10":10,"14":0,"15":0}]}}`, appear,
	)))
	ops := unionRaceOperations(s, testRacePolicy(), 0, now)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take far-future CD task, got %+v", ops)
		}
	}
}

func TestUnionRacePrefersReadyOverUpcoming(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	readyAppear := now.UnixMilli()
	upcomingAppear := now.Add(2 * time.Second).UnixMilli()
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"111":{"1":1},"114":[{"0":1,"4":3036,"5":%d,"6":[23001],"10":5,"14":0,"15":0},{"0":2,"4":3036,"5":%d,"6":[23001],"10":99,"14":0,"15":0}]}}`,
		readyAppear, upcomingAppear,
	)))
	ops := unionRaceOperations(s, testRacePolicy(), 0, now)
	if len(ops) == 0 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take op, got %+v", ops)
	}
	if ops[0].TaskMsID != 1 {
		t.Fatalf("ready task must win over higher-score upcoming, got msId=%d", ops[0].TaskMsID)
	}
}

func TestUnionRacePreemptiveTakeWithinLead(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	appear := now.Add(2 * time.Second).UnixMilli()
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"111":{"1":1},"114":[{"0":7,"4":3036,"5":%d,"6":[23001],"10":10,"14":0,"15":0}]}}`, appear,
	)))
	ops := unionRaceOperations(s, testRacePolicy(), 0, now)
	if len(ops) == 0 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected preemptive take within lead, got %+v", ops)
	}
	if ops[0].TaskMsID != 7 {
		t.Fatalf("taskMsId = %d, want 7", ops[0].TaskMsID)
	}
}

func TestUnionRaceSkipsUncultivatedPlantHarvest(t *testing.T) {
	s := state.New()
	// Only plant-harvest with unknown / uncultivated flower — no take.
	s.ApplyV(json.RawMessage(raceStateJSONWithParams([][5]int32{{1, 3036, 10, 0, 0}}, 23099)))
	ops := unionRaceOperations(s, testRacePolicy(), 0, time.Now())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take uncultivated plant-harvest, got %+v", ops)
		}
	}
}

func TestUnionRaceTakesCultivatedPlantHarvestOverUncultivated(t *testing.T) {
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"114":[
		{"0":1,"4":3036,"6":[23099],"10":99,"14":0,"15":0},
		{"0":2,"4":3036,"6":[23001],"10":10,"14":0,"15":0}
	]}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 0, time.Now())
	if len(ops) == 0 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take of cultivated plant-harvest, got %+v", ops)
	}
	if ops[0].TaskMsID != 2 {
		t.Fatalf("taskMsId = %d, want 2 (cultivated), FlowerID=%d", ops[0].TaskMsID, ops[0].FlowerID)
	}
	if ops[0].FlowerID != 23001 || ops[0].TaskID != 3036 {
		t.Fatalf("take op must carry task detail, got taskID=%d flowerID=%d", ops[0].TaskID, ops[0].FlowerID)
	}
}

func TestUnionRaceFallsBackToNonPlantWhenPlantUncultivated(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"114":[
		{"0":1,"4":3036,"6":[23099],"10":50,"14":0,"15":0},
		{"0":2,"4":3044,"10":10,"14":0,"15":0}
	]}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3036: 5, 3044: 4}
	ops := unionRaceOperations(s, policy, 0, time.Now())
	if len(ops) == 0 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected fallback take, got %+v", ops)
	}
	if ops[0].TaskMsID != 2 {
		t.Fatalf("taskMsId = %d, want 2 (non-plant fallback)", ops[0].TaskMsID)
	}
}

func TestFormatRaceTaskOpDesc(t *testing.T) {
	got := FormatRaceTaskOpDesc(3036, 23001)
	if !strings.Contains(got, "种植收获") || !strings.Contains(got, "白百合") {
		t.Fatalf("FormatRaceTaskOpDesc = %q, want 种植收获 · 白百合", got)
	}
	if got != "种植收获 · 白百合" {
		t.Fatalf("FormatRaceTaskOpDesc = %q, want exact title", got)
	}
}

func TestRaceTakeSkipReason(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.Local)
	leadMs := now.Add(raceTakeLeadWindow).UnixMilli()

	s := state.New()
	// Cultivate 23001 for plantability cases; leave others uncultivated.
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})

	uid := int64(42)
	policyBase := func() *pb.UnionRacePolicy {
		return &pb.UnionRacePolicy{
			MaxTaskScore:             0,
			OnlyUpgradeTask:          false,
			ExcludeOthersUpgradeTask: false,
		}
	}

	cases := []struct {
		name   string
		task   state.FmlRaceTaskView
		policy *pb.UnionRacePolicy
		want   string
	}{
		{
			name: "taken by other",
			task: state.FmlRaceTaskView{MsId: 1, TaskId: 3030, TaskType: 3030, Score: 20, UID: 99},
			policy: policyBase(),
			want: "已被接取",
		},
		{
			name: "far CD",
			task: state.FmlRaceTaskView{
				MsId: 2, TaskId: 3030, TaskType: 3030, Score: 20,
				AppearTime: now.Add(time.Hour).UnixMilli(),
			},
			policy: policyBase(),
			want: "冷却中，" + time.UnixMilli(now.Add(time.Hour).UnixMilli()).Local().Format("15:04") + " 后可接",
		},
		{
			name: "within lead is takeable",
			task: state.FmlRaceTaskView{
				MsId: 3, TaskId: 3030, TaskType: 3030, Score: 20,
				AppearTime: leadMs - 1,
			},
			policy: policyBase(),
			want: "",
		},
		{
			name: "exactly at lead boundary is takeable",
			task: state.FmlRaceTaskView{
				MsId: 12, TaskId: 3030, TaskType: 3030, Score: 20,
				AppearTime: leadMs,
			},
			policy: policyBase(),
			want: "",
		},
		{
			name: "one ms past lead is CD",
			task: state.FmlRaceTaskView{
				MsId: 13, TaskId: 3030, TaskType: 3030, Score: 20,
				AppearTime: leadMs + 1,
			},
			policy: policyBase(),
			want: "冷却中，" + time.UnixMilli(leadMs+1).Local().Format("15:04") + " 后可接",
		},
		{
			name: "score too low",
			task: state.FmlRaceTaskView{MsId: 4, TaskId: 3030, TaskType: 3030, Score: 10},
			policy: &pb.UnionRacePolicy{MaxTaskScore: 15},
			want: "分数不足（≤15）",
		},
		{
			name: "only upgrade",
			task: state.FmlRaceTaskView{MsId: 5, TaskId: 3030, TaskType: 3030, Score: 20, IsUpgrade: 0},
			policy: &pb.UnionRacePolicy{OnlyUpgradeTask: true},
			want: "仅接已升级任务",
		},
		{
			name: "others upgraded",
			task: state.FmlRaceTaskView{MsId: 6, TaskId: 3030, TaskType: 3030, Score: 20, IsUpgrade: 1, UpgradeUid: 99},
			policy: &pb.UnionRacePolicy{ExcludeOthersUpgradeTask: true},
			want: "他人已升级",
		},
		{
			name: "own upgraded ok",
			task: state.FmlRaceTaskView{MsId: 7, TaskId: 3030, TaskType: 3030, Score: 20, IsUpgrade: 1, UpgradeUid: uid},
			policy: &pb.UnionRacePolicy{ExcludeOthersUpgradeTask: true},
			want: "",
		},
		{
			name: "plant not cultivated",
			task: state.FmlRaceTaskView{MsId: 8, TaskId: 3036, TaskType: 3036, Score: 30, ParamID: 23999},
			policy: policyBase(),
			want: "目标花卉未培养",
		},
		{
			name: "plant missing param",
			task: state.FmlRaceTaskView{MsId: 14, TaskId: 3036, TaskType: 3036, Score: 30, ParamID: 0},
			policy: policyBase(),
			want: "目标花卉未培养",
		},
		{
			name: "plant cultivated ok",
			task: state.FmlRaceTaskView{MsId: 9, TaskId: 3036, TaskType: 3036, Score: 30, ParamID: 23001},
			policy: policyBase(),
			want: "",
		},
		{
			name: "priority: taken wins over CD",
			task: state.FmlRaceTaskView{
				MsId: 10, TaskId: 3030, TaskType: 3030, Score: 20, UID: 7,
				AppearTime: now.Add(time.Hour).UnixMilli(),
			},
			policy: policyBase(),
			want: "已被接取",
		},
		{
			name: "priority: CD wins over score",
			task: state.FmlRaceTaskView{
				MsId: 11, TaskId: 3030, TaskType: 3030, Score: 5,
				AppearTime: now.Add(time.Hour).UnixMilli(),
			},
			policy: &pb.UnionRacePolicy{MaxTaskScore: 20},
			want: "冷却中，" + time.UnixMilli(now.Add(time.Hour).UnixMilli()).Local().Format("15:04") + " 后可接",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RaceTakeSkipReason(s, tc.task, tc.policy, uid, now)
			if got != tc.want {
				t.Fatalf("RaceTakeSkipReason = %q, want %q", got, tc.want)
			}
		})
	}
}
