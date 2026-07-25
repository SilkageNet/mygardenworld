package runner

import (
	"strings"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestResidentOrderFinishSuccessMessage(t *testing.T) {
	normal := &automation.PlannedOp{
		Kind:     clientproto.RPCOrderFlowerFinishOrder.String(),
		TargetID: 3,
		Reason:   "居民订单可交付 (玫瑰×2、百合×1)",
	}
	got := residentOrderFinishSuccessMessage("普通居民订单", normal)
	if !strings.Contains(got, "完成普通居民订单") || !strings.Contains(got, "格子=3") || !strings.Contains(got, "玫瑰×2、百合×1") {
		t.Fatalf("normal message=%q", got)
	}

	satin := &automation.PlannedOp{
		Kind:   clientproto.RPCOrderFlowerFinishSatinOrder.String(),
		Reason: "绸缎居民订单可交付 (郁金香×3)",
	}
	got = residentOrderFinishSuccessMessage("绸缎订单", satin)
	if got != "完成绸缎订单: 郁金香×3" {
		t.Fatalf("satin message=%q", got)
	}

	decorate := &automation.PlannedOp{
		Kind: clientproto.RPCOrderFlowerFinishDecorateOrder.String(),
	}
	got = residentOrderFinishSuccessMessage("建材订单", decorate)
	if got != "完成建材订单" {
		t.Fatalf("decorate message=%q", got)
	}
}

func TestOperationEventLabelResidentOrders(t *testing.T) {
	cases := []struct {
		kind string
		want string
	}{
		{clientproto.RPCOrderFlowerFinishOrder.String(), "普通居民订单"},
		{clientproto.RPCOrderFlowerFinishSatinOrder.String(), "绸缎订单"},
		{clientproto.RPCOrderFlowerFinishDecorateOrder.String(), "建材订单"},
		{clientproto.RPCOrderFlowerRecvOrderRwd.String(), "居民订单领奖"},
	}
	for _, tc := range cases {
		if got := operationEventLabel(&automation.PlannedOp{Kind: tc.kind}); got != tc.want {
			t.Fatalf("operationEventLabel(%s)=%q, want %q", tc.kind, got, tc.want)
		}
	}
}
