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

// CleanLogBatchBefore bounds each tick and yields the writer between tables.
// A backlog is drained over subsequent ticks instead of monopolizing startup.
func (d *DB) CleanLogBatchBefore(ctx context.Context, cutoff time.Time) (LogCleanupResult, error) {
	var out LogCleanupResult
	for _, item := range []struct {
		table string
		count *int64
	}{
		{"event_log", &out.EventLogs}, {"operation_log", &out.OperationLogs},
	} {
		res, err := d.ExecContext(ctx, `DELETE FROM `+item.table+` WHERE id IN (
SELECT id FROM `+item.table+` WHERE ts < ? ORDER BY ts,id LIMIT ?)`, cutoff.UTC().Format(sqliteTimestampFormat), logCleanupBatchSize)
		if err != nil {
			return out, fmt.Errorf("delete expired %s rows: %w", item.table, err)
		}
		*item.count, err = res.RowsAffected()
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

// CompactRedeemHistory drops only verbose terminal messages after 90 days.
// Keep attempt keys, validation, observations and code fingerprints: deleting
// those would lose deduplication or revive previously invalid/expired codes.
func (d *DB) CompactRedeemHistory(ctx context.Context, now time.Time) error {
	cutoff := now.UTC().Add(-90 * 24 * time.Hour)
	_, err := d.ExecContext(ctx, `UPDATE redeem_attempts SET message='' WHERE id IN (
SELECT id FROM redeem_attempts WHERE updated_at < ? AND message <> ''
AND status IN ('success','already_redeemed','expired','invalid') ORDER BY id LIMIT 1000)`, cutoff)
	return err
}

// CleanLogsBefore drains all expired rows for offline callers/tests. The live
// daemon uses CleanLogBatchBefore with an overall tick budget instead.
func (d *DB) CleanLogsBefore(ctx context.Context, cutoff time.Time) (LogCleanupResult, error) {
	var result LogCleanupResult
	for {
		batch, err := d.CleanLogBatchBefore(ctx, cutoff)
		result.EventLogs += batch.EventLogs
		result.OperationLogs += batch.OperationLogs
		if err != nil {
			return result, err
		}
		if batch.EventLogs < logCleanupBatchSize && batch.OperationLogs < logCleanupBatchSize {
			return result, nil
		}
	}
}
