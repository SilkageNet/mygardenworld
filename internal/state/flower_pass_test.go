package state

import (
	"testing"
)

func TestFlowerPassApplyBuildsReadyTaskAndFreeLevel(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"131": map[string]any{
			"0": map[string]any{
				"15": map[string]any{"1": 15, "2": 3, "3": 10, "4": 0, "5": 0, "6": map[string]any{"1": []any{1}}},
			},
			"1": map[string]any{
				"15": map[string]any{
					"1": 15,
					"2": map[string]any{"3044_0": 12},
					"4": map[string]any{},
					"5": []any{1001, 1003},
					"6": map[string]any{"2010_0": 3, "3006_0": 10},
					"8": map[string]any{},
				},
			},
		},
	})

	if !s.FlowerPassObserved() {
		t.Fatal("expected flower pass observed")
	}
	view := s.FlowerPassView()
	if !view.Found || view.Bid != 15 || view.Lvl != 3 {
		t.Fatalf("view=%+v", view)
	}
	if view.ReadyFreeLevelCount < 1 {
		t.Fatalf("expected free levels ready, got %+v", view.ReadyFreeLevels)
	}
	for _, lvl := range view.ReadyFreeLevels {
		if lvl == 1 {
			t.Fatalf("level 1 already claimed, got %+v", view.ReadyFreeLevels)
		}
	}

	bid, taskIDs := s.ReadyFlowerPassTaskIDs()
	if bid != 15 || len(taskIDs) == 0 {
		t.Fatalf("ready tasks bid=%d ids=%v", bid, taskIDs)
	}
	foundDaily := false
	for _, task := range view.Tasks {
		if task.TaskID == 1001 && task.Ready && task.Progress == 3 {
			foundDaily = true
		}
		if task.TaskID == 1002 {
			t.Fatalf("daily task 1002 should be filtered by dailyTaskIds")
		}
	}
	if !foundDaily {
		t.Fatalf("expected ready daily task 1001 in %+v", view.Tasks)
	}
}

func TestFlowerElvesPassApplyFromNS132(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"132": map[string]any{
			"2": map[string]any{},
			"3": map[string]any{"14": map[string]any{"1": 14, "2": 2, "3": 0, "6": map[string]any{"1": []any{}}}},
			"4": map[string]any{"14": map[string]any{"1": 14, "5": []any{1001}, "6": map[string]any{"3006_0": 50}, "8": map[string]any{}}},
		},
	})
	if !s.FlowerElvesPassObserved() {
		t.Fatal("expected elves pass observed")
	}
	view := s.FlowerElvesPassView()
	if !view.Found || view.Bid != 14 {
		t.Fatalf("view=%+v", view)
	}
	bid, ids := s.ReadyFlowerElvesPassTaskIDs()
	if bid != 14 || len(ids) == 0 {
		t.Fatalf("ready elves pass tasks bid=%d ids=%v view=%+v", bid, ids, view)
	}
}

func TestPassProgressLookupBareType(t *testing.T) {
	m := map[string]int32{"2010": 5}
	if got := passProgressLookup(m, "2010_0", 2010); got != 5 {
		t.Fatalf("got %d", got)
	}
}

func TestFlowerPassPartialDeltaDoesNotInventFreeClaims(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"131": map[string]any{
			"0": map[string]any{"15": map[string]any{"1": 15, "2": 3, "3": 0}},
		},
	})
	if s.FlowerPassRwdMapObserved() {
		t.Fatal("rwdMap should be unobserved without field 6")
	}
	if bid, levels := s.ReadyFlowerPassFreeLevels(); len(levels) != 0 {
		t.Fatalf("want no free levels without rwdMap, bid=%d levels=%v", bid, levels)
	}

	applyMap(t, s, map[string]any{
		"131": map[string]any{
			"0": map[string]any{"15": map[string]any{"2": 4, "6": map[string]any{"free": []any{1, 2}}}},
		},
	})
	if !s.FlowerPassRwdMapObserved() {
		t.Fatal("expected rwdMap observed after field 6")
	}
	bid, levels := s.ReadyFlowerPassFreeLevels()
	if bid != 15 || len(levels) == 0 {
		t.Fatalf("want remaining free levels, bid=%d levels=%v", bid, levels)
	}
	for _, lvl := range levels {
		if lvl == 1 || lvl == 2 {
			t.Fatalf("claimed levels still ready: %v", levels)
		}
	}
}

func TestDecodePassRwdMapNamedKeys(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"131": map[string]any{
			"0": map[string]any{
				"15": map[string]any{
					"1": 15, "2": 37, "3": 40, "4": 1, "5": 0,
					"6": map[string]any{
						"free":   []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36},
						"pay":    []any{1, 2, 3},
						"addPay": []any{},
					},
				},
			},
		},
	})
	bid, levels := s.ReadyFlowerPassFreeLevels()
	if bid != 15 {
		t.Fatalf("bid=%d", bid)
	}
	if len(levels) != 1 || levels[0] != 37 {
		t.Fatalf("want only lvl 37 ready, got %v", levels)
	}
}

func TestMarkFlowerPassFreeRecvRejected(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"131": map[string]any{
			"0": map[string]any{"15": map[string]any{"1": 15, "2": 3, "6": map[string]any{"1": []any{}}}},
		},
	})
	if bid, levels := s.ReadyFlowerPassFreeLevels(); bid != 15 || len(levels) == 0 {
		t.Fatalf("setup ready levels bid=%d levels=%v", bid, levels)
	}
	s.MarkFlowerPassFreeRecvRejected(15)
	if bid, levels := s.ReadyFlowerPassFreeLevels(); len(levels) != 0 {
		t.Fatalf("after reject still ready bid=%d levels=%v", bid, levels)
	}
}
