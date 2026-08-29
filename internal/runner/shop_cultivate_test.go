package runner

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestShopCultivateOfferExhaustedErrorClassification(t *testing.T) {
	rpcErr := &babigame.RPCServerError{
		Name:     clientproto.RPCShopCultivateBuy,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"code":312,"args":[]}`)},
	}
	cases := []struct {
		name string
		kind string
		err  error
		want bool
	}{
		{name: "typed server error", kind: clientproto.RPCShopCultivateBuy.String(), err: rpcErr, want: true},
		{name: "wrapped code", kind: clientproto.RPCShopCultivateBuy.String(), err: errors.New(`rpc shopCultivate.buy: server: {"code":312,"args":[]}`), want: true},
		{name: "localized message", kind: clientproto.RPCShopCultivateBuy.String(), err: errors.New("无法再购买当前商品"), want: true},
		{name: "other operation", kind: clientproto.RPCShopGiftbagBuy.String(), err: rpcErr, want: false},
		{name: "other code", kind: clientproto.RPCShopCultivateBuy.String(), err: errors.New(`{"code":301}`), want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isShopCultivateOfferExhaustedError(tc.kind, tc.err); got != tc.want {
				t.Fatalf("isShopCultivateOfferExhaustedError()=%t, want %t", got, tc.want)
			}
		})
	}
}

func TestHandleShopCultivateOfferExhaustedContinuesWithSibling(t *testing.T) {
	r := newOperationEventTestRunner()
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	r.state.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"44": 200_000}},
		"113": map[string]any{
			"1": map[string]any{
				"10001": []int32{11, 3214},
				"10002": []int32{11, 4215},
			},
			"2": now.UnixMilli(),
			"3": now.UnixMilli(),
			"4": 3,
			"6": map[string]any{},
		},
	})
	op := &automation.PlannedOp{
		Kind:        clientproto.RPCShopCultivateBuy.String(),
		Lane:        automation.LaneSide,
		Category:    automation.CategoryBasic,
		Domain:      "basic.shop.cultivate",
		Action:      "buy",
		OperationID: "shopCultivate.buy|target=10001",
		TargetID:    10001,
		ItemID:      1401,
		Count:       1,
	}
	r.setSideOperationCooldown(op, now, errors.New("prior failure"), "prior failure", time.Minute)
	err := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              errors.New(`rpc shopCultivate.buy: server: {"code":312,"args":[]}`),
		finishedAt:       now,
	})
	if err != nil {
		t.Fatalf("handleOperationError=%v, want handled stale offer", err)
	}
	if _, cooling := r.operationCoolingDown(op, now.Add(time.Second)); cooling {
		t.Fatal("exhausted offer kept ordinary failure cooldown")
	}

	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Shop.CultivateShop.AutoBuy = true
	policy.Basic.Shop.CultivateShop.MaxSpendGold = 200_000
	plan := automation.BuildPlan(r.state, policy, now)
	for _, planned := range plan.Operations {
		if planned.Domain != "basic.shop.cultivate" {
			continue
		}
		if planned.Kind != clientproto.RPCShopCultivateBuy.String() || planned.TargetID != 10002 || !planned.Executable {
			t.Fatalf("material-shop operation=%+v, want executable sibling shop 10002", planned)
		}
		return
	}
	t.Fatalf("missing sibling material-shop buy after code 312: %+v", plan.Operations)
}
