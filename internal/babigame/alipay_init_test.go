package babigame

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func alipayInitFixture() map[string]any {
	return map[string]any{
		"status": "success",
		"url":    "https://hygncdn.babigame.cn/index-gn-zfb-2.2.209.html?uuid=launch-uuid&env=prod&mdgid=163&mdclid=538&packageId=520&userParams=1",
		"userParams": map[string]any{
			"uuid": "launch-uuid", "env": "prod", "mdgid": "163", "mdclid": "538", "packageId": "520",
			"gameVersion": "2.2.209", "session1Cipher": "1", "userParams": "1",
			"yxtGame": "wdhysj", "yxtChannel": "myxyx", "yxtSubChannel": "myxyx",
		},
	}
}

func TestAlipayInitialization(t *testing.T) {
	for _, tc := range []struct {
		name        string
		change      func(map[string]any)
		wantErr     string
		wantSession string
	}{
		{name: "current production"},
		{name: "server session", change: func(r map[string]any) { mapOf(r["userParams"])["mdSession1"] = "server-session" }, wantSession: "server-session"},
		{name: "audit route", change: func(r map[string]any) { r["url"] = strings.ReplaceAll(stringOf(r["url"]), "env=prod", "env=aud-zfb") }, wantErr: "env"},
		{name: "cross channel options", change: func(r map[string]any) { mapOf(r["userParams"])["mdclid"] = "459" }, wantErr: "mdclid"},
		{name: "conflicting UUID", change: func(r map[string]any) { mapOf(r["userParams"])["uuid"] = "other" }, wantErr: "UUID"},
		{name: "missing launch UUID", change: func(r map[string]any) { r["url"] = strings.ReplaceAll(stringOf(r["url"]), "uuid=launch-uuid&", "") }, wantErr: "UUID"},
		{name: "missing YXT config", change: func(r map[string]any) { delete(mapOf(r["userParams"]), "yxtGame") }, wantErr: "yxtGame"},
		{name: "rejected init", change: func(r map[string]any) { r["status"] = "error" }, wantErr: "rejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := ConfigForChannel(ChannelAlipay)
			c := NewHTTPClient(cfg, "device", "restored-uuid", "session0")
			c.gameSession1 = "previous-session"
			calls := 0
			c.HTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.URL.Path != "/pack/init/packageName/cn.hysj.zfb.minigame" {
					t.Fatalf("path=%s", req.URL.Path)
				}
				var body map[string]any
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["version"] != "450.0.15" || body["userParams"] != float64(1) || body["scene"] != "1000" {
					t.Fatalf("init body=%v", body)
				}
				if body["system_info"] == nil || body["open_data"] == nil || body["uuid"] != nil || body["yxtSign"] != nil {
					t.Fatalf("unexpected init body=%v", body)
				}
				fixture := alipayInitFixture()
				if tc.change != nil {
					tc.change(fixture)
				}
				raw, _ := json.Marshal(fixture)
				return jsonResponse(string(raw)), nil
			})
			err := c.prepareAlipayLogin(context.Background())
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err=%v, want %s", err, tc.wantErr)
				}
				if c.UUID != "restored-uuid" {
					t.Fatal("failed initialization replaced UUID")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || c.UUID != "launch-uuid" {
				t.Fatalf("calls=%d UUID=%q", calls, c.UUID)
			}
			if c.Cfg.ClientVersion != "450.0.15" || c.Cfg.GameVersion != "450.0.15" {
				t.Fatal("SDK entry version replaced executable version")
			}
			body := c.withGameLoginOptions(map[string]any{"yxtSign": "signed-unchanged"})
			session := stringOf(body["session1"])
			if tc.wantSession != "" {
				if session != tc.wantSession {
					t.Fatalf("session=%q", session)
				}
			} else if len(session) != 32 || !strings.HasPrefix(session, "s1") {
				t.Fatalf("session=%q", session)
			}
			if body["uuid"] != "launch-uuid" || body["yxtSign"] != "signed-unchanged" {
				t.Fatalf("login body=%v", body)
			}
			var appInfo map[string]any
			if err := json.Unmarshal([]byte(stringOf(body["appInfo"])), &appInfo); err != nil {
				t.Fatal(err)
			}
			if appInfo["_game_version"] != "450.0.15" {
				t.Fatalf("appInfo=%v", appInfo)
			}
		})
	}
}

func TestAlipayLoginInitializesBeforeChannelAuthorization(t *testing.T) {
	for _, rejectPreflight := range []bool{false, true} {
		t.Run(map[bool]string{false: "login rejected once", true: "preflight rejected before auth"}[rejectPreflight], func(t *testing.T) {
			cfg, _ := ConfigForChannel(ChannelAlipay)
			game := NewHTTPClient(cfg, "device", "random-uuid", "session0")
			provider := NewAlipayClient(cfg)
			var paths []string
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				paths = append(paths, req.URL.Path)
				switch {
				case strings.HasPrefix(req.URL.Path, "/pack/init/"):
					fixture := alipayInitFixture()
					mapOf(fixture["userParams"])["getRequestToken"] = "1"
					raw, _ := json.Marshal(fixture)
					return jsonResponse(string(raw)), nil
				case req.URL.Path == "/game/getRequestToken":
					if req.URL.Host != "apizfbfastrq.babigame.cn" {
						t.Fatal(req.URL.Host)
					}
					if rejectPreflight {
						return jsonResponse(`{"status":0,"code":302}`), nil
					}
					return jsonResponse(`{"status":1}`), nil
				case strings.Contains(req.URL.Path, "/queryPcGameInfo/"):
					return jsonResponse(`{"success":true,"data":{"appId":"2021004163668677"}}`), nil
				case strings.Contains(req.URL.Path, "/queryPcGameAuthInfo/"):
					return jsonResponse(`{"success":true,"data":{"authCode":"auth-code"}}`), nil
				case req.URL.Path == "/Channel/login/yxtGame/wdhysj/yxtChannel/myxyx/yxtSubChannel/myxyx":
					return jsonResponse(`{"errorCode":0,"data":{"yxtUserId":"u1","yxtLoginTime":100,"yxtSign":"sig","yxtChannelUserId":"cu1"}}`), nil
				case req.URL.Path == "/game/login/mdcl/c538/mdgid/163/env/prod":
					var body map[string]any
					if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if body["uuid"] != "launch-uuid" || body["yxtLoginTime"] != float64(100) || body["yxtSign"] != "sig" || body["yxtGame"] != "wdhysj" {
						t.Fatalf("body=%v", body)
					}
					return jsonResponse(`{"status":0,"code":302,"bizCode":902054}`), nil
				default:
					t.Fatalf("unexpected request %s", req.URL)
					return nil, nil
				}
			})
			game.HTTPClient.Transport, provider.HTTPClient.Transport = transport, transport
			_, err := provider.LoginWithWebGrant(context.Background(), game, AlipayWebGrant{Token: "web-token", UserID: "1000000000001234"})
			var refusal *GameLoginError
			if !errors.As(err, &refusal) {
				t.Fatalf("err=%v", err)
			}
			wantCalls, wantCode := 6, 902054
			if rejectPreflight {
				wantCalls, wantCode = 2, 902048
			}
			if len(paths) != wantCalls || refusal.BizCode != wantCode {
				t.Fatalf("requests=%v err=%v", paths, err)
			}
		})
	}
}
