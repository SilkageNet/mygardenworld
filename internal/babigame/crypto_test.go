package babigame

import (
	"encoding/json"
	"strings"
	"testing"
)

// Captured `index.login` /gw frame. capturedSign and capturedA live in
// testdata_captured.go so we can keep this test file readable.

func TestGWVerifyCapturedFrame(t *testing.T) {
	cfg := DefaultConfig()
	body := GWBody{Sign: capturedSign, A: capturedA, L: "zh"}
	if !VerifyGWBody(body, cfg) {
		t.Fatalf("captured /gw frame fails MD5 sign verification")
	}
}

func TestGWDecodeCapturedFrame(t *testing.T) {
	cfg := DefaultConfig()
	clear, err := GWDecode(capturedA, cfg.GWXorMask)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// First element of the decoded array should be the rpc name "index.login".
	var tuple []json.RawMessage
	if err := json.Unmarshal(clear, &tuple); err != nil {
		t.Fatalf("decoded payload not a JSON array: %v\nplaintext: %s", err, clear)
	}
	if len(tuple) < 1 {
		t.Fatalf("decoded array empty")
	}
	var name string
	if err := json.Unmarshal(tuple[0], &name); err != nil {
		t.Fatalf("first elem not a string: %v", err)
	}
	if name != "index.login" {
		t.Fatalf("expected rpc=index.login, got %q", name)
	}
}

func TestGWEncodeRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	src := []any{
		"usrLand.harvest",
		map[string]any{"landId": 1017},
		"tok|2482",
	}
	body, err := BuildGWBody(src, cfg, "zh")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !VerifyGWBody(body, cfg) {
		t.Fatalf("self-built body fails its own sign verification")
	}
	clear, err := GWDecode(body.A, cfg.GWXorMask)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Must be a JSON array with the same first/second elements.
	var tuple []json.RawMessage
	if err := json.Unmarshal(clear, &tuple); err != nil {
		t.Fatalf("not array: %v\nplaintext: %s", err, clear)
	}
	if len(tuple) != 3 {
		t.Fatalf("expected 3-tuple, got %d", len(tuple))
	}
	var name string
	_ = json.Unmarshal(tuple[0], &name)
	if name != "usrLand.harvest" {
		t.Fatalf("name mismatch: %q", name)
	}
}

func TestBuildRequestEnvelope(t *testing.T) {
	cfg := DefaultConfig()
	frame, k, err := BuildRequest("usrLand.harvest", map[string]any{"landId": 1017}, "tok|2482", 12, cfg)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.HasPrefix(frame, "$#|#$") {
		t.Fatalf("missing sentinel prefix")
	}
	env, err := ParseTextFrame(frame)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if env.E != "request" {
		t.Fatalf("envelope.e=%q", env.E)
	}
	var d WSEnvelopeOutD
	if err := json.Unmarshal(env.D, &d); err != nil {
		t.Fatalf("decode d: %v", err)
	}
	if d.K != k {
		t.Fatalf("k roundtrip mismatch: env=%q want=%q", d.K, k)
	}
	if d.R != "gs" {
		t.Fatalf("r=%q", d.R)
	}
	if !VerifyGWBody(d.P, cfg) {
		t.Fatalf("inner gw body sign fails")
	}
}
