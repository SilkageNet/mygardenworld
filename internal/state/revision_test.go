package state

import (
	"encoding/json"
	"testing"
)

func TestRevisionAdvancesForServerAndCatalogMutations(t *testing.T) {
	s := New()
	initial := s.Revision()
	s.SetFarmLands([]FarmLandInfo{{ID: 1001, OpenLevel: 1}})
	afterLocal := s.Revision()
	if afterLocal <= initial {
		t.Fatalf("revision after local mutation=%d, want > %d", afterLocal, initial)
	}
	s.ApplyV(json.RawMessage(`{"7":{"0":{"44":10}}}`))
	afterServer := s.Revision()
	if afterServer <= afterLocal {
		t.Fatalf("revision after server apply=%d, want > %d", afterServer, afterLocal)
	}
}
