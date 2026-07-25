package runner

import (
	"strings"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestOpKindDescFlowerRack(t *testing.T) {
	if got := opKindDesc(clientproto.RPCFlowerRackSell.String()); got != "花艺上架" {
		t.Fatalf("opKindDesc(sell)=%q", got)
	}
	if got := opKindDesc(clientproto.RPCFlowerRackRecvSellMoney.String()); got != "花艺售出领取" {
		t.Fatalf("opKindDesc(claim)=%q", got)
	}
}

func TestFlowerRackSellSuccessMessage(t *testing.T) {
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFlowerRackSell.String(),
		TargetID: 2,
		ItemID:   300208,
		Count:    12,
	}
	got := flowerRackSellSuccessMessage(op)
	if !strings.Contains(got, "花艺上架成功") || !strings.Contains(got, "花架=2") || !strings.Contains(got, "×12") {
		t.Fatalf("flowerRackSellSuccessMessage=%q", got)
	}
}

func TestFlowerRackClaimSuccessMessageUsesGoldDelta(t *testing.T) {
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFlowerRackRecvSellMoney.String(),
		TargetID: 3,
		ItemID:   300208,
		Count:    4,
	}
	got := flowerRackClaimSuccessMessage(op, 1000, 1572)
	if !strings.Contains(got, "花艺售出领取成功") || !strings.Contains(got, "金币+572") || !strings.Contains(got, "花架=3") {
		t.Fatalf("flowerRackClaimSuccessMessage=%q", got)
	}
}

func TestFlowerRackOperationTargetSuffix(t *testing.T) {
	sell := &automation.PlannedOp{
		Kind:     clientproto.RPCFlowerRackSell.String(),
		TargetID: 1,
		ItemID:   300208,
		Count:    12,
	}
	got := operationTargetSuffix(sell)
	if !strings.Contains(got, "花架=1") || !strings.Contains(got, "×12") {
		t.Fatalf("sell suffix=%q", got)
	}
}

func TestOperationEventLabelFlowerRack(t *testing.T) {
	if got := operationEventLabel(&automation.PlannedOp{Kind: clientproto.RPCFlowerRackSell.String()}); got != "花艺上架" {
		t.Fatalf("sell label=%q", got)
	}
	if got := operationEventLabel(&automation.PlannedOp{Kind: clientproto.RPCFlowerRackRecvSellMoney.String()}); got != "花艺售出" {
		t.Fatalf("claim label=%q", got)
	}
}
