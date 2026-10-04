package runner

import (
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
)

func TestOperationBackoffSurvivesDeadlineAndDiagnosticReads(t *testing.T) {
	for _, tc := range []struct {
		kind      string
		base, cap time.Duration
	}{
		{"shopCultivate.buy", time.Minute, 10 * time.Minute},
		{"fmlRace.getTaskList", 30 * time.Second, 5 * time.Minute},
		{"fmlRace.enter", 30 * time.Second, 5 * time.Minute},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			r := newOperationEventTestRunner()
			op := &automation.PlannedOp{Kind: tc.kind, Lane: automation.LaneSide}
			now := time.Now()
			for attempt := range 7 {
				cd := r.setSideOperationCooldown(op, now, errors.New("unavailable"), "", 0)
				want := min(tc.base*time.Duration(1<<attempt), tc.cap)
				if cd.Until.Sub(now) != want || cd.FailureCount != int32(attempt+1) {
					t.Fatalf("attempt %d: %+v want %s", attempt, cd, want)
				}
				if _, blocked := r.operationCoolingDown(op, cd.Until.Add(-time.Nanosecond)); !blocked {
					t.Fatal("cooldown ended early")
				}
				now = cd.Until
				if len(r.operationCooldownSnapshots(now)) != 0 {
					t.Fatal("expired deadline still visible")
				}
				if _, blocked := r.operationCoolingDown(op, now); blocked {
					t.Fatal("history blocked a due retry")
				}
			}
			r.clearOperationCooldown(op)
			if cd := r.setSideOperationCooldown(op, now, nil, "", 0); cd.Until.Sub(now) != tc.base {
				t.Fatal("success did not reset backoff")
			}
		})
	}
}

func TestCooldownHistoryExpiresAndExplicitDelaysStayExplicit(t *testing.T) {
	for _, read := range []string{"none", "scheduler", "diagnostics"} {
		t.Run(read, func(t *testing.T) {
			r := newOperationEventTestRunner()
			op := &automation.PlannedOp{Kind: "orderFlower.finishOrder", Lane: automation.LaneSide}
			now := time.Now()
			for range 2 {
				cd := r.setSideOperationCooldown(op, now, nil, "", 30*time.Second)
				if cd.Until.Sub(now) != 30*time.Second {
					t.Fatal("explicit cooldown escalated")
				}
				now = cd.Until
			}
			now = now.Add(operationFailureRetention)
			switch read {
			case "scheduler":
				r.operationCoolingDown(op, now)
			case "diagnostics":
				r.operationCooldownSnapshots(now)
			}
			if read != "none" && len(r.operationCooldowns) != 0 {
				t.Fatal("expired history not pruned")
			}
			if cd := r.setSideOperationCooldown(op, now, nil, "", 0); cd.FailureCount != 1 {
				t.Fatal("idle history contaminated new failures")
			}
		})
	}
}

func TestRaceSyncBackoffCannotBeBypassedByNewPlan(t *testing.T) {
	r := newOperationEventTestRunner()
	now := time.Now()
	op := &automation.PlannedOp{Kind: "fmlRace.getTaskList", Lane: automation.LaneSide, OperationID: "periodic", CooldownKey: "periodic"}
	result := operationResult{operationAttempt: operationAttempt{op: op}, finishedAt: now, err: errors.New("unavailable")}
	if err := r.handleRaceSyncFailure(t.Context(), result, "test"); err != nil {
		t.Fatal(err)
	}
	op.OperationID, op.CooldownKey = "push", "push"
	if cd, blocked := r.operationCoolingDown(op, now); !blocked || cd.Until.Sub(now) != 30*time.Second {
		t.Fatalf("new plan bypassed RPC backoff: %+v", cd)
	}
	r.operationCooldownSnapshots(now.Add(30 * time.Second))
	result.finishedAt = now.Add(30 * time.Second)
	if err := r.handleRaceSyncFailure(t.Context(), result, "test"); err != nil {
		t.Fatal(err)
	}
	if cd, blocked := r.operationCoolingDown(op, result.finishedAt); !blocked || cd.Until.Sub(result.finishedAt) != time.Minute {
		t.Fatalf("race retry did not escalate: %+v", cd)
	}
	other := &automation.PlannedOp{Kind: "orderFlower.enter", Lane: automation.LaneSide}
	if _, blocked := r.operationCoolingDown(other, now); blocked {
		t.Fatal("local failure blocked another domain")
	}
}
