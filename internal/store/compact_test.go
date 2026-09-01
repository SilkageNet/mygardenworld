package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompactReclaimsFreePagesAndPreservesData(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "garden.db")
	db, err := Open(ctx, dbPath)
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
	payload := strings.Repeat("x", 8*1024)
	old := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC).Format(sqliteTimestampFormat)
	for range 256 {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO event_log(account_id, account_name, ts, kind, payload_json) VALUES (?, ?, ?, ?, ?)",
			account.ID, account.Name, old, "compact-test", payload,
		); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CleanLogsBefore(ctx, time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	var freeBefore int
	if err := db.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&freeBefore); err != nil {
		t.Fatal(err)
	}
	if freeBefore == 0 {
		t.Fatal("fixture produced no free SQLite pages")
	}
	if err := db.Compact(ctx); err != nil {
		t.Fatal(err)
	}
	var freeAfter int
	if err := db.QueryRowContext(ctx, "PRAGMA freelist_count").Scan(&freeAfter); err != nil {
		t.Fatal(err)
	}
	if freeAfter != 0 {
		t.Fatalf("freelist_count=%d after compact, want 0", freeAfter)
	}
	loaded, err := db.GetAccountByID(ctx, account.ID)
	if err != nil || loaded.Name != account.Name {
		t.Fatalf("account after compact=%+v error=%v", loaded, err)
	}
}
