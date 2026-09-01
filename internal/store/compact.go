package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Compact rewrites the SQLite database and truncates its WAL. Callers must
// stop the daemon first: this maintenance operation needs exclusive database
// access and can temporarily require additional disk space.
func (d *DB) Compact(ctx context.Context) error {
	conn, err := d.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire sqlite connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if err := checkpointSQLiteWAL(ctx, conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, "VACUUM"); err != nil {
		return fmt.Errorf("vacuum sqlite: %w", err)
	}
	if err := checkpointSQLiteWAL(ctx, conn); err != nil {
		return err
	}
	return nil
}

func checkpointSQLiteWAL(ctx context.Context, conn *sql.Conn) error {
	var busy, logFrames, checkpointedFrames int
	if err := conn.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointedFrames); err != nil {
		return fmt.Errorf("checkpoint sqlite WAL: %w", err)
	}
	if busy != 0 {
		return fmt.Errorf("checkpoint sqlite WAL: database is busy; stop gardend before compacting")
	}
	return nil
}
