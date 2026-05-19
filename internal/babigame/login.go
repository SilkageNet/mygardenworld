package babigame

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// PerformLoginWithPassword runs the captured 14-step login chain and returns
// a Session ready for Client.Connect. Mirrors the Python helper of the same
// name in scripts/tools/garden_client.py.
//
// Steps that aren't strictly required (BI reports, queryInitParams variants,
// pack/getIosCaid) are skipped - they're observability sugar.
func PerformLoginWithPassword(ctx context.Context, http *HTTPClient, username, password string, isSimulator int) (*Session, error) {
	// Best-effort startup probes. Failures here are not fatal; the iOS client
	// always runs them but the server doesn't gate login on the result.
	for _, fn := range []func(context.Context) (map[string]any, error){
		http.AccountTokenVerify,
		http.QueryInitParams,
	} {
		_, _ = fn(ctx)
	}

	native, err := http.AccountLoginUsername(ctx, username, password)
	if err != nil {
		return nil, fmt.Errorf("account/login: %w", err)
	}
	return finishLogin(ctx, http, native, isSimulator)
}

// PerformLoginWithNative is the moat-bridge: skip the SDK login and go straight
// from a captured / cached NativeLogin to a Session. Useful in tests and when
// you already have a token from elsewhere.
func PerformLoginWithNative(ctx context.Context, http *HTTPClient, native NativeLogin, isSimulator int) (*Session, error) {
	return finishLogin(ctx, http, native, isSimulator)
}

func finishLogin(ctx context.Context, http *HTTPClient, native NativeLogin, isSimulator int) (*Session, error) {
	gameLogin, err := http.GameLogin(ctx, native, "", "", "", "")
	if err != nil {
		return nil, err
	}
	if _, err := http.QueryLoginParams(ctx); err != nil {
		// Non-fatal; server returns this for UI flags only.
	}

	gw, err := http.GWIndexLogin(ctx, gameLogin, isSimulator)
	if err != nil {
		return nil, err
	}
	v, _ := gw["v"].(map[string]any)
	if v == nil {
		return nil, fmt.Errorf("gw index.login response missing v: %v", gw)
	}
	acc, _ := v["acc"].(map[string]any)
	other, _ := v["$other"].(map[string]any)
	aid := readInt64(acc, "id")
	gsIdx := readInt(acc, "lastGsIdx")
	routeToken, _ := other["token"].(string)
	if aid == 0 || gsIdx == 0 || routeToken == "" {
		return nil, fmt.Errorf("gw index.login missing acc.id/lastGsIdx/$other.token: aid=%d gsIdx=%d route=%q", aid, gsIdx, routeToken)
	}

	chnID := readInt(acc, "chnId", "loginChnId")
	if chnID == 0 {
		chnID = http.Cfg.ChannelID
	}
	infos, _ := http.GWGetGsInfoList(ctx, aid, gsIdx, chnID)

	session := &Session{
		Cfg:        http.Cfg,
		DeviceID:   http.DeviceID,
		UUID:       http.UUID,
		Session0:   http.Session0,
		Native:     native,
		GameLogin:  gameLogin,
		AID:        aid,
		GsIdx:      gsIdx,
		RouteToken: routeToken,
		AccountRaw: acc,
	}
	for _, info := range infos {
		if info.Status != 1 {
			continue
		}
		session.GsHost = info.Host
		session.GsPort = info.Port
		session.GsPortSSL = info.PortSSL
		break
	}
	if session.GsHost == "" && len(infos) > 0 {
		// No status=1 candidate; take the first regardless.
		info := infos[0]
		session.GsHost = info.Host
		session.GsPort = info.Port
		session.GsPortSSL = info.PortSSL
	}
	return session, nil
}

func readInt64(m map[string]any, key string) int64 {
	if m == nil {
		return 0
	}
	switch x := m[key].(type) {
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case int64:
		return x
	case json.Number:
		i, _ := x.Int64()
		return i
	case string:
		// Some servers stringify large ids; parse defensively.
		var i int64
		_, _ = fmt.Sscan(x, &i)
		return i
	}
	return 0
}

// MarshalSessionJSON converts a Session to a JSON byte slice for sqlite
// storage. Restoring goes through UnmarshalSessionJSON.
func MarshalSessionJSON(s *Session) ([]byte, error) {
	return json.Marshal(struct {
		DeviceID   string          `json:"device_id"`
		UUID       string          `json:"uuid"`
		Session0   string          `json:"session0"`
		Native     NativeLogin     `json:"native"`
		GameLogin  GameLoginResult `json:"game_login"`
		AID        int64           `json:"aid"`
		GsIdx      int             `json:"gs_idx"`
		RouteToken string          `json:"route_token"`
		AccountRaw map[string]any  `json:"account_raw,omitempty"`
		GsHost     string          `json:"gs_host,omitempty"`
		GsPort     int             `json:"gs_port,omitempty"`
		GsPortSSL  int             `json:"gs_port_ssl,omitempty"`
		SavedAt    int64           `json:"saved_at"`
	}{
		DeviceID:   s.DeviceID,
		UUID:       s.UUID,
		Session0:   s.Session0,
		Native:     s.Native,
		GameLogin:  s.GameLogin,
		AID:        s.AID,
		GsIdx:      s.GsIdx,
		RouteToken: s.RouteToken,
		AccountRaw: s.AccountRaw,
		GsHost:     s.GsHost,
		GsPort:     s.GsPort,
		GsPortSSL:  s.GsPortSSL,
		SavedAt:    time.Now().Unix(),
	})
}

// UnmarshalSessionJSON inflates a session blob written by MarshalSessionJSON.
func UnmarshalSessionJSON(data []byte, cfg Config) (*Session, error) {
	var raw struct {
		DeviceID   string          `json:"device_id"`
		UUID       string          `json:"uuid"`
		Session0   string          `json:"session0"`
		Native     NativeLogin     `json:"native"`
		GameLogin  GameLoginResult `json:"game_login"`
		AID        int64           `json:"aid"`
		GsIdx      int             `json:"gs_idx"`
		RouteToken string          `json:"route_token"`
		AccountRaw map[string]any  `json:"account_raw,omitempty"`
		GsHost     string          `json:"gs_host,omitempty"`
		GsPort     int             `json:"gs_port,omitempty"`
		GsPortSSL  int             `json:"gs_port_ssl,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &Session{
		Cfg:        cfg,
		DeviceID:   raw.DeviceID,
		UUID:       raw.UUID,
		Session0:   raw.Session0,
		Native:     raw.Native,
		GameLogin:  raw.GameLogin,
		AID:        raw.AID,
		GsIdx:      raw.GsIdx,
		RouteToken: raw.RouteToken,
		AccountRaw: raw.AccountRaw,
		GsHost:     raw.GsHost,
		GsPort:     raw.GsPort,
		GsPortSSL:  raw.GsPortSSL,
	}, nil
}
