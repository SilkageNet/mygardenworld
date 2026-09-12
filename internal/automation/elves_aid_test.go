package automation

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestFlowerElvesAidReceiveAndRequest(t *testing.T) {
	s := state.New()
	now := time.UnixMilli(1_789_178_000_000)
	// Aid toggles must run with auto-plant disabled.
	policy := &pb.PlantPolicy{
		ElvesPlant: &pb.ElvesPlantPolicy{
			Enabled:    false,
			ReceiveAid: true,
			RequestAid: true,
		},
	}

	ops := flowerElvesAidOperations(s, policy, now)
	if len(ops) == 0 || ops[0].Kind != clientproto.RPCFlowerElvesCheckConvert.String() {
		t.Fatalf("expected sync first, got %+v", ops)
	}

	applyMap(t, s, map[string]any{
		"132": map[string]any{"5": map[string]any{
			"1": map[string]any{"7": float64(now.UnixMilli())},
			"2": float64(now.Add(-time.Minute).UnixMilli()),
			"4": 1,
		}},
	})
	ops = flowerElvesAidOperations(s, policy, now)
	if len(ops) == 0 || ops[0].Kind != clientproto.RPCFlowerElvesAidRecvAidEff.String() || ops[0].FeatureID != "plant.elves_aid_receive" {
		t.Fatalf("expected recv, got %+v", ops)
	}

	// reqAid may clear once helpers filled; still claim.
	applyMap(t, s, map[string]any{
		"132": map[string]any{"5": map[string]any{
			"4": 0,
		}},
	})
	ops = flowerElvesAidOperations(s, policy, now)
	if len(ops) == 0 || ops[0].Kind != clientproto.RPCFlowerElvesAidRecvAidEff.String() {
		t.Fatalf("expected recv after reqAid cleared, got %+v", ops)
	}

	applyMap(t, s, map[string]any{
		"132": map[string]any{"5": map[string]any{
			"1": map[string]any{},
			"2": float64(now.Add(-3 * time.Hour).UnixMilli()),
			"3": 0,
			"4": 0,
		}},
	})
	ops = flowerElvesAidOperations(s, policy, now)
	foundReq := false
	for _, op := range ops {
		if op.Kind == clientproto.RPCFlowerElvesAidReqAid.String() && op.FeatureID == "plant.elves_aid_request" {
			foundReq = true
		}
	}
	if !foundReq {
		t.Fatalf("expected reqAid, got %+v", ops)
	}
}

func TestFlowerElvesAidIgnoresPlantEnabled(t *testing.T) {
	s := state.New()
	now := time.UnixMilli(1_789_178_000_000)
	applyMap(t, s, map[string]any{
		"132": map[string]any{"5": map[string]any{
			"1": map[string]any{"7": float64(now.UnixMilli())},
			"2": float64(now.Add(-time.Minute).UnixMilli()),
			"4": 1,
		}},
	})
	ops := flowerElvesAidOperations(s, &pb.PlantPolicy{
		ElvesPlant: &pb.ElvesPlantPolicy{Enabled: false, ReceiveAid: true},
		Elves:      &pb.FlowerElvesPolicy{Enabled: false},
	}, now)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFlowerElvesAidRecvAidEff.String() {
		t.Fatalf("aid must not depend on plant/elves enabled, got %+v", ops)
	}
	ops = flowerElvesAidOperations(s, &pb.PlantPolicy{
		ElvesPlant: &pb.ElvesPlantPolicy{Enabled: true, MainFlowerId: 1, SecondaryFlowerId: 2},
	}, now)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFlowerElvesAidRecvAidEff.String() || ops[0].Priority != elvesPlantAidGatePriority {
		t.Fatalf("auto plant must still claim pending aid before planting, got %+v", ops)
	}
}

func TestElvesPlantWaitsForAidClaim(t *testing.T) {
	s := state.New()
	now := time.UnixMilli(1_700_000_000_000)
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": map[string]any{
			"1001": map[string]any{},
			"1002": map[string]any{},
		}}},
		"132": map[string]any{"5": map[string]any{
			"1": map[string]any{"9": float64(now.UnixMilli())},
			"2": float64(now.Add(-time.Minute).UnixMilli()),
			"4": 1,
		}},
	})
	policy := &pb.Policy{
		AutomationEnabled: true,
		Plant: &pb.PlantPolicy{
			Planting: &pb.PlantingPolicy{AutoEnabled: false, MinWaterDrops: 0},
			ElvesPlant: &pb.ElvesPlantPolicy{
				Enabled:           true,
				MainFlowerId:      23517,
				SecondaryFlowerId: 23331,
				MainLandCount:     1,
				UseSpeedUpTicket:  true,
			},
		},
	}
	ops := PlanOperations(s, policy, now)
	if len(ops) == 0 || ops[0].Kind != clientproto.RPCFlowerElvesAidRecvAidEff.String() {
		t.Fatalf("expected claim before plant, got %+v", ops)
	}
	for _, op := range ops {
		if op.Domain == "farm.plant" || op.Domain == "farm.water" || op.FeatureID == "plant.elves_plant_speed_up" {
			t.Fatalf("plant/water/speedup must wait for aid claim, got %+v", ops)
		}
	}

	applyMap(t, s, map[string]any{
		"132": map[string]any{"5": map[string]any{"1": map[string]any{}, "3": float64(now.Add(time.Hour).UnixMilli()), "4": 0}},
	})
	ops = farmOps(s, policy.GetPlant(), nil, now, false, false, false)
	foundPlant := false
	for _, op := range ops {
		if op.Domain == "farm.plant" && op.Executable {
			foundPlant = true
		}
	}
	if !foundPlant {
		t.Fatalf("after claim cleared, expected plant ops, got %+v", ops)
	}
}

func TestElvesPlantContinuesWhileAidBuffActive(t *testing.T) {
	s := state.New()
	now := time.UnixMilli(1_700_000_000_000)
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": map[string]any{
			"1001": map[string]any{},
			"1002": map[string]any{},
		}}},
		"132": map[string]any{"5": map[string]any{
			"3": float64(now.Add(2 * time.Hour).UnixMilli()),
			"4": 0,
		}},
	})
	policy := &pb.PlantPolicy{
		Planting: &pb.PlantingPolicy{AutoEnabled: false, MinWaterDrops: 0},
		ElvesPlant: &pb.ElvesPlantPolicy{
			Enabled:           true,
			MainFlowerId:      23517,
			SecondaryFlowerId: 23331,
			MainLandCount:     1,
		},
	}
	if elvesPlantBlockedByPendingAid(s, now) {
		t.Fatal("active aid buff must not block planting")
	}
	ops := farmOps(s, policy, nil, now, false, false, false)
	foundPlant := false
	for _, op := range ops {
		if op.Domain == "farm.plant" && op.Executable {
			foundPlant = true
		}
	}
	if !foundPlant {
		t.Fatalf("expected plant while aid buff active, got %+v", ops)
	}
}

func TestFlowerElvesAidHelpFriend(t *testing.T) {
	s := state.New()
	p := &pb.ElvesPlantPolicy{Enabled: false, HelpFriend: true}

	op, ok := PlanOneFlowerElvesAidHelp(s, p, time.Now())
	if !ok || op.Kind != clientproto.RPCFrdEnter.String() {
		t.Fatalf("expected friend sync, got ok=%v op=%+v", ok, op)
	}

	now := applyFriendTouchFixture(s, []int64{55}, map[int64]bool{55: false}, map[int64]int32{55: 0})
	s.ApplyV([]byte(`{"110":{"1":{"55":{"0":0,"1":1}}}}`))
	now = time.Now()

	op, ok = PlanOneFlowerElvesAidHelp(s, p, now)
	if !ok || op.Kind != clientproto.RPCFlowerElvesAidHelpFrd.String() || op.TargetUID != 55 || op.FeatureID != "plant.elves_aid_help" {
		t.Fatalf("expected helpFrd, got ok=%v op=%+v", ok, op)
	}

	s.NoteFlowerElvesAidHelped(55, now)
	if _, ok := PlanOneFlowerElvesAidHelp(s, p, now); ok {
		t.Fatal("already helped today should not re-plan help")
	}
}
