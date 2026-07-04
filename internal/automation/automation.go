// Package automation is the pure decision engine. It turns observed state and
// user-enabled goals into demands, a cycle-local inventory ledger, and an
// ordered operation queue.
package automation

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	CategoryAccount  = "account"
	CategoryBasic    = "basic"
	CategoryPlant    = "plant"
	CategoryOrder    = "order"
	CategoryUnion    = "union"
	CategoryActivity = "activity"
	CategorySystem   = "system"
)

const (
	KindHarvest = "harvest"
	KindPlant   = "plant"
	KindWater   = "water"
	KindWait    = "wait"
	KindUnknown = "unknown"
)

const harvestReadyGrace = 5 * time.Second

const (
	DemandKindFlower    = "flower"
	DemandKindFlowerArt = "flower_art"
	DemandKindVase      = "vase"
	DemandKindAction    = "action"
)

const (
	GateResourceGold      = "gold"
	GateResourceDiamond   = "diamond"
	GateResourceItem      = "item"
	GateResourceWaterDrop = "water_drop"
	GateResourceLevel     = "level"
	GateResourceVase      = "vase"
	GateResourcePolicy    = "policy"
	GateResourceState     = "state"
	GateResourceAdapter   = "adapter"
)

const (
	GoalResidentOrder = "order.resident"
	GoalCustomerOrder = "order.customer"
	GoalFlowerArt     = "order.flower_art"
	GoalPalaceOrder   = "order.palace"
	GoalTeamOrder     = "order.team"
	GoalMainTask      = "basic.task.main"
	GoalDailyTask     = "basic.task.daily"
	GoalWeeklyTask    = "basic.task.weekly"
	GoalAutoReplant   = "fallback.auto_replant"
)

const flowerRackPerSlotCount int32 = 12

// Goal is one enabled product objective. Feature enablement is still owned by
// policy sections; goals only give the planner a unified priority surface.
type Goal struct {
	ID       string
	Category string
	Domain   string
	Label    string
	Priority int32
}

// Demand is an item/capability requirement emitted by enabled goals.
type Demand struct {
	ID             string
	GoalID         string
	Category       string
	Domain         string
	EntityID       string
	Source         string
	Label          string
	Kind           string
	ItemID         int32
	Count          int32
	Have           int32
	Allocated      int32
	Available      int32
	Missing        int32
	Priority       int32
	BlockedReasons []string
	Status         string
	BlockingStage  string
	CostGates      []CostGate
}

// CostGate is a structured precondition for a demand or operation. It is the
// planner-side source for UI diagnostics and the runner's final resource check.
type CostGate struct {
	ID             string
	ResourceKind   string
	Label          string
	ItemID         int32
	Required       int64
	Available      int64
	Status         string
	BlockedReasons []string
	Hard           bool
	Source         string
}

func (g CostGate) Blocking() bool {
	return g.Status == PlanStatusBlocked || g.Status == PlanStatusAdapterMissing || len(g.BlockedReasons) > 0
}

// InventoryLedger is the single inventory accounting surface for one planning
// cycle. All demand fulfillment and lower-priority inventory consumers must use
// it instead of reading raw State.Inventory directly.
type InventoryLedger struct {
	owned     map[int32]int32
	allocated map[int32]int32
	byDemand  map[string]map[int32]int32
}

// ArtAvailability describes whether a flower-art recipe can be crafted now.
type ArtAvailability struct {
	Recipe         state.FlowerArtRecipe
	VaseUnlocked   bool
	Craftable      bool
	Requirements   []Demand
	BlockedReasons []string
}

// PlanResult is the full explainable output of one decision pass.
type PlanResult struct {
	Goals      []Goal
	Demands    []Demand
	Ledger     *InventoryLedger
	Operations []PlannedOp
}

func NewInventoryLedger(inventory map[int32]int32) *InventoryLedger {
	owned := make(map[int32]int32, len(inventory))
	for itemID, count := range inventory {
		if count > 0 {
			owned[itemID] = count
		}
	}
	return &InventoryLedger{
		owned:     owned,
		allocated: map[int32]int32{},
		byDemand:  map[string]map[int32]int32{},
	}
}

func (l *InventoryLedger) Owned(itemID int32) int32 {
	if l == nil {
		return 0
	}
	return l.owned[itemID]
}

func (l *InventoryLedger) Allocated(itemID int32) int32 {
	if l == nil {
		return 0
	}
	return l.allocated[itemID]
}

func (l *InventoryLedger) Available(itemID int32) int32 {
	if l == nil {
		return 0
	}
	available := l.owned[itemID] - l.allocated[itemID]
	if available < 0 {
		return 0
	}
	return available
}

func (l *InventoryLedger) Allocate(demand Demand) int32 {
	if l == nil || demand.ID == "" || demand.ItemID <= 0 || demand.Count <= 0 || len(demand.BlockedReasons) > 0 {
		return 0
	}
	if demand.Kind != DemandKindFlower && demand.Kind != DemandKindFlowerArt {
		return 0
	}
	count := demand.Count
	if available := l.Available(demand.ItemID); count > available {
		count = available
	}
	if count <= 0 {
		return 0
	}
	l.allocated[demand.ItemID] += count
	if l.byDemand[demand.ID] == nil {
		l.byDemand[demand.ID] = map[int32]int32{}
	}
	l.byDemand[demand.ID][demand.ItemID] += count
	return count
}

func (l *InventoryLedger) AllocatedForDemand(demandID string, itemID int32) int32 {
	if l == nil || demandID == "" {
		return 0
	}
	return l.byDemand[demandID][itemID]
}

func (l *InventoryLedger) CanSpendItems(items map[int32]int32) bool {
	if l == nil {
		return len(items) == 0
	}
	for itemID, count := range items {
		if count > 0 && l.Available(itemID) < count {
			return false
		}
	}
	return true
}

func (l *InventoryLedger) AllocatedItems() map[int32]int32 {
	if l == nil || len(l.allocated) == 0 {
		return nil
	}
	out := make(map[int32]int32, len(l.allocated))
	for itemID, count := range l.allocated {
		out[itemID] = count
	}
	return out
}

// PlannedOp is one operation candidate. Runners execute only operations marked
// executable and supported by their operation registry.
type PlannedOp struct {
	OperationID    string
	GoalID         string
	DemandID       string
	Kind           string
	FeatureID      string
	Category       string
	Label          string
	Domain         string
	Action         string
	Status         string
	Executable     bool
	SyncOnly       bool
	Reason         string
	BlockedReasons []string
	Priority       int32
	LandIDs        []int32
	SlotIDs        []int32
	FlowerID       int32
	TargetUID      int64
	TargetID       int32
	ItemID         int32
	Count          int32
	VaseID         int32
	FlowerIDs      []int32
	GoldCost       int32
	DiamondCost    int32
	ItemCost       map[int32]int32
	CostGates      []CostGate
	BlockingStage  string
}

func Recommend(land state.LandView, now time.Time) (kind, reason string) {
	if !land.Observed {
		return KindUnknown, "no observed primary state"
	}
	if !land.IsPlanted() {
		return KindPlant, "land is empty"
	}
	if land.State == 3 {
		return KindHarvest, "state=3 (initial bloom ready)"
	}
	if land.State == 2 {
		if land.NextTimeMs > 0 {
			readyAt := time.UnixMilli(land.NextTimeMs).Add(harvestReadyGrace)
			if !now.Before(readyAt) {
				return KindHarvest, fmt.Sprintf("state=2, nextTime(%d)+grace elapsed", land.NextTimeMs)
			}
			return KindWait, fmt.Sprintf("state=2 regrowing; nextTime=%d graceUntil=%d", land.NextTimeMs, readyAt.UnixMilli())
		}
		return KindWait, fmt.Sprintf("state=2 regrowing; nextTime=%d", land.NextTimeMs)
	}
	if land.State == 1 {
		return KindWater, "state=1, awaiting first water"
	}
	return KindWait, fmt.Sprintf("state=%d not actionable", land.State)
}

// Plan returns the first directly executable operation from the full queue.
func Plan(s *state.State, policy *pb.Policy, now time.Time) *PlannedOp {
	result := BuildPlan(s, policy, now)
	for _, op := range result.Operations {
		if op.Executable && !op.SyncOnly && op.Status != PlanStatusAdapterMissing && op.Status != PlanStatusBlocked && len(op.BlockedReasons) == 0 {
			cp := op
			return &cp
		}
	}
	return nil
}

// PlanOperations returns the categorized operation list in execution order.
func PlanOperations(s *state.State, policy *pb.Policy, now time.Time) []PlannedOp {
	return BuildPlan(s, policy, now).Operations
}

// BuildPlan produces enabled goals, ledger-accounted demands, and the full
// ranked operation queue.
func BuildPlan(s *state.State, policy *pb.Policy, now time.Time) PlanResult {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() {
		return PlanResult{}
	}
	policy = DefaultPolicyIfNil(policy)
	goals := enabledGoals(policy)
	ledger := NewInventoryLedger(s.Inventory())
	demands := buildDirectDemands(s, policy, goals)
	applyLedgerAllocations(demands, ledger)
	production := buildProductionDemands(s, policy, goals, demands, ledger)
	applyLedgerAllocations(production, ledger)
	demands = append(demands, production...)
	annotateDemandStatuses(demands)
	sortDemands(demands)
	ops := buildOperations(s, policy, goals, demands, ledger, now)
	annotateOperationGates(s, ops, now)
	sortOperations(ops)
	annotateSequentialResourceBudget(s, ops, now)
	return PlanResult{
		Goals:      goals,
		Demands:    demands,
		Ledger:     ledger,
		Operations: ops,
	}
}

func enabledGoals(policy *pb.Policy) []Goal {
	plant := policy.GetPlant()
	priorities := plant.GetPlanting().GetGoalPriority()
	var goals []Goal
	add := func(enabled bool, id, category, domain, label string) {
		if !enabled {
			return
		}
		goals = append(goals, Goal{
			ID:       id,
			Category: category,
			Domain:   domain,
			Label:    label,
			Priority: priorityFor(priorities, id),
		})
	}
	basic := policy.GetBasic()
	task := basic.GetTask()
	order := policy.GetOrder()
	add(order.GetResident().GetNormalEnabled() || order.GetResident().GetRewardEnabled() ||
		order.GetResident().GetDecorateEnabled() || order.GetResident().GetSatinEnabled(), GoalResidentOrder, CategoryOrder, "order.resident", "居民订单")
	add(order.GetCustomer().GetEnabled(), GoalCustomerOrder, CategoryOrder, "order.customer", "顾客订单")
	add(order.GetPalace().GetEnabled(), GoalPalaceOrder, CategoryOrder, "order.palace", "宫廷订单")
	add(order.GetTeam().GetEnabled(), GoalTeamOrder, CategoryOrder, "order.team", "组团订单")
	flowerArt := order.GetFlowerArt()
	add(flowerArt.GetSellEnabled() || flowerArt.GetCraftEnabled() || flowerArt.GetCreateRewardEnabled() || flowerArt.GetCollectRewardEnabled(), GoalFlowerArt, CategoryOrder, "order.flower_art", "花艺/花架")
	add(task.GetMainEnabled(), GoalMainTask, CategoryBasic, "basic.task.main", "主线任务")
	add(task.GetDailyEnabled(), GoalDailyTask, CategoryBasic, "basic.task.daily", "每日任务")
	add(task.GetWeeklyEnabled(), GoalWeeklyTask, CategoryBasic, "basic.task.weekly", "每周任务")
	return goals
}

func priorityFor(priorities map[string]int32, id string) int32 {
	if v := priorities[id]; v != 0 {
		return v
	}
	return defaultGoalPriority()[id]
}

func goalByID(goals []Goal, id string) (Goal, bool) {
	for _, goal := range goals {
		if goal.ID == id {
			return goal, true
		}
	}
	return Goal{}, false
}

func buildDirectDemands(s *state.State, policy *pb.Policy, goals []Goal) []Demand {
	inventory := s.Inventory()
	var out []Demand
	add := func(goal Goal, source, kind string, itemID, count int32, entityID, label string, blocked []string) {
		if itemID <= 0 || count <= 0 {
			return
		}
		have := inventory[itemID]
		missing := count - have
		if missing < 0 {
			missing = 0
		}
		out = append(out, Demand{
			ID:             demandID(goal.ID, entityID, source, kind, itemID),
			GoalID:         goal.ID,
			Category:       goal.Category,
			Domain:         goal.Domain,
			EntityID:       entityID,
			Source:         source,
			Label:          label,
			Kind:           kind,
			ItemID:         itemID,
			Count:          count,
			Have:           have,
			Available:      have,
			Missing:        missing,
			Priority:       goal.Priority,
			BlockedReasons: append([]string(nil), blocked...),
		})
	}
	if goal, ok := goalByID(goals, GoalResidentOrder); ok {
		resident := policy.GetOrder().GetResident()
		if resident.GetNormalEnabled() {
			for boxID, order := range s.FlowerOrders() {
				if !residentFlowerOrderAllowed(order, resident) {
					continue
				}
				entityID := strconv.FormatInt(int64(boxID), 10)
				for _, req := range order.Requires {
					add(goal, "direct", DemandKindFlower, req.FlowerID, req.Count, entityID, fmt.Sprintf("居民订单 #%d", boxID), nil)
				}
			}
		}
	}
	if goal, ok := goalByID(goals, GoalCustomerOrder); ok {
		for npcID, order := range s.CustomerOrderDetails() {
			if order == nil {
				continue
			}
			entityID := strconv.FormatInt(int64(npcID), 10)
			label := fmt.Sprintf("顾客订单 NPC=%d", npcID)
			for _, req := range order.Requires {
				add(goal, "direct", DemandKindFlower, req.FlowerID, req.Count, entityID, label, nil)
			}
			for _, req := range order.ItemRequires {
				if req.ItemID <= 0 || req.Count <= 0 {
					continue
				}
				missingArt := req.Count - inventory[req.ItemID]
				if missingArt < 0 {
					missingArt = 0
				}
				recipe, ok := state.FlowerArtRecipeByID(req.ItemID)
				if !ok {
					add(goal, "direct", DemandKindFlowerArt, req.ItemID, req.Count, entityID, label, []string{"缺少花艺配方"})
					continue
				}
				var blocked []string
				if missingArt > 0 {
					blocked = artBlockedReasons(s, recipe)
				}
				add(goal, "direct", DemandKindFlowerArt, req.ItemID, req.Count, entityID, label, blocked)
			}
		}
	}
	if goal, ok := goalByID(goals, GoalPalaceOrder); ok {
		order := s.PalaceOrder()
		palace := policy.GetOrder().GetPalace()
		if palaceOrderAllowed(order, palace) {
			add(goal, "direct", DemandKindFlower, order.FlowerID, order.Num, "current", "宫廷订单", nil)
		}
	}
	if goal, ok := goalByID(goals, GoalTeamOrder); ok {
		order := s.TeamOrder()
		team := policy.GetOrder().GetTeam()
		if teamOrderAllowed(order, team, s) {
			add(goal, "direct", DemandKindFlower, order.FlowerID, teamOrderNeedCount(order), "current", "组团订单", nil)
		}
	}
	if goal, ok := goalByID(goals, GoalMainTask); ok {
		if task, taskOK := s.MainTask(); taskOK {
			if flowerID, missing, reqOK := state.MainTaskFlowerRequirement(task.TaskID, task.Finished); reqOK {
				add(goal, "direct", DemandKindFlower, flowerID, missing, strconv.FormatInt(int64(task.TaskID), 10), state.MainTaskTitle(task.TaskID), nil)
			}
		}
	}
	sortDemands(out)
	return out
}

func buildProductionDemands(s *state.State, policy *pb.Policy, goals []Goal, direct []Demand, ledger *InventoryLedger) []Demand {
	var out []Demand
	if goal, ok := goalByID(goals, GoalCustomerOrder); ok {
		for _, demand := range direct {
			if demand.GoalID != goal.ID || demand.Kind != DemandKindFlowerArt || demand.Missing <= 0 || len(demand.BlockedReasons) > 0 {
				continue
			}
			out = appendCraftFlowerDemands(out, s, ledger, goal, demand.EntityID,
				"craft:"+strconv.FormatInt(int64(demand.ItemID), 10),
				fmt.Sprintf("%s 制作 %s", demand.Label, itemLabel(demand.ItemID)), demand.ItemID, demand.Missing)
		}
	}
	sortDemands(out)
	return out
}

func appendCraftFlowerDemands(out []Demand, s *state.State, ledger *InventoryLedger, goal Goal, entityID, source, label string, artID, craftCount int32) []Demand {
	if craftCount <= 0 {
		return out
	}
	recipe, ok := state.FlowerArtRecipeByID(artID)
	if !ok || len(artBlockedReasons(s, recipe)) > 0 {
		return out
	}
	if ledger == nil {
		ledger = NewInventoryLedger(s.Inventory())
	}
	for flowerID, count := range recipeFlowerCounts(recipe) {
		required := count * craftCount
		have := ledger.Owned(flowerID)
		available := ledger.Available(flowerID)
		missing := required - available
		if missing < 0 {
			missing = 0
		}
		out = append(out, Demand{
			ID:        demandID(goal.ID, entityID, source, DemandKindFlower, flowerID),
			GoalID:    goal.ID,
			Category:  goal.Category,
			Domain:    goal.Domain,
			EntityID:  entityID,
			Source:    source,
			Label:     label,
			Kind:      DemandKindFlower,
			ItemID:    flowerID,
			Count:     required,
			Have:      have,
			Available: available,
			Missing:   missing,
			Priority:  goal.Priority,
		})
	}
	return out
}

func applyLedgerAllocations(demands []Demand, ledger *InventoryLedger) {
	for i := range demands {
		demands[i].Have = ledger.Owned(demands[i].ItemID)
		demands[i].Allocated = ledger.Allocate(demands[i])
		demands[i].Available = ledger.Available(demands[i].ItemID)
		demands[i].Missing = demands[i].Count - demands[i].Allocated
		if demands[i].Missing < 0 {
			demands[i].Missing = 0
		}
	}
}

func annotateDemandStatuses(demands []Demand) {
	for i := range demands {
		d := &demands[i]
		if len(d.BlockedReasons) > 0 {
			d.Status = PlanStatusBlocked
			d.BlockingStage = inferBlockingStage(d.BlockedReasons)
			continue
		}
		if d.Missing > 0 {
			d.Status = PlanStatusManaged
			continue
		}
		d.Status = PlanStatusReady
	}
}

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
	waterDrops int64
	items      map[int32]int64
}

type operationResourceCost struct {
	gold       int64
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
	return c.gold <= 0 && c.waterDrops <= 0 && len(c.items) == 0
}

func (b sequentialResourceBudget) queueBlockedGates(cost operationResourceCost) []CostGate {
	var gates []CostGate
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
		if len(gate.BlockedReasons) == 0 {
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

func sortDemands(demands []Demand) {
	sort.SliceStable(demands, func(i, j int) bool {
		if demands[i].Priority != demands[j].Priority {
			return demands[i].Priority > demands[j].Priority
		}
		if demands[i].Missing != demands[j].Missing {
			return demands[i].Missing > demands[j].Missing
		}
		return demands[i].ID < demands[j].ID
	})
}

func buildOperations(s *state.State, policy *pb.Policy, goals []Goal, demands []Demand, ledger *InventoryLedger, now time.Time) []PlannedOp {
	var ops []PlannedOp
	ops = append(ops, farmOps(s, policy.GetPlant(), demands, now)...)
	ops = append(ops, orderOperations(s, policy, goals, demands, ledger, now)...)
	ops = append(ops, basicOperations(s, policy, goals, now)...)
	ops = append(ops, shopOperations(s, policy)...)
	ops = append(ops, maintenanceOperations(s, policy, ledger, now)...)
	ops = append(ops, unionOperations(s, policy.GetUnion())...)
	ops = append(ops, blockedUnknownOperations(policy)...)
	return ops
}

func farmOps(s *state.State, policy *pb.PlantPolicy, demands []Demand, now time.Time) []PlannedOp {
	if policy == nil {
		return nil
	}
	plantingPolicy := policy.GetPlanting()
	lands := s.Lands()
	var harvest, water, plant []int32
	ids := make([]int32, 0, len(lands))
	for id := range lands {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		kind, _ := Recommend(lands[id], now)
		switch kind {
		case KindHarvest:
			harvest = append(harvest, id)
		case KindWater:
			water = append(water, id)
		case KindPlant:
			plant = append(plant, id)
		}
	}
	var ops []PlannedOp
	if !plantingPolicy.GetAutoEnabled() {
		return ops
	}
	if len(harvest) > 0 {
		ops = append(ops, landOp(clientproto.RPCUsrLandHarvest.String(), "farm.harvest", "harvest", fmt.Sprintf("%d ready lands", len(harvest)), 10000, harvest, 0, "", ""))
	}
	if len(plant) > 0 {
		assignments := plantAssignments(s, policy, demands, int32(len(plant)))
		cursor := 0
		for _, assignment := range assignments {
			if cursor >= len(plant) {
				break
			}
			count := int(assignment.Count)
			if count > len(plant)-cursor {
				count = len(plant) - cursor
			}
			picks := append([]int32(nil), plant[cursor:cursor+count]...)
			cursor += count
			kind := clientproto.RPCUsrLandPlant.String()
			if len(picks) > 1 {
				kind = clientproto.RPCUsrLandPlantBatch.String()
			}
			ops = append(ops, landOp(kind, "farm.plant", "plant", assignment.Reason, assignment.Priority, picks, assignment.FlowerID, assignment.GoalID, assignment.DemandID))
		}
	}
	if len(water) > 0 {
		waterDrops, _, _ := s.AvailableWaterDrops(now)
		minDrops := plantingPolicy.GetMinWaterDrops()
		if minDrops < 0 {
			minDrops = 0
		}
		usableDrops := waterDrops - minDrops
		if usableDrops > 0 {
			want := int32(len(water))
			if want > usableDrops {
				want = usableDrops
			}
			if want > 0 {
				picks := water[:want]
				switch {
				case len(picks) > 1:
					ops = append(ops, landOp(clientproto.RPCUsrLandWaterBatch.String(), "farm.water", "water", "lands need water", 8000, picks, 0, "", ""))
				default:
					ops = append(ops, landOp(clientproto.RPCUsrLandWater.String(), "farm.water", "water", "land needs water", 8000, picks, 0, "", ""))
				}
			}
		}
	}
	return ops
}

type plantAssignment struct {
	FlowerID int32
	Count    int32
	Priority int32
	GoalID   string
	DemandID string
	Reason   string
}

func plantAssignments(s *state.State, policy *pb.PlantPolicy, demands []Demand, emptyCount int32) []plantAssignment {
	if emptyCount <= 0 {
		return nil
	}
	plantingPolicy := policy.GetPlanting()
	allowed, blocked := autoReplantFlowerFilters(plantingPolicy)
	candidates := s.PlantableFlowers(allowed, blocked)
	plantable := map[int32]state.PlantableFlower{}
	for _, candidate := range s.PlantableFlowers(nil, nil) {
		plantable[candidate.FlowerID] = candidate
	}
	var out []plantAssignment
	remaining := emptyCount
	for _, demand := range demands {
		if remaining <= 0 {
			break
		}
		if demand.Kind != DemandKindFlower || demand.Missing <= 0 || len(demand.BlockedReasons) > 0 {
			continue
		}
		if _, ok := plantable[demand.ItemID]; !ok {
			out = append(out, plantAssignment{
				FlowerID: demand.ItemID,
				Priority: demand.Priority*100 + 500,
				GoalID:   demand.GoalID,
				DemandID: demand.ID,
				Reason:   "需求花朵尚未培育，无法种植",
			})
			continue
		}
		count := demand.Missing
		if count > remaining {
			count = remaining
		}
		if count <= 0 {
			continue
		}
		out = append(out, plantAssignment{
			FlowerID: demand.ItemID,
			Count:    count,
			Priority: demand.Priority*100 + 500,
			GoalID:   demand.GoalID,
			DemandID: demand.ID,
			Reason:   demand.Label,
		})
		remaining -= count
	}
	if len(executableAssignments(out)) > 0 || remaining <= 0 {
		return executableAssignments(out)
	}
	return autoReplantAssignments(candidates, remaining)
}

func executableAssignments(in []plantAssignment) []plantAssignment {
	out := in[:0]
	for _, assignment := range in {
		if assignment.Count > 0 {
			out = append(out, assignment)
		}
	}
	return out
}

func autoReplantAssignments(candidates []state.PlantableFlower, limit int32) []plantAssignment {
	if len(candidates) == 0 || limit <= 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Stock != candidates[j].Stock {
			return candidates[i].Stock < candidates[j].Stock
		}
		if candidates[i].Gold != candidates[j].Gold {
			return candidates[i].Gold > candidates[j].Gold
		}
		return candidates[i].FlowerID < candidates[j].FlowerID
	})
	var out []plantAssignment
	remaining := limit
	for remaining > 0 {
		minStock := candidates[0].Stock
		nextStock := minStock + 1
		for _, candidate := range candidates {
			if candidate.Stock > minStock {
				nextStock = candidate.Stock
				break
			}
		}
		advanced := false
		for i := range candidates {
			if remaining <= 0 {
				break
			}
			if candidates[i].Stock > minStock {
				break
			}
			count := nextStock - candidates[i].Stock
			if count <= 0 {
				count = 1
			}
			if count > remaining {
				count = remaining
			}
			out = append(out, plantAssignment{
				FlowerID: candidates[i].FlowerID,
				Count:    count,
				Priority: priorityFor(defaultGoalPriority(), GoalAutoReplant)*100 + 100,
				GoalID:   GoalAutoReplant,
				Reason:   "自主补种",
			})
			candidates[i].Stock += count
			remaining -= count
			advanced = true
		}
		if !advanced {
			break
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].Stock != candidates[j].Stock {
				return candidates[i].Stock < candidates[j].Stock
			}
			if candidates[i].Gold != candidates[j].Gold {
				return candidates[i].Gold > candidates[j].Gold
			}
			return candidates[i].FlowerID < candidates[j].FlowerID
		})
	}
	return out
}

func autoReplantFlowerFilters(policy *pb.PlantingPolicy) (allowed []int32, blocked []int32) {
	if policy == nil {
		return nil, nil
	}
	switch policy.GetAutoReplantMode() {
	case pb.SelectionMode_SELECTION_MODE_SPECIFIC:
		return uniquePositiveInt32s(policy.GetAutoReplantFlowerIds()), nil
	case pb.SelectionMode_SELECTION_MODE_EXCLUDE:
		return nil, uniquePositiveInt32s(policy.GetAutoReplantExcludeFlowerIds())
	default:
		return nil, nil
	}
}

func uniquePositiveInt32s(values []int32) []int32 {
	seen := map[int32]bool{}
	var out []int32
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func orderOperations(s *state.State, policy *pb.Policy, goals []Goal, demands []Demand, ledger *InventoryLedger, now time.Time) []PlannedOp {
	var ops []PlannedOp
	order := policy.GetOrder()
	if goal, ok := goalByID(goals, GoalResidentOrder); ok {
		resident := order.GetResident()
		if resident.GetNormalEnabled() {
			if blocked, ok := residentOrderLimitBlock(s, resident, goal); ok {
				ops = append(ops, blocked)
			} else {
				statsObserved := s.Statistics().Observed
				for boxID, flowerOrder := range s.FlowerOrders() {
					if !residentFlowerOrderAllowed(flowerOrder, resident) {
						ops = append(ops, blockedResidentOrderOp(flowerOrder, boxID, goal, "居民订单品质不符合策略"))
						continue
					}
					if !flowerOrder.CooldownReady(now) {
						continue
					}
					if canFulfillFlowerOrder(flowerOrder, boxID, goal, ledger) {
						reason := "居民订单可交付"
						if !statsObserved {
							reason = "居民订单可交付；未观察到今日统计 namespace 124"
						}
						ops = append(ops, op(clientproto.RPCOrderFlowerFinishOrder.String(), goal, "finish", reason, goal.Priority*100+700, boxID, 0, 0))
					}
				}
			}
		}
		if resident.GetRewardEnabled() {
			for _, target := range s.ReadyFlowerOrderRewardTargets() {
				ops = append(ops, op(clientproto.RPCOrderFlowerRecvOrderRwd.String(), goal, "reward", "居民订单阶段奖励可领取", goal.Priority*100+620, target, 0, 0))
			}
		}
		if resident.GetSatinEnabled() {
			ops = append(ops, residentSpecialOrderBlockedOp("order.resident.satin", "绸缎居民订单", s.ResidentSatinOrder(), s.Statistics().OrderSatinFinishNum, resident.GetSatinDailyLimit(), goal))
		}
		if resident.GetDecorateEnabled() {
			ops = append(ops, residentSpecialOrderBlockedOp("order.resident.decorate", "建材居民订单", s.ResidentDecorateOrder(), s.Statistics().OrderDecorateFinishNum, resident.GetDecorateDailyLimit(), goal))
		}
	}
	if goal, ok := goalByID(goals, GoalCustomerOrder); ok {
		for npcID, customerOrder := range s.CustomerOrderDetails() {
			if canFulfillCustomerOrder(customerOrder, npcID, goal, ledger) {
				ops = append(ops, op(clientproto.RPCOrderCustomerFinishOrder.String(), goal, "finish", "顾客订单可交付", customerOperationPriority(goal, 200), npcID, 0, 0))
				continue
			}
			rejectable, blockedReasons := customerOrderUnavailableReasons(s, customerOrder)
			if len(rejectable) > 0 {
				if order.GetCustomer().GetRejectUnavailableEnabled() {
					reject := op(clientproto.RPCOrderCustomerRejectOrder.String(), goal, "reject", "顾客订单需求未解锁，执行暂时无货: "+strings.Join(rejectable, "；"), customerOperationPriority(goal, 180), npcID, 0, 0)
					ops = append(ops, reject)
					continue
				}
				blocked := op(clientproto.RPCOrderCustomerRejectOrder.String(), goal, "reject", "顾客订单需求未解锁，等待策略允许暂时无货", goal.Priority*100+130, npcID, 0, 0)
				blocked.Status = PlanStatusBlocked
				blocked.Executable = false
				blocked.BlockedReasons = append([]string{"order.customer.reject_unavailable_enabled 未开启"}, rejectable...)
				ops = append(ops, blocked)
				continue
			}
			if len(blockedReasons) > 0 {
				blocked := op(clientproto.RPCOrderCustomerRejectOrder.String(), goal, "reject", "顾客订单状态不完整，暂不拒单", goal.Priority*100+120, npcID, 0, 0)
				blocked.Status = PlanStatusBlocked
				blocked.Executable = false
				blocked.BlockedReasons = append([]string(nil), blockedReasons...)
				ops = append(ops, blocked)
				continue
			}
			if craft, ok := craftOperationForCustomerOrder(s, customerOrder, npcID, goal, demands, ledger); ok {
				ops = append(ops, craft)
			}
		}
		if s.CustomerOrderGenerationReady(now) {
			ops = append(ops, op(clientproto.RPCOrderCustomerGenOrder.String(), goal, "generate", "顾客订单为空且刷新时间已到，生成顾客订单", customerOperationPriority(goal, 190), 0, 0, 0))
		}
	}
	if goal, ok := goalByID(goals, GoalPalaceOrder); ok {
		palace := order.GetPalace()
		palaceOrder := s.PalaceOrder()
		switch {
		case !palaceOrder.Observed:
			ops = append(ops, syncOnlyOperation(domainOp(clientproto.RPCOrderPalaceEnter.String(), goal, "order.palace", "sync", "宫廷订单未同步，保留同步提示", goal.Priority*100+580, 0, 0, 0), "宫廷订单本轮保持 sync_only，不自动执行 RPC"))
		case palaceOrder.IsFinish != 0:
		case !palaceOrderAllowed(palaceOrder, palace):
			ops = append(ops, blockedPalaceOrderOp(palaceOrder, goal, palace))
		case canFulfillSingleFlowerDemand(goal, "current", palaceOrder.FlowerID, palaceOrder.Num, ledger):
			ops = append(ops, syncOnlyOperation(op(clientproto.RPCOrderPalaceFinishOrder.String(), goal, "finish", "宫廷订单可交付但执行化暂停", goal.Priority*100+570, 0, palaceOrder.FlowerID, palaceOrder.Num), "宫廷订单本轮保持 sync_only，不自动提交"))
		}
	}
	if goal, ok := goalByID(goals, GoalTeamOrder); ok {
		team := order.GetTeam()
		teamOrder := s.TeamOrder()
		switch {
		case !teamOrder.Observed:
			ops = append(ops, syncOnlyOperation(domainOp(clientproto.RPCOrderTeamRefreshOrder.String(), goal, "order.team", "sync", "组团订单未同步，保留同步提示", goal.Priority*100+560, 0, 0, 0), "组团订单本轮保持 sync_only，不自动执行 RPC"))
		case !teamOrderAllowed(teamOrder, team, s):
			ops = append(ops, blockedTeamOrderOp(teamOrder, goal, team, s))
		case canFulfillSingleFlowerDemand(goal, "current", teamOrder.FlowerID, teamOrderNeedCount(teamOrder), ledger):
			ops = append(ops, syncOnlyOperation(op(clientproto.RPCOrderTeamSubmitOrder.String(), goal, "submit", "组团订单可提交但执行化暂停", goal.Priority*100+550, 0, teamOrder.FlowerID, teamOrderNeedCount(teamOrder)), "组团订单本轮保持 sync_only，不自动提交"))
		}
	}
	if goal, ok := goalByID(goals, GoalFlowerArt); ok {
		if order.GetFlowerArt().GetCreateRewardEnabled() {
			for _, vaseID := range s.ReadyArtCreateRewardVaseIDs() {
				claim := domainOp(clientproto.RPCCollectRwdRecvArtCreateRwdByVase.String(), goal, "order.flower_art.create_reward", "claim", "花艺制作经验奖励可领取", goal.Priority*100+670, vaseID, 0, 0)
				ops = append(ops, claim)
				break
			}
		}
		if order.GetFlowerArt().GetCollectRewardEnabled() {
			for _, typeID := range s.ReadyCollectRewardTypes(11, 12, 13) {
				claim := domainOp(clientproto.RPCCollectRwdRecv.String(), goal, "order.flower_art.collect_reward", "claim", "图鉴奖励可领取", goal.Priority*100+660, typeID, 0, 0)
				ops = append(ops, claim)
				break
			}
		}
		if order.GetFlowerArt().GetSellEnabled() {
			for _, rackID := range s.FlowerRackClaimableSlotIDs(now) {
				claim := op(clientproto.RPCFlowerRackRecvSellMoney.String(), goal, "claim", "花架售卖时间已到，可领取收益", flowerRackClaimPriority(goal), rackID, 0, 0)
				ops = append(ops, claim)
				break
			}
			for _, rackID := range s.EmptyFlowerRackSlotIDs() {
				if artID, count, ok := bestRackArt(ledger); ok {
					sell := op(clientproto.RPCFlowerRackSell.String(), goal, "sell", "花架空位可上架未预留花艺", goal.Priority*100+400, rackID, artID, count)
					ops = append(ops, sell)
					break
				}
				if craft, ok := craftOperationForFlowerRack(s, order.GetFlowerArt(), goal, ledger); ok {
					ops = append(ops, craft)
					break
				}
			}
		}
	}
	return ops
}

func craftOperationForCustomerOrder(s *state.State, order *state.CustomerOrder, npcID int32, goal Goal, demands []Demand, ledger *InventoryLedger) (PlannedOp, bool) {
	if order == nil {
		return PlannedOp{}, false
	}
	entityID := strconv.FormatInt(int64(npcID), 10)
	for _, req := range order.ItemRequires {
		demand, ok := demandByID(demands, demandID(goal.ID, entityID, "direct", DemandKindFlowerArt, req.ItemID))
		if !ok || demand.Missing <= 0 {
			continue
		}
		allocated := allocatedCraftFlowerCounts(goal, entityID, req.ItemID, demands)
		availability := FlowerArtAvailabilityWithAllocated(s, req.ItemID, demand.Missing, ledger, allocated)
		if !availability.Craftable {
			blocked := op(clientproto.RPCFlowerArtMakeFlowerArt.String(), goal, "craft", "顾客订单花艺暂不可制作", goal.Priority*100+500, npcID, req.ItemID, demand.Missing)
			blocked.Status = PlanStatusBlocked
			blocked.Executable = false
			blocked.VaseID = availability.Recipe.VaseID
			blocked.FlowerIDs = append([]int32(nil), availability.Recipe.Flowers...)
			blocked.BlockedReasons = append([]string(nil), availability.BlockedReasons...)
			return blocked, true
		}
		craft := op(clientproto.RPCFlowerArtMakeFlowerArt.String(), goal, "craft", "顾客订单缺少花艺成品，材料已满足", customerOperationPriority(goal, 150), npcID, req.ItemID, demand.Missing)
		craft.VaseID = availability.Recipe.VaseID
		craft.FlowerIDs = append([]int32(nil), availability.Recipe.Flowers...)
		return craft, true
	}
	return PlannedOp{}, false
}

func allocatedCraftFlowerCounts(goal Goal, entityID string, artID int32, demands []Demand) map[int32]int32 {
	source := "craft:" + strconv.FormatInt(int64(artID), 10)
	var allocated map[int32]int32
	for _, demand := range demands {
		if demand.GoalID != goal.ID || demand.EntityID != entityID || demand.Source != source || demand.Kind != DemandKindFlower || demand.Allocated <= 0 {
			continue
		}
		if allocated == nil {
			allocated = make(map[int32]int32)
		}
		allocated[demand.ItemID] += demand.Allocated
	}
	return allocated
}

func customerOperationPriority(goal Goal, offset int32) int32 {
	return 11000 + goal.Priority + offset
}

func flowerRackClaimPriority(goal Goal) int32 {
	return 10500 + goal.Priority
}

func craftOperationForFlowerRack(s *state.State, policy *pb.FlowerArtPolicy, goal Goal, ledger *InventoryLedger) (PlannedOp, bool) {
	artID, count, ok := rackCraftTarget(s, policy, ledger)
	if !ok {
		return PlannedOp{}, false
	}
	availability := FlowerArtAvailability(s, artID, count, ledger)
	if !availability.Craftable {
		return PlannedOp{}, false
	}
	craft := op(clientproto.RPCFlowerArtMakeFlowerArt.String(), goal, "craft", "花架缺少花艺成品，材料已满足", goal.Priority*100+450, 0, artID, count)
	craft.VaseID = availability.Recipe.VaseID
	craft.FlowerIDs = append([]int32(nil), availability.Recipe.Flowers...)
	return craft, true
}

func basicOperations(s *state.State, policy *pb.Policy, goals []Goal, now time.Time) []PlannedOp {
	var ops []PlannedOp
	basic := policy.GetBasic()
	task := basic.GetTask()
	benefit := basic.GetBenefit()
	sign := basic.GetSign()
	add := func(enabled bool, kind, domain, action, reason string, priority int32, targetID int32) {
		if !enabled {
			return
		}
		goal := Goal{ID: domain, Category: CategoryBasic, Domain: domain, Label: domain, Priority: priority / 100}
		ops = append(ops, op(kind, goal, action, reason, priority, targetID, 0, 0))
	}
	if basic.GetWaterwheelEnabled() && waterClaimAllowed(s, basic, now) && s.WaterwheelCooldownReady() {
		add(true, clientproto.RPCWaterwheelRecv.String(), "basic.waterwheel", "claim", "水车水滴可领取", 6500, 0)
	}
	if basic.GetFreeWaterEnabled() && waterClaimAllowed(s, basic, now) {
		if idx, ok := s.NextFreeWaterIndex(); ok {
			add(true, clientproto.RPCFreeWaterRecv.String(), "basic.free_water", "claim", "限时水滴可领取", 6450, idx)
		}
	}
	if benefit.GetBoxEnabled() && s.BenefitBoxReady() {
		add(true, clientproto.RPCBenefitBoxDraw.String(), "basic.benefit", "claim", "福利宝箱可领取", 6400, 0)
	}
	if benefit.GetDoubleCoinEnabled() && !s.VideoDoubleActive(now) {
		reason := "双倍金币未生效，看视频奖励需要客户端 SDK token"
		if !s.VideoDoubleObserved() {
			reason = "双倍金币状态未同步，看视频奖励需要客户端 SDK token"
		}
		blocked := markerOp(CategoryBasic, "basic.benefit.double_coin", "claim", reason, 6385)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"客户端通过 UT.share(11,{opType:1}) 完成激励视频后调用 usr.share；本地 runner 不伪造视频完成或 SDK token"}
		ops = append(ops, blocked)
	}
	if benefit.GetAntiScamBoxEnabled() {
		if status, ok := s.AntiFraudQAStatus(); ok && status != state.AntiFraudQAStatusClaimed {
			if status == 1 {
				add(true, clientproto.RPCUsrExtraRecvAntiFraudQARwd.String(), "basic.benefit.anti_scam", "claim", "防骗宝箱问答奖励可领取", 6370, 0)
			} else {
				add(true, clientproto.RPCUsrExtraUpdateAntiFraudQAStatus.String(), "basic.benefit.anti_scam", "answer", "防骗宝箱问答未完成，更新问答状态", 6375, 0)
			}
		}
	}
	if task.GetDailyEnabled() {
		for _, id := range s.ReadyDailyTaskIDs() {
			add(true, clientproto.RPCTaskDlyRecv.String(), "basic.task.daily", "claim", "每日任务奖励可领取", 6250, id)
			break
		}
	}
	if task.GetWeeklyEnabled() {
		for _, id := range s.ReadyWeeklyTaskIDs() {
			add(true, clientproto.RPCTaskWeekRecv.String(), "basic.task.weekly", "claim", "每周任务奖励可领取", 6200, id)
			break
		}
	}
	if basic.GetRoadGrowRewardEnabled() {
		for _, id := range s.ReadyRoadGrowTaskIDs() {
			add(true, clientproto.RPCRoadGrowRecv.String(), "basic.road_grow", "claim", "成长之路奖励可领取", 5980, id)
			break
		}
	}
	if basic.GetRandomEventEnabled() {
		for _, id := range s.ReadyRandomEventIDs() {
			add(true, clientproto.RPCRandomEventDoAffair.String(), "basic.random_event", "claim", "地图事件可处理", 5960, id)
			break
		}
	}
	ops = append(ops, zooOperations(s, basic.GetFeedCat(), now)...)
	if basic.GetMailEnabled() {
		if !s.MailObserved() {
			add(true, clientproto.RPCMailGetList.String(), "basic.mail", "sync", "邮件列表未同步，先获取列表", 5700, 0)
		} else {
			goal := Goal{ID: "basic.mail", Category: CategoryBasic, Domain: "basic.mail", Label: "邮件", Priority: 57}
			for _, target := range s.ReadyMailPickTargets() {
				claim := op(clientproto.RPCMailPick.String(), goal, "claim", "邮件奖励可领取", 5700, target.MsID, target.AllID, 0)
				ops = append(ops, claim)
				break
			}
		}
	}
	if sign.GetDailyEnabled() {
		add(true, clientproto.RPCSignTypeSign.String(), "basic.sign", "claim", "签到由调度退避控制", 5600, 1)
	}
	ops = append(ops, pearlOperations(s, basic.GetPearl(), now)...)
	return ops
}

func waterClaimAllowed(s *state.State, basic *pb.BasicPolicy, now time.Time) bool {
	if s == nil {
		return false
	}
	waterDrops, total, _ := s.AvailableWaterDrops(now)
	if total > 0 && waterDrops >= total {
		return false
	}
	if threshold := basic.GetWaterClaimThreshold(); threshold > 0 && waterDrops >= threshold {
		return false
	}
	return true
}

func zooOperations(s *state.State, policy *pb.FeedCatPolicy, now time.Time) []PlannedOp {
	if policy == nil || !policy.GetEnabled() {
		return nil
	}
	goal := Goal{ID: "basic.zoo", Category: CategoryBasic, Domain: "basic.zoo", Label: "喂猫撸猫", Priority: 57}
	var ops []PlannedOp
	if !s.ZooObserved() {
		return []PlannedOp{domainOp(clientproto.RPCZooEnterZoo.String(), goal, "basic.zoo", "sync", "动物/猫状态未同步，先进入动物园", 5690, 0, 0, 0)}
	}
	if policy.GetAutoFeed() {
		for _, petID := range s.ReadyZooFeedPetIDs() {
			ops = append(ops, domainOp(clientproto.RPCZooFeedPets.String(), goal, "basic.zoo.feed", "feed", "猫碗中已有食物且当前状态可进食", 5680, petID, 0, 0))
			break
		}
	}
	if policy.GetAutoStroke() {
		for _, petID := range s.ReadyZooStrokePetIDs(now) {
			ops = append(ops, domainOp(clientproto.RPCZooStrokePet.String(), goal, "basic.zoo.stroke", "stroke", "猫当前可撸且心情未满", 5670, petID, 0, 0))
			break
		}
	}
	if policy.GetAutoBuyFood() {
		blocked := markerOp(CategoryBasic, "basic.zoo.buy_food", "buy", "购买猫粮涉及成本和商品选择，暂不自动执行", 5660)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"猫粮购买成本和商品选择尚未放开自动执行"}
		ops = append(ops, blocked)
	}
	if policy.GetAutoRecall() {
		blocked := markerOp(CategoryBasic, "basic.zoo.recall", "recall", "自动召回猫的事件链路尚未确认，暂不自动执行", 5650)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"召回事件链路与成本尚未确认"}
		ops = append(ops, blocked)
	}
	return ops
}

func pearlOperations(s *state.State, policy *pb.PearlPolicy, now time.Time) []PlannedOp {
	if policy == nil || !pearlPolicyEnabled(policy) {
		return nil
	}
	goal := Goal{ID: "basic.pearl", Category: CategoryBasic, Domain: "basic.pearl", Label: "珍珠", Priority: 55}
	if !s.PearlObserved() {
		return []PlannedOp{domainOp(clientproto.RPCPearlRefresh.String(), goal, "basic.pearl", "sync", "珍珠状态未同步，先刷新珍珠数据", 5590, 0, 0, 0)}
	}
	var ops []PlannedOp
	if policy.GetFreeEnabled() && s.PearlDailyFreeReady(now) {
		ops = append(ops, domainOp(clientproto.RPCPearlRecvDailyFree.String(), goal, "basic.pearl.free", "claim", "每日免费珍珠可领取", 5580, 0, 0, 0))
	}
	for _, placeID := range s.ReadyPearlPlaceIDs() {
		ops = append(ops, domainOp(clientproto.RPCPearlPlaceRecv.String(), goal, "basic.pearl.place", "claim", "珍珠位产出可收取", 5570, placeID, 0, 0))
		break
	}
	pearl := s.Pearl()
	if policy.GetProtectEnabled() && pearl.ProtectState != 1 {
		protect := domainOp(clientproto.RPCPearlSetProtectState.String(), goal, "basic.pearl.protect", "enable", "珍珠防身未开启", 5560, 1, 0, 0)
		if pearl.ProtectNum <= 0 {
			protect.Status = PlanStatusAdapterMissing
			protect.Executable = false
			protect.BlockedReasons = []string{"防身符不足或未观测"}
		}
		ops = append(ops, protect)
	}
	if policy.GetDrawEnabled() {
		if count := s.PearlDrawCount(); count > 0 {
			draw := domainOp(clientproto.RPCPearlDraw.String(), goal, "basic.pearl.draw", "draw", "存在可开启珍珠", 5550, 0, 0, 1)
			if count < draw.Count {
				draw.Count = count
			}
			ops = append(ops, draw)
		}
	}
	if policy.GetAutoHireEnabled() {
		hire := markerOp(CategoryBasic, "basic.pearl.hire", "hire", "珍珠雇佣需要候选用户与成本确认", 120)
		hire.Label = "雇佣劳工"
		hire.Status = PlanStatusAdapterMissing
		hire.Executable = false
		hire.BlockedReasons = []string{"自动雇佣需要好友/推荐 UID、雇佣券消耗与等级过滤的协议确认"}
		ops = append(ops, hire)
	}
	if policy.GetAutoBuyHireTicket() {
		buy := markerOp(CategoryBasic, "basic.pearl.buy_hire_ticket", "buy", "购买雇佣书涉及元宝成本", 110)
		buy.Label = "购买雇佣书"
		buy.Status = PlanStatusAdapterMissing
		buy.Executable = false
		if policy.GetMaxSpendDiamond() <= 0 {
			buy.BlockedReasons = []string{"购买雇佣书需要先设置元宝上限"}
		} else {
			buy.BlockedReasons = []string{"元宝成本操作尚未放开自动执行"}
		}
		ops = append(ops, buy)
	}
	return ops
}

func pearlPolicyEnabled(policy *pb.PearlPolicy) bool {
	return policy.GetFreeEnabled() || policy.GetAutoHireEnabled() || policy.GetDrawEnabled() ||
		policy.GetProtectEnabled() || policy.GetAutoBuyHireTicket()
}

func shopOperations(s *state.State, policy *pb.Policy) []PlannedOp {
	shop := policy.GetBasic().GetShop()
	var ops []PlannedOp
	ops = append(ops, giftbagOperations(s, shop)...)
	ops = append(ops, cultivateShopOperations(s, shop)...)
	vipShop := shop.GetVipShop()
	if vipShop.GetAutoBuy() {
		blocked := markerOp(CategoryBasic, "basic.shop.vip", "buy", "VIP 商店购买需要成本和状态确认", 115)
		blocked.Label = "VIP 商店"
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"VIP 商店商品状态、花坊币/元宝成本尚未完成协议确认"}
		ops = append(ops, blocked)
	}
	return ops
}

func giftbagOperations(s *state.State, shop *pb.ShopPolicy) []PlannedOp {
	if !shop.GetVideoFreeGiftEnabled() {
		return nil
	}
	goal := Goal{ID: "basic.shop.giftbag", Category: CategoryBasic, Domain: "basic.shop.giftbag", Label: "视频礼包", Priority: 54}
	if !s.ShopGiftbagObserved() {
		return []PlannedOp{domainOp(clientproto.RPCShopGiftbagEnter.String(), goal, "basic.shop.giftbag", "sync", "礼包商店状态未同步，先进入商店获取购买记录", 5480, 0, 0, 0)}
	}
	for _, offer := range s.ShopGiftbagOffers() {
		if !freeVideoGiftbag(offer) || offer.Remaining <= 0 {
			continue
		}
		buy := domainOp(clientproto.RPCShopGiftbagBuy.String(), goal, "basic.shop.video_gift", "claim", "视频免费礼包可领取", 5470, offer.ShopID, 0, 1)
		return []PlannedOp{buy}
	}
	return nil
}

func freeVideoGiftbag(offer state.ShopGiftbagOfferView) bool {
	return offer.Type == 1 && offer.ShareID > 0 && offer.RchgID == 0 &&
		offer.MoneyID == 0 && offer.Price == 0 && offer.PriceMax == 0
}

func cultivateShopOperations(s *state.State, shop *pb.ShopPolicy) []PlannedOp {
	cultivateShop := shop.GetCultivateShop()
	if !cultivateShop.GetAutoBuy() {
		return nil
	}
	goal := Goal{ID: "basic.shop.cultivate", Category: CategoryBasic, Domain: "basic.shop.cultivate", Label: "材料商店", Priority: 54}
	if !s.ShopCultivateObserved() {
		return []PlannedOp{domainOp(clientproto.RPCShopCultivateEnter.String(), goal, "basic.shop.cultivate", "sync", "材料商店状态未同步，先进入商店获取价格", 5450, 0, 0, 0)}
	}
	allowed := int32Set(cultivateShop.GetItemIds())
	inventory := s.Inventory()
	var firstBlocked *PlannedOp
	for _, offer := range s.ShopCultivateOffers() {
		if offer.ShopID <= 0 || offer.Remaining <= 0 {
			continue
		}
		if len(allowed) > 0 && !allowed[offer.ShopID] && !allowed[offer.ItemID] {
			continue
		}
		buy := domainOp(clientproto.RPCShopCultivateBuy.String(), goal, "basic.shop.cultivate", "buy", "材料商店白名单商品可购买", 5400, offer.ShopID, offer.ItemID, offer.ItemCount)
		buy.ItemCost = map[int32]int32{}
		if blocked := applyShopCultivateCostGate(&buy, offer, cultivateShop, s, inventory); len(blocked) > 0 {
			buy.Status = PlanStatusAdapterMissing
			buy.Executable = false
			buy.BlockedReasons = blocked
			if firstBlocked == nil {
				cp := buy
				firstBlocked = &cp
			}
			continue
		}
		if len(buy.ItemCost) == 0 {
			buy.ItemCost = nil
		}
		return []PlannedOp{buy}
	}
	if firstBlocked != nil {
		return []PlannedOp{*firstBlocked}
	}
	return nil
}

func applyShopCultivateCostGate(op *PlannedOp, offer state.ShopCultivateOfferView, policy *pb.ShopBuyPolicy, s *state.State, inventory map[int32]int32) []string {
	if offer.CostItemID <= 0 || offer.CostCount <= 0 {
		return []string{"材料商店价格未观测"}
	}
	switch offer.CostItemID {
	case 11:
		if policy.GetMaxSpendGold() <= 0 {
			return []string{"材料商店金币预算未设置"}
		}
		if int64(offer.CostCount) > policy.GetMaxSpendGold() {
			return []string{"材料商店金币成本超过策略上限"}
		}
		if s.Gold() < offer.CostCount {
			return []string{"金币不足"}
		}
		op.GoldCost = offer.CostCount
	case 1:
		op.DiamondCost = offer.CostCount
		if policy.GetMaxSpendDiamond() <= 0 {
			return []string{"材料商店元宝预算未设置"}
		}
		if int64(offer.CostCount) > policy.GetMaxSpendDiamond() {
			return []string{"材料商店元宝成本超过策略上限"}
		}
		return []string{"元宝成本操作尚未放开自动执行"}
	default:
		if inventory[offer.CostItemID] < offer.CostCount {
			return []string{"成本物品不足或未观测"}
		}
		op.ItemCost[offer.CostItemID] = offer.CostCount
	}
	return nil
}

func unionOperations(s *state.State, union *pb.UnionPolicy) []PlannedOp {
	if union == nil {
		return nil
	}
	var ops []PlannedOp
	ops = append(ops, unionBuildOperations(s, union.GetBuild())...)
	ops = append(ops, unionFlowerOperations(s, union.GetFlower())...)
	ops = append(ops, unionLandOperations(s, union.GetLand())...)
	ops = append(ops, unionForestOperations(s, union.GetForestEnabled())...)
	return ops
}

func unionFlowerOperations(s *state.State, policy *pb.UnionFlowerPolicy) []PlannedOp {
	if policy == nil || (!policy.GetShareEnabled() && !policy.GetTakeEnabled()) {
		return nil
	}
	goal := Goal{ID: "union.flower", Category: CategoryUnion, Domain: "union.flower", Label: "公会鲜花共享", Priority: 44}
	var ops []PlannedOp
	if policy.GetShareEnabled() {
		if !s.FmlFlowerShareObserved() {
			sync := domainOp(clientproto.RPCFmlFlowerShareRefresh.String(), goal, "union.flower.reward", "claim", "公会分享状态未观测，先刷新", 4470, 0, 0, 0)
			ops = append(ops, sync)
		} else if slotIDs := s.ReadyFmlFlowerShareRewardSlotIDs(); len(slotIDs) > 0 {
			claim := domainOp(clientproto.RPCFmlFlowerShareRecvRwd.String(), goal, "union.flower.reward", "claim", "公会分享槽位有可领取奖励", 4470, 0, 0, int32(len(slotIDs)))
			claim.SlotIDs = slotIDs
			ops = append(ops, claim)
		}
	}
	if policy.GetTakeEnabled() {
		if !s.OtherFmlFlowerSharesObserved() {
			sync := domainOp(clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String(), goal, "union.flower.take", "take", "公会摸花列表未观测，先同步", 4460, 0, 0, 0)
			ops = append(ops, sync)
		} else {
			for _, candidate := range s.FmlFlowerTakeCandidates() {
				if !unionFlowerTakeAllowed(candidate, policy) {
					continue
				}
				take := domainOp(clientproto.RPCFmlFlowerShareTake.String(), goal, "union.flower.take", "take", "公会成员分享鲜花可摸取", 4460, candidate.SlotID, 0, 1)
				take.TargetUID = candidate.UID
				take.FlowerID = candidate.FlowerID
				ops = append(ops, take)
				break
			}
		}
	}
	return ops
}

func unionFlowerTakeAllowed(candidate state.FmlFlowerTakeCandidate, policy *pb.UnionFlowerPolicy) bool {
	if candidate.FlowerID <= 0 {
		return false
	}
	mode := policy.GetTakeMode()
	flowers := int32Set(policy.GetTakeFlowerIds())
	qualities := int32Set(policy.GetTakeQualities())
	switch mode {
	case pb.SelectionMode_SELECTION_MODE_SPECIFIC:
		return len(flowers) == 0 || flowers[candidate.FlowerID]
	case pb.SelectionMode_SELECTION_MODE_EXCLUDE:
		return !flowers[candidate.FlowerID]
	case pb.SelectionMode_SELECTION_MODE_QUALITY:
		return len(qualities) == 0 || qualities[flowerQuality(candidate.FlowerID)]
	case pb.SelectionMode_SELECTION_MODE_ALL:
		return true
	default:
		if len(flowers) > 0 && !flowers[candidate.FlowerID] {
			return false
		}
		if len(qualities) > 0 && !qualities[flowerQuality(candidate.FlowerID)] {
			return false
		}
		return true
	}
}

func flowerQuality(flowerID int32) int32 {
	item, ok := state.ItemInfoByID(flowerID)
	if !ok {
		return 0
	}
	return item.Color
}

func unionBuildOperations(s *state.State, policy *pb.UnionBuildPolicy) []PlannedOp {
	if policy == nil || !unionBuildPolicyEnabled(policy) {
		return nil
	}
	goal := Goal{ID: "union.build", Category: CategoryUnion, Domain: "union.build", Label: "公会建设", Priority: 45}
	if !s.FmlBuildObserved() {
		blocked := domainOp(clientproto.RPCFmlBuild.String(), goal, "union.build", "build", "公会建设状态未观测", 4590, 0, 0, 0)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"未观察到公会 namespace 25，需先进入公会或补充 fml.enter 同步链路"}
		return []PlannedOp{blocked}
	}
	build := s.FmlBuild()
	if !build.BuildCountsObserved {
		blocked := domainOp(clientproto.RPCFmlBuild.String(), goal, "union.build", "build", "公会建设次数未观测", 4590, 0, 0, 0)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"未观察到 bldCountMap，无法确认今日建设次数"}
		return []PlannedOp{blocked}
	}
	inventory := s.Inventory()
	var firstBlocked *PlannedOp
	for _, id := range unionBuildOptionIDs(policy) {
		option, ok := state.FmlBuildOptionByID(id)
		if !ok {
			blocked := domainOp(clientproto.RPCFmlBuild.String(), goal, "union.build", "build", "公会建设档位配置缺失", 4500-id, id, 0, 0)
			blocked.Status = PlanStatusAdapterMissing
			blocked.Executable = false
			blocked.BlockedReasons = []string{"缺少 c_fmlBld 静态配置"}
			if firstBlocked == nil {
				firstBlocked = &blocked
			}
			continue
		}
		if option.DailyLimit > 0 && build.BuildCounts[id] >= option.DailyLimit {
			continue
		}
		reason := strings.TrimSpace(option.Name)
		if reason == "" {
			reason = fmt.Sprintf("公会建设 #%d", id)
		}
		buildOp := domainOp(clientproto.RPCFmlBuild.String(), goal, "union.build", "build", reason+"可执行", 4500-id, id, 0, 1)
		if blocked := applyUnionBuildCostGate(&buildOp, option, policy, s, inventory); len(blocked) > 0 {
			buildOp.Status = PlanStatusAdapterMissing
			buildOp.Executable = false
			buildOp.BlockedReasons = blocked
			if firstBlocked == nil {
				cp := buildOp
				firstBlocked = &cp
			}
			continue
		}
		if len(buildOp.ItemCost) == 0 {
			buildOp.ItemCost = nil
		}
		return []PlannedOp{buildOp}
	}
	if firstBlocked != nil {
		return []PlannedOp{*firstBlocked}
	}
	return nil
}

func unionBuildPolicyEnabled(policy *pb.UnionBuildPolicy) bool {
	return policy.GetFreeEnabled() || policy.GetGoldEnabled() || policy.GetDiamondEnabled()
}

func unionBuildOptionIDs(policy *pb.UnionBuildPolicy) []int32 {
	var ids []int32
	if policy.GetFreeEnabled() {
		ids = append(ids, 1)
	}
	if policy.GetGoldEnabled() {
		ids = append(ids, 2)
	}
	if policy.GetDiamondEnabled() {
		ids = append(ids, 3)
	}
	return ids
}

func applyUnionBuildCostGate(op *PlannedOp, option state.FmlBuildOption, policy *pb.UnionBuildPolicy, s *state.State, inventory map[int32]int32) []string {
	if option.ItemID <= 0 || option.Cost <= 0 {
		return nil
	}
	switch option.ItemID {
	case 11:
		if policy.GetMaxSpendGold() <= 0 {
			return []string{"公会金币建设预算未设置"}
		}
		if int64(option.Cost) > policy.GetMaxSpendGold() {
			return []string{"公会金币建设成本超过策略上限"}
		}
		if s.Gold() < option.Cost {
			return []string{"金币不足"}
		}
		op.GoldCost = option.Cost
	case 1:
		op.DiamondCost = option.Cost
		if policy.GetMaxSpendDiamond() <= 0 {
			return []string{"公会元宝建设预算未设置"}
		}
		if int64(option.Cost) > policy.GetMaxSpendDiamond() {
			return []string{"公会元宝建设成本超过策略上限"}
		}
		if s.SpendableDiamonds() < option.Cost {
			return []string{"元宝不足"}
		}
		return []string{"元宝成本操作尚未放开自动执行"}
	default:
		if inventory[option.ItemID] < option.Cost {
			return []string{"公会建设成本物品不足或未观测"}
		}
		op.ItemCost = map[int32]int32{option.ItemID: option.Cost}
	}
	return nil
}

func unionLandOperations(s *state.State, policy *pb.UnionLandPolicy) []PlannedOp {
	if policy == nil || !policy.GetHarvestEnabled() {
		return nil
	}
	goal := Goal{ID: "union.land", Category: CategoryUnion, Domain: "union.land", Label: "公会土地", Priority: 44}
	if !s.FmlLandObserved() {
		blocked := domainOp(clientproto.RPCFmlLandHarvest.String(), goal, "union.land.harvest", "harvest", "公会土地状态未观测", 4480, 0, 0, 0)
		blocked.Status = PlanStatusAdapterMissing
		blocked.Executable = false
		blocked.BlockedReasons = []string{"未观察到 25.102.fmlLand，需先进入公会土地或等待同步"}
		return []PlannedOp{blocked}
	}
	landIDs := s.ReadyFmlLandHarvestIDs()
	if len(landIDs) == 0 {
		return nil
	}
	harvest := domainOp(clientproto.RPCFmlLandHarvest.String(), goal, "union.land.harvest", "harvest", "公会土地有成熟鲜花可收获", 4480, 0, 0, int32(len(landIDs)))
	harvest.LandIDs = landIDs
	return []PlannedOp{harvest}
}

func unionForestOperations(s *state.State, enabled bool) []PlannedOp {
	if !enabled {
		return nil
	}
	goal := Goal{ID: "union.forest", Category: CategoryUnion, Domain: "union.forest", Label: "能量森林", Priority: 43}
	if !s.FmlForestEnergyObserved() {
		sync := domainOp(clientproto.RPCFmlForestRefresh.String(), goal, "union.forest", "collect", "能量森林状态未观测，先刷新并自动收集", 4430, 1, 0, 0)
		return []PlannedOp{sync}
	}
	energy := s.FmlForestEnergy()
	types := s.ReadyFmlForestEnergyTypes()
	if len(types) == 0 {
		return nil
	}
	collect := domainOp(clientproto.RPCFmlForestRefresh.String(), goal, "union.forest", "collect", "能量森林有临时能量可收集", 4430, 1, 0, energy.PendingTempEnergyTotal)
	return []PlannedOp{collect}
}

func maintenanceOperations(s *state.State, policy *pb.Policy, ledger *InventoryLedger, now time.Time) []PlannedOp {
	plant := policy.GetPlant()
	planting := plant.GetPlanting()
	cultivate := plant.GetCultivate()
	var ops []PlannedOp
	goal := Goal{ID: "farm.maintenance", Category: CategoryPlant, Domain: "farm.maintenance", Label: "农场维护", Priority: 55}
	if planting.GetAutoUnlockLand() {
		if landID, goldCost, ok := nextLandUnlockCandidate(s); ok {
			unlock := op(clientproto.RPCUsrLandUnlockLand.String(), goal, "unlock", "有可开垦土地", 7600, landID, 0, 0)
			unlock.GoldCost = goldCost
			ops = append(ops, unlock)
		}
	}
	if planting.GetUseSpeedUpTicket() {
		if lands, count := speedUpCandidates(s, now); count > 0 {
			speed := op(clientproto.RPCUsrLandSpeedUpBatch.String(), goal, "speed_up", "存在可加速土地", 7400, 0, 0, count)
			speed.LandIDs = lands
			speed.ItemCost = map[int32]int32{1001: count}
			ops = append(ops, speed)
		}
	}
	if cultivate.GetEnabled() || cultivate.GetUpgradeEnabled() {
		if cultivate, ok := cultivateOperation(s, plant, ledger, now); ok {
			ops = append(ops, cultivate)
		}
	}
	return ops
}

func blockedUnknownOperations(policy *pb.Policy) []PlannedOp {
	var ops []PlannedOp
	add := func(enabled bool, category, domain, label string) {
		if !enabled {
			return
		}
		op := markerOp(category, domain, "blocked", "协议或状态不明确，已按计划阻塞", 100)
		op.Label = label
		op.Status = PlanStatusAdapterMissing
		op.Executable = false
		op.BlockedReasons = []string{"该领域尚未完成协议确认，先记录文档，不自动执行"}
		ops = append(ops, op)
	}
	union := policy.GetUnion()
	unionFlower := union.GetFlower()
	unionRace := union.GetRace()
	unionLand := union.GetLand()
	add(unionFlower.GetShareEnabled() || unionRace.GetEnabled() ||
		unionLand.GetAutoPlantEnabled() ||
		union.GetRedPacketEnabled(), CategoryUnion, "union.unknown", "公会扩展功能")
	if policy.GetActivity().GetEnabled() {
		for name, module := range policy.GetActivity().GetModules() {
			if module != nil && module.GetEnabled() {
				add(true, CategoryActivity, "activity."+name, "活动 "+name)
			}
		}
	}
	return ops
}

func FlowerArtAvailability(s *state.State, artID, count int32, ledger *InventoryLedger) ArtAvailability {
	return FlowerArtAvailabilityWithAllocated(s, artID, count, ledger, nil)
}

func FlowerArtAvailabilityWithAllocated(s *state.State, artID, count int32, ledger *InventoryLedger, allocated map[int32]int32) ArtAvailability {
	recipe, ok := state.FlowerArtRecipeByID(artID)
	if !ok {
		return ArtAvailability{BlockedReasons: []string{"缺少花艺配方"}}
	}
	availability := ArtAvailability{Recipe: recipe}
	availability.VaseUnlocked = s.HasVase(recipe.VaseID)
	if !s.VaseObserved() {
		availability.BlockedReasons = append(availability.BlockedReasons, "未观察到花瓶状态 namespace 102")
	} else if !availability.VaseUnlocked {
		availability.BlockedReasons = append(availability.BlockedReasons, fmt.Sprintf("花瓶 #%d 未解锁", recipe.VaseID))
	}
	if ledger == nil {
		ledger = NewInventoryLedger(s.Inventory())
	}
	for flowerID, needEach := range recipeFlowerCounts(recipe) {
		required := needEach * count
		have := ledger.Owned(flowerID)
		available := ledger.Available(flowerID) + allocated[flowerID]
		missing := required - available
		if missing < 0 {
			missing = 0
		}
		demand := Demand{
			ID:        demandID("flower_art.availability", strconv.FormatInt(int64(artID), 10), "availability", DemandKindFlower, flowerID),
			Kind:      DemandKindFlower,
			ItemID:    flowerID,
			Count:     required,
			Have:      have,
			Available: available,
			Missing:   missing,
			Label:     fmt.Sprintf("制作 %s", itemLabel(artID)),
			Priority:  0,
		}
		if missing > 0 {
			demand.BlockedReasons = append(demand.BlockedReasons, "花朵库存不足")
			availability.BlockedReasons = append(availability.BlockedReasons, fmt.Sprintf("%s 缺少 %d", itemLabel(flowerID), missing))
		}
		if !flowerCultivated(s, flowerID) {
			demand.BlockedReasons = append(demand.BlockedReasons, "花朵尚未培育/解锁")
			availability.BlockedReasons = append(availability.BlockedReasons, fmt.Sprintf("%s 尚未培育/解锁", itemLabel(flowerID)))
		}
		availability.Requirements = append(availability.Requirements, demand)
	}
	availability.Craftable = len(availability.BlockedReasons) == 0
	return availability
}

func artBlockedReasons(s *state.State, recipe state.FlowerArtRecipe) []string {
	var blocked []string
	if !s.VaseObserved() {
		blocked = append(blocked, "未观察到花瓶状态 namespace 102")
	} else if !s.HasVase(recipe.VaseID) {
		blocked = append(blocked, fmt.Sprintf("花瓶 #%d 未解锁", recipe.VaseID))
	}
	for flowerID := range recipeFlowerCounts(recipe) {
		if !flowerCultivated(s, flowerID) {
			blocked = append(blocked, fmt.Sprintf("%s 尚未培育/解锁", itemLabel(flowerID)))
		}
	}
	return blocked
}

func residentFlowerOrderAllowed(order *state.FlowerOrder, policy *pb.ResidentOrderPolicy) bool {
	if order == nil || len(order.Requires) == 0 {
		return false
	}
	qualities := int32Set(policy.GetQualities())
	if len(qualities) == 0 {
		return true
	}
	for _, req := range order.Requires {
		if req.FlowerID <= 0 {
			continue
		}
		if !qualities[flowerQuality(req.FlowerID)] {
			return false
		}
	}
	return true
}

func palaceOrderAllowed(order state.PalaceOrderView, policy *pb.PalaceOrderPolicy) bool {
	if !order.Observed || order.IsFinish != 0 || order.FlowerID <= 0 || order.Num <= 0 {
		return false
	}
	qualities := int32Set(policy.GetQualities())
	return len(qualities) == 0 || qualities[flowerQuality(order.FlowerID)]
}

func teamOrderAllowed(order state.TeamOrderView, policy *pb.TeamOrderPolicy, s *state.State) bool {
	if !order.Observed || order.FlowerID <= 0 || teamOrderNeedCount(order) <= 0 {
		return false
	}
	qualities := int32Set(policy.GetQualities())
	if len(qualities) > 0 && !qualities[flowerQuality(order.FlowerID)] {
		return false
	}
	if policy.GetSubmitOnlyCultivated() && !flowerCultivated(s, order.FlowerID) {
		return false
	}
	return true
}

func teamOrderNeedCount(order state.TeamOrderView) int32 {
	if order.RemainingNum > 0 {
		return order.RemainingNum
	}
	return order.OrderNum
}

func flowerCultivated(s *state.State, flowerID int32) bool {
	for id, cv := range s.Cultivations() {
		if id == flowerID && cv.Status == 2 && cv.Lvl > 0 {
			return true
		}
	}
	return false
}

func customerOrderUnavailableReasons(s *state.State, order *state.CustomerOrder) (rejectable, blocked []string) {
	if order == nil {
		return nil, []string{"顾客订单状态缺失"}
	}
	hasRequirements := false
	addReject := func(reason string) {
		if reason != "" && !containsString(rejectable, reason) {
			rejectable = append(rejectable, reason)
		}
	}
	addBlocked := func(reason string) {
		if reason != "" && !containsString(blocked, reason) {
			blocked = append(blocked, reason)
		}
	}
	for _, req := range order.Requires {
		if req.FlowerID <= 0 || req.Count <= 0 {
			addBlocked("顾客订单缺少可识别花朵需求")
			continue
		}
		hasRequirements = true
		if !flowerCultivated(s, req.FlowerID) {
			addReject(fmt.Sprintf("%s 尚未培育/解锁", itemLabel(req.FlowerID)))
		}
	}
	for _, req := range order.ItemRequires {
		if req.ItemID <= 0 || req.Count <= 0 {
			addBlocked("顾客订单缺少可识别花艺需求")
			continue
		}
		hasRequirements = true
		recipe, ok := state.FlowerArtRecipeByID(req.ItemID)
		if !ok {
			addBlocked(fmt.Sprintf("%s 缺少花艺配方", itemLabel(req.ItemID)))
			continue
		}
		if !s.VaseObserved() {
			addBlocked("未观察到花瓶状态 namespace 102")
		} else if !s.HasVase(recipe.VaseID) {
			addReject(fmt.Sprintf("花瓶 #%d 未解锁", recipe.VaseID))
		}
		for flowerID := range recipeFlowerCounts(recipe) {
			if !flowerCultivated(s, flowerID) {
				addReject(fmt.Sprintf("%s 尚未培育/解锁", itemLabel(flowerID)))
			}
		}
	}
	if !hasRequirements {
		addBlocked("顾客订单缺少可识别需求")
	}
	return rejectable, blocked
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func syncOnlyOperation(op PlannedOp, reasons ...string) PlannedOp {
	op.Status = PlanStatusSyncOnly
	op.SyncOnly = true
	op.Executable = false
	op.BlockedReasons = append(op.BlockedReasons, reasons...)
	op.BlockingStage = inferBlockingStage(op.BlockedReasons)
	return op
}

func residentOrderLimitBlock(s *state.State, policy *pb.ResidentOrderPolicy, goal Goal) (PlannedOp, bool) {
	limit := policy.GetNormalDailyLimit()
	if limit <= 0 {
		blocked := markerOp(CategoryOrder, "order.resident", "finish", "居民订单普通订单上限未设置", goal.Priority*100+690)
		blocked.GoalID = goal.ID
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.BlockedReasons = []string{"普通居民订单上限必须大于 0"}
		return blocked, true
	}
	stats := s.Statistics()
	if stats.Observed && stats.OrderFlowerFinishNum >= limit {
		blocked := markerOp(CategoryOrder, "order.resident", "finish", "居民订单今日上限已达", goal.Priority*100+690)
		blocked.GoalID = goal.ID
		blocked.Status = PlanStatusBlocked
		blocked.Executable = false
		blocked.BlockedReasons = []string{fmt.Sprintf("普通居民订单今日已完成 %d/%d", stats.OrderFlowerFinishNum, limit)}
		return blocked, true
	}
	return PlannedOp{}, false
}

func residentSpecialOrderBlockedOp(domain, label string, order state.ResidentSpecialOrder, finished, limit int32, goal Goal) PlannedOp {
	blocked := markerOp(CategoryOrder, domain, "finish", label+"暂不执行", goal.Priority*100+130)
	blocked.GoalID = goal.ID
	blocked.Status = PlanStatusAdapterMissing
	blocked.Executable = false
	switch {
	case limit <= 0:
		blocked.BlockedReasons = []string{label + "日上限必须大于 0"}
		blocked.Status = PlanStatusBlocked
	case finished >= limit:
		blocked.BlockedReasons = []string{fmt.Sprintf("%s今日已完成 %d/%d", label, finished, limit)}
		blocked.Status = PlanStatusBlocked
	case !order.Observed:
		blocked.BlockedReasons = []string{label + "状态未观察到，无法确认需求和次数"}
	default:
		blocked.BlockedReasons = []string{label + "已观察到状态，但协议仅暴露聚合 flowers 字段，缺少可安全提交的花朵需求列表"}
	}
	return blocked
}

func blockedResidentOrderOp(order *state.FlowerOrder, boxID int32, goal Goal, reason string) PlannedOp {
	blocked := op(clientproto.RPCOrderFlowerFinishOrder.String(), goal, "finish", reason, goal.Priority*100+120, boxID, 0, 0)
	blocked.Status = PlanStatusBlocked
	blocked.Executable = false
	if order == nil || len(order.Requires) == 0 {
		blocked.BlockedReasons = []string{"居民订单缺少可识别需求"}
		return blocked
	}
	var reasons []string
	for _, req := range order.Requires {
		quality := flowerQuality(req.FlowerID)
		reasons = append(reasons, fmt.Sprintf("%s 品质 %d 不在策略范围内", itemLabel(req.FlowerID), quality))
	}
	blocked.BlockedReasons = reasons
	return blocked
}

func blockedPalaceOrderOp(order state.PalaceOrderView, goal Goal, policy *pb.PalaceOrderPolicy) PlannedOp {
	blocked := op(clientproto.RPCOrderPalaceFinishOrder.String(), goal, "finish", "宫廷订单暂不可提交", goal.Priority*100+120, 0, order.FlowerID, order.Num)
	blocked.Status = PlanStatusBlocked
	blocked.Executable = false
	blocked.BlockedReasons = orderPolicyBlockedReasons(order.Observed, order.FlowerID, order.Num, policy.GetQualities(), false)
	if len(blocked.BlockedReasons) == 0 {
		blocked.BlockedReasons = []string{"宫廷订单状态显示已完成或缺少可识别需求"}
	}
	return blocked
}

func blockedTeamOrderOp(order state.TeamOrderView, goal Goal, policy *pb.TeamOrderPolicy, s *state.State) PlannedOp {
	count := teamOrderNeedCount(order)
	blocked := op(clientproto.RPCOrderTeamSubmitOrder.String(), goal, "submit", "组团订单暂不可提交", goal.Priority*100+120, 0, order.FlowerID, count)
	blocked.Status = PlanStatusBlocked
	blocked.Executable = false
	blocked.BlockedReasons = orderPolicyBlockedReasons(order.Observed, order.FlowerID, count, policy.GetQualities(), policy.GetSubmitOnlyCultivated() && !flowerCultivated(s, order.FlowerID))
	if len(blocked.BlockedReasons) == 0 {
		blocked.BlockedReasons = []string{"组团订单缺少可识别需求"}
	}
	return blocked
}

func orderPolicyBlockedReasons(observed bool, flowerID, count int32, qualities []int32, notCultivated bool) []string {
	var reasons []string
	if !observed {
		reasons = append(reasons, "订单状态未观察到")
	}
	if flowerID <= 0 || count <= 0 {
		reasons = append(reasons, "订单缺少可识别花朵需求")
	}
	if len(qualities) > 0 && flowerID > 0 && !int32Set(qualities)[flowerQuality(flowerID)] {
		reasons = append(reasons, fmt.Sprintf("%s 品质 %d 不在策略范围内", itemLabel(flowerID), flowerQuality(flowerID)))
	}
	if notCultivated {
		reasons = append(reasons, fmt.Sprintf("%s 未达到已培育可提交条件", itemLabel(flowerID)))
	}
	return reasons
}

func canFulfillSingleFlowerDemand(goal Goal, entityID string, flowerID, count int32, ledger *InventoryLedger) bool {
	if flowerID <= 0 || count <= 0 {
		return false
	}
	id := demandID(goal.ID, entityID, "direct", DemandKindFlower, flowerID)
	return ledger.AllocatedForDemand(id, flowerID) >= count
}

func recipeFlowerCounts(recipe state.FlowerArtRecipe) map[int32]int32 {
	out := make(map[int32]int32)
	for _, flowerID := range recipe.Flowers {
		if flowerID > 0 {
			out[flowerID]++
		}
	}
	return out
}

func maxCraftableCount(recipe state.FlowerArtRecipe, ledger *InventoryLedger) int32 {
	if recipe.ArtID <= 0 || ledger == nil {
		return 0
	}
	var max int32
	for flowerID, needEach := range recipeFlowerCounts(recipe) {
		if needEach <= 0 {
			continue
		}
		available := ledger.Available(flowerID) / needEach
		if max == 0 || available < max {
			max = available
		}
	}
	return max
}

func bestRackArt(ledger *InventoryLedger) (int32, int32, bool) {
	if ledger == nil {
		return 0, 0, false
	}
	for _, recipe := range rackCandidateRecipes() {
		available := ledger.Available(recipe.ArtID)
		if available <= 0 {
			continue
		}
		if available > flowerRackPerSlotCount {
			available = flowerRackPerSlotCount
		}
		return recipe.ArtID, available, true
	}
	return 0, 0, false
}

func rackCraftTarget(s *state.State, policy *pb.FlowerArtPolicy, ledger *InventoryLedger) (int32, int32, bool) {
	if policy == nil || !policy.GetSellEnabled() || !policy.GetCraftEnabled() {
		return 0, 0, false
	}
	if len(s.EmptyFlowerRackSlotIDs()) == 0 {
		return 0, 0, false
	}
	if ledger == nil {
		ledger = NewInventoryLedger(s.Inventory())
	}
	for _, recipe := range rackCandidateRecipes() {
		if ledger.Available(recipe.ArtID) > 0 {
			return 0, 0, false
		}
	}
	for _, recipe := range rackCandidateRecipes() {
		if len(artBlockedReasons(s, recipe)) > 0 {
			continue
		}
		count := maxCraftableCount(recipe, ledger)
		if count <= 0 {
			continue
		}
		if count > flowerRackPerSlotCount {
			count = flowerRackPerSlotCount
		}
		if count > 0 {
			return recipe.ArtID, count, true
		}
	}
	return 0, 0, false
}

func rackCandidateRecipes() []state.FlowerArtRecipe {
	return state.AllFlowerArtRecipes()
}

func canFulfillFlowerOrder(order *state.FlowerOrder, boxID int32, goal Goal, ledger *InventoryLedger) bool {
	if order == nil || len(order.Requires) == 0 {
		return false
	}
	entityID := strconv.FormatInt(int64(boxID), 10)
	for _, req := range order.Requires {
		id := demandID(goal.ID, entityID, "direct", DemandKindFlower, req.FlowerID)
		if req.FlowerID == 0 || req.Count <= 0 || ledger.AllocatedForDemand(id, req.FlowerID) < req.Count {
			return false
		}
	}
	return true
}

func canFulfillCustomerOrder(order *state.CustomerOrder, npcID int32, goal Goal, ledger *InventoryLedger) bool {
	if order == nil {
		return false
	}
	hasRequirements := false
	entityID := strconv.FormatInt(int64(npcID), 10)
	for _, req := range order.Requires {
		if req.FlowerID == 0 || req.Count <= 0 {
			continue
		}
		hasRequirements = true
		id := demandID(goal.ID, entityID, "direct", DemandKindFlower, req.FlowerID)
		if ledger.AllocatedForDemand(id, req.FlowerID) < req.Count {
			return false
		}
	}
	for _, req := range order.ItemRequires {
		if req.ItemID == 0 || req.Count <= 0 {
			continue
		}
		hasRequirements = true
		id := demandID(goal.ID, entityID, "direct", DemandKindFlowerArt, req.ItemID)
		if ledger.AllocatedForDemand(id, req.ItemID) < req.Count {
			return false
		}
	}
	return hasRequirements
}

func speedUpCandidates(s *state.State, now time.Time) ([]int32, int32) {
	available := s.Inventory()[1001]
	if available <= 0 {
		return nil, 0
	}
	var ids []int32
	for id, land := range s.Lands() {
		if land.State == 2 && land.NextTimeMs > now.UnixMilli() {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	want := int32(len(ids))
	if want > available {
		want = available
	}
	if want > 5 {
		want = 5
	}
	return ids[:want], want
}

func cultivateOperation(s *state.State, policy *pb.PlantPolicy, ledger *InventoryLedger, now time.Time) (PlannedOp, bool) {
	goal := Goal{ID: "farm.cultivate", Category: CategoryPlant, Domain: "farm.cultivate", Label: "培育", Priority: 55}
	cultivatePolicy := policy.GetCultivate()
	cultivations := s.Cultivations()
	ids := make([]int32, 0, len(cultivations))
	for id := range cultivations {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if cultivatePolicy.GetEnabled() {
		nowMs := now.UnixMilli()
		for _, id := range ids {
			cv := cultivations[id]
			if cv.Status == 1 && cv.CulTimeMs > 0 && cv.CulTimeMs <= nowMs {
				op := op(clientproto.RPCCultivateRecv.String(), goal, "recv", "培育完成可领取", 7200, 0, 0, 0)
				op.FlowerID = id
				return op, true
			}
		}
	}
	if cultivatePolicy.GetUpgradeEnabled() {
		targetLevel := cultivatePolicy.GetTargetLevel()
		for _, id := range ids {
			cv := cultivations[id]
			if cv.Status != 2 || cv.Lvl <= 0 {
				continue
			}
			if targetLevel > 0 && cv.Lvl >= targetLevel {
				continue
			}
			cost, ok := state.FlowerUpgradeCostForLevel(id, cv.Lvl)
			if !ok || s.Inventory()[cost.ItemID] < cost.Count || s.Gold() < cost.Gold {
				continue
			}
			op := op(clientproto.RPCCultivateUpgrade.String(), goal, "upgrade", "鲜花培育等级可升级", 7100, 0, 0, 0)
			op.FlowerID = id
			op.GoldCost = cost.Gold
			op.ItemCost = map[int32]int32{cost.ItemID: cost.Count}
			return op, true
		}
	}
	if cultivatePolicy.GetEnabled() {
		for _, flower := range s.PlantableFlowers(nil, nil) {
			if _, exists := cultivations[flower.FlowerID]; exists {
				continue
			}
			costs, ok := state.CultivateCost(flower.FlowerID)
			if !ok {
				blocked := op(clientproto.RPCCultivateCultivate.String(), goal, "cultivate", "培育材料配置未确认", 7050, 0, 0, 0)
				blocked.FlowerID = flower.FlowerID
				blocked.Status = PlanStatusAdapterMissing
				blocked.Executable = false
				blocked.BlockedReasons = []string{"缺少培育材料静态配置，已阻塞等待确认"}
				return blocked, true
			}
			itemCost := map[int32]int32{}
			for _, cost := range costs {
				if cost.ItemID > 0 && cost.Count > 0 {
					itemCost[cost.ItemID] += cost.Count
				}
			}
			if !ledger.CanSpendItems(itemCost) {
				continue
			}
			op := op(clientproto.RPCCultivateCultivate.String(), goal, "cultivate", "有未培育花朵", 7050, 0, 0, 0)
			op.FlowerID = flower.FlowerID
			op.ItemCost = itemCost
			return op, true
		}
	}
	return PlannedOp{}, false
}

func nextLandUnlockCandidate(st *state.State) (int32, int32, bool) {
	if !st.LandRosterObserved() || !st.FarmLandConfigObserved() {
		return 0, 0, false
	}
	opened := st.Lands()
	reclaimable := 0
	for _, land := range st.FarmLands() {
		if _, ok := opened[land.ID]; ok {
			continue
		}
		reclaimable++
		if reclaimable > 6 {
			break
		}
		if len(land.Cost) < 2 {
			continue
		}
		actualCost := land.Cost[1] - land.Cost[0] + 11
		if st.Level() >= land.OpenLevel && st.Gold() >= actualCost {
			return land.ID, actualCost, true
		}
	}
	return 0, 0, false
}

func landOp(kind, domain, action, reason string, priority int32, landIDs []int32, flowerID int32, goalID, demandID string) PlannedOp {
	op := PlannedOp{
		OperationID: operationID(kind, landIDs, flowerID, 0, 0),
		GoalID:      goalID,
		DemandID:    demandID,
		Kind:        kind,
		Category:    CategoryPlant,
		Domain:      domain,
		Action:      action,
		Reason:      reason,
		Priority:    priority,
		LandIDs:     append([]int32(nil), landIDs...),
		FlowerID:    flowerID,
	}
	return enrichPlannedOp(op)
}

func markerOp(category, domain, action, reason string, priority int32) PlannedOp {
	op := PlannedOp{
		OperationID: operationID(domain+"."+action, nil, 0, 0, 0),
		Kind:        domain + "." + action,
		Category:    category,
		Domain:      domain,
		Action:      action,
		Reason:      reason,
		Priority:    priority,
	}
	return enrichPlannedOp(op)
}

func op(kind string, goal Goal, action, reason string, priority, targetID, itemID, count int32) PlannedOp {
	out := PlannedOp{
		OperationID: operationID(kind, nil, 0, targetID, itemID),
		GoalID:      goal.ID,
		Kind:        kind,
		Category:    goal.Category,
		Domain:      goal.Domain,
		Action:      action,
		Reason:      reason,
		Priority:    priority,
		TargetID:    targetID,
		ItemID:      itemID,
		Count:       count,
	}
	return enrichPlannedOp(out)
}

func domainOp(kind string, goal Goal, domain, action, reason string, priority, targetID, itemID, count int32) PlannedOp {
	out := PlannedOp{
		OperationID: operationID(kind, nil, 0, targetID, itemID),
		GoalID:      goal.ID,
		Kind:        kind,
		Category:    goal.Category,
		Domain:      domain,
		Action:      action,
		Reason:      reason,
		Priority:    priority,
		TargetID:    targetID,
		ItemID:      itemID,
		Count:       count,
	}
	return enrichPlannedOp(out)
}

func sortOperations(ops []PlannedOp) {
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Priority != ops[j].Priority {
			return ops[i].Priority > ops[j].Priority
		}
		if ops[i].Category != ops[j].Category {
			return categoryRank(ops[i].Category) < categoryRank(ops[j].Category)
		}
		if ops[i].Domain != ops[j].Domain {
			return ops[i].Domain < ops[j].Domain
		}
		return ops[i].OperationID < ops[j].OperationID
	})
}

func categoryRank(category string) int {
	switch category {
	case CategoryPlant:
		return 0
	case CategoryOrder:
		return 1
	case CategoryBasic:
		return 2
	case CategoryUnion:
		return 3
	case CategoryActivity:
		return 4
	case CategoryAccount:
		return 5
	default:
		return 9
	}
}

func int32Set(ids []int32) map[int32]bool {
	if len(ids) == 0 {
		return nil
	}
	out := make(map[int32]bool, len(ids))
	for _, id := range ids {
		if id > 0 {
			out[id] = true
		}
	}
	return out
}

func DefaultPolicy() *pb.Policy {
	return &pb.Policy{
		AutomationEnabled: false,
		Basic: &pb.BasicPolicy{
			Reputation: &pb.ReputationPolicy{Enabled: true, Threshold: 80},
			Task:       &pb.BasicTaskPolicy{},
			Benefit:    &pb.BenefitPolicy{},
			Sign:       &pb.SignPolicy{},
			Pearl:      &pb.PearlPolicy{},
			Shop: &pb.ShopPolicy{
				CultivateShop: &pb.ShopBuyPolicy{},
				VipShop:       &pb.VipShopPolicy{},
			},
			FeedCat: &pb.FeedCatPolicy{},
		},
		Plant: &pb.PlantPolicy{
			Cultivate: &pb.CultivatePolicy{
				TargetLevel: 20,
			},
			Planting: &pb.PlantingPolicy{
				AutoEnabled:     true,
				GoalPriority:    defaultGoalPriority(),
				MinWaterDrops:   5,
				AutoReplantMode: pb.SelectionMode_SELECTION_MODE_ALL,
			},
			FriendSteal: &pb.FriendStealPolicy{},
			Elves:       &pb.FlowerElvesPolicy{},
			Market: &pb.FlowerMarketPolicy{
				PutMode:    pb.MarketPutMode_MARKET_PUT_MODE_INVENTORY,
				BuyMode:    pb.MarketBuyMode_MARKET_BUY_MODE_ALL,
				PriceIndex: 2,
				MaxSell:    25,
			},
		},
		Order: &pb.OrderPolicy{
			Customer:  &pb.CustomerOrderPolicy{},
			Resident:  &pb.ResidentOrderPolicy{NormalDailyLimit: 1200, DecorateDailyLimit: 120, SatinDailyLimit: 120},
			Palace:    &pb.PalaceOrderPolicy{},
			Team:      &pb.TeamOrderPolicy{},
			FlowerArt: &pb.FlowerArtPolicy{},
		},
		Union: &pb.UnionPolicy{
			Build:  &pb.UnionBuildPolicy{},
			Flower: &pb.UnionFlowerPolicy{},
			Race:   &pb.UnionRacePolicy{TaskTypePriority: defaultUnionRacePriority()},
			Land:   &pb.UnionLandPolicy{},
		},
		Activity:                &pb.ActivityPolicy{},
		DecisionIntervalSeconds: 4,
	}
}

func DefaultPolicyIfNil(p *pb.Policy) *pb.Policy {
	if p == nil {
		return DefaultPolicy()
	}
	return p
}

func defaultGoalPriority() map[string]int32 {
	return map[string]int32{
		GoalCustomerOrder: 90,
		GoalResidentOrder: 80,
		GoalMainTask:      70,
		GoalDailyTask:     60,
		GoalWeeklyTask:    55,
		GoalFlowerArt:     40,
		GoalAutoReplant:   10,
	}
}

func defaultUnionRacePriority() map[int32]int32 {
	return map[int32]int32{
		2004: 0,
		3006: 2,
		3016: 2,
		3017: 3,
		3018: 2,
		3023: 3,
		3024: 3,
		3030: 2,
		3034: 2,
		3035: 3,
		3036: 1,
		3044: 3,
		3052: 3,
	}
}

func demandByID(demands []Demand, id string) (Demand, bool) {
	for _, demand := range demands {
		if demand.ID == id {
			return demand, true
		}
	}
	return Demand{}, false
}

func demandID(goalID, entityID, source, kind string, itemID int32) string {
	return goalID + ":" + entityID + ":" + source + ":" + kind + ":" + strconv.FormatInt(int64(itemID), 10)
}

func operationID(kind string, landIDs []int32, flowerID, targetID, itemID int32) string {
	parts := []string{kind}
	if targetID != 0 {
		parts = append(parts, "target="+strconv.FormatInt(int64(targetID), 10))
	}
	if itemID != 0 {
		parts = append(parts, "item="+strconv.FormatInt(int64(itemID), 10))
	}
	if flowerID != 0 {
		parts = append(parts, "flower="+strconv.FormatInt(int64(flowerID), 10))
	}
	if len(landIDs) > 0 {
		ids := make([]string, 0, len(landIDs))
		for _, id := range landIDs {
			ids = append(ids, strconv.FormatInt(int64(id), 10))
		}
		parts = append(parts, "lands="+strings.Join(ids, ","))
	}
	return strings.Join(parts, "|")
}

func itemLabel(itemID int32) string {
	if name := state.ItemName(itemID); name != "" {
		return name
	}
	return fmt.Sprintf("#%d", itemID)
}
