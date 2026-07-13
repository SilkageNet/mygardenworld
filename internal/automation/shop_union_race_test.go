package automation

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// raceStateJSON builds a namespace-25 JSON blob with the given race task pool.
// tasks is a slice of [msId, taskId, score, isUpgrade, upgradeUid].
// Fields are at the TOP LEVEL of ns25 (not nested under "0").
func raceStateJSON(tasks [][5]int32) string {
	pool := "[]"
	if len(tasks) > 0 {
		parts := make([]string, 0, len(tasks))
		for _, t := range tasks {
			parts = append(parts, fmt.Sprintf(`{"0":%d,"4":%d,"10":%d,"14":%d,"15":%d}`, t[0], t[1], t[2], t[3], t[4]))
		}
		pool = "[" + strings.Join(parts, ",") + "]"
	}
	return `{"25":{"111":{"1":1},"114":` + pool + `}}`
}

// testRacePolicy returns a policy with the common defaults for race tests:
// enabled, autoEnableModules on, and a score limit of 28.
func testRacePolicy() *pb.UnionRacePolicy {
	return &pb.UnionRacePolicy{
		Enabled:           true,
		AutoEnableModules: true,
		MaxTaskScore:      28,
	}
}

func TestUnionRaceDisabledProducesNoOps(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{{1, 3036, 10, 0, 0}})))
	policy := &pb.UnionRacePolicy{Enabled: false}
	ops := unionRaceOperations(s, policy, 0)
	if len(ops) != 0 {
		t.Fatalf("expected 0 ops when disabled, got %d: %+v", len(ops), ops)
	}
}

func TestUnionRaceAutoModulesOffProducesNoOps(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{{1, 3036, 10, 0, 0}})))
	policy := &pb.UnionRacePolicy{Enabled: true, AutoEnableModules: false, MaxTaskScore: 28}
	ops := unionRaceOperations(s, policy, 0)
	if len(ops) != 0 {
		t.Fatalf("expected 0 ops when autoEnableModules off, got %d: %+v", len(ops), ops)
	}
}

func TestUnionRaceScoreLimitFiltersHighScoreTasks(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{
		{1, 3036, 10, 0, 0},
		{2, 3036, 30, 0, 0},
	})))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0)
	if len(ops) != 1 {
		t.Fatalf("expected 1 take op, got %d: %+v", len(ops), ops)
	}
	if ops[0].Kind != clientproto.RPCFmlRaceTakeTask.String() {
		t.Fatalf("expected take RPC, got %s", ops[0].Kind)
	}
	if ops[0].TaskMsID != 1 {
		t.Fatalf("expected taskMsId 1, got %d", ops[0].TaskMsID)
	}
}

func TestUnionRacePrioritySorting(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{
		{1, 3044, 10, 0, 0}, // 花种培育, priority 4
		{2, 3036, 10, 0, 0}, // 种植收获, priority 5
	})))
	policy := testRacePolicy()
	policy.TaskTypePriority = map[int32]int32{
		3036: 5,
		3044: 4,
	}
	ops := unionRaceOperations(s, policy, 0)
	if len(ops) != 1 {
		t.Fatalf("expected 1 take op, got %d", len(ops))
	}
	if ops[0].TaskMsID != 2 {
		t.Fatalf("expected taskMsId 2 (higher priority 3036), got %d", ops[0].TaskMsID)
	}
}

func TestUnionRaceExcludeOthersUpgraded(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{
		{1, 3036, 10, 1, 100}, // upgraded by uid 100
		{2, 3036, 10, 0, 0},   // not upgraded
	})))
	policy := testRacePolicy()
	policy.ExcludeOthersUpgradeTask = true
	ops := unionRaceOperations(s, policy, 999) // current uid 999
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
	ops := unionRaceOperations(s, policy, 999)
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
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{
		{1, 3036, 10, 0, 0}, // not upgraded
		{2, 3036, 10, 1, 0}, // upgraded
	})))
	policy := testRacePolicy()
	policy.OnlyUpgradeTask = true
	ops := unionRaceOperations(s, policy, 0)
	if len(ops) != 1 {
		t.Fatalf("expected 1 take op, got %d", len(ops))
	}
	if ops[0].TaskMsID != 2 {
		t.Fatalf("expected taskMsId 2 (only upgraded), got %d", ops[0].TaskMsID)
	}
}

func TestUnionRaceBatchInactiveProducesNoOps(t *testing.T) {
	s := state.New()
	// No field 111 in JSON → BatchActive = false
	s.ApplyV(json.RawMessage(`{"25":{"114":[{"0":1,"4":3036,"10":10,"14":0,"15":0}]}}`))
	policy := testRacePolicy()
	ops := unionRaceOperations(s, policy, 0)
	if len(ops) != 0 {
		t.Fatalf("expected 0 ops when batch inactive, got %d: %+v", len(ops), ops)
	}
}

func TestUnionRaceUpgradeOpEmission(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{
		{1, 3036, 10, 0, 0}, // not upgraded, score within limit
	})))
	policy := testRacePolicy()
	policy.UpgradeTask = true
	ops := unionRaceOperations(s, policy, 0)
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
	s.ApplyV(json.RawMessage(raceStateJSON([][5]int32{
		{1, 3036, 5, 0, 0},  // low score, should be deleted
		{2, 3036, 20, 0, 0}, // higher score, taken instead
	})))
	policy := testRacePolicy()
	policy.DeleteLowScoreTask = true
	policy.DeleteTaskMaxScore = 10
	ops := unionRaceOperations(s, policy, 0)
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
