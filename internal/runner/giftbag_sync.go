package runner

import (
	"context"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

// syncGiftbagState performs one bounded read-side refresh per connection. It
// is independent of automation policy because the Web status is a manual
// reminder; no gift is purchased and no advertising callback is fabricated.
func (r *Runner) syncGiftbagState(ctx context.Context, client *babigame.Client, session *babigame.Session) {
	if client == nil || session == nil || r.state == nil {
		return
	}
	rpc := r.runnerRPC(client, session)
	_, d, err := rpcResult(rpc.ShopGiftbag().Enter(ctx, clientproto.ShopGiftbagEnterRequest{}))
	if r.isSessionInvalidated() {
		return
	}
	if err != nil {
		r.log.Debug("giftbag status sync failed", "err", err)
		return
	}
	if d.IsError() {
		r.log.Debug("giftbag status sync rejected", "msg", d.ErrorMsg())
	}
}
