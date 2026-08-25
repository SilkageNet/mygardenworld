package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

// Anti-cheat verification point types observed from captures.
const (
	rqstPointHarvest       = 2
	rqstPointPlant         = 1
	rqstPointWater         = 3
	rqstPointCustomerOrder = 4
	rqstPointFlowerOrder   = 5
	rqstPointFriendSteal   = 22
)

// sendRqstVerification sends the anti-cheat verification RPC that the game
// client sends before batch operations. The server validates the device
// fingerprint and may reject subsequent operations if this is missing.
func (r *Runner) sendRqstVerification(ctx context.Context, rpcName string, pointType int) error {
	r.mu.RLock()
	client := r.client
	session := r.session
	r.mu.RUnlock()
	if client == nil || session == nil {
		return nil
	}

	fingerprint := buildDeviceFingerprint(session.DeviceID)
	point := []any{pointType, fingerprint}
	rpc := r.runnerRPC(client, session)
	var d babigame.WSResponseD
	var err error
	switch rpcName {
	case clientproto.RPCReapPopupShjm.String():
		_, d, err = rpcResult(rpc.ReapPopup().Shjm(ctx, clientproto.ReapPopupShjmRequest{Point: point}))
	case clientproto.RPCPlantRqstZhtc.String():
		_, d, err = rpcResult(rpc.PlantRqst().Zhtc(ctx, clientproto.PlantRqstZhtcRequest{Point: point}))
	case clientproto.RPCCustomerOrderRqstDkgkck.String():
		_, d, err = rpcResult(rpc.CustomerOrderRqst().Dkgkck(ctx, clientproto.CustomerOrderRqstDkgkckRequest{Point: point}))
	case clientproto.RPCFlowerOrderRqstShowR.String():
		_, d, err = rpcResult(rpc.FlowerOrderRqst().ShowR(ctx, clientproto.FlowerOrderRqstShowRRequest{Point: point}))
	case clientproto.RPCWaterRqstDjst.String():
		_, d, err = rpcResult(rpc.WaterRqst().Djst(ctx, clientproto.WaterRqstDjstRequest{Point: point}))
	default:
		return fmt.Errorf("unknown rqst rpc %s", rpcName)
	}
	if err != nil {
		return fmt.Errorf("rqst %s: %w", rpcName, err)
	}
	if d.IsError() {
		return fmt.Errorf("rqst %s: %s", rpcName, d.ErrorMsg())
	}
	return nil
}

// sendHarvestVerification sends ReapPopup.shjm before harvest operations.
func (r *Runner) sendHarvestVerification(ctx context.Context) error {
	return r.sendRqstVerification(ctx, "ReapPopup.shjm", rqstPointHarvest)
}

// sendPlantVerification sends PlantRqst.zhtc before plant operations.
func (r *Runner) sendPlantVerification(ctx context.Context) error {
	return r.sendRqstVerification(ctx, "PlantRqst.zhtc", rqstPointPlant)
}

// sendWaterVerification sends waterRqst.djst before water operations.
func (r *Runner) sendWaterVerification(ctx context.Context) error {
	return r.sendRqstVerification(ctx, "waterRqst.djst", rqstPointWater)
}

// sendCustomerOrderVerification sends customerOrderRqst.dkgkck before customer order operations.
func (r *Runner) sendCustomerOrderVerification(ctx context.Context) error {
	return r.sendRqstVerification(ctx, "customerOrderRqst.dkgkck", rqstPointCustomerOrder)
}

// sendFlowerOrderVerification sends flowerOrderRqst.showR before flower order operations.
func (r *Runner) sendFlowerOrderVerification(ctx context.Context) error {
	return r.sendRqstVerification(ctx, "flowerOrderRqst.showR", rqstPointFlowerOrder)
}

// buildDeviceFingerprint encodes the device ID into the char-code array format
// expected by the server. The capture shows the format as a stringified array
// of character codes, e.g. "[98,100,103,54,49,50,49,96,49,101,52,49,98,51,55,53]".
func buildDeviceFingerprint(deviceID string) string {
	// Strip dashes and lowercase to get a compact hex-like fingerprint.
	fp := strings.ToLower(strings.ReplaceAll(deviceID, "-", ""))
	if len(fp) > 16 {
		fp = fp[:16]
	}
	if len(fp) == 0 {
		fp = "0000000000000000"
	}
	codes := make([]string, len(fp))
	for i, c := range fp {
		codes[i] = fmt.Sprintf("%d", c)
	}
	return "[" + strings.Join(codes, ",") + "]"
}

// sendHomeVerification sends homeRqst.showBird (home screen verification).
func (r *Runner) sendHomeVerification(ctx context.Context) error {
	r.mu.RLock()
	client := r.client
	session := r.session
	r.mu.RUnlock()
	if client == nil || session == nil {
		return nil
	}
	rpc := r.runnerRPC(client, session)
	_, d, err := rpcResult(rpc.HomeRqst().ShowBird(ctx, clientproto.HomeRqstShowBirdRequest{Time: 1}))
	if err != nil {
		return fmt.Errorf("homeRqst.showBird: %w", err)
	}
	if d.IsError() {
		return fmt.Errorf("homeRqst.showBird: %s", d.ErrorMsg())
	}
	return nil
}

// rqstState tracks whether verification has been sent for the current session.
type rqstState struct {
	harvestSent       bool
	plantSent         bool
	waterSent         bool
	customerOrderSent bool
	flowerOrderSent   bool
	homeSent          bool
}

// ensureHarvestRqst sends harvest verification once per session cycle.
func (r *Runner) ensureHarvestRqst(ctx context.Context) error {
	r.mu.RLock()
	sent := r.rqst.harvestSent
	r.mu.RUnlock()
	if sent {
		return nil
	}
	if err := r.sendHarvestVerification(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	r.rqst.harvestSent = true
	r.mu.Unlock()
	return nil
}

// ensurePlantRqst sends plant verification once per session cycle.
func (r *Runner) ensurePlantRqst(ctx context.Context) error {
	r.mu.RLock()
	sent := r.rqst.plantSent
	r.mu.RUnlock()
	if sent {
		return nil
	}
	if err := r.sendPlantVerification(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	r.rqst.plantSent = true
	r.mu.Unlock()
	return nil
}

// ensureWaterRqst sends water verification once per session cycle.
func (r *Runner) ensureWaterRqst(ctx context.Context) error {
	r.mu.RLock()
	sent := r.rqst.waterSent
	r.mu.RUnlock()
	if sent {
		return nil
	}
	if err := r.sendWaterVerification(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	r.rqst.waterSent = true
	r.mu.Unlock()
	return nil
}

// ensureCustomerOrderRqst sends customer order verification once per session cycle.
func (r *Runner) ensureCustomerOrderRqst(ctx context.Context) error {
	r.mu.RLock()
	sent := r.rqst.customerOrderSent
	r.mu.RUnlock()
	if sent {
		return nil
	}
	if err := r.sendCustomerOrderVerification(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	r.rqst.customerOrderSent = true
	r.mu.Unlock()
	return nil
}

// ensureFlowerOrderRqst sends flower order verification once per session cycle.
func (r *Runner) ensureFlowerOrderRqst(ctx context.Context) error {
	r.mu.RLock()
	sent := r.rqst.flowerOrderSent
	r.mu.RUnlock()
	if sent {
		return nil
	}
	if err := r.sendFlowerOrderVerification(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	r.rqst.flowerOrderSent = true
	r.mu.Unlock()
	return nil
}

// ensureHomeRqst sends home verification once per session cycle.
func (r *Runner) ensureHomeRqst(ctx context.Context) error {
	r.mu.RLock()
	sent := r.rqst.homeSent
	r.mu.RUnlock()
	if sent {
		return nil
	}
	if err := r.sendHomeVerification(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	r.rqst.homeSent = true
	r.mu.Unlock()
	return nil
}
