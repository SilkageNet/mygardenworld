package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/SilkageNet/mygardenworld/internal/outbound"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

// ValidateProviderEndpoint rejects cross-provider credentials and URLs copied
// with an expiring signature. DNS/IP checks still run at connection time.
func ValidateProviderEndpoint(provider, endpoint string) error {
	if err := outbound.ValidateEndpoint(endpoint); err != nil {
		return err
	}
	u, _ := url.Parse(endpoint)
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return errors.New("接收地址查询参数无效")
	}
	if provider == "custom" {
		return nil
	}
	if u.Port() != "" && u.Port() != "443" {
		return errors.New("机器人地址必须使用 HTTPS 默认端口")
	}
	valid := false
	switch provider {
	case "wecom":
		valid = u.Hostname() == "qyapi.weixin.qq.com" && u.Path == "/cgi-bin/webhook/send" && len(q) == 1 && len(q["key"]) == 1 && q.Get("key") != ""
	case "dingtalk":
		valid = u.Hostname() == "oapi.dingtalk.com" && u.Path == "/robot/send" && len(q) == 1 && len(q["access_token"]) == 1 && q.Get("access_token") != ""
	case "feishu":
		token, found := strings.CutPrefix(u.Path, "/open-apis/bot/v2/hook/")
		valid = u.Hostname() == "open.feishu.cn" && found && token != "" && !strings.Contains(token, "/") && len(q) == 0
	}
	if !valid {
		return errors.New("地址与所选渠道不匹配，请复制机器人的原始 Webhook 地址；加签密钥单独填写")
	}
	return nil
}

// providerRequest renders only curated facts. The durable outbox keeps the
// generic payload; time-sensitive signatures are regenerated on every attempt.
// Provider contracts:
// https://developer.work.weixin.qq.com/document/path/91770
// https://open.dingtalk.com/document/robots/customize-robot-security-settings
// https://open.dingtalk.com/document/orgapp/custom-robots-send-group-messages
// https://open.feishu.cn/document/client-docs/bot-v3/add-custom-bot
func providerRequest(ctx context.Context, target store.NotificationTarget, payload string, now time.Time) (*http.Request, error) {
	if err := ValidateProviderEndpoint(target.Provider, target.Endpoint); err != nil {
		return nil, err
	}
	endpoint, body := target.Endpoint, payload
	if target.Provider != "custom" {
		var p store.NotificationPayload
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return nil, errors.New("通知内容无效")
		}
		content := robotText(p)
		var message map[string]any
		switch target.Provider {
		case "wecom", "dingtalk":
			message = map[string]any{"msgtype": "text", "text": map[string]string{"content": content}}
			if target.Provider == "dingtalk" {
				message["at"] = map[string]bool{"isAtAll": false}
				if target.SigningSecret != "" {
					timestamp := strconv.FormatInt(now.UnixMilli(), 10)
					u, _ := url.Parse(endpoint)
					q := u.Query()
					q.Set("timestamp", timestamp)
					q.Set("sign", signature(target.SigningSecret, timestamp+"\n"+target.SigningSecret))
					u.RawQuery = q.Encode()
					endpoint = u.String()
				}
			}
		case "feishu":
			message = map[string]any{"msg_type": "text", "content": map[string]string{"text": content}}
			if target.SigningSecret != "" {
				timestamp := strconv.FormatInt(now.Unix(), 10)
				message["timestamp"] = timestamp
				message["sign"] = signature(timestamp+"\n"+target.SigningSecret, "")
			}
		}
		encoded, err := json.Marshal(message)
		if err != nil {
			return nil, errors.New("无法生成机器人消息")
		}
		body = string(encoded)
	}
	return http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
}

func signature(key, message string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func robotText(p store.NotificationPayload) string {
	state := "异常提醒"
	if p.Recovered {
		state = "已恢复"
	} else if p.Kind == "test" {
		state = "测试通知"
	}
	text := "小云朵 · " + state
	if p.AccountID != 0 {
		text += "\n账号：" + safeText(p.AccountName, 240)
	}
	text += "\n" + safeText(p.Message, 1000)
	text += "\n时间：" + p.TS.Format(time.RFC3339)
	if p.Kind != "test" {
		text += fmt.Sprintf("\n累计 %d 次 · 持续 %d 秒", p.Occurrences, p.DurationSeconds)
	}
	return text
}

// Keep text below the smallest provider limit (WeCom: 2048 UTF-8 bytes),
// prevent account names from injecting lines or mention markup.
func safeText(text string, limit int) string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		if r == '<' {
			return '＜'
		}
		return r
	}, text)
	if len(text) <= limit {
		return text
	}
	cut := limit - len("…")
	for !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "…"
}

// A 2xx response is insufficient for native robots. Missing/malformed codes
// fail closed. Never persist upstream error messages which may echo secrets.
func providerAcknowledgement(provider string, body []byte) (string, string) {
	if provider == "custom" {
		return "sent", ""
	}
	var ack struct {
		ErrCode *int64 `json:"errcode"`
		Code    *int64 `json:"code"`
	}
	if err := json.Unmarshal(body, &ack); err != nil {
		return "failed", "平台响应无法识别，请检查机器人地址"
	}
	code := ack.ErrCode
	if provider == "feishu" {
		code = ack.Code
	}
	if code == nil {
		return "failed", "平台响应缺少结果码，未确认接收"
	}
	if *code == 0 {
		return "sent", ""
	}
	if (provider == "dingtalk" && (*code == -1 || *code == 410100)) || (provider == "wecom" && (*code == -1 || *code == 45009)) || (provider == "feishu" && *code == 11232) {
		return "pending", fmt.Sprintf("平台繁忙或限流（错误码 %d），稍后重试", *code)
	}
	return "failed", fmt.Sprintf("平台拒绝消息（错误码 %d），请检查机器人密钥、安全设置和发送限制", *code)
}

// CustomPayloadExample is synthetic and shares the actual serialization type.
func CustomPayloadExample() string {
	encoded, _ := json.MarshalIndent(store.NotificationPayload{
		ID: "example-notification-id", Kind: "account_request", Level: "error",
		Message: "账号进入请求保护，游戏请求已暂停，请查看控制台", AccountID: 123, AccountName: "示例账号",
		TS: time.Date(2026, 9, 7, 6, 0, 0, 0, time.UTC), Occurrences: 1,
	}, "", "  ")
	return string(encoded)
}
