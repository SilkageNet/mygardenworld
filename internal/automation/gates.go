package automation

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func annotateOperationGates(s *state.State, ops []PlannedOp, now time.Time) {
	for i := range ops {
		op := &ops[i]
		op.CostGates = mergeCostGates(op.CostGates, implicitOperationCostGates(s, op, now))
		if op.BlockingStage == "" {
			op.BlockingStage = inferBlockingStage(op.BlockedReasons)
		}
		if len(op.BlockedReasons) > 0 && op.Status == "" {
			op.Status = PlanStatusBlocked
		}
		if op.Status == PlanStatusBlocked || op.Status == PlanStatusAdapterMissing || op.SyncOnly || !op.Executable {
			continue
		}
		var reasons []string
		status := PlanStatusBlocked
		for _, gate := range op.CostGates {
			if !gate.Blocking() {
				continue
			}
			reasons = append(reasons, gate.BlockedReasons...)
			if gate.Status == PlanStatusAdapterMissing {
				status = PlanStatusAdapterMissing
			}
		}
		if len(reasons) == 0 {
			continue
		}
		op.Status = status
		op.Executable = false
		op.BlockedReasons = append(op.BlockedReasons, reasons...)
		op.BlockingStage = inferBlockingStage(op.BlockedReasons)
	}
}

type sequentialResourceBudget struct {
	gold       int64
	diamonds   int64
	waterDrops int64
	items      map[int32]int64
}

type operationResourceCost struct {
	gold       int64
	diamonds   int64
	waterDrops int64
	items      map[int32]int64
}

func annotateSequentialResourceBudget(s *state.State, ops []PlannedOp, now time.Time) {
	if s == nil || len(ops) == 0 {
		return
	}
	waterDrops, _, _ := s.AvailableWaterDrops(now)
	budget := sequentialResourceBudget{
		gold:       int64(s.Gold()),
		diamonds:   int64(s.SpendableDiamonds()),
		waterDrops: int64(waterDrops),
		items:      int64Inventory(s.Inventory()),
	}
	for i := range ops {
		op := &ops[i]
		if !operationConsumesQueueBudget(*op) {
			continue
		}
		cost := operationCostFromGates(op.CostGates)
		if cost.empty() {
			continue
		}
		gates := budget.queueBlockedGates(cost)
		if len(gates) > 0 {
			var reasons []string
			for _, gate := range gates {
				reasons = append(reasons, gate.BlockedReasons...)
			}
			op.Status = PlanStatusBlocked
			op.Executable = false
			op.BlockedReasons = append(op.BlockedReasons, reasons...)
			op.BlockingStage = inferBlockingStage(op.BlockedReasons)
			op.CostGates = append(op.CostGates, gates...)
			continue
		}
		budget.spend(cost)
	}
}

func operationConsumesQueueBudget(op PlannedOp) bool {
	return op.Executable &&
		!op.SyncOnly &&
		op.Status != PlanStatusAdapterMissing &&
		op.Status != PlanStatusBlocked &&
		len(op.BlockedReasons) == 0
}

func int64Inventory(in map[int32]int32) map[int32]int64 {
	out := make(map[int32]int64, len(in))
	for id, count := range in {
		out[id] = int64(count)
	}
	return out
}

func operationCostFromGates(gates []CostGate) operationResourceCost {
	var cost operationResourceCost
	for _, gate := range gates {
		if gate.Required <= 0 || !gate.Hard || gate.Source == "operation.queue" {
			continue
		}
		switch gate.ResourceKind {
		case GateResourceDiamond:
			cost.diamonds = max(cost.diamonds, gate.Required)
		case GateResourceGold:
			if gate.Required > cost.gold {
				cost.gold = gate.Required
			}
		case GateResourceWaterDrop:
			if gate.Required > cost.waterDrops {
				cost.waterDrops = gate.Required
			}
		case GateResourceItem:
			if gate.ItemID <= 0 {
				continue
			}
			if cost.items == nil {
				cost.items = make(map[int32]int64)
			}
			if gate.Required > cost.items[gate.ItemID] {
				cost.items[gate.ItemID] = gate.Required
			}
		}
	}
	return cost
}

func (c operationResourceCost) empty() bool {
	return c.gold <= 0 && c.diamonds <= 0 && c.waterDrops <= 0 && len(c.items) == 0
}

func (b sequentialResourceBudget) queueBlockedGates(cost operationResourceCost) []CostGate {
	var gates []CostGate
	if cost.diamonds > b.diamonds {
		gates = append(gates, queueBudgetGate("diamond", GateResourceDiamond, "元宝", 1, cost.diamonds, b.diamonds))
	}
	if cost.gold > b.gold {
		gates = append(gates, queueBudgetGate("gold", GateResourceGold, "金币", 0, cost.gold, b.gold))
	}
	if cost.waterDrops > b.waterDrops {
		gates = append(gates, queueBudgetGate("water_drop", GateResourceWaterDrop, "水滴", 7, cost.waterDrops, b.waterDrops))
	}
	if len(cost.items) > 0 {
		ids := make([]int32, 0, len(cost.items))
		for id := range cost.items {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			required := cost.items[id]
			available := b.items[id]
			if required <= available {
				continue
			}
			gates = append(gates, queueBudgetGate("item:"+strconv.FormatInt(int64(id), 10), GateResourceItem, itemLabel(id), id, required, available))
		}
	}
	return gates
}

func queueBudgetGate(id, kind, label string, itemID int32, required, available int64) CostGate {
	return CostGate{
		ID:             "queue_budget:" + id,
		ResourceKind:   kind,
		Label:          label,
		ItemID:         itemID,
		Required:       required,
		Available:      available,
		Status:         PlanStatusBlocked,
		BlockedReasons: []string{fmt.Sprintf("队列资源不足: %s需要 %d，队列剩余 %d", label, required, available)},
		Hard:           true,
		Source:         "operation.queue",
	}
}

func (b *sequentialResourceBudget) spend(cost operationResourceCost) {
	b.diamonds = max(0, b.diamonds-cost.diamonds)
	b.gold -= cost.gold
	if b.gold < 0 {
		b.gold = 0
	}
	b.waterDrops -= cost.waterDrops
	if b.waterDrops < 0 {
		b.waterDrops = 0
	}
	for id, count := range cost.items {
		b.items[id] -= count
		if b.items[id] < 0 {
			b.items[id] = 0
		}
	}
}

func implicitOperationCostGates(s *state.State, op *PlannedOp, now time.Time) []CostGate {
	if s == nil || op == nil {
		return nil
	}
	var out []CostGate
	if op.GoldCost > 0 {
		out = append(out, resourceGate("gold", GateResourceGold, "金币", 0, int64(op.GoldCost), int64(s.Gold()), "operation.cost"))
	}
	if op.DiamondCost > 0 {
		available := s.SpendableDiamonds()
		gate := resourceGate("diamond", GateResourceDiamond, "元宝", 1, int64(op.DiamondCost), int64(available), "operation.cost")
		if len(gate.BlockedReasons) == 0 && op.Kind != clientproto.RPCFmlRaceUpgradeTask.String() {
			gate.Status = PlanStatusAdapterMissing
			gate.BlockedReasons = []string{"元宝成本操作默认不自动执行"}
		}
		out = append(out, gate)
	}
	if len(op.ItemCost) > 0 {
		inventory := s.Inventory()
		ids := make([]int32, 0, len(op.ItemCost))
		for itemID := range op.ItemCost {
			ids = append(ids, itemID)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, itemID := range ids {
			count := op.ItemCost[itemID]
			if count <= 0 {
				continue
			}
			out = append(out, resourceGate(
				"item:"+strconv.FormatInt(int64(itemID), 10),
				GateResourceItem,
				itemLabel(itemID),
				itemID,
				int64(count),
				int64(inventory[itemID]),
				"operation.cost",
			))
		}
	}
	if isWaterRPC(op.Kind) {
		need := int32(len(op.LandIDs))
		available, _, _ := s.AvailableWaterDrops(now)
		gate := resourceGate("water_drop", GateResourceWaterDrop, "水滴", 7, int64(need), int64(available), "operation.resource")
		if need <= 0 {
			gate.Status = PlanStatusBlocked
			gate.BlockedReasons = []string{"浇水操作缺少田地"}
		}
		out = append(out, gate)
	}
	return out
}

func mergeCostGates(existing, implicit []CostGate) []CostGate {
	if len(existing) == 0 {
		return implicit
	}
	if len(implicit) == 0 {
		return existing
	}
	seen := make(map[string]struct{}, len(existing))
	for _, gate := range existing {
		seen[gate.ID] = struct{}{}
	}
	out := append([]CostGate(nil), existing...)
	for _, gate := range implicit {
		if _, ok := seen[gate.ID]; ok {
			continue
		}
		out = append(out, gate)
	}
	return out
}

func resourceGate(id, kind, label string, itemID int32, required, available int64, source string) CostGate {
	gate := CostGate{
		ID:           id,
		ResourceKind: kind,
		Label:        label,
		ItemID:       itemID,
		Required:     required,
		Available:    available,
		Status:       PlanStatusReady,
		Hard:         true,
		Source:       source,
	}
	if required > available {
		gate.Status = PlanStatusBlocked
		gate.BlockedReasons = []string{fmt.Sprintf("%s不足: 需要 %d，当前 %d", label, required, available)}
	}
	return gate
}

func isWaterRPC(kind string) bool {
	return kind == clientproto.RPCUsrLandWater.String() ||
		kind == clientproto.RPCUsrLandWaterBatch.String()
}

func inferBlockingStage(reasons []string) string {
	for _, reason := range reasons {
		switch {
		case strings.Contains(reason, "策略") || strings.Contains(reason, "上限") || strings.Contains(reason, "预算"):
			return "policy"
		case strings.Contains(reason, "配方") || strings.Contains(reason, "配置"):
			return "recipe"
		case strings.Contains(reason, "花瓶"):
			return "vase"
		case strings.Contains(reason, "等级"):
			return "level"
		case strings.Contains(reason, "金币") || strings.Contains(reason, "元宝") || strings.Contains(reason, "水滴") ||
			strings.Contains(reason, "库存") || strings.Contains(reason, "不足"):
			return "resource"
		case strings.Contains(reason, "未观察") || strings.Contains(reason, "未观测") || strings.Contains(reason, "状态"):
			return "state"
		case strings.Contains(reason, "adapter") || strings.Contains(reason, "协议") || strings.Contains(reason, "执行"):
			return "adapter"
		}
	}
	return ""
}
