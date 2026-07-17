package runner

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestActivityItemGateNeverFallsBackToNormalInventory(t *testing.T) {
	raw, err := os.ReadFile("../state/testdata/dessert_activity.json")
	if err != nil {
		t.Fatal(err)
	}
	st := state.New()
	st.ApplyV(raw)
	st.ApplyVMap(map[string]any{"7": map[string]any{"0": map[string]any{"32": map[string]any{"1347": 999}}}})
	r := &Runner{state: st}
	gate := automation.CostGate{ResourceKind: automation.GateResourceActivityItem, ItemID: 1347, Required: 1, Label: "甜糕奖励箱"}

	if err := r.checkOperationResources(&automation.PlannedOp{BatchID: 9101, CostGates: []automation.CostGate{gate}}, time.Now()); err != nil {
		t.Fatalf("observed activity bag rejected: %v", err)
	}
	if err := r.checkOperationResources(&automation.PlannedOp{BatchID: 9102, CostGates: []automation.CostGate{gate}}, time.Now()); err == nil || !strings.Contains(err.Error(), "活动背包尚未完整同步") {
		t.Fatalf("wrong batch used normal inventory: %v", err)
	}
	if err := r.checkOperationResources(&automation.PlannedOp{CostGates: []automation.CostGate{gate}}, time.Now()); err == nil || !strings.Contains(err.Error(), "缺少活动批次") {
		t.Fatalf("missing batch accepted: %v", err)
	}

	gate.ItemID = 9999
	if err := r.checkOperationResources(&automation.PlannedOp{BatchID: 9101, CostGates: []automation.CostGate{gate}}, time.Now()); err == nil || !strings.Contains(err.Error(), "当前 0") {
		t.Fatalf("observed zero activity balance accepted: %v", err)
	}
}
