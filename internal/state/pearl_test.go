package state

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestPearlProductionTimingFromCatalog(t *testing.T) {
	timing, ok := PearlProductionTimingFromCatalog()
	if !ok {
		t.Fatal("pearl production timing missing")
	}
	if timing.HireTimeSeconds != 7200 || timing.GatherCDSeconds != 180 {
		t.Fatalf("timing=%+v, want hire=7200 gather=180", timing)
	}
}

func TestPearlReceivableCountBoundariesAndEndCap(t *testing.T) {
	end := time.UnixMilli(7_200_000)
	base := PearlPlaceView{
		PlaceID: 1, LaborEndTime: end.UnixMilli(), EveryMakeNum: 5,
		LaborEndTimeObserved: true, EveryMakeNumObserved: true,
		RecvCntObserved: true, SurplusRecvNumObserved: true,
	}
	tests := []struct {
		name   string
		now    time.Time
		mutate func(*PearlPlaceView)
		want   int64
		known  bool
	}{
		{name: "before start", now: time.UnixMilli(-1), want: 0, known: true},
		{name: "179 seconds", now: time.UnixMilli(179_000), want: 0, known: true},
		{name: "180 seconds", now: time.UnixMilli(180_000), want: 5, known: true},
		{name: "359 seconds", now: time.UnixMilli(359_000), want: 5, known: true},
		{name: "360 seconds", now: time.UnixMilli(360_000), want: 10, known: true},
		{name: "far after end capped", now: end.Add(72 * time.Hour), want: 200, known: true},
		{name: "recv count and surplus", now: time.UnixMilli(540_000), mutate: func(p *PearlPlaceView) { p.RecvCnt = 1; p.SurplusRecvNum = 2 }, want: 12, known: true},
		{name: "recv count ahead uses surplus", now: time.UnixMilli(180_000), mutate: func(p *PearlPlaceView) { p.RecvCnt = 4; p.SurplusRecvNum = 3 }, want: 3, known: true},
		{name: "explicit empty ignores surplus like client", now: end, mutate: func(p *PearlPlaceView) { p.LaborEndTime = 0; p.EveryMakeNum = 0; p.SurplusRecvNum = 9 }, want: 0, known: true},
		{name: "missing recv observation", now: end, mutate: func(p *PearlPlaceView) { p.RecvCntObserved = false }, want: 0, known: false},
		{name: "negative value rejected", now: end, mutate: func(p *PearlPlaceView) { p.RecvCnt = -1 }, want: 0, known: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			place := base
			if tc.mutate != nil {
				tc.mutate(&place)
			}
			got, known := PearlReceivableCount(place, tc.now)
			if got != tc.want || known != tc.known {
				t.Fatalf("PearlReceivableCount=%d,%t want %d,%t place=%+v", got, known, tc.want, tc.known, place)
			}
		})
	}
}

func TestPearlRecvOneKeyCaptureFixtureTotals560AndClearsSlots(t *testing.T) {
	var fixture struct {
		NowMS         int64           `json:"now_ms"`
		ExpectedCount int64           `json:"expected_count"`
		Before        json.RawMessage `json:"before"`
		Response      json.RawMessage `json:"response"`
	}
	raw, err := os.ReadFile("testdata/pearl_recv_one_key.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}

	s := New()
	s.ApplyV(fixture.Before)
	now := time.UnixMilli(fixture.NowMS)
	var total int64
	for _, place := range s.PearlPlaces() {
		count, known := PearlReceivableCount(place, now)
		if !known {
			t.Fatalf("place %d count unknown: %+v", place.PlaceID, place)
		}
		total += count
	}
	if total != fixture.ExpectedCount {
		t.Fatalf("total=%d, want %d", total, fixture.ExpectedCount)
	}
	if got := s.ReadyPearlPlaceIDsAt(now); !reflect.DeepEqual(got, []int32{1, 2, 3}) {
		t.Fatalf("ready=%v, want [1 2 3]", got)
	}
	snapshot, ok := s.PearlClaimSnapshot(now)
	if !ok {
		t.Fatal("claim snapshot unavailable")
	}

	s.ApplyV(fixture.Response)
	if !s.PearlClaimApplied(snapshot) {
		t.Fatal("capture-shaped recvOneKey response did not satisfy claim postcondition")
	}
	if got := s.Inventory()[1006]; got != 800 {
		t.Fatalf("pearl inventory=%d, want captured absolute total 800", got)
	}
	for id, place := range s.PearlPlaces() {
		if place.LaborEndTime != 0 || !place.LaborEndTimeObserved ||
			place.EveryMakeNum != 0 || place.RecvCnt != 0 || place.SurplusRecvNum != 0 {
			t.Fatalf("place %d not cleared: %+v", id, place)
		}
	}
}

func TestApplyPearlPlaceSparseNullAndEntryDeletion(t *testing.T) {
	now := time.UnixMilli(8_000_000)
	s := New()
	s.ApplyVMap(map[string]any{"115": map[string]any{"0": map[string]any{
		"1": map[string]any{"1": 1, "3": int64(7_200_000), "6": 5, "7": 0, "8": 2, "10": int64(55)},
		"2": map[string]any{"1": 2, "3": int64(7_200_000), "6": 4, "7": 0, "8": 0},
	}}})
	s.ApplyVMap(map[string]any{"115": map[string]any{"0": map[string]any{
		"1": map[string]any{"8": 0},
	}}})
	place := s.PearlPlaces()[1]
	if place.LaborEndTime != 7_200_000 || place.EveryMakeNum != 5 || place.CTimeMs != 55 {
		t.Fatalf("sparse update lost fields: %+v", place)
	}
	if got := s.ReadyPearlPlaceIDsAt(now); !reflect.DeepEqual(got, []int32{1, 2}) {
		t.Fatalf("ready after sparse update=%v, want [1 2]", got)
	}

	s.ApplyVMap(map[string]any{"115": map[string]any{"0": map[string]any{
		"1": map[string]any{"3": nil, "6": 0, "7": 0, "8": 0},
		"2": nil,
	}}})
	places := s.PearlPlaces()
	if _, exists := places[2]; exists {
		t.Fatal("explicit null pearl place entry was not deleted")
	}
	place = places[1]
	if place.LaborEndTime != 0 || !place.LaborEndTimeObserved {
		t.Fatalf("explicit null laborEndTime was not cleared/observed: %+v", place)
	}
	if got := s.ReadyPearlPlaceIDsAt(now); len(got) != 0 {
		t.Fatalf("ready after clear=%v, want none", got)
	}

	s.ApplyVMap(map[string]any{"115": map[string]any{"0": nil}})
	if got := s.PearlPlaces(); len(got) != 1 || got[1].PlaceID != 1 {
		t.Fatalf("null place map should be a sparse no-op: %+v", got)
	}
}

func TestPearlClaimAppliedRejectsPartialOrUnknownResponse(t *testing.T) {
	now := time.UnixMilli(8_000_000)
	s := New()
	s.ApplyVMap(map[string]any{"115": map[string]any{"0": map[string]any{
		"1": map[string]any{"1": 1, "3": int64(7_200_000), "6": 5, "7": 0, "8": 0},
		"2": map[string]any{"1": 2, "3": int64(7_200_000), "6": 4, "7": 0, "8": 0},
	}}})
	snapshot, ok := s.PearlClaimSnapshot(now)
	if !ok {
		t.Fatal("claim snapshot unavailable")
	}
	s.ApplyVMap(map[string]any{"115": map[string]any{"0": map[string]any{
		"1": map[string]any{"3": nil, "6": 0, "7": 0, "8": 0},
	}}})
	if s.PearlClaimApplied(snapshot) {
		t.Fatal("partial recvOneKey clear unexpectedly satisfied postcondition")
	}
	s.ApplyVMap(map[string]any{"115": map[string]any{"0": map[string]any{
		"2": map[string]any{"7": 0.5},
	}}})
	if s.PearlClaimApplied(snapshot) {
		t.Fatal("malformed critical field unexpectedly satisfied postcondition")
	}
}

func TestApplyPearlPlaceRejectsInvalidMapKeys(t *testing.T) {
	s := New()
	invalidPlace := map[string]any{"1": 1, "3": int64(7_200_000), "6": 5, "7": 0, "8": 0}
	s.ApplyVMap(map[string]any{"115": map[string]any{"0": map[string]any{
		"-1":         invalidPlace,
		"1.5":        invalidPlace,
		"2147483648": invalidPlace,
	}}})
	if got := s.PearlPlaces(); len(got) != 0 {
		t.Fatalf("invalid place map keys created slots: %+v", got)
	}
	if got := s.ReadyPearlPlaceIDsAt(time.UnixMilli(8_000_000)); len(got) != 0 {
		t.Fatalf("invalid place map keys became ready: %v", got)
	}
}
