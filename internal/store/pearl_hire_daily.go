package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PearlHireTicketUsed returns the persisted hire-ticket spend count for one
// calendar day. Missing rows are treated as zero.
func (d *DB) PearlHireTicketUsed(ctx context.Context, accountID int64, dayID int32) (int32, error) {
	if accountID <= 0 || dayID <= 0 {
		return 0, fmt.Errorf("PearlHireTicketUsed: account_id and day_id required")
	}
	var used int32
	err := d.QueryRowContext(ctx,
		`SELECT used_count FROM account_pearl_hire_daily WHERE account_id = ? AND day_id = ?`,
		accountID, dayID,
	).Scan(&used)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("PearlHireTicketUsed: %w", err)
	}
	if used < 0 {
		return 0, nil
	}
	return used, nil
}

// SetPearlHireTicketUsed replaces the persisted spend count for one calendar
// day. used must already reflect continuous consumption since that day's 00:00.
func (d *DB) SetPearlHireTicketUsed(ctx context.Context, accountID int64, dayID, used int32) error {
	if accountID <= 0 || dayID <= 0 {
		return fmt.Errorf("SetPearlHireTicketUsed: account_id and day_id required")
	}
	if used < 0 {
		return fmt.Errorf("SetPearlHireTicketUsed: used_count cannot be negative")
	}
	_, err := d.ExecContext(ctx, `
		INSERT INTO account_pearl_hire_daily(account_id, day_id, used_count, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(account_id, day_id) DO UPDATE SET
			used_count = excluded.used_count,
			updated_at = CURRENT_TIMESTAMP`,
		accountID, dayID, used,
	)
	if err != nil {
		return fmt.Errorf("SetPearlHireTicketUsed: %w", err)
	}
	return nil
}

// IncrementPearlHireTicketUsed adds one ticket spend for the calendar day and
// returns the new total.
func (d *DB) IncrementPearlHireTicketUsed(ctx context.Context, accountID int64, dayID int32) (int32, error) {
	if accountID <= 0 || dayID <= 0 {
		return 0, fmt.Errorf("IncrementPearlHireTicketUsed: account_id and day_id required")
	}
	var used int32
	err := d.QueryRowContext(ctx, `
		INSERT INTO account_pearl_hire_daily(account_id, day_id, used_count, updated_at)
		VALUES(?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(account_id, day_id) DO UPDATE SET
			used_count = used_count + 1,
			updated_at = CURRENT_TIMESTAMP
		RETURNING used_count`,
		accountID, dayID,
	).Scan(&used)
	if err != nil {
		return 0, fmt.Errorf("IncrementPearlHireTicketUsed: %w", err)
	}
	return used, nil
}

// CountPearlHireTicketSpendsSince recovers observed ticket spends from event_log
// since dayStart. Success acks and hireFailCnt contested outcomes consume a
// ticket; pearl_tips4 does not.
func (d *DB) CountPearlHireTicketSpendsSince(ctx context.Context, accountID int64, dayStart time.Time) (int32, error) {
	if accountID <= 0 || dayStart.IsZero() {
		return 0, fmt.Errorf("CountPearlHireTicketSpendsSince: account_id and dayStart required")
	}
	since := dayStart.UTC()
	var success, contested int32
	if err := d.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM event_log
		WHERE account_id = ? AND ts >= ? AND (
			kind = 'pearl_hire'
			OR message LIKE '%pearlPlace.hire 完成%'
			OR (domain = 'basic.pearl.hire' AND kind IN ('operation_ack', 'pearl_hire') AND message LIKE '%雇佣劳工成功%')
		)`,
		accountID, since,
	).Scan(&success); err != nil {
		return 0, fmt.Errorf("CountPearlHireTicketSpendsSince success: %w", err)
	}
	if err := d.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM event_log
		WHERE account_id = ? AND ts >= ? AND message LIKE '%hireFailCnt=%'`,
		accountID, since,
	).Scan(&contested); err != nil {
		return 0, fmt.Errorf("CountPearlHireTicketSpendsSince contested: %w", err)
	}
	total := success + contested
	if total < 0 {
		return 0, nil
	}
	return total, nil
}
