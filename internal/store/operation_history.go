package store

import (
	"context"
	"strings"
	"time"
)

const (
	defaultOperationHistoryLimit = 50
	// One extra row lets the transport determine has_more for a 200-row page.
	maxOperationHistoryLimit = 201
)

// OperationHistory is the persisted, structured automation audit record.
// Transport packages decide how to present args/result JSON to callers.
type OperationHistory struct {
	ID         int64
	AccountID  int64
	TS         time.Time
	Kind       string
	ArgsJSON   string
	ResultJSON string
}

type ListOperationHistoryOptions struct {
	AccountID int64
	BeforeID  int64
	Limit     int
}

// ListOperationHistory returns newest-first audit rows for one account.
func (d *DB) ListOperationHistory(ctx context.Context, opts ListOperationHistoryOptions) ([]OperationHistory, error) {
	limit := normalizeOperationHistoryLimit(opts.Limit)
	where := []string{"account_id = ?"}
	args := []any{opts.AccountID}
	if opts.BeforeID > 0 {
		where = append(where, "id < ?")
		args = append(args, opts.BeforeID)
	}
	args = append(args, limit)
	rows, err := d.QueryContext(ctx, `
		SELECT id, account_id, ts, kind, args_json, result_json
		FROM operation_log
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY id DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]OperationHistory, 0, limit)
	for rows.Next() {
		var row OperationHistory
		if err := rows.Scan(&row.ID, &row.AccountID, &row.TS, &row.Kind, &row.ArgsJSON, &row.ResultJSON); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func normalizeOperationHistoryLimit(limit int) int {
	if limit <= 0 {
		return defaultOperationHistoryLimit
	}
	if limit > maxOperationHistoryLimit {
		return maxOperationHistoryLimit
	}
	return limit
}
