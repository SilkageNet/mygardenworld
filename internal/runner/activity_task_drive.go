package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

type cyclicNoteDriveContextKey struct{}

func cyclicNotePendingKey(drive automation.CyclicNoteTaskDrive) automation.CyclicNoteTaskDrive {
	drive.Progress, drive.FinishCount = 0, 0
	return drive
}

func (r *Runner) validateCyclicNoteDrive(op *automation.PlannedOp, now time.Time) error {
	if op == nil || op.CyclicNoteDrive == nil {
		return nil
	}
	if err := automation.ValidateCyclicNoteTaskOperation(r.state, r.Policy(), op, now); err != nil {
		return err
	}
	key := cyclicNotePendingKey(*op.CyclicNoteDrive)
	r.mu.Lock()
	expected, pending := r.cyclicNotePending[key]
	if pending && op.CyclicNoteDrive.Progress >= expected {
		delete(r.cyclicNotePending, key)
		pending = false
	}
	r.mu.Unlock()
	if pending {
		return fmt.Errorf("花笺集芳操作已提交，等待服务器任务进度确认（至少 %d）；不重复补做", expected)
	}
	return nil
}

func (r *Runner) validateCyclicNoteDriveBeforeSend(ctx context.Context, name string) error {
	op, _ := ctx.Value(cyclicNoteDriveContextKey{}).(*automation.PlannedOp)
	if op == nil || name != op.Kind {
		return nil
	}
	return r.checkOperationResources(op, time.Now())
}

// Do not infer task progress from a successful business RPC. While namespace
// 23 lags behind, park only this task and reconcile through the existing
// bounded activity sync queue. Other automation remains runnable.
func (r *Runner) noteCyclicNoteDriveSuccess(op *automation.PlannedOp) {
	if op == nil {
		return
	}
	if op.Kind == clientproto.RPCActCyclicNoteRecvTaskRwd.String() {
		r.mu.Lock()
		for key := range r.cyclicNotePending {
			if key.BatchID == op.BatchID && key.SlotID == op.SlotID && key.TaskID == op.TaskID {
				delete(r.cyclicNotePending, key)
			}
		}
		r.mu.Unlock()
		return
	}
	count := automation.CyclicNoteOperationCount(op)
	if count <= 0 {
		return
	}
	// Seeding and clearing mature plots are prerequisites, not a confirmed
	// planting-task increment. Fencing them would block the first watering.
	if op.Kind == clientproto.RPCUsrLandPlant.String() || op.Kind == clientproto.RPCUsrLandPlantBatch.String() || op.Kind == clientproto.RPCUsrLandHarvest.String() {
		return
	}
	key := cyclicNotePendingKey(*op.CyclicNoteDrive)
	view, _ := r.state.CyclicNoteView(time.Now())
	for _, task := range view.Tasks {
		if view.BatchID == key.BatchID && task.SlotID == key.SlotID && task.TaskID == key.TaskID && task.ProgressObserved && task.Progress >= op.CyclicNoteDrive.Progress+count {
			return
		}
	}
	r.mu.Lock()
	if r.cyclicNotePending == nil {
		r.cyclicNotePending = make(map[automation.CyclicNoteTaskDrive]int32)
	}
	// Only a bounded number of live slots can need confirmation. Discard
	// replaced batches/tasks rather than accumulating historical task IDs.
	for previous := range r.cyclicNotePending {
		if previous.BatchID != key.BatchID || (previous.SlotID == key.SlotID && previous.TaskID != key.TaskID) {
			delete(r.cyclicNotePending, previous)
		}
	}
	r.cyclicNotePending[key] = op.CyclicNoteDrive.Progress + count
	r.mu.Unlock()
	if err := r.validateCyclicNoteDrive(op, time.Now()); err != nil {
		r.queueActivityBatchSync([]int32{key.BatchID})
	}
}
