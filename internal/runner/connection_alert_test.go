package runner

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestConnectionRecoveryAlertLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		cancel, maintenance, invalid bool
	}{
		{name: "persistent outage"}, {name: "manual stop", cancel: true},
		{name: "maintenance", maintenance: true}, {name: "displaced session", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := New(babigame.Config{}, nil, &store.Account{ID: 1}, NewBus(), slog.New(slog.NewTextHandler(io.Discard, nil)))
				r.gameGate = &gameGate{}
				events := func() []Event {
					r.bus.mu.RLock()
					defer r.bus.mu.RUnlock()
					return append([]Event(nil), r.bus.recentEvents...)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				stop := r.watchConnectionRecovery(ctx)
				synctest.Wait()
				time.Sleep(59 * time.Second)
				synctest.Wait()
				if len(events()) != 0 {
					t.Fatal("short outage alerted")
				}
				if tc.cancel {
					cancel()
				}
				if tc.maintenance {
					r.gameGate.block()
				}
				if tc.invalid {
					r.mu.Lock()
					r.sessionInvalidated = true
					r.mu.Unlock()
				}
				time.Sleep(time.Second)
				synctest.Wait()
				want := 1
				if tc.cancel || tc.maintenance || tc.invalid {
					want = 0
				}
				if got := events(); len(got) != want {
					t.Fatalf("events=%+v", got)
				}
				stop()
				stop() // Joining twice is safe on all reconnect exits.
				r.emitConnectionRecovered()
				time.Sleep(2 * time.Minute)
				synctest.Wait()
				if got := events(); len(got) != want+1 || got[want].Kind != "connection_recovered" {
					t.Fatalf("late alert after recovery: %+v", got)
				}
			})
		})
	}
}
