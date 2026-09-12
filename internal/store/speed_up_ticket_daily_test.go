package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSpeedUpTicketDailyPersistAndRecover(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "garden.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	user, err := db.CreateUser(ctx, "owner", "owner@example.com", "secret")
	if err != nil {
		t.Fatal(err)
	}
	acc, err := db.CreateAccount(ctx, user.ID, "speedup-daily", "ios", "speedup@test", "x")
	if err != nil {
		t.Fatal(err)
	}
	dayID := int32(20260831)
	if used, err := db.SpeedUpTicketUsed(ctx, acc.ID, dayID); err != nil || used != 0 {
		t.Fatalf("initial used=%d err=%v", used, err)
	}
	if used, err := db.AddSpeedUpTicketUsed(ctx, acc.ID, dayID, 64); err != nil || used != 64 {
		t.Fatalf("add 64 => %d err=%v", used, err)
	}
	if used, err := db.AddSpeedUpTicketUsed(ctx, acc.ID, dayID, 5); err != nil || used != 69 {
		t.Fatalf("add 5 => %d err=%v", used, err)
	}
	if err := db.SetSpeedUpTicketUsed(ctx, acc.ID, dayID, 100); err != nil {
		t.Fatal(err)
	}
	if used, err := db.SpeedUpTicketUsed(ctx, acc.ID, dayID); err != nil || used != 100 {
		t.Fatalf("set used=%d err=%v", used, err)
	}
}

func TestCountSpeedUpTicketSpendsSinceParsesLandBatches(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "garden.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	user, err := db.CreateUser(ctx, "owner", "owner@example.com", "secret")
	if err != nil {
		t.Fatal(err)
	}
	acc, err := db.CreateAccount(ctx, user.ID, "speedup-recover", "ios", "recover@test", "x")
	if err != nil {
		t.Fatal(err)
	}
	dayStart := time.Date(2026, 8, 31, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	events := []struct {
		ts      time.Time
		message string
	}{
		{dayStart.Add(time.Hour), "usrLand.speedUpBatch 完成 (田地=[1001 1002 1003 1004 1005])"},
		{dayStart.Add(2 * time.Hour), "usrLand.speedUpBatch 完成 (田地=[1001 1002])"},
		{dayStart.Add(-time.Hour), "usrLand.speedUpBatch 完成 (田地=[1001])"},
	}
	for _, ev := range events {
		if _, err := db.LogEvent(ctx, EventLog{
			AccountID:   acc.ID,
			AccountName: acc.Name,
			TS:          ev.ts.UTC(),
			Kind:        "operation_ack",
			Action:      "speed_up",
			Message:     ev.message,
			Level:       "info",
		}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := db.CountSpeedUpTicketSpendsSince(ctx, acc.ID, dayStart)
	if err != nil {
		t.Fatal(err)
	}
	if got != 7 {
		t.Fatalf("recovered=%d, want 7", got)
	}
}
