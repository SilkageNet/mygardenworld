package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	res, err := d.ExecContext(ctx, `UPDATE accounts SET deletion_pending=1 WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		return ErrAccountNotFound
	}
	return err
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

func (d *DB) SetAccountDeletionFailed(ctx context.Context, id int64, failed bool) error {
	_, err := d.ExecContext(ctx, `UPDATE accounts SET deletion_failed=? WHERE id=? AND deletion_pending=1 AND deletion_failed<>?`, failed, id, failed)
	return err
}

// CleanAccountDeletionBatch is called only after the manager drained game I/O.
// Each transaction removes at most limit rows from ONE history table, releasing
// the writer before the next batch. Remaining rows are the durable cursor.
// Singleton dependents are small enough for the final account cascade.
func (d *DB) CleanAccountDeletionBatch(ctx context.Context, id int64, limit int) (done bool, removed int64, err error) {
	if limit < 1 || limit > 250 {
		return false, 0, errors.New("account deletion batch size must be 1..250")
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var pending bool
	if err := tx.QueryRowContext(ctx, `SELECT deletion_pending FROM accounts WHERE id=?`, id).Scan(&pending); errors.Is(err, sql.ErrNoRows) {
		return true, 0, nil
	} else if err != nil {
		return false, 0, err
	} else if !pending {
		return false, 0, errors.New("account deletion not requested")
	}
	for _, table := range []string{"event_log", "operation_log", "redeem_attempts", "notification_outbox", "notification_incidents"} {
		res, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE rowid IN (SELECT rowid FROM `+table+` WHERE account_id=? LIMIT ?)`, id, limit)
		if err != nil {
			return false, 0, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return false, 0, err
		}
		if n > 0 {
			if err := tx.Commit(); err != nil {
				return false, 0, err
			}
			return false, n, nil
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM accounts WHERE id=?`, id); err != nil {
		return false, 0, err
	}
	return true, 0, tx.Commit()
}
