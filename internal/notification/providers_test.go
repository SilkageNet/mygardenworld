package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestProviderEndpoints(t *testing.T) {
	for _, tc := range []struct {
		provider, url string
		valid         bool
	}{
		{"custom", "https://example.com/hook?token=example", true},
		{"wecom", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=example", true},
		{"dingtalk", "https://oapi.dingtalk.com/robot/send?access_token=example", true},
		{"feishu", "https://open.feishu.cn/open-apis/bot/v2/hook/example", true},
		{"wecom", "https://qyapi.weixin.qq.com.attacker.example/cgi-bin/webhook/send?key=example", false},
		{"wecom", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=", false},
		{"dingtalk", "https://oapi.dingtalk.com/robot/send?access_token=example&timestamp=1&sign=old", false},
		{"dingtalk", "https://oapi.dingtalk.com/robot/send?access_token=one&access_token=two", false},
		{"dingtalk", "https://oapi.dingtalk.com:8080/robot/send?access_token=example", false},
		{"feishu", "https://open.feishu.cn/open-apis/bot/v2/hook/", false},
		{"feishu", "https://open.feishu.cn/open-apis/bot/v2/hook/one/two", false},
		{"feishu", "https://open.feishu.cn/open-apis/bot/v2/hook/token?sign=old", false},
		{"wecom", "https://example.com/hook", false},
		{"other", "https://example.com/hook", false},
		{"custom", "http://example.com/hook", false},
	} {
		t.Run(tc.provider+"/"+tc.url, func(t *testing.T) {
			if err := ValidateProviderEndpoint(tc.provider, tc.url); (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}

func TestProviderRequestsAndFreshSignatures(t *testing.T) {
	now := time.Unix(1700000000, 123000000).UTC()
	for _, target := range []store.NotificationTarget{
		{Provider: "custom", Endpoint: "https://example.com/hook"},
		{Provider: "wecom", Endpoint: "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=example"},
		{Provider: "dingtalk", Endpoint: "https://oapi.dingtalk.com/robot/send?access_token=example", SigningSecret: "test-secret"},
		{Provider: "feishu", Endpoint: "https://open.feishu.cn/open-apis/bot/v2/hook/example", SigningSecret: "test-secret"},
	} {
		t.Run(target.Provider, func(t *testing.T) {
			r, err := providerRequest(context.Background(), target, CustomPayloadExample(), now)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(r.Body)
			if r.Method != http.MethodPost || strings.Contains(string(body), "test-secret") {
				t.Fatal("invalid request")
			}
			if target.Provider == "custom" {
				if string(body) != CustomPayloadExample() {
					t.Fatal("custom payload changed")
				}
				return
			}
			var msg map[string]json.RawMessage
			if err := json.Unmarshal(body, &msg); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), "小云朵") || !strings.Contains(string(body), "示例账号") || !strings.Contains(string(body), "请求保护") {
				t.Fatal("missing curated message")
			}
			var content map[string]string
			if target.Provider == "feishu" {
				_ = json.Unmarshal(msg["content"], &content)
				if string(msg["msg_type"]) != `"text"` || content["text"] == "" {
					t.Fatal("invalid Feishu format")
				}
				if string(msg["timestamp"]) != `"1700000000"` || string(msg["sign"]) != `"mbm4Y4oluIPQ00qlBIhX8vAZ0EKv3nw0LuTb91jPL84="` {
					t.Fatalf("Feishu signature %s", body)
				}
			} else {
				_ = json.Unmarshal(msg["text"], &content)
				if string(msg["msgtype"]) != `"text"` || content["content"] == "" {
					t.Fatal("invalid text format")
				}
			}
			if target.Provider == "dingtalk" {
				if r.URL.Query().Get("timestamp") != "1700000000123" || r.URL.Query().Get("sign") != "emvuTAxMTWOXmz4z3p3ifyzxqMNeLEBESaq9BQR0d6w=" {
					t.Fatalf("DingTalk signature %s", r.URL.RawQuery)
				}
				if string(msg["at"]) != `{"isAtAll":false}` {
					t.Fatal("unexpected mention")
				}
			}
			later, err := providerRequest(context.Background(), target, CustomPayloadExample(), now.Add(time.Minute))
			if err != nil {
				t.Fatal(err)
			}
			laterBody, _ := io.ReadAll(later.Body)
			if target.SigningSecret != "" && later.URL.String() == r.URL.String() && bytes.Equal(laterBody, body) {
				t.Fatal("signature reused across retries")
			}
			target.SigningSecret = ""
			unsigned, err := providerRequest(context.Background(), target, CustomPayloadExample(), now)
			if err != nil {
				t.Fatal(err)
			}
			unsignedBody, _ := io.ReadAll(unsigned.Body)
			if strings.Contains(string(unsignedBody), `"sign"`) || unsigned.URL.Query().Has("sign") {
				t.Fatal("unsigned request has signature")
			}
		})
	}
}

func TestProviderAcknowledgements(t *testing.T) {
	for _, tc := range []struct{ provider, body, status string }{
		{"custom", "", "sent"}, {"custom", `{"code":1}`, "sent"},
		{"wecom", `{"errcode":0,"errmsg":"ok"}`, "sent"},
		{"dingtalk", `{"errcode":0,"errmsg":"ok"}`, "sent"},
		{"feishu", `{"code":0,"msg":"success"}`, "sent"},
		{"dingtalk", `{"errcode":410100,"errmsg":"SECRET"}`, "pending"},
		{"dingtalk", `{"errcode":-1}`, "pending"},
		{"wecom", `{"errcode":45009}`, "pending"},
		{"feishu", `{"code":11232,"msg":"SECRET"}`, "pending"},
		{"wecom", `{"errcode":40058,"errmsg":"SECRET"}`, "failed"},
		{"dingtalk", `{"errcode":310000,"errmsg":"SECRET"}`, "failed"},
		{"feishu", `{"code":19021,"msg":"SECRET"}`, "failed"},
		{"wecom", `{"code":0}`, "failed"}, {"feishu", `{"errcode":0}`, "failed"},
		{"dingtalk", `{"errcode":null}`, "failed"}, {"wecom", `{}`, "failed"},
		{"feishu", `{"code":"0"}`, "failed"}, {"wecom", `<html>SECRET</html>`, "failed"},
		{"feishu", `{"code":0} garbage`, "failed"},
	} {
		t.Run(tc.provider+"/"+tc.body, func(t *testing.T) {
			status, safeError := providerAcknowledgement(tc.provider, []byte(tc.body))
			if status != tc.status || strings.Contains(safeError, "SECRET") {
				t.Fatalf("status=%s error=%s", status, safeError)
			}
		})
	}
}

func TestRobotTextIsBoundedAndRecoveryReadable(t *testing.T) {
	for _, tc := range []struct {
		kind      string
		recovered bool
		label     string
	}{{"session", false, "异常提醒"}, {"session", true, "已恢复"}, {"test", false, "测试通知"}} {
		p := store.NotificationPayload{Kind: tc.kind, Recovered: tc.recovered, AccountID: 1, AccountName: "<@all>\n" + strings.Repeat("长", 2000), Message: strings.Repeat("长", 2000), TS: time.Now(), Occurrences: 5, DurationSeconds: 60}
		text := robotText(p)
		if len(text) > 2048 || !utf8.ValidString(text) || strings.Contains(text, "<@all>") || !strings.Contains(text, tc.label) {
			t.Fatal("unsafe robot text")
		}
	}
}

func TestCustomExampleUsesExactPayloadContract(t *testing.T) {
	var example store.NotificationPayload
	decoder := json.NewDecoder(strings.NewReader(CustomPayloadExample()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&example); err != nil {
		t.Fatal(err)
	}
	if example.AccountID != 123 || example.ID == "" || example.Recovered || example.Occurrences != 1 || example.DurationSeconds != 0 || example.TS.IsZero() {
		t.Fatal("invalid example")
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(CustomPayloadExample()), &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 10 {
		t.Fatalf("contract changed: %v", fields)
	}
}
