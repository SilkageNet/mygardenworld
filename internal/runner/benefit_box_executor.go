package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

// benefitBoxDrawExecution drains unopened benefit boxes the same way the mini
// client does: read getBenefitBoxInfo() (local accrual over namespace 116), then
// call gs.benefitBox.draw once per remaining box.
type benefitBoxDrawExecution struct {
	remaining func(time.Time) int32
	observed  func() bool
	draw      func(context.Context) (json.RawMessage, error)
	apply     func(json.RawMessage)
	now       func() time.Time
}

type benefitBoxDrawResult struct {
	Opened int32             `json:"opened"`
	Last   json.RawMessage   `json:"last,omitempty"`
	Draws  []json.RawMessage `json:"draws,omitempty"`
}

func runBenefitBoxDraw(ctx context.Context, rt operationRuntime, _ *automation.PlannedOp) (json.RawMessage, error) {
	if rt.runner == nil || rt.runner.state == nil || rt.rpc == nil {
		return nil, fmt.Errorf("benefitBox.draw runner state or RPC unavailable")
	}
	exec := benefitBoxDrawExecution{
		remaining: rt.runner.state.BenefitBoxDrawsRemaining,
		observed:  rt.runner.state.BenefitBoxObserved,
		draw: func(ctx context.Context) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.BenefitBox().Draw(ctx, clientproto.BenefitBoxDrawRequest{}, babigame.WithPayloadApply(false)))
		},
		apply: rt.runner.state.ApplyV,
		now:   time.Now,
	}
	return executeBenefitBoxDraw(ctx, exec)
}

func executeBenefitBoxDraw(ctx context.Context, exec benefitBoxDrawExecution) (json.RawMessage, error) {
	if exec.remaining == nil || exec.observed == nil || exec.draw == nil || exec.apply == nil || exec.now == nil {
		return nil, fmt.Errorf("benefitBox.draw execution is incomplete")
	}
	now := exec.now()
	want := exec.remaining(now)
	if want <= 0 {
		if exec.observed() {
			return nil, fmt.Errorf("benefitBox: getBenefitBoxInfo reports 0 unopened boxes")
		}
		// Namespace 116 never synced; one draw may return state or a clear error.
		want = 1
	}
	const maxDraws = 8 // c_benefitBox.$boxMax
	if want > maxDraws {
		want = maxDraws
	}

	result := benefitBoxDrawResult{Draws: make([]json.RawMessage, 0, want)}
	for opened := int32(0); opened < want; opened++ {
		if opened > 0 {
			left := exec.remaining(exec.now())
			if left <= 0 {
				break
			}
		}
		raw, err := exec.draw(ctx)
		if err != nil {
			if result.Opened > 0 {
				out, _ := json.Marshal(result)
				return out, fmt.Errorf("benefitBox.draw opened %d then failed: %w", result.Opened, err)
			}
			return nil, err
		}
		if babigame.HasPayload(raw) {
			exec.apply(raw)
		}
		result.Opened++
		result.Last = raw
		result.Draws = append(result.Draws, raw)
	}
	if result.Opened <= 0 {
		return nil, fmt.Errorf("benefitBox.draw opened 0 boxes")
	}
	out, err := json.Marshal(result)
	if err != nil {
		return result.Last, nil
	}
	return out, nil
}
