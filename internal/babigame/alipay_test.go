package babigame

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestAlipayConfigIsChannelScoped(t *testing.T) {
	cfg, err := ConfigForChannel(ChannelAlipay)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PackageName != "cn.hysj.zfb.minigame" || cfg.MdGid != 163 || cfg.ChannelID != 538 || cfg.PackageID != 520 {
		t.Fatalf("unexpected Alipay identity: %+v", cfg)
	}
	if cfg.ClientVersion != "412.0.4" || cfg.HostAPI != "apizfbfast.babigame.cn" || cfg.HostGW != "hygnhmzfb.babigame.cn" {
		t.Fatalf("unexpected Alipay version/hosts: %+v", cfg)
	}
	if cfg.SDKPlatform != "Browser" || cfg.IsNative || cfg.OSType != 0 || cfg.IsSimulator != 0 {
		t.Fatalf("unexpected Alipay device mode: %+v", cfg)
	}
}

func TestAlipayQRBeginAndPoll(t *testing.T) {
	cfg, _ := ConfigForChannel(ChannelAlipay)
	requests := 0
	client := NewAlipayClient(cfg)
	client.GatewayURL = "https://gateway.test"
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests++
		if req.URL.Query().Get("ctoken") == "" {
			t.Fatal("ctoken query missing")
		}
		if req.Header.Get("x-webgw-appId") != alipayWebGWAppID {
			t.Fatalf("x-webgw-appId=%q", req.Header.Get("x-webgw-appId"))
		}
		switch {
		case strings.HasSuffix(req.URL.Path, "/getLoginToken"):
			return jsonResponse(`{"success":true,"data":{"qrCode":{"url":"alipays://qr","token":"qr-token"}}}`), nil
		case strings.HasSuffix(req.URL.Path, "/loginForPc"):
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["token"] != "qr-token" {
				t.Fatalf("poll token=%v", body["token"])
			}
			return jsonResponse(`{"success":true,"data":{"token":"web-token","userId":"42"}}`), nil
		default:
			t.Fatalf("unexpected path %s", req.URL.Path)
			return nil, nil
		}
	})}

	challenge, err := client.BeginQR(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if challenge.URL != "alipays://qr" || challenge.Token != "qr-token" {
		t.Fatalf("challenge=%+v", challenge)
	}
	grant, authorized, err := client.PollQR(context.Background(), challenge.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !authorized || grant.Token != "web-token" || grant.UserID != "42" {
		t.Fatalf("grant=%+v authorized=%v", grant, authorized)
	}
	if requests != 2 {
		t.Fatalf("requests=%d, want 2", requests)
	}
}

func TestAlipayQRPollBusinessErrorStaysPending(t *testing.T) {
	cfg, _ := ConfigForChannel(ChannelAlipay)
	client := NewAlipayClient(cfg)
	client.GatewayURL = "https://gateway.test"
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return jsonResponse(`{"success":false,"errorCode":"WAIT_SCAN","errorMessage":"waiting"}`), nil
	})}
	grant, authorized, err := client.PollQR(context.Background(), "qr-token")
	if err != nil || authorized || grant.Token != "" {
		t.Fatalf("grant=%+v authorized=%v err=%v", grant, authorized, err)
	}
}

func TestAlipayAuthCodeAndYXTPayload(t *testing.T) {
	cfg, _ := ConfigForChannel(ChannelAlipay)
	client := NewAlipayClient(cfg)
	client.GatewayURL = "https://gateway.test"
	client.YXTURL = "https://yxt.test"
	grant := AlipayWebGrant{Token: "web-secret", UserID: "1000000000001234"}
	client.HTTPClient = &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "gateway.test":
			if req.Header.Get(alipayWebTokenHeader) != "web-secret" {
				t.Fatalf("web token header missing")
			}
			if req.Header.Get("x-webgw-ldc-uid") != "23" {
				t.Fatalf("x-webgw-ldc-uid=%q", req.Header.Get("x-webgw-ldc-uid"))
			}
			var body map[string]any
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			switch {
			case strings.Contains(req.URL.Path, "/queryPcGameInfo/"):
				if body["gameId"] != alipayGameSlug {
					t.Fatalf("game info body=%v", body)
				}
				return jsonResponse(`{"success":true,"data":{"appId":"2021000000000001"}}`), nil
			case strings.Contains(req.URL.Path, "/queryPcGameAuthInfo/"):
				if body["appId"] != "2021000000000001" {
					t.Fatalf("game auth body=%v", body)
				}
				return jsonResponse(`{"success":true,"data":{"authCode":"auth-code"}}`), nil
			default:
				t.Fatalf("unexpected gateway path %s", req.URL.Path)
				return nil, nil
			}
		case "yxt.test":
			if err := req.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if req.Form.Get("authCode") != "auth-code" || req.Form.Get("scene") != "other" {
				t.Fatalf("form=%v", req.Form)
			}
			return jsonResponse(`{"errorCode":0,"data":{"yxtGame":"wdhysj","yxtChannel":"myxyx","yxtSubChannel":"myxyx","yxtUserId":"u1","yxtLoginTime":100,"yxtSign":"sig","yxtChannelUserId":"cu1"}}`), nil
		default:
			t.Fatalf("unexpected host %s", req.URL.Host)
			return nil, nil
		}
	})}

	gameAppID, err := client.queryGameAppID(context.Background(), grant)
	if err != nil {
		t.Fatal(err)
	}
	if gameAppID != "2021000000000001" {
		t.Fatalf("gameAppID=%q", gameAppID)
	}
	authCode, err := client.queryGameAuthCode(context.Background(), grant, gameAppID)
	if err != nil {
		t.Fatal(err)
	}
	yxt, err := client.exchangeYXT(context.Background(), authCode)
	if err != nil {
		t.Fatal(err)
	}
	gameHTTP := NewHTTPClient(cfg, "device", "uuid", "session")
	payload, err := client.gameLoginPayload(gameHTTP, yxt)
	if err != nil {
		t.Fatal(err)
	}
	if payload["yxtSign"] != "sig" || payload["yxtChannelUserId"] != "cu1" {
		t.Fatalf("payload=%v", payload)
	}
	if payload["yxtLoginTime"] != json.Number("100") {
		t.Fatalf("yxtLoginTime=%#v, want preserved JSON number", payload["yxtLoginTime"])
	}
	if _, present := payload["yxtSubChannel"]; present {
		t.Fatalf("official game/login payload must not contain yxtSubChannel: %v", payload)
	}
	if !gameLoginSucceeded(float64(1)) || !gameLoginSucceeded("success") || gameLoginSucceeded(float64(0)) {
		t.Fatal("game login status parsing mismatch")
	}
}
