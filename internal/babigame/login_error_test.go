package babigame

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestGameLoginRefusalClassificationAndRedaction(t *testing.T) {
	for _, tc := range []struct {
		code   int
		reason string
		risk   bool
	}{
		{902049, "IP 存在登录风险", true},
		{902050, "封禁当前 IP", true},
		{902051, "IP 的请求频率", true},
		{902052, "触发限流", true},
		{902053, "SDK 账号被封禁", true},
		{902048, "原因尚未识别", false},
		{123456, "原因尚未识别", false},
	} {
		t.Run(fmt.Sprint(tc.code), func(t *testing.T) {
			cfg, _ := ConfigForChannel(ChannelIOS)
			c := NewHTTPClient(cfg, "", "", "")
			calls := 0
			c.HTTPClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return jsonResponse(fmt.Sprintf(`{"status":"fail","code":302,"bizCode":%d,"url":"https://secret.test/?mdErrCode=106&token=secret-token&uuid=secret-uuid","data":{"content":"secret-content"},"message":"secret-message"}`, tc.code)), nil
			})
			_, err := c.GameLoginWithPayload(context.Background(), map[string]any{})
			var rejected *GameLoginError
			if !errors.As(err, &rejected) || rejected.BizCode != tc.code || rejected.Code != 302 || rejected.MDErrCode != 106 {
				t.Fatalf("wrong diagnostics: %v", err)
			}
			wrapped := fmt.Errorf("login: %w", err)
			if IsLoginRiskError(wrapped) != tc.risk || !strings.Contains(err.Error(), tc.reason) {
				t.Fatal(err)
			}
			if strings.Contains(err.Error(), "secret") || calls != 1 || c.Token != "" {
				t.Fatalf("leaked data, followed URL or accepted refusal: %v, calls=%d", err, calls)
			}
			if tc.risk && !strings.Contains(err.Error(), "同一服务器上集中登录过多账号") {
				t.Fatal("deployment advice missing")
			}
		})
	}
	if IsLoginRiskError(errors.New("902049")) {
		t.Fatal("unstructured text is not evidence")
	}
}

func TestGameLoginSuccessStillParsesSession(t *testing.T) {
	for _, status := range []string{`"success"`, `"1"`, `1`} {
		cfg, _ := ConfigForChannel(ChannelIOS)
		c := NewHTTPClient(cfg, "", "", "")
		c.HTTPClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return jsonResponse(fmt.Sprintf(`{"status":%s,"url":"https://game.test/?token=valid&open_id=player","data":{"content":"payload"}}`, status)), nil
		})
		result, err := c.GameLoginWithPayload(context.Background(), map[string]any{})
		if err != nil || result.OpenID != "player" || result.Content != "payload" || c.Token != "valid" {
			t.Fatalf("status %s: %+v %v", status, result, err)
		}
	}
}

func TestGameLoginErrorMalformedDiagnostics(t *testing.T) {
	for _, raw := range []string{"%", "https://game.test/?mdErrCode=secret", ""} {
		e := gameLoginError(map[string]any{"bizCode": "902049", "code": "302", "url": raw})
		if e.BizCode != 902049 || e.Code != 302 || e.MDErrCode != 0 {
			t.Fatal(e)
		}
	}
	for _, code := range []any{"902049secret", 902049.5, nil, "999999999999999999999999"} {
		if e := gameLoginError(map[string]any{"bizCode": code}); e.BizCode != 0 || IsLoginRiskError(e) {
			t.Fatal(e)
		}
	}
}
