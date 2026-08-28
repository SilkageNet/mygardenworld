package automation

import (
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// Reproduces Siri account: 15 guild lands all 绛红报春花 #23307, specified
// flower allowlist excludes 23307, all specified flowers at level 11+.
func TestBuildPlan_UnionLandAutoPlant_SiriSpecifiedFlowersOverOccupied23307(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	plantStart := now.Add(-9 * time.Hour)

	lands := make(map[string]any, 15)
	for i := 1; i <= 15; i++ {
		lands[itoa(i)] = map[string]any{
			"1": map[string]any{
				"0": 0,
				"1": 23307,
				"2": plantStart.UnixMilli(),
				"3": 1, // pending harvest — blocks replace in planner
				"4": 0,
			},
		}
	}

	s := stateForUnionLandTest(t, lands, cultivateAtLevel(11,
		23029, 23031, 23071, 23072, 23073, 23074, 23075, 23076, 23077,
		23122, 23127, 23135, 23136, 23137, 23138, 23139,
		23281, 23282, 23536, 23542, 23565, 23587,
	))

	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union.Land.AutoPlantEnabled = true
	p.Union.Land.HarvestEnabled = true
	p.Union.Land.FlowerIds = []int32{
		23029, 23031, 23071, 23072, 23073, 23074, 23075, 23076, 23077,
		23122, 23127, 23135, 23136, 23137, 23138, 23139,
		23281, 23282, 23536, 23542, 23565, 23587,
	}
	p.Union.Land.MinMaturityMinutes = 20
	p.Union.Land.MinReplantMinutes = 0 // UI 0 → code default 60

	t.Run("all_lands_pending_harvest_wrong_flower", func(t *testing.T) {
		result := BuildPlan(s, p, now)
		var plant *PlannedOp
		for i := range result.Operations {
			if result.Operations[i].Domain == "union.land.plant" {
				plant = &result.Operations[i]
				break
			}
		}
		if plant == nil {
			t.Fatalf("should replace non-allowlisted 23307 even with pending harvest: %+v", result.Operations)
		}
		if plant.FlowerID == 23307 {
			t.Fatalf("should not replant 23307: %+v", plant)
		}
	})

	t.Run("after_harvest_no_pending", func(t *testing.T) {
		// Simulate post-harvest: mature count cleared, harvested incremented.
		for i := 1; i <= 15; i++ {
			lands[itoa(i)] = map[string]any{
				"1": map[string]any{
					"0": 0,
					"1": 23307,
					"2": plantStart.UnixMilli(),
					"3": 0,
					"4": 1,
				},
			}
		}
		s2 := stateForUnionLandTest(t, lands, cultivateAtLevel(11,
			23029, 23031, 23071, 23072, 23073, 23074, 23075, 23076, 23077,
			23122, 23127, 23135, 23136, 23137, 23138, 23139,
			23281, 23282, 23536, 23542, 23565, 23587,
		))
		result := BuildPlan(s2, p, now)
		var plant *PlannedOp
		for i := range result.Operations {
			if result.Operations[i].Domain == "union.land.plant" {
				plant = &result.Operations[i]
				break
			}
		}
		if plant == nil {
			t.Fatalf("expected plant op after harvest window, got: %+v", result.Operations)
		}
		if plant.FlowerID == 23307 {
			t.Fatalf("should not replant non-allowlisted 23307: %+v", plant)
		}
		found := false
		for _, id := range p.Union.Land.FlowerIds {
			if id == plant.FlowerID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("flower %d not in allowlist: %+v", plant.FlowerID, plant)
		}
		if len(plant.LandIDs) == 0 {
			t.Fatalf("expected land IDs to replace: %+v", plant)
		}
		if plant.Kind != clientproto.RPCFmlLandPlant.String() {
			t.Fatalf("kind=%q", plant.Kind)
		}
		if !strings.Contains(plant.Reason, "11") {
			t.Fatalf("reason=%q", plant.Reason)
		}
	})
}

func stateForUnionLandTest(t *testing.T, lands map[string]any, cultivations map[string]any) *state.State {
	t.Helper()
	s := state.New()
	applyMap(t, s, map[string]any{
		"101": map[string]any{"0": cultivations},
		"25":  map[string]any{"102": map[string]any{"1": lands}},
	})
	return s
}
