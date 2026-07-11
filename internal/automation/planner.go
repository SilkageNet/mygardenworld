package automation

import (
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"time"
)

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

func buildOperations(s *state.State, policy *pb.Policy, goals []Goal, demands []Demand, ledger *InventoryLedger, now time.Time) []PlannedOp {
	var ops []PlannedOp
	ops = append(ops, farmOps(s, policy.GetPlant(), demands, now)...)
	ops = append(ops, orderOperations(s, policy, goals, demands, ledger, now)...)
	ops = append(ops, basicOperations(s, policy, goals, now)...)
	ops = append(ops, shopOperations(s, policy)...)
	ops = append(ops, maintenanceOperations(s, policy, ledger, now)...)
	ops = append(ops, unionOperations(s, policy.GetUnion())...)
	ops = append(ops, activityOperations(s, policy.GetActivity(), now)...)
	ops = append(ops, blockedUnknownOperations(policy)...)
	return ops
}
