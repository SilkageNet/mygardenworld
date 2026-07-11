package state

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestZooSouvenirCaptureDerivedFixtureTracksRewardsThenRead(t *testing.T) {
	raw, err := os.ReadFile("testdata/zoo_souvenir.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture struct {
		Initial     json.RawMessage `json:"initial"`
		RewardDelta json.RawMessage `json:"reward_delta"`
		ReadDelta   json.RawMessage `json:"read_delta"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}

	s := New()
	s.ApplyV(fixture.Initial)
	if !s.ZooSouvenirsObserved() || s.ZooSouvenirCount() != 2 {
		t.Fatalf("initial souvenir state observed=%t count=%d", s.ZooSouvenirsObserved(), s.ZooSouvenirCount())
	}
	if got := s.UnreadZooSouvenirIDs(); !reflect.DeepEqual(got, []int32{32901}) {
		t.Fatalf("unread=%v, want [32901]", got)
	}
	if got := s.ReadyZooSouvenirRewardIDs(); !reflect.DeepEqual(got, []int32{2}) {
		t.Fatalf("ready rewards=%v, want [2]", got)
	}

	s.ApplyV(fixture.RewardDelta)
	if !s.ZooSouvenirRewardsClaimed([]int32{2}) || len(s.ReadyZooSouvenirRewardIDs()) != 0 {
		t.Fatalf("reward delta not applied: zoo=%+v ready=%v", s.Zoo(), s.ReadyZooSouvenirRewardIDs())
	}
	s.ApplyV(fixture.ReadDelta)
	if !s.ZooSouvenirsAcknowledged([]int32{32901}) || len(s.UnreadZooSouvenirIDs()) != 0 {
		t.Fatalf("read delta not applied: souvenirs=%+v", s.ZooSouvenirs())
	}
}

func TestZooSouvenirSparseMergeAndNullDeletion(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{"33": map[string]any{"4": map[string]any{
		"32901": map[string]any{"1": 32901, "2": 0, "3": int64(20), "4": int64(10)},
	}}})
	applyMap(t, s, map[string]any{"33": map[string]any{"4": map[string]any{
		"32901": map[string]any{"3": int64(30)},
	}}})
	got := s.ZooSouvenirs()[32901]
	if !got.IsReadObserved || got.IsRead != 0 || got.UpdatedAtMs != 30 || got.CreatedAtMs != 10 {
		t.Fatalf("sparse merge lost fields: %+v", got)
	}

	// A top-level null is not a map replacement in the client manager.
	applyMap(t, s, map[string]any{"33": map[string]any{"4": nil}})
	if !s.ZooSouvenirsObserved() || s.ZooSouvenirCount() != 1 {
		t.Fatalf("top-level null changed map: observed=%t souvenirs=%+v", s.ZooSouvenirsObserved(), s.ZooSouvenirs())
	}
	applyMap(t, s, map[string]any{"33": map[string]any{"4": map[string]any{"32901": nil}}})
	if !s.ZooSouvenirsObserved() || s.ZooSouvenirCount() != 0 || len(s.ZooSouvenirs()) != 0 {
		t.Fatalf("entry null did not delete: observed=%t souvenirs=%+v", s.ZooSouvenirsObserved(), s.ZooSouvenirs())
	}
}

func TestZooSouvenirMalformedDataFailsClosed(t *testing.T) {
	for name, value := range map[string]any{
		"null isRead":       nil,
		"fractional isRead": 0.5,
		"overflow isRead":   int64(2147483648),
	} {
		t.Run(name, func(t *testing.T) {
			s := New()
			applyMap(t, s, map[string]any{"33": map[string]any{"4": map[string]any{
				"32901": map[string]any{"1": 32901, "2": 0},
			}}})
			applyMap(t, s, map[string]any{"33": map[string]any{"4": map[string]any{
				"32901": map[string]any{"2": value},
			}}})
			if !s.ZooSouvenirsObserved() || s.ZooSouvenirCount() != 1 || len(s.UnreadZooSouvenirIDs()) != 0 {
				t.Fatalf("malformed isRead changed collection progress or remained unread: observed=%t count=%d souvenirs=%+v", s.ZooSouvenirsObserved(), s.ZooSouvenirCount(), s.ZooSouvenirs())
			}
			if got := s.ZooSouvenirs()[32901]; got.IsReadObserved {
				t.Fatalf("malformed isRead remained observed: %+v", got)
			}
		})
	}

	t.Run("non-key scalar fields do not change collection count", func(t *testing.T) {
		s := New()
		applyMap(t, s, map[string]any{"33": map[string]any{"4": map[string]any{
			"32901": map[string]any{"0": int64(123), "1": 32901, "2": 0, "3": int64(20), "4": int64(10)},
		}}})
		applyMap(t, s, map[string]any{"33": map[string]any{"4": map[string]any{
			"32901": map[string]any{"0": "bad", "1": 30201, "3": 20.5, "4": nil},
		}}})
		got := s.ZooSouvenirs()[32901]
		if !s.ZooSouvenirsObserved() || s.ZooSouvenirCount() != 1 || got.UIDObserved || got.TempIDObserved || got.UpdatedAtObserved || got.CreatedAtObserved || !got.IsReadObserved || got.IsRead != 0 {
			t.Fatalf("non-key malformed fields changed collection trust/count: observed=%t count=%d souvenir=%+v", s.ZooSouvenirsObserved(), s.ZooSouvenirCount(), got)
		}
	})

	for name, key := range map[string]string{
		"zero":       "0",
		"fractional": "32901.0",
		"overflow":   "2147483648",
	} {
		t.Run("key "+name, func(t *testing.T) {
			s := New()
			applyMap(t, s, map[string]any{"33": map[string]any{"4": map[string]any{
				key: map[string]any{"1": 32901, "2": 0},
			}}})
			if s.ZooSouvenirsObserved() || s.ZooSouvenirCount() != 0 {
				t.Fatalf("invalid key trusted: observed=%t souvenirs=%+v", s.ZooSouvenirsObserved(), s.ZooSouvenirs())
			}
		})
	}
}

func TestZooSouvenirMalformedMapCannotReuseStaleCount(t *testing.T) {
	malformedCases := map[string]any{
		"whole array":   []any{},
		"invalid key":   map[string]any{"32901.0": map[string]any{"1": 32901, "2": 0}},
		"invalid entry": map[string]any{"32901": "bad"},
	}
	for name, malformed := range malformedCases {
		t.Run(name, func(t *testing.T) {
			s := New()
			applyMap(t, s, map[string]any{"33": map[string]any{"4": map[string]any{
				"30201": map[string]any{"1": 30201, "2": 1},
				"32901": map[string]any{"1": 32901, "2": 0},
			}}})
			applyMap(t, s, map[string]any{"33": map[string]any{"4": malformed}})
			if s.ZooSouvenirsObserved() || s.ZooSouvenirCount() != 0 || len(s.ZooSouvenirs()) != 0 {
				t.Fatalf("malformed map retained stale collection state: observed=%t count=%d souvenirs=%+v", s.ZooSouvenirsObserved(), s.ZooSouvenirCount(), s.ZooSouvenirs())
			}

			applyMap(t, s, map[string]any{"33": map[string]any{"4": map[string]any{
				"32902": map[string]any{"1": 32902, "2": 1},
			}}})
			if !s.ZooSouvenirsObserved() || s.ZooSouvenirCount() != 1 {
				t.Fatalf("sparse recovery reused or lost keys: observed=%t count=%d souvenirs=%+v", s.ZooSouvenirsObserved(), s.ZooSouvenirCount(), s.ZooSouvenirs())
			}
		})
	}
}

func TestZooSouvenirRewardListObservedSemantics(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{"33": map[string]any{
		"0": map[string]any{"13": []int32{2, 1, 2}},
		"4": map[string]any{"32901": map[string]any{"1": 32901, "2": 0}},
	}})
	if zoo := s.Zoo(); !zoo.SouvenirRewardIDsObserved || !reflect.DeepEqual(zoo.SouvenirRewardIDs, []int32{1, 2}) {
		t.Fatalf("initial reward list=%+v", zoo)
	}

	// Missing field 13 is a sparse delta and preserves the last known list.
	applyMap(t, s, map[string]any{"33": map[string]any{"0": map[string]any{"8": int64(10)}}})
	if zoo := s.Zoo(); !zoo.SouvenirRewardIDsObserved || !reflect.DeepEqual(zoo.SouvenirRewardIDs, []int32{1, 2}) {
		t.Fatalf("missing field 13 did not preserve list: %+v", zoo)
	}

	for _, empty := range []any{nil, []int32{}} {
		applyMap(t, s, map[string]any{"33": map[string]any{"0": map[string]any{"13": empty}}})
		if zoo := s.Zoo(); !zoo.SouvenirRewardIDsObserved || len(zoo.SouvenirRewardIDs) != 0 {
			t.Fatalf("explicit empty reward list=%+v", zoo)
		}
	}

	for name, malformed := range map[string]any{
		"object":     map[string]any{"1": 1},
		"fractional": []any{1, 2.5},
		"zero":       []int32{0},
		"overflow":   []int64{2147483648},
	} {
		t.Run(name, func(t *testing.T) {
			applyMap(t, s, map[string]any{"33": map[string]any{"0": map[string]any{"13": malformed}}})
			if zoo := s.Zoo(); zoo.SouvenirRewardIDsObserved || len(zoo.SouvenirRewardIDs) != 0 {
				t.Fatalf("malformed reward list remained observed: %+v", zoo)
			}
		})
	}
}

func TestZooSouvenirCatalogThresholdsAndClaimHoles(t *testing.T) {
	second, ok := ZooSouvenirCollectInfoByIndex(2)
	if !ok || second.Required != 2 || len(second.Reward) != 2 || second.Reward[0] != (ItemCount{ItemID: 1, Count: 10}) || second.Reward[1] != (ItemCount{ItemID: 957, Count: 2}) {
		t.Fatalf("milestone 2=%+v ok=%t", second, ok)
	}
	last, ok := ZooSouvenirCollectInfoByIndex(30)
	if !ok || last.Required != 60 {
		t.Fatalf("milestone 30=%+v ok=%t", last, ok)
	}
	milestones := ZooSouvenirCollectMilestones()
	if len(milestones) != 30 || milestones[10].Index != 11 || milestones[10].Required != 12 || milestones[20].Index != 21 || milestones[20].Required != 33 {
		t.Fatalf("milestones invariant failed: len=%d row11=%+v row21=%+v", len(milestones), milestones[10], milestones[20])
	}

	s := New()
	entries := map[string]any{}
	for _, id := range []int32{30201, 32901, 32902, 32903} {
		entries[itoaState(int(id))] = map[string]any{"1": id, "2": 1}
	}
	applyMap(t, s, map[string]any{"33": map[string]any{
		"0": map[string]any{"13": []int32{1, 3}},
		"4": entries,
	}})
	if got := s.ReadyZooSouvenirRewardIDs(); !reflect.DeepEqual(got, []int32{2, 4}) {
		t.Fatalf("ready rewards with claimed hole=%v, want [2 4]", got)
	}
	if !s.ZooSouvenirRewardsReady([]int32{2, 4}) || s.ZooSouvenirRewardsReady([]int32{2, 5}) {
		t.Fatalf("reward preflight mismatch ready=%v", s.ReadyZooSouvenirRewardIDs())
	}
}

func TestZooSouvenirReadAndRewardPostconditions(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{"33": map[string]any{
		"0": map[string]any{"13": nil},
		"4": map[string]any{"32901": map[string]any{"1": 32901, "2": 0}},
	}})
	if !s.ZooSouvenirsUnread([]int32{32901}) || s.ZooSouvenirsAcknowledged([]int32{32901}) {
		t.Fatalf("initial read pre/post mismatch: %+v", s.ZooSouvenirs())
	}
	if s.ZooSouvenirsReadyToAcknowledge([]int32{32901}) {
		t.Fatal("unread souvenir became readable before its ready reward was claimed")
	}
	if !s.ZooSouvenirRewardsReady([]int32{1}) || s.ZooSouvenirRewardsClaimed([]int32{1}) {
		t.Fatalf("initial reward pre/post mismatch: ready=%v zoo=%+v", s.ReadyZooSouvenirRewardIDs(), s.Zoo())
	}
	applyMap(t, s, map[string]any{"33": map[string]any{"0": map[string]any{"13": []int32{1}}}})
	if !s.ZooSouvenirRewardsClaimed([]int32{1}) {
		t.Fatal("claimed reward did not satisfy postcondition")
	}
	if !s.ZooSouvenirsReadyToAcknowledge([]int32{32901}) {
		t.Fatal("unread souvenir did not become readable after all rewards were claimed")
	}
	applyMap(t, s, map[string]any{"33": map[string]any{"4": map[string]any{"32901": nil}}})
	if !s.ZooSouvenirsAcknowledged([]int32{32901}) {
		t.Fatal("explicit null deletion did not satisfy read postcondition")
	}
}
