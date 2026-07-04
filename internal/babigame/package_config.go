package babigame

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// PackageConfig is the live package metadata returned by /pack/queryPackageConfig.
type PackageConfig struct {
	GameVersion string
	EntryPath   string
	CDNs        []string
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
