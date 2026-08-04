package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestCleanLogsBeforeDeletesOnlyExpiredLogRows(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	for _, indexName := range []string{"idx_event_log_ts", "idx_oplog_ts"} {
		var count int
		if err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?",
			indexName,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("timestamp index %q is missing", indexName)
		}
	}

	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.CreateAccount(ctx, user.ID, "main", "ios", "game", "password")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	old := now.Add(-8 * 24 * time.Hour)
	boundary := now.Add(-7 * 24 * time.Hour)
	recent := now.Add(-6 * 24 * time.Hour)
	for _, ts := range []time.Time{old, boundary, recent} {
		if _, err := db.LogEvent(ctx, EventLog{AccountID: account.ID, AccountName: account.Name, TS: ts, Kind: "test"}); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO operation_log(account_id, ts, kind, args_json, result_json) VALUES (?, ?, ?, '{}', '{}')`,
			account.ID, ts.UTC().Format(sqliteTimestampFormat), "test",
		); err != nil {
			t.Fatal(err)
		}
	}

	result, err := db.CleanLogsBefore(ctx, now.Add(-7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if result.EventLogs != 1 || result.OperationLogs != 1 {
		t.Fatalf("cleanup result=%+v, want one row from each table", result)
	}

	var eventCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_log").Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 2 {
		t.Fatalf("event_log rows=%d, want boundary and recent rows", eventCount)
	}
	var operationCount int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM operation_log").Scan(&operationCount); err != nil {
		t.Fatal(err)
	}
	if operationCount != 2 {
		t.Fatalf("operation_log rows=%d, want boundary and recent rows", operationCount)
	}
}

func TestCleanLogsBeforeDeletesMultipleBatches(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.CreateAccount(ctx, user.ID, "main", "ios", "game", "password")
	if err != nil {
		t.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	old := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC).Format(sqliteTimestampFormat)
	want := logCleanupBatchSize + 3
	for range want {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO event_log(account_id, account_name, ts, kind) VALUES (?, ?, ?, ?)",
			account.ID, account.Name, old, "test",
		); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO operation_log(account_id, ts, kind) VALUES (?, ?, ?)",
			account.ID, old, "test",
		); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	result, err := db.CleanLogsBefore(ctx, time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if result.EventLogs != int64(want) || result.OperationLogs != int64(want) {
		t.Fatalf("cleanup result=%+v, want %d rows from each table", result, want)
	}
}
