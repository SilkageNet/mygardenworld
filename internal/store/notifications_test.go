package store

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func notificationFixture(t *testing.T) (*DB, *User, *Account, *User, *Account) {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	u, err := db.CreateUser(context.Background(), "owner", "owner@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(context.Background(), u.ID, "own", "ios", "game", "password")
	if err != nil {
		t.Fatal(err)
	}
	v, err := db.CreateUser(context.Background(), "other", "other@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.CreateAccount(context.Background(), v.ID, "other", "ios", "game2", "password")
	if err != nil {
		t.Fatal(err)
	}
	return db, u, a, v, b
}

func notificationTestSignal(e EventLog) *NotificationSignal {
	if e.Kind == "ignored" {
		return nil
	}
	severity := 1
	if e.Level == "error" {
		severity = 2
	}
	return &NotificationSignal{Kind: "incident", Message: "safe summary", Severity: severity, Recovered: e.Kind == "recovery"}
}

func TestNotificationMigrationPreservesV9DataAndIsDisabledByDefault(t *testing.T) {
	db, u, a, _, _ := notificationFixture(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `DROP TABLE notification_outbox; DROP TABLE notification_incidents; DROP TABLE user_notifications; PRAGMA user_version = 9`); err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(ctx, db.DB); err != nil {
		t.Fatal(err)
	}
	if version, err := databaseVersion(ctx, db.DB); err != nil || version != 10 {
		t.Fatalf("version %d: %v", version, err)
	}
	if _, p, err := db.GetCredentials(ctx, a.ID); err != nil || p != "password" {
		t.Fatalf("credentials: %s %v", p, err)
	}
	s, err := db.NotificationSettings(ctx, u.ID)
	if err != nil || s.Enabled || s.HasEndpoint || s.CooldownMinutes != 30 {
		t.Fatalf("defaults: %+v %v", s, err)
	}
	if _, err := db.NotificationSettings(ctx, 0); !errors.Is(err, ErrNotificationSettings) {
		t.Fatal("user 0 accepted", err)
	}
}

func TestNotificationSecretsAreEncryptedUserBoundAndNeverInHistory(t *testing.T) {
	db, u, _, v, _ := notificationFixture(t)
	ctx := context.Background()
	endpoint := "https://example.com/hook?token=TOP-SECRET"
	if err := db.SaveNotificationSettings(ctx, u.ID, true, &endpoint, 30); err != nil {
		t.Fatal(err)
	}
	var encrypted string
	if err := db.QueryRowContext(ctx, `SELECT endpoint_enc FROM user_notifications WHERE user_id = ?`, u.ID).Scan(&encrypted); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encrypted, "SECRET") {
		t.Fatal("plaintext endpoint stored")
	}
	if got, err := db.decodeNotificationEndpoint(u.ID, encrypted); err != nil || got != endpoint {
		t.Fatal("cannot decrypt own endpoint", err)
	}
	if _, err := db.decodeNotificationEndpoint(v.ID, encrypted); err == nil {
		t.Fatal("ciphertext can move across users")
	}
	if _, err := db.decodeSession(u.ID, "v1:"+encrypted); err == nil {
		t.Fatal("notification ciphertext accepted as session")
	}
	if _, err := db.QueueNotificationTest(ctx, u.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	rows, err := db.NotificationDeliveries(ctx, v.ID, 0)
	if err != nil || len(rows) != 0 {
		t.Fatalf("cross-owner history: %+v %v", rows, err)
	}
	n, err := db.ClaimNotification(ctx, time.Now())
	if err != nil || n == nil {
		t.Fatal("claim", err)
	}
	if strings.Contains(n.Payload, "SECRET") || strings.Contains(n.Payload, "token") {
		t.Fatal("payload leaks endpoint")
	}
	if got, err := db.NotificationEndpoint(ctx, n); err != nil || got != endpoint {
		t.Fatal("delivery endpoint", err)
	}
	if err := db.SaveNotificationSettings(ctx, u.ID, false, nil, 30); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NotificationEndpoint(ctx, n); err == nil {
		t.Fatal("disabled delivery still authorized")
	}
	s, err := db.NotificationSettings(ctx, u.ID)
	if err != nil || !s.HasEndpoint || s.Enabled {
		t.Fatalf("retain: %+v %v", s, err)
	}
	empty := ""
	if err := db.SaveNotificationSettings(ctx, u.ID, false, &empty, 30); err != nil {
		t.Fatal(err)
	}
	s, _ = db.NotificationSettings(ctx, u.ID)
	if s.HasEndpoint {
		t.Fatal("clear retained endpoint")
	}
}

func TestNotificationCursorIsolationDedupEscalationRecoveryAndCooldownEdits(t *testing.T) {
	db, u, a, v, b := notificationFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	endpoint := "https://example.com/hook"
	appendEvent := func(accountID int64, kind, level string, at time.Time) {
		t.Helper()
		if _, err := db.LogEvent(ctx, EventLog{AccountID: accountID, Kind: kind, Level: level, TS: at, Message: "secret raw message", PayloadJSON: `{"secret":"credentials"}`}); err != nil {
			t.Fatal(err)
		}
	}
	appendEvent(a.ID, "failure", "warn", now.Add(-time.Hour))
	for _, id := range []int64{u.ID, v.ID} {
		if err := db.SaveNotificationSettings(ctx, id, true, &endpoint, 30); err != nil {
			t.Fatal(err)
		}
	}
	appendEvent(a.ID, "failure", "warn", now)
	appendEvent(b.ID, "failure", "warn", now)
	appendEvent(a.ID, "failure", "warn", now.Add(time.Second))
	appendEvent(a.ID, "failure", "error", now.Add(2*time.Second))
	consume := func(at time.Time) {
		t.Helper()
		if err := db.ConsumeNotificationEvents(ctx, u.ID, at, notificationTestSignal); err != nil {
			t.Fatal(err)
		}
	}
	consume(now.Add(2 * time.Second))
	rows, _ := db.NotificationDeliveries(ctx, u.ID, 0)
	if len(rows) != 2 {
		t.Fatalf("first and escalation, got %d", len(rows))
	}
	other, _ := db.NotificationDeliveries(ctx, v.ID, 0)
	if len(other) != 0 {
		t.Fatal("consumed other user's events")
	}
	consume(now.Add(3 * time.Second))
	rows, _ = db.NotificationDeliveries(ctx, u.ID, 0)
	if len(rows) != 2 {
		t.Fatal("replayed same events")
	}
	// Cooldown-only edits must not reset the incident or discard queued events.
	appendEvent(a.ID, "failure", "warn", now.Add(4*time.Second))
	if err := db.SaveNotificationSettings(ctx, u.ID, true, nil, 60); err != nil {
		t.Fatal(err)
	}
	consume(now.Add(4 * time.Second))
	rows, _ = db.NotificationDeliveries(ctx, u.ID, 0)
	if len(rows) != 2 || rows[0].Status != "pending" {
		t.Fatal("cooldown change lost incident/outbox")
	}
	appendEvent(a.ID, "failure", "warn", now.Add(61*time.Minute))
	consume(now.Add(61 * time.Minute))
	appendEvent(a.ID, "recovery", "info", now.Add(62*time.Minute))
	consume(now.Add(62 * time.Minute))
	rows, _ = db.NotificationDeliveries(ctx, u.ID, 0)
	if len(rows) != 4 {
		t.Fatalf("cooldown and recovery, got %d", len(rows))
	}
	var raw string
	if err := db.QueryRowContext(ctx, `SELECT payload FROM notification_outbox WHERE id = ?`, rows[0].ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var payload NotificationPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Recovered || payload.Occurrences != 5 || payload.DurationSeconds != 62*60 || strings.Contains(raw, "secret") || payload.AccountID != a.ID {
		t.Fatalf("summary: %+v", payload)
	}
}

func TestNotificationLeasesOrderingAndOwnership(t *testing.T) {
	db, u, a, v, b := notificationFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	endpoint := "https://example.com/hook"
	for _, id := range []int64{u.ID, v.ID} {
		if err := db.SaveNotificationSettings(ctx, id, true, &endpoint, 30); err != nil {
			t.Fatal(err)
		}
	}
	for _, a := range []*Account{a, b} {
		for _, kind := range []string{"failure", "recovery"} {
			if _, err := db.LogEvent(ctx, EventLog{AccountID: a.ID, Kind: kind, TS: now}); err != nil {
				t.Fatal(err)
			}
		}
		if err := db.ConsumeNotificationEvents(ctx, a.UserID, now, notificationTestSignal); err != nil {
			t.Fatal(err)
		}
	}
	first, err := db.ClaimNotification(ctx, now)
	if err != nil || first == nil {
		t.Fatal(err)
	}
	other, err := db.ClaimNotification(ctx, now)
	if err != nil || other == nil || other.UserID == first.UserID {
		t.Fatal("later recovery overtook earlier active delivery", err)
	}
	retry, err := db.ClaimNotification(ctx, now.Add(2*time.Minute))
	if err != nil || retry == nil || retry.ID != first.ID || retry.Key != first.Key || retry.Attempts != 2 {
		t.Fatalf("lease retry: %+v %v", retry, err)
	}
	if err := db.FinishNotification(ctx, first, "sent", "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NotificationEndpoint(ctx, retry); err != nil {
		t.Fatal("stale acknowledgement overwrote new lease", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE accounts SET user_id = ? WHERE id = ?`, v.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NotificationEndpoint(ctx, retry); err == nil {
		t.Fatal("ownership recheck omitted")
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET status = 'disabled' WHERE id = ?`, v.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.NotificationEndpoint(ctx, other); err == nil {
		t.Fatal("disabled user can receive")
	}
	if _, err := db.NotificationDeliveries(ctx, 0, 0); !errors.Is(err, ErrNotificationSettings) {
		t.Fatal("unscoped history")
	}
}

func TestNotificationQueueFullRollsBackCursorAndTestRateLimit(t *testing.T) {
	db, u, a, _, _ := notificationFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	endpoint := "https://example.com/hook"
	if err := db.SaveNotificationSettings(ctx, u.ID, true, &endpoint, 30); err != nil {
		t.Fatal(err)
	}
	if _, err := db.QueueNotificationTest(ctx, u.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.QueueNotificationTest(ctx, u.ID, now.Add(time.Second)); !errors.Is(err, ErrNotificationTestCooldown) {
		t.Fatal("test flood not limited", err)
	}
	for i := 1; i < 200; i++ {
		if _, err := db.QueueNotificationTest(ctx, u.ID, now.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.LogEvent(ctx, EventLog{AccountID: a.ID, Kind: "failure", TS: now}); err != nil {
		t.Fatal(err)
	}
	if err := db.ConsumeNotificationEvents(ctx, u.ID, now, notificationTestSignal); !errors.Is(err, ErrNotificationQueueFull) {
		t.Fatal("unbounded queue", err)
	}
	var cursor int64
	if err := db.QueryRowContext(ctx, `SELECT event_cursor FROM user_notifications WHERE user_id = ?`, u.ID).Scan(&cursor); err != nil || cursor != 0 {
		t.Fatal("cursor advanced despite failed outbox", cursor, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE notification_outbox SET status = 'sent'`); err != nil {
		t.Fatal(err)
	}
	if err := db.ConsumeNotificationEvents(ctx, u.ID, now, notificationTestSignal); err != nil {
		t.Fatal(err)
	}
	rows, _ := db.NotificationDeliveries(ctx, u.ID, 0)
	if len(rows) != 6 {
		t.Fatal("history must return at most one page plus lookahead", len(rows))
	}
}

func TestNotificationRestartAndConcurrentClaims(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	path := filepath.Join(t.TempDir(), "garden.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	u, err := db.CreateUser(ctx, "owner", "owner@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(ctx, u.ID, "own", "ios", "game", "secret")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "https://example.com/hook"
	if err := db.SaveNotificationSettings(ctx, u.ID, true, &endpoint, 30); err != nil {
		t.Fatal(err)
	}
	if _, err := db.LogEvent(ctx, EventLog{AccountID: a.ID, Kind: "failure", TS: now}); err != nil {
		t.Fatal(err)
	}
	if err := db.ConsumeNotificationEvents(ctx, u.ID, now, notificationTestSignal); err != nil {
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
	if err := db.ConsumeNotificationEvents(ctx, u.ID, now, notificationTestSignal); err != nil {
		t.Fatal(err)
	}
	rows, _ := db.NotificationDeliveries(ctx, u.ID, 0)
	if len(rows) != 1 {
		t.Fatal("restart lost or duplicated outbox", len(rows))
	}
	var workers sync.WaitGroup
	claims := make(chan *NotificationDelivery, 4)
	for range 4 {
		workers.Go(func() {
			n, err := db.ClaimNotification(ctx, now)
			if err != nil {
				t.Error(err)
			}
			if n != nil {
				claims <- n
			}
		})
	}
	workers.Wait()
	close(claims)
	if len(claims) != 1 {
		t.Fatal("concurrent workers claimed same delivery", len(claims))
	}
	n := <-claims
	if got, err := db.NotificationEndpoint(ctx, n); err != nil || got != endpoint {
		t.Fatal("credential not recoverable after restart", err)
	}
	// A stopped worker's last attempt eventually becomes visible as failed.
	if _, err := db.ExecContext(ctx, `UPDATE notification_outbox SET attempts = 5, lease_ms = ? WHERE id = ?`, now.UnixMilli(), n.ID); err != nil {
		t.Fatal(err)
	}
	if next, err := db.ClaimNotification(ctx, now.Add(time.Second)); err != nil || next != nil {
		t.Fatal("exhausted lease retried", err)
	}
	rows, _ = db.NotificationDeliveries(ctx, u.ID, 0)
	if rows[0].Status != "failed" {
		t.Fatal("exhausted delivery not failed")
	}
	if _, err := db.ClaimNotification(ctx, now.Add(8*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	rows, _ = db.NotificationDeliveries(ctx, u.ID, 0)
	if len(rows) != 0 {
		t.Fatal("old history retained beyond 7 days")
	}
}
