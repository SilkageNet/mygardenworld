package babigame

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
)

// GameLoginError contains only numeric diagnostics, never the upstream URL,
// UUID, token, content or arbitrary server message. Code is a JSON business
// field, not an HTTP status. Unknown codes remain unknown.
type GameLoginError struct {
	BizCode   int
	Code      int
	MDErrCode int
}

func gameLoginError(resp map[string]any) *GameLoginError {
	e := &GameLoginError{BizCode: loginDiagnosticCode(resp["bizCode"]), Code: loginDiagnosticCode(resp["code"])}
	if raw, ok := resp["url"].(string); ok {
		if u, err := url.Parse(raw); err == nil {
			e.MDErrCode = loginDiagnosticCode(u.Query().Get("mdErrCode"))
		}
	}
	return e
}

func loginDiagnosticCode(value any) int {
	code, err := strconv.Atoi(fmt.Sprint(value))
	if err != nil {
		return 0
	}
	return code
}

func (e *GameLoginError) Error() string {
	reason := "游戏服务器拒绝登录，原因尚未识别"
	switch e.BizCode {
	case 902049:
		reason = "游戏服务器判定当前 IP 存在登录风险"
	case 902050:
		reason = "游戏服务器已封禁当前 IP"
	case 902051:
		reason = "游戏服务器限制了当前 IP 的请求频率"
	case 902052:
		reason = "游戏服务器触发限流"
	case 902053:
		reason = "游戏服务器提示 SDK 账号被封禁"
	}
	message := fmt.Sprintf("%s（bizCode=%d, code=%d, mdErrCode=%d）", reason, e.BizCode, e.Code, e.MDErrCode)
	if IsLoginRiskError(e) {
		message += "。请勿反复尝试登录；避免在同一服务器上集中登录过多账号，以降低触发游戏服务自身风控的风险。具体阈值及解除时间由游戏服务决定"
	}
	return message
}

// IsLoginRiskError identifies the explicit SDK risk/ban/rate-limit codes
// observed in tmp/mini/176/src/assets/scripts/game.js (modosdk LOGIN).
// These refusals must not enter an automatic network reconnect loop.
func IsLoginRiskError(err error) bool {
	var e *GameLoginError
	if !errors.As(err, &e) || e == nil {
		return false
	}
	switch e.BizCode {
	case 902049, 902050, 902051, 902052, 902053:
		return true
	default:
		return false
	}
}

// IsLoginRefusalError includes unknown business refusals. Unknown does not
// mean transient, and must not cause automatic repeated authentication. This
// deliberately does not label unknown codes (including 902054) as IP risk.
func IsLoginRefusalError(err error) bool {
	var rejected *GameLoginError
	return errors.As(err, &rejected) && rejected != nil
}
