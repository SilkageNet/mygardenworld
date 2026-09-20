package runner

import (
	"context"
	"time"
)

// RunAccountDeletions is the daemon-owned, restartable account cleanup worker.
// It is independent of log retention (including retention=0). A small batch
// releases the single SQLite writer between attempts; slow disks shrink batches.
func (m *Manager) RunAccountDeletions(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	type retry struct {
		after time.Time
		limit int
	}
	retries := make(map[int64]retry)
	var cursor int64
	for ctx.Err() == nil {
		listCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		ids, err := m.db.PendingAccountDeletions(listCtx, cursor)
		cancel()
		switch {
		case err != nil:
			if m.log != nil && ctx.Err() == nil {
				m.log.Warn("scan pending account deletions", "error", err)
			}
		case len(ids) == 0:
			cursor = 0
		default:
			for _, id := range ids {
				cursor = id
				r := retries[id]
				if time.Now().Before(r.after) {
					continue
				}
				if r.limit == 0 {
					r.limit = 250
				}
				started := time.Now()
				batchCtx, cancelBatch := context.WithTimeout(ctx, 2*time.Second)
				done, removed, phase, err := m.cleanAccountDeletion(batchCtx, id, r.limit)
				cancelBatch()
				if ctx.Err() != nil {
					return
				}
				statusCtx, cancelStatus := context.WithTimeout(ctx, time.Second)
				statusErr := m.db.SetAccountDeletionFailed(statusCtx, id, err != nil)
				cancelStatus()
				if statusErr != nil && m.log != nil {
					m.log.Warn("persist account deletion status", "account_id", id, "error", statusErr)
				}
				switch {
				case err != nil:
					r.limit = max(1, r.limit/2)
					r.after = time.Now().Add(10 * time.Second)
					retries[id] = r
					if m.log != nil {
						m.log.Warn("account deletion deferred", "account_id", id, "phase", phase, "elapsed", time.Since(started), "next_batch_size", r.limit, "retry_in", "10s", "error", err)
					}
				case done:
					delete(retries, id)
					if m.log != nil {
						m.log.Info("account deleted", "account_id", id)
					}
				case m.log != nil:
					m.log.Debug("account deletion batch", "account_id", id, "removed_rows", removed)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-m.deletionWake:
			cursor = 0
		case <-ticker.C:
		}
	}
}

func (m *Manager) cleanAccountDeletion(ctx context.Context, id int64, limit int) (bool, int64, string, error) {
	lock := m.accountLock(id)
	lock.game.block()
	if err := lock.LockContext(ctx); err != nil {
		return false, 0, "wait_lifecycle", err
	}
	defer lock.Unlock()
	if m.Get(id) != nil {
		if err := m.stop(id); err != nil {
			return false, 0, "stop_runner", err
		}
	}
	// Stop closes sockets; tracked RPC/start/connection lifetimes must also
	// finish before any database rows are physically removed.
	if err := lock.game.wait(ctx); err != nil {
		return false, 0, "drain_game_work", err
	}
	done, removed, err := m.db.CleanAccountDeletionBatch(ctx, id, limit)
	if done && err == nil {
		m.mu.Lock()
		delete(m.lastStats, id)
		delete(m.lastDiag, id)
		delete(m.pacers, id)
		// Keep the blocked lifecycle gate: existing waiters may reference it.
		m.mu.Unlock()
	}
	return done, removed, "delete_persistence", err
}
