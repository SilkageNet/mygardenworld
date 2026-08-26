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
			return nil, &harvestLandError{LandID: req.LandId, Err: err}
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
