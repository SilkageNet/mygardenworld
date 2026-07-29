package runner

import (
	"context"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

const residentOrderSyncPeriod = 2 * time.Minute

// syncResidentOrderState refreshes namespace 105 (including satin/decorate) via
// orderFlower.enter so daily counters and isVideo flags match the server.
func (r *Runner) syncResidentOrderState(ctx context.Context, client *babigame.Client, session *babigame.Session, reason string) {
	if client == nil || session == nil || r.state == nil {
		return
	}
	rpc := r.runnerRPC(client, session)
	_, d, err := rpcResult(rpc.OrderFlower().Enter(ctx, clientproto.OrderFlowerEnterRequest{}))
	if r.isSessionInvalidated() {
		return
	}
	if err != nil {
		r.log.Debug("resident order sync failed", "reason", reason, "err", err)
		return
	}
	if d.IsError() {
		r.log.Debug("resident order sync rejected", "reason", reason, "msg", d.ErrorMsg())
		return
	}
	r.mu.Lock()
	r.lastResidentOrderSyncTick = time.Now()
	r.mu.Unlock()
	// Fresh state may clear an ad pause the user already resolved in-client.
	r.clearResidentSpecialOrderCooldowns()
}

func (r *Runner) tickResidentOrderSync(ctx context.Context, client *babigame.Client, session *babigame.Session, policy *pb.Policy) {
	if policy == nil || !policy.GetAutomationEnabled() {
		return
	}
	resident := policy.GetOrder().GetResident()
	satinOn := resident.GetSatinEnabled()
	decorateOn := resident.GetDecorateEnabled()
	if !satinOn && !decorateOn {
		return
	}

	now := time.Now()
	needSync := false
	if satinOn {
		if _, limited := r.state.ResidentSatinDailyLimitReached(now); !limited {
			satin := r.state.ResidentSatinOrder()
			if !satin.Observed || satin.IsVideo != 0 {
				needSync = true
			}
		}
	}
	if decorateOn {
		if _, limited := r.state.ResidentDecorateDailyLimitReached(now); !limited {
			decorate := r.state.ResidentDecorateOrder()
			if !decorate.Observed || decorate.IsVideo != 0 {
				needSync = true
			}
		}
	}
	if !needSync {
		return
	}

	r.mu.RLock()
	last := r.lastResidentOrderSyncTick
	r.mu.RUnlock()
	if !last.IsZero() && now.Sub(last) < residentOrderSyncPeriod {
		return
	}
	r.syncResidentOrderState(ctx, client, session, "ad_or_unobserved")
}

func (r *Runner) resetResidentOrderSession() {
	r.clearResidentSpecialOrderCooldowns()
	r.mu.Lock()
	r.lastResidentOrderSyncTick = time.Time{}
	r.mu.Unlock()
}

func (r *Runner) clearResidentSpecialOrderCooldowns() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.operationCooldowns == nil {
		return
	}
	for key := range r.operationCooldowns {
		if isResidentSpecialOrderCooldownKey(key) {
			delete(r.operationCooldowns, key)
		}
	}
}

// clearResidentSpecialOrderRetryTimers drops short side-op cooldowns for one
// satin/decorate finish kind after the server daily cap is hit, so decision
// ticks do not keep a retry countdown.
func (r *Runner) clearResidentSpecialOrderRetryTimers(kind string) {
	if kind == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.operationCooldowns == nil {
		return
	}
	for key := range r.operationCooldowns {
		if strings.HasPrefix(key, kind) ||
			(kind == clientproto.RPCOrderFlowerFinishSatinOrder.String() && strings.Contains(key, "order.resident.satin")) ||
			(kind == clientproto.RPCOrderFlowerFinishDecorateOrder.String() && strings.Contains(key, "order.resident.decorate")) {
			delete(r.operationCooldowns, key)
		}
	}
}

func isResidentSpecialOrderCooldownKey(key string) bool {
	switch {
	case strings.HasPrefix(key, clientproto.RPCOrderFlowerFinishSatinOrder.String()):
		return true
	case strings.HasPrefix(key, clientproto.RPCOrderFlowerFinishDecorateOrder.String()):
		return true
	case strings.Contains(key, "order.resident.satin"):
		return true
	case strings.Contains(key, "order.resident.decorate"):
		return true
	default:
		return false
	}
}
