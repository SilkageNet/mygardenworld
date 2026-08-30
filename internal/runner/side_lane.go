package runner

import (
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
)

const sideLaneMaxWait = 20 * time.Second

type runnableOperationCandidate struct {
	op    automation.PlannedOp
	scope string
}

// selectRunnableOperation preserves the planner's Farm-first order until a
// runnable, non-cooling Side scope has waited for sideLaneMaxWait. The input is
// already deterministically sorted by automation.PlanOperations, so the first
// due Side below is also the highest-ranked due Side.
func (r *Runner) selectRunnableOperation(candidates []automation.PlannedOp, now time.Time) *automation.PlannedOp {
	runnable := make([]runnableOperationCandidate, 0, len(candidates))
	activeSideScopes := make(map[string]struct{})
	for _, candidate := range candidates {
		if !runnablePlannedOp(candidate) {
			continue
		}
		op := candidate
		if r.cultivateUpgradeResourceRejectedUnchanged(&op) {
			continue
		}
		if _, cooling := r.operationCoolingDown(&op, now); cooling {
			continue
		}
		filtered := r.applyHarvestBlocks(&op, now)
		if filtered == nil {
			continue
		}
		entry := runnableOperationCandidate{op: *filtered}
		if isSideLane(entry.op) {
			entry.scope = sideLaneWaitScope(&entry.op)
			if entry.scope != "" {
				activeSideScopes[entry.scope] = struct{}{}
			}
		}
		runnable = append(runnable, entry)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sideLaneFirstWait == nil {
		r.sideLaneFirstWait = make(map[string]time.Time)
	}
	for scope := range r.sideLaneFirstWait {
		if _, active := activeSideScopes[scope]; !active {
			delete(r.sideLaneFirstWait, scope)
		}
	}
	for scope := range activeSideScopes {
		firstWait, tracked := r.sideLaneFirstWait[scope]
		if !tracked || firstWait.After(now) {
			r.sideLaneFirstWait[scope] = now
		}
	}
	if len(activeSideScopes) == 0 {
		r.sideLaneFarmTurn = false
		r.raceSyncNeedsFarmTurn = false
	}

	firstFarm := -1
	firstDueSide := -1
	for i := range runnable {
		entry := runnable[i]
		if entry.op.Lane == automation.LaneFarm && firstFarm < 0 {
			firstFarm = i
		}
		if firstDueSide >= 0 || entry.scope == "" {
			continue
		}
		firstWait := r.sideLaneFirstWait[entry.scope]
		if !firstWait.IsZero() && !now.Before(firstWait.Add(sideLaneMaxWait)) {
			firstDueSide = i
		}
	}

	// Contested race mutations must not wait for farm-first fairness. Read-only
	// race syncs may preempt once, but a still-runnable duplicate yields one tick
	// to Farm so a future state bug cannot stop every business operation.
	if urgent := firstUrgentRaceOp(runnable); urgent != nil {
		if isYieldingRaceSync(urgent.op) && r.raceSyncNeedsFarmTurn && firstFarm >= 0 {
			r.raceSyncNeedsFarmTurn = false
			r.sideLaneFarmTurn = false
			selected := runnable[firstFarm].op
			return &selected
		}
		if urgent.scope != "" {
			delete(r.sideLaneFirstWait, urgent.scope)
		}
		r.raceSyncNeedsFarmTurn = isYieldingRaceSync(urgent.op)
		r.sideLaneFarmTurn = true
		selected := urgent.op
		return &selected
	}

	// A forced Side is followed by one Farm operation whenever Farm work is
	// currently available. With no Farm candidate, due Side scopes may continue.
	if r.sideLaneFarmTurn && firstFarm >= 0 {
		r.sideLaneFarmTurn = false
		r.raceSyncNeedsFarmTurn = false
		selected := runnable[firstFarm].op
		return &selected
	}
	if firstDueSide >= 0 {
		selected := runnable[firstDueSide]
		delete(r.sideLaneFirstWait, selected.scope)
		r.sideLaneFarmTurn = true
		return &selected.op
	}
	if len(runnable) == 0 {
		return nil
	}

	selected := runnable[0]
	r.raceSyncNeedsFarmTurn = false
	if selected.op.Lane == automation.LaneFarm {
		r.sideLaneFarmTurn = false
	} else if selected.scope != "" {
		// The scope received an ordinary execution opportunity because no
		// higher-ranked Farm work blocked it; any future wait starts afresh.
		delete(r.sideLaneFirstWait, selected.scope)
	}
	return &selected.op
}

func firstUrgentRaceOp(runnable []runnableOperationCandidate) *runnableOperationCandidate {
	for i := range runnable {
		if automation.IsUrgentRaceOp(runnable[i].op) {
			return &runnable[i]
		}
	}
	return nil
}

func isYieldingRaceSync(op automation.PlannedOp) bool {
	return op.PreemptFarm && op.Action == "sync"
}

func isSideLane(op automation.PlannedOp) bool {
	return op.Lane == automation.LaneSide || op.Lane == ""
}

func sideLaneWaitScope(op *automation.PlannedOp) string {
	if op == nil {
		return ""
	}
	if scope := strings.TrimSpace(op.CooldownKey); scope != "" {
		return scope
	}
	return strings.TrimSpace(op.OperationID)
}

func (r *Runner) resetSideLaneFairness() {
	r.mu.Lock()
	r.resetSideLaneFairnessLocked()
	r.mu.Unlock()
}

func (r *Runner) resetSideLaneFairnessLocked() {
	clear(r.sideLaneFirstWait)
	r.sideLaneFarmTurn = false
	r.raceSyncNeedsFarmTurn = false
}
