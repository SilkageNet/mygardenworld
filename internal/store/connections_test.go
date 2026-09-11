package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func waitForWriterQueue(t *testing.T, db *DB, before int64) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for db.writer.Stats().WaitCount <= before {
		select {
		case <-deadline.C:
			t.Fatal("operation did not join the writer queue")
		case <-tick.C:
		}
	}
}

func TestWriterQueueCancellationAndReadIsolation(t *testing.T) {
	for _, kind := range []string{"exec", "returning", "transaction"} {
		t.Run(kind, func(t *testing.T) {
			db, user := accountQuotaFixture(t)
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			account, err := db.CreateAccountWithPolicy(ctx, user.ID, "queue", "ios", "fixture", "fixture", `{}`)
			if err != nil {
				t.Fatal(err)
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback() }()
			if _, err := tx.ExecContext(ctx, `UPDATE account_policies SET policy_json = 'uncommitted' WHERE account_id = ?`, account.ID); err != nil {
				t.Fatal(err)
			}
			before := db.writer.Stats().WaitCount
			writeCtx, cancelWrite := context.WithCancel(ctx)
			defer cancelWrite()
			result := make(chan error, 1)
			go func() {
				var err error
				switch kind {
				case "exec":
					err = db.SavePolicyJSON(writeCtx, account.ID, `{"automation_enabled":true}`)
				case "returning":
					_, err = db.AdvancePearlHireTicketUsed(writeCtx, account.ID, 20260911, 1)
				case "transaction":
					var queued *sql.Tx
					queued, err = db.BeginTx(writeCtx, nil)
					if queued != nil {
						_ = queued.Rollback()
					}
				}
				result <- err
			}()
			waitForWriterQueue(t, db, before)
			if policy, err := db.LoadPolicyJSON(ctx, account.ID); err != nil || policy != `{}` {
				t.Fatalf("writer blocked read or leaked uncommitted data: %q %v", policy, err)
			}
			cancelWrite()
			if err := <-result; !errors.Is(err, context.Canceled) {
				t.Fatalf("queued cancellation: %v", err)
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if policy, err := db.LoadPolicyJSON(ctx, account.ID); err != nil || policy != `{}` {
				t.Fatalf("canceled write changed policy: %q %v", policy, err)
			}
			if used, err := db.PearlHireTicketUsed(ctx, account.ID, 20260911); err != nil || used != 0 {
				t.Fatalf("canceled write spent tickets: %d %v", used, err)
			}
			if err := db.SavePolicyJSON(ctx, account.ID, `{}`); err != nil {
				t.Fatalf("writer reservation leaked after cancellation: %v", err)
			}
		})
	}
}

func TestReadPoolIsBoundedAndRejectsWrites(t *testing.T) {
	db, _ := accountQuotaFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	if db.writer.Stats().MaxOpenConnections != 1 || db.reader.Stats().MaxOpenConnections != 4 {
		t.Fatal("unexpected connection limits")
	}
	var conns []*sql.Conn
	defer func() {
		for _, conn := range conns {
			_ = conn.Close()
		}
	}()
	for range 4 {
		conn, err := db.reader.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, conn)
		var readOnly int
		if err := conn.QueryRowContext(ctx, `PRAGMA query_only`).Scan(&readOnly); err != nil || readOnly != 1 {
			t.Fatalf("read-only connection: %d %v", readOnly, err)
		}
		if _, err := conn.ExecContext(ctx, `UPDATE users SET max_accounts = 999`); err == nil {
			t.Fatal("read pool allowed a write")
		}
	}
	shortCtx, shortCancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer shortCancel()
	if conn, err := db.reader.Conn(shortCtx); !errors.Is(err, context.DeadlineExceeded) {
		if conn != nil {
			_ = conn.Close()
		}
		t.Fatalf("read pool did not bound/cancel acquisition: %v", err)
	}
	// Saturated readers must not consume the writer's connection.
	if _, err := db.ExecContext(ctx, `UPDATE users SET max_accounts = 8`); err != nil {
		t.Fatal(err)
	}
	for _, conn := range conns {
		_ = conn.Close()
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET max_accounts = 999`); err == nil {
		t.Fatal("read-only transaction allowed a write")
	}
}

func TestDatabasePathIsNotDSNOptions(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"/data/garden ?#%.db", "file:///data/garden%20%3F%23%25.db"},
		{"C:/data/garden ?#%.db", "file:///C:/data/garden%20%3F%23%25.db"},
	} {
		if got := databaseFileURL(tc.path); got != tc.want {
			t.Errorf("databaseFileURL(%q)=%q, want %q", tc.path, got, tc.want)
		}
	}
	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "garden ?mode=ro#test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.CreateUser(t.Context(), "path", "path@example.test", "fixture"); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentAccountPersistenceAndBackgroundWork(t *testing.T) {
	db, user := accountQuotaFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, `UPDATE users SET max_accounts = 8 WHERE id = ?`, user.ID); err != nil {
		t.Fatal(err)
	}
	endpoint := "https://example.test/hook"
	if err := db.SaveNotificationSettings(ctx, user.ID, NotificationUpdate{Enabled: true, Endpoint: &endpoint, Provider: "custom", CooldownMinutes: 30}); err != nil {
		t.Fatal(err)
	}
	// With a single writer, local contention must succeed even without SQLite
	// lock retries. Eight account loops overlap notification and cleanup writes.
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout = 0`); err != nil {
		t.Fatal(err)
	}
	const cycles = 30
	var wg sync.WaitGroup
	for worker := range 8 {
		wg.Go(func() {
			account, err := db.CreateAccountWithPolicy(ctx, user.ID, fmt.Sprintf("account-%d", worker), "ios", fmt.Sprintf("fixture-%d", worker), "fixture", `{}`)
			if err != nil {
				t.Error(err)
				return
			}
			for i := range cycles {
				policy := fmt.Sprintf(`{"automation_enabled":%t}`, i%2 == 0)
				if err := db.SavePolicyJSON(ctx, account.ID, policy); err != nil {
					t.Error(err)
					return
				}
				if got, err := db.LoadPolicyJSON(ctx, account.ID); err != nil || got != policy {
					t.Errorf("policy round trip: %q %v", got, err)
				}
				if err := db.SaveSession(ctx, account.ID, []byte("fixture session"), nil); err != nil {
					t.Error(err)
				}
				if _, err := db.LogEvent(ctx, EventLog{AccountID: account.ID, Kind: "failure", Level: "warn"}); err != nil {
					t.Error(err)
				}
				if err := db.LogOperation(ctx, account.ID, "fixture", nil, nil); err != nil {
					t.Error(err)
				}
				if used, err := db.AdvancePearlHireTicketUsed(ctx, account.ID, 20260911, 1); err != nil || used != int32(i+1) {
					t.Errorf("returning count: %d %v", used, err)
				}
			}
		})
	}
	for range 4 {
		wg.Go(func() {
			for range cycles {
				if err := db.ConsumeNotificationEvents(ctx, user.ID, time.Now(), notificationTestSignal); err != nil {
					t.Error(err)
				}
				n, err := db.ClaimNotification(ctx, time.Now())
				if err != nil {
					t.Error(err)
				}
				if n != nil {
					if err := db.FinishNotification(ctx, n, "sent", "", time.Now()); err != nil {
						t.Error(err)
					}
				}
				if _, err := db.CleanLogsBefore(ctx, time.Now().Add(-7*24*time.Hour)); err != nil {
					t.Error(err)
				}
			}
		})
	}
	wg.Wait()
	for _, table := range []string{"event_log", "operation_log"} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil || count != 8*cycles {
			t.Fatalf("%s lost writes: count=%d err=%v", table, count, err)
		}
	}
}
