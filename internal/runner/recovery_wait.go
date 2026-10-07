package runner

import (
	"context"
	"time"
)

// recoveryBlockedError is a local admission decision, not a failed server
// probe. It must never move the durable server restriction deadline.
type recoveryBlockedError struct {
	reason  string
	retryAt time.Time
}

func (e *recoveryBlockedError) Error() string { return e.reason }

func (r *Runner) wakeRecovery() {
	select {
	case r.recoveryWake <- struct{}{}:
	default:
	}
}

func (r *Runner) setRecoveryWait(reason string, retryAt time.Time) {
	r.mu.Lock()
	changed := r.recoveryWaitReason != reason
	r.recoveryWaitReason, r.recoveryRetryAt = reason, retryAt
	r.mu.Unlock()
	if changed && reason != "" {
		r.emit(Event{Kind: "account_recovery_waiting", Category: "account", Domain: "account.request", Action: "waiting",
			Label: "账号恢复等待", Message: reason, Level: "warn"})
	}
}

// Settings changes wake local admission without polling or contacting the
// game. A known login-budget deadline may also wake it; shutdown always wins.
func (r *Runner) waitRecovery(ctx context.Context, retryAt time.Time) bool {
	var timer <-chan time.Time
	if !retryAt.IsZero() {
		t := time.NewTimer(max(0, time.Until(retryAt)))
		defer t.Stop()
		timer = t.C
	}
	select {
	case <-ctx.Done():
		return false
	case <-r.recoveryWake:
	case <-timer:
	}
	return ctx.Err() == nil
}

type manualRecoveryKey struct{}
type recoveryAttemptKey struct{}

func (r *Runner) manualRecoveryAuthorized(ctx context.Context) bool {
	return ctx.Value(manualRecoveryKey{}) == r
}

func (r *Runner) recoveryContext(ctx context.Context) context.Context {
	r.mu.RLock()
	pending := r.manualRecoveryPending
	r.mu.RUnlock()
	if pending {
		return context.WithValue(ctx, manualRecoveryKey{}, r)
	}
	return ctx
}
