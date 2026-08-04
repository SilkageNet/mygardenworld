package runner

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestWaterClaimSuccessMessages(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 48},
			"33": map[string]any{"7": map[string]any{"1": 130}},
		}},
		"114": map[string]any{
			"1": 12,
		},
	})

	gotWW := waterwheelClaimSuccessMessage(43, st)
	if !strings.Contains(gotWW, "水车水滴领取成功") || !strings.Contains(gotWW, "+5") || !strings.Contains(gotWW, "当前 48/130") || !strings.Contains(gotWW, "今日 12/") {
		t.Fatalf("waterwheelClaimSuccessMessage=%q", gotWW)
	}

	op := &automation.PlannedOp{Kind: clientproto.RPCFreeWaterRecv.String(), TargetID: 1}
	gotFree := freeWaterClaimSuccessMessage(op, 28, st)
	if !strings.Contains(gotFree, "限时水滴领取成功") || !strings.Contains(gotFree, "时段#1") || !strings.Contains(gotFree, "+20") || !strings.Contains(gotFree, "当前 48/130") {
		t.Fatalf("freeWaterClaimSuccessMessage=%q", gotFree)
	}
}

func TestOperationEventLabelWaterClaims(t *testing.T) {
	if got := operationEventLabel(&automation.PlannedOp{Kind: clientproto.RPCWaterwheelRecv.String()}); got != "水车水滴" {
		t.Fatalf("waterwheel label=%q", got)
	}
	if got := operationEventLabel(&automation.PlannedOp{Kind: clientproto.RPCFreeWaterRecv.String()}); got != "限时水滴" {
		t.Fatalf("free water label=%q", got)
	}
	if got := opKindDesc(clientproto.RPCWaterwheelRecv.String()); got != "水车水滴" {
		t.Fatalf("opKindDesc waterwheel=%q", got)
	}
	if got := opKindDesc(clientproto.RPCFreeWaterRecv.String()); got != "限时水滴" {
		t.Fatalf("opKindDesc free water=%q", got)
	}
}

func TestHandleOperationSuccessEmitsWaterCategory(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 50},
			"33": map[string]any{"7": map[string]any{"1": 130}},
		}},
		"114": map[string]any{"1": 5},
	})
	bus := NewBus()
	ch, cancel := bus.SubscribeLive(4)
	defer cancel()

	r := &Runner{
		state:   st,
		bus:     bus,
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		account: &store.Account{ID: 1, Name: "test"},
	}
	r.handleOperationSuccess(context.Background(), operationResult{
		operationAttempt: operationAttempt{
			op: &automation.PlannedOp{
				Kind:     clientproto.RPCWaterwheelRecv.String(),
				Category: automation.CategoryWater,
				Domain:   "basic.waterwheel",
				Action:   "claim",
			},
			waterDropsBefore: 45,
		},
		finishedAt: time.Now(),
	})

	select {
	case got := <-ch:
		if got.Kind != "waterwheel" {
			t.Fatalf("kind=%q, want waterwheel", got.Kind)
		}
		if got.Category != automation.CategoryWater {
			t.Fatalf("category=%q, want water", got.Category)
		}
		if got.Label != "水车水滴" {
			t.Fatalf("label=%q", got.Label)
		}
		if !strings.Contains(got.Message, "水车水滴领取成功") {
			t.Fatalf("message=%q", got.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for waterwheel event")
	}
}

func TestEventCategoryWaterKinds(t *testing.T) {
	if got := eventCategory("waterwheel"); got != "water" {
		t.Fatalf("eventCategory(waterwheel)=%q", got)
	}
	if got := eventCategory("free_water"); got != "water" {
		t.Fatalf("eventCategory(free_water)=%q", got)
	}
	if got := normalizeEventCategory("water", "waterwheel"); got != "water" {
		t.Fatalf("normalizeEventCategory=%q", got)
	}
}
