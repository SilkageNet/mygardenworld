package state

import (
	"encoding/json"
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
	if got := view.FailedUntilMs[2001]; got != at.Add(time.Minute).UnixMilli() {
		t.Fatalf("failure until=%d", got)
	}
	if !time.UnixMilli(view.FailedUntilMs[2001]).After(at.Add(time.Minute - time.Millisecond)) {
		t.Fatal("candidate should still be cooling before 60s")
	}
	if time.UnixMilli(view.FailedUntilMs[2001]).After(at.Add(time.Minute)) {
		t.Fatal("candidate should be eligible at exactly 60s")
	}
	s.SkipPearlHireCandidate(2002)
	if _, skipped := s.PearlHire().SkippedUIDs[2002]; !skipped {
		t.Fatal("session candidate skip was not recorded")
	}
	s.LockPearlHireSession("fallback")
	s.ResetPearlHireSession()
	view = s.PearlHire()
	if view.SessionLocked || len(view.FailedUntilMs) != 0 || len(view.SkippedUIDs) != 0 || view.FriendsObserved || view.RecommendObserved || view.EnemiesObserved {
		t.Fatalf("session reset incomplete: %+v", view)
	}
}

func TestPearlHireCatalogConstants(t *testing.T) {
	config, ok := PearlHireConfigFromCatalog()
	if !ok || config.TicketItemID != 1003 || config.RestTimeSeconds != 3600 || config.EnemyMaxDays != 3 ||
		len(config.Slots) != 4 || !config.Slots[4].MonthlyCardUnlock {
		t.Fatalf("pearl hire config = %+v, %t", config, ok)
	}
}

func TestPearlHireDailyUsageUsesShanghaiCalendarBoundary(t *testing.T) {
	s := New()
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	beforeMidnight := time.Date(2026, 8, 29, 23, 59, 59, 0, shanghai)
	s.SetPearlHireTicketUsed(PearlHireTicketDayID(beforeMidnight), 3)
	if got := s.PearlHireAt(beforeMidnight).TicketUsedToday; got != 3 {
		t.Fatalf("used before midnight=%d, want 3", got)
	}
	afterMidnight := beforeMidnight.Add(time.Second)
	if got := s.PearlHireAt(afterMidnight).TicketUsedToday; got != 0 {
		t.Fatalf("used after midnight=%d, want 0", got)
	}
	s.NotePearlHireTicketUsed(afterMidnight)
	if got := s.PearlHireAt(afterMidnight).TicketUsedToday; got != 1 {
		t.Fatalf("used after first next-day spend=%d, want 1", got)
	}
	if got := s.PearlHireAt(beforeMidnight).TicketUsedToday; got != 0 {
		t.Fatalf("prior-day query after rollover=%d, want 0", got)
	}
}

func TestPearlHireTicketDecreasedRequiresExactOneTicketDelta(t *testing.T) {
	s := New()
	at := time.UnixMilli(1_800_000_000_000)
	s.ApplyV(json.RawMessage(`{"7":{"0":{"32":{"1003":3}}}}`))
	snapshot := PearlHireAttemptSnapshot{At: at, TicketCount: 3}
	if s.PearlHireTicketDecreased(snapshot) {
		t.Fatal("unchanged ticket count reported as spent")
	}
	s.ApplyV(json.RawMessage(`{"7":{"0":{"32":{"1003":2}}}}`))
	if !s.PearlHireTicketDecreased(snapshot) {
		t.Fatal("exact one-ticket decrement was not observed")
	}
	s.ApplyV(json.RawMessage(`{"7":{"0":{"32":{"1003":1}}}}`))
	if s.PearlHireTicketDecreased(snapshot) {
		t.Fatal("two-ticket decrement was accepted")
	}
}
