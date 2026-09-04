package automation

import (
	"reflect"
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"google.golang.org/protobuf/proto"
)

func pearlHirePolicyForTest() *pb.PearlPolicy {
	return &pb.PearlPolicy{AutoHireEnabled: true, MaxHireTicketUsage: 2}
}

func newPearlHireStateForTest(t *testing.T, self int64) *state.State {
	t.Helper()
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"0": self, "32": map[string]any{"1003": 3}}},
		"115": map[string]any{
			"0": map[string]any{"1": map[string]any{"2": int64(0), "3": nil, "4": 0, "9": int64(1)}},
			"1": map[string]any{"5": map[string]any{}},
		},
	})
	return s
}

func applyPearlFriendForTest(t *testing.T, s *state.State, self, friend int64) {
	t.Helper()
	applyMap(t, s, map[string]any{"24": map[string]any{
		"0": map[string]any{"0": self},
		"1": []any{map[string]any{"0": self, "1": friend}},
	}})
}

func TestPlanOneSafePearlHireIsSingleStepAndTicketGated(t *testing.T) {
	s := newPearlHireStateForTest(t, 9001)
	policy := pearlHirePolicyForTest()
	now := time.Now()

	op, ok := PlanOneSafePearlHire(s, policy, now, PearlHireIntent{})
	if !ok || op.Kind != clientproto.RPCFrdEnter.String() || len(op.TargetUIDs) != 0 {
		t.Fatalf("first step = %+v, %t", op, ok)
	}
	applyPearlFriendForTest(t, s, 9001, 2001)
	op, _ = PlanOneSafePearlHire(s, policy, time.Now(), PearlHireIntent{})
	if op.Kind != clientproto.RPCOpptGetDetailOppts.String() || !reflect.DeepEqual(op.TargetUIDs, []int64{2001}) {
		t.Fatalf("detail step = %+v", op)
	}
	applyMap(t, s, map[string]any{"28": map[string]any{"5": []any{map[string]any{"0": int64(2001), "1": "safe", "4": 12}}}})
	op, _ = PlanOneSafePearlHire(s, policy, time.Now(), PearlHireIntent{})
	if op.Kind != clientproto.RPCPearlGetHireStateByUids.String() || !reflect.DeepEqual(op.TargetUIDs, []int64{2001}) {
		t.Fatalf("hire-state step = %+v", op)
	}
	applyMap(t, s, map[string]any{"115": map[string]any{"5": map[string]any{"2001": int64(0)}}})
	op, _ = PlanOneSafePearlHire(s, policy, time.Now(), PearlHireIntent{})
	if op.Kind != clientproto.RPCPearlPlaceHire.String() || op.TargetID != 1 || op.TargetUID != 2001 || op.Count != 1 ||
		op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 1 || op.ItemCost[1003] != 1 {
		t.Fatalf("hire step = %+v", op)
	}
	if len(op.TargetUIDs) != 0 {
		t.Fatalf("hire contains sync UIDs: %+v", op)
	}
}

func TestPlanOneSafePearlHireBoundariesAndNoBypass(t *testing.T) {
	s := newPearlHireStateForTest(t, 9001)
	applyPearlFriendForTest(t, s, 9001, 2001)
	applyMap(t, s, map[string]any{
		"28":  map[string]any{"5": []any{map[string]any{"0": int64(2001), "4": 12}}},
		"115": map[string]any{"5": map[string]any{"2001": int64(0)}},
	})
	view := s.PearlHire()
	observedAt := view.Profiles[2001].ObservedAtMs
	policy := pearlHirePolicyForTest()

	op, _ := PlanOneSafePearlHire(s, policy, time.UnixMilli(observedAt).Add(30*time.Second-time.Millisecond), PearlHireIntent{})
	if op.Kind != clientproto.RPCPearlPlaceHire.String() {
		t.Fatalf("cache should be fresh before 30s: %+v", op)
	}
	op, _ = PlanOneSafePearlHire(s, policy, time.UnixMilli(observedAt).Add(30*time.Second), PearlHireIntent{})
	if op.Kind != clientproto.RPCOpptGetDetailOppts.String() {
		t.Fatalf("cache should be stale at 30s: %+v", op)
	}
	op, _ = PlanOneSafePearlHire(s, policy, time.UnixMilli(observedAt).Add(-time.Millisecond), PearlHireIntent{})
	if op.Kind != clientproto.RPCOpptGetDetailOppts.String() {
		t.Fatalf("future-dated cache should fail closed: %+v", op)
	}

	failureAt := time.UnixMilli(observedAt)
	s.MarkPearlHireFailed(2001, failureAt)
	op, _ = PlanOneSafePearlHire(s, policy, failureAt.Add(time.Minute-time.Millisecond), PearlHireIntent{})
	if op.Kind == clientproto.RPCOpptGetDetailOppts.String() && reflect.DeepEqual(op.TargetUIDs, []int64{2001}) {
		t.Fatalf("failed UID retried before 60s: %+v", op)
	}
	op, _ = PlanOneSafePearlHire(s, policy, failureAt.Add(time.Minute), PearlHireIntent{})
	if op.Kind != clientproto.RPCOpptGetDetailOppts.String() || !reflect.DeepEqual(op.TargetUIDs, []int64{2001}) {
		t.Fatalf("failed UID not eligible at exactly 60s: %+v", op)
	}

	disabled := proto.Clone(policy).(*pb.PearlPolicy)
	disabled.AutoHireEnabled = false
	if _, ok := PlanOneSafePearlHire(s, disabled, time.Now(), PearlHireIntent{Category: CategoryActivity, Domain: "activity.cyclicNote"}); ok {
		t.Fatal("activity intent bypassed disabled pearl module")
	}
}

func TestPlanOneSafePearlHireSkipsGoldFallbackCandidateForSession(t *testing.T) {
	s := newPearlHireStateForTest(t, 9001)
	applyMap(t, s, map[string]any{
		"24": map[string]any{
			"0": map[string]any{"0": int64(9001)},
			"1": []any{
				map[string]any{"0": int64(9001), "1": int64(2001)},
				map[string]any{"0": int64(9001), "1": int64(2002)},
			},
		},
		"28": map[string]any{"5": []any{
			map[string]any{"0": int64(2001), "4": 12},
			map[string]any{"0": int64(2002), "4": 12},
		}},
		"115": map[string]any{"5": map[string]any{
			"2001": int64(0),
			"2002": int64(0),
		}},
	})
	s.SkipPearlHireCandidate(2001)

	op, ok := PlanOneSafePearlHire(s, pearlHirePolicyForTest(), time.Now(), PearlHireIntent{})
	if !ok || op.Kind != clientproto.RPCPearlPlaceHire.String() || op.TargetUID != 2002 {
		t.Fatalf("planner did not continue with the next candidate: %+v, %t", op, ok)
	}
}

func TestPlanOneSafePearlHireFailClosedGates(t *testing.T) {
	tests := []struct {
		name   string
		state  func(*testing.T) *state.State
		policy func() *pb.PearlPolicy
		want   string
	}{
		{
			name:   "unknown self",
			state:  func(t *testing.T) *state.State { return newPearlHireStateForTest(t, 0) },
			policy: pearlHirePolicyForTest,
			want:   "自己的 UID",
		},
		{
			name:   "zero max disables",
			state:  func(t *testing.T) *state.State { return newPearlHireStateForTest(t, 9001) },
			policy: func() *pb.PearlPolicy { return &pb.PearlPolicy{AutoHireEnabled: true, MaxHireTicketUsage: 0} },
			want:   "上限为 0",
		},
		{
			name: "partial slot blocks active count",
			state: func(t *testing.T) *state.State {
				s := state.New()
				applyMap(t, s, map[string]any{
					"7": map[string]any{"0": map[string]any{"0": int64(9001), "32": map[string]any{"1003": 3}, "36": 9}},
					"115": map[string]any{"0": map[string]any{
						"1": map[string]any{"2": int64(0), "3": nil},
						"2": map[string]any{"1": 2},
					}},
				})
				return s
			},
			policy: pearlHirePolicyForTest,
			want:   "占用字段不完整",
		},
		{
			name: "monthly card slot remains blocked",
			state: func(t *testing.T) *state.State {
				s := state.New()
				applyMap(t, s, map[string]any{
					"7":   map[string]any{"0": map[string]any{"0": int64(9001), "32": map[string]any{"1003": 3}, "36": 9}},
					"115": map[string]any{"0": map[string]any{"4": map[string]any{"2": int64(0), "3": nil}}},
				})
				return s
			},
			policy: pearlHirePolicyForTest,
			want:   "没有已解锁",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			op, ok := PlanOneSafePearlHire(tc.state(t), tc.policy(), time.Now(), PearlHireIntent{})
			if !ok || op.Status != PlanStatusBlocked || !strings.Contains(op.Reason, tc.want) {
				t.Fatalf("blocked op = %+v, %t", op, ok)
			}
		})
	}
}

func TestPlanOneSafePearlHireUnknownEnemySourceRefreshes(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"0": int64(9001), "32": map[string]any{"1003": 3}}},
		"115": map[string]any{"0": map[string]any{"1": map[string]any{"2": int64(0), "3": nil}}},
		"24":  map[string]any{"0": map[string]any{"0": int64(9001)}, "1": []any{}},
	})
	applyMap(t, s, map[string]any{"115": map[string]any{"6": []any{}}})
	view := s.PearlHire()
	now := time.UnixMilli(view.RecommendObservedAtMs)
	op, _ := PlanOneSafePearlHire(s, pearlHirePolicyForTest(), now, PearlHireIntent{})
	if op.Kind != clientproto.RPCPearlRefresh.String() {
		t.Fatalf("unknown enemy source was skipped: %+v", op)
	}
}

func TestPlanOneSafePearlHireDailyLimitAndExpiredSlot(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 8, 29, 12, 0, 0, 0, shanghai)
	s := newPearlHireStateForTest(t, 9001)
	policy := pearlHirePolicyForTest()
	policy.DailyHireTicketLimit = 2
	s.SetPearlHireTicketUsed(state.PearlHireTicketDayID(now), 2)
	op, ok := PlanOneSafePearlHire(s, policy, now, PearlHireIntent{})
	if !ok || op.Status != PlanStatusBlocked || !strings.Contains(op.Reason, "每日上限 2") {
		t.Fatalf("daily limit op=%+v ok=%t", op, ok)
	}

	s.SetPearlHireTicketUsed(state.PearlHireTicketDayID(now), 1)
	applyPearlFriendForTest(t, s, 9001, 2001)
	applyMap(t, s, map[string]any{
		"28": map[string]any{"5": []any{map[string]any{"0": int64(2001), "1": "safe", "4": 12}}},
		"115": map[string]any{
			"0": map[string]any{"1": map[string]any{"2": int64(3001), "3": int64(1), "4": 0, "9": now.UnixMilli()}},
			"5": map[string]any{"2001": int64(0)},
		},
	})
	now = time.UnixMilli(s.PearlHire().Profiles[2001].ObservedAtMs)
	s.SetPearlHireTicketUsed(state.PearlHireTicketDayID(now), 1)
	op, ok = PlanOneSafePearlHire(s, policy, now, PearlHireIntent{})
	if !ok || op.Kind != clientproto.RPCPearlPlaceHire.String() || op.TargetID != 1 || op.TargetUID != 2001 {
		t.Fatalf("expired slot was not reused safely: %+v ok=%t", op, ok)
	}
}
