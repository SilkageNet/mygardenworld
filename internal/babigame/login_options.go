package babigame

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// preparePasswordLogin consumes the native package bootstrap before sending
// credentials. A locally generated UUID has no server-side init parameters.
func (c *HTTPClient) preparePasswordLogin(ctx context.Context) error {
	if c.launchUUID == "" {
		if _, err := c.QueryPackageConfig(ctx); err != nil {
			return fmt.Errorf("login package initialization: %w", err)
		}
	}
	c.UUID = c.launchUUID
	c.loginParams = nil
	c.gameSession1 = ""
	if _, err := c.QueryInitParams(ctx); err != nil {
		return fmt.Errorf("login SDK initialization: %w", err)
	}
	return c.prepareRequestToken(ctx)
}

func (c *HTTPClient) prepareRequestToken(ctx context.Context) error {
	// This preflight predates session1Cipher. Do not silently omit it if the
	// server enables it, and do not copy the SDK's unbounded retry loop.
	if sdkOptionEnabled(c.loginOption("getRequestToken")) {
		host := strings.SplitN(c.Cfg.HostAPI, ".", 2)
		if len(host) != 2 {
			return fmt.Errorf("invalid SDK request-token host")
		}
		resp, _, err := c.PostJSON(ctx, host[0]+"rq."+host[1], "/game/getRequestToken", map[string]any{
			"mdgid": c.Cfg.MdGid, "mdcl": c.Cfg.ChannelID,
			"packageId": strconv.Itoa(c.Cfg.PackageID), "env": c.Cfg.Env,
			"uuid": c.UUID, "lang": c.Cfg.RuntimeLanguage,
		}, c.headersBasic())
		if err != nil {
			return fmt.Errorf("login request-token preflight: %w", err)
		}
		if !gameLoginSucceeded(resp["status"]) {
			return &GameLoginError{BizCode: 902048, Code: loginDiagnosticCode(resp["code"])}
		}
	}
	return nil
}

func (c *HTTPClient) acceptLaunchURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid package launch URL")
	}
	params := make(map[string]any)
	for key, values := range u.Query() {
		if len(values) != 1 {
			return fmt.Errorf("ambiguous package launch options")
		}
		params[key] = values[0]
	}
	if err := c.validateLoginScope(params); err != nil {
		return err
	}
	uuid, _ := params["uuid"].(string)
	if strings.TrimSpace(uuid) == "" {
		return fmt.Errorf("package launch URL missing UUID")
	}
	c.launchUUID = uuid
	c.launchParams = params
	return nil
}

// SDK options come from userParams first, with the launch URL as fallback.
func (c *HTTPClient) loginOption(key string) any {
	if value, present := c.loginParams[key]; present {
		return value
	}
	return c.launchParams[key]
}

func (c *HTTPClient) validateLoginScope(params map[string]any) error {
	for _, field := range []struct{ key, want string }{
		{"mdgid", strconv.Itoa(c.Cfg.MdGid)},
		{"mdclid", strconv.Itoa(c.Cfg.ChannelID)},
		{"packageId", strconv.Itoa(c.Cfg.PackageID)},
		{"env", c.Cfg.Env},
	} {
		if value, present := params[field.key]; present && stringOf(value) != field.want {
			return fmt.Errorf("SDK initialization %s does not match configured channel", field.key)
		}
	}
	return nil
}

func sdkOptionEnabled(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return v != "" && v != "0" && v != "false"
	default:
		return loginDiagnosticCode(value) != 0
	}
}

// withGameLoginOptions mirrors modosdk.filterSendData/getSession1 from the
// authorized 187 client and the 450.0.15 web SDK. mdSession1 is opaque: forward
// it unchanged when enabled, otherwise retain a locally generated s1 value.
func (c *HTTPClient) withGameLoginOptions(payload map[string]any) map[string]any {
	body := make(map[string]any, len(payload)+4)
	for key, value := range payload {
		body[key] = value
	}
	if sdkOptionEnabled(c.loginOption("session1Cipher")) {
		if server, ok := c.loginOption("mdSession1").(string); ok && server != "" {
			c.gameSession1 = server
		}
	}
	if c.gameSession1 == "" {
		c.gameSession1 = "s1" + RandomSessionID()[:17] + strconv.FormatInt(nowMsTime(), 10)
	}
	body["session1"] = c.gameSession1
	body["uuid"] = c.UUID
	body["lang"] = c.Cfg.RuntimeLanguage
	if _, present := body["appInfo"]; !present {
		body["appInfo"] = c.gameLoginAppInfo("")
	}
	return body
}

func (c *HTTPClient) gameLoginAppInfo(clientIP string) string {
	if clientIP == "" {
		clientIP, _ = c.loginOption("ip").(string)
	}
	fields := map[string]string{
		"_ip": clientIP, "_os": c.Cfg.MobilePlatform,
		"_ram": c.Cfg.RAMMB, "_os_version": c.Cfg.OSVersion,
		"_cpu_type": c.Cfg.CPUType, "_time_zone": c.Cfg.TimeZoneHour,
		"_game_platform": c.Cfg.GamePlatform, "_game_version": c.Cfg.GameVersion,
		"_package_name": strings.ToLower(c.Cfg.PackageName), "_sdk_version": c.Cfg.SDKVersion,
		"_screen_height": c.Cfg.ScreenHeightPx, "_screen_width": c.Cfg.ScreenWidthPx,
		"_network_type": c.Cfg.NetworkType, "_package_version": c.Cfg.AppVersion,
		"_native_version": c.Cfg.AppVersion, "_equipment_model": strings.ToLower(c.Cfg.DeviceModel),
		"_equipment_brand": strings.ToLower(c.Cfg.DeviceBrand), "_deviceId": c.DeviceID,
		"_equipment_language": "zh", "_runtime_language": c.Cfg.RuntimeLanguage,
	}
	for key, value := range fields {
		if value == "" {
			delete(fields, key)
		}
	}
	raw, _ := json.Marshal(fields)
	return string(raw)
}
