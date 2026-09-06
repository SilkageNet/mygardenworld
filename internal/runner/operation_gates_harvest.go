package runner

import (
	"errors"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

type harvestFailureKey struct {
	Kind   string
	LandID int32
}

type harvestFailure struct {
	Attempts int
	Until    time.Time
}

// Unknown harvest failures also need backoff: Farm operations intentionally
// bypass the ordinary Side cooldown. Keep rejection history scoped to a land
// and RPC, so a failed garden land cannot hold up other lands or guild work.
func (r *Runner) deferFailedHarvest(op *automation.PlannedOp, err error, now time.Time) time.Duration {
	ids := op.LandIDs
	var landErr *harvestLandError
	if errors.As(err, &landErr) {
		ids = []int32{landErr.LandID}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.harvestFailures == nil {
		r.harvestFailures = make(map[harvestFailureKey]harvestFailure)
	}
	wait := harvestRetryWait
	for _, id := range ids {
		key := harvestFailureKey{op.Kind, id}
		failure := r.harvestFailures[key]
		failure.Attempts = min(failure.Attempts+1, 5)
		delay := min(harvestRetryWait*time.Duration(1<<(failure.Attempts-1)), 5*time.Minute)
		failure.Until = now.Add(delay)
		r.harvestFailures[key] = failure
		wait = max(wait, delay)
	}
	return wait
}

func (r *Runner) applyHarvestBlocks(op *automation.PlannedOp, now time.Time) *automation.PlannedOp {
	if op == nil || !isHarvestOp(op.Kind) {
		return op
	}
	if len(op.LandIDs) == 0 {
		return op
	}
	blocked := make(map[int32]bool, len(op.LandIDs))
	anyBlocked := false
	r.mu.RLock()
	for _, id := range op.LandIDs {
		until := r.harvestBlockedUntil[id]
		if rejected := r.harvestFailures[harvestFailureKey{op.Kind, id}].Until; rejected.After(until) {
			until = rejected
		}
		if until.IsZero() || !now.Before(until) {
			continue
		}
		blocked[id] = true
		anyBlocked = true
	}
	r.mu.RUnlock()
	if !anyBlocked {
		return op
	}
	landIDs := make([]int32, 0, len(op.LandIDs)-len(blocked))
	for _, id := range op.LandIDs {
		if !blocked[id] {
			landIDs = append(landIDs, id)
		}
	}
	if len(landIDs) == 0 {
		return nil
	}
	cp := *op
	cp.LandIDs = landIDs
	cp.Count = int32(len(landIDs))
	return &cp
}

func isWaterOp(kind string) bool {
	return kind == clientproto.RPCUsrLandWater.String() ||
		kind == clientproto.RPCUsrLandWaterBatch.String()
}

func (r *Runner) setHarvestBlockedUntil(landIDs []int32, until time.Time) {
	if len(landIDs) == 0 {
		return
	}
	r.mu.Lock()
	if r.harvestBlockedUntil == nil {
		r.harvestBlockedUntil = make(map[int32]time.Time)
	}
	for _, id := range landIDs {
		r.harvestBlockedUntil[id] = until
	}
	r.mu.Unlock()
}

func isHarvestOp(kind string) bool {
	return kind == clientproto.RPCUsrLandHarvest.String() ||
		kind == clientproto.RPCFmlLandHarvest.String() ||
		kind == clientproto.RPCFmlLandHarvestAll.String()
}
