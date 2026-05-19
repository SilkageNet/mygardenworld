package babigame

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/andybalholm/brotli"
)

// HTTPClient drives the REST side of the protocol (login chain, BI reports,
// LiveActivity push). One per session. Safe for concurrent use only when
// callers serialize their requests.
type HTTPClient struct {
	Cfg      Config
	DeviceID string
	UUID     string
	Session0 string

	// Token is updated after successful /game/login and used by every
	// token-bearing call.
	Token string

	HTTPClient *http.Client
}

// NewHTTPClient initialises an HTTPClient with sensible defaults. Pass a
// pre-allocated DeviceID/UUID/Session0 to keep them stable across restarts.
func NewHTTPClient(cfg Config, deviceID, uuidStr, session0 string) *HTTPClient {
	if deviceID == "" {
		deviceID = RandomDeviceID()
	}
	if uuidStr == "" {
		uuidStr = RandomUUID()
	}
	if session0 == "" {
		session0 = RandomSessionID()
	}
	return &HTTPClient{
		Cfg:      cfg,
		DeviceID: deviceID,
		UUID:     uuidStr,
		Session0: session0,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DisableCompression: true,
			},
		},
	}
}

func (c *HTTPClient) urlOf(host, path string) string {
	return "https://" + host + path
}

func (c *HTTPClient) gamePath(name string) string {
	return fmt.Sprintf("/game/%s/mdcl/c%d/mdgid/%d/env/%s", name, c.Cfg.ChannelID, c.Cfg.MdGid, c.Cfg.Env)
}

func (c *HTTPClient) headersBasic() http.Header {
	h := http.Header{}
	h.Set("accept", "*/*")
	h.Set("accept-language", "zh-CN,zh-Hans;q=0.9")
	h.Set("accept-encoding", "gzip, deflate, br")
	h.Set("cache-control", "no-cache")
	h.Set("content-type", "application/json")
	h.Set("user-agent", c.Cfg.UserAgent)
	return h
}

func (c *HTTPClient) headersSDK() http.Header {
	h := c.headersBasic()
	h.Set("appid", c.Cfg.AppID)
	h.Set("packagename", c.Cfg.PackageName)
	h.Set("platform", "iOS")
	h.Set("version", "v"+c.Cfg.AppVersion)
	h.Set("deviceid", c.DeviceID)
	h.Set("lang", "zh")
	h.Set("token", c.Token)
	return h
}

// PostJSON sends body as JSON to host+path with the given headers, returning
// decoded JSON. Returns the raw response too so callers can pull non-JSON
// responses (rare in this protocol, but live_activity sometimes empty-bodies).
//
// Failures surface as *UpstreamError (with host/status/preview) when the wire
// data is recoverable; transport-layer errors round-trip as-is. Response
// strings are sanitized through SanitizeMap so any GBK-encoded Chinese the
// server emits ends up as valid UTF-8 by the time it reaches policy
// persistence, proto marshaling, or `%v`-formatted error wrapping.
func (c *HTTPClient) PostJSON(ctx context.Context, host, path string, body any, headers http.Header) (map[string]any, []byte, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.urlOf(host, path), bytes.NewReader(raw))
	if err != nil {
		return nil, nil, err
	}
	req.Header = headers
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, respBytes, &UpstreamError{
			Op:          "POST " + path,
			Host:        host,
			Path:        path,
			StatusCode:  resp.StatusCode,
			ContentType: resp.Header.Get("Content-Type"),
			BodyLen:     len(respBytes),
			BodyPreview: previewBytes(respBytes),
			Cause:       err,
			Message:     "read body",
		}
	}
	respBytes, err = decompressBody(respBytes, resp.Header.Get("Content-Encoding"))
	if err != nil {
		return nil, respBytes, &UpstreamError{
			Op:          "POST " + path,
			Host:        host,
			Path:        path,
			StatusCode:  resp.StatusCode,
			ContentType: resp.Header.Get("Content-Type"),
			BodyLen:     len(respBytes),
			BodyPreview: previewBytes(respBytes),
			Cause:       err,
			Message:     "decompress body",
		}
	}
	if resp.StatusCode/100 != 2 {
		return nil, respBytes, &UpstreamError{
			Op:          "POST " + path,
			Host:        host,
			Path:        path,
			StatusCode:  resp.StatusCode,
			ContentType: resp.Header.Get("Content-Type"),
			BodyLen:     len(respBytes),
			BodyPreview: previewBytes(respBytes),
			Message:     "non-2xx status",
		}
	}
	if len(respBytes) == 0 {
		return nil, respBytes, nil
	}
	var out map[string]any
	if err := json.Unmarshal(respBytes, &out); err != nil {
		return nil, respBytes, &UpstreamError{
			Op:          "POST " + path,
			Host:        host,
			Path:        path,
			StatusCode:  resp.StatusCode,
			ContentType: resp.Header.Get("Content-Type"),
			BodyLen:     len(respBytes),
			BodyPreview: previewBytes(respBytes),
			Cause:       err,
			Message:     "json unmarshal failed (likely proxy interception or wrong host route)",
		}
	}
	return SanitizeMap(out), respBytes, nil
}

func decompressBody(data []byte, encoding string) ([]byte, error) {
	switch encoding {
	case "br":
		return io.ReadAll(brotli.NewReader(bytes.NewReader(data)))
	case "gzip":
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return data, err
		}
		defer r.Close()
		return io.ReadAll(r)
	case "deflate":
		r := flate.NewReader(bytes.NewReader(data))
		defer r.Close()
		return io.ReadAll(r)
	default:
		return data, nil
	}
}

// AccountLoginUsername performs the gfsdk username/password login.
//
// The password is sent base64-encoded in the `value` field; this is *not*
// encryption, just SDK encoding. The server responds with a payload that
// (somewhere inside) contains content/timestamp/signature/session1; we
// DFS the response for the first dict that has those three keys and treat
// it as the NativeLogin.
func (c *HTTPClient) AccountLoginUsername(ctx context.Context, username, password string) (NativeLogin, error) {
	body := map[string]any{
		"clid":                   1,
		"lang":                   "zh",
		"deviceId":               c.DeviceID,
		"appId":                  c.Cfg.AppID,
		"packageName":            c.Cfg.PackageName,
		"platform":               "iOS",
		"version":                "v" + c.Cfg.AppVersion,
		"name":                   username,
		"value":                  base64.StdEncoding.EncodeToString([]byte(password)),
		"showVerEighteenAgeTips": false,
		"storeCountryCode":       "CN",
		"sysLanguage":            c.Cfg.SysLanguage,
	}
	resp, _, err := c.PostJSON(ctx, c.Cfg.HostMOAC, "/account/login/username/v2", body, c.headersSDK())
	if err != nil {
		return NativeLogin{}, fmt.Errorf("account/login: %w", err)
	}
	native, found := findFirstDictWithKeys(resp, "content", "timestamp", "signature")
	if !found {
		return NativeLogin{}, fmt.Errorf("account/login response missing content/timestamp/signature: %v", resp)
	}
	return nativeFromMap(native), nil
}

// QueryInitParams probes the per-version feature flags. Idempotent; useful
// to call once on startup to surface server-side switches like timeStep.
func (c *HTTPClient) QueryInitParams(ctx context.Context) (map[string]any, error) {
	resp, _, err := c.PostJSON(ctx, c.Cfg.HostAPI, c.gamePath("queryInitParams"),
		map[string]any{"uuid": c.UUID, "lang": "zh"}, c.headersBasic())
	return resp, err
}

// AccountTokenVerify is the SDK-side "is the cached session still valid"
// probe. Always called by the iOS client at startup. We don't depend on its
// result for anything; we always do a fresh login when the daemon spins up.
func (c *HTTPClient) AccountTokenVerify(ctx context.Context) (map[string]any, error) {
	body := map[string]any{
		"clid":        1,
		"lang":        "zh",
		"deviceId":    c.DeviceID,
		"appId":       c.Cfg.AppID,
		"packageName": c.Cfg.PackageName,
		"platform":    "iOS",
		"version":     "v" + c.Cfg.AppVersion,
	}
	resp, _, err := c.PostJSON(ctx, c.Cfg.HostMOAC, "/account/v3/token/verify", body, c.headersSDK())
	return resp, err
}

// GameLogin posts the captured NativeLogin to /game/login and parses out
// token+open_id+content. After this returns, c.Token is set.
func (c *HTTPClient) GameLogin(ctx context.Context, native NativeLogin, clientIP, idfv, caid1MD5, caid2MD5 string) (GameLoginResult, error) {
	if idfv == "" {
		idfv = c.DeviceID
	}
	appInfo := map[string]any{
		"_ip":                 clientIP,
		"_os":                 "ios",
		"_ram":                c.Cfg.RAMMB,
		"_os_version":         c.Cfg.OSVersion,
		"_cpu_type":           c.Cfg.CPUType,
		"_time_zone":          c.Cfg.TimeZoneHour,
		"_game_platform":      "mobilegame",
		"_game_version":       c.Cfg.GameVersion,
		"_sdk_version":        c.Cfg.SDKVersion,
		"_screen_height":      c.Cfg.ScreenHeightPx,
		"_screen_width":       c.Cfg.ScreenWidthPx,
		"_network_type":       c.Cfg.NetworkType,
		"_package_version":    c.Cfg.AppVersion,
		"_native_version":     c.Cfg.AppVersion,
		"_equipment_model":    "iphone 15 pro",
		"_equipment_brand":    "apple",
		"_deviceId":           c.DeviceID,
		"_equipment_language": "zh",
		"_runtime_language":   c.Cfg.RuntimeLanguage,
	}
	appInfoJSON, _ := json.Marshal(appInfo)
	body := map[string]any{
		"content":        native.Content,
		"timestamp":      native.Timestamp,
		"signature":      native.Signature,
		"loginType":      defaultStr(native.LoginType, "account"),
		"appId":          c.Cfg.AppID,
		"apiVersion":     2,
		"isNewDevice":    native.IsNewDevice,
		"hotCloudData":   `""`,
		"deeplinkUrl":    "",
		"mobilePlatform": "ios",
		"idfv":           idfv,
		"caid1_md5":      caid1MD5,
		"caid2_md5":      caid2MD5,
		"packageId":      fmt.Sprintf("%d", c.Cfg.PackageID),
		"lang":           "zh",
		"session1":       defaultStr(native.Session1, RandomSessionID()),
		"appInfo":        string(appInfoJSON),
		"uuid":           c.UUID,
	}
	resp, _, err := c.PostJSON(ctx, c.Cfg.HostAPI, c.gamePath("login"), body, c.headersBasic())
	if err != nil {
		return GameLoginResult{}, fmt.Errorf("game/login: %w", err)
	}
	if status, _ := resp["status"].(string); status != "success" {
		return GameLoginResult{}, fmt.Errorf("game/login non-success: %v", resp)
	}
	rawURL, _ := resp["url"].(string)
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return GameLoginResult{}, fmt.Errorf("game/login url parse: %w", err)
	}
	q := parsed.Query()
	token := q.Get("token")
	openID := q.Get("open_id")
	dataMap, _ := resp["data"].(map[string]any)
	content, _ := dataMap["content"].(string)
	if token == "" || openID == "" || content == "" {
		return GameLoginResult{}, fmt.Errorf("game/login missing token/open_id/content: token_present=%t open_id_present=%t content_len=%d", token != "", openID != "", len(content))
	}
	c.Token = token
	return GameLoginResult{
		Raw:         resp,
		Token:       token,
		OpenID:      openID,
		Content:     content,
		RedirectURL: rawURL,
	}, nil
}

// QueryLoginParams pulls the post-login feature config (token-bound).
func (c *HTTPClient) QueryLoginParams(ctx context.Context) (map[string]any, error) {
	body := map[string]any{"lang": "zh", "token": c.Token}
	resp, _, err := c.PostJSON(ctx, c.Cfg.HostAPI, c.gamePath("queryLoginParams"), body, c.headersBasic())
	return resp, err
}

// gwCall posts a payload tuple through /gw, applying the XOR/MD5 envelope.
// Returns the parsed response object as-is.
func (c *HTTPClient) gwCall(ctx context.Context, payload any, lang string) (map[string]any, error) {
	body, err := BuildGWBody(payload, c.Cfg, lang)
	if err != nil {
		return nil, err
	}
	headers := c.headersBasic()
	headers.Set("connection", "keep-alive")
	headers.Set("host", c.Cfg.HostGW)
	resp, _, err := c.PostJSON(ctx, c.Cfg.HostGW, "/gw", body, headers)
	return resp, err
}

// GWIndexLogin is the second login step: trade token+open_id+content for
// aid + lastGsIdx + routeToken.
func (c *HTTPClient) GWIndexLogin(ctx context.Context, login GameLoginResult, isSimulator int) (map[string]any, error) {
	before := nowMsTime()
	params := map[string]any{
		"token":           login.Token,
		"open_id":         login.OpenID,
		"reportDataGeted": map[string]any{"reyun": map[string]any{}},
		"isNewDevice":     true,
		"content":         login.Content,
		"zoneCode":        c.Cfg.ZoneCode,
		"appVersion":      c.Cfg.AppVersion,
		"packageId":       c.Cfg.PackageID,
		"$ms":             nowMsTime(),
	}
	payload := []any{
		"index.login",
		map[string]any{
			"sdkId":         c.Cfg.SDKID,
			"chnId":         c.Cfg.ChannelID,
			"params":        params,
			"msBeforeLogin": before,
			"msAfterLogin":  nowMsTime(),
			"osType":        1,
			"deviceId":      c.DeviceID,
			"isSimulator":   isSimulator,
			"clientVersion": c.Cfg.ClientVersion,
		},
		nil,
	}
	return c.gwCall(ctx, payload, "zh")
}

// GsInfo is one entry returned by gw.index.getGsInfoList. The server emits
// a numeric-keyed shape; we unpack it here.
type GsInfo struct {
	Host    string
	Port    int
	PortSSL int
	Status  int
	Count   int
	Idx     int
}

// GWGetGsInfoList queries the active game-server endpoint table for the given
// (aid, gsIdx, chnId). The returned list is what the Cocos client would feed
// into its connect-with-load-balance helper. Sorted by ascending count
// (least-loaded first).
func (c *HTTPClient) GWGetGsInfoList(ctx context.Context, aid int64, gsIdx, chnID int) ([]GsInfo, error) {
	payload := []any{
		"index.getGsInfoList",
		map[string]any{
			"aid":   aid,
			"idx":   gsIdx,
			"chnId": chnID,
		},
		nil,
	}
	resp, err := c.gwCall(ctx, payload, "zh")
	if err != nil {
		return nil, err
	}
	v, _ := resp["v"].(map[string]any)
	if v == nil {
		return nil, nil
	}
	// Captured shape: v.6.2 (numeric-keyed; 6 = $gsTot, 2 = gsInfoList).
	var rawList []any
	if six, ok := v["6"].(map[string]any); ok {
		if two, ok := six["2"].([]any); ok {
			rawList = two
		}
	}
	if rawList == nil {
		// Fallback for plaintext shape used by some bundles.
		if gsTot, ok := v["$gsTot"].(map[string]any); ok {
			if gl, ok := gsTot["gsInfoList"].([]any); ok {
				rawList = gl
			}
		}
	}
	out := make([]GsInfo, 0, len(rawList))
	for _, raw := range rawList {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		var info GsInfo
		if v, ok := m["host"].(string); ok {
			info.Host = v
		} else if v, ok := m["0"].(string); ok {
			info.Host = v
		}
		info.Port = readInt(m, "port", "1")
		info.PortSSL = readInt(m, "port_ssl", "2")
		info.Status = readInt(m, "status", "4")
		info.Count = readInt(m, "count", "5")
		info.Idx = readInt(m, "idx", "8")
		if info.Idx == 0 {
			info.Idx = gsIdx
		}
		if info.Host == "" {
			continue
		}
		out = append(out, info)
	}
	// Sort by count ascending so callers can grab out[0] for least-loaded.
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && out[j-1].Count > out[j].Count {
			out[j-1], out[j] = out[j], out[j-1]
			j--
		}
	}
	return out, nil
}

func readInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case float64:
				return int(x)
			case int:
				return x
			case int64:
				return int(x)
			case json.Number:
				if i, err := x.Int64(); err == nil {
					return int(i)
				}
			}
		}
	}
	return 0
}

func defaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func nativeFromMap(m map[string]any) NativeLogin {
	out := NativeLogin{
		Content:   stringOf(m["content"]),
		Signature: stringOf(m["signature"]),
		LoginType: stringOf(firstNonEmpty(m, "loginType", "login_type")),
		Session1:  stringOf(firstNonEmpty(m, "session1", "sessionId", "sessionid")),
	}
	switch ts := m["timestamp"].(type) {
	case float64:
		out.Timestamp = int64(ts)
	case int64:
		out.Timestamp = ts
	case json.Number:
		i, _ := ts.Int64()
		out.Timestamp = i
	}
	if v, ok := m["isNewDevice"].(bool); ok {
		out.IsNewDevice = v
	} else {
		out.IsNewDevice = true
	}
	if out.LoginType == "" {
		out.LoginType = "account"
	}
	if out.Session1 == "" {
		out.Session1 = RandomSessionID()
	}
	return out
}

func firstNonEmpty(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil && v != "" {
			return v
		}
	}
	return nil
}

func stringOf(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// findFirstDictWithKeys recursively scans v looking for the first map that
// contains every required key. Used to pull the gfsdk-wrapped NativeLogin
// payload out of nested response envelopes.
func findFirstDictWithKeys(v any, required ...string) (map[string]any, bool) {
	switch x := v.(type) {
	case map[string]any:
		ok := true
		for _, k := range required {
			if _, has := x[k]; !has {
				ok = false
				break
			}
		}
		if ok {
			return x, true
		}
		for _, child := range x {
			if m, found := findFirstDictWithKeys(child, required...); found {
				return m, true
			}
		}
	case []any:
		for _, child := range x {
			if m, found := findFirstDictWithKeys(child, required...); found {
				return m, true
			}
		}
	}
	return nil, false
}
