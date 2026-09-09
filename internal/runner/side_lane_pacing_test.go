package runner

import (
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
)

func recordTestPacingRequest(r *Runner, kind string, now time.Time) {
	r.pacer.mu.Lock()
	defer r.pacer.mu.Unlock()
	scope, _ := r.pacer.scope(kind)
	r.pacer.lastRequest, r.pacer.lastScope[scope] = now, now
}

func TestSideLanePacingPreservesEligibility(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind string
	}{
		{"account spacing", "unrelated.rpc"},
		{"repeated RPC", "order.finish"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Unix(1_000_000, 0)
			r := newSideLaneTestRunner()
			r.pacer = newRequestPacer(RequestPacing{})
			farm := runnableLaneOp("farm.rpc", automation.LaneFarm, "")
			side := runnableLaneOp("order.finish", automation.LaneSide, "order")
			ops := []automation.PlannedOp{farm, side}
			assertSelectedOperation(t, r.selectRunnableOperation(ops, now), farm.OperationID)
			recordTestPacingRequest(r, tc.kind, now.Add(19*time.Second))
			if selected := r.selectRunnableOperation(ops, now.Add(20*time.Second)); selected != nil {
				t.Fatalf("global spacing bypassed: %+v", selected)
			}
			if got := r.sideLaneFirstWait["order"]; !got.Equal(now) {
				t.Fatalf("spacing reset wait: got %v want %v", got, now)
			}
			assertSelectedOperation(t, r.selectRunnableOperation(ops, now.Add(27*time.Second)), side.OperationID)
		})
	}
}

func TestSideLaneLongRunningFairnessWithPacingAndWakes(t *testing.T) {
	for _, wake := range []bool{false, true} {
		for _, raceSync := range []bool{false, true} {
			r := newSideLaneTestRunner()
			r.pacer = newRequestPacer(RequestPacing{})
			ops := []automation.PlannedOp{
				runnableLaneOp("farm.rpc", automation.LaneFarm, "farm"),
				runnableLaneOp("order.rpc", automation.LaneSide, "order"),
				runnableLaneOp("union.rpc", automation.LaneSide, "union"),
				runnableLaneOp("activity.rpc", automation.LaneSide, "activity"),
			}
			if raceSync {
				sync := runnableLaneOp("fmlRace.getTaskList", automation.LaneSide, "race.sync")
				sync.PreemptFarm, sync.Action = true, "sync"
				ops = append([]automation.PlannedOp{sync}, ops...)
			}
			now := time.Unix(1_000_000, 0)
			last := map[string]time.Time{}
			counts := map[string]int{}
			for i := range 750 { // 100 simulated minutes, no game requests or sleeps.
				at := now.Add(time.Duration(i) * 8 * time.Second)
				op := r.selectRunnableOperation(ops, at)
				if op == nil {
					t.Fatal("unexpected idle outside spacing")
				}
				counts[op.Kind]++
				last[op.Kind] = at
				recordTestPacingRequest(r, op.Kind, at)
				if wake {
					if got := r.selectRunnableOperation(ops, at.Add(100*time.Millisecond)); got != nil {
						t.Fatalf("early wake bypassed pacing: %+v", got)
					}
				}
				for _, candidate := range ops {
					previous, ok := last[candidate.Kind]
					if !ok {
						previous = now
					}
					if at.Sub(previous) > 2*time.Minute {
						t.Fatalf("starved %s with wake=%t raceSync=%t at=%v counts=%v", candidate.Kind, wake, raceSync, at, counts)
					}
				}
			}
			t.Logf("wake=%t raceSync=%t selections=%v", wake, raceSync, counts)
		}
	}
}

func TestSideLaneOldestOverdueScopeWins(t *testing.T) {
	r := newSideLaneTestRunner()
	now := time.Unix(1_000_000, 0)
	high := runnableLaneOp("high", automation.LaneSide, "high")
	low := runnableLaneOp("low", automation.LaneSide, "low")
	r.sideLaneFirstWait["high"] = now.Add(-time.Minute)
	r.sideLaneFirstWait["low"] = now.Add(-2 * time.Minute)
	assertSelectedOperation(t, r.selectRunnableOperation([]automation.PlannedOp{high, low}, now), "low")
}

func TestSideLanePrunesIneligibleEvenWhenPaced(t *testing.T) {
	for _, tc := range []string{"disappeared", "disabled", "resource blocked"} {
		t.Run(tc, func(t *testing.T) {
			r := newSideLaneTestRunner()
			r.pacer = newRequestPacer(RequestPacing{})
			now := time.Unix(1_000_000, 0)
			side := runnableLaneOp("order.rpc", automation.LaneSide, "order")
			r.sideLaneFirstWait["order"] = now.Add(-time.Minute)
			recordTestPacingRequest(r, "other", now)
			ops := []automation.PlannedOp{side}
			switch tc {
			case "disappeared":
				ops = nil
			case "disabled":
				ops[0].Executable = false
			case "resource blocked":
				ops[0].BlockedReasons = []string{"insufficient inventory"}
			}
			if got := r.selectRunnableOperation(ops, now); got != nil {
				t.Fatal("ineligible task selected")
			}
			if len(r.sideLaneFirstWait) != 0 {
				t.Fatal("ineligible wait retained")
			}
		})
	}
}

func TestRaceReservationDoesNotEraseWaitOrAdmitReadSync(t *testing.T) {
	r := newSideLaneTestRunner()
	r.pacer = newRequestPacer(RequestPacing{})
	now := time.Unix(1_000_000, 0)
	sync := runnableLaneOp("fmlRace.getTaskList", automation.LaneSide, "sync")
	sync.PreemptFarm, sync.Action = true, "sync"
	take := runnableLaneOp("fmlRace.takeTask", automation.LaneSide, "take")
	take.PreemptFarm, take.Action = true, "take"
	side := runnableLaneOp("order.rpc", automation.LaneSide, "order")
	r.sideLaneFirstWait["order"] = now.Add(-time.Minute)
	recordTestPacingRequest(r, take.Kind, now.Add(-7*time.Second))
	ops := []automation.PlannedOp{sync, take, side}
	if got := r.selectRunnableOperation(ops, now); got != nil {
		t.Fatalf("read stole reserved slot: %+v", got)
	}
	if got := r.sideLaneFirstWait["order"]; !got.Equal(now.Add(-time.Minute)) {
		t.Fatal("reserved slot reset wait")
	}
	assertSelectedOperation(t, r.selectRunnableOperation(ops, now.Add(time.Second)), take.Kind)
}

func TestIneligibleRaceMutationCannotReserveSlot(t *testing.T) {
	for _, kind := range []string{"fmlRace.upgradeTask", "fmlRace.delTask"} {
		t.Run(kind, func(t *testing.T) {
			r := newSideLaneTestRunner()
			r.pacer = newRequestPacer(RequestPacing{})
			now := time.Unix(1_000_000, 0)
			mutation := runnableLaneOp(kind, automation.LaneSide, kind)
			mutation.PreemptFarm = true
			mutation.RaceBatchID, mutation.TaskMsID = 1, 1
			if kind == "fmlRace.upgradeTask" {
				r.raceUpgradeAttempts = map[[2]int64]bool{{1, 1}: true}
			} else {
				r.safety.LastRaceDeleteMS = now.Add(-time.Minute).UnixMilli()
			}
			recordTestPacingRequest(r, kind, now.Add(-7*time.Second))
			farm := runnableLaneOp("farm.rpc", automation.LaneFarm, "")
			assertSelectedOperation(t, r.selectRunnableOperation([]automation.PlannedOp{mutation, farm}, now), farm.Kind)
		})
	}
}
