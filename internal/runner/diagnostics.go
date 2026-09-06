package runner

import "time"

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
	SessionInvalidatedReason  string
	BlockedReasons            []string
	UnknownRPCCount           int32
	UnknownNamespaceCount     int32
	ObservedNamespaces        []string
	OperationCooldowns        []OperationCooldownSnapshot
}

type OperationCooldownSnapshot struct {
	OperationID  string
	Category     string
	Domain       string
	Lane         string
	Reason       string
	Until        time.Time
	FailureCount int32
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
	sessionInvalidated := r.sessionInvalidated
	connected := r.client != nil && !r.client.Closed()
	r.mu.RUnlock()

	if sessionInvalidated {
		out.BlockedReasons = append(out.BlockedReasons, "会话已失效")
	}
	if !connected && !sessionInvalidated {
		out.BlockedReasons = append(out.BlockedReasons, "WebSocket 未连接")
	}
	out.UnknownNamespaceCount = r.state.UnknownNamespaceCount()
	out.ObservedNamespaces = r.state.ObservedNamespaces()
	out.OperationCooldowns = r.operationCooldownSnapshots(now)
	if err := r.restrictionError(); err != nil {
		out.BlockedReasons = append(out.BlockedReasons, err.Error())
		s, _ := r.accountSafetySnapshot()
		out.OperationCooldowns = append(out.OperationCooldowns, OperationCooldownSnapshot{
			OperationID: "account.request", Category: "account", Domain: "account.request",
			Reason: err.Error(), Until: time.UnixMilli(s.RestrictedUntilMS), FailureCount: int32(s.RestrictionAttempts),
		})
	}
	if wait := r.raceDeleteWait(now); wait > 0 {
		out.OperationCooldowns = append(out.OperationCooldowns, OperationCooldownSnapshot{
			OperationID: "union.race.delete.interval", Category: "race", Domain: "union.race.delete",
			Reason: "账号级竞赛删除间隔（自动与手动共用）", Until: now.Add(wait),
		})
	}
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
			return
		}
		r.lastOperationError = ""
		r.lastOperationErrorAt = time.Time{}
	}
}
