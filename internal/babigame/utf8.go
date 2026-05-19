package babigame

import (
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// SafeUTF8 ensures s is a valid UTF-8 string. Bytes that aren't valid get
// replaced with `�` so the result is always safe to drop into a proto
// string or anywhere else that requires UTF-8.
//
// Use at trust boundaries - anything coming from the babigame backend that
// might be GBK-encoded (server names, error messages, role names) should be
// sanitized before being stored or surfaced through the gRPC API.
func SafeUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	// Try GBK first - the babigame server returns simplified Chinese in
	// GBK for fields that didn't go through their JSON serializer (server
	// list names, certain error messages). If that decoding produces a
	// valid UTF-8 string, keep it; otherwise drop to replacement chars.
	if decoded, err := simplifiedchinese.GBK.NewDecoder().String(s); err == nil && utf8.ValidString(decoded) {
		return decoded
	}
	if decoded, err := simplifiedchinese.GB18030.NewDecoder().String(s); err == nil && utf8.ValidString(decoded) {
		return decoded
	}
	return replaceInvalidBytes(s)
}

// replaceInvalidBytes is the worst-case fallback: walk the string byte by
// byte, replacing any sequence that isn't a valid UTF-8 rune with U+FFFD.
func replaceInvalidBytes(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			out = append(out, '�')
			i++
			continue
		}
		out = append(out, r)
		i += size
	}
	return string(out)
}

// SanitizeMap walks v and applies SafeUTF8 to every string value found
// (including inside nested maps and slices). Returns a new map; v is not
// mutated. Used to scrub a parsed-JSON response before it gets %v-formatted
// into an error or stored into sqlite.
func SanitizeMap(v map[string]any) map[string]any {
	out := make(map[string]any, len(v))
	for k, val := range v {
		out[SafeUTF8(k)] = sanitizeAny(val)
	}
	return out
}

func sanitizeAny(v any) any {
	switch x := v.(type) {
	case string:
		return SafeUTF8(x)
	case map[string]any:
		return SanitizeMap(x)
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = sanitizeAny(e)
		}
		return out
	default:
		return v
	}
}
