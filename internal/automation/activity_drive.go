package automation

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	cyclicNoteTaskTypeResidentOrder int32 = 1009
	cyclicNoteTaskTypePearlHire     int32 = 1010
	cyclicNoteTaskTypePlantAny      int32 = 3001
	cyclicNoteTaskTypeFlowerRack    int32 = 3015
	cyclicNoteTaskTypeCustomerOrder int32 = 3016

	cyclicNoteDemandPriority int32 = 50
	cyclicNotePlantOpFloor   int32 = 5500
	cyclicNoteRackOpFloor    int32 = 5400
	cyclicNoteActionGoal           = "activity.cyclicNote"
)

type cyclicNoteTaskActionDemand struct {
	TaskType int32
	Demand   Demand
}

// cyclicNoteTaskActionDemands converts only capture-confirmed, incomplete
// cyclic-note tasks into non-inventory demands. It runs after all ledger
// allocations so ItemID=0 can never reserve or release user inventory.
func cyclicNoteTaskActionDemands(s *state.State, policy *pb.Policy, now time.Time) []cyclicNoteTaskActionDemand {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() {
		return nil
	}
	activity := policy.GetActivity()
	if activity == nil {
		return nil
	}
	module := activity.GetModules()[cyclicNoteModuleKey]
	if module == nil || !module.GetEnabled() || !module.GetBoolParams()[cyclicNoteSatisfyTasksKey] {
		return nil
	}
	view, ok := s.CyclicNoteView(now)
	if !ok || !view.Valid || view.Phase != 2 || view.BatchID <= 0 ||
		!view.TaskListObserved || !view.TaskRecordObserved {
		return nil
	}

	type representative struct {
		task      state.CyclicNoteTaskSlotView
		remaining int32
	}
	byType := make(map[int32]representative)
	for _, task := range view.Tasks {
		if !cyclicNoteTaskTypeSupported(task.TaskType) || !cyclicNoteBusinessModuleEnabled(policy, task.TaskType) ||
			!task.Unlocked || task.TaskID <= 0 ||
			!task.CatalogKnown || task.Target <= 0 || task.Progress < 0 || !task.ProgressObserved ||
			!task.ReceiptObserved || task.Received || task.Progress >= task.Target {
			continue
		}
		remaining := task.Target - task.Progress
		current, exists := byType[task.TaskType]
		if exists && (current.remaining > remaining ||
			(current.remaining == remaining && current.task.SlotID <= task.SlotID)) {
			continue
		}
		byType[task.TaskType] = representative{task: task, remaining: remaining}
	}
	if len(byType) == 0 {
		return nil
	}
	taskTypes := make([]int32, 0, len(byType))
	for taskType := range byType {
		taskTypes = append(taskTypes, taskType)
	}
	sort.Slice(taskTypes, func(i, j int) bool { return taskTypes[i] < taskTypes[j] })
	out := make([]cyclicNoteTaskActionDemand, 0, len(taskTypes))
	for _, taskType := range taskTypes {
		rep := byType[taskType]
		entityID := strconv.FormatInt(int64(view.BatchID), 10) + ":" + strconv.FormatInt(int64(taskType), 10)
		id := cyclicNoteActionGoal + ":" + entityID
		label := rep.task.Title
		if label == "" {
			label = cyclicNoteTaskTypeLabel(taskType)
		}
		out = append(out, cyclicNoteTaskActionDemand{
			TaskType: taskType,
			Demand: Demand{
				ID:        id,
				GoalID:    cyclicNoteActionGoal,
				Category:  CategoryActivity,
				Domain:    cyclicNoteActionGoal,
				EntityID:  entityID,
				Source:    "task_type:" + strconv.FormatInt(int64(taskType), 10),
				Label:     label,
				Kind:      DemandKindAction,
				ItemID:    0,
				Count:     rep.task.Target,
				Have:      rep.task.Progress,
				Available: rep.task.Progress,
				Missing:   rep.remaining,
				Priority:  cyclicNoteDemandPriority,
			},
		})
	}
	return out
}

func cyclicNoteBusinessModuleEnabled(policy *pb.Policy, taskType int32) bool {
	if policy == nil {
		return false
	}
	switch taskType {
	case cyclicNoteTaskTypePlantAny:
		return policy.GetPlant().GetPlanting().GetAutoEnabled()
	case cyclicNoteTaskTypeFlowerRack:
		return policy.GetOrder().GetFlowerArt().GetSellEnabled()
	case cyclicNoteTaskTypeCustomerOrder:
		return policy.GetOrder().GetCustomer().GetEnabled()
	case cyclicNoteTaskTypeResidentOrder:
		return policy.GetOrder().GetResident().GetNormalEnabled()
	case cyclicNoteTaskTypePearlHire:
		return policy.GetBasic().GetPearl().GetAutoHireEnabled()
	default:
		return false
	}
}

func cyclicNoteTaskTypeSupported(taskType int32) bool {
	switch taskType {
	case cyclicNoteTaskTypePlantAny, cyclicNoteTaskTypeFlowerRack, cyclicNoteTaskTypeCustomerOrder,
		cyclicNoteTaskTypeResidentOrder, cyclicNoteTaskTypePearlHire:
		return true
	default:
		return false
	}
}

func cyclicNoteTaskTypeLabel(taskType int32) string {
	switch taskType {
	case cyclicNoteTaskTypePlantAny:
		return "花笺集芳任意种植"
	case cyclicNoteTaskTypeFlowerRack:
		return "花笺集芳花架出售"
	case cyclicNoteTaskTypeCustomerOrder:
		return "花笺集芳顾客订单"
	case cyclicNoteTaskTypeResidentOrder:
		return "花笺集芳居民订单"
	case cyclicNoteTaskTypePearlHire:
		return "花笺集芳珍珠雇佣"
	default:
		return "花笺集芳任务"
	}
}

// driveCyclicNoteTaskOperations only decorates operations already admitted by
// their owning module. It never turns a disabled module on and never copies
// activity batch/slot/task metadata into a business RPC.
func driveCyclicNoteTaskOperations(policy *pb.Policy, actions []cyclicNoteTaskActionDemand, ledger *InventoryLedger, ops []PlannedOp) []PlannedOp {
	if policy == nil || len(actions) == 0 || len(ops) == 0 {
		return ops
	}
	for _, action := range actions {
		if action.Demand.Missing <= 0 {
			continue
		}
		switch action.TaskType {
		case cyclicNoteTaskTypePlantAny:
			ops = driveCyclicNotePlant(policy, action.Demand, ops)
		case cyclicNoteTaskTypeFlowerRack:
			driveCyclicNoteFlowerRack(policy, action.Demand, ledger, ops)
		case cyclicNoteTaskTypeCustomerOrder:
			if policy.GetOrder().GetCustomer().GetEnabled() {
				linkCyclicNoteBusinessOperation(action.Demand, ops, func(op PlannedOp) bool {
					return runnableBusinessOperation(op) && op.Kind == clientproto.RPCOrderCustomerFinishOrder.String()
				})
			}
		case cyclicNoteTaskTypeResidentOrder:
			if policy.GetOrder().GetResident().GetNormalEnabled() {
				linkCyclicNoteBusinessOperation(action.Demand, ops, func(op PlannedOp) bool {
					return runnableBusinessOperation(op) && op.Kind == clientproto.RPCOrderFlowerFinishOrder.String()
				})
			}
		case cyclicNoteTaskTypePearlHire:
			if policy.GetBasic().GetPearl().GetAutoHireEnabled() {
				linkCyclicNoteBusinessOperation(action.Demand, ops, func(op PlannedOp) bool {
					return op.FeatureID == "basic.pearl_hire" && cyclicNotePearlPlannerKind(op.Kind)
				})
			}
		}
	}
	return ops
}

func driveCyclicNotePlant(policy *pb.Policy, demand Demand, ops []PlannedOp) []PlannedOp {
	if !policy.GetPlant().GetPlanting().GetAutoEnabled() || hasConcretePlantOperation(ops) {
		return ops
	}
	isFallback := func(op PlannedOp) bool {
		return runnableBusinessOperation(op) && isPlantOperation(op.Kind) && op.GoalID == GoalAutoReplant && len(op.LandIDs) > 0
	}
	idx := deterministicOperationIndex(ops, isFallback)
	if idx < 0 {
		return ops
	}
	selected := ops[idx]
	landSet := make(map[int32]struct{})
	for _, candidate := range ops {
		if !isFallback(candidate) || candidate.FlowerID != selected.FlowerID {
			continue
		}
		for _, landID := range candidate.LandIDs {
			if landID > 0 {
				landSet[landID] = struct{}{}
			}
		}
	}
	lands := make([]int32, 0, len(landSet))
	for landID := range landSet {
		lands = append(lands, landID)
	}
	sort.Slice(lands, func(i, j int) bool { return lands[i] < lands[j] })
	if int32(len(lands)) > demand.Missing {
		lands = lands[:demand.Missing]
	}
	if len(lands) == 0 {
		return ops
	}
	selected.LandIDs = append([]int32(nil), lands...)
	if len(lands) == 1 {
		selected.Kind = clientproto.RPCUsrLandPlant.String()
	} else {
		selected.Kind = clientproto.RPCUsrLandPlantBatch.String()
	}
	selected.OperationID = operationID(selected.Kind, selected.LandIDs, selected.FlowerID, 0, 0)
	selected.DemandID = demand.ID
	selected.Reason = cyclicNoteDriveReason(demand, selected.Reason)
	if selected.Priority < cyclicNotePlantOpFloor {
		selected.Priority = cyclicNotePlantOpFloor
	}

	// Auto-replant may naturally emit one operation per balancing step. While
	// satisfying one activity task, retain exactly one capped RPC this tick so
	// unlinked farm-lane fallbacks cannot jump ahead of its activity priority.
	out := make([]PlannedOp, 0, len(ops))
	inserted := false
	for i := range ops {
		if !isFallback(ops[i]) {
			out = append(out, ops[i])
			continue
		}
		if i == idx && !inserted {
			out = append(out, selected)
			inserted = true
		}
	}
	if !inserted {
		out = append(out, selected)
	}
	return out
}

func driveCyclicNoteFlowerRack(policy *pb.Policy, demand Demand, ledger *InventoryLedger, ops []PlannedOp) {
	if !policy.GetOrder().GetFlowerArt().GetSellEnabled() || ledger == nil {
		return
	}
	idx := deterministicOperationIndex(ops, func(op PlannedOp) bool {
		return runnableBusinessOperation(op) && op.Kind == clientproto.RPCFlowerRackSell.String() &&
			op.TargetID > 0 && op.ItemID > 0 && op.Count > 0
	})
	if idx < 0 {
		return
	}
	op := &ops[idx]
	count := op.Count
	if count > demand.Missing {
		count = demand.Missing
	}
	if available := ledger.Available(op.ItemID); count > available {
		count = available
	}
	if count > flowerRackPerSlotCount {
		count = flowerRackPerSlotCount
	}
	if count <= 0 {
		return
	}
	op.Count = count
	op.ItemCost = map[int32]int32{op.ItemID: count}
	op.DemandID = demand.ID
	op.Reason = cyclicNoteDriveReason(demand, op.Reason)
	if op.Priority < cyclicNoteRackOpFloor {
		op.Priority = cyclicNoteRackOpFloor
	}
}

func linkCyclicNoteBusinessOperation(demand Demand, ops []PlannedOp, match func(PlannedOp) bool) {
	idx := deterministicOperationIndex(ops, match)
	if idx < 0 {
		return
	}
	ops[idx].DemandID = demand.ID
	ops[idx].Reason = cyclicNoteDriveReason(demand, ops[idx].Reason)
}

func deterministicOperationIndex(ops []PlannedOp, match func(PlannedOp) bool) int {
	best := -1
	for i := range ops {
		if !match(ops[i]) {
			continue
		}
		if best < 0 || operationComesBefore(ops[i], ops[best]) {
			best = i
		}
	}
	return best
}

func runnableBusinessOperation(op PlannedOp) bool {
	return op.Executable && !op.SyncOnly && op.Status != PlanStatusBlocked && op.Status != PlanStatusAdapterMissing &&
		len(op.BlockedReasons) == 0
}

func hasConcretePlantOperation(ops []PlannedOp) bool {
	for _, op := range ops {
		// A concrete demand owns its assigned lands even when its operation is
		// currently resource-blocked. Adding an activity-only fallback beside it
		// would double-book those lands and hide the actionable diagnostic.
		if isPlantOperation(op.Kind) && op.GoalID != GoalAutoReplant && op.DemandID != "" && len(op.LandIDs) > 0 {
			return true
		}
	}
	return false
}

func isPlantOperation(kind string) bool {
	return kind == clientproto.RPCUsrLandPlant.String() || kind == clientproto.RPCUsrLandPlantBatch.String()
}

func cyclicNotePearlPlannerKind(kind string) bool {
	switch kind {
	case clientproto.RPCFrdEnter.String(), clientproto.RPCOpptGetDetailOppts.String(),
		clientproto.RPCPearlGetHireStateByUids.String(), clientproto.RPCPearlGetRecommendList.String(),
		clientproto.RPCPearlRefresh.String(), clientproto.RPCPearlPlaceHire.String(), "basic.pearl.hire.blocked":
		return true
	default:
		return false
	}
}

func cyclicNoteDriveReason(demand Demand, existing string) string {
	prefix := fmt.Sprintf("花笺集芳任务剩余 %d 次", demand.Missing)
	if existing == "" {
		return prefix
	}
	return prefix + "；" + existing
}
