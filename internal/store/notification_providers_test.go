package store

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNativeNotificationPacingIsAtomicAndDoesNotBlockOtherUsers(t *testing.T) {
	db, u, a, other, _ := notificationFixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	endpoint := "https://example.com/hook"
	for _, user := range []*User{u, other} {
		if err := db.SaveNotificationSettings(ctx, user.ID, NotificationUpdate{Enabled: true, Endpoint: &endpoint, Provider: "dingtalk", CooldownMinutes: 30}); err != nil {
			t.Fatal(err)
		}
	}
	// Separate accounts avoid per-account ordering masking the per-user gate.
	a2, err := db.CreateAccount(ctx, u.ID, "second", "ios", "fixture", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{a.ID, a2.ID} {
		if _, err := db.LogEvent(ctx, EventLog{AccountID: id, Kind: "failure", TS: now}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.ConsumeNotificationEvents(ctx, u.ID, now, notificationTestSignal); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	claimed := make(chan *NotificationDelivery, 4)
	for range 4 {
		wg.Go(func() {
			n, err := db.ClaimNotification(ctx, now)
			if err != nil {
				t.Error(err)
			}
			if n != nil {
				claimed <- n
			}
		})
	}
	wg.Wait()
	close(claimed)
	if len(claimed) != 1 {
		t.Fatalf("parallel workers bypassed pacing: %d claims", len(claimed))
	}
	first := <-claimed
	if err := db.FinishNotification(ctx, first, "sent", "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.QueueNotificationTest(ctx, other.ID, now); err != nil {
		t.Fatal(err)
	}
	n, err := db.ClaimNotification(ctx, now)
	if err != nil || n == nil || n.UserID != other.ID {
		t.Fatal("other user blocked", err)
	}
	if err := db.FinishNotification(ctx, n, "sent", "", now); err != nil {
		t.Fatal(err)
	}
	if n, err := db.ClaimNotification(ctx, now.Add(3999*time.Millisecond)); err != nil || n != nil {
		t.Fatal("pacing ended early", err)
	}
	n, err = db.ClaimNotification(ctx, now.Add(4*time.Second))
	if err != nil || n == nil || n.UserID != u.ID {
		t.Fatal("next account not released at pacing boundary", err)
	}
}

func TestNotificationProviderMigrationPreservesV10SettingsAndOutbox(t *testing.T) {
	db, u, _, _, _ := notificationFixture(t)
	ctx := context.Background()
	endpoint := "https://example.com/hook?key=example"
	if err := db.SaveNotificationSettings(ctx, u.ID, NotificationUpdate{Enabled: true, Endpoint: &endpoint, Provider: "custom", CooldownMinutes: 45}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.QueueNotificationTest(ctx, u.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	before, err := db.NotificationSettings(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE daemon_maintenance; ALTER TABLE user_notifications DROP COLUMN provider; ALTER TABLE user_notifications DROP COLUMN signing_secret_enc; ALTER TABLE user_notifications DROP COLUMN retry_after_ms; DROP INDEX idx_notification_outbox_attempt; ALTER TABLE notification_outbox DROP COLUMN last_attempt_ms; PRAGMA user_version = 10`); err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(ctx, db.writer); err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(ctx, db.writer); err != nil {
		t.Fatal("migration not idempotent", err)
	}
	after, err := db.NotificationSettings(ctx, u.ID)
	if err != nil || after != before {
		t.Fatalf("settings changed: %+v -> %+v (%v)", before, after, err)
	}
	n, err := db.ClaimNotification(ctx, time.Now())
	if err != nil || n == nil {
		t.Fatal("outbox lost", err)
	}
	target, err := db.NotificationDestination(ctx, n)
	if err != nil || target.Endpoint != endpoint || target.Provider != "custom" || target.SigningSecret != "" {
		t.Fatal("existing encrypted route not preserved", err)
	}
}

func TestNotificationSigningCredentialsAreBoundAndRoutingChangesCancelPending(t *testing.T) {
	for _, provider := range []string{"dingtalk", "feishu"} {
		t.Run(provider, func(t *testing.T) {
			db, u, _, other, _ := notificationFixture(t)
			ctx := context.Background()
			endpoint, secret := "https://example.com/hook?token=fixture", "SIGNING-SECRET"
			update := NotificationUpdate{Enabled: true, Endpoint: &endpoint, CooldownMinutes: 30, Provider: provider, SigningSecret: &secret}
			if err := db.SaveNotificationSettings(ctx, u.ID, update); err != nil {
				t.Fatal(err)
			}
			var encrypted string
			if err := db.QueryRowContext(ctx, `SELECT signing_secret_enc FROM user_notifications WHERE user_id = ?`, u.ID).Scan(&encrypted); err != nil {
				t.Fatal(err)
			}
			if encrypted == "" || strings.Contains(encrypted, secret) {
				t.Fatal("plaintext secret saved")
			}
			if _, err := db.decodeNotificationCredential(other.ID, "signing", encrypted); err == nil {
				t.Fatal("cross-user decryption succeeded")
			}
			if _, err := db.decodeNotificationCredential(u.ID, "endpoint", encrypted); err == nil {
				t.Fatal("signing secret accepted as endpoint")
			}
			if _, err := db.QueueNotificationTest(ctx, u.ID, time.Now()); err != nil {
				t.Fatal(err)
			}
			n, err := db.ClaimNotification(ctx, time.Now())
			if err != nil || n == nil {
				t.Fatal(err)
			}
			target, err := db.NotificationDestination(ctx, n)
			if err != nil || target.Provider != provider || target.SigningSecret != secret || strings.Contains(n.Payload, secret) {
				t.Fatal("incorrect credentials", err)
			}
			// Editing just cooldown must retain credentials and the live lease.
			update.Endpoint, update.SigningSecret, update.CooldownMinutes = nil, nil, 60
			if err := db.SaveNotificationSettings(ctx, u.ID, update); err != nil {
				t.Fatal(err)
			}
			if got, err := db.NotificationDestination(ctx, n); err != nil || got != target {
				t.Fatal("cooldown edit changed routing", err)
			}
			newSecret := "REPLACEMENT"
			update.SigningSecret = &newSecret
			if err := db.SaveNotificationSettings(ctx, u.ID, update); err != nil {
				t.Fatal(err)
			}
			if _, err := db.NotificationDestination(ctx, n); err == nil {
				t.Fatal("old lease remains authorized after key change")
			}
			// An endpoint replacement cannot silently inherit the previous key.
			update.Endpoint, update.SigningSecret = &endpoint, nil
			if err := db.SaveNotificationSettings(ctx, u.ID, update); err != nil {
				t.Fatal(err)
			}
			settings, err := db.NotificationSettings(ctx, u.ID)
			if err != nil || settings.HasSigningSecret {
				t.Fatal("new destination inherited old key", err)
			}
			update.Provider, update.Endpoint = "custom", nil
			if err := db.SaveNotificationSettings(ctx, u.ID, update); !errors.Is(err, ErrNotificationSettings) {
				t.Fatal("provider change reused old URL", err)
			}
			update.Endpoint, update.SigningSecret = &endpoint, &secret
			if err := db.SaveNotificationSettings(ctx, u.ID, update); !errors.Is(err, ErrNotificationSettings) {
				t.Fatal("custom accepted signing key", err)
			}
		})
	}
}

func TestNotificationProviderValidationAndExplicitRemoval(t *testing.T) {
	for _, provider := range []string{"", "unknown", "custom", "wecom"} {
		db, u, _, _, _ := notificationFixture(t)
		endpoint, secret := "https://example.com/hook", "test-key"
		if err := db.SaveNotificationSettings(context.Background(), u.ID, NotificationUpdate{Enabled: true, Provider: provider, Endpoint: &endpoint, SigningSecret: &secret, CooldownMinutes: 30}); !errors.Is(err, ErrNotificationSettings) {
			t.Fatalf("invalid settings accepted for %q: %v", provider, err)
		}
	}
	for _, clearEndpoint := range []bool{false, true} {
		db, u, _, _, _ := notificationFixture(t)
		ctx := context.Background()
		endpoint, secret, empty := "https://example.com/hook", "test-key", ""
		update := NotificationUpdate{Enabled: true, Provider: "feishu", Endpoint: &endpoint, SigningSecret: &secret, CooldownMinutes: 30}
		if err := db.SaveNotificationSettings(ctx, u.ID, update); err != nil {
			t.Fatal(err)
		}
		update.Endpoint, update.SigningSecret = nil, &empty
		if clearEndpoint {
			update.Enabled, update.Endpoint, update.SigningSecret = false, &empty, nil
		}
		if err := db.SaveNotificationSettings(ctx, u.ID, update); err != nil {
			t.Fatal(err)
		}
		settings, err := db.NotificationSettings(ctx, u.ID)
		if err != nil || settings.HasSigningSecret || settings.HasEndpoint == clearEndpoint {
			t.Fatal("explicit removal failed", err)
		}
	}
}
