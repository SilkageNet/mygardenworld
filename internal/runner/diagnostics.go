package runner

import (
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

// Diagnostics is the runner-facing status model used by API snapshots and the
// monitoring dashboard. It intentionally describes automation/runtime health,
// while state.State remains focused on game data.
type Diagnostics struct {
	CurrentOperation          string
	CurrentOperationStartedAt time.Time
	LastOperation             string
	LastOperationAt           time.Time
	LastOperationError        string
	LastOperationErrorAt      time.Time
	NextDecisionAt            time.Time
	NextMiscAt                time.Time
	NextCultivateAt           time.Time
	SessionInvalidatedReason  string
	BlockedReasons            []string
	UnknownRPCCount           int32
	UnknownNamespaceCount     int32
	ObservedNamespaces        []string
}

func (r *Runner) Diagnostics(now time.Time) Diagnostics {
	r.mu.RLock()
	out := Diagnostics{
		CurrentOperation:          r.currentOperation,
		CurrentOperationStartedAt: r.currentOperationStartedAt,
		LastOperation:             r.lastOperation,
		LastOperationAt:           r.lastOperationAt,
		LastOperationError:        r.lastOperationError,
		LastOperationErrorAt:      r.lastOperationErrorAt,
		NextDecisionAt:            r.nextDecisionAt,
		SessionInvalidatedReason:  r.sessionInvalidatedReason,
		UnknownRPCCount:           int32(len(r.unknownRPCCounts)),
	}
	lastMiscTick := r.lastMiscTick
	lastCultivateTick := r.lastCultivateTick
	waterBlocked, waterBlockedUntil := r.waterBlocked, r.waterBlockedUntil
	residentOrderBlockedUntil := r.residentOrderBlockedUntil
	freeWaterBlockedUntil := r.freeWaterBlockedUntil
	dailyTaskBlockedUntil := r.dailyTaskBlockedUntil
	sessionInvalidated := r.sessionInvalidated
	connected := r.client != nil && !r.client.Closed()
	materialBlockCount := len(r.flowerUpgradeBlocked) + len(r.cultivateBlocked)
	r.mu.RUnlock()

	if !lastMiscTick.IsZero() {
		out.NextMiscAt = lastMiscTick.Add(60 * time.Second)
	}
	if !lastCultivateTick.IsZero() {
		out.NextCultivateAt = lastCultivateTick.Add(60 * time.Second)
	}
	if sessionInvalidated {
		out.BlockedReasons = append(out.BlockedReasons, "会话已失效")
	}
	if !connected && !sessionInvalidated {
		out.BlockedReasons = append(out.BlockedReasons, "WebSocket 未连接")
	}
	if waterBlocked && now.Before(waterBlockedUntil) {
		out.BlockedReasons = append(out.BlockedReasons, fmt.Sprintf("缺水冷却至 %s", waterBlockedUntil.Local().Format("15:04:05")))
	}
	if now.Before(residentOrderBlockedUntil) {
		out.BlockedReasons = append(out.BlockedReasons, fmt.Sprintf("居民订单冷却至 %s", residentOrderBlockedUntil.Local().Format("15:04:05")))
	}
	if now.Before(freeWaterBlockedUntil) {
		out.BlockedReasons = append(out.BlockedReasons, fmt.Sprintf("免费水滴冷却至 %s", freeWaterBlockedUntil.Local().Format("15:04:05")))
	}
	if now.Before(dailyTaskBlockedUntil) {
		out.BlockedReasons = append(out.BlockedReasons, fmt.Sprintf("日常任务冷却至 %s", dailyTaskBlockedUntil.Local().Format("15:04:05")))
	}
	if materialBlockCount > 0 {
		out.BlockedReasons = append(out.BlockedReasons, fmt.Sprintf("材料相关冷却 %d 项", materialBlockCount))
	}
	out.UnknownNamespaceCount = r.state.UnknownNamespaceCount()
	out.ObservedNamespaces = r.state.ObservedNamespaces()
	return out
}

func (r *Runner) setNextDecisionAt(t time.Time) {
	r.mu.Lock()
	r.nextDecisionAt = t
	r.mu.Unlock()
}

func (r *Runner) beginOperation(kind string) func(error) {
	now := time.Now()
	r.mu.Lock()
	r.currentOperation = kind
	r.currentOperationStartedAt = now
	r.mu.Unlock()
	return func(err error) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.lastOperation = kind
		r.lastOperationAt = time.Now()
		r.currentOperation = ""
		r.currentOperationStartedAt = time.Time{}
		if err != nil {
			r.lastOperationError = err.Error()
			r.lastOperationErrorAt = r.lastOperationAt
		}
	}
}

func (r *Runner) recordRPCName(name string) {
	if name == "" || clientproto.IsKnownRPCName(name) {
		return
	}
	r.mu.Lock()
	if r.unknownRPCCounts == nil {
		r.unknownRPCCounts = make(map[string]int32)
	}
	r.unknownRPCCounts[name]++
	r.mu.Unlock()
}
