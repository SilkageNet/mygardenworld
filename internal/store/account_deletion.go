package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var ErrAccountDeleting = errors.New("账号正在删除，暂不能操作或重新添加，请等待后台清理完成")

func migrateAccountDeletion(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE accounts ADD COLUMN deletion_pending INTEGER NOT NULL DEFAULT 0 CHECK(deletion_pending IN (0,1));
ALTER TABLE accounts ADD COLUMN deletion_failed INTEGER NOT NULL DEFAULT 0 CHECK(deletion_failed IN (0,1));
CREATE INDEX idx_accounts_deletion ON accounts(deletion_pending,id);`); err != nil {
		return err
	}
	// A late callback must not replenish rows after the worker passed a table,
	// nor persist fresh credentials/policies for an account being removed.
	for _, table := range []string{"sessions", "account_policies", "account_pearl_hire_usage", "account_request_safety", "redeem_attempts", "operation_log", "event_log", "notification_incidents", "notification_outbox"} {
		action := "RAISE(ABORT, 'account deletion pending')"
		if table == "operation_log" || table == "event_log" || table == "notification_incidents" || table == "notification_outbox" {
			action = "RAISE(IGNORE)"
		}
		for _, verb := range []string{"INSERT", "UPDATE"} {
			condition := ""
			if verb == "UPDATE" && table != "sessions" && table != "account_policies" {
				// Updating an existing delivery/lease does not replenish history.
				// Do not abort multi-account maintenance or delivery acknowledgments.
				condition = " AND NEW.account_id IS NOT OLD.account_id"
			}
			if _, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE TRIGGER deletion_guard_%s_%s BEFORE %s ON %s
WHEN EXISTS(SELECT 1 FROM accounts WHERE id=NEW.account_id AND deletion_pending=1)%s
BEGIN SELECT %s; END;`, table, verb, verb, table, condition, action)); err != nil {
				return err
			}
		}
	}
	return nil
}

// RequestAccountDeletion only records durable intent. It does not wait for game
// I/O or cascade through history. Repeated requests are idempotent.
func (d *DB) RequestAccountDeletion(ctx context.Context, id int64) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE accounts SET deletion_pending=1 WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return ErrAccountNotFound
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO account_deletion_progress(account_id,tracking_started_ms) VALUES(?,?)`, id, time.Now().UnixMilli()); err != nil {
		return err
	}
	return tx.Commit()
}

// PendingAccountDeletions uses a keyset so failed/large accounts cannot starve
// subsequent accounts. The caller wraps to zero after reaching the end.
func (d *DB) PendingAccountDeletions(ctx context.Context, after int64) ([]int64, error) {
	rows, err := d.QueryContext(ctx, `SELECT id FROM accounts WHERE deletion_pending=1 AND id>? ORDER BY id LIMIT 32`, after)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CleanAccountDeletionBatch is called only after the manager drained game I/O.
// Each transaction removes at most limit rows from ONE history table, releasing
// the writer before the next batch. Remaining rows are the durable cursor.
// Singleton dependents are small enough for the final account cascade.
func (d *DB) CleanAccountDeletionBatch(ctx context.Context, id int64, limit int) (result DeletionBatch, err error) {
	result.DeletionAttempt = DeletionAttempt{Phase: "wait_writer", AttemptMS: time.Now().UnixMilli(), BatchSize: limit}
	if limit < 1 || limit > 250 {
		return result, errors.New("account deletion batch size must be 1..250")
	}
	started := time.Now()
	waitCtx, cancelWait := context.WithTimeout(ctx, deletionWriterWait)
	conn, err := d.writer.Conn(waitCtx)
	cancelWait()
	result.WaitMS = time.Since(started).Milliseconds()
	if err != nil {
		return result, err
	}
	defer func() { _ = conn.Close() }()
	// Do not hold the only writer while an external process owns SQLite's
	// write lock. SQLITE_BUSY is a wait failure, never a reason to shrink rows.
	workCtx, cancelWork := context.WithTimeout(ctx, deletionWorkBudget)
	defer cancelWork()
	if _, err := conn.ExecContext(workCtx, "PRAGMA busy_timeout=0"); err != nil {
		return result, err
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), "PRAGMA busy_timeout=5000") }()
	result.Phase = "begin_transaction"
	tx, err := conn.BeginTx(workCtx, nil)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback() }()
	started = time.Now()
	defer func() { result.WorkMS = time.Since(started).Milliseconds() }()
	ctx = workCtx
	var pending bool
	if err := tx.QueryRowContext(ctx, `SELECT deletion_pending FROM accounts WHERE id=?`, id).Scan(&pending); errors.Is(err, sql.ErrNoRows) {
		result.Done = true
		return result, nil
	} else if err != nil {
		return result, err
	} else if !pending {
		return result, errors.New("account deletion not requested")
	}
	for _, table := range []string{"event_log", "operation_log", "redeem_attempts", "notification_outbox", "notification_incidents"} {
		result.Phase = table
		res, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE rowid IN (SELECT rowid FROM `+table+` WHERE account_id=? LIMIT ?)`, id, limit)
		if err != nil {
			return result, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return result, err
		}
		if n > 0 {
			// The counter and last-progress timestamp commit WITH the deletion.
			// No COUNT(*) and no second transaction for each successful batch.
			var total int64
			if err := tx.QueryRowContext(ctx, `UPDATE account_deletion_progress SET removed_rows=removed_rows+?,last_progress_ms=?,
phase=?,attempt_ms=?,error_kind='',retry_at_ms=0,failures=0,batch_size=?,wait_ms=?,work_ms=0 WHERE account_id=? RETURNING removed_rows`,
				n, time.Now().UnixMilli(), table, result.AttemptMS, limit, result.WaitMS, id).Scan(&total); err != nil {
				return result, err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE accounts SET deletion_failed=0 WHERE id=? AND deletion_failed=1`, id); err != nil {
				return result, err
			}
			result.Phase = "commit"
			if err := tx.Commit(); err != nil {
				return result, err
			}
			result.Phase, result.Removed, result.TotalRemoved = table, n, total
			return result, nil
		}
	}
	result.Phase = "finalize"
	if _, err := tx.ExecContext(ctx, `DELETE FROM accounts WHERE id=?`, id); err != nil {
		return result, err
	}
	err = tx.Commit()
	result.Done = err == nil
	return result, err
}
