package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientrpc"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// RedeemResult is the sanitized outcome of one redeem.useCode call.
type RedeemResult struct {
	Code    string
	Items   []RedeemItemGain
	MailNew int
}

// RedeemItemGain is one observed gain after a successful redeem.
type RedeemItemGain struct {
	ItemID int32  `json:"item_id"`
	Name   string `json:"name"`
	Count  int32  `json:"count"`
}

// RedeemCode calls gs.redeem.useCode on the live session.
// The account must already be connected.
func (r *Runner) RedeemCode(ctx context.Context, code string) (RedeemResult, error) {
	code = strings.TrimSpace(code)
	out := RedeemResult{Code: code}
	if code == "" {
		return out, fmt.Errorf("empty redeem code")
	}
	r.operationMu.Lock()
	defer r.operationMu.Unlock()

	r.mu.RLock()
	client := r.client
	session := r.session
	invalidated := r.sessionInvalidated
	r.mu.RUnlock()
	if client == nil || client.Closed() || invalidated || session == nil {
		return out, fmt.Errorf("account not connected")
	}

	beforeInv := r.state.Inventory()
	beforeGold := r.state.Gold()
	beforeDiamonds := r.state.SpendableDiamonds()
	beforeMails := mailKeySet(r.state.Mails())

	rawRPC := babigame.NewRPCClient(
		client,
		session,
		babigame.WithDefaultTimeout(30*time.Second),
		babigame.WithApplyV(r.state.ApplyV),
	)
	rpc := clientrpc.NewClient(rawRPC)
	resp, err := rpc.Redeem().UseCode(ctx, clientproto.RedeemUseCodeRequest{Code: code})
	if err != nil {
		msg := formatRedeemServerError(err, babigame.WSResponseD{})
		r.emitRedeemEvent(false, code, nil, 0, msg)
		return out, fmt.Errorf("%s", msg)
	}
	if resp.Envelope.IsError() {
		msg := formatRedeemServerError(nil, resp.Envelope)
		r.emitRedeemEvent(false, code, nil, 0, msg)
		return out, fmt.Errorf("%s", msg)
	}

	out.Items = redeemGains(beforeInv, r.state.Inventory(), beforeGold, r.state.Gold(), beforeDiamonds, r.state.SpendableDiamonds())
	out.MailNew = countNewRewardMails(beforeMails, r.state.Mails())
	r.emitRedeemEvent(true, code, out.Items, out.MailNew, "")
	return out, nil
}

func (r *Runner) emitRedeemEvent(ok bool, code string, items []RedeemItemGain, mailNew int, errMsg string) {
	payload := map[string]any{
		"code":     code,
		"items":    items,
		"mail_new": mailNew,
	}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	raw, _ := json.Marshal(payload)

	if !ok {
		r.emit(Event{
			Kind:        "redeem_code",
			Category:    automation.CategorySystem,
			Domain:      "redeem.code",
			Action:      "failed",
			Label:       "兑换码",
			Level:       "error",
			Message:     redeemFailureMessage(code, errMsg),
			PayloadJSON: string(raw),
		})
		return
	}
	r.emit(Event{
		Kind:        "redeem_code",
		Category:    automation.CategorySystem,
		Domain:      "redeem.code",
		Action:      "use",
		Label:       "兑换码",
		Message:     redeemSuccessMessage(code, items, mailNew),
		PayloadJSON: string(raw),
	})
}

func redeemFailureMessage(code, errMsg string) string {
	if code == "" {
		return fmt.Sprintf("兑换失败：%s", errMsg)
	}
	return fmt.Sprintf("兑换失败 [%s]：%s", code, errMsg)
}

func formatRedeemServerError(err error, envelope babigame.WSResponseD) string {
	if text := redeemMsgCodeText(envelope.ErrorCode()); text != "" {
		return text
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil {
		if text := redeemMsgCodeText(rpcErr.Envelope.ErrorCode()); text != "" {
			return text
		}
		if text := redeemMsgCodeTextFromRaw(rpcErr.Envelope.ErrorMsg()); text != "" {
			return text
		}
		if text := redeemMsgCodeTextFromRaw(string(rpcErr.Envelope.M)); text != "" {
			return text
		}
	}
	if text := redeemMsgCodeTextFromRaw(envelope.ErrorMsg()); text != "" {
		return text
	}
	if text := redeemMsgCodeTextFromRaw(string(envelope.M)); text != "" {
		return text
	}
	if err != nil {
		if text := redeemMsgCodeTextFromRaw(err.Error()); text != "" {
			return text
		}
		msg := strings.TrimSpace(err.Error())
		if msg != "" {
			return msg
		}
	}
	msg := strings.TrimSpace(envelope.ErrorMsg())
	if msg != "" {
		return msg
	}
	return "server returned error"
}

func redeemMsgCodeText(code int) string {
	if code == 0 {
		return ""
	}
	return state.MsgCodeText(int32(code))
}

func redeemMsgCodeTextFromRaw(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Prefer an embedded {"code":N,...} payload, including rpc wrapper strings.
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start >= 0 && end > start {
		var payload struct {
			Code int `json:"code"`
		}
		if json.Unmarshal([]byte(raw[start:end+1]), &payload) == nil {
			if text := redeemMsgCodeText(payload.Code); text != "" {
				return text
			}
		}
	}
	return ""
}

func redeemSuccessMessage(code string, items []RedeemItemGain, mailNew int) string {
	parts := make([]string, 0, len(items)+1)
	for _, item := range items {
		name := item.Name
		if name == "" {
			name = fmt.Sprintf("#%d", item.ItemID)
		}
		parts = append(parts, fmt.Sprintf("%sx%d", name, item.Count))
	}
	msg := fmt.Sprintf("兑换成功 [%s]", code)
	switch {
	case len(parts) > 0:
		msg += " → " + strings.Join(parts, "、")
	case mailNew > 0:
		msg += fmt.Sprintf(" → 奖励已入邮件（%d 封待领取）", mailNew)
	default:
		msg += " → 未观察到即时物品变化（可能已发邮件或此前已兑换）"
	}
	if len(parts) > 0 && mailNew > 0 {
		msg += fmt.Sprintf("；另有 %d 封奖励邮件", mailNew)
	}
	return msg
}

func redeemGains(beforeInv, afterInv map[int32]int32, beforeGold, afterGold, beforeDiamonds, afterDiamonds int32) []RedeemItemGain {
	type gain struct {
		id    int32
		count int32
	}
	var gains []gain
	seen := make(map[int32]struct{})
	for id, after := range afterInv {
		before := beforeInv[id]
		if after > before {
			gains = append(gains, gain{id: id, count: after - before})
			seen[id] = struct{}{}
		}
	}
	for id, before := range beforeInv {
		if _, ok := seen[id]; ok {
			continue
		}
		after := afterInv[id]
		if after > before {
			gains = append(gains, gain{id: id, count: after - before})
		}
	}
	sort.Slice(gains, func(i, j int) bool {
		if gains[i].count == gains[j].count {
			return gains[i].id < gains[j].id
		}
		return gains[i].count > gains[j].count
	})

	out := make([]RedeemItemGain, 0, len(gains)+2)
	if afterGold > beforeGold {
		out = append(out, RedeemItemGain{ItemID: 0, Name: "金币", Count: afterGold - beforeGold})
	}
	if afterDiamonds > beforeDiamonds {
		out = append(out, RedeemItemGain{ItemID: 1, Name: "元宝", Count: afterDiamonds - beforeDiamonds})
	}
	for _, g := range gains {
		out = append(out, RedeemItemGain{
			ItemID: g.id,
			Name:   state.ItemLabel(g.id),
			Count:  g.count,
		})
	}
	return out
}

func mailKeySet(mails []state.MailView) map[string]struct{} {
	out := make(map[string]struct{}, len(mails))
	for _, mail := range mails {
		key := fmt.Sprintf("%d:%d", mail.MsID, mail.AllID)
		out[key] = struct{}{}
	}
	return out
}

func countNewRewardMails(before map[string]struct{}, after []state.MailView) int {
	count := 0
	for _, mail := range after {
		key := fmt.Sprintf("%d:%d", mail.MsID, mail.AllID)
		if _, ok := before[key]; ok {
			continue
		}
		if mail.IsDel != 0 || mail.IsPick != 0 || len(mail.ItemsRaw) == 0 || string(mail.ItemsRaw) == "null" {
			continue
		}
		count++
	}
	return count
}
