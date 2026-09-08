package runner

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
)

func TestRequestPacingScopesAndBoundaries(t *testing.T) {
	p := newRequestPacer(RequestPacing{})
	now := time.Now()
	p.lastRequest = now
	p.lastScope["shopCultivate.purchase"] = now
	for _, tc := range []struct {
		name        string
		after, want time.Duration
	}{
		{"shopCultivate.buy", 0, 30 * time.Second},
		{"shopCultivate.buyOneKey", 2 * time.Second, 28 * time.Second},
		{"usrLand.harvest", 0, 2 * time.Second},
		{"usrLand.harvest", 2 * time.Second, 0},
		{"shopCultivate.buy", 30 * time.Second, 0},
		{"usr.heartTick", 0, 0},
	} {
		if got := p.delay(tc.name, now.Add(tc.after)); got != tc.want {
			t.Errorf("%s after %s: %s want %s", tc.name, tc.after, got, tc.want)
		}
	}
	p.lastScope["usrLand.harvest"] = now
	if p.delay("usrLand.harvest", now.Add(3*time.Second)) != 5*time.Second {
		t.Fatal("repeated harvest not paced")
	}
}

func TestRequestPacingConcurrentNoBurstAndCancellation(t *testing.T) {
	p := newRequestPacer(RequestPacing{RequestInterval: 10 * time.Millisecond, RepeatInterval: 20 * time.Millisecond, PurchaseInterval: 40 * time.Millisecond})
	guard := func() error { return nil }
	start := time.Now()
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if err := p.wait(context.Background(), "usrLand.harvest", guard); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if time.Since(start) < 60*time.Millisecond {
		t.Fatal("concurrent callers bypassed repeated RPC spacing")
	}
	before := p.lastScope["usrLand.harvest"]
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.wait(ctx, "usrLand.harvest", guard); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if p.lastScope["usrLand.harvest"] != before {
		t.Fatal("cancelled queue reserved a new slot")
	}
	blocked := errors.New("protected")
	if err := p.wait(context.Background(), "usr.heartTick", func() error { return blocked }); !errors.Is(err, blocked) {
		t.Fatal("heartbeat bypassed protection")
	}
}

func TestPacedShopYieldsToOtherWork(t *testing.T) {
	r := newOperationEventTestRunner()
	r.pacer = newRequestPacer(RequestPacing{})
	now := time.Now()
	r.pacer.lastScope["shopCultivate.purchase"] = now
	shop := automation.PlannedOp{Kind: "shopCultivate.buy", Executable: true, Lane: automation.LaneSide}
	farm := automation.PlannedOp{Kind: "usrLand.harvest", Executable: true, Lane: automation.LaneFarm}
	got := r.selectRunnableOperation([]automation.PlannedOp{shop, farm}, now.Add(3*time.Second))
	if got == nil || got.Kind != farm.Kind {
		t.Fatalf("paced shop blocked other work: %+v", got)
	}
}

func TestRequestPacingValidation(t *testing.T) {
	for _, tc := range []struct {
		p     RequestPacing
		valid bool
	}{
		{RequestPacing{}, true},
		{RequestPacing{RequestInterval: -time.Second}, false},
		{RequestPacing{RequestInterval: time.Millisecond}, false},
		{RequestPacing{RepeatInterval: time.Second}, false},
		{RequestPacing{PurchaseInterval: time.Second}, false},
		{RequestPacing{PurchaseInterval: time.Hour}, false},
	} {
		if (tc.p.Validate() == nil) != tc.valid {
			t.Errorf("validation %+v", tc.p)
		}
	}
}
