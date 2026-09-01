package main

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/store"
)

const (
	defaultLogRetentionDays = 7
	logCleanupInterval      = 24 * time.Hour
)

func logRetentionDuration(days int) (time.Duration, error) {
	if days < 0 {
		return 0, fmt.Errorf("--log-retention-days must be 0 or greater")
	}
	if int64(days) > math.MaxInt64/int64(24*time.Hour) {
		return 0, fmt.Errorf("--log-retention-days is too large: %d", days)
	}
	return time.Duration(days) * 24 * time.Hour, nil
}

func runLogCleanupLoop(ctx context.Context, db *store.DB, log *slog.Logger, retention time.Duration) {
	if retention == 0 {
		log.Info("log retention disabled", "retention_days", 0)
		return
	}
	retentionDays := int64(retention / (24 * time.Hour))
	log.Info("log retention enabled", "retention_days", retentionDays, "cleanup_interval", logCleanupInterval.String())
	cleanExpiredLogs(ctx, db, log, time.Now().UTC(), retention)
	ticker := time.NewTicker(logCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			cleanExpiredLogs(ctx, db, log, now.UTC(), retention)
		}
	}
}

func cleanExpiredLogs(ctx context.Context, db *store.DB, log *slog.Logger, now time.Time, retention time.Duration) {
	result, err := db.CleanLogsBefore(ctx, now.Add(-retention))
	if err != nil {
		if ctx.Err() == nil {
			log.Warn("clean expired logs failed", "error", err)
		}
		return
	}
	if result.EventLogs > 0 || result.OperationLogs > 0 {
		log.Info("cleaned expired logs",
			"retention_days", int64(retention/(24*time.Hour)),
			"event_logs", result.EventLogs,
			"operation_logs", result.OperationLogs,
		)
	}
}
