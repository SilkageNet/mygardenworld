package state

import (
	"encoding/json"
	"testing"
)

func TestPlayerDisplayName(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"7":{"0":{"0":1001}},"28":{"5":[{"0":2001,"1":"阿花","4":20}]}}`))

	if got := s.PlayerDisplayName(1001, "自己"); got != "自己" {
		t.Fatalf("self name=%q, want 自己", got)
	}
	if got := s.PlayerDisplayName(2001, "自己"); got != "阿花" {
		t.Fatalf("profile name=%q, want 阿花", got)
	}
	if got := s.PlayerDisplayName(3001, "自己"); got != "" {
		t.Fatalf("unknown uid=%q, want empty", got)
	}
	if got := s.PlayerDisplayName(0, "自己"); got != "" {
		t.Fatalf("zero uid=%q, want empty", got)
	}
}
