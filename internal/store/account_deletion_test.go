package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// Older migration tests construct historical schemas by removing newer fields.
func removeAccountDeletionSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`DROP TABLE account_deletion_progress`); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT name FROM sqlite_schema WHERE type='trigger' AND name LIKE 'deletion_guard_%'`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	_ = rows.Close()
	for _, name := range names {
		if _, err := db.Exec(`DROP TRIGGER ` + name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`DROP INDEX idx_accounts_deletion; ALTER TABLE accounts DROP COLUMN deletion_pending; ALTER TABLE accounts DROP COLUMN deletion_failed;`); err != nil {
		t.Fatal(err)
	}
}

func cleanDeletionTestBatch(ctx context.Context, db *DB, id int64, limit int) (bool, int64, error) {
	b, err := db.CleanAccountDeletionBatch(ctx, id, limit)
	return b.Done, b.Removed, err
}

func TestAccountDeletionResumesAfterDatabaseReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garden.db")
	db, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	u, err := db.CreateUser(t.Context(), "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(t.Context(), u.ID, "fixture", "ios", "fixture", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<501) INSERT INTO operation_log(account_id,kind) SELECT ?, 'fixture' FROM n`, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.RequestAccountDeletion(t.Context(), a.ID); err != nil {
		t.Fatal(err)
	}
	if done, n, err := cleanDeletionTestBatch(t.Context(), db, a.ID, 250); err != nil || done || n != 250 {
		t.Fatal(done, n, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := db.PendingAccountDeletions(t.Context(), 0)
	if err != nil || len(ids) != 1 || ids[0] != a.ID {
		t.Fatal(ids, err)
	}
	if done, n, err := cleanDeletionTestBatch(t.Context(), db, a.ID, 250); err != nil || done || n != 250 {
		t.Fatal(done, n, err)
	}
	if done, n, err := cleanDeletionTestBatch(t.Context(), db, a.ID, 250); err != nil || done || n != 1 {
		t.Fatal(done, n, err)
	}
	if done, n, err := cleanDeletionTestBatch(t.Context(), db, a.ID, 250); err != nil || !done || n != 0 {
		t.Fatal(done, n, err)
	}
}

func deleteAccountFixture(ctx context.Context, db *DB, id int64) error {
	if err := db.RequestAccountDeletion(ctx, id); err != nil {
		return err
	}
	for {
		done, _, err := cleanDeletionTestBatch(ctx, db, id, 250)
		if err != nil || done {
			return err
		}
	}
}

func TestAccountDeletionMigrationPreservesDataAndIsOneWay(t *testing.T) {
	db, _, account, _, _ := notificationFixture(t)
	removeAccountDeletionSchema(t, db.writer)
	if _, err := db.ExecContext(t.Context(), `PRAGMA user_version=14`); err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(t.Context(), db.writer); err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(t.Context(), db.writer); err != nil {
		t.Fatal(err)
	}
	acc, err := db.GetAccountByID(t.Context(), account.ID)
	if err != nil || acc.DeletionPending || acc.DeletionFailed {
		t.Fatal(acc, err)
	}
	if _, password, err := db.GetCredentials(t.Context(), account.ID); err != nil || password != "password" {
		t.Fatal(password, err)
	}
}

func TestAccountDeletionBatchesAreDurableBoundedAndIsolated(t *testing.T) {
	db, owner, account, _, other := notificationFixture(t)
	ctx := t.Context()
	for _, id := range []int64{account.ID, other.ID} {
		if _, err := db.ExecContext(ctx, `WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<601) INSERT INTO event_log(account_id,kind) SELECT ?, 'fixture' FROM n`, id); err != nil {
			t.Fatal(err)
		}
		if err := db.LogOperation(ctx, id, "fixture", nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := cleanDeletionTestBatch(ctx, db, account.ID, 250); err == nil {
		t.Fatal("cleaned active account")
	}
	for range 2 {
		if err := db.RequestAccountDeletion(ctx, account.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.GetAccountByID(ctx, account.ID); !errors.Is(err, ErrAccountDeleting) {
		t.Fatal(err)
	}
	if _, _, err := db.GetCredentials(ctx, account.ID); err == nil {
		t.Fatal("credentials accessible")
	}
	if err := db.SaveSession(ctx, account.ID, []byte(`{}`), nil); err == nil {
		t.Fatal("late session accepted")
	}
	if err := db.SavePolicyJSON(ctx, account.ID, `{}`); err == nil {
		t.Fatal("late policy accepted")
	}
	if id, err := db.LogEvent(ctx, EventLog{AccountID: account.ID, Kind: "late"}); err != nil || id != 0 {
		t.Fatal("late event", id, err)
	}
	active, err := db.ListAccounts(ctx, owner.ID)
	if err != nil || len(active) != 0 {
		t.Fatal(active, err)
	}
	visible, err := db.ListAccountsIncludingDeleting(ctx, owner.ID)
	if err != nil || len(visible) != 1 || !visible[0].DeletionPending {
		t.Fatal(visible, err)
	}
	done, n, err := cleanDeletionTestBatch(ctx, db, account.ID, 250)
	if err != nil || done || n != 250 {
		t.Fatal(done, n, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := cleanDeletionTestBatch(cancelled, db, account.ID, 250); err == nil {
		t.Fatal("cancelled batch succeeded")
	}
	var left int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM event_log WHERE account_id=?`, account.ID).Scan(&left); err != nil || left != 351 {
		t.Fatal(left, err)
	}
	if err := deleteAccountFixture(ctx, db, account.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetAccountIncludingDeleting(ctx, account.ID); !errors.Is(err, ErrAccountNotFound) {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM event_log WHERE account_id=?`, other.ID).Scan(&left); err != nil || left != 601 {
		t.Fatal(left, err)
	}
	newAccount, err := db.CreateAccount(ctx, owner.ID, account.Name, account.Channel, account.Username, "new")
	if err != nil || newAccount.ID == account.ID {
		t.Fatal(newAccount, err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM event_log WHERE account_id=?`, newAccount.ID).Scan(&left); err != nil || left != 0 {
		t.Fatal(left, err)
	}
}

func TestPendingDeletionDoesNotBlockOtherAccountsBackgroundWork(t *testing.T) {
	db, owner, account, otherOwner, other := notificationFixture(t)
	ctx := t.Context()
	now := time.Now().UTC()
	if _, err := db.RedeemInstanceID(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.UpsertRedeemCode(ctx, RedeemCodeInput{Code: "DELETIONFIXTURE", Channel: "ios", SourceKey: "test:deletion"}); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureRedeemAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		userID, accountID int64
		key               string
	}{{owner.ID, account.ID, "pending"}, {otherOwner.ID, other.ID, "other"}} {
		if _, err := db.ExecContext(ctx, `INSERT INTO user_notifications(user_id) VALUES(?);`, row.userID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, `INSERT INTO notification_outbox(delivery_key,user_id,account_id,revision,payload,title,next_ms,created_ms) VALUES(?,?,?,1,'{}','fixture',?,?)`, row.key, row.userID, row.accountID, now.UnixMilli(), now.UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.RequestAccountDeletion(ctx, account.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureRedeemAttempts(ctx); err != nil {
		t.Fatal("pending account blocked attempt scheduling", err)
	}
	ids, err := db.DueRedeemAttemptAccountIDs(ctx)
	if err != nil || len(ids) != 1 || ids[0] != other.ID {
		t.Fatal(ids, err)
	}
	// Updating existing history must not abort global maintenance; it cannot
	// replenish deleted rows, unlike a late insert or account-id reassignment.
	if _, err := db.ExecContext(ctx, `UPDATE redeem_attempts SET message='maintenance'`); err != nil {
		t.Fatal(err)
	}
	claimed, err := db.ClaimNotification(ctx, now)
	if err != nil || claimed == nil || claimed.UserID != otherOwner.ID {
		t.Fatal(claimed, err)
	}
	if err := deleteAccountFixture(ctx, db, account.ID); err != nil {
		t.Fatal(err)
	}
	var fkViolations int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_foreign_key_check`).Scan(&fkViolations); err != nil || fkViolations != 0 {
		t.Fatal(fkViolations, err)
	}
}
