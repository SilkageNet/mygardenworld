package store

import (
	"context"
	"database/sql"
	"fmt"
)

// AccountRequestSafety survives runner replacement and daemon restart. Absolute
// milliseconds deliberately avoid SQLite's textual DATETIME comparisons.
type AccountRequestSafety struct {
	LastRaceDeleteMS    int64
	RestrictedUntilMS   int64
	RestrictionCode     int
	RestrictionAttempts int
	FreshLoginAttempted bool
	LastFreshLoginMS    int64
}

func (d *DB) LoadAccountRequestSafety(ctx context.Context, accountID int64) (AccountRequestSafety, error) {
	var s AccountRequestSafety
	err := d.QueryRowContext(ctx, `SELECT last_race_delete_ms, restricted_until_ms,
		restriction_code, restriction_attempts, fresh_login_attempted, last_fresh_login_ms FROM account_request_safety WHERE account_id = ?`, accountID).
		Scan(&s.LastRaceDeleteMS, &s.RestrictedUntilMS, &s.RestrictionCode, &s.RestrictionAttempts, &s.FreshLoginAttempted, &s.LastFreshLoginMS)
	if err == sql.ErrNoRows {
		return s, nil
	}
	return s, err
}

// ReserveRaceDelete commits the attempt BEFORE preflight/RPC. A failed or
// ambiguous attempt also consumes its slot; success must not clear it.
func (d *DB) ReserveRaceDelete(ctx context.Context, accountID, nowMS, intervalMS int64) (bool, error) {
	if accountID <= 0 || nowMS <= 0 || intervalMS <= 0 {
		return false, fmt.Errorf("invalid race delete reservation")
	}
	result, err := d.ExecContext(ctx, `INSERT INTO account_request_safety(account_id, last_race_delete_ms)
		VALUES(?, ?) ON CONFLICT(account_id) DO UPDATE SET last_race_delete_ms = excluded.last_race_delete_ms
		WHERE last_race_delete_ms = 0 OR last_race_delete_ms + ? <= excluded.last_race_delete_ms`, accountID, nowMS, intervalMS)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

// SaveAccountRestriction changes only restriction fields, preserving the delete
// reservation. The runner serializes observations and recovery with safetyMu.
func (d *DB) SaveAccountRestriction(ctx context.Context, accountID int64, s AccountRequestSafety) error {
	_, err := d.ExecContext(ctx, `INSERT INTO account_request_safety
		(account_id, restricted_until_ms, restriction_code, restriction_attempts, fresh_login_attempted, last_fresh_login_ms) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(account_id) DO UPDATE SET restricted_until_ms = excluded.restricted_until_ms,
		restriction_code = excluded.restriction_code, restriction_attempts = excluded.restriction_attempts,
		fresh_login_attempted = excluded.fresh_login_attempted, last_fresh_login_ms = excluded.last_fresh_login_ms`,
		accountID, s.RestrictedUntilMS, s.RestrictionCode, s.RestrictionAttempts, s.FreshLoginAttempted, s.LastFreshLoginMS)
	return err
}

// ReserveFreshRecovery commits before authentication, including failed or
// uncertain attempts. A restart must not replenish this incident's allowance.
func (d *DB) ReserveFreshRecovery(ctx context.Context, accountID, nowMS, intervalMS int64) (bool, error) {
	if accountID <= 0 || nowMS <= 0 || intervalMS <= 0 {
		return false, fmt.Errorf("invalid recovery reservation")
	}
	res, err := d.ExecContext(ctx, `UPDATE account_request_safety SET fresh_login_attempted=1, last_fresh_login_ms=?
		WHERE account_id=? AND restriction_code=5000 AND restriction_attempts>=1 AND restricted_until_ms<=?
		AND fresh_login_attempted=0 AND (last_fresh_login_ms=0 OR last_fresh_login_ms+?<=?)`, nowMS, accountID, nowMS, intervalMS, nowMS)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
