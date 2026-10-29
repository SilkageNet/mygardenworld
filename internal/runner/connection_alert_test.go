package runner

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

// A redeem-only connection and its displaced-session recoveries must retain
// pause intent. Recovery closes transport incidents, not automation pauses.
func TestConnectionRecoveryPreservesAndReportsAutomationIntent(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "redeem only", true: "automation enabled"}[enabled], func(t *testing.T) {
			_, r, _ := startupCommitFixture(t)
			p := r.Policy()
			p.AutomationEnabled = enabled
			p.Basic.DisplacedSessionReloginEnabled = true
			r.SetPolicy(p)
			raw, err := policycfg.ToJSON(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := r.db.SavePolicyJSON(t.Context(), r.account.ID, raw); err != nil {
				t.Fatal(err)
			}
			r.startSource = StartSourceRedeemAutoConnect
			if err := r.completeStartup(t.Context(), false); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				r.sessionInvalidated, r.sessionAutoRelogin = true, true
				if !r.prepareAutoReloginAttempt() || !r.completeAutoRelogin() {
					t.Fatal("recovery rejected")
				}
				r.emitConnectionRecovered()
				assertStartupIntent(t, r, enabled)
				event := r.bus.recentEvents[len(r.bus.recentEvents)-1]
				var payload struct {
					Enabled bool `json:"automation_enabled"`
				}
				if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
					t.Fatal(err)
				}
				want := "自动化仍暂停"
				if enabled {
					want = "自动化已启用"
				}
				if event.Kind != "connection_recovered" || payload.Enabled != enabled || !strings.Contains(event.Message, want) {
					t.Fatalf("incorrect recovery status: %+v", event)
				}
			}
		})
	}
}

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
