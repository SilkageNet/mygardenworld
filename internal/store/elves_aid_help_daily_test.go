package store

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestElvesAidHelpDailyPersist(t *testing.T) {
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
	acc, err := db.CreateAccount(ctx, user.ID, "elves-aid-daily", "ios", "elves@test", "x")
	if err != nil {
		t.Fatal(err)
	}
	dayID := int32(20260912)
	if uids, err := db.ElvesAidHelpedUIDs(ctx, acc.ID, dayID); err != nil || len(uids) != 0 {
		t.Fatalf("initial uids=%v err=%v", uids, err)
	}
	got, err := db.AddElvesAidHelpedUID(ctx, acc.ID, dayID, 55)
	if err != nil || len(got) != 1 || got[0] != 55 {
		t.Fatalf("add 55 => %v err=%v", got, err)
	}
	got, err = db.AddElvesAidHelpedUID(ctx, acc.ID, dayID, 55)
	if err != nil || len(got) != 1 {
		t.Fatalf("dup 55 => %v err=%v", got, err)
	}
	got, err = db.AddElvesAidHelpedUID(ctx, acc.ID, dayID, 77)
	if err != nil || len(got) != 2 || got[0] != 55 || got[1] != 77 {
		t.Fatalf("add 77 => %v err=%v", got, err)
	}
	if err := db.SetElvesAidHelpedUIDs(ctx, acc.ID, dayID, []int64{9, 1, 9}); err != nil {
		t.Fatal(err)
	}
	uids, err := db.ElvesAidHelpedUIDs(ctx, acc.ID, dayID)
	if err != nil || len(uids) != 2 || uids[0] != 1 || uids[1] != 9 {
		t.Fatalf("set uids=%v err=%v", uids, err)
	}
}

func TestListElvesAidHelpTargetsSince(t *testing.T) {
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
	acc, err := db.CreateAccount(ctx, user.ID, "elves-aid-recover", "ios", "recover@test", "x")
	if err != nil {
		t.Fatal(err)
	}
	dayStart := time.Date(2026, 9, 12, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	ops := []struct {
		ts   time.Time
		args map[string]any
	}{
		{dayStart.Add(time.Hour), map[string]any{"dstUid": 11}},
		{dayStart.Add(2 * time.Hour), map[string]any{"dstUid": 22}},
		{dayStart.Add(3 * time.Hour), map[string]any{"dstUid": 11}},
		{dayStart.Add(-time.Hour), map[string]any{"dstUid": 99}},
	}
	for _, op := range ops {
		argsJSON, _ := json.Marshal(op.args)
		if _, err := db.ExecContext(ctx,
			`INSERT INTO operation_log(account_id, ts, kind, args_json, result_json) VALUES (?, ?, ?, ?, '{}')`,
			acc.ID, op.ts.UTC(), "flowerElvesAid.helpFrd", string(argsJSON),
		); err != nil {
			t.Fatal(err)
		}
	}
	got, err := db.ListElvesAidHelpTargetsSince(ctx, acc.ID, dayStart)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 11 || got[1] != 22 {
		t.Fatalf("recovered=%v", got)
	}
}
