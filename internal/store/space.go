package store

import (
	"context"
	"database/sql"
	"fmt"
)

// DatabaseSpace distinguishes live pages from reusable free space; file size
// alone cannot show whether retention is working. No user data is read.
type DatabaseSpace struct {
	PageSize, Pages, FreePages int64
	AutoVacuum                 int
}

func (d *DB) DatabaseSpace(ctx context.Context) (DatabaseSpace, error) {
	var s DatabaseSpace
	err := d.QueryRowContext(ctx, `SELECT page_size, page_count, freelist_count, auto_vacuum
FROM pragma_page_size(), pragma_page_count(), pragma_freelist_count(), pragma_auto_vacuum()`).Scan(&s.PageSize, &s.Pages, &s.FreePages, &s.AutoVacuum)
	return s, err
}

// EnableAutomaticReclaim converts an existing NONE database exactly once.
// Call only during startup before workers/read transactions. VACUUM is atomic:
// cancellation or insufficient disk space leaves the original database intact.
// It needs temporary disk space and is deliberately not retried during live play.
// This changes SQLite's file layout, not tables or PRAGMA user_version.
func (d *DB) EnableAutomaticReclaim(ctx context.Context) error {
	conn, err := d.writer.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	var mode int
	if err := conn.QueryRowContext(ctx, "PRAGMA auto_vacuum").Scan(&mode); err != nil {
		return err
	}
	if mode == 2 {
		return nil
	}
	if _, err := conn.ExecContext(ctx, "PRAGMA auto_vacuum=INCREMENTAL"); err != nil {
		return err
	}
	if mode == 0 {
		if _, err := conn.ExecContext(ctx, "VACUUM"); err != nil {
			return fmt.Errorf("convert automatic SQLite reclaim: %w", err)
		}
	}
	return checkpointSQLiteWAL(ctx, conn)
}

// ReclaimDatabaseSpace is bounded in pages and by the caller's deadline. It
// never runs VACUUM. A busy reader may defer physical WAL truncation, not block
// game writes for the usual busy_timeout. The next maintenance tick retries.
func (d *DB) ReclaimDatabaseSpace(ctx context.Context) (DatabaseSpace, error) {
	s, err := d.DatabaseSpace(ctx)
	if err != nil {
		return s, err
	}
	conn, err := d.writer.Conn(ctx)
	if err != nil {
		return s, err
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "PRAGMA busy_timeout=0"); err != nil {
		return s, err
	}
	defer func() {
		// Restore even after the maintenance context expires, before returning
		// this sole writer to business operations. PRAGMA itself does not lock.
		_, _ = conn.ExecContext(context.Background(), "PRAGMA busy_timeout=5000")
	}()
	if s.AutoVacuum == 2 && s.FreePages*s.PageSize >= 1<<20 {
		// incremental_vacuum produces multiple rows on SQLite; drain the
		// statement to completion rather than assuming one Exec step is enough.
		// Commit small chunks so a slow disk or an expired deadline does not
		// roll back an entire tick's progress and retry it forever.
		for range 16 {
			if err := drainPragma(ctx, conn, "PRAGMA incremental_vacuum(64)"); err != nil {
				return s, err
			}
		}
	}
	var busy, frames, checkpointed int
	err = conn.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &frames, &checkpointed)
	return s, err
}

func drainPragma(ctx context.Context, conn *sql.Conn, query string) error {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
	}
	return rows.Err()
}
