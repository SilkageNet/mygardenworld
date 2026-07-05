package runner

import (
	"sort"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
)

const (
	sideOperationBaseCooldown = 60 * time.Second
	sideOperationMaxCooldown  = 10 * time.Minute
)

type operationCooldown struct {
	OperationID  string
	Category     string
	Domain       string
	Lane         string
	Reason       string
	Until        time.Time
	FailureCount int32
}

func (r *Runner) operationCoolingDown(op *automation.PlannedOp, now time.Time) (operationCooldown, bool) {
	if op == nil || op.Lane == automation.LaneFarm {
		return operationCooldown{}, false
	}
	key := operationCooldownKey(op)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.operationCooldowns == nil {
		r.operationCooldowns = make(map[string]operationCooldown)
		return operationCooldown{}, false
	}
	cd, ok := r.operationCooldowns[key]
	if !ok {
		return operationCooldown{}, false
	}
	if !cd.Until.After(now) {
		delete(r.operationCooldowns, key)
		return operationCooldown{}, false
	}
	return cd, true
}

func (r *Runner) setSideOperationCooldown(op *automation.PlannedOp, now time.Time, err error, reason string, explicit time.Duration) operationCooldown {
	if op == nil || op.Lane == automation.LaneFarm {
		return operationCooldown{}
	}
	if reason == "" && err != nil {
		reason = err.Error()
	}
	if reason == "" {
		reason = "操作失败，暂缓重试"
	}
	key := operationCooldownKey(op)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.operationCooldowns == nil {
		r.operationCooldowns = make(map[string]operationCooldown)
	}
	prev := r.operationCooldowns[key]
	failures := prev.FailureCount + 1
	duration := explicit
	if duration <= 0 {
		duration = exponentialCooldown(failures)
	}
	cd := operationCooldown{
		OperationID:  key,
		Category:     op.Category,
		Domain:       op.Domain,
		Lane:         op.Lane,
		Reason:       reason,
		Until:        now.Add(duration),
		FailureCount: failures,
	}
	r.operationCooldowns[key] = cd
	return cd
}

func (r *Runner) cooldownSideOperation(op *automation.PlannedOp, now time.Time, err error, reason string, explicit time.Duration) *automation.PlannedOp {
	if op == nil || op.Lane == automation.LaneFarm {
		return op
	}
	cd := r.setSideOperationCooldown(op, now, err, reason, explicit)
	if cd.OperationID == "" {
		return op
	}
	cp := *op
	cp.CooldownUntil = cd.Until
	cp.CooldownReason = cd.Reason
	return &cp
}

func (r *Runner) clearOperationCooldown(op *automation.PlannedOp) {
	if op == nil {
		return
	}
	key := operationCooldownKey(op)
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.operationCooldowns, key)
}

func (r *Runner) operationCooldownSnapshots(now time.Time) []OperationCooldownSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]OperationCooldownSnapshot, 0, len(r.operationCooldowns))
	for key, cd := range r.operationCooldowns {
		if !cd.Until.After(now) {
			delete(r.operationCooldowns, key)
			continue
		}
		out = append(out, OperationCooldownSnapshot{
			OperationID:  cd.OperationID,
			Category:     cd.Category,
			Domain:       cd.Domain,
			Lane:         cd.Lane,
			Reason:       cd.Reason,
			Until:        cd.Until,
			FailureCount: cd.FailureCount,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Until.Equal(out[j].Until) {
			return out[i].Until.Before(out[j].Until)
		}
		return out[i].OperationID < out[j].OperationID
	})
	return out
}

func operationCooldownKey(op *automation.PlannedOp) string {
	if op == nil {
		return ""
	}
	if op.OperationID != "" {
		return op.OperationID
	}
	parts := []string{op.Kind, op.Domain, op.Action}
	return strings.Join(parts, "|")
}

func exponentialCooldown(failures int32) time.Duration {
	if failures <= 1 {
		return sideOperationBaseCooldown
	}
	duration := sideOperationBaseCooldown
	for i := int32(1); i < failures; i++ {
		duration *= 2
		if duration >= sideOperationMaxCooldown {
			return sideOperationMaxCooldown
		}
	}
	return duration
}
