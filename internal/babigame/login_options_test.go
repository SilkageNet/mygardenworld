package babigame

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

const testLoginPackage = `{"status":"success","url":"https://cdn.test/index.html?uuid=launch-uuid&mdgid=160&mdclid=459&packageId=494&env=prod","data":{"gameConfig":{"gameVersion":"450.0.15"},"entryConfig":{"path":"index.json","cdnList":["https://cdn.test/"]}}}`

func TestFreshLoginUsesLaunchIdentityAndSDKSession(t *testing.T) {
	for _, tc := range []struct {
		name, params, wantSession string
		native, requestToken      bool
	}{
		{"password local session", `{"session1Cipher":"1"}`, "", false, false},
		{"password server session", `{"session1Cipher":"1","mdSession1":"opaque-server-session"}`, "opaque-server-session", false, false},
		{"native assertion", `{"session1Cipher":true,"mdSession1":"opaque-server-session"}`, "opaque-server-session", true, false},
		{"disabled server session", `{"session1Cipher":0,"mdSession1":"unused-secret"}`, "", false, false},
		{"request token enabled", `{"getRequestToken":"1"}`, "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := ConfigForChannel(ChannelIOS)
			c := NewHTTPClient(cfg, "device", "local-uuid", "session0")
			var paths []string
			c.HTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				paths = append(paths, req.URL.Path)
				var body map[string]any
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				switch req.URL.Path {
				case "/pack/queryPackageConfig":
					return jsonResponse(testLoginPackage), nil
				case c.gamePath("queryInitParams"):
					if body["uuid"] != "launch-uuid" {
						t.Fatal("init used an unregistered UUID")
					}
					return jsonResponse(`{"status":"success","code":302,"data":` + tc.params + `}`), nil
				case "/game/getRequestToken":
					if !tc.requestToken || req.URL.Host != "apirq.babigame.cn" || body["mdcl"] != float64(459) {
						t.Fatal("wrong request-token preflight")
					}
					return jsonResponse(`{"status":1}`), nil
				case "/account/v3/token/verify":
					return jsonResponse(`{"status":"fail"}`), nil
				case "/account/login/username/v2":
					return jsonResponse(`{"data":{"content":"signed-content","timestamp":123,"signature":"signature","session1":"account-sdk-session"}}`), nil
				case c.gamePath("login"):
					if body["uuid"] != "launch-uuid" || body["content"] != "signed-content" || body["signature"] != "signature" || body["timestamp"] != float64(123) {
						t.Fatal("launch identity or signed assertion was lost")
					}
					session, _ := body["session1"].(string)
					if tc.wantSession != "" {
						if session != tc.wantSession {
							t.Fatal("server session was not forwarded intact")
						}
					} else if !regexp.MustCompile(`^s1[A-Za-z0-9]{17}[0-9]{13}$`).MatchString(session) {
						t.Fatal("game session does not use current SDK format")
					}
					var appInfo map[string]string
					if err := json.Unmarshal([]byte(body["appInfo"].(string)), &appInfo); err != nil {
						t.Fatal(err)
					}
					if appInfo["_game_version"] != "450.0.15" || appInfo["_package_name"] != cfg.PackageName {
						t.Fatal("login did not use package metadata")
					}
					return jsonResponse(`{"status":"fail","code":302,"bizCode":902054,"url":"https://secret.test/?token=private"}`), nil
				default:
					t.Fatalf("unexpected request %s", req.URL.Path)
					return nil, nil
				}
			})
			// This is the runner's metadata read before attempting cached restore.
			if _, err := c.QueryPackageConfig(context.Background()); err != nil {
				t.Fatal(err)
			}
			if c.UUID != "local-uuid" {
				t.Fatal("metadata lookup changed an existing session identity")
			}
			var err error
			if tc.native {
				_, err = PerformLoginWithNative(context.Background(), c, NativeLogin{Content: "signed-content", Signature: "signature", Timestamp: 123, Session1: "account-sdk-session"}, 1)
			} else {
				_, err = PerformLoginWithPassword(context.Background(), c, "account", "password", 1)
			}
			var refusal *GameLoginError
			if !errors.As(err, &refusal) || refusal.BizCode != 902054 || strings.Contains(err.Error(), "private") {
				t.Fatalf("unexpected error %v", err)
			}
			want := []string{"/pack/queryPackageConfig", c.gamePath("queryInitParams")}
			if tc.requestToken {
				want = append(want, "/game/getRequestToken")
			}
			if !tc.native {
				want = append(want, "/account/v3/token/verify", "/account/login/username/v2")
			}
			want = append(want, c.gamePath("login"))
			if !reflect.DeepEqual(paths, want) {
				t.Fatalf("requests=%v want %v", paths, want)
			}
		})
	}
}

func TestLoginInitializationFailsBeforeCredentials(t *testing.T) {
	for _, tc := range []struct{ name, pkg, init string }{
		{"package refusal", `{"status":"fail","msg":"private"}`, ""},
		{"missing launch UUID", `{"status":"success","data":{"gameConfig":{"gameVersion":"450.0.15"}},"url":"https://cdn.test/?token=private"}`, ""},
		{"wrong channel", strings.Replace(testLoginPackage, "mdclid=459", "mdclid=538", 1), ""},
		{"ambiguous UUID", strings.Replace(testLoginPackage, "uuid=launch-uuid", "uuid=launch-uuid&uuid=private", 1), ""},
		{"invalid init UUID", testLoginPackage, `{"status":"fail","code":302,"msg":"private"}`},
		{"missing init data", testLoginPackage, `{"status":"success"}`},
		{"wrong init scope", testLoginPackage, `{"status":"success","data":{"env":"aud-zfb","mdSession1":"private"}}`},
		{"wrong init UUID", testLoginPackage, `{"status":"success","data":{"uuid":"private"}}`},
		{"preflight refusal", testLoginPackage, `{"status":"success","data":{"getRequestToken":1}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := ConfigForChannel(ChannelIOS)
			c := NewHTTPClient(cfg, "", "", "")
			c.HTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.Path {
				case "/pack/queryPackageConfig":
					return jsonResponse(tc.pkg), nil
				case c.gamePath("queryInitParams"):
					return jsonResponse(tc.init), nil
				case "/game/getRequestToken":
					return jsonResponse(`{"status":0,"code":302,"msg":"private"}`), nil
				default:
					t.Fatalf("sent credentials after failed initialization: %s", req.URL.Path)
					return nil, nil
				}
			})
			_, err := PerformLoginWithPassword(context.Background(), c, "account", "password", 1)
			if err == nil || strings.Contains(err.Error(), "private") {
				t.Fatalf("expected redacted initialization failure, got %v", err)
			}
		})
	}
}

func TestLoginSessionOptionsPreferInitResponseThenLaunchURL(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params map[string]any
		want   string
	}{
		{"launch fallback", nil, "launch-session"},
		{"init precedence", map[string]any{"mdSession1": "init-session"}, "init-session"},
		{"disabled", map[string]any{"session1Cipher": false}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, _ := ConfigForChannel(ChannelIOS)
			c := NewHTTPClient(cfg, "", "", "")
			if err := c.acceptLaunchURL("https://cdn.test/?uuid=launch-uuid&session1Cipher=1&mdSession1=launch-session"); err != nil {
				t.Fatal(err)
			}
			c.loginParams = tc.params
			body := c.withGameLoginOptions(nil)
			if tc.want != "" && body["session1"] != tc.want || tc.want == "" && !strings.HasPrefix(body["session1"].(string), "s1") {
				t.Fatal("incorrect SDK session source")
			}
		})
	}
}

func TestGameLoginOptionsAreSharedAndPreserveSignedPayload(t *testing.T) {
	for _, channel := range SupportedChannels() {
		t.Run(string(channel), func(t *testing.T) {
			cfg, _ := ConfigForChannel(channel)
			c := NewHTTPClient(cfg, "device", "uuid", "session0")
			payload := map[string]any{"yxtLoginTime": json.Number("123"), "yxtSign": "signature", "content": "signed-content"}
			c.HTTPClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
				var body map[string]any
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if req.URL.Host != cfg.HostAPI || req.URL.Path != c.gamePath("login") || body["uuid"] != "uuid" || body["lang"] != "zh" || body["yxtSign"] != "signature" || body["yxtLoginTime"] != float64(123) {
					t.Fatal("common filter changed signed fields or channel")
				}
				if !strings.HasPrefix(body["session1"].(string), "s1") || body["appInfo"] == "" {
					t.Fatal("common SDK login fields missing")
				}
				return jsonResponse(`{"status":"success","url":"https://cdn.test/?token=token&open_id=player","data":{"content":"opaque"}}`), nil
			})
			if _, err := c.GameLoginWithPayload(context.Background(), payload); err != nil {
				t.Fatal(err)
			}
			if len(payload) != 3 || payload["yxtLoginTime"] != json.Number("123") {
				t.Fatal("caller payload was mutated")
			}
			first := c.gameSession1
			c.withGameLoginOptions(payload)
			if c.gameSession1 != first {
				t.Fatal("local session rotated within one login client")
			}
		})
	}
}

func TestLoginRefusalIsDistinctFromRisk(t *testing.T) {
	for _, code := range []int{902049, 902054, 123456} {
		err := fmt.Errorf("wrapped: %w", &GameLoginError{BizCode: code})
		if !IsLoginRefusalError(err) || IsLoginRiskError(err) != (code == 902049) {
			t.Fatalf("incorrect refusal/risk classification for %d", code)
		}
	}
	for _, err := range []error{nil, context.DeadlineExceeded, errors.New("902054"), (*GameLoginError)(nil)} {
		if IsLoginRefusalError(err) {
			t.Fatalf("ordinary error is not a business refusal: %v", err)
		}
	}
}
