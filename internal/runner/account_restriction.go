package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

// All game RPC paths, including heartbeat and executor-internal follow-up
// reads, share this guard. Only a post-wait login
// can pass while recovery is pending. HTTP reconnect is separately gated.
func (r *Runner) beforeGameRPC(_ context.Context, name string) error {
	s, _ := r.accountSafetySnapshot()
	if s.RestrictionCode == 0 {
		return nil
	}
	if time.Now().UnixMilli() >= s.RestrictedUntilMS {
		if name == clientproto.RPCIndexLogin.String() || name == clientproto.RPCIndexReLogin.String() {
			return nil
		}
	}
	return accountRestrictionError(s)
}

func (r *Runner) observeGameRPC(name string, d babigame.WSResponseD) {
	code := d.ErrorCode()
	if code != 97777 && code != 97778 {
		return
	}
	r.recordAccountRestriction(name, d, time.Now())
}

func (r *Runner) recordAccountRestriction(name string, d babigame.WSResponseD, now time.Time) {
	r.safetyMu.Lock()
	previous := r.safety
	next := nextAccountRestriction(previous, d, now)
	r.safetyRevision++ // Even a late duplicate invalidates an in-flight probe.
	r.safety = next    // Fail closed in memory even if persistence is unavailable.
	changed := previous.RestrictedUntilMS != next.RestrictedUntilMS || previous.RestrictionCode != next.RestrictionCode
	err := r.persistRestrictionLocked(next)
	r.safetyMu.Unlock()
	if !changed && err == nil {
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"rpc": name, "server_code": next.RestrictionCode,
		"restricted_until_ms": next.RestrictedUntilMS, "attempt": next.RestrictionAttempts,
	})
	message := fmt.Sprintf("服务端返回 %d，暂停该账号全部游戏请求；%s 后验证恢复。97777 含义尚未确认，不能据此断定封禁或删除频率阈值",
		d.ErrorCode(), time.UnixMilli(next.RestrictedUntilMS).Local().Format("01/02 15:04:05"))
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
	wait := min(30*time.Minute, 5*time.Minute*time.Duration(1<<uint(max(0, next.RestrictionAttempts-1))))
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
	if active && previous.RestrictionCode == 97778 {
		next.RestrictionCode = 97778
	}
	return next
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
	if err := r.persistRestrictionLocked(next); err != nil {
		r.safetyMu.Unlock()
		return fmt.Errorf("恢复状态保存失败，账号继续暂停: %w", err)
	}
	r.safety = next
	r.safetyMu.Unlock()
	r.emit(Event{Kind: "account_request_resumed", Category: "account", Domain: "account.request", Action: "resumed",
		Label: "账号请求保护", Message: "冷却后状态验证成功，恢复账号游戏请求", Level: "info"})
	return nil
}

// Resume through the existing cached-session reconnect path, which obtains a
// full login baseline. LazySync does not refresh farm/inventory, and reLogin on
// an already initialized socket does not refresh its snapshot either. Close
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
	r.safety.RestrictedUntilMS = time.Now().Add(5 * time.Minute).UnixMilli()
	r.safetyRevision++
	err := r.persistRestrictionLocked(r.safety)
	r.safetyMu.Unlock()
	message := fmt.Sprintf("账号恢复验证未成功，继续暂停 5 分钟: %v", probeErr)
	if err != nil {
		message += fmt.Sprintf("；保存保护状态失败: %v", err)
	}
	r.emit(Event{Kind: "account_request_paused", Category: "account", Domain: "account.request", Action: "blocked",
		Label: "账号请求保护", Message: message, Level: "warn"})
}
