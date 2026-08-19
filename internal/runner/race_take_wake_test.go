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

func TestNextTickIntervalKeepsDefaultWithoutRace(t *testing.T) {
	r := &Runner{state: state.New(), policy: &pb.Policy{DecisionIntervalSeconds: 4}}
	if got := r.nextTickInterval(time.UnixMilli(1_000_000)); got != 4*time.Second {
		t.Fatalf("nextTickInterval=%v, want 4s", got)
	}
}
