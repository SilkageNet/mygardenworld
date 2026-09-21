package runner

import (
	"context"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/store"
)

type deletionSchedule struct {
	limit, fastBatches, failures int
	after, reported              time.Time
}

func (s *deletionSchedule) observe(b store.DeletionBatch, err error, now time.Time) {
	if s.limit == 0 {
		s.limit = 250
	}
	if err != nil {
		s.failures++
		s.fastBatches = 0
		// Only an execution deadline suggests the row batch is too large.
		// Pool/lifecycle waits, external locks and I/O failures do not.
		if store.DeletionErrorKind(err) == "timeout" && deletionExecuting(b.Phase) {
			s.limit = max(1, s.limit/2)
		}
		delay := 10 * time.Second
		switch store.DeletionErrorKind(err) {
		case "disk_full", "io", "permission", "corrupt", "database":
			delay = time.Minute
		}
		s.after = now.Add(delay)
		return
	}
	s.failures = 0
	s.after = time.Time{}
	switch {
	case b.WorkMS > 1000:
		s.limit = max(1, s.limit/2)
		s.fastBatches = 0
	case b.Removed > 0 && b.WorkMS < 500:
		s.fastBatches++
		if s.fastBatches >= 3 {
			s.limit = min(250, s.limit*2)
			s.fastBatches = 0
		}
	default:
		s.fastBatches = 0
	}
}

func deletionExecuting(phase string) bool {
	switch phase {
	case "event_log", "operation_log", "redeem_attempts", "notification_outbox", "notification_incidents", "commit":
		return true
	default:
		return false
	}
}

// RunAccountDeletions retains fair, bounded batches and separates lifecycle,
// writer admission and execution budgets. Counts commit with row removal;
// process-local attempts also remain visible when the writer cannot save errors.
func (m *Manager) RunAccountDeletions(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	retries := make(map[int64]*deletionSchedule)
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
				s := retries[id]
				if s == nil {
					loadCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
					p, loadErr := m.db.AccountDeletionProgress(loadCtx, id)
					cancel()
					if loadErr != nil {
						if m.log != nil {
							m.log.Warn("load deletion progress", "account_id", id, "error", loadErr)
						}
						continue
					}
					s = &deletionSchedule{limit: p.BatchSize, failures: p.Failures}
					if s.limit < 1 || s.limit > 250 {
						s.limit = 250
					}
					if p.RetryAtMS > 0 {
						s.after = time.UnixMilli(p.RetryAtMS)
					}
					retries[id] = s
				}
				if time.Now().Before(s.after) {
					continue
				}
				b, err := m.cleanAccountDeletion(ctx, id, s.limit)
				if ctx.Err() != nil {
					return
				}
				s.observe(b, err, time.Now())
				a := b.DeletionAttempt
				a.ErrorKind, a.Failures, a.BatchSize = store.DeletionErrorKind(err), s.failures, s.limit
				if !s.after.IsZero() {
					a.RetryAtMS = s.after.UnixMilli()
				}
				m.mu.Lock()
				if m.deletionAttempts == nil {
					m.deletionAttempts = make(map[int64]store.DeletionAttempt)
				}
				m.deletionAttempts[id] = a
				if b.Done && err == nil {
					delete(m.deletionAttempts, id)
				}
				m.mu.Unlock()
				switch {
				case err != nil:
					statusCtx, cancel := context.WithTimeout(ctx, time.Second)
					statusErr := m.db.RecordDeletionFailure(statusCtx, id, a)
					cancel()
					if m.log != nil {
						m.log.Warn("account deletion deferred", "account_id", id, "phase", b.Phase, "wait_ms", b.WaitMS, "work_ms", b.WorkMS, "next_batch_size", s.limit, "failures", s.failures, "retry_at", s.after, "error", err)
						if statusErr != nil {
							m.log.Warn("persist account deletion status", "account_id", id, "error", statusErr)
						}
					}
				case b.Done:
					delete(retries, id)
					if m.log != nil {
						m.log.Info("account deleted", "account_id", id)
					}
				case m.log != nil && time.Since(s.reported) >= time.Minute:
					s.reported = time.Now()
					m.log.Info("account deletion progress", "account_id", id, "phase", b.Phase, "batch_removed_rows", b.Removed, "committed_removed_rows", b.TotalRemoved, "wait_ms", b.WaitMS, "work_ms", b.WorkMS, "next_batch_size", s.limit)
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

func (m *Manager) LatestDeletionAttempt(id int64) (store.DeletionAttempt, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	a, ok := m.deletionAttempts[id]
	return a, ok
}

func (m *Manager) cleanAccountDeletion(ctx context.Context, id int64, limit int) (store.DeletionBatch, error) {
	b := store.DeletionBatch{DeletionAttempt: store.DeletionAttempt{Phase: "wait_lifecycle", AttemptMS: time.Now().UnixMilli(), BatchSize: limit}}
	lock := m.accountLock(id)
	lock.game.block()
	lifecycleCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	started := time.Now()
	if err := lock.LockContext(lifecycleCtx); err != nil {
		b.WaitMS = time.Since(started).Milliseconds()
		return b, err
	}
	defer lock.Unlock()
	if m.Get(id) != nil {
		b.Phase = "stop_runner"
		if err := m.stop(id); err != nil {
			return b, err
		}
	}
	b.Phase = "drain_game_work"
	if err := lock.game.wait(lifecycleCtx); err != nil {
		b.WaitMS = time.Since(started).Milliseconds()
		return b, err
	}
	lifecycleWait := time.Since(started).Milliseconds()
	b, err := m.db.CleanAccountDeletionBatch(ctx, id, limit)
	b.WaitMS += lifecycleWait
	if b.Done && err == nil {
		m.mu.Lock()
		delete(m.lastStats, id)
		delete(m.lastDiag, id)
		delete(m.pacers, id)
		m.mu.Unlock()
	}
	return b, err
}
