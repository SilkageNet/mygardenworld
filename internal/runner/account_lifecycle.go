package runner

import (
	"context"
	"fmt"
	"time"
)

// A lifecycle command may wait behind a network login. Cancellation must
// remove that waiter rather than execute a stale command when login finishes.
type accountLifecycleLock struct{ token chan struct{} }

func (l *accountLifecycleLock) LockContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case l.token <- struct{}{}:
		if err := ctx.Err(); err != nil {
			l.Unlock()
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *accountLifecycleLock) Lock()   { _ = l.LockContext(context.Background()) }
func (l *accountLifecycleLock) Unlock() { <-l.token }

// DeleteAccount owns the same lifecycle boundary as start, reconnect and
// pause. A pending start must finish before deletion; a subsequent start must
// read the missing account, never reuse a runner removed from persistence.
// Persistence remains atomic: on failure the account remains, disconnected.
func (m *Manager) DeleteAccount(ctx context.Context, accountID int64) (err error) {
	started := time.Now()
	phase := "wait_lifecycle"
	defer func() {
		if m.log != nil {
			if err != nil {
				m.log.Warn("account deletion failed", "account_id", accountID, "phase", phase, "elapsed", time.Since(started), "error", err)
			} else {
				m.log.Info("account deleted", "account_id", accountID, "elapsed", time.Since(started))
			}
		}
	}()
	lock := m.accountLock(accountID)
	if err := lock.LockContext(ctx); err != nil {
		return fmt.Errorf("等待账号登录或其他生命周期操作结束时取消，未删除账号: %w", err)
	}
	defer lock.Unlock()
	phase = "stop_runner"
	if m.Get(accountID) != nil {
		if err := m.stop(accountID); err != nil {
			return err
		}
	}
	phase = "delete_persistence"
	if err := m.db.DeleteAccount(ctx, accountID); err != nil {
		return fmt.Errorf("清理账号及相关记录未确认，请刷新列表核对: %w", err)
	}
	m.mu.Lock()
	delete(m.lastStats, accountID)
	delete(m.lastDiag, accountID)
	delete(m.pacers, accountID)
	// Retain the lifecycle lock: existing waiters may still hold its address.
	m.mu.Unlock()
	return nil
}
