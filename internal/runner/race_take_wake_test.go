package runner

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestNextTickIntervalWakesForRaceLead(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	appear := now.Add(3 * time.Second)
	st := state.New()
	st.ApplyVMap(map[string]any{"101": map[string]any{"0": map[string]any{
		"23001": map[string]any{"1": 23001, "2": 1, "4": 2},
	}}})
	st.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":7,"4":3036,"5":%d,"6":[23001],"10":10,"14":0,"15":0}]}}`,
		appear.UnixMilli(),
	)))
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Race.Enabled = true
	p.Union.Race.AutoEnableModules = true
	p.Union.Race.MinTaskScore = 0
	p.Union.Race.TaskTypePriority = map[int32]int32{3036: 5}
	p.DecisionIntervalSeconds = 4
	r := &Runner{state: st, policy: p}

	got := r.nextTickInterval(now)
	want := 3*time.Second - 300*time.Millisecond
	if got != want {
		t.Fatalf("nextTickInterval=%v, want %v (AppearTime-300ms lead)", got, want)
	}
}

func TestNextTickIntervalWakesForRaceTakeCooldown(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	r := &Runner{
		state:  state.New(),
		policy: &pb.Policy{DecisionIntervalSeconds: 4, AutomationEnabled: true},
		operationCooldowns: map[string]operationCooldown{
			"union.race.take": {
				Domain: "union.race.take",
				Until:  now.Add(800 * time.Millisecond),
			},
		},
	}
	got := r.nextTickInterval(now)
	if got != 800*time.Millisecond {
		t.Fatalf("nextTickInterval=%v, want 800ms cooldown until", got)
	}
}

func TestNextTickIntervalWakesForRaceSyncCooldown(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	r := &Runner{
		state:  state.New(),
		policy: &pb.Policy{DecisionIntervalSeconds: 4, AutomationEnabled: true},
		operationCooldowns: map[string]operationCooldown{
			"union.race.sync": {
				Domain: "union.race.sync",
				Until:  now.Add(time.Second),
			},
		},
	}
	got := r.nextTickInterval(now)
	if got != time.Second {
		t.Fatalf("nextTickInterval=%v, want 1s sync cooldown wake", got)
	}
}

func TestRaceTakeRetrySleep(t *testing.T) {
	appear := time.UnixMilli(2_000_000)
	if got := raceTakeRetrySleep(appear.Add(-150*time.Millisecond), appear); got != 150*time.Millisecond {
		t.Fatalf("before appear sleep=%v, want 150ms", got)
	}
	if got := raceTakeRetrySleep(appear, appear); got != raceTakeCDRetryGap {
		t.Fatalf("at appear sleep=%v, want retry gap", got)
	}
	if got := raceTakeRetrySleep(appear.Add(time.Millisecond), appear); got != raceTakeCDRetryGap {
		t.Fatalf("after appear sleep=%v, want retry gap", got)
	}
}

func TestNextTickIntervalWakesImmediatelyForRaceBootstrap(t *testing.T) {
	st := state.New()
	st.ApplyV(json.RawMessage(`{"25":{"1":{"0":999,"1":88},"111":{"1":1},"117":{"5":4}}}`))
	st.MarkFmlRaceTaskPoolStale()
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Race.Enabled = true
	p.DecisionIntervalSeconds = 4
	r := &Runner{state: st, policy: p}
	if got := r.nextTickInterval(time.UnixMilli(1_000_000)); got != minDecisionWake {
		t.Fatalf("nextTickInterval=%v, want immediate bootstrap %v", got, minDecisionWake)
	}
}

func TestNextTickIntervalWaitsForCoolingRaceBootstrap(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	st := state.New()
	st.ApplyV(json.RawMessage(`{"25":{"1":{"0":999,"1":88},"111":{"1":1},"117":{"5":4}}}`))
	st.MarkFmlRaceTaskPoolStale()
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Race.Enabled = true
	p.DecisionIntervalSeconds = 4
	r := &Runner{
		state:  st,
		policy: p,
		operationCooldowns: map[string]operationCooldown{
			"union.race.sync": {
				Domain: "union.race.sync",
				Until:  now.Add(time.Second),
			},
		},
	}

	if got := r.nextTickInterval(now); got != time.Second {
		t.Fatalf("nextTickInterval=%v, want race bootstrap cooldown 1s", got)
	}
}

func TestNextTickIntervalDoesNotBootstrapCompletedRaceWhenAutoModulesOff(t *testing.T) {
	now := time.UnixMilli(1_000_000)
	st := state.New()
	st.ApplyV(json.RawMessage(`{"7":{"0":{"0":999}},"25":{"111":{"1":1},"117":{"5":4},"114":[{"0":5,"4":3036,"7":3,"8":3,"10":24,"12":999}],"110":{"999":{"7":{"0":5,"1":3036,"2":3,"3":3}}}}}`))
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Race.Enabled = true
	p.Union.Race.AutoEnableModules = false
	p.DecisionIntervalSeconds = 4
	r := &Runner{state: st, policy: p}

	if got := r.nextTickInterval(now); got != 4*time.Second {
		t.Fatalf("nextTickInterval=%v, want default interval with race auto modules off", got)
	}
}

func TestNextTickIntervalKeepsDefaultWithoutRace(t *testing.T) {
	r := &Runner{state: state.New(), policy: &pb.Policy{DecisionIntervalSeconds: 4}}
	if got := r.nextTickInterval(time.UnixMilli(1_000_000)); got != 4*time.Second {
		t.Fatalf("nextTickInterval=%v, want 4s", got)
	}
}

func TestNextTickIntervalDoesNotBootstrapOutsideContestWithoutBatch(t *testing.T) {
	// Monday 2026-07-13 is outside Tue–Sun race calendar; bare !Observed enter
	// must not force minDecisionWake.
	st := state.New()
	p := automation.DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Race.Enabled = true
	p.DecisionIntervalSeconds = 4
	r := &Runner{state: st, policy: p}
	monday := time.Date(2026, 7, 13, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	if got := r.nextTickInterval(monday); got != 4*time.Second {
		t.Fatalf("nextTickInterval=%v, want 4s outside contest calendar", got)
	}
}
