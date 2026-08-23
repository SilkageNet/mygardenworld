package runner

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestHandleOperationSuccessBacksOffNoopCustomerGeneration(t *testing.T) {
	now := time.Date(2026, 8, 23, 22, 15, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	r := newOperationEventTestRunner()
	r.state.ApplyVMap(map[string]any{
		"109": map[string]any{"0": map[string]any{
			"1": map[string]any{},
			"2": now.Add(-time.Minute).UnixMilli(),
		}},
	})
	op := customerOrderGenerationTestOp()

	r.handleOperationSuccess(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		finishedAt:       now,
		raw:              json.RawMessage(`{}`),
	})

	cd, cooling := r.operationCoolingDown(op, now.Add(time.Second))
	if !cooling {
		t.Fatal("successful no-op customer generation did not enter cooldown")
	}
	if want := now.Add(customerOrderGenerationNoopCooldown); !cd.Until.Equal(want) {
		t.Fatalf("cooldown until=%v, want %v", cd.Until, want)
	}
	if _, cooling := r.operationCoolingDown(op, cd.Until); cooling {
		t.Fatal("customer generation cooldown did not expire at its boundary")
	}
}

func TestHandleOperationSuccessDoesNotBackOffGeneratedCustomerOrder(t *testing.T) {
	now := time.Date(2026, 8, 23, 22, 15, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	r := newOperationEventTestRunner()
	// executePlannedOp applies the response before handleOperationSuccess. Seed
	// the resulting active order to model a generation response with progress.
	r.state.ApplyVMap(map[string]any{
		"109": map[string]any{"0": map[string]any{
			"1": map[string]any{"7": map[string]any{"1": 300505, "2": 1}},
		}},
	})
	op := customerOrderGenerationTestOp()

	r.handleOperationSuccess(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		finishedAt:       now,
		raw:              json.RawMessage(`{"109":{"0":{"1":{"7":{"1":300505,"2":1}}}}}`),
	})

	if _, cooling := r.operationCoolingDown(op, now.Add(time.Second)); cooling {
		t.Fatal("customer generation with an active order entered no-op cooldown")
	}
}

func customerOrderGenerationTestOp() *automation.PlannedOp {
	kind := clientproto.RPCOrderCustomerGenOrder.String()
	return &automation.PlannedOp{
		OperationID: kind,
		Kind:        kind,
		Lane:        automation.LaneSide,
		Category:    automation.CategoryOrder,
		Domain:      "order.customer",
		Action:      "generate",
	}
}
