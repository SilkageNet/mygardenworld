package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"modernc.org/sqlite"
)

func init() {
	// A test-only disk-latency surrogate; no live account or filesystem is used.
	if err := sqlite.RegisterScalarFunction("deletion_test_delay", 0, func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
		time.Sleep(10 * time.Millisecond)
		return int64(0), nil
	}); err != nil {
		panic(err)
	}
}

func TestDeletionExecutionDeadlineRollsBackAndCanResume(t *testing.T) {
	db, a, _ := deletionProgressFixture(t)
	if _, err := db.ExecContext(t.Context(), `CREATE TRIGGER slow_delete BEFORE DELETE ON event_log BEGIN SELECT deletion_test_delay(); END;`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	b, err := db.CleanAccountDeletionBatch(ctx, a.ID, 250)
	cancel()
	if err == nil || b.Removed != 0 || b.Phase != "event_log" || DeletionErrorKind(err) != "timeout" {
		t.Fatal(b, err)
	}
	p, err := db.AccountDeletionProgress(t.Context(), a.ID)
	if err != nil || p.RemovedRows != 0 || p.LastProgressMS != 0 {
		t.Fatal("timeout counted as progress", p, err)
	}
	if _, err := db.ExecContext(t.Context(), `DROP TRIGGER slow_delete`); err != nil {
		t.Fatal(err)
	}
	b, err = db.CleanAccountDeletionBatch(t.Context(), a.ID, 250)
	if err != nil || b.Removed != 250 {
		t.Fatal("cancelled connection did not recover", b, err)
	}
}

func deletionProgressFixture(t *testing.T) (*DB, *Account, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "garden.db")
	db, err := Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	u, err := db.CreateUser(t.Context(), "owner", "owner@test.example", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(t.Context(), u.ID, "garden", "ios", "u", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<1001) INSERT INTO event_log(account_id,kind) SELECT ?, 'fixture' FROM n`, a.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.RequestAccountDeletion(t.Context(), a.ID); err != nil {
		t.Fatal(err)
	}
	return db, a, path
}

func TestDeletionProgressCommitsAtomicallyAndSurvivesReopen(t *testing.T) {
	db, a, path := deletionProgressFixture(t)
	b, err := db.CleanAccountDeletionBatch(t.Context(), a.ID, 250)
	if err != nil || b.Removed != 250 {
		t.Fatal(b, err)
	}
	p, err := db.AccountDeletionProgress(t.Context(), a.ID)
	if err != nil || p.RemovedRows != 250 || p.LastProgressMS == 0 || p.TrackingStartedMS == 0 {
		t.Fatal(p, err)
	}
	if _, err := db.ExecContext(t.Context(), `CREATE TRIGGER fail_progress BEFORE UPDATE OF removed_rows ON account_deletion_progress BEGIN SELECT RAISE(ABORT,'private path fixture'); END`); err != nil {
		t.Fatal(err)
	}
	if b, err := db.CleanAccountDeletionBatch(t.Context(), a.ID, 250); err == nil || b.Removed != 0 {
		t.Fatal("rolled-back batch counted", b, err)
	}
	var left int
	if err := db.QueryRowContext(t.Context(), `SELECT count(*) FROM event_log WHERE account_id=?`, a.ID).Scan(&left); err != nil || left != 751 {
		t.Fatal(left, err)
	}
	failure := DeletionAttempt{Phase: "event_log", AttemptMS: time.Now().UnixMilli(), ErrorKind: "database", RetryAtMS: time.Now().Add(time.Minute).UnixMilli(), Failures: 1, BatchSize: 250}
	if err := db.RecordDeletionFailure(t.Context(), a.ID, failure); err != nil {
		t.Fatal(err)
	}
	if err := db.RequestAccountDeletion(t.Context(), a.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	p2, err := db.AccountDeletionProgress(t.Context(), a.ID)
	if err != nil || p2.RemovedRows != 250 || p2.TrackingStartedMS != p.TrackingStartedMS || p2.LastProgressMS != p.LastProgressMS || p2.DeletionAttempt != failure {
		t.Fatal(p2, err)
	}
	if _, err := db.ExecContext(t.Context(), `DROP TRIGGER fail_progress`); err != nil {
		t.Fatal(err)
	}
	if b, err := db.CleanAccountDeletionBatch(t.Context(), a.ID, 250); err != nil || b.Removed != 250 {
		t.Fatal(b, err)
	}
	p2, err = db.AccountDeletionProgress(t.Context(), a.ID)
	if err != nil || p2.RemovedRows != 500 || p2.ErrorKind != "" || p2.Failures != 0 {
		t.Fatal(p2, err)
	}
	failure.AttemptMS = p2.AttemptMS - 1
	if err := db.RecordDeletionFailure(t.Context(), a.ID, failure); err != nil {
		t.Fatal(err)
	}
	acc, err := db.GetAccountIncludingDeleting(t.Context(), a.ID)
	if err != nil || acc.DeletionFailed {
		t.Fatal("stale failure replaced committed progress", acc, err)
	}
	if err := deleteAccountFixture(t.Context(), db, a.ID); err != nil {
		t.Fatal(err)
	}
	if p, err := db.AccountDeletionProgress(t.Context(), a.ID); err != nil || p.TrackingStartedMS != 0 {
		t.Fatal("progress orphaned", p, err)
	}
}

func TestDeletionErrorClassificationDoesNotEchoMessages(t *testing.T) {
	if got := DeletionErrorKind(errors.New("SQLITE_FULL /private/secret")); got != "database" {
		t.Fatal(got)
	}
	if got := DeletionErrorKind(context.DeadlineExceeded); got != "timeout" {
		t.Fatal(got)
	}
}

func TestDeletionProgressMigrationSeedsExistingPendingAccounts(t *testing.T) {
	db, a, _ := deletionProgressFixture(t)
	if _, err := db.ExecContext(t.Context(), `DROP TABLE account_deletion_progress; PRAGMA user_version=15`); err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(t.Context(), db.writer); err != nil {
		t.Fatal(err)
	}
	p, err := db.AccountDeletionProgress(t.Context(), a.ID)
	if err != nil || p.TrackingStartedMS == 0 || p.RemovedRows != 0 || p.Phase != "queued" {
		t.Fatal(p, err)
	}
	if err := applyMigrations(t.Context(), db.writer); err != nil {
		t.Fatal(err)
	}
	b, err := db.CleanAccountDeletionBatch(t.Context(), a.ID, 250)
	if err != nil || b.Removed != 250 {
		t.Fatal(b, err)
	}
}

func TestDeletionWriterWaitAndExternalLockDoNotUseExecutionBudget(t *testing.T) {
	db, a, path := deletionProgressFixture(t)
	conn, err := db.writer.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	b, err := db.CleanAccountDeletionBatch(ctx, a.ID, 250)
	cancel()
	_ = conn.Close()
	if !errors.Is(err, context.DeadlineExceeded) || b.Phase != "wait_writer" || b.WorkMS != 0 {
		t.Fatal(b, err)
	}
	other, err := sql.Open("sqlite", databaseFileURL(path)+"?_txlock=immediate&_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Close() }()
	tx, err := other.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err = db.CleanAccountDeletionBatch(t.Context(), a.ID, 250)
	_ = tx.Rollback()
	if DeletionErrorKind(err) != "busy" || b.Phase != "begin_transaction" || b.WorkMS != 0 {
		t.Fatal(b, err)
	}
	var timeout int
	if err := db.writer.QueryRowContext(t.Context(), `PRAGMA busy_timeout`).Scan(&timeout); err != nil || timeout != 5000 {
		t.Fatal("writer settings leaked", timeout, err)
	}
	b, err = db.CleanAccountDeletionBatch(t.Context(), a.ID, 250)
	if err != nil || b.Removed != 250 {
		t.Fatal("did not recover", b, err)
	}
}
