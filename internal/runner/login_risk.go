package runner

import (
	"context"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

// stopForLoginRefusal terminates both automatic recovery paths on an explicit
// login refusal. Unlike a displacement notification, this always revokes a
// pending relogin, even if another displacement arrived during HTTP login.
// It does not invent a server expiry or disable other accounts on the host.
func (r *Runner) stopForLoginRefusal(err error) bool {
	if !babigame.IsLoginRefusalError(err) {
		return false
	}
	reason := err.Error()
	r.mu.Lock()
	r.sessionInvalidated = true
	r.sessionInvalidatedReason = reason
	r.sessionAutoRelogin = false
	r.resetSideLaneFairnessLocked()
	r.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	r.disableAutomationPreferenceForInvalidatedSession(ctx, reason)
	// Reuse the connection incident so the existing notification pipeline
	// reports the stop and resolves it after a successful manual connection.
	r.emit(Event{Kind: "connection_unavailable", Category: "account", Domain: "account.connection", Action: "blocked",
		Label: "登录保护", Level: "warn", Message: reason + "；已停止自动化和自动重登，请处理后手动启动"})
	r.Stop()
	return true
}
