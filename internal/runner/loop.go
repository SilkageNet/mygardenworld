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
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientrpc"
)

const (
	harvestRetryWait      = 30 * time.Second
	harvestRPCInterval    = 120 * time.Millisecond
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
	if err := r.enforceReputationGuard(ctx, client, session, "tick", now); err != nil {
		if isReputationGuardError(err) {
			r.Stop()
		}
		return
	}
	if p == nil || !p.AutomationEnabled {
		return
	}

	r.emitCustomerOrderInfo()

	op := r.nextRunnableOperation(p, now)
	if op == nil {
		return
	}

	var opErr error
	finishOperation := r.beginOperation(op.Kind)
	defer func() { finishOperation(opErr) }()

	if err := r.checkOperationResources(op, now); err != nil {
		opErr = err
		payloadOp := r.cooldownSideOperation(op, time.Now(), err, "", 0)
		r.emit(Event{
			Kind:        "operation_failed",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 已阻塞: 资源前置校验失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(payloadOp, nil, nil, err),
			Level:       "warn",
		})
		_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, nil, map[string]any{"error": err.Error(), "stage": "resource_gate"})
		return
	}

	if err := r.ensurePlannedOperationRqst(ctx, op); err != nil {
		opErr = fmt.Errorf("rqst: %w", err)
		payloadOp := r.cooldownSideOperation(op, time.Now(), opErr, "前置校验失败，暂缓重试", 0)
		r.emit(Event{
			Kind:        "operation_failed",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Message:     fmt.Sprintf("%s 已跳过: 前置校验失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(payloadOp, nil, nil, err),
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
		payloadOp := r.cooldownSideOperation(op, time.Now(), err, "", 0)
		r.emit(Event{
			Kind:        "operation_failed",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "failed",
			Message:     fmt.Sprintf("%s 失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(payloadOp, nil, nil, err),
		})
		_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, nil, map[string]any{"error": err.Error()})
		return
	}
	r.emit(Event{
		Kind:        "operation_planned",
		Category:    op.Category,
		Domain:      op.Domain,
		Action:      op.Action,
		Message:     fmt.Sprintf("计划执行 %s%s", opDesc(op), r.opSuffix(op)),
		PayloadJSON: operationPayload(op, args, nil, nil),
	})
	v, err := r.executePlannedOp(ctx, client, session, op)
	if err != nil {
		if isHarvestOp(op.Kind) && isFlowerNotMatureError(err) {
			landIDs := op.LandIDs
			var landErr *harvestLandError
			if errors.As(err, &landErr) && landErr.LandID != 0 {
				landIDs = []int32{landErr.LandID}
			}
			r.setHarvestBlockedUntil(landIDs, time.Now().Add(harvestRetryWait))
			r.emit(Event{
				Kind:        "operation_deferred",
				Category:    op.Category,
				Domain:      op.Domain,
				Action:      "blocked",
				Message:     fmt.Sprintf("%s 暂缓: 服务端提示鲜花尚未成熟，稍后重试 (田地=%v)", opDesc(op), landIDs),
				PayloadJSON: operationPayload(op, args, nil, err),
				Level:       "warn",
			})
			_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, map[string]any{"error": err.Error(), "retryAfterSeconds": int(harvestRetryWait.Seconds())})
			return
		}
		if isResidentOrderCooldownError(op.Kind, err) {
			payloadOp := r.cooldownSideOperation(op, time.Now(), err, "服务端提示订单冷却中", 30*time.Second)
			r.emit(Event{
				Kind:        "operation_deferred",
				Category:    op.Category,
				Domain:      op.Domain,
				Action:      "blocked",
				Message:     fmt.Sprintf("%s 暂缓: 服务端提示订单冷却中，稍后重试", opDesc(op)),
				PayloadJSON: operationPayload(payloadOp, args, nil, err),
				Level:       "warn",
			})
			_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, map[string]any{"error": err.Error(), "stage": "cooldown"})
			return
		}
		if isResidentOrderDailyLimitError(op.Kind, err) {
			now := time.Now()
			r.state.MarkResidentOrderDailyLimitReached(now)
			until, ok := r.state.ResidentOrderDailyLimitReached(now)
			cooldown := 24 * time.Hour
			if ok {
				cooldown = until.Sub(now)
			}
			payloadOp := r.cooldownSideOperation(op, now, err, "服务端提示今日完成订单次数已达上限", cooldown)
			r.emit(Event{
				Kind:        "operation_deferred",
				Category:    op.Category,
				Domain:      op.Domain,
				Action:      "blocked",
				Message:     fmt.Sprintf("%s 暂停: 服务端提示今日完成订单次数已达上限，已跳过居民订单以继续执行其他流程", opDesc(op)),
				PayloadJSON: operationPayload(payloadOp, args, nil, err),
				Level:       "warn",
			})
			_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, map[string]any{"error": err.Error(), "stage": "daily_limit"})
			return
		}
		if isWaterwheelInvalidDataError(op.Kind, err) {
			r.state.MarkWaterwheelUnavailable(time.Now())
			payloadOp := r.cooldownSideOperation(op, time.Now(), err, "服务端提示水车数据暂不可领取", time.Minute)
			r.emit(Event{
				Kind:        "operation_deferred",
				Category:    op.Category,
				Domain:      op.Domain,
				Action:      "blocked",
				Message:     fmt.Sprintf("%s 暂缓: 服务端提示水车数据暂不可领取，稍后重试", opDesc(op)),
				PayloadJSON: operationPayload(payloadOp, args, nil, err),
				Level:       "warn",
			})
			_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, map[string]any{"error": err.Error(), "stage": "invalid_state"})
			return
		}
		if isWaterwheelDailyLimitError(op.Kind, err) {
			r.state.MarkWaterwheelDailyLimitReached(time.Now())
			payloadOp := r.cooldownSideOperation(op, time.Now(), err, "服务端提示今日水车领取已达上限", 24*time.Hour)
			r.emit(Event{
				Kind:        "operation_deferred",
				Category:    op.Category,
				Domain:      op.Domain,
				Action:      "blocked",
				Message:     fmt.Sprintf("%s 暂停: 服务端提示今日水车领取已达上限，已跳过水车以继续执行其他任务", opDesc(op)),
				PayloadJSON: operationPayload(payloadOp, args, nil, err),
				Level:       "warn",
			})
			_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, map[string]any{"error": err.Error(), "stage": "daily_limit"})
			return
		}
		if isWaterDropResourceRejectedError(op.Kind, err) {
			r.state.MarkWaterDropsExhausted(time.Now())
			r.emit(Event{
				Kind:        "operation_deferred",
				Category:    op.Category,
				Domain:      op.Domain,
				Action:      "blocked",
				Message:     fmt.Sprintf("%s 暂缓: 服务端提示水滴不足，已校正本地数量，等待恢复后重试", opDesc(op)),
				PayloadJSON: operationPayload(op, args, nil, err),
				Level:       "warn",
			})
			_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, map[string]any{"error": err.Error(), "stage": "resource_stale"})
			return
		}
		if isTaskGroupFinishedError(op.Kind, err) {
			payloadOp := r.cooldownSideOperation(op, time.Now(), err, "服务端提示本组任务已经完结", sideOperationMaxCooldown)
			r.emit(Event{
				Kind:        "operation_deferred",
				Category:    op.Category,
				Domain:      op.Domain,
				Action:      "blocked",
				Message:     fmt.Sprintf("%s 暂停: 服务端提示本组任务已经完结，已暂缓该任务以继续执行其他流程", opDesc(op)),
				PayloadJSON: operationPayload(payloadOp, args, nil, err),
				Level:       "warn",
			})
			_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, map[string]any{"error": err.Error(), "stage": "group_finished"})
			return
		}
		opErr = err
		payloadOp := r.cooldownSideOperation(op, time.Now(), err, "", 0)
		r.emit(Event{
			Kind:        "operation_failed",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "failed",
			Message:     fmt.Sprintf("%s 失败: %v", opDesc(op), err),
			PayloadJSON: operationPayload(payloadOp, args, nil, err),
		})
		_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, map[string]any{"error": err.Error()})
		return
	}
	r.emit(Event{
		Kind:        "operation_ack",
		Category:    op.Category,
		Domain:      op.Domain,
		Action:      op.Action,
		Message:     fmt.Sprintf("%s 完成%s", opDesc(op), r.opSuffix(op)),
		PayloadJSON: operationPayload(op, args, v, nil),
	})
	_ = r.db.LogOperation(ctx, r.account.ID, op.Kind, args, json.RawMessage(v))
	r.clearOperationCooldown(op)

	// Some successful water RPC responses omit inventory deltas. When the
	// server did include item 7, ApplyV has already installed the authoritative
	// remaining drop count, so do not spend it locally again.
	if isWaterOp(op.Kind) && !waterResponseIncludesDrops(v) {
		r.state.MarkLandsWatered(op.LandIDs)
	}
}

func (r *Runner) nextRunnableOperation(policy *pb.Policy, now time.Time) *automation.PlannedOp {
	for _, candidate := range automation.PlanOperations(r.state, policy, now) {
		if !runnablePlannedOp(candidate) {
			continue
		}
		op := candidate
		if _, ok := r.operationCoolingDown(&op, now); ok {
			continue
		}
		if filtered := r.applyHarvestBlocks(&op, now); filtered != nil {
			return filtered
		}
	}
	return nil
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
		if err := r.checkCostGate(gate, now); err != nil {
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

func (r *Runner) checkCostGate(gate automation.CostGate, now time.Time) error {
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

type operationRuntime struct {
	runner *Runner
	rpc    *clientrpc.Client
	rawRPC *babigame.RPCClient
}

type operationSpec struct {
	args func(*automation.PlannedOp) (any, error)
	run  func(context.Context, operationRuntime, *automation.PlannedOp) (json.RawMessage, error)
}

var plannedOperationSpecs = map[string]operationSpec{
	clientproto.RPCUsrLandHarvest.String(): {
		args: harvestOperationArgs,
		run:  runUsrLandHarvest,
	},
	clientproto.RPCUsrLandPlantBatch.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.UsrLandPlantBatchRequest, error) {
			if op.FlowerID == 0 {
				return clientproto.UsrLandPlantBatchRequest{}, fmt.Errorf("plantBatch missing flower id")
			}
			return clientproto.UsrLandPlantBatchRequest{LandIds: op.LandIDs, FlowerId: op.FlowerID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrLandPlantBatchRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrLand().PlantBatch(ctx, req)
		},
	),
	clientproto.RPCUsrLandPlant.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.UsrLandPlantRequest, error) {
			if op.FlowerID == 0 {
				return clientproto.UsrLandPlantRequest{}, fmt.Errorf("plant missing flower id")
			}
			landID, err := plannedOpSingleLandID(op)
			if err != nil {
				return clientproto.UsrLandPlantRequest{}, err
			}
			return clientproto.UsrLandPlantRequest{LandId: landID, FlowerId: op.FlowerID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrLandPlantRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrLand().Plant(ctx, req)
		},
	),
	clientproto.RPCUsrLandWaterBatch.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.UsrLandWaterBatchRequest, error) {
			return clientproto.UsrLandWaterBatchRequest{LandIds: op.LandIDs}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrLandWaterBatchRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrLand().WaterBatch(ctx, req)
		},
	),
	clientproto.RPCUsrLandWater.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.UsrLandWaterRequest, error) {
			landID, err := plannedOpSingleLandID(op)
			if err != nil {
				return clientproto.UsrLandWaterRequest{}, err
			}
			return clientproto.UsrLandWaterRequest{LandId: landID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrLandWaterRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrLand().Water(ctx, req)
		},
	),
	clientproto.RPCUsrLandUnlockLand.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.UsrLandUnlockLandRequest, error) {
			return clientproto.UsrLandUnlockLandRequest{LandId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrLandUnlockLandRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrLand().UnlockLand(ctx, req)
		},
	),
	clientproto.RPCUsrLandSpeedUpBatch.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.UsrLandSpeedUpBatchRequest, error) {
			return clientproto.UsrLandSpeedUpBatchRequest{LandIds: op.LandIDs}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrLandSpeedUpBatchRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrLand().SpeedUpBatch(ctx, req)
		},
	),
	clientproto.RPCCultivateRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.CultivateRecvRequest, error) {
			return clientproto.CultivateRecvRequest{FlowerId: op.FlowerID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.CultivateRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Cultivate().Recv(ctx, req)
		},
	),
	clientproto.RPCCultivateUpgrade.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.CultivateUpgradeRequest, error) {
			return clientproto.CultivateUpgradeRequest{FlowerId: op.FlowerID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.CultivateUpgradeRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Cultivate().Upgrade(ctx, req)
		},
	),
	clientproto.RPCCultivateCultivate.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.CultivateCultivateRequest, error) {
			return clientproto.CultivateCultivateRequest{FlowerId: op.FlowerID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.CultivateCultivateRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Cultivate().Cultivate(ctx, req)
		},
	),
	clientproto.RPCOrderFlowerFinishOrder.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.OrderFlowerFinishOrderRequest, error) {
			return clientproto.OrderFlowerFinishOrderRequest{BoxId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderFlowerFinishOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderFlower().FinishOrder(ctx, req)
		},
	),
	clientproto.RPCOrderFlowerRecvOrderRwd.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.OrderFlowerRecvOrderRwdRequest, error) {
			return clientproto.OrderFlowerRecvOrderRwdRequest{Target: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderFlowerRecvOrderRwdRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderFlower().RecvOrderRwd(ctx, req)
		},
	),
	clientproto.RPCOrderCustomerFinishOrder.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.OrderCustomerFinishOrderRequest, error) {
			return clientproto.OrderCustomerFinishOrderRequest{NPCId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderCustomerFinishOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderCustomer().FinishOrder(ctx, req)
		},
	),
	clientproto.RPCOrderCustomerGenOrder.String(): stateDeltaOperation(
		staticRequest(clientproto.OrderCustomerGenOrderRequest{GuestNpcIdList: clientproto.RPCIDList{}}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderCustomerGenOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderCustomer().GenOrder(ctx, req)
		},
	),
	clientproto.RPCOrderCustomerRejectOrder.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.OrderCustomerRejectOrderRequest, error) {
			return clientproto.OrderCustomerRejectOrderRequest{NPCId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderCustomerRejectOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderCustomer().RejectOrder(ctx, req)
		},
	),
	clientproto.RPCOrderPalaceEnter.String(): stateDeltaOperation(
		staticRequest(clientproto.OrderPalaceEnterRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderPalaceEnterRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderPalace().Enter(ctx, req)
		},
	),
	clientproto.RPCOrderPalaceFinishOrder.String(): stateDeltaOperation(
		staticRequest(clientproto.OrderPalaceFinishOrderRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderPalaceFinishOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderPalace().FinishOrder(ctx, req)
		},
	),
	clientproto.RPCOrderTeamRefreshOrder.String(): stateDeltaOperation(
		staticRequest(clientproto.OrderTeamRefreshOrderRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderTeamRefreshOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderTeam().RefreshOrder(ctx, req)
		},
	),
	clientproto.RPCOrderTeamSubmitOrder.String(): stateDeltaOperation(
		staticRequest(clientproto.OrderTeamSubmitOrderRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderTeamSubmitOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderTeam().SubmitOrder(ctx, req)
		},
	),
	clientproto.RPCFlowerArtMakeFlowerArt.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FlowerArtMakeFlowerArtRequest, error) {
			if op.VaseID <= 0 || len(op.FlowerIDs) == 0 || op.Count <= 0 {
				return clientproto.FlowerArtMakeFlowerArtRequest{}, fmt.Errorf("flower art craft missing vase/flowers/count")
			}
			return clientproto.FlowerArtMakeFlowerArtRequest{VaseId: op.VaseID, FlowersIds: op.FlowerIDs, Num: op.Count}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FlowerArtMakeFlowerArtRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FlowerArt().MakeFlowerArt(ctx, req)
		},
	),
	clientproto.RPCCollectRwdRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.CollectRwdRecvRequest, error) {
			return clientproto.CollectRwdRecvRequest{Type: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.CollectRwdRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.CollectRwd().Recv(ctx, req)
		},
	),
	clientproto.RPCCollectRwdRecvArtCreateRwdByVase.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.CollectRwdRecvArtCreateRwdByVaseRequest, error) {
			return clientproto.CollectRwdRecvArtCreateRwdByVaseRequest{"flowerArtId": op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.CollectRwdRecvArtCreateRwdByVaseRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.CollectRwd().RecvArtCreateRwdByVase(ctx, req)
		},
	),
	clientproto.RPCFlowerRackSell.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FlowerRackSellRequest, error) {
			return clientproto.FlowerRackSellRequest{RackId: op.TargetID, Iid: op.ItemID, Num: op.Count}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FlowerRackSellRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FlowerRack().Sell(ctx, req)
		},
	),
	clientproto.RPCFlowerRackRecvSellMoney.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FlowerRackRecvSellMoneyRequest, error) {
			return clientproto.FlowerRackRecvSellMoneyRequest{RackId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FlowerRackRecvSellMoneyRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FlowerRack().RecvSellMoney(ctx, req)
		},
	),
	clientproto.RPCWaterwheelRecv.String(): {
		args: staticAnyRequest(clientproto.WaterwheelRecvRequest{}),
		run:  runWaterwheelRecv,
	},
	clientproto.RPCFreeWaterRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FreeWaterRecvRequest, error) {
			return clientproto.FreeWaterRecvRequest{Idx: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FreeWaterRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FreeWater().Recv(ctx, req)
		},
	),
	clientproto.RPCBenefitBoxDraw.String(): stateDeltaOperation(
		staticRequest(clientproto.BenefitBoxDrawRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.BenefitBoxDrawRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.BenefitBox().Draw(ctx, req)
		},
	),
	clientproto.RPCZooEnterZoo.String(): stateDeltaOperation(
		staticRequest(clientproto.ZooEnterZooRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ZooEnterZooRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Zoo().EnterZoo(ctx, req)
		},
	),
	clientproto.RPCZooFeedPets.String(): rawStateDeltaOperation(
		func(op *automation.PlannedOp) (map[string]any, error) {
			return map[string]any{"petIdList": []int32{op.TargetID}}, nil
		},
		clientproto.RPCZooFeedPets,
	),
	clientproto.RPCZooStrokePet.String(): rawStateDeltaOperation(
		func(op *automation.PlannedOp) (map[string]any, error) {
			return map[string]any{"petId": op.TargetID}, nil
		},
		clientproto.RPCZooStrokePet,
	),
	clientproto.RPCUsrExtraUpdateAntiFraudQAStatus.String(): stateDeltaOperation(
		staticRequest(clientproto.UsrExtraUpdateAntiFraudQAStatusRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrExtraUpdateAntiFraudQAStatusRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrExtra().UpdateAntiFraudQAStatus(ctx, req)
		},
	),
	clientproto.RPCUsrExtraRecvAntiFraudQARwd.String(): stateDeltaOperation(
		staticRequest(clientproto.UsrExtraRecvAntiFraudQARwdRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrExtraRecvAntiFraudQARwdRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrExtra().RecvAntiFraudQARwd(ctx, req)
		},
	),
	clientproto.RPCShopCultivateEnter.String(): stateDeltaOperation(
		staticRequest(clientproto.ShopCultivateEnterRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ShopCultivateEnterRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.ShopCultivate().Enter(ctx, req)
		},
	),
	clientproto.RPCShopCultivateBuy.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.ShopCultivateBuyRequest, error) {
			return clientproto.ShopCultivateBuyRequest{ShopId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ShopCultivateBuyRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.ShopCultivate().Buy(ctx, req)
		},
	),
	clientproto.RPCShopGiftbagEnter.String(): stateDeltaOperation(
		staticRequest(clientproto.ShopGiftbagEnterRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ShopGiftbagEnterRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.ShopGiftbag().Enter(ctx, req)
		},
	),
	clientproto.RPCShopGiftbagBuy.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.ShopGiftbagBuyRequest, error) {
			return clientproto.ShopGiftbagBuyRequest{ShopId: op.TargetID, Num: op.Count}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ShopGiftbagBuyRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.ShopGiftbag().Buy(ctx, req)
		},
	),
	clientproto.RPCPearlRefresh.String(): stateDeltaOperation(
		staticRequest(clientproto.PearlRefreshRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.PearlRefreshRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Pearl().Refresh(ctx, req)
		},
	),
	clientproto.RPCPearlRecvDailyFree.String(): stateDeltaOperation(
		staticRequest(clientproto.PearlRecvDailyFreeRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.PearlRecvDailyFreeRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Pearl().RecvDailyFree(ctx, req)
		},
	),
	clientproto.RPCPearlPlaceRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.PearlPlaceRecvRequest, error) {
			return clientproto.PearlPlaceRecvRequest{PlaceId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.PearlPlaceRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.PearlPlace().Recv(ctx, req)
		},
	),
	clientproto.RPCPearlSetProtectState.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.PearlSetProtectStateRequest, error) {
			return clientproto.PearlSetProtectStateRequest{ProtectState: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.PearlSetProtectStateRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Pearl().SetProtectState(ctx, req)
		},
	),
	clientproto.RPCPearlDraw.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.PearlDrawRequest, error) {
			return clientproto.PearlDrawRequest{Count: op.Count}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.PearlDrawRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Pearl().Draw(ctx, req)
		},
	),
	clientproto.RPCFmlBuild.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlBuildRequest, error) {
			return clientproto.FmlBuildRequest{ID: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlBuildRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Fml().Build(ctx, req)
		},
	),
	clientproto.RPCFmlLandHarvest.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlLandHarvestRequest, error) {
			return clientproto.FmlLandHarvestRequest{LandIds: op.LandIDs}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlLandHarvestRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlLand().Harvest(ctx, req)
		},
	),
	clientproto.RPCFmlForestRefresh.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlForestRefreshRequest, error) {
			return clientproto.FmlForestRefreshRequest{IsAutoCollect: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlForestRefreshRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlForest().Refresh(ctx, req)
		},
	),
	clientproto.RPCFmlFlowerShareRefresh.String(): stateDeltaOperation(
		staticRequest(clientproto.FmlFlowerShareRefreshRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlFlowerShareRefreshRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlFlowerShare().Refresh(ctx, req)
		},
	),
	clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String(): stateDeltaOperation(
		staticRequest(clientproto.FmlFlowerShareGetFmlOtherShareListRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlFlowerShareGetFmlOtherShareListRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlFlowerShare().GetFmlOtherShareList(ctx, req)
		},
	),
	clientproto.RPCFmlFlowerShareRecvRwd.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlFlowerShareRecvRwdRequest, error) {
			return clientproto.FmlFlowerShareRecvRwdRequest{SlotIds: op.SlotIDs}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlFlowerShareRecvRwdRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlFlowerShare().RecvRwd(ctx, req)
		},
	),
	clientproto.RPCFmlFlowerShareTake.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlFlowerShareTakeRequest, error) {
			return clientproto.FmlFlowerShareTakeRequest{DstUid: op.TargetUID, SlotId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlFlowerShareTakeRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlFlowerShare().Take(ctx, req)
		},
	),
	clientproto.RPCTaskDlyRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.TaskDlyRecvRequest, error) {
			return clientproto.TaskDlyRecvRequest{ID: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.TaskDlyRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.TaskDly().Recv(ctx, req)
		},
	),
	clientproto.RPCTaskWeekRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.TaskWeekRecvRequest, error) {
			return clientproto.TaskWeekRecvRequest{ID: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.TaskWeekRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.TaskWeek().Recv(ctx, req)
		},
	),
	clientproto.RPCStoryMainEnter.String(): stateDeltaOperation(
		staticRequest(clientproto.StoryMainEnterRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.StoryMainEnterRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.StoryMain().Enter(ctx, req)
		},
	),
	clientproto.RPCStoryMainUnlock.String(): stateDeltaOperation(
		staticRequest(clientproto.StoryMainUnlockRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.StoryMainUnlockRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.StoryMain().Unlock(ctx, req)
		},
	),
	clientproto.RPCTaskAchRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.TaskAchRecvRequest, error) {
			return clientproto.TaskAchRecvRequest{ID: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.TaskAchRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.TaskAch().Recv(ctx, req)
		},
	),
	clientproto.RPCRoadGrowRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.RoadGrowRecvRequest, error) {
			return clientproto.RoadGrowRecvRequest{ID: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.RoadGrowRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.RoadGrow().Recv(ctx, req)
		},
	),
	clientproto.RPCRandomEventEnter.String(): stateDeltaOperation(
		staticRequest(clientproto.RandomEventEnterRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.RandomEventEnterRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.RandomEvent().Enter(ctx, req)
		},
	),
	clientproto.RPCRandomEventDoAffair.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.RandomEventDoAffairRequest, error) {
			return clientproto.RandomEventDoAffairRequest{EventId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.RandomEventDoAffairRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.RandomEvent().DoAffair(ctx, req)
		},
	),
	clientproto.RPCZooFindPet.String(): rawStateDeltaOperation(
		func(op *automation.PlannedOp) (map[string]any, error) {
			return map[string]any{"petId": op.TargetID, "isShareVideo": 0}, nil
		},
		clientproto.RPCZooFindPet,
	),
	clientproto.RPCZooHandleEvent.String(): rawStateDeltaOperation(
		func(op *automation.PlannedOp) (map[string]any, error) {
			return map[string]any{"petId": op.TargetID, "tableId": op.ItemID, "agree": op.Count != 0, "isShareVideo": 0}, nil
		},
		clientproto.RPCZooHandleEvent,
	),
	clientproto.RPCMailGetList.String(): stateDeltaOperation(
		staticRequest(clientproto.MailGetListRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.MailGetListRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Mail().GetList(ctx, req)
		},
	),
	clientproto.RPCMailPick.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.MailPickRequest, error) {
			return clientproto.MailPickRequest{MsId: op.TargetID, AllId: op.ItemID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.MailPickRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Mail().Pick(ctx, req)
		},
	),
	clientproto.RPCSignTypeSign.String(): {
		args: staticAnyRequest(clientproto.SignTypeSignRequest{Type: 1}),
		run:  runSignTypeSign,
	},
}

func operationSpecFor(kind string) (operationSpec, bool) {
	spec, ok := plannedOperationSpecs[kind]
	return spec, ok
}

func staticRequest[Req any](req Req) func(*automation.PlannedOp) (Req, error) {
	return func(*automation.PlannedOp) (Req, error) {
		return req, nil
	}
}

func staticAnyRequest(req any) func(*automation.PlannedOp) (any, error) {
	return func(*automation.PlannedOp) (any, error) {
		return req, nil
	}
}

func harvestRequests(op *automation.PlannedOp) ([]clientproto.UsrLandHarvestRequest, error) {
	if op == nil || len(op.LandIDs) == 0 {
		return nil, fmt.Errorf("operation %s requires at least one land id", clientproto.RPCUsrLandHarvest.String())
	}
	reqs := make([]clientproto.UsrLandHarvestRequest, 0, len(op.LandIDs))
	for _, landID := range op.LandIDs {
		if landID == 0 {
			return nil, fmt.Errorf("operation %s has empty land id", clientproto.RPCUsrLandHarvest.String())
		}
		reqs = append(reqs, clientproto.UsrLandHarvestRequest{LandId: landID})
	}
	return reqs, nil
}

func harvestOperationArgs(op *automation.PlannedOp) (any, error) {
	reqs, err := harvestRequests(op)
	if err != nil {
		return nil, err
	}
	if len(reqs) == 1 {
		return reqs[0], nil
	}
	return reqs, nil
}

type harvestCallResult struct {
	LandID int32           `json:"landId"`
	Raw    json.RawMessage `json:"raw,omitempty"`
}

type harvestLandError struct {
	LandID int32
	Err    error
}

func (e *harvestLandError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return fmt.Sprintf("land %d: %v", e.LandID, e.Err)
}

func (e *harvestLandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func runUsrLandHarvest(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	reqs, err := harvestRequests(op)
	if err != nil {
		return nil, err
	}
	results := make([]harvestCallResult, 0, len(reqs))
	for i, req := range reqs {
		raw, err := checkedStateDelta(rt.rpc.UsrLand().Harvest(ctx, req))
		if err != nil {
			return nil, &harvestLandError{LandID: req.LandId, Err: err}
		}
		results = append(results, harvestCallResult{LandID: req.LandId, Raw: raw})
		if i == len(reqs)-1 || harvestRPCInterval <= 0 {
			continue
		}
		timer := time.NewTimer(harvestRPCInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	raw, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func stateDeltaOperation[Req any](
	build func(*automation.PlannedOp) (Req, error),
	call func(context.Context, *clientrpc.Client, Req) (babigame.RPCResponse[clientproto.StateDelta], error),
) operationSpec {
	return operationSpec{
		args: func(op *automation.PlannedOp) (any, error) {
			return build(op)
		},
		run: func(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
			req, err := build(op)
			if err != nil {
				return nil, err
			}
			return checkedStateDelta(call(ctx, rt.rpc, req))
		},
	}
}

func rawStateDeltaOperation(
	build func(*automation.PlannedOp) (map[string]any, error),
	name clientproto.RPCName,
) operationSpec {
	return operationSpec{
		args: func(op *automation.PlannedOp) (any, error) {
			return build(op)
		},
		run: func(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
			req, err := build(op)
			if err != nil {
				return nil, err
			}
			return checkedStateDelta(babigame.CallRPC[clientproto.StateDelta](ctx, rt.rawRPC, name, req))
		},
	}
}

func checkedStateDelta(resp babigame.RPCResponse[clientproto.StateDelta], err error) (json.RawMessage, error) {
	v, d, err := rpcResult(resp, err)
	return checkedPayload(v, d, err)
}

func runSignTypeSign(ctx context.Context, rt operationRuntime, _ *automation.PlannedOp) (json.RawMessage, error) {
	if v, d, err := rpcResult(rt.rpc.SignType().Enter(ctx, clientproto.SignTypeEnterRequest{Type: 1})); err != nil || d.IsError() {
		return checkedPayload(v, d, err)
	} else if babigame.HasPayload(v) {
		rt.runner.state.ApplyV(v)
	}
	if v, d, err := rpcResult(rt.rpc.SignType().Sign(ctx, clientproto.SignTypeSignRequest{Type: 1})); err != nil || d.IsError() {
		return checkedPayload(v, d, err)
	} else if babigame.HasPayload(v) {
		rt.runner.state.ApplyV(v)
	}
	return checkedStateDelta(rt.rpc.SignType().Recv(ctx, clientproto.SignTypeRecvRequest{Type: 1}))
}

func runWaterwheelRecv(ctx context.Context, rt operationRuntime, _ *automation.PlannedOp) (json.RawMessage, error) {
	if rt.runner != nil && rt.runner.state.WaterwheelNextClaimRequiresSkip() {
		if v, d, err := rpcResult(rt.rpc.Waterwheel().Skip(ctx, clientproto.WaterwheelSkipRequest{})); err != nil || d.IsError() {
			return checkedPayload(v, d, err)
		} else if babigame.HasPayload(v) {
			rt.runner.state.ApplyV(v)
		}
	}
	return checkedStateDelta(rt.rpc.Waterwheel().Recv(ctx, clientproto.WaterwheelRecvRequest{}))
}

func (r *Runner) executePlannedOp(ctx context.Context, client *babigame.Client, session *babigame.Session, op *automation.PlannedOp) (json.RawMessage, error) {
	if op == nil {
		return nil, fmt.Errorf("nil planned operation")
	}
	spec, ok := operationSpecFor(op.Kind)
	if !ok {
		return nil, fmt.Errorf("unsupported planned operation %s", op.Kind)
	}
	rawRPC := babigame.NewRPCClient(
		client,
		session,
		babigame.WithDefaultTimeout(30*time.Second),
		babigame.WithApplyV(r.state.ApplyV),
	)
	rt := operationRuntime{runner: r, rpc: clientrpc.NewClient(rawRPC), rawRPC: rawRPC}
	return spec.run(ctx, rt, op)
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
	spec, ok := operationSpecFor(op.Kind)
	if !ok {
		return nil, fmt.Errorf("unsupported planned operation %s", op.Kind)
	}
	return spec.args(op)
}

func isWaterOp(kind string) bool {
	return kind == clientproto.RPCUsrLandWater.String() ||
		kind == clientproto.RPCUsrLandWaterBatch.String()
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
	return kind == clientproto.RPCUsrLandHarvest.String()
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
	return kind == clientproto.RPCOrderFlowerFinishOrder.String() && err != nil && strings.Contains(err.Error(), "冷却中")
}

func isResidentOrderDailyLimitError(kind string, err error) bool {
	return kind == clientproto.RPCOrderFlowerFinishOrder.String() && err != nil && strings.Contains(err.Error(), "今日完成订单次数已达上限")
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
	r.lastWaterSyncTick = now

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
	r.state.MarkWaterwheelEntered(now)
	if babigame.HasPayload(v) {
		r.state.ApplyV(v)
	}
}

// emitCustomerOrderInfo emits a dedicated order-category event whenever the
// observed customer order requirements change. The event carries the formatted
// flower art info (vase + recipe flowers) so the logs-order module can display
// what each active order needs.
func (r *Runner) emitCustomerOrderInfo() {
	orders := r.state.CustomerOrderDetails()
	seen := make(map[int32]bool, len(orders))
	for npcID, order := range orders {
		seen[npcID] = true
		summary := automation.FormatCustomerOrderRequires(r.state, order)
		if summary == r.lastCustomerOrderInfo[npcID] {
			continue
		}
		r.lastCustomerOrderInfo[npcID] = summary
		if summary == "" {
			continue
		}
		r.emit(Event{
			Kind:     "order_customer_info",
			Category: "order",
			Domain:   "order.customer",
			Action:   "info",
			Message:  fmt.Sprintf("顾客订单 NPC=%d %s", npcID, summary),
			Level:    "info",
		})
	}
	for npcID := range r.lastCustomerOrderInfo {
		if !seen[npcID] {
			delete(r.lastCustomerOrderInfo, npcID)
		}
	}
}

func operationPayload(op *automation.PlannedOp, args any, raw json.RawMessage, err error) string {
	payload := map[string]any{
		"rpc":       op.Kind,
		"lane":      op.Lane,
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
	if !op.CooldownUntil.IsZero() {
		payload["cooldownUntilMs"] = op.CooldownUntil.UnixMilli()
		payload["cooldownReason"] = op.CooldownReason
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

func operationTargetSuffix(op *automation.PlannedOp) string {
	if op == nil {
		return ""
	}
	if suffix := landSuffix(op.LandIDs); suffix != "" {
		return suffix
	}
	switch op.Kind {
	case clientproto.RPCStoryMainUnlock.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (剧情小节=%d)", op.TargetID)
		}
	case clientproto.RPCTaskAchRecv.String(), clientproto.RPCTaskDlyRecv.String(), clientproto.RPCTaskWeekRecv.String(), clientproto.RPCRoadGrowRecv.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (任务=%d)", op.TargetID)
		}
	case clientproto.RPCRandomEventDoAffair.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (事件=%d)", op.TargetID)
		}
	case clientproto.RPCZooFeedPets.String(), clientproto.RPCZooStrokePet.String(), clientproto.RPCZooFindPet.String():
		if op.TargetID > 0 {
			return fmt.Sprintf(" (宠物=%d)", op.TargetID)
		}
	case clientproto.RPCZooHandleEvent.String():
		if op.TargetID > 0 && op.ItemID > 0 {
			return fmt.Sprintf(" (宠物=%d 事件=%d)", op.TargetID, op.ItemID)
		}
		if op.TargetID > 0 {
			return fmt.Sprintf(" (宠物=%d)", op.TargetID)
		}
	case clientproto.RPCFlowerArtMakeFlowerArt.String():
		if desc := automation.FormatFlowerArtOpDesc(op.ItemID, op.Count); desc != "" {
			return " " + desc
		}
	}
	return ""
}

func landSuffix(landIDs []int32) string {
	if len(landIDs) == 0 {
		return ""
	}
	return fmt.Sprintf(" (田地=%v)", landIDs)
}

// orderCustomerSuffix generates a suffix for customer order operations using
// the live state to show NPC and flower-art requirements.
func (r *Runner) orderCustomerSuffix(op *automation.PlannedOp) string {
	switch op.Kind {
	case clientproto.RPCOrderCustomerFinishOrder.String(), clientproto.RPCOrderCustomerRejectOrder.String():
	default:
		return ""
	}
	if op.TargetID == 0 {
		return ""
	}
	orders := r.state.CustomerOrderDetails()
	order, ok := orders[op.TargetID]
	if !ok || order == nil {
		return fmt.Sprintf(" (NPC=%d)", op.TargetID)
	}
	summary := automation.FormatCustomerOrderRequires(r.state, order)
	if summary == "" {
		return fmt.Sprintf(" (NPC=%d)", op.TargetID)
	}
	return fmt.Sprintf(" (NPC=%d %s)", op.TargetID, summary)
}

// opSuffix combines the static operation target suffix with state-backed
// customer order details.
func (r *Runner) opSuffix(op *automation.PlannedOp) string {
	if suffix := operationTargetSuffix(op); suffix != "" {
		return suffix
	}
	return r.orderCustomerSuffix(op)
}
