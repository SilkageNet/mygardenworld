package store

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRequestSafetyV13MigrationPreservesReservationsAndAccepts5000(t *testing.T) {
	db, _, account, _, _ := notificationFixture(t)
	ctx := t.Context()
	// Recreate the actual v12 safety constraint, retaining all other tables.
	if _, err := db.ExecContext(ctx, `DROP TABLE account_request_safety;`+migrations[8].sql+`
INSERT INTO account_request_safety VALUES (?,1234,5678,97778,2);
DROP INDEX idx_redeem_attempts_account;
DROP INDEX idx_notification_incidents_account;
DROP INDEX idx_notification_outbox_account;
PRAGMA user_version=12;`, account.ID); err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(ctx, db.writer); err != nil {
		t.Fatal(err)
	}
	s, err := db.LoadAccountRequestSafety(ctx, account.ID)
	if err != nil || s.LastRaceDeleteMS != 1234 || s.RestrictedUntilMS != 5678 || s.RestrictionCode != 97778 || s.RestrictionAttempts != 2 {
		t.Fatal("v12 protection changed", s, err)
	}
	s.RestrictionCode = 5000
	if err := db.SaveAccountRestriction(ctx, account.ID, s); err != nil {
		t.Fatal(err)
	}
	loaded, err := db.LoadAccountRequestSafety(ctx, account.ID)
	if err != nil || loaded != s {
		t.Fatal("5000 protection did not persist", loaded, err)
	}
	for _, table := range []string{"redeem_attempts", "notification_incidents", "notification_outbox"} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT count(*) FROM pragma_index_list(?) l JOIN pragma_index_info(l.name) i WHERE i.seqno=0 AND i.name='account_id'`, table).Scan(&count); err != nil || count == 0 {
			t.Fatalf("%s missing account cascade index: %v", table, err)
		}
	}
	if err := db.DeleteAccount(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err = db.LoadAccountRequestSafety(ctx, account.ID)
	if err != nil || loaded != (AccountRequestSafety{}) {
		t.Fatal("migrated foreign key did not cascade", loaded, err)
	}
}

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
	if _, err := db.ExecContext(ctx, `DROP TABLE daemon_maintenance; DROP TABLE notification_outbox; DROP TABLE notification_incidents; DROP TABLE user_notifications; DROP TABLE account_request_safety; PRAGMA user_version=8`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if version, err := databaseVersion(ctx, db.writer); err != nil || version != currentSchemaVersion {
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
