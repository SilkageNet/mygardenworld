package runner

import (
	"context"
	"errors"
	"fmt"

	"github.com/SilkageNet/mygardenworld/internal/policycfg"
)

// PauseAutomation is serialized with starts, including starts not yet visible
// in the registry. A logout queued during login must not leave restart intent
// enabled after the new connection has been stopped.
func (m *Manager) PauseAutomation(ctx context.Context, accountID int64, disconnect bool) error {
	lock := m.accountLock(accountID)
	lock.Lock()
	defer lock.Unlock()
	if disconnect {
		defer func() { _ = m.stop(accountID) }()
	}
	r := m.Get(accountID)
	raw, err := m.db.LoadPolicyJSON(ctx, accountID)
	if err != nil {
		return err
	}
	p, err := policycfg.FromJSON(raw)
	if err != nil {
		return err
	}
	if r != nil {
		p = r.Policy()
	}
	wasEnabled := p.GetAutomationEnabled()
	p.AutomationEnabled = false
	if r != nil {
		r.SetPolicy(p)
		if wasEnabled {
			r.emit(Event{Kind: "policy_changed", Category: "system", Domain: "policy", Action: "set",
				Message: "自动化已停止", PayloadJSON: `{"automation_enabled":false}`})
		}
	}
	raw, err = policycfg.ToJSON(p)
	if err != nil {
		return err
	}
	return m.db.SavePolicyJSON(ctx, accountID, raw)
}

// completeStartup is the last step before publishing a runner or launching
// background recovery. Canceled requests never transfer unfinished starts to
// the background. Any failure closes the newly owned connection and runtime.
func (r *Runner) completeStartup(ctx context.Context, activate bool) (err error) {
	defer func() {
		if err != nil {
			r.Stop()
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	if activate {
		return r.enableAutomation(ctx)
	}
	r.mu.RLock()
	stopped := r.cancel == nil || (r.sessionInvalidated && !r.sessionAutoRelogin)
	r.mu.RUnlock()
	if stopped {
		return errors.New("账号启动已终止，请重新连接")
	}
	return nil
}

// enableAutomation commits explicit start intent before enabling execution.
// Holding mu keeps Stop, policy changes and session-invalidation transitions
// from being overwritten by the old policy snapshot during the database write.
// A successful write is the commit boundary: later request cancellation must
// not leave durable and live intent disagreeing.
func (r *Runner) enableAutomation(ctx context.Context) error {
	r.mu.Lock()
	if r.cancel == nil || (r.sessionInvalidated && !r.sessionAutoRelogin) {
		r.mu.Unlock()
		return errors.New("账号启动已终止，请重新连接")
	}
	p := policycfg.Clone(r.policy)
	wasEnabled := p.GetAutomationEnabled()
	p.AutomationEnabled = true
	raw, err := policycfg.ToJSON(p)
	if err == nil {
		err = r.db.SavePolicyJSON(ctx, r.account.ID, raw)
	}
	if err != nil {
		r.mu.Unlock()
		return fmt.Errorf("启用自动化失败: %w", err)
	}
	r.policy = p
	r.mu.Unlock()
	if !wasEnabled {
		r.emit(Event{Kind: "policy_changed", Category: "system", Domain: "policy", Action: "set",
			Message: "自动化已启动", PayloadJSON: `{"automation_enabled":true}`})
	}
	r.wakeDecision()
	return nil
}
