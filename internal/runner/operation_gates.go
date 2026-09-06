package runner

import (
	"fmt"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
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
	// Defense in depth: even if a future planner regression emits one of these
	// operations as executable, do not send a request that depends on fabricated
	// advertising SDK callbacks or tokens.
	switch op.Kind {
	case clientproto.RPCFmlRaceUpgradeTask.String():
		if !r.Policy().GetAutomationEnabled() {
			return fmt.Errorf("自动化已暂停，不消费元宝")
		}
		if err := automation.ValidateRaceUpgrade(r.state, r.Policy().GetUnion().GetRace(), op, now); err != nil {
			return err
		}
		r.mu.RLock()
		attempted := r.raceUpgradeAttempts[[2]int64{op.RaceBatchID, op.TaskMsID}]
		r.mu.RUnlock()
		if attempted {
			return fmt.Errorf("此任务已提交过升级，不重复消费元宝")
		}
	case clientproto.RPCShopBuy.String():
		if err := automation.ValidateZooFoodPurchase(r.state, r.Policy().GetBasic().GetZoo(), op, now); err != nil {
			return err
		}
	case clientproto.RPCShopGiftbagBuy.String():
		for _, offer := range r.state.ShopGiftbagOffers() {
			if offer.ShopID == op.TargetID && offer.ShareID > 0 {
				return fmt.Errorf("%s: 视频礼包 shareId=%d", automation.SDKAdUnsupportedReason, offer.ShareID)
			}
		}
	case clientproto.RPCFmlBld.String():
		if option, ok := state.FmlBuildOptionByID(op.TargetID); ok && option.ShareID > 0 {
			return fmt.Errorf("%s: 公会建设 shareId=%d", automation.SDKAdUnsupportedReason, option.ShareID)
		}
	case clientproto.RPCFrdStealSteal.String(), clientproto.RPCFrdExtBuyStealCnt.String():
		if err := automation.ValidateFriendTouchMutation(r.state, r.Policy().GetPlant().GetFriendSteal(), op, now); err != nil {
			return err
		}
	}
	for _, gate := range op.CostGates {
		if err := r.checkCostGate(op, gate, now); err != nil {
			return err
		}
	}
	if op.DiamondCost > 0 && op.Kind != clientproto.RPCFmlRaceUpgradeTask.String() {
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
		if op.Kind != clientproto.RPCFmlRaceUpgradeTask.String() {
			return fmt.Errorf("元宝成本操作默认不自动执行: %d", required)
		}
	case automation.GateResourceItem:
		available := int64(r.state.Inventory()[gate.ItemID])
		if available < required {
			return fmt.Errorf("%s不足: 需要 %d，当前 %d", gateLabel(gate, flowerName(int(gate.ItemID))), required, available)
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
