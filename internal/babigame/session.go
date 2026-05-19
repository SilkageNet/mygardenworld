package babigame

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// NativeLogin captures the gfsdk-account-login output that gets forwarded into
// /game/login. The Python reference fills these from
// `POST moac.babigame.cn/account/login/username/v2`.
type NativeLogin struct {
	Content     string `json:"content"`
	Timestamp   int64  `json:"timestamp"`
	Signature   string `json:"signature"`
	LoginType   string `json:"loginType"`
	Session1    string `json:"session1"`
	IsNewDevice bool   `json:"isNewDevice"`
}

// GameLoginResult is the parsed response of POST /game/login.
type GameLoginResult struct {
	// Raw is the full server response, kept verbatim for forensics.
	Raw map[string]any `json:"raw"`

	// Token + OpenID extracted from data.url's query string. Token is the
	// HTTP "bearer" used by every subsequent /game/* and /liveActivity/push
	// call. OpenID is the platform open id, equal to acc.name.
	Token  string `json:"token"`
	OpenID string `json:"open_id"`

	// Content is the data.content field; opaque server-encrypted blob that
	// gets passed verbatim into /gw index.login.
	Content string `json:"content"`

	// RedirectURL is the literal `url` field, kept for debugging.
	RedirectURL string `json:"redirect_url"`
}

// Session is the state needed to hold a live game session. Built by
// PerformLogin and consumed by Client.Connect.
type Session struct {
	Cfg Config

	DeviceID string
	UUID     string
	Session0 string

	Native    NativeLogin
	GameLogin GameLoginResult

	// AID is the platform account id (`acc.id`).
	AID int64
	// GsIdx is the game server index (`acc.lastGsIdx`). Drives the WS sgid.
	GsIdx int
	// RouteToken is `$other.token` from /gw index.login. Used as the WS routing arg.
	RouteToken string

	// AccountRaw is the full `acc` object, kept for diagnostic / re-login.
	AccountRaw map[string]any

	// Resolved game-server endpoint from /gw index.getGsInfoList. Falls back
	// to Cfg defaults if discovery fails.
	GsHost    string
	GsPort    int // ws port
	GsPortSSL int // wss port (used by default)
}

// Token returns the HTTP-side token (used as `token` field in /game/* requests
// and as `Authorization`-equivalent on apipush/apirp endpoints).
func (s *Session) Token() string { return s.GameLogin.Token }

// OpenID is the platform open id.
func (s *Session) OpenID() string { return s.GameLogin.OpenID }

// RouteArg is the second element appended to every WS RPC tuple, formatted
// as "<routeToken>|<gsIdx>".
func (s *Session) RouteArg() string {
	return fmt.Sprintf("%s|%d", s.RouteToken, s.GsIdx)
}

// WSURL returns the live wss URL for this session, preferring the discovered
// gs_host/port_ssl over the config defaults.
func (s *Session) WSURL() string {
	host := s.GsHost
	if host == "" {
		host = s.Cfg.HostWSDefault
	}
	port := s.GsPortSSL
	if port <= 0 {
		port = s.Cfg.HostWSDefaultPort
	}
	if port == 443 {
		return fmt.Sprintf("wss://%s/?sgid=%d", host, s.GsIdx)
	}
	return fmt.Sprintf("wss://%s:%d/?sgid=%d", host, port, s.GsIdx)
}

// RandomSessionID matches the gfsdk SDK's session id format (32 alpha-num).
func RandomSessionID() string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	var b [32]byte
	_, _ = rand.Read(b[:])
	out := make([]byte, 32)
	for i, x := range b {
		out[i] = alphabet[int(x)%len(alphabet)]
	}
	return string(out)
}

// RandomDeviceID returns a UUID-shaped uppercase string used as the iOS IDFV.
func RandomDeviceID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hexStr := strings.ToUpper(hex.EncodeToString(b[:]))
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hexStr[0:8], hexStr[8:12], hexStr[12:16], hexStr[16:20], hexStr[20:32])
}

// RandomUUID returns a lowercase RFC4122-shaped UUID v4 string.
func RandomUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hexStr := hex.EncodeToString(b[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hexStr[0:8], hexStr[8:12], hexStr[12:16], hexStr[16:20], hexStr[20:32])
}

// nowMsTime returns the current time as a millisecond timestamp.
func nowMsTime() int64 { return time.Now().UnixMilli() }
