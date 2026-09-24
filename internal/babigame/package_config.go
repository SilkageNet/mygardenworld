package babigame

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// PackageConfig is the live package metadata returned by /pack/queryPackageConfig.
type PackageConfig struct {
	GameVersion string
	EntryPath   string
	CDNs        []string
}

// PackInitResult is the live /pack/init response used before account/game login.
// Official iOS treats code=302 + status=success as OK and stores url as GAME_URL.
type PackInitResult struct {
	URL            string
	Params         map[string]string
	Session1Cipher bool
	Raw            map[string]any
}

// PackInit asks the platform for the CDN entry URL and feature flags.
// Populates c.NotifyURL and c.Session1Cipher on success.
func (c *HTTPClient) PackInit(ctx context.Context) (PackInitResult, error) {
	body := map[string]any{
		"packageName":  c.Cfg.PackageName,
		"deviceId":     c.DeviceID,
		"platform":     "ios",
		"version":      c.Cfg.AppVersionCode,
		"session0":     c.Session0,
		"lang":         "zh",
		"appVersion":   c.Cfg.AppVersion,
		"gameVersion":  c.Cfg.GameVersion,
		"uuid":         c.UUID,
		"packageId":    c.Cfg.PackageID,
		"zoneCode":     c.Cfg.ZoneCode,
	}
	path := "/pack/init/packageName/" + c.Cfg.PackageName
	resp, _, err := c.PostJSON(ctx, c.Cfg.HostAPI, path, body, c.headersBasic())
	if err != nil {
		return PackInitResult{}, err
	}
	status, _ := resp["status"].(string)
	rawURL, _ := resp["url"].(string)
	if status != "success" || rawURL == "" {
		return PackInitResult{}, fmt.Errorf("pack/init non-success: %v", resp)
	}
	out := PackInitResult{URL: rawURL, Raw: resp, Params: map[string]string{}}
	if data, _ := resp["data"].(map[string]any); data != nil {
		if params, _ := data["params"].(string); params != "" {
			for _, part := range strings.Split(params, "&") {
				k, v, ok := strings.Cut(part, "=")
				if !ok || k == "" {
					continue
				}
				out.Params[k] = v
			}
		}
	}
	if out.Params["session1Cipher"] == "1" {
		out.Session1Cipher = true
	}
	c.NotifyURL = rawURL
	c.Session1Cipher = out.Session1Cipher
	// Official client loads GAME_URL and reads uuid from its query string for
	// subsequent /game/login. The pack/init response URL carries a server-
	// issued uuid that must match body.uuid on /game/login (bizCode 902054
	// otherwise).
	adoptNotifyUUID(c)
	if gv := out.Params["gameVersion"]; gv != "" {
		c.Cfg.GameVersion = gv
		c.Cfg.ClientVersion = gv
	}
	return out, nil
}

// adoptNotifyUUID copies uuid from the pack/init notifyUrl into c.UUID when
// present. No-op if NotifyURL is empty or has no uuid query param.
func adoptNotifyUUID(c *HTTPClient) {
	if c == nil || c.NotifyURL == "" {
		return
	}
	u, err := url.Parse(c.NotifyURL)
	if err != nil {
		return
	}
	if id := u.Query().Get("uuid"); id != "" {
		c.UUID = id
	}
}

// QueryPackageConfig asks the platform which game bundle should be used for
// this launch. The returned gameVersion is the current client protocol version
// that the official app uses for /game/login and /gw index.login.
func (c *HTTPClient) QueryPackageConfig(ctx context.Context) (PackageConfig, error) {
	equipmentInfoJSON, err := json.Marshal(struct {
		EquipmentBrand string `json:"equipmentBrand"`
		PushDeviceID   string `json:"pushDeviceId"`
		IDFA           string `json:"idfa"`
		NetworkType    string `json:"netWorkType"`
		RAM            string `json:"ram"`
		DeviceID       string `json:"deviceId"`
		OSVersion      string `json:"osVersion"`
		OS             string `json:"os"`
		ScreenHeight   string `json:"screenHeight"`
		EquipmentModel string `json:"equipmentModel"`
		IDFV           string `json:"idfv"`
		ScreenDensity  string `json:"screenDensity"`
		ScreenWidth    string `json:"screenWidth"`
		CPUType        string `json:"cpuType"`
	}{
		EquipmentBrand: c.Cfg.DeviceBrand,
		NetworkType:    c.Cfg.NetworkType,
		RAM:            c.Cfg.RAMMB,
		DeviceID:       c.DeviceID,
		OSVersion:      c.Cfg.OSVersion,
		OS:             "iOS",
		ScreenHeight:   c.Cfg.ScreenHeightPx,
		EquipmentModel: c.Cfg.DeviceModel,
		IDFV:           c.DeviceID,
		ScreenDensity:  "3",
		ScreenWidth:    c.Cfg.ScreenWidthPx,
		CPUType:        c.Cfg.CPUType,
	})
	if err != nil {
		return PackageConfig{}, fmt.Errorf("marshal equipmentInfo: %w", err)
	}
	equipmentInfo := string(equipmentInfoJSON)
	body := map[string]any{
		"equipmentInfo":    equipmentInfo,
		"sysLanguage":      c.Cfg.SysLanguage,
		"simCountryCode":   "",
		"version":          c.Cfg.AppVersionCode,
		"session0":         c.Session0,
		"isp":              "",
		"deviceId":         c.DeviceID,
		"packageName":      c.Cfg.PackageName,
		"appStartTime":     nowMsTime(),
		"platform":         "ios",
		"timeZone":         c.Cfg.TimeZoneHour,
		"language":         c.Cfg.RuntimeLanguage,
		"storeCountryCode": "CN",
	}
	resp, _, err := c.PostJSON(ctx, c.Cfg.HostAPI, "/pack/queryPackageConfig", body, c.headersBasic())
	if err != nil {
		return PackageConfig{}, err
	}
	data, _ := resp["data"].(map[string]any)
	gameConfig, _ := data["gameConfig"].(map[string]any)
	entryConfig, _ := data["entryConfig"].(map[string]any)
	pkg := PackageConfig{
		GameVersion: stringOf(gameConfig["gameVersion"]),
		EntryPath:   stringOf(entryConfig["path"]),
	}
	for _, cdn := range anySlice(entryConfig["cdnList"]) {
		if s := strings.TrimSpace(stringOf(cdn)); s != "" {
			pkg.CDNs = append(pkg.CDNs, strings.TrimRight(s, "/"))
		}
	}
	if pkg.GameVersion == "" && pkg.EntryPath == "" {
		return PackageConfig{}, fmt.Errorf("queryPackageConfig missing gameVersion/entryConfig: %v", resp)
	}
	return pkg, nil
}

// GetURL fetches an absolute URL with the same compression handling as PostJSON.
func (c *HTTPClient) GetURL(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header = c.headersBasic()
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	body, err = decompressBody(body, resp.Header.Get("Content-Encoding"))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, &UpstreamError{
			Op:          "GET " + rawURL,
			Host:        resp.Request.URL.Host,
			Path:        resp.Request.URL.Path,
			StatusCode:  resp.StatusCode,
			ContentType: resp.Header.Get("Content-Type"),
			BodyLen:     len(body),
			BodyPreview: previewBytes(body),
			Message:     "non-2xx status",
		}
	}
	return body, nil
}

func anySlice(v any) []any {
	if xs, ok := v.([]any); ok {
		return xs
	}
	return nil
}
