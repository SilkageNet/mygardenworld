package babigame

import (
	"context"
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
	equipmentInfo := fmt.Sprintf(`{"equipmentBrand":"%s","pushDeviceId":"","idfa":"","netWorkType":"%s","ram":"%s","deviceId":"%s","osVersion":"%s","os":"iOS","screenHeight":"%s","equipmentModel":"%s","idfv":"%s","screenDensity":"3","screenWidth":"%s","cpuType":"%s"}`,
		c.Cfg.DeviceBrand,
		c.Cfg.NetworkType,
		c.Cfg.RAMMB,
		c.DeviceID,
		c.Cfg.OSVersion,
		c.Cfg.ScreenHeightPx,
		c.Cfg.DeviceModel,
		c.DeviceID,
		c.Cfg.ScreenWidthPx,
		c.Cfg.CPUType,
	)
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
	defer resp.Body.Close()
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
