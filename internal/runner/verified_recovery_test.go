package runner

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

var recoveryReputation = json.RawMessage(`{"7":{"17":{"0":{"1":100}}}}`)

func recoveryTestRunner() *Runner {
	r := newOperationEventTestRunner()
	r.safety = store.AccountRequestSafety{RestrictionCode: 5000, RestrictionAttempts: 2, RestrictedUntilMS: time.Now().Add(-time.Second).UnixMilli(), FreshLoginAttempts: 1, LastFreshLoginMS: 12345}
	return r
}

func TestRecoveryPermitOnlyAllowsCurrentNonSpendingProbes(t *testing.T) {
	r := recoveryTestRunner()
	_, revision := r.accountSafetySnapshot()
	ctx := context.WithValue(t.Context(), recoveryProbeKey{}, recoveryProbePermit{r, revision})
	for _, name := range []string{"usr.lazySync", "reputation.view", "usrLand.harvest", "pearlPlace.hire", "index.heartbeat", "fmlRace.upgradeTask"} {
		want := name == "usr.lazySync" || name == "reputation.view"
		if allowed := r.checkGameRPCContext(ctx, name) == nil; allowed != want {
			t.Fatalf("%s allowed=%v", name, allowed)
		}
		if err := r.checkGameRPC(name); err == nil {
			t.Fatalf("ordinary %s bypassed protection", name)
		}
	}
	other := recoveryTestRunner()
	if other.checkGameRPCContext(ctx, "reputation.view") == nil {
		t.Fatal("permit leaked across accounts")
	}
	r.safetyRevision++
	if r.checkGameRPCContext(ctx, "reputation.view") == nil {
		t.Fatal("stale permit survived new failure")
	}
	r.safetyRevision--
	r.safety.RestrictedUntilMS = time.Now().Add(time.Minute).UnixMilli()
	if r.checkGameRPCContext(ctx, "reputation.view") == nil {
		t.Fatal("permit bypassed cooldown")
	}
}

func TestRecoveryRequiresBusinessSuccessAndFreshEvidence(t *testing.T) {
	for _, scenario := range []string{"healthy", "login baseline with delta", "lazy failure", "reputation 5000", "empty", "late failure", "cancel", "invalidated", "maintenance", "low reputation"} {
		t.Run(scenario, func(t *testing.T) {
			r := recoveryTestRunner()
			if scenario == "low reputation" {
				p := automation.DefaultPolicy()
				p.AutomationEnabled = true
				p.Basic.Reputation.Enabled = true
				p.Basic.Reputation.Threshold = 80
				r.SetPolicy(p)
			}
			r.state.ApplyV(recoveryReputation) // Old evidence must not validate an empty new session.
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var baseline json.RawMessage
			if scenario == "login baseline with delta" {
				baseline = recoveryReputation
			}
			calls := 0
			err := r.verifyRecoveryState(ctx, 0, baseline, func(ctx context.Context, name clientproto.RPCName) (json.RawMessage, error) {
				calls++
				if r.checkGameRPC("usrLand.harvest") == nil {
					t.Fatal("business resumed before verification")
				}
				if err := r.checkGameRPCContext(ctx, name.String()); err != nil {
					t.Fatal(err)
				}
				if name == clientproto.RPCUsrLazySync {
					if scenario == "lazy failure" {
						return nil, errors.New("timeout")
					}
					return nil, nil
				}
				if scenario == "reputation 5000" || scenario == "late failure" {
					d := babigame.WSResponseD{M: json.RawMessage(`{"code":5000}`)}
					r.observeGameRPC(name.String(), d)
					if scenario == "reputation 5000" {
						return nil, &babigame.RPCServerError{Name: name, Envelope: d}
					}
				}
				if scenario == "cancel" {
					cancel()
				}
				if scenario == "invalidated" {
					r.sessionInvalidated = true
				}
				if scenario == "maintenance" {
					r.gameGate = &gameGate{blocked: true}
				}
				if scenario == "low reputation" {
					return json.RawMessage(`{"7":{"17":{"0":{"1":79}}}}`), nil
				}
				if scenario == "empty" || scenario == "login baseline with delta" {
					return json.RawMessage(`{}`), nil
				}
				return recoveryReputation, nil
			})
			wantHealthy := scenario == "healthy" || scenario == "login baseline with delta"
			if (err == nil) != wantHealthy {
				t.Fatalf("err=%v", err)
			}
			if scenario == "low reputation" && (!isReputationGuardError(err) || r.Policy().GetAutomationEnabled()) {
				t.Fatal("recovery bypassed reputation stop", err)
			}
			s, _ := r.accountSafetySnapshot()
			if (s.RestrictionCode == 0) != wantHealthy || s.LastFreshLoginMS != 12345 {
				t.Fatalf("bad protection: %+v", s)
			}
			if wantHealthy && (s.RestrictionAttempts != 0 || s.FreshLoginAttempts != 0 || calls != 2) {
				t.Fatalf("bad success: %+v calls=%d", s, calls)
			}
			if !wantHealthy && s.FreshLoginAttempts != 1 {
				t.Fatal("failed verification replenished the budget", s)
			}
			if scenario == "reputation 5000" && (s.RestrictionAttempts != 3 || time.Until(time.UnixMilli(s.RestrictedUntilMS)) < 19*time.Minute) {
				t.Fatalf("backoff reset: %+v", s)
			}
		})
	}
}

func TestFreshRecoveryRequiresIndependentOptInAndDurableBudget(t *testing.T) {
	_, r, _ := startupCommitFixture(t)
	now := time.Now()
	r.safety = store.AccountRequestSafety{RestrictionCode: 5000, RestrictionAttempts: 1, RestrictedUntilMS: now.Add(-time.Second).UnixMilli()}
	if err := r.db.SaveAccountRestriction(t.Context(), r.account.ID, r.safety); err != nil {
		t.Fatal(err)
	}
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.DisplacedSessionReloginEnabled = true
	p.Basic.ServerErrorFreshLoginMaxAttempts = 1
	r.SetPolicy(p)
	if r.prefersFreshRecovery() || r.reserveFreshRecovery(t.Context(), now) == nil {
		t.Fatal("displacement setting granted 5000 authentication")
	}
	p.Basic.ServerErrorFreshLoginEnabled = true
	p.AutomationEnabled = false
	r.SetPolicy(p)
	if r.prefersFreshRecovery() || r.reserveFreshRecovery(t.Context(), now) == nil {
		t.Fatal("paused account authenticated")
	}
	p.AutomationEnabled = true
	r.SetPolicy(p)
	if !r.prefersFreshRecovery() {
		t.Fatal("explicitly permitted recovery blocked")
	}
	if err := r.reserveFreshRecovery(t.Context(), now); err != nil {
		t.Fatal(err)
	}
	p.AutomationEnabled = false
	r.SetPolicy(p)
	if r.checkFreshRecoveryAuthorization(t.Context()) == nil {
		t.Fatal("pause after reservation did not veto authentication")
	}
	p.AutomationEnabled = true
	r.SetPolicy(p)
	if err := r.reserveFreshRecovery(t.Context(), now.Add(time.Hour)); err == nil {
		t.Fatal("same incident authenticated twice")
	}
	reloaded := newOperationEventTestRunner()
	reloaded.db, reloaded.account = r.db, r.account
	if err := reloaded.loadAccountSafety(t.Context()); err != nil {
		t.Fatal(err)
	}
	s, _ := reloaded.accountSafetySnapshot()
	reloaded.SetPolicy(p)
	if s.FreshLoginAttempts != 1 || reloaded.reserveFreshRecovery(t.Context(), now.Add(time.Hour)) == nil {
		t.Fatal("restart replenished allowance")
	}
	if err := r.clearAccountRestriction(0); err != nil {
		t.Fatal(err)
	}
	s, _ = r.accountSafetySnapshot()
	s.RestrictionCode, s.RestrictionAttempts = 5000, 2
	r.safety = s
	if err := r.db.SaveAccountRestriction(t.Context(), r.account.ID, s); err != nil {
		t.Fatal(err)
	}
	if r.reserveFreshRecovery(t.Context(), now.Add(29*time.Minute)) == nil || r.reserveFreshRecovery(t.Context(), now.Add(30*time.Minute)) != nil {
		t.Fatal("cross-incident rate limit lost")
	}
}

func TestFreshRecoveryFailsClosedWithoutDurableReservation(t *testing.T) {
	r := recoveryTestRunner()
	r.safety.FreshLoginAttempts = 0
	r.safety.LastFreshLoginMS = 0
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.ServerErrorFreshLoginEnabled = true
	r.SetPolicy(p)
	if r.reserveFreshRecovery(t.Context(), time.Now()) == nil {
		t.Fatal("authentication allowed without durable reservation")
	}
	if r.safety.FreshLoginAttempts != 0 {
		t.Fatal("failed reservation changed incident")
	}
}

func TestFreshRecoveryCountSurvivesReplacementAndResetsOnlyAfterVerification(t *testing.T) {
	for _, code := range []int{5000, 97777, 97778} {
		_, r, _ := startupCommitFixture(t)
		now := time.Now()
		p := automation.DefaultPolicy()
		p.AutomationEnabled, p.Basic.ServerErrorFreshLoginEnabled = true, true
		r.SetPolicy(p)
		r.safety = store.AccountRequestSafety{RestrictionCode: code, RestrictionAttempts: 1}
		if err := r.db.SaveAccountRestriction(t.Context(), r.account.ID, r.safety); err != nil {
			t.Fatal(err)
		}
		for attempt := 1; attempt <= 3; attempt++ {
			if err := r.reserveFreshRecovery(t.Context(), now.Add(time.Duration(attempt-1)*freshRecoveryInterval)); err != nil {
				t.Fatal(err)
			}
			if err := r.verifyRecoveryState(t.Context(), 0, nil, func(context.Context, clientproto.RPCName) (json.RawMessage, error) {
				return nil, errors.New("business unavailable")
			}); err == nil {
				t.Fatal("failed probe cleared protection")
			}
			replacement := newOperationEventTestRunner()
			replacement.db, replacement.account = r.db, r.account
			replacement.SetPolicy(p)
			if err := replacement.loadAccountSafety(t.Context()); err != nil {
				t.Fatal(err)
			}
			r = replacement
			if r.safety.FreshLoginAttempts != attempt {
				t.Fatal("replacement reset budget", r.safety)
			}
		}
		if err := r.reserveFreshRecovery(t.Context(), now.Add(3*freshRecoveryInterval)); err == nil {
			t.Fatal("fourth automatic attempt admitted")
		}
		_, revision := r.accountSafetySnapshot()
		if err := r.verifyRecoveryState(t.Context(), revision, nil, func(context.Context, clientproto.RPCName) (json.RawMessage, error) { return recoveryReputation, nil }); err != nil {
			t.Fatal(err)
		}
		got, err := r.db.LoadAccountRequestSafety(t.Context(), r.account.ID)
		if err != nil || got.FreshLoginAttempts != 0 || got.RestrictionCode != 0 || got.LastFreshLoginMS != now.Add(2*freshRecoveryInterval).UnixMilli() {
			t.Fatal("verified recovery did not persist reset and retain spacing", got, err)
		}
	}
}

func TestFreshRecoveryDurableAdmissionBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*store.AccountRequestSafety, time.Time)
		want   bool
	}{
		{name: "first cooldown deadline", want: true},
		{name: "before deadline", change: func(s *store.AccountRequestSafety, now time.Time) {
			s.RestrictedUntilMS = now.Add(time.Millisecond).UnixMilli()
		}},
		{name: "no incident", change: func(s *store.AccountRequestSafety, _ time.Time) { s.RestrictionCode = 0 }},
		{name: "no recorded attempt", change: func(s *store.AccountRequestSafety, _ time.Time) { s.RestrictionAttempts = 0 }},
		{name: "explicit server wait", change: func(s *store.AccountRequestSafety, _ time.Time) { s.RestrictionCode = 97778 }, want: true},
		{name: "other server rejection", change: func(s *store.AccountRequestSafety, _ time.Time) { s.RestrictionCode = 97777 }, want: true},
		{name: "allowance consumed", change: func(s *store.AccountRequestSafety, _ time.Time) { s.FreshLoginAttempts = 1 }},
		{name: "cross incident too soon", change: func(s *store.AccountRequestSafety, now time.Time) {
			s.LastFreshLoginMS = now.Add(-freshRecoveryInterval + time.Millisecond).UnixMilli()
		}},
		{name: "cross incident deadline", change: func(s *store.AccountRequestSafety, now time.Time) {
			s.LastFreshLoginMS = now.Add(-freshRecoveryInterval).UnixMilli()
		}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, r, _ := startupCommitFixture(t)
			now := time.Now()
			s := store.AccountRequestSafety{RestrictionCode: 5000, RestrictionAttempts: 1, RestrictedUntilMS: now.UnixMilli()}
			if tc.change != nil {
				tc.change(&s, now)
			}
			if err := r.db.SaveAccountRestriction(t.Context(), r.account.ID, s); err != nil {
				t.Fatal(err)
			}
			got, err := r.db.ReserveFreshRecovery(t.Context(), r.account.ID, now.UnixMilli(), freshRecoveryInterval.Milliseconds(), 1)
			if err != nil || got != tc.want {
				t.Fatalf("durable admission=%v want=%v err=%v", got, tc.want, err)
			}
		})
	}
}

func TestFailedCachedRecoveryDoesNotInventServerCooldown(t *testing.T) {
	for _, code := range []int{91102, 12345} {
		r := recoveryTestRunner()
		r.safety.RestrictionAttempts = 1
		r.safety.FreshLoginAttempts = 0
		r.safety.LastFreshLoginMS = 0
		before, revision := r.accountSafetySnapshot()
		preserved := r.preserveCachedSession(t.Context(), rejectedRestore(code), revision)
		if preserved != (code != 91102) {
			t.Fatal("cache decision did not follow positive expiry evidence")
		}
		if after, _ := r.accountSafetySnapshot(); after != before {
			t.Fatal("cache failure changed server restriction", after)
		}
	}
}

func TestAutomaticReconnectPreservesAmbiguousPaidFence(t *testing.T) {
	r := recoveryTestRunner()
	r.state.LockPearlHireSession("ambiguous purchase")
	r.state.SkipPearlHireCandidate(2001)
	r.operationCooldowns["basic.pearl.hire.blocked"] = operationCooldown{}
	r.resetFreshSessionAutomationState()
	view := r.state.PearlHire()
	if !view.SessionLocked || len(view.SkippedUIDs) != 1 {
		t.Fatalf("reconnect erased paid state: %+v", view)
	}
	if _, ok := r.operationCooldowns["basic.pearl.hire.blocked"]; !ok {
		t.Fatal("reconnect erased paid fence")
	}
}
