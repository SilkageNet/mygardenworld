package store

import (
	"context"
	"database/sql"
	"fmt"
)

// PearlHireTicketUsed returns the persisted ticket spend for dayID. The
// single account row is treated as zero when it belongs to another day.
func (d *DB) PearlHireTicketUsed(ctx context.Context, accountID int64, dayID int32) (int32, error) {
	if accountID <= 0 || dayID <= 0 {
		return 0, fmt.Errorf("PearlHireTicketUsed: account_id and day_id required")
	}
	var (
		storedDay int32
		used      int32
	)
	err := d.QueryRowContext(ctx,
		`SELECT day_id, used_count FROM account_pearl_hire_usage WHERE account_id = ?`,
		accountID,
	).Scan(&storedDay, &used)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("PearlHireTicketUsed: %w", err)
	}
	if storedDay != dayID {
		return 0, nil
	}
	if used < 0 {
		return 0, fmt.Errorf("PearlHireTicketUsed: negative used_count %d", used)
	}
	return used, nil
}

// AdvancePearlHireTicketUsed atomically records one spend for dayID and repairs
// the persisted value up to minimumUsed. The high-water mark matters when an
// earlier write failed after the in-memory counter had already advanced.
// Crossing the calendar-day boundary replaces yesterday's count instead of
// retaining an unbounded row history.
func (d *DB) AdvancePearlHireTicketUsed(ctx context.Context, accountID int64, dayID, minimumUsed int32) (int32, error) {
	if accountID <= 0 || dayID <= 0 || minimumUsed <= 0 {
		return 0, fmt.Errorf("AdvancePearlHireTicketUsed: account_id, day_id, and minimum_used required")
	}
	var used int32
	err := d.QueryRowContext(ctx, `
		INSERT INTO account_pearl_hire_usage(account_id, day_id, used_count, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(account_id) DO UPDATE SET
			day_id = excluded.day_id,
			used_count = CASE
				WHEN account_pearl_hire_usage.day_id = excluded.day_id
				THEN MAX(account_pearl_hire_usage.used_count + 1, excluded.used_count)
				ELSE excluded.used_count
			END,
			updated_at = CURRENT_TIMESTAMP
		RETURNING used_count`,
		accountID, dayID, minimumUsed,
	).Scan(&used)
	if err != nil {
		return 0, fmt.Errorf("AdvancePearlHireTicketUsed: %w", err)
	}
	return used, nil
}
