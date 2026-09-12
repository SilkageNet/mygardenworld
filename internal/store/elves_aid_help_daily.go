package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// ElvesAidHelpedUIDs returns the persisted helpFrd targets for one calendar day.
func (d *DB) ElvesAidHelpedUIDs(ctx context.Context, accountID int64, dayID int32) ([]int64, error) {
	if accountID <= 0 || dayID <= 0 {
		return nil, fmt.Errorf("ElvesAidHelpedUIDs: account_id and day_id required")
	}
	var raw string
	err := d.QueryRowContext(ctx,
		`SELECT helped_uids_json FROM account_elves_aid_help_daily WHERE account_id = ? AND day_id = ?`,
		accountID, dayID,
	).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ElvesAidHelpedUIDs: %w", err)
	}
	uids, err := decodeElvesAidHelpedUIDs(raw)
	if err != nil {
		return nil, fmt.Errorf("ElvesAidHelpedUIDs: %w", err)
	}
	return uids, nil
}

// SetElvesAidHelpedUIDs replaces the persisted helpFrd targets for one day.
func (d *DB) SetElvesAidHelpedUIDs(ctx context.Context, accountID int64, dayID int32, uids []int64) error {
	if accountID <= 0 || dayID <= 0 {
		return fmt.Errorf("SetElvesAidHelpedUIDs: account_id and day_id required")
	}
	normalized := normalizeElvesAidHelpedUIDs(uids)
	raw, err := json.Marshal(normalized)
	if err != nil {
		return fmt.Errorf("SetElvesAidHelpedUIDs: %w", err)
	}
	_, err = d.ExecContext(ctx, `
		INSERT INTO account_elves_aid_help_daily(account_id, day_id, helped_uids_json, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(account_id, day_id) DO UPDATE SET
			helped_uids_json = excluded.helped_uids_json,
			updated_at = CURRENT_TIMESTAMP`,
		accountID, dayID, string(raw),
	)
	if err != nil {
		return fmt.Errorf("SetElvesAidHelpedUIDs: %w", err)
	}
	return nil
}

// AddElvesAidHelpedUID records dstUid for the calendar day and returns the
// full unique list. Duplicate UIDs are ignored.
func (d *DB) AddElvesAidHelpedUID(ctx context.Context, accountID int64, dayID int32, uid int64) ([]int64, error) {
	if accountID <= 0 || dayID <= 0 {
		return nil, fmt.Errorf("AddElvesAidHelpedUID: account_id and day_id required")
	}
	if uid <= 0 {
		return nil, fmt.Errorf("AddElvesAidHelpedUID: uid required")
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("AddElvesAidHelpedUID begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var raw string
	err = tx.QueryRowContext(ctx,
		`SELECT helped_uids_json FROM account_elves_aid_help_daily WHERE account_id = ? AND day_id = ?`,
		accountID, dayID,
	).Scan(&raw)
	var uids []int64
	switch {
	case err == sql.ErrNoRows:
		uids = nil
	case err != nil:
		return nil, fmt.Errorf("AddElvesAidHelpedUID select: %w", err)
	default:
		uids, err = decodeElvesAidHelpedUIDs(raw)
		if err != nil {
			return nil, fmt.Errorf("AddElvesAidHelpedUID decode: %w", err)
		}
	}
	uids = normalizeElvesAidHelpedUIDs(append(uids, uid))
	blob, err := json.Marshal(uids)
	if err != nil {
		return nil, fmt.Errorf("AddElvesAidHelpedUID encode: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO account_elves_aid_help_daily(account_id, day_id, helped_uids_json, updated_at)
		VALUES(?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(account_id, day_id) DO UPDATE SET
			helped_uids_json = excluded.helped_uids_json,
			updated_at = CURRENT_TIMESTAMP`,
		accountID, dayID, string(blob),
	)
	if err != nil {
		return nil, fmt.Errorf("AddElvesAidHelpedUID upsert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("AddElvesAidHelpedUID commit: %w", err)
	}
	return uids, nil
}

// ListElvesAidHelpTargetsSince recovers successful helpFrd dstUid values from
// operation_log since dayStart (inclusive, UTC comparison on stored ts).
func (d *DB) ListElvesAidHelpTargetsSince(ctx context.Context, accountID int64, dayStart time.Time) ([]int64, error) {
	if accountID <= 0 || dayStart.IsZero() {
		return nil, fmt.Errorf("ListElvesAidHelpTargetsSince: account_id and dayStart required")
	}
	rows, err := d.QueryContext(ctx, `
		SELECT args_json FROM operation_log
		WHERE account_id = ? AND ts >= ? AND kind = 'flowerElvesAid.helpFrd'
		ORDER BY id ASC`,
		accountID, dayStart.UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("ListElvesAidHelpTargetsSince: %w", err)
	}
	defer rows.Close()

	var out []int64
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("ListElvesAidHelpTargetsSince scan: %w", err)
		}
		if uid, ok := dstUIDFromArgsJSON(raw); ok {
			out = append(out, uid)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListElvesAidHelpTargetsSince rows: %w", err)
	}
	return normalizeElvesAidHelpedUIDs(out), nil
}

func dstUIDFromArgsJSON(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	var args struct {
		DstUid int64 `json:"dstUid"`
	}
	if json.Unmarshal([]byte(raw), &args) != nil || args.DstUid <= 0 {
		return 0, false
	}
	return args.DstUid, true
}

func decodeElvesAidHelpedUIDs(raw string) ([]int64, error) {
	if raw == "" || raw == "null" {
		return nil, nil
	}
	var uids []int64
	if err := json.Unmarshal([]byte(raw), &uids); err != nil {
		return nil, err
	}
	return normalizeElvesAidHelpedUIDs(uids), nil
}

func normalizeElvesAidHelpedUIDs(uids []int64) []int64 {
	if len(uids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(uids))
	out := make([]int64, 0, len(uids))
	for _, uid := range uids {
		if uid <= 0 {
			continue
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		out = append(out, uid)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
