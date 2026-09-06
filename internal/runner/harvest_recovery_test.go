package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestUnknownHarvestFailureBacksOffOnlyRejectedLand(t *testing.T) {
	for _, rejection := range []string{`rpc usrLand.harvest: server: {"code":97777,"args":[]}`, "context deadline exceeded"} {
		t.Run(rejection, func(t *testing.T) {
			r := newOperationEventTestRunner()
			now := time.Now()
			op := automation.PlannedOp{Kind: clientproto.RPCUsrLandHarvest.String(), Lane: automation.LaneFarm,
				Executable: true, LandIDs: []int32{1001, 1002}, Count: 2}
			err := &harvestLandError{LandID: 1001, Err: errors.New(rejection)}
			for _, delay := range []time.Duration{30, 60, 120, 240, 300, 300} {
				if got := r.deferFailedHarvest(&op, err, now); got != delay*time.Second {
					t.Fatalf("backoff = %v, want %vs", got, delay)
				}
				filtered := r.applyHarvestBlocks(&op, now.Add(time.Second))
				if filtered == nil || len(filtered.LandIDs) != 1 || filtered.LandIDs[0] != 1002 {
					t.Fatalf("healthy land blocked: %+v", filtered)
				}
				if got := r.applyHarvestBlocks(&op, now.Add(delay*time.Second)); len(got.LandIDs) != 2 {
					t.Fatal("backoff never expires")
				}
				now = now.Add(delay * time.Second)
			}
			if err := r.handleOperationError(context.Background(), operationResult{operationAttempt: operationAttempt{op: &op}, err: err, finishedAt: now}); err != nil {
				t.Fatal(err)
			}
			op.LandIDs = []int32{1001}
			deletion := automation.PlannedOp{Kind: clientproto.RPCFmlRaceDelTask.String(), Lane: automation.LaneSide, Executable: true, TaskMsID: 7}
			selected := r.selectRunnableOperation([]automation.PlannedOp{op, deletion}, now.Add(time.Second))
			if selected == nil || selected.Kind != deletion.Kind {
				t.Fatalf("harvest rejection blocked deletion: %+v", selected)
			}
			guild := op
			guild.Kind = clientproto.RPCFmlLandHarvest.String()
			if r.applyHarvestBlocks(&guild, now) == nil {
				t.Fatal("garden failure leaked into guild land")
			}
		})
	}
}
