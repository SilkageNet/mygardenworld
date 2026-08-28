package state

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
	"time"
)

func pearlHireFixture(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile("testdata/pearl_hire_sparse.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func TestPearlHireSparseStateAndStrictUIDs(t *testing.T) {
	fixture := pearlHireFixture(t)
	s := New()
	s.ApplyV(fixture["initial"])
	s.ApplyV(fixture["friends_full"])
	view := s.PearlHire()
	if !view.FriendsObserved || !reflect.DeepEqual(view.FriendUIDs, []int64{2001, 2002}) {
		t.Fatalf("full friends = observed:%t uids:%v", view.FriendsObserved, view.FriendUIDs)
	}
	s.ApplyV(fixture["friends_delta"])
	if got := s.PearlHire().FriendUIDs; !reflect.DeepEqual(got, []int64{2001, 2002, 2003}) {
		t.Fatalf("delta friends = %v", got)
	}

	before := s.PearlHire().FriendUIDs
	s.ApplyV(json.RawMessage(`{"24":{"0":{"0":9001},"1":[{"0":1.5,"1":9001}]}}`))
	if got := s.PearlHire().FriendUIDs; !reflect.DeepEqual(got, before) {
		t.Fatalf("fractional UID changed friends: %v", got)
	}
	s.ApplyV(json.RawMessage(`{"24":{"0":{"0":9001},"1":[{"0":"9223372036854775808","1":9001}]}}`))
	if got := s.PearlHire().FriendUIDs; !reflect.DeepEqual(got, before) {
		t.Fatalf("overflow UID changed friends: %v", got)
	}
}

func TestPearlHireMalformedCollectionsRemainUnknown(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		known func(PearlHireView) bool
	}{
		{name: "friends null", raw: `{"24":{"1":null}}`, known: func(v PearlHireView) bool { return v.FriendsObserved }},
		{name: "friends malformed", raw: `{"24":{"1":"bad"}}`, known: func(v PearlHireView) bool { return v.FriendsObserved }},
		{name: "recommend null", raw: `{"115":{"6":null}}`, known: func(v PearlHireView) bool { return v.RecommendObserved }},
		{name: "recommend malformed", raw: `{"115":{"6":{"bad":1}}}`, known: func(v PearlHireView) bool { return v.RecommendObserved }},
		{name: "enemies null", raw: `{"115":{"1":{"5":null}}}`, known: func(v PearlHireView) bool { return v.EnemiesObserved }},
		{name: "enemies malformed", raw: `{"115":{"1":{"5":{"1.5":123}}}}`, known: func(v PearlHireView) bool { return v.EnemiesObserved }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			s.ApplyV(json.RawMessage(tc.raw))
			if tc.known(s.PearlHire()) {
				t.Fatal("malformed collection was marked observed")
			}
		})
	}
}

func TestPearlHireSubsetMergeReplaceAndProfileTTL(t *testing.T) {
	fixture := pearlHireFixture(t)
	s := New()
	s.ApplyV(fixture["initial"])
	s.ApplyV(fixture["candidate_subset"])
	view := s.PearlHire()
	if len(view.Profiles) != 2 || len(view.HireStates) != 2 || !reflect.DeepEqual(view.RecommendUIDs, []int64{2001, 2002}) {
		t.Fatalf("candidate subset = %+v", view)
	}
	profile := view.Profiles[2001]
	if !profile.LevelObserved || profile.Level != 12 || profile.ObservedAtMs <= 0 {
		t.Fatalf("profile = %+v", profile)
	}

	s.mu.Lock()
	s.pearlProfiles[2001].ObservedAtMs = 123
	s.mu.Unlock()
	s.ApplyV(json.RawMessage(`{"28":{"5":[{"0":2001,"1":"renamed-only"}]}}`))
	if got := s.PearlHire().Profiles[2001]; got.ObservedAtMs != 123 || got.Name != "renamed-only" {
		t.Fatalf("partial profile refreshed level TTL: %+v", got)
	}

	s.ApplyV(json.RawMessage(`{"115":{"5":{"2001":null,"2003":0},"6":[2003]}}`))
	view = s.PearlHire()
	if _, exists := view.HireStates[2001]; exists {
		t.Fatal("explicit null did not delete hire-state entry")
	}
	if _, exists := view.HireStates[2003]; !exists || !reflect.DeepEqual(view.RecommendUIDs, []int64{2003}) {
		t.Fatalf("sparse hire merge/recommend replacement = %+v", view)
	}
}

func TestPearlHireFailureBoundaryAndSessionReset(t *testing.T) {
	s := New()
	at := time.UnixMilli(1_700_000_000_000)
	s.MarkPearlHireFailed(2001, at)
	view := s.PearlHire()
	if got := view.FailedUntilMs[2001]; got != math.MaxInt64 {
		t.Fatalf("failure until=%d, want MaxInt64", got)
	}
	if !(view.FailedUntilMs[2001] > at.Add(24*time.Hour).UnixMilli()) {
		t.Fatal("failed UID should remain excluded for the login session")
	}
	s.LockPearlHireSession("fallback")
	s.ResetPearlHireSession()
	view = s.PearlHire()
	if view.SessionLocked || len(view.FailedUntilMs) != 0 || view.FriendsObserved || view.RecommendObserved || view.EnemiesObserved {
		t.Fatalf("session reset incomplete: %+v", view)
	}
}

func TestPearlHireDailyTicketUsageAndWorldEmpty(t *testing.T) {
	s := New()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s.NotePearlHireTicketUsed(now)
	s.NotePearlHireTicketUsed(now)
	view := s.PearlHireAt(now)
	if view.TicketUsedToday != 2 {
		t.Fatalf("ticket used today=%d", view.TicketUsedToday)
	}
	if got := PearlHireTicketDayID(now); got != 20260828 {
		t.Fatalf("day id=%d", got)
	}
	justAfterMidnight := time.Date(2026, 8, 29, 0, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	if got := s.PearlHireAt(justAfterMidnight).TicketUsedToday; got != 0 {
		t.Fatalf("ticket used after midnight=%d", got)
	}
	beforeMidnight := time.Date(2026, 8, 28, 23, 59, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	if got := s.PearlHireAt(beforeMidnight).TicketUsedToday; got != 2 {
		t.Fatalf("ticket used before midnight=%d", got)
	}
	s.MarkPearlHireWorldEmpty(now)
	view = s.PearlHireAt(now)
	if view.WorldEmptyUntilMs != now.Add(time.Minute).UnixMilli() {
		t.Fatalf("world empty until=%d", view.WorldEmptyUntilMs)
	}
	if view.FriendsObserved || view.RecommendObserved || view.EnemiesObserved {
		t.Fatalf("candidate caches should be invalidated: %+v", view)
	}
	s.NotePearlHireTicketUsed(now)
	if s.PearlHireAt(now).WorldEmptyUntilMs != 0 {
		t.Fatal("successful hire should clear world-empty wait")
	}
	s.SetPearlHireTicketUsed(20260828, 5)
	if got := s.PearlHireAt(now).TicketUsedToday; got != 5 {
		t.Fatalf("hydrated ticket used=%d", got)
	}
}

func TestPearlHireCatalogConstants(t *testing.T) {
	config, ok := PearlHireConfigFromCatalog()
	if !ok || config.TicketItemID != 1003 || config.RestTimeSeconds != 3600 || config.EnemyMaxDays != 3 ||
		len(config.Slots) != 4 || !config.Slots[4].MonthlyCardUnlock {
		t.Fatalf("pearl hire config = %+v, %t", config, ok)
	}
}
