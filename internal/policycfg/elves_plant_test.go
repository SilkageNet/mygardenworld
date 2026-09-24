package policycfg

import (
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestNormalizeElvesPlantRequiresCatalogPair(t *testing.T) {
	pairs := state.FlowerElvesBookPairs()
	if len(pairs) == 0 {
		t.Fatal("flower elf book catalog missing")
	}
	pair := pairs[0]
	for _, tc := range []struct {
		name            string
		main, secondary int32
		wantEnabled     bool
	}{
		{"valid", pair.MainFlowerID, pair.SecondaryFlower, true},
		{"same flower", pair.MainFlowerID, pair.MainFlowerID, false},
		{"unknown pair", 999999, 999998, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &pb.Policy{Plant: &pb.PlantPolicy{ElvesPlant: &pb.ElvesPlantPolicy{
				Enabled: true, MainFlowerId: tc.main, SecondaryFlowerId: tc.secondary,
				MainLandCount: -3, ElvesSpawnCap: -1, HarvestDelaySeconds: -5,
			}}}
			got := Normalize(p).GetPlant().GetElvesPlant()
			if got.GetEnabled() != tc.wantEnabled || got.GetMainLandCount() != 0 || got.GetElvesSpawnCap() != 0 || got.GetHarvestDelaySeconds() != 0 {
				t.Fatalf("normalized elves plant=%+v", got)
			}
			if !p.GetPlant().GetElvesPlant().GetEnabled() {
				t.Fatal("normalization mutated input")
			}
		})
	}
}

func TestNormalizeFriendElvesTargets(t *testing.T) {
	p := &pb.Policy{Plant: &pb.PlantPolicy{ElvesPlant: &pb.ElvesPlantPolicy{
		StealFriendElvesEnabled: true, FriendUids: []int64{42, -1, 7, 42, 0},
	}}}
	got := Normalize(p).GetPlant().GetElvesPlant()
	if !got.GetStealFriendElvesEnabled() || len(got.GetFriendUids()) != 2 || got.FriendUids[0] != 7 || got.FriendUids[1] != 42 {
		t.Fatalf("normalized friend elf targets=%+v", got)
	}
	if len(p.GetPlant().GetElvesPlant().GetFriendUids()) != 5 {
		t.Fatal("input mutated")
	}
}
