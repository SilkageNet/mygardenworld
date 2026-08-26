package runner

import (
	"context"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func (r *Runner) tickWaterSourceSync(ctx context.Context, client *babigame.Client, session *babigame.Session) {
	now := time.Now()
	if !r.state.WaterwheelEnterDue(now) {
		return
	}
	if !r.lastWaterSyncTick.IsZero() && now.Sub(r.lastWaterSyncTick) < waterSourceSyncPeriod {
		return
	}

	rpc := r.runnerRPC(client, session)
	_, d, err := rpcResult(rpc.Waterwheel().Enter(ctx, clientproto.WaterwheelEnterRequest{}))
	if r.isSessionInvalidated() {
		return
	}
	if err != nil {
		r.log.Debug("waterwheel sync failed", "err", err)
		return
	}
	if d.IsError() {
		return
	}
	r.lastWaterSyncTick = now
	// CooldownReady requires wwObserved. If enter returned an empty delta and
	// login never pushed namespace 114, marking entered would permanently block
	// both enter retries and claims.
	if !r.state.WaterwheelObserved() {
		r.log.Debug("waterwheel enter succeeded without namespace 114; deferring local bucket lifecycle")
		return
	}
	r.state.MarkWaterwheelEntered(now)
}
