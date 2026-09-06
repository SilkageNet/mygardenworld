package runner

import (
	"context"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestRacePreflightFailureYieldsToOtherTask(t *testing.T) {
	for _, kind := range []string{clientproto.RPCFmlRaceDelTask.String(), clientproto.RPCFmlRaceTakeTask.String()} {
		t.Run(kind, func(t *testing.T) {
			r := newOperationEventTestRunner()
			op := automation.PlannedOp{
				Kind: kind, OperationID: kind, CooldownKey: "race-task:1",
				Lane: automation.LaneSide, Category: automation.CategoryRace,
				Executable: true, Status: automation.PlanStatusManaged, TaskMsID: 1,
			}
			// A failed refresh must leave the same retry safeguards as an RPC
			// failure. No game client means this exercises the real early return.
			if err := r.executeOperation(context.Background(), nil, nil, &op, time.Now()); err == nil {
				t.Fatal("expected failed task-pool refresh")
			}
			now := time.Now()
			if _, cooling := r.operationCoolingDown(&op, now.Add(4*time.Second)); !cooling {
				t.Fatal("preflight failure must yield at least the next ordinary tick")
			}
			other := op
			other.OperationID = kind + ":2"
			other.CooldownKey = "race-task:2"
			other.TaskMsID = 2
			selected := r.selectRunnableOperation([]automation.PlannedOp{op, other}, now)
			if kind == clientproto.RPCFmlRaceDelTask.String() {
				if selected != nil {
					t.Fatalf("delete failure must retain account-wide interval: %+v", selected)
				}
			} else if selected == nil || selected.TaskMsID != 2 {
				t.Fatalf("failed preflight blocked another task: %+v", selected)
			}
			if _, cooling := r.operationCoolingDown(&op, now.Add(6*time.Second)); cooling {
				t.Fatal("preflight backoff must allow a later retry")
			}
		})
	}
}
