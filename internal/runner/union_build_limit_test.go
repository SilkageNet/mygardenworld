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

func TestIsFmlBuildDailyLimitError(t *testing.T) {
	rpcErr := &babigame.RPCServerError{
		Name:     clientproto.RPCFmlBld,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"code":383,"args":[]}`)},
	}
	if !isFmlBuildDailyLimitError(clientproto.RPCFmlBld.String(), rpcErr) {
		t.Fatal("expected code 383 to match")
	}
	if !isFmlBuildDailyLimitError(clientproto.RPCFmlBld.String(), errors.New("每日建设次数已达上限")) {
		t.Fatal("expected translated message to match")
	}
	if isFmlBuildDailyLimitError(clientproto.RPCFmlRaceDelTask.String(), rpcErr) {
		t.Fatal("must not match another RPC")
	}
}

func TestHandleOperationErrorFmlBuildDailyLimitSoftDefersUntilTomorrow(t *testing.T) {
	r := newOperationEventTestRunner()
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.Local)
	rpcErr := &babigame.RPCServerError{
		Name:     clientproto.RPCFmlBld,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"code":383,"args":[]}`)},
	}
	op := &automation.PlannedOp{
		OperationID: "fml.bld:2",
		Kind:        clientproto.RPCFmlBld.String(),
		Lane:        automation.LaneSide,
		Category:    automation.CategoryUnion,
		Domain:      "union.build",
		Action:      "build",
		TargetID:    2,
	}

	got := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              rpcErr,
		finishedAt:       now,
	})
	if got != nil {
		t.Fatalf("daily limit must soft-defer, got %v", got)
	}
	if op.CooldownKey != "union.build" {
		t.Fatalf("cooldown key=%q, want shared union.build", op.CooldownKey)
	}
	cd, cooling := r.operationCoolingDown(op, now.Add(time.Second))
	if !cooling {
		t.Fatal("expected shared build cooldown")
	}
	want := time.Date(2026, 8, 29, 0, 0, 0, 0, time.Local)
	if !cd.Until.Equal(want) {
		t.Fatalf("cooldown until=%v, want %v", cd.Until, want)
	}
}
