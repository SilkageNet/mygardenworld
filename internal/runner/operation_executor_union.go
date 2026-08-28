package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func fmlForestRefreshRequest(op *automation.PlannedOp) clientproto.FmlForestRefreshRequest {
	return clientproto.FmlForestRefreshRequest{IsAutoCollect: op.TargetID != 0}
}

func runFmlForestRefresh(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	if rt.runner == nil || rt.runner.state == nil {
		return nil, fmt.Errorf("fmlForest.refresh requires runner state")
	}
	v, d, err := rpcResult(rt.rpc.FmlForest().Refresh(
		ctx,
		fmlForestRefreshRequest(op),
		babigame.WithPayloadApply(false),
	))
	v, err = checkedPayload(v, d, err)
	if err == nil && babigame.HasPayload(v) {
		v = normalizeFmlForestRefreshV(v)
		rt.runner.state.ApplyV(v)
	}
	// Record empty acknowledgements and errors as well. The runner already
	// reports the error; this state timestamp prevents an immediate retry loop.
	rt.runner.state.MarkFmlForestRefreshAttempt()
	return v, err
}

// normalizeFmlForestRefreshV accepts all observed response shapes:
// a normal top-level state delta, a bare IFmlTot containing field 127, or the
// forest-energy object itself. Unknown top-level state remains untouched.
func normalizeFmlForestRefreshV(v json.RawMessage) json.RawMessage {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(v, &top); err != nil || len(top) == 0 {
		return v
	}
	if _, ok := top["25"]; ok {
		return v
	}
	if _, ok := top["127"]; ok {
		return wrapFmlNamespace(v)
	}
	if !looksLikeFmlForestEnergy(top) {
		return v
	}
	energy, err := json.Marshal(map[string]json.RawMessage{"127": v})
	if err != nil {
		return v
	}
	return wrapFmlNamespace(energy)
}

func looksLikeFmlForestEnergy(fields map[string]json.RawMessage) bool {
	for _, key := range []string{"2", "6", "8"} {
		if _, ok := fields[key]; ok {
			return true
		}
	}
	return false
}

func wrapFmlNamespace(v json.RawMessage) json.RawMessage {
	wrapped, err := json.Marshal(map[string]json.RawMessage{"25": v})
	if err != nil {
		return v
	}
	return wrapped
}
