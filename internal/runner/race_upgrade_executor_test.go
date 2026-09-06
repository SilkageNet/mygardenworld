package runner

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
)

func TestRaceUpgradeNeverRetriesAmbiguousPayment(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		requestError, syncError error
		confirmed               bool
	}{
		{"success", nil, nil, true},
		{"timeout", context.DeadlineExceeded, nil, false},
		{"empty acknowledgement", nil, nil, false},
		{"sync failure", nil, errors.New("offline"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newOperationEventTestRunner()
			op := automation.PlannedOp{Kind: "fmlRace.upgradeTask", Executable: true, RaceBatchID: 42, TaskMsID: 1, Lane: automation.LaneSide}
			calls := 0
			exec := raceUpgradeExecution{
				preflight: func(context.Context) error { return nil },
				reserve:   func() bool { return r.reserveRaceUpgrade(&op) },
				upgrade:   func(context.Context) (json.RawMessage, error) { calls++; return nil, tc.requestError },
				confirm:   func(context.Context) (bool, error) { return tc.confirmed, tc.syncError },
				markStale: func() {},
			}
			_, err := executeRaceUpgrade(context.Background(), exec)
			if (err == nil) != tc.confirmed {
				t.Fatalf("unexpected confirmation: %v", err)
			}
			if _, err := executeRaceUpgrade(context.Background(), exec); err == nil {
				t.Fatal("duplicate paid operation allowed")
			}
			if calls != 1 {
				t.Fatalf("upgrade RPCs = %d, want 1", calls)
			}
			if r.selectRunnableOperation([]automation.PlannedOp{op}, time.Now()) != nil {
				t.Fatal("attempted upgrade kept monopolizing scheduler")
			}
			op.TaskMsID = 2
			if !r.reserveRaceUpgrade(&op) {
				t.Fatal("unrelated task blocked")
			}
		})
	}
}

func TestRaceUpgradePreflightFailureDoesNotSpend(t *testing.T) {
	exec := raceUpgradeExecution{
		preflight: func(context.Context) error { return errors.New("price or held task changed") },
		reserve:   func() bool { t.Fatal("preflight failure reserved a payment"); return false },
	}
	if _, err := executeRaceUpgrade(context.Background(), exec); err == nil {
		t.Fatal("preflight error lost")
	}
}
