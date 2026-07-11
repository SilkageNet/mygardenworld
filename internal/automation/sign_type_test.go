package automation

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestSignTypePlanUsesObservedServerStatus(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	cases := []struct {
		name        string
		stateJSON   string
		wantKind    string
		wantBlocked bool
		wantNone    bool
	}{
		{name: "namespace absent", wantBlocked: true},
		{name: "entry absent", stateJSON: `{"140":{"0":{}}}`, wantBlocked: true},
		{name: "malformed status", stateJSON: `{"140":{"0":{"1":{"0":123,"1":1,"3":1,"4":0.5}}}}`, wantBlocked: true},
		{name: "can sign", stateJSON: `{"140":{"0":{"1":{"0":123,"1":1,"3":1,"4":0}}}}`, wantKind: clientproto.RPCSignTypeSign.String()},
		{name: "can receive", stateJSON: `{"140":{"0":{"1":{"0":123,"1":1,"3":1,"4":1}}}}`, wantKind: clientproto.RPCSignTypeRecv.String()},
		{name: "already received", stateJSON: fmt.Sprintf(`{"140":{"0":{"1":{"0":123,"1":1,"3":1,"4":2,"5":%d}}}}`, now.Add(-time.Hour).UnixMilli()), wantNone: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := state.New()
			s.ApplyV(json.RawMessage(`{"7":{"7":{"2":{"0":123,"1":2,"2":2,"3":1780411445819,"4":1778405962518}}}}`))
			if tc.stateJSON != "" {
				s.ApplyV(json.RawMessage(tc.stateJSON))
			}
			policy := DefaultPolicy()
			policy.AutomationEnabled = true
			policy.Basic.Sign.DailyEnabled = true
			plan := BuildPlan(s, policy, now)

			var signOps []PlannedOp
			for _, planned := range plan.Operations {
				if planned.Domain == "basic.sign" {
					signOps = append(signOps, planned)
				}
			}
			if tc.wantNone {
				if len(signOps) != 0 {
					t.Fatalf("received status planned operations: %+v", signOps)
				}
				return
			}
			if len(signOps) != 1 {
				t.Fatalf("sign operations = %+v", signOps)
			}
			got := signOps[0]
			if tc.wantBlocked {
				if got.Executable || got.Status != PlanStatusBlocked || len(got.BlockedReasons) == 0 {
					t.Fatalf("operation should fail closed: %+v", got)
				}
				return
			}
			if got.Kind != tc.wantKind || !got.Executable || got.TargetID != state.SignTypeAntiFraud {
				t.Fatalf("operation = %+v, want kind=%s", got, tc.wantKind)
			}
		})
	}
}

func TestSignTypePlanSparseProgressDoesNotRepeatSign(t *testing.T) {
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"7":{"2":{"0":123,"1":2,"2":2,"3":1780411445819,"4":1778405962518}}}}`))
	s.ApplyV(json.RawMessage(`{"140":{"0":{"1":{"0":123,"1":1,"2":100,"3":1,"4":0,"5":100,"6":1}}}}`))
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Sign.DailyEnabled = true

	if planned := Plan(s, policy, now); planned == nil || planned.Kind != clientproto.RPCSignTypeSign.String() {
		t.Fatalf("initial plan = %+v", planned)
	}
	s.ApplyV(json.RawMessage(fmt.Sprintf(`{"140":{"0":{"1":{"2":%d,"4":1,"5":%d}}}}`, now.Add(-2*time.Minute).UnixMilli(), now.Add(-2*time.Minute).UnixMilli())))
	if planned := Plan(s, policy, now); planned == nil || planned.Kind != clientproto.RPCSignTypeRecv.String() {
		t.Fatalf("after sign plan = %+v", planned)
	}
	s.ApplyV(json.RawMessage(fmt.Sprintf(`{"140":{"0":{"1":{"2":%d,"4":2,"5":%d}}}}`, now.Add(-time.Minute).UnixMilli(), now.Add(-time.Minute).UnixMilli())))
	if planned := Plan(s, policy, now); planned != nil {
		t.Fatalf("after recv plan = %+v, want nil", planned)
	}
}

func TestSignTypePlanHonorsBaseRewardAndCrossDayGates(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 7, 11, 12, 0, 0, 0, location)
	old := now.Add(-24 * time.Hour).UnixMilli()
	today := now.Add(-time.Hour).UnixMilli()
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Sign.DailyEnabled = true

	t.Run("base reward today suppresses signType", func(t *testing.T) {
		s := state.New()
		s.ApplyV(json.RawMessage(fmt.Sprintf(`{"7":{"7":{"2":{"0":123,"1":2,"2":2,"3":%d,"4":1}}},"140":{"0":{"1":{"0":123,"1":1,"3":1,"4":0,"5":%d}}}}`, today, old)))
		assertNoSignTypeOperation(t, BuildPlan(s, policy, now).Operations)
	})

	t.Run("unreceived base reward blocks without rwd calls", func(t *testing.T) {
		s := state.New()
		s.ApplyV(json.RawMessage(fmt.Sprintf(`{"7":{"7":{"2":{"0":123,"1":2,"2":1,"3":%d,"4":1}}},"140":{"0":{"1":{"0":123,"1":1,"3":1,"4":0,"5":%d}}}}`, old, old)))
		plan := BuildPlan(s, policy, now)
		var blocked bool
		for _, planned := range plan.Operations {
			if planned.Domain == "basic.sign" && planned.Status == PlanStatusBlocked {
				blocked = true
			}
			if planned.Kind == "rwd.recv" || planned.Kind == "rwd.setCanRecv" {
				t.Fatalf("unsafe base reward operation planned: %+v", planned)
			}
		}
		if !blocked {
			t.Fatalf("missing base reward block: %+v", plan.Operations)
		}
	})

	t.Run("old terminal syncs once per day", func(t *testing.T) {
		s := state.New()
		s.ApplyV(json.RawMessage(fmt.Sprintf(`{"7":{"7":{"2":{"0":123,"1":2,"2":2,"3":%d,"4":1}}},"140":{"0":{"1":{"0":123,"1":1,"3":1,"4":2,"5":%d}}}}`, old, old)))
		planned := Plan(s, policy, now)
		if planned == nil || planned.Kind != clientproto.RPCSignTypeEnter.String() || planned.Action != "sync" {
			t.Fatalf("cross-day plan = %+v", planned)
		}
		s.MarkSignTypeEnterAttempt(state.SignTypeAntiFraud, now)
		assertNoSignTypeOperation(t, BuildPlan(s, policy, now).Operations)
	})
}

func assertNoSignTypeOperation(t *testing.T, operations []PlannedOp) {
	t.Helper()
	for _, planned := range operations {
		if planned.Domain == "basic.sign" {
			t.Fatalf("unexpected signType operation: %+v", planned)
		}
	}
}
