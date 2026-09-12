package automation

import (
	"fmt"
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestElvesPlantSkipsMainFlowerWater(t *testing.T) {
	s := state.New()
	now := time.UnixMilli(1_700_000_000_000)
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23517, "1": 1, "2": 1, "7": float64(now.UnixMilli())},
			"1002": map[string]any{"0": 23331, "1": 1, "2": 1, "7": float64(now.UnixMilli())},
		}}},
		"114": map[string]any{"0": 100},
		// Aid observed with nothing to claim so planting/watering is not gated.
		"132": map[string]any{"5": map[string]any{"4": 0}},
	})
	policy := &pb.PlantPolicy{
		Planting: &pb.PlantingPolicy{AutoEnabled: true, MinWaterDrops: 0},
		ElvesPlant: &pb.ElvesPlantPolicy{
			Enabled:           true,
			MainFlowerId:      23517,
			SecondaryFlowerId: 23331,
			MainLandCount:     1,
		},
	}
	ops := farmOps(s, policy, nil, now, false, false, false)
	for _, op := range ops {
		if op.Domain != "farm.water" {
			continue
		}
		for _, id := range op.LandIDs {
			if id == 1001 {
				t.Fatalf("should not water main flower land, ops=%+v", ops)
			}
		}
	}
}

func TestElvesPlantSpeedUpStopsAtCap(t *testing.T) {
	s := state.New()
	now := time.UnixMilli(1_700_000_000_000)
	lands := map[string]any{}
	for i := 0; i < 30; i++ {
		lid := fmt.Sprintf("%d", 1001+i)
		lands[lid] = map[string]any{
			"0": 23331, "1": 2, "2": 1,
			"5": float64(now.UnixMilli() + 60_000),
			"6": 110132,
			"7": float64(now.UnixMilli()),
		}
	}
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": lands}},
	})
	if got := s.ElvesProducedCount(); got < 30 {
		t.Fatalf("produced=%d want >=30", got)
	}
	policy := &pb.Policy{
		AutomationEnabled: true,
		Plant: &pb.PlantPolicy{
			Planting: &pb.PlantingPolicy{SpeedUpTicketMax: 0},
			ElvesPlant: &pb.ElvesPlantPolicy{
				Enabled:           true,
				MainFlowerId:      23517,
				SecondaryFlowerId: 23331,
				UseSpeedUpTicket:  true,
				ElvesSpawnCap:     30,
			},
		},
	}
	if ops := elvesPlantSpeedUpOps(s, policy, now); len(ops) != 0 {
		t.Fatalf("expected no speedup at cap, got %d", len(ops))
	}
}

func TestElvesSecondaryFirstBloomImmediateHarvest(t *testing.T) {
	s := state.New()
	now := time.UnixMilli(1_700_000_000_000)
	// state=3 initial bloom after watering: no elves yet.
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23517, "1": 1, "2": 1, "7": float64(now.UnixMilli())},
			"1002": map[string]any{
				"0": 23331, "1": 3, "2": 1, "3": 0,
				"7": float64(now.UnixMilli() - 1_000),
			},
		}}},
	})
	policy := &pb.PlantPolicy{
		Planting: &pb.PlantingPolicy{AutoHarvestEnabled: false, HarvestDelaySeconds: 0},
		ElvesPlant: &pb.ElvesPlantPolicy{
			Enabled:             true,
			MainFlowerId:        23517,
			SecondaryFlowerId:   23331,
			MainLandCount:       1,
			HarvestDelaySeconds: 300,
		},
	}
	ops := farmOps(s, policy, nil, now, false, false, false)
	found := false
	for _, op := range ops {
		if op.Kind == "usrLand.harvest" && strings.Contains(op.Reason, "花灵副花延迟收获") && op.Executable {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected immediate first-bloom elves harvest, got %+v", ops)
	}
}

func TestElvesSecondarySecondBloomWaitsDelay(t *testing.T) {
	s := state.New()
	plantedAt := int64(1_700_000_000_000)
	now := time.UnixMilli(plantedAt + 60_000) // 60s after mature; delay=300s
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": map[string]any{
			"1002": map[string]any{
				"0": 23331, "1": 2, "2": 1, "3": 1,
				"5": float64(plantedAt),
				"6": 110132,
				"7": float64(plantedAt),
			},
		}}},
	})
	policy := &pb.PlantPolicy{
		Planting: &pb.PlantingPolicy{AutoHarvestEnabled: false, HarvestDelaySeconds: 0},
		ElvesPlant: &pb.ElvesPlantPolicy{
			Enabled:             true,
			MainFlowerId:        23517,
			SecondaryFlowerId:   23331,
			HarvestDelaySeconds: 300,
		},
	}
	ops := farmOps(s, policy, nil, now, false, false, false)
	for _, op := range ops {
		if op.Kind == "usrLand.harvest" {
			t.Fatalf("expected delay hold on second bloom, got %+v", ops)
		}
	}

	later := time.UnixMilli(plantedAt + 301_000)
	ops = farmOps(s, policy, nil, later, false, false, false)
	found := false
	for _, op := range ops {
		if op.Kind == "usrLand.harvest" && strings.Contains(op.Reason, "花灵副花延迟收获") && op.Executable {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected delayed second-bloom harvest, got %+v", ops)
	}
}

// Speed-up into the elves bloom often leaves plantTime (field 7) at the original
// plant tick. Delay must start from that maturity observation, not plantTime.
func TestElvesSecondarySpeedUpBloomWaitsFromReadyNotPlantTime(t *testing.T) {
	s := state.New()
	plantedAt := int64(1_700_000_000_000)
	// Growing after first harvest; plantTime still the original plant tick.
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1002": map[string]any{
				"0": 23331, "1": 2, "2": 1, "3": 1,
				"5": float64(plantedAt + 600_000),
				"7": float64(plantedAt),
			},
		}},
	})
	readyAt := plantedAt + 120_000
	// Speed-up: state=3 with elves, plantTime unchanged.
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1002": map[string]any{
				"0": 23331, "1": 3, "2": 1, "3": 1,
				"5": float64(plantedAt + 600_000),
				"6": 110132,
				"7": float64(plantedAt),
			},
		}},
	})
	land := s.Lands()[1002]
	if land.HarvestableSinceMs < readyAt-5_000 {
		// ApplyV stamps with time.Now(); allow skew vs plantedAt fixture.
		if land.HarvestableSinceMs <= plantedAt {
			t.Fatalf("harvestableSince=%d should not stay at plantTime=%d", land.HarvestableSinceMs, plantedAt)
		}
	}
	policy := &pb.PlantPolicy{
		Planting: &pb.PlantingPolicy{AutoHarvestEnabled: false, HarvestDelaySeconds: 0},
		ElvesPlant: &pb.ElvesPlantPolicy{
			Enabled:             true,
			MainFlowerId:        23517,
			SecondaryFlowerId:   23331,
			HarvestDelaySeconds: 300,
		},
	}
	// Immediately after becoming ready: must hold.
	now := time.UnixMilli(land.HarvestableSinceMs + 60_000)
	ops := farmOps(s, policy, nil, now, false, false, false)
	for _, op := range ops {
		if op.Kind == "usrLand.harvest" {
			t.Fatalf("expected hold 300s after speed-up ready, got %+v", ops)
		}
	}
	later := time.UnixMilli(land.HarvestableSinceMs + 301_000)
	ops = farmOps(s, policy, nil, later, false, false, false)
	found := false
	for _, op := range ops {
		if op.Kind == "usrLand.harvest" && strings.Contains(op.Reason, "花灵副花延迟收获") && op.Executable {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected harvest after delay from ready tick, got %+v since=%d", ops, land.HarvestableSinceMs)
	}
}

func TestElvesPlantHarvestDelayForLand(t *testing.T) {
	p := &pb.ElvesPlantPolicy{
		Enabled:             true,
		MainFlowerId:        23517,
		SecondaryFlowerId:   23331,
		HarvestDelaySeconds: 120,
	}
	first := state.LandView{State: 3, HarvestCnt: 0}
	if d := elvesPlantHarvestDelayForLand(p, first); d != 0 {
		t.Fatalf("first bloom delay=%s want 0", d)
	}
	second := state.LandView{State: 2, HarvestCnt: 1, ElvesID: 110132, NextTimeMs: 1}
	if d := elvesPlantHarvestDelayForLand(p, second); d != 120*time.Second {
		t.Fatalf("second bloom delay=%s want 120s", d)
	}
}

func TestFriendStealElvesSyncsFriendsBeforeSelection(t *testing.T) {
	s := state.New()
	plant := &pb.PlantPolicy{
		ElvesPlant: &pb.ElvesPlantPolicy{
			StealFriendElvesEnabled: true,
			// No friend UIDs yet — picker needs the list first.
		},
	}
	op, ok := PlanOneFriendStealElves(s, plant, time.Now())
	if !ok || op.Kind != clientproto.RPCFrdEnter.String() || op.FeatureID != "plant.friend_steal_elves" {
		t.Fatalf("want frd.enter to populate picker, got %+v ok=%v", op, ok)
	}
}

func TestFriendStealElvesPlansStealElvesFlag(t *testing.T) {
	s := state.New()
	plant := &pb.PlantPolicy{
		ElvesPlant: &pb.ElvesPlantPolicy{
			StealFriendElvesEnabled: true,
			FriendUids:              []int64{2001},
		},
	}
	now := applyFriendTouchFixture(s, []int64{2001}, map[int64]bool{2001: true}, map[int64]int32{2001: 0})
	op, ok := PlanOneFriendStealElves(s, plant, now)
	if !ok || op.Kind != clientproto.RPCFrdHomeGetFrdHomeInfo.String() || op.FeatureID != "plant.friend_steal_elves" {
		t.Fatalf("want enter for elves, got %+v ok=%v", op, ok)
	}

	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"21":{"0":23331,"1":3,"6":110132,"8":[]}}}}}`))
	now = time.Now()
	op, ok = PlanOneFriendStealElves(s, plant, now)
	if !ok || op.Kind != clientproto.RPCFrdStealSteal.String() || op.Action != "steal_elves" || op.TargetID != 21 {
		t.Fatalf("want steal_elves land 21, got %+v ok=%v", op, ok)
	}
	if op.ItemID != 110132 {
		t.Fatalf("want elves ItemID 110132, got %d", op.ItemID)
	}
	if err := ValidateFriendStealElvesMutation(s, plant, &op, now); err != nil {
		t.Fatalf("preflight: %v", err)
	}
}

func TestFriendStealElvesSkipsUnavailableLandAndPicksNext(t *testing.T) {
	s := state.New()
	plant := &pb.PlantPolicy{
		ElvesPlant: &pb.ElvesPlantPolicy{
			StealFriendElvesEnabled: true,
			FriendUids:              []int64{2001},
		},
	}
	now := applyFriendTouchFixture(s, []int64{2001}, map[int64]bool{2001: true}, map[int64]int32{2001: 0})
	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{
		"1020":{"0":23404,"1":3,"6":110156,"7":100,"8":[]},
		"1040":{"0":23404,"1":3,"6":110156,"7":100,"8":[]}
	}}}}`))
	s.MarkFriendStealElvesLandUnavailable(2001, 1020)
	op, ok := PlanOneFriendStealElves(s, plant, now)
	if !ok || op.Kind != clientproto.RPCFrdStealSteal.String() || op.TargetID != 1040 {
		t.Fatalf("want steal land 1040 after skip 1020, got %+v ok=%v", op, ok)
	}
}

func TestFriendStealElvesRespectsSneakMaxAndFriendQuota(t *testing.T) {
	s := state.New()
	plant := &pb.PlantPolicy{
		ElvesPlant: &pb.ElvesPlantPolicy{
			StealFriendElvesEnabled: true,
			FriendUids:              []int64{2001},
		},
	}
	now := applyFriendTouchFixture(s, []int64{2001}, map[int64]bool{2001: true}, map[int64]int32{2001: 0})
	// Cap out daily elf steals ($sneakMax=4).
	s.ApplyV([]byte(fmt.Sprintf(`{"111":{"0":{"0":9001,"3":%d,"7":4}}}`, now.UnixMilli())))
	if op, ok := PlanOneFriendStealElves(s, plant, now); ok {
		t.Fatalf("sneakMax should stop planning, got %+v", op)
	}

	// Reset elves cnt but exhaust per-friend flower-steal quota.
	s.ApplyV([]byte(fmt.Sprintf(`{"111":{"0":{"0":9001,"1":{"2001":10},"3":%d,"7":0}}}`, now.UnixMilli())))
	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"21":{"0":23331,"1":3,"6":110132,"8":[]}}}}}`))
	now = time.Now()
	if op, ok := PlanOneFriendStealElves(s, plant, now); ok {
		t.Fatalf("friend steal quota exhausted should skip, got %+v", op)
	}
}

func TestFriendStealElvesIgnoresOrdinaryIsSteal(t *testing.T) {
	s := state.New()
	plant := &pb.PlantPolicy{
		ElvesPlant: &pb.ElvesPlantPolicy{
			StealFriendElvesEnabled: true,
			FriendUids:              []int64{2001},
		},
	}
	// isSteal=false (no ordinary mature flower), but friend still has elf quota.
	now := applyFriendTouchFixture(s, []int64{2001}, map[int64]bool{2001: false}, map[int64]int32{2001: 0})
	op, ok := PlanOneFriendStealElves(s, plant, now)
	if !ok || op.Kind != clientproto.RPCFrdHomeGetFrdHomeInfo.String() || op.TargetUID != 2001 {
		t.Fatalf("want enter despite isSteal=false, got %+v ok=%v", op, ok)
	}

	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"21":{"0":23331,"1":3,"6":110132,"8":[]}}}}}`))
	now = time.Now()
	op, ok = PlanOneFriendStealElves(s, plant, now)
	if !ok || op.Kind != clientproto.RPCFrdStealSteal.String() || op.Action != "steal_elves" || op.TargetID != 21 {
		t.Fatalf("want steal_elves with isSteal=false, got %+v ok=%v", op, ok)
	}
}

func TestFriendStealElvesReenterAfterPlantingVsIdle(t *testing.T) {
	if got := FriendStealElvesReenterAfter(map[int32]state.LandView{
		1: {FlowerID: 23331, State: 1},
	}); got != friendStealElvesPlantingRefresh {
		t.Fatalf("planting secondary reenter=%s want %s", got, friendStealElvesPlantingRefresh)
	}
	if got := FriendStealElvesReenterAfter(map[int32]state.LandView{
		1: {FlowerID: 23001, State: 3},
	}); got != friendStealElvesIdleEnter {
		t.Fatalf("idle garden reenter=%s want %s", got, friendStealElvesIdleEnter)
	}
	if got := FriendStealElvesReenterAfter(map[int32]state.LandView{
		1: {FlowerID: 23331, State: 3, ElvesID: 110132},
	}); got != friendStealElvesIdleEnter {
		t.Fatalf("elves already visible reenter=%s want %s", got, friendStealElvesIdleEnter)
	}
}

func TestFriendStealElvesPollsPlantingGardenEvery10s(t *testing.T) {
	s := state.New()
	plant := &pb.PlantPolicy{
		ElvesPlant: &pb.ElvesPlantPolicy{
			StealFriendElvesEnabled: true,
			FriendUids:              []int64{2001},
		},
	}
	now := applyFriendTouchFixture(s, []int64{2001}, map[int64]bool{2001: true}, map[int64]int32{2001: 0})
	// Secondary planted, no elves yet → planting in progress.
	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"21":{"0":23331,"1":1,"6":0,"8":[]}}}}}`))
	now = time.Now()

	op, ok := PlanOneFriendStealElves(s, plant, now)
	if ok {
		t.Fatalf("fresh planting visit with no elves should defer, got %+v", op)
	}
	if !s.FriendTouchSkipEnter(2001, now.Add(time.Second)) {
		t.Fatal("expected 10s skip while planting")
	}
	if s.FriendTouchSkipEnter(2001, now.Add(11*time.Second)) {
		t.Fatal("10s skip should expire")
	}

	// After refresh window, re-enter even though visit lands still cached.
	op, ok = PlanOneFriendStealElves(s, plant, now.Add(11*time.Second))
	if !ok || op.Kind != clientproto.RPCFrdHomeGetFrdHomeInfo.String() || op.TargetUID != 2001 {
		t.Fatalf("want re-enter after 10s, got %+v ok=%v", op, ok)
	}
}

func TestFriendStealElvesIdleGardenWaits5Minutes(t *testing.T) {
	s := state.New()
	plant := &pb.PlantPolicy{
		ElvesPlant: &pb.ElvesPlantPolicy{
			StealFriendElvesEnabled: true,
			FriendUids:              []int64{2001},
		},
	}
	now := applyFriendTouchFixture(s, []int64{2001}, map[int64]bool{2001: true}, map[int64]int32{2001: 0})
	// Ordinary flower only → not planting elves.
	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"21":{"0":23001,"1":3,"6":0,"8":[]}}}}}`))
	now = time.Now()
	view := s.FriendTouch(now)
	if friendStealElvesVisitFresh(view, 2001, now) != true {
		t.Fatal("idle visit should be fresh immediately after enter")
	}
	if friendStealElvesVisitFresh(view, 2001, now.Add(5*time.Minute)) {
		t.Fatal("idle visit should expire at 5m so garden is re-entered")
	}

	op, ok := PlanOneFriendStealElves(s, plant, now)
	if ok {
		t.Fatalf("idle visit with no elves should defer, got %+v", op)
	}
	if !s.FriendTouchSkipEnter(2001, now.Add(4*time.Minute)) {
		t.Fatal("expected 5m skip when not planting elves")
	}
	if s.FriendTouchSkipEnter(2001, now.Add(5*time.Minute+time.Second)) {
		t.Fatal("5m skip should expire")
	}
}

func TestFriendStealElvesSlowsAfterElvesVisible(t *testing.T) {
	s := state.New()
	plant := &pb.PlantPolicy{
		ElvesPlant: &pb.ElvesPlantPolicy{
			StealFriendElvesEnabled: true,
			FriendUids:              []int64{2001},
		},
	}
	now := applyFriendTouchFixture(s, []int64{2001}, map[int64]bool{2001: true}, map[int64]int32{2001: 0})
	// Secondary + elvesId already taken by someone else → visible but not stealable.
	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"21":{"0":23331,"1":3,"6":110132,"8":[1]}}}}}`))
	now = time.Now()

	op, ok := PlanOneFriendStealElves(s, plant, now)
	if ok {
		t.Fatalf("taken elves should defer, got %+v", op)
	}
	if !s.FriendTouchSkipEnter(2001, now.Add(4*time.Minute)) {
		t.Fatal("expected 5m skip once elves are already visible")
	}
	if s.FriendTouchSkipEnter(2001, now.Add(5*time.Minute+time.Second)) {
		t.Fatal("5m skip should expire")
	}
	if friendStealElvesVisitFresh(s.FriendTouch(now), 2001, now.Add(11*time.Second)) != true {
		t.Fatal("visit with visible elves should stay fresh past the old 10s planting window")
	}
}
