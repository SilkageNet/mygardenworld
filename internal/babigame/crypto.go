package babigame

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// CompactJSON marshals v with no spaces, matching json.dumps(separators=(",", ":"))
// from the Python reference. The signature key depends on byte-for-byte equality
// with that form so callers must not pretty-print.
func CompactJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

// MD5Hex returns the hex digest of s with lowercase letters, matching Python's
// hashlib.md5().hexdigest().
func MD5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// GWEncode JSON-stringifies payload then XORs each byte of the resulting UTF-8
// string with mask, returning a JSON int array as a string. Matches the
// gw_encode helper in Python.
//
// Note the Python reference iterates over Python str (i.e. Unicode code points)
// and XORs each code point. For ASCII payloads (the only kind we observe in
// captured /gw bodies) this matches byte-wise XOR. We replicate the Python
// behavior exactly so signatures match.
func GWEncode(payload any, mask byte) (string, error) {
	clear, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("gw encode: marshal: %w", err)
	}
	// The Python reference treats `clear` as a Python str (Unicode), so we
	// decode the UTF-8 here and XOR runes-by-rune-as-int. For purely-ASCII
	// payloads the result is identical to byte-wise XOR.
	runes := []rune(string(clear))
	parts := make([]string, len(runes))
	for i, r := range runes {
		parts[i] = strconv.Itoa(int(r) ^ int(mask))
	}
	return "[" + strings.Join(parts, ",") + "]", nil
}

// GWDecode reverses GWEncode. Accepts the "[a,b,c,...]" string and returns
// the raw plaintext bytes (which is typically JSON; callers parse on demand).
func GWDecode(encodedArray string, mask byte) ([]byte, error) {
	s := strings.TrimSpace(encodedArray)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	clear := make([]rune, 0, len(parts))
	for _, raw := range parts {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, fmt.Errorf("gw decode: bad int %q: %w", raw, err)
		}
		clear = append(clear, rune(n^int(mask)))
	}
	return []byte(string(clear)), nil
}

// GWBody is the {sign, a, l} structure /gw and the WS envelope wrap.
type GWBody struct {
	Sign string `json:"sign"`
	A    string `json:"a"`
	L    string `json:"l"`
}

// BuildGWBody encodes payload and signs it, returning the wire body.
func BuildGWBody(payload any, cfg Config, lang string) (GWBody, error) {
	encoded, err := GWEncode(payload, cfg.GWXorMask)
	if err != nil {
		return GWBody{}, err
	}
	if lang == "" {
		lang = "zh"
	}
	return GWBody{
		Sign: MD5Hex(encoded + cfg.GWSignKey),
		A:    encoded,
		L:    lang,
	}, nil
}

// VerifyGWBody returns true when body.Sign matches MD5(body.A + signKey).
// Used to validate captured frames during tests.
func VerifyGWBody(body GWBody, cfg Config) bool {
	return MD5Hex(body.A+cfg.GWSignKey) == body.Sign
}
