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

// This permit belongs only to one synchronous user command. It is never
// inherited by background reconnects and a new server rejection revokes it.
type manualRecoveryPermit struct {
	runner   *Runner
	revision uint64
}

func (r *Runner) manualRecoveryAuthorized(ctx context.Context) bool {
	permit, ok := ctx.Value(manualRecoveryKey{}).(manualRecoveryPermit)
	if !ok || permit.runner != r {
		return false
	}
	_, revision := r.accountSafetySnapshot()
	return permit.revision == revision
}

func (r *Runner) manualRecoveryAtRevision(ctx context.Context, revision uint64) bool {
	permit, ok := ctx.Value(manualRecoveryKey{}).(manualRecoveryPermit)
	return ok && permit.runner == r && permit.revision == revision
}

func (r *Runner) manualRecoveryContext(ctx context.Context) context.Context {
	_, revision := r.accountSafetySnapshot()
	return context.WithValue(ctx, manualRecoveryKey{}, manualRecoveryPermit{r, revision})
}
