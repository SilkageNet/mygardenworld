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
		t.Fatalf("failed UID retried while session exclusion active: %+v", op)
	}
	op, _ = PlanOneSafePearlHire(s, policy, failureAt.Add(24*time.Hour), PearlHireIntent{})
	if op.Kind == clientproto.RPCOpptGetDetailOppts.String() && reflect.DeepEqual(op.TargetUIDs, []int64{2001}) {
		t.Fatalf("failed UID retried after long wait without session reset: %+v", op)
	}

	disabled := proto.Clone(policy).(*pb.PearlPolicy)
	disabled.AutoHireEnabled = false
	if _, ok := PlanOneSafePearlHire(s, disabled, time.Now(), PearlHireIntent{Category: CategoryActivity, Domain: "activity.cyclicNote"}); ok {
		t.Fatal("activity intent bypassed disabled pearl module")
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

func TestPlanOneSafePearlHireDailyLimitSkipsFailedAndRehiresExpired(t *testing.T) {
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s := newPearlHireStateForTest(t, 9001)
	applyPearlFriendForTest(t, s, 9001, 2001)
	applyMap(t, s, map[string]any{
		"28":  map[string]any{"5": []any{map[string]any{"0": int64(2001), "1": "a", "4": 10}}},
		"115": map[string]any{"5": map[string]any{"2001": int64(0)}},
	})
	policy := pearlHirePolicyForTest()
	policy.DailyHireTicketLimit = 1
	s.NotePearlHireTicketUsed(now)
	op, ok := PlanOneSafePearlHire(s, policy, now, PearlHireIntent{})
	if !ok || op.Status != PlanStatusBlocked || !strings.Contains(op.Reason, "每日上限") {
		t.Fatalf("daily limit = %+v, %t", op, ok)
	}

	s2 := newPearlHireStateForTest(t, 9001)
	applyMap(t, s2, map[string]any{"24": map[string]any{
		"0": map[string]any{"0": int64(9001)},
		"1": []any{
			map[string]any{"0": int64(9001), "1": int64(2001)},
			map[string]any{"0": int64(9001), "1": int64(2002)},
		},
	}})
	applyMap(t, s2, map[string]any{
		"28": map[string]any{"5": []any{
			map[string]any{"0": int64(2001), "1": "a", "4": 10},
			map[string]any{"0": int64(2002), "1": "b", "4": 11},
		}},
		"115": map[string]any{"5": map[string]any{"2001": int64(0), "2002": int64(0)}},
	})
	freshNow := time.UnixMilli(s2.PearlHire().Profiles[2002].ObservedAtMs)
	s2.MarkPearlHireFailed(2001, freshNow)
	op, _ = PlanOneSafePearlHire(s2, pearlHirePolicyForTest(), freshNow, PearlHireIntent{})
	if op.Kind != clientproto.RPCPearlPlaceHire.String() || op.TargetUID != 2002 {
		t.Fatalf("should skip failed UID and hire next: %+v", op)
	}

	s3 := newPearlHireStateForTest(t, 9001)
	applyPearlFriendForTest(t, s3, 9001, 2001)
	applyMap(t, s3, map[string]any{
		"28":  map[string]any{"5": []any{map[string]any{"0": int64(2001), "1": "a", "4": 10}}},
		"115": map[string]any{"5": map[string]any{"2001": int64(0)}},
	})
	freshNow = time.UnixMilli(s3.PearlHire().Profiles[2001].ObservedAtMs)
	ended := freshNow.Add(-time.Minute).UnixMilli()
	applyMap(t, s3, map[string]any{
		"115": map[string]any{"0": map[string]any{"1": map[string]any{"2": int64(3001), "3": ended}}},
	})
	op, _ = PlanOneSafePearlHire(s3, pearlHirePolicyForTest(), freshNow, PearlHireIntent{})
	if op.Kind != clientproto.RPCPearlPlaceHire.String() || op.TargetID != 1 || op.TargetUID != 2001 {
		t.Fatalf("expired labor should free slot for rehire: %+v", op)
	}
}

func TestPlanOneSafePearlHireWorldEmptyWaitsOneMinute(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7":   map[string]any{"0": map[string]any{"0": int64(9001), "32": map[string]any{"1003": 3}}},
		"115": map[string]any{"0": map[string]any{"1": map[string]any{"2": int64(0), "3": nil}}, "1": map[string]any{"5": map[string]any{}}},
		"24":  map[string]any{"0": map[string]any{"0": int64(9001)}, "1": []any{}},
	})
	applyMap(t, s, map[string]any{"115": map[string]any{"6": []any{}}})
	now := time.UnixMilli(s.PearlHire().RecommendObservedAtMs)
	op, ok := PlanOneSafePearlHire(s, pearlHirePolicyForTest(), now, PearlHireIntent{})
	if !ok || op.Status != PlanStatusBlocked || !strings.Contains(op.Reason, "1 分钟后重新拉取") {
		t.Fatalf("empty world = %+v, %t", op, ok)
	}
	view := s.PearlHireAt(now)
	if view.WorldEmptyUntilMs != now.Add(time.Minute).UnixMilli() || view.FriendsObserved {
		t.Fatalf("empty world state = %+v", view)
	}
	op, _ = PlanOneSafePearlHire(s, pearlHirePolicyForTest(), now.Add(30*time.Second), PearlHireIntent{})
	if op.Status != PlanStatusBlocked || !strings.Contains(op.Reason, "后重新拉取候选") {
		t.Fatalf("waiting empty world = %+v", op)
	}
	op, _ = PlanOneSafePearlHire(s, pearlHirePolicyForTest(), now.Add(time.Minute), PearlHireIntent{})
	if op.Kind != clientproto.RPCFrdEnter.String() {
		t.Fatalf("after wait should re-pull friends: %+v", op)
	}
}
