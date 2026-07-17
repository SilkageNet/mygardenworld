package automation

import (
	"fmt"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"sort"
	"time"
)

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
	raceDriven := hasRacePlantDemand(demands)
	if (plantingPolicy.GetAutoHarvestEnabled() || raceDriven) && len(harvest) > 0 {
		ops = append(ops, landOp(clientproto.RPCUsrLandHarvest.String(), "farm.harvest", "harvest", fmt.Sprintf("%d ready lands", len(harvest)), 10000, harvest, 0, "", ""))
	}
	if !plantingPolicy.GetAutoEnabled() && !raceDriven {
		return ops
	}
	if len(plant) > 0 {
		plan := planPlantAssignments(s, policy, demands, int32(len(plant)))
		cursor := 0
		for _, assignment := range plan.executable {
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
		for _, diagnostic := range plan.blockedDiagnostic {
			ops = append(ops, blockedPlantDiagnosticOp(diagnostic))
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

func blockedPlantDiagnosticOp(assignment plantAssignment) PlannedOp {
	op := landOp(clientproto.RPCUsrLandPlant.String(), "farm.plant", "plant", assignment.Reason, assignment.Priority, nil, assignment.FlowerID, assignment.GoalID, assignment.DemandID)
	op.OperationID += "|demand=" + assignment.DemandID
	op.Status = PlanStatusBlocked
	op.Executable = false
	op.LandIDs = nil
	op.BlockedReasons = []string{assignment.Reason}
	op.BlockingStage = "state"
	return op
}

type plantAssignment struct {
	FlowerID int32
	Count    int32
	Priority int32
	GoalID   string
	DemandID string
	Reason   string
}

type plantAssignmentPlan struct {
	executable        []plantAssignment
	blockedDiagnostic []plantAssignment
}

const blockedPlantDiagnosticPriority int32 = 1000

const blockedPlantDiagnosticReason = "需求花朵尚未培育，无法种植"

func plantAssignments(s *state.State, policy *pb.PlantPolicy, demands []Demand, emptyCount int32) []plantAssignment {
	return planPlantAssignments(s, policy, demands, emptyCount).executable
}

func planPlantAssignments(s *state.State, policy *pb.PlantPolicy, demands []Demand, emptyCount int32) plantAssignmentPlan {
	if emptyCount <= 0 {
		return plantAssignmentPlan{}
	}
	plantingPolicy := policy.GetPlanting()
	allowed, blocked := autoReplantFlowerFilters(plantingPolicy)
	candidates := s.PlantableFlowers(allowed, blocked)
	plantable := map[int32]state.PlantableFlower{}
	for _, candidate := range s.PlantableFlowers(nil, nil) {
		plantable[candidate.FlowerID] = candidate
	}
	var out []plantAssignment
	var diagnostics []plantAssignment
	remaining := emptyCount
	for _, demand := range demands {
		if demand.Kind != DemandKindFlower || demand.Missing <= 0 || len(demand.BlockedReasons) > 0 {
			continue
		}
		if _, ok := plantable[demand.ItemID]; !ok {
			diagnostics = append(diagnostics, plantAssignment{
				FlowerID: demand.ItemID,
				Priority: blockedPlantDiagnosticPriority,
				GoalID:   demand.GoalID,
				DemandID: demand.ID,
				Reason:   blockedPlantDiagnosticReason,
			})
			continue
		}
		if remaining <= 0 {
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
	executable := executableAssignments(out)
	if len(executable) > 0 || remaining <= 0 {
		return plantAssignmentPlan{executable: executable, blockedDiagnostic: diagnostics}
	}
	return plantAssignmentPlan{executable: autoReplantAssignments(candidates, remaining), blockedDiagnostic: diagnostics}
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
				Priority: priorityFor(defaultDemandPriority(), GoalAutoReplant)*100 + 100,
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
