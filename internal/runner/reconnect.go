package runner

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

func (r *Runner) connectionLoop(ctx context.Context, username, password string, client *babigame.Client) {
	current := client

connection:
	for {
		if current != nil {
			select {
			case <-ctx.Done():
				return
			case <-current.Done():
			}
			r.clearDisconnectedClient(current)
		}
		if ctx.Err() != nil {
			return
		}
		if r.autoReloginPending() {
			next, ok := r.reloginAfterDisplacement(ctx, username, password)
			if !ok {
				return
			}
			current = next
			continue
		}
		if r.isSessionInvalidated() {
			return
		}
		message := "网络连接断开，准备重连"
		if current == nil {
			message = "WebSocket 首次连接失败，准备自动重连"
		}
		r.emit(Event{Kind: "ws_disconnected", Message: message, Level: "warn"})

		wait := reconnectInitialWait
		for {
			if !sleepOrDone(ctx, wait) || r.isSessionInvalidated() {
				if r.autoReloginPending() {
					current = nil
					continue connection
				}
				return
			}
			next, err := r.connectStoredOrFresh(ctx, username, password)
			if err == nil {
				current = next
				break
			}
			if isReputationGuardError(err) {
				return
			}
			if r.restrictionError() != nil {
				// Coded failures already carry a single pause event. Do not
				// relabel them as a 2-second network retry or evict the cache.
				if s, revision := r.accountSafetySnapshot(); s.RestrictedUntilMS <= time.Now().UnixMilli() {
					r.deferRestrictionProbe(revision, err)
				}
				if !r.waitAccountRestriction(ctx) {
					return
				}
				wait = reconnectInitialWait
				continue
			}
			if ctx.Err() != nil || r.isSessionInvalidated() {
				if r.autoReloginPending() {
					current = nil
					continue connection
				}
				return
			}
			r.emit(Event{
				Kind:    "ws_disconnected",
				Message: fmt.Sprintf("重连失败: %v；%s 后重试", err, nextReconnectWait(wait)),
				Level:   "warn",
			})
			wait = nextReconnectWait(wait)
		}
	}
}

func (r *Runner) reloginAfterDisplacement(ctx context.Context, username, password string) (*babigame.Client, bool) {
	baseWait := r.reloginInterval()
	wait := baseWait
	for {
		if ctx.Err() != nil {
			return nil, false
		}
		if !sleepOrDone(ctx, wait) {
			return nil, false
		}
		if ctx.Err() != nil {
			return nil, false
		}
		if !r.prepareAutoReloginAttempt() {
			return nil, false
		}
		r.emit(Event{Kind: "session_relogin", Message: "被挤号等待结束，正在自动登录", Level: "info"})
		next, err := r.connectFresh(ctx, username, password)
		if err == nil {
			if ctx.Err() != nil || !r.completeAutoRelogin() {
				_ = next.Close()
				r.clearDisconnectedClient(next)
				if ctx.Err() != nil {
					return nil, false
				}
				if r.autoReloginPending() {
					wait = baseWait
					continue
				}
				r.failClosedPendingDisplacedRelogin()
				return nil, false
			}
			return next, true
		}
		if ctx.Err() != nil || isReputationGuardError(err) || r.sessionInvalidatedWithoutAutoRelogin() {
			return nil, false
		}
		if r.autoReloginPending() {
			wait = baseWait
			continue
		}
		nextWait := nextReloginWait(wait, baseWait)
		r.emit(Event{
			Kind:    "session_relogin",
			Message: fmt.Sprintf("自动登录失败: %v；%s 后重试", err, nextWait),
			Level:   "warn",
		})
		wait = nextWait
	}
}

func (r *Runner) reloginInterval() time.Duration {
	seconds := r.Policy().GetBasic().GetReconnectIntervalSeconds()
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
		return defaultReloginWait
	}
	if seconds > maxReloginWait.Seconds() {
		return maxReloginWait
	}
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Second {
		return time.Second
	}
	return d
}

func nextReloginWait(current, base time.Duration) time.Duration {
	if base <= 0 {
		base = defaultReloginWait
	}
	if current < base {
		current = base
	}
	capWait := reconnectMaxWait
	if base > capWait {
		capWait = base
	}
	if current >= capWait || current > capWait/2 {
		return capWait
	}
	return current * 2
}

func (r *Runner) autoReloginPending() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessionInvalidated && r.sessionAutoRelogin
}

func (r *Runner) sessionInvalidatedWithoutAutoRelogin() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessionInvalidated && !r.sessionAutoRelogin
}

func (r *Runner) beginAutoReloginAttempt() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.sessionAutoRelogin {
		return false
	}
	if r.policy == nil || !r.policy.GetBasic().GetDisplacedSessionReloginEnabled() {
		return false
	}
	r.sessionInvalidated = false
	return true
}

// prepareAutoReloginAttempt is the final policy gate immediately before a
// fresh HTTP login. SetPolicy normally cancels the wait as soon as the switch
// is turned off; this recheck also covers a change racing with timer expiry.
func (r *Runner) prepareAutoReloginAttempt() bool {
	if !r.Policy().GetBasic().GetDisplacedSessionReloginEnabled() {
		r.failClosedPendingDisplacedRelogin()
		return false
	}
	if !r.beginAutoReloginAttempt() {
		r.failClosedPendingDisplacedRelogin()
		return false
	}
	return true
}

// completeAutoRelogin atomically accepts a newly connected client only while
// this is still the active displaced-session attempt and its policy switch is
// still enabled. A concurrent SetPolicy(false) therefore wins cleanly instead
// of having its fail-closed state overwritten by a late login success.
func (r *Runner) completeAutoRelogin() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.sessionAutoRelogin ||
		r.sessionInvalidated ||
		r.policy == nil ||
		!r.policy.GetBasic().GetDisplacedSessionReloginEnabled() {
		return false
	}
	r.sessionInvalidated = false
	r.sessionInvalidatedReason = ""
	r.sessionAutoRelogin = false
	return true
}

func (r *Runner) clearDisconnectedClient(client *babigame.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client == client {
		r.client = nil
		r.session = nil
		r.httpc = nil
		r.resetSideLaneFairnessLocked()
	}
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextReconnectWait(d time.Duration) time.Duration {
	d *= 2
	if d > reconnectMaxWait {
		return reconnectMaxWait
	}
	return d
}
