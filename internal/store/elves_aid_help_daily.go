package store

import (
	"context"
	"fmt"
)

func (d *DB) ElvesAidHelpReservations(ctx context.Context, accountID int64, dayID int32) ([]int64, error) {
	if accountID <= 0 || dayID <= 0 {
		return nil, fmt.Errorf("invalid aid account or day")
	}
	rows, err := d.QueryContext(ctx, `SELECT friend_uid FROM account_elves_aid_help_daily WHERE account_id = ? AND day_id = ? ORDER BY friend_uid`, accountID, dayID)
	if err != nil {
		return nil, fmt.Errorf("load aid reservations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var uids []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			return nil, err
		}
		uids = append(uids, uid)
	}
	return uids, rows.Err()
}

// ReserveElvesAidHelp atomically enforces the daily cap before the RPC is sent.
// Uncertain results retain their reservation.
func (d *DB) ReserveElvesAidHelp(ctx context.Context, accountID int64, dayID int32, uid int64, limit int32) (bool, error) {
	if accountID <= 0 || dayID <= 0 || uid <= 0 || limit <= 0 {
		return false, fmt.Errorf("invalid aid reservation")
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var existing int32
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_elves_aid_help_daily WHERE account_id=? AND day_id=? AND friend_uid=?`, accountID, dayID, uid).Scan(&existing); err != nil {
		return false, err
	}
	if existing > 0 {
		return false, nil
	}
	var used int32
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_elves_aid_help_daily WHERE account_id=? AND day_id=?`, accountID, dayID).Scan(&used); err != nil {
		return false, err
	}
	if used >= limit {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO account_elves_aid_help_daily(account_id,day_id,friend_uid) VALUES(?,?,?)`, accountID, dayID, uid); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
