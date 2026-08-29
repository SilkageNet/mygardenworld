package runner

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestRunnerPearlHireUsageHydratesAndStaysConservativeOnWriteFailure(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.CreateAccount(ctx, user.ID, "main", "ios", "game", "secret")
	if err != nil {
		t.Fatal(err)
	}
	r := New(babigame.Config{}, db, account, NewBus(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	at := time.Date(2026, 8, 29, 12, 0, 0, 0, shanghai)
	dayID := state.PearlHireTicketDayID(at)
	for range 2 {
		if _, err := db.IncrementPearlHireTicketUsed(ctx, account.ID, dayID); err != nil {
			t.Fatal(err)
		}
	}
	r.hydratePearlHireTicketUsage(ctx, at)
	if got := r.state.PearlHireAt(at).TicketUsedToday; got != 2 {
		t.Fatalf("hydrated usage=%d, want 2", got)
	}

	r.notePearlHireTicketUsed(ctx, at)
	if got := r.state.PearlHireAt(at).TicketUsedToday; got != 3 {
		t.Fatalf("usage after persisted spend=%d, want 3", got)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	r.notePearlHireTicketUsed(canceled, at)
	if got := r.state.PearlHireAt(at).TicketUsedToday; got != 4 {
		t.Fatalf("usage after failed persistence=%d, want conservative 4", got)
	}
	if used, err := db.PearlHireTicketUsed(ctx, account.ID, dayID); err != nil || used != 3 {
		t.Fatalf("persisted usage after canceled context=(%d,%v), want 3,nil", used, err)
	}

	r.notePearlHireTicketUsed(ctx, at)
	if got := r.state.PearlHireAt(at).TicketUsedToday; got != 5 {
		t.Fatalf("usage regressed after later database success=%d, want 5", got)
	}
}
