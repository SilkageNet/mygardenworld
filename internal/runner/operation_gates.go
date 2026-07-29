package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func (r *Runner) nextRunnableOperation(policy *pb.Policy, now time.Time) *automation.PlannedOp {
	if policy == nil || !policy.GetAutomationEnabled() {
		r.resetSideLaneFairness()
		return nil
	}
	return r.selectRunnableOperation(automation.PlanOperations(r.state, policy, now), now)
}

func runnablePlannedOp(op automation.PlannedOp) bool {
	return op.Executable &&
		!op.SyncOnly &&
		op.Status != automation.PlanStatusAdapterMissing &&
		op.Status != automation.PlanStatusBlocked &&
		len(op.BlockedReasons) == 0
}

func (r *Runner) checkOperationResources(op *automation.PlannedOp, now time.Time) error {
	if op == nil {
		return nil
	}
	for _, gate := range op.CostGates {
		if err := r.checkCostGate(op, gate, now); err != nil {
			return err
		}
	}
	if op.DiamondCost > 0 {
		return fmt.Errorf("钻石消耗操作默认不自动执行: %d", op.DiamondCost)
	}
	if op.GoldCost > 0 && r.state.Gold() < op.GoldCost {
		return fmt.Errorf("金币不足: 需要 %d，当前 %d", op.GoldCost, r.state.Gold())
	}
	if len(op.ItemCost) > 0 {
		inventory := r.state.Inventory()
		for itemID, count := range op.ItemCost {
			if count <= 0 {
				continue
			}
			if inventory[itemID] < count {
				return fmt.Errorf("%s 不足: 需要 %d，当前 %d", flowerName(int(itemID)), count, inventory[itemID])
			}
		}
	}
	if isWaterOp(op.Kind) {
		need := int32(len(op.LandIDs))
		if need <= 0 {
			return fmt.Errorf("浇水操作缺少田地")
		}
		available, _, _ := r.state.AvailableWaterDrops(now)
		if available < need {
			return fmt.Errorf("水滴不足: 需要 %d，当前 %d", need, available)
		}
	}
	return nil
}

func (r *Runner) lockOperationWaterDrops(op *automation.PlannedOp, now time.Time) (func(), error) {
	if !isWaterOp(op.Kind) {
		return func() {}, nil
	}
	lockedWaterDrops := int32(len(op.LandIDs))
	if !r.state.LockWaterDrops(lockedWaterDrops, now) {
		return nil, fmt.Errorf("insufficient local water drops")
	}
	return func() { r.state.ReleaseWaterDropsLock(lockedWaterDrops) }, nil
}

func (r *Runner) checkCostGate(op *automation.PlannedOp, gate automation.CostGate, now time.Time) error {
	required := gate.Required
	if required <= 0 {
		return nil
	}
	switch gate.ResourceKind {
	case automation.GateResourceGold:
		available := int64(r.state.Gold())
		if available < required {
			return fmt.Errorf("%s不足: 需要 %d，当前 %d", gateLabel(gate, "金币"), required, available)
		}
	case automation.GateResourceDiamond:
		available := int64(r.state.SpendableDiamonds())
		if available < required {
			return fmt.Errorf("%s不足: 需要 %d，当前 %d", gateLabel(gate, "元宝"), required, available)
		}
		return fmt.Errorf("元宝成本操作默认不自动执行: %d", required)
	case automation.GateResourceItem:
		available := int64(r.state.Inventory()[gate.ItemID])
		if available < required {
			return fmt.Errorf("%s不足: 需要 %d，当前 %d", gateLabel(gate, flowerName(int(gate.ItemID))), required, available)
		}
	case automation.GateResourceActivityItem:
		if op == nil || op.BatchID <= 0 {
			return fmt.Errorf("%s缺少活动批次", gateLabel(gate, "活动道具"))
		}
		count, observed := r.state.ActivityItemCount(op.BatchID, gate.ItemID)
		if !observed {
			return fmt.Errorf("%s所在活动背包尚未完整同步", gateLabel(gate, "活动道具"))
		}
		available := int64(count)
		if available < required {
			return fmt.Errorf("%s不足: 需要 %d，当前 %d", gateLabel(gate, "活动道具"), required, available)
		}
	case automation.GateResourceWaterDrop:
		available, _, _ := r.state.AvailableWaterDrops(now)
		if int64(available) < required {
			return fmt.Errorf("%s不足: 需要 %d，当前 %d", gateLabel(gate, "水滴"), required, available)
		}
	default:
		if gate.Blocking() {
			if len(gate.BlockedReasons) > 0 {
				return fmt.Errorf("%s", strings.Join(gate.BlockedReasons, "; "))
			}
			return fmt.Errorf("%s 未满足", gateLabel(gate, "前置条件"))
		}
	}
	return nil
}

func gateLabel(gate automation.CostGate, fallback string) string {
	if strings.TrimSpace(gate.Label) != "" {
		return gate.Label
	}
	return fallback
}

func (r *Runner) applyHarvestBlocks(op *automation.PlannedOp, now time.Time) *automation.PlannedOp {
	if op == nil || !isHarvestOp(op.Kind) {
		return op
	}
	if len(op.LandIDs) == 0 {
		return op
	}
	blocked := make(map[int32]bool, len(op.LandIDs))
	anyBlocked := false
	r.mu.RLock()
	for _, id := range op.LandIDs {
		until := r.harvestBlockedUntil[id]
		if until.IsZero() || !now.Before(until) {
			continue
		}
		blocked[id] = true
		anyBlocked = true
	}
	r.mu.RUnlock()
	if !anyBlocked {
		return op
	}
	landIDs := make([]int32, 0, len(op.LandIDs)-len(blocked))
	for _, id := range op.LandIDs {
		if !blocked[id] {
			landIDs = append(landIDs, id)
		}
	}
	if len(landIDs) == 0 {
		return nil
	}
	cp := *op
	cp.LandIDs = landIDs
	cp.Count = int32(len(landIDs))
	return &cp
}

func isWaterOp(kind string) bool {
	return kind == clientproto.RPCUsrLandWater.String() ||
		kind == clientproto.RPCUsrLandWaterBatch.String()
}

func (r *Runner) setHarvestBlockedUntil(landIDs []int32, until time.Time) {
	if len(landIDs) == 0 {
		return
	}
	r.mu.Lock()
	if r.harvestBlockedUntil == nil {
		r.harvestBlockedUntil = make(map[int32]time.Time)
	}
	for _, id := range landIDs {
		r.harvestBlockedUntil[id] = until
	}
	r.mu.Unlock()
}

func isHarvestOp(kind string) bool {
	return kind == clientproto.RPCUsrLandHarvest.String() ||
		kind == clientproto.RPCFmlLandHarvest.String() ||
		kind == clientproto.RPCFmlLandHarvestAll.String()
}

func (r *Runner) ensurePlannedOperationRqst(ctx context.Context, op *automation.PlannedOp) error {
	if op == nil {
		return nil
	}
	if isHarvestOp(op.Kind) {
		return r.ensureHarvestRqst(ctx)
	}
	if op.Kind == clientproto.RPCUsrLandPlant.String() ||
		op.Kind == clientproto.RPCUsrLandPlantBatch.String() {
		return r.ensurePlantRqst(ctx)
	}
	if isWaterOp(op.Kind) {
		return r.ensureWaterRqst(ctx)
	}
	if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() ||
		op.Kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() ||
		op.Kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String() ||
		op.Kind == clientproto.RPCOrderFlowerRecvOrderRwd.String() {
		return r.ensureFlowerOrderRqst(ctx)
	}
	if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() ||
		op.Kind == clientproto.RPCOrderCustomerGenOrder.String() ||
		op.Kind == clientproto.RPCOrderCustomerRejectOrder.String() ||
		op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() {
		return r.ensureCustomerOrderRqst(ctx)
	}
	return nil
}

func isFlowerNotMatureError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "鲜花尚未成熟")
}

func isResidentOrderCooldownError(kind string, err error) bool {
	return (kind == clientproto.RPCOrderFlowerFinishOrder.String() ||
		kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() ||
		kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String()) &&
		err != nil && strings.Contains(err.Error(), "冷却中")
}

func isResidentOrderDailyLimitError(kind string, err error) bool {
	return (kind == clientproto.RPCOrderFlowerFinishOrder.String() ||
		kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() ||
		kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String()) &&
		err != nil && strings.Contains(err.Error(), "今日完成订单次数已达上限")
}

func isWaterwheelInvalidDataError(kind string, err error) bool {
	return kind == clientproto.RPCWaterwheelRecv.String() && err != nil && strings.Contains(err.Error(), "数据有误")
}

func isWaterwheelDailyLimitError(kind string, err error) bool {
	return kind == clientproto.RPCWaterwheelRecv.String() && err != nil && strings.Contains(err.Error(), "已达到领取上限")
}

func isWaterDropResourceRejectedError(kind string, err error) bool {
	if !isWaterOp(kind) || err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, `"code":301`) && strings.Contains(msg, `"iid":7`)
}

func isRaceTakeAlreadyTakenError(kind string, err error) bool {
	if kind != clientproto.RPCFmlRaceTakeTask.String() || err == nil {
		return false
	}
	var rpcErr *babigame.RPCServerError
	if !errors.As(err, &rpcErr) || rpcErr == nil {
		return false
	}
	return rpcErr.Envelope.ErrorCodeOfLangJS() == "fmlRace_tips1"
}

func isFmlFlowerTakeDailyLimitError(kind string, err error) bool {
	if kind != clientproto.RPCFmlFlowerShareTake.String() || err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "今日拿取次数已达上限") || strings.Contains(msg, "fmlShare_tips8") {
		return true
	}
	var rpcErr *babigame.RPCServerError
	if errors.As(err, &rpcErr) && rpcErr != nil {
		if rpcErr.Envelope.ErrorCodeOfLangJS() == "fmlShare_tips8" {
			return true
		}
		if strings.Contains(rpcErr.Envelope.ErrorMsg(), "今日拿取次数已达上限") {
			return true
		}
	}
	return false
}

func isTaskGroupFinishedError(kind string, err error) bool {
	if err == nil {
		return false
	}
	switch kind {
	case clientproto.RPCTaskDlyRecv.String(), clientproto.RPCTaskWeekRecv.String(), clientproto.RPCTaskAchRecv.String():
		return strings.Contains(err.Error(), "本组任务已经完结")
	default:
		return false
	}
}

func waterResponseIncludesDrops(raw json.RawMessage) bool {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return false
	}
	raw7, ok := top["7"]
	if !ok {
		return false
	}
	var ns7 map[string]json.RawMessage
	if err := json.Unmarshal(raw7, &ns7); err != nil {
		return false
	}
	if raw0, ok := ns7["0"]; ok && nestedMapHasItem(raw0, "32", "7") {
		return true
	}
	raw2, ok := ns7["2"]
	if !ok {
		return false
	}
	return nestedMapHasItem(raw2, "0", "7") || nestedMapHasItem(raw2, "2", "7")
}

func nestedMapHasItem(raw json.RawMessage, field, itemID string) bool {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil {
		return false
	}
	innerRaw, ok := outer[field]
	if !ok {
		return false
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(innerRaw, &inner); err != nil {
		return false
	}
	_, ok = inner[itemID]
	return ok
}

func (r *Runner) tickWaterSourceSync(ctx context.Context, client *babigame.Client, session *babigame.Session) {
	now := time.Now()
	if !r.state.WaterwheelEnterDue(now) {
		return
	}
	if !r.lastWaterSyncTick.IsZero() && now.Sub(r.lastWaterSyncTick) < waterSourceSyncPeriod {
		return
	}

	rpc := r.runnerRPC(client, session)
	_, d, err := rpcResult(rpc.Waterwheel().Enter(ctx, clientproto.WaterwheelEnterRequest{}))
	if r.isSessionInvalidated() {
		return
	}
	if err != nil {
		r.log.Debug("waterwheel sync failed", "err", err)
		return
	}
	if d.IsError() {
		return
	}
	r.lastWaterSyncTick = now
	r.state.MarkWaterwheelEntered(now)
}
