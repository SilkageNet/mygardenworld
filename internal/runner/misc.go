package runner

import (
	"context"
	"fmt"
	"sort"
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
	orderBlocked := r.orderBlocked
	landUnlockBlocked := r.landUnlockBlocked
	taskRecvBlocked := r.taskRecvBlocked
	storyUnlockBlocked := r.storyUnlockBlocked
	freeWaterBlockedUntil := r.freeWaterBlockedUntil
	dailyTaskBlockedUntil := r.dailyTaskBlockedUntil
	r.mu.RUnlock()
	misc := policy.GetMisc()

	// 水资源奖励
	if misc.WaterwheelEnabled && r.state.WaterwheelCooldownReady() {
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
	if misc.WaterwheelEnabled && time.Now().After(freeWaterBlockedUntil) {
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
	if misc.OrderEnabled {
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
				r.emit(Event{Kind: "order_finish", Message: fmt.Sprintf("完成居民订单 #%d: %s", boxID, d.ErrorMsg())})
				continue
			}
			if babigame.HasPayload(v) {
				r.state.ApplyV(v)
				r.emit(Event{Kind: "order_finish", Message: fmt.Sprintf("成功完成居民订单 #%d", boxID)})
				inv = r.state.FlowerInventory()
			}
		}

		// 顾客订单
		if !orderBlocked {
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
					if r.tryCraftCustomerOrderArt(ctx, client, session, npcId, order, inventory) {
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
					r.setOrderBlocked(true)
					break
				}
				if babigame.HasPayload(v) {
					r.state.ApplyV(v)
					r.emit(Event{Kind: "order_customer", Message: fmt.Sprintf("成功完成顾客订单 (NPC=%d)", npcId)})
					inventory = r.state.Inventory()
				}
			}
		}
	}

	// 土地开垦
	if misc.LandUnlockEnabled && !landUnlockBlocked {
		const maxLandID = 1064 // 1001 + 64 - 1
		nextLandID := r.state.MaxLandID() + 1
		if nextLandID <= maxLandID {
			openLevel, ok := state.LandUnlockOpenLevel(nextLandID)
			if !ok {
				r.setLandUnlockBlocked(true)
			} else if level := r.state.Level(); level < openLevel {
				// Not actionable yet according to the current client table.
				// Keep this silent: the dashboard can show the locked level, but
				// the automation loop should not flood logs with static lock data.
			} else if r.state.Gold() >= state.LandUnlockCostGold {
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
	}

	// 主线任务奖励
	if misc.TaskRecvEnabled && !taskRecvBlocked {
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
	if misc.TaskRecvEnabled && time.Now().After(dailyTaskBlockedUntil) {
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
	if misc.TaskRecvEnabled {
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

func (r *Runner) setOrderBlocked(v bool) {
	r.mu.Lock()
	r.orderBlocked = v
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

func (r *Runner) tryCraftCustomerOrderArt(ctx context.Context, client *babigame.Client, session *babigame.Session, npcID int32, order *state.CustomerOrder, inventory map[int32]int32) bool {
	for _, req := range order.ItemRequires {
		missingArt := req.Count - inventory[req.ItemID]
		if req.ItemID == 0 || missingArt <= 0 {
			continue
		}
		recipe, ok := state.FlowerArtRecipeByID(req.ItemID)
		if !ok {
			continue
		}
		craftNum := missingArt
		if recipe.VaseID != 0 {
			if have := inventory[recipe.VaseID]; have < craftNum {
				craftNum = have
			}
		}
		for _, flowerID := range recipe.Flowers {
			have := inventory[flowerID]
			if have < craftNum {
				craftNum = have
			}
		}
		if craftNum <= 0 {
			continue
		}
		v, d, err := client.RPC(ctx, "flowerArt.makeFlowerArt", map[string]any{
			"vaseId":     int(recipe.VaseID),
			"flowersIds": recipe.Flowers,
			"num":        int(craftNum),
		}, session.RouteArg(), 10*time.Second)
		if r.isSessionInvalidated() {
			return true
		}
		artName := state.ItemName(req.ItemID)
		if artName == "" {
			artName = fmt.Sprintf("#%d", req.ItemID)
		}
		if err != nil {
			r.emit(Event{Kind: "flower_art", Message: fmt.Sprintf("制作花艺 %s 失败: %v", artName, err)})
			return true
		}
		if d.IsError() {
			r.setOrderBlocked(true)
			return true
		}
		if babigame.HasPayload(v) {
			r.state.ApplyV(v)
			r.emit(Event{Kind: "flower_art", Message: fmt.Sprintf("成功制作花艺 %s x%d (NPC=%d)", artName, craftNum, npcID)})
			return true
		}
	}
	return false
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
