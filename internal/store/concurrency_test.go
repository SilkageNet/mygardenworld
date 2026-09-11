package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"modernc.org/sqlite"
)

// Use separate physical connections: a single connection would hide the
// deferred read-to-write upgrade failure reported when submitting redeem codes.
func TestTransactionWriteReservation(t *testing.T) {
	for _, readOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("readOnly=%v", readOnly), func(t *testing.T) {
			ctx := t.Context()
			path := filepath.Join(t.TempDir(), "garden.db")
			db, err := Open(ctx, path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			if _, err := db.RedeemInstanceID(ctx); err != nil {
				t.Fatal(err)
			}
			// An independent handle also models the operator maintenance CLI.
			other, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(0)")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = other.Close() })
			tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: readOnly})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			var before, after int64
			if err := tx.QueryRowContext(ctx, `SELECT next_revision FROM redeem_node_state WHERE id = 1`).Scan(&before); err != nil {
				t.Fatal(err)
			}
			_, err = other.ExecContext(ctx, `UPDATE redeem_node_state SET next_revision = next_revision + 1 WHERE id = 1`)
			if readOnly {
				if err != nil {
					t.Fatalf("read-only snapshot blocked writer: %v", err)
				}
				if err := tx.QueryRowContext(ctx, `SELECT next_revision FROM redeem_node_state WHERE id = 1`).Scan(&after); err != nil || after != before {
					t.Fatalf("snapshot changed: before=%d after=%d err=%v", before, after, err)
				}
			} else {
				var sqliteErr *sqlite.Error
				if !errors.As(err, &sqliteErr) || sqliteErr.Code()&0xff != 5 {
					t.Fatalf("write transaction did not reserve writer before reading: %v", err)
				}
				if revision, err := nextRedeemRevision(ctx, tx); err != nil || revision != before+1 {
					t.Fatalf("advance revision=%d err=%v", revision, err)
				}
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if _, err := other.ExecContext(ctx, `UPDATE redeem_node_state SET next_revision = next_revision + 1 WHERE id = 1`); err != nil {
				t.Fatalf("rollback did not release writer: %v", err)
			}
		})
	}
}

func TestConcurrentRedeemSubmissionsAndLogs(t *testing.T) {
	ctx := t.Context()
	db, user := accountQuotaFixture(t)
	account, err := db.CreateAccount(ctx, user.ID, "fixture", "ios", "fixture", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.RedeemInstanceID(ctx); err != nil {
		t.Fatal(err)
	}
	const workers, codes = 12, 20
	start := make(chan struct{})
	var wg sync.WaitGroup
	for worker := range workers {
		wg.Go(func() {
			<-start
			for code := range codes {
				_, _, err := db.UpsertRedeemCode(ctx, RedeemCodeInput{
					Code: fmt.Sprintf("concurrent-%d", code), Channel: "ios", SourceKey: fmt.Sprintf("public:%d", worker),
				})
				if err != nil {
					t.Errorf("worker %d code %d: %v", worker, code, err)
					return
				}
				if _, err := db.LogEvent(ctx, EventLog{AccountID: account.ID, Kind: "system", Level: "info", Action: "concurrent write"}); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	close(start)
	wg.Wait()
	for _, check := range []struct {
		query string
		want  int
	}{
		{`SELECT COUNT(*) FROM redeem_codes`, codes},
		{`SELECT COUNT(DISTINCT revision) FROM redeem_codes`, codes},
		{`SELECT next_revision FROM redeem_node_state WHERE id = 1`, codes},
		{`SELECT COUNT(*) FROM redeem_code_observations`, workers * codes},
		{`SELECT COUNT(*) FROM event_log`, workers * codes},
	} {
		var got int
		if err := db.QueryRowContext(ctx, check.query).Scan(&got); err != nil || got != check.want {
			t.Errorf("%s: got=%d want=%d err=%v", check.query, got, check.want, err)
		}
	}
	if err := db.EnsureRedeemAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	// Claim and completion also read before writing. Verify concurrent workers
	// preserve exclusive leases and commit every result exactly once.
	claimed := make(chan int64, workers*codes)
	for range workers {
		wg.Go(func() {
			for range codes {
				attempt, err := db.NextRedeemAttempt(ctx)
				if err != nil {
					t.Error(err)
					return
				}
				if attempt == nil {
					return
				}
				claimed <- attempt.ID
				if err := db.CompleteRedeemAttempt(ctx, attempt.ID, attempt.RunToken, RedeemValidationSuccess, "fixture", nil); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	wg.Wait()
	close(claimed)
	seen := make(map[int64]bool)
	for id := range claimed {
		if seen[id] {
			t.Errorf("attempt %d claimed twice", id)
		}
		seen[id] = true
	}
	var completed int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM redeem_attempts WHERE status = 'success' AND attempt_count = 1`).Scan(&completed); err != nil || completed != codes || len(seen) != codes {
		t.Fatalf("claimed=%d completed=%d want=%d err=%v", len(seen), completed, codes, err)
	}
}

func TestCanceledTransactionDoesNotRetainWriter(t *testing.T) {
	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if tx, err := db.BeginTx(ctx, nil); !errors.Is(err, context.Canceled) {
		if tx != nil {
			_ = tx.Rollback()
		}
		t.Fatalf("canceled BeginTx: %v", err)
	}
	if _, err := db.RedeemInstanceID(t.Context()); err != nil {
		t.Fatalf("write after cancellation: %v", err)
	}
	activeCtx, cancelActive := context.WithCancel(t.Context())
	defer cancelActive()
	tx, err := db.BeginTx(activeCtx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := nextRedeemRevision(activeCtx, tx); err != nil {
		t.Fatal(err)
	}
	cancelActive()
	var revision int
	if err := db.writeRowContext(t.Context(), `UPDATE redeem_node_state SET next_revision = next_revision + 1 WHERE id = 1 RETURNING next_revision`).Scan(&revision); err != nil || revision != 1 {
		t.Fatalf("write after active cancellation: revision=%d err=%v", revision, err)
	}
}
