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

type wsErrorMessage struct {
	Msg          string `json:"msg"`
	CodeOfLangJS string `json:"codeOfLangJs"`
	Type         int    `json:"type"`
	Code         int    `json:"code"`
	Param        struct {
		IID         int32  `json:"iid"`
		Reason      string `json:"reason"`
		ExpiredTime int64  `json:"expiredTime"`
	} `json:"param"`
}

// ErrorMsg returns the server error message from the M field, or empty string
// if the response indicates success. Format: {"codeOfLangJs":"$code","msg":"..."}.
// Some gateway errors omit msg and put the human reason in param.reason.
func (d WSResponseD) ErrorMsg() string {
	if len(d.M) == 0 || string(d.M) == "{}" || string(d.M) == "null" {
		return ""
	}
	m, ok := d.parseErrorMessage()
	if ok && strings.TrimSpace(m.Msg) != "" {
		return m.Msg
	}
	if ok {
		if msg := m.paramReasonMessage(); msg != "" {
			return msg
		}
	}
	return string(d.M)
}

// ErrorCode returns the numeric server error code from M, when present.
func (d WSResponseD) ErrorCode() int {
	m, ok := d.parseErrorMessage()
	if !ok {
		return 0
	}
	return m.Code
}

// ErrorType returns the numeric server error type from M, when present.
func (d WSResponseD) ErrorType() int {
	m, ok := d.parseErrorMessage()
	if !ok {
		return 0
	}
	return m.Type
}

// ErrorCodeOfLangJS returns codeOfLangJs from M when present.
func (d WSResponseD) ErrorCodeOfLangJS() string {
	m, ok := d.parseErrorMessage()
	if !ok {
		return ""
	}
	return strings.TrimSpace(m.CodeOfLangJS)
}

// MissingItemID returns param.iid from material-shortage errors, when present.
func (d WSResponseD) MissingItemID() int32 {
	m, ok := d.parseErrorMessage()
	if !ok {
		return 0
	}
	return m.Param.IID
}

func (d WSResponseD) parseErrorMessage() (wsErrorMessage, bool) {
	if len(d.M) == 0 || string(d.M) == "{}" || string(d.M) == "null" {
		return wsErrorMessage{}, false
	}
	var m wsErrorMessage
	if json.Unmarshal(d.M, &m) != nil {
		return wsErrorMessage{}, false
	}
	return m, true
}

func (m wsErrorMessage) paramReasonMessage() string {
	reason := strings.TrimSpace(m.Param.Reason)
	if reason == "" {
		return ""
	}
	if m.Param.ExpiredTime <= 0 {
		return reason
	}
	expiresAt := time.UnixMilli(m.Param.ExpiredTime).In(time.FixedZone("CST", 8*60*60))
	return fmt.Sprintf("%s（解封时间：%s）", reason, expiresAt.Format("2006-01-02 15:04:05"))
}

func (d WSResponseD) hasBlockingErrorCode() bool {
	if d.ErrorCode() != 105 {
		return false
	}
	msg := d.ErrorMsg()
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "封禁") || strings.Contains(msg, "禁止") || strings.Contains(msg, "封号")
}

func (d WSResponseD) hasSessionExpiredMessage() bool {
	msg := d.ErrorMsg()
	if msg == "" {
		return false
	}
	return strings.Contains(msg, "会话已过期") ||
		strings.Contains(msg, "重新登录") ||
		IsSessionDisplacementReason(msg)
}

// IsSessionDisplaced reports whether this error explicitly says that another
// login replaced the current session. Generic expiry and gateway blocks are
// deliberately excluded: callers may safely use this narrower predicate to
// decide whether an automatic login retry is appropriate.
func (d WSResponseD) IsSessionDisplaced() bool {
	return d.IsError() && IsSessionDisplacementReason(d.ErrorMsg())
}

// IsSessionDisplacementReason recognizes only explicit same-account login
// replacement messages. It intentionally does not match generic requests to
// re-login, expired credentials, bans, maintenance, or ordinary RPC errors.
func IsSessionDisplacementReason(reason string) bool {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return false
	}
	for _, marker := range []string{
		"异地登录",
		"挤下线",
		"挤号",
		"顶下线",
		"其他设备登录",
		"其它设备登录",
		"别处登录",
		"会话已被替换",
		"会话被替换",
		"登录状态已被替换",
		"登录被替换",
	} {
		if strings.Contains(reason, marker) {
			return true
		}
	}
	return false
}

// SessionDisplacementFromBinary recognizes the observed bst usr_kick notice
// used by the official client for a same-account login elsewhere. Server/GM
// kicks, kick-all notices, and maintenance closures fail closed.
func SessionDisplacementFromBinary(items []json.RawMessage) (string, bool) {
	for _, item := range items {
		var raw map[string]json.RawMessage
		if json.Unmarshal(item, &raw) != nil {
			continue
		}
		name := rawJSONString(raw["name"])
		if name == "" {
			name = rawJSONString(raw["0"])
		}
		if name != "usr_kick" {
			continue
		}

		content := raw["content"]
		if len(content) == 0 {
			content = raw["1"]
		}
		var kick map[string]json.RawMessage
		if json.Unmarshal(content, &kick) != nil {
			encoded := rawJSONString(content)
			if encoded == "" || json.Unmarshal([]byte(encoded), &kick) != nil {
				continue
			}
		}
		isGM, gmOK := optionalRawJSONInt(kick, "isGM", "4")
		isClose, closeOK := optionalRawJSONInt(kick, "isClose", "5")
		if !gmOK || !closeOK || isGM != 0 || isClose != 0 {
			continue
		}
		return "账号在其他设备登录，当前会话被替换", true
	}
	return "", false
}

func rawJSONString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return value
}

func optionalRawJSONInt(values map[string]json.RawMessage, keys ...string) (int64, bool) {
	for _, key := range keys {
		raw, exists := values[key]
		if !exists || len(raw) == 0 {
			continue
		}
		var value int64
		if json.Unmarshal(raw, &value) == nil {
			return value, true
		}
		return 0, false
	}
	return 0, true
}

func (d WSResponseD) hasSessionExpiredType() bool {
	return d.ErrorType() == 90000
}

func (d WSResponseD) isSessionInvalidatingError() bool {
	if !d.IsError() {
		return false
	}
	if d.hasSessionExpiredType() {
		return true
	}
	if d.hasBlockingErrorCode() {
		return true
	}
	return d.hasSessionExpiredMessage()
}

// IsError returns true if the response contains a server-side error in M.
func (d WSResponseD) IsError() bool {
	return len(d.M) > 0 && string(d.M) != "{}" && string(d.M) != "null"
}

// IsSessionExpired reports whether the server invalidated or rejected this
// websocket session, usually because another login displaced it or a gateway
// block prevents the session from continuing.
func (d WSResponseD) IsSessionExpired() bool {
	return d.isSessionInvalidatingError()
}

// requestSeq is a process-wide counter used as a fallback if the WS client's
// internal sequence is missing. The server doesn't care about the value; it
// just echoes the K back.
var requestSeq atomic.Int64

// nextSeq atomically advances the seq counter and returns the new value.
func nextSeq() int64 { return requestSeq.Add(1) }

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
