package state

import (
	"encoding/json"
	"strconv"
	"testing"
	"time"
)

func TestApplyFrdStealAndFriendTouchFriends(t *testing.T) {
	s := New()
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, gameDayLocation())
	nowMs := strconv.FormatInt(now.UnixMilli(), 10)
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":100}}}`))
	s.ApplyV(json.RawMessage(`{"24":{"0":{"0":100,"9":` + nowMs + `,"104":{"2001":2}},"1":[{"0":100,"1":2001},{"0":100,"1":2002}]}}`))
	s.ApplyV(json.RawMessage(`{"28":{"5":[{"0":2001,"1":"阿花","4":20},{"0":2002,"1":"阿明","4":18}]}}`))
	s.ApplyV(json.RawMessage(`{"111":{"0":{"0":100,"1":{"2001":3},"3":` + nowMs + `}}}`))
	s.ApplyV(json.RawMessage(`{"110":{"1":{"2001":{"0":1},"2002":{"0":0}}}}`))

	friends := s.FriendTouchFriends(now)
	if len(friends) != 2 {
		t.Fatalf("friends=%d, want 2", len(friends))
	}
	if friends[0].Name != "阿花" && friends[1].Name != "阿花" {
		t.Fatalf("missing friend name: %+v", friends)
	}
	var picked FriendTouchFriendView
	for _, friend := range friends {
		if friend.UID == 2001 {
			picked = friend
		}
	}
	if picked.StolenCount != 3 {
		t.Fatalf("stolen=%d, want 3", picked.StolenCount)
	}
	if picked.StealMax != 12 {
		t.Fatalf("max=%d, want 12 (10 base + 2 bought)", picked.StealMax)
	}
	if !picked.CanSteal {
		t.Fatalf("expected can steal for uid 2001")
	}
}

func TestPickFriendStealLandIDPrefersQualityAndStock(t *testing.T) {
	now := time.Now()
	lands := map[int32]LandView{
		11: {FlowerID: 23001, State: 3, Observed: true},
		12: {FlowerID: 23002, State: 3, Observed: true},
	}
	inventory := map[int32]int32{23001: 5, 23002: 50}
	landID, ok := PickFriendStealLandID(lands, inventory, 100, now)
	if !ok || landID != 11 {
		t.Fatalf("land=%d ok=%v, want 11", landID, ok)
	}
}

func TestPickFriendStealLandIDSkipsAlreadyStolenPlot(t *testing.T) {
	now := time.Now()
	lands := map[int32]LandView{
		11: {FlowerID: 23002, State: 3, Observed: true, StealUIDs: []int64{100}},
		12: {FlowerID: 23001, State: 3, Observed: true},
	}
	landID, ok := PickFriendStealLandID(lands, nil, 100, now)
	if !ok || landID != 12 {
		t.Fatalf("land=%d ok=%v, want 12", landID, ok)
	}
}
