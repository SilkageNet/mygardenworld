package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const freshRecoveryInterval = 30 * time.Minute

type recoveryProbeKey struct{}
type recoveryProbePermit struct {
	runner   *Runner
	revision uint64
}

// Routing is independent of admission: exhaustion must never fall back to cache.
func (r *Runner) prefersFreshRecovery() bool {
	p := r.Policy()
	s, _ := r.accountSafetySnapshot()
	return s.RestrictionCode != 0 && p.GetAutomationEnabled() && p.GetBasic().GetServerErrorFreshLoginEnabled()
}

// Reserve at the last boundary before channel authentication. Never let cache
// fallback silently turn a protected reconnect into an unlimited fresh login.
func (r *Runner) reserveFreshRecovery(ctx context.Context, now time.Time) error {
	if err := r.checkFreshRecoveryAuthorization(ctx); err != nil {
		return err
	}
	p := r.Policy()
	r.safetyMu.Lock()
	defer r.safetyMu.Unlock()
	manual := r.manualRecoveryAuthorized(ctx)
	if r.safety.RestrictionCode == 0 {
		return nil
	}
	if !p.GetAutomationEnabled() {
		return &recoveryBlockedError{reason: "自动化已暂停，等待手动启动；未发送恢复认证，未延长服务端冷却"}
	}
	maxAttempts := int(p.GetBasic().GetServerErrorFreshLoginMaxAttempts())
	if !manual && r.safety.FreshLoginAttempts >= maxAttempts {
		return &recoveryBlockedError{reason: fmt.Sprintf("异常恢复已尝试 %d/%d 次，达到自动重新认证上限；等待手动处理，未延长服务端冷却", r.safety.FreshLoginAttempts, maxAttempts)}
	}
	if until := time.UnixMilli(r.safety.LastFreshLoginMS).Add(freshRecoveryInterval); r.safety.LastFreshLoginMS != 0 && now.Before(until) {
		return &recoveryBlockedError{reason: fmt.Sprintf("异常恢复已尝试 %d/%d 次；重新认证间隔未到，%s 后尝试；未延长服务端冷却", r.safety.FreshLoginAttempts, maxAttempts, until.Local().Format("15:04:05")), retryAt: until}
	}
	if r.db == nil {
		return fmt.Errorf("无法持久化恢复认证额度，未发送登录请求")
	}
	var allowed bool
	var err error
	if manual {
		allowed, err = r.db.ReserveManualRecovery(ctx, r.account.ID, now.UnixMilli(), freshRecoveryInterval.Milliseconds())
	} else {
		allowed, err = r.db.ReserveFreshRecovery(ctx, r.account.ID, now.UnixMilli(), freshRecoveryInterval.Milliseconds(), maxAttempts)
	}
	if err != nil {
		return fmt.Errorf("保存恢复认证额度失败，未发送登录请求: %w", err)
	}
	if !allowed {
		return &recoveryBlockedError{reason: "恢复认证额度未获批准，未发送登录请求；请检查设置后手动重新登录"}
	}
	r.safety.FreshLoginAttempts++
	r.safety.LastFreshLoginMS = now.UnixMilli()
	if manual {
		r.mu.Lock()
		r.manualRecoveryPending = false
		r.mu.Unlock()
	}
	r.log.Info("recovery reserved fresh authentication", "account_id", r.account.ID, "manual", manual, "attempt", r.safety.FreshLoginAttempts, "max_attempts", maxAttempts)
	return nil
}

func (r *Runner) checkFreshRecoveryAuthorization(ctx context.Context) error {
	if err := r.checkGameRPCContext(ctx, clientproto.RPCIndexLogin.String()); err != nil {
		return err
	}
	if r.isSessionInvalidated() {
		return r.sessionInvalidatedError("取消恢复认证")
	}
	s, _ := r.accountSafetySnapshot()
	p := r.Policy()
	if s.RestrictionCode != 0 && !p.GetAutomationEnabled() {
		return &recoveryBlockedError{reason: "自动化已暂停，等待手动启动；未发送恢复认证，未延长服务端冷却"}
	}
	if s.RestrictionCode != 0 && !p.GetBasic().GetServerErrorFreshLoginEnabled() && !r.manualRecoveryAuthorized(ctx) {
		return &recoveryBlockedError{reason: "缓存会话不可用，且未允许自动重新登录；请手动重新登录或开启“异常后重新认证”，未延长服务端冷却"}
	}
	return nil
}

func (r *Runner) observeRecoveryDisplacement(err error) {
	var serverErr *babigame.RPCServerError
	if errors.As(err, &serverErr) && serverErr.Envelope.IsSessionDisplaced() {
		r.handleSessionInvalidated(serverErr.Envelope.ErrorMsg(), true)
	}
}

func (r *Runner) verifyRestrictionRecovery(ctx context.Context, client *babigame.Client, session *babigame.Session, revision uint64, resume bool, baseline json.RawMessage) error {
	mode := "新认证会话"
	if resume {
		mode = "缓存会话"
	}
	r.emit(Event{Kind: "account_request_verifying", Category: "account", Domain: "account.request", Action: "verifying",
		Label: "账号恢复核验", Message: mode + "登录成功，正在核验业务状态；普通操作仍暂停", Level: "info"})
	rpc := babigame.NewRPCClient(client, session, babigame.WithDefaultTimeout(15*time.Second))
	return r.verifyRecoveryState(ctx, revision, baseline, func(ctx context.Context, name clientproto.RPCName) (json.RawMessage, error) {
		result, err := babigame.CallRPC[clientproto.StateDelta](ctx, rpc, name, struct{}{}, babigame.WithPayloadApply(false))
		return result.Payload, err
	})
}

// Login acceptance alone says nothing about business availability. Use only
// non-spending probes and fresh response evidence, never cached reputation.
// The permit is bound to this runner and protection revision; late errors win.
func (r *Runner) verifyRecoveryState(ctx context.Context, revision uint64, baseline json.RawMessage, probe func(context.Context, clientproto.RPCName) (json.RawMessage, error)) error {
	ctx = context.WithValue(ctx, recoveryProbeKey{}, recoveryProbePermit{runner: r, revision: revision})
	// Namespace responses may be partial deltas. The new login baseline is
	// current-cycle evidence; the previous connection's cached state is not.
	fresh := state.New()
	fresh.ApplyV(baseline)
	for _, name := range []clientproto.RPCName{clientproto.RPCUsrLazySync, clientproto.RPCReputationView} {
		if err := r.checkGameRPCContext(ctx, name.String()); err != nil {
			return err
		}
		v, err := probe(ctx, name)
		if err != nil {
			return fmt.Errorf("%s 恢复核验失败: %w", name, err)
		}
		fresh.ApplyV(v)
		if name == clientproto.RPCReputationView {
			rep, observed := fresh.Reputation()
			if !observed {
				return fmt.Errorf("%s 未返回有效健康分状态，继续保持保护", name)
			}
			r.state.ApplyV(v)
			r.mu.Lock()
			r.lastReputationSyncTick = time.Now()
			r.mu.Unlock()
			if enabled, threshold := reputationGuardConfig(r.Policy()); enabled && rep.Score < threshold {
				r.disableAutomationForReputation(rep.Score, threshold, "recovery")
				return reputationGuardError{Score: rep.Score, Threshold: threshold}
			}
		} else {
			r.state.ApplyV(v)
		}
	}
	if r.isSessionInvalidated() {
		return r.sessionInvalidatedError("恢复核验期间会话失效")
	}
	if err := r.checkGameRPCContext(ctx, clientproto.RPCReputationView.String()); err != nil {
		return err
	}
	return r.clearAccountRestriction(revision)
}
