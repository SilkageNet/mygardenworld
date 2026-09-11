package store

import (
	"context"
	"database/sql"
	"errors"
)

// ConnectionStats exposes pool pressure without querying SQLite or recording
// SQL/parameters. WaitDuration measures completed connection waits, not query
// execution time or the duration of an external process's SQLite lock.
func (d *DB) ConnectionStats() (reader, writer sql.DBStats) {
	return d.reader.Stats(), d.writer.Stats()
}

// Close releases both pools. Callers must first stop workers using the store.
func (d *DB) Close() error {
	return errors.Join(d.reader.Close(), d.writer.Close())
}

// ExecContext serializes statements that do not return rows on the writer.
func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.writer.ExecContext(ctx, query, args...)
}

// QueryContext runs a read on the bounded read-only pool. Close rows before
// calling another store method; nested pool acquisition can exhaust the pool.
func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.reader.QueryContext(ctx, query, args...)
}

// QueryRowContext is read-only. Mutations with RETURNING use writeRowContext
// explicitly; routing never guesses from SQL prefixes or result shape.
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.reader.QueryRowContext(ctx, query, args...)
}

func (d *DB) writeRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.writer.QueryRowContext(ctx, query, args...)
}

func (d *DB) writeQueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.writer.QueryContext(ctx, query, args...)
}

// BeginTx reserves the writer unless explicitly read-only. All statements in
// a transaction must use tx, never re-enter the store or call external services.
// database/sql releases the reservation on commit, rollback or cancellation.
func (d *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	if opts != nil && opts.ReadOnly {
		return d.reader.BeginTx(ctx, opts)
	}
	return d.writer.BeginTx(ctx, opts)
}
