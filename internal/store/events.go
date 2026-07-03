package store

import (
	"context"
	"strings"
	"time"
)

const (
	defaultEventLogLimit = 500
	maxEventLogLimit     = 2000
)

// EventLog is the persisted shape behind QueryService.StreamEvents.
type EventLog struct {
	ID          int64
	AccountID   int64
	AccountName string
	TS          time.Time
	Kind        string
	Message     string
	PayloadJSON string
	Category    string
	Domain      string
	Action      string
	Label       string
	Level       string
}

// ListEventLogsOptions filters persisted event replay.
type ListEventLogsOptions struct {
	AccountIDs []int64
	Kinds      []string
	AfterID    int64
	Limit      int
}

// LogEvent appends a persisted stream event and returns its monotonic id.
func (d *DB) LogEvent(ctx context.Context, e EventLog) (int64, error) {
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	}
	if e.PayloadJSON == "" {
		e.PayloadJSON = "{}"
	}
	res, err := d.ExecContext(ctx,
		`INSERT INTO event_log(account_id, account_name, ts, kind, message, payload_json, category, domain, action, label, level)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.AccountID, e.AccountName, e.TS.UTC(), e.Kind, e.Message, e.PayloadJSON, e.Category, e.Domain, e.Action, e.Label, e.Level,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// ListEventLogs returns chronological events after applying filters. It queries
// the newest rows first for efficiency, then reverses them for replay.
func (d *DB) ListEventLogs(ctx context.Context, opts ListEventLogsOptions) ([]EventLog, error) {
	limit := normalizeEventLogLimit(opts.Limit)
	where := make([]string, 0, 3)
	args := make([]any, 0, len(opts.AccountIDs)+len(opts.Kinds)+2)

	if opts.AfterID > 0 {
		where = append(where, "id > ?")
		args = append(args, opts.AfterID)
	}
	if opts.AccountIDs != nil {
		if len(opts.AccountIDs) == 0 {
			return nil, nil
		}
		where = append(where, "account_id IN ("+placeholders(len(opts.AccountIDs))+")")
		for _, id := range opts.AccountIDs {
			args = append(args, id)
		}
	}
	if len(opts.Kinds) > 0 {
		where = append(where, "kind IN ("+placeholders(len(opts.Kinds))+")")
		for _, kind := range opts.Kinds {
			args = append(args, kind)
		}
	}

	query := `SELECT id, account_id, account_name, ts, kind, message, payload_json, category, domain, action, label, level FROM event_log`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]EventLog, 0, limit)
	for rows.Next() {
		var e EventLog
		if err := rows.Scan(
			&e.ID,
			&e.AccountID,
			&e.AccountName,
			&e.TS,
			&e.Kind,
			&e.Message,
			&e.PayloadJSON,
			&e.Category,
			&e.Domain,
			&e.Action,
			&e.Label,
			&e.Level,
		); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}

func normalizeEventLogLimit(limit int) int {
	if limit <= 0 {
		return defaultEventLogLimit
	}
	if limit > maxEventLogLimit {
		return maxEventLogLimit
	}
	return limit
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", n), ",")
}
