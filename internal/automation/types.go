package automation

import (
	"time"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	CategoryAccount   = "account"
	CategoryBasic     = "basic"
	CategoryPlant     = "plant"
	CategoryOrder     = "order"
	CategoryFlowerArt = "flower_art"
	CategoryWater     = "water"
	CategoryUnion     = "union"
	CategoryRace      = "race"
	CategoryActivity  = "activity"
	CategoryRedeem    = "redeem"
	CategorySystem    = "system"
)

const (
	LaneFarm = "farm"
	LaneSide = "side"
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
	OperationID string
	// CooldownKey optionally groups multiple concrete targets into one
	// runner-local retry scope. It is intentionally not exposed in protobuf.
	CooldownKey string
	GoalID      string
	DemandID    string
	Kind        string
	Lane        string
	FeatureID   string
	Category    string
	Label       string
	Domain      string
	Action      string
	Status      string
	Executable  bool
	SyncOnly    bool
	// PreemptFarm ranks this op above the farm lane (login race sync/take,
	// finish, giveUp). Side-lane locking is unchanged.
	PreemptFarm    bool
	Reason         string
	BlockedReasons []string
	Priority       int32
	LandIDs        []int32
	SlotIDs        []int32
	FlowerID       int32
	TargetUID      int64
	TargetUIDs     []int64
	BatchID        int32
	SlotID         int32
	TaskID         int32
	TaskMsID       int64
	MilestoneIndex int32
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
	CooldownUntil  time.Time
	CooldownReason string
	// RaceTaskGuard captures the exact pool facts that authorized a race take
	// upgrade or automatic deletion. The runner refreshes the authoritative pool and
	// revalidates this guard immediately before sending the mutating RPC.
	RaceTaskGuard *RaceTaskMutationGuard
	RaceBatchID   int64
}

// RaceTaskMutationGuard is runner-only decision evidence for a guild-race
// pool mutation. It is intentionally not part of the public workspace schema.
type RaceTaskMutationGuard struct {
	AutomaticDelete bool
	Planned         RaceTaskMutationFacts
	Current         RaceTaskMutationFacts
}

// RaceTaskMutationFacts are the mutable fields that can invalidate a task
// choice between planning and execution.
type RaceTaskMutationFacts struct {
	MsID       int64
	TaskID     int32
	TaskType   int32
	Score      int32
	IsUpgrade  int32
	UpgradeUID int64
	UID        int64
	ParamID    int32
	FinishCnt  int32
}
