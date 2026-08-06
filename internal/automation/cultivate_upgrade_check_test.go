package automation

import (
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func flowerUpgradeFixture(t *testing.T, level int32) (*state.State, state.FlowerUpgradeCost) {
	t.Helper()
	cost, ok := state.FlowerUpgradeCostForLevel(23006, level)
	if !ok {
		t.Fatalf("missing upgrade cost for 23006 lvl %d", level)
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{itoa32(cost.ItemID): cost.Count},
			"44": cost.Gold,
		}},
		"101": map[string]any{"0": map[string]any{
			"23006": map[string]any{"1": 23006, "2": level, "4": 2},
		}},
	})
	return s, cost
}

func TestBuildPlan_FlowerUpgradeExecutable(t *testing.T) {
	s, cost := flowerUpgradeFixture(t, 4)
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union = nil
	p.Plant.Cultivate.Enabled = false
	p.Plant.Cultivate.UpgradeEnabled = true
	p.Plant.Cultivate.TargetLevel = 20

	result := BuildPlan(s, p, time.Now())
	var upgrade *PlannedOp
	for i := range result.Operations {
		op := &result.Operations[i]
		if op.Kind == clientproto.RPCCultivateUpgrade.String() {
			upgrade = op
			break
		}
	}
	if upgrade == nil {
		t.Fatalf("expected cultivate.upgrade op, ops=%+v", result.Operations)
	}
	if upgrade.Domain != "farm.upgrade" || upgrade.FeatureID != "plant.upgrade" || upgrade.Label != "鲜花升级" {
		t.Fatalf("upgrade identity mismatch: %+v", upgrade)
	}
	if !upgrade.Executable || upgrade.Status == PlanStatusAdapterMissing || len(upgrade.BlockedReasons) > 0 {
		t.Fatalf("upgrade should be executable, got status=%s exec=%v blocked=%v", upgrade.Status, upgrade.Executable, upgrade.BlockedReasons)
	}
	if upgrade.FlowerID != 23006 || upgrade.GoldCost != cost.Gold || upgrade.ItemCost[cost.ItemID] != cost.Count {
		t.Fatalf("upgrade cost/target mismatch: %+v want gold=%d item=%d x%d", upgrade, cost.Gold, cost.ItemID, cost.Count)
	}
	if upgrade.Count != 4 {
		t.Fatalf("upgrade Count(from-level)=%d, want 4", upgrade.Count)
	}

	planned := Plan(s, p, time.Now())
	if planned == nil || planned.Kind != clientproto.RPCCultivateUpgrade.String() {
		t.Fatalf("Plan() should pick upgrade, got %+v", planned)
	}
}

func TestBuildPlan_FlowerUpgradeRespectsTargetLevel(t *testing.T) {
	s, _ := flowerUpgradeFixture(t, 4)
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union = nil
	p.Plant.Cultivate.UpgradeEnabled = true
	p.Plant.Cultivate.TargetLevel = 4

	for _, op := range BuildPlan(s, p, time.Now()).Operations {
		if op.Kind == clientproto.RPCCultivateUpgrade.String() {
			t.Fatalf("should not upgrade when lvl>=target, got %+v", op)
		}
	}
}

func TestBuildPlan_FlowerUpgradeDisabled(t *testing.T) {
	s, _ := flowerUpgradeFixture(t, 4)
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union = nil
	p.Plant.Cultivate.UpgradeEnabled = false
	p.Plant.Cultivate.TargetLevel = 20

	for _, op := range BuildPlan(s, p, time.Now()).Operations {
		if op.Kind == clientproto.RPCCultivateUpgrade.String() {
			t.Fatalf("upgrade should stay off when upgradeEnabled=false, got %+v", op)
		}
	}
}

func TestBuildPlan_FlowerUpgradeUsesCfgFallback(t *testing.T) {
	// 星垂绮夜 has no per-flower c_flowerLvl rows; cost must come from c_flowerLvlCfg.
	const flowerID int32 = 23590
	cost, ok := state.FlowerUpgradeCostForLevel(flowerID, 9)
	if !ok {
		t.Fatal("missing cfg fallback upgrade cost for 23590 lvl 9")
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{itoa32(cost.ItemID): cost.Count},
			"44": cost.Gold,
		}},
		"101": map[string]any{"0": map[string]any{
			itoa32(flowerID): map[string]any{"1": flowerID, "2": 9, "4": 2},
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union = nil
	p.Plant.Cultivate.Enabled = false
	p.Plant.Cultivate.UpgradeEnabled = true
	p.Plant.Cultivate.TargetLevel = 11

	planned := Plan(s, p, time.Now())
	if planned == nil || planned.Kind != clientproto.RPCCultivateUpgrade.String() {
		t.Fatalf("Plan() should upgrade cfg-fallback flower, got %+v", planned)
	}
	if planned.FlowerID != flowerID || planned.GoldCost != cost.Gold || planned.ItemCost[cost.ItemID] != cost.Count {
		t.Fatalf("upgrade mismatch: %+v want gold=%d item=%d x%d", planned, cost.Gold, cost.ItemID, cost.Count)
	}
}

func TestBuildPlan_FlowerUpgradeStopsAtMaxLevel(t *testing.T) {
	max := state.FlowerMaxLevel()
	if max <= 1 {
		t.Fatalf("FlowerMaxLevel()=%d, want >1", max)
	}
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"22006": 999},
			"44": 999999,
		}},
		"101": map[string]any{"0": map[string]any{
			"23006": map[string]any{"1": 23006, "2": max, "4": 2},
		}},
	})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Union = nil
	p.Plant.Cultivate.UpgradeEnabled = true
	p.Plant.Cultivate.TargetLevel = max + 5

	for _, op := range BuildPlan(s, p, time.Now()).Operations {
		if op.Kind == clientproto.RPCCultivateUpgrade.String() {
			t.Fatalf("should not upgrade past catalog max level %d, got %+v", max, op)
		}
	}
}
