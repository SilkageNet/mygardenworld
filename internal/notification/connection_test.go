package notification

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestConnectionIncidentPersistsDeduplicatesAndRecovers(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "garden.db")
	db, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	u, err := db.CreateUser(ctx, "owner", "owner@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(ctx, u.ID, "own", "ios", "game", "password")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.CreateUser(ctx, "other", "other@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.CreateAccount(ctx, other.ID, "other", "ios", "game2", "password")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "https://example.com/hook"
	if err = db.SaveNotificationSettings(ctx, u.ID, store.NotificationUpdate{Enabled: true, Endpoint: &endpoint, CooldownMinutes: 30, Provider: "custom"}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	appendEvent := func(id int64, kind string) {
		t.Helper()
		if _, err := db.LogEvent(ctx, store.EventLog{AccountID: id, TS: now, Kind: kind, Message: "private transport error", PayloadJSON: "{}"}); err != nil {
			t.Fatal(err)
		}
	}
	consume := func(want int) {
		t.Helper()
		if err := db.ConsumeNotificationEvents(ctx, u.ID, now, Classify); err != nil {
			t.Fatal(err)
		}
		rows, err := db.NotificationDeliveries(ctx, u.ID, 0)
		if err != nil || len(rows) != want {
			t.Fatalf("deliveries=%+v error=%v want=%d", rows, err, want)
		}
	}
	appendEvent(a.ID, "connection_recovered")   // Initial healthy startup does not notify.
	appendEvent(b.ID, "connection_unavailable") // Another user's outage is invisible.
	appendEvent(a.ID, "ws_disconnected")        // Short jitter stays a log only.
	consume(0)
	appendEvent(a.ID, "connection_unavailable")
	consume(1)
	now = now.Add(time.Minute)
	appendEvent(a.ID, "connection_unavailable")
	consume(1)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	appendEvent(a.ID, "connection_recovered")
	consume(2)
	appendEvent(a.ID, "connection_recovered")
	consume(2)
}
