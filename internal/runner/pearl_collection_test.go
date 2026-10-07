package runner

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestPearlCollectionPacingAndPolicyUpdates(t *testing.T) {
	r := newOperationEventTestRunner()
	r.pacer = newRequestPacer(RequestPacing{})
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Pearl.CollectEnabled = true
	r.SetPolicy(p)
	rpc := clientproto.RPCPearlPlaceRecvOneKey.String()
	if err := r.pacer.wait(context.Background(), rpc, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	at := r.pacer.lastScope[rpc]
	for _, tc := range []struct{ after, want time.Duration }{{0, 300 * time.Second}, {299 * time.Second, time.Second}, {300 * time.Second, 0}} {
		if got := r.pacer.delay(rpc, at.Add(tc.after)); got != tc.want {
			t.Fatalf("delay=%s want %s", got, tc.want)
		}
	}
	p.Basic.Pearl.CollectIntervalSeconds = 600
	r.SetPolicy(p)
	if got := r.pacer.delay(rpc, at.Add(300*time.Second)); got != 300*time.Second {
		t.Fatal("interval edit reset or ignored previous attempt", got)
	}
	p.Basic.Pearl.CollectEnabled = false
	r.SetPolicy(p)
	p.Basic.Pearl.CollectEnabled = true
	r.SetPolicy(p)
	other := newOperationEventTestRunner()
	other.pacer = r.pacer // Manager retains the same per-account pacer across runner replacement.
	other.SetPolicy(p)
	if got := other.pacer.delay(rpc, at.Add(300*time.Second)); got != 300*time.Second {
		t.Fatal("runner replacement reset pacing", got)
	}
	claim := automation.PlannedOp{OperationID: rpc, Kind: rpc, Executable: true, Lane: automation.LaneSide}
	farm := automation.PlannedOp{OperationID: "farm", Kind: "usrLand.harvest", Executable: true, Lane: automation.LaneFarm}
	assertSelectedOperation(t, other.selectRunnableOperation([]automation.PlannedOp{claim, farm}, at.Add(300*time.Second)), "farm")
	if len(other.sideLaneFirstWait) != 0 {
		t.Fatal("intentional collection interval reported as scheduler starvation")
	}
	assertSelectedOperation(t, other.selectRunnableOperation([]automation.PlannedOp{claim}, at.Add(600*time.Second)), rpc)
}

func TestPearlCollectionDoesNotParkScheduledLoop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := newRequestPacer(RequestPacing{})
		rpc := clientproto.RPCPearlPlaceRecvOneKey.String()
		p.lastScope[rpc] = time.Now()
		ctx := context.WithValue(context.Background(), scheduledOperationKey{}, true)
		before := time.Now()
		err := p.wait(ctx, rpc, func() error { return nil })
		if err == nil || !strings.Contains(err.Error(), "收取间隔未到") || time.Now() != before {
			t.Fatal("scheduled loop blocked", err)
		}
		if p.lastScope[rpc] != before {
			t.Fatal("deferred operation consumed a new interval")
		}
		if err := p.wait(context.Background(), rpc, func() error { return nil }); err == nil || time.Now() != before {
			t.Fatal("manual collection blocked the shared operation lock", err)
		}
	})
}

func TestPearlCollectionLocalDeferralIsNotServerFailure(t *testing.T) {
	r := newOperationEventTestRunner()
	r.bus = NewBus()
	events, unsubscribe := r.bus.SubscribeLive(10)
	defer unsubscribe()
	op := &automation.PlannedOp{Kind: clientproto.RPCPearlPlaceRecvOneKey.String(), Domain: "basic.pearl.place", Lane: automation.LaneSide}
	before := r.setSideOperationCooldown(op, time.Now(), errors.New("previous real failure"), "", time.Minute)
	err := r.handleOperationError(context.Background(), operationResult{operationAttempt: operationAttempt{op: op}, err: &pearlCollectDeferredError{reason: "收取间隔未到"}, finishedAt: time.Now()})
	if err != nil {
		t.Fatal("local wait became runtime error", err)
	}
	if after, _ := r.operationCoolingDown(op, time.Now()); after != before {
		t.Fatal("local wait extended or cleared previous failure", after)
	}
	if event := <-events; event.Kind != "operation_deferred" || event.Level != "info" {
		t.Fatal(event)
	}
	if s, _ := r.accountSafetySnapshot(); s.RestrictionCode != 0 {
		t.Fatal("local wait opened account protection")
	}
}

func TestPearlCollectionSendGuard(t *testing.T) {
	for _, tc := range []struct {
		name                                     string
		automation, collect, ready, manual, want bool
	}{
		{"enabled", true, true, true, false, true},
		{"collection disabled", true, false, true, false, false},
		{"automation paused", false, true, true, false, false},
		{"state changed", true, true, false, false, false},
		{"explicit manual does not require automatic opt-in", false, false, true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newOperationEventTestRunner()
			if tc.ready {
				r.state = newReadyPearlRunnerState()
			}
			p := automation.DefaultPolicy()
			p.AutomationEnabled = tc.automation
			p.Basic.Pearl.CollectEnabled = tc.collect
			r.SetPolicy(p)
			ctx := context.Background()
			if !tc.manual {
				ctx = context.WithValue(ctx, scheduledOperationKey{}, true)
			}
			err := r.beforeGameRPC(ctx, clientproto.RPCPearlPlaceRecvOneKey.String())
			if (err == nil) != tc.want {
				t.Fatalf("guard err=%v want allowed=%v", err, tc.want)
			}
		})
	}
}
