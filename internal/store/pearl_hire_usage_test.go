package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestPearlHireTicketUsageIsAtomicAndBoundedToOneAccountRow(t *testing.T) {
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
	account, err := db.CreateAccount(ctx, user.ID, "main", "ios", "game", "secret")
	if err != nil {
		t.Fatal(err)
	}

	const firstDay int32 = 20260829
	if used, err := db.PearlHireTicketUsed(ctx, account.ID, firstDay); err != nil || used != 0 {
		t.Fatalf("missing usage=(%d,%v), want 0,nil", used, err)
	}
	for want := int32(1); want <= 3; want++ {
		used, err := db.AdvancePearlHireTicketUsed(ctx, account.ID, firstDay, want)
		if err != nil || used != want {
			t.Fatalf("increment=%d err=%v, want %d", used, err, want)
		}
	}

	const nextDay int32 = 20260830
	used, err := db.AdvancePearlHireTicketUsed(ctx, account.ID, nextDay, 1)
	if err != nil || used != 1 {
		t.Fatalf("next-day increment=%d err=%v, want 1", used, err)
	}
	if old, err := db.PearlHireTicketUsed(ctx, account.ID, firstDay); err != nil || old != 0 {
		t.Fatalf("old-day usage=(%d,%v), want 0,nil", old, err)
	}
	var rows int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_pearl_hire_usage WHERE account_id = ?`, account.ID).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("usage rows=%d, want one bounded row", rows)
	}

	if err := db.DeleteAccount(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_pearl_hire_usage`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 0 {
		t.Fatalf("usage rows after account delete=%d, want 0", rows)
	}
}

func TestPearlHireTicketUsageRejectsInvalidKeys(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.PearlHireTicketUsed(ctx, 0, 20260829); err == nil {
		t.Fatal("read accepted zero account id")
	}
	if _, err := db.AdvancePearlHireTicketUsed(ctx, 1, 0, 1); err == nil {
		t.Fatal("increment accepted zero day id")
	}
	if _, err := db.AdvancePearlHireTicketUsed(ctx, 1, 20260829, 0); err == nil {
		t.Fatal("increment accepted zero minimum usage")
	}
}
