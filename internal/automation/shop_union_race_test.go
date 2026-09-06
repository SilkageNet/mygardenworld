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
	"google.golang.org/protobuf/proto"
)

// raceStateJSON builds a namespace-25 JSON blob with the given race task pool.
// tasks is a slice of [msId, taskId, score, isUpgrade, upgradeUid].
// Plant-harvest rows (type/id 3036) include param [23001] and flower-art craft
// rows (type/id 3034) include vase param [3002].
// Fields are at the TOP LEVEL of ns25 (not nested under "0").
func raceStateJSON(tasks [][5]int32) string {
	return raceStateJSONWithParams(tasks, 23001)
}

func raceStateJSONWithParams(tasks [][5]int32, plantParam int32) string {
	const craftVaseParam int32 = 3002
	pool := "[]"
	if len(tasks) > 0 {
		parts := make([]string, 0, len(tasks))
		for _, t := range tasks {
			taskID := t[1]
			taskType := state.FmlRaceTaskTypeByID(taskID)
			if taskType == raceTaskTypePlantHarvest && plantParam > 0 {
				parts = append(parts, fmt.Sprintf(
					`{"0":%d,"4":%d,"6":[%d],"10":%d,"14":%d,"15":%d}`,
					t[0], taskID, plantParam, t[2], t[3], t[4],
				))
				continue
			}
			if taskType == raceTaskTypeFlowerArtCraft {
				parts = append(parts, fmt.Sprintf(
					`{"0":%d,"4":%d,"6":[%d],"10":%d,"14":%d,"15":%d}`,
					t[0], taskID, craftVaseParam, t[2], t[3], t[4],
				))
				continue
			}
			parts = append(parts, fmt.Sprintf(`{"0":%d,"4":%d,"10":%d,"14":%d,"15":%d}`, t[0], taskID, t[2], t[3], t[4]))
		}
		pool = "[" + strings.Join(parts, ",") + "]"
	}
	return `{"25":{"111":{"0":42,"1":1},"117":{"5":4},"110":{"42":{"3":0,"4":0}},"114":` + pool + `}}`
}

// applyRaceState seeds cultivated flower 23001, unlocked vase 3002, and an
// active race task pool.
func applyRaceState(s *state.State, tasks [][5]int32) {
	s.ApplyVMap(map[string]any{
		"101": map[string]any{"0": cultivate(23001)},
		"102": map[string]any{"0": map[string]any{"3002": map[string]any{"1": 3002}}},
	})
	s.ApplyV(json.RawMessage(raceStateJSON(tasks)))
	// Bind rank-list uid to self so ScoreObserved/RankObserved stick and
	// getFmlRaceUsrRankList does not preempt take/finish in unit tests.
	uid := s.RoleID()
	if uid <= 0 {
		uid = 999
		s.ApplyV(json.RawMessage(fmt.Sprintf(`{"7":{"0":{"0":%d}}}`, uid)))
	}
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"116":[{"0":%d,"1":42,"3":0,"4":0}],"110":{"42":{"0":%d,"3":0,"4":0}}}}`,
		uid, uid,
	)))
}

func applyRaceDeletePosition(s *state.State, position int32) {
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"1":{"0":999,"1":42,"2":%d}}}`,
		position,
	)))
}

// testRacePolicy returns a policy with the common defaults for race tests:
// enabled, automatic completion and explicit automatic give-up on, with no
// score filtering. Product defaults keep both mutating switches off.
func testRacePolicy() *pb.UnionRacePolicy {
	return &pb.UnionRacePolicy{
		Enabled:           true,
		AutoEnableModules: true,
		AutoGiveUpTask:    true,
		MinTaskScore:      0,
	}
}

func testEnabledRaceFullPolicy() *pb.Policy {
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Race = testRacePolicy()
	p.Union.Race.TaskTypePriority = defaultUnionRacePriority()
	return p
}

func raceGatesOn() RaceModuleGates {
	return RaceModuleGates{Customer: true, Pearl: true, Cultivate: true}
}

func raceGatesNoCustomer() RaceModuleGates {
	return RaceModuleGates{Customer: false, Pearl: true, Cultivate: true}
}

func raceGatesNoPearl() RaceModuleGates {
	return RaceModuleGates{Customer: true, Pearl: false, Cultivate: true}
}

func raceGatesNoCultivate() RaceModuleGates {
	return RaceModuleGates{Customer: true, Pearl: true, Cultivate: false}
}

func TestUnionRaceDisabledProducesNoOps(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{{1, 3036, 10, 0, 0}})))
	policy := &pb.UnionRacePolicy{Enabled: false}
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 0 {
		t.Fatalf("expected 0 ops when disabled, got %d: %+v", len(ops), ops)
	}
}

func TestManualRaceTakeOperationUsesObservedPolicyGates(t *testing.T) {
	now := time.Now()
	newState := func() *state.State {
		s := state.New()
		applyRaceState(s, [][5]int32{{1, 3036, 36, 0, 0}})
		return s
	}

	t.Run("ready task", func(t *testing.T) {
		s := newState()
		policy := testEnabledRaceFullPolicy()
		// An explicit click is permitted while automatic completion is off.
		policy.Union.Race.AutoEnableModules = false
		op, err := ManualRaceTakeOperation(s, policy, 1, now)
		if err != nil {
			t.Fatalf("ManualRaceTakeOperation: %v", err)
		}
		if op.Kind != clientproto.RPCFmlRaceTakeTask.String() || op.TaskMsID != 1 || !op.PreemptFarm {
			t.Fatalf("unexpected manual take op: %+v", op)
		}
	})

	t.Run("future cooldown", func(t *testing.T) {
		s := newState()
		s.ApplyV(json.RawMessage(fmt.Sprintf(
			`{"25":{"114":[{"0":1,"4":3036,"5":%d,"6":[23001],"10":36}]}}`,
			now.Add(time.Minute).UnixMilli(),
		)))
		if _, err := ManualRaceTakeOperation(s, testEnabledRaceFullPolicy(), 1, now); err == nil || !strings.Contains(err.Error(), "冷却中") {
			t.Fatalf("expected cooldown error, got %v", err)
		}
	})

	t.Run("score filter", func(t *testing.T) {
		s := newState()
		policy := testEnabledRaceFullPolicy()
		policy.Union.Race.MinTaskScore = 36
		if _, err := ManualRaceTakeOperation(s, policy, 1, now); err == nil || !strings.Contains(err.Error(), "分数不足") {
			t.Fatalf("expected score filter error, got %v", err)
		}
	})

	t.Run("progressed task follows policy", func(t *testing.T) {
		s := newState()
		s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":3036,"6":[23001],"7":10,"8":3,"10":36}]}}`))
		policy := testEnabledRaceFullPolicy()
		policy.Union.Race.AvoidProgressedTasks = proto.Bool(true)
		if _, err := ManualRaceTakeOperation(s, policy, 1, now); err == nil || !strings.Contains(err.Error(), "已有进度（3/10）") {
			t.Fatalf("expected progressed-task policy error, got %v", err)
		}
		policy.Union.Race.AvoidProgressedTasks = proto.Bool(false)
		if _, err := ManualRaceTakeOperation(s, policy, 1, now); err != nil {
			t.Fatalf("explicitly allowed progressed task: %v", err)
		}
	})

	t.Run("already holding", func(t *testing.T) {
		s := newState()
		s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"7":{"0":1,"1":3036,"2":10,"3":0,"4":[23001]}}}}}`))
		if _, err := ManualRaceTakeOperation(s, testEnabledRaceFullPolicy(), 1, now); err == nil || !strings.Contains(err.Error(), "已有竞赛任务") {
			t.Fatalf("expected held-task error, got %v", err)
		}
	})
}

func TestManualRaceDeleteOperationUsesCurrentPermissionAndTaskState(t *testing.T) {
	now := time.Now()
	newState := func(position int32) *state.State {
		s := state.New()
		applyRaceState(s, [][5]int32{{1, 3036, 300, 0, 0}})
		applyRaceDeletePosition(s, position)
		return s
	}

	t.Run("explicit high score delete ignores automatic threshold", func(t *testing.T) {
		s := newState(1)
		policy := testEnabledRaceFullPolicy()
		policy.Union.Race.DeleteLowScoreTask = false
		policy.Union.Race.DeleteTaskMaxScore = 10
		op, err := ManualRaceDeleteOperation(s, policy, 1, now)
		if err != nil {
			t.Fatalf("ManualRaceDeleteOperation: %v", err)
		}
		if op.Kind != clientproto.RPCFmlRaceDelTask.String() || op.TaskMsID != 1 || op.CooldownKey != "union.race.delete:1" {
			t.Fatalf("unexpected manual delete op: %+v", op)
		}
	})

	t.Run("position without delete permission", func(t *testing.T) {
		if _, err := ManualRaceDeleteOperation(newState(3), testEnabledRaceFullPolicy(), 1, now); err == nil || !strings.Contains(err.Error(), "没有删除权限") {
			t.Fatalf("expected permission error, got %v", err)
		}
	})

	t.Run("claimed task", func(t *testing.T) {
		s := newState(2)
		s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":3036,"6":[23001],"10":300,"12":123}]}}`))
		if _, err := ManualRaceDeleteOperation(s, testEnabledRaceFullPolicy(), 1, now); err == nil || !strings.Contains(err.Error(), "已被成员接取") {
			t.Fatalf("expected claimed-task error, got %v", err)
		}
	})

	t.Run("replacement slot cooldown", func(t *testing.T) {
		s := newState(2)
		s.ApplyV(json.RawMessage(fmt.Sprintf(
			`{"25":{"114":[{"0":1,"4":3036,"5":%d,"6":[23001],"10":300}]}}`,
			now.Add(time.Minute).UnixMilli(),
		)))
		if _, err := ManualRaceDeleteOperation(s, testEnabledRaceFullPolicy(), 1, now); err == nil || !strings.Contains(err.Error(), "冷却中") {
			t.Fatalf("expected cooldown error, got %v", err)
		}
	})
}

func TestUnionRaceEnterIsExecutable(t *testing.T) {
	s := state.New()
	// Real startup baselines may include the guild object (25.0) while omitting
	// the current-member object (25.1). Finalization must preserve membership.
	s.ApplyV(json.RawMessage(`{"25":{"0":{"0":88}}}`))
	s.FinalizeFmlMembershipSnapshot()
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
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

func TestUnionRaceWithoutGuildProducesNoOps(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"0":{}}}`))
	policy := testRacePolicy()
	if ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn()); len(ops) != 0 {
		t.Fatalf("account without guild must not run race logic: %+v", ops)
	}
	full := testEnabledRaceFullPolicy()
	if RaceBootstrapDue(s, full, time.Date(2026, time.August, 25, 10, 0, 0, 0, time.Local)) {
		t.Fatal("account without guild must not trigger urgent race bootstrap")
	}
}

func TestUnionRaceIgnoresStaleRaceSnapshotAfterLeavingGuild(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"0":{"0":88},"1":null,"111":{"0":1787658000000,"1":1,"2":1787658000000,"3":1788109200000},"114":[{"0":1,"4":3036,"10":36}]}}`))
	policy := testRacePolicy()
	if ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn()); len(ops) != 0 {
		t.Fatalf("stale race snapshot after leaving guild must be ignored: %+v", ops)
	}
	full := testEnabledRaceFullPolicy()
	if RaceBootstrapDue(s, full, time.Now()) {
		t.Fatal("stale race snapshot after leaving guild must not trigger bootstrap")
	}
}

func TestUnionRaceEmptyEnterProbeBacksOffWithoutStarvingOtherWork(t *testing.T) {
	now := time.Date(2026, time.August, 25, 10, 0, 0, 0, time.Local)
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"0":{"0":88}}}`))
	s.MarkFmlRaceLvlSyncAttemptAt(now)
	policy := testRacePolicy()

	ops := unionRaceOperations(s, policy, 0, now.Add(time.Second), raceGatesOn())
	if len(ops) != 0 {
		t.Fatalf("recent successful enter probe must back off, got %+v", ops)
	}
	full := testEnabledRaceFullPolicy()
	if RaceBootstrapDue(s, full, now.Add(time.Second)) {
		t.Fatal("recent empty enter probe must not keep urgent bootstrap awake")
	}

	ops = unionRaceOperations(s, policy, 0, now.Add(raceEnterProbeInterval+time.Second), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceEnter.String() {
		t.Fatalf("enter probe must become eligible after backoff, got %+v", ops)
	}
}

func TestUnionRaceEnterNotEmittedWhenObserved(t *testing.T) {
	s := state.New()
	// Race data present → Observed=true → no enter op.
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{{1, 3036, 10, 0, 0}})))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceEnter.String() {
			t.Fatalf("should not emit enter when race data already observed")
		}
	}
}

func TestUnionRaceAutoModulesOffProducesNoOps(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3036, 10, 0, 0}})
	policy := &pb.UnionRacePolicy{Enabled: true, AutoEnableModules: false, MinTaskScore: 28}
	ops := unionRaceOperations(s, policy, s.RoleID(), time.Now(), raceGatesOn())
	if len(ops) != 0 {
		t.Fatalf("expected 0 ops when autoEnableModules off, got %d: %+v", len(ops), ops)
	}
}

func TestUnionRaceDoesNotGiveUpWithoutExplicitOptIn(t *testing.T) {
	s := state.New()
	// A low-score task may have been taken manually in the game client. Automatic
	// completion must not silently authorize releasing it.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":5,"12":999}],"110":{"999":{"7":{"0":1,"1":3036,"2":60,"3":0,"4":[23001]}}}}}`))
	policy := testRacePolicy()
	policy.AutoGiveUpTask = false
	policy.MinTaskScore = 28

	for _, op := range unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn()) {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("manual task must be kept when auto give-up is off: %+v", op)
		}
	}
}

func TestUnionRaceAutoGiveUpIsIndependentFromAutoComplete(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":5,"12":999}],"110":{"999":{"7":{"0":1,"1":3036,"2":60,"3":0,"4":[23001]}}}}}`))
	policy := testRacePolicy()
	policy.AutoEnableModules = false
	policy.AutoGiveUpTask = true
	policy.MinTaskScore = 28

	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGiveUpTask.String() {
		t.Fatalf("explicit auto give-up should work independently, got %+v", ops)
	}
}

func TestUnionRaceAutoModulesOffStillSyncsAndRefreshes(t *testing.T) {
	// Enabled + !AutoEnableModules: observe/sync the pool for UI, but never take.
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"0":{"0":88}}}`))
	policy := &pb.UnionRacePolicy{Enabled: true, AutoEnableModules: false}
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceEnter.String() {
		t.Fatalf("expected enter sync when modules off, got %+v", ops)
	}

	s = state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4}}}`))
	ops = unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected getTaskList sync when modules off, got %+v", ops)
	}

	s = state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4},"114":[]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	now := time.UnixMilli(synced).Add(raceTaskPoolRefreshInterval + time.Second)
	ops = unionRaceOperations(s, policy, 0, now, raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take before quota exhausted, got %+v", ops)
	}
	s.MarkFmlRaceTakeQuotaExhausted()
	ops = unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take when free quota done, got %+v", ops)
		}
	}

	// With the switch off, keep taking until the server rejects.
	policy.AutoStopOnQuotaDone = false
	ops = unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take when auto-stop off, got %+v", ops)
	}

	// Remaining free quota still allows take while auto-stop is on.
	s = state.New()
	applyRaceState(s, [][5]int32{{1, 3036, 10, 0, 0}})
	s.ApplyV(json.RawMessage(`{"25":{"110":{"1":{"3":17}}}}`))
	policy = testRacePolicy()
	policy.AutoStopOnQuotaDone = true
	ops = unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
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

func TestUnionRaceGiveUpLowScoreEvenWithProgress(t *testing.T) {
	s := state.New()
	// Low score + FinishCnt>0 → still give up (do not plant a sub-threshold hold to completion).
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":5,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":60,"3":16,"4":[23001]}}}}}`))
	policy := testRacePolicy()
	policy.MinTaskScore = 24
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
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
		t.Fatalf("expected giveUp for low-score taken task with progress, got %+v", ops)
	}
}

func TestUnionRaceGiveUpTakenMissingFromPool(t *testing.T) {
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	// Taken msId=99 not present in observed pool; completable plant-harvest, FinishCnt=0 → give up for pool gap.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":30,"14":0,"15":0}],"110":{"999":{"7":{"0":99,"1":3036,"2":280,"3":0,"4":[23001]}}}}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now(), raceGatesOn())
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
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	// Completable hold missing from pool with FinishCnt>0 → keep (avoid dropping on a transient pool gap).
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":30}],"110":{"999":{"7":{"0":99,"1":3036,"2":280,"3":12,"4":[23001]}}}}}`))
	for _, op := range unionRaceOperations(s, testRacePolicy(), 999, time.Now(), raceGatesOn()) {
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
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn()) // current uid 999
	if len(ops) != 1 {
		t.Fatalf("expected 1 take op, got %d", len(ops))
	}
	if ops[0].TaskMsID != 2 {
		t.Fatalf("expected taskMsId 2 (exclude uid-100 upgraded task), got %d", ops[0].TaskMsID)
	}

	// System upgrade (IsUpgrade=1, UpgradeUid=0) remains takeable.
	s2 := state.New()
	s2.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s2.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4},"114":[
		{"0":1,"4":3036,"6":[23001],"10":28,"14":1,"15":0},
		{"0":2,"4":3036,"6":[23001],"10":10,"14":0,"15":0}
	]}}`))
	ops2 := unionRaceOperations(s2, policy, 999, time.Now(), raceGatesOn())
	if len(ops2) != 1 || ops2[0].TaskMsID != 1 {
		t.Fatalf("expected take system-upgraded msId 1, got %+v", ops2)
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
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 {
		t.Fatalf("expected 1 take op, got %d", len(ops))
	}
	if ops[0].TaskMsID != 4 {
		t.Fatalf("expected taskMsId 4 (upgraded, score > 5), got %d", ops[0].TaskMsID)
	}
}

func TestUnionRaceBatchInactiveProducesNoOps(t *testing.T) {
	s := state.New()
	// Ended batch (status=2) with closed window → no race ops off-season.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":9,"1":2,"2":1000,"3":2000},"117":{"5":4},"114":[{"0":1,"4":3036,"10":10,"14":0,"15":0}]}}`))
	policy := testRacePolicy()
	monday := time.Date(2026, 8, 17, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	ops := unionRaceOperations(s, policy, 0, monday, raceGatesOn())
	if len(ops) != 0 {
		t.Fatalf("expected 0 ops when batch inactive, got %d: %+v", len(ops), ops)
	}
}

func TestUnionRaceEnterAtWeeklyOpen(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":9,"1":2,"2":1000,"3":2000},"117":{"5":4}}}`))
	open := time.Date(2026, 8, 18, 9, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	ops := unionRaceOperations(s, testRacePolicy(), 0, open, raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceEnter.String() {
		t.Fatalf("expected enter at Tuesday 09:00, got %+v", ops)
	}
	if ops[0].CooldownKey != "union.race.enter.batch" {
		t.Fatalf("cooldown key = %q", ops[0].CooldownKey)
	}
}

func TestUnionRaceNoEnterBeforeWeeklyOpen(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":9,"1":2,"2":1000,"3":2000},"117":{"5":4}}}`))
	before := time.Date(2026, 8, 18, 8, 59, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	ops := unionRaceOperations(s, testRacePolicy(), 0, before, raceGatesOn())
	if len(ops) != 0 {
		t.Fatalf("expected no enter before Tuesday 09:00, got %+v", ops)
	}
}

func TestUnionRaceEnterWhenPublishedStartArrives(t *testing.T) {
	s := state.New()
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	start := time.Date(2026, 8, 18, 9, 0, 0, 0, loc)
	end := time.Date(2026, 8, 23, 21, 0, 0, 0, loc)
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"111":{"0":42,"1":0,"2":%d,"3":%d},"117":{"5":4}}}`,
		start.UnixMilli(), end.UnixMilli(),
	)))
	ops := unionRaceOperations(s, testRacePolicy(), 0, start, raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceEnter.String() {
		t.Fatalf("expected enter when published start arrives, got %+v", ops)
	}
}

func TestUnionRaceInactiveEnterWaitsForRetry(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":9,"1":2,"2":1000,"3":2000},"117":{"5":4}}}`))
	open := time.Date(2026, 8, 18, 9, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s.MarkFmlRaceLvlSyncAttemptAt(open)
	ops := unionRaceOperations(s, testRacePolicy(), 0, open.Add(10*time.Second), raceGatesOn())
	if len(ops) != 0 {
		t.Fatalf("expected no enter within retry interval, got %+v", ops)
	}
	ops = unionRaceOperations(s, testRacePolicy(), 0, open.Add(raceInactiveEnterRetryInterval), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceEnter.String() {
		t.Fatalf("expected enter after retry interval, got %+v", ops)
	}
}

func TestUnionRaceEnterEmittedWhenOnlyTaskStubsObserved(t *testing.T) {
	s := state.New()
	// Task pool / usr stubs without a real CurFmlRaceBatch must still trigger enter.
	s.ApplyV(json.RawMessage(`{"25":{"0":{"0":88},"114":[{"0":1,"4":3036,"10":10,"14":0,"15":0}],"110":{}}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceEnter.String() {
		t.Fatalf("expected enter when batch not synced, got %+v", ops)
	}
}

func TestUnionRaceGetTaskListAfterActiveBatch(t *testing.T) {
	s := state.New()
	// Enter response carries batch 111 but not task pool 114.
	// Seed fTaskNum so usr-rank quota sync does not preempt getTaskList.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1783872000000,"1":1,"2":1783990800000,"3":1784466000000},"117":{"5":4},"110":{"1783872000000":{"3":0}}}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected getTaskList after enter batch, got %+v", ops)
	}
	if !ops[0].Executable || ops[0].SyncOnly {
		t.Fatalf("getTaskList must be executable, got %+v", ops[0])
	}
}

func TestUnionRaceUnobservedTaskPoolEmptySuccessUsesBoundedRetry(t *testing.T) {
	now := time.UnixMilli(1_783_990_800_000)
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1783872000000,"1":1,"2":1783990800000,"3":1784466000000},"117":{"5":4},"110":{"1783872000000":{"3":0}}}}`))
	s.NoteFmlRaceTaskPoolSync(now)
	policy := testRacePolicy()

	ops := unionRaceOperations(s, policy, 0, now.Add(time.Second), raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("successful empty task-pool sync must back off, got %+v", ops)
		}
	}
	ops = unionRaceOperations(s, policy, 0, now.Add(raceTaskPoolBootstrapRetryInterval), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("task-pool bootstrap must retry after bounded interval, got %+v", ops)
	}
}

func TestUnionRaceGetTaskListWhenPlantHarvestMissingParam(t *testing.T) {
	s := state.New()
	// Observed pool with a plant-harvest row that never got field-6 param detail.
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1783872000000,"1":1,"2":1783990800000,"3":1784466000000},"117":{"5":4},"110":{"1783872000000":{"3":0}},"114":[{"0":178397176088908,"4":4011,"10":25,"14":0,"15":0,"6":[]},{"0":178397176088909,"4":4011,"6":[23001],"10":28,"14":0,"15":0}]}}`))
	if got := s.FmlRace(); !got.TasksObserved || len(got.Tasks) != 2 || got.Tasks[0].ParamID != 0 || got.Tasks[1].ParamID != 23001 {
		t.Fatalf("seed pool = %+v", got)
	}
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected getTaskList to refresh missing flower param, got %+v", ops)
	}

	// A successful getTaskList marks the refresh attempt even when its delta
	// omits field 114 — do not loop on the same incomplete snapshot.
	s.NoteFmlRaceTaskPoolSync(time.Now())
	ops = unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("getTaskList must not re-fire for the same incomplete pool: %+v", ops)
		}
	}
}

func TestUnionRaceGetTaskListWhenFlowerArtCraftMissingVase(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":1783872000000,"1":1,"2":1783990800000,"3":1784466000000},"117":{"5":4},"110":{"1783872000000":{"3":0}},"114":[{"0":178397176088910,"4":3034,"10":24,"14":0,"15":0,"6":[]}]}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3034: 4}
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected getTaskList to refresh missing vase param, got %+v", ops)
	}

	// Regression: affected channels can acknowledge getTaskList without field
	// 114. The successful attempt must suppress another immediate sync while
	// retaining the incomplete row as unselectable.
	now := time.Now()
	s.NoteFmlRaceTaskPoolSync(now)
	ops = unionRaceOperations(s, policy, 0, now.Add(time.Second), raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("empty successful getTaskList must not re-fire next tick: %+v", ops)
		}
	}
}

func TestUnionRaceUpgradeOpEmission(t *testing.T) {
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1},"117":{"5":4},"114":[{"0":1,"4":4001,"6":[23001],"10":9,"12":999,"14":0,"15":0}],"110":{"42":{"3":0,"4":10,"7":{"0":1,"1":4001,"2":10,"3":1,"4":[23001]}}},"116":[{"0":999,"1":42,"3":0,"4":10,"5":1000}]}}`))
	policy := testRacePolicy()
	policy.UpgradeTask = true
	policy.MaxSpendDiamond = 100
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
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

	for _, op := range unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn()) {
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
	applyRaceDeletePosition(s, 2)
	policy := testRacePolicy()
	policy.DeleteLowScoreTask = true
	policy.DeleteTaskMaxScore = 10
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
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
	applyRaceDeletePosition(s, 1)
	policy := testRacePolicy()
	policy.DeleteLowScoreTask = true
	policy.DeleteTaskMaxScore = 10

	for _, op := range unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn()) {
		if op.Kind == clientproto.RPCFmlRaceDelTask.String() {
			t.Fatalf("must not delete an occupied race task: %+v", op)
		}
	}
}

func TestUnionRaceDeleteSkipsUpgradedAndProgressedTasks(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{
		{1, 3036, 5, 1, 88},
		{2, 3036, 6, 0, 0},
	})
	// A sparse progress update makes task 2 unsafe for unattended deletion.
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":2,"4":3036,"6":[23001],"7":10,"8":1,"10":6,"14":0,"15":0}]}}`))
	applyRaceDeletePosition(s, 1)
	policy := testRacePolicy()
	policy.DeleteLowScoreTask = true
	policy.DeleteTaskMaxScore = 10

	for _, op := range unionRaceOperations(s, policy, s.RoleID(), time.Now(), raceGatesOn()) {
		if op.Kind == clientproto.RPCFmlRaceDelTask.String() {
			t.Fatalf("must not automatically delete upgraded or progressed tasks: %+v", op)
		}
	}
}

func TestValidateRaceTaskMutationRejectsChangedScore(t *testing.T) {
	now := time.Now()
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3036, 28, 0, 0}})
	policy := testEnabledRaceFullPolicy()
	policy.Union.Race.MinTaskScore = 27
	op, err := ManualRaceTakeOperation(s, policy, 1, now)
	if err != nil {
		t.Fatal(err)
	}
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(raceStateJSON([][5]int32{{1, 3036, 21, 0, 0}})))
	s.NoteFmlRaceTaskPoolSync(now)

	err = ValidateRaceTaskMutation(s, policy, &op, now)
	if err == nil || !strings.Contains(err.Error(), "原分数 28，当前 21") {
		t.Fatalf("changed score preflight error=%v", err)
	}
	if op.RaceTaskGuard.Current.Score != 21 {
		t.Fatalf("current task evidence=%+v, want score 21", op.RaceTaskGuard.Current)
	}
}

func TestUnionRaceDeleteRunsWhenAutoModulesOff(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3036, 25, 0, 0}})
	applyRaceDeletePosition(s, 2)
	policy := &pb.UnionRacePolicy{
		Enabled:                  true,
		AutoEnableModules:        false,
		DeleteLowScoreTask:       true,
		DeleteTaskMaxScore:       25,
		TaskTypePriority:         map[int32]int32{3036: 0},
		AutoStopOnQuotaDone:      true,
		ExcludeOthersUpgradeTask: true,
	}

	ops := unionRaceOperations(s, policy, s.RoleID(), time.Now(), raceGatesNoCultivate())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceDelTask.String() || ops[0].TaskMsID != 1 {
		t.Fatalf("delete must be independent of auto-complete, got %+v", ops)
	}
	if !ops[0].Executable || ops[0].Status == PlanStatusBlocked {
		t.Fatalf("authorized delete must be executable: %+v", ops[0])
	}
}

func TestUnionRaceDeleteActivelySyncsMissingMemberPosition(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3036, 25, 0, 0}})
	policy := &pb.UnionRacePolicy{
		Enabled:            true,
		DeleteLowScoreTask: true,
		DeleteTaskMaxScore: 25,
	}
	now := time.Now()

	ops := unionRaceOperations(s, policy, s.RoleID(), now, raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlEnter.String() || ops[0].Domain != "union.race.sync" || !ops[0].PreemptFarm {
		t.Fatalf("missing member position must schedule fml.enter, got %+v", ops)
	}

	s.MarkFmlMemberPositionSyncAttemptAt(now)
	ops = unionRaceOperations(s, policy, s.RoleID(), now.Add(time.Minute), raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlEnter.String() {
			t.Fatalf("member sync must back off after an empty attempt: %+v", ops)
		}
	}
	ops = unionRaceOperations(s, policy, s.RoleID(), now.Add(fmlMemberPositionSyncInterval), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlEnter.String() {
		t.Fatalf("member sync must retry after backoff, got %+v", ops)
	}
}

func TestUnionRacePositionSyncPreemptsUnobservedForest(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"102": map[string]any{},
			"111": map[string]any{"0": 42, "1": 1},
			"114": []any{},
		},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.ForestEnabled = true
	p.Union.Race.DeleteLowScoreTask = true
	p.Union.Race.DeleteTaskMaxScore = 25

	op := Plan(s, p, time.Now())
	if op == nil || op.Kind != clientproto.RPCFmlEnter.String() || op.Domain != "union.race.sync" || !op.Executable || op.Status != PlanStatusManaged {
		t.Fatalf("position bootstrap must run before forest refresh, got %+v", op)
	}
}

func TestUnionRaceDeleteRequiresObservedPermission(t *testing.T) {
	tests := []struct {
		name       string
		position   int32
		observed   bool
		executable bool
	}{
		{name: "president", position: 1, observed: true, executable: true},
		{name: "vice president", position: 2, observed: true, executable: true},
		{name: "director", position: 5, observed: true, executable: false},
		{name: "elite", position: 3, observed: true, executable: false},
		{name: "member", position: 4, observed: true, executable: false},
		{name: "unknown", observed: false, executable: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := state.New()
			applyRaceState(s, [][5]int32{{1, 3036, 10, 0, 0}})
			if tc.observed {
				applyRaceDeletePosition(s, tc.position)
			} else {
				s.MarkFmlMemberPositionSyncAttemptAt(time.Now())
			}
			policy := &pb.UnionRacePolicy{
				Enabled:            true,
				DeleteLowScoreTask: true,
				DeleteTaskMaxScore: 10,
			}
			ops := unionRaceOperations(s, policy, s.RoleID(), time.Now(), raceGatesOn())
			if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceDelTask.String() {
				t.Fatalf("delete plan=%+v", ops)
			}
			if ops[0].Executable != tc.executable {
				t.Fatalf("executable=%t want %t: %+v", ops[0].Executable, tc.executable, ops[0])
			}
			if !tc.executable && (ops[0].Status != PlanStatusBlocked || len(ops[0].BlockedReasons) == 0) {
				t.Fatalf("unauthorized delete must explain its block: %+v", ops[0])
			}
		})
	}
}

func TestUnionRaceDeleteOrdersEligibleTasksDeterministically(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{
		{2, 3036, 10, 0, 0},
		{3, 3036, 5, 0, 0},
		{1, 3036, 5, 0, 0},
		{4, 3036, 0, 0, 0}, // Missing/unknown score must fail closed.
		{5, 3036, 11, 0, 0},
	})
	applyRaceDeletePosition(s, 1)
	policy := &pb.UnionRacePolicy{
		Enabled:            true,
		DeleteLowScoreTask: true,
		DeleteTaskMaxScore: 10,
	}

	ops := unionRaceOperations(s, policy, s.RoleID(), time.Now(), raceGatesOn())
	sortOperations(ops)
	if len(ops) != 3 {
		t.Fatalf("delete ops=%+v, want three eligible rows", ops)
	}
	want := []int64{1, 3, 2}
	seenScopes := make(map[string]struct{}, len(ops))
	for i, op := range ops {
		if op.TaskMsID != want[i] || op.Kind != clientproto.RPCFmlRaceDelTask.String() {
			t.Fatalf("ops[%d]=%+v want taskMsId=%d", i, op, want[i])
		}
		if op.CooldownKey == "" {
			t.Fatalf("ops[%d] missing per-task cooldown scope", i)
		}
		seenScopes[op.CooldownKey] = struct{}{}
	}
	if len(seenScopes) != len(ops) {
		t.Fatalf("delete cooldown scopes are not task-specific: %+v", ops)
	}
}

func TestUnionRaceDeleteSkipsCoolingSlots(t *testing.T) {
	for _, autoComplete := range []bool{false, true} {
		for _, tt := range []struct {
			name      string
			remaining time.Duration
			want      []int64
		}{
			{"cooling lower score yields", 90 * time.Second, []int64{2}},
			{"one millisecond before ready", time.Millisecond, []int64{2}},
			{"ready at exact boundary", 0, []int64{1, 2}},
			{"ready after boundary", -time.Millisecond, []int64{1, 2}},
		} {
			t.Run(fmt.Sprintf("auto=%t/%s", autoComplete, tt.name), func(t *testing.T) {
				now := time.Now()
				s := state.New()
				applyRaceState(s, [][5]int32{{1, 3036, 5, 0, 0}, {2, 3036, 10, 0, 0}})
				applyRaceDeletePosition(s, 1)
				s.ApplyV(json.RawMessage(fmt.Sprintf(
					`{"25":{"114":[{"0":1,"4":3036,"5":%d,"10":5,"14":0,"15":0},{"0":2,"4":3036,"5":%d,"10":10,"14":0,"15":0}]}}`,
					now.Add(tt.remaining).UnixMilli(), now.Add(-time.Second).UnixMilli(),
				)))
				policy := &pb.UnionRacePolicy{
					Enabled: true, AutoEnableModules: autoComplete, MinTaskScore: 29,
					DeleteLowScoreTask: true, DeleteTaskMaxScore: 10,
				}
				var ids []int64
				for _, op := range unionRaceOperations(s, policy, s.RoleID(), now, raceGatesOn()) {
					if op.Kind == clientproto.RPCFmlRaceDelTask.String() {
						ids = append(ids, op.TaskMsID)
						fullPolicy := testEnabledRaceFullPolicy()
						fullPolicy.Union.Race = policy
						if err := ValidateRaceTaskMutation(s, fullPolicy, &op, now); err != nil {
							t.Fatalf("planner/preflight disagreement: %v", err)
						}
					}
				}
				if fmt.Sprint(ids) != fmt.Sprint(tt.want) {
					t.Fatalf("deletes=%v, want %v", ids, tt.want)
				}
			})
		}
	}
}

func TestUnionRaceDeleteRefreshesStalePoolBeforeMutation(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3036, 10, 0, 0}})
	applyRaceDeletePosition(s, 2)
	policy := &pb.UnionRacePolicy{
		Enabled:            true,
		DeleteLowScoreTask: true,
		DeleteTaskMaxScore: 10,
	}
	view := s.FmlRace()
	now := time.UnixMilli(view.TasksSyncedAtMs).Add(raceTaskPoolRefreshInterval)

	ops := unionRaceOperations(s, policy, s.RoleID(), now, raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("stale pool must sync before delete, got %+v", ops)
	}
}

func TestUnionRaceGiveUpTaskBelowScoreThreshold(t *testing.T) {
	s := state.New()
	// Task pool: task msId=1, score=5. Taken task: msId=1, not completed (0/3).
	// Field 110: {"999":{"7":{"0":1,"1":3036,"2":3,"3":0}}}  — FinishCnt=0
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":5,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":3,"3":0}}}}}`))
	policy := testRacePolicy()
	policy.MinTaskScore = 10 // lower bound: skip tasks with Score <= 10
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
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

func TestUnionRaceGiveUpUsesPoolScoreWhenTakenScoreUnset(t *testing.T) {
	s := state.New()
	// Pool only (no field 110): score=5 lives on the pool row.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23001],"10":5,"14":0,"15":0}]}}`))
	// Progress/take ACK creates Taken without score in the payload; finalize/enrich
	// or raceTakenScore must still see pool score=5 and give up under min=10.
	s.ApplyV(json.RawMessage(`{"25":{"134":{"1":{"3":{"0":1,"1":3036,"2":3,"3":0,"4":[23001]}}}}}`))
	if got := s.FmlRace().Taken; !got.HasTask {
		t.Fatalf("setup Taken=%+v, want HasTask", got)
	}
	if score := raceTakenScore(s.FmlRace()); score != 5 {
		t.Fatalf("raceTakenScore=%d, want 5 from pool", score)
	}
	policy := testRacePolicy()
	policy.MinTaskScore = 10
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
	var hasGiveUp bool
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			hasGiveUp = true
		}
	}
	if !hasGiveUp {
		t.Fatalf("expected giveUp using pool score when take ACK omitted score, got %+v view=%+v", ops, s.FmlRace())
	}
}

func TestUnionRaceGiveUpUncultivatedTakenPlantHarvest(t *testing.T) {
	s := state.New()
	// Taken plant-harvest for uncultivated flower 23099 — impossible to complete.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"6":[23099],"10":56,"14":0,"15":0}],"110":{"999":{"7":{"0":1,"1":3036,"2":600,"3":0,"4":[23099]}}}}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("should not giveUp a completed task, got %+v", ops)
		}
	}
}

func TestUnionRaceDoesNotFinishUnknownZeroTarget(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1},"117":{"5":4},"114":[{"0":1,"4":4001,"6":[23001],"10":9}],"110":{"42":{"7":{"0":1,"1":4001,"2":0,"3":0,"4":[23001]}}}}}`))

	for _, op := range unionRaceOperations(s, testRacePolicy(), 0, time.Now(), raceGatesOn()) {
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
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take far-future CD task, got %+v", ops)
		}
	}
}

func TestUnionRacePrefersReadyOverUpcoming(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	readyAppear := now.UnixMilli()
	upcomingAppear := now.Add(raceTakeLeadWindow / 2).UnixMilli()
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":1,"4":3036,"5":%d,"6":[23001],"10":5,"14":0,"15":0},{"0":2,"4":3036,"5":%d,"6":[23001],"10":99,"14":0,"15":0}]}}`,
		readyAppear, upcomingAppear,
	)))
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, raceGatesOn())
	if len(ops) == 0 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take op, got %+v", ops)
	}
	if ops[0].TaskMsID != 1 {
		t.Fatalf("ready task must win over higher-score upcoming, got msId=%d", ops[0].TaskMsID)
	}
}

func TestUnionRacePreemptiveTakeWithinLead(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	appear := now.Add(raceTakeLeadWindow / 2).UnixMilli()
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":7,"4":3036,"5":%d,"6":[23001],"10":10,"14":0,"15":0}]}}`, appear,
	)))
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, raceGatesOn())
	if len(ops) == 0 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected preemptive take within lead, got %+v", ops)
	}
	if ops[0].TaskMsID != 7 {
		t.Fatalf("taskMsId = %d, want 7", ops[0].TaskMsID)
	}
}

func TestRaceTakeWakeAt(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"1":{"0":999,"1":88},"111":{"1":1},"117":{"5":4},"114":[{"0":7,"4":3036,"5":%d,"6":[23001],"10":10,"14":0,"15":0}]}}`,
		now.Add(5*time.Second).UnixMilli(),
	)))
	policy := testEnabledRaceFullPolicy()

	wake := RaceTakeWakeAt(s, policy, now)
	want := now.Add(5*time.Second - raceTakeLeadWindow)
	if !wake.Equal(want) {
		t.Fatalf("RaceTakeWakeAt=%v, want AppearTime-lead %v", wake, want)
	}
	if RaceTakeDue(s, policy, now) {
		t.Fatal("5s before appear must not be due yet")
	}

	if got := RaceTakeWakeAt(s, policy, want); !got.IsZero() {
		t.Fatalf("inside lead window must not schedule a future wake, got %v", got)
	}
	if !RaceTakeDue(s, policy, want) {
		t.Fatal("inside lead window must be due for immediate take")
	}

	off := testEnabledRaceFullPolicy()
	off.Union.Race.AutoEnableModules = false
	if got := RaceTakeWakeAt(s, off, now); !got.IsZero() {
		t.Fatalf("auto-complete off must not wake, got %v", got)
	}
}

func TestRaceTakeWakeAtSkipsWhenAlreadyTakeableOrHeld(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	policy := testEnabledRaceFullPolicy()

	ready := state.New()
	ready.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	ready.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":7,"4":3036,"5":%d,"6":[23001],"10":10,"14":0,"15":0}]}}`,
		now.UnixMilli(),
	)))
	if got := RaceTakeWakeAt(ready, policy, now); !got.IsZero() {
		t.Fatalf("already-due task must take this tick, got wake %v", got)
	}

	held := state.New()
	held.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	held.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":7,"4":3036,"6":[23001],"10":10,"14":0,"15":0}],"110":{"999":{"7":{"0":7,"1":3036,"2":3,"3":0,"4":[23001]}}}}}`))
	if got := RaceTakeWakeAt(held, policy, now); !got.IsZero() {
		t.Fatalf("held task must not wake for another take, got %v", got)
	}
}

func TestRaceTakeLaneRankPreemptsFarm(t *testing.T) {
	take := PlannedOp{Domain: "union.race.take", Lane: LaneSide, Priority: 4380, PreemptFarm: true}
	harvest := PlannedOp{Domain: "farm.harvest", Lane: LaneFarm, Priority: 9000}
	if !operationComesBefore(take, harvest) {
		t.Fatal("race take must sort before farm harvest")
	}
}

func TestRaceEnterSyncFinishLaneRankPreemptsFarmAndOrders(t *testing.T) {
	harvest := PlannedOp{Domain: "farm.harvest", Lane: LaneFarm, Priority: 9000}
	customer := PlannedOp{Domain: "order.customer", Lane: LaneSide, Priority: 5000, Category: CategoryOrder}
	for _, domain := range []string{"union.race.enter", "union.race.sync", "union.race.finish"} {
		op := PlannedOp{Domain: domain, Lane: LaneSide, Priority: 4380, Category: CategoryRace, PreemptFarm: true}
		if !operationComesBefore(op, harvest) {
			t.Fatalf("%s must sort before farm harvest", domain)
		}
		if !operationComesBefore(op, customer) {
			t.Fatalf("%s must sort before customer order", domain)
		}
	}
	plainEnter := PlannedOp{Domain: "union.race.enter", Lane: LaneSide, Priority: 4400, Category: CategoryRace}
	if operationComesBefore(plainEnter, harvest) {
		t.Fatal("non-preempt enter must not sort before farm harvest")
	}
}

func TestRaceBootstrapDueAfterLoginUnobserved(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	s := state.New()
	s.ApplyVMap(map[string]any{"101": map[string]any{"0": cultivate(23001)}})
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"1":{"0":999,"1":88},"111":{"1":1},"117":{"5":4},"114":[{"0":7,"4":3036,"5":%d,"6":[23001],"10":10,"14":0,"15":0}]}}`,
		now.UnixMilli(),
	)))
	policy := testEnabledRaceFullPolicy()
	if !RaceBootstrapDue(s, policy, now) {
		t.Fatal("observed pool with takeable row must bootstrap")
	}
	s.MarkFmlRaceTaskPoolStale()
	if !RaceBootstrapDue(s, policy, now) {
		t.Fatal("login-unobserved pool must bootstrap before farm/order")
	}
	off := testEnabledRaceFullPolicy()
	off.Union.Race.Enabled = false
	if RaceBootstrapDue(s, off, now) {
		t.Fatal("race disabled must not bootstrap")
	}
}

func TestBuildPlan_RaceSyncPreemptsHarvestAfterLogin(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999,"32":{"23001":10}}},"100":{"1":{"1001":{"0":23001,"1":3,"2":1,"7":1}}},"25":{"1":{"0":999,"1":88},"111":{"1":1},"117":{"5":4}}}`))
	s.MarkFmlRaceTaskPoolStale()
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Plant.Planting.AutoHarvestEnabled = true
	p.Union.Race.Enabled = true
	p.Union.Race.AutoEnableModules = true
	p.Union.Race.MinTaskScore = 0
	p.Union.Race.TaskTypePriority = map[int32]int32{3036: 5}

	op := Plan(s, p, now)
	if op == nil || op.Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("Plan()=%+v, want getTaskList before harvest after login", op)
	}
}

func TestUnionRaceSkipsUncultivatedPlantHarvest(t *testing.T) {
	s := state.New()
	// Only plant-harvest with unknown / uncultivated flower — no take.
	s.ApplyV(json.RawMessage(raceStateJSONWithParams([][5]int32{{1, 3036, 10, 0, 0}}, 23099)))
	ops := unionRaceOperations(s, testRacePolicy(), 0, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, testRacePolicy(), 0, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take an unsupported task merely as a fallback: %+v", op)
		}
	}
}

func TestUnionRaceAvoidsProgressedTaskDuringAutomaticSelection(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3030, 40, 0, 0}, {2, 3030, 24, 0, 0}})
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"25":{"114":[
		{"0":1,"4":3030,"7":10,"8":3,"10":40,"12":0},
		{"0":2,"4":3030,"7":10,"8":0,"10":24,"12":0}
	]}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3030: 4}
	policy.AvoidProgressedTasks = proto.Bool(true)

	ops := unionRaceOperations(s, policy, s.RoleID(), time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() || ops[0].TaskMsID != 2 {
		t.Fatalf("avoid-progress plan=%+v, want fresh task 2", ops)
	}

	policy.AvoidProgressedTasks = proto.Bool(false)
	ops = unionRaceOperations(s, policy, s.RoleID(), time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() || ops[0].TaskMsID != 1 {
		t.Fatalf("allow-progress plan=%+v, want higher-score task 1", ops)
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
	// Cultivate 23001 for plantability cases and unlock vase 3002 for craft.
	s.ApplyVMap(map[string]any{
		"101": map[string]any{"0": cultivate(23001)},
		"102": map[string]any{"0": map[string]any{"3002": map[string]any{"1": 3002}}},
	})

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
			name:   "progressed task skipped",
			task:   state.FmlRaceTaskView{MsId: 33, TaskId: 3030, TaskType: 3030, Score: 24, FinishCnt: 3, TargetCnt: 10},
			policy: &pb.UnionRacePolicy{AvoidProgressedTasks: proto.Bool(true), TaskTypePriority: map[int32]int32{3030: 4}},
			want:   "已有进度（3/10）",
		},
		{
			name:   "progressed task allowed when policy disabled",
			task:   state.FmlRaceTaskView{MsId: 34, TaskId: 3030, TaskType: 3030, Score: 24, FinishCnt: 3, TargetCnt: 10},
			policy: &pb.UnionRacePolicy{AvoidProgressedTasks: proto.Bool(false), TaskTypePriority: map[int32]int32{3030: 4}},
			want:   "",
		},
		{
			name:   "progressed task without target skipped",
			task:   state.FmlRaceTaskView{MsId: 35, TaskId: 3030, TaskType: 3030, Score: 24, FinishCnt: 3},
			policy: &pb.UnionRacePolicy{AvoidProgressedTasks: proto.Bool(true), TaskTypePriority: map[int32]int32{3030: 4}},
			want:   "已有进度（3）",
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
			name: "system upgraded ok",
			task: state.FmlRaceTaskView{MsId: 16, TaskId: 3036, TaskType: 3036, Score: 28, ParamID: 23001, IsUpgrade: 1, UpgradeUid: 0},
			policy: func() *pb.UnionRacePolicy {
				p := takeablePlant()
				p.ExcludeOthersUpgradeTask = true
				return p
			}(),
			want: "",
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
			name:   "pearl hire takeable with priority",
			task:   state.FmlRaceTaskView{MsId: 22, TaskId: 1022, TaskType: 3023, Score: 24},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3023: 4}},
			want:   "",
		},
		{
			name:   "pearl hire priority zero",
			task:   state.FmlRaceTaskView{MsId: 23, TaskId: 1022, TaskType: 3023, Score: 24},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3023: 0}},
			want:   "优先级为0",
		},
		{
			name:   "flower art sell takeable with priority",
			task:   state.FmlRaceTaskView{MsId: 26, TaskId: 3030, TaskType: 3030, Score: 24},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3030: 4}},
			want:   "",
		},
		{
			name:   "flower art craft takeable with priority",
			task:   state.FmlRaceTaskView{MsId: 24, TaskId: 3034, TaskType: 3034, Score: 24, ParamID: 3002},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3034: 4}},
			want:   "",
		},
		{
			name:   "flower art craft missing vase param",
			task:   state.FmlRaceTaskView{MsId: 27, TaskId: 3034, TaskType: 3034, Score: 24},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3034: 4}},
			want:   "目标花瓶未知",
		},
		{
			name:   "flower art craft locked vase",
			task:   state.FmlRaceTaskView{MsId: 28, TaskId: 3034, TaskType: 3034, Score: 24, ParamID: 3074},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3034: 4}},
			want:   "目标花瓶未解锁",
		},
		{
			name:   "flower art craft priority zero",
			task:   state.FmlRaceTaskView{MsId: 25, TaskId: 3034, TaskType: 3034, Score: 24, ParamID: 3002},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3034: 0}},
			want:   "优先级为0",
		},
		{
			name:   "flower cultivate score 36 ok",
			task:   state.FmlRaceTaskView{MsId: 30, TaskId: 3044, TaskType: 3044, Score: 36},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3044: 4}},
			want:   "",
		},
		{
			name:   "flower cultivate non-36 skipped",
			task:   state.FmlRaceTaskView{MsId: 31, TaskId: 3044, TaskType: 3044, Score: 18},
			policy: &pb.UnionRacePolicy{TaskTypePriority: map[int32]int32{3044: 4}},
			want:   "仅接36分花种培育",
		},
		{
			name:   "flower cultivate progress skipped",
			task:   state.FmlRaceTaskView{MsId: 32, TaskId: 3044, TaskType: 3044, Score: 36, FinishCnt: 1, TargetCnt: 4},
			policy: &pb.UnionRacePolicy{AvoidProgressedTasks: proto.Bool(true), TaskTypePriority: map[int32]int32{3044: 4}},
			want:   "已有进度（1/4）",
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
			got := RaceTakeSkipReason(s, tc.task, tc.policy, uid, now, raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 0, now, raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected periodic getTaskList, got %+v", ops)
	}
	if !ops[0].PreemptFarm {
		t.Fatal("periodic task-pool refresh must not wait behind a busy farm lane")
	}
}

func TestUnionRaceNoPeriodicGetTaskListWithinTTL(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"1":1},"117":{"5":4},"114":[]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	policy := testRacePolicy()
	now := time.UnixMilli(synced).Add(raceTaskPoolRefreshInterval - time.Second)
	ops := unionRaceOperations(s, policy, 0, now, raceGatesOn())
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
	ops := unionRaceOperations(s, testRacePolicy(), 0, plannerNow, raceGatesOn())
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
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("near taken CD must not defer periodic sync, got %+v", ops)
	}
}

func TestUnionRacePeriodicDeferredForNearTakeableCD(t *testing.T) {
	// Only the final approach window suppresses TTL sync; 5s is inside it.
	s, now := raceStateAtTTL(t, 5*time.Second, 23001, 0)
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, raceGatesOn())
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
	// Keep refreshing during a longer wait so upgrades/claims are observed
	// before AppearTime.
	s, now := raceStateAtTTL(t, 5*time.Minute, 23001, 0)
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("mid takeable CD must not block periodic sync, got %+v", ops)
	}
}

func TestUnionRacePeriodicRunsDespiteFarTakeableCD(t *testing.T) {
	s, now := raceStateAtTTL(t, 15*time.Minute, 23001, 0)
	ops := unionRaceOperations(s, testRacePolicy(), 0, now, raceGatesOn())
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
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"117":{"5":4},"114":[{"0":55,"4":4012,"6":[23363],"7":100,"8":10,"10":25,"12":999},{"0":56,"4":4001,"6":[23001],"7":50,"8":0,"10":30,"12":0}],"110":{"42":{"3":0}}}}`))
	if !s.FmlRace().Taken.HasTask {
		t.Fatal("expected synthesized taken")
	}
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now(), raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take while holding synthesized task, ops=%+v", ops)
		}
	}
}

func TestUnionRaceNoFinishWhenTakenProgressUnknown(t *testing.T) {
	s := state.New()
	// Synthesized taken with TargetCnt=0/FinishCnt=0 must not finish.
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"117":{"5":4},"114":[{"0":55,"4":4012,"6":[23363],"10":25,"12":999}],"110":{"42":{"3":0}}}}`))
	ops := unionRaceOperations(s, testRacePolicy(), 999, time.Now(), raceGatesOn())
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
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":42,"1":1,"2":1000,"3":9000000000},"117":{"5":4},"114":[{"0":55,"4":3017,"7":10,"8":0,"10":25,"12":999}],"110":{"42":{"3":0,"4":0}},"116":[{"0":999,"1":42,"3":0,"4":0}]}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{
		3017: 0,
		3036: 5,
	}
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
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
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take of customer-order race task, got %+v", ops)
	}
	if ops[0].TaskMsID != 1 || ops[0].TaskID != 3016 {
		t.Fatalf("take detail = msId=%d taskID=%d, want 1/3016", ops[0].TaskMsID, ops[0].TaskID)
	}
}

func TestUnionRaceTakesPearlHireWhenEnabled(t *testing.T) {
	s := state.New()
	// Catalog task 1022 has type 3023 (珍珠采集雇佣).
	applyRaceState(s, [][5]int32{{1, 1022, 24, 0, 0}})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3023: 4, 3036: 5}
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take of pearl-hire race task, got %+v", ops)
	}
	if ops[0].TaskMsID != 1 || ops[0].TaskID != 3023 {
		t.Fatalf("take detail = msId=%d taskID=%d, want 1/3023", ops[0].TaskMsID, ops[0].TaskID)
	}
}

func TestUnionRaceTakesFlowerCultivateOnlyScore36(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{
		{1, 3044, 18, 0, 0},
		{2, 3044, 36, 0, 0},
		{3, 3044, 9, 0, 0},
	})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3044: 4}
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take of 36-score flower-cultivate, got %+v", ops)
	}
	if ops[0].TaskMsID != 2 || ops[0].TaskID != 3044 {
		t.Fatalf("take detail = msId=%d taskID=%d, want 2/3044", ops[0].TaskMsID, ops[0].TaskID)
	}
}

func TestUnionRaceTakesFlowerCultivateWhenModuleOff(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3044, 36, 0, 0}})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3044: 4}
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesNoCultivate())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take of flower-cultivate without cultivate module, got %+v", ops)
	}
	got := RaceTakeSkipReason(s, state.FmlRaceTaskView{MsId: 1, TaskId: 3044, TaskType: 3044, Score: 36}, policy, 0, time.Now(), raceGatesNoCultivate())
	if got != "" {
		t.Fatalf("RaceTakeSkipReason = %q, want empty", got)
	}
}

func TestUnionRaceSkipsFlowerCultivateWithProgress(t *testing.T) {
	s := state.New()
	policy := testRacePolicy()
	policy.AvoidProgressedTasks = proto.Bool(true)
	policy.TaskTypePriority = map[int32]int32{3044: 4}
	task := state.FmlRaceTaskView{MsId: 1, TaskId: 3044, TaskType: 3044, Score: 36, FinishCnt: 2, TargetCnt: 4}
	got := RaceTakeSkipReason(s, task, policy, 0, time.Now(), raceGatesNoCultivate())
	if got != "已有进度（2/4）" {
		t.Fatalf("RaceTakeSkipReason = %q, want 已有进度（2/4）", got)
	}
	policy.AvoidProgressedTasks = proto.Bool(false)
	if got := RaceTakeSkipReason(s, task, policy, 0, time.Now(), raceGatesNoCultivate()); got != "" {
		t.Fatalf("explicitly allowed progressed cultivate task: %q", got)
	}
}

func TestUnionRaceGiveUpFlowerCultivateNon36(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3044,"2":2,"3":0}}},"114":[{"0":71,"4":3044,"7":2,"8":0,"10":18,"12":999}]}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3044: 4}
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGiveUpTask.String() {
		t.Fatalf("expected giveUp for non-36 flower-cultivate, got %+v", ops)
	}
}

func TestUnionRaceKeepsFlowerCultivate36EvenIfUncompletable(t *testing.T) {
	s := state.New()
	// 36-score flower-cultivate hold; cultivate module off + min_task_score above 36
	// must still keep (never give up once held at score 36).
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3044,"2":2,"3":0}}},"114":[{"0":71,"4":3044,"7":2,"8":0,"10":36,"12":999}]}}`))
	policy := testRacePolicy()
	policy.MinTaskScore = 40
	policy.TaskTypePriority = map[int32]int32{3044: 4}
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesNoCultivate())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("must not giveUp 36-score flower-cultivate, got %+v", ops)
		}
	}
}

func TestUnionRaceKeepsFlowerCultivate36EvenIfPriorityZero(t *testing.T) {
	// Manual take or leftover hold: score 36 must not be given up when type
	// priority is 0 (automation will not take new ones, but keeps existing).
	cases := []struct {
		name string
		raw  string
	}{
		{
			name: "progress zero",
			raw:  `{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3044,"2":2,"3":0}}},"114":[{"0":71,"4":3044,"7":2,"8":0,"10":36,"12":999}]}}`,
		},
		{
			name: "mid progress",
			raw:  `{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3044,"2":4,"3":1}}},"114":[{"0":71,"4":3044,"7":4,"8":1,"10":36,"12":999}]}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := state.New()
			s.ApplyV(json.RawMessage(tc.raw))
			policy := testRacePolicy()
			policy.TaskTypePriority = map[int32]int32{3044: 0}
			ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesNoCultivate())
			for _, op := range ops {
				if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
					t.Fatalf("must not giveUp 36 flower-cultivate when priority 0, got %+v", ops)
				}
			}
		})
	}
}

func TestUnionRaceSkipsCustomerOrderWhenModuleOff(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3019, 24, 0, 0}})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3016: 4}
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesNoCustomer())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take customer-order race without customer module: %+v", op)
		}
	}
	got := RaceTakeSkipReason(s, state.FmlRaceTaskView{MsId: 1, TaskId: 3019, TaskType: 3016, Score: 24}, policy, 0, time.Now(), raceGatesNoCustomer())
	if got != "顾客订单模块未开启" {
		t.Fatalf("RaceTakeSkipReason = %q, want 顾客订单模块未开启", got)
	}
}

func TestUnionRaceSkipsPearlHireWhenModuleOff(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 1022, 24, 0, 0}})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3023: 4}
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesNoPearl())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take pearl-hire race without pearl hire module: %+v", op)
		}
	}
	got := RaceTakeSkipReason(s, state.FmlRaceTaskView{MsId: 1, TaskId: 1022, TaskType: 3023, Score: 24}, policy, 0, time.Now(), raceGatesNoPearl())
	if got != "珍珠雇佣模块未开启" {
		t.Fatalf("RaceTakeSkipReason = %q, want 珍珠雇佣模块未开启", got)
	}
}

func TestUnionRaceTakesFlowerArtCraft(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3034, 24, 0, 0}})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3034: 4}
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take of flower-art craft, got %+v", ops)
	}
	if ops[0].TaskMsID != 1 || ops[0].TaskID != 3034 {
		t.Fatalf("take detail = msId=%d taskID=%d, want 1/3034", ops[0].TaskMsID, ops[0].TaskID)
	}
}

func TestUnionRaceSkipsFlowerArtCraftWithoutTargetVase(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3034, 24, 0, 0}})
	s.ApplyVMap(map[string]any{"102": map[string]any{"0": map[string]any{}}})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3034: 4}
	ops := unionRaceOperations(s, policy, 0, time.Now(), raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceTakeTask.String() {
			t.Fatalf("must not take craft task without its vase: %+v", op)
		}
	}
	got := RaceTakeSkipReason(s, state.FmlRaceTaskView{MsId: 1, TaskId: 3034, TaskType: 3034, Score: 24, ParamID: 3002}, policy, 0, time.Now(), raceGatesOn())
	if got != "目标花瓶未解锁" {
		t.Fatalf("RaceTakeSkipReason=%q, want 目标花瓶未解锁", got)
	}
}

func TestUnionRaceGivesUpFlowerArtCraftWithoutTargetVase(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3034,"2":5,"3":1,"4":[3074]}}},"114":[{"0":71,"4":3034,"6":[3074],"7":5,"8":1,"10":24,"12":999}]},"102":{"0":{"3002":{"1":3002}}}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3034: 4}
	ops := unionRaceOperations(s, policy, 999, time.Now(), RaceModuleGates{})
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGiveUpTask.String() {
		t.Fatalf("expected giveUp for unavailable target vase, got %+v", ops)
	}
}

func TestUnionRaceTakesFlowerArtWithoutCraftModule(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3034, 24, 0, 0}})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3034: 4}
	// Gates without flower-art toggle still take — race owns craft progression.
	ops := unionRaceOperations(s, policy, 0, time.Now(), RaceModuleGates{})
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take of flower-art craft without craft module, got %+v", ops)
	}
	got := RaceTakeSkipReason(s, state.FmlRaceTaskView{MsId: 1, TaskId: 3034, TaskType: 3034, Score: 24, ParamID: 3002}, policy, 0, time.Now(), RaceModuleGates{})
	if got != "" {
		t.Fatalf("RaceTakeSkipReason = %q, want empty", got)
	}
}

func TestUnionRaceTakesFlowerArtSell(t *testing.T) {
	s := state.New()
	applyRaceState(s, [][5]int32{{1, 3030, 24, 0, 0}})
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3030: 4}
	ops := unionRaceOperations(s, policy, 0, time.Now(), RaceModuleGates{})
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take of flower-art sell, got %+v", ops)
	}
	if ops[0].TaskMsID != 1 || ops[0].TaskID != 3030 {
		t.Fatalf("take detail = msId=%d taskID=%d, want 1/3030", ops[0].TaskMsID, ops[0].TaskID)
	}
}

func TestUnionRaceFinishesCompletedCustomerOrder(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3019,"2":5,"3":5}}},"114":[{"0":71,"4":3019,"7":5,"8":5,"10":24,"12":999}]}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3016: 4}
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceFinishTask.String() {
		t.Fatalf("expected finish of completed customer-order race, got %+v", ops)
	}
	if ops[0].TaskMsID != 71 || ops[0].TaskID != 3016 || ops[0].Priority != raceCustomerFinishPriority {
		t.Fatalf("finish op = %+v, want msId=71 type=3016 prio=%d", ops[0], raceCustomerFinishPriority)
	}
}

func TestUnionRaceFinishesCompletedPearlHire(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":1022,"2":3,"3":3}}},"114":[{"0":71,"4":1022,"7":3,"8":3,"10":24,"12":999}]}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3023: 4}
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceFinishTask.String() {
		t.Fatalf("expected finish of completed pearl-hire race, got %+v", ops)
	}
	if ops[0].TaskMsID != 71 || ops[0].TaskID != 3023 || ops[0].Priority != racePearlFinishPriority {
		t.Fatalf("finish op = %+v, want msId=71 type=3023 prio=%d", ops[0], racePearlFinishPriority)
	}
}

func TestUnionRaceGiveUpCustomerOrderWhenModuleOff(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3019,"2":5,"3":0}}},"114":[{"0":71,"4":3019,"7":5,"8":0,"10":24,"12":999}]}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3016: 4}
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesNoCustomer())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGiveUpTask.String() {
		t.Fatalf("expected giveUp when customer module off, got %+v", ops)
	}
}

func TestUnionRaceGiveUpPearlHireWhenModuleOff(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":1022,"2":3,"3":0}}},"114":[{"0":71,"4":1022,"7":3,"8":0,"10":24,"12":999}]}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3023: 4}
	ops := unionRaceOperations(s, policy, 999, time.Now(), raceGatesNoPearl())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGiveUpTask.String() {
		t.Fatalf("expected giveUp when pearl hire module off, got %+v", ops)
	}
}

func TestUnionRaceKeepsFlowerArtWhenCraftModuleOff(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3034,"2":5,"3":1,"4":[3002]}}},"114":[{"0":71,"4":3034,"6":[3002],"7":5,"8":1,"10":24,"12":999}]},"102":{"0":{"3002":{"1":3002}}}}`))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3034: 4}
	ops := unionRaceOperations(s, policy, 999, time.Now(), RaceModuleGates{})
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGiveUpTask.String() {
			t.Fatalf("must not giveUp flower-art craft when craft toggle off: %+v", ops)
		}
	}
}

func TestUnionRaceCustomerProgressSyncAfterInterval(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3019,"2":5,"3":1}}},"114":[{"0":71,"4":3019,"7":5,"8":1,"10":24,"12":999}]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3016: 4}
	now := time.UnixMilli(synced).Add(raceModuleProgressSyncInterval + time.Second)
	ops := unionRaceOperations(s, policy, 999, now, raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected customer progress sync, got %+v taken=%+v", ops, s.FmlRace().Taken)
	}
	if ops[0].Priority != raceCustomerSyncPriority {
		t.Fatalf("sync priority=%d, want %d", ops[0].Priority, raceCustomerSyncPriority)
	}
}

func TestUnionRacePearlProgressSyncAfterInterval(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":1022,"2":3,"3":1}}},"114":[{"0":71,"4":1022,"7":3,"8":1,"10":24,"12":999}]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3023: 4}
	now := time.UnixMilli(synced).Add(raceModuleProgressSyncInterval + time.Second)
	ops := unionRaceOperations(s, policy, 999, now, raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected pearl progress sync, got %+v taken=%+v", ops, s.FmlRace().Taken)
	}
	if ops[0].Priority != racePearlSyncPriority {
		t.Fatalf("sync priority=%d, want %d", ops[0].Priority, racePearlSyncPriority)
	}
}

func TestUnionRaceFlowerArtProgressSyncAfterInterval(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3034,"2":5,"3":1,"4":[3002]}}},"114":[{"0":71,"4":3034,"6":[3002],"7":5,"8":1,"10":24,"12":999}]},"102":{"0":{"3002":{"1":3002}}}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3034: 4}
	now := time.UnixMilli(synced).Add(raceModuleProgressSyncInterval + time.Second)
	ops := unionRaceOperations(s, policy, 999, now, raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected flower-art craft progress sync, got %+v taken=%+v", ops, s.FmlRace().Taken)
	}
	if ops[0].Priority != raceFlowerArtSyncPriority || !strings.Contains(ops[0].Reason, "花艺制作") {
		t.Fatalf("sync op = %+v, want craft sync at prio %d", ops[0], raceFlowerArtSyncPriority)
	}
}

func TestUnionRaceFlowerArtSellProgressSyncAfterInterval(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3030,"2":5,"3":1}}},"114":[{"0":71,"4":3030,"7":5,"8":1,"10":24,"12":999}]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3030: 4}
	now := time.UnixMilli(synced).Add(raceModuleProgressSyncInterval + time.Second)
	ops := unionRaceOperations(s, policy, 999, now, raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected flower-art sell progress sync, got %+v taken=%+v", ops, s.FmlRace().Taken)
	}
	if ops[0].Priority != raceFlowerArtSyncPriority || !strings.Contains(ops[0].Reason, "花艺售卖") {
		t.Fatalf("sync op = %+v, want sell sync at prio %d", ops[0], raceFlowerArtSyncPriority)
	}
}

func TestUnionRaceCultivateProgressSyncAfterInterval(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3044,"2":4,"3":1}}},"114":[{"0":71,"4":3044,"7":4,"8":1,"10":36,"12":999}]}}`))
	synced := s.FmlRace().TasksSyncedAtMs
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{3044: 4}
	now := time.UnixMilli(synced).Add(raceModuleProgressSyncInterval + time.Second)
	ops := unionRaceOperations(s, policy, 999, now, raceGatesNoCultivate())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected cultivate progress sync with module off, got %+v taken=%+v", ops, s.FmlRace().Taken)
	}
	if ops[0].Priority != raceCultivateSyncPriority || !strings.Contains(ops[0].Reason, "花种培育") {
		t.Fatalf("sync op = %+v, want cultivate sync at prio %d", ops[0], raceCultivateSyncPriority)
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
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"0":1785081600000,"1":1,"2":1000,"3":9000},"117":{"5":4},"110":{"1785081600000":{"3":0,"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}},"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":48,"10":28,"12":0}]}}`))
	s.ApplyV(json.RawMessage(`{"25":{"134":{"1785081600000":{"3":{"0":715,"1":4013,"2":300,"3":300,"4":[23577]},"4":1785358559363}}}}`))
	// Re-apply pool so FinishCnt stays lagging at 48 while LocalFinish is 300.
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":48,"10":28,"12":0}],"110":{"1785081600000":{"3":0,"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}}}}`))
	got := s.FmlRace()
	if got.LocalFinishCnt < 300 || got.Taken.FinishCnt != 48 {
		t.Fatalf("seed local=%d finish=%d, want local>=300 finish=48", got.LocalFinishCnt, got.Taken.FinishCnt)
	}
	policy := testRacePolicy()
	now := time.UnixMilli(got.TasksSyncedAtMs).Add(time.Second)
	ops := unionRaceOperations(s, policy, 999, now, raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("finish-progress sync must respect cooldown, got %+v", ops)
		}
	}
	later := time.UnixMilli(got.TasksSyncedAtMs).Add(raceFinishProgressSyncInterval + time.Second)
	ops = unionRaceOperations(s, policy, 999, later, raceGatesOn())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFmlRaceGetTaskList.String() {
		t.Fatalf("expected finish-progress sync after cooldown, got %+v", ops)
	}
	// One authoritative getTaskList that still lags must clamp LocalFinish so
	// the planner does not spin sync every raceFinishProgressSyncInterval.
	s.ApplyVFullFmlRaceTaskPool(json.RawMessage(`{"25":{"114":[{"0":715,"4":4013,"6":[23577],"7":300,"8":48,"10":28,"12":0}],"110":{"1785081600000":{"3":0,"7":{"0":715,"1":4013,"2":300,"3":48,"4":[23577]}}}}}`))
	afterSync := s.FmlRace()
	if afterSync.LocalFinishCnt != 48 {
		t.Fatalf("LocalFinishCnt=%d after full pool, want clamped 48", afterSync.LocalFinishCnt)
	}
	ops = unionRaceOperations(s, policy, 999, time.UnixMilli(afterSync.TasksSyncedAtMs).Add(raceFinishProgressSyncInterval+time.Second), raceGatesOn())
	for _, op := range ops {
		if op.Kind == clientproto.RPCFmlRaceGetTaskList.String() {
			t.Fatalf("must not keep finish-progress syncing after clamp, got %+v", ops)
		}
	}
}

func TestBuildPlan_RaceCustomerOrderLinksFinish(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999,"32":{"23005":10}}},"25":{"1":{"0":999,"1":88},"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3019,"2":5,"3":1}}},"114":[{"0":71,"4":3019,"7":5,"8":1,"10":24,"12":999}]},"109":{"0":{"1":{"10":{"0":[[23005,1]],"1":10}},"2":` + fmt.Sprintf("%d", now.Add(time.Hour).UnixMilli()) + `}}}`))
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Union.Race.Enabled = true
	p.Union.Race.AutoEnableModules = true
	p.Union.Race.MinTaskScore = 0
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

func TestBuildPlan_RaceGiveUpPreemptsCustomerFinish(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999,"32":{"23005":10}}},"25":{"1":{"0":999,"1":88},"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":3019,"2":5,"3":1}}},"114":[{"0":71,"4":3019,"7":5,"8":1,"10":24,"12":999}]},"109":{"0":{"1":{"10":{"0":[[23005,1]],"1":10}},"2":` + fmt.Sprintf("%d", now.Add(time.Hour).UnixMilli()) + `}}}`))
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.Customer.Enabled = true
	p.Union.Race.Enabled = true
	p.Union.Race.AutoEnableModules = true
	p.Union.Race.AutoGiveUpTask = true
	p.Union.Race.MinTaskScore = 24
	p.Union.Race.TaskTypePriority = map[int32]int32{3016: 4}

	result := BuildPlan(s, p, now)
	if len(result.Operations) == 0 || result.Operations[0].Kind != clientproto.RPCFmlRaceGiveUpTask.String() {
		t.Fatalf("giveUp must preempt customer finish for rejected held task, ops=%+v", result.Operations)
	}
	if op := Plan(s, p, now); op == nil || op.Kind != clientproto.RPCFmlRaceGiveUpTask.String() {
		t.Fatalf("Plan()=%+v, want immediate race giveUp", op)
	}
}

func TestBuildPlan_RacePearlHireLinksHire(t *testing.T) {
	now := time.Now().Add(2 * time.Second)
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":999,"32":{"1003":3}}},"25":{"111":{"1":1},"117":{"5":4},"110":{"999":{"7":{"0":71,"1":1022,"2":3,"3":1}}},"114":[{"0":71,"4":1022,"7":3,"8":1,"10":24,"12":999}]},"115":{"0":{"1":{"2":0,"3":null,"4":0,"9":1}},"1":{"5":{}}},"24":{"0":{"0":999},"1":[{"0":999,"1":2001}]},"28":{"5":[{"0":2001,"1":"safe","4":12}]}}`))
	s.ApplyV(json.RawMessage(`{"115":{"5":{"2001":0}}}`))
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Pearl.AutoHireEnabled = true
	p.Basic.Pearl.MaxHireTicketUsage = 2
	p.Union.Race.Enabled = true
	p.Union.Race.AutoEnableModules = true
	p.Union.Race.MinTaskScore = 0
	p.Union.Race.TaskTypePriority = map[int32]int32{3023: 4}

	result := BuildPlan(s, p, now)
	var linked *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.FeatureID == "basic.pearl_hire" && strings.HasPrefix(op.DemandID, raceActionGoal+":") {
			linked = op
			break
		}
	}
	if linked == nil {
		t.Fatalf("expected race-linked pearl hire, ops=%+v demands=%+v taken=%+v", result.Operations, result.Demands, s.FmlRace().Taken)
	}
	if !strings.Contains(linked.Reason, "公会竞赛珍珠雇佣剩余") {
		t.Fatalf("reason missing race pressure: %q", linked.Reason)
	}
}

func TestBuildPlan_RaceFlowerArtCraftEmitsMake(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	s := state.New()
	// Held flower-art-craft race (1/5) targets vase 3002. Craft materials + vase
	// unlocked; craft/sell toggles off — race still emits makeFlowerArt for 3002.
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0":  999,
			"32": map[string]any{"23005": 4, "23007": 4, "23008": 4},
			"34": 12,
		}},
		"25": map[string]any{
			"111": map[string]any{"1": 1},
			"117": map[string]any{"5": 4},
			"110": map[string]any{"999": map[string]any{"7": map[string]any{"0": 71, "1": 3034, "2": 5, "3": 1, "4": []any{3002}}}},
			"114": []any{map[string]any{"0": 71, "4": 3034, "6": []any{3002}, "7": 5, "8": 1, "10": 24, "12": 999}},
		},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3002": map[string]any{"1": 3002}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.FlowerArt.CraftEnabled = false
	p.Order.FlowerArt.SellEnabled = false
	p.Union.Race.Enabled = true
	p.Union.Race.AutoEnableModules = true
	p.Union.Race.MinTaskScore = 0
	p.Union.Race.TaskTypePriority = map[int32]int32{3034: 4}

	result := BuildPlan(s, p, now)
	var linked *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() && strings.HasPrefix(op.DemandID, raceActionGoal+":") {
			linked = op
			break
		}
	}
	if linked == nil {
		t.Fatalf("expected race-driven flower-art craft, ops=%+v demands=%+v taken=%+v", result.Operations, result.Demands, s.FmlRace().Taken)
	}
	if !strings.Contains(linked.Reason, "公会竞赛花艺制作剩余") {
		t.Fatalf("reason missing race pressure: %q", linked.Reason)
	}
	if linked.VaseID != 3002 {
		t.Fatalf("craft vase=%d, want task target 3002", linked.VaseID)
	}
	if linked.Count <= 0 || linked.Count > 4 {
		t.Fatalf("craft count=%d, want 1..4 (remaining race progress)", linked.Count)
	}
	if !linked.Executable {
		t.Fatalf("race craft should be executable: %+v", linked)
	}
	if linked.Priority != raceFlowerArtCraftOpPriority {
		t.Fatalf("craft priority=%d, want race floor %d", linked.Priority, raceFlowerArtCraftOpPriority)
	}
	if linked.ItemCost[23005] != linked.Count || linked.ItemCost[23007] != linked.Count || linked.ItemCost[23008] != linked.Count {
		t.Fatalf("craft ItemCost=%v, want one of each recipe flower per craft", linked.ItemCost)
	}
}

func TestBuildPlan_RaceFlowerArtCraftPicksHighestSeededPrice(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	s := state.New()
	// The task targets vase 3016. A vase-3002 recipe is also craftable but must
	// not be linked to this task.
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0": 999,
			"32": map[string]any{
				"23005": 4, "23007": 4, "23008": 4,
				"23070": 4, "23075": 4, "23003": 4,
			},
			"34": 12,
		}},
		"25": map[string]any{
			"111": map[string]any{"1": 1},
			"117": map[string]any{"5": 4},
			"110": map[string]any{"999": map[string]any{"7": map[string]any{"0": 71, "1": 3034, "2": 5, "3": 1, "4": []any{3016}}}},
			"114": []any{map[string]any{"0": 71, "4": 3034, "6": []any{3016}, "7": 5, "8": 1, "10": 24, "12": 999}},
		},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008, 23070, 23075, 23003)},
		"102": map[string]any{"0": map[string]any{
			"3002": map[string]any{"1": 3002},
			"3016": map[string]any{"1": 3016},
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Race.Enabled = true
	p.Union.Race.AutoEnableModules = true
	p.Union.Race.MinTaskScore = 0
	p.Union.Race.TaskTypePriority = map[int32]int32{3034: 4}

	result := BuildPlan(s, p, now)
	var craft *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() && strings.HasPrefix(op.DemandID, raceActionGoal+":") {
			craft = op
			break
		}
	}
	if craft == nil {
		t.Fatalf("expected race craft, ops=%+v", result.Operations)
	}
	if craft.ItemID != 301612 || craft.VaseID != 3016 {
		t.Fatalf("craft art=%d vase=%d, want 301612/3016; op=%+v", craft.ItemID, craft.VaseID, craft)
	}
}

func TestBuildPlan_RaceFlowerArtSellCancelsAfterFiveMinutes(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	listedAt := now.Add(-raceFlowerArtRelistAfter - time.Second).UnixMilli()
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"0": 999, "32": map[string]any{"300208": 4}}},
		"25": map[string]any{
			"111": map[string]any{"1": 1},
			"117": map[string]any{"5": 4},
			"110": map[string]any{"999": map[string]any{"7": map[string]any{"0": 71, "1": 3030, "2": 5, "3": 1}}},
			"114": []any{map[string]any{"0": 71, "4": 3030, "7": 5, "8": 1, "10": 24, "12": 999}},
		},
		// Two occupied racks past raceFlowerArtRelistAfter; SellReady far in the future.
		"104": map[string]any{"0": map[string]any{
			"1": map[string]any{"1": 1, "2": 300208, "3": 2, "4": listedAt},
			"2": map[string]any{"1": 2, "2": 300208, "3": 2, "4": listedAt},
			"3": map[string]any{"1": 3, "2": 0, "3": 0},
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Race.Enabled = true
	p.Union.Race.AutoEnableModules = true
	p.Union.Race.MinTaskScore = 0
	p.Union.Race.TaskTypePriority = map[int32]int32{3030: 4}

	result := BuildPlan(s, p, now)
	var cancels []PlannedOp
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCFlowerRackCancelSell.String() && strings.HasPrefix(op.DemandID, raceActionGoal+":") {
			cancels = append(cancels, op)
		}
	}
	if len(cancels) != 2 {
		t.Fatalf("expected cancel of both stale racks, got %d ops=%+v", len(cancels), result.Operations)
	}
	for _, cancel := range cancels {
		if !cancel.Executable || cancel.Priority != raceFlowerArtCancelOpPriority {
			t.Fatalf("cancel not executable at race priority: %+v", cancel)
		}
		if !strings.Contains(cancel.Reason, "5分钟") {
			t.Fatalf("cancel reason = %q", cancel.Reason)
		}
	}
}

func TestBuildPlan_RaceFlowerArtSellEmitsListing(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	s := state.New()
	// Held flower-art-sell race (1/5). Finished art in stock + empty rack;
	// sell_enabled off — race must list on its own.
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0":  999,
			"32": map[string]any{"300208": 4},
		}},
		"25": map[string]any{
			"111": map[string]any{"1": 1},
			"117": map[string]any{"5": 4},
			"110": map[string]any{"999": map[string]any{"7": map[string]any{"0": 71, "1": 3030, "2": 5, "3": 1}}},
			"114": []any{map[string]any{"0": 71, "4": 3030, "7": 5, "8": 1, "10": 24, "12": 999}},
		},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.FlowerArt.SellEnabled = false
	p.Order.FlowerArt.CraftEnabled = false
	p.Union.Race.Enabled = true
	p.Union.Race.AutoEnableModules = true
	p.Union.Race.MinTaskScore = 0
	p.Union.Race.TaskTypePriority = map[int32]int32{3030: 4}

	result := BuildPlan(s, p, now)
	var linked *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCFlowerRackSell.String() && strings.HasPrefix(op.DemandID, raceActionGoal+":") {
			linked = op
			break
		}
	}
	if linked == nil {
		t.Fatalf("expected race-driven flower-art sell, ops=%+v demands=%+v taken=%+v", result.Operations, result.Demands, s.FmlRace().Taken)
	}
	if !strings.Contains(linked.Reason, "公会竞赛花艺售卖剩余") {
		t.Fatalf("reason missing race pressure: %q", linked.Reason)
	}
	if linked.ItemID != 300208 || linked.Count <= 0 || linked.Count > 4 {
		t.Fatalf("sell op mismatch: %+v", linked)
	}
	if !linked.Executable || linked.Priority != raceFlowerArtSellOpPriority {
		t.Fatalf("race sell should be executable at race priority: %+v", linked)
	}
}

func TestBuildPlan_RaceFlowerArtSellCraftsWhenNoStock(t *testing.T) {
	now := time.UnixMilli(1_700_000)
	s := state.New()
	// Sell race held, empty rack, flower materials only — must craft before list.
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"0":  999,
			"32": map[string]any{"23005": 4, "23007": 4, "23008": 4},
			"34": 12,
		}},
		"25": map[string]any{
			"111": map[string]any{"1": 1},
			"117": map[string]any{"5": 4},
			"110": map[string]any{"999": map[string]any{"7": map[string]any{"0": 71, "1": 3030, "2": 5, "3": 1}}},
			"114": []any{map[string]any{"0": 71, "4": 3030, "7": 5, "8": 1, "10": 24, "12": 999}},
		},
		"101": map[string]any{"0": cultivate(23005, 23007, 23008)},
		"102": map[string]any{"0": map[string]any{"3002": map[string]any{"1": 3002}}},
		"104": map[string]any{"0": map[string]any{"1": map[string]any{"1": 1, "2": 0, "3": 0}}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Order.FlowerArt.SellEnabled = false
	p.Order.FlowerArt.CraftEnabled = false
	p.Union.Race.Enabled = true
	p.Union.Race.AutoEnableModules = true
	p.Union.Race.MinTaskScore = 0
	p.Union.Race.TaskTypePriority = map[int32]int32{3030: 4}

	result := BuildPlan(s, p, now)
	var craft *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() && strings.HasPrefix(op.DemandID, raceActionGoal+":") {
			craft = op
			break
		}
	}
	if craft == nil {
		t.Fatalf("expected race sell to craft when stock empty, ops=%+v", result.Operations)
	}
	if !strings.Contains(craft.Reason, "公会竞赛花艺售卖剩余") || !strings.Contains(craft.Reason, "先制作") {
		t.Fatalf("craft reason = %q, want sell pressure + craft hint", craft.Reason)
	}
	if !craft.Executable {
		t.Fatalf("support craft should be executable: %+v", craft)
	}
}
