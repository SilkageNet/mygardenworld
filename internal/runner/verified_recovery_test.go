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
	r.safety = store.AccountRequestSafety{RestrictionCode: 5000, RestrictionAttempts: 2, RestrictedUntilMS: time.Now().Add(-time.Second).UnixMilli(), FreshLoginAttempted: true, LastFreshLoginMS: 12345}
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
			if wantHealthy && (s.RestrictionAttempts != 0 || s.FreshLoginAttempted || calls != 2) {
				t.Fatalf("bad success: %+v calls=%d", s, calls)
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
	r.SetPolicy(p)
	if r.freshRecoveryEligible(now) || r.reserveFreshRecovery(t.Context(), now) == nil {
		t.Fatal("displacement setting granted 5000 authentication")
	}
	p.Basic.ServerErrorFreshLoginEnabled = true
	p.AutomationEnabled = false
	r.SetPolicy(p)
	if r.freshRecoveryEligible(now) || r.reserveFreshRecovery(t.Context(), now) == nil {
		t.Fatal("paused account authenticated")
	}
	p.AutomationEnabled = true
	r.SetPolicy(p)
	if !r.freshRecoveryEligible(now) {
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
	if !s.FreshLoginAttempted || freshRecoveryAvailable(s, now.Add(time.Hour)) {
		t.Fatal("restart replenished allowance")
	}
	if err := r.clearAccountRestriction(0); err != nil {
		t.Fatal(err)
	}
	s, _ = r.accountSafetySnapshot()
	s.RestrictionCode, s.RestrictionAttempts = 5000, 2
	if freshRecoveryAvailable(s, now.Add(29*time.Minute)) || !freshRecoveryAvailable(s, now.Add(30*time.Minute)) {
		t.Fatal("cross-incident rate limit lost")
	}
}

func TestFreshRecoveryFailsClosedWithoutDurableReservation(t *testing.T) {
	r := recoveryTestRunner()
	r.safety.FreshLoginAttempted = false
	r.safety.LastFreshLoginMS = 0
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.ServerErrorFreshLoginEnabled = true
	r.SetPolicy(p)
	if r.reserveFreshRecovery(t.Context(), time.Now()) == nil {
		t.Fatal("authentication allowed without durable reservation")
	}
	if r.safety.FreshLoginAttempted {
		t.Fatal("failed reservation changed incident")
	}
}

func TestFreshRecoveryFirstCooldownMatchesDurableAdmission(t *testing.T) {
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
		{name: "explicit server wait", change: func(s *store.AccountRequestSafety, _ time.Time) { s.RestrictionCode = 97778 }},
		{name: "other server rejection", change: func(s *store.AccountRequestSafety, _ time.Time) { s.RestrictionCode = 97777 }},
		{name: "allowance consumed", change: func(s *store.AccountRequestSafety, _ time.Time) { s.FreshLoginAttempted = true }},
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
			if got := freshRecoveryAvailable(s, now); got != tc.want {
				t.Fatalf("memory admission=%v want=%v", got, tc.want)
			}
			got, err := r.db.ReserveFreshRecovery(t.Context(), r.account.ID, now.UnixMilli(), freshRecoveryInterval.Milliseconds())
			if err != nil || got != tc.want {
				t.Fatalf("durable admission=%v want=%v err=%v", got, tc.want, err)
			}
		})
	}
}

func TestFailedCachedRecoveryRetainsCooldownBeforeFreshAuthentication(t *testing.T) {
	for _, code := range []int{91102, 12345} {
		r := recoveryTestRunner()
		r.safety.RestrictionAttempts = 1
		r.safety.FreshLoginAttempted = false
		r.safety.LastFreshLoginMS = 0
		r.deferRestrictionProbe(0, rejectedRestore(code))
		s, _ := r.accountSafetySnapshot()
		if freshRecoveryAvailable(s, time.Now()) {
			t.Fatal("expired cache bypassed cooldown")
		}
		if !freshRecoveryAvailable(s, time.UnixMilli(s.RestrictedUntilMS)) {
			t.Fatalf("code %d blocked recovery after cooldown", code)
		}
		if code == 91102 && (s.RestrictionAttempts != 2 || time.Until(time.UnixMilli(s.RestrictedUntilMS)) < 9*time.Minute) {
			t.Fatal("expired cache did not retain backoff", s)
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
