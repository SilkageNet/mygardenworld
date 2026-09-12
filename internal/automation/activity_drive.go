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
	cyclicNoteTaskTypeResidentOrder int32 = 1009
	cyclicNoteTaskTypePearlHire     int32 = 1010
	cyclicNoteTaskTypePlantAny      int32 = 3001
	cyclicNoteTaskTypeFlowerRack    int32 = 3015
	cyclicNoteTaskTypeCustomerOrder int32 = 3016

	cyclicNoteDemandPriority int32 = 50
	cyclicNotePlantOpFloor   int32 = 5500
	cyclicNoteRackOpFloor    int32 = 5400
	cyclicNoteRackClaimFloor int32 = 5450
	// Cancel must outrank ordinary customer reject/finish (~11.2k) and module
	// flower-rack sell (~11.3k). Otherwise the final listing wave after local
	// quota hits 0 is starved for hours while side-lane customer work runs.
	cyclicNoteRackCancelFloor int32 = 12050
	cyclicNoteActionGoal            = "activity.cyclicNote"

	// Cyclic-note flower-rack relist: 5 minutes first, 7 minutes if nothing
	// stale at 5 (server 23.3 often lags listing). Do not wait for full sell.
	// After the 3015 task reaches target, a separate one-shot waits 7 minutes
	// then cancels every occupied rack (see driveCyclicNoteFlowerRackPostCompleteCancel).
	cyclicNoteFlowerRackRelistAfter     = 5 * time.Minute
	cyclicNoteFlowerRackRelistSlowAfter = 7 * time.Minute
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
		have      int32
	}
	byType := make(map[int32]representative)
	for _, task := range view.Tasks {
		if !cyclicNoteTaskTypeSupported(task.TaskType) || !cyclicNoteBusinessModuleEnabled(policy, task.TaskType) ||
			!task.Unlocked || task.TaskID <= 0 ||
			!task.CatalogKnown || task.Target <= 0 || task.Progress < 0 || !task.ProgressObserved ||
			!task.ReceiptObserved || task.Received {
			continue
		}
		have := s.CyclicNoteEffectiveProgress(view.BatchID, task.TaskType, task.Progress)
		// Server progress is authoritative for "task still open". Local
		// high-water only caps further plant/list work (remaining may be 0)
		// so flower-rack can keep claim / 5-minute cancel while 23.3 lags.
		if task.Progress >= task.Target {
			continue
		}
		remaining := task.Target - have
		if remaining < 0 {
			remaining = 0
		}
		current, exists := byType[task.TaskType]
		if exists && (current.remaining > remaining ||
			(current.remaining == remaining && current.task.SlotID <= task.SlotID)) {
			continue
		}
		byType[task.TaskType] = representative{task: task, remaining: remaining, have: have}
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
				Have:      rep.have,
				Available: rep.have,
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
		// Plant-any progresses under satisfy_tasks without requiring plant.auto.
		// Gated by auto_plant_any (missing key = enabled). Flower choice /
		// harvest delay / water floor still follow PlantingPolicy.
		return cyclicNoteAutoPlantAny(policy)
	case cyclicNoteTaskTypeFlowerRack:
		// Flower-rack progresses under satisfy_tasks without requiring
		// sell_enabled. Gated by auto_sell_flower_art (missing key = enabled).
		return cyclicNoteAutoSellFlowerArt(policy)
	case cyclicNoteTaskTypeCustomerOrder:
		return policy.GetOrder().GetCustomer().GetEnabled()
	case cyclicNoteTaskTypeResidentOrder:
		return policy.GetOrder().GetResident().GetNormalEnabled() || cyclicNoteAutoCompleteResidentOrders(policy)
	case cyclicNoteTaskTypePearlHire:
		return policy.GetBasic().GetPearl().GetAutoHireEnabled() || cyclicNoteAutoHire(policy)
	default:
		return false
	}
}

func cyclicNoteBoolParam(policy *pb.Policy, key string) bool {
	if policy == nil || key == "" {
		return false
	}
	module := policy.GetActivity().GetModules()[cyclicNoteModuleKey]
	if module == nil {
		return false
	}
	return module.GetBoolParams()[key]
}

// cyclicNoteBoolParamDefaultTrue reads an activity bool param; a missing key
// is treated as true so existing saved policies keep the historical default.
func cyclicNoteBoolParamDefaultTrue(policy *pb.Policy, key string) bool {
	if policy == nil || key == "" {
		return true
	}
	module := policy.GetActivity().GetModules()[cyclicNoteModuleKey]
	if module == nil {
		return true
	}
	bools := module.GetBoolParams()
	if bools == nil {
		return true
	}
	v, ok := bools[key]
	if !ok {
		return true
	}
	return v
}

func cyclicNoteAutoPlantAny(policy *pb.Policy) bool {
	return cyclicNoteBoolParamDefaultTrue(policy, cyclicNoteAutoPlantAnyKey)
}

func cyclicNoteAutoSellFlowerArt(policy *pb.Policy) bool {
	return cyclicNoteBoolParamDefaultTrue(policy, cyclicNoteAutoSellFlowerArtKey)
}

func cyclicNoteAutoHire(policy *pb.Policy) bool {
	return cyclicNoteBoolParam(policy, cyclicNoteAutoHireKey)
}

func cyclicNoteAutoCompleteResidentOrders(policy *pb.Policy) bool {
	return cyclicNoteBoolParam(policy, cyclicNoteAutoCompleteResidentOrdersKey)
}

func cyclicNoteRespectResidentOrderDailyLimit(policy *pb.Policy) bool {
	return cyclicNoteBoolParam(policy, cyclicNoteRespectResidentOrderDailyLimitKey)
}

func cyclicNoteSatisfyTasksEnabled(policy *pb.Policy) bool {
	return cyclicNoteBoolParam(policy, cyclicNoteSatisfyTasksKey)
}

func cyclicNoteResidentOrderTaskActive(actions []cyclicNoteTaskActionDemand) bool {
	for _, action := range actions {
		if action.TaskType == cyclicNoteTaskTypeResidentOrder {
			return true
		}
	}
	return false
}

// cyclicNoteShouldDeferOrderModuleResidentOrders reports whether ordinary
// order-module resident finishes should wait until a 花笺集芳 resident-order
// task is active, so daily quota is not consumed on other tasks first.
func cyclicNoteShouldDeferOrderModuleResidentOrders(s *state.State, policy *pb.Policy, actions []cyclicNoteTaskActionDemand, now time.Time) bool {
	if !cyclicNoteRespectResidentOrderDailyLimit(policy) || !cyclicNoteSatisfyTasksEnabled(policy) {
		return false
	}
	if cyclicNoteResidentOrderTaskActive(actions) {
		return false
	}
	view, ok := s.CyclicNoteView(now)
	return ok && view.Valid && view.Phase == 2 && view.BatchID > 0
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

// cyclicNoteForceFarmCycle reports that 花笺集芳 plant-any is still open on the
// server (demand present; Progress < Target). Ordinary auto_enabled /
// auto_harvest_enabled may be off; water / harvest must keep turning over even
// when local high-water has already capped Missing at 0 (23.3 lag). Replant
// filters still come from PlantingPolicy; further planting uses
// cyclicNoteForceFarmPlant.
func cyclicNoteForceFarmCycle(actions []cyclicNoteTaskActionDemand) bool {
	for _, action := range actions {
		if action.TaskType == cyclicNoteTaskTypePlantAny {
			return true
		}
	}
	return false
}

// cyclicNoteForceFarmPlant is true while plant-any still has quota after local
// high-water. Missing==0 must not keep auto-replanting when AutoEnabled is off.
func cyclicNoteForceFarmPlant(actions []cyclicNoteTaskActionDemand) bool {
	for _, action := range actions {
		if action.TaskType == cyclicNoteTaskTypePlantAny && action.Demand.Missing > 0 {
			return true
		}
	}
	return false
}

// driveCyclicNoteTaskOperations links or emits plant / flower-rack ops for
// unfinished cyclic-note tasks. Plant-any and flower-rack are self-driven
// (no plant.auto / sell_enabled required); plant flower selection still uses
// 土地与种植 replant filters. auto_hire / auto_complete_resident_orders
// similarly self-drive pearl hire and resident finishes (normal / satin /
// decorate); otherwise customer / resident / pearl only decorate ops admitted
// by their owning module.
func driveCyclicNoteTaskOperations(s *state.State, policy *pb.Policy, actions []cyclicNoteTaskActionDemand, ledger *InventoryLedger, ops []PlannedOp, now time.Time) []PlannedOp {
	if policy == nil || len(actions) == 0 {
		return ops
	}
	for _, action := range actions {
		switch action.TaskType {
		case cyclicNoteTaskTypePlantAny:
			if action.Demand.Missing <= 0 {
				continue
			}
			ops = driveCyclicNotePlant(action.Demand, ops)
		case cyclicNoteTaskTypeFlowerRack:
			// Missing may be 0 when local high-water is ahead of 23.3; still
			// claim sold gold / cancel stale listings so racks turn over.
			ops = driveCyclicNoteFlowerRackOperations(s, action.Demand, ledger, ops, now)
		case cyclicNoteTaskTypeCustomerOrder:
			if action.Demand.Missing <= 0 || !policy.GetOrder().GetCustomer().GetEnabled() {
				continue
			}
			linkCyclicNoteBusinessOperation(action.Demand, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) && op.Kind == clientproto.RPCOrderCustomerFinishOrder.String()
			})
		case cyclicNoteTaskTypeResidentOrder:
			if action.Demand.Missing <= 0 {
				continue
			}
			if cyclicNoteAutoCompleteResidentOrders(policy) {
				ops = driveCyclicNoteResidentOrderOperations(s, policy, action.Demand, ledger, ops, now)
				continue
			}
			if !policy.GetOrder().GetResident().GetNormalEnabled() {
				continue
			}
			linkCyclicNoteBusinessOperation(action.Demand, ops, func(op PlannedOp) bool {
				return runnableBusinessOperation(op) && op.Kind == clientproto.RPCOrderFlowerFinishOrder.String()
			})
		case cyclicNoteTaskTypePearlHire:
			if action.Demand.Missing <= 0 {
				continue
			}
			if cyclicNoteAutoHire(policy) {
				ops = driveCyclicNotePearlHireOperations(s, policy, action.Demand, ops, now)
				continue
			}
			if !policy.GetBasic().GetPearl().GetAutoHireEnabled() {
				continue
			}
			linkCyclicNoteBusinessOperation(action.Demand, ops, func(op PlannedOp) bool {
				return op.FeatureID == "basic.pearl_hire" && cyclicNotePearlPlannerKind(op.Kind)
			})
		}
	}
	return ops
}

func driveCyclicNotePlant(demand Demand, ops []PlannedOp) []PlannedOp {
	if hasConcretePlantOperation(ops) {
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

// driveCyclicNoteFlowerRackOperations links ordinary flowerRack.sell ops to the
// unfinished 花笺集芳 flower-rack task, or emits 5/7-minute cancel / sell when
// none is planned. Does not require sell_enabled. Never crafts — only lists
// finished art that already has inventory (highest stock first).
//
// Turnover is cancel/relist (5 minutes, 7-minute fallback), not waiting for
// recvSellMoney. Local high-water drops on cancel so 23.3 lag cannot freeze
// Missing at 0; stranded recovery clears local high-water whenever an empty
// rack remains while the server task is still open (restart used to be the
// only way out of "empty shelf + Missing=0").
func driveCyclicNoteFlowerRackOperations(s *state.State, demand Demand, ledger *InventoryLedger, ops []PlannedOp, now time.Time) []PlannedOp {
	prefix := cyclicNoteDriveReason(demand, "")

	if cancels := cyclicNoteFlowerArtCancelStaleListings(s, demand, prefix, now); len(cancels) > 0 {
		return append(suppressUnlinkedFlowerRackSells(ops), cancels...)
	}
	if demand.Missing <= 0 {
		s.MarkCyclicNoteFlowerRackRelistSlow()
		recovered, ok := recoverStrandedCyclicNoteFlowerRackProgress(s, demand, now)
		if !ok {
			return suppressUnlinkedFlowerRackSells(ops)
		}
		// Local high-water was covering listings that never reached observed
		// 23.3; fall through and list the real remaining onto empty racks.
		demand = recovered
		prefix = cyclicNoteDriveReason(demand, "")
	}

	matchSell := func(op PlannedOp) bool {
		return runnableBusinessOperation(op) && op.Kind == clientproto.RPCFlowerRackSell.String() &&
			op.TargetID > 0 && op.ItemID > 0 && op.Count > 0
	}
	// Activity listing always prefers highest finished stock, not sell_art_ids.
	if sell, ok := raceFlowerArtSellOperation(s, demand, ledger, prefix); ok {
		sell.Priority = cyclicNoteRackOpFloor
		return append(suppressUnlinkedFlowerRackSells(ops), sell)
	}
	if linked, ok := linkCyclicNoteFlowerRackSells(demand, ledger, ops, matchSell); ok {
		return linked
	}
	// Never auto-craft for 花笺集芳: only list finished art with inventory.
	return suppressUnlinkedFlowerRackSells(ops)
}

// recoverStrandedCyclicNoteFlowerRackProgress clears local high-water when the
// planner still sees an open 3015 task (server < target) but Missing==0, and at
// least one rack is empty. Full shelves keep waiting for 5/7-minute cancel;
// partial empties used to freeze until process restart. Returns the demand
// with Missing restored from server progress.
func recoverStrandedCyclicNoteFlowerRackProgress(s *state.State, demand Demand, now time.Time) (Demand, bool) {
	if s == nil || demand.Missing > 0 || demand.Count <= 0 {
		return demand, false
	}
	if len(s.EmptyFlowerRackSlotIDs()) == 0 {
		return demand, false
	}
	batchID, taskType, ok := ParseCyclicNoteDemandID(demand.ID)
	if !ok || batchID <= 0 || taskType != cyclicNoteTaskTypeFlowerRack {
		return demand, false
	}
	serverBatch, serverProgress, _ := s.CyclicNoteServerProgressForType(now, taskType)
	if serverBatch > 0 {
		batchID = serverBatch
	}
	if serverProgress >= demand.Count {
		return demand, false
	}
	if !s.ClearCyclicNoteLocalProgress(batchID, taskType) {
		return demand, false
	}
	demand.Have = serverProgress
	demand.Available = serverProgress
	demand.Missing = demand.Count - serverProgress
	if demand.Missing < 0 {
		demand.Missing = 0
	}
	return demand, demand.Missing > 0
}

// CyclicNoteAutoCompleteResidentOrders reports whether 花笺集芳 should drive
// resident order completion during resident-order activity tasks.
func CyclicNoteAutoCompleteResidentOrders(policy *pb.Policy) bool {
	return cyclicNoteAutoCompleteResidentOrders(policy)
}

// cyclicNoteResidentOrdersShouldPauseForAds reports whether every cooldown-ready
// resident order (normal / satin / decorate) is ad-only, so automation should
// pause instead of waiting on inventory that cannot arrive without ads.
func cyclicNoteResidentOrdersShouldPauseForAds(s *state.State, now time.Time) (bool, int32) {
	if s == nil {
		return false, 0
	}
	var adBoxID int32
	hasAd := false
	hasNonAdReady := false
	for boxID, order := range s.FlowerOrders() {
		if order == nil || !order.CooldownReady(now) {
			continue
		}
		if order.Mode == 8 && len(order.Requires) == 0 {
			hasAd = true
			if adBoxID == 0 {
				adBoxID = boxID
			}
			continue
		}
		if len(order.Requires) > 0 {
			hasNonAdReady = true
		}
	}
	satin := s.ResidentSatinOrder()
	if satin.Observed && satin.CooldownReady(now) {
		switch {
		case satin.IsVideo != 0:
			hasAd = true
		case len(satin.Requires) > 0:
			hasNonAdReady = true
		}
	}
	decorate := s.ResidentDecorateOrder()
	if decorate.Observed && decorate.CooldownReady(now) {
		switch {
		case decorate.IsVideo != 0:
			hasAd = true
		case len(decorate.Requires) > 0:
			hasNonAdReady = true
		}
	}
	if !hasAd || hasNonAdReady {
		return false, 0
	}
	return true, adBoxID
}

func cyclicNoteResidentOrderKind(kind string) bool {
	switch kind {
	case clientproto.RPCOrderFlowerFinishOrder.String(),
		clientproto.RPCOrderFlowerFinishSatinOrder.String(),
		clientproto.RPCOrderFlowerFinishDecorateOrder.String():
		return true
	default:
		return false
	}
}

func cyclicNoteResidentOrderModuleOwned(candidate PlannedOp) bool {
	return runnableBusinessOperation(candidate) && cyclicNoteResidentOrderKind(candidate.Kind) &&
		!strings.HasPrefix(candidate.DemandID, cyclicNoteActionGoal+":")
}

func cyclicNoteResidentOrderFilterOps(ops []PlannedOp, linkedIdx map[int]PlannedOp, emitted []PlannedOp) []PlannedOp {
	out := make([]PlannedOp, 0, len(ops)+len(emitted))
	for i, candidate := range ops {
		if !cyclicNoteResidentOrderKind(candidate.Kind) {
			out = append(out, candidate)
			continue
		}
		if linked, ok := linkedIdx[i]; ok {
			out = append(out, linked)
		}
		// Drop unlinked resident finishes; activity owns the tick.
	}
	return append(out, emitted...)
}

func cyclicNoteResidentOrderAppendPause(ops []PlannedOp, pause PlannedOp) []PlannedOp {
	out := make([]PlannedOp, 0, len(ops)+1)
	for _, candidate := range ops {
		if cyclicNoteResidentOrderModuleOwned(candidate) {
			continue
		}
		out = append(out, candidate)
	}
	return append(out, pause)
}

func cyclicNoteResidentRequiresInventoryReady(requires []state.FlowerRequire, ledger *InventoryLedger, spent map[int32]int32) bool {
	if ledger == nil || len(requires) == 0 {
		return false
	}
	for _, req := range requires {
		if req.FlowerID <= 0 || req.Count <= 0 {
			return false
		}
		available := ledger.Available(req.FlowerID) - spent[req.FlowerID]
		if available < req.Count {
			return false
		}
	}
	return true
}

func cyclicNoteResidentRequiresSpend(requires []state.FlowerRequire, spent map[int32]int32) {
	for _, req := range requires {
		if req.FlowerID > 0 && req.Count > 0 {
			spent[req.FlowerID] += req.Count
		}
	}
}

func cyclicNoteLinkResidentFinish(linked PlannedOp, demand Demand, goal Goal, reason string, priority int32) PlannedOp {
	linked.DemandID = demand.ID
	linked.Reason = cyclicNoteDriveReason(demand, reason)
	linked.Executable = true
	linked.Status = PlanStatusReady
	linked.BlockedReasons = nil
	if linked.Priority < priority {
		linked.Priority = priority
	}
	return linked
}

// driveCyclicNoteResidentOrderOperations finishes every ordinary, satin, and
// decorate resident order that inventory can cover while the activity task
// remains open. Ignores order-module enable / quality gates; daily-limit gates
// apply only when respect_resident_order_daily_limit is on. Pauses only when
// every cooldown-ready order slot is ad-only.
func driveCyclicNoteResidentOrderOperations(s *state.State, policy *pb.Policy, demand Demand, ledger *InventoryLedger, ops []PlannedOp, now time.Time) []PlannedOp {
	prefix := cyclicNoteDriveReason(demand, "")
	if s == nil {
		return ops
	}

	goal := Goal{ID: GoalResidentOrder, Category: CategoryOrder, Domain: "order.resident", Label: "居民订单", Priority: 80}
	if ledger == nil {
		ledger = NewInventoryLedger(s.Inventory())
	}
	resident := (*pb.ResidentOrderPolicy)(nil)
	respectDailyLimit := false
	if policy != nil {
		resident = policy.GetOrder().GetResident()
		respectDailyLimit = cyclicNoteRespectResidentOrderDailyLimit(policy)
	}
	var normalDailyLimited bool
	var satinDailyLimited bool
	var decorateDailyLimited bool
	if respectDailyLimit && resident != nil {
		_, normalDailyLimited = residentNormalDailyLimitReached(s, resident, now)
		_, satinDailyLimited = residentSatinDailyLimitReached(s, resident, now)
		_, decorateDailyLimited = residentDecorateDailyLimitReached(s, resident, now)
	}
	type indexed struct {
		idx int
		op  PlannedOp
	}
	existingNormal := make(map[int32]indexed)
	var existingSatinIdx = -1
	var existingSatin PlannedOp
	var existingDecorateIdx = -1
	var existingDecorate PlannedOp
	for i, candidate := range ops {
		switch candidate.Kind {
		case clientproto.RPCOrderFlowerFinishOrder.String():
			if candidate.TargetID > 0 {
				existingNormal[candidate.TargetID] = indexed{idx: i, op: candidate}
			}
		case clientproto.RPCOrderFlowerFinishSatinOrder.String():
			if existingSatinIdx < 0 {
				existingSatinIdx = i
				existingSatin = candidate
			}
		case clientproto.RPCOrderFlowerFinishDecorateOrder.String():
			if existingDecorateIdx < 0 {
				existingDecorateIdx = i
				existingDecorate = candidate
			}
		}
	}

	linkedIdx := make(map[int]PlannedOp)
	var emitted []PlannedOp
	spent := map[int32]int32{}

	orders := s.FlowerOrders()
	boxIDs := make([]int32, 0, len(orders))
	for boxID := range orders {
		boxIDs = append(boxIDs, boxID)
	}
	sort.Slice(boxIDs, func(i, j int) bool { return boxIDs[i] < boxIDs[j] })

	for _, boxID := range boxIDs {
		if normalDailyLimited {
			break
		}
		flowerOrder := orders[boxID]
		if flowerOrder == nil || len(flowerOrder.Requires) == 0 || !flowerOrder.CooldownReady(now) {
			continue
		}
		if !cyclicNoteResidentRequiresInventoryReady(flowerOrder.Requires, ledger, spent) {
			continue
		}
		reason := withOrderReason(prefix+"；居民订单可交付", FormatFlowerRequires(flowerOrder.Requires))
		priority := goal.Priority*100 + 700
		if prior, ok := existingNormal[boxID]; ok {
			linkedIdx[prior.idx] = cyclicNoteLinkResidentFinish(prior.op, demand, goal, reason, priority)
		} else {
			finish := op(clientproto.RPCOrderFlowerFinishOrder.String(), goal, "finish", reason, priority, boxID, 0, 0)
			finish.DemandID = demand.ID
			emitted = append(emitted, finish)
		}
		cyclicNoteResidentRequiresSpend(flowerOrder.Requires, spent)
	}

	satin := s.ResidentSatinOrder()
	if !satinDailyLimited && satin.Observed && satin.IsVideo == 0 && len(satin.Requires) > 0 && satin.CooldownReady(now) &&
		cyclicNoteResidentRequiresInventoryReady(satin.Requires, ledger, spent) {
		reason := withOrderReason(prefix+"；绸缎居民订单可交付", FormatFlowerRequires(satin.Requires))
		priority := goal.Priority*100 + 710
		if existingSatinIdx >= 0 {
			linkedIdx[existingSatinIdx] = cyclicNoteLinkResidentFinish(existingSatin, demand, goal, reason, priority)
		} else {
			finish := op(clientproto.RPCOrderFlowerFinishSatinOrder.String(), goal, "finish", reason, priority, 0, 0, 0)
			finish.Domain = "order.resident.satin"
			finish.DemandID = demand.ID
			emitted = append(emitted, finish)
		}
		cyclicNoteResidentRequiresSpend(satin.Requires, spent)
	}

	decorate := s.ResidentDecorateOrder()
	if !decorateDailyLimited && decorate.Observed && decorate.IsVideo == 0 && len(decorate.Requires) > 0 && decorate.CooldownReady(now) &&
		cyclicNoteResidentRequiresInventoryReady(decorate.Requires, ledger, spent) {
		reason := withOrderReason(prefix+"；建材居民订单可交付", FormatFlowerRequires(decorate.Requires))
		priority := goal.Priority*100 + 705
		if existingDecorateIdx >= 0 {
			linkedIdx[existingDecorateIdx] = cyclicNoteLinkResidentFinish(existingDecorate, demand, goal, reason, priority)
		} else {
			finish := op(clientproto.RPCOrderFlowerFinishDecorateOrder.String(), goal, "finish", reason, priority, 0, 0, 0)
			finish.Domain = "order.resident.decorate"
			finish.DemandID = demand.ID
			emitted = append(emitted, finish)
		}
		cyclicNoteResidentRequiresSpend(decorate.Requires, spent)
	}

	if len(linkedIdx) > 0 || len(emitted) > 0 {
		return cyclicNoteResidentOrderFilterOps(ops, linkedIdx, emitted)
	}

	if adBlocked, adBoxID := cyclicNoteResidentOrdersShouldPauseForAds(s, now); adBlocked {
		pause := markerOp(CategoryActivity, "activity.cyclicNote.resident_ad", "pause",
			prefix+"；居民订单出现广告位，暂停自动完成", cyclicNotePriority)
		pause.DemandID = demand.ID
		pause.GoalID = GoalResidentOrder
		pause.Status = PlanStatusBlocked
		pause.Executable = false
		pause.BlockedReasons = []string{"当前居民订单为广告订单，暂不自动提交；请自行看广告，看完或重启后会重新同步并继续"}
		pause.TargetID = adBoxID
		return cyclicNoteResidentOrderAppendPause(ops, pause)
	}

	return cyclicNoteResidentOrderFilterOps(ops, nil, nil)
}

// cyclicNoteResidentOrderInventoryReady reports whether inventory (minus
// already-claimed spends this tick) covers the order. Unlike
// canFulfillFlowerOrder it does not require order-module ledger allocations.
func cyclicNoteResidentOrderInventoryReady(order *state.FlowerOrder, ledger *InventoryLedger, spent map[int32]int32) bool {
	if order == nil {
		return false
	}
	return cyclicNoteResidentRequiresInventoryReady(order.Requires, ledger, spent)
}

// driveCyclicNotePearlHireOperations links an ordinary safe-hire step when the
// basic switch already planned one, otherwise emits PlanOneSafePearlHire while
// bypassing only auto_hire_enabled. Other pearl gates stay enforced.
func driveCyclicNotePearlHireOperations(s *state.State, policy *pb.Policy, demand Demand, ops []PlannedOp, now time.Time) []PlannedOp {
	match := func(candidate PlannedOp) bool {
		return candidate.FeatureID == "basic.pearl_hire" && cyclicNotePearlPlannerKind(candidate.Kind)
	}
	if idx := deterministicOperationIndex(ops, match); idx >= 0 {
		ops[idx].DemandID = demand.ID
		ops[idx].Reason = cyclicNoteDriveReason(demand, ops[idx].Reason)
		return ops
	}
	hire, ok := PlanOneSafePearlHire(s, policy.GetBasic().GetPearl(), now, PearlHireIntent{
		GoalID:                cyclicNoteActionGoal,
		DemandID:              demand.ID,
		Category:              CategoryActivity,
		Domain:                cyclicNoteActionGoal,
		Label:                 cyclicNoteTaskTypeLabel(cyclicNoteTaskTypePearlHire),
		Reason:                cyclicNoteDriveReason(demand, ""),
		Priority:              pearlHirePriority,
		BypassAutoHireEnabled: true,
	})
	if !ok {
		return ops
	}
	hire.DemandID = demand.ID
	hire.Reason = cyclicNoteDriveReason(demand, hire.Reason)
	return append(ops, hire)
}

// cyclicNoteFlowerArtCancelStaleListings cancels occupied racks past the active
// relist threshold (5 minutes, escalated to 7 when 23.3 stalls), including
// claimable slots — cyclic-note 3015 turns over by relist, not recvSellMoney.
func cyclicNoteFlowerArtCancelStaleListings(s *state.State, demand Demand, prefix string, now time.Time) []PlannedOp {
	if s == nil || demand.ID == "" {
		return nil
	}
	d := demand
	if d.Missing <= 0 {
		d.Missing = 1
	}
	after := s.CyclicNoteFlowerRackRelistAfter()
	return flowerArtCancelStaleListings(s, d, prefix, now, after,
		cyclicNoteRackCancelFloor, "activity.cyclicNote.flower_art_cancel", true, true, false)
}

// driveCyclicNoteFlowerRackPostCompleteCancel takes down every occupied
// flower-rack slot 7 minutes after the 花笺集芳 花艺上架 task reaches its
// target. Mid-task 5/7-minute relist is unchanged; this only runs once the
// server progress is complete and no unfinished 3015 demand remains.
//
// recvTaskRwd replaces the 3015 slot, so completion must keep an armed timer
// (or recover from newest ListedAt) after the task disappears from the view.
func driveCyclicNoteFlowerRackPostCompleteCancel(s *state.State, policy *pb.Policy, actions []cyclicNoteTaskActionDemand, ops []PlannedOp, now time.Time) []PlannedOp {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() ||
		!cyclicNoteSatisfyTasksEnabled(policy) || !cyclicNoteAutoSellFlowerArt(policy) {
		return ops
	}
	for _, action := range actions {
		if action.TaskType == cyclicNoteTaskTypeFlowerRack {
			// Unfinished 3015 still owns turnover (including Missing==0 lag).
			s.ClearCyclicNoteFlowerRackPostCompleteCancel()
			return ops
		}
	}
	batchID, ok := cyclicNoteCompletedFlowerRackBatch(s, now)
	if ok {
		s.BeginCyclicNoteFlowerRackPostCompleteCancel(batchID, now)
	} else if armedID, armed := s.CyclicNoteFlowerRackPostCompleteArmed(); armed {
		view, found := s.CyclicNoteView(now)
		if !found || !view.Valid || view.Phase != 2 || view.BatchID != armedID {
			return ops
		}
		batchID = armedID
	} else if orphanBatch, atMs, orphan := cyclicNoteFlowerRackPostCompleteOrphan(s, now); orphan {
		batchID = orphanBatch
		s.BeginCyclicNoteFlowerRackPostCompleteCancelAt(batchID, atMs)
	} else {
		return ops
	}
	if s.CyclicNoteFlowerRackPostCompleteCancelDone(batchID) {
		return ops
	}
	if !s.CyclicNoteFlowerRackPostCompleteCancelReady(batchID, now) {
		return ops
	}
	entityID := strconv.FormatInt(int64(batchID), 10) + ":" + strconv.FormatInt(int64(cyclicNoteTaskTypeFlowerRack), 10)
	demand := Demand{
		ID:       cyclicNoteActionGoal + ":" + entityID,
		GoalID:   cyclicNoteActionGoal,
		Category: CategoryActivity,
		Domain:   cyclicNoteActionGoal,
		EntityID: entityID,
		Source:   "task_type:" + strconv.FormatInt(int64(cyclicNoteTaskTypeFlowerRack), 10),
		Label:    cyclicNoteTaskTypeLabel(cyclicNoteTaskTypeFlowerRack),
		Kind:     DemandKindAction,
		Missing:  1,
		Priority: cyclicNoteDemandPriority,
	}
	prefix := "花笺集芳花艺上架任务已完成"
	cancels := cyclicNoteFlowerArtCancelAllOccupied(s, demand, prefix, now,
		"activity.cyclicNote.flower_art_post_complete_cancel")
	if occupiedFlowerRackCount(s) == 0 {
		s.MarkCyclicNoteFlowerRackPostCompleteCancelDone(batchID)
		return ops
	}
	if len(cancels) == 0 {
		// Occupied shelves but no cancel built — keep trying next tick.
		return ops
	}
	for i := range cancels {
		cancels[i].Reason = fmt.Sprintf("%s；满7分钟，全部下架", prefix)
	}
	return append(ops, cancels...)
}

// cyclicNoteFlowerRackPostCompleteOrphan recovers when 3015 was claimed away
// (task list no longer contains type 3015) and the in-memory timer was lost.
// Newest occupied ListedAtMs proxies the final listing / completion clock.
func cyclicNoteFlowerRackPostCompleteOrphan(s *state.State, now time.Time) (batchID int32, atMs int64, ok bool) {
	if s == nil {
		return 0, 0, false
	}
	view, found := s.CyclicNoteView(now)
	if !found || !view.Valid || view.Phase != 2 || view.BatchID <= 0 ||
		!view.TaskListObserved || !view.TaskRecordObserved {
		return 0, 0, false
	}
	for _, task := range view.Tasks {
		if task.TaskType == cyclicNoteTaskTypeFlowerRack && task.Unlocked && task.TaskID > 0 {
			// Slot still present — completed-visible or mid-task path owns it.
			return 0, 0, false
		}
	}
	newest := int64(0)
	occupied := 0
	for _, slot := range s.FlowerRackSlots() {
		if slot.ItemID <= 0 || slot.Count <= 0 {
			continue
		}
		occupied++
		if slot.ListedAtMs > newest {
			newest = slot.ListedAtMs
		}
	}
	if occupied == 0 {
		return 0, 0, false
	}
	if newest <= 0 {
		return view.BatchID, now.UnixMilli(), true
	}
	return view.BatchID, newest, true
}

func occupiedFlowerRackCount(s *state.State) int {
	if s == nil {
		return 0
	}
	n := 0
	for _, slot := range s.FlowerRackSlots() {
		if slot.ItemID > 0 && slot.Count > 0 {
			n++
		}
	}
	return n
}

// cyclicNoteFlowerArtCancelAllOccupied cancels every occupied rack for
// post-complete cleanup. ListedAtMs<=0 still cancels — sparse 104 deltas after
// the final wave can omit field 4 and would otherwise freeze the one-shot clear.
func cyclicNoteFlowerArtCancelAllOccupied(s *state.State, demand Demand, prefix string, now time.Time, featureID string) []PlannedOp {
	if s == nil || demand.ID == "" {
		return nil
	}
	d := demand
	if d.Missing <= 0 {
		d.Missing = 1
	}
	return flowerArtCancelStaleListings(s, d, prefix, now, time.Millisecond,
		cyclicNoteRackCancelFloor, featureID, true, true, true)
}

// cyclicNoteCompletedFlowerRackBatch reports the current batch when a 3015
// task has Progress >= Target (including after reward claim).
func cyclicNoteCompletedFlowerRackBatch(s *state.State, now time.Time) (batchID int32, ok bool) {
	if s == nil {
		return 0, false
	}
	view, found := s.CyclicNoteView(now)
	if !found || !view.Valid || view.Phase != 2 || view.BatchID <= 0 ||
		!view.TaskListObserved || !view.TaskRecordObserved {
		return 0, false
	}
	for _, task := range view.Tasks {
		if task.TaskType != cyclicNoteTaskTypeFlowerRack || !task.Unlocked || task.TaskID <= 0 ||
			!task.CatalogKnown || task.Target <= 0 || !task.ProgressObserved || task.Progress < 0 {
			continue
		}
		if task.Progress >= task.Target {
			return view.BatchID, true
		}
	}
	return 0, false
}

// linkCyclicNoteFlowerRackSells caps every runnable flowerRack.sell this tick to
// the activity remaining quota and drops the rest. sell_enabled otherwise emits
// one sell per empty rack (6×12=72) while only the first was activity-linked.
func linkCyclicNoteFlowerRackSells(demand Demand, ledger *InventoryLedger, ops []PlannedOp, match func(PlannedOp) bool) ([]PlannedOp, bool) {
	type indexed struct {
		idx int
		op  PlannedOp
	}
	candidates := make([]indexed, 0)
	for i := range ops {
		if match(ops[i]) {
			candidates = append(candidates, indexed{idx: i, op: ops[i]})
		}
	}
	if len(candidates) == 0 {
		return ops, false
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return operationComesBefore(candidates[i].op, candidates[j].op)
	})

	remaining := demand.Missing
	spent := map[int32]int32{}
	keep := make(map[int]PlannedOp, len(candidates))
	for _, c := range candidates {
		if remaining <= 0 {
			break
		}
		count := c.op.Count
		if count > remaining {
			count = remaining
		}
		if ledger != nil {
			available := ledger.Available(c.op.ItemID) - spent[c.op.ItemID]
			if available < 0 {
				available = 0
			}
			if count > available {
				count = available
			}
		}
		if count > flowerRackPerSlotCount {
			count = flowerRackPerSlotCount
		}
		if count <= 0 {
			continue
		}
		linked := c.op
		linked.Count = count
		linked.ItemCost = map[int32]int32{linked.ItemID: count}
		linked.DemandID = demand.ID
		linked.Reason = cyclicNoteDriveReason(demand, linked.Reason)
		if linked.Priority < cyclicNoteRackOpFloor {
			linked.Priority = cyclicNoteRackOpFloor
		}
		keep[c.idx] = linked
		spent[linked.ItemID] += count
		remaining -= count
	}
	if len(keep) == 0 {
		return suppressUnlinkedFlowerRackSells(ops), true
	}
	out := make([]PlannedOp, 0, len(ops))
	for i, op := range ops {
		if !match(op) {
			out = append(out, op)
			continue
		}
		if linked, ok := keep[i]; ok {
			out = append(out, linked)
		}
	}
	return out, true
}

func suppressUnlinkedFlowerRackSells(ops []PlannedOp) []PlannedOp {
	out := make([]PlannedOp, 0, len(ops))
	for _, op := range ops {
		if runnableBusinessOperation(op) && op.Kind == clientproto.RPCFlowerRackSell.String() &&
			!strings.HasPrefix(op.DemandID, cyclicNoteActionGoal+":") &&
			!strings.HasPrefix(op.DemandID, raceActionGoal+":") {
			continue
		}
		out = append(out, op)
	}
	return out
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

// IsPlantOperation reports whether kind is a personal-land plant RPC.
func IsPlantOperation(kind string) bool {
	return isPlantOperation(kind)
}

// ParseCyclicNoteDemandID extracts batch id and task type from
// activity.cyclicNote:<batchID>:<taskType>.
func ParseCyclicNoteDemandID(demandID string) (batchID, taskType int32, ok bool) {
	if !strings.HasPrefix(demandID, cyclicNoteActionGoal+":") {
		return 0, 0, false
	}
	parts := strings.Split(demandID, ":")
	if len(parts) != 3 {
		return 0, 0, false
	}
	batch, err1 := strconv.ParseInt(parts[1], 10, 32)
	task, err2 := strconv.ParseInt(parts[2], 10, 32)
	if err1 != nil || err2 != nil || batch <= 0 || task <= 0 {
		return 0, 0, false
	}
	return int32(batch), int32(task), true
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
