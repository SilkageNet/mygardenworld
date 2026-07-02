package babigame

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// WSEnvelopeOut is the client→server text frame body (after the sentinel prefix).
//
//	$#|#$ + json.dumps({"e":"request","d":{"r":"gs","p":<gw_body>,"k":"ws|ms|seq"}})
type WSEnvelopeOut struct {
	E string         `json:"e"` // always "request"
	D WSEnvelopeOutD `json:"d"`
}

type WSEnvelopeOutD struct {
	R string `json:"r"` // always "gs"
	P GWBody `json:"p"` // gw signed body
	K string `json:"k"` // request key, mirrored back in the response
}

// WSEnvelopeIn is the server→client text frame.
type WSEnvelopeIn struct {
	E string          `json:"e"` // typically "response"
	D json.RawMessage `json:"d"`
}

// WSResponseD is the inner d field of a server response.
type WSResponseD struct {
	R json.RawMessage `json:"r,omitempty"` // route — string "gs" or numeric timestamp
	V json.RawMessage `json:"v,omitempty"` // namespace payload
	D json.RawMessage `json:"d,omitempty"` // server hash or encoded array
	T json.RawMessage `json:"t,omitempty"` // server response ms
	K string          `json:"k,omitempty"` // matches the request k
	M json.RawMessage `json:"m,omitempty"` // message/error envelope
}

// ErrorMsg returns the server error message from the M field, or empty string
// if the response indicates success. Format: {"codeOfLangJs":"$code","msg":"..."}
func (d WSResponseD) ErrorMsg() string {
	if len(d.M) == 0 || string(d.M) == "{}" || string(d.M) == "null" {
		return ""
	}
	var m struct {
		Msg  string `json:"msg"`
		Code string `json:"codeOfLangJs"`
	}
	if json.Unmarshal(d.M, &m) == nil && m.Msg != "" {
		return m.Msg
	}
	return string(d.M)
}

// ErrorType returns the numeric server error type from M, when present.
func (d WSResponseD) ErrorType() int {
	if len(d.M) == 0 || string(d.M) == "{}" || string(d.M) == "null" {
		return 0
	}
	var m struct {
		Type int `json:"type"`
	}
	if json.Unmarshal(d.M, &m) != nil {
		return 0
	}
	return m.Type
}

// MissingItemID returns param.iid from material-shortage errors, when present.
func (d WSResponseD) MissingItemID() int32 {
	if len(d.M) == 0 || string(d.M) == "{}" || string(d.M) == "null" {
		return 0
	}
	var m struct {
		Param struct {
			IID int32 `json:"iid"`
		} `json:"param"`
	}
	if json.Unmarshal(d.M, &m) != nil {
		return 0
	}
	return m.Param.IID
}

// IsError returns true if the response contains a server-side error in M.
func (d WSResponseD) IsError() bool {
	return len(d.M) > 0 && string(d.M) != "{}" && string(d.M) != "null"
}

// IsSessionExpired reports whether the server invalidated this websocket
// session, usually because another login displaced it.
func (d WSResponseD) IsSessionExpired() bool {
	if !d.IsError() {
		return false
	}
	if d.ErrorType() == 90000 {
		return true
	}
	msg := d.ErrorMsg()
	return strings.Contains(msg, "会话已过期") ||
		strings.Contains(msg, "重新登录") ||
		strings.Contains(msg, "异地登录") ||
		strings.Contains(msg, "挤下线")
}

// requestSeq is a process-wide counter used as a fallback if the WS client's
// internal sequence is missing. The server doesn't care about the value; it
// just echoes the K back.
var requestSeq int64

// nextSeq atomically advances the seq counter and returns the new value.
func nextSeq() int64 { return atomic.AddInt64(&requestSeq, 1) }

// nowMs returns wall-clock milliseconds. Captures matched on this format.
func nowMs() int64 { return time.Now().UnixMilli() }

// BuildRequest constructs the full sentinel-prefixed wire frame for the
// (rpcName, args, routeArg) tuple. The returned k is the correlation token.
//
// payload corresponds to the Python ws_request: a 3-tuple [name, args, route].
func BuildRequest(rpcName string, args any, routeArg string, seq int64, cfg Config) (string, string, error) {
	if seq <= 0 {
		seq = nextSeq()
	}
	k := fmt.Sprintf("ws|%d|%d", nowMs(), seq)
	payload := []any{rpcName, args, nil}
	if routeArg != "" {
		payload = []any{rpcName, args, routeArg}
	}
	body, err := BuildGWBody(payload, cfg, "zh")
	if err != nil {
		return "", "", err
	}
	env := WSEnvelopeOut{
		E: "request",
		D: WSEnvelopeOutD{R: "gs", P: body, K: k},
	}
	raw, err := json.Marshal(env)
	if err != nil {
		return "", "", fmt.Errorf("envelope marshal: %w", err)
	}
	return cfg.WSSentinel + string(raw), k, nil
}

// ParseTextFrame strips the sentinel prefix (if present) and decodes the
// outer envelope. Server text frames typically lack the sentinel; client
// frames always carry it.
func ParseTextFrame(text string) (WSEnvelopeIn, error) {
	text = strings.TrimPrefix(text, "$#|#$")
	var env WSEnvelopeIn
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		return env, err
	}
	return env, nil
}

// ParseBinaryFrame slides a json.Decoder across the bytes and pulls out every
// embedded JSON object. Mirrors the Python parse_ws_binary helper.
//
// Server-pushed binary frames carry self-described chunks like:
//
//	{"t":"bst"}
//	{"t":"sysMsg","e":3,"sn":"G.ISysMsg"}
//	{"4":{...payload...},"13":<ms>}
//
// We don't bother decoding the wrapping length-prefix bytes; sliding the
// decoder is sufficient for every observed shape and survives schema drift.
func ParseBinaryFrame(raw []byte) []json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make([]json.RawMessage, 0, 4)
	i := 0
	n := len(raw)
	for i < n {
		c := raw[i]
		if c != '{' && c != '[' {
			i++
			continue
		}
		dec := json.NewDecoder(strings.NewReader(string(raw[i:])))
		var obj json.RawMessage
		if err := dec.Decode(&obj); err != nil {
			i++
			continue
		}
		out = append(out, obj)
		i += int(dec.InputOffset())
	}
	return out
}
