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
	return `{"25":{"111":{"1":1},"117":{"5":4},"114":` + pool + `}}`
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
		MinTaskScore:      0,
	}
}

func TestUnionRaceDisabledProducesNoOps(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{{1, 3036, 10, 0, 0}})))
	policy := &pb.UnionRacePolicy{Enabled: false}
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 0 {
		t.Fatalf("expected 0 ops when disabled, got %d: %+v", len(ops), ops)
	}
}

func TestUnionRaceEnterIsExecutable(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"0":{}}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceEnter.String() {
			t.Fatalf("should not emit enter when race data already observed")
		}
	}
}

func TestUnionRaceAutoModulesOffProducesNoOps(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{{1, 3036, 10, 0, 0}})))
	policy := &pb.UnionRacePolicy{Enabled: true, AutoEnableModules: false, MinTaskScore: 28}
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 0 {
		t.Fatalf("expected 0 ops when autoEnableModules off, got %d: %+v", len(ops), ops)
	}
}

func TestUnionRaceAutoModulesOffStillSyncsAndRefreshes(t *testing.T) {
	// Enabled + !AutoEnableModules: observe/sync the pool for UI, but never take.
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"0":{}}}`))
	policy := &pb.UnionRacePolicy{Enabled: true, AutoEnableModules: false}
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceEnter.String() {
		t.Fatalf("expected enter sync when modules off, got %+v", ops)
	}

	s = state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4}}}`))
	ops = unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected getTaskList sync when modules off, got %+v", ops)
	}

	s = state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4},"114":[]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	now := time.UnixMilli(synced).Add(raceTaskPoolRefreshInterval + time.Second)
	ops = unionRaceOperations(s, policy, 0, now, true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected TTL refresh when modules off, got %+v", ops)
	}
}

func TestUnionRaceScoreLimitFiltersLowScoreTasks(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{
		{1, 3036, 3, 0, 0},  // score below threshold → filtered
		{2, 3036, 10, 0, 0}, // score above threshold → eligible
	})
	policy := testRacePolicy()
	policy.MinTaskScore = 5 // lower bound: skip tasks with Score <= 5
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
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

func TestUnionRaceTakeQuotaExhaustedSkipsTake(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3036, 10, 0, 0}})
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take before quota exhausted, got %+v", ops)
	}
	s.MarkFmlRaceTakeQuotaExhausted()
	ops = unionRaceOperations(s, policy, 0, time.Now(), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take after quota exhausted, got %+v", ops)
		}
	}
}

func TestUnionRaceAutoStopOnQuotaDoneSkipsTake(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3036, 10, 0, 0}})
	// raceLvl=4 → free total 18; finished=18 means free quota is done.
	s.ApplyV(json.RawMessage(`{"25":{"110":{"1":{"3":18}}}}`))
	if !s.FmlRace().TaskQuotaObserved || s.FmlRace().FinishedTaskNum != 18 {
		t.Fatalf("quota not applied: %+v", s.FmlRace())
	}

	policy := testRacePolicy()
	policy.AutoStopOnQuotaDone = true
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take when free quota done, got %+v", ops)
		}
	}

	// With the switch off, keep taking until the server rejects.
	policy.AutoStopOnQuotaDone = false
	ops = unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take when auto-stop off, got %+v", ops)
	}

	// Remaining free quota still allows take while auto-stop is on.
	s = state.New()
	applyRaceState(s, [][5]int32{{1, 3036, 10, 0, 0}})
	s.ApplyV(json.RawMessage(`{"25":{"110":{"1":{"3":17}}}}`))
	policy = testRacePolicy()
	policy.AutoStopOnQuotaDone = true
	ops = unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take with remaining quota, got %+v", ops)
	}
}

func TestUnionRaceAutoStopOnQuotaDoneStillFinishesHeldTask(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3036, 10, 0, 0}})
	// Field 110 takeTaskData: TargetCnt=3, FinishCnt=3; fTaskNum=18 (free quota done).
	s.ApplyV(json.RawMessage(`{"25":{"110":{"1":{"3":18,"7":{"0":1,"1":3036,"2":3,"3":3}}}}}`))
	policy := testRacePolicy()
	policy.AutoStopOnQuotaDone = true
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceFinishTask.String() {
		t.Fatalf("expected finish despite quota done, got %+v", ops)
	}
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take while finishing held task, got %+v", ops)
		}
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 1 {
		t.Fatalf("expected 1 take op, got %d", len(ops))
	}
	if ops[0].TaskMsID != 2 {
		t.Fatalf("expected taskMsId 2 (higher priority 3036), got %d", ops[0].TaskMsID)
	}
}

func TestUnionRacePriorityZeroNotTaken(t *testing.T) {
	s := state.New()
	// Material-shop (3017) and floral-sale (3030) only; both priority 0.
	applyRaceState(s, [][5]int32{
		{1, 3017, 24, 0, 0},
		{2, 3030, 30, 0, 0},
	})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{
		3017: 0,
		3030: 0,
		3036: 5,
		3044: 4,
	}
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("priority 0 tasks must not be taken, got take msId=%d taskId=%d", op.TaskMsID, op.TaskID)
		}
	}
}

func TestUnionRacePriorityZeroFallsThroughToPositive(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{
		{1, 3017, 40, 0, 0}, // material shop, priority 0, higher score
		{2, 3036, 10, 0, 0}, // plant harvest, priority 5
	})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{
		3017: 0,
		3036: 5,
	}
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take of priority>0 task, got %+v", ops)
	}
	if ops[0].TaskMsID != 2 {
		t.Fatalf("expected plant-harvest msId 2, got %d", ops[0].TaskMsID)
	}
}

func TestUnionRaceGiveUpTakenPriorityZero(t *testing.T) {
	s := state.New()
	// Taken material-shop 3017 (priority 0), unfinished with no progress.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3017,"10":24,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3017,"2":60,"3":0}}}}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{
		3017: 0,
		3036: 5,
	}
	ops := unionRaceOperations(s, policy, 999, time.Now(), true)
	var hasGiveUp bool
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			hasGiveUp = true
			if op.TaskMsID != 1 {
				t.Fatalf("giveUp taskMsId = %d, want 1", op.TaskMsID)
			}
		}
	}
	if !hasGiveUp {
		t.Fatalf("expected giveUp for priority-0 taken task, got %+v", ops)
	}
}

func TestUnionRaceNoGiveUpWhenTakenHasProgress(t *testing.T) {
	s := state.New()
	// Low score + priority 0 + FinishCnt>0 → keep (do not cancel mid-progress).
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3017,"10":5,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3017,"2":60,"3":16}}}}}`))
	policy := testRacePolicy()
	policy.MinTaskScore = 24
	policy.TaskTypePriority = map[int32]int32{3017: 0, 3036: 5}
	for _, op := range unionRaceOperations(s, policy, 999, time.Now(), true) {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("must not giveUp a started task, got %+v", op)
		}
	}
}

func TestUnionRaceGiveUpTakenMissingFromPool(t *testing.T) {
	s := state.New()
	// Taken msId=99 not present in observed pool; FinishCnt=0 → give up.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":30,"14":0,"15":0}],"110":{"999":{"7":{"0":99,"1":3036,"2":280,"3":0,"4":[23022]}}}}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now(), true)
	var giveUp *PlannedOp
	for i := range ops {
		if ops[i].Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			giveUp = &ops[i]
			break
		}
	}
	if giveUp == nil {
		t.Fatalf("expected giveUp for taken task missing from pool, got %+v", ops)
	}
	if !strings.Contains(giveUp.Reason, "不在任务池") {
		t.Fatalf("giveUp reason = %q, want 不在任务池", giveUp.Reason)
	}
}

func TestUnionRaceNoGiveUpMissingFromPoolWhenHasProgress(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":30}],"110":{"999":{"7":{"0":99,"1":3036,"2":280,"3":12,"4":[23022]}}}}}`))
	for _, op := range unionRaceOperations(s, testRacePolicy(), 999, time.Now(), true) {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("must not giveUp started task missing from pool, got %+v", op)
		}
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
	ops := unionRaceOperations(s, policy, 999, time.Now(), true) // current uid 999
	if len(ops) != 1 {
		t.Fatalf("expected 1 take op, got %d", len(ops))
	}
	if ops[0].TaskMsID != 2 {
		t.Fatalf("expected taskMsId 2 (exclude uid-100 upgraded task), got %d", ops[0].TaskMsID)
	}

	// Server often sets IsUpgrade without UpgradeUid; those must also be excluded.
	s2 := state.New()
	s2.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s2.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4},"114":[
		{"0":1,"4":3036,"6":[23001],"10":28,"14":1,"15":0},
		{"0":2,"4":3036,"6":[23001],"10":10,"14":0,"15":0}
	]}}`))
	ops2 := unionRaceOperations(s2, policy, 999, time.Now(), true)
	if len(ops2) != 1 || ops2[0].TaskMsID != 2 {
		t.Fatalf("expected take msId 2 only when upgraded-without-uid present, got %+v", ops2)
	}
}

func TestUnionRaceFinishCompletedTask(t *testing.T) {
	s := state.New()
	// User uid 999 holds task msId 5, FinishCnt 3 == TargetCnt 3 -> finish.
	// One available task in pool (msId 1, score 10).
	// Field 110 is a map keyed by UID string: {"999":{"7":{"0":5,"1":3036,"2":3,"3":3}}}
	// Field 7 is TakeTaskData with TaskMsId(0), TaskId(1), TargetCnt(2), FinishCnt(3).
	// Set roleID=999 via namespace 7 -> "0" -> "0".
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":10,"14":0,"15":0}],"110":{"999":{"7":{"0":5,"1":3036,"2":3,"3":3}}}}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 999, time.Now(), true)
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
	policy.MinTaskScore = 5
	policy.OnlyUpgradeTask = true
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
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
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":9,"1":2,"2":1000,"3":2000},"117":{"5":4},"114":[{"0":1,"4":3036,"10":10,"14":0,"15":0}]}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 0 {
		t.Fatalf("expected 0 ops when batch inactive, got %d: %+v", len(ops), ops)
	}
}

func TestUnionRaceEnterEmittedWhenOnlyTaskStubsObserved(t *testing.T) {
	s := state.New()
	// Task pool / usr stubs without a real CurFmlRaceBatch must still trigger enter.
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":3036,"10":10,"14":0,"15":0}],"110":{}}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceEnter.String() {
		t.Fatalf("expected enter when batch not synced, got %+v", ops)
	}
}

func TestUnionRaceGetTaskListAfterActiveBatch(t *testing.T) {
	s := state.New()
	// Enter response carries batch 111 but not task pool 114.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1783872000000,"1":1,"2":1783990800000,"3":1784466000000},"117":{"5":4}}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected getTaskList after enter batch, got %+v", ops)
	}
	if !ops[0].Executable || ops[0].SyncOnly {
		t.Fatalf("getTaskList must be executable, got %+v", ops[0])
	}
}

func TestUnionRaceGetTaskListWhenPlantHarvestMissingParam(t *testing.T) {
	s := state.New()
	// Observed pool with a plant-harvest row that never got field-6 param detail.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1783872000000,"1":1,"2":1783990800000,"3":1784466000000},"117":{"5":4},"114":[{"0":178397176088908,"4":4011,"10":25,"14":0,"15":0,"6":[]},{"0":178397176088909,"4":4011,"6":[23001],"10":28,"14":0,"15":0}]}}`))
	if got := s.FmlRace(); !got.TasksObserved || len(got.Tasks) != 2 || got.Tasks[0].ParamID != 0 || got.Tasks[1].ParamID != 23001 {
		t.Fatalf("seed pool = %+v", got)
	}
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected getTaskList to refresh missing flower param, got %+v", ops)
	}

	// Same incomplete pool after apply marks the refresh fingerprint — do not loop.
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":178397176088908,"4":4011,"10":25,"14":0,"15":0,"6":[]},{"0":178397176088909,"4":4011,"6":[23001],"10":28,"14":0,"15":0}]}}`))
	ops = unionRaceOperations(s, policy, 0, time.Now(), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("getTaskList must not re-fire for the same incomplete pool: %+v", ops)
		}
	}
}

func TestUnionRaceUpgradeOpEmission(t *testing.T) {
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1},"117":{"5":4},"114":[{"0":1,"4":4001,"6":[23001],"10":9,"12":999,"14":0,"15":0}],"110":{"42":{"7":{"0":1,"1":4001,"2":10,"3":1,"4":[23001]}}}}}`))
	policy := testRacePolicy()
	policy.UpgradeTask = true
	policy.MaxSpendDiamond = 100
	ops := unionRaceOperations(s, policy, 999, time.Now(), true)
	var hasUpgrade bool
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceUpgradeTask.String() {
			hasUpgrade = true
			if op.TaskMsID != 1 {
				t.Fatalf("upgrade taskMsId = %d, want 1", op.TaskMsID)
			}
			if op.DiamondCost != 27 {
				t.Fatalf("upgrade diamond cost = %d, want 27", op.DiamondCost)
			}
		}
	}
	if !hasUpgrade {
		t.Fatalf("expected an upgrade op, got %+v", ops)
	}
}

func TestUnionRaceDoesNotUpgradeUnheldPoolTask(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 4001, 9, 0, 0}})
	policy := testRacePolicy()
	policy.UpgradeTask = true
	policy.MaxSpendDiamond = 100

	for _, op := range unionRaceOperations(s, policy, 999, time.Now(), true) {
		if op.Kind == clientproto.RPCFmlRaceUpgradeTask.String() {
			t.Fatalf("empty-request upgrade RPC must not target an unheld pool row: %+v", op)
		}
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
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

func TestUnionRaceDeleteSkipsOccupiedTask(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":4001,"6":[23001],"10":5,"12":88,"14":0,"15":0}]}}`))
	policy := testRacePolicy()
	policy.DeleteLowScoreTask = true
	policy.DeleteTaskMaxScore = 10

	for _, op := range unionRaceOperations(s, policy, 0, time.Now(), true) {
		if op.Kind == clientproto.RPCFmlRaceDelTask.String() {
			t.Fatalf("must not delete an occupied race task: %+v", op)
		}
	}
}

func TestUnionRaceGiveUpTaskBelowScoreThreshold(t *testing.T) {
	s := state.New()
	// Task pool: task msId=1, score=5. Taken task: msId=1, not completed (0/3).
	// Field 110: {"999":{"7":{"0":1,"1":3036,"2":3,"3":0}}}  — FinishCnt=0
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":5,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":3,"3":0}}}}}`))
	policy := testRacePolicy()
	policy.MinTaskScore = 10 // lower bound: skip tasks with Score <= 10
	ops := unionRaceOperations(s, policy, 999, time.Now(), true)
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
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23099],"10":56,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":600,"3":0,"4":[23099]}}}}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now(), true)
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
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":56,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":600,"3":0,"4":[23001]}}}}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now(), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("must not giveUp cultivated plant-harvest, got %+v", ops)
		}
	}
}

func TestUnionRaceNoGiveUpWhenTaskComplete(t *testing.T) {
	s := state.New()
	// Task pool: task msId=1, score=5. Taken task: msId=1, completed (3/3).
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"10":5,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":3,"3":3}}}}}`))
	policy := testRacePolicy()
	policy.MinTaskScore = 10
	ops := unionRaceOperations(s, policy, 999, time.Now(), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("should not giveUp a completed task, got %+v", ops)
		}
	}
}

func TestUnionRaceDoesNotFinishUnknownZeroTarget(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1},"117":{"5":4},"114":[{"0":1,"4":4001,"6":[23001],"10":9}],"110":{"42":{"7":{"0":1,"1":4001,"2":0,"3":0,"4":[23001]}}}}}`))

	for _, op := range unionRaceOperations(s, testRacePolicy(), 0, time.Now(), true) {
		if op.Kind == clientproto.RPCFmlRaceFinishTask.String() {
			t.Fatalf("zero/zero unresolved progress must not be treated as complete: %+v", op)
		}
	}
}

func TestUnionRaceNoGiveUpWhenNoScoreLimit(t *testing.T) {
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	// Task pool: task msId=1, score=5. Taken plant-harvest is completable; no score limit.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":5,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":3,"3":0,"4":[23001]}}}}}`))
	policy := testRacePolicy()
	policy.MinTaskScore = 0 // no filtering
	ops := unionRaceOperations(s, policy, 999, time.Now(), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("should not giveUp when no score limit, got %+v", ops)
		}
	}
}

func TestUnionRaceNoGiveUpWhenTakenScoreUnknown(t *testing.T) {
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	// Task is in the pool but Score unresolved (0); FinishCnt=0 → do not give up for score alone.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":0,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":3,"3":0,"4":[23001]}}}}}`))
	policy := testRacePolicy()
	policy.MinTaskScore = 10
	ops := unionRaceOperations(s, policy, 999, time.Now(), true)
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
		`{"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"5":%d,"6":[23001],"10":10,"14":0,"15":0}]}}`, appear,
	)))
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, true)
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
		`{"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"5":%d,"6":[23001],"10":5,"14":0,"15":0},{"0":2,"4":3036,"5":%d,"6":[23001],"10":99,"14":0,"15":0}]}}`,
		readyAppear, upcomingAppear,
	)))
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, true)
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
		`{"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":7,"4":3036,"5":%d,"6":[23001],"10":10,"14":0,"15":0}]}}`, appear,
	)))
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, true)
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
	ops := unionRaceOperations(s, testRacePolicy(), 0, time.Now(), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take uncultivated plant-harvest, got %+v", ops)
		}
	}
}

func TestUnionRaceTakesCultivatedPlantHarvestOverUncultivated(t *testing.T) {
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4},"114":[
		{"0":1,"4":3036,"6":[23099],"10":99,"14":0,"15":0},
		{"0":2,"4":3036,"6":[23001],"10":10,"14":0,"15":0}
	]}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 0, time.Now(), true)
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

func TestUnionRaceDoesNotTakeUnsupportedFallback(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4},"114":[
		{"0":1,"4":3036,"6":[23099],"10":50,"14":0,"15":0},
		{"0":2,"4":3044,"10":10,"14":0,"15":0}
	]}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3036: 5, 3044: 4}
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take an unsupported task merely as a fallback: %+v", op)
		}
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
			MinTaskScore:             0,
			OnlyUpgradeTask:          false,
			ExcludeOthersUpgradeTask: false,
			TaskTypePriority:         defaultUnionRacePriority(),
		}
	}
	takeablePlant := policyBase

	cases := []struct {
		name   string
		task   state.FmlRaceTaskView
		policy *pb.UnionRacePolicy
		want   string
	}{
		{
			name:   "taken by other",
			task:   state.FmlRaceTaskView{MsId: 1, TaskId: 3030, TaskType: 3030, Score: 20, UID: 99},
			policy: policyBase(),
			want:   "已被接取",
		},
		{
			name: "far CD otherwise takeable",
			task: state.FmlRaceTaskView{
				MsId: 2, TaskId: 3036, TaskType: 3036, Score: 20, ParamID: 23001,
				AppearTime: now.Add(time.Hour).UnixMilli(),
			},
			policy: takeablePlant(),
			want:   "冷却中，" + time.UnixMilli(now.Add(time.Hour).UnixMilli()).Local().Format("15:04:05") + " 后可接",
		},
		{
			name: "far CD plant not cultivated → refresh",
			task: state.FmlRaceTaskView{
				MsId: 18, TaskId: 3036, TaskType: 3036, Score: 30, ParamID: 23999,
				AppearTime: now.Add(time.Hour).UnixMilli(),
			},
			policy: policyBase(),
			want:   time.UnixMilli(now.Add(time.Hour).UnixMilli()).Local().Format("15:04:05") + " 后刷新",
		},
		{
			name: "far CD score gate would fail → refresh",
			task: state.FmlRaceTaskView{
				MsId: 19, TaskId: 3030, TaskType: 3030, Score: 5,
				AppearTime: now.Add(time.Hour).UnixMilli(),
			},
			policy: func() *pb.UnionRacePolicy {
				p := takeablePlant()
				p.MinTaskScore = 20
				return p
			}(),
			want: time.UnixMilli(now.Add(time.Hour).UnixMilli()).Local().Format("15:04:05") + " 后刷新",
		},
		{
			name: "within lead is takeable",
			task: state.FmlRaceTaskView{
				MsId: 3, TaskId: 3036, TaskType: 3036, Score: 20, ParamID: 23001,
				AppearTime: leadMs - 1,
			},
			policy: takeablePlant(),
			want:   "",
		},
		{
			name: "exactly at lead boundary is takeable",
			task: state.FmlRaceTaskView{
				MsId: 12, TaskId: 3036, TaskType: 3036, Score: 20, ParamID: 23001,
				AppearTime: leadMs,
			},
			policy: takeablePlant(),
			want:   "",
		},
		{
			name: "one ms past lead otherwise takeable is CD",
			task: state.FmlRaceTaskView{
				MsId: 13, TaskId: 3036, TaskType: 3036, Score: 20, ParamID: 23001,
				AppearTime: leadMs + 1,
			},
			policy: takeablePlant(),
			want:   "冷却中，" + time.UnixMilli(leadMs+1).Local().Format("15:04:05") + " 后可接",
		},
		{
			name:   "score too low",
			task:   state.FmlRaceTaskView{MsId: 4, TaskId: 3030, TaskType: 3030, Score: 10},
			policy: &pb.UnionRacePolicy{MinTaskScore: 15},
			want:   "分数不足（≤15）",
		},
		{
			name:   "only upgrade",
			task:   state.FmlRaceTaskView{MsId: 5, TaskId: 3030, TaskType: 3030, Score: 20, IsUpgrade: 0},
			policy: &pb.UnionRacePolicy{OnlyUpgradeTask: true},
			want:   "仅接已升级任务",
		},
		{
			name:   "others upgraded",
			task:   state.FmlRaceTaskView{MsId: 6, TaskId: 3030, TaskType: 3030, Score: 20, IsUpgrade: 1, UpgradeUid: 99},
			policy: &pb.UnionRacePolicy{ExcludeOthersUpgradeTask: true},
			want:   "他人已升级",
		},
		{
			name:   "upgraded with missing upgradeUid",
			task:   state.FmlRaceTaskView{MsId: 16, TaskId: 3036, TaskType: 3036, Score: 28, ParamID: 23001, IsUpgrade: 1, UpgradeUid: 0},
			policy: &pb.UnionRacePolicy{ExcludeOthersUpgradeTask: true},
			want:   "他人已升级",
		},
		{
			name: "own upgraded ok",
			task: state.FmlRaceTaskView{MsId: 7, TaskId: 3036, TaskType: 3036, Score: 20, ParamID: 23001, IsUpgrade: 1, UpgradeUid: uid},
			policy: func() *pb.UnionRacePolicy {
				p := takeablePlant()
				p.ExcludeOthersUpgradeTask = true
				return p
			}(),
			want: "",
		},
		{
			name:   "plant not cultivated",
			task:   state.FmlRaceTaskView{MsId: 8, TaskId: 3036, TaskType: 3036, Score: 30, ParamID: 23999},
			policy: policyBase(),
			want:   "目标花卉未培养",
		},
		{
			name:   "plant missing param",
			task:   state.FmlRaceTaskView{MsId: 14, TaskId: 3036, TaskType: 3036, Score: 30, ParamID: 0},
			policy: policyBase(),
			want:   "目标花卉未培养",
		},
		{
			name:   "plant cultivated ok",
			task:   state.FmlRaceTaskView{MsId: 9, TaskId: 3036, TaskType: 3036, Score: 30, ParamID: 23001},
			policy: policyBase(),
			want:   "",
		},
		{
			name:   "type priority zero",
			task:   state.FmlRaceTaskView{MsId: 15, TaskId: 3017, TaskType: 3017, Score: 24},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3017: 0}},
			want:   "优先级为0",
		},
		{
			name:   "unsupported despite positive priority",
			task:   state.FmlRaceTaskView{MsId: 16, TaskId: 3017, TaskType: 3017, Score: 24},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3017: 3}},
			want:   "暂不支持自动完成",
		},
		{
			name:   "customer order takeable with priority",
			task:   state.FmlRaceTaskView{MsId: 20, TaskId: 3016, TaskType: 3016, Score: 24},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3016: 4}},
			want:   "",
		},
		{
			name:   "customer order priority zero",
			task:   state.FmlRaceTaskView{MsId: 21, TaskId: 3016, TaskType: 3016, Score: 24},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3016: 0}},
			want:   "优先级为0",
		},
		{
			name:   "default zero type skipped when map empty",
			task:   state.FmlRaceTaskView{MsId: 17, TaskId: 3017, TaskType: 3017, Score: 24},
			policy: &pb.UnionRacePolicy{},
			want:   "优先级为0",
		},
		{
			name: "priority: taken wins over CD",
			task: state.FmlRaceTaskView{
				MsId: 10, TaskId: 3030, TaskType: 3030, Score: 20, UID: 7,
				AppearTime: now.Add(time.Hour).UnixMilli(),
			},
			policy: policyBase(),
			want:   "已被接取",
		},
		{
			name: "priority: CD time copy over score detail",
			task: state.FmlRaceTaskView{
				MsId: 11, TaskId: 3030, TaskType: 3030, Score: 5,
				AppearTime: now.Add(time.Hour).UnixMilli(),
			},
			policy: &pb.UnionRacePolicy{MinTaskScore: 20},
			want:   time.UnixMilli(now.Add(time.Hour).UnixMilli()).Local().Format("15:04:05") + " 后刷新",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RaceTakeSkipReason(s, tc.task, tc.policy, uid, now, true)
			if got != tc.want {
				t.Fatalf("RaceTakeSkipReason = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestUnionRacePeriodicGetTaskListAfterTTL(t *testing.T) {
	s := state.New()
	// Empty pool: TasksObserved, nothing to take.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4},"114":[]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	if synced <= 0 {
		t.Fatal("need TasksSyncedAtMs from apply")
	}
	policy := testRacePolicy()
	now := time.UnixMilli(synced).Add(raceTaskPoolRefreshInterval + time.Second)
	ops := unionRaceOperations(s, policy, 0, now, true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected periodic getTaskList, got %+v", ops)
	}
}

func TestUnionRaceNoPeriodicGetTaskListWithinTTL(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4},"114":[]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	policy := testRacePolicy()
	now := time.UnixMilli(synced).Add(raceTaskPoolRefreshInterval - time.Second)
	ops := unionRaceOperations(s, policy, 0, now, true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("unexpected getTaskList within TTL: %+v", ops)
		}
	}
}

func TestUnionRaceTakeWinsOverPeriodicSync(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3036, 20, 0, 0}})
	synced := s.FmlRace().TasksSyncedAtMs
	plannerNow := time.UnixMilli(synced).Add(raceTaskPoolRefreshInterval + time.Second)
	ops := unionRaceOperations(s, testRacePolicy(), 0, plannerNow, true)
	if len(ops) < 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take first, got %+v", ops)
	}
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("take tick must not also sync, got %+v", ops)
		}
	}
}

// raceStateAtTTL returns state whose pool has AppearTime = plannerNow+appearRem,
// and plannerNow is exactly TasksSyncedAtMs + refreshInterval + 1s (TTL due).
func raceStateAtTTL(t *testing.T, appearRem time.Duration, plantParam int32, taskUID int64) (*state.State, time.Time) {
	t.Helper()
	s := state.New()
	if plantParam > 0 {
		s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(plantParam)}})
	}
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":20}]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	plannerNow := time.UnixMilli(synced).Add(raceTaskPoolRefreshInterval + time.Second)
	appear := plannerNow.Add(appearRem).UnixMilli()
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"114":[{"0":1,"4":3036,"5":%d,"6":[%d],"10":20,"12":%d,"14":0,"15":0}]}}`,
		appear, plantParam, taskUID,
	)))
	delta := s.FmlRace().TasksSyncedAtMs - synced
	plannerNow = plannerNow.Add(time.Duration(delta) * time.Millisecond)
	return s, plannerNow
}

func TestUnionRacePeriodicRunsDespiteNearTakenCD(t *testing.T) {
	s, now := raceStateAtTTL(t, 5*time.Minute, 23001, 99)
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("near taken CD must not defer periodic sync, got %+v", ops)
	}
}

func TestUnionRacePeriodicDeferredForNearTakeableCD(t *testing.T) {
	// Only the final approach window suppresses TTL sync; 20s is inside it.
	s, now := raceStateAtTTL(t, 20*time.Second, 23001, 0)
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("near takeable CD must defer periodic sync, got %+v", ops)
		}
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("outside lead must not take yet, got %+v", ops)
		}
	}
}

func TestUnionRacePeriodicRunsDespiteMidTakeableCD(t *testing.T) {
	// 5m remaining used to suppress sync (tied to 10m TTL); keep refreshing so
	// mid-wait upgrades/claims are observed before AppearTime.
	s, now := raceStateAtTTL(t, 5*time.Minute, 23001, 0)
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("mid takeable CD must not block periodic sync, got %+v", ops)
	}
}

func TestUnionRacePeriodicRunsDespiteFarTakeableCD(t *testing.T) {
	s, now := raceStateAtTTL(t, 15*time.Minute, 23001, 0)
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("far takeable CD must not block periodic sync, got %+v", ops)
	}
}

func TestUnionRaceNoTakeWhenTakenSynthesizedFromPoolUID(t *testing.T) {
	s := state.New()
	// Cultivate 23001 so the free alternate plant-harvest (msId 56) is genuinely
	// takeable — a HasTask-guard regression would emit takeTask.
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	// No 110 takeTaskData; pool marks uid=999 on msId=55. Another free task exists.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"117":{"5":4},"114":[{"0":55,"4":4012,"6":[23363],"7":100,"8":10,"10":25,"12":999},{"0":56,"4":4001,"6":[23001],"7":50,"8":0,"10":30,"12":0}],"110":{}}}`))
	if !s.FmlRace().Taken.HasTask {
		t.Fatal("expected synthesized taken")
	}
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now(), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take while holding synthesized task, ops=%+v", ops)
		}
	}
}

func TestUnionRaceNoFinishWhenTakenProgressUnknown(t *testing.T) {
	s := state.New()
	// Synthesized taken with TargetCnt=0/FinishCnt=0 must not finish.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"117":{"5":4},"114":[{"0":55,"4":4012,"6":[23363],"10":25,"12":999}],"110":{}}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now(), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceFinishTask.String() {
			t.Fatalf("must not finish when TargetCnt unknown, ops=%+v", ops)
		}
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take while HasTask, ops=%+v", ops)
		}
	}
}

func TestUnionRaceGiveUpSynthesizedTakenPriorityZero(t *testing.T) {
	s := state.New()
	// Synthesized taken from pool UID: task type 3017 (priority 0), no progress.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"117":{"5":4},"114":[{"0":55,"4":3017,"7":10,"8":0,"10":25,"12":999}],"110":{}}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{
		3017: 0,
		3036: 5,
	}
	ops := unionRaceOperations(s, policy, 999, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGiveUpTask.String() {
		t.Fatalf("expected giveUp for priority-0 synthesized taken, got %+v", ops)
	}
}

func TestUnionRaceTakesCustomerOrderWhenEnabled(t *testing.T) {
	s := state.New()
	// Catalog task 3019 has type 3016 (顾客订单). Bare 3016 is a different task.
	applyRaceState(s, [][5]int32{{1, 3019, 24, 0, 0}})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3016: 4, 3036: 5}
	ops := unionRaceOperations(s, policy, 0, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take of customer-order race task, got %+v", ops)
	}
	if ops[0].TaskMsID != 1 || ops[0].TaskID != 3016 {
		t.Fatalf("take detail = msId=%d taskID=%d, want 1/3016", ops[0].TaskMsID, ops[0].TaskID)
	}
}

func TestUnionRaceSkipsCustomerOrderWhenModuleOff(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3019, 24, 0, 0}})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3016: 4}
	ops := unionRaceOperations(s, policy, 0, time.Now(), false)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take customer-order race without customer module: %+v", op)
		}
	}
	got := RaceTakeSkipReason(s, state.FmlRaceTaskView{MsId: 1, TaskId: 3019, TaskType: 3016, Score: 24}, policy, 0, time.Now(), false)
	if got != "顾客订单模块未开启" {
		t.Fatalf("RaceTakeSkipReason = %q, want 顾客订单模块未开启", got)
	}
}

func TestUnionRaceFinishesCompletedCustomerOrder(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3019,"2":5,"3":5}}},"114":[{"0":71,"4":3019,"7":5,"8":5,"10":24,"12":999}]}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3016: 4}
	ops := unionRaceOperations(s, policy, 999, time.Now(), true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceFinishTask.String() {
		t.Fatalf("expected finish of completed customer-order race, got %+v", ops)
	}
	if ops[0].TaskMsID != 71 || ops[0].TaskID != 3016 || ops[0].Priority != raceCustomerFinishPriority {
		t.Fatalf("finish op = %+v, want msId=71 type=3016 prio=%d", ops[0], raceCustomerFinishPriority)
	}
}

func TestUnionRaceGiveUpCustomerOrderWhenModuleOff(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3019,"2":5,"3":0}}},"114":[{"0":71,"4":3019,"7":5,"8":0,"10":24,"12":999}]}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3016: 4}
	ops := unionRaceOperations(s, policy, 999, time.Now(), false)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGiveUpTask.String() {
		t.Fatalf("expected giveUp when customer module off, got %+v", ops)
	}
}

func TestUnionRaceCustomerProgressSyncAfterInterval(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3019,"2":5,"3":1}}},"114":[{"0":71,"4":3019,"7":5,"8":1,"10":24,"12":999}]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3016: 4}
	now := time.UnixMilli(synced).Add(raceCustomerProgressSyncInterval + time.Second)
	ops := unionRaceOperations(s, policy, 999, now, true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected customer progress sync, got %+v taken=%+v", ops, s.FmlRace().Taken)
	}
	if ops[0].Priority != raceCustomerSyncPriority {
		t.Fatalf("sync priority=%d, want %d", ops[0].Priority, raceCustomerSyncPriority)
	}
}

func TestRaceNeedsFinishProgressSyncCooldown(t *testing.T) {
	view := state.FmlRaceView{
		TasksObserved:       true,
		TasksSyncedAtMs:     1_700_000_000_000,
		Taken:               state.FmlRaceTakenView{HasTask: true, TaskMsId: 715, TargetCnt: 300, FinishCnt: 48},
		LocalFinishTaskMsId: 715,
		LocalFinishCnt:      300,
	}
	within := time.UnixMilli(view.TasksSyncedAtMs).Add(time.Second)
	if raceNeedsFinishProgressSync(view, within) {
		t.Fatal("must not re-sync within raceFinishProgressSyncInterval")
	}
	due := time.UnixMilli(view.TasksSyncedAtMs).Add(raceFinishProgressSyncInterval + time.Second)
	if !raceNeedsFinishProgressSync(view, due) {
		t.Fatal("must sync after raceFinishProgressSyncInterval")
	}
}

func TestUnionRaceFinishProgressSyncRespectsCooldown(t *testing.T) {
	s := state.New()
	// Held plant-harvest at 48/300; field 134 raises LocalFinishCnt to 300.
	// Include 117 raceLvl so planner does not divert to enter for tier sync.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":1785081600000,"1":1,"2":1000,"3":9000},"117":{"5":4},"110":{"1785081600000":{"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}},"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":48,"10":28,"12":0}]}}`))
	s.ApplyV(json.RawMessage(`{"25":{"134":{"1785081600000":{"3":{"0":715,"1":4013,"2":300,"3":300,"4":[23577]},"4":1785358559363}}}}`))
	// Re-apply pool so FinishCnt stays lagging at 48 while LocalFinish is 300.
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":48,"10":28,"12":0}],"110":{"1785081600000":{"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}}}}`))
	got := s.FmlRace()
	if got.LocalFinishCnt < 300 || got.Taken.FinishCnt != 48 {
		t.Fatalf("seed local=%d finish=%d, want local>=300 finish=48", got.LocalFinishCnt, got.Taken.FinishCnt)
	}
	policy := testRacePolicy()
	now := time.UnixMilli(got.TasksSyncedAtMs).Add(time.Second)
	ops := unionRaceOperations(s, policy, 999, now, true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("finish-progress sync must respect cooldown, got %+v", ops)
		}
	}
	later := time.UnixMilli(got.TasksSyncedAtMs).Add(raceFinishProgressSyncInterval + time.Second)
	ops = unionRaceOperations(s, policy, 999, later, true)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected finish-progress sync after cooldown, got %+v", ops)
	}
	// One authoritative getTaskList that still lags must clamp LocalFinish so
	// the planner does not spin sync every raceFinishProgressSyncInterval.
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"25":{"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":48,"10":28,"12":0}],"110":{"1785081600000":{"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}}}}`))
	afterSync := s.FmlRace()
	if afterSync.LocalFinishCnt != 48 {
		t.Fatalf("LocalFinishCnt=%d after full pool, want clamped 48", afterSync.LocalFinishCnt)
	}
	ops = unionRaceOperations(s, policy, 999, time.UnixMilli(afterSync.TasksSyncedAtMs).Add(raceFinishProgressSyncInterval+time.Second), true)
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("must not keep finish-progress syncing after clamp, got %+v", ops)
		}
	}
}

func TestBuildPlan_RaceCustomerOrderLinksFinish(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999,"32":{"23005":10}}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3019,"2":5,"3":1}}},"114":[{"0":71,"4":3019,"7":5,"8":1,"10":24,"12":999}]},"109":{"0":{"1":{"10":{"0":[[23005,1]],"1":10}},"2":` + fmt.Sprintf("%d", now.Add(time.Hour).UnixMilli()) + `}}}`))
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Union.Race.Enabled = true
	p.Union.Race.AutoEnableModules = true
	p.Union.Race.TaskTypePriority = map[int32]int32{3016: 4}

	result := BuildPlan(s, p, now)
	var linked *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() && strings.HasPrefix(op.DemandID, raceActionGoal+":") {
			linked = op
			break
		}
	}
	if linked == nil {
		t.Fatalf("expected race-linked customer finish, ops=%+v demands=%+v taken=%+v", result.Operations, result.Demands, s.FmlRace().Taken)
	}
	if !strings.Contains(linked.Reason, "公会竞赛顾客订单剩余") {
		t.Fatalf("reason missing race pressure: %q", linked.Reason)
	}
}
