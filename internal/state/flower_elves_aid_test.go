package state

import (
	"testing"
	"time"
)

func TestFlowerElvesAidParseAndGates(t *testing.T) {
	s := New()
	now := time.UnixMilli(1_789_178_000_000)

	if s.FlowerElvesAidObserved() {
		t.Fatal("expected unobserved")
	}
	if s.CanRecvFlowerElvesAid(now) || s.CanReqFlowerElvesAid(now) {
		t.Fatal("unobserved must not allow req/recv")
	}

	s.ApplyVMap(map[string]any{
		"132": map[string]any{"5": map[string]any{
			"1": map[string]any{"1001": float64(now.UnixMilli())},
			"4": 1,
		}},
	})
	if !s.FlowerElvesAidObserved() {
		t.Fatal("expected observed")
	}
	if !s.CanRecvFlowerElvesAid(now) {
		t.Fatal("reqAid=1 with helpers should allow recv")
	}
	if s.CanReqFlowerElvesAid(now) {
		t.Fatal("open request should block req")
	}

	// Enough helpers with reqAid cleared is still claimable (observed live).
	s.ApplyVMap(map[string]any{
		"132": map[string]any{"5": map[string]any{
			"2": float64(now.Add(-time.Hour).UnixMilli()),
			"3": nil,
			"4": 0,
		}},
	})
	if !s.CanRecvFlowerElvesAid(now) {
		t.Fatal("helpers with closed reqAid should still allow recv")
	}
	if s.CanReqFlowerElvesAid(now) {
		t.Fatal("pending claim should block new req")
	}

	s.ApplyVMap(map[string]any{
		"132": map[string]any{"5": map[string]any{
			"1": map[string]any{},
			"2": float64(now.Add(-3 * time.Hour).UnixMilli()),
			"3": float64(0),
			"4": 0,
		}},
	})
	if s.CanRecvFlowerElvesAid(now) {
		t.Fatal("empty aidMap should not recv")
	}
	if !s.CanReqFlowerElvesAid(now) {
		t.Fatal("cooldown elapsed should allow req")
	}

	s.ApplyVMap(map[string]any{
		"132": map[string]any{"5": map[string]any{
			"1": map[string]any{"1001": float64(now.UnixMilli())},
			"2": float64(now.Add(-time.Hour).UnixMilli()),
			"3": float64(now.Add(time.Hour).UnixMilli()),
			"4": 0,
		}},
	})
	if s.CanRecvFlowerElvesAid(now) {
		t.Fatal("active effect should block recv")
	}
	if s.CanReqFlowerElvesAid(now) {
		t.Fatal("active effect should block req")
	}

	s.ApplyVMap(map[string]any{
		"132": map[string]any{"5": map[string]any{
			"3": nil,
			"4": 0,
		}},
	})
	if s.FlowerElvesAid().EffEndTime != 0 {
		t.Fatalf("null effEndTime should clear, got %d", s.FlowerElvesAid().EffEndTime)
	}
	// Same preReq already claimed via the active-buff observation above.
	if s.CanRecvFlowerElvesAid(now) {
		t.Fatal("already-claimed preReq should not recv again after buff clears")
	}
}

func TestFlowerElvesAidHelpDaily(t *testing.T) {
	s := New()
	now := time.UnixMilli(1_789_178_000_000)
	s.NoteFlowerElvesAidHelped(42, now)
	if !s.FlowerElvesAidHelpedToday(42, now) {
		t.Fatal("expected helped")
	}
	if s.FlowerElvesAidHelpCountToday(now) != 1 {
		t.Fatalf("count=%d", s.FlowerElvesAidHelpCountToday(now))
	}
	nextDay := now.Add(25 * time.Hour)
	if s.FlowerElvesAidHelpedToday(42, nextDay) || s.FlowerElvesAidHelpCountToday(nextDay) != 0 {
		t.Fatal("expected day rollover")
	}

	s.SetFlowerElvesAidHelped(20260912, []int64{1, 2, 3, 4, 5})
	setAt := time.Date(2026, 9, 12, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	if s.FlowerElvesAidHelpCountToday(setAt) != 5 {
		t.Fatalf("set count=%d", s.FlowerElvesAidHelpCountToday(setAt))
	}
	if !s.FlowerElvesAidHelpedToday(3, setAt) || s.FlowerElvesAidHelpedToday(9, setAt) {
		t.Fatal("set helped membership mismatch")
	}
}

func TestFriendOtherInfoParsesIsAid(t *testing.T) {
	s := New()
	s.ApplyVMap(map[string]any{
		"110": map[string]any{"1": map[string]any{
			"99": map[string]any{"0": 1, "1": 1},
		}},
	})
	view := s.FriendTouch(time.UnixMilli(1_700_000_000_000))
	info := view.OtherInfo[99]
	if !info.IsSteal || !info.IsAid {
		t.Fatalf("info=%+v", info)
	}
}
