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
	"github.com/SilkageNet/mygardenworld/internal/store"
)

const freshRecoveryInterval = 30 * time.Minute

type recoveryProbeKey struct{}
type recoveryProbePermit struct {
	runner   *Runner
	revision uint64
}

func freshRecoveryAvailable(s store.AccountRequestSafety, now time.Time) bool {
	return s.RestrictionCode == 5000 && s.RestrictionAttempts >= 1 && now.UnixMilli() >= s.RestrictedUntilMS &&
		!s.FreshLoginAttempted && (s.LastFreshLoginMS == 0 || now.UnixMilli() >= s.LastFreshLoginMS+freshRecoveryInterval.Milliseconds())
}

func (r *Runner) freshRecoveryEligible(now time.Time) bool {
	p := r.Policy()
	s, _ := r.accountSafetySnapshot()
	return p.GetAutomationEnabled() && p.GetBasic().GetServerErrorFreshLoginEnabled() && freshRecoveryAvailable(s, now)
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
	if r.safety.RestrictionCode != 5000 {
		return nil
	}
	if !p.GetAutomationEnabled() || !p.GetBasic().GetServerErrorFreshLoginEnabled() || !freshRecoveryAvailable(r.safety, now) {
		return fmt.Errorf("5000 保护中未获准重新认证（需开启自动化和独立开关、冷却已结束，且本次未尝试、距上次至少 30 分钟）；可检查设置后手动登录")
	}
	if r.db == nil {
		return fmt.Errorf("无法持久化恢复认证额度，未发送登录请求")
	}
	allowed, err := r.db.ReserveFreshRecovery(ctx, r.account.ID, now.UnixMilli(), freshRecoveryInterval.Milliseconds())
	if err != nil {
		return fmt.Errorf("保存恢复认证额度失败，未发送登录请求: %w", err)
	}
	if !allowed {
		return fmt.Errorf("恢复认证额度已占用，未发送登录请求")
	}
	r.safety.FreshLoginAttempted = true
	r.safety.LastFreshLoginMS = now.UnixMilli()
	r.log.Info("5000 recovery reserved one fresh authentication", "account_id", r.account.ID)
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
	if s.RestrictionCode == 5000 && (!p.GetAutomationEnabled() || !p.GetBasic().GetServerErrorFreshLoginEnabled()) {
		return fmt.Errorf("自动化或 5000 重新登录开关已关闭，取消恢复认证")
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
