package store

import (
	"context"
	"testing"
	"time"
)

func TestIdleNotificationsDoNotAcquireWriter(t *testing.T) {
	db, user, account, _, _ := notificationFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	endpoint := "https://example.test/hook"
	if err := db.SaveNotificationSettings(ctx, user.ID, NotificationUpdate{Enabled: true, Endpoint: &endpoint, Provider: "custom", CooldownMinutes: 30}); err != nil {
		t.Fatal(err)
	}
	// A future retry is also idle, not just an empty outbox.
	id, err := db.QueueNotificationTest(ctx, user.ID, time.Now().Add(time.Hour))
	if err != nil || id == 0 {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	before := db.writer.Stats().WaitCount
	if n, err := db.ClaimNotification(ctx, time.Now()); err != nil || n != nil {
		t.Fatalf("idle claim tried to write: %+v %v", n, err)
	}
	if err := db.ConsumeNotificationEvents(ctx, user.ID, time.Now(), notificationTestSignal); err != nil {
		t.Fatalf("idle ingestion tried to write: %v", err)
	}
	if db.writer.Stats().WaitCount != before {
		t.Fatal("idle polling acquired writer")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.LogEvent(ctx, EventLog{AccountID: account.ID, Kind: "failure"}); err != nil {
		t.Fatal(err)
	}
	if err := db.ConsumeNotificationEvents(ctx, user.ID, time.Now(), notificationTestSignal); err != nil {
		t.Fatal(err)
	}
	rows, err := db.NotificationDeliveries(ctx, user.ID, 0)
	if err != nil || len(rows) != 2 {
		t.Fatalf("new event not consumed after idle poll: %d %v", len(rows), err)
	}
}

func TestNotificationIngestionRechecksSettingsAfterWriterWait(t *testing.T) {
	db, user, account, _, _ := notificationFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	endpoint := "https://example.test/hook"
	if err := db.SaveNotificationSettings(ctx, user.ID, NotificationUpdate{Enabled: true, Endpoint: &endpoint, Provider: "custom", CooldownMinutes: 30}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.LogEvent(ctx, EventLog{AccountID: account.ID, Kind: "failure"}); err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE user_notifications SET enabled = 0 WHERE user_id = ?`, user.ID); err != nil {
		t.Fatal(err)
	}
	before := db.writer.Stats().WaitCount
	result := make(chan error, 1)
	go func() { result <- db.ConsumeNotificationEvents(ctx, user.ID, time.Now(), notificationTestSignal) }()
	waitForWriterQueue(t, db, before)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	rows, err := db.NotificationDeliveries(ctx, user.ID, 0)
	if err != nil || len(rows) != 0 {
		t.Fatalf("queued consumer ignored disabled settings: %d %v", len(rows), err)
	}
}

func TestNotificationClaimRechecksLeaseAfterWriterWait(t *testing.T) {
	db, user, _, _, _ := notificationFixture(t)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	now := time.Now()
	endpoint := "https://example.test/hook"
	if err := db.SaveNotificationSettings(ctx, user.ID, NotificationUpdate{Enabled: true, Endpoint: &endpoint, Provider: "custom", CooldownMinutes: 30}); err != nil {
		t.Fatal(err)
	}
	id, err := db.QueueNotificationTest(ctx, user.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `UPDATE notification_outbox SET status = 'sending', attempts = 1, lease_ms = ? WHERE id = ?`, now.Add(time.Minute).UnixMilli(), id); err != nil {
		t.Fatal(err)
	}
	before := db.writer.Stats().WaitCount
	result := make(chan *NotificationDelivery, 1)
	go func() {
		n, err := db.ClaimNotification(ctx, now)
		if err != nil {
			t.Error(err)
		}
		result <- n
	}()
	waitForWriterQueue(t, db, before)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if n := <-result; n != nil {
		t.Fatalf("queued claim stole an active lease: %+v", n)
	}
}

func TestNotificationExpiryBeforeCleanup(t *testing.T) {
	db, user, _, _, _ := notificationFixture(t)
	ctx := t.Context()
	now := time.Now()
	endpoint := "https://example.test/hook"
	if err := db.SaveNotificationSettings(ctx, user.ID, NotificationUpdate{Enabled: true, Endpoint: &endpoint, Provider: "custom", CooldownMinutes: 30}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.QueueNotificationTest(ctx, user.ID, now.Add(-25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if n, err := db.ClaimNotification(ctx, now); err != nil || n != nil {
		t.Fatalf("expired notification claimed before cleanup: %+v %v", n, err)
	}
	if err := db.CleanNotificationOutbox(ctx, now); err != nil {
		t.Fatal(err)
	}
	rows, err := db.NotificationDeliveries(ctx, user.ID, 0)
	if err != nil || len(rows) != 1 || rows[0].Status != "failed" {
		t.Fatalf("expiry status: %+v %v", rows, err)
	}
}

func TestNotificationCleanupIsBounded(t *testing.T) {
	db, user, _, _, _ := notificationFixture(t)
	ctx := t.Context()
	now := time.Now()
	endpoint := "https://example.test/hook"
	if err := db.SaveNotificationSettings(ctx, user.ID, NotificationUpdate{Enabled: true, Endpoint: &endpoint, Provider: "custom", CooldownMinutes: 30}); err != nil {
		t.Fatal(err)
	}
	// Model retained history plus an expired backlog without thousands of
	// independent fixture transactions. Queue quotas are tested separately.
	if _, err := db.ExecContext(ctx, `WITH RECURSIVE n(id) AS (VALUES(1) UNION ALL SELECT id+1 FROM n WHERE id < 1001)
INSERT INTO notification_outbox(delivery_key, user_id, revision, payload, title, next_ms, created_ms)
SELECT 'expired-' || id, ?, 1, '{}', 'fixture', 0, ? FROM n`, user.ID, now.Add(-8*24*time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []int{1, 0} {
		if err := db.CleanNotificationOutbox(ctx, now); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_outbox`).Scan(&count); err != nil || count != want {
			t.Fatalf("cleanup count=%d want=%d err=%v", count, want, err)
		}
	}
}
