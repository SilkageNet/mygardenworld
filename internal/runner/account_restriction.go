package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

// All game RPC paths, including heartbeat and executor-internal follow-up
// reads, share this guard. Only post-wait login and revision-bound, non-spending
// recovery probes can pass while protection is pending. HTTP login is gated too.
func (r *Runner) beforeGameRPC(ctx context.Context, name string) (err error) {
	guard, guardedHire := ctx.Value(pearlHireSendGuardKey{}).(func() error)
	guardedHire = guardedHire && name == clientproto.RPCPearlPlaceHire.String()
	defer func() {
		if guardedHire && err != nil {
			err = &pearlHireNotSentError{err: err}
		}
	}()
	if err := r.checkGameRPCContext(ctx, name); err != nil {
		return err
	}
	if err := r.waitRaceTakeReady(ctx, name); err != nil {
		return err
	}
	if err := r.pacer.wait(ctx, name, func() error { return r.checkGameRPCContext(ctx, name) }); err != nil {
		return err
	}
	if err := r.validateActivitySyncBeforeSend(ctx, name); err != nil {
		return err
	}
	if err := r.validateFmlMembershipBeforeSend(ctx, name); err != nil {
		return err
	}
	if guardedHire {
		if scheduled, _ := ctx.Value(scheduledOperationKey{}).(bool); scheduled && !r.Policy().GetAutomationEnabled() {
			return fmt.Errorf("自动化已关闭，取消尚未发送的珍珠雇佣")
		}
		if err := guard(); err != nil {
			return err
		}
	}
	return r.validateRaceMutationBeforeSend(ctx, name)
}

func (r *Runner) checkGameRPC(name string) error {
	return r.checkGameRPCContext(context.Background(), name)
}

func (r *Runner) checkGameRPCContext(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.gameGate != nil {
		if blocked, _ := r.gameGate.status(); blocked {
			return ErrMaintenance
		}
	}
	s, revision := r.accountSafetySnapshot()
	if s.RestrictionCode == 0 {
		return nil
	}
	if time.Now().UnixMilli() >= s.RestrictedUntilMS {
		if name == clientproto.RPCIndexLogin.String() || name == clientproto.RPCIndexReLogin.String() {
			return nil
		}
		if permit, ok := ctx.Value(recoveryProbeKey{}).(recoveryProbePermit); ok && permit.runner == r && permit.revision == revision &&
			(name == clientproto.RPCUsrLazySync.String() || name == clientproto.RPCReputationView.String()) {
			return nil
		}
	}
	return accountRestrictionError(s)
}

func (r *Runner) observeGameRPC(name string, d babigame.WSResponseD) {
	r.observeGameRPCAt(name, d, time.Now())
}

type serverFailure struct {
	rpc string
	at  time.Time
}

// 5000 has no confirmed protocol meaning. Only a burst across different RPCs
// opens account-wide protection: a single domain failure stays local. These
// are client-side circuit-breaker thresholds, not server rate-limit claims.
const serverFailureWindow = time.Minute
const serverFailureThreshold = 3

func (r *Runner) observeGameRPCAt(name string, d babigame.WSResponseD, now time.Time) {
	code := d.ErrorCode()
	if code != 5000 && code != 97777 && code != 97778 {
		return
	}
	r.safetyMu.Lock()
	if code == 5000 && r.safety.RestrictionCode == 0 {
		kept := r.serverFailures[:0]
		for _, failure := range r.serverFailures {
			if !failure.at.Before(now.Add(-serverFailureWindow)) && !failure.at.After(now) {
				kept = append(kept, failure)
			}
		}
		r.serverFailures = kept
		r.serverFailures = append(r.serverFailures, serverFailure{rpc: name, at: now})
		if len(r.serverFailures) > serverFailureThreshold {
			r.serverFailures = r.serverFailures[len(r.serverFailures)-serverFailureThreshold:]
		}
		crossRPC := false
		for _, failure := range r.serverFailures {
			crossRPC = crossRPC || failure.rpc != name
		}
		if len(r.serverFailures) < serverFailureThreshold || !crossRPC {
			r.safetyMu.Unlock()
			return
		}
	}
	// A failed recovery probe immediately reopens protection, even if its
	// observation window expired. Hold the lock through the transition so a
	// late error cannot be cleared by an older successful probe.
	r.recordAccountRestrictionLocked(name, d, now)
}

func (r *Runner) recordAccountRestriction(name string, d babigame.WSResponseD, now time.Time) {
	r.safetyMu.Lock()
	r.recordAccountRestrictionLocked(name, d, now)
}

// Takes ownership of safetyMu and releases it before emitting an event.
func (r *Runner) recordAccountRestrictionLocked(name string, d babigame.WSResponseD, now time.Time) {
	previous := r.safety
	next := nextAccountRestriction(previous, d, now)
	r.safetyRevision++ // Even a late duplicate invalidates an in-flight probe.
	r.safety = next    // Fail closed in memory even if persistence is unavailable.
	changed := previous.RestrictedUntilMS != next.RestrictedUntilMS || previous.RestrictionCode != next.RestrictionCode
	failureRPCs := make([]string, 0, len(r.serverFailures))
	for _, failure := range r.serverFailures {
		failureRPCs = append(failureRPCs, failure.rpc)
	}
	err := r.persistRestrictionLocked(next)
	r.safetyMu.Unlock()
	if !changed && err == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"rpc": name, "server_code": next.RestrictionCode,
		"restricted_until_ms": next.RestrictedUntilMS, "attempt": next.RestrictionAttempts,
		"recent_failure_rpcs": failureRPCs,
	})
	message := fmt.Sprintf("%s 返回 %d，暂停该账号全部游戏请求；%s 后验证恢复。错误码不能单独证明封禁、挤号或安全频率阈值",
		name, d.ErrorCode(), time.UnixMilli(next.RestrictedUntilMS).Local().Format("01/02 15:04:05"))
	if next.RestrictionCode == 5000 {
		message += "；短时间跨接口重复失败或恢复验证仍失败，已触发本地请求保护；保留自动化设置，不重放失败操作"
	}
	if err != nil {
		message += fmt.Sprintf("；保护状态保存失败，当前进程仍保持暂停: %v", err)
	}
	r.emit(Event{Kind: "account_request_paused", Category: "account", Domain: "account.request", Action: "blocked",
		Label: "账号请求保护", Message: message, PayloadJSON: string(payload), Level: "warn"})
}

func nextAccountRestriction(previous store.AccountRequestSafety, d babigame.WSResponseD, now time.Time) store.AccountRequestSafety {
	next := previous
	active := previous.RestrictionCode != 0 && previous.RestrictedUntilMS > now.UnixMilli()
	if !active {
		next.RestrictionAttempts = min(4, previous.RestrictionAttempts+1)
	}
	wait := restrictionBackoff(next.RestrictionAttempts)
	until := now.Add(wait)
	if d.ErrorCode() == 97778 {
		// The official client formats args[0] as a retry date. Unknown or
		// elapsed dates get a conservative fallback, never immediate retry.
		if serverUntil, ok := restrictionRetryAt(d.M, now); ok {
			until = serverUntil.Add(30 * time.Second)
		} else {
			until = now.Add(30 * time.Minute)
		}
	}
	if active {
		// Already in-flight duplicates must not slide the deadline forever.
		// A later explicit server date or escalation to 97778 can extend it.
		until = time.UnixMilli(previous.RestrictedUntilMS)
		if serverUntil, ok := restrictionRetryAt(d.M, now); d.ErrorCode() == 97778 && ok {
			until = maxTime(until, serverUntil.Add(30*time.Second))
		} else if d.ErrorCode() == 97778 && previous.RestrictionCode != 97778 {
			until = maxTime(until, now.Add(30*time.Minute))
		}
	}
	next.RestrictedUntilMS = max(until.UnixMilli(), previous.RestrictedUntilMS)
	next.RestrictionCode = d.ErrorCode()
	if active && (previous.RestrictionCode == 97778 || (previous.RestrictionCode == 97777 && d.ErrorCode() == 5000)) {
		next.RestrictionCode = previous.RestrictionCode
	}
	return next
}

func restrictionBackoff(attempts int) time.Duration {
	return min(30*time.Minute, 5*time.Minute*time.Duration(1<<uint(min(3, max(0, attempts-1)))))
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func restrictionRetryAt(raw json.RawMessage, now time.Time) (time.Time, bool) {
	var message struct {
		Args []json.RawMessage `json:"args"`
	}
	if json.Unmarshal(raw, &message) != nil || len(message.Args) == 0 {
		return time.Time{}, false
	}
	value := string(message.Args[0])
	if len(value) > 0 && value[0] == '"' {
		if json.Unmarshal(message.Args[0], &value) != nil {
			return time.Time{}, false
		}
	}
	var at time.Time
	if ms, err := strconv.ParseInt(value, 10, 64); err == nil {
		// JS Date's numeric argument is milliseconds, not seconds. Do not
		// invent a seconds conversion or parse a locale-dependent date.
		at = time.UnixMilli(ms)
	} else {
		at, _ = time.Parse(time.RFC3339Nano, value)
	}
	return at, !at.IsZero() && at.After(now)
}

// clearAccountRestriction requires both a successful probe and an unchanged
// revision. A concurrent/late error therefore wins over an old success.
func (r *Runner) clearAccountRestriction(revision uint64) error {
	r.safetyMu.Lock()
	if r.safety.RestrictionCode == 0 {
		r.safetyMu.Unlock()
		return nil
	}
	if r.safetyRevision != revision {
		err := accountRestrictionError(r.safety)
		r.safetyMu.Unlock()
		return err
	}
	next := r.safety
	next.RestrictedUntilMS, next.RestrictionCode, next.RestrictionAttempts = 0, 0, 0
	next.FreshLoginAttempted = false // Keep the cross-incident authentication rate limit.
	if err := r.persistRestrictionLocked(next); err != nil {
		r.safetyMu.Unlock()
		return fmt.Errorf("恢复状态保存失败，账号继续暂停: %w", err)
	}
	r.safety = next
	r.serverFailures = nil
	r.safetyMu.Unlock()
	r.emit(Event{Kind: "account_request_resumed", Category: "account", Domain: "account.request", Action: "resumed",
		Label: "账号请求保护", Message: "冷却后状态验证成功，恢复账号游戏请求", Level: "info"})
	return nil
}

// Resume through the shared session recovery path, which obtains a full login
// baseline (cached or explicitly opted-in fresh auth). LazySync does not refresh
// farm/inventory, and reLogin on an initialized socket does not refresh its
// snapshot either. Close
// the old socket first; never create a concurrent game connection or replay
// the operation that triggered the restriction.
func (r *Runner) recoverAccountRestriction(client *babigame.Client, now time.Time) bool {
	s, _ := r.accountSafetySnapshot()
	if s.RestrictionCode == 0 {
		return false
	}
	if now.UnixMilli() < s.RestrictedUntilMS {
		return true
	}
	r.operationMu.Lock()
	defer r.operationMu.Unlock()
	s, _ = r.accountSafetySnapshot()
	if s.RestrictionCode == 0 || time.Now().UnixMilli() < s.RestrictedUntilMS {
		return true
	}
	if !client.Closed() {
		_ = client.Close()
	}
	return true
}

func (r *Runner) deferRestrictionProbe(revision uint64, probeErr error) {
	r.safetyMu.Lock()
	// A coded failure was already recorded by the response observer.
	if r.safetyRevision != revision || r.safety.RestrictionCode == 0 {
		r.safetyMu.Unlock()
		return
	}
	// A positively expired cached token advances recovery backoff. Transport
	// failures and arbitrary error text do not establish token expiry. Fresh
	// authentication keeps its independent opt-in, cooldown and durable budget.
	var rejected *babigame.RPCServerError
	if r.safety.RestrictionCode == 5000 && errors.As(probeErr, &rejected) && rejected.Name == clientproto.RPCIndexReLogin &&
		rejected.Envelope.ErrorCode() == 91102 && !rejected.Envelope.IsSessionDisplaced() {
		r.safety.RestrictionAttempts = min(4, r.safety.RestrictionAttempts+1)
	}
	wait := restrictionBackoff(r.safety.RestrictionAttempts)
	r.safety.RestrictedUntilMS = time.Now().Add(wait).UnixMilli()
	r.safetyRevision++
	err := r.persistRestrictionLocked(r.safety)
	r.safetyMu.Unlock()
	message := fmt.Sprintf("账号恢复验证未成功，继续暂停 %s: %v", wait, probeErr)
	if err != nil {
		message += fmt.Sprintf("；保存保护状态失败: %v", err)
	}
	r.emit(Event{Kind: "account_request_paused", Category: "account", Domain: "account.request", Action: "blocked",
		Label: "账号请求保护", Message: message, Level: "warn"})
}
