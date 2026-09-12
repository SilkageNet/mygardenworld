package state

import (
	"testing"
	"time"
)

func TestFromPrimaryReadsElvesFields(t *testing.T) {
	land := FromPrimary(map[string]any{
		"0": 23331,
		"1": 3,
		"2": 1,
		"3": 0,
		"4": []any{float64(1)},
		"5": float64(0),
		"6": 110132,
		"7": float64(1000),
		"8": []any{float64(99)},
	})
	if land.FlowerID != 23331 || land.ElvesID != 110132 {
		t.Fatalf("flower/elves = %d/%d", land.FlowerID, land.ElvesID)
	}
	if land.HasStealableElves() {
		t.Fatal("expected not stealable when elvesStealUids set")
	}
	land.ElvesStealUIDs = nil
	if !land.HasStealableElves() {
		t.Fatal("expected stealable when mature elves present and steal uids empty")
	}
	land.State = 2
	land.NextTimeMs = 0
	if land.HasStealableElves() {
		t.Fatal("regrowing land without due nextTime must not be stealable")
	}
	now := time.UnixMilli(1_700_000_000_000)
	land.NextTimeMs = now.UnixMilli()
	if !land.HasStealableElvesAt(now) {
		t.Fatal("state=2 with due nextTime should be stealable like ordinary friend steal")
	}
	if land.HasStealableElvesAt(now.Add(-time.Second)) {
		t.Fatal("state=2 before nextTime must not be stealable")
	}
}

func TestHasStealableElvesForSkipsOwnFlowerSteal(t *testing.T) {
	land := LandView{ElvesID: 110132, State: 3, StealUIDs: []int64{9001}}
	if !land.HasStealableElvesFor(0, time.Now()) {
		t.Fatal("other accounts should still see the elf")
	}
	if land.HasStealableElvesFor(9001, time.Now()) {
		t.Fatal("own flower-steal uid must block elf steal")
	}
	if !land.HasStealableElvesFor(9002, time.Now()) {
		t.Fatal("different uid should still steal elf")
	}
}

func TestPickFriendStealElvesLandForSkipsStickyAndSelfSteal(t *testing.T) {
	now := time.Now()
	lands := map[int32]LandView{
		1020: {ElvesID: 110156, State: 3, PlantTimeMs: 100, StealUIDs: []int64{9001}},
		1031: {ElvesID: 110156, State: 3, PlantTimeMs: 100},
		1040: {ElvesID: 110156, State: 3, PlantTimeMs: 100},
	}
	landID, _, ok := PickFriendStealElvesLandFor(lands, now, 9001, func(id int32, land LandView) bool {
		return id == 1031 && land.PlantTimeMs == 100
	})
	if !ok || landID != 1040 {
		t.Fatalf("want land 1040 after skipping self-stolen 1020 and sticky 1031, got %d ok=%v", landID, ok)
	}
}

func TestMarkFriendStealElvesLandUnavailableStickyAcrossRefresh(t *testing.T) {
	s := New()
	s.roleID = 9001
	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"1020":{"0":23404,"1":3,"6":110156,"7":100,"8":[]},"1040":{"0":23404,"1":3,"6":110156,"7":100,"8":[]}}}}}`))
	s.MarkFriendStealElvesLandUnavailable(2001, 1020)
	if !s.FriendStealElvesLandSkipped(2001, 1020, 100) {
		t.Fatal("expected sticky skip for plantTime=100")
	}
	// Refresh still shows elvesId / empty elvesStealUids — skip must hold.
	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"1020":{"0":23404,"1":3,"6":110156,"7":100,"8":[]},"1040":{"0":23404,"1":3,"6":110156,"7":100,"8":[]}}}}}`))
	landID, _, ok := PickFriendStealElvesLandFor(s.FriendTouch(time.Now()).VisitLands, time.Now(), 9001, func(id int32, land LandView) bool {
		return s.FriendStealElvesLandSkipped(2001, id, land.PlantTimeMs)
	})
	if !ok || landID != 1040 {
		t.Fatalf("want next land 1040, got %d ok=%v", landID, ok)
	}
	// Replant clears skip.
	s.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"1020":{"0":23404,"1":3,"6":110156,"7":200,"8":[]}}}}}`))
	if s.FriendStealElvesLandSkipped(2001, 1020, 200) {
		t.Fatal("replant plantTime must clear sticky skip")
	}
}

func TestFlowerElvesBookByPairHuaiyue(t *testing.T) {
	pair, ok := FlowerElvesBookByPair(23517, 23331)
	if !ok {
		t.Fatal("expected book pair for 槐月+花笼流芳")
	}
	if pair.FlowerElvesItem != 110132 {
		t.Fatalf("elves item = %d", pair.FlowerElvesItem)
	}
}

func TestFriendLandsPlantingElves(t *testing.T) {
	if FriendLandsPlantingElves(nil) || FriendLandsPlantingElves(map[int32]LandView{}) {
		t.Fatal("empty lands should not count as planting")
	}
	if FriendLandsPlantingElves(map[int32]LandView{
		1: {FlowerID: 23001, State: 3},
	}) {
		t.Fatal("ordinary flower should not count as elves planting")
	}
	if !FriendLandsPlantingElves(map[int32]LandView{
		1: {FlowerID: 23331, State: 1}, // 花笼流芳 secondary
	}) {
		t.Fatal("catalog secondary should count as planting while waiting for spawn")
	}
	if FriendLandsPlantingElves(map[int32]LandView{
		1: {FlowerID: 23001, ElvesID: 110132},
	}) {
		t.Fatal("elvesId already present should not keep fast planting poll")
	}
	if FriendLandsPlantingElves(map[int32]LandView{
		1: {FlowerID: 23331, State: 3, ElvesID: 110132},
		2: {FlowerID: 23331, State: 1},
	}) {
		t.Fatal("any elvesId should stop fast poll even if other secondaries are still growing")
	}
}

func TestElvesPickedDone(t *testing.T) {
	if ElvesPickedDone(10, 5, 20) {
		t.Fatal("10+5 < 20 should be false")
	}
	if !ElvesPickedDone(10, 15, 20) {
		t.Fatal("10+15 >= 20 should be true")
	}
	if !ElvesPickedDone(5, 30, 20) {
		t.Fatal("overshoot secondary should still be done")
	}
}

func TestResolveElvesSpawnCap(t *testing.T) {
	if got := ResolveElvesSpawnCap(8); got != 8 {
		t.Fatalf("got %d", got)
	}
	if got := ResolveElvesSpawnCap(0); got != 30 && got != DefaultElvesSpawnCap {
		// catalog may yield 18+12=30
		if got < 18 {
			t.Fatalf("unexpected default cap %d", got)
		}
	}
}

func TestFlowerElvesHouseAggregates(t *testing.T) {
	s := New()
	now := time.Unix(1_788_768_000, 0) // inside 1046 天穹币 season
	s.ApplyVMap(map[string]any{
		"7": map[string]any{
			"0": map[string]any{"32": map[string]any{
				"1014": 42, "1046": 9658, "110001": 3, "110002": 2, "1001": 9,
			}},
			"4": map[string]any{
				"103": map[string]any{
					"1": 103,
					"2": 30,
					"3": 100,
					"4": now.UnixMilli(),
				},
			},
		},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23331, "1": 3, "6": 110132, "8": []any{}},
			"1002": map[string]any{"0": 23331, "1": 3, "6": 110132, "8": []any{float64(99)}},
			"1003": map[string]any{"0": 23331, "1": 3, "6": 0},
		}},
		"111": map[string]any{"0": map[string]any{
			"0": int64(9001),
			"7": 1,
		}},
		"132": map[string]any{"2": map[string]any{
			"1": map[string]any{"1": 1, "2": 110001, "3": 5, "4": int64(9_000_000)},
			"2": map[string]any{"1": 2, "2": 110002, "3": 1},
		}},
	})
	view := s.FlowerElvesHouseAt(now)
	if !view.PlacesObserved {
		t.Fatal("expected places observed")
	}
	if view.MoneyItemID != 1046 || view.MoneyCount != 9658 {
		t.Fatalf("money=%d/%d, want 1046/9658", view.MoneyItemID, view.MoneyCount)
	}
	if view.DispatchableCount != 5 {
		t.Fatalf("dispatchable=%d, want 5", view.DispatchableCount)
	}
	if !view.PlantedObserved || view.PlantedCount != 30 || view.PlantedCap != 30 {
		t.Fatalf("planted=%v %d/%d", view.PlantedObserved, view.PlantedCount, view.PlantedCap)
	}
	if !view.HarvestableObserved || view.HarvestableCount != 1 || view.HarvestableCap != 4 {
		t.Fatalf("steal=%v %d/%d", view.HarvestableObserved, view.HarvestableCount, view.HarvestableCap)
	}
	if view.DispatchedCount != 6 {
		t.Fatalf("dispatched=%d, want 6", view.DispatchedCount)
	}
	if view.ElvesLimit != 300 {
		t.Fatalf("elves_limit=%d, want 300", view.ElvesLimit)
	}
	if view.SlotCount < 2 || len(view.Places) < 2 {
		t.Fatalf("slots=%d places=%d", view.SlotCount, len(view.Places))
	}
	// place1: 5 elves of color2 → 5*2*1=10; place2: 1 elf color2 → 2; total 12
	// (110001/110002 are color 2 in catalog)
	if view.PendingRewardMoney <= 0 {
		t.Fatalf("pending_reward=%d, want >0", view.PendingRewardMoney)
	}
}

func TestFlowerElvesHouseAidExpiry(t *testing.T) {
	s := New()
	now := time.UnixMilli(1_700_000_000_000)
	end := now.Add(90 * time.Minute).UnixMilli()
	s.ApplyVMap(map[string]any{
		"132": map[string]any{"5": map[string]any{
			"3": end,
			"4": 0,
		}},
	})
	view := s.FlowerElvesHouseAt(now)
	if !view.AidObserved || view.AidEffEndTimeMs != end {
		t.Fatalf("aid=%+v", view)
	}
	if view.AidFriendAddRate != 10 {
		t.Fatalf("rate=%d want 10", view.AidFriendAddRate)
	}
	if view.AidCanRecv || view.AidReqOpen {
		t.Fatalf("active buff without open req: %+v", view)
	}
}

func TestFlowerElvesHouseAidCooldownAndHelpers(t *testing.T) {
	s := New()
	now := time.UnixMilli(1_789_178_000_000)
	pre := now.Add(-30 * time.Minute).UnixMilli()
	s.ApplyVMap(map[string]any{
		"132": map[string]any{"5": map[string]any{
			"1": map[string]any{"1001": float64(now.UnixMilli()), "1002": float64(now.UnixMilli())},
			"2": float64(pre),
			"3": nil,
			"4": 1,
		}},
	})
	view := s.FlowerElvesHouseAt(now)
	if view.AidHelperCount != 2 || !view.AidReqOpen || !view.AidCanRecv {
		t.Fatalf("helpers/req/canRecv: %+v", view)
	}
	wantReady := pre + 150*60*1000
	if view.AidReqReadyAtMs != wantReady {
		t.Fatalf("readyAt=%d want %d (cooldown clock keeps running while request is open)", view.AidReqReadyAtMs, wantReady)
	}

	s.ApplyVMap(map[string]any{
		"132": map[string]any{"5": map[string]any{
			"4": 0,
		}},
	})
	view = s.FlowerElvesHouseAt(now)
	if !view.AidCanRecv || view.AidReqOpen {
		t.Fatalf("closed request with helpers should stay claimable: %+v", view)
	}
	if view.AidReqReadyAtMs != wantReady {
		t.Fatalf("readyAt=%d want %d", view.AidReqReadyAtMs, wantReady)
	}
}

func TestFlowerElvesDispatchRewardMoney(t *testing.T) {
	// color 4 rate is $elvesMoney[3]=4
	if got := FlowerElvesDispatchRewardMoney(110132, 3, 1); got != 12 {
		t.Fatalf("color4 3*4*1 = %d, want 12", got)
	}
	if got := FlowerElvesDispatchRewardMoney(110132, 3, 2); got != 24 {
		t.Fatalf("double multi = %d, want 24", got)
	}
	if got := FlowerElvesDispatchRewardMoney(0, 3, 1); got != 0 {
		t.Fatalf("empty = %d", got)
	}
}

func TestGetTdyCountFlowerElvesHarvestResetsAcrossDay(t *testing.T) {
	s := New()
	day1 := time.Date(2026, 9, 11, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	day2 := time.Date(2026, 9, 12, 1, 0, 0, 0, time.FixedZone("CST", 8*3600))
	s.ApplyVMap(map[string]any{
		"7": map[string]any{"4": map[string]any{
			"103": map[string]any{"1": 103, "2": 28, "4": day1.UnixMilli()},
		}},
	})
	count, obs := s.GetTdyCount(UsrCountTypeFlowerElvesHarvest, day1)
	if !obs || count != 28 {
		t.Fatalf("day1 count=%d obs=%v", count, obs)
	}
	count, obs = s.GetTdyCount(UsrCountTypeFlowerElvesHarvest, day2)
	if !obs || count != 0 {
		t.Fatalf("day2 count=%d obs=%v, want 0 after calendar reset", count, obs)
	}
}

func TestActiveFlowerElvesMoneyItemID(t *testing.T) {
	id, ok := ActiveFlowerElvesMoneyItemID(time.Unix(1_788_768_000, 0))
	if !ok || id != 1046 {
		t.Fatalf("got %d,%v want 1046", id, ok)
	}
	id, ok = ActiveFlowerElvesMoneyItemID(time.Unix(1_760_000_000, 0))
	if !ok || id != 1014 {
		t.Fatalf("got %d,%v want 1014", id, ok)
	}
}

func TestFlowerElvesPlaceNullEntryDeletes(t *testing.T) {
	s := New()
	s.ApplyVMap(map[string]any{"132": map[string]any{"2": map[string]any{
		"1": map[string]any{"1": 1, "2": 110001, "3": 2},
	}}})
	s.ApplyVMap(map[string]any{"132": map[string]any{"2": map[string]any{
		"1": nil,
	}}})
	view := s.FlowerElvesHouse()
	if !view.PlacesObserved || view.DispatchedCount != 0 {
		t.Fatalf("view=%+v", view)
	}
	for _, place := range view.Places {
		if place.PlaceID == 1 && (place.ElvesID != 0 || place.ElvesNum != 0) {
			t.Fatalf("place1 should be cleared, got %+v", place)
		}
	}
}
