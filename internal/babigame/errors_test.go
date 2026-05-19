package babigame

import (
	"errors"
	"strings"
	"testing"
)

func TestUpstreamError_ErrorFormat(t *testing.T) {
	cause := errors.New("invalid character")
	e := &UpstreamError{
		Op:          "POST /game/login",
		Host:        "api.babigame.cn",
		Path:        "/game/login",
		StatusCode:  200,
		ContentType: "application/json",
		BodyLen:     1024,
		BodyPreview: []byte{0xff, 0x80, 'A', 'B'},
		Cause:       cause,
		Message:     "json unmarshal failed",
	}
	got := e.Error()
	for _, want := range []string{
		"upstream POST /game/login",
		"host=api.babigame.cn/game/login",
		"status=200",
		"body_len=1024",
		"body_hex=ff804142",
		"json unmarshal failed",
		"invalid character",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("Error() missing %q in %q", want, got)
		}
	}
	// Critical: the formatted output must be valid UTF-8 (we're going to
	// shove it into a proto string field).
	if !strings.ContainsAny(got, "\xff\x80") {
		// Good - the raw invalid bytes were hex-encoded, not pasted in.
	} else {
		t.Errorf("formatted error contains raw invalid bytes: %q", got)
	}
}

func TestUpstreamError_Unwrap(t *testing.T) {
	cause := errors.New("eof")
	e := &UpstreamError{Cause: cause}
	if !errors.Is(e, cause) {
		t.Fatal("errors.Is should find Cause via Unwrap")
	}
}

func TestUpstreamError_AsUnwrapsThroughChain(t *testing.T) {
	inner := &UpstreamError{Op: "test", Message: "boom"}
	wrapped := errors.New("outer wrapper") // simulates fmt.Errorf("...: %w", inner) etc
	_ = wrapped
	chained := errFmt(inner)
	got := AsUpstreamError(chained)
	if got == nil {
		t.Fatalf("AsUpstreamError returned nil for wrapped UpstreamError")
	}
	if got.Op != "test" {
		t.Errorf("recovered op mismatch: %q", got.Op)
	}
}

func TestUpstreamError_LooksLikeProxyInterception(t *testing.T) {
	cases := []struct {
		name    string
		preview []byte
		want    bool
	}{
		{"json", []byte(`{"x":1}`), false},
		{"html", []byte(`<html><body>...`), false},
		{"binary garbage", []byte{0x99, 0xab, 0xcd}, true},
		{"empty", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &UpstreamError{BodyPreview: tc.preview}
			if got := e.LooksLikeProxyInterception(); got != tc.want {
				t.Errorf("got %v want %v for %v", got, tc.want, tc.preview)
			}
		})
	}
}

// errFmt is a tiny helper that lets us produce an error that wraps `e`
// without pulling fmt into the test imports.
func errFmt(e error) error {
	return &wrapErr{Cause: e}
}

type wrapErr struct{ Cause error }

func (w *wrapErr) Error() string { return "wrapped: " + w.Cause.Error() }
func (w *wrapErr) Unwrap() error { return w.Cause }
