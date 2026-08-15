package apiserver

import (
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestPlantableFlowersProto_CDUsesCultivationLevel(t *testing.T) {
	// 花笼流芳 only has a base c_flowerLvl row; CD must scale with lvl.
	flowers := []state.PlantableFlower{
		{FlowerID: 23331, Stock: 1, Lvl: 1},
		{FlowerID: 23331, Stock: 1, Lvl: 11},
		{FlowerID: 23331, Stock: 1, Lvl: 20},
	}
	out := plantableFlowersProto(flowers)
	if len(out) != 3 {
		t.Fatalf("len=%d, want 3", len(out))
	}
	if out[0].GetCdSeconds() != 3300 {
		t.Fatalf("lvl1 cd=%d, want 3300", out[0].GetCdSeconds())
	}
	if out[1].GetCdSeconds() != 2475 {
		t.Fatalf("lvl11 cd=%d, want 2475 (scaled from base, not lvl1 3300)", out[1].GetCdSeconds())
	}
	if out[2].GetCdSeconds() != 1925 {
		t.Fatalf("lvl20 cd=%d, want 1925", out[2].GetCdSeconds())
	}
	if out[0].GetCdSeconds() == out[1].GetCdSeconds() {
		t.Fatal("lvl1 and lvl11 must not share the same maturity CD")
	}
}
