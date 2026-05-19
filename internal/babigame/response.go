package babigame

import (
	"bytes"
	"encoding/json"
)

// HasPayload reports whether an RPC response contains a non-empty JSON
// payload worth applying to local state. It avoids byte-length guesses in
// callers while still tolerating empty objects, arrays, strings, and nulls.
func HasPayload(v json.RawMessage) bool {
	trimmed := bytes.TrimSpace(v)
	if len(trimmed) == 0 {
		return false
	}
	return !bytes.Equal(trimmed, []byte("null")) &&
		!bytes.Equal(trimmed, []byte("{}")) &&
		!bytes.Equal(trimmed, []byte("[]")) &&
		!bytes.Equal(trimmed, []byte(`""`))
}
