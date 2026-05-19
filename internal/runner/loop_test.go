package runner

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
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

func TestApplyHarvestBlocksSkipsBlockedSingleLand(t *testing.T) {
	now := time.Now()
	r := &Runner{harvestBlockedUntil: map[int32]time.Time{1002: now.Add(time.Minute)}}
	op := &automation.PlannedOp{
		Kind:    "usrLand.harvest",
		LandIDs: []int32{1002},
		Args:    map[string]any{"landId": 1002},
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
		Args:    map[string]any{},
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
	if got.Args["landId"] != 1001 {
		t.Fatalf("landId arg=%v, want 1001", got.Args["landId"])
	}
}

func TestApplyHarvestBlocksIgnoresExpiredBlock(t *testing.T) {
	now := time.Now()
	r := &Runner{harvestBlockedUntil: map[int32]time.Time{1002: now.Add(-time.Second)}}
	op := &automation.PlannedOp{
		Kind:    "usrLand.harvest",
		LandIDs: []int32{1002},
		Args:    map[string]any{"landId": 1002},
	}

	if got := r.applyHarvestBlocks(op, now); got != op {
		t.Fatalf("applyHarvestBlocks()=%+v, want original op", got)
	}
}
