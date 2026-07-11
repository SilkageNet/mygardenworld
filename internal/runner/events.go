package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func (r *Runner) emit(e Event) {
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	}
	r.mu.Lock()
	r.lastEventAt = e.TS
	r.mu.Unlock()
	e.AccountID = fmt.Sprintf("%d", r.account.ID)
	e.AccountName = r.account.Name
	if e.PayloadJSON == "" {
		e.PayloadJSON = "{}"
	}
	if e.Category == "" {
		e.Category = eventCategory(e.Kind)
	}
	e.Category = normalizeEventCategory(e.Category, e.Kind)
	if e.Domain == "" {
		e.Domain = eventDomain(e.Kind)
	}
	if e.Action == "" {
		e.Action = eventAction(e.Kind)
	}
	if e.Label == "" {
		e.Label = eventLabel(e.Kind)
	}
	if e.Level == "" {
		e.Level = eventLevel(e.Kind, e.Message)
	}
	log := r.log.With("event_kind", e.Kind, "category", e.Category, "label", e.Label)
	if r.db != nil {
		id, err := r.db.LogEvent(context.Background(), store.EventLog{
			AccountID:   r.account.ID,
			AccountName: e.AccountName,
			TS:          e.TS,
			Kind:        e.Kind,
			Message:     e.Message,
			PayloadJSON: e.PayloadJSON,
			Category:    e.Category,
			Domain:      e.Domain,
			Action:      e.Action,
			Label:       e.Label,
			Level:       e.Level,
		})
		if err != nil {
			log.Warn("persist event failed", "error", err)
		} else {
			e.ID = id
		}
	}
	if r.bus != nil {
		r.bus.Publish(e)
	}
	if isNoisyStateEvent(e) {
		log.Debug(e.Message)
		return
	}
	switch e.Level {
	case "error":
		log.Error(e.Message)
	case "warn":
		log.Warn(e.Message)
	default:
		log.Info(e.Message)
	}
}

func isNoisyStateEvent(e Event) bool {
	return e.Kind == "land_changed" || e.Kind == "resource_changed" || e.Kind == "inventory_changed"
}

func (r *Runner) emitLandChanges(changes []state.LandChange) {
	if len(changes) == 0 {
		return
	}
	if len(changes) == 1 {
		r.emitLandChange(changes[0])
		return
	}
	payloadChanges := make([]map[string]any, 0, len(changes))
	for _, c := range changes {
		payloadChanges = append(payloadChanges, landChangePayload(c))
	}
	raw, _ := json.Marshal(map[string]any{"changes": payloadChanges})
	r.emit(Event{
		Kind:        "land_changed",
		Message:     landChangesMessage(changes),
		PayloadJSON: string(raw),
	})
}

func (r *Runner) emitLandChange(c state.LandChange) {
	raw, _ := json.Marshal(landChangePayload(c))
	r.emit(Event{
		Kind:        "land_changed",
		Message:     fmt.Sprintf("田地 %d: %s (%s)", c.LandID, landStateDesc(c.After), flowerName(c.After.FlowerID)),
		PayloadJSON: string(raw),
	})
}

func landChangePayload(c state.LandChange) map[string]any {
	return map[string]any{
		"landId": c.LandID,
		"before": c.Before.ToJSON(),
		"after":  c.After.ToJSON(),
	}
}

func landChangesMessage(changes []state.LandChange) string {
	counts := make(map[string]int)
	order := make([]string, 0)
	for _, c := range changes {
		desc := landStateDesc(c.After)
		if _, ok := counts[desc]; !ok {
			order = append(order, desc)
		}
		counts[desc]++
	}
	parts := make([]string, 0, len(order))
	for _, desc := range order {
		parts = append(parts, fmt.Sprintf("%s %d", desc, counts[desc]))
	}
	return fmt.Sprintf("田地更新: %d 块变化（%s）", len(changes), strings.Join(parts, "，"))
}

func landStateDesc(v state.LandView) string {
	if v.FlowerID == 0 {
		return "空地"
	}
	switch v.State {
	case 1:
		return "已播种，待浇水"
	case 2:
		return "生长中"
	case 3:
		return "可收获"
	default:
		return fmt.Sprintf("未知状态(%d)", v.State)
	}
}

func flowerName(id int) string {
	if id == 0 {
		return "无"
	}
	if name := state.FlowerName(int32(id)); name != "" {
		return name
	}
	return fmt.Sprintf("花卉#%d", id)
}

func opKindDesc(kind string) string {
	switch kind {
	case "usrLand.plant":
		return "播种"
	case "usrLand.plantBatch":
		return "批量播种"
	case "usrLand.water":
		return "浇水"
	case "usrLand.waterBatch":
		return "批量浇水"
	case "usrLand.harvest":
		return "收获"
	case "usrLand.clear":
		return "铲除"
	case "usrLand.speedUp":
		return "加速"
	case "usrLand.speedUpFree":
		return "免费加速"
	case "usrLand.unlockLand":
		return "解锁田地"
	case clientproto.RPCZooEnterZoo.String():
		return "进入宠物模块"
	case clientproto.RPCZooRefreshPetStatus.String():
		return "刷新宠物状态"
	case clientproto.RPCZooAddFoodstuff.String():
		return "补充宠物食盆"
	case clientproto.RPCZooFeedPets.String():
		return "宠物喂食"
	case clientproto.RPCZooStrokePet.String():
		return "宠物互动"
	case clientproto.RPCZooFindPet.String():
		return "宠物寻回"
	case clientproto.RPCZooHandleEvent.String():
		return "宠物事件"
	case clientproto.RPCZooReadLog.String():
		return "确认宠物日志已读"
	case clientproto.RPCZooRecvSouvenirRwd.String():
		return "领取宠物纪念品奖励"
	case clientproto.RPCZooReadSouvenir.String():
		return "确认宠物纪念品已读"
	case clientproto.RPCRandomEventEnter.String():
		return "同步地图随机事件"
	case clientproto.RPCRandomEventDoAffair.String():
		return "处理地图随机事件"
	case clientproto.RPCStoryMainEnter.String():
		return "同步主线剧情"
	case clientproto.RPCStoryMainUnlock.String():
		return "解锁主线剧情"
	case clientproto.RPCTaskAchRecv.String():
		return "领取成就任务"
	case clientproto.RPCUsrExtraUpdateAntiFraudQAStatus.String():
		return "防骗问答"
	case clientproto.RPCUsrExtraRecvAntiFraudQARwd.String():
		return "领取防骗宝箱"
	default:
		return kind
	}
}

func inventoryChangeMessage(snap state.InventorySnapshot) string {
	if len(snap.Changes) == 0 {
		return "库存更新"
	}
	limit := len(snap.Changes)
	if limit > 4 {
		limit = 4
	}
	parts := make([]string, 0, limit+1)
	for _, change := range snap.Changes[:limit] {
		name := state.ItemName(change.ItemID)
		if name == "" {
			name = fmt.Sprintf("#%d", change.ItemID)
		}
		parts = append(parts, fmt.Sprintf("%s %d→%d", name, change.Before, change.After))
	}
	if extra := len(snap.Changes) - limit; extra > 0 {
		parts = append(parts, fmt.Sprintf("另%d项", extra))
	}
	return "库存更新: " + strings.Join(parts, "，")
}

func eventCategory(kind string) string {
	switch kind {
	case "session", "session_expired", "ws_disconnected":
		return "account"
	case "operation_planned", "operation_ack", "operation_failed", "operation_deferred":
		return "plant"
	case "policy_changed":
		return "system"
	case "land_changed", "land_unlock":
		return "plant"
	case "resource_changed", "inventory_changed":
		return "basic"
	case "waterwheel", "free_water", "benefit_box", "mail_claim", "sign_claim", "random_event":
		return "basic"
	case "task_recv", "task_daily", "task_weekly", "road_grow", "story_unlock":
		return "basic"
	case "order_finish", "order_customer", "order_customer_info", "order_reward", "order_ad", "flower_art", "flower_rack":
		return "order"
	case "union_build":
		return "union"
	case "cultivate_recv", "cultivate_new", "flower_upgrade":
		return "plant"
	default:
		return "system"
	}
}

func normalizeEventCategory(category, kind string) string {
	switch category {
	case "account", "basic", "plant", "order", "union", "activity", "system":
		return category
	case "session":
		return "account"
	case "operation", "land", "cultivation":
		return "plant"
	case "resource", "reward", "task":
		return "basic"
	default:
		return eventCategory(kind)
	}
}

func eventDomain(kind string) string {
	switch kind {
	case "session", "session_expired", "ws_disconnected":
		return "account.session"
	case "resource_changed":
		return "basic.resource"
	case "inventory_changed":
		return "basic.inventory"
	case "land_changed":
		return "farm.land"
	case "land_unlock":
		return "farm.land"
	case "operation_planned", "operation_ack", "operation_failed", "operation_deferred":
		return "farm.operation"
	case "waterwheel":
		return "basic.waterwheel"
	case "free_water":
		return "basic.free_water"
	case "benefit_box":
		return "basic.benefit"
	case "mail_claim":
		return "basic.mail"
	case "sign_claim":
		return "basic.sign"
	case "task_recv":
		return "basic.task.main"
	case "task_daily":
		return "basic.task.daily"
	case "task_weekly":
		return "basic.task.weekly"
	case "road_grow":
		return "basic.road_grow"
	case "random_event":
		return "basic.map_event"
	case "story_unlock":
		return "basic.story"
	case "order_finish", "order_reward", "order_ad":
		return "order.resident"
	case "order_customer", "order_customer_info":
		return "order.customer"
	case "flower_art", "flower_rack":
		return "order.flower_art"
	case "union_build":
		return "union.build"
	case "cultivate_recv", "cultivate_new":
		return "farm.cultivate"
	case "flower_upgrade":
		return "farm.upgrade"
	case "policy_changed":
		return "policy"
	default:
		return "system"
	}
}

func eventAction(kind string) string {
	switch {
	case kind == "operation_deferred":
		return "blocked"
	case strings.HasSuffix(kind, "_failed"):
		return "failed"
	case strings.Contains(kind, "changed"):
		return "changed"
	case strings.Contains(kind, "unlock"):
		return "unlock"
	case strings.HasPrefix(kind, "task_") || strings.Contains(kind, "claim") || strings.Contains(kind, "recv") || strings.Contains(kind, "reward") ||
		strings.Contains(kind, "waterwheel") || strings.Contains(kind, "free_water") || strings.Contains(kind, "benefit_box"):
		return "claim"
	case strings.Contains(kind, "order"):
		return "order"
	case strings.Contains(kind, "build"):
		return "build"
	case strings.Contains(kind, "upgrade"):
		return "upgrade"
	case strings.Contains(kind, "cultivate"):
		return "cultivate"
	default:
		return kind
	}
}

func eventLabel(kind string) string {
	switch kind {
	case "session":
		return "连接"
	case "session_expired":
		return "过期"
	case "ws_disconnected":
		return "断开"
	case "resource_changed":
		return "资源"
	case "inventory_changed":
		return "库存"
	case "land_changed":
		return "田地"
	case "land_unlock":
		return "开垦"
	case "operation_planned":
		return "计划"
	case "operation_ack":
		return "完成"
	case "operation_failed":
		return "失败"
	case "operation_deferred":
		return "暂缓"
	case "policy_changed":
		return "策略"
	case "waterwheel":
		return "水车"
	case "free_water":
		return "水滴"
	case "benefit_box":
		return "福利箱"
	case "mail_claim":
		return "邮件"
	case "sign_claim":
		return "签到"
	case "task_recv":
		return "主线任务"
	case "task_daily":
		return "日常任务"
	case "task_weekly":
		return "每周任务"
	case "road_grow":
		return "成长之路"
	case "random_event":
		return "地图随机事件"
	case "story_unlock":
		return "剧情"
	case "order_finish":
		return "居民订单"
	case "order_customer", "order_customer_info":
		return "顾客订单"
	case "order_reward":
		return "订单奖励"
	case "order_ad":
		return "居民订单"
	case "flower_art":
		return "花艺"
	case "flower_rack":
		return "花架"
	case "union_build":
		return "公会建设"
	case "cultivate_recv":
		return "培育领取"
	case "cultivate_new":
		return "培育"
	case "flower_upgrade":
		return "花卉升级"
	default:
		return kind
	}
}

func eventLevel(kind, message string) string {
	if strings.Contains(kind, "failed") || strings.Contains(message, "失败") {
		return "error"
	}
	if kind == "operation_deferred" ||
		kind == "session_expired" ||
		strings.Contains(message, "跳过") ||
		strings.Contains(message, "缺少") ||
		strings.Contains(message, "需要") {
		return "warn"
	}
	return "info"
}
