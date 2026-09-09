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
// eligible, non-cooling Side scope has waited for sideLaneMaxWait. Request
// spacing only determines send readiness; it must not erase eligibility or
// accumulated wait. Among overdue scopes, oldest wins, with planner order as
// the deterministic tie-break. This is an aging threshold, not a deadline:
// request pacing, required Farm turns and contested mutations still apply.
func (r *Runner) selectRunnableOperation(candidates []automation.PlannedOp, now time.Time) *automation.PlannedOp {
	runnable := make([]runnableOperationCandidate, 0, len(candidates))
	activeSideScopes := make(map[string]struct{})
	reserveUrgentSlot := false
	for _, candidate := range candidates {
		if !runnablePlannedOp(candidate) {
			continue
		}
		op := candidate
		if op.Kind == "fmlRace.upgradeTask" {
			r.mu.RLock()
			attempted := r.raceUpgradeAttempts[[2]int64{op.RaceBatchID, op.TaskMsID}]
			r.mu.RUnlock()
			if attempted {
				continue
			}
		}
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
		// Only genuinely eligible urgent work may reserve a send slot. A
		// longer durable deletion interval cannot reserve an earlier slot.
		deleteWait := time.Duration(0)
		if op.Kind == "fmlRace.delTask" {
			deleteWait = r.raceDeleteWait(now)
		}
		delay := r.pacer.delay(op.Kind, now)
		if automation.IsUrgentRaceOp(op) && !isYieldingRaceSync(op) && deleteWait <= delay && r.pacer.reserveUrgentSlot(op.Kind, now) {
			reserveUrgentSlot = true
		}
		if delay > 0 || deleteWait > 0 {
			continue
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
	urgent := firstUrgentRaceOp(runnable)
	if reserveUrgentSlot && (urgent == nil || isYieldingRaceSync(urgent.op)) {
		return nil
	}

	firstFarm := -1
	firstDueSide := -1
	for i := range runnable {
		entry := runnable[i]
		if entry.op.Lane == automation.LaneFarm && firstFarm < 0 {
			firstFarm = i
		}
		if entry.scope == "" {
			continue
		}
		firstWait := r.sideLaneFirstWait[entry.scope]
		if !firstWait.IsZero() && !now.Before(firstWait.Add(sideLaneMaxWait)) &&
			(firstDueSide < 0 || firstWait.Before(r.sideLaneFirstWait[runnable[firstDueSide].scope])) {
			firstDueSide = i
		}
	}

	// Contested race mutations must not wait for farm-first fairness. Read-only
	// race syncs yield both to overdue Side work and to the required Farm turn;
	// repeated reads must not monopolize execution in either lane.
	if urgent != nil && (!isYieldingRaceSync(urgent.op) || firstDueSide < 0) {
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
		r.raceSyncNeedsFarmTurn = false
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
	var sync *runnableOperationCandidate
	for i := range runnable {
		if automation.IsUrgentRaceOp(runnable[i].op) {
			if !isYieldingRaceSync(runnable[i].op) {
				return &runnable[i]
			}
			if sync == nil {
				sync = &runnable[i]
			}
		}
	}
	return sync
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
	r.lastSchedulerWaitDiagnostic = time.Time{}
	r.sideLaneFarmTurn = false
	r.raceSyncNeedsFarmTurn = false
}
