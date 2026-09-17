package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func spaceFixture(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "garden.db")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// Separate fixture rows isolate physical reclamation from log policy.
	if _, err := db.ExecContext(context.Background(), `CREATE TABLE reclaim_fixture(id INTEGER PRIMARY KEY, payload BLOB);
WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<512)
INSERT INTO reclaim_fixture SELECT x,zeroblob(8192) FROM n`); err != nil {
		t.Fatal(err)
	}
	return db, path
}

func TestAutomaticReclaimShrinksFileWithoutManualCompact(t *testing.T) {
	ctx := context.Background()
	db, path := spaceFixture(t)
	if _, err := db.ReclaimDatabaseSpace(ctx); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM reclaim_fixture WHERE id>1`); err != nil {
		t.Fatal(err)
	}
	space, err := db.DatabaseSpace(ctx)
	if err != nil || space.AutoVacuum != 2 || space.FreePages == 0 {
		t.Fatalf("space=%+v err=%v", space, err)
	}
	for range 5 {
		if _, err := db.ReclaimDatabaseSpace(ctx); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.Stat(path)
	if err != nil || after.Size() >= before.Size() {
		t.Fatalf("before=%v after=%v err=%v", before, after, err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reclaim_fixture`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	if err := db.writer.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&count); err != nil || count != 5000 {
		t.Fatalf("writer busy_timeout=%d err=%v", count, err)
	}
}

func TestAutomaticReclaimDefersForReaderAndRecovers(t *testing.T) {
	ctx := context.Background()
	db, _ := spaceFixture(t)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM reclaim_fixture`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM reclaim_fixture`); err != nil {
		t.Fatal(err)
	}
	bounded, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if _, err := db.ReclaimDatabaseSpace(bounded); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ReclaimDatabaseSpace(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestExistingDatabaseConvertsAutomaticallyAndPreservesCredentials(t *testing.T) {
	ctx := context.Background()
	db, path := spaceFixture(t)
	u, err := db.CreateUser(ctx, "owner", "o@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(ctx, u.ID, "garden", "ios", "game", "private-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSession(ctx, a.ID, []byte(`{"session":"preserved"}`), nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	old, err := sql.Open("sqlite", databaseFileURL(path))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := old.ExecContext(ctx, `PRAGMA auto_vacuum=NONE; VACUUM`); err != nil {
		t.Fatal(err)
	}
	if err := old.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	space, err := db.DatabaseSpace(ctx)
	if err != nil || space.AutoVacuum != 0 {
		t.Fatalf("space=%+v err=%v", space, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := db.EnableAutomaticReclaim(canceled); err == nil {
		t.Fatal("canceled conversion succeeded")
	}
	if err := db.EnableAutomaticReclaim(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.EnableAutomaticReclaim(ctx); err != nil {
		t.Fatal(err)
	}
	space, err = db.DatabaseSpace(ctx)
	if err != nil || space.AutoVacuum != 2 {
		t.Fatalf("space=%+v err=%v", space, err)
	}
	loaded, err := db.GetAccountByID(ctx, a.ID)
	if err != nil || loaded.Username != "game" {
		t.Fatalf("account=%+v err=%v", loaded, err)
	}
	var integrity string
	session, err := db.LoadSession(ctx, a.ID)
	if err != nil || string(session) != `{"session":"preserved"}` {
		t.Fatal("session changed during conversion")
	}
	username, password, err := db.GetCredentials(ctx, a.ID)
	if err != nil || username != "game" || password != "private-password" {
		t.Fatal("credentials changed during conversion")
	}
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity=%s err=%v", integrity, err)
	}
}
