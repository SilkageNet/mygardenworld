package runner

import (
	"encoding/json"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestNormalizeFmlRaceEnterVWrapsBareUsrRankList(t *testing.T) {
	// getFmlRaceUsrRankList can answer with only the rank list at the top
	// level; without the wrap ApplyV would route 116 to the benefit-box
	// namespace instead of 25.116 and the quota would never recover.
	bare := json.RawMessage(`{"116":[{"0":82665,"1":1786291200000,"3":2,"6":1,"9":1786300000000}]}`)
	got := normalizeFmlRaceEnterV(bare)
	var top map[string]json.RawMessage
	if err := json.Unmarshal(got, &top); err != nil {
		t.Fatal(err)
	}
	if _, ok := top["25"]; !ok {
		t.Fatalf("expected wrap under 25, got %s", got)
	}

	s := state.New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":82665}},"25":{"111":{"0":1786291200000,"1":1,"2":1786410000000,"3":1786885200000}}}`))
	if s.FmlRace().TaskQuotaObserved {
		t.Fatal("precondition: quota must start unobserved")
	}
	s.ApplyV(got)
	view := s.FmlRace()
	if !view.TaskQuotaObserved || view.FinishedTaskNum != 2 || view.BuyTaskNum != 1 {
		t.Fatalf("normalized rank list must recover quota: %+v", view)
	}
}
