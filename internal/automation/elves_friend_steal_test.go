package automation

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestFriendElvesStealOptInAndSendTimeValidation(t *testing.T) {
	s := state.New()
	plant := &pb.PlantPolicy{ElvesPlant: &pb.ElvesPlantPolicy{FriendUids: []int64{2001}}}
	if op, ok := PlanOneFriendStealElves(s, plant, time.Now()); ok {
		t.Fatalf("disabled feature planned %+v", op)
	}
	plant.ElvesPlant.StealFriendElvesEnabled = true
	now := applyFriendTouchFixture(s, []int64{2001}, map[int64]bool{2001: true}, map[int64]int32{2001: 0})
	op, ok := PlanOneFriendStealElves(s, plant, now)
	if !ok || op.Kind != clientproto.RPCFrdHomeGetFrdHomeInfo.String() {
		t.Fatalf("want friend garden sync, got %+v ok=%v", op, ok)
	}
	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"21":{"0":23331,"1":3,"6":110132,"8":[]}}}}}`))
	op, ok = PlanOneFriendStealElves(s, plant, time.Now())
	if !ok || op.Kind != clientproto.RPCFrdStealSteal.String() || op.Action != "steal_elves" || op.TargetID != 21 || op.ItemID != 110132 {
		t.Fatalf("want elf steal, got %+v ok=%v", op, ok)
	}
	if err := ValidateFriendStealElvesMutation(s, plant, &op, time.Now()); err != nil {
		t.Fatalf("preflight: %v", err)
	}
	s.MarkFriendStealElvesLandUnavailable(2001, 21)
	if err := ValidateFriendStealElvesMutation(s, plant, &op, time.Now()); err == nil {
		t.Fatal("sticky skipped land passed preflight")
	}
}
