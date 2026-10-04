package runner

import "time"

const rpcObservationLimit = 256

type rpcObservation struct {
	name   string
	code   int
	failed bool
	at     time.Time
}

// Guarded by safetyMu. Count matched RPC responses, not planner operations:
// batches and heartbeats are included, unmatched/late responses are not.
// Never retain arguments, tokens, response bodies or account identifiers.
type rpcObservationWindow struct {
	items     []rpcObservation
	droppedAt time.Time
}

type rpcResponseCounts struct {
	Responses  int         `json:"responses"`
	Errors     int         `json:"errors"`
	ErrorCodes map[int]int `json:"error_codes,omitempty"`
}

type rpcResponseSnapshot struct {
	WindowSeconds int                          `json:"window_seconds"`
	Responses     int                          `json:"responses"`
	Limited       bool                         `json:"limited"`
	ByRPC         map[string]rpcResponseCounts `json:"by_rpc"`
}

func (w *rpcObservationWindow) record(name string, code int, failed bool, now time.Time) {
	if name == "" {
		return
	}
	kept := w.items[:0]
	for _, item := range w.items {
		if !item.at.Before(now.Add(-time.Minute)) && !item.at.After(now) {
			kept = append(kept, item)
		}
	}
	w.items = kept
	if len(w.items) == rpcObservationLimit {
		w.droppedAt = w.items[0].at
		copy(w.items, w.items[1:])
		w.items = w.items[:rpcObservationLimit-1]
	}
	w.items = append(w.items, rpcObservation{name: name, code: code, failed: failed, at: now})
}

func (w *rpcObservationWindow) snapshot(now time.Time) rpcResponseSnapshot {
	result := rpcResponseSnapshot{WindowSeconds: 60, ByRPC: make(map[string]rpcResponseCounts),
		Limited: !w.droppedAt.IsZero() && !w.droppedAt.Before(now.Add(-time.Minute)) && !w.droppedAt.After(now)}
	for _, item := range w.items {
		if item.at.Before(now.Add(-time.Minute)) || item.at.After(now) {
			continue
		}
		counts := result.ByRPC[item.name]
		counts.Responses++
		if item.failed {
			counts.Errors++
		}
		if item.code != 0 {
			if counts.ErrorCodes == nil {
				counts.ErrorCodes = make(map[int]int)
			}
			counts.ErrorCodes[item.code]++
		}
		result.Responses++
		result.ByRPC[item.name] = counts
	}
	return result
}
