package store

import (
	"context"
	"fmt"
	"time"
)

const (
	logCleanupBatchSize   = 1000
	sqliteTimestampFormat = "2006-01-02 15:04:05"
)

// LogCleanupResult reports how many persisted diagnostic rows were removed.
type LogCleanupResult struct {
	EventLogs     int64
	OperationLogs int64
}

// CleanLogsBefore deletes event and operation log rows older than cutoff in
// bounded transactions. Account, session, and policy data are never affected.
func (d *DB) CleanLogsBefore(ctx context.Context, cutoff time.Time) (LogCleanupResult, error) {
	result := LogCleanupResult{}
	cutoffValue := cutoff.UTC().Format(sqliteTimestampFormat)
	var err error
	result.EventLogs, err = deleteLogsBefore(ctx, d, "event_log", `
		DELETE FROM event_log
		WHERE id IN (
			SELECT id FROM event_log
			WHERE ts < ?
			ORDER BY ts, id
			LIMIT ?
		)`, cutoffValue)
	if err != nil {
		return result, err
	}
	result.OperationLogs, err = deleteLogsBefore(ctx, d, "operation_log", `
		DELETE FROM operation_log
		WHERE id IN (
			SELECT id FROM operation_log
			WHERE ts < ?
			ORDER BY ts, id
			LIMIT ?
		)`, cutoffValue)
	if err != nil {
		return result, err
	}
	return result, nil
}

func deleteLogsBefore(ctx context.Context, db *DB, table, query, cutoff string) (int64, error) {
	var total int64
	for {
		res, err := db.ExecContext(ctx, query, cutoff, logCleanupBatchSize)
		if err != nil {
			return total, fmt.Errorf("delete expired %s rows: %w", table, err)
		}
		count, err := res.RowsAffected()
		if err != nil {
			return total, fmt.Errorf("count deleted %s rows: %w", table, err)
		}
		total += count
		if count < logCleanupBatchSize {
			return total, nil
		}
	}
}
