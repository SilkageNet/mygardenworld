package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RedeemCodeEntry is a row from the redeem_codes table.
type RedeemCodeEntry struct {
	ID         int64
	Code       string
	SourceTime int64
	FetchedAt  time.Time
}

// RedeemHistoryEntry is a row from the redeem_history table.
type RedeemHistoryEntry struct {
	ID           int64
	AccountID    int64
	AccountName  string
	Code         string
	Status       string
	Message      string
	AttemptCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SaveRedeemCodes replaces all redeem codes with the latest set from the
// external API and records the sync timestamp.
func (d *DB) SaveRedeemCodes(ctx context.Context, entries []RedeemCodeEntry) error {
	now := time.Now().UTC()
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM redeem_codes`); err != nil {
		return fmt.Errorf("delete old codes: %w", err)
	}
	for _, e := range entries {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO redeem_codes(code, source_time, fetched_at) VALUES (?, ?, ?)`,
			e.Code, e.SourceTime, now,
		); err != nil {
			return fmt.Errorf("save redeem code %q: %w", e.Code, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE auto_redeem_config SET last_sync_at = ? WHERE id = 1`, now,
	); err != nil {
		return fmt.Errorf("update last_sync_at: %w", err)
	}
	return tx.Commit()
}

// ListRedeemCodes returns redeem codes ordered by source time descending.
func (d *DB) ListRedeemCodes(ctx context.Context, limit, offset int) ([]RedeemCodeEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := d.QueryContext(ctx,
		`SELECT id, code, source_time, fetched_at FROM redeem_codes ORDER BY source_time DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []RedeemCodeEntry
	for rows.Next() {
		var e RedeemCodeEntry
		if err := rows.Scan(&e.ID, &e.Code, &e.SourceTime, &e.FetchedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetRedeemHistoryStatus returns the status of a redeem attempt, or
// (0, "", false) when no record exists.
func (d *DB) GetRedeemHistoryStatus(ctx context.Context, accountID int64, code string) (attemptCount int, status string, found bool, err error) {
	err = d.QueryRowContext(ctx,
		`SELECT attempt_count, status FROM redeem_history WHERE account_id = ? AND code = ?`,
		accountID, code,
	).Scan(&attemptCount, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, err
	}
	return attemptCount, status, true, nil
}

// UpsertRedeemHistory inserts or updates the redeem result for an account+code.
func (d *DB) UpsertRedeemHistory(ctx context.Context, accountID int64, code, status, message string) error {
	now := time.Now().UTC()
	_, err := d.ExecContext(ctx,
		`INSERT INTO redeem_history(account_id, code, status, message, attempt_count, created_at, updated_at)
         VALUES (?, ?, ?, ?, 1, ?, ?)
         ON CONFLICT(account_id, code) DO UPDATE SET
             status = CASE
                 WHEN excluded.status = 'redeemed' THEN 'redeemed'
                 WHEN excluded.status = 'expired' THEN 'expired'
                 WHEN excluded.status = 'already_claimed' THEN 'already_claimed'
                 ELSE redeem_history.status
             END,
             message = excluded.message,
             attempt_count = redeem_history.attempt_count + 1,
             updated_at = excluded.updated_at`,
		accountID, code, status, message, now, now,
	)
	return err
}

// ListRedeemHistory returns history records, optionally filtered.
func (d *DB) ListRedeemHistory(ctx context.Context, accountID int64, limit, offset int) ([]RedeemHistoryEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if accountID > 0 {
		rows, err = d.QueryContext(ctx,
			`SELECT h.id, h.account_id, COALESCE(a.name, ''), h.code, h.status, h.message, h.attempt_count, h.created_at, h.updated_at
			 FROM redeem_history h LEFT JOIN accounts a ON a.id = h.account_id
			 WHERE h.account_id = ? ORDER BY h.updated_at DESC LIMIT ? OFFSET ?`,
			accountID, limit, offset,
		)
	} else {
		rows, err = d.QueryContext(ctx,
			`SELECT h.id, h.account_id, COALESCE(a.name, ''), h.code, h.status, h.message, h.attempt_count, h.created_at, h.updated_at
			 FROM redeem_history h LEFT JOIN accounts a ON a.id = h.account_id
			 ORDER BY h.updated_at DESC LIMIT ? OFFSET ?`,
			limit, offset,
		)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []RedeemHistoryEntry
	for rows.Next() {
		var e RedeemHistoryEntry
		if err := rows.Scan(&e.ID, &e.AccountID, &e.AccountName, &e.Code, &e.Status, &e.Message, &e.AttemptCount, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetLastSyncAt returns the last time codes were synced from the external API.
func (d *DB) GetLastSyncAt(ctx context.Context) (time.Time, error) {
	var t sql.NullTime
	err := d.QueryRowContext(ctx, `SELECT last_sync_at FROM auto_redeem_config WHERE id = 1`).Scan(&t)
	if err != nil {
		return time.Time{}, err
	}
	if !t.Valid {
		return time.Time{}, nil
	}
	return t.Time, nil
}

// GetAutoRedeemEnabled returns whether auto-redeem is enabled.
func (d *DB) GetAutoRedeemEnabled(ctx context.Context) (bool, error) {
	var enabled int
	err := d.QueryRowContext(ctx, `SELECT enabled FROM auto_redeem_config WHERE id = 1`).Scan(&enabled)
	if err != nil {
		return false, err
	}
	return enabled != 0, nil
}

// SetAutoRedeemEnabled toggles the auto-redeem switch.
func (d *DB) SetAutoRedeemEnabled(ctx context.Context, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	_, err := d.ExecContext(ctx,
		`UPDATE auto_redeem_config SET enabled = ?, updated_at = ? WHERE id = 1`,
		v, time.Now().UTC(),
	)
	return err
}
