package runner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestClassifyOperationErrorOrderServerAnomaly(t *testing.T) {
	err5000 := errors.New(`rpc orderCustomer.genOrder: server: {"code":5000,"args":[]}`)
	if got := classifyOperationError(clientproto.RPCOrderCustomerGenOrder.String(), err5000); got != operationErrorOrderServerAnomaly {
		t.Fatalf("genOrder classify=%s, want order_server_anomaly", got)
	}
	if got := classifyOperationError(clientproto.RPCOrderFlowerFinishOrder.String(), err5000); got != operationErrorOrderServerAnomaly {
		t.Fatalf("finishOrder classify=%s, want order_server_anomaly", got)
	}
	// benefitBox keeps its own path
	if got := classifyOperationError(clientproto.RPCBenefitBoxDraw.String(), err5000); got != operationErrorBenefitBoxDrawRejected {
		t.Fatalf("benefitBox classify=%s, want benefit_box_draw_rejected", got)
	}
}

func TestHandleRqstFailurePausesOrdersOnCode5000(t *testing.T) {
	r := &Runner{
		account:            &store.Account{ID: 1, Name: "test"},
		log:                slog.New(slog.NewTextHandler(io.Discard, nil)),
		state:              state.New(),
		operationCooldowns: map[string]operationCooldown{},
	}
	op := &automation.PlannedOp{
		OperationID: "orderFlower.finishOrder|target=3",
		Kind:        clientproto.RPCOrderFlowerFinishOrder.String(),
		Lane:        automation.LaneSide,
		Category:    automation.CategoryOrder,
		Domain:      "order.resident",
		Action:      "finish",
		TargetID:    3,
	}
	err := errors.New(`rqst flowerOrderRqst.showR: {"code":5000,"args":[]}`)
	before := time.Now()
	r.handleRqstFailure(context.Background(), op, err, err)

	mid := before.Add(time.Minute)
	if !r.orderAnomalyPauseActive(mid) {
		t.Fatal("order anomaly pause not active after rqst 5000")
	}
	otherBox := &automation.PlannedOp{
		OperationID: "orderFlower.finishOrder|target=7",
		Kind:        clientproto.RPCOrderFlowerFinishOrder.String(),
		Lane:        automation.LaneSide,
		Category:    automation.CategoryOrder,
		Domain:      "order.resident",
		Action:      "finish",
		TargetID:    7,
	}
	if !r.orderAnomalyBlocks(otherBox, mid) {
		t.Fatal("other resident box should be blocked by shared anomaly pause")
	}
	gen := &automation.PlannedOp{
		Kind:     clientproto.RPCOrderCustomerGenOrder.String(),
		Lane:     automation.LaneSide,
		Category: automation.CategoryOrder,
		Domain:   "order.customer",
		Action:   "generate",
	}
	if !r.orderAnomalyBlocks(gen, mid) {
		t.Fatal("customer gen should be blocked by shared anomaly pause")
	}
	if r.orderAnomalyBlocks(gen, before.Add(serverAnomalyOrderPause+time.Second)) {
		t.Fatal("anomaly pause should expire")
	}
}

func TestHandleOperationErrorOrderGenAnomaly(t *testing.T) {
	now := time.Date(2026, 9, 22, 1, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	r := newOperationEventTestRunner()
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCOrderCustomerGenOrder.String(),
		Lane:     automation.LaneSide,
		Category: automation.CategoryOrder,
		Domain:   "order.customer",
		Action:   "generate",
	}
	err := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              errors.New(`rpc orderCustomer.genOrder: server: {"code":5000,"args":[]}`),
		finishedAt:       now,
	})
	if err != nil {
		t.Fatalf("handleOperationError=%v, want nil (deferred)", err)
	}
	if !r.orderAnomalyPauseActive(now.Add(time.Minute)) {
		t.Fatal("genOrder 5000 should pause order automation")
	}
}

func TestSelectRunnableOperationSkipsOrderAnomaly(t *testing.T) {
	now := time.Date(2026, 9, 22, 0, 30, 0, 0, time.UTC)
	r := newOperationEventTestRunner()
	r.orderAnomalyUntil = now.Add(10 * time.Minute)
	candidates := []automation.PlannedOp{
		{
			OperationID: "orderFlower.finishOrder|target=1",
			Kind:        clientproto.RPCOrderFlowerFinishOrder.String(),
			Lane:        automation.LaneSide,
			Category:    automation.CategoryOrder,
			Domain:      "order.resident",
			Action:      "finish",
			Status:      automation.PlanStatusReady,
			Executable:  true,
			Priority:    9000,
		},
		{
			OperationID: "pearlPlace.recvOneKey",
			Kind:        clientproto.RPCPearlPlaceRecvOneKey.String(),
			Lane:        automation.LaneSide,
			Category:    automation.CategoryBasic,
			Domain:      "basic.pearl",
			Action:      "claim",
			Status:      automation.PlanStatusReady,
			Executable:  true,
			Priority:    1000,
		},
	}
	got := r.selectRunnableOperation(candidates, now)
	if got == nil {
		t.Fatal("expected non-order op to run")
	}
	if got.Kind != clientproto.RPCPearlPlaceRecvOneKey.String() {
		t.Fatalf("selected %s, want pearl claim while orders paused", got.Kind)
	}
}
