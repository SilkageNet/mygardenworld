package runner

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	flowerUpgradeRetryWait = 30 * time.Minute
	cultivateRetryWait     = 30 * time.Minute
)

func (r *Runner) tickCultivate(ctx context.Context, client *babigame.Client, session *babigame.Session) {
	r.mu.RLock()
	policy := r.policy
	r.mu.RUnlock()
	misc := policy.GetMisc()
	cultivateEnabled := misc != nil && misc.GetCultivateEnabled()
	flowerUpgradeEnabled := misc != nil && misc.GetFlowerUpgradeEnabled()
	if !cultivateEnabled && !flowerUpgradeEnabled {
		return
	}

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
				v, d, err := client.RPC(ctx, "cultivate.recv", map[string]any{"flowerId": int(fid)}, session.RouteArg(), 10*time.Second)
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
				v, d, err := client.RPC(ctx, "cultivate.upgrade", map[string]any{"flowerId": int(fid)}, session.RouteArg(), 10*time.Second)
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
	v, d, err := client.RPC(ctx, "cultivate.cultivate", map[string]any{"flowerId": int(fid)}, session.RouteArg(), 10*time.Second)
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
	misc := policy.GetMisc()

	// 水资源奖励
	if misc.GetWaterwheelEnabled() && r.state.WaterwheelCooldownReady() {
		prevWW := r.state.WaterwheelClaimedCount()
		v, d, err := client.RPC(ctx, "waterwheel.recv", map[string]any{}, session.RouteArg(), 10*time.Second)
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
				v2, _, _ := client.RPC(ctx, "waterwheel.skip", map[string]any{}, session.RouteArg(), 10*time.Second)
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
			v, d, err := client.RPC(ctx, "freeWater.recv", map[string]any{"idx": int(idx)}, session.RouteArg(), 10*time.Second)
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

	// 居民订单
	if misc.GetResidentOrderEnabled() && time.Now().After(residentOrderBlockedUntil) {
		inv := r.state.FlowerInventory()
		orders := r.state.FlowerOrders()
		for boxID := int32(1); boxID <= 6; boxID++ {
			order, exists := orders[boxID]
			if !exists || len(order.Requires) == 0 {
				continue
			}
			canFulfill := true
			for _, req := range order.Requires {
				if inv[req.FlowerID] < req.Count {
					canFulfill = false
					break
				}
			}
			if !canFulfill {
				continue
			}
			v, d, err := client.RPC(ctx, "orderFlower.finishOrder", map[string]any{"boxId": int(boxID)}, session.RouteArg(), 10*time.Second)
			if r.isSessionInvalidated() {
				return
			}
			if err != nil {
				r.emit(Event{Kind: "order_finish", Message: fmt.Sprintf("完成居民订单 #%d 失败: %v", boxID, err)})
				break
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
					break
				}
				r.emit(Event{Kind: "order_finish", Message: fmt.Sprintf("完成居民订单 #%d: %s", boxID, msg)})
				continue
			}
			if babigame.HasPayload(v) {
				r.state.ApplyV(v)
				r.emit(Event{Kind: "order_finish", Message: fmt.Sprintf("成功完成居民订单 #%d", boxID)})
				inv = r.state.FlowerInventory()
			}
		}
	}

	// 顾客订单
	if misc.GetCustomerOrderEnabled() {
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
					v, d, err := client.RPC(ctx, "orderCustomer.rejectOrder", map[string]any{"npcId": int(npcId)}, session.RouteArg(), 10*time.Second)
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
			v, d, err := client.RPC(ctx, "orderCustomer.finishOrder", map[string]any{"npcId": int(npcId)}, session.RouteArg(), 10*time.Second)
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
		if r.tryStockFlowerRack(ctx, client, session, misc.GetFlowerRackCraftEnabled(), misc.GetFlowerArtRewardEnabled()) {
			return
		}
	}

	if misc.GetResidentOrderRewardEnabled() {
		for _, target := range r.state.ReadyFlowerOrderRewardTargets() {
			v, d, err := client.RPC(ctx, "orderFlower.recvOrderRwd", map[string]any{"target": int(target)}, session.RouteArg(), 10*time.Second)
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
			v, d, err := client.RPC(ctx, "usr.share", map[string]any{
				"shareId": 5,
				"ext": map[string]any{
					"opType": 1,
					"id":     int(boxID),
				},
			}, session.RouteArg(), 10*time.Second)
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
			v, d, err := client.RPC(ctx, "usrLand.unlockLand", map[string]any{"landId": int(nextLandID)}, session.RouteArg(), 10*time.Second)
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
		v, d, err := client.RPC(ctx, "taskMain.recv", map[string]any{}, session.RouteArg(), 10*time.Second)
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
			v, d, err := client.RPC(ctx, "taskDly.recv", map[string]any{"id": int(taskID)}, session.RouteArg(), 10*time.Second)
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
			v, d, err := client.RPC(ctx, "roadGrow.recv", map[string]any{"id": int(taskID)}, session.RouteArg(), 10*time.Second)
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
		for _, eventID := range r.state.ReadyRandomEventIDs() {
			v, d, err := client.RPC(ctx, "randomEvent.doAffair", map[string]any{"eventId": int(eventID)}, session.RouteArg(), 10*time.Second)
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

	// 主线剧情解锁
	if misc.StoryUnlockEnabled && !storyUnlockBlocked {
		v, d, err := client.RPC(ctx, "storyMain.unlock", map[string]any{}, session.RouteArg(), 10*time.Second)
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

func (r *Runner) setLandUnlockBlocked(v bool) {
	r.mu.Lock()
	r.landUnlockBlocked = v
	r.mu.Unlock()
}

func nextLandUnlockCandidate(st *state.State) (int32, bool) {
	if !st.LandRosterObserved() || !st.FarmLandConfigObserved() {
		return 0, false
	}
	lands := st.Lands()
	level := st.Level()
	gold := st.Gold()
	for _, info := range st.FarmLands() {
		if _, opened := lands[info.ID]; opened {
			continue
		}
		if info.OpenLevel <= 0 || level < info.OpenLevel {
			continue
		}
		if len(info.Cost) < 2 || gold < info.Cost[1] {
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
	v, d, err := client.RPC(ctx, "flowerArt.makeFlowerArt", map[string]any{
		"vaseId":     int(recipe.VaseID),
		"flowersIds": recipe.Flowers,
		"num":        1,
	}, session.RouteArg(), 10*time.Second)
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
	v, d, err := client.RPC(ctx, "flowerRack.sell", map[string]any{
		"rackId": int(rackID),
		"iid":    int(artID),
		"num":    int(num),
	}, session.RouteArg(), 10*time.Second)
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
		v, d, err := client.RPC(ctx, "flowerArt.makeFlowerArt", map[string]any{
			"vaseId":     int(recipe.VaseID),
			"flowersIds": recipe.Flowers,
			"num":        int(missingArt),
		}, session.RouteArg(), 10*time.Second)
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
	for _, call := range []struct {
		name string
		args map[string]any
	}{
		{name: "collectRwd.recvArtCreateRwd", args: map[string]any{"flowerArtId": int(artID)}},
		{name: "usr.share", args: map[string]any{
			"shareId": 6,
			"ext": map[string]any{
				"opType": 2,
				"id":     int(artID),
			},
		}},
	} {
		v, d, err := client.RPC(ctx, call.name, call.args, session.RouteArg(), 10*time.Second)
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
