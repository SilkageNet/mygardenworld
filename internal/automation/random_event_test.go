package automation

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestRandomEventOperationsIncludeEveryValidEventWithSharedCooldown(t *testing.T) {
	s := state.New()
	s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{"6008":{"0":6008,"1":0,"2":60080101},"6004":{"0":6004,"1":2,"2":60040901}}}}}`))
	ops := randomEventOperations(s)
	if len(ops) != 2 {
		t.Fatalf("operations=%+v, want both captured events", ops)
	}
	var ids []int32
	for _, op := range ops {
		if op.Kind != clientproto.RPCRandomEventDoAffair.String() || !op.Executable || op.Status != PlanStatusManaged {
			t.Fatalf("unsafe claim op=%+v", op)
		}
		if op.CooldownKey != "basic.map_event:claim" {
			t.Fatalf("cooldown key=%q", op.CooldownKey)
		}
		ids = append(ids, op.TargetID)
	}
	if !reflect.DeepEqual(ids, []int32{6004, 6008}) {
		t.Fatalf("event order=%v", ids)
	}
}

func TestRandomEventOperationsFailClosedForUnobservedMalformedAndUnsafeState(t *testing.T) {
	t.Run("unobserved enters", func(t *testing.T) {
		s := state.New()
		ops := randomEventOperations(s)
		if len(ops) != 1 || ops[0].Kind != clientproto.RPCRandomEventEnter.String() || ops[0].CooldownKey != "basic.map_event:sync" {
			t.Fatalf("operations=%+v", ops)
		}
	})

	t.Run("malformed table resyncs", func(t *testing.T) {
		s := state.New()
		s.ApplyV(json.RawMessage(`{"129":{"0":{"1":[]}}}`))
		ops := randomEventOperations(s)
		if len(ops) != 1 || ops[0].Kind != clientproto.RPCRandomEventEnter.String() || !ops[0].Executable || ops[0].CooldownKey != "basic.map_event:sync" {
			t.Fatalf("operations=%+v", ops)
		}
	})

	t.Run("unsafe entry blocks exact target", func(t *testing.T) {
		s := state.New()
		s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{"6004":{"0":6004,"1":3,"2":60040901}}}}}`))
		ops := randomEventOperations(s)
		if len(ops) != 1 || ops[0].TargetID != 6004 || ops[0].Status != PlanStatusBlocked || ops[0].Executable || len(ops[0].BlockedReasons) == 0 {
			t.Fatalf("operations=%+v", ops)
		}
	})
}
