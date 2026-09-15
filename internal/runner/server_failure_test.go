package runner

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

func TestServerFailureBurstIsAccountScopedAndBounded(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rpcs    []string
		spacing time.Duration
		want    bool
	}{
		{"one rejection", []string{"usrLand.harvest"}, time.Second, false},
		{"same RPC stays local", []string{"usrLand.harvest", "usrLand.harvest", "usrLand.harvest", "usrLand.harvest"}, time.Second, false},
		{"cross domain burst", []string{"usrLand.harvest", "fmlForest.refresh", "usrLand.harvest"}, 10 * time.Second, true},
		{"expired observations", []string{"usrLand.harvest", "fmlForest.refresh", "pearlPlace.recvOneKey"}, 40 * time.Second, false},
		{"inclusive window", []string{"usrLand.harvest", "fmlForest.refresh", "pearlPlace.recvOneKey"}, 30 * time.Second, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newOperationEventTestRunner()
			now := time.Now()
			failure := babigame.WSResponseD{M: json.RawMessage(`{"code":5000,"args":[]}`)}
			for i, rpc := range tc.rpcs {
				at := now.Add(time.Duration(i) * tc.spacing)
				r.observeGameRPCAt(rpc, failure, at)
				// A successful heartbeat is not proof of healthy business RPCs.
				r.observeGameRPCAt("index.heartbeat", babigame.WSResponseD{}, at)
			}
			s, _ := r.accountSafetySnapshot()
			if (s.RestrictionCode == 5000) != tc.want {
				t.Fatalf("protection=%+v want active=%v", s, tc.want)
			}
			if tc.want {
				for _, rpc := range []string{"index.heartbeat", "usrLand.harvest", "fmlRace.upgradeTask", "index.login"} {
					if err := r.checkGameRPC(rpc); err == nil {
						t.Fatalf("%s bypassed account protection", rpc)
					}
				}
			}
			other := newOperationEventTestRunner()
			if other.checkGameRPC("usrLand.harvest") != nil {
				t.Fatal("one account paused unrelated accounts")
			}
		})
	}
}

func TestServerFailureRecoveryPreservesSafetyRevisionAndPaidFence(t *testing.T) {
	r := newOperationEventTestRunner()
	failure := babigame.WSResponseD{M: json.RawMessage(`{"code":5000}`)}
	now := time.Now()
	for _, rpc := range []string{"usrLand.harvest", "fmlForest.refresh", "pearlPlace.recvOneKey"} {
		r.observeGameRPCAt(rpc, failure, now)
	}
	first, revision := r.accountSafetySnapshot()
	// Responses already in flight do not perpetually extend the pause.
	r.observeGameRPCAt("usrLand.harvest", failure, now.Add(time.Second))
	duplicate, _ := r.accountSafetySnapshot()
	if first != duplicate {
		t.Fatal("duplicate moved cooldown", first, duplicate)
	}
	if err := r.clearAccountRestriction(revision); err == nil {
		t.Fatal("old probe cleared a newer error")
	}
	// A single failed probe after cooldown opens the next backoff immediately.
	probeAt := time.UnixMilli(first.RestrictedUntilMS)
	r.observeGameRPCAt("index.login", failure, probeAt)
	next, revision := r.accountSafetySnapshot()
	if next.RestrictionAttempts != 2 || next.RestrictedUntilMS != probeAt.Add(10*time.Minute).UnixMilli() {
		t.Fatal("failed probe did not back off", next)
	}
	r.raceUpgradeAttempts = map[[2]int64]bool{{42, 1}: true}
	if err := r.clearAccountRestriction(revision); err != nil {
		t.Fatal(err)
	}
	if len(r.serverFailures) != 0 || !r.raceUpgradeAttempts[[2]int64{42, 1}] {
		t.Fatal("recovery retained stale burst or cleared paid mutation fence")
	}
	r.observeGameRPCAt("usrLand.harvest", failure, probeAt)
	if s, _ := r.accountSafetySnapshot(); s.RestrictionCode != 0 {
		t.Fatal("old observations contaminated a healthy recovery")
	}
}

func TestConcurrentServerErrorsDoNotDowngradeExplicitRestriction(t *testing.T) {
	r := newOperationEventTestRunner()
	now := time.Now()
	var wg sync.WaitGroup
	for _, code := range []string{"5000", "97777", "97778", "5000", "5000"} {
		wg.Go(func() {
			r.observeGameRPCAt("index.login", babigame.WSResponseD{M: json.RawMessage(`{"code":` + code + `}`)}, now)
		})
	}
	wg.Wait()
	s, _ := r.accountSafetySnapshot()
	if s.RestrictionCode != 97778 || s.RestrictedUntilMS != now.Add(30*time.Minute).UnixMilli() {
		t.Fatal("explicit protection weakened", s)
	}
}
