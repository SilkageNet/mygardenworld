package runner

import (
	"context"
	"fmt"
	"time"
)

const riskPausePollInterval = 5 * time.Second

// shanghai is the wall clock for the risk-control rest cycle (00:00 local).
var shanghai = time.FixedZone("Asia/Shanghai", 8*60*60)

// riskPausePhase is one instant on the midnight-aligned run/pause cycle.
type riskPausePhase struct {
	pausing     bool
	nextPauseAt time.Time
	pauseUntil  time.Time
	cycleStart  time.Time
}

func (r *Runner) riskPauseDeadline() (time.Time, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.riskPauseUntil.IsZero() {
		return time.Time{}, false
	}
	return r.riskPauseUntil, true
}

func (r *Runner) clearRiskPauseLocked() {
	r.riskPauseUntil = time.Time{}
}

// riskPausePhaseAt maps now onto the Shanghai-midnight cycle
// [run, run+pause, run, run+pause, ...]. 120/12 stops at 02:00 and resumes at 02:12.
func riskPausePhaseAt(now time.Time, runMinutes, pauseMinutes int32) (riskPausePhase, bool) {
	if runMinutes <= 0 || pauseMinutes <= 0 {
		return riskPausePhase{}, false
	}
	local := now.In(shanghai)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, shanghai)
	elapsed := local.Sub(midnight)
	if elapsed < 0 {
		elapsed = 0
	}
	cycle := time.Duration(runMinutes+pauseMinutes) * time.Minute
	run := time.Duration(runMinutes) * time.Minute
	pos := elapsed % cycle
	cycleStart := midnight.Add(elapsed - pos)
	if pos < run {
		return riskPausePhase{
			nextPauseAt: cycleStart.Add(run),
			cycleStart:  cycleStart,
		}, true
	}
	return riskPausePhase{
		pausing:    true,
		pauseUntil: cycleStart.Add(cycle),
		cycleStart: cycleStart,
	}, true
}

func (r *Runner) currentRiskPausePhase(now time.Time) (riskPausePhase, bool) {
	r.mu.RLock()
	policy := r.policy
	r.mu.RUnlock()
	if policy == nil || !policy.GetBasic().GetRunPauseEnabled() {
		return riskPausePhase{}, false
	}
	return riskPausePhaseAt(now, policy.GetBasic().GetRunDurationMinutes(), policy.GetBasic().GetPauseDurationMinutes())
}

// RiskPauseStatus is the monitoring-facing snapshot of the timed rest cycle.
type RiskPauseStatus struct {
	Enabled              bool
	Pausing              bool
	RunSegmentStartedAt  time.Time
	NextPauseAt          time.Time
	PauseUntil           time.Time
	RunDurationMinutes   int32
	PauseDurationMinutes int32
}

// RiskPauseSnapshot reports the wall-clock rest window. Login time and kicks
// do not move it.
func (r *Runner) RiskPauseSnapshot(now time.Time) RiskPauseStatus {
	r.mu.RLock()
	policy := r.policy
	r.mu.RUnlock()
	out := RiskPauseStatus{}
	if policy == nil || !policy.GetBasic().GetRunPauseEnabled() {
		return out
	}
	out.Enabled = true
	out.RunDurationMinutes = policy.GetBasic().GetRunDurationMinutes()
	out.PauseDurationMinutes = policy.GetBasic().GetPauseDurationMinutes()
	phase, ok := riskPausePhaseAt(now, out.RunDurationMinutes, out.PauseDurationMinutes)
	if !ok {
		return out
	}
	out.RunSegmentStartedAt = phase.cycleStart
	if phase.pausing {
		out.Pausing = true
		out.PauseUntil = phase.pauseUntil
		return out
	}
	out.NextPauseAt = phase.nextPauseAt
	return out
}

// inRiskPauseLocked reports an intentional rest disconnect. Caller must hold r.mu.
func (r *Runner) inRiskPauseLocked(now time.Time) bool {
	return !r.riskPauseUntil.IsZero() && r.riskPauseUntil.After(now)
}

// deferStartForRiskPause skips the initial login when Start lands inside a rest
// window. The connection loop waits until the scheduled resume time.
func (r *Runner) deferStartForRiskPause(now time.Time) bool {
	phase, ok := r.currentRiskPausePhase(now)
	if !ok || !phase.pausing {
		return false
	}
	r.mu.Lock()
	r.riskPauseUntil = phase.pauseUntil
	r.mu.Unlock()
	r.emit(Event{
		Kind: "risk_pause",
		Message: fmt.Sprintf(
			"休眠：当前处于休息时段，%s 自动启动",
			phase.pauseUntil.In(shanghai).Format("15:04:05"),
		),
		Level: "info",
	})
	return true
}

// maybeEnterRiskPause disconnects when the Shanghai wall clock enters a rest
// window. Returns true when a pause was started (caller should skip the rest
// of the tick).
func (r *Runner) maybeEnterRiskPause(now time.Time) bool {
	phase, ok := r.currentRiskPausePhase(now)
	r.mu.Lock()
	if !ok {
		r.clearRiskPauseLocked()
		r.mu.Unlock()
		return false
	}
	if !phase.pausing {
		r.clearRiskPauseLocked()
		r.mu.Unlock()
		return false
	}
	r.riskPauseUntil = phase.pauseUntil
	client := r.client
	alreadyClosed := client == nil || client.Closed()
	r.mu.Unlock()
	if alreadyClosed {
		return true
	}
	r.emit(Event{
		Kind: "risk_pause",
		Message: fmt.Sprintf(
			"休眠：%s 起断开，%s 自动启动",
			now.In(shanghai).Format("15:04:05"),
			phase.pauseUntil.In(shanghai).Format("15:04:05"),
		),
		Level: "info",
	})
	_ = client.Close()
	return true
}

// awaitRiskPause blocks in the connection loop until the scheduled rest window
// ends or the feature is disabled. Returns false when the runner context is cancelled.
func (r *Runner) awaitRiskPause(ctx context.Context) bool {
	announced := false
	for {
		now := time.Now()
		phase, ok := r.currentRiskPausePhase(now)
		if !ok || !phase.pausing {
			r.mu.Lock()
			r.clearRiskPauseLocked()
			r.mu.Unlock()
			if ok {
				r.emit(Event{Kind: "risk_pause", Message: "休眠结束，正在自动启动", Level: "info"})
			} else {
				r.emit(Event{Kind: "risk_pause", Message: "休眠已关闭，准备重新连接", Level: "info"})
			}
			return true
		}
		r.mu.Lock()
		r.riskPauseUntil = phase.pauseUntil
		r.mu.Unlock()
		wait := phase.pauseUntil.Sub(now)
		if wait <= 0 {
			continue
		}
		if !announced {
			r.emit(Event{
				Kind: "risk_pause",
				Message: fmt.Sprintf(
					"休眠中，%s 自动启动",
					phase.pauseUntil.In(shanghai).Format("15:04:05"),
				),
				Level: "info",
			})
			announced = true
		}
		chunk := wait
		if chunk > riskPausePollInterval {
			chunk = riskPausePollInterval
		}
		if !sleepOrDone(ctx, chunk) {
			return false
		}
	}
}

// riskPauseReconnectPath reports whether the connection loop should wait out a
// risk pause instead of using the normal quick-reconnect path.
func (r *Runner) riskPauseReconnectPath() bool {
	_, active := r.riskPauseDeadline()
	if active {
		return true
	}
	phase, ok := r.currentRiskPausePhase(time.Now())
	return ok && phase.pausing
}
