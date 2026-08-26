package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
