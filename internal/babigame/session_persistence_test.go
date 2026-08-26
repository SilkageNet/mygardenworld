package babigame

import (
	"strings"
	"testing"
)

func TestSessionJSONRoundTrip(t *testing.T) {
	cfg := testConfig(t)
	want := &Session{
		Cfg:        cfg,
		DeviceID:   "device",
		UUID:       "uuid",
		Session0:   "session0",
		Native:     NativeLogin{Content: "native-secret", Session1: "session1"},
		GameLogin:  GameLoginResult{Token: "game-secret", OpenID: "openid", Content: "content"},
		AID:        42,
		GsIdx:      3,
		RouteToken: "route-secret",
		AccountRaw: map[string]any{"nick": "海棠"},
		GsHost:     "game.example.test",
		GsPortSSL:  443,
	}
	blob, err := MarshalSessionJSON(want)
	if err != nil {
		t.Fatal(err)
	}
	got, err := UnmarshalSessionJSON(blob, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.DeviceID != want.DeviceID || got.UUID != want.UUID || got.Session0 != want.Session0 ||
		got.Token() != want.Token() || got.AID != want.AID || got.GsIdx != want.GsIdx || got.RouteToken != want.RouteToken {
		t.Fatalf("round trip mismatch: got=%+v want=%+v", got, want)
	}
}

func TestSessionJSONRejectsUnversionedAndIncompletePayloads(t *testing.T) {
	cfg := testConfig(t)
	for _, payload := range []string{
		`{"device_id":"legacy"}`,
		`{"version":1,"device_id":"device"}`,
	} {
		if _, err := UnmarshalSessionJSON([]byte(payload), cfg); err == nil {
			t.Fatalf("UnmarshalSessionJSON(%s) succeeded, want error", payload)
		}
	}
	if _, err := MarshalSessionJSON(&Session{}); err == nil || !strings.Contains(err.Error(), "device id") {
		t.Fatalf("MarshalSessionJSON(empty) error=%v", err)
	}
}
