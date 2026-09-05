package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestRedeemUseCodeRequestJSON(t *testing.T) {
	req := clientproto.RedeemUseCodeRequest{Code: "ABC-123"}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["code"] != "ABC-123" {
		t.Fatalf("code field = %#v, want ABC-123", decoded["code"])
	}
}

func TestRedeemCodeRejectsEmpty(t *testing.T) {
	r := &Runner{state: state.New()}
	if _, err := r.RedeemCode(t.Context(), "   "); err == nil {
		t.Fatal("expected empty code error")
	}
	if _, err := r.RedeemCode(t.Context(), "CODE"); err == nil {
		t.Fatal("expected not connected error")
	}
}

func TestRedeemCodeDefersWhenAutomationOperationIsBusy(t *testing.T) {
	r := &Runner{state: state.New()}
	r.operationMu.Lock()
	_, err := r.RedeemCode(context.Background(), "CODE")
	r.operationMu.Unlock()
	if !errors.Is(err, ErrAccountOperationBusy) {
		t.Fatalf("RedeemCode error=%v, want ErrAccountOperationBusy", err)
	}
}

func TestRedeemSuccessMessageIncludesCodeAndItems(t *testing.T) {
	msg := redeemSuccessMessage("SUMMER-1", []RedeemItemGain{
		{ItemID: 1, Name: "元宝", Count: 100},
		{ItemID: 23001, Name: "玫瑰", Count: 2},
	}, 0)
	if msg != "兑换成功 [SUMMER-1] → 元宝x100、玫瑰x2" {
		t.Fatalf("message = %q", msg)
	}
}

func TestRedeemGainsDiff(t *testing.T) {
	before := map[int32]int32{10: 1, 20: 5}
	after := map[int32]int32{10: 1, 20: 8, 30: 2}
	gains := redeemGains(before, after, 100, 150, 10, 10)
	if len(gains) != 3 {
		t.Fatalf("gains=%+v, want 3 entries", gains)
	}
	if gains[0].Name != "金币" || gains[0].Count != 50 {
		t.Fatalf("gold gain = %+v", gains[0])
	}
}

func TestEventMetadataRedeemCode(t *testing.T) {
	if got := eventCategory("redeem_code"); got != "system" {
		t.Fatalf("category=%q", got)
	}
	if got := eventDomain("redeem_code"); got != "redeem.code" {
		t.Fatalf("domain=%q", got)
	}
	if got := eventLabel("redeem_code"); got != "兑换码" {
		t.Fatalf("label=%q", got)
	}
	if got := normalizeEventCategory("redeem", "redeem_code"); got != "system" {
		t.Fatalf("normalize=%q", got)
	}
}

func TestFormatRedeemServerErrorUsesMsgCode(t *testing.T) {
	if got := state.MsgCodeText(335); got != "已领取过该奖励" {
		t.Fatalf("MsgCodeText(335)=%q", got)
	}
	raw := `rpc redeem.useCode: server: {"code":335,"args":[]}`
	if got := formatRedeemServerError(fmt.Errorf("%s", raw), babigame.WSResponseD{}); got != "已领取过该奖励" {
		t.Fatalf("format from raw err = %q", got)
	}
	env := babigame.WSResponseD{M: json.RawMessage(`{"code":335,"args":[]}`)}
	if got := formatRedeemServerError(nil, env); got != "已领取过该奖励" {
		t.Fatalf("format from envelope = %q", got)
	}
	rpcErr := &babigame.RPCServerError{
		Name:     clientproto.RPCRedeemUseCode,
		Envelope: env,
	}
	if got := formatRedeemServerError(rpcErr, babigame.WSResponseD{}); got != "已领取过该奖励" {
		t.Fatalf("format from RPCServerError = %q", got)
	}
	if got := redeemFailureMessage("杨紫666", "已领取过该奖励"); got != "兑换失败 [杨紫666]：已领取过该奖励" {
		t.Fatalf("failure message = %q", got)
	}
}

func TestClassifyRedeemOutcomeUsesObservedGameCodes(t *testing.T) {
	tests := []struct {
		code int
		want RedeemOutcome
	}{
		{330, RedeemOutcomeSuccess},
		{331, RedeemOutcomeInvalid},
		{332, RedeemOutcomeRetryable},
		{333, RedeemOutcomeExpired},
		{334, RedeemOutcomeAlreadyRedeemed},
		{335, RedeemOutcomeAlreadyRedeemed},
		{337, RedeemOutcomeRetryable},
		{999999, RedeemOutcomeUnknown},
	}
	for _, tt := range tests {
		if got := classifyRedeemOutcome(tt.code, nil); got != tt.want {
			t.Errorf("classifyRedeemOutcome(%d)=%q want %q", tt.code, got, tt.want)
		}
	}
}
