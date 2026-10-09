package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestManualRecoveryBypassesOnlyAutomaticAdmission(t *testing.T) {
	for _, code := range []int{5000, 97777, 97778} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/automation=%v", code, enabled), func(t *testing.T) {
				_, r, _ := startupCommitFixture(t)
				p := automation.DefaultPolicy()
				p.AutomationEnabled = enabled
				r.SetPolicy(p)
				now := time.Now()
				r.safety = store.AccountRequestSafety{RestrictionCode: code, RestrictionAttempts: 3, RestrictedUntilMS: now.Add(time.Hour).UnixMilli(), FreshLoginAttempts: 3, LastFreshLoginMS: now.UnixMilli()}
				before, _ := r.accountSafetySnapshot()
				if err := r.db.SaveAccountRestriction(t.Context(), r.account.ID, before); err != nil {
					t.Fatal(err)
				}
				ctx := r.manualRecoveryContext(t.Context())
				if !r.waitAccountRestriction(ctx) {
					t.Fatal("manual login waited for automatic cooldown")
				}
				for _, name := range []string{"index.login", "index.reLogin"} {
					if err := r.checkGameRPCContext(ctx, name); err != nil {
						t.Fatal(name, err)
					}
					if err := r.checkGameRPC(name); err == nil {
						t.Fatal("background login bypassed cooldown")
					}
				}
				for _, name := range []string{"usr.lazySync", "reputation.view", "usrLand.harvest", "pearlPlace.hire", "usr.heartTick"} {
					if err := r.checkGameRPCContext(ctx, name); err == nil {
						t.Fatal("manual login unlocked ordinary RPC", name)
					}
				}
				if err := r.reserveFreshRecovery(ctx, now); err != nil {
					t.Fatal(err)
				}
				before.FreshLoginAttempts++
				if got, err := r.db.LoadAccountRequestSafety(t.Context(), r.account.ID); err != nil || got != before {
					t.Fatal("manual reservation cleared protection", got, err)
				}
				if r.Policy().GetAutomationEnabled() != enabled || r.Policy().GetBasic().GetServerErrorFreshLoginEnabled() {
					t.Fatal("manual request changed policy")
				}
				r.gameGate = &gameGate{blocked: true}
				if err := r.checkFreshRecoveryAuthorization(ctx); !errors.Is(err, ErrMaintenance) {
					t.Fatal("manual login bypassed maintenance", err)
				}
				r.gameGate = nil
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				if r.waitAccountRestriction(cancelled) || !errors.Is(r.checkFreshRecoveryAuthorization(cancelled), context.Canceled) {
					t.Fatal("manual login ignored cancellation")
				}
			})
		}
	}
}

func TestManualLoginRequiresCurrentBusinessVerification(t *testing.T) {
	for _, failure := range []bool{false, true} {
		t.Run(fmt.Sprint(failure), func(t *testing.T) {
			r := recoveryTestRunner()
			r.safety.RestrictedUntilMS = time.Now().Add(time.Hour).UnixMilli()
			ctx := r.manualRecoveryContext(t.Context())
			_, revision := r.accountSafetySnapshot()
			err := r.verifyRecoveryState(ctx, revision, nil, func(probeCtx context.Context, name clientproto.RPCName) (json.RawMessage, error) {
				if err := r.checkGameRPCContext(probeCtx, name.String()); err != nil {
					t.Fatal(err)
				}
				if r.checkGameRPCContext(probeCtx, "usrLand.harvest") == nil {
					t.Fatal("probe permit authorized a mutation")
				}
				if failure {
					r.observeGameRPC(name.String(), babigame.WSResponseD{M: json.RawMessage(`{"code":97778}`)})
					if r.checkGameRPCContext(probeCtx, "reputation.view") == nil {
						t.Fatal("new rejection did not revoke manual/probe permit")
					}
					return nil, errors.New("server rejected probe")
				}
				return recoveryReputation, nil
			})
			if (err != nil) != failure || (r.restrictionError() != nil) != failure {
				t.Fatal("incorrect recovery outcome", err)
			}
		})
	}
}

func TestManualCachedLoginFallsBackOnlyForExpiredSession(t *testing.T) {
	r := recoveryTestRunner()
	r.safety.RestrictedUntilMS = time.Now().Add(time.Hour).UnixMilli()
	ctx := r.manualRecoveryContext(t.Context())
	_, revision := r.accountSafetySnapshot()
	for _, code := range []int{91102, 5000, 97777, 97778} {
		err := &babigame.RPCServerError{Name: clientproto.RPCIndexReLogin, Envelope: babigame.WSResponseD{M: json.RawMessage(fmt.Sprintf(`{"code":%d}`, code))}}
		if preserved := r.preserveCachedSession(ctx, err, revision); preserved != (code != 91102) {
			t.Fatalf("code=%d preserved=%v", code, preserved)
		}
	}
}
