package apiserver

import (
	"fmt"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func flowerRequirements(reqs []state.FlowerRequire, inventory map[int32]int32) []*pb.RequirementView {
	out := make([]*pb.RequirementView, 0, len(reqs))
	for _, req := range reqs {
		if req.FlowerID == 0 || req.Count <= 0 {
			continue
		}
		out = append(out, requirementView(req.FlowerID, req.Count, inventory[req.FlowerID]))
	}
	return out
}

func itemRequirements(reqs []state.ItemCount, inventory map[int32]int32) []*pb.RequirementView {
	out := make([]*pb.RequirementView, 0, len(reqs))
	for _, req := range reqs {
		if req.ItemID == 0 || req.Count <= 0 {
			continue
		}
		out = append(out, requirementView(req.ItemID, req.Count, inventory[req.ItemID]))
	}
	return out
}

func customerOrderRequirements(order *state.CustomerOrder, inventory map[int32]int32) []*pb.RequirementView {
	out := flowerRequirements(order.Requires, inventory)
	for _, req := range order.ItemRequires {
		if req.ItemID == 0 || req.Count <= 0 {
			continue
		}
		missingArt := req.Count - inventory[req.ItemID]
		recipe, ok := state.FlowerArtRecipeByID(req.ItemID)
		if !ok {
			out = append(out, requirementView(req.ItemID, req.Count, inventory[req.ItemID]))
			continue
		}
		if missingArt <= 0 {
			out = append(out, requirementView(req.ItemID, req.Count, inventory[req.ItemID]))
			continue
		}
		for _, flowerID := range recipe.Flowers {
			out = append(out, requirementView(flowerID, missingArt, inventory[flowerID]))
		}
	}
	return out
}

func requirementView(itemID, required, owned int32) *pb.RequirementView {
	missing := required - owned
	if missing < 0 {
		missing = 0
	}
	name := state.ItemName(itemID)
	if name == "" {
		name = fmt.Sprintf("#%d", itemID)
	}
	return &pb.RequirementView{
		ItemId:           itemID,
		ItemName:         name,
		Required:         required,
		Owned:            owned,
		Missing:          missing,
		PlantingRelevant: state.IsFlowerItemID(itemID),
	}
}

func requirementsStatus(reqs []*pb.RequirementView) pb.PlanStatus {
	for _, req := range reqs {
		if req.GetMissing() > 0 {
			return pb.PlanStatus_PLAN_STATUS_MANAGED
		}
	}
	return pb.PlanStatus_PLAN_STATUS_READY
}

func plannedOperationsProto(ops []automation.PlannedOp, diag runner.Diagnostics) []*pb.PlannedOperation {
	cooldowns := cooldownsByOperation(diag)
	out := make([]*pb.PlannedOperation, 0, len(ops))
	for _, op := range ops {
		cooldownUntil := op.CooldownUntil
		cooldownReason := op.CooldownReason
		if cd, ok := lookupPlannedOperationCooldown(cooldowns, op); ok {
			cooldownUntil = cd.Until
			cooldownReason = cd.Reason
		}
		out = append(out, &pb.PlannedOperation{
			Category:        op.Category,
			Domain:          op.Domain,
			Action:          op.Action,
			Rpc:             op.Kind,
			Lane:            executionLaneProto(op.Lane),
			Reason:          op.Reason,
			LandIds:         append([]int32(nil), op.LandIDs...),
			FlowerId:        op.FlowerID,
			Priority:        op.Priority,
			GoldCost:        op.GoldCost,
			DiamondCost:     op.DiamondCost,
			ItemCost:        cloneInt32Map(op.ItemCost),
			FeatureId:       op.FeatureID,
			Label:           op.Label,
			Status:          planStatusProto(op.Status),
			Executable:      op.Executable,
			SyncOnly:        op.SyncOnly,
			BlockedReasons:  append([]string(nil), op.BlockedReasons...),
			OperationId:     op.OperationID,
			GoalId:          op.GoalID,
			DemandId:        op.DemandID,
			TargetId:        op.TargetID,
			ItemId:          op.ItemID,
			Count:           op.Count,
			VaseId:          op.VaseID,
			FlowerIds:       append([]int32(nil), op.FlowerIDs...),
			CostGates:       costGatesProto(op.CostGates),
			BlockingStage:   op.BlockingStage,
			CooldownUntilMs: timeToUnixMilli(cooldownUntil),
			CooldownReason:  cooldownReason,
			TargetUid:       op.TargetUID,
			TargetUids:      append([]int64(nil), op.TargetUIDs...),
			BatchId:         op.BatchID,
			SlotId:          op.SlotID,
			TaskId:          op.TaskID,
			MilestoneIndex:  op.MilestoneIndex,
		})
		if cd, paused := cooldowns["account.request"]; paused {
			view := out[len(out)-1]
			view.Executable = false
			view.Status = pb.PlanStatus_PLAN_STATUS_BLOCKED
			view.BlockedReasons = append(view.BlockedReasons, cd.Reason)
		}
	}
	return out
}

func cooldownsByOperation(diag runner.Diagnostics) map[string]runner.OperationCooldownSnapshot {
	out := make(map[string]runner.OperationCooldownSnapshot, len(diag.OperationCooldowns))
	for _, cd := range diag.OperationCooldowns {
		if cd.OperationID == "" {
			continue
		}
		out[cd.OperationID] = cd
	}
	return out
}

func lookupPlannedOperationCooldown(cooldowns map[string]runner.OperationCooldownSnapshot, op automation.PlannedOp) (runner.OperationCooldownSnapshot, bool) {
	if cd, ok := cooldowns["account.request"]; ok {
		return cd, true
	}
	var selected runner.OperationCooldownSnapshot
	if op.Kind == "fmlRace.delTask" {
		selected = cooldowns["union.race.delete.interval"]
	}
	if key := strings.TrimSpace(op.CooldownKey); key != "" {
		if cd, ok := cooldowns[key]; ok {
			if selected.Until.IsZero() || cd.Until.After(selected.Until) {
				selected = cd
			}
		}
	}
	if op.OperationID != "" {
		if cd, ok := cooldowns[op.OperationID]; ok {
			if selected.Until.IsZero() || cd.Until.After(selected.Until) {
				selected = cd
			}
		}
	}
	return selected, !selected.Until.IsZero()
}

func executionLaneProto(lane string) pb.ExecutionLane {
	switch lane {
	case automation.LaneFarm:
		return pb.ExecutionLane_EXECUTION_LANE_FARM
	case automation.LaneSide:
		return pb.ExecutionLane_EXECUTION_LANE_SIDE
	default:
		return pb.ExecutionLane_EXECUTION_LANE_UNSPECIFIED
	}
}

func timeToUnixMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

func demandsProto(demands []automation.Demand) []*pb.DemandView {
	out := make([]*pb.DemandView, 0, len(demands))
	for _, demand := range demands {
		out = append(out, &pb.DemandView{
			Id:             demand.ID,
			GoalId:         demand.GoalID,
			Category:       demand.Category,
			Domain:         demand.Domain,
			EntityId:       demand.EntityID,
			Label:          demand.Label,
			ItemId:         demand.ItemID,
			ItemName:       itemNameOrID(demand.ItemID),
			Required:       demand.Count,
			Owned:          demand.Have,
			Allocated:      demand.Allocated,
			Available:      demand.Available,
			Missing:        demand.Missing,
			Priority:       demand.Priority,
			Kind:           demand.Kind,
			Source:         demand.Source,
			BlockedReasons: append([]string(nil), demand.BlockedReasons...),
			Status:         planStatusProto(demand.Status),
			BlockingStage:  demand.BlockingStage,
			CostGates:      costGatesProto(demand.CostGates),
		})
	}
	return out
}

func costGatesProto(gates []automation.CostGate) []*pb.CostGate {
	if len(gates) == 0 {
		return nil
	}
	out := make([]*pb.CostGate, 0, len(gates))
	for _, gate := range gates {
		out = append(out, &pb.CostGate{
			Id:             gate.ID,
			ResourceKind:   gateResourceKindProto(gate.ResourceKind),
			Label:          gate.Label,
			ItemId:         gate.ItemID,
			Required:       gate.Required,
			Available:      gate.Available,
			Status:         planStatusProto(gate.Status),
			BlockedReasons: append([]string(nil), gate.BlockedReasons...),
			Hard:           gate.Hard,
			Source:         gate.Source,
		})
	}
	return out
}

func planStatusProto(status string) pb.PlanStatus {
	switch status {
	case automation.PlanStatusReady:
		return pb.PlanStatus_PLAN_STATUS_READY
	case automation.PlanStatusManaged:
		return pb.PlanStatus_PLAN_STATUS_MANAGED
	case automation.PlanStatusSyncOnly:
		return pb.PlanStatus_PLAN_STATUS_SYNC_ONLY
	case automation.PlanStatusAdapterMissing:
		return pb.PlanStatus_PLAN_STATUS_ADAPTER_MISSING
	case automation.PlanStatusBlocked:
		return pb.PlanStatus_PLAN_STATUS_BLOCKED
	case automation.PlanStatusSkipped:
		return pb.PlanStatus_PLAN_STATUS_SKIPPED
	default:
		return pb.PlanStatus_PLAN_STATUS_UNSPECIFIED
	}
}

func gateResourceKindProto(kind string) pb.GateResourceKind {
	switch kind {
	case automation.GateResourceGold:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_GOLD
	case automation.GateResourceDiamond:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_DIAMOND
	case automation.GateResourceItem:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_ITEM
	case automation.GateResourceWaterDrop:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_WATER_DROP
	case automation.GateResourceLevel:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_LEVEL
	case automation.GateResourceVase:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_VASE
	case automation.GateResourcePolicy:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_POLICY
	case automation.GateResourceState:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_STATE
	case automation.GateResourceAdapter:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_ADAPTER
	default:
		return pb.GateResourceKind_GATE_RESOURCE_KIND_UNSPECIFIED
	}
}
