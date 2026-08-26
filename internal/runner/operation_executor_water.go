package runner

import (
	"context"
	"encoding/json"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func runWaterwheelRecv(ctx context.Context, rt operationRuntime, _ *automation.PlannedOp) (json.RawMessage, error) {
	if rt.runner != nil && rt.runner.state.WaterwheelNextClaimRequiresSkip() {
		if v, d, err := rpcResult(rt.rpc.Waterwheel().Skip(ctx, clientproto.WaterwheelSkipRequest{})); err != nil || d.IsError() {
			return checkedPayload(v, d, err)
		}
	}
	return checkedStateDelta(rt.rpc.Waterwheel().Recv(ctx, clientproto.WaterwheelRecvRequest{}))
}
