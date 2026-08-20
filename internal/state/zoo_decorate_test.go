package state

import "testing"

func TestZooDecorateNamespacesTrackSparseStateAndObservation(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{"33": map[string]any{
		"5": map[string]any{
			"101": map[string]any{"0": 9001, "1": 101, "2": 1, "3": 15, "4": 200, "5": 100},
		},
		"6": map[string]any{
			"201": map[string]any{"0": 9001, "1": 201, "2": 2, "3": 200, "4": 100},
		},
	}})

	if !s.ZooDecoratesObserved() || !s.ZooDecorateSuitsObserved() {
		t.Fatalf("decorate namespaces not observed: decorate=%t suits=%t", s.ZooDecoratesObserved(), s.ZooDecorateSuitsObserved())
	}
	decorate := s.ZooDecorates()[101]
	if decorate.MapTempID != 101 || decorate.TempID != 101 || decorate.UID != 9001 || decorate.IsRead != 1 || decorate.Comfort != 15 || decorate.UpdatedAtMs != 200 || decorate.CreatedAtMs != 100 ||
		!decorate.TempIDObserved || !decorate.UIDObserved || !decorate.IsReadObserved || !decorate.ComfortObserved || !decorate.UpdatedAtObserved || !decorate.CreatedAtObserved {
		t.Fatalf("decorate=%+v", decorate)
	}
	suit := s.ZooDecorateSuits()[201]
	if suit.MapTempID != 201 || suit.TempID != 201 || suit.UID != 9001 || suit.ActCount != 2 || suit.UpdatedAtMs != 200 || suit.CreatedAtMs != 100 ||
		!suit.TempIDObserved || !suit.UIDObserved || !suit.ActCountObserved || !suit.UpdatedAtObserved || !suit.CreatedAtObserved {
		t.Fatalf("decorate suit=%+v", suit)
	}

	applyMap(t, s, map[string]any{"33": map[string]any{
		"5": map[string]any{"101": map[string]any{"0": nil, "3": 17}},
		"6": map[string]any{"201": map[string]any{"0": nil, "2": 3}},
	}})
	decorate = s.ZooDecorates()[101]
	if decorate.UIDObserved || decorate.UID != 0 || decorate.Comfort != 17 || !decorate.ComfortObserved || decorate.CreatedAtMs != 100 {
		t.Fatalf("sparse decorate merge=%+v", decorate)
	}
	suit = s.ZooDecorateSuits()[201]
	if suit.UIDObserved || suit.UID != 0 || suit.ActCount != 3 || !suit.ActCountObserved || suit.CreatedAtMs != 100 {
		t.Fatalf("sparse decorate suit merge=%+v", suit)
	}

	applyMap(t, s, map[string]any{"33": map[string]any{
		"5": map[string]any{"101": nil},
		"6": map[string]any{"201": nil},
	}})
	if len(s.ZooDecorates()) != 0 || len(s.ZooDecorateSuits()) != 0 {
		t.Fatalf("null entries did not delete: decorates=%+v suits=%+v", s.ZooDecorates(), s.ZooDecorateSuits())
	}

	applyMap(t, s, map[string]any{"33": map[string]any{"5": nil, "6": nil}})
	if s.ZooDecoratesObserved() || s.ZooDecorateSuitsObserved() {
		t.Fatalf("null maps remained observed: decorate=%t suits=%t", s.ZooDecoratesObserved(), s.ZooDecorateSuitsObserved())
	}
}
