package runner

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
)

const (
	pearlDiagnosticInterval = 30 * time.Second
	pearlDiagnosticRepeat   = 5 * time.Minute
)

type pearlDiagnosticLogState struct {
	PolicyKey   string
	EvaluatedAt time.Time
	EmittedAt   time.Time
	Payload     string
}

// Record why no hire was sent as well as why one is eligible. This uses only
// the pure planner and cached runner state, never an extra connection or RPC.
// Policy/session changes bypass the sampling interval; ordinary candidate
// churn is sampled at most every 30s and identical active states every 5m.
func (r *Runner) emitPearlHireDiagnostic(snapshot tickSnapshot, now time.Time) {
	if r.state == nil || r.account == nil || r.log == nil || snapshot.policy == nil {
		return
	}
	policy := snapshot.policy
	p := policy.GetBasic().GetPearl()
	disconnected := snapshot.sessionInvalidated || snapshot.client == nil || snapshot.session == nil
	key := fmt.Sprintf("%t:%t:%d:%d:%d:%t", policy.GetAutomationEnabled(), p.GetAutoHireEnabled(),
		p.GetMaxHireLevel(), p.GetMaxHireTicketUsage(), p.GetDailyHireTicketLimit(), disconnected)
	r.mu.Lock()
	previous := r.pearlDiagnosticLog
	if key == previous.PolicyKey && !now.Before(previous.EvaluatedAt) && now.Sub(previous.EvaluatedAt) < pearlDiagnosticInterval {
		r.mu.Unlock()
		return
	}
	r.pearlDiagnosticLog.PolicyKey = key
	r.pearlDiagnosticLog.EvaluatedAt = now
	r.mu.Unlock()

	diagnostic := automation.DiagnosePearlHire(r.state, policy, now)
	if disconnected && diagnostic.AutoHireEnabled && diagnostic.AutomationEnabled {
		diagnostic.Status, diagnostic.Reason = "disconnected", "游戏会话不可用，不执行雇佣"
		diagnostic.NextOperation = ""
		diagnostic.TargetUID, diagnostic.PlaceID = 0, 0
		// Cached eligibility is not current execution evidence while offline.
		diagnostic.Sources = nil
	}
	raw, _ := json.Marshal(diagnostic)
	payload := string(raw)
	inactive := diagnostic.Status == "disabled" || diagnostic.Status == "paused" || diagnostic.Status == "disconnected"
	if payload == previous.Payload && (inactive || (!now.Before(previous.EmittedAt) && now.Sub(previous.EmittedAt) < pearlDiagnosticRepeat)) {
		return
	}
	r.mu.Lock()
	r.pearlDiagnosticLog.Payload = payload
	r.pearlDiagnosticLog.EmittedAt = now
	r.mu.Unlock()
	r.emit(Event{
		Kind: "pearl_hire_diagnostic", Category: "basic", Domain: "basic.pearl.hire",
		Action: "diagnostic", Label: "雇佣筛选诊断", Level: "info",
		Message: diagnostic.Summary(), PayloadJSON: payload, TS: now,
	})
}
