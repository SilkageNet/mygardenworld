package main

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/store"
)

// Diagnostics go to the operator log, never back into the contended database.
// Pool wait totals distinguish connection pressure from game/network latency.
func runDatabaseDiagnosticsLoop(ctx context.Context, db *store.DB, log *slog.Logger) {
	reader, writer := db.ConnectionStats()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			nextReader, nextWriter := db.ConnectionStats()
			logDatabaseWaits(log, "reader", reader, nextReader)
			logDatabaseWaits(log, "writer", writer, nextWriter)
			reader, writer = nextReader, nextWriter
		}
	}
}

func logDatabaseWaits(log *slog.Logger, pool string, before, after sql.DBStats) {
	waited := after.WaitDuration - before.WaitDuration
	// This is aggregate wait time across callers, not one slow request. Keep
	// ordinary short serialization waits from producing a warning each minute.
	if waited < 10*time.Second {
		return
	}
	log.Warn("sqlite connection contention", "pool", pool,
		"wait_count_delta", after.WaitCount-before.WaitCount,
		"completed_wait_duration", waited,
		"open_connections", after.OpenConnections, "in_use", after.InUse)
}
