package store

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestAccountRequestSafetyMigrationPersistenceAndAtomicReservation(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "garden.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.CreateAccount(ctx, user.ID, "main", "ios", "game", "secret")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.CreateAccount(ctx, user.ID, "other", "ios", "game2", "secret2")
	if err != nil {
		t.Fatal(err)
	}
	// Restore a v8 fixture; migration v9 must leave account data untouched.
	if _, err := db.ExecContext(ctx, `DROP TABLE account_request_safety; PRAGMA user_version=8`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if version, err := databaseVersion(ctx, db.DB); err != nil || version != 9 {
		t.Fatalf("v8 migration: %d %v", version, err)
	}
	if u, p, err := db.GetCredentials(ctx, account.ID); err != nil || u != "game" || p != "secret" {
		t.Fatalf("credentials changed: %v", err)
	}
	const nowMS int64 = 1800000000000
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			ok, err := db.ReserveRaceDelete(ctx, account.ID, nowMS, 120000)
			if err != nil {
				t.Error(err)
			}
			if ok {
				allowed.Add(1)
			}
		})
	}
	wg.Wait()
	if allowed.Load() != 1 {
		t.Fatalf("reserved %d slots, want 1", allowed.Load())
	}
	if ok, err := db.ReserveRaceDelete(ctx, other.ID, nowMS, 120000); err != nil || !ok {
		t.Fatalf("other account blocked: %v", err)
	}
	s := AccountRequestSafety{RestrictedUntilMS: nowMS + 300000, RestrictionCode: 97777, RestrictionAttempts: 1}
	if err := db.SaveAccountRestriction(ctx, account.ID, s); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	loaded, err := db.LoadAccountRequestSafety(ctx, account.ID)
	if err != nil || loaded.LastRaceDeleteMS != nowMS || loaded.RestrictedUntilMS != s.RestrictedUntilMS || loaded.RestrictionCode != 97777 {
		t.Fatalf("lost persisted state: %+v %v", loaded, err)
	}
	for _, tt := range []struct {
		offset int64
		want   bool
	}{{119999, false}, {120000, true}} {
		if ok, err := db.ReserveRaceDelete(ctx, account.ID, nowMS+tt.offset, 120000); err != nil || ok != tt.want {
			t.Fatalf("boundary %d: %v %v", tt.offset, ok, err)
		}
	}
	if err := db.SaveAccountRestriction(ctx, account.ID, AccountRequestSafety{}); err != nil {
		t.Fatal(err)
	}
	loaded, err = db.LoadAccountRequestSafety(ctx, account.ID)
	if err != nil || loaded.RestrictionCode != 0 || loaded.LastRaceDeleteMS != nowMS+120000 {
		t.Fatalf("clearing restriction reset delete spacing: %+v %v", loaded, err)
	}
	if err := db.DeleteAccount(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err = db.LoadAccountRequestSafety(ctx, account.ID)
	if err != nil || loaded != (AccountRequestSafety{}) {
		t.Fatalf("orphan safety row: %+v %v", loaded, err)
	}
}
