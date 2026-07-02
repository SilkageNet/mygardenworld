package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientrpc"
)

const (
	harvestRetryWait      = 30 * time.Second
	waterSourceSyncPeriod = 60 * time.Second
)

func (r *Runner) decisionLoop(ctx context.Context) {
	for {
		interval := r.tickInterval()
		r.setNextDecisionAt(time.Now().Add(interval))
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			r.tick(ctx)
		}
	}
}

func (r *Runner) tickInterval() time.Duration {
	r.mu.RLock()
	p := r.policy
	r.mu.RUnlock()
	d := time.Duration(0)
	if p != nil {
		d = time.Duration(p.GetDecisionIntervalSeconds() * float64(time.Second))
	}
	if d <= 0 {
		d = 4 * time.Second
	}
	return d
}

func (r *Runner) tick(ctx context.Context) {
	r.mu.RLock()
	p := r.policy
	client := r.client
	session := r.session
	sessionInvalidated := r.sessionInvalidated
	r.mu.RUnlock()

	if sessionInvalidated {
		return
	}
	if client == nil || session == nil {
		return
	}
	now := time.Now()
	r.state.RefreshWaterDrops(now)
	r.tickWaterSourceSync(ctx, client, session)
	if r.isSessionInvalidated() {
		return
	}
	if p == nil || !p.AutomationEnabled {
		return
	}

	op := automation.Plan(r.state, p, now)
	if op == nil {
		return
	}
	op = r.applyHarvestBlocks(op, now)
	if op == nil {
		return
	}

	var opErr error
	finishOperation := r.beginOperation(op.Kind)
	defer func() { finishOperation(opErr) }()

	if err := r.checkOperationResources(op, now); err != nil {
		opErr = err
		r.emit(Event{
			Kind:        "operation_failed",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 已阻塞: 资源前置校验失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(op, nil, nil, err),
			Level:       "warn",
		})
		_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, nil, map[string]any{"error": err.Error(), "stage": "resource_gate"})
		return
	}

	if err := r.ensurePlannedOperationRqst(ctx, op); err != nil {
		opErr = fmt.Errorf("rqst: %w", err)
		r.emit(Event{
			Kind:        "operation_failed",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 已跳过: 前置校验失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(op, nil, nil, err),
			Level:       "warn",
		})
		_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, nil, map[string]any{"error": err.Error(), "stage": "rqst"})
		return
	}

	lockedWaterDrops := int32(0)
	if isWaterOp(op.Kind) {
		lockedWaterDrops = int32(len(op.LandIDs))
		if !r.state.LockWaterDrops(lockedWaterDrops, now) {
			opErr = fmt.Errorf("insufficient local water drops")
			return
		}
		defer r.state.ReleaseWaterDropsLock(lockedWaterDrops)
	}
	args, err := operationArgs(op)
	if err != nil {
		opErr = err
		r.emit(Event{
			Kind:        "operation_failed",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "failed",
			Message:     fmt.Sprintf("%s 失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(op, nil, nil, err),
		})
		_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, nil, map[string]any{"error": err.Error()})
		return
	}
	r.emit(Event{
		Kind:        "operation_planned",
		Category:    op.Category,
		Domain:      op.Domain,
		Action:      op.Action,
		Message:     fmt.Sprintf("计划执行 %s (田地=%v)", opDesc(op), op.LandIDs),
		PayloadJSON: operationPayload(op, args, nil, nil),
	})
	v, err := r.executePlannedOp(ctx, client, session, op)
	if err != nil {
		opErr = err
		if isHarvestOp(op.Kind) && isFlowerNotMatureError(err) {
			r.setHarvestBlockedUntil(op.LandIDs, time.Now().Add(harvestRetryWait))
			r.emit(Event{
				Kind:        "operation_failed",
				Category:    op.Category,
				Domain:      op.Domain,
				Action:      "blocked",
				Message:     fmt.Sprintf("%s 暂缓: 服务端提示鲜花尚未成熟，稍后重试 (田地=%v)", opDesc(op), op.LandIDs),
				PayloadJSON: operationPayload(op, args, nil, err),
				Level:       "warn",
			})
			_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, map[string]any{"error": err.Error(), "retryAfterSeconds": int(harvestRetryWait.Seconds())})
			return
		}
		r.emit(Event{
			Kind:        "operation_failed",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "failed",
			Message:     fmt.Sprintf("%s 失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(op, args, nil, err),
		})
		_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, map[string]any{"error": err.Error()})
		return
	}
	r.emit(Event{
		Kind:        "operation_ack",
		Category:    op.Category,
		Domain:      op.Domain,
		Action:      op.Action,
		Message:     fmt.Sprintf("%s 完成 (田地=%v)", opDesc(op), op.LandIDs),
		PayloadJSON: operationPayload(op, args, v, nil),
	})
	_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, json.RawMessage(v))

	// Some successful water RPC responses omit inventory deltas. When the
	// server did include item 7, ApplyV has already installed the authoritative
	// remaining drop count, so do not spend it locally again.
	if isWaterOp(op.Kind) && !waterResponseIncludesDrops(v) {
		r.state.MarkLandsWatered(op.LandIDs)
	}
}

func (r *Runner) checkOperationResources(op *automation.PlannedOp, now time.Time) error {
	if op == nil {
		return nil
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
	if op.Kind == "usrLand.harvest" {
		return nil
	}

	for _, id := range op.LandIDs {
		if blocked[id] {
			continue
		}
		return &automation.PlannedOp{
			Kind:     "usrLand.harvest",
			Category: op.Category,
			Domain:   op.Domain,
			Action:   op.Action,
			Reason:   op.Reason,
			Priority: op.Priority,
			LandIDs:  []int32{id},
		}
	}
	return nil
}

func (r *Runner) executePlannedOp(ctx context.Context, client *babigame.Client, session *babigame.Session, op *automation.PlannedOp) (json.RawMessage, error) {
	if op == nil {
		return nil, fmt.Errorf("nil planned operation")
	}
	rpc := clientrpc.NewClient(babigame.NewRPCClient(
		client,
		session,
		babigame.WithDefaultTimeout(30*time.Second),
		babigame.WithApplyV(r.state.ApplyV),
	))
	rawRPC := babigame.NewRPCClient(
		client,
		session,
		babigame.WithDefaultTimeout(30*time.Second),
		babigame.WithApplyV(r.state.ApplyV),
	)
	switch op.Kind {
	case clientproto.RPCUsrLandHarvestOneKey.String():
		resp, err := rpc.UsrLand().HarvestOneKey(ctx, clientproto.UsrLandHarvestOneKeyRequest{})
		return resp.Payload, err
	case clientproto.RPCUsrLandHarvest.String():
		landID, err := plannedOpSingleLandID(op)
		if err != nil {
			return nil, err
		}
		resp, err := rpc.UsrLand().Harvest(ctx, clientproto.UsrLandHarvestRequest{LandId: landID})
		return resp.Payload, err
	case clientproto.RPCUsrLandPlantBatch.String():
		if op.FlowerID == 0 {
			return nil, fmt.Errorf("plantBatch missing flower id")
		}
		resp, err := rpc.UsrLand().PlantBatch(ctx, clientproto.UsrLandPlantBatchRequest{LandIds: op.LandIDs, FlowerId: op.FlowerID})
		return resp.Payload, err
	case clientproto.RPCUsrLandPlant.String():
		if op.FlowerID == 0 {
			return nil, fmt.Errorf("plant missing flower id")
		}
		landID, err := plannedOpSingleLandID(op)
		if err != nil {
			return nil, err
		}
		resp, err := rpc.UsrLand().Plant(ctx, clientproto.UsrLandPlantRequest{LandId: landID, FlowerId: op.FlowerID})
		return resp.Payload, err
	case clientproto.RPCUsrLandWaterBatch.String():
		resp, err := rpc.UsrLand().WaterBatch(ctx, clientproto.UsrLandWaterBatchRequest{LandIds: op.LandIDs})
		return resp.Payload, err
	case clientproto.RPCUsrLandWaterOneKey.String():
		resp, err := rpc.UsrLand().WaterOneKey(ctx, clientproto.UsrLandWaterOneKeyRequest{})
		return resp.Payload, err
	case clientproto.RPCUsrLandWater.String():
		landID, err := plannedOpSingleLandID(op)
		if err != nil {
			return nil, err
		}
		resp, err := rpc.UsrLand().Water(ctx, clientproto.UsrLandWaterRequest{LandId: landID})
		return resp.Payload, err
	case clientproto.RPCUsrLandUnlockLand.String():
		v, d, err := rpcResult(rpc.UsrLand().UnlockLand(ctx, clientproto.UsrLandUnlockLandRequest{LandId: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCUsrLandSpeedUpBatch.String():
		v, d, err := rpcResult(rpc.UsrLand().SpeedUpBatch(ctx, clientproto.UsrLandSpeedUpBatchRequest{LandIds: op.LandIDs}))
		return checkedPayload(v, d, err)
	case clientproto.RPCCultivateRecv.String():
		v, d, err := rpcResult(rpc.Cultivate().Recv(ctx, clientproto.CultivateRecvRequest{FlowerId: op.FlowerID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCCultivateUpgrade.String():
		v, d, err := rpcResult(rpc.Cultivate().Upgrade(ctx, clientproto.CultivateUpgradeRequest{FlowerId: op.FlowerID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCCultivateCultivate.String():
		v, d, err := rpcResult(rpc.Cultivate().Cultivate(ctx, clientproto.CultivateCultivateRequest{FlowerId: op.FlowerID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCOrderFlowerFinishOrder.String():
		v, d, err := rpcResult(rpc.OrderFlower().FinishOrder(ctx, clientproto.OrderFlowerFinishOrderRequest{BoxId: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCOrderFlowerRecvOrderRwd.String():
		v, d, err := rpcResult(rpc.OrderFlower().RecvOrderRwd(ctx, clientproto.OrderFlowerRecvOrderRwdRequest{Target: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCOrderCustomerFinishOrder.String():
		v, d, err := rpcResult(rpc.OrderCustomer().FinishOrder(ctx, clientproto.OrderCustomerFinishOrderRequest{NPCId: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCFlowerArtMakeFlowerArt.String():
		if op.VaseID <= 0 || len(op.FlowerIDs) == 0 || op.Count <= 0 {
			return nil, fmt.Errorf("flower art craft missing vase/flowers/count")
		}
		v, d, err := rpcResult(rpc.FlowerArt().MakeFlowerArt(ctx, clientproto.FlowerArtMakeFlowerArtRequest{
			VaseId:     op.VaseID,
			FlowersIds: op.FlowerIDs,
			Num:        op.Count,
		}))
		return checkedPayload(v, d, err)
	case clientproto.RPCCollectRwdRecv.String():
		v, d, err := rpcResult(rpc.CollectRwd().Recv(ctx, clientproto.CollectRwdRecvRequest{Type: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCCollectRwdRecvArtCreateRwdByVase.String():
		v, d, err := rpcResult(rpc.CollectRwd().RecvArtCreateRwdByVase(ctx, clientproto.CollectRwdRecvArtCreateRwdByVaseRequest{"flowerArtId": op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCFlowerRackSell.String():
		v, d, err := rpcResult(rpc.FlowerRack().Sell(ctx, clientproto.FlowerRackSellRequest{RackId: op.TargetID, Iid: op.ItemID, Num: op.Count}))
		return checkedPayload(v, d, err)
	case clientproto.RPCFlowerRackRecvOneKey.String():
		v, d, err := rpcResult(rpc.FlowerRack().RecvOneKey(ctx, clientproto.FlowerRackRecvOneKeyRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCWaterwheelRecv.String():
		v, d, err := rpcResult(rpc.Waterwheel().Recv(ctx, clientproto.WaterwheelRecvRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCFreeWaterRecv.String():
		v, d, err := rpcResult(rpc.FreeWater().Recv(ctx, clientproto.FreeWaterRecvRequest{Idx: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCBenefitBoxDraw.String():
		v, d, err := rpcResult(rpc.BenefitBox().Draw(ctx, clientproto.BenefitBoxDrawRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCZooEnterZoo.String():
		v, d, err := rpcResult(rpc.Zoo().EnterZoo(ctx, clientproto.ZooEnterZooRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCZooFeedPets.String():
		args := map[string]any{"petIdList": []int32{op.TargetID}}
		v, d, err := rpcResult(babigame.CallRPC[clientproto.StateDelta](ctx, rawRPC, clientproto.RPCZooFeedPets, args))
		return checkedPayload(v, d, err)
	case clientproto.RPCZooStrokePet.String():
		args := map[string]any{"petId": op.TargetID}
		v, d, err := rpcResult(babigame.CallRPC[clientproto.StateDelta](ctx, rawRPC, clientproto.RPCZooStrokePet, args))
		return checkedPayload(v, d, err)
	case clientproto.RPCUsrExtraUpdateAntiFraudQAStatus.String():
		v, d, err := rpcResult(rpc.UsrExtra().UpdateAntiFraudQAStatus(ctx, clientproto.UsrExtraUpdateAntiFraudQAStatusRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCUsrExtraRecvAntiFraudQARwd.String():
		v, d, err := rpcResult(rpc.UsrExtra().RecvAntiFraudQARwd(ctx, clientproto.UsrExtraRecvAntiFraudQARwdRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCShopCultivateEnter.String():
		v, d, err := rpcResult(rpc.ShopCultivate().Enter(ctx, clientproto.ShopCultivateEnterRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCShopCultivateBuy.String():
		v, d, err := rpcResult(rpc.ShopCultivate().Buy(ctx, clientproto.ShopCultivateBuyRequest{ShopId: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCShopGiftbagEnter.String():
		v, d, err := rpcResult(rpc.ShopGiftbag().Enter(ctx, clientproto.ShopGiftbagEnterRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCShopGiftbagBuy.String():
		v, d, err := rpcResult(rpc.ShopGiftbag().Buy(ctx, clientproto.ShopGiftbagBuyRequest{ShopId: op.TargetID, Num: op.Count}))
		return checkedPayload(v, d, err)
	case clientproto.RPCPearlRefresh.String():
		v, d, err := rpcResult(rpc.Pearl().Refresh(ctx, clientproto.PearlRefreshRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCPearlRecvDailyFree.String():
		v, d, err := rpcResult(rpc.Pearl().RecvDailyFree(ctx, clientproto.PearlRecvDailyFreeRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCPearlPlaceRecv.String():
		v, d, err := rpcResult(rpc.PearlPlace().Recv(ctx, clientproto.PearlPlaceRecvRequest{PlaceId: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCPearlSetProtectState.String():
		v, d, err := rpcResult(rpc.Pearl().SetProtectState(ctx, clientproto.PearlSetProtectStateRequest{ProtectState: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCPearlDraw.String():
		v, d, err := rpcResult(rpc.Pearl().Draw(ctx, clientproto.PearlDrawRequest{Count: op.Count}))
		return checkedPayload(v, d, err)
	case clientproto.RPCFmlBuild.String():
		v, d, err := rpcResult(rpc.Fml().Build(ctx, clientproto.FmlBuildRequest{ID: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCFmlLandHarvest.String():
		v, d, err := rpcResult(rpc.FmlLand().Harvest(ctx, clientproto.FmlLandHarvestRequest{LandIds: op.LandIDs}))
		return checkedPayload(v, d, err)
	case clientproto.RPCFmlForestRefresh.String():
		v, d, err := rpcResult(rpc.FmlForest().Refresh(ctx, clientproto.FmlForestRefreshRequest{IsAutoCollect: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCFmlFlowerShareRefresh.String():
		v, d, err := rpcResult(rpc.FmlFlowerShare().Refresh(ctx, clientproto.FmlFlowerShareRefreshRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String():
		v, d, err := rpcResult(rpc.FmlFlowerShare().GetFmlOtherShareList(ctx, clientproto.FmlFlowerShareGetFmlOtherShareListRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCFmlFlowerShareRecvRwd.String():
		v, d, err := rpcResult(rpc.FmlFlowerShare().RecvRwd(ctx, clientproto.FmlFlowerShareRecvRwdRequest{SlotIds: op.SlotIDs}))
		return checkedPayload(v, d, err)
	case clientproto.RPCFmlFlowerShareTake.String():
		v, d, err := rpcResult(rpc.FmlFlowerShare().Take(ctx, clientproto.FmlFlowerShareTakeRequest{DstUid: op.TargetUID, SlotId: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCTaskDlyRecv.String():
		v, d, err := rpcResult(rpc.TaskDly().Recv(ctx, clientproto.TaskDlyRecvRequest{ID: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCTaskWeekRecv.String():
		v, d, err := rpcResult(rpc.TaskWeek().Recv(ctx, clientproto.TaskWeekRecvRequest{ID: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCRoadGrowRecv.String():
		v, d, err := rpcResult(rpc.RoadGrow().Recv(ctx, clientproto.RoadGrowRecvRequest{ID: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCRandomEventDoAffair.String():
		v, d, err := rpcResult(rpc.RandomEvent().DoAffair(ctx, clientproto.RandomEventDoAffairRequest{EventId: op.TargetID}))
		return checkedPayload(v, d, err)
	case clientproto.RPCMailPickOneKey.String():
		if v, d, err := rpcResult(rpc.Mail().GetList(ctx, clientproto.MailGetListRequest{})); err != nil || d.IsError() {
			return checkedPayload(v, d, err)
		} else if babigame.HasPayload(v) {
			r.state.ApplyV(v)
		}
		v, d, err := rpcResult(rpc.Mail().PickOneKey(ctx, clientproto.MailPickOneKeyRequest{}))
		return checkedPayload(v, d, err)
	case clientproto.RPCSignTypeSign.String():
		if v, d, err := rpcResult(rpc.SignType().Enter(ctx, clientproto.SignTypeEnterRequest{Type: 1})); err != nil || d.IsError() {
			return checkedPayload(v, d, err)
		} else if babigame.HasPayload(v) {
			r.state.ApplyV(v)
		}
		if v, d, err := rpcResult(rpc.SignType().Sign(ctx, clientproto.SignTypeSignRequest{Type: 1})); err != nil || d.IsError() {
			return checkedPayload(v, d, err)
		} else if babigame.HasPayload(v) {
			r.state.ApplyV(v)
		}
		v, d, err := rpcResult(rpc.SignType().Recv(ctx, clientproto.SignTypeRecvRequest{Type: 1}))
		return checkedPayload(v, d, err)
	default:
		return nil, fmt.Errorf("unsupported planned operation %s", op.Kind)
	}
}

func checkedPayload(v json.RawMessage, d babigame.WSResponseD, err error) (json.RawMessage, error) {
	if err != nil {
		return nil, err
	}
	if d.IsError() {
		msg := d.ErrorMsg()
		if msg == "" {
			msg = "server returned error"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return v, nil
}

func operationArgs(op *automation.PlannedOp) (any, error) {
	if op == nil {
		return nil, fmt.Errorf("nil planned operation")
	}
	switch op.Kind {
	case clientproto.RPCUsrLandHarvestOneKey.String():
		return clientproto.UsrLandHarvestOneKeyRequest{}, nil
	case clientproto.RPCUsrLandHarvest.String():
		landID, err := plannedOpSingleLandID(op)
		if err != nil {
			return nil, err
		}
		return clientproto.UsrLandHarvestRequest{LandId: landID}, nil
	case clientproto.RPCUsrLandPlantBatch.String():
		if op.FlowerID == 0 {
			return nil, fmt.Errorf("plantBatch missing flower id")
		}
		return clientproto.UsrLandPlantBatchRequest{LandIds: op.LandIDs, FlowerId: op.FlowerID}, nil
	case clientproto.RPCUsrLandPlant.String():
		if op.FlowerID == 0 {
			return nil, fmt.Errorf("plant missing flower id")
		}
		landID, err := plannedOpSingleLandID(op)
		if err != nil {
			return nil, err
		}
		return clientproto.UsrLandPlantRequest{LandId: landID, FlowerId: op.FlowerID}, nil
	case clientproto.RPCUsrLandWaterBatch.String():
		return clientproto.UsrLandWaterBatchRequest{LandIds: op.LandIDs}, nil
	case clientproto.RPCUsrLandWaterOneKey.String():
		return clientproto.UsrLandWaterOneKeyRequest{}, nil
	case clientproto.RPCUsrLandWater.String():
		landID, err := plannedOpSingleLandID(op)
		if err != nil {
			return nil, err
		}
		return clientproto.UsrLandWaterRequest{LandId: landID}, nil
	case clientproto.RPCUsrLandUnlockLand.String():
		return clientproto.UsrLandUnlockLandRequest{LandId: op.TargetID}, nil
	case clientproto.RPCUsrLandSpeedUpBatch.String():
		return clientproto.UsrLandSpeedUpBatchRequest{LandIds: op.LandIDs}, nil
	case clientproto.RPCCultivateRecv.String():
		return clientproto.CultivateRecvRequest{FlowerId: op.FlowerID}, nil
	case clientproto.RPCCultivateUpgrade.String():
		return clientproto.CultivateUpgradeRequest{FlowerId: op.FlowerID}, nil
	case clientproto.RPCCultivateCultivate.String():
		return clientproto.CultivateCultivateRequest{FlowerId: op.FlowerID}, nil
	case clientproto.RPCOrderFlowerFinishOrder.String():
		return clientproto.OrderFlowerFinishOrderRequest{BoxId: op.TargetID}, nil
	case clientproto.RPCOrderFlowerRecvOrderRwd.String():
		return clientproto.OrderFlowerRecvOrderRwdRequest{Target: op.TargetID}, nil
	case clientproto.RPCOrderCustomerFinishOrder.String():
		return clientproto.OrderCustomerFinishOrderRequest{NPCId: op.TargetID}, nil
	case clientproto.RPCFlowerArtMakeFlowerArt.String():
		return clientproto.FlowerArtMakeFlowerArtRequest{VaseId: op.VaseID, FlowersIds: op.FlowerIDs, Num: op.Count}, nil
	case clientproto.RPCCollectRwdRecv.String():
		return clientproto.CollectRwdRecvRequest{Type: op.TargetID}, nil
	case clientproto.RPCCollectRwdRecvArtCreateRwdByVase.String():
		return clientproto.CollectRwdRecvArtCreateRwdByVaseRequest{"flowerArtId": op.TargetID}, nil
	case clientproto.RPCFlowerRackSell.String():
		return clientproto.FlowerRackSellRequest{RackId: op.TargetID, Iid: op.ItemID, Num: op.Count}, nil
	case clientproto.RPCFlowerRackRecvOneKey.String():
		return clientproto.FlowerRackRecvOneKeyRequest{}, nil
	case clientproto.RPCWaterwheelRecv.String():
		return clientproto.WaterwheelRecvRequest{}, nil
	case clientproto.RPCFreeWaterRecv.String():
		return clientproto.FreeWaterRecvRequest{Idx: op.TargetID}, nil
	case clientproto.RPCBenefitBoxDraw.String():
		return clientproto.BenefitBoxDrawRequest{}, nil
	case clientproto.RPCZooEnterZoo.String():
		return clientproto.ZooEnterZooRequest{}, nil
	case clientproto.RPCZooFeedPets.String():
		return map[string]any{"petIdList": []int32{op.TargetID}}, nil
	case clientproto.RPCZooStrokePet.String():
		return map[string]any{"petId": op.TargetID}, nil
	case clientproto.RPCUsrExtraUpdateAntiFraudQAStatus.String():
		return clientproto.UsrExtraUpdateAntiFraudQAStatusRequest{}, nil
	case clientproto.RPCUsrExtraRecvAntiFraudQARwd.String():
		return clientproto.UsrExtraRecvAntiFraudQARwdRequest{}, nil
	case clientproto.RPCShopCultivateEnter.String():
		return clientproto.ShopCultivateEnterRequest{}, nil
	case clientproto.RPCShopCultivateBuy.String():
		return clientproto.ShopCultivateBuyRequest{ShopId: op.TargetID}, nil
	case clientproto.RPCShopGiftbagEnter.String():
		return clientproto.ShopGiftbagEnterRequest{}, nil
	case clientproto.RPCShopGiftbagBuy.String():
		return clientproto.ShopGiftbagBuyRequest{ShopId: op.TargetID, Num: op.Count}, nil
	case clientproto.RPCPearlRefresh.String():
		return clientproto.PearlRefreshRequest{}, nil
	case clientproto.RPCPearlRecvDailyFree.String():
		return clientproto.PearlRecvDailyFreeRequest{}, nil
	case clientproto.RPCPearlPlaceRecv.String():
		return clientproto.PearlPlaceRecvRequest{PlaceId: op.TargetID}, nil
	case clientproto.RPCPearlSetProtectState.String():
		return clientproto.PearlSetProtectStateRequest{ProtectState: op.TargetID}, nil
	case clientproto.RPCPearlDraw.String():
		return clientproto.PearlDrawRequest{Count: op.Count}, nil
	case clientproto.RPCFmlBuild.String():
		return clientproto.FmlBuildRequest{ID: op.TargetID}, nil
	case clientproto.RPCFmlLandHarvest.String():
		return clientproto.FmlLandHarvestRequest{LandIds: op.LandIDs}, nil
	case clientproto.RPCFmlForestRefresh.String():
		return clientproto.FmlForestRefreshRequest{IsAutoCollect: op.TargetID}, nil
	case clientproto.RPCFmlFlowerShareRefresh.String():
		return clientproto.FmlFlowerShareRefreshRequest{}, nil
	case clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String():
		return clientproto.FmlFlowerShareGetFmlOtherShareListRequest{}, nil
	case clientproto.RPCFmlFlowerShareRecvRwd.String():
		return clientproto.FmlFlowerShareRecvRwdRequest{SlotIds: op.SlotIDs}, nil
	case clientproto.RPCFmlFlowerShareTake.String():
		return clientproto.FmlFlowerShareTakeRequest{DstUid: op.TargetUID, SlotId: op.TargetID}, nil
	case clientproto.RPCTaskDlyRecv.String():
		return clientproto.TaskDlyRecvRequest{ID: op.TargetID}, nil
	case clientproto.RPCTaskWeekRecv.String():
		return clientproto.TaskWeekRecvRequest{ID: op.TargetID}, nil
	case clientproto.RPCRoadGrowRecv.String():
		return clientproto.RoadGrowRecvRequest{ID: op.TargetID}, nil
	case clientproto.RPCRandomEventDoAffair.String():
		return clientproto.RandomEventDoAffairRequest{EventId: op.TargetID}, nil
	case clientproto.RPCMailPickOneKey.String():
		return clientproto.MailPickOneKeyRequest{}, nil
	case clientproto.RPCSignTypeSign.String():
		return clientproto.SignTypeSignRequest{Type: 1}, nil
	default:
		return nil, fmt.Errorf("unsupported planned operation %s", op.Kind)
	}
}

func isWaterOp(kind string) bool {
	return kind == clientproto.RPCUsrLandWater.String() ||
		kind == clientproto.RPCUsrLandWaterBatch.String() ||
		kind == clientproto.RPCUsrLandWaterOneKey.String()
}

func plannedOpSingleLandID(op *automation.PlannedOp) (int32, error) {
	if op == nil || len(op.LandIDs) != 1 || op.LandIDs[0] == 0 {
		return 0, fmt.Errorf("operation %s requires exactly one land id", op.Kind)
	}
	return op.LandIDs[0], nil
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
	return kind == "usrLand.harvest" || kind == "usrLand.harvestOneKey"
}

func (r *Runner) ensurePlannedOperationRqst(ctx context.Context, op *automation.PlannedOp) error {
	if op == nil {
		return nil
	}
	if isHarvestOp(op.Kind) {
		return r.ensureHarvestRqst(ctx)
	}
	if op.Kind == clientproto.RPCUsrLandPlant.String() ||
		op.Kind == clientproto.RPCUsrLandPlantBatch.String() ||
		op.Kind == clientproto.RPCUsrLandPlantOneKey.String() {
		return r.ensurePlantRqst(ctx)
	}
	if isWaterOp(op.Kind) {
		return r.ensureWaterRqst(ctx)
	}
	if op.Kind == clientproto.RPCOrderFlowerFinishOrder.String() ||
		op.Kind == clientproto.RPCOrderFlowerRecvOrderRwd.String() {
		return r.ensureFlowerOrderRqst(ctx)
	}
	if op.Kind == clientproto.RPCOrderCustomerFinishOrder.String() ||
		op.Kind == clientproto.RPCFlowerArtMakeFlowerArt.String() {
		return r.ensureCustomerOrderRqst(ctx)
	}
	return nil
}

func isFlowerNotMatureError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "鲜花尚未成熟")
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
	if time.Since(r.lastWaterSyncTick) < waterSourceSyncPeriod {
		return
	}
	r.lastWaterSyncTick = time.Now()

	rpc := r.runnerRPC(client, session)
	v, d, err := rpcResult(rpc.Waterwheel().Enter(ctx, clientproto.WaterwheelEnterRequest{}))
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
	if babigame.HasPayload(v) {
		r.state.ApplyV(v)
	}
}

func operationPayload(op *automation.PlannedOp, args any, raw json.RawMessage, err error) string {
	payload := map[string]any{
		"rpc":       op.Kind,
		"category":  op.Category,
		"domain":    op.Domain,
		"action":    op.Action,
		"priority":  op.Priority,
		"reason":    op.Reason,
		"label":     opDesc(op),
		"landIds":   op.LandIDs,
		"slotIds":   op.SlotIDs,
		"args":      args,
		"flowerId":  op.FlowerID,
		"targetUid": op.TargetUID,
		"targetId":  op.TargetID,
		"itemId":    op.ItemID,
		"count":     op.Count,
		"vaseId":    op.VaseID,
		"flowerIds": op.FlowerIDs,
	}
	if len(raw) > 0 {
		payload["raw"] = json.RawMessage(raw)
	}
	if err != nil {
		payload["error"] = err.Error()
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func opDesc(op *automation.PlannedOp) string {
	desc := opKindDesc(op.Kind)
	if op.FlowerID == 0 {
		return desc
	}
	return fmt.Sprintf("%s %s(#%d)", desc, flowerName(int(op.FlowerID)), op.FlowerID)
}
