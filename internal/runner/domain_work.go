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
	plant := (*pb.PlantPolicy)(nil)
	if policy != nil {
		plant = policy.GetPlant()
	}
	cultivateEnabled := plant != nil && plant.GetCultivateEnabled()
	flowerUpgradeEnabled := plant != nil && plant.GetFlowerUpgradeEnabled()
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
				switch {
				case err != nil:
					r.emit(Event{Kind: "cultivate_recv", Message: fmt.Sprintf("领取培育 %s 失败: %v", state.FlowerName(fid), err)})
				case d.IsError():
					r.emit(Event{Kind: "cultivate_recv", Message: fmt.Sprintf("领取培育 %s: %s", state.FlowerName(fid), d.ErrorMsg())})
				case babigame.HasPayload(v):
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
				switch {
				case err != nil:
					r.emit(Event{Kind: "flower_upgrade", Message: fmt.Sprintf("升级 %s 失败: %v", state.FlowerName(fid), err)})
				case d.IsError():
					if missingItemID := d.MissingItemID(); missingItemID > 0 {
						have := r.state.Inventory()[missingItemID]
						r.setFlowerUpgradeBlockedForItem(fid, missingItemID, have)
					} else {
						r.setFlowerUpgradeBlocked(fid, true)
						r.emit(Event{Kind: "flower_upgrade", Message: fmt.Sprintf("升级 %s: %s", state.FlowerName(fid), d.ErrorMsg())})
					}
				case babigame.HasPayload(v):
					r.state.ApplyV(v)
					newCv := r.state.Cultivations()[fid]
					if newCv.Lvl > prevLvl {
						r.setFlowerUpgradeBlocked(fid, false)
						r.emit(Event{Kind: "flower_upgrade", Message: fmt.Sprintf("成功升级 %s (等级 %d→%d)", state.FlowerName(fid), prevLvl, newCv.Lvl)})
					}
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
	switch {
	case err != nil:
		r.emit(Event{Kind: "cultivate_new", Message: fmt.Sprintf("开始培育 %s 失败: %v", state.FlowerName(fid), err)})
	case d.IsError():
		r.emit(Event{Kind: "cultivate_new", Message: fmt.Sprintf("开始培育 %s: %s", state.FlowerName(fid), d.ErrorMsg())})
	case babigame.HasPayload(v):
		r.state.ApplyV(v)
		r.emit(Event{Kind: "cultivate_new", Message: fmt.Sprintf("成功开始培育 %s", state.FlowerName(fid))})
	}
}

func (r *Runner) tickDomainWork(ctx context.Context, client *babigame.Client, session *babigame.Session) {
	if time.Since(r.lastDomainTick) < 60*time.Second {
		return
	}
	r.lastDomainTick = time.Now()

	r.mu.RLock()
	policy := r.policy
	landUnlockBlocked := r.landUnlockBlocked
	taskRecvBlocked := r.taskRecvBlocked
	storyUnlockBlocked := r.storyUnlockBlocked
	freeWaterBlockedUntil := r.freeWaterBlockedUntil
	dailyTaskBlockedUntil := r.dailyTaskBlockedUntil
	weeklyTaskBlockedUntil := r.weeklyTaskBlockedUntil
	mailBlockedUntil := r.mailBlockedUntil
	signBlockedUntil := r.signBlockedUntil
	unionBuildBlockedUntil := r.unionBuildBlockedUntil
	residentOrderBlockedUntil := r.residentOrderBlockedUntil
	r.mu.RUnlock()
	if policy == nil {
		return
	}
	basic := policy.GetBasic()
	plant := policy.GetPlant()
	orderPolicy := policy.GetOrder()
	unionPolicy := policy.GetUnion()
	rpc := r.runnerRPC(client, session)

	// 水资源奖励
	if basic.GetWaterwheelEnabled() && r.state.WaterwheelCooldownReady() {
		prevWW := r.state.WaterwheelClaimedCount()
		v, d, err := rpcResult(rpc.Waterwheel().Recv(ctx, clientproto.WaterwheelRecvRequest{}))
		if r.isSessionInvalidated() {
			return
		}
		switch {
		case err != nil:
			r.emit(Event{Kind: "waterwheel", Message: fmt.Sprintf("领取水车失败: %v", err)})
		case d.IsError():
			// 冷却未到或无可领取，静默
		case babigame.HasPayload(v):
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
	if basic.GetFreeWaterEnabled() && time.Now().After(freeWaterBlockedUntil) {
		if idx, ok := r.state.NextFreeWaterIndex(); ok {
			v, d, err := rpcResult(rpc.FreeWater().Recv(ctx, clientproto.FreeWaterRecvRequest{Idx: idx}))
			if r.isSessionInvalidated() {
				return
			}
			switch {
			case err != nil:
				r.setFreeWaterBlockedUntil(time.Now().Add(freeWaterRetryWait))
				r.emit(Event{Kind: "free_water", Message: fmt.Sprintf("领取免费水滴 #%d 失败: %v", idx, err)})
			case d.IsError():
				r.setFreeWaterBlockedUntil(time.Now().Add(freeWaterRetryWait))
			case babigame.HasPayload(v):
				r.state.ApplyV(v)
				r.emit(Event{Kind: "free_water", Message: fmt.Sprintf("成功领取免费水滴 #%d", idx)})
			default:
				r.setFreeWaterBlockedUntil(time.Now().Add(freeWaterRetryWait))
			}
		}
	}

	if basic.GetMailEnabled() && time.Now().After(mailBlockedUntil) {
		r.tickMail(ctx, rpc)
		if r.isSessionInvalidated() {
			return
		}
	}

	// 福利箱
	if basic.GetBenefitBoxEnabled() && r.state.BenefitBoxReady() {
		v, d, err := rpcResult(rpc.BenefitBox().Draw(ctx, clientproto.BenefitBoxDrawRequest{}))
		if r.isSessionInvalidated() {
			return
		}
		switch {
		case err != nil:
			r.emit(Event{Kind: "benefit_box", Message: fmt.Sprintf("福利箱抽奖失败: %v", err)})
		case d.IsError():
			// 无可领取，静默
		case babigame.HasPayload(v):
			r.state.ApplyV(v)
			r.emit(Event{Kind: "benefit_box", Message: fmt.Sprintf("成功领取福利箱 (剩余%d次)", r.state.BenefitBoxDrawsRemaining())})
		}
	}

	// 批量加速
	if plant.GetSpeedUpEnabled() {
		r.tickSpeedUp(ctx, client, session)
		if r.isSessionInvalidated() {
			return
		}
	}

	// 居民订单
	if residentOrderEnabled(orderPolicy.GetResident()) && time.Now().After(residentOrderBlockedUntil) {
		r.tickResidentOrders(ctx, rpc)
		if r.isSessionInvalidated() {
			return
		}
	}

	// 顾客订单
	if orderPolicy.GetCustomer().GetEnabled() {
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
				if orderPolicy.GetCustomer().GetCraftEnabled() {
					crafted, stop := r.tryCraftCustomerOrderArt(ctx, client, session, npcId, order, inventory, orderPolicy.GetFlowerArt().GetRewardEnabled())
					if stop {
						return
					}
					if crafted {
						inventory = r.state.Inventory()
					}
				}
			}
			if !canFulfillCustomerOrder(order, inventory) {
				if orderPolicy.GetCustomer().GetRejectEnabled() {
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

	if orderPolicy.GetFlowerArt().GetSellEnabled() {
		// 一键收取花架收入
		v, d, err := rpcResult(rpc.FlowerRack().RecvOneKey(ctx, clientproto.FlowerRackRecvOneKeyRequest{}))
		if r.isSessionInvalidated() {
			return
		}
		if err == nil && !d.IsError() && babigame.HasPayload(v) {
			r.state.ApplyV(v)
		}
		// 上架花艺
		if r.tryStockFlowerRack(ctx, client, session, orderPolicy.GetFlowerArt().GetCraftEnabled(), orderPolicy.GetFlowerArt().GetRewardEnabled()) {
			return
		}
	}

	if orderPolicy.GetResident().GetRewardEnabled() {
		if targets := r.state.ReadyFlowerOrderRewardTargets(); len(targets) > 0 {
			target := targets[0]
			v, d, err := rpcResult(rpc.OrderFlower().RecvOrderRwd(ctx, clientproto.OrderFlowerRecvOrderRwdRequest{Target: target}))
			if r.isSessionInvalidated() {
				return
			}
			if err != nil {
				r.emit(Event{Kind: "order_reward", Message: fmt.Sprintf("领取居民订单阶段奖励 #%d 失败: %v", target, err)})
			} else if !d.IsError() && babigame.HasPayload(v) {
				r.state.ApplyV(v)
				r.emit(Event{Kind: "order_reward", Message: fmt.Sprintf("成功领取居民订单阶段奖励 #%d", target)})
			}
		}
	}

	if orderPolicy.GetResident().GetAdRefreshEnabled() {
		if boxIDs := r.state.ReadyFlowerOrderAdBoxIDs(); len(boxIDs) > 0 {
			boxID := boxIDs[0]
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
			} else if !d.IsError() && babigame.HasPayload(v) {
				r.state.ApplyV(v)
				afterOrder := r.state.FlowerOrders()[boxID]
				if residentAdSlotMaterialized(beforeOrder, afterOrder) {
					r.emit(Event{Kind: "order_ad", Message: fmt.Sprintf("居民订单广告位 #%d 已刷新为订单", boxID)})
				}
			}
		}
	}

	// 土地开垦
	if plant.GetLandUnlockEnabled() && !landUnlockBlocked {
		if nextLandID, ok := nextLandUnlockCandidate(r.state); ok {
			v, d, err := rpcResult(rpc.UsrLand().UnlockLand(ctx, clientproto.UsrLandUnlockLandRequest{LandId: nextLandID}))
			if r.isSessionInvalidated() {
				return
			}
			switch {
			case err != nil:
				r.emit(Event{Kind: "land_unlock", Message: fmt.Sprintf("开垦 #%d 失败: %v", nextLandID, err)})
			case d.IsError():
				r.setLandUnlockBlocked(true)
				r.emit(Event{Kind: "land_unlock", Message: fmt.Sprintf("开垦 #%d: %s", nextLandID, d.ErrorMsg())})
			case babigame.HasPayload(v):
				r.state.ApplyV(v)
				r.emit(Event{Kind: "land_unlock", Message: fmt.Sprintf("成功开垦 #%d", nextLandID)})
			default:
				r.setLandUnlockBlocked(true)
			}
		}
	}

	// 主线任务奖励
	if basic.GetMainTaskEnabled() && !taskRecvBlocked {
		v, d, err := rpcResult(rpc.TaskMain().Recv(ctx, clientproto.TaskMainRecvRequest{}))
		if r.isSessionInvalidated() {
			return
		}
		switch {
		case err != nil:
			r.emit(Event{Kind: "task_recv", Message: fmt.Sprintf("领取任务奖励失败: %v", err)})
		case d.IsError():
			r.setTaskRecvBlocked(true)
		case babigame.HasPayload(v):
			r.state.ApplyV(v)
			r.emit(Event{Kind: "task_recv", Message: "成功领取主线任务奖励"})
		default:
			r.setTaskRecvBlocked(true)
		}
	}
	if basic.GetDailyTaskEnabled() && time.Now().After(dailyTaskBlockedUntil) {
		if taskIDs := r.state.ReadyDailyTaskIDs(); len(taskIDs) > 0 {
			taskID := taskIDs[0]
			v, d, err := rpcResult(rpc.TaskDly().Recv(ctx, clientproto.TaskDlyRecvRequest{ID: taskID}))
			if r.isSessionInvalidated() {
				return
			}
			switch {
			case err != nil:
				r.setDailyTaskBlockedUntil(time.Now().Add(dailyTaskRetryWait))
				r.emit(Event{Kind: "task_daily", Message: fmt.Sprintf("领取日常任务 #%d 失败: %v", taskID, err)})
			case d.IsError():
				r.setDailyTaskBlockedUntil(time.Now().Add(dailyTaskRetryWait))
			case babigame.HasPayload(v):
				r.state.ApplyV(v)
				r.emit(Event{Kind: "task_daily", Message: fmt.Sprintf("成功领取日常任务 #%d", taskID)})
			default:
				r.setDailyTaskBlockedUntil(time.Now().Add(dailyTaskRetryWait))
			}
		}
	}
	if basic.GetWeeklyTaskEnabled() && time.Now().After(weeklyTaskBlockedUntil) {
		if taskIDs := r.state.ReadyWeeklyTaskIDs(); len(taskIDs) > 0 {
			taskID := taskIDs[0]
			v, d, err := rpcResult(rpc.TaskWeek().Recv(ctx, clientproto.TaskWeekRecvRequest{ID: taskID}))
			if r.isSessionInvalidated() {
				return
			}
			switch {
			case err != nil:
				r.setWeeklyTaskBlockedUntil(time.Now().Add(weeklyTaskRetryWait))
				r.emit(Event{Kind: "task_weekly", Message: fmt.Sprintf("领取每周任务 #%d 失败: %v", taskID, err)})
			case d.IsError():
				r.setWeeklyTaskBlockedUntil(time.Now().Add(weeklyTaskRetryWait))
			case babigame.HasPayload(v):
				r.state.ApplyV(v)
				r.emit(Event{Kind: "task_weekly", Message: fmt.Sprintf("成功领取每周任务 #%d", taskID)})
			default:
				r.setWeeklyTaskBlockedUntil(time.Now().Add(weeklyTaskRetryWait))
			}
		}
	}
	if basic.GetRoadGrowRewardEnabled() {
		if taskIDs := r.state.ReadyRoadGrowTaskIDs(); len(taskIDs) > 0 {
			taskID := taskIDs[0]
			v, d, err := rpcResult(rpc.RoadGrow().Recv(ctx, clientproto.RoadGrowRecvRequest{ID: taskID}))
			if r.isSessionInvalidated() {
				return
			}
			if err != nil {
				r.emit(Event{Kind: "road_grow", Message: fmt.Sprintf("领取成长之路 #%d 失败: %v", taskID, err)})
			} else if !d.IsError() && babigame.HasPayload(v) {
				r.state.ApplyV(v)
				r.emit(Event{Kind: "road_grow", Message: fmt.Sprintf("成功领取成长之路 #%d", taskID)})
			}
		}
	}
	if basic.GetSignEnabled() && time.Now().After(signBlockedUntil) {
		r.tickSign(ctx, rpc)
		if r.isSessionInvalidated() {
			return
		}
	}
	if basic.GetRandomEventEnabled() {
		_, _, _ = rpcResult(rpc.RandomEvent().Enter(ctx, clientproto.RandomEventEnterRequest{}))
		if r.isSessionInvalidated() {
			return
		}
		if eventIDs := r.state.ReadyRandomEventIDs(); len(eventIDs) > 0 {
			eventID := eventIDs[0]
			v, d, err := rpcResult(rpc.RandomEvent().DoAffair(ctx, clientproto.RandomEventDoAffairRequest{EventId: eventID}))
			if r.isSessionInvalidated() {
				return
			}
			if err != nil {
				r.emit(Event{Kind: "random_event", Message: fmt.Sprintf("领取地图事件 #%d 失败: %v", eventID, err)})
			} else if !d.IsError() && babigame.HasPayload(v) {
				r.state.ApplyV(v)
				r.emit(Event{Kind: "random_event", Message: fmt.Sprintf("成功领取地图事件 #%d", eventID)})
			}
		}
	}

	r.tickCaptureAlignedDomains(ctx, rpc, policy)
	if r.isSessionInvalidated() {
		return
	}

	// 成就任务奖励
	if basic.GetAchievementTaskEnabled() {
		r.tickTaskAchRewards(ctx, client, session)
		if r.isSessionInvalidated() {
			return
		}
	}

	// 主线剧情解锁
	if basic.GetStoryEnabled() && !storyUnlockBlocked {
		v, d, err := rpcResult(rpc.StoryMain().Unlock(ctx, clientproto.StoryMainUnlockRequest{}))
		if r.isSessionInvalidated() {
			return
		}
		switch {
		case err != nil:
			r.emit(Event{Kind: "story_unlock", Message: fmt.Sprintf("解锁剧情失败: %v", err)})
		case d.IsError():
			r.setStoryUnlockBlocked(true)
		case babigame.HasPayload(v):
			r.state.ApplyV(v)
			r.emit(Event{Kind: "story_unlock", Message: "成功解锁主线剧情"})
		default:
			r.setStoryUnlockBlocked(true)
		}
	}

	if unionBuildEnabled(unionPolicy) && time.Now().After(unionBuildBlockedUntil) {
		r.tickUnionBuild(ctx, rpc, policy)
		if r.isSessionInvalidated() {
			return
		}
	}
}

func unionBuildEnabled(policy *pb.UnionPolicy) bool {
	return policy != nil && (policy.GetBuildFreeEnabled() || policy.GetBuildGoldEnabled() || policy.GetBuildDiamondEnabled())
}

func (r *Runner) tickUnionBuild(ctx context.Context, rpc *clientrpc.Client, policy *pb.Policy) {
	union := policy.GetUnion()
	if union.GetBuildFreeEnabled() && r.unionBuildFreeReady() {
		r.runUnionBuildShare(ctx, rpc)
		return
	}
	if union.GetBuildGoldEnabled() && r.canRunUnionBuildSpend(policy, 2) {
		r.runUnionBuildSpend(ctx, rpc, 2)
		return
	}
	if union.GetBuildDiamondEnabled() && r.canRunUnionBuildSpend(policy, 3) {
		r.runUnionBuildSpend(ctx, rpc, 3)
		return
	}
}

func (r *Runner) runUnionBuildShare(ctx context.Context, rpc *clientrpc.Client) {
	v, d, err := rpcResult(rpc.Usr().Share(ctx, clientproto.UsrShareRequest{
		ShareId: 14,
		Ext: map[string]any{
			"opType": 1,
			"id":     1,
		},
	}))
	if r.isSessionInvalidated() {
		return
	}
	switch {
	case err != nil:
		r.setUnionBuildBlockedUntil(time.Now().Add(unionBuildRetryWait))
		r.emit(Event{Kind: "union_build", Message: fmt.Sprintf("公会视频建设失败: %v", err)})
	case d.IsError():
		if unionBuildDailyLimitLike(d.ErrorMsg()) {
			r.setUnionBuildFreeBlockedUntil(nextLocalDay(time.Now()))
		} else {
			r.setUnionBuildBlockedUntil(time.Now().Add(unionBuildRetryWait))
		}
	case babigame.HasPayload(v):
		r.state.ApplyV(v)
		r.setUnionBuildFreeBlockedUntil(nextLocalDay(time.Now()))
		r.emit(Event{Kind: "union_build", Message: "成功完成公会视频建设"})
	default:
		r.setUnionBuildBlockedUntil(time.Now().Add(unionBuildRetryWait))
	}
}

func (r *Runner) canRunUnionBuildSpend(policy *pb.Policy, buildID int32) bool {
	option, ok := state.FmlBuildOptionByID(buildID)
	if !ok || option.Cost <= 0 {
		return false
	}
	switch option.ItemID {
	case 11:
		if policy.GetSafety().GetMaxGoldSpendPerTick() < option.Cost {
			return false
		}
		return r.state.Gold() >= option.Cost
	case 1:
		if policy.GetSafety().GetMaxDiamondSpendPerTick() < option.Cost {
			return false
		}
		free, paid := r.state.Diamonds()
		return free+paid >= option.Cost
	default:
		if policy.GetSafety().GetMaxItemSpendPerTick() < option.Cost {
			return false
		}
		return r.state.Inventory()[option.ItemID] >= option.Cost
	}
}

func (r *Runner) runUnionBuildSpend(ctx context.Context, rpc *clientrpc.Client, buildID int32) {
	option, _ := state.FmlBuildOptionByID(buildID)
	v, d, err := rpcResult(rpc.Fml().Bld(ctx, clientproto.FmlBldRequest{"id": buildID}))
	if r.isSessionInvalidated() {
		return
	}
	name := option.Name
	if name == "" {
		name = fmt.Sprintf("建设 #%d", buildID)
	}
	switch {
	case err != nil:
		r.setUnionBuildBlockedUntil(time.Now().Add(unionBuildRetryWait))
		r.emit(Event{Kind: "union_build", Message: fmt.Sprintf("%s 失败: %v", name, err)})
	case d.IsError():
		if unionBuildDailyLimitLike(d.ErrorMsg()) {
			r.setUnionBuildBlockedUntil(nextLocalDay(time.Now()))
		} else {
			r.setUnionBuildBlockedUntil(time.Now().Add(unionBuildRetryWait))
		}
	case babigame.HasPayload(v):
		r.state.ApplyV(v)
		r.emit(Event{Kind: "union_build", Message: fmt.Sprintf("成功完成%s", name)})
	default:
		r.setUnionBuildBlockedUntil(time.Now().Add(unionBuildRetryWait))
	}
}

func (r *Runner) unionBuildFreeReady() bool {
	r.mu.RLock()
	blockedUntil := r.unionBuildFreeBlockedUntil
	r.mu.RUnlock()
	return time.Now().After(blockedUntil)
}

func unionBuildDailyLimitLike(msg string) bool {
	if strings.Contains(msg, "今日") || strings.Contains(msg, "上限") || strings.Contains(msg, "次数") {
		return true
	}
	return false
}

func (r *Runner) tickMail(ctx context.Context, rpc *clientrpc.Client) {
	v, d, err := rpcResult(rpc.Mail().GetList(ctx, clientproto.MailGetListRequest{}))
	if r.isSessionInvalidated() {
		return
	}
	switch {
	case err != nil:
		r.setMailBlockedUntil(time.Now().Add(mailRetryWait))
		r.emit(Event{Kind: "mail_claim", Message: fmt.Sprintf("同步邮件失败: %v", err)})
		return
	case d.IsError():
		r.setMailBlockedUntil(time.Now().Add(mailRetryWait))
		return
	case babigame.HasPayload(v):
		r.state.ApplyV(v)
	}

	v, d, err = rpcResult(rpc.Mail().PickOneKey(ctx, clientproto.MailPickOneKeyRequest{}))
	if r.isSessionInvalidated() {
		return
	}
	switch {
	case err != nil:
		r.setMailBlockedUntil(time.Now().Add(mailRetryWait))
		r.emit(Event{Kind: "mail_claim", Message: fmt.Sprintf("一键领取邮件失败: %v", err)})
	case d.IsError():
		r.setMailBlockedUntil(time.Now().Add(mailRetryWait))
	case babigame.HasPayload(v):
		r.state.ApplyV(v)
		r.emit(Event{Kind: "mail_claim", Message: "成功一键领取邮件附件"})
	default:
		r.setMailBlockedUntil(time.Now().Add(mailRetryWait))
	}
}

func (r *Runner) tickSign(ctx context.Context, rpc *clientrpc.Client) {
	v, d, err := rpcResult(rpc.SignType().Enter(ctx, clientproto.SignTypeEnterRequest{Type: 1}))
	if r.isSessionInvalidated() {
		return
	}
	if err != nil {
		r.setSignBlockedUntil(time.Now().Add(signRetryWait))
		r.emit(Event{Kind: "sign_claim", Message: fmt.Sprintf("同步签到失败: %v", err)})
		return
	}
	if !d.IsError() && babigame.HasPayload(v) {
		r.state.ApplyV(v)
	}

	signed := false
	v, d, err = rpcResult(rpc.SignType().Sign(ctx, clientproto.SignTypeSignRequest{Type: 1}))
	if r.isSessionInvalidated() {
		return
	}
	switch {
	case err != nil:
		r.setSignBlockedUntil(time.Now().Add(signRetryWait))
		r.emit(Event{Kind: "sign_claim", Message: fmt.Sprintf("签到失败: %v", err)})
		return
	case d.IsError():
		r.setSignBlockedUntil(nextLocalDay(time.Now()))
	case babigame.HasPayload(v):
		r.state.ApplyV(v)
		signed = true
	}

	v, d, err = rpcResult(rpc.SignType().Recv(ctx, clientproto.SignTypeRecvRequest{Type: 1}))
	if r.isSessionInvalidated() {
		return
	}
	switch {
	case err != nil:
		r.setSignBlockedUntil(time.Now().Add(signRetryWait))
		r.emit(Event{Kind: "sign_claim", Message: fmt.Sprintf("领取签到奖励失败: %v", err)})
	case d.IsError():
		if signed {
			r.emit(Event{Kind: "sign_claim", Message: "成功签到"})
		}
		r.setSignBlockedUntil(nextLocalDay(time.Now()))
	case babigame.HasPayload(v):
		r.state.ApplyV(v)
		r.setSignBlockedUntil(nextLocalDay(time.Now()))
		r.emit(Event{Kind: "sign_claim", Message: "成功领取签到/祈愿奖励"})
	default:
		r.setSignBlockedUntil(time.Now().Add(signRetryWait))
	}
}

func residentOrderEnabled(policy *pb.ResidentOrderPolicy) bool {
	return policy != nil && (policy.GetNormalEnabled() || policy.GetDecorateEnabled() || policy.GetSatinEnabled())
}

func (r *Runner) tickCaptureAlignedDomains(ctx context.Context, rpc *clientrpc.Client, policy *pb.Policy) {
	if policy == nil {
		return
	}
	basic := policy.GetBasic()
	order := policy.GetOrder()
	union := policy.GetUnion()
	activity := policy.GetActivity()
	if order.GetPalace().GetEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCOrderPalaceEnter.String(), map[string]any{})
		if r.isSessionInvalidated() {
			return
		}
		r.observeStateDelta(ctx, rpc, clientproto.RPCOrderPalaceGetOrderRcdList.String(), map[string]any{})
	}
	if order.GetCustomer().GetEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCOrderCustomerGenOrder.String(), map[string]any{"guestNpcIdList": []int32{}})
	}
	if order.GetTeam().GetEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCOrderTeamRefreshOrder.String(), map[string]any{})
	}
	if basic.GetSignEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCSignTypeEnter.String(), map[string]any{"type": int32(1)})
	}
	if basic.GetPearl().GetEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCPearlRefresh.String(), map[string]any{})
		if r.isSessionInvalidated() {
			return
		}
		r.observeStateDelta(ctx, rpc, clientproto.RPCPearlPlaceRecvOneKey.String(), map[string]any{})
	}
	if basic.GetShop().GetCultivateShopEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCShopCultivateEnter.String(), map[string]any{})
	}
	if basic.GetZoo().GetSyncEnabled() || basic.GetZoo().GetEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCZooEnterZoo.String(), map[string]any{})
		if r.isSessionInvalidated() {
			return
		}
		r.observeStateDelta(ctx, rpc, clientproto.RPCZooFindPetByUsrBack.String(), map[string]any{})
		if r.isSessionInvalidated() {
			return
		}
		r.observeStateDelta(ctx, rpc, clientproto.RPCZooRefreshPetStatus.String(), map[string]any{"petIdList": []int32{1}})
		if r.isSessionInvalidated() {
			return
		}
		r.observeStateDelta(ctx, rpc, clientproto.RPCZooReadLog.String(), map[string]any{"petId": int32(1)})
	}
	if union.GetBuildFreeEnabled() || union.GetBuildGoldEnabled() || union.GetBuildDiamondEnabled() ||
		union.GetFlowerShareEnabled() || union.GetFlowerTakeEnabled() || union.GetRaceEnabled() ||
		union.GetLandAutoPlant() || union.GetLandHarvest() || union.GetForestEnabled() {
		r.observeStateDelta(ctx, rpc, clientproto.RPCFmlEnter.String(), map[string]any{"fml": int32(1)})
		if r.isSessionInvalidated() {
			return
		}
		if union.GetRaceEnabled() {
			r.observeStateDelta(ctx, rpc, clientproto.RPCFmlRaceEnter.String(), map[string]any{})
			r.observeStateDelta(ctx, rpc, clientproto.RPCFmlRaceGetTaskList.String(), map[string]any{})
		}
		if union.GetFlowerShareEnabled() || union.GetFlowerTakeEnabled() {
			r.observeStateDelta(ctx, rpc, clientproto.RPCFmlFlowerShareRefresh.String(), map[string]any{})
		}
		if union.GetForestEnabled() {
			r.observeStateDelta(ctx, rpc, clientproto.RPCFmlForestEnter.String(), map[string]any{})
		}
	}
	if activity.GetEnabled() {
		r.syncEnabledActivityModules(ctx, rpc, activity)
	}
}

func (r *Runner) syncEnabledActivityModules(ctx context.Context, rpc *clientrpc.Client, policy *pb.ActivityPolicy) {
	keys := make([]string, 0, len(policy.GetModules()))
	for name, module := range policy.GetModules() {
		if module != nil && module.GetEnabled() {
			keys = append(keys, name)
		}
	}
	sort.Strings(keys)
	for _, name := range keys {
		switch name {
		case "actCyclicStory":
			r.observeStateDelta(ctx, rpc, clientproto.RPCActCyclicStoryEnter.String(), map[string]any{})
		case "actDessert":
			r.observeStateDelta(ctx, rpc, clientproto.RPCActDessertEnter.String(), map[string]any{})
		case "actElim":
			r.observeStateDelta(ctx, rpc, clientproto.RPCActElimEnter.String(), map[string]any{})
		case "actMerge2":
			r.observeStateDelta(ctx, rpc, clientproto.RPCActMerge2Enter.String(), map[string]any{})
		case "actSpool":
			r.observeStateDelta(ctx, rpc, clientproto.RPCActSpoolEnter.String(), map[string]any{})
		case "cyclicNote":
			r.observeStateDelta(ctx, rpc, clientproto.RPCActCyclicNoteEnter.String(), map[string]any{})
		case "moneyTree":
			r.observeStateDelta(ctx, rpc, clientproto.RPCActMoneyTreeEnter.String(), map[string]any{})
		case "redPacket":
			r.observeStateDelta(ctx, rpc, clientproto.RPCActRedpacketRedPacketRecv.String(), map[string]any{})
		case "zooGameElim":
			r.observeStateDelta(ctx, rpc, clientproto.RPCZooGameEnter.String(), map[string]any{"type": int32(1)})
		default:
			r.emit(Event{
				Kind:     "activity_sync",
				Category: "activity",
				Domain:   "activity." + name,
				Action:   "sync",
				Message:  fmt.Sprintf("活动模块 %s 暂无安全同步入口", name),
				Level:    "warn",
			})
		}
		if r.isSessionInvalidated() || ctx.Err() != nil {
			return
		}
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

func (r *Runner) setWeeklyTaskBlockedUntil(until time.Time) {
	r.mu.Lock()
	r.weeklyTaskBlockedUntil = until
	r.mu.Unlock()
}

func (r *Runner) setMailBlockedUntil(until time.Time) {
	r.mu.Lock()
	r.mailBlockedUntil = until
	r.mu.Unlock()
}

func (r *Runner) setSignBlockedUntil(until time.Time) {
	r.mu.Lock()
	r.signBlockedUntil = until
	r.mu.Unlock()
}

func (r *Runner) setUnionBuildBlockedUntil(until time.Time) {
	r.mu.Lock()
	r.unionBuildBlockedUntil = until
	r.mu.Unlock()
}

func (r *Runner) setUnionBuildFreeBlockedUntil(until time.Time) {
	r.mu.Lock()
	r.unionBuildFreeBlockedUntil = until
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
