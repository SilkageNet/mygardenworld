package automation

import (
	"encoding/json"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestUnionMembershipRecoveryBackoff(t *testing.T) {
	s := state.New()
	p := &pb.Policy{AutomationEnabled: true, Union: &pb.UnionPolicy{Land: &pb.UnionLandPolicy{HarvestEnabled: true}}}
	now := time.Now()
	check := func(at time.Time, want bool) {
		t.Helper()
		ops := unionOperations(s, p, at)
		if (len(ops) == 1) != want {
			t.Fatalf("at %v operations=%+v", at, ops)
		}
		if want && (ops[0].Kind != clientproto.RPCFmlEnter.String() || ops[0].PreemptFarm) {
			t.Fatalf("recovery must be read-only, non-urgent: %+v", ops)
		}
	}
	check(now, true)
	for _, delay := range []time.Duration{30 * time.Second, 2 * time.Minute, 5 * time.Minute, 5 * time.Minute} {
		s.MarkFmlMembershipSyncAttemptAt(now)
		check(now.Add(delay-time.Millisecond), false)
		now = now.Add(delay)
		check(now, true)
	}
	s.ApplyVFmlMembership(json.RawMessage(`{"25":{"1":{"1":88},"102":{"1":{}}}}`))
	check(now, false)
	s.MarkNoFmlMembershipAt(now)
	check(now.Add(5*time.Minute-time.Millisecond), false)
	check(now.Add(5*time.Minute), true)
	p.Union.Land.HarvestEnabled = false
	check(now.Add(time.Hour), false)
}

func TestUnionMembershipRecoveryRegisteredAndExecutable(t *testing.T) {
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Race.Enabled = true
	plan := BuildPlan(state.New(), p, time.Now())
	found := false
	for _, op := range plan.Operations {
		if op.Category != CategoryUnion && op.Category != CategoryRace {
			continue
		}
		if op.Kind != clientproto.RPCFmlEnter.String() || !op.Executable {
			t.Fatalf("unsafe/disabled recovery: %+v", op)
		}
		found = true
	}
	if !found {
		t.Fatal("missing recovery")
	}
}

func TestUnconfirmedGuildDoesNotDriveRaceFarmOrSpeedup(t *testing.T) {
	for _, absent := range []bool{false, true} {
		s := raceTakenPlantState(t, 0, 10)
		policy := racePlantPolicy(true)
		policy.Union.Race.AutoGiveUpTask = false
		now := time.UnixMilli(1_500_000)
		if len(raceTaskProgressDemands(s, policy, now)) == 0 || !raceSpeedupEnabledAt(s, policy.Union.Race, now) {
			t.Fatal("fixture must initially drive a real race task")
		}
		if absent {
			s.MarkNoFmlMembershipAt(now)
		} else {
			s.MarkFmlMembershipUncertainAt(now)
		}
		if len(raceTaskProgressDemands(s, policy, now)) != 0 || raceSpeedupEnabledAt(s, policy.Union.Race, now) || raceSuppressesAutoReplant(s, policy, now) {
			t.Fatal("unconfirmed guild still influences farm or spends speedup tickets")
		}
		if len(unionRaceOperations(s, policy.Union.Race, s.RoleID(), now, raceGatesOn())) != 0 {
			t.Fatal("cached race state reopened guild operations")
		}
	}
}
