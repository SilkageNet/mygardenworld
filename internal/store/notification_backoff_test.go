package store

import (
	"context"
	"testing"
	"time"
)

func TestNotificationBackoffPausesWholeRouteAndSurvivesSettingsEdits(t *testing.T) {
	for _, provider := range []string{"custom", "dingtalk", "feishu", "wecom"} {
		t.Run(provider, func(t *testing.T) {
			db, u, a, other, _ := notificationFixture(t)
			ctx, now := context.Background(), time.Now().UTC()
			endpoint := "https://example.com/hook"
			for _, user := range []*User{u, other} {
				if err := db.SaveNotificationSettings(ctx, user.ID, NotificationUpdate{Enabled: true, Provider: provider, Endpoint: &endpoint, CooldownMinutes: 30}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.LogEvent(ctx, EventLog{AccountID: a.ID, Kind: "failure", TS: now}); err != nil {
				t.Fatal(err)
			}
			if err := db.ConsumeNotificationEvents(ctx, u.ID, now, notificationTestSignal); err != nil {
				t.Fatal(err)
			}
			first, err := db.ClaimNotification(ctx, now)
			if err != nil || first == nil {
				t.Fatal("missing incident", err)
			}
			if err := db.FinishNotification(ctx, first, "pending", "limited", now.Add(10*time.Minute)); err != nil {
				t.Fatal(err)
			}
			// Test messages have a different account key but the same destination.
			for _, user := range []*User{u, other} {
				if _, err := db.QueueNotificationTest(ctx, user.ID, now); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.SaveNotificationSettings(ctx, u.ID, NotificationUpdate{Enabled: true, Provider: provider, CooldownMinutes: 60}); err != nil {
				t.Fatal(err)
			}
			n, err := db.ClaimNotification(ctx, now.Add(time.Minute))
			if err != nil || n == nil || n.UserID != other.ID {
				t.Fatal("other user blocked or limited user escaped", err)
			}
			if err := db.FinishNotification(ctx, n, "sent", "", now); err != nil {
				t.Fatal(err)
			}
			if n, err := db.ClaimNotification(ctx, now.Add(9*time.Minute)); err != nil || n != nil {
				t.Fatal("route backoff bypassed", err)
			}
			n, err = db.ClaimNotification(ctx, now.Add(10*time.Minute))
			if err != nil || n == nil || n.ID != first.ID {
				t.Fatal("retry not released", err)
			}
			if err := db.FinishNotification(ctx, n, "pending", "limited", now.Add(20*time.Minute)); err != nil {
				t.Fatal(err)
			}
			if err := db.SaveNotificationSettings(ctx, u.ID, NotificationUpdate{Enabled: true, Provider: provider, Endpoint: &endpoint, CooldownMinutes: 60}); err != nil {
				t.Fatal(err)
			}
			// Late completion cannot transfer the previous destination's backoff.
			if err := db.FinishNotification(ctx, n, "pending", "stale", now.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			if _, err := db.QueueNotificationTest(ctx, u.ID, now.Add(11*time.Minute)); err != nil {
				t.Fatal(err)
			}
			n, err = db.ClaimNotification(ctx, now.Add(11*time.Minute))
			if err != nil || n == nil || n.UserID != u.ID {
				t.Fatal("new route inherited backoff", err)
			}
		})
	}
}
