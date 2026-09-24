package automation

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestElvesPlantSyncsAidBeforePlanting(t *testing.T) {
	s := state.New()
	plant := &pb.PlantPolicy{ElvesPlant: &pb.ElvesPlantPolicy{Enabled: true, MainFlowerId: 1, SecondaryFlowerId: 2}}
	if !elvesPlantBlockedByPendingAid(s, time.Now()) {
		t.Fatal("unobserved aid should gate planting")
	}
	ops := flowerElvesAidOperations(s, plant, time.Now())
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFlowerElvesCheckConvert.String() {
		t.Fatalf("expected aid sync, got %+v", ops)
	}
}
