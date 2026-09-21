package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"modernc.org/sqlite"
)

const accountDeletionProgressMigration = `
CREATE TABLE account_deletion_progress (
 account_id INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
 tracking_started_ms INTEGER NOT NULL,
 removed_rows INTEGER NOT NULL DEFAULT 0 CHECK(removed_rows>=0),
 last_progress_ms INTEGER NOT NULL DEFAULT 0,
 phase TEXT NOT NULL DEFAULT 'queued',
 attempt_ms INTEGER NOT NULL DEFAULT 0,
 error_kind TEXT NOT NULL DEFAULT '',
 retry_at_ms INTEGER NOT NULL DEFAULT 0,
 failures INTEGER NOT NULL DEFAULT 0 CHECK(failures>=0),
 batch_size INTEGER NOT NULL DEFAULT 250 CHECK(batch_size BETWEEN 1 AND 250),
 wait_ms INTEGER NOT NULL DEFAULT 0,
 work_ms INTEGER NOT NULL DEFAULT 0
);
INSERT INTO account_deletion_progress(account_id, tracking_started_ms)
SELECT id, CAST(unixepoch('subsec')*1000 AS INTEGER) FROM accounts WHERE deletion_pending=1;
`

// DeletionAttempt is diagnostic state, not evidence of committed row removal.
// ErrorKind is an allowlisted code; raw SQLite errors never enter Web status.
type DeletionAttempt struct {
	Phase                string
	AttemptMS, RetryAtMS int64
	ErrorKind            string
	Failures, BatchSize  int
	WaitMS, WorkMS       int64
}

type AccountDeletionProgress struct {
	TrackingStartedMS, RemovedRows, LastProgressMS int64
	DeletionAttempt
}

func (d *DB) AccountDeletionProgress(ctx context.Context, id int64) (AccountDeletionProgress, error) {
	var p AccountDeletionProgress
	err := d.QueryRowContext(ctx, `SELECT tracking_started_ms,removed_rows,last_progress_ms,
phase,attempt_ms,error_kind,retry_at_ms,failures,batch_size,wait_ms,work_ms
FROM account_deletion_progress WHERE account_id=?`, id).Scan(&p.TrackingStartedMS, &p.RemovedRows, &p.LastProgressMS,
		&p.Phase, &p.AttemptMS, &p.ErrorKind, &p.RetryAtMS, &p.Failures, &p.BatchSize, &p.WaitMS, &p.WorkMS)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	return p, err
}

// RecordDeletionFailure does not touch committed progress. Both the account
// list badge and detailed failure are updated in one bounded transaction.
func (d *DB) RecordDeletionFailure(ctx context.Context, id int64, a DeletionAttempt) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE account_deletion_progress SET phase=?,attempt_ms=?,error_kind=?,retry_at_ms=?,
failures=?,batch_size=?,wait_ms=?,work_ms=? WHERE account_id=? AND attempt_ms<=?`,
		a.Phase, a.AttemptMS, a.ErrorKind, a.RetryAtMS, a.Failures, a.BatchSize, a.WaitMS, a.WorkMS, id, a.AttemptMS)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET deletion_failed=1 WHERE id=? AND deletion_pending=1 AND deletion_failed=0`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// DeletionErrorKind classifies only typed driver/context errors. Message text
// can contain SQL, filesystem paths or values, and is for daemon logs only.
func DeletionErrorKind(err error) string {
	if err == nil {
		return ""
	}
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() & 0xff {
		case 5, 6:
			return "busy"
		case 13:
			return "disk_full"
		case 10:
			return "io"
		case 8, 3, 23:
			return "permission"
		case 11, 26:
			return "corrupt"
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	return "database"
}

// DeletionBatch reports actual connection wait separately from execution and
// commit. Removed is populated only after a successful transaction commit.
type DeletionBatch struct {
	Done         bool
	Removed      int64
	TotalRemoved int64
	DeletionAttempt
}

const deletionWriterWait = 2 * time.Second
const deletionWorkBudget = 5 * time.Second
