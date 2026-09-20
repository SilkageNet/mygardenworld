package runner

import (
	"context"

	"github.com/SilkageNet/mygardenworld/internal/store"
)

// A lifecycle command may wait behind a network login. Cancellation must
// remove that waiter rather than execute a stale command when login finishes.
type accountLifecycleLock struct {
	token chan struct{}
	game  gameGate
}

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

// DeleteAccount commits intent before canceling game work. It does not wait
// behind a login or execute an unbounded SQLite cascade in the HTTP request.
// Once committed, cleanup survives both client cancellation and daemon restart.
func (m *Manager) DeleteAccount(ctx context.Context, accountID int64) error {
	if err := m.db.RequestAccountDeletion(ctx, accountID); err != nil {
		return err
	}
	m.accountLock(accountID).game.block()
	if m.log != nil {
		m.log.Info("account deletion requested", "account_id", accountID)
	}
	select {
	case m.deletionWake <- struct{}{}:
	default:
	}
	return nil
}

// BeginAccountGameWork tracks account-bound I/O, including reauthorization
// probes outside a runner. API callers must authorize ownership first.
func (m *Manager) BeginAccountGameWork(ctx context.Context, id int64) (context.Context, func(), error) {
	ctx, release, err := m.BeginGameWork(ctx)
	if err != nil {
		return nil, nil, err
	}
	ctx, releaseAccount, err := m.accountLock(id).game.begin(ctx)
	if err != nil {
		release()
		if err == ErrMaintenance {
			err = store.ErrAccountDeleting
		}
		return nil, nil, err
	}
	done := func() { releaseAccount(); release() }
	return ctx, done, nil
}
