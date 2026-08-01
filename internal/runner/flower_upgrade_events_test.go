package runner

import (
	"strings"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestOpKindDescCultivateUpgrade(t *testing.T) {
	if got := opKindDesc(clientproto.RPCCultivateUpgrade.String()); got != "鲜花升级" {
		t.Fatalf("opKindDesc(upgrade)=%q", got)
	}
}

func TestFlowerUpgradeSuccessMessage(t *testing.T) {
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCCultivateUpgrade.String(),
		FlowerID: 23006,
	}
	got := flowerUpgradeSuccessMessage(op, 4, nil)
	if got != "鲜花升级: "+flowerName(23006)+" lv4-lv5" {
		t.Fatalf("flowerUpgradeSuccessMessage(from only)=%q", got)
	}

	s := state.New()
	s.ApplyVMap(map[string]any{
		"101": map[string]any{"0": map[string]any{
			"23006": map[string]any{"1": 23006, "2": 5, "4": 2},
		}},
	})
	got = flowerUpgradeSuccessMessage(op, 4, s)
	if !strings.Contains(got, "lv4-lv5") {
		t.Fatalf("flowerUpgradeSuccessMessage(with state)=%q", got)
	}
}
