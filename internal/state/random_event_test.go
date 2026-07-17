package state

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRandomEventCatalogAndObservedCaptureSemantics(t *testing.T) {
	definition, ok := RandomEventDefinition(6004)
	if !ok || !definition.CostFree || definition.PlaceCount != 3 {
		t.Fatalf("RandomEventDefinition(6004)=%+v ok=%t", definition, ok)
	}
	dialogFound := false
	for _, id := range definition.DialogIDs {
		if id == 60040901 {
			dialogFound = true
		}
	}
	if !dialogFound {
		t.Fatalf("6004 dialogs=%v, missing captured dialog", definition.DialogIDs)
	}

	s := New()
	s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{"6004":{"0":6004,"1":2,"2":60040901},"6008":{"0":6008,"1":0,"2":60080101}}}}}`))
	if got, want := s.ReadyRandomEventIDs(), []int32{6004, 6008}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ReadyRandomEventIDs=%v, want %v", got, want)
	}
	events := s.RandomEvents()
	if events[6004].PositionIndex != 2 || events[6004].DialogID != 60040901 || !events[6004].Valid {
		t.Fatalf("event 6004=%+v", events[6004])
	}
	if events[6008].PositionIndex != 0 || !events[6008].Valid {
		t.Fatalf("event 6008=%+v", events[6008])
	}
}

func TestRandomEventWholeReplacementSparseRetentionAndExplicitClear(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{"6004":{"0":6004,"1":2,"2":60040901},"6008":{"0":6008,"1":0,"2":60080101}}}}}`))

	// A sparse delta that omits 129.0.1 must retain the prior table.
	s.ApplyV(json.RawMessage(`{"129":{"0":{"2":1234}}}`))
	if got := s.ReadyRandomEventIDs(); !reflect.DeepEqual(got, []int32{6004, 6008}) {
		t.Fatalf("sparse delta replaced events: %v", got)
	}

	// Presence of 129.0.1 replaces the complete table.
	s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{"6008":{"0":6008,"1":0,"2":60080101}}}}}`))
	if got := s.ReadyRandomEventIDs(); !reflect.DeepEqual(got, []int32{6008}) {
		t.Fatalf("whole replacement events=%v", got)
	}

	for name, raw := range map[string]string{
		"empty object": `{"129":{"0":{"1":{}}}}`,
		"null":         `{"129":{"0":{"1":null}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			s.ApplyV(json.RawMessage(raw))
			if !s.RandomEventTableReady() || len(s.RandomEvents()) != 0 || len(s.ReadyRandomEventIDs()) != 0 {
				t.Fatalf("explicit clear not retained as valid empty table: status=%v events=%v", s.RandomEventTableReady(), s.RandomEvents())
			}
		})
	}
}

func TestRandomEventMalformedAndUnsafeEntriesFailClosed(t *testing.T) {
	t.Run("malformed replacement clears executable view", func(t *testing.T) {
		s := New()
		s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{"6008":{"0":6008,"1":0,"2":60080101}}}}}`))
		s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{"6008":{"0":6008,"1":"zero","2":60080101}}}}}`))
		observed, valid, reason := s.RandomEventMapStatus()
		if !observed || valid || reason == "" || len(s.ReadyRandomEventIDs()) != 0 || len(s.RandomEvents()) != 0 {
			t.Fatalf("malformed status observed=%t valid=%t reason=%q events=%v", observed, valid, reason, s.RandomEvents())
		}
	})

	tests := []struct {
		name    string
		payload string
		want    string
	}{
		{name: "key mismatch", payload: `{"6004":{"0":6008,"1":0,"2":60080101}}`, want: "不一致"},
		{name: "invalid position", payload: `{"6008":{"0":6008,"1":1,"2":60080101}}`, want: "posIdx"},
		{name: "invalid dialog", payload: `{"6008":{"0":6008,"1":0,"2":60040901}}`, want: "dialogId"},
		{name: "unknown catalog", payload: `{"6999":{"0":6999,"1":0,"2":69990101}}`, want: "c_randomEvent"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			s.ApplyV(json.RawMessage(`{"129":{"0":{"1":` + tc.payload + `}}}`))
			observed, valid, reason := s.RandomEventMapStatus()
			if !observed || !valid || reason != "" || len(s.ReadyRandomEventIDs()) != 0 {
				t.Fatalf("status observed=%t valid=%t reason=%q ready=%v", observed, valid, reason, s.ReadyRandomEventIDs())
			}
			var event RandomEventView
			for _, candidate := range s.RandomEvents() {
				event = candidate
			}
			if event.Valid || !strings.Contains(event.BlockedReason, tc.want) {
				t.Fatalf("unsafe event=%+v, want reason containing %q", event, tc.want)
			}
		})
	}
}

func TestRandomEventClaimSnapshotRequiresAuthoritativeRemoval(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{"6008":{"0":6008,"1":0,"2":60080101}}}}}`))
	snapshot, ok := s.RandomEventClaimSnapshot(6008)
	if !ok || snapshot.EventID != 6008 || snapshot.PositionIndex != 0 || snapshot.DialogID != 60080101 {
		t.Fatalf("snapshot=%+v ok=%t", snapshot, ok)
	}
	s.ApplyV(json.RawMessage(`{"129":{"0":{"2":1234}}}`))
	if s.RandomEventClaimApplied(snapshot) {
		t.Fatal("sparse delta incorrectly satisfied removal postcondition")
	}
	s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{}}}}`))
	if !s.RandomEventClaimApplied(snapshot) {
		t.Fatal("authoritative empty table did not satisfy removal postcondition")
	}
}
