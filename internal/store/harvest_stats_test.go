package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestLatestHarvestStatsUsesRecentContiguousWindow(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}

	a1, err := db.CreateAccount(ctx, user.ID, "one", "ios", "u1", "p1")
	if err != nil {
		t.Fatal(err)
	}
	a2, err := db.CreateAccount(ctx, user.ID, "two", "ios", "u2", "p2")
	if err != nil {
		t.Fatal(err)
	}
	insertHarvest(t, db, a1.ID, "2026-05-20T22:00:00Z", `{"7":{"2":{"0":{"2":1,"23001":99}}}}`)
	insertHarvest(t, db, a1.ID, "2026-05-20T23:00:00Z", `{"7":{"2":{"0":{"2":5,"23001":2,"22001":1}}}}`)
	insertHarvest(t, db, a2.ID, "2026-05-20T23:10:00Z", `{"7":{"2":{"0":{"2":3,"23002":4}}}}`)
	insertHarvest(t, db, a1.ID, "2026-05-20T23:20:00Z", `{"error":"鲜花尚未成熟"}`)

	stats, err := db.LatestHarvestStats(ctx, HarvestStatsOptions{RunGap: 30 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if stats.HarvestOps != 2 {
		t.Fatalf("HarvestOps=%d, want 2", stats.HarvestOps)
	}
	if got := stats.WindowStart.Format(time.RFC3339); got != "2026-05-20T23:00:00Z" {
		t.Fatalf("WindowStart=%s", got)
	}
	if got := stats.WindowEnd.Format(time.RFC3339); got != "2026-05-20T23:10:00Z" {
		t.Fatalf("WindowEnd=%s", got)
	}
	if len(stats.Accounts) != 2 {
		t.Fatalf("accounts=%d, want 2", len(stats.Accounts))
	}
	if stats.Accounts[0].Items[0] != (HarvestItemTotal{ItemID: 2, Count: 5}) ||
		stats.Accounts[0].Items[1] != (HarvestItemTotal{ItemID: 22001, Count: 1}) ||
		stats.Accounts[0].Items[2] != (HarvestItemTotal{ItemID: 23001, Count: 2}) {
		t.Fatalf("account one items=%+v", stats.Accounts[0].Items)
	}
	if stats.Accounts[1].Items[0] != (HarvestItemTotal{ItemID: 2, Count: 3}) ||
		stats.Accounts[1].Items[1] != (HarvestItemTotal{ItemID: 23002, Count: 4}) {
		t.Fatalf("account two items=%+v", stats.Accounts[1].Items)
	}
}

func insertHarvest(t *testing.T, db *DB, accountID int64, ts string, result string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO operation_log(account_id, ts, kind, args_json, result_json) VALUES (?, ?, 'usrLand.harvest', '{"landId":1001}', ?)`, accountID, ts, result)
	if err != nil {
		t.Fatal(err)
	}
}
