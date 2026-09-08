package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

type raceTimingKey struct{}

// Operation-local measurements: no account-global clocks that another manual
// request or heartbeat can overwrite. Admission is not proof of server receipt.
type raceTiming struct {
	started, locked, preflightDone, admitted time.Time
	reusedPool                               bool
	preflightPassed                          bool
	poolAgeAtStartMs                         int64
}

func (r *Runner) startRaceTiming(ctx context.Context, op *automation.PlannedOp) (context.Context, *raceTiming) {
	if op.Kind != clientproto.RPCFmlRaceTakeTask.String() {
		return ctx, nil
	}
	timing := &raceTiming{started: time.Now()}
	if fullAt := r.state.FmlRace().FullTasksSyncedAtMs; fullAt > 0 {
		timing.poolAgeAtStartMs = timing.started.UnixMilli() - fullAt
	} else {
		timing.poolAgeAtStartMs = -1
	}
	return context.WithValue(ctx, raceTimingKey{}, timing), timing
}

func (r *Runner) emitRaceTiming(op *automation.PlannedOp, timing *raceTiming, err error) {
	if timing == nil {
		return
	}
	stage := "before_preflight"
	if !timing.preflightDone.IsZero() {
		stage = "preflight_rejected"
		if timing.preflightPassed {
			stage = "before_admission"
		}
	}
	if !timing.admitted.IsZero() {
		stage = "rpc_completed"
	}
	data := map[string]any{
		"task_ms_id": op.TaskMsID, "stage": stage, "reused_full_pool": timing.reusedPool,
		"total_ms":                  time.Since(timing.started).Milliseconds(),
		"full_pool_age_at_start_ms": timing.poolAgeAtStartMs,
	}
	if !timing.locked.IsZero() {
		data["serialization_wait_ms"] = timing.locked.Sub(timing.started).Milliseconds()
	}
	if !timing.preflightDone.IsZero() {
		data["preflight_ms"] = timing.preflightDone.Sub(timing.locked).Milliseconds()
	}
	if !timing.admitted.IsZero() {
		data["admitted_at_ms"] = timing.admitted.UnixMilli()
		data["ready_and_pacing_wait_ms"] = timing.admitted.Sub(timing.preflightDone).Milliseconds()
		if appear := raceTakeAppearTime(r.state, op); !appear.IsZero() {
			data["admission_after_appear_ms"] = timing.admitted.Sub(appear).Milliseconds()
		}
	}
	outcome := "success"
	if err != nil {
		outcome = string(classifyOperationError(op.Kind, err))
		data["error"] = err.Error()
	}
	data["outcome"] = outcome
	raw, _ := json.Marshal(data)
	r.emit(Event{Kind: "race_take_diagnostic", Category: "race", Domain: "union.race.take", Action: "diagnostic",
		Label: "竞赛接单耗时", Message: fmt.Sprintf("竞赛接单 #%d: %s，耗时 %dms（详见明细）", op.TaskMsID, outcome, time.Since(timing.started).Milliseconds()),
		PayloadJSON: string(raw), Level: "info"})
}
