package state

import (
	"encoding/json"
	"testing"
)

func raceQuotaFixture() *State {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":99}},"25":{"111":{"0":42,"1":1},"117":{"5":4},"110":{"42":{"0":99,"1":42,"3":18,"6":0}}}}`))
	return s
}

func TestRaceQuotaReopensOnlyForAdditionalUsableSlots(t *testing.T) {
	for _, tc := range []struct {
		name          string
		delta         string
		finished, buy int32
		reopened      bool
	}{
		{"personal record", `{"110":{"42":{"3":18,"6":2}}}`, 18, 2, true},
		{"purchase delta", `{"110":{"42":{"6":2}}}`, 18, 2, true},
		{"rank list", `{"116":[{"0":99,"1":42,"3":18,"6":2}]}`, 18, 2, true},
		{"already consumed", `{"110":{"42":{"3":20,"6":2}}}`, 20, 2, false},
		{"consumed after lagging rank in same response", `{"116":[{"0":99,"1":42,"3":18,"6":2}],"110":{"42":{"3":20,"6":2}}}`, 20, 2, false},
		{"same counters", `{"110":{"42":{"3":18,"6":0}}}`, 18, 0, false},
		{"older counters", `{"110":{"42":{"3":1,"6":0}}}`, 18, 0, false},
		{"pool only", `{"114":[]}`, 18, 0, false},
		{"sparse timestamps", `{"110":{"42":{"9":12345}}}`, 18, 0, false},
		{"null purchase", `{"110":{"42":{"6":null}}}`, 18, 0, false},
		{"negative purchase", `{"110":{"42":{"6":-1}}}`, 18, 0, false},
		{"other batch key", `{"110":{"41":{"3":0,"6":2}}}`, 18, 0, false},
		{"other batch identity", `{"110":{"42":{"1":41,"3":0,"6":2}}}`, 18, 0, false},
		{"other member", `{"110":{"42":{"0":100,"3":0,"6":2}}}`, 18, 0, false},
		{"other member rank", `{"116":[{"0":100,"1":42,"3":0,"6":2}]}`, 18, 0, false},
		{"old batch rank", `{"116":[{"0":99,"1":41,"3":0,"6":2}]}`, 18, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := raceQuotaFixture()
			s.MarkFmlRaceTakeQuotaExhausted()
			s.ApplyV(json.RawMessage(`{"25":` + tc.delta + `}`))
			v := s.FmlRace()
			if v.TakeQuotaExhausted == tc.reopened || v.FinishedTaskNum != tc.finished || v.BuyTaskNum != tc.buy {
				t.Fatalf("unexpected quota: %+v", v)
			}
		})
	}
}

func TestRaceQuotaRejectionBaselineSurvivesLaggingSnapshots(t *testing.T) {
	s := raceQuotaFixture()
	s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"3":19,"6":2}}}}`))
	s.MarkFmlRaceTakeQuotaExhausted()
	for _, delta := range []string{
		`{"110":{"42":{"3":18,"6":0}}}`,
		`{"116":[{"0":99,"1":42,"3":18,"6":0}]}`,
		`{"110":{"42":{"3":19,"6":2}}}`,
	} {
		s.ApplyV(json.RawMessage(`{"25":` + delta + `}`))
		v := s.FmlRace()
		if !v.TakeQuotaExhausted || v.FinishedTaskNum != 19 || v.BuyTaskNum != 2 {
			t.Fatalf("old snapshot changed quota or reopened rejection: %+v", v)
		}
	}
	s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"6":3}}}}`))
	if s.FmlRace().TakeQuotaExhausted {
		t.Fatal("additional purchased slot did not reopen quota")
	}
	s.MarkFmlRaceTakeQuotaExhausted()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":43,"1":1},"110":{"42":{"3":19,"6":3},"43":{"3":0,"6":0}}}}`))
	v := s.FmlRace()
	if v.TakeQuotaExhausted || v.FinishedTaskNum != 0 || v.BuyTaskNum != 0 {
		t.Fatalf("new batch inherited old quota: %+v", v)
	}
}

func TestRacePurchaseOnlyCannotEstablishFinishedCount(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"111":{"0":42,"1":1},"117":{"5":4}}}`))
	s.MarkFmlRaceTakeQuotaExhausted()
	s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"3":null,"6":2}}}}`))
	if v := s.FmlRace(); v.TaskQuotaObserved || !v.TakeQuotaExhausted || v.BuyTaskNum != 2 {
		t.Fatalf("purchase inferred missing finished count: %+v", v)
	}
	s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"3":18}}}}`))
	if v := s.FmlRace(); !v.TaskQuotaObserved || v.TakeQuotaExhausted {
		t.Fatalf("observed remaining quota not recovered: %+v", v)
	}
}

func TestRaceQuotaOnlyOrForeignRecordsPreserveHeldTask(t *testing.T) {
	for _, delta := range []string{
		`{"110":{"42":{"6":2}}}`,
		`{"110":{"41":{"3":0,"6":2}}}`,
		`{"110":{"42":{"0":100,"3":0,"6":2}}}`,
	} {
		s := raceQuotaFixture()
		s.ApplyV(json.RawMessage(`{"25":{"110":{"42":{"7":{"0":7,"1":3036,"2":560,"3":12}}}}}`))
		before := s.FmlRace().Taken
		if !before.HasTask {
			t.Fatal("fixture missing held task")
		}
		s.ApplyV(json.RawMessage(`{"25":` + delta + `}`))
		if got := s.FmlRace().Taken; got != before {
			t.Fatalf("unrelated delta cleared held task: %s", delta)
		}
	}
}
