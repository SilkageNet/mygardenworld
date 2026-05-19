package runner

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/state"
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
	if e.Label == "" {
		e.Label = eventLabel(e.Kind)
	}
	if e.Level == "" {
		e.Level = eventLevel(e.Kind, e.Message)
	}
	if r.bus != nil {
		r.bus.Publish(e)
	}
	log := r.log.With("event_kind", e.Kind, "category", e.Category, "label", e.Label)
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
	case "usrLand.plantOneKey":
		return "一键播种"
	case "usrLand.water":
		return "浇水"
	case "usrLand.waterBatch":
		return "批量浇水"
	case "usrLand.waterOneKey":
		return "一键浇水"
	case "usrLand.harvest":
		return "收获"
	case "usrLand.harvestOneKey":
		return "一键收获"
	case "usrLand.clear":
		return "铲除"
	case "usrLand.clearOneKey":
		return "一键铲除"
	case "usrLand.speedUp":
		return "加速"
	case "usrLand.speedUpFree":
		return "免费加速"
	case "usrLand.speedUpOneKey":
		return "一键加速"
	case "usrLand.unlockLand":
		return "解锁田地"
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
		return "session"
	case "operation_planned", "operation_ack", "operation_failed":
		return "operation"
	case "policy_changed":
		return "system"
	case "land_changed", "land_unlock":
		return "land"
	case "resource_changed", "inventory_changed":
		return "resource"
	case "waterwheel", "free_water", "random_event":
		return "reward"
	case "task_recv", "task_daily", "road_grow", "story_unlock":
		return "task"
	case "order_finish", "order_customer", "flower_art":
		return "order"
	case "cultivate_recv", "cultivate_new", "flower_upgrade":
		return "cultivation"
	default:
		return "system"
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
	case "policy_changed":
		return "策略"
	case "waterwheel":
		return "水车"
	case "free_water":
		return "水滴"
	case "task_recv":
		return "主线任务"
	case "task_daily":
		return "日常任务"
	case "road_grow":
		return "成长之路"
	case "random_event":
		return "地图事件"
	case "story_unlock":
		return "剧情"
	case "order_finish":
		return "居民订单"
	case "order_customer":
		return "顾客订单"
	case "flower_art":
		return "花艺"
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
	if kind == "session_expired" ||
		strings.Contains(message, "跳过") ||
		strings.Contains(message, "缺少") ||
		strings.Contains(message, "需要") {
		return "warn"
	}
	return "info"
}
