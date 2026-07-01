package runner

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestWaterResponseIncludesDrops(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want bool
	}{
		{
			name: "water batch current total",
			raw:  json.RawMessage(`{"7":{"2":{"1":{"7":8},"2":{"7":5}}}}`),
			want: true,
		},
		{
			name: "cold snapshot inventory",
			raw:  json.RawMessage(`{"7":{"0":{"32":{"7":12}}}}`),
			want: true,
		},
		{
			name: "spend count only is not remaining drops",
			raw:  json.RawMessage(`{"7":{"2":{"1":{"7":8}}}}`),
			want: false,
		},
		{
			name: "no water namespace",
			raw:  json.RawMessage(`{"100":{"1":{"1001":{"1":3}}}}`),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := waterResponseIncludesDrops(tc.raw); got != tc.want {
				t.Fatalf("waterResponseIncludesDrops()=%t, want %t", got, tc.want)
			}
		})
	}
}

func TestNextLandUnlockCandidateDoesNotInventNextFourLands(t *testing.T) {
	st := state.New()
	roster := map[string]any{}
	for id := int32(1001); id <= 1024; id++ {
		roster[fmt.Sprint(id)] = map[string]any{}
	}
	st.ApplyVMap(map[string]any{
		"100": map[string]any{"0": map[string]any{"1": roster}},
		"7":   map[string]any{"0": map[string]any{"34": 13, "44": 999999}},
	})

	if id, ok := nextLandUnlockCandidate(st); ok {
		t.Fatalf("nextLandUnlockCandidate()=(%d,true), want no guessed candidate", id)
	}
}

func TestNextLandUnlockCandidateUsesRuntimeLandConfig(t *testing.T) {
	st := state.New()
	roster := map[string]any{}
	for id := int32(1001); id <= 1024; id++ {
		roster[fmt.Sprint(id)] = map[string]any{}
	}
	st.ApplyVMap(map[string]any{
		"100": map[string]any{"0": map[string]any{"1": roster}},
		"7":   map[string]any{"0": map[string]any{"34": 13, "44": 1500}},
	})
	st.SetFarmLands([]state.FarmLandInfo{{ID: 1025, OpenLevel: 13, Cost: []int32{37, 1500}}})

	id, ok := nextLandUnlockCandidate(st)
	if !ok || id != 1025 {
		t.Fatalf("nextLandUnlockCandidate()=(%d,%t), want (1025,true)", id, ok)
	}
}

func TestApplyHarvestBlocksSkipsBlockedSingleLand(t *testing.T) {
	now := time.Now()
	r := &Runner{harvestBlockedUntil: map[int32]time.Time{1002: now.Add(time.Minute)}}
	op := &automation.PlannedOp{
		Kind:    "usrLand.harvest",
		LandIDs: []int32{1002},
	}

	if got := r.applyHarvestBlocks(op, now); got != nil {
		t.Fatalf("applyHarvestBlocks()=%+v, want nil", got)
	}
}

func TestApplyHarvestBlocksDowngradesOneKeyWhenSomeLandsBlocked(t *testing.T) {
	now := time.Now()
	r := &Runner{harvestBlockedUntil: map[int32]time.Time{1002: now.Add(time.Minute)}}
	op := &automation.PlannedOp{
		Kind:    "usrLand.harvestOneKey",
		LandIDs: []int32{1001, 1002, 1003},
	}

	got := r.applyHarvestBlocks(op, now)
	if got == nil {
		t.Fatal("applyHarvestBlocks()=nil, want single harvest op")
	}
	if got.Kind != "usrLand.harvest" {
		t.Fatalf("kind=%q, want usrLand.harvest", got.Kind)
	}
	if len(got.LandIDs) != 1 || got.LandIDs[0] != 1001 {
		t.Fatalf("landIDs=%v, want [1001]", got.LandIDs)
	}
	args, err := operationArgs(got)
	if err != nil {
		t.Fatalf("operationArgs() error: %v", err)
	}
	req, ok := args.(babigame.UsrLandHarvestRequest)
	if !ok {
		t.Fatalf("operationArgs()=%T, want UsrLandHarvestRequest", args)
	}
	if req.LandId != 1001 {
		t.Fatalf("LandId=%v, want 1001", req.LandId)
	}
}

func TestApplyHarvestBlocksIgnoresExpiredBlock(t *testing.T) {
	now := time.Now()
	r := &Runner{harvestBlockedUntil: map[int32]time.Time{1002: now.Add(-time.Second)}}
	op := &automation.PlannedOp{
		Kind:    "usrLand.harvest",
		LandIDs: []int32{1002},
	}

	if got := r.applyHarvestBlocks(op, now); got != op {
		t.Fatalf("applyHarvestBlocks()=%+v, want original op", got)
	}
}

func TestResidentOrderDailyLimitHelpers(t *testing.T) {
	if !isResidentOrderDailyLimit("今日完成订单次数已达上限") {
		t.Fatal("daily limit message was not recognized")
	}
	if isResidentOrderDailyLimit("鲜花数量不足") {
		t.Fatal("unrelated order error was recognized as daily limit")
	}

	loc := time.FixedZone("CST", 8*60*60)
	got := nextLocalDay(time.Date(2026, 5, 21, 8, 19, 50, 0, loc))
	want := time.Date(2026, 5, 22, 0, 5, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("nextLocalDay()=%s, want %s", got, want)
	}
}
