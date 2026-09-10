package state

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFmlMembershipSparseMerge(t *testing.T) {
	for _, tc := range []struct {
		name, member     string
		guild, position  int32
		positionObserved bool
	}{
		{"contribution", `{"3":10}`, 88, 1, true},
		{"position", `{"2":2}`, 88, 2, true},
		{"same_guild", `{"1":88}`, 88, 1, true},
		{"empty", `{}`, 88, 1, true},
		{"malformed", `[]`, 88, 1, true},
		{"invalid_id", `{"1":"bad","2":2}`, 88, 1, true},
		{"other_user", `{"0":22,"1":99,"2":2}`, 88, 1, true},
		{"invalid_user", `{"0":"bad","1":99}`, 88, 1, true},
		{"null_user", `{"0":null,"1":99}`, 88, 1, true},
		{"zero_user", `{"0":0,"1":99}`, 88, 1, true},
		{"null_id", `{"1":null}`, 88, 1, true},
		{"leave", `null`, 0, 0, false},
		{"zero_id", `{"1":0,"2":2}`, 0, 0, false},
		{"switch_guild", `{"1":99}`, 99, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			s.ApplyV(json.RawMessage(`{"7":{"0":{"0":11}},"25":{"1":{"0":11,"1":88,"2":1}}}`))
			s.ApplyV(json.RawMessage(`{"25":{"1":` + tc.member + `}}`))
			got := s.FmlBuild()
			if !got.MembershipObserved || got.MemberFmlID != tc.guild || got.MemberPosition != tc.position || got.MemberPositionObserved != tc.positionObserved {
				t.Fatalf("membership=%+v", got)
			}
		})
	}
}

func TestFmlMembershipUnknownStaysUnknownOnPartialMember(t *testing.T) {
	for _, member := range []string{`{}`, `{"3":2}`, `{"2":1}`, `[]`, `{"1":"bad"}`} {
		s := New()
		s.ApplyV(json.RawMessage(`{"25":{"1":` + member + `}}`))
		if got := s.FmlBuild(); got.MembershipObserved || got.MemberPositionObserved {
			t.Fatalf("partial %s must not prove membership: %+v", member, got)
		}
	}
}

func TestFmlMembershipRecoveryRequiresFreshEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, delta string
		observed    bool
		guild       int32
	}{
		{"empty_ack", `{}`, false, 0},
		{"race_only", `{"25":{"111":{"0":42,"1":1}}}`, false, 0},
		{"fresh_guild", `{"25":{"0":{"0":88}}}`, true, 88},
		{"fresh_member", `{"25":{"1":{"0":11,"1":88,"2":1}}}`, true, 88},
		{"member_list", `{"25":{"2":[{"0":22,"1":99},{"0":11,"1":88,"2":2}]}}`, true, 88},
		{"partial_with_list", `{"25":{"1":{"3":12},"2":[{"0":11,"1":88}]}}`, true, 88},
		{"null_wins", `{"25":{"0":{"0":88},"1":null,"2":[{"0":11,"1":88}]}}`, true, 0},
		{"list_leave_wins", `{"25":{"0":{"0":88},"2":[{"0":11,"1":0}]}}`, true, 0},
		{"explicit_member_wins", `{"25":{"1":{"0":11,"1":88},"2":[{"0":11,"1":99,"2":1}]}}`, true, 88},
		{"malformed_no_fallback", `{"25":{"0":{"0":88},"1":[]}}`, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			s.ApplyV(json.RawMessage(`{"7":{"0":{"0":11}},"25":{"0":{"0":88},"1":{"1":88,"2":1}}}`))
			s.MarkFmlMembershipUncertainAt(time.Now())
			s.ApplyVFmlMembership(json.RawMessage(tc.delta))
			got := s.FmlBuild()
			if got.MembershipObserved != tc.observed || got.MemberFmlID != tc.guild {
				t.Fatalf("recovery=%+v", got)
			}
		})
	}
	s := New()
	s.MarkFmlMembershipUncertainAt(time.Now())
	s.ApplyV(json.RawMessage(`{"25":{"0":{"0":88}}}`))
	if s.FmlBuild().MembershipObserved {
		t.Fatal("ordinary guild delta must not confirm recovery")
	}
}

func TestFmlMembershipSwitchInvalidatesOldGuildFacts(t *testing.T) {
	for _, reconnect := range []bool{false, true} {
		s := New()
		s.ApplyV(json.RawMessage(`{"25":{"1":{"1":88,"2":1},"111":{"0":42,"1":1},"114":[{"0":1,"4":4001,"10":9}],"102":{"1":{"1":{"1":23001,"3":99}}}}`))
		if reconnect {
			s.BeginFmlMembershipSnapshot()
		} else {
			s.MarkNoFmlMembership()
		}
		s.ApplyVFmlMembership(json.RawMessage(`{"25":{"0":{"0":99,"19":2},"1":{"1":99}}}`))
		if s.FmlRace().Observed || s.FmlRace().TasksObserved || s.FmlLandObserved() || len(s.FmlLands()) != 0 {
			t.Fatal("old guild facts survived switch")
		}
		got := s.FmlBuild()
		if got.MemberFmlID != 99 || got.TodayBuildNum != 2 || got.MemberPositionObserved {
			t.Fatalf("new guild data lost or old position retained: %+v", got)
		}
	}
}
