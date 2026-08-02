package state

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStoryMainCatalogIsExactThroughTerminal(t *testing.T) {
	rawSentinel, exists := StaticRow("c_storyMainChapter", -1)
	if !exists {
		t.Fatal("missing c_storyMainChapter/-1")
	}
	var sentinel struct {
		Max int32 `json:"$max"`
	}
	if json.Unmarshal(rawSentinel, &sentinel) != nil || sentinel.Max != 164 {
		t.Fatalf("story sentinel=%+v", sentinel)
	}
	terminalChapter, terminalSection, ok := StoryMainTerminal()
	if !ok || terminalChapter != 165 || terminalSection != 0 {
		t.Fatalf("StoryMainTerminal=%d:%d,%t, want 165:0", terminalChapter, terminalSection, ok)
	}
	first32, ok := StoryMainSection(32, 0)
	if !ok || first32.SectionID != 4101 || !reflect.DeepEqual(first32.Cost, []ItemCount{{ItemID: 56, Count: 85}}) {
		t.Fatalf("chapter 32 first section=%+v,%t", first32, ok)
	}
	last, ok := StoryMainSection(164, 5)
	if !ok || last.SectionID != 17306 || !reflect.DeepEqual(last.Cost, []ItemCount{{ItemID: 56, Count: 195}}) {
		t.Fatalf("last story section=%+v,%t", last, ok)
	}
	for chapter := int32(1); chapter < terminalChapter; chapter++ {
		definition, valid := storyMainChapter(chapter)
		if !valid {
			t.Fatalf("chapter %d invalid", chapter)
		}
		for idx := range definition.SectionIDs {
			section, valid := StoryMainSection(chapter, int32(idx))
			if !valid || section.SectionID != definition.SectionIDs[idx] || len(section.Cost) == 0 {
				t.Fatalf("chapter %d section %d=%+v,%t", chapter, idx, section, valid)
			}
		}
	}
}

func TestParseStoryMainCostAggregatesAndRejectsMalformed(t *testing.T) {
	got, ok := parseStoryMainCost(json.RawMessage(`[[56,10],[7,1],[56,20]]`))
	want := []ItemCount{{ItemID: 7, Count: 1}, {ItemID: 56, Count: 30}}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("parseStoryMainCost=%v,%t want %v", got, ok, want)
	}
	for _, raw := range []string{
		`null`, `[]`, `[[56]]`, `[["56",10]]`, `[[56,1.5]]`, `[[56,-1]]`, `[[56,2147483647],[56,1]]`,
	} {
		if got, ok := parseStoryMainCost(json.RawMessage(raw)); ok || got != nil {
			t.Fatalf("malformed cost %s accepted: %v,%t", raw, got, ok)
		}
	}
}

func TestApplyStoryMainStrictSparseAndComplete(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{"7": map[string]any{"101": map[string]any{"0": int64(9001), "1": 32, "2": 0}}})
	story, ok := s.StoryMain()
	if !ok || !story.Valid || story.Complete || !story.ChapterObserved || !story.SectionObserved || story.SectionID != 4101 {
		t.Fatalf("initial story=%+v,%t", story, ok)
	}

	applyMap(t, s, map[string]any{"7": map[string]any{"101": map[string]any{"2": 1}}})
	story, _ = s.StoryMain()
	if !story.Valid || story.Chapter != 32 || story.SectionIdx != 1 || story.SectionID != 4102 {
		t.Fatalf("section-only delta=%+v", story)
	}

	applyMap(t, s, map[string]any{"7": map[string]any{"101": map[string]any{"1": 33}}})
	story, _ = s.StoryMain()
	if story.Valid || story.SectionObserved {
		t.Fatalf("chapter-only transition reused stale section: %+v", story)
	}
	applyMap(t, s, map[string]any{"7": map[string]any{"101": map[string]any{"2": 0}}})
	story, _ = s.StoryMain()
	if !story.Valid || story.Chapter != 33 || story.SectionIdx != 0 || story.SectionID != 4201 {
		t.Fatalf("section completed sparse transition=%+v", story)
	}

	for _, malformed := range []any{"33", 33.5, int64(1) << 40, -1} {
		applyMap(t, s, map[string]any{"7": map[string]any{"101": map[string]any{"1": malformed}}})
		story, _ = s.StoryMain()
		if story.Valid || story.ChapterObserved {
			t.Fatalf("malformed chapter %v accepted: %+v", malformed, story)
		}
		applyMap(t, s, map[string]any{"7": map[string]any{"101": map[string]any{"1": 33, "2": 0}}})
	}

	applyMap(t, s, map[string]any{"7": map[string]any{"101": map[string]any{"1": 165, "2": 0}}})
	story, _ = s.StoryMain()
	if !story.Valid || !story.Complete || story.SectionID != 0 || len(story.Cost) != 0 || !s.StoryMainReady() {
		t.Fatalf("terminal story=%+v", story)
	}
	applyMap(t, s, map[string]any{"7": map[string]any{"101": map[string]any{"2": 1}}})
	story, _ = s.StoryMain()
	if story.Valid || story.Complete {
		t.Fatalf("invalid terminal index accepted: %+v", story)
	}

	applyMap(t, s, map[string]any{"7": map[string]any{"101": nil}})
	story, _ = s.StoryMain()
	if story.Valid || story.ChapterObserved || story.SectionObserved {
		t.Fatalf("null story preserved stale target: %+v", story)
	}

	empty := New()
	applyMap(t, empty, map[string]any{"7": map[string]any{"101": map[string]any{}}})
	emptyStory, observed := empty.StoryMain()
	if !observed || emptyStory.Valid || emptyStory.Complete {
		t.Fatalf("empty story=%+v,%t, want observed-invalid", emptyStory, observed)
	}
}

func TestStoryUnlockSnapshotRequiresExactProgressAndCost(t *testing.T) {
	insufficient := New()
	applyMap(t, insufficient, map[string]any{"7": map[string]any{
		"0":   map[string]any{"32": map[string]any{"56": 84}},
		"101": map[string]any{"1": 32, "2": 0},
	}})
	if snapshot, ok := insufficient.StoryUnlockSnapshot(); ok {
		t.Fatalf("insufficient inventory produced snapshot: %+v", snapshot)
	}

	tests := []struct {
		name        string
		chapter     int32
		section     int32
		before      int32
		nextChapter int32
		nextSection int32
		after       int32
		want        bool
	}{
		{name: "within chapter", chapter: 32, section: 0, before: 200, nextChapter: 32, nextSection: 1, after: 115, want: true},
		{name: "next chapter", chapter: 32, section: 5, before: 200, nextChapter: 33, nextSection: 0, after: 115, want: true},
		{name: "terminal", chapter: 164, section: 5, before: 200, nextChapter: 165, nextSection: 0, after: 5, want: true},
		{name: "wrong decrement", chapter: 32, section: 0, before: 200, nextChapter: 32, nextSection: 1, after: 116},
		{name: "jumped progress", chapter: 32, section: 0, before: 200, nextChapter: 32, nextSection: 2, after: 115},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := New()
			applyMap(t, s, map[string]any{"7": map[string]any{
				"0":   map[string]any{"32": map[string]any{"56": tc.before}},
				"101": map[string]any{"1": tc.chapter, "2": tc.section},
			}})
			snapshot, ok := s.StoryUnlockSnapshot()
			if !ok {
				t.Fatal("preflight snapshot unavailable")
			}
			applyMap(t, s, map[string]any{"7": map[string]any{
				"2":   map[string]any{"2": map[string]any{"56": tc.after}},
				"101": map[string]any{"1": tc.nextChapter, "2": tc.nextSection},
			}})
			if got := s.StoryUnlockApplied(snapshot); got != tc.want {
				t.Fatalf("StoryUnlockApplied=%t want %t snapshot=%+v story=%+v", got, tc.want, snapshot, mustStoryMain(t, s))
			}
		})
	}
}

func mustStoryMain(t *testing.T, s *State) StoryMainView {
	t.Helper()
	view, ok := s.StoryMain()
	if !ok {
		t.Fatal("story not observed")
	}
	return view
}
