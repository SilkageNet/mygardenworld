package automation

import (
	"fmt"
	"math"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"google.golang.org/protobuf/proto"
)

// CyclicNoteTaskDrive freezes the server task authorizing a business action.
// FinishCount distinguishes replacement tasks that reuse a slot/task ID.
type CyclicNoteTaskDrive struct {
	BatchID, SlotID, TaskID, TaskType int32
	Progress, FinishCount             int32
}

// scopedCyclicNoteOperations fills gaps that ordinary business policy cannot
// execute. The temporary policy is used ONLY to generate this task's next
// allowed RPC; it is never published, persisted or fed to the general planner.
func scopedCyclicNoteOperations(s *state.State, policy *pb.Policy, actions []cyclicNoteTaskActionDemand, ledger *InventoryLedger, ops []PlannedOp, now time.Time) []PlannedOp {
	for _, action := range actions {
		linked := false
		for i := range ops {
			// Independently authorized daily/race work can also advance this
			// task; don't duplicate it or attach a revocable activity permit.
			if ops[i].DemandID == action.Demand.ID || (runnableBusinessOperation(ops[i]) && cyclicNoteOperationMatches(action.TaskType, ops[i].Kind)) {
				linked = true
			}
		}
		if linked || (action.TaskType == cyclicNoteTaskTypePlantAny && hasConcretePlantOperation(ops)) {
			continue
		}
		scoped := cyclicNoteScopedPolicy(policy, action.TaskType)
		var candidates []PlannedOp
		switch action.TaskType {
		case cyclicNoteTaskTypePlantAny:
			candidates = scopedCyclicNoteFarmOperations(s, scoped, action.Demand, now)
		case cyclicNoteTaskTypePearlHire:
			if candidate, ok := PlanOneSafePearlHire(s, scoped.GetBasic().GetPearl(), now, PearlHireIntent{
				DemandID: action.Demand.ID, Reason: cyclicNoteDriveReason(action.Demand, ""),
			}); ok {
				candidates = append(candidates, candidate)
			}
		default:
			goalID := GoalResidentOrder
			switch action.TaskType {
			case cyclicNoteTaskTypeCustomerOrder:
				goalID = GoalCustomerOrder
			case cyclicNoteTaskTypeFlowerRack:
				goalID = GoalFlowerArt
			}
			goal, ok := goalByID(enabledGoals(scoped), goalID)
			if !ok {
				continue
			}
			goals := []Goal{goal}
			// Keep existing inventory reservations. Additional task demands may
			// reserve only stock not already assigned to ordinary work.
			demands := buildDirectDemands(s, scoped, goals, now)
			for _, demand := range demands {
				demand.Count -= ledger.AllocatedForDemand(demand.ID, demand.ItemID)
				ledger.Allocate(demand)
			}
			candidates = orderOperations(s, scoped, goals, demands, ledger, now)
			candidates = driveCyclicNoteTaskOperations(scoped, []cyclicNoteTaskActionDemand{action}, ledger, candidates)
		}
		// Import only explicitly bounded task actions, never incidental craft,
		// reject, purchase or reward operations from reused business planners.
		idx := deterministicOperationIndex(candidates, func(op PlannedOp) bool {
			return op.DemandID == action.Demand.ID && cyclicNoteOperationMatches(action.TaskType, op.Kind)
		})
		if idx < 0 {
			continue
		}
		candidate := candidates[idx]
		drive := action.Drive
		candidate.CyclicNoteDrive = &drive
		ops = append(ops, candidate)
	}
	return ops
}

// Seeding leaves a land awaiting its first watering; the live game does not
// advance the planting task at that intermediate step. Finish those lands
// before seeding more. Harvest only when occupied plots prevent further work.
func scopedCyclicNoteFarmOperations(s *state.State, policy *pb.Policy, demand Demand, now time.Time) []PlannedOp {
	policy.Plant.Planting.AutoHarvestEnabled = true // private task policy only
	ops := farmOps(s, policy.GetPlant(), nil, now, false)
	var awaitingWater, empty int
	for _, land := range s.Lands() {
		kind, _ := Recommend(land, now, 0)
		switch kind {
		case KindWater:
			awaitingWater++
		case KindPlant:
			empty++
		}
	}
	if awaitingWater == 0 && empty > 0 {
		return driveCyclicNotePlant(policy, demand, ops)
	}
	for _, op := range ops {
		water := op.Kind == clientproto.RPCUsrLandWater.String() || op.Kind == clientproto.RPCUsrLandWaterBatch.String()
		if (awaitingWater > 0 && !water) || (awaitingWater == 0 && op.Kind != clientproto.RPCUsrLandHarvest.String()) {
			continue
		}
		if !runnableBusinessOperation(op) || len(op.LandIDs) == 0 {
			continue
		}
		op.LandIDs = append([]int32(nil), op.LandIDs[:min(len(op.LandIDs), int(demand.Missing))]...)
		if water && len(op.LandIDs) == 1 {
			op.Kind = clientproto.RPCUsrLandWater.String()
		}
		op.OperationID = operationID(op.Kind, op.LandIDs, op.FlowerID, 0, 0)
		op.DemandID = demand.ID
		op.Reason = cyclicNoteDriveReason(demand, op.Reason)
		return []PlannedOp{op}
	}
	return nil
}

func cyclicNoteScopedPolicy(policy *pb.Policy, taskType int32) *pb.Policy {
	p := proto.Clone(policy).(*pb.Policy)
	if p.Plant == nil {
		p.Plant = &pb.PlantPolicy{}
	}
	if p.Plant.Planting == nil {
		p.Plant.Planting = &pb.PlantingPolicy{}
	}
	if p.Order == nil {
		p.Order = &pb.OrderPolicy{}
	}
	if p.Order.Resident == nil {
		p.Order.Resident = &pb.ResidentOrderPolicy{}
	}
	if p.Order.Customer == nil {
		p.Order.Customer = &pb.CustomerOrderPolicy{}
	}
	if p.Order.FlowerArt == nil {
		p.Order.FlowerArt = &pb.FlowerArtPolicy{}
	}
	if p.Basic == nil {
		p.Basic = &pb.BasicPolicy{}
	}
	if p.Basic.Pearl == nil {
		p.Basic.Pearl = &pb.PearlPolicy{}
	}
	switch taskType {
	case cyclicNoteTaskTypePlantAny:
		p.Plant.Planting.AutoEnabled = true
	case cyclicNoteTaskTypeResidentOrder:
		p.Order.Resident.NormalEnabled = true
		// The regular helper still clamps this to the game's catalog hard cap
		// and checks server-reported daily exhaustion/cooldowns.
		p.Order.Resident.NormalDailyLimit = math.MaxInt32
		p.Order.Resident.SatinEnabled = false
		p.Order.Resident.DecorateEnabled = false
		p.Order.Resident.RewardEnabled = false
	case cyclicNoteTaskTypeCustomerOrder:
		p.Order.Customer.Enabled = true
		p.Order.Customer.DailyLimit = 0
	case cyclicNoteTaskTypeFlowerRack:
		p.Order.FlowerArt.SellEnabled = true
	case cyclicNoteTaskTypePearlHire:
		// Ticket budget, level, occupied slots and uncertain-result lock stay
		// authoritative. The activity only grants permission to try hiring.
		p.Basic.Pearl.AutoHireEnabled = true
	}
	return p
}

func cyclicNoteOperationMatches(taskType int32, kind string) bool {
	switch taskType {
	case cyclicNoteTaskTypePlantAny:
		return isPlantOperation(kind) || kind == clientproto.RPCUsrLandWater.String() ||
			kind == clientproto.RPCUsrLandWaterBatch.String() || kind == clientproto.RPCUsrLandHarvest.String()
	case cyclicNoteTaskTypeResidentOrder:
		return kind == clientproto.RPCOrderFlowerFinishOrder.String()
	case cyclicNoteTaskTypeCustomerOrder:
		return kind == clientproto.RPCOrderCustomerFinishOrder.String()
	case cyclicNoteTaskTypeFlowerRack:
		return kind == clientproto.RPCFlowerRackSell.String()
	case cyclicNoteTaskTypePearlHire:
		return cyclicNotePearlPlannerKind(kind)
	}
	return false
}

// CyclicNoteOperationCount bounds task-sized units per mutation, including
// preparatory seeding/harvest. Discovery reads consume no task quota; the
// runner separately decides which actions need progress confirmation.
func CyclicNoteOperationCount(op *PlannedOp) int32 {
	if op == nil || op.CyclicNoteDrive == nil {
		return 0
	}
	switch op.Kind {
	case clientproto.RPCUsrLandPlant.String(), clientproto.RPCUsrLandPlantBatch.String(),
		clientproto.RPCUsrLandWater.String(), clientproto.RPCUsrLandWaterBatch.String(), clientproto.RPCUsrLandHarvest.String():
		return int32(len(op.LandIDs))
	case clientproto.RPCFlowerRackSell.String():
		return op.Count
	case clientproto.RPCOrderFlowerFinishOrder.String(), clientproto.RPCOrderCustomerFinishOrder.String(), clientproto.RPCPearlPlaceHire.String():
		return 1
	}
	return 0
}

// ValidateCyclicNoteTaskOperation is repeated at the send boundary, after
// pacing. A completed/replaced task or a disabled policy revokes permission.
func ValidateCyclicNoteTaskOperation(s *state.State, policy *pb.Policy, op *PlannedOp, now time.Time) error {
	if op == nil || op.CyclicNoteDrive == nil {
		return nil
	}
	drive := *op.CyclicNoteDrive
	for _, action := range cyclicNoteTaskActionDemands(s, policy, now) {
		if action.Drive != drive {
			continue
		}
		if !cyclicNoteOperationMatches(drive.TaskType, op.Kind) || op.DemandID != action.Demand.ID {
			break
		}
		count := CyclicNoteOperationCount(op)
		if count < 0 || count > action.Demand.Missing || (count == 0 && drive.TaskType != cyclicNoteTaskTypePearlHire) {
			return fmt.Errorf("花笺集芳操作超出任务剩余数量，等待重新规划")
		}
		if drive.TaskType == cyclicNoteTaskTypeResidentOrder {
			p := cyclicNoteScopedPolicy(policy, drive.TaskType)
			if reason, blocked := residentNormalDailyLimitReached(s, p.GetOrder().GetResident(), now); blocked {
				return fmt.Errorf("%s", reason)
			}
		}
		if drive.TaskType == cyclicNoteTaskTypePlantAny && (op.Kind == clientproto.RPCUsrLandWater.String() || op.Kind == clientproto.RPCUsrLandWaterBatch.String()) {
			lands := s.Lands()
			for _, id := range op.LandIDs {
				kind, _ := Recommend(lands[id], now, 0)
				if kind != KindWater {
					return fmt.Errorf("花笺集芳待浇水土地已变化，等待重新规划")
				}
			}
			drops, _, _ := s.AvailableWaterDrops(now)
			if drops-max(0, policy.GetPlant().GetPlanting().GetMinWaterDrops()) < count {
				return fmt.Errorf("花笺集芳浇水需保留配置的水滴数量，等待水滴恢复")
			}
		}
		return nil
	}
	return fmt.Errorf("花笺集芳任务已完成、变更或授权已关闭，取消旧操作")
}

// CyclicNotePearlPolicy grants only the switch override after validating the
// live task. All paid-hire safety policy is retained by the cloned message.
func CyclicNotePearlPolicy(s *state.State, policy *pb.Policy, op *PlannedOp, now time.Time) (*pb.PearlPolicy, error) {
	if op == nil || op.CyclicNoteDrive == nil {
		return policy.GetBasic().GetPearl(), nil
	}
	if err := ValidateCyclicNoteTaskOperation(s, policy, op, now); err != nil {
		return nil, err
	}
	return cyclicNoteScopedPolicy(policy, cyclicNoteTaskTypePearlHire).GetBasic().GetPearl(), nil
}
