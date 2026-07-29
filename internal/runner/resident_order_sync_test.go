package runner

import (
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestIsResidentSpecialOrderCooldownKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{clientproto.RPCOrderFlowerFinishSatinOrder.String(), true},
		{clientproto.RPCOrderFlowerFinishDecorateOrder.String(), true},
		{clientproto.RPCOrderFlowerFinishSatinOrder.String() + "|order.resident.satin|finish", true},
		{"order.resident.satin.finish", true},
		{"order.resident.decorate.finish", true},
		{clientproto.RPCOrderFlowerFinishOrder.String(), false},
		{"order.customer.finish", false},
	}
	for _, tc := range cases {
		if got := isResidentSpecialOrderCooldownKey(tc.key); got != tc.want {
			t.Fatalf("isResidentSpecialOrderCooldownKey(%q)=%v, want %v", tc.key, got, tc.want)
		}
	}
}

func TestClearResidentSpecialOrderCooldowns(t *testing.T) {
	r := &Runner{operationCooldowns: map[string]operationCooldown{
		clientproto.RPCOrderFlowerFinishSatinOrder.String(): {
			Until:  time.Now().Add(time.Hour),
			Reason: "服务端提示订单冷却中",
		},
		clientproto.RPCOrderFlowerFinishDecorateOrder.String(): {
			Until:  time.Now().Add(time.Hour),
			Reason: "服务端提示今日完成订单次数已达上限",
		},
		clientproto.RPCOrderFlowerFinishOrder.String(): {Until: time.Now().Add(time.Hour)},
	}}
	r.clearResidentSpecialOrderCooldowns()
	if _, ok := r.operationCooldowns[clientproto.RPCOrderFlowerFinishSatinOrder.String()]; ok {
		t.Fatal("satin cooldown should be cleared")
	}
	if _, ok := r.operationCooldowns[clientproto.RPCOrderFlowerFinishDecorateOrder.String()]; ok {
		t.Fatal("decorate cooldown should be cleared")
	}
	if _, ok := r.operationCooldowns[clientproto.RPCOrderFlowerFinishOrder.String()]; !ok {
		t.Fatal("ordinary resident cooldown should remain")
	}
}

func TestClearResidentSpecialOrderRetryTimers(t *testing.T) {
	r := &Runner{operationCooldowns: map[string]operationCooldown{
		clientproto.RPCOrderFlowerFinishSatinOrder.String() + "|order.resident.satin|finish": {
			Until:  time.Now().Add(61 * time.Second),
			Reason: "服务端提示订单冷却中",
		},
		clientproto.RPCOrderFlowerFinishDecorateOrder.String(): {
			Until:  time.Now().Add(61 * time.Second),
			Reason: "服务端提示订单冷却中",
		},
	}}
	r.clearResidentSpecialOrderRetryTimers(clientproto.RPCOrderFlowerFinishSatinOrder.String())
	if _, ok := r.operationCooldowns[clientproto.RPCOrderFlowerFinishSatinOrder.String()+"|order.resident.satin|finish"]; ok {
		t.Fatal("satin retry timer should be closed")
	}
	if _, ok := r.operationCooldowns[clientproto.RPCOrderFlowerFinishDecorateOrder.String()]; !ok {
		t.Fatal("decorate retry timer should remain when clearing satin")
	}
}

func TestResetResidentOrderSessionClearsSyncTick(t *testing.T) {
	r := &Runner{
		operationCooldowns:        map[string]operationCooldown{},
		lastResidentOrderSyncTick: time.Now(),
	}
	r.resetResidentOrderSession()
	if !r.lastResidentOrderSyncTick.IsZero() {
		t.Fatalf("sync tick=%v, want zero after session reset", r.lastResidentOrderSyncTick)
	}
}
