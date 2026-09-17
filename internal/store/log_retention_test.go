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

func TestCleanupTickBoundsWorkAndPreservesRedeemKeys(t *testing.T) {
	db, _ := spaceFixture(t)
	ctx := context.Background()
	u, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(ctx, u.ID, "main", "ios", "game", "password")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<1005)
INSERT INTO event_log(account_id,kind,ts) SELECT ?, 'test', '2026-01-01 00:00:00' FROM n`, a.ID); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	batch, err := db.CleanLogBatchBefore(ctx, cutoff)
	if err != nil || batch.EventLogs != 1000 {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	batch, err = db.CleanLogBatchBefore(ctx, cutoff)
	if err != nil || batch.EventLogs != 5 {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	if _, err := db.RedeemInstanceID(ctx); err != nil {
		t.Fatal(err)
	}
	code, _, err := db.UpsertRedeemCode(ctx, RedeemCodeInput{Code: "PERMANENT", Channel: "ios", SourceKey: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureRedeemAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE redeem_attempts SET status='success',message='old detailed result',updated_at='2026-01-01 00:00:00' WHERE redeem_code_id=?`, code.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.CompactRedeemHistory(ctx, cutoff); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureRedeemAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	var status, message string
	if err := db.QueryRowContext(ctx, `SELECT status,message FROM redeem_attempts WHERE redeem_code_id=? AND account_id=?`, code.ID, a.ID).Scan(&status, &message); err != nil || status != "success" || message != "" {
		t.Fatalf("status=%s message=%s err=%v", status, message, err)
	}
}
