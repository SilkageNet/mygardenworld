package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestCultivateUpgradeResourceRejectionBlocksUntilObservationChanges(t *testing.T) {
	r := newOperationEventTestRunner()
	r.state.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"22526": 741},
			"44": 51873,
		}},
		"101": map[string]any{"0": map[string]any{
			"23526": map[string]any{"1": 23526, "2": 12, "4": 2},
		}},
	})
	op := &automation.PlannedOp{
		OperationID: "cultivate.upgrade|flower=23526",
		Kind:        clientproto.RPCCultivateUpgrade.String(),
		Lane:        automation.LaneSide,
		Category:    automation.CategoryPlant,
		Domain:      "farm.upgrade",
		Action:      "upgrade",
		Status:      automation.PlanStatusManaged,
		Executable:  true,
		FlowerID:    23526,
		Count:       12,
		GoldCost:    27000,
		ItemCost:    map[int32]int32{22526: 500},
	}
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	err := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              errors.New(`rpc cultivate.upgrade: server: {"code":301,"param":{"iid":11}}`),
		finishedAt:       now,
	})
	if err != nil {
		t.Fatalf("handleOperationError=%v, want handled resource rejection", err)
	}
	if !r.cultivateUpgradeResourceRejectedUnchanged(op) {
		t.Fatal("unchanged upgrade resources should remain blocked")
	}
	if got := r.selectRunnableOperation([]automation.PlannedOp{*op}, now.Add(time.Second)); got != nil {
		t.Fatalf("rejected upgrade became runnable without an observation change: %+v", got)
	}

	r.state.ApplyVMap(map[string]any{"7": map[string]any{"0": map[string]any{"44": 51874}}})
	if r.cultivateUpgradeResourceRejectedUnchanged(op) {
		t.Fatal("gold observation change should release the rejection guard")
	}
	if got := r.selectRunnableOperation([]automation.PlannedOp{*op}, now.Add(2*time.Second)); got == nil {
		t.Fatal("upgrade should become runnable after a resource observation changes")
	}
}

func TestCultivateUpgradeResourceRejectionClassification(t *testing.T) {
	err := errors.New(`rpc cultivate.upgrade: server: {"code":301,"param":{"iid":11}}`)
	if got := resourceRejectedItemID(err); got != 11 {
		t.Fatalf("resourceRejectedItemID=%d, want 11", got)
	}
	if got := classifyOperationError(clientproto.RPCCultivateUpgrade.String(), err); got != operationErrorCultivateUpgradeRejected {
		t.Fatalf("classifyOperationError=%q, want %q", got, operationErrorCultivateUpgradeRejected)
	}
	if got := classifyOperationError(clientproto.RPCCultivateCultivate.String(), err); got != operationErrorOrdinary {
		t.Fatalf("non-upgrade classifyOperationError=%q, want ordinary", got)
	}
}
