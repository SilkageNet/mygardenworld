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
	logCleanupInterval      = time.Minute
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
	} else {
		log.Info("log retention enabled", "retention_days", int64(retention/(24*time.Hour)), "cleanup_interval", logCleanupInterval.String())
	}
	if db == nil {
		return
	}
	maintainDatabase(ctx, db, log, time.Now().UTC(), retention)
	ticker := time.NewTicker(logCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			maintainDatabase(ctx, db, log, now.UTC(), retention)
		}
	}
}

func cleanExpiredLogs(ctx context.Context, db *store.DB, log *slog.Logger, now time.Time, retention time.Duration) {
	cleanupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var result store.LogCleanupResult
	var err error
	// At most 5000 rows per table per tick, with a shared deadline and a
	// separate writer reservation for every 1000 rows. No unbounded catch-up.
	for range 5 {
		var batch store.LogCleanupResult
		batch, err = db.CleanLogBatchBefore(cleanupCtx, now.Add(-retention))
		result.EventLogs += batch.EventLogs
		result.OperationLogs += batch.OperationLogs
		if err != nil || (batch.EventLogs < 1000 && batch.OperationLogs < 1000) {
			break
		}
	}
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

func maintainDatabase(ctx context.Context, db *store.DB, log *slog.Logger, now time.Time, retention time.Duration) {
	if retention > 0 {
		cleanExpiredLogs(ctx, db, log, now, retention)
	}
	// Reclamation continues even with log retention disabled: it never deletes
	// live rows, only pages already freed by account deletion or other cleanup.
	reclaimCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	space, err := db.ReclaimDatabaseSpace(reclaimCtx)
	cancel()
	if err != nil && ctx.Err() == nil {
		log.Warn("automatic sqlite reclaim deferred", "error", err)
	}
	if now.Minute() == 0 {
		log.Info("sqlite space", "allocated_bytes", space.Pages*space.PageSize,
			"free_bytes", space.FreePages*space.PageSize,
			"used_bytes", (space.Pages-space.FreePages)*space.PageSize, "auto_vacuum", space.AutoVacuum)
		historyCtx, cancelHistory := context.WithTimeout(ctx, time.Second)
		defer cancelHistory()
		if err := db.CompactRedeemHistory(historyCtx, now); err != nil && ctx.Err() == nil {
			log.Warn("compact redeem history deferred", "error", err)
		}
	}
}

// Existing databases need a one-time pointer-map rebuild. This happens before
// any game connection or background worker, not as a surprise online VACUUM.
func prepareAutomaticReclaim(ctx context.Context, db *store.DB, log *slog.Logger) {
	space, err := db.DatabaseSpace(ctx)
	if err != nil {
		log.Warn("inspect automatic sqlite reclaim", "error", err)
		return
	}
	if space.AutoVacuum == 2 {
		return
	}
	log.Info("preparing automatic sqlite reclaim before startup", "database_bytes", space.Pages*space.PageSize,
		"note", "one-time conversion may need additional disk space; no game sessions started")
	conversionCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := db.EnableAutomaticReclaim(conversionCtx); err != nil {
		log.Warn("automatic sqlite reclaim conversion deferred until next startup", "error", err,
			"note", "original database retained; retention and free-page reuse continue; check disk space")
		return
	}
	log.Info("automatic sqlite reclaim enabled")
}
