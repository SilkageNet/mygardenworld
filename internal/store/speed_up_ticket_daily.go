package store

import (
	"context"
	"database/sql"
	"fmt"
)

func (d *DB) SpeedUpTicketsReserved(ctx context.Context, accountID int64, dayID int32) (int32, error) {
	if accountID <= 0 || dayID <= 0 {
		return 0, fmt.Errorf("invalid speed-up ticket account or day")
	}
	var storedDay, count int32
	err := d.QueryRowContext(ctx, `SELECT day_id, reserved_count FROM account_speed_up_ticket_daily WHERE account_id = ?`, accountID).Scan(&storedDay, &count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load speed-up ticket reservations: %w", err)
	}
	if storedDay != dayID {
		return 0, nil
	}
	return count, nil
}

// ReserveSpeedUpTickets is the send-time concurrency guard. An uncertain RPC
// outcome keeps its reservation, favoring a lower spend over exceeding a cap.
func (d *DB) ReserveSpeedUpTickets(ctx context.Context, accountID int64, dayID, count, limit int32) (int32, bool, error) {
	if accountID <= 0 || dayID <= 0 || count <= 0 || limit <= 0 {
		return 0, false, fmt.Errorf("invalid speed-up ticket reservation")
	}
	var total int32
	err := d.writeRowContext(ctx, `
		INSERT INTO account_speed_up_ticket_daily(account_id, day_id, reserved_count, updated_at)
		SELECT ?, ?, ?, CURRENT_TIMESTAMP WHERE ? <= ?
		ON CONFLICT(account_id) DO UPDATE SET
			day_id = excluded.day_id,
			reserved_count = CASE
				WHEN account_speed_up_ticket_daily.day_id = excluded.day_id
				THEN account_speed_up_ticket_daily.reserved_count + excluded.reserved_count
				ELSE excluded.reserved_count END,
			updated_at = CURRENT_TIMESTAMP
		WHERE CASE
			WHEN account_speed_up_ticket_daily.day_id = excluded.day_id
			THEN account_speed_up_ticket_daily.reserved_count + excluded.reserved_count
			ELSE excluded.reserved_count END <= ?
		RETURNING reserved_count`, accountID, dayID, count, count, limit, limit).Scan(&total)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("reserve speed-up tickets: %w", err)
	}
	return total, true, nil
}
