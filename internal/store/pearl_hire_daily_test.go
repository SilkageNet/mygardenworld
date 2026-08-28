package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestPearlHireDailyTicketUsagePersistsAcrossReload(t *testing.T) {
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
	acc, err := db.CreateAccount(ctx, user.ID, "pearl", "ios", "user", "pass")
	if err != nil {
		t.Fatal(err)
	}

	const dayID int32 = 20260828
	if used, err := db.PearlHireTicketUsed(ctx, acc.ID, dayID); err != nil || used != 0 {
		t.Fatalf("initial used=%d err=%v", used, err)
	}
	if used, err := db.IncrementPearlHireTicketUsed(ctx, acc.ID, dayID); err != nil || used != 1 {
		t.Fatalf("increment1 used=%d err=%v", used, err)
	}
	if used, err := db.IncrementPearlHireTicketUsed(ctx, acc.ID, dayID); err != nil || used != 2 {
		t.Fatalf("increment2 used=%d err=%v", used, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if used, err := db.PearlHireTicketUsed(ctx, acc.ID, dayID); err != nil || used != 2 {
		t.Fatalf("reloaded used=%d err=%v", used, err)
	}
	if used, err := db.PearlHireTicketUsed(ctx, acc.ID, 20260829); err != nil || used != 0 {
		t.Fatalf("next day used=%d err=%v", used, err)
	}
}

func TestCountPearlHireTicketSpendsSinceFiltersTips4(t *testing.T) {
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
	acc, err := db.CreateAccount(ctx, user.ID, "pearl", "ios", "user", "pass")
	if err != nil {
		t.Fatal(err)
	}

	dayStart := time.Date(2026, 8, 28, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	events := []struct {
		ts      time.Time
		kind    string
		domain  string
		message string
	}{
		{dayStart.Add(time.Hour), "operation_ack", "", "pearlPlace.hire 完成"},
		{dayStart.Add(90 * time.Minute), "pearl_hire", "basic.pearl.hire", "雇佣劳工成功 槽位=1 劳工=2001 产出=5/次 预计获取珍珠=200"},
		{dayStart.Add(2 * time.Hour), "operation_failed", "basic.pearl.hire", "雇佣劳工 失败: pearlPlace.hire candidate was contested (hireFailCnt=1)"},
		{dayStart.Add(3 * time.Hour), "operation_failed", "basic.pearl.hire", `雇佣劳工 失败: pearlPlace.hire: rpc pearlPlace.hire: server: {"code":"pearl_tips4","msg":"对方已被其他人雇佣"}`},
		{dayStart.Add(-time.Hour), "pearl_hire", "basic.pearl.hire", "雇佣劳工成功 槽位=2 预计获取珍珠=200"},
	}
	for _, ev := range events {
		if _, err := db.LogEvent(ctx, EventLog{
			AccountID:   acc.ID,
			AccountName: acc.Name,
			TS:          ev.ts.UTC(),
			Kind:        ev.kind,
			Domain:      ev.domain,
			Message:     ev.message,
			Level:       "info",
		}); err != nil {
			t.Fatal(err)
		}
	}

	got, err := db.CountPearlHireTicketSpendsSince(ctx, acc.ID, dayStart)
	if err != nil {
		t.Fatal(err)
	}
	if got != 3 {
		t.Fatalf("spent=%d, want 3 (legacy success + new success + hireFailCnt, excluding tips4 and previous day)", got)
	}
}
