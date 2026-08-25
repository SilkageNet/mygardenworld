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

func TestFriendTouchPurchaseMapOmittedIsObservedEmpty(t *testing.T) {
	s := New()
	now := time.Now()
	s.ApplyV(json.RawMessage(`{"24":{"0":{"0":100,"9":` + strconv.FormatInt(now.UnixMilli(), 10) + `},"1":[]}}`))
	view := s.FriendTouch(now)
	if !view.StealCntBuyObserved {
		t.Fatal("full namespace-24 base with omitted field 104 must mean observed empty purchases")
	}
	if len(view.StealCntBuyMap) != 0 {
		t.Fatalf("unexpected purchase map: %+v", view.StealCntBuyMap)
	}
}

func TestPickFriendStealLandIDPrefersQualityAndStock(t *testing.T) {
	now := time.Now()
	lands := map[int32]LandView{
		11: {FlowerID: 23001, State: 3, Lvl: 1, Observed: true},
		12: {FlowerID: 23002, State: 3, Lvl: 1, Observed: true},
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
		11: {FlowerID: 23002, State: 3, Lvl: 10, Observed: true, StealUIDs: []int64{100}},
		12: {FlowerID: 23001, State: 3, Lvl: 1, Observed: true},
	}
	landID, ok := PickFriendStealLandID(lands, nil, 100, now)
	if !ok || landID != 12 {
		t.Fatalf("land=%d ok=%v, want 12", landID, ok)
	}
}

func TestPickFriendStealLandIDAppliesPolicySelection(t *testing.T) {
	now := time.Now()
	lands := map[int32]LandView{
		11: {FlowerID: 23001, State: 3, Lvl: 1},
		12: {FlowerID: 23002, State: 3, Lvl: 1},
	}
	landID, ok := PickFriendStealLandIDWithSelection(lands, nil, 100, now, FriendStealSelection{FlowerIDs: []int32{23002}})
	if !ok || landID != 12 {
		t.Fatalf("specific flower selection got land=%d ok=%v", landID, ok)
	}
	if _, ok := PickFriendStealLandIDWithSelection(lands, nil, 100, now, FriendStealSelection{ExcludeFlowerIDs: []int32{23001, 23002}}); ok {
		t.Fatal("excluded flowers must not be selected")
	}
}

func TestPickFriendStealLandIDRespectsOwnerReserve(t *testing.T) {
	now := time.Now()
	lands := map[int32]LandView{
		// Level 1 has cropGets=2 and left=1, so one previous friend pick
		// exhausts this plot even when the current account is not in stealUids.
		11: {FlowerID: 23001, State: 3, Lvl: 1, StealUIDs: []int64{999}},
		12: {FlowerID: 23002, State: 3, Lvl: 1},
	}
	landID, ok := PickFriendStealLandID(lands, nil, 100, now)
	if !ok || landID != 12 {
		t.Fatalf("land=%d ok=%v, want reserve-safe land 12", landID, ok)
	}
}

func TestPickFriendStealLandIDSkipsFlowerElfPlot(t *testing.T) {
	lands := map[int32]LandView{
		11: {FlowerID: 23001, State: 3, Lvl: 1, ElvesID: 7001},
		12: {FlowerID: 23002, State: 3, Lvl: 1},
	}
	landID, ok := PickFriendStealLandID(lands, nil, 100, time.Now())
	if !ok || landID != 12 {
		t.Fatalf("land=%d ok=%v, want ordinary flower land 12", landID, ok)
	}
}

func TestApplyFrdHomeGarden(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"133":{"0":{"0":2001,"1":{"11":{"0":23001,"1":3,"2":1,"6":7001,"8":[999]}}}}}`))
	view := s.FriendTouch(time.Now())
	land := view.VisitLands[11]
	if view.VisitUID != 2001 || land.FlowerID != 23001 || land.Lvl != 1 || land.ElvesID != 7001 || len(land.ElvesStealUIDs) != 1 || land.ElvesStealUIDs[0] != 999 {
		t.Fatalf("unexpected frdHome snapshot: uid=%d land=%+v", view.VisitUID, land)
	}
}

func TestFriendStealSuccessReconcilesOmittedDelta(t *testing.T) {
	s := New()
	now := time.Now()
	nowText := strconv.FormatInt(now.UnixMilli(), 10)
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":100}},"24":{"0":{"0":100,"9":` + nowText + `,"104":{}},"1":[]},"111":{"0":{"1":{"2001":2},"3":` + nowText + `}},"133":{"0":{"0":2001,"1":{"11":{"0":23001,"1":3,"2":1}}}}}`))
	used, bought, usedObserved, boughtObserved := s.FriendStealCounters(2001, now)
	if used != 2 || bought != 0 || !usedObserved || !boughtObserved {
		t.Fatalf("unexpected counters before reconcile: %d/%d %v/%v", used, bought, usedObserved, boughtObserved)
	}
	s.NoteFriendStealSuccess(2001, 11, used, usedObserved, now)
	s.NoteFriendStealPurchase(2001, bought, boughtObserved, now)
	used, bought, _, _ = s.FriendStealCounters(2001, now)
	if used != 3 || bought != 1 {
		t.Fatalf("omitted deltas not reconciled: used=%d bought=%d", used, bought)
	}
	if _, ok := PickFriendStealLandID(s.FriendTouch(now).VisitLands, nil, 100, now); ok {
		t.Fatal("locally reconciled land must not be selected again")
	}
}
