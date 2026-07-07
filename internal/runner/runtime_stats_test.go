package runner

import (
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestRuntimeStatsResourceGainsSkipInitialSnapshot(t *testing.T) {
	stats := newRuntimeStats(time.Unix(100, 0))
	stats.ObserveResourceSnapshot(state.ResourceSnapshot{
		Gold:         1000,
		WaterDrops:   8,
		Experience:   200,
		DiamondsFree: 3,
	}, time.Unix(101, 0))
	stats.ObserveInventorySnapshot(state.InventorySnapshot{
		Inventory: map[int32]int32{7: 8, 23001: 12},
		Changes: []state.InventoryItemDelta{
			{ItemID: 7, Before: 0, After: 8},
			{ItemID: 23001, Before: 0, After: 12},
		},
	}, time.Unix(101, 0))

	stats.ObserveResourceSnapshot(state.ResourceSnapshot{
		Gold:         1400,
		WaterDrops:   10,
		Experience:   260,
		DiamondsFree: 2,
	}, time.Unix(102, 0))
	stats.ObserveInventorySnapshot(state.InventorySnapshot{
		Inventory: map[int32]int32{7: 10, 23001: 15, 1002: 5},
		Changes: []state.InventoryItemDelta{
			{ItemID: 7, Before: 8, After: 10},
			{ItemID: 23001, Before: 12, After: 15},
			{ItemID: 1002, Before: 0, After: 5},
		},
	}, time.Unix(102, 0))

	got := stats.Snapshot()
	gains := resourceGainsByKey(got.ResourceGains)
	if gains["gold"] != 400 {
		t.Fatalf("gold gain=%d, want 400", gains["gold"])
	}
	if gains["water_drops"] != 2 {
		t.Fatalf("water drop gain=%d, want 2", gains["water_drops"])
	}
	if gains["experience"] != 60 {
		t.Fatalf("experience gain=%d, want 60", gains["experience"])
	}
	if gains["diamonds_free"] != 0 {
		t.Fatalf("diamond gain=%d, want 0 for negative delta", gains["diamonds_free"])
	}
	if gains["item:7"] != 0 {
		t.Fatalf("item water drops gain=%d, want skipped to avoid double count", gains["item:7"])
	}
	if gains["item:23001"] != 3 || gains["item:1002"] != 5 {
		t.Fatalf("item gains=%v, want item:23001=3 item:1002=5", gains)
	}
}

func TestRuntimeStatsOperationSuccessClassifiesOrdersAndTasks(t *testing.T) {
	stats := newRuntimeStats(time.Unix(100, 0))
	stats.RecordOperationSuccess(&automation.PlannedOp{Kind: clientproto.RPCOrderFlowerFinishOrder.String()}, time.Unix(101, 0))
	stats.RecordOperationSuccess(&automation.PlannedOp{Kind: clientproto.RPCOrderCustomerFinishOrder.String()}, time.Unix(102, 0))
	stats.RecordOperationSuccess(&automation.PlannedOp{Kind: clientproto.RPCTaskDlyRecv.String()}, time.Unix(103, 0))
	stats.RecordOperationSuccess(&automation.PlannedOp{Kind: clientproto.RPCRandomEventDoAffair.String()}, time.Unix(104, 0))

	got := stats.Snapshot()
	if got.TotalOperations != 4 {
		t.Fatalf("TotalOperations=%d, want 4", got.TotalOperations)
	}
	orders := actionTotalsByKey(got.OrderCompletions)
	if orders["resident_normal"] != 1 || orders["customer"] != 1 {
		t.Fatalf("order completions=%v, want resident_normal=1 customer=1", orders)
	}
	tasks := actionTotalsByKey(got.TaskCompletions)
	if tasks["daily"] != 1 || tasks["random_event"] != 1 {
		t.Fatalf("task completions=%v, want daily=1 random_event=1", tasks)
	}
}

func resourceGainsByKey(items []RuntimeResourceTotal) map[string]int64 {
	out := make(map[string]int64, len(items))
	for _, item := range items {
		out[item.Key] = item.Gained
	}
	return out
}

func actionTotalsByKey(items []RuntimeActionTotal) map[string]int64 {
	out := make(map[string]int64, len(items))
	for _, item := range items {
		out[item.Key] = item.Count
	}
	return out
}
