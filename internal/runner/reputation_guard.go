package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
)

const reputationSyncPeriod = 5 * time.Minute

type reputationGuardError struct {
	Score     int32
	Threshold int32
}

func (e reputationGuardError) Error() string {
	return fmt.Sprintf("健康分 %d 低于阈值 %d，已停止自动化和登录", e.Score, e.Threshold)
}

func isReputationGuardError(err error) bool {
	var guardErr reputationGuardError
	return errors.As(err, &guardErr)
}

func (r *Runner) enforceReputationGuard(ctx context.Context, client *babigame.Client, session *babigame.Session, stage string, now time.Time) error {
	enabled, threshold := reputationGuardConfig(r.Policy())
	if err := r.refreshReputationIfDue(ctx, client, session, now); err != nil {
		if enabled && !r.reputationObserved() {
			return fmt.Errorf("获取健康分失败: %w", err)
		}
		r.emitReputationCheckFailed(err, stage)
	}
	if !enabled {
		return nil
	}
	rep, ok := r.state.Reputation()
	if !ok {
		return fmt.Errorf("尚未观测到健康分")
	}
	if rep.Score >= threshold {
		return nil
	}
	r.disableAutomationForReputation(rep.Score, threshold, stage)
	return reputationGuardError{Score: rep.Score, Threshold: threshold}
}

func reputationGuardConfig(policy *pb.Policy) (bool, int32) {
	rep := policy.GetBasic().GetReputation()
	if rep == nil || !rep.GetEnabled() {
		return false, 0
	}
	threshold := rep.GetThreshold()
	if threshold <= 0 {
		threshold = 80
	}
	return true, threshold
}

func (r *Runner) reputationObserved() bool {
	_, ok := r.state.Reputation()
	return ok
}

func (r *Runner) refreshReputationIfDue(ctx context.Context, client *babigame.Client, session *babigame.Session, now time.Time) error {
	if client == nil || session == nil {
		return nil
	}
	if _, ok := r.state.Reputation(); ok {
		r.mu.RLock()
		last := r.lastReputationSyncTick
		r.mu.RUnlock()
		if !last.IsZero() && now.Sub(last) < reputationSyncPeriod {
			return nil
		}
	}
	r.mu.Lock()
	if !r.lastReputationSyncTick.IsZero() && now.Sub(r.lastReputationSyncTick) < reputationSyncPeriod {
		r.mu.Unlock()
		return nil
	}
	r.lastReputationSyncTick = now
	r.mu.Unlock()

	rpc := r.runnerRPC(client, session)
	v, d, err := rpcResult(rpc.Reputation().View(ctx, clientproto.ReputationViewRequest{}))
	if r.isSessionInvalidated() {
		return r.sessionInvalidatedError("session invalidated while checking reputation")
	}
	if err != nil {
		return err
	}
	if d.IsError() {
		msg := d.ErrorMsg()
		if msg == "" {
			msg = "server returned error"
		}
		return fmt.Errorf("%s", msg)
	}
	if babigame.HasPayload(v) {
		r.state.ApplyV(v)
	}
	return nil
}

func (r *Runner) disableAutomationForReputation(score, threshold int32, stage string) {
	p := r.Policy()
	if p.GetAutomationEnabled() {
		p.AutomationEnabled = false
		r.SetPolicy(p)
		if r.db != nil {
			_ = r.db.DeleteSession(context.Background(), r.account.ID)
			if raw, err := policycfg.ToJSON(p); err == nil {
				_ = r.db.SavePolicyJSON(context.Background(), r.account.ID, raw)
			}
		}
	} else if r.db != nil {
		_ = r.db.DeleteSession(context.Background(), r.account.ID)
	}
	payload, _ := json.Marshal(map[string]any{
		"score":              score,
		"threshold":          threshold,
		"stage":              stage,
		"automation_enabled": false,
	})
	r.emit(Event{
		Kind:        "reputation_guard",
		Category:    "account",
		Domain:      "basic.reputation",
		Action:      "blocked",
		Label:       "礼仪分监控",
		Level:       "error",
		Message:     fmt.Sprintf("健康分 %d 低于阈值 %d，已停止自动化并断开登录", score, threshold),
		PayloadJSON: string(payload),
	})
	if r.db != nil {
		_ = r.db.LogOperation(context.Background(), r.account.ID, "reputation.guard",
			map[string]any{"stage": stage},
			map[string]any{"score": score, "threshold": threshold, "automation_enabled": false},
		)
	}
}

func (r *Runner) emitReputationCheckFailed(err error, stage string) {
	payload, _ := json.Marshal(map[string]any{"stage": stage, "error": err.Error()})
	r.emit(Event{
		Kind:        "reputation_guard",
		Category:    "account",
		Domain:      "basic.reputation",
		Action:      "check",
		Label:       "礼仪分监控",
		Level:       "warn",
		Message:     fmt.Sprintf("健康分检查失败: %v", err),
		PayloadJSON: string(payload),
	})
}
