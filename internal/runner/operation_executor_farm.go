package runner

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
)

func runUsrLandHarvest(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	reqs, err := harvestRequests(op)
	if err != nil {
		return nil, err
	}
	results := make([]harvestCallResult, 0, len(reqs))
	for i, req := range reqs {
		raw, err := checkedStateDelta(rt.rpc.UsrLand().Harvest(ctx, req))
		if err != nil {
			// Reconcile through the existing session after a rejection. 97777 has
			// no observed public meaning; never reinterpret it as success or
			// force a fresh login. The error handler still backs off this land.
			if ctx.Err() == nil && rt.runner != nil {
				rt.runner.mu.RLock()
				client := rt.runner.client
				invalid := rt.runner.sessionInvalidated
				rt.runner.mu.RUnlock()
				if client != nil && !invalid {
					syncCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
					if v, syncErr := client.LazySync(syncCtx); syncErr == nil {
						rt.runner.state.ApplyV(v)
					}
					cancel()
				}
			}
			partial, _ := json.Marshal(results)
			return partial, &harvestLandError{LandID: req.LandId, Err: err}
		}
		if rt.runner != nil {
			rt.runner.mu.Lock()
			delete(rt.runner.harvestFailures, harvestFailureKey{op.Kind, req.LandId})
			rt.runner.mu.Unlock()
		}
		results = append(results, harvestCallResult{LandID: req.LandId, Raw: raw})
		if i == len(reqs)-1 || harvestRPCInterval <= 0 {
			continue
		}
		timer := time.NewTimer(harvestRPCInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	raw, err := json.Marshal(results)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
