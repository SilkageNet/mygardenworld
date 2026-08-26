package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
)

// Stop terminates the runner. Idempotent.
func (r *Runner) Stop() {
	r.stopOnce.Do(func() {
		if r.stats != nil {
			r.stats.MarkStopped(time.Now())
		}
		r.mu.Lock()
		cancel := r.cancel
		r.cancel = nil
		client := r.client
		r.client = nil
		r.resetSideLaneFairnessLocked()
		debugWriter := r.debugWriter
		r.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if client != nil {
			_ = client.Close()
		}
		if debugWriter != nil {
			debugWriter.Close()
		}
		close(r.done)
		r.emit(Event{Kind: "ws_disconnected", Message: "已断开连接"})
	})
}

func (r *Runner) markSessionInvalidated(reason string) {
	r.handleSessionInvalidated(reason, babigame.IsSessionDisplacementReason(reason))
}

func (r *Runner) handleSessionInvalidated(reason string, sessionDisplaced bool) {
	if reason == "" {
		reason = "会话已过期，请重新登录"
	}
	autoRelogin := sessionDisplaced && r.Policy().GetBasic().GetDisplacedSessionReloginEnabled()
	ctx := context.Background()
	if r.db != nil {
		if err := r.db.DeleteSession(ctx, r.account.ID); err != nil {
			r.log.Warn("delete invalidated session failed", "err", err)
		}
	}
	r.mu.Lock()
	if r.sessionInvalidated {
		r.mu.Unlock()
		return
	}
	r.sessionInvalidated = true
	r.sessionInvalidatedReason = reason
	r.sessionAutoRelogin = autoRelogin
	client := r.client
	r.resetSideLaneFairnessLocked()
	r.mu.Unlock()
	if autoRelogin {
		wait := r.reloginInterval()
		r.emit(Event{
			Kind:    "session_expired",
			Message: fmt.Sprintf("检测到账号在其他设备登录，%s 后自动登录：%s", wait, reason),
			Level:   "warn",
		})
		if client != nil {
			_ = client.Close()
		}
		return
	}

	r.disableAutomationPreferenceForInvalidatedSession(ctx, reason)
	r.emit(Event{Kind: "session_expired", Message: fmt.Sprintf("检测到会话失效，已停止自动化：%s", reason)})
	r.Stop()
}

// failClosedPendingDisplacedRelogin cancels an already scheduled displaced-
// session login after the user disables the recovery switch. Marking the
// pending flag false before updating the policy prevents SetPolicy from
// recursively entering this path while automation_enabled is persisted off.
func (r *Runner) failClosedPendingDisplacedRelogin() bool {
	r.mu.Lock()
	if !r.sessionAutoRelogin {
		r.mu.Unlock()
		return false
	}
	reason := r.sessionInvalidatedReason
	r.sessionInvalidated = true
	r.sessionAutoRelogin = false
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if reason == "" {
		reason = "账号在其他设备登录"
	}

	ctx := context.Background()
	r.disableAutomationPreferenceForInvalidatedSession(ctx, reason)
	r.emit(Event{
		Kind:    "session_expired",
		Message: fmt.Sprintf("被挤号自动重登已关闭，已停止自动化：%s", reason),
		Level:   "warn",
	})
	r.Stop()
	return true
}

func (r *Runner) sessionInvalidatedError(message string) error {
	r.mu.RLock()
	reason := r.sessionInvalidatedReason
	r.mu.RUnlock()
	if reason == "" {
		return errors.New(message)
	}
	return fmt.Errorf("%s: %s", message, reason)
}

// discardSessionInvalidation clears kick/expiry markers before an intentional
// Manager.Stop so forgetWhenDone does not re-cache them as 异常.
func (r *Runner) discardSessionInvalidation() {
	r.mu.Lock()
	r.sessionInvalidated = false
	r.sessionInvalidatedReason = ""
	r.sessionAutoRelogin = false
	r.mu.Unlock()
}

func (r *Runner) disableAutomationPreferenceForInvalidatedSession(ctx context.Context, reason string) {
	p := r.Policy()
	if !p.GetAutomationEnabled() {
		return
	}
	p.AutomationEnabled = false
	r.SetPolicy(p)
	if r.db != nil {
		raw, err := policycfg.ToJSON(p)
		if err != nil {
			r.log.Warn("marshal invalidated-session policy failed", "err", err, "reason", reason)
		} else if err := r.db.SavePolicyJSON(ctx, r.account.ID, raw); err != nil {
			r.log.Warn("persist invalidated-session policy failed", "err", err, "reason", reason)
		}
	}
	r.emit(policyDisabledBySessionInvalidatedEvent(reason))
}

func policyDisabledBySessionInvalidatedEvent(reason string) Event {
	payload, _ := json.Marshal(map[string]any{
		"automation_enabled": false,
		"reason":             reason,
	})
	return Event{
		Kind:        "policy_changed",
		Category:    "account",
		Domain:      "account.session",
		Action:      "blocked",
		Label:       "连接",
		Level:       "warn",
		Message:     "会话失效，已关闭自动恢复",
		PayloadJSON: string(payload),
	}
}

// Done returns a channel that closes once the runner has fully stopped.
func (r *Runner) Done() <-chan struct{} { return r.done }
