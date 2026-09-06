package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCreateAccountInitialPolicyIsAtomic(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	user, err := db.CreateUser(ctx, "owner", "owner@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	const policy = `{"schema_version":3,"automation_enabled":false}`
	acc, err := db.CreateAccountWithPolicy(ctx, user.ID, "with-policy", "ios", "game", "pw", policy)
	if err != nil {
		t.Fatal(err)
	}
	got, err := db.LoadPolicyJSON(ctx, acc.ID)
	if err != nil || got != policy {
		t.Fatalf("policy=%q, err=%v", got, err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER reject_initial_policy BEFORE INSERT ON account_policies BEGIN SELECT RAISE(ABORT, 'test failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateAccountWithPolicy(ctx, user.ID, "must-rollback", "ios", "other-game", "pw", policy); err == nil {
		t.Fatal("expected policy write failure")
	}
	count, err := db.CountAccountsByUser(ctx, user.ID)
	if err != nil || count != 1 {
		t.Fatalf("partial account persisted: count=%d err=%v", count, err)
	}
}
