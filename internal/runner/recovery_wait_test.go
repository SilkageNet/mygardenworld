package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestMissingCacheWaitsForAuthorizationWithoutExtendingRestriction(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "paused", true: "fresh auth disabled"}[enabled], func(t *testing.T) {
			_, r, _ := startupCommitFixture(t)
			p := automation.DefaultPolicy()
			p.AutomationEnabled = enabled
			r.SetPolicy(p)
			r.safety = store.AccountRequestSafety{RestrictionCode: 5000, RestrictionAttempts: 3, RestrictedUntilMS: time.Now().Add(-time.Hour).UnixMilli()}
			before, revision := r.accountSafetySnapshot()
			if err := r.db.SaveAccountRestriction(t.Context(), r.account.ID, before); err != nil {
				t.Fatal(err)
			}
			for range 3 {
				_, err := r.connectStoredOrFresh(t.Context(), "unused", "unused")
				var blocked *recoveryBlockedError
				if !errors.As(err, &blocked) || !blocked.retryAt.IsZero() {
					t.Fatalf("expected local admission wait before any network I/O, got %v", err)
				}
				r.setRecoveryWait(blocked.reason, blocked.retryAt)
			}
			if after, rev := r.accountSafetySnapshot(); after != before || rev != revision {
				t.Fatalf("local refusal changed safety: %+v", after)
			}
			if saved, err := r.db.LoadAccountRequestSafety(t.Context(), r.account.ID); err != nil || saved != before {
				t.Fatal("local refusal changed durable deadline", saved, err)
			}
			d := r.Diagnostics(time.Now())
			if !d.RequestsPaused || d.RequestRetryAtMS != 0 || !strings.Contains(strings.Join(d.BlockedReasons, " "), "未延长服务端冷却") {
				t.Fatalf("misleading recovery status: %+v", d)
			}
		})
	}
}

func TestPausedReconnectDoesNotRetryOrSlideDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := recoveryTestRunner()
		r.recoveryWake = make(chan struct{}, 1)
		r.bus = NewBus()
		events, unsubscribe := r.bus.SubscribeLive(32)
		defer unsubscribe()
		p := automation.DefaultPolicy()
		p.AutomationEnabled = false
		r.SetPolicy(p)
		before, revision := r.accountSafetySnapshot()
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() {
			defer close(done)
			if r.reconnect(ctx, "unused", "unused") != nil {
				t.Error("paused recovery connected")
			}
		}()
		time.Sleep(45 * time.Minute)
		synctest.Wait()
		if after, rev := r.accountSafetySnapshot(); after != before || rev != revision {
			t.Fatal("waiting extended protection", after)
		}
		count := 0
		for len(events) > 0 {
			if event := <-events; event.Kind == "account_recovery_waiting" {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("wanted one admission event, got %d", count)
		}
		cancel()
		<-done
	})
}

func TestRecoveryWaitWakesOnPolicyDeadlineAndCancellation(t *testing.T) {
	for _, trigger := range []string{"policy", "deadline", "cancel"} {
		t.Run(trigger, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := recoveryTestRunner()
				r.recoveryWake = make(chan struct{}, 1)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				var deadline time.Time
				if trigger == "deadline" {
					deadline = time.Now().Add(time.Minute)
				}
				done := make(chan bool, 1)
				go func() { done <- r.waitRecovery(ctx, deadline) }()
				synctest.Wait()
				switch trigger {
				case "policy":
					r.SetPolicy(automation.DefaultPolicy())
				case "deadline":
					time.Sleep(time.Minute)
				case "cancel":
					cancel()
				}
				if got := <-done; got != (trigger != "cancel") {
					t.Fatal("incorrect wake result", got)
				}
			})
		})
	}
}

func TestManualRecoveryIsOneShotWithoutChangingAutomaticPolicy(t *testing.T) {
	_, r, _ := startupCommitFixture(t)
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	r.SetPolicy(p)
	now := time.Now()
	r.safety = store.AccountRequestSafety{RestrictionCode: 5000, RestrictionAttempts: 3, RestrictedUntilMS: now.Add(-time.Minute).UnixMilli(), FreshLoginAttempts: 1, LastFreshLoginMS: now.Add(-time.Hour).UnixMilli()}
	if err := r.db.SaveAccountRestriction(t.Context(), r.account.ID, r.safety); err != nil {
		t.Fatal(err)
	}
	ctx := r.manualRecoveryContext(t.Context())
	if err := r.reserveFreshRecovery(ctx, now); err != nil {
		t.Fatal("explicit manual login incorrectly required automatic opt-in", err)
	}
	if r.Policy().GetBasic().GetServerErrorFreshLoginEnabled() {
		t.Fatal("one-shot authorization became permanent")
	}
	if err := r.checkFreshRecoveryAuthorization(ctx); err != nil {
		t.Fatal("reserved manual request lost its authorization", err)
	}
	if err := r.checkFreshRecoveryAuthorization(t.Context()); err == nil {
		t.Fatal("next background attempt reused manual grant")
	}
	if recoveryTestRunner().manualRecoveryAuthorized(ctx) {
		t.Fatal("manual permit leaked across accounts")
	}
	r.safetyRevision++
	if r.checkFreshRecoveryAuthorization(ctx) == nil {
		t.Fatal("new server rejection failed to revoke manual permit")
	}
}

func TestFreshRecoveryLocalBudgetWaitDoesNotExtendServerDeadline(t *testing.T) {
	for _, consumed := range []bool{false, true} {
		_, r, _ := startupCommitFixture(t)
		p := automation.DefaultPolicy()
		p.AutomationEnabled, p.Basic.ServerErrorFreshLoginEnabled = true, true
		r.SetPolicy(p)
		r.safety = store.AccountRequestSafety{RestrictionCode: 5000, RestrictionAttempts: 2, RestrictedUntilMS: time.Now().Add(-time.Minute).UnixMilli(), LastFreshLoginMS: time.Now().Add(-time.Minute).UnixMilli()}
		if consumed {
			r.safety.FreshLoginAttempts = 3
		}
		before, _ := r.accountSafetySnapshot()
		var blocked *recoveryBlockedError
		if err := r.reserveFreshRecovery(t.Context(), time.Now()); !errors.As(err, &blocked) {
			t.Fatalf("expected local budget wait, got %v", err)
		}
		if blocked.retryAt.IsZero() != consumed {
			t.Fatal("only spacing has an automatic retry deadline", blocked)
		}
		if after, _ := r.accountSafetySnapshot(); after != before {
			t.Fatal("budget wait extended protection")
		}
	}
}

func TestExhaustedFreshRecoveryNeverFallsBackToCache(t *testing.T) {
	for _, code := range []int{5000, 97777, 97778} {
		_, r, _ := startupCommitFixture(t)
		p := automation.DefaultPolicy()
		p.AutomationEnabled, p.Basic.ServerErrorFreshLoginEnabled = true, true
		r.SetPolicy(p)
		r.safety = store.AccountRequestSafety{RestrictionCode: code, RestrictionAttempts: 1, FreshLoginAttempts: 3}
		if err := r.db.SaveAccountRestriction(t.Context(), r.account.ID, r.safety); err != nil {
			t.Fatal(err)
		}
		// A corrupt cache would produce a different error if the route touched it.
		if err := r.db.SaveSession(t.Context(), r.account.ID, []byte("not a session"), nil); err != nil {
			t.Fatal(err)
		}
		_, err := r.connectStoredOrFresh(t.Context(), "unused", "unused")
		var blocked *recoveryBlockedError
		if !errors.As(err, &blocked) || !blocked.retryAt.IsZero() || !strings.Contains(blocked.reason, "3/3") {
			t.Fatal("exhausted recovery used cache or network", err)
		}
		if cache, err := r.db.LoadSession(t.Context(), r.account.ID); err != nil || string(cache) != "not a session" {
			t.Fatal("recovery destroyed cache", err)
		}
	}
}

func TestEveryProtectedCodeRequiresFreshAuthenticationOptIn(t *testing.T) {
	for _, code := range []int{5000, 97777, 97778} {
		_, r, _ := startupCommitFixture(t)
		p := automation.DefaultPolicy()
		p.AutomationEnabled = true
		r.SetPolicy(p)
		r.safety = store.AccountRequestSafety{RestrictionCode: code, RestrictionAttempts: 1}
		_, err := r.connectStoredOrFresh(t.Context(), "unused", "unused")
		var blocked *recoveryBlockedError
		if !errors.As(err, &blocked) || !strings.Contains(blocked.reason, "未允许自动重新登录") {
			t.Fatalf("code %d bypassed opt-in with missing cache: %v", code, err)
		}
	}
}

func TestExplicitLoginIsSynchronousAndNeverHandsFailureToBackground(t *testing.T) {
	for _, tc := range []struct {
		source   StartSource
		activate bool
		want     bool
	}{
		{StartSourceControlPanel, true, true},
		{StartSourceControlPanel, false, true},
		{StartSourceAlipayLogin, false, true},
		{StartSourceAutomationEnable, true, false},
		{StartSourceDaemonRestore, false, false},
		{StartSourceRedeemAutoConnect, false, false},
		{StartSourceManualOperation, false, false},
	} {
		t.Run(string(tc.source)+map[bool]string{true: "/activate", false: "/connect"}[tc.activate], func(t *testing.T) {
			_, existing, _ := startupCommitFixture(t)
			// Invalid URL fails before any network I/O. Reaching the dial proves
			// the cached path was selected despite fresh-auth opt-in/quota/cooldown.
			r := New(babigame.Config{}, existing.db, existing.account, existing.bus, existing.log)
			r.startSource = tc.source
			cache, err := babigame.MarshalSessionJSON(&babigame.Session{DeviceID: "fixture", UUID: "fixture", Session0: "fixture", GameLogin: babigame.GameLoginResult{Token: "fixture"}, AID: 1, GsIdx: 1, RouteToken: "fixture", GsHost: "%"})
			if err != nil {
				t.Fatal(err)
			}
			if err := r.db.SaveSession(t.Context(), r.account.ID, cache, nil); err != nil {
				t.Fatal(err)
			}
			p := automation.DefaultPolicy()
			p.Basic.ServerErrorFreshLoginEnabled = true
			r.SetPolicy(p)
			s := store.AccountRequestSafety{RestrictionCode: 5000, RestrictionAttempts: 3, RestrictedUntilMS: time.Now().Add(time.Hour).UnixMilli(), FreshLoginAttempts: 3, LastFreshLoginMS: time.Now().UnixMilli()}
			if err := existing.db.SaveAccountRestriction(t.Context(), existing.account.ID, s); err != nil {
				t.Fatal(err)
			}
			err = r.start(t.Context(), tc.activate)
			t.Cleanup(r.Stop)
			if tc.want {
				if !errors.Is(err, errWebSocketSessionStart) || r.cancel != nil {
					t.Fatalf("manual attempt did not return dial failure and stop: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if r.Connected() {
				t.Fatal("failed login connected")
			}
			if !r.Policy().GetBasic().GetServerErrorFreshLoginEnabled() {
				t.Fatal("manual login changed automatic policy")
			}
			if after, _ := r.accountSafetySnapshot(); after != s {
				t.Fatal("start cleared durable protection")
			}
		})
	}
}

func TestPauseVetoesAlreadyPlannedCachedRecoveryRPC(t *testing.T) {
	r := recoveryTestRunner()
	ctx := context.WithValue(t.Context(), recoveryAttemptKey{}, r)
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	r.SetPolicy(p)
	if err := r.checkGameRPCContext(ctx, "index.reLogin"); err != nil {
		t.Fatal(err)
	}
	p.AutomationEnabled = false
	r.SetPolicy(p)
	var blocked *recoveryBlockedError
	if err := r.checkGameRPCContext(ctx, "index.reLogin"); !errors.As(err, &blocked) {
		t.Fatal("pause did not veto cached login", err)
	}
}
