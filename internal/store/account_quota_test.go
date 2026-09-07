package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func accountQuotaFixture(t *testing.T) (*DB, *User) {
	t.Helper()
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	u, err := db.CreateUser(ctx, "owner", "owner@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	return db, u
}

func TestConcurrentAccountCreationHonorsQuotaAtomically(t *testing.T) {
	ctx := context.Background()
	db, u := accountQuotaFixture(t)
	if _, err := db.ExecContext(ctx, `UPDATE users SET max_accounts = 1 WHERE id = ?`, u.ID); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var created, rejected atomic.Int32
	for i := range 8 {
		wg.Go(func() {
			_, err := db.CreateAccountWithPolicy(ctx, u.ID, fmt.Sprintf("account-%d", i), "ios", fmt.Sprintf("game-%d", i), "fixture", `{}`)
			switch {
			case err == nil:
				created.Add(1)
			case errors.Is(err, ErrAccountQuota):
				rejected.Add(1)
			default:
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if created.Load() != 1 || rejected.Load() != 7 {
		t.Fatalf("created=%d rejected=%d", created.Load(), rejected.Load())
	}
	var policies int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM account_policies`).Scan(&policies); err != nil || policies != 1 {
		t.Fatalf("policies=%d err=%v", policies, err)
	}
}

func TestAccountCreationRejectsInactiveOwner(t *testing.T) {
	ctx := context.Background()
	db, u := accountQuotaFixture(t)
	if _, err := db.ExecContext(ctx, `UPDATE users SET status = 'disabled' WHERE id = ?`, u.ID); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{u.ID, 999999} {
		if _, err := db.CreateAccount(ctx, id, "blocked", "ios", "fixture", "fixture"); !errors.Is(err, ErrUserInactive) {
			t.Fatalf("inactive owner %d accepted: %v", id, err)
		}
	}
}
