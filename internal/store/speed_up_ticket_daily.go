package store

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var speedUpLandIDsRe = regexp.MustCompile(`田地=\[([^\]]*)\]`)

// SpeedUpTicketUsed returns the persisted speed-up ticket spend for one
// calendar day. Missing rows are treated as zero.
func (d *DB) SpeedUpTicketUsed(ctx context.Context, accountID int64, dayID int32) (int32, error) {
	if accountID <= 0 || dayID <= 0 {
		return 0, fmt.Errorf("SpeedUpTicketUsed: account_id and day_id required")
	}
	var used int32
	err := d.QueryRowContext(ctx,
		`SELECT used_count FROM account_speed_up_ticket_daily WHERE account_id = ? AND day_id = ?`,
		accountID, dayID,
	).Scan(&used)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("SpeedUpTicketUsed: %w", err)
	}
	if used < 0 {
		return 0, nil
	}
	return used, nil
}

// SetSpeedUpTicketUsed replaces the persisted spend count for one calendar day.
func (d *DB) SetSpeedUpTicketUsed(ctx context.Context, accountID int64, dayID, used int32) error {
	if accountID <= 0 || dayID <= 0 {
		return fmt.Errorf("SetSpeedUpTicketUsed: account_id and day_id required")
	}
	if used < 0 {
		return fmt.Errorf("SetSpeedUpTicketUsed: used_count cannot be negative")
	}
	_, err := d.ExecContext(ctx, `
		INSERT INTO account_speed_up_ticket_daily(account_id, day_id, used_count, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(account_id, day_id) DO UPDATE SET
			used_count = excluded.used_count,
			updated_at = CURRENT_TIMESTAMP`,
		accountID, dayID, used,
	)
	if err != nil {
		return fmt.Errorf("SetSpeedUpTicketUsed: %w", err)
	}
	return nil
}

// AddSpeedUpTicketUsed adds delta tickets for the calendar day and returns the
// new total. delta <= 0 is a no-op that returns the current total.
func (d *DB) AddSpeedUpTicketUsed(ctx context.Context, accountID int64, dayID, delta int32) (int32, error) {
	if accountID <= 0 || dayID <= 0 {
		return 0, fmt.Errorf("AddSpeedUpTicketUsed: account_id and day_id required")
	}
	if delta < 0 {
		return 0, fmt.Errorf("AddSpeedUpTicketUsed: delta cannot be negative")
	}
	if delta == 0 {
		return d.SpeedUpTicketUsed(ctx, accountID, dayID)
	}
	var used int32
	err := d.QueryRowContext(ctx, `
		INSERT INTO account_speed_up_ticket_daily(account_id, day_id, used_count, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(account_id, day_id) DO UPDATE SET
			used_count = used_count + excluded.used_count,
			updated_at = CURRENT_TIMESTAMP
		RETURNING used_count`,
		accountID, dayID, delta,
	).Scan(&used)
	if err != nil {
		return 0, fmt.Errorf("AddSpeedUpTicketUsed: %w", err)
	}
	return used, nil
}

// CountSpeedUpTicketSpendsSince recovers observed speed-up ticket spends from
// event_log since dayStart by summing land IDs on successful speedUpBatch acks.
func (d *DB) CountSpeedUpTicketSpendsSince(ctx context.Context, accountID int64, dayStart time.Time) (int32, error) {
	if accountID <= 0 || dayStart.IsZero() {
		return 0, fmt.Errorf("CountSpeedUpTicketSpendsSince: account_id and dayStart required")
	}
	since := dayStart.UTC()
	rows, err := d.QueryContext(ctx, `
		SELECT message FROM event_log
		WHERE account_id = ? AND ts >= ? AND kind = 'operation_ack'
		  AND (
		    action = 'speed_up'
		    OR message LIKE '%usrLand.speedUpBatch 完成%'
		  )`,
		accountID, since,
	)
	if err != nil {
		return 0, fmt.Errorf("CountSpeedUpTicketSpendsSince: %w", err)
	}
	defer rows.Close()

	var total int32
	for rows.Next() {
		var message string
		if err := rows.Scan(&message); err != nil {
			return 0, fmt.Errorf("CountSpeedUpTicketSpendsSince scan: %w", err)
		}
		total += countSpeedUpTicketsInMessage(message)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("CountSpeedUpTicketSpendsSince rows: %w", err)
	}
	if total < 0 {
		return 0, nil
	}
	return total, nil
}

func countSpeedUpTicketsInMessage(message string) int32 {
	m := speedUpLandIDsRe.FindStringSubmatch(message)
	if len(m) != 2 {
		return 1
	}
	var n int32
	for _, part := range strings.Fields(m[1]) {
		part = strings.Trim(part, ",")
		if part == "" {
			continue
		}
		n++
	}
	if n <= 0 {
		return 1
	}
	return n
}
