package apiserver

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

func TestIdentityProbeWaitAndDeviceReuse(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		wait time.Duration
	}{
		{"network", errors.New("network"), identityProbeRetryWait},
		{"cancelled", context.Canceled, identityProbeRetryWait},
		{"risk", fmt.Errorf("wrapped: %w", &babigame.GameLoginError{BizCode: 902049}), identityProbeRiskWait},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var g identityProbeGuard
			now := time.Now()
			key := identityProbeKey{1, "ios", "player"}
			first, finish, err := g.begin(key, now)
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := g.begin(key, now.Add(time.Hour)); connect.CodeOf(err) != connect.CodeResourceExhausted {
				t.Fatalf("active probe evicted: %v", err)
			}
			finish(tc.err, now)
			finish(nil, now) // completion must be idempotent
			if _, _, err := g.begin(key, now.Add(tc.wait-time.Nanosecond)); connect.CodeOf(accountLoginError(err)) != connect.CodeResourceExhausted {
				t.Fatalf("lost wait or Connect code: %v", err)
			}
			second, done, err := g.begin(key, now.Add(tc.wait))
			if err != nil || first != second || first.deviceID == "" || first.uuid == "" {
				t.Fatalf("device rotated during retry: %v", err)
			}
			done(nil, now.Add(tc.wait))
			if len(g.entries) != 0 {
				t.Fatal("successful identity retained")
			}
		})
	}
}

func TestIdentityProbeIsolationAndBoundedCache(t *testing.T) {
	var g identityProbeGuard
	now := time.Now()
	devices := make(map[probeDevice]bool)
	for _, key := range []identityProbeKey{{1, "ios", "p"}, {2, "ios", "p"}, {1, "alipay", "p"}, {1, "ios", "q"}} {
		d, finish, err := g.begin(key, now)
		if err != nil || devices[d] {
			t.Fatal("identity crossed scope")
		}
		devices[d] = true
		finish(errors.New("failed"), now)
	}
	for i := len(g.entries); i < identityProbeMaxEntries; i++ {
		_, finish, err := g.begin(identityProbeKey{3, "ios", fmt.Sprint(i)}, now)
		if err != nil {
			t.Fatal(err)
		}
		finish(errors.New("failed"), now)
	}
	key := identityProbeKey{9, "ios", "extra"}
	if _, _, err := g.begin(key, now); connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatal("cache unbounded")
	}
	_, finish, err := g.begin(key, now.Add(identityProbeTTL))
	if err != nil || len(g.entries) != 1 {
		t.Fatalf("expired entries retained: %v", err)
	}
	finish(nil, now)
}

func TestIdentityProbeConcurrentAdmission(t *testing.T) {
	var g identityProbeGuard
	var wg sync.WaitGroup
	var admitted atomic.Int32
	now := time.Now()
	for range 32 {
		wg.Go(func() {
			if _, _, err := g.begin(identityProbeKey{1, "ios", "p"}, now); err == nil {
				admitted.Add(1)
			}
		})
	}
	wg.Wait()
	if admitted.Load() != 1 {
		t.Fatalf("admitted %d probes", admitted.Load())
	}
}

func TestIdentityProbeRejectsBeforeNetwork(t *testing.T) {
	svc := &Services{}
	if _, err := svc.probeAccountIdentity(context.Background(), "ios", "p", "secret"); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal(err)
	}
	ctx := auth.ContextWithIdentity(context.Background(), &auth.Identity{UserID: 1})
	_, finish, err := svc.identityProbes.begin(identityProbeKey{1, "ios", "p"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	defer finish(nil, time.Now())
	if _, err := svc.probeAccountIdentity(ctx, "ios", "p", "different-secret"); connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatal(err)
	}
}
