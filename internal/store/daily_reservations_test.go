package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDailyReservationsEnforceCapsAndReset(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	user, err := db.CreateUser(ctx, "daily", "daily@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	acc, err := db.CreateAccountWithPolicy(ctx, user.ID, "daily", "ios", "game", "pw", `{"schema_version":3}`)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		day, count, limit, wantTotal int32
		wantOK                       bool
	}{
		{1, 3, 2, 0, false}, // the first insert must also enforce the cap
		{1, 2, 2, 2, true},
		{1, 1, 2, 0, false},
		{2, 1, 2, 1, true},
	} {
		total, ok, err := db.ReserveSpeedUpTickets(ctx, acc.ID, tc.day, tc.count, tc.limit)
		if err != nil || ok != tc.wantOK || total != tc.wantTotal {
			t.Fatalf("speed reservation %+v: total=%d ok=%v err=%v", tc, total, ok, err)
		}
	}
	got, err := db.SpeedUpTicketsReserved(ctx, acc.ID, 2)
	if err != nil || got != 1 {
		t.Fatalf("speed reservations day 2=%d err=%v", got, err)
	}

	for _, tc := range []struct {
		day    int32
		uid    int64
		wantOK bool
	}{
		{1, 101, true}, {1, 101, false}, {1, 102, true}, {1, 103, false}, {2, 101, true},
	} {
		ok, err := db.ReserveElvesAidHelp(ctx, acc.ID, tc.day, tc.uid, 2)
		if err != nil || ok != tc.wantOK {
			t.Fatalf("aid reservation %+v: ok=%v err=%v", tc, ok, err)
		}
	}
	uids, err := db.ElvesAidHelpReservations(ctx, acc.ID, 1)
	if err != nil || len(uids) != 2 || uids[0] != 101 || uids[1] != 102 {
		t.Fatalf("aid reservations=%v err=%v", uids, err)
	}
}
