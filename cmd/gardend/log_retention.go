package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/store"
)

const (
	logRetentionPeriod = 7 * 24 * time.Hour
	logCleanupInterval = 24 * time.Hour
)

func runLogCleanupLoop(ctx context.Context, db *store.DB, log *slog.Logger) {
	cleanup := func(now time.Time) {
		result, err := db.CleanLogsBefore(ctx, now.Add(-logRetentionPeriod))
		if err != nil {
			if ctx.Err() == nil {
				log.Warn("clean expired logs failed", "error", err)
			}
			return
		}
		if result.EventLogs > 0 || result.OperationLogs > 0 {
			log.Info("cleaned expired logs",
				"retention", logRetentionPeriod.String(),
				"event_logs", result.EventLogs,
				"operation_logs", result.OperationLogs,
			)
		}
	}

	cleanup(time.Now().UTC())
	ticker := time.NewTicker(logCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			cleanup(now.UTC())
		}
	}
}
