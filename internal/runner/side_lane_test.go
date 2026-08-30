package runner

import (
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

func TestSideLaneRaceSyncPreemptsFarm(t *testing.T) {
	now := time.Date(2026, 7, 12, 11, 5, 0, 0, time.UTC)
	r := newSideLaneTestRunner()
	farm := runnableLaneOp("farm.harvest", automation.LaneFarm, "")
	sync := runnableLaneOp("fmlRace.getTaskList", automation.LaneSide, "union.race.sync")
	sync.Domain = "union.race.sync"
	sync.Action = "sync"
	sync.PreemptFarm = true
	assertSelectedOperation(t, r.selectRunnableOperation([]automation.PlannedOp{farm, sync}, now), sync.OperationID)
}

func TestSideLaneRepeatedRaceSyncYieldsToFarm(t *testing.T) {
	now := time.Date(2026, 8, 30, 22, 4, 0, 0, time.UTC)
	r := newSideLaneTestRunner()
	farm := runnableLaneOp("farm.harvest", automation.LaneFarm, "")
	sync := runnableLaneOp("fmlRace.getTaskList", automation.LaneSide, "union.race.sync")
	sync.Domain = "union.race.sync"
	sync.Action = "sync"
	sync.PreemptFarm = true
	candidates := []automation.PlannedOp{farm, sync}

	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now), sync.OperationID)
	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now.Add(time.Second)), farm.OperationID)
	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now.Add(2*time.Second)), sync.OperationID)
}

func TestSideLaneRaceTakeStillPreemptsAfterRaceSync(t *testing.T) {
	now := time.Date(2026, 8, 30, 22, 4, 0, 0, time.UTC)
	r := newSideLaneTestRunner()
	farm := runnableLaneOp("farm.harvest", automation.LaneFarm, "")
	sync := runnableLaneOp("fmlRace.getTaskList", automation.LaneSide, "union.race.sync")
	sync.Action = "sync"
	sync.PreemptFarm = true
	take := runnableLaneOp("fmlRace.takeTask", automation.LaneSide, "union.race.take")
	take.Action = "take"
	take.PreemptFarm = true

	assertSelectedOperation(t, r.selectRunnableOperation([]automation.PlannedOp{farm, sync}, now), sync.OperationID)
	assertSelectedOperation(t, r.selectRunnableOperation([]automation.PlannedOp{farm, take}, now.Add(time.Second)), take.OperationID)
}

func TestSideLaneRaceTakePreemptsFarmAndFarmTurn(t *testing.T) {
	now := time.Date(2026, 7, 12, 11, 0, 0, 0, time.UTC)
	r := newSideLaneTestRunner()
	r.sideLaneFarmTurn = true
	farm := runnableLaneOp("farm.harvest", automation.LaneFarm, "")
	take := runnableLaneOp("fmlRace.takeTask", automation.LaneSide, "union.race.take")
	take.Domain = "union.race.take"
	take.PreemptFarm = true
	candidates := []automation.PlannedOp{farm, take}

	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now), take.OperationID)
	if !r.sideLaneFarmTurn {
		t.Fatal("race take should still request a following farm turn")
	}
}

func TestSideLaneFairnessForcesScopeAtTwentySeconds(t *testing.T) {
	now := time.Date(2026, 7, 12, 10, 0, 0, 0, time.UTC)
	r := newSideLaneTestRunner()
	farm := runnableLaneOp("farm.harvest", automation.LaneFarm, "")
	side := runnableLaneOp("task.daily", automation.LaneSide, "basic.daily")
	candidates := []automation.PlannedOp{farm, side}

	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now), farm.OperationID)
	if first := r.sideLaneFirstWait["basic.daily"]; !first.Equal(now) {
		t.Fatalf("first wait=%v, want %v", first, now)
	}
	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now.Add(sideLaneMaxWait-time.Nanosecond)), farm.OperationID)
	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now.Add(sideLaneMaxWait)), side.OperationID)
	if _, tracked := r.sideLaneFirstWait["basic.daily"]; tracked {
		t.Fatal("forced Side scope retained its old wait timestamp")
	}
	if !r.sideLaneFarmTurn {
		t.Fatal("forced Side did not require a Farm turn")
	}
}

func TestSideLaneFairnessUsesHighestSortedDueScopeAndReinsertsFarm(t *testing.T) {
	now := time.Date(2026, 7, 12, 10, 15, 0, 0, time.UTC)
	r := newSideLaneTestRunner()
	farm := runnableLaneOp("farm.water", automation.LaneFarm, "")
	high := runnableLaneOp("side.high", automation.LaneSide, "scope.high")
	low := runnableLaneOp("side.low", automation.LaneSide, "") // OperationID fallback is the scope.
	candidates := []automation.PlannedOp{farm, high, low}

	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now), farm.OperationID)
	if len(r.sideLaneFirstWait) != 2 {
		t.Fatalf("tracked scopes=%v, want two independent scopes", r.sideLaneFirstWait)
	}
	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now.Add(sideLaneMaxWait)), high.OperationID)
	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now.Add(sideLaneMaxWait+time.Second)), farm.OperationID)
	assertSelectedOperation(t, r.selectRunnableOperation(candidates, now.Add(sideLaneMaxWait+2*time.Second)), low.OperationID)
}

func TestSideLaneFairnessResetsFutureWaitAfterClockRollback(t *testing.T) {
	now := time.Date(2026, 7, 12, 10, 25, 0, 0, time.UTC)
	r := newSideLaneTestRunner()
	farm := runnableLaneOp("farm.water", automation.LaneFarm, "")
	side := runnableLaneOp("side.clock", automation.LaneSide, "scope.clock")
	r.sideLaneFirstWait["scope.clock"] = now.Add(time.Minute)

	assertSelectedOperation(t, r.selectRunnableOperation([]automation.PlannedOp{farm, side}, now), farm.OperationID)
	if first := r.sideLaneFirstWait["scope.clock"]; !first.Equal(now) {
		t.Fatalf("clock rollback first wait=%v, want reset to %v", first, now)
	}
}

func TestSideLaneFairnessAllowsConsecutiveSideWithoutFarm(t *testing.T) {
	now := time.Date(2026, 7, 12, 10, 30, 0, 0, time.UTC)
	r := newSideLaneTestRunner()
	farm := runnableLaneOp("farm.plant", automation.LaneFarm, "")
	first := runnableLaneOp("side.first", automation.LaneSide, "scope.first")
	second := runnableLaneOp("side.second", automation.LaneSide, "scope.second")

	assertSelectedOperation(t, r.selectRunnableOperation([]automation.PlannedOp{farm, first, second}, now), farm.OperationID)
	assertSelectedOperation(t, r.selectRunnableOperation([]automation.PlannedOp{farm, first, second}, now.Add(sideLaneMaxWait)), first.OperationID)
	assertSelectedOperation(t, r.selectRunnableOperation([]automation.PlannedOp{first, second}, now.Add(sideLaneMaxWait+time.Second)), second.OperationID)
}

func TestSideLaneFairnessPrunesCoolingAndDisappearedScopes(t *testing.T) {
	now := time.Date(2026, 7, 12, 10, 45, 0, 0, time.UTC)
	r := newSideLaneTestRunner()
	farm := runnableLaneOp("farm.harvest", automation.LaneFarm, "")
	cooling := runnableLaneOp("side.cooling", automation.LaneSide, "scope.cooling")
	disappeared := runnableLaneOp("side.disappeared", automation.LaneSide, "scope.disappeared")
	surviving := runnableLaneOp("side.surviving", automation.LaneSide, "scope.surviving")

	assertSelectedOperation(t, r.selectRunnableOperation([]automation.PlannedOp{farm, cooling, disappeared, surviving}, now), farm.OperationID)
	r.setSideOperationCooldown(&cooling, now.Add(time.Second), errors.New("retry later"), "", time.Minute)
	assertSelectedOperation(t, r.selectRunnableOperation([]automation.PlannedOp{farm, cooling, surviving}, now.Add(2*time.Second)), farm.OperationID)
	if len(r.sideLaneFirstWait) != 1 {
		t.Fatalf("tracked scopes after prune=%v, want only surviving scope", r.sideLaneFirstWait)
	}
	if _, ok := r.sideLaneFirstWait["scope.surviving"]; !ok {
		t.Fatalf("surviving scope was pruned: %v", r.sideLaneFirstWait)
	}

	r.sideLaneFarmTurn = true
	assertSelectedOperation(t, r.selectRunnableOperation([]automation.PlannedOp{farm}, now.Add(3*time.Second)), farm.OperationID)
	if len(r.sideLaneFirstWait) != 0 || r.sideLaneFarmTurn || r.raceSyncNeedsFarmTurn {
		t.Fatalf("empty Side set did not clear fairness state: waits=%v farmTurn=%t raceSyncFarmTurn=%t", r.sideLaneFirstWait, r.sideLaneFarmTurn, r.raceSyncNeedsFarmTurn)
	}
}

func TestSideLaneFairnessLifecycleResets(t *testing.T) {
	t.Run("policy off", func(t *testing.T) {
		r := newSideLaneTestRunner()
		seedSideLaneFairness(r)
		r.SetPolicy(automation.DefaultPolicy())
		assertSideLaneFairnessReset(t, r)
	})

	t.Run("disconnect", func(t *testing.T) {
		r := newSideLaneTestRunner()
		seedSideLaneFairness(r)
		client := &babigame.Client{}
		r.client = client
		r.clearDisconnectedClient(client)
		assertSideLaneFairnessReset(t, r)
	})

	t.Run("fresh session", func(t *testing.T) {
		r := newSideLaneTestRunner()
		seedSideLaneFairness(r)
		r.resetFreshSessionAutomationState()
		assertSideLaneFairnessReset(t, r)
	})

	t.Run("explicit disabled policy passed to selector", func(t *testing.T) {
		r := newSideLaneTestRunner()
		seedSideLaneFairness(r)
		if op := r.nextRunnableOperation(automation.DefaultPolicy(), time.Now()); op != nil {
			t.Fatalf("disabled policy selected operation %+v", op)
		}
		assertSideLaneFairnessReset(t, r)
	})
}

func newSideLaneTestRunner() *Runner {
	return &Runner{
		harvestBlockedUntil: make(map[int32]time.Time),
		operationCooldowns:  make(map[string]operationCooldown),
		sideLaneFirstWait:   make(map[string]time.Time),
	}
}

func runnableLaneOp(operationID, lane, cooldownKey string) automation.PlannedOp {
	return automation.PlannedOp{
		OperationID: operationID,
		CooldownKey: cooldownKey,
		Kind:        operationID,
		Lane:        lane,
		Executable:  true,
	}
}

func assertSelectedOperation(t *testing.T, op *automation.PlannedOp, want string) {
	t.Helper()
	if op == nil || op.OperationID != want {
		t.Fatalf("selected=%+v, want %q", op, want)
	}
}

func seedSideLaneFairness(r *Runner) {
	r.sideLaneFirstWait["scope"] = time.Now().Add(-sideLaneMaxWait)
	r.sideLaneFarmTurn = true
	r.raceSyncNeedsFarmTurn = true
}

func assertSideLaneFairnessReset(t *testing.T, r *Runner) {
	t.Helper()
	if len(r.sideLaneFirstWait) != 0 || r.sideLaneFarmTurn || r.raceSyncNeedsFarmTurn {
		t.Fatalf("fairness state not reset: waits=%v farmTurn=%t raceSyncFarmTurn=%t", r.sideLaneFirstWait, r.sideLaneFarmTurn, r.raceSyncNeedsFarmTurn)
	}
}
