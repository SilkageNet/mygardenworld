package runner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestLoginRiskStopsAllRecoveryAndPersistsDisabledAutomation(t *testing.T) {
	for _, code := range []int{902049, 902050, 902051, 902052, 902053} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			ctx := context.Background()
			db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			u, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
			if err != nil {
				t.Fatal(err)
			}
			a, err := db.CreateAccount(ctx, u.ID, "game", "ios", "u", "p")
			if err != nil {
				t.Fatal(err)
			}
			r := newSessionLifecycleTestRunner(automation.DefaultPolicy())
			r.db, r.account = db, a
			p := r.Policy()
			p.AutomationEnabled = true
			p.Basic.DisplacedSessionReloginEnabled = true
			r.SetPolicy(p)
			raw, err := policycfg.ToJSON(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.SavePolicyJSON(ctx, a.ID, raw); err != nil {
				t.Fatal(err)
			}
			// Simulate a displacement arriving while a fresh login is in flight.
			r.sessionInvalidated, r.sessionAutoRelogin = true, true
			if !r.stopForLoginRisk(fmt.Errorf("login: %w", &babigame.GameLoginError{BizCode: code})) {
				t.Fatal("refusal not handled")
			}
			if !r.sessionInvalidatedWithoutAutoRelogin() || r.Policy().GetAutomationEnabled() || r.prepareAutoReloginAttempt() {
				t.Fatal("automatic recovery remains possible")
			}
			select {
			case <-r.Done():
			default:
				t.Fatal("runner still running")
			}
			stored, err := db.LoadPolicyJSON(ctx, a.ID)
			if err != nil {
				t.Fatal(err)
			}
			persisted, err := policycfg.FromJSON(stored)
			if err != nil {
				t.Fatal(err)
			}
			if persisted.GetAutomationEnabled() {
				t.Fatal("automation not durably disabled")
			}
			r.bus.mu.RLock()
			defer r.bus.mu.RUnlock()
			incidents := 0
			for _, e := range r.bus.recentEvents {
				if e.Kind == "connection_unavailable" && e.Action == "blocked" {
					incidents++
				}
			}
			if incidents != 1 {
				t.Fatalf("connection incidents = %d, want 1", incidents)
			}
		})
	}
}

func TestOrdinaryLoginErrorsDoNotTriggerRiskStop(t *testing.T) {
	for _, err := range []error{nil, context.DeadlineExceeded, errors.New("902049"), &babigame.GameLoginError{BizCode: 123456}} {
		r := newOperationEventTestRunner()
		if r.stopForLoginRisk(err) || r.isSessionInvalidated() {
			t.Fatalf("ordinary error classified as risk: %v", err)
		}
	}
}
