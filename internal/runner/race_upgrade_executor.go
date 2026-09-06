package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SilkageNet/mygardenworld/internal/automation"
)

type raceUpgradeExecution struct {
	preflight func(context.Context) error
	reserve   func() bool
	upgrade   func(context.Context) (json.RawMessage, error)
	confirm   func(context.Context) (bool, error)
	markStale func()
}

// Keep the attempted batch/task across reconnects of this runner. A timeout
// does not prove the server rejected a paid request and must never unlock it.
func (r *Runner) reserveRaceUpgrade(op *automation.PlannedOp) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := [2]int64{op.RaceBatchID, op.TaskMsID}
	if r.raceUpgradeAttempts[key] {
		return false
	}
	if r.raceUpgradeAttempts == nil {
		r.raceUpgradeAttempts = make(map[[2]int64]bool)
	}
	r.raceUpgradeAttempts[key] = true
	return true
}

func executeRaceUpgrade(ctx context.Context, exec raceUpgradeExecution) (json.RawMessage, error) {
	if err := exec.preflight(ctx); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !exec.reserve() {
		return nil, fmt.Errorf("此任务已提交过升级，不重复消费元宝")
	}
	raw, err := exec.upgrade(ctx)
	if err != nil {
		exec.markStale()
		return raw, fmt.Errorf("升级结果未确认，当前会话对此任务不再自动重试以避免重复扣费: %w", err)
	}
	confirmed, err := exec.confirm(ctx)
	if err != nil {
		exec.markStale()
		return raw, fmt.Errorf("升级已提交但状态同步失败，当前会话对此任务不再自动重试: %w", err)
	}
	if !confirmed {
		return raw, fmt.Errorf("升级已提交但尚未观测到升级状态，当前会话对此任务不再自动重试")
	}
	return raw, nil
}
