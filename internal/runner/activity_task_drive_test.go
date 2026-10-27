package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func activityDriveTestRunner(t *testing.T) (*Runner, *automation.PlannedOp) {
	t.Helper()
	r := newOperationEventTestRunner()
	r.state = cyclicNoteRunnerState(t, true)
	now := time.Now()
	r.state.ApplyV(json.RawMessage(fmt.Sprintf(`{"23":{"0":{"9001":{"5":%d,"7":%d}},"3":{"9001|0":{"3":{"4003":75},"5":{}}}}}`, now.Add(-time.Hour).UnixMilli(), now.Add(time.Hour).UnixMilli())))
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Plant.Planting.AutoEnabled = false
	p.Activity.CyclicNote.Enabled = true
	p.Activity.CyclicNote.SatisfyTasks = true
	r.SetPolicy(p)
	view, _ := r.state.CyclicNoteView(now)
	op := &automation.PlannedOp{Kind: clientproto.RPCUsrLandPlantBatch.String(), LandIDs: []int32{1001, 1002}, DemandID: "activity.cyclicNote:9001:3001",
		CyclicNoteDrive: &automation.CyclicNoteTaskDrive{BatchID: 9001, SlotID: 1, TaskID: 4003, TaskType: 3001, Progress: 75, FinishCount: view.FinishCount}}
	if err := r.validateCyclicNoteDrive(op, now); err != nil {
		t.Fatal(err)
	}
	return r, op
}

func TestCyclicNoteDriveWaitsForAuthoritativeProgress(t *testing.T) {
	r, op := activityDriveTestRunner(t)
	r.noteCyclicNoteDriveSuccess(op)
	if len(r.activityBatchSync) != 1 {
		t.Fatal("missing bounded reconciliation")
	}
	if err := r.validateCyclicNoteDrive(op, time.Now()); err == nil {
		t.Fatal("repeated mutation on stale progress")
	}
	for _, progress := range []int32{76, 77} {
		r.state.ApplyV(json.RawMessage(fmt.Sprintf(`{"23":{"3":{"9001|0":{"3":{"4003":%d}}}}}`, progress)))
		op.CyclicNoteDrive.Progress = progress
		err := r.validateCyclicNoteDrive(op, time.Now())
		if (err == nil) != (progress == 77) {
			t.Fatalf("progress=%d error=%v", progress, err)
		}
	}
	if len(r.cyclicNotePending) != 0 {
		t.Fatal("acknowledged reservation retained")
	}
}

func TestCyclicNoteDriveImmediateProgressDoesNotScheduleExtraSync(t *testing.T) {
	r, op := activityDriveTestRunner(t)
	r.state.ApplyV(json.RawMessage(`{"23":{"3":{"9001|0":{"3":{"4003":77}}}}}`))
	r.noteCyclicNoteDriveSuccess(op)
	if len(r.activityBatchSync) != 0 || len(r.cyclicNotePending) != 0 {
		t.Fatal("unnecessary sync after confirmed progress")
	}
}

func TestCyclicNoteDriveRevokedDuringPacing(t *testing.T) {
	for _, change := range []string{"policy", "completion", "replacement", "expiry"} {
		t.Run(change, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r, op := activityDriveTestRunner(t)
				r.pacer = newRequestPacer(RequestPacing{})
				r.pacer.lastRequest = time.Now()
				ctx := context.WithValue(t.Context(), cyclicNoteDriveContextKey{}, op)
				result := make(chan error, 1)
				go func() { result <- r.beforeGameRPC(ctx, op.Kind) }()
				synctest.Wait()
				switch change {
				case "policy":
					p := r.Policy()
					p.Activity.CyclicNote.SatisfyTasks = false
					r.SetPolicy(p)
				case "completion":
					r.state.ApplyV(json.RawMessage(`{"23":{"3":{"9001|0":{"3":{"4003":80}}}}}`))
				case "replacement":
					r.state.ApplyV(json.RawMessage(`{"23":{"0":{"9001":{"14":{"105":{"0":[1005,2001,1006]}}}}}}`))
				case "expiry":
					r.state.ApplyV(json.RawMessage(fmt.Sprintf(`{"23":{"0":{"9001":{"7":%d}}}}`, time.Now().Add(-time.Second).UnixMilli())))
				}
				if err := <-result; err == nil {
					t.Fatal("revoked task mutation passed send guard")
				}
				if err := r.validateCyclicNoteDriveBeforeSend(ctx, clientproto.RPCUsrLazySync.String()); err != nil {
					t.Fatal("confirmation read was blocked", err)
				}
			})
		})
	}
}
