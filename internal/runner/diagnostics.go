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
