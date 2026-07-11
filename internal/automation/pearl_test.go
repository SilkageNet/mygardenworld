package automation

import (
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestBuildPlanPearlUsesOneKeyOnceForAllMatureSlots(t *testing.T) {
	now := time.UnixMilli(8_000_000)
	s := state.New()
	applyMap(t, s, map[string]any{"115": map[string]any{"0": map[string]any{
		"1": map[string]any{"1": 1, "3": int64(7_200_000), "6": 5, "7": 0, "8": 0},
		"2": map[string]any{"1": 2, "3": int64(7_200_000), "6": 4, "7": 0, "8": 0},
		"3": map[string]any{"1": 3, "3": int64(7_200_000), "6": 5, "7": 0, "8": 0},
	}}})
	p := DefaultPolicy()
	p.AutomationEnabled = true
	p.Basic.Pearl.FreeEnabled = true

	result := BuildPlan(s, p, now)
	var oneKeyCount int
	for _, op := range result.Operations {
		if op.Kind == clientproto.RPCPearlPlaceRecv.String() {
			t.Fatalf("planner emitted per-slot compatibility RPC: %+v", op)
		}
		if op.Kind != clientproto.RPCPearlPlaceRecvOneKey.String() {
			continue
		}
		oneKeyCount++
		if op.TargetID != 0 || op.ItemID != 0 || op.Count != 0 ||
			op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
			t.Fatalf("one-key operation contains request/cost fields: %+v", op)
		}
	}
	if oneKeyCount != 1 {
		t.Fatalf("one-key operation count=%d, want 1: %+v", oneKeyCount, result.Operations)
	}
}

func TestBuildPlanPearlMaturityAndObservationGates(t *testing.T) {
	policy := DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Pearl.FreeEnabled = true
	tests := []struct {
		name  string
		nowMS int64
		place map[string]any
		want  bool
	}{
		{name: "179 seconds not mature", nowMS: 179_000, place: map[string]any{"1": 1, "3": int64(7_200_000), "6": 5, "7": 0, "8": 0}},
		{name: "180 seconds mature", nowMS: 180_000, place: map[string]any{"1": 1, "3": int64(7_200_000), "6": 5, "7": 0, "8": 0}, want: true},
		{name: "all cycles received", nowMS: 8_000_000, place: map[string]any{"1": 1, "3": int64(7_200_000), "6": 5, "7": 40, "8": 0}},
		{name: "missing recv observation", nowMS: 8_000_000, place: map[string]any{"1": 1, "3": int64(7_200_000), "6": 5, "8": 0}},
		{name: "surplus only without end is ignored", nowMS: 8_000_000, place: map[string]any{"1": 1, "3": nil, "6": 0, "7": 0, "8": 9}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := state.New()
			applyMap(t, s, map[string]any{"115": map[string]any{"0": map[string]any{"1": tc.place}}})
			result := BuildPlan(s, policy, time.UnixMilli(tc.nowMS))
			got := false
			for _, op := range result.Operations {
				if op.Kind == clientproto.RPCPearlPlaceRecvOneKey.String() {
					got = true
				}
			}
			if got != tc.want {
				t.Fatalf("one-key planned=%t, want %t: %+v", got, tc.want, result.Operations)
			}
		})
	}
}
