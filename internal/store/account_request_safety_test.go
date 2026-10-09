package store

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Recreate the pre-v19 columns for migration fixtures, not runtime compatibility.
func restoreV18RecoverySchema(t *testing.T, db *DB) {
	t.Helper()
	if _, err := db.ExecContext(t.Context(), `ALTER TABLE account_request_safety ADD COLUMN fresh_login_attempted INTEGER NOT NULL DEFAULT 0 CHECK(fresh_login_attempted IN (0,1));
UPDATE account_request_safety SET fresh_login_attempted = (fresh_login_attempts > 0);
ALTER TABLE account_request_safety DROP COLUMN fresh_login_attempts;`); err != nil {
		t.Fatal(err)
	}
}

func TestV19RecoveryMigrationPreservesConsumedAttempt(t *testing.T) {
	for _, count := range []int{0, 1} {
		db, _, account, _, _ := notificationFixture(t)
		before := AccountRequestSafety{RestrictionCode: 97778, RestrictionAttempts: 2, RestrictedUntilMS: 2345, FreshLoginAttempts: count, LastFreshLoginMS: 1234}
		if err := db.SaveAccountRestriction(t.Context(), account.ID, before); err != nil {
			t.Fatal(err)
		}
		restoreV18RecoverySchema(t, db)
		if _, err := db.ExecContext(t.Context(), `PRAGMA user_version=18`); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			if err := applyMigrations(t.Context(), db.writer); err != nil {
				t.Fatal(err)
			}
		}
		if got, err := db.LoadAccountRequestSafety(t.Context(), account.ID); err != nil || got != before {
			t.Fatal("migration lost durable protection", got, err)
		}
	}
}

func TestFreshRecoveryReservesConfiguredBudgetAtomically(t *testing.T) {
	for _, code := range []int{5000, 97777, 97778} {
		db, _, account, _, _ := notificationFixture(t)
		ctx := t.Context()
		const now, interval = int64(1000000), int64(10000)
		s := AccountRequestSafety{RestrictionCode: code, RestrictionAttempts: 1, RestrictedUntilMS: now}
		if err := db.SaveAccountRestriction(ctx, account.ID, s); err != nil {
			t.Fatal(err)
		}
		for attempt := range 4 {
			var admitted atomic.Int32
			var wg sync.WaitGroup
			for range 12 {
				wg.Go(func() {
					ok, err := db.ReserveFreshRecovery(ctx, account.ID, now+int64(attempt)*interval, interval, 3)
					if err != nil {
						t.Error(err)
					}
					if ok {
						admitted.Add(1)
					}
				})
			}
			wg.Wait()
			want := int32(1)
			if attempt == 3 {
				want = 0
			}
			if admitted.Load() != want {
				t.Fatalf("code=%d attempt=%d admitted=%d", code, attempt, admitted.Load())
			}
		}
		got, err := db.LoadAccountRequestSafety(ctx, account.ID)
		if err != nil || got.FreshLoginAttempts != 3 {
			t.Fatal(got, err)
		}
		if ok, err := db.ReserveFreshRecovery(ctx, account.ID, now+4*interval, interval, 4); err != nil || !ok {
			t.Fatal("raising limit did not allow next attempt", ok, err)
		}
		if ok, err := db.ReserveFreshRecovery(ctx, account.ID, now+5*interval, interval, 2); err != nil || ok {
			t.Fatal("lowering limit reset budget", ok, err)
		}
	}
}

func TestManualRecoveryRecordsAttemptsWithoutAutomaticAdmissionLimits(t *testing.T) {
	for _, code := range []int{5000, 97777, 97778} {
		db, _, account, _, _ := notificationFixture(t)
		now := time.Now().UnixMilli()
		s := AccountRequestSafety{RestrictionCode: code, RestrictionAttempts: 3, RestrictedUntilMS: now + 3600000, FreshLoginAttempts: 3, LastFreshLoginMS: now, LastRaceDeleteMS: 1234}
		if ok, err := db.ReserveRaceDelete(t.Context(), account.ID, s.LastRaceDeleteMS, 1); err != nil || !ok {
			t.Fatal(ok, err)
		}
		if err := db.SaveAccountRestriction(t.Context(), account.ID, s); err != nil {
			t.Fatal(err)
		}
		var admitted atomic.Int32
		var wg sync.WaitGroup
		for range 12 {
			wg.Go(func() {
				ok, err := db.ReserveManualRecovery(t.Context(), account.ID, now)
				if err != nil {
					t.Error(err)
				}
				if ok {
					admitted.Add(1)
				}
			})
		}
		wg.Wait()
		if admitted.Load() != 12 {
			t.Fatalf("manual admission count=%d", admitted.Load())
		}
		stored, err := db.LoadAccountRequestSafety(t.Context(), account.ID)
		s.FreshLoginAttempts += 12
		if err != nil || stored != s {
			t.Fatal("reservation cleared server protection", stored, err)
		}
	}
}

func TestRequestSafetyV14MigrationAndDurableFreshReservation(t *testing.T) {
	db, _, account, _, _ := notificationFixture(t)
	removeAccountDeletionSchema(t, db.writer)
	ctx := t.Context()
	if _, err := db.ExecContext(ctx, `ALTER TABLE account_request_safety DROP COLUMN fresh_login_attempts;
ALTER TABLE account_request_safety DROP COLUMN last_fresh_login_ms;
INSERT INTO account_request_safety VALUES (?,1234,5678,5000,2); PRAGMA user_version=13;`, account.ID); err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(ctx, db.writer); err != nil {
		t.Fatal(err)
	}
	s, err := db.LoadAccountRequestSafety(ctx, account.ID)
	if err != nil || s.LastRaceDeleteMS != 1234 || s.RestrictionCode != 5000 || s.FreshLoginAttempts != 0 || s.LastFreshLoginMS != 0 {
		t.Fatal(s, err)
	}
	nowMS := time.Now().UnixMilli()
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			ok, err := db.ReserveFreshRecovery(ctx, account.ID, nowMS, time.Hour.Milliseconds(), 3)
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
		t.Fatalf("%d simultaneous attempts admitted", allowed.Load())
	}
	s, err = db.LoadAccountRequestSafety(ctx, account.ID)
	if err != nil || s.FreshLoginAttempts != 1 || s.LastFreshLoginMS != nowMS || s.LastRaceDeleteMS != 1234 {
		t.Fatal(s, err)
	}
	s.FreshLoginAttempts = 0
	if err := db.SaveAccountRestriction(ctx, account.ID, s); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.ReserveFreshRecovery(ctx, account.ID, nowMS+1, time.Hour.Milliseconds(), 3); ok || err != nil {
		t.Fatal(ok, err)
	}
}

func TestRequestSafetyV13MigrationPreservesReservationsAndAccepts5000(t *testing.T) {
	db, _, account, _, _ := notificationFixture(t)
	removeAccountDeletionSchema(t, db.writer)
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
	if err := deleteAccountFixture(ctx, db, account.ID); err != nil {
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
	removeAccountDeletionSchema(t, db.writer)
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
	if err := deleteAccountFixture(ctx, db, account.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err = db.LoadAccountRequestSafety(ctx, account.ID)
	if err != nil || loaded != (AccountRequestSafety{}) {
		t.Fatalf("orphan safety row: %+v %v", loaded, err)
	}
}
