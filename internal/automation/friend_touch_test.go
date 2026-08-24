package automation

import (
	"strconv"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestFriendTouchPlansStealAfterSync(t *testing.T) {
	s := state.New()
	policy := &pb.FriendTouchPolicy{
		Enabled:      true,
		Mode:         pb.SelectionMode_SELECTION_MODE_SPECIFIC,
		FriendCounts: map[int64]int32{2001: 5},
	}
	now := applyFriendTouchFixture(s, []int64{2001}, map[int64]bool{2001: true}, map[int64]int32{2001: 1})

	op, ok := PlanOneFriendTouch(s, policy, now)
	if !ok {
		t.Fatal("expected enter garden op")
	}
	if op.Kind != clientproto.RPCFrdStealEnterFrdSteal.String() || op.TargetUID != 2001 {
		t.Fatalf("want enterFrdSteal, got %+v", op)
	}

	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"11":{"0":23001,"1":3},"12":{"0":23002,"1":1}}}}}`))
	now = time.Now()
	op, ok = PlanOneFriendTouch(s, policy, now)
	if !ok {
		t.Fatal("expected steal op")
	}
	if op.Kind != clientproto.RPCFrdStealSteal.String() || op.TargetUID != 2001 || op.TargetID != 11 {
		t.Fatalf("want frdSteal.steal land 11, got %+v", op)
	}
}

func TestFriendTouchSyncsFriendsFirst(t *testing.T) {
	s := state.New()
	now := time.Now()
	policy := &pb.FriendTouchPolicy{
		Enabled:      true,
		Mode:         pb.SelectionMode_SELECTION_MODE_SPECIFIC,
		FriendCounts: map[int64]int32{2001: 1},
	}
	op, ok := PlanOneFriendTouch(s, policy, now)
	if !ok || op.Kind != clientproto.RPCFrdEnter.String() {
		t.Fatalf("want frd.enter, got %+v ok=%v", op, ok)
	}
}

func TestFriendListIdleSyncWithoutTouchEnabled(t *testing.T) {
	s := state.New()
	now := time.Now()
	ops := friendTouchOperations(s, &pb.FriendTouchPolicy{Enabled: false}, now)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCFrdEnter.String() {
		t.Fatalf("want idle frd.enter, got %+v", ops)
	}
	if ops[0].Priority != friendListIdlePriority+1 || ops[0].FeatureID != "basic.friend_list_sync" {
		t.Fatalf("idle sync meta=%+v", ops[0])
	}
}

func TestFriendListIdleSyncProfilesAfterFriends(t *testing.T) {
	s := state.New()
	now := applyFriendTouchFixture(s, []int64{2001}, map[int64]bool{2001: true}, map[int64]int32{2001: 0})
	// Drop profile observation so idle sync still requests names.
	s.ApplyV([]byte(`{"28":{"5":[{"0":2001,"1":"","4":-1}]}}`))
	ops := friendListIdleSyncOperations(s, now)
	if len(ops) != 1 || ops[0].Kind != clientproto.RPCOpptGetDetailOppts.String() {
		t.Fatalf("want idle profile sync, got %+v", ops)
	}
}

func TestFriendTouchAllModeSkipsExcluded(t *testing.T) {
	s := state.New()
	policy := &pb.FriendTouchPolicy{
		Enabled:     true,
		Mode:        pb.SelectionMode_SELECTION_MODE_ALL,
		ExcludeUids: []int64{2001},
	}
	now := applyFriendTouchFixture(s, []int64{2001, 2002}, map[int64]bool{2001: true, 2002: true}, map[int64]int32{2001: 0, 2002: 0})

	op, ok := PlanOneFriendTouch(s, policy, now)
	if !ok {
		t.Fatal("expected enter garden op")
	}
	if op.Kind != clientproto.RPCFrdStealEnterFrdSteal.String() || op.TargetUID != 2002 {
		t.Fatalf("want enter uid 2002, got %+v", op)
	}
}

func TestFriendTouchSpecificRespectsExclude(t *testing.T) {
	s := state.New()
	policy := &pb.FriendTouchPolicy{
		Enabled:      true,
		Mode:         pb.SelectionMode_SELECTION_MODE_SPECIFIC,
		FriendCounts: map[int64]int32{2001: 5, 2002: 5},
		ExcludeUids:  []int64{2001},
	}
	now := applyFriendTouchFixture(s, []int64{2001, 2002}, map[int64]bool{2001: true, 2002: true}, map[int64]int32{2001: 0, 2002: 0})

	op, ok := PlanOneFriendTouch(s, policy, now)
	if !ok {
		t.Fatal("expected enter garden op")
	}
	if op.TargetUID != 2002 || op.Kind != clientproto.RPCFrdStealEnterFrdSteal.String() {
		t.Fatalf("want enter uid 2002, got %+v", op)
	}
}

func TestFriendTouchSkipsNonStealableFriend(t *testing.T) {
	s := state.New()
	policy := &pb.FriendTouchPolicy{
		Enabled: true,
		Mode:    pb.SelectionMode_SELECTION_MODE_ALL,
	}
	now := applyFriendTouchFixture(s, []int64{2001, 2002}, map[int64]bool{2001: false, 2002: true}, map[int64]int32{2001: 0, 2002: 0})

	op, ok := PlanOneFriendTouch(s, policy, now)
	if !ok {
		t.Fatal("expected enter garden op")
	}
	if op.TargetUID != 2002 || op.Kind != clientproto.RPCFrdStealEnterFrdSteal.String() {
		t.Fatalf("want enter uid 2002, got %+v", op)
	}
}

func TestFriendTouchNoOpWhenNobodyStealable(t *testing.T) {
	s := state.New()
	policy := &pb.FriendTouchPolicy{
		Enabled: true,
		Mode:    pb.SelectionMode_SELECTION_MODE_ALL,
	}
	now := applyFriendTouchFixture(s, []int64{2001, 2002}, map[int64]bool{2001: false, 2002: false}, map[int64]int32{2001: 0, 2002: 0})

	if op, ok := PlanOneFriendTouch(s, policy, now); ok {
		t.Fatalf("expected no op, got %+v", op)
	}
}

func TestFriendTouchPrefersHigherQualityLowerStockLand(t *testing.T) {
	s := state.New()
	s.ApplyV([]byte(`{"7":{"0":{"0":100,"1":{"23001":5,"23002":50}}}}`))
	policy := &pb.FriendTouchPolicy{
		Enabled:      true,
		Mode:         pb.SelectionMode_SELECTION_MODE_SPECIFIC,
		FriendCounts: map[int64]int32{2001: 1},
	}
	now := applyFriendTouchFixture(s, []int64{2001}, map[int64]bool{2001: true}, map[int64]int32{2001: 0})
	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"11":{"0":23001,"1":3},"12":{"0":23002,"1":3}}}}}`))

	op, ok := PlanOneFriendTouch(s, policy, now)
	if !ok {
		t.Fatal("expected steal op")
	}
	if op.Kind != clientproto.RPCFrdStealSteal.String() || op.TargetID != 11 {
		t.Fatalf("want higher-quality lower-stock land 11, got %+v", op)
	}
}

func applyFriendTouchFixture(s *state.State, friendUIDs []int64, canSteal map[int64]bool, stolen map[int64]int32) time.Time {
	relations := ""
	profiles := ""
	other := ""
	stealMap := ""
	for i, uid := range friendUIDs {
		if i > 0 {
			relations += ","
			profiles += ","
			other += ","
			stealMap += ","
		}
		uidText := formatInt(uid)
		relations += `{"0":100,"1":` + uidText + `}`
		profiles += `{"0":` + uidText + `,"1":"好友` + uidText + `","4":20}`
		stealFlag := "0"
		if canSteal[uid] {
			stealFlag = "1"
		}
		other += `"` + uidText + `":{"0":` + stealFlag + `}`
		stealMap += `"` + uidText + `":` + strconv.FormatInt(int64(stolen[uid]), 10)
	}
	s.ApplyV([]byte(`{"7":{"0":{"0":100}}}`))
	now := time.Now()
	nowMs := formatInt(now.UnixMilli())
	s.ApplyV([]byte(`{"24":{"0":{"0":100,"9":` + nowMs + `},"1":[` + relations + `]}}`))
	s.ApplyV([]byte(`{"28":{"5":[` + profiles + `]}}`))
	s.ApplyV([]byte(`{"110":{"1":{` + other + `}}}`))
	s.ApplyV([]byte(`{"111":{"0":{"0":100,"1":{` + stealMap + `},"3":` + nowMs + `}}}`))
	return time.Now()
}

func formatInt(v int64) string {
	return strconv.FormatInt(v, 10)
}
