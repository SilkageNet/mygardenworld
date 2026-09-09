package runner

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
)

const schedulerWaitDiagnosticInterval = time.Minute

// Report prolonged eligible work, not every normal spacing delay. One bounded
// summary per account/minute is enough to distinguish a live but busy scheduler
// from absent/blocked planner candidates without flooding execution logs.
func (r *Runner) emitSchedulerWaitDiagnostic(candidates []automation.PlannedOp, selected *automation.PlannedOp, now time.Time) {
	r.mu.Lock()
	if !r.lastSchedulerWaitDiagnostic.IsZero() && !now.Before(r.lastSchedulerWaitDiagnostic) &&
		now.Sub(r.lastSchedulerWaitDiagnostic) < schedulerWaitDiagnosticInterval {
		r.mu.Unlock()
		return
	}
	var oldest *automation.PlannedOp
	var first time.Time
	for i := range candidates {
		op := &candidates[i]
		if !isSideLane(*op) || !runnablePlannedOp(*op) {
			continue
		}
		wait, ok := r.sideLaneFirstWait[sideLaneWaitScope(op)]
		if !ok || now.Sub(wait) < schedulerWaitDiagnosticInterval {
			continue
		}
		if oldest == nil || wait.Before(first) {
			oldest, first = op, wait
		}
	}
	if oldest == nil {
		r.mu.Unlock()
		return
	}
	r.lastSchedulerWaitDiagnostic = now
	r.mu.Unlock()

	delay := r.pacer.delay(oldest.Kind, now)
	if oldest.Kind == "fmlRace.delTask" {
		delay = max(delay, r.raceDeleteWait(now))
	}
	reason := "等待其他任务的执行机会"
	if delay > 0 {
		reason = "等待请求间隔，保留排队时间"
	} else if selected != nil && automation.IsUrgentRaceOp(*selected) && !isYieldingRaceSync(*selected) {
		reason = "让位于紧急竞赛操作"
	}
	selectedKind := ""
	if selected != nil {
		selectedKind = selected.Kind
	}
	raw, _ := json.Marshal(map[string]any{
		"waiting_operation": oldest.Kind, "waiting_domain": oldest.Domain,
		"wait_seconds": int64(now.Sub(first).Seconds()), "send_delay_ms": delay.Milliseconds(),
		"selected_operation": selectedKind, "reason": reason,
	})
	r.emit(Event{Kind: "scheduler_wait", Category: "system", Domain: "system.scheduler", Action: "wait",
		Label: "调度等待", Message: fmt.Sprintf("%s 已等待 %d 秒：%s", oldest.Kind, int64(now.Sub(first).Seconds()), reason),
		PayloadJSON: string(raw)})
}
