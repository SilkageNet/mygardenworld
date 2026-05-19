package babigame

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// UpstreamError is the structured form of "babigame returned something we
// couldn't make sense of". It distinguishes three failure modes:
//
//   - non-2xx HTTP status: server reachable but rejecting (StatusCode > 0)
//   - JSON unmarshal failure: server returned something that isn't JSON
//     (typical when an upstream proxy intercepts the request, e.g. a Clash
//     route mismatch corrupts the response stream)
//   - empty/missing key: parsed JSON, but the field we needed was absent
//
// We always include a short hex preview of the body so misrouted-traffic
// patterns are visible without dumping arbitrary bytes into log lines or
// proto string fields.
type UpstreamError struct {
	Op          string // "game/login", "/gw index.login", etc.
	Host        string
	Path        string
	StatusCode  int    // 0 if the request never reached the wire / response read failed
	ContentType string // raw Content-Type header
	BodyLen     int
	BodyPreview []byte // first 64 bytes verbatim (may include non-UTF-8)
	Cause       error  // optional underlying error (json/io/http)
	Message     string // brief, human-readable category
}

// Error formats a single line that's safe to log or surface through proto
// strings (the preview is hex-encoded so no GBK / binary leaks).
func (e *UpstreamError) Error() string {
	parts := []string{
		fmt.Sprintf("upstream %s", e.Op),
	}
	if e.Host != "" {
		parts = append(parts, fmt.Sprintf("host=%s%s", e.Host, e.Path))
	}
	if e.StatusCode > 0 {
		parts = append(parts, fmt.Sprintf("status=%d", e.StatusCode))
	}
	if e.ContentType != "" {
		parts = append(parts, fmt.Sprintf("content_type=%q", e.ContentType))
	}
	if e.BodyLen > 0 {
		parts = append(parts, fmt.Sprintf("body_len=%d", e.BodyLen))
	}
	if len(e.BodyPreview) > 0 {
		parts = append(parts, fmt.Sprintf("body_hex=%s", hex.EncodeToString(e.BodyPreview)))
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	if e.Cause != nil {
		parts = append(parts, fmt.Sprintf("cause=%v", e.Cause))
	}
	return strings.Join(parts, " ")
}

// Unwrap exposes the underlying error so errors.Is / errors.As work.
func (e *UpstreamError) Unwrap() error { return e.Cause }

// LooksLikeProxyInterception returns true when the response shape suggests an
// MITM proxy or DNS-fake-IP routing hijack rather than a real server reply.
// The most common signature is a non-JSON body whose first bytes don't match
// any expected wire format. Useful to give the operator a clearer diagnosis
// than "unmarshal failed".
func (e *UpstreamError) LooksLikeProxyInterception() bool {
	if e == nil || len(e.BodyPreview) == 0 {
		return false
	}
	// JSON would start with { or [ (after optional whitespace).
	// HTML (e.g. a 502 page) would start with < or BOM.
	first := e.BodyPreview[0]
	if first == '{' || first == '[' || first == '<' {
		return false
	}
	// High-bit-set first byte without a valid UTF-8 prefix is suspicious.
	if first >= 0x80 {
		return true
	}
	return false
}

// previewBytes returns up to 64 bytes for diagnostic purposes.
func previewBytes(body []byte) []byte {
	const max = 64
	if len(body) <= max {
		out := make([]byte, len(body))
		copy(out, body)
		return out
	}
	out := make([]byte, max)
	copy(out, body[:max])
	return out
}

// AsUpstreamError unwraps an UpstreamError from any error chain. Returns
// nil if none is present.
func AsUpstreamError(err error) *UpstreamError {
	var ue *UpstreamError
	if errors.As(err, &ue) {
		return ue
	}
	return nil
}
