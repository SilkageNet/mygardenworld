package automation

import (
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const stablePlanAttempts = 3

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
	for attempt := 0; attempt < stablePlanAttempts; attempt++ {
		revision := s.Revision()
		result := buildPlanAtRevision(s, policy, now)
		if revision == s.Revision() {
			return result
		}
	}
	// A continuously changing state must not produce a queue assembled from
	// multiple server generations. The next runner tick/query will retry.
	return PlanResult{}
}

func buildPlanAtRevision(s *state.State, policy *pb.Policy, now time.Time) PlanResult {
	policy = DefaultPolicyIfNil(policy)
	goals := enabledGoals(policy)
	ledger := NewInventoryLedger(s.Inventory())
	demands := buildDirectDemands(s, policy, goals, now)
	applyLedgerAllocations(demands, ledger)
	production := buildProductionDemands(s, policy, goals, demands, ledger)
	applyLedgerAllocations(production, ledger)
	demands = append(demands, production...)
	activityActions := cyclicNoteTaskActionDemands(s, policy, now)
	for _, action := range activityActions {
		demands = append(demands, action.Demand)
	}
	dailyActions := dailyTaskActionDemands(s, policy)
	for _, action := range dailyActions {
		demands = append(demands, action.Demand)
	}
	// Race progress demands use FinishCnt as Have so they must skip the
	// inventory ledger (inventory stock does not satisfy harvest counts).
	demands = append(demands, raceTaskProgressDemands(s, policy, now)...)
	annotateDemandStatuses(demands)
	sortDemands(demands)
	ops := buildOperations(s, policy, goals, demands, activityActions, dailyActions, ledger, now)
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

func buildOperations(s *state.State, policy *pb.Policy, goals []Goal, demands []Demand, activityActions []cyclicNoteTaskActionDemand, dailyActions []dailyTaskActionDemand, ledger *InventoryLedger, now time.Time) []PlannedOp {
	var ops []PlannedOp
	ops = append(ops, farmOps(s, policy.GetPlant(), demands, now, raceSuppressesAutoReplant(s, policy, now))...)
	ops = append(ops, friendTouchOperations(s, policy.GetPlant().GetFriendSteal(), now)...)
	ops = append(ops, orderOperations(s, policy, goals, demands, ledger, now)...)
	ops = append(ops, basicOperations(s, policy, goals, now)...)
	ops = append(ops, shopOperations(s, policy, now)...)
	ops = append(ops, maintenanceOperations(s, policy, ledger, now)...)
	ops = append(ops, unionOperations(s, policy, now)...)
	ops = driveCyclicNoteTaskOperations(policy, activityActions, ledger, ops)
	ops = driveRaceCustomerOrderOperations(policy, demands, ops)
	ops = driveRacePearlHireOperations(policy, demands, ops)
	ops = driveRaceFlowerArtSellOperations(s, policy, demands, ops, ledger, now)
	ops = driveRaceFlowerArtCraftOperations(s, policy, demands, ops, ledger)
	ops = driveDailyTaskOperations(dailyActions, ops)
	ops = append(ops, activityOperations(s, policy.GetActivity(), now)...)
	ops = append(ops, blockedUnknownOperations(policy)...)
	return ops
}
