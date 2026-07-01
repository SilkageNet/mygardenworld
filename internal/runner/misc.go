package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientrpc"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	flowerUpgradeRetryWait  = 30 * time.Minute
	cultivateRetryWait      = 30 * time.Minute
	residentOrderDrainLimit = 24
	residentOrderDrainDelay = 500 * time.Millisecond
)

func (r *Runner) tickCultivate(ctx context.Context, client *babigame.Client, session *babigame.Session) {
	r.mu.RLock()
	policy := r.policy
	r.mu.RUnlock()
	misc := (*pb.MiscPolicy)(nil)
	if policy != nil {
		misc = policy.GetMisc()
	}
	cultivateEnabled := misc != nil && misc.GetCultivateEnabled()
	flowerUpgradeEnabled := misc != nil && misc.GetFlowerUpgradeEnabled()
	if !cultivateEnabled && !flowerUpgradeEnabled {
		return
	}
	rpc := r.runnerRPC(client, session)

	cultivations := r.state.Cultivations()
	cultivationIDs := make([]int32, 0, len(cultivations))
	for fid := range cultivations {
		cultivationIDs = append(cultivationIDs, fid)
	}
	sort.Slice(cultivationIDs, func(i, j int) bool { return cultivationIDs[i] < cultivationIDs[j] })
	now := time.Now().UnixMilli()
	gold := r.state.Gold()

	if cultivateEnabled {
		for _, fid := range cultivationIDs {
			cv := cultivations[fid]
			// 自动领取：培育中且完成时间已到
			if cv.Status == 1 && cv.CulTimeMs > 0 && cv.CulTimeMs <= now {
				v, d, err := rpcResult(rpc.Cultivate().Recv(ctx, clientproto.CultivateRecvRequest{FlowerId: fid}))
				if r.isSessionInvalidated() {
					return
				}
				if err != nil {
					r.emit(Event{Kind: "cultivate_recv", Message: fmt.Sprintf("领取培育 %s 失败: %v", state.FlowerName(fid), err)})
				} else if d.IsError() {
					r.emit(Event{Kind: "cultivate_recv", Message: fmt.Sprintf("领取培育 %s: %s", state.FlowerName(fid), d.ErrorMsg())})
				} else if babigame.HasPayload(v) {
					r.state.ApplyV(v)
					r.emit(Event{Kind: "cultivate_recv", Message: fmt.Sprintf("成功领取培育 %s", state.FlowerName(fid))})
				}
				return
			}
		}
	}

	if flowerUpgradeEnabled {
		for _, fid := range cultivationIDs {
			if r.isFlowerUpgradeBlocked(fid) {
				continue
			}
			cv := cultivations[fid]
			// 自动升级：已领取(status=2)，且客户端配置显示材料与金币足够。
			if cv.Status == 2 && cv.Lvl > 0 {
				cost, ok := state.FlowerUpgradeCostForLevel(fid, cv.Lvl)
				if !ok {
					continue
				}
				inventory := r.state.Inventory()
				if inventory[cost.ItemID] < cost.Count || gold < cost.Gold {
					continue
				}
				prevLvl := cv.Lvl
				v, d, err := rpcResult(rpc.Cultivate().Upgrade(ctx, clientproto.CultivateUpgradeRequest{FlowerId: fid}))
				if r.isSessionInvalidated() {
					return
				}
				if err != nil {
					r.emit(Event{Kind: "flower_upgrade", Message: fmt.Sprintf("升级 %s 失败: %v", state.FlowerName(fid), err)})
				} else if d.IsError() {
					if missingItemID := d.MissingItemID(); missingItemID > 0 {
						have := r.state.Inventory()[missingItemID]
						r.setFlowerUpgradeBlockedForItem(fid, missingItemID, have)
					} else {
						r.setFlowerUpgradeBlocked(fid, true)
						r.emit(Event{Kind: "flower_upgrade", Message: fmt.Sprintf("升级 %s: %s", state.FlowerName(fid), d.ErrorMsg())})
					}
				} else if babigame.HasPayload(v) {
					r.state.ApplyV(v)
					newCv := r.state.Cultivations()[fid]
					if newCv.Lvl > prevLvl {
						r.setFlowerUpgradeBlocked(fid, false)
						r.emit(Event{Kind: "flower_upgrade", Message: fmt.Sprintf("成功升级 %s (等级 %d→%d)", state.FlowerName(fid), prevLvl, newCv.Lvl)})
					}
					gold = r.state.Gold()
				}
				return
			}
		}
	}

	if cultivateEnabled {
		for _, fid := range cultivationIDs {
			cv := cultivations[fid]
			if cv.Status != 0 || cv.Lvl <= 0 {
				continue
			}
			if !r.canStartCultivate(fid) {
				continue
			}
			r.startCultivate(ctx, client, session, fid)
			return
		}

		// 自动培育新花：库存中有但未培育的花
		inventory := r.state.FlowerInventory()
		inventoryIDs := make([]int32, 0, len(inventory))
		for fid := range inventory {
			inventoryIDs = append(inventoryIDs, fid)
		}
		sort.Slice(inventoryIDs, func(i, j int) bool { return inventoryIDs[i] < inventoryIDs[j] })
		for _, fid := range inventoryIDs {
			if _, exists := cultivations[fid]; exists {
				continue
			}
			if !r.canStartCultivate(fid) {
				continue
			}
			r.startCultivate(ctx, client, session, fid)
			return
		}
	}
}

func (r *Runner) canStartCultivate(fid int32) bool {
	if r.isCultivateBlocked(fid) {
		return false
	}
	costs, ok := state.CultivateCost(fid)
	if !ok {
		r.setCultivateBlocked(fid, true)
		return false
	}
	inventory := r.state.Inventory()
	for _, cost := range costs {
		have := inventory[cost.ItemID]
		if have >= cost.Count {
			continue
		}
		r.setCultivateBlocked(fid, true)
		return false
	}
	return true
}

func (r *Runner) startCultivate(ctx context.Context, client *babigame.Client, session *babigame.Session, fid int32) {
	rpc := r.runnerRPC(client, session)
	v, d, err := rpcResult(rpc.Cultivate().Cultivate(ctx, clientproto.CultivateCultivateRequest{FlowerId: fid}))
	if r.isSessionInvalidated() {
		return
	}
	if err != nil {
		r.emit(Event{Kind: "cultivate_new", Message: fmt.Sprintf("开始培育 %s 失败: %v", state.FlowerName(fid), err)})
	} else if d.IsError() {
		r.emit(Event{Kind: "cultivate_new", Message: fmt.Sprintf("开始培育 %s: %s", state.FlowerName(fid), d.ErrorMsg())})
	} else if babigame.HasPayload(v) {
		r.state.ApplyV(v)
		r.emit(Event{Kind: "cultivate_new", Message: fmt.Sprintf("成功开始培育 %s", state.FlowerName(fid))})
	}
}

func (r *Runner) tickMisc(ctx context.Context, client *babigame.Client, session *babigame.Session) {
	if time.Since(r.lastMiscTick) < 60*time.Second {
		return
	}
	r.lastMiscTick = time.Now()

	r.mu.RLock()
	policy := r.policy
	landUnlockBlocked := r.landUnlockBlocked
	taskRecvBlocked := r.taskRecvBlocked
	storyUnlockBlocked := r.storyUnlockBlocked
	freeWaterBlockedUntil := r.freeWaterBlockedUntil
	dailyTaskBlockedUntil := r.dailyTaskBlockedUntil
	residentOrderBlockedUntil := r.residentOrderBlockedUntil
	r.mu.RUnlock()
	misc := (*pb.MiscPolicy)(nil)
	if policy != nil {
		misc = policy.GetMisc()
	}
	if misc == nil {
		return
	}
	rpc := r.runnerRPC(client, session)

	// 水资源奖励
	if misc.GetWaterwheelEnabled() && r.state.WaterwheelCooldownReady() {
		prevWW := r.state.WaterwheelClaimedCount()
		v, d, err := rpcResult(rpc.Waterwheel().Recv(ctx, clientproto.WaterwheelRecvRequest{}))
		if r.isSessionInvalidated() {
			return
		}
		if err != nil {
			r.emit(Event{Kind: "waterwheel", Message: fmt.Sprintf("领取水车失败: %v", err)})
		} else if d.IsError() {
			// 冷却未到或无可领取，静默
		} else if babigame.HasPayload(v) {
			r.state.ApplyV(v)
			newWW := r.state.WaterwheelClaimedCount()
			if newWW > prevWW {
				r.emit(Event{Kind: "waterwheel", Message: fmt.Sprintf("成功领取水车奖励 (第%d次)", newWW)})
				v2, _, _ := rpcResult(rpc.Waterwheel().Skip(ctx, clientproto.WaterwheelSkipRequest{}))
				if r.isSessionInvalidated() {
					return
				}
				if babigame.HasPayload(v2) {
					r.state.ApplyV(v2)
				}
			}
		}
	}
	if misc.GetFreeWaterEnabled() && time.Now().After(freeWaterBlockedUntil) {
		if idx, ok := r.state.NextFreeWaterIndex(); ok {
			v, d, err := rpcResult(rpc.FreeWater().Recv(ctx, clientproto.FreeWaterRecvRequest{Idx: idx}))
			if r.isSessionInvalidated() {
				return
			}
			if err != nil {
				r.setFreeWaterBlockedUntil(time.Now().Add(freeWaterRetryWait))
				r.emit(Event{Kind: "free_water", Message: fmt.Sprintf("领取免费水滴 #%d 失败: %v", idx, err)})
			} else if d.IsError() {
				r.setFreeWaterBlockedUntil(time.Now().Add(freeWaterRetryWait))
			} else if babigame.HasPayload(v) {
				r.state.ApplyV(v)
				r.emit(Event{Kind: "free_water", Message: fmt.Sprintf("成功领取免费水滴 #%d", idx)})
			} else {
				r.setFreeWaterBlockedUntil(time.Now().Add(freeWaterRetryWait))
			}
		}
	}

	// 福利箱
	if misc.GetBenefitBoxEnabled() && r.state.BenefitBoxReady() {
		v, d, err := rpcResult(rpc.BenefitBox().Draw(ctx, clientproto.BenefitBoxDrawRequest{}))
		if r.isSessionInvalidated() {
			return
		}
		if err != nil {
			r.emit(Event{Kind: "benefit_box", Message: fmt.Sprintf("福利箱抽奖失败: %v", err)})
		} else if d.IsError() {
			// 无可领取，静默
		} else if babigame.HasPayload(v) {
			r.state.ApplyV(v)
			r.emit(Event{Kind: "benefit_box", Message: fmt.Sprintf("成功领取福利箱 (剩余%d次)", r.state.BenefitBoxDrawsRemaining())})
		}
	}

	// 批量加速
	if misc.GetSpeedUpEnabled() {
		r.tickSpeedUp(ctx, client, session)
		if r.isSessionInvalidated() {
			return
		}
	}

	// 居民订单
	if misc.GetResidentOrderEnabled() && time.Now().After(residentOrderBlockedUntil) {
		r.tickResidentOrders(ctx, rpc)
		if r.isSessionInvalidated() {
			return
		}
	}

	// 顾客订单
	if misc.GetCustomerOrderEnabled() {
		if err := r.ensureCustomerOrderRqst(ctx); err != nil {
			r.log.Debug("customer order rqst failed", "err", err)
		}
		inventory := r.state.Inventory()
		customerOrders := r.state.CustomerOrderDetails()
		npcIDs := make([]int32, 0, len(customerOrders))
		for npcID := range customerOrders {
			npcIDs = append(npcIDs, npcID)
		}
		sort.Slice(npcIDs, func(i, j int) bool { return npcIDs[i] < npcIDs[j] })
		for _, npcId := range npcIDs {
			order := customerOrders[npcId]
			if order == nil {
				continue
			}
			if !canFulfillCustomerOrder(order, inventory) {
				if misc.GetCustomerOrderCraftEnabled() {
					crafted, stop := r.tryCraftCustomerOrderArt(ctx, client, session, npcId, order, inventory, misc.GetFlowerArtRewardEnabled())
					if stop {
						return
					}
					if crafted {
						inventory = r.state.Inventory()
					}
				}
			}
			if !canFulfillCustomerOrder(order, inventory) {
				if misc.GetCustomerOrderRejectEnabled() {
					v, d, err := rpcResult(rpc.OrderCustomer().RejectOrder(ctx, clientproto.OrderCustomerRejectOrderRequest{NPCId: npcId}))
					if r.isSessionInvalidated() {
						return
					}
					if err != nil {
						r.emit(Event{Kind: "order_customer", Message: fmt.Sprintf("顾客订单 NPC=%d 暂时没货失败: %v", npcId, err)})
						continue
					}
					if d.IsError() {
						continue
					}
					if babigame.HasPayload(v) {
						r.state.ApplyV(v)
						r.emit(Event{Kind: "order_customer", Message: fmt.Sprintf("顾客订单 NPC=%d 已标记暂时没货", npcId)})
						inventory = r.state.Inventory()
					}
					return
				}
				continue
			}
			v, d, err := rpcResult(rpc.OrderCustomer().FinishOrder(ctx, clientproto.OrderCustomerFinishOrderRequest{NPCId: npcId}))
			if r.isSessionInvalidated() {
				return
			}
			if err != nil {
				r.emit(Event{Kind: "order_customer", Message: fmt.Sprintf("完成顾客订单 NPC=%d 失败: %v", npcId, err)})
				continue
			}
			if d.IsError() {
				break
			}
			if babigame.HasPayload(v) {
				r.state.ApplyV(v)
				r.emit(Event{Kind: "order_customer", Message: fmt.Sprintf("成功完成顾客订单 (NPC=%d)", npcId)})
				inventory = r.state.Inventory()
			}
		}
	}

	if misc.GetFlowerRackEnabled() {
		// 一键收取花架收入
		v, d, err := rpcResult(rpc.FlowerRack().RecvOneKey(ctx, clientproto.FlowerRackRecvOneKeyRequest{}))
		if r.isSessionInvalidated() {
			return
		}
		if err == nil && !d.IsError() && babigame.HasPayload(v) {
			r.state.ApplyV(v)
		}
		// 上架花艺
		if r.tryStockFlowerRack(ctx, client, session, misc.GetFlowerRackCraftEnabled(), misc.GetFlowerArtRewardEnabled()) {
			return
		}
	}

	if misc.GetResidentOrderRewardEnabled() {
		for _, target := range r.state.ReadyFlowerOrderRewardTargets() {
			v, d, err := rpcResult(rpc.OrderFlower().RecvOrderRwd(ctx, clientproto.OrderFlowerRecvOrderRwdRequest{Target: target}))
			if r.isSessionInvalidated() {
				return
			}
			if err != nil {
				r.emit(Event{Kind: "order_reward", Message: fmt.Sprintf("领取居民订单阶段奖励 #%d 失败: %v", target, err)})
				break
			}
			if d.IsError() {
				break
			}
			if babigame.HasPayload(v) {
				r.state.ApplyV(v)
				r.emit(Event{Kind: "order_reward", Message: fmt.Sprintf("成功领取居民订单阶段奖励 #%d", target)})
			}
			break
		}
	}

	if misc.GetResidentOrderAdEnabled() {
		for _, boxID := range r.state.ReadyFlowerOrderAdBoxIDs() {
			beforeOrder := r.state.FlowerOrders()[boxID]
			v, d, err := rpcResult(rpc.Usr().Share(ctx, clientproto.UsrShareRequest{
				ShareId: 5,
				Ext: map[string]any{
					"opType": 1,
					"id":     int(boxID),
				},
			}))
			if r.isSessionInvalidated() {
				return
			}
			if err != nil {
				r.emit(Event{Kind: "order_ad", Message: fmt.Sprintf("刷新居民订单广告位 #%d 失败: %v", boxID, err)})
				break
			}
			if d.IsError() {
				break
			}
			if babigame.HasPayload(v) {
				r.state.ApplyV(v)
				afterOrder := r.state.FlowerOrders()[boxID]
				if residentAdSlotMaterialized(beforeOrder, afterOrder) {
					r.emit(Event{Kind: "order_ad", Message: fmt.Sprintf("居民订单广告位 #%d 已刷新为订单", boxID)})
				}
			}
			break
		}
	}

	// 土地开垦
	if misc.LandUnlockEnabled && !landUnlockBlocked {
		if nextLandID, ok := nextLandUnlockCandidate(r.state); ok {
			v, d, err := rpcResult(rpc.UsrLand().UnlockLand(ctx, clientproto.UsrLandUnlockLandRequest{LandId: nextLandID}))
			if r.isSessionInvalidated() {
				return
			}
			if err != nil {
				r.emit(Event{Kind: "land_unlock", Message: fmt.Sprintf("开垦 #%d 失败: %v", nextLandID, err)})
			} else if d.IsError() {
				r.setLandUnlockBlocked(true)
				r.emit(Event{Kind: "land_unlock", Message: fmt.Sprintf("开垦 #%d: %s", nextLandID, d.ErrorMsg())})
			} else if babigame.HasPayload(v) {
				r.state.ApplyV(v)
				r.emit(Event{Kind: "land_unlock", Message: fmt.Sprintf("成功开垦 #%d", nextLandID)})
			} else {
				r.setLandUnlockBlocked(true)
			}
		}
	}

	// 主线任务奖励
	if misc.GetTaskMainRewardEnabled() && !taskRecvBlocked {
		v, d, err := rpcResult(rpc.TaskMain().Recv(ctx, clientproto.TaskMainRecvRequest{}))
		if r.isSessionInvalidated() {
			return
		}
		if err != nil {
			r.emit(Event{Kind: "task_recv", Message: fmt.Sprintf("领取任务奖励失败: %v", err)})
		} else if d.IsError() {
			r.setTaskRecvBlocked(true)
		} else if babigame.HasPayload(v) {
			r.state.ApplyV(v)
			r.emit(Event{Kind: "task_recv", Message: "成功领取主线任务奖励"})
		} else {
			r.setTaskRecvBlocked(true)
		}
	}
	if misc.GetTaskDailyRewardEnabled() && time.Now().After(dailyTaskBlockedUntil) {
		for _, taskID := range r.state.ReadyDailyTaskIDs() {
			v, d, err := rpcResult(rpc.TaskDly().Recv(ctx, clientproto.TaskDlyRecvRequest{ID: taskID}))
			if r.isSessionInvalidated() {
				return
			}
			if err != nil {
				r.setDailyTaskBlockedUntil(time.Now().Add(dailyTaskRetryWait))
				r.emit(Event{Kind: "task_daily", Message: fmt.Sprintf("领取日常任务 #%d 失败: %v", taskID, err)})
				break
			}
			if d.IsError() {
				r.setDailyTaskBlockedUntil(time.Now().Add(dailyTaskRetryWait))
				break
			}
			if babigame.HasPayload(v) {
				r.state.ApplyV(v)
				r.emit(Event{Kind: "task_daily", Message: fmt.Sprintf("成功领取日常任务 #%d", taskID)})
			} else {
				r.setDailyTaskBlockedUntil(time.Now().Add(dailyTaskRetryWait))
			}
			break
		}
	}
	if misc.GetRoadGrowRewardEnabled() {
		for _, taskID := range r.state.ReadyRoadGrowTaskIDs() {
			v, d, err := rpcResult(rpc.RoadGrow().Recv(ctx, clientproto.RoadGrowRecvRequest{ID: taskID}))
			if r.isSessionInvalidated() {
				return
			}
			if err != nil {
				r.emit(Event{Kind: "road_grow", Message: fmt.Sprintf("领取成长之路 #%d 失败: %v", taskID, err)})
				break
			}
			if d.IsError() {
				break
			}
			if babigame.HasPayload(v) {
				r.state.ApplyV(v)
				r.emit(Event{Kind: "road_grow", Message: fmt.Sprintf("成功领取成长之路 #%d", taskID)})
			}
			break
		}
	}
	if misc.GetRandomEventEnabled() {
		_, _, _ = rpcResult(rpc.RandomEvent().Enter(ctx, clientproto.RandomEventEnterRequest{}))
		if r.isSessionInvalidated() {
			return
		}
		for _, eventID := range r.state.ReadyRandomEventIDs() {
			v, d, err := rpcResult(rpc.RandomEvent().DoAffair(ctx, clientproto.RandomEventDoAffairRequest{EventId: eventID}))
			if r.isSessionInvalidated() {
				return
			}
			if err != nil {
				r.emit(Event{Kind: "random_event", Message: fmt.Sprintf("领取地图事件 #%d 失败: %v", eventID, err)})
				break
			}
			if d.IsError() {
				break
			}
			if babigame.HasPayload(v) {
				r.state.ApplyV(v)
				r.emit(Event{Kind: "random_event", Message: fmt.Sprintf("成功领取地图事件 #%d", eventID)})
			}
			break
		}
	}

	r.tickCaptureAlignedDomains(ctx, rpc, misc)
	if r.isSessionInvalidated() {
		return
	}

	// 成就任务奖励
	if misc.GetTaskAchRewardEnabled() {
		r.tickTaskAchRewards(ctx, client, session)
		if r.isSessionInvalidated() {
			return
		}
	}

	// 主线剧情解锁
	if misc.StoryUnlockEnabled && !storyUnlockBlocked {
		v, d, err := rpcResult(rpc.StoryMain().Unlock(ctx, clientproto.StoryMainUnlockRequest{}))
		if r.isSessionInvalidated() {
			return
		}
		if err != nil {
			r.emit(Event{Kind: "story_unlock", Message: fmt.Sprintf("解锁剧情失败: %v", err)})
		} else if d.IsError() {
			r.setStoryUnlockBlocked(true)
		} else if babigame.HasPayload(v) {
			r.state.ApplyV(v)
			r.emit(Event{Kind: "story_unlock", Message: "成功解锁主线剧情"})
		} else {
			r.setStoryUnlockBlocked(true)
		}
	}
}

func (r *Runner) tickCaptureAlignedDomains(ctx context.Context, rpc *clientrpc.Client, misc *pb.MiscPolicy) {
	if misc == nil {
		return
	}
	if misc.GetOrderPalaceEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCOrderPalaceEnter.String(), map[string]any{})
		if r.isSessionInvalidated() {
			return
		}
		r.observeStateDelta(ctx, rpc, clientproto.RPCOrderPalaceGetOrderRcdList.String(), map[string]any{})
	}
	if misc.GetCustomerOrderEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCOrderCustomerGenOrder.String(), map[string]any{"guestNpcIdList": []int32{}})
	}
	if misc.GetPlayerBackEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCPlayerBackPlayerBackPassEnter.String(), map[string]any{})
		if r.isSessionInvalidated() {
			return
		}
		r.observeStateDelta(ctx, rpc, clientproto.RPCPlayerBackSignEnter.String(), map[string]any{})
	}
	if misc.GetSignEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCSignTypeEnter.String(), map[string]any{"type": int32(1)})
	}
	if misc.GetZooSyncEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCZooEnterZoo.String(), map[string]any{})
		if r.isSessionInvalidated() {
			return
		}
		r.observeStateDelta(ctx, rpc, clientproto.RPCZooFindPetByUsrBack.String(), map[string]any{"petId": int32(1)})
		if r.isSessionInvalidated() {
			return
		}
		r.observeStateDelta(ctx, rpc, clientproto.RPCZooRefreshPetStatus.String(), map[string]any{"petIdList": []int32{1}})
		if r.isSessionInvalidated() {
			return
		}
		r.observeStateDelta(ctx, rpc, clientproto.RPCZooReadLog.String(), map[string]any{"petId": int32(1)})
	}
	if misc.GetActivityRewardEnabled() {
		// Activity enter/claim RPCs are batch-scoped. The analyzer records
		// redacted samples, but captured batch ids may be stale, so leave these
		// read-only until runtime activity state is modeled.
		r.log.Debug("activity reward sync waits for runtime activity state")
	}
	if misc.GetFlowerPassEnabled() || misc.GetFlowerElvesPassEnabled() || misc.GetZooFeedEnabled() {
		r.log.Debug("capture-aligned domain has no safe claimable state yet")
	}
}

func (r *Runner) observeStateDelta(ctx context.Context, rpc *clientrpc.Client, name string, args map[string]any) {
	r.recordRPCName(name)
	_, d, err := rpcResult(rpc.CallStateDelta(ctx, name, args))
	if r.isSessionInvalidated() {
		return
	}
	if err != nil {
		r.log.Debug("capture-aligned rpc failed", "rpc", name, "err", err)
		return
	}
	if d.IsError() {
		r.log.Debug("capture-aligned rpc rejected", "rpc", name, "msg", d.ErrorMsg())
	}
}

func (r *Runner) setLandUnlockBlocked(v bool) {
	r.mu.Lock()
	r.landUnlockBlocked = v
	r.mu.Unlock()
}

const maxReclaimableLands = 6

func nextLandUnlockCandidate(st *state.State) (int32, bool) {
	if !st.LandRosterObserved() || !st.FarmLandConfigObserved() {
		return 0, false
	}
	lands := st.Lands()
	gold := st.Gold()
	// 游戏允许在已开垦地块之后最多开垦 6 块。
	// 实际金币 = cost[1] - cost[0] + 11（配置表编码方式）。
	reclaimable := 0
	for _, info := range st.FarmLands() {
		if _, opened := lands[info.ID]; opened {
			continue
		}
		reclaimable++
		if reclaimable > maxReclaimableLands {
			break
		}
		if len(info.Cost) < 2 {
			continue
		}
		actualCost := info.Cost[1] - info.Cost[0] + 11
		if gold < actualCost {
			continue
		}
		return info.ID, true
	}
	return 0, false
}

func (r *Runner) setTaskRecvBlocked(v bool) {
	r.mu.Lock()
	r.taskRecvBlocked = v
	r.mu.Unlock()
}

func (r *Runner) setStoryUnlockBlocked(v bool) {
	r.mu.Lock()
	r.storyUnlockBlocked = v
	r.mu.Unlock()
}

func (r *Runner) setWaterBlocked(v bool) {
	r.mu.Lock()
	r.waterBlocked = v
	if !v {
		r.waterBlockedUntil = time.Time{}
	}
	r.mu.Unlock()
}

func (r *Runner) setWaterBlockedUntil(until time.Time) {
	r.mu.Lock()
	r.waterBlocked = true
	r.waterBlockedUntil = until
	r.mu.Unlock()
}

func (r *Runner) setFreeWaterBlockedUntil(until time.Time) {
	r.mu.Lock()
	r.freeWaterBlockedUntil = until
	r.mu.Unlock()
}

func (r *Runner) setDailyTaskBlockedUntil(until time.Time) {
	r.mu.Lock()
	r.dailyTaskBlockedUntil = until
	r.mu.Unlock()
}

func (r *Runner) setResidentOrderBlockedUntil(until time.Time) {
	r.mu.Lock()
	r.residentOrderBlockedUntil = until
	r.mu.Unlock()
}

func isResidentOrderDailyLimit(msg string) bool {
	return strings.Contains(msg, "今日完成订单次数已达上限")
}

func nextLocalDay(now time.Time) time.Time {
	y, m, d := now.Date()
	loc := now.Location()
	return time.Date(y, m, d+1, 0, 5, 0, 0, loc)
}

func flowerLabel(fid int32) string {
	if name := state.FlowerName(fid); name != "" {
		return name
	}
	return fmt.Sprintf("#%d", fid)
}

func (r *Runner) tickResidentOrders(ctx context.Context, rpc *clientrpc.Client) {
	if err := r.ensureFlowerOrderRqst(ctx); err != nil {
		r.log.Debug("flower order rqst failed", "err", err)
	}
	skipped := make(map[int32]bool)
	completed := 0
	for completed < residentOrderDrainLimit {
		boxID, ok := nextFulfillableFlowerOrderBox(r.state.FlowerOrders(), r.state.FlowerInventory(), skipped)
		if !ok {
			return
		}
		v, d, err := rpcResult(rpc.OrderFlower().FinishOrder(ctx, clientproto.OrderFlowerFinishOrderRequest{BoxId: boxID}))
		if r.isSessionInvalidated() {
			return
		}
		if err != nil {
			r.emit(Event{Kind: "order_finish", Message: fmt.Sprintf("完成居民订单 #%d 失败: %v", boxID, err)})
			return
		}
		if d.IsError() {
			msg := d.ErrorMsg()
			if isResidentOrderDailyLimit(msg) {
				until := nextLocalDay(time.Now())
				r.setResidentOrderBlockedUntil(until)
				r.emit(Event{
					Kind:    "order_finish",
					Message: fmt.Sprintf("居民订单今日次数已达上限，暂停到 %s 后重试", until.Format("2006-01-02 15:04")),
					Level:   "warn",
				})
				return
			}
			r.emit(Event{Kind: "order_finish", Message: fmt.Sprintf("完成居民订单 #%d: %s", boxID, msg)})
			skipped[boxID] = true
			continue
		}
		if !babigame.HasPayload(v) {
			skipped[boxID] = true
			continue
		}
		r.state.ApplyV(v)
		completed++
		skipped = make(map[int32]bool)
		r.emit(Event{Kind: "order_finish", Message: fmt.Sprintf("成功完成居民订单 #%d", boxID)})
		if completed >= residentOrderDrainLimit {
			r.emit(Event{
				Kind:    "order_finish",
				Message: fmt.Sprintf("居民订单本轮已提交 %d 个，达到保护上限，下轮继续", completed),
				Level:   "warn",
			})
			return
		}
		if !sleepContext(ctx, residentOrderDrainDelay) {
			return
		}
	}
}

func nextFulfillableFlowerOrderBox(orders map[int32]*state.FlowerOrder, inventory map[int32]int32, skipped map[int32]bool) (int32, bool) {
	boxIDs := make([]int32, 0, len(orders))
	for boxID, order := range orders {
		if skipped[boxID] || !canFulfillFlowerOrder(order, inventory) {
			continue
		}
		boxIDs = append(boxIDs, boxID)
	}
	if len(boxIDs) == 0 {
		return 0, false
	}
	sort.Slice(boxIDs, func(i, j int) bool { return boxIDs[i] < boxIDs[j] })
	return boxIDs[0], true
}

func canFulfillFlowerOrder(order *state.FlowerOrder, inventory map[int32]int32) bool {
	if order == nil || len(order.Requires) == 0 {
		return false
	}
	for _, req := range order.Requires {
		if req.FlowerID == 0 || req.Count <= 0 || inventory[req.FlowerID] < req.Count {
			return false
		}
	}
	return true
}

func sleepContext(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func canFulfillCustomerOrder(order *state.CustomerOrder, inventory map[int32]int32) bool {
	hasRequirements := false
	for _, req := range order.Requires {
		if req.FlowerID == 0 || req.Count <= 0 {
			continue
		}
		hasRequirements = true
		if inventory[req.FlowerID] < req.Count {
			return false
		}
	}
	for _, req := range order.ItemRequires {
		if req.ItemID == 0 || req.Count <= 0 {
			continue
		}
		hasRequirements = true
		if inventory[req.ItemID] < req.Count {
			return false
		}
	}
	return hasRequirements
}

func residentAdSlotMaterialized(before, after *state.FlowerOrder) bool {
	return before != nil && before.Mode == 8 && len(before.Requires) == 0 &&
		after != nil && after.Mode != 8 && len(after.Requires) > 0
}

func (r *Runner) tryStockFlowerRack(ctx context.Context, client *babigame.Client, session *babigame.Session, craftEnabled, rewardEnabled bool) bool {
	emptySlots := r.state.EmptyFlowerRackSlotIDs()
	if len(emptySlots) == 0 {
		return false
	}
	rackID := emptySlots[0]
	inventory := r.state.Inventory()
	reservedArt := requiredCustomerArtCounts(r.state.CustomerOrderDetails())
	if artID, num, ok := bestOwnedFlowerArtForRack(inventory, reservedArt); ok {
		return r.sellFlowerArtOnRack(ctx, client, session, rackID, artID, num)
	}

	if !craftEnabled {
		return false
	}
	recipe, ok := bestCraftableFlowerArtForRack(inventory, r.state.FlowerOrderDeficits(), r.state.Level())
	if !ok {
		return false
	}
	if !r.makeFlowerArtForRack(ctx, client, session, recipe, rewardEnabled) {
		return true
	}
	inventory = r.state.Inventory()
	num := inventory[recipe.ArtID] - reservedArt[recipe.ArtID]
	if num <= 0 {
		return true
	}
	return r.sellFlowerArtOnRack(ctx, client, session, rackID, recipe.ArtID, num)
}

func requiredCustomerArtCounts(orders map[int32]*state.CustomerOrder) map[int32]int32 {
	out := make(map[int32]int32)
	for _, order := range orders {
		if order == nil {
			continue
		}
		for _, req := range order.ItemRequires {
			if req.ItemID > 0 && req.Count > 0 {
				out[req.ItemID] += req.Count
			}
		}
	}
	return out
}

func bestOwnedFlowerArtForRack(inventory map[int32]int32, reserved map[int32]int32) (int32, int32, bool) {
	var best state.FlowerArtRecipe
	var bestCount int32
	for itemID, count := range inventory {
		available := count - reserved[itemID]
		if available <= 0 {
			continue
		}
		recipe, ok := state.FlowerArtRecipeByID(itemID)
		if !ok {
			continue
		}
		if best.ArtID == 0 || recipe.SaleValue > best.SaleValue || (recipe.SaleValue == best.SaleValue && recipe.ArtID > best.ArtID) {
			best = recipe
			bestCount = available
		}
	}
	if best.ArtID == 0 {
		return 0, 0, false
	}
	return best.ArtID, bestCount, true
}

func bestCraftableFlowerArtForRack(inventory map[int32]int32, flowerDeficits map[int32]int32, playerLevel int32) (state.FlowerArtRecipe, bool) {
	for _, recipe := range state.AllFlowerArtRecipes() {
		if recipe.Level > 0 && playerLevel > 0 && playerLevel < recipe.Level {
			continue
		}
		need := make(map[int32]int32)
		for _, flowerID := range recipe.Flowers {
			if flowerID != 0 {
				need[flowerID]++
			}
		}
		canCraft := true
		for flowerID, count := range need {
			if inventory[flowerID]-flowerDeficits[flowerID] < count {
				canCraft = false
				break
			}
		}
		if canCraft {
			return recipe, true
		}
	}
	return state.FlowerArtRecipe{}, false
}

func (r *Runner) makeFlowerArtForRack(ctx context.Context, client *babigame.Client, session *babigame.Session, recipe state.FlowerArtRecipe, rewardEnabled bool) bool {
	rpc := r.runnerRPC(client, session)
	v, d, err := rpcResult(rpc.FlowerArt().MakeFlowerArt(ctx, clientproto.FlowerArtMakeFlowerArtRequest{
		VaseId:     recipe.VaseID,
		FlowersIds: recipe.Flowers,
		Num:        1,
	}))
	if r.isSessionInvalidated() {
		return false
	}
	artName := state.ItemName(recipe.ArtID)
	if artName == "" {
		artName = fmt.Sprintf("#%d", recipe.ArtID)
	}
	if err != nil {
		r.emit(Event{Kind: "flower_rack", Message: fmt.Sprintf("制作待上架花艺 %s 失败: %v", artName, err)})
		return false
	}
	if d.IsError() {
		return false
	}
	if babigame.HasPayload(v) {
		r.state.ApplyV(v)
		r.emit(Event{Kind: "flower_rack", Message: fmt.Sprintf("成功制作待上架花艺 %s", artName)})
		if rewardEnabled {
			r.claimFlowerArtPostCraftRewards(ctx, client, session, recipe.ArtID)
		}
		return true
	}
	return false
}

func (r *Runner) sellFlowerArtOnRack(ctx context.Context, client *babigame.Client, session *babigame.Session, rackID, artID, num int32) bool {
	if rackID <= 0 || artID <= 0 || num <= 0 {
		return false
	}
	rpc := r.runnerRPC(client, session)
	v, d, err := rpcResult(rpc.FlowerRack().Sell(ctx, clientproto.FlowerRackSellRequest{
		RackId: rackID,
		Iid:    artID,
		Num:    num,
	}))
	if r.isSessionInvalidated() {
		return true
	}
	artName := state.ItemName(artID)
	if artName == "" {
		artName = fmt.Sprintf("#%d", artID)
	}
	if err != nil {
		r.emit(Event{Kind: "flower_rack", Message: fmt.Sprintf("上架花艺 %s 失败: %v", artName, err)})
		return true
	}
	if d.IsError() {
		r.emit(Event{Kind: "flower_rack", Message: fmt.Sprintf("上架花艺 %s: %s", artName, d.ErrorMsg())})
		return true
	}
	if babigame.HasPayload(v) {
		r.state.ApplyV(v)
		r.emit(Event{Kind: "flower_rack", Message: fmt.Sprintf("成功上架花艺 %s x%d (货架 #%d)", artName, num, rackID)})
		return true
	}
	return true
}

func (r *Runner) tryCraftCustomerOrderArt(ctx context.Context, client *babigame.Client, session *babigame.Session, npcID int32, order *state.CustomerOrder, inventory map[int32]int32, rewardEnabled bool) (bool, bool) {
	for _, req := range order.ItemRequires {
		missingArt := req.Count - inventory[req.ItemID]
		if req.ItemID == 0 || missingArt <= 0 {
			continue
		}
		recipe, ok := state.FlowerArtRecipeByID(req.ItemID)
		if !ok {
			continue
		}
		if recipe.Level > 0 && r.state.Level() < recipe.Level {
			return false, false
		}
		for _, flowerID := range recipe.Flowers {
			if inventory[flowerID] < missingArt {
				return false, false
			}
		}
		rpc := r.runnerRPC(client, session)
		v, d, err := rpcResult(rpc.FlowerArt().MakeFlowerArt(ctx, clientproto.FlowerArtMakeFlowerArtRequest{
			VaseId:     recipe.VaseID,
			FlowersIds: recipe.Flowers,
			Num:        missingArt,
		}))
		if r.isSessionInvalidated() {
			return false, true
		}
		artName := state.ItemName(req.ItemID)
		if artName == "" {
			artName = fmt.Sprintf("#%d", req.ItemID)
		}
		if err != nil {
			r.emit(Event{Kind: "flower_art", Message: fmt.Sprintf("制作花艺 %s 失败: %v", artName, err)})
			return false, true
		}
		if d.IsError() {
			return false, false
		}
		if babigame.HasPayload(v) {
			r.state.ApplyV(v)
			r.emit(Event{Kind: "flower_art", Message: fmt.Sprintf("成功制作花艺 %s x%d (NPC=%d)", artName, missingArt, npcID)})
			if rewardEnabled {
				r.claimFlowerArtPostCraftRewards(ctx, client, session, req.ItemID)
			}
			inventory = r.state.Inventory()
			continue
		}
	}
	return canFulfillCustomerOrder(order, inventory), false
}

func (r *Runner) claimFlowerArtPostCraftRewards(ctx context.Context, client *babigame.Client, session *babigame.Session, artID int32) {
	rpc := r.runnerRPC(client, session)
	for _, call := range []func() (json.RawMessage, babigame.WSResponseD, error){
		func() (json.RawMessage, babigame.WSResponseD, error) {
			return rpcResult(rpc.CollectRwd().RecvArtCreateRwd(ctx, clientproto.CollectRwdRecvArtCreateRwdRequest{FlowerArtId: artID}))
		},
		func() (json.RawMessage, babigame.WSResponseD, error) {
			return rpcResult(rpc.Usr().Share(ctx, clientproto.UsrShareRequest{
				ShareId: 6,
				Ext: map[string]any{
					"opType": 2,
					"id":     int(artID),
				},
			}))
		},
	} {
		v, d, err := call()
		if r.isSessionInvalidated() || err != nil || d.IsError() {
			return
		}
		if babigame.HasPayload(v) {
			r.state.ApplyV(v)
		}
	}
}

type flowerUpgradeBlock struct {
	Until  time.Time
	ItemID int32
	Have   int32
}

func (r *Runner) isFlowerUpgradeBlocked(fid int32) bool {
	r.mu.RLock()
	block, ok := r.flowerUpgradeBlocked[fid]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	if block.ItemID > 0 && r.state.Inventory()[block.ItemID] > block.Have {
		r.setFlowerUpgradeBlocked(fid, false)
		return false
	}
	if time.Now().Before(block.Until) {
		return true
	}
	r.setFlowerUpgradeBlocked(fid, false)
	return false
}

func (r *Runner) setFlowerUpgradeBlocked(fid int32, v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.flowerUpgradeBlocked == nil {
		r.flowerUpgradeBlocked = make(map[int32]flowerUpgradeBlock)
	}
	if v {
		r.flowerUpgradeBlocked[fid] = flowerUpgradeBlock{Until: time.Now().Add(flowerUpgradeRetryWait)}
		return
	}
	delete(r.flowerUpgradeBlocked, fid)
}

func (r *Runner) setFlowerUpgradeBlockedForItem(fid, itemID, have int32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.flowerUpgradeBlocked == nil {
		r.flowerUpgradeBlocked = make(map[int32]flowerUpgradeBlock)
	}
	r.flowerUpgradeBlocked[fid] = flowerUpgradeBlock{
		Until:  time.Now().Add(flowerUpgradeRetryWait),
		ItemID: itemID,
		Have:   have,
	}
}

func (r *Runner) isCultivateBlocked(fid int32) bool {
	r.mu.RLock()
	blockedUntil, ok := r.cultivateBlocked[fid]
	r.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().Before(blockedUntil) {
		return true
	}
	r.setCultivateBlocked(fid, false)
	return false
}

func (r *Runner) setCultivateBlocked(fid int32, v bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cultivateBlocked == nil {
		r.cultivateBlocked = make(map[int32]time.Time)
	}
	if v {
		r.cultivateBlocked[fid] = time.Now().Add(cultivateRetryWait)
		return
	}
	delete(r.cultivateBlocked, fid)
}

const speedUpItemID int32 = 1001
const speedUpMaxBatch = 5

var taskAchMilestoneIDs = []int32{10001, 10002, 10003, 10004, 10005, 20001, 20002, 20003, 20004, 20005}

func (r *Runner) tickTaskAchRewards(ctx context.Context, client *babigame.Client, session *babigame.Session) {
	r.mu.RLock()
	blocked := r.taskAchBlocked
	r.mu.RUnlock()
	if blocked {
		return
	}
	rpc := r.runnerRPC(client, session)
	for _, id := range taskAchMilestoneIDs {
		v, d, err := rpcResult(rpc.TaskAch().Recv(ctx, clientproto.TaskAchRecvRequest{ID: id}))
		if r.isSessionInvalidated() {
			return
		}
		if err != nil {
			r.setTaskAchBlocked(true)
			return
		}
		if d.IsError() {
			continue
		}
		if babigame.HasPayload(v) {
			r.state.ApplyV(v)
			r.emit(Event{Kind: "task_ach", Message: fmt.Sprintf("成功领取成就奖励 #%d", id)})
			return
		}
	}
	r.setTaskAchBlocked(true)
}

func (r *Runner) setTaskAchBlocked(v bool) {
	r.mu.Lock()
	r.taskAchBlocked = v
	r.mu.Unlock()
}

func (r *Runner) tickSpeedUp(ctx context.Context, client *babigame.Client, session *babigame.Session) {
	available := r.state.Inventory()[speedUpItemID]
	if available <= 0 {
		return
	}
	lands := r.state.Lands()
	now := time.Now().UnixMilli()
	var candidates []int32
	for id, land := range lands {
		if land.State == 2 && land.NextTimeMs > now {
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 0 {
		return
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i] < candidates[j] })
	want := int32(len(candidates))
	if want > available {
		want = available
	}
	if want > speedUpMaxBatch {
		want = speedUpMaxBatch
	}
	batch := candidates[:want]
	rpc := r.runnerRPC(client, session)
	v, d, err := rpcResult(rpc.UsrLand().SpeedUpBatch(ctx, clientproto.UsrLandSpeedUpBatchRequest{LandIds: batch}))
	if r.isSessionInvalidated() {
		return
	}
	if err != nil {
		r.emit(Event{Kind: "speed_up", Message: fmt.Sprintf("批量加速失败: %v", err)})
		return
	}
	if d.IsError() {
		return
	}
	if babigame.HasPayload(v) {
		r.state.ApplyV(v)
		r.emit(Event{Kind: "speed_up", Message: fmt.Sprintf("成功加速 %d 块地", len(batch))})
	}
}
