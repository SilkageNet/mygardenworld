package state

import (
	"testing"
	"time"
)

func TestSpeedUpTicketsUsedTodayResetsAtMidnight(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	s := New()
	day := time.Date(2026, 8, 31, 12, 0, 0, 0, loc)
	s.NoteSpeedUpTicketsUsed(day, 64)
	s.NoteSpeedUpTicketsUsed(day, 5)
	if got := s.SpeedUpTicketsUsedToday(day); got != 69 {
		t.Fatalf("used=%d, want 69", got)
	}
	next := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	if got := s.SpeedUpTicketsUsedToday(next); got != 0 {
		t.Fatalf("after midnight used=%d, want 0", got)
	}
	s.NoteSpeedUpTicketsUsed(next, 3)
	if got := s.SpeedUpTicketsUsedToday(next); got != 3 {
		t.Fatalf("new day used=%d, want 3", got)
	}
}

func TestSetSpeedUpTicketsUsedHydrate(t *testing.T) {
	s := New()
	s.SetSpeedUpTicketsUsed(20260831, 1169)
	now := time.Date(2026, 8, 31, 17, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	if got := s.SpeedUpTicketsUsedToday(now); got != 1169 {
		t.Fatalf("hydrated used=%d, want 1169", got)
	}
}
