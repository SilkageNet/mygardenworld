package babigame

import (
	"encoding/json"
	"testing"
)

func TestDisplayNameFromState(t *testing.T) {
	raw := json.RawMessage(`{"7":{"0":{"5":" 茉莉  花 "}}}`)
	got := DisplayNameFromState(raw, 12, "fallback")
	if got != "茉莉 花 · 第12区" {
		t.Fatalf("display name=%q, want 茉莉 花 · 第12区", got)
	}
}

func TestDisplayNameFromSessionFallsBackToUsername(t *testing.T) {
	session := &Session{GsIdx: 8, AccountRaw: map[string]any{"name": "openid"}}
	got := DisplayNameFromSession(session, "user@example.test")
	if got != "user@example.test · 第8区" {
		t.Fatalf("display name=%q, want username with area", got)
	}
}

func TestDisplayNameFromSessionUsesNicknameHint(t *testing.T) {
	session := &Session{GsIdx: 3, AccountRaw: map[string]any{"nick": "海棠"}}
	got := DisplayNameFromSession(session, "user@example.test")
	if got != "海棠 · 第3区" {
		t.Fatalf("display name=%q, want nickname with area", got)
	}
}
