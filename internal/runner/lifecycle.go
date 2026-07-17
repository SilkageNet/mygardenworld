package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	reconnectInitialWait = 2 * time.Second
	reconnectMaxWait     = 30 * time.Second
	defaultReloginWait   = 5 * time.Minute
	maxReloginWait       = 24 * time.Hour
	waterDropsItemID     = int32(7)
)

// Start kicks off the runner. Blocks until login completes (or fails); the
// WebSocket loop and decision loop run in background goroutines.
func (r *Runner) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.cancel != nil {
		r.mu.Unlock()
		return errors.New("runner already started")
	}
	rctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.mu.Unlock()
	fail := func(err error) error {
		cancel()
		r.mu.Lock()
		r.cancel = nil
		r.mu.Unlock()
		return err
	}

	username, password, err := r.db.GetCredentials(ctx, r.account.ID)
	if err != nil {
		return fail(fmt.Errorf("creds: %w", err))
	}

	r.installStateHandlers()
	client, err := r.connectFresh(ctx, username, password)
	if err != nil {
		if r.autoReloginPending() {
			go r.decisionLoop(rctx)
			go r.connectionLoop(rctx, username, password, nil)
			return nil
		}
		return fail(err)
	}

	go r.decisionLoop(rctx)
	go r.connectionLoop(rctx, username, password, client)
	return nil
}

func (r *Runner) connectFresh(ctx context.Context, username, password string) (*babigame.Client, error) {
	httpc := babigame.NewHTTPClient(r.cfg, "", "", "")
	if pkg, err := httpc.QueryPackageConfig(ctx); err == nil {
		if pkg.GameVersion != "" {
			httpc.Cfg.GameVersion = pkg.GameVersion
			httpc.Cfg.ClientVersion = pkg.GameVersion
		}
		if rows, err := babigame.LoadFarmLandConfig(ctx, httpc, pkg); err == nil {
			lands := make([]state.FarmLandInfo, 0, len(rows))
			for _, row := range rows {
				lands = append(lands, state.FarmLandInfo{
					ID:        row.ID,
					OpenLevel: row.OpenLevel,
					Cost:      append([]int32(nil), row.Cost...),
					Wasteland: append([]int32(nil), row.Wasteland...),
				})
			}
			r.state.SetFarmLands(lands)
			r.log.Info("loaded runtime land config", "version", pkg.GameVersion, "lands", len(lands))
		} else {
			r.log.Warn("load runtime land config failed", "err", err, "version", pkg.GameVersion)
		}
	} else {
		r.log.Warn("query package config failed", "err", err)
	}
	session, err := babigame.PerformLoginWithPassword(ctx, httpc, username, password, 1)
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}

	if blob, err := babigame.MarshalSessionJSON(session); err == nil {
		_ = r.db.SaveSession(ctx, r.account.ID, blob, nil)
	}
	now := time.Now().UTC()
	_ = r.db.UpdateLogin(ctx, r.account.ID, session.AID, int32(session.GsIdx), session.WSURL(), now)

	client := r.newClient(session)
	if err := client.Connect(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ws connect: %w", err)
	}
	r.resetFreshSessionAutomationState()

	r.mu.Lock()
	r.session = session
	r.httpc = httpc
	r.client = client
	r.rqst = rqstState{}
	r.mu.Unlock()

	// The official client sends index.login as the first WS initialization
	// call after the HTTP login and route-token bootstrap.
	if v, err := client.Login(ctx, 1); err == nil {
		r.state.ApplyV(v)
		r.syncAccountDisplayName(ctx, v, session)
	} else {
		r.log.Warn("ws index.login failed", "err", err)
		_ = client.Close()
		r.clearDisconnectedClient(client)
		return nil, fmt.Errorf("启动登录失败: %w", err)
	}
	if r.isSessionInvalidated() {
		_ = client.Close()
		r.clearDisconnectedClient(client)
		return nil, r.sessionInvalidatedError("session invalidated during startup")
	}
	if v, err := client.LazySync(ctx); err == nil {
		r.state.ApplyV(v)
	}
	// Only now may the shadow controller observe activity state. During a
	// reconnecting fresh login the State still contains the previous epoch's
	// board until index.login/lazySync have supplied this epoch's baseline.
	r.markDessertSessionStateReady()
	if r.isSessionInvalidated() {
		_ = client.Close()
		r.clearDisconnectedClient(client)
		return nil, r.sessionInvalidatedError("session invalidated during startup")
	}
	if err := r.enforceReputationGuard(ctx, client, session, "startup", time.Now()); err != nil {
		_ = client.Close()
		r.clearDisconnectedClient(client)
		return nil, err
	}

	// Send home verification after login to satisfy anti-cheat.
	if err := r.ensureHomeRqst(ctx); err != nil {
		r.log.Debug("home verification failed", "err", err)
	}

	r.emit(Event{Kind: "session", Message: fmt.Sprintf("已连接 (服务器=%s 区=%d)", session.GsHost, session.GsIdx)})
	return client, nil
}

func (r *Runner) resetPearlHireSession() {
	if r.state != nil {
		r.state.ResetPearlHireSession()
	}
	r.mu.Lock()
	for key := range r.operationCooldowns {
		if strings.HasPrefix(key, clientproto.RPCPearlPlaceHire.String()+":") ||
			strings.HasPrefix(key, clientproto.RPCOpptGetDetailOppts.String()+":") ||
			strings.HasPrefix(key, clientproto.RPCPearlGetHireStateByUids.String()+":") ||
			strings.HasPrefix(key, clientproto.RPCPearlGetRecommendList.String()+":") ||
			strings.HasPrefix(key, clientproto.RPCPearlRefresh.String()+":") ||
			strings.HasPrefix(key, clientproto.RPCFrdEnter.String()+":") || key == "basic.pearl.hire.blocked" {
			delete(r.operationCooldowns, key)
		}
	}
	r.mu.Unlock()
}

func (r *Runner) resetFreshSessionAutomationState() {
	r.resetSideLaneFairness()
	r.resetPearlHireSession()
	r.resetDessertRoundSession()
	if r.state != nil {
		r.state.ResetDessertSession()
	}
}

func (r *Runner) syncAccountDisplayName(ctx context.Context, rawV json.RawMessage, session *babigame.Session) {
	desired := babigame.DisplayNameFromState(rawV, session.GsIdx, r.account.Name)
	if desired == "" || desired == r.account.Name {
		return
	}
	name, err := r.db.UniqueAccountName(ctx, r.account.UserID, r.account.ID, desired)
	if err != nil {
		r.log.Warn("choose account display name failed", "err", err, "desired", desired)
		return
	}
	if name == r.account.Name {
		return
	}
	acc, err := r.db.RenameAccount(ctx, r.account.ID, name)
	if err != nil {
		r.log.Warn("sync account display name failed", "err", err, "desired", name)
		return
	}
	r.mu.Lock()
	r.account = acc
	r.mu.Unlock()
	r.log.Info("synced account display name", "name", name)
}

func (r *Runner) newClient(session *babigame.Session) *babigame.Client {
	client := babigame.NewClient(session)
	client.DebugWriter = r.debugWriter
	client.OnSessionExpired(func(d babigame.WSResponseD) {
		r.handleSessionInvalidated(d.ErrorMsg(), d.IsSessionDisplaced())
	})
	client.OnBinary(func(items []json.RawMessage) {
		if reason, ok := babigame.SessionDisplacementFromBinary(items); ok {
			r.handleSessionInvalidated(reason, true)
		}
	})
	for _, ns := range observedCaptureNamespaces() {
		ns := ns
		client.OnNamespace(ns, func(_ string, raw json.RawMessage, _ babigame.WSResponseD) {
			fragment, _ := json.Marshal(map[string]json.RawMessage{ns: raw})
			r.state.ApplyV(fragment)
		})
	}
	return client
}

func observedCaptureNamespaces() []string {
	return babigame.ObservedNamespaceKeys()
}

func (r *Runner) installStateHandlers() {
	r.state.SetOnChange(func(changes []state.LandChange) {
		if len(changes) > 0 {
			r.mu.Lock()
			for _, change := range changes {
				delete(r.harvestBlockedUntil, change.LandID)
			}
			r.mu.Unlock()
		}
		r.emitLandChanges(changes)
	})
	r.state.SetOnResourceChange(func(snap state.ResourceSnapshot) {
		r.stats.ObserveResourceSnapshot(snap, time.Now())
		raw, _ := json.Marshal(snap)
		r.emit(Event{
			Kind:        "resource_changed",
			Message:     fmt.Sprintf("资源更新: 金币=%d 水滴=%d/%d Lv.%d", snap.Gold, snap.WaterDrops, snap.WaterDropsTotal, snap.Level),
			PayloadJSON: string(raw),
		})
	})
	r.state.SetOnInventoryChange(func(snap state.InventorySnapshot) {
		r.stats.ObserveInventorySnapshot(snap, time.Now())
		raw, _ := json.Marshal(snap)
		r.emit(Event{
			Kind:        "inventory_changed",
			Message:     inventoryChangeMessage(snap),
			PayloadJSON: string(raw),
		})
	})
}

func (r *Runner) connectionLoop(ctx context.Context, username, password string, client *babigame.Client) {
	current := client

connection:
	for {
		if current != nil {
			select {
			case <-ctx.Done():
				return
			case <-current.Done():
			}
			r.clearDisconnectedClient(current)
		}
		if ctx.Err() != nil {
			return
		}
		if r.autoReloginPending() {
			next, ok := r.reloginAfterDisplacement(ctx, username, password)
			if !ok {
				return
			}
			current = next
			continue
		}
		if r.isSessionInvalidated() || current == nil {
			return
		}
		r.emit(Event{Kind: "ws_disconnected", Message: "网络连接断开，准备重连", Level: "warn"})

		wait := reconnectInitialWait
		for {
			if !sleepOrDone(ctx, wait) || r.isSessionInvalidated() {
				if r.autoReloginPending() {
					current = nil
					continue connection
				}
				return
			}
			next, err := r.connectFresh(ctx, username, password)
			if err == nil {
				current = next
				break
			}
			if isReputationGuardError(err) {
				return
			}
			if ctx.Err() != nil || r.isSessionInvalidated() {
				if r.autoReloginPending() {
					current = nil
					continue connection
				}
				return
			}
			r.emit(Event{
				Kind:    "ws_disconnected",
				Message: fmt.Sprintf("重连失败: %v；%s 后重试", err, nextReconnectWait(wait)),
				Level:   "warn",
			})
			wait = nextReconnectWait(wait)
		}
	}
}

func (r *Runner) reloginAfterDisplacement(ctx context.Context, username, password string) (*babigame.Client, bool) {
	baseWait := r.reloginInterval()
	wait := baseWait
	for {
		if ctx.Err() != nil {
			return nil, false
		}
		if !sleepOrDone(ctx, wait) {
			return nil, false
		}
		if ctx.Err() != nil {
			return nil, false
		}
		if !r.prepareAutoReloginAttempt() {
			return nil, false
		}
		r.emit(Event{Kind: "session_relogin", Message: "被挤号等待结束，正在自动登录", Level: "info"})
		next, err := r.connectFresh(ctx, username, password)
		if err == nil {
			if ctx.Err() != nil || !r.completeAutoRelogin() {
				_ = next.Close()
				r.clearDisconnectedClient(next)
				if ctx.Err() != nil {
					return nil, false
				}
				if r.autoReloginPending() {
					wait = baseWait
					continue
				}
				r.failClosedPendingDisplacedRelogin()
				return nil, false
			}
			return next, true
		}
		if ctx.Err() != nil || isReputationGuardError(err) || r.sessionInvalidatedWithoutAutoRelogin() {
			return nil, false
		}
		if r.autoReloginPending() {
			wait = baseWait
			continue
		}
		nextWait := nextReloginWait(wait, baseWait)
		r.emit(Event{
			Kind:    "session_relogin",
			Message: fmt.Sprintf("自动登录失败: %v；%s 后重试", err, nextWait),
			Level:   "warn",
		})
		wait = nextWait
	}
}

func (r *Runner) reloginInterval() time.Duration {
	seconds := r.Policy().GetBasic().GetReconnectIntervalSeconds()
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 {
		return defaultReloginWait
	}
	if seconds > maxReloginWait.Seconds() {
		return maxReloginWait
	}
	d := time.Duration(seconds * float64(time.Second))
	if d < time.Second {
		return time.Second
	}
	return d
}

func nextReloginWait(current, base time.Duration) time.Duration {
	if base <= 0 {
		base = defaultReloginWait
	}
	if current < base {
		current = base
	}
	capWait := reconnectMaxWait
	if base > capWait {
		capWait = base
	}
	if current >= capWait || current > capWait/2 {
		return capWait
	}
	return current * 2
}

func (r *Runner) autoReloginPending() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessionInvalidated && r.sessionAutoRelogin
}

func (r *Runner) sessionInvalidatedWithoutAutoRelogin() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessionInvalidated && !r.sessionAutoRelogin
}

func (r *Runner) beginAutoReloginAttempt() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.sessionAutoRelogin {
		return false
	}
	if r.policy == nil || !r.policy.GetBasic().GetDisplacedSessionReloginEnabled() {
		return false
	}
	r.sessionInvalidated = false
	return true
}

// prepareAutoReloginAttempt is the final policy gate immediately before a
// fresh HTTP login. SetPolicy normally cancels the wait as soon as the switch
// is turned off; this recheck also covers a change racing with timer expiry.
func (r *Runner) prepareAutoReloginAttempt() bool {
	if !r.Policy().GetBasic().GetDisplacedSessionReloginEnabled() {
		r.failClosedPendingDisplacedRelogin()
		return false
	}
	if !r.beginAutoReloginAttempt() {
		r.failClosedPendingDisplacedRelogin()
		return false
	}
	return true
}

// completeAutoRelogin atomically accepts a newly connected client only while
// this is still the active displaced-session attempt and its policy switch is
// still enabled. A concurrent SetPolicy(false) therefore wins cleanly instead
// of having its fail-closed state overwritten by a late login success.
func (r *Runner) completeAutoRelogin() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.sessionAutoRelogin ||
		r.sessionInvalidated ||
		r.policy == nil ||
		!r.policy.GetBasic().GetDisplacedSessionReloginEnabled() {
		return false
	}
	r.sessionInvalidated = false
	r.sessionInvalidatedReason = ""
	r.sessionAutoRelogin = false
	return true
}

func (r *Runner) clearDisconnectedClient(client *babigame.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client == client {
		r.client = nil
		r.session = nil
		r.httpc = nil
		r.resetSideLaneFairnessLocked()
	}
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextReconnectWait(d time.Duration) time.Duration {
	d *= 2
	if d > reconnectMaxWait {
		return reconnectMaxWait
	}
	return d
}

// Stop terminates the runner. Idempotent.
func (r *Runner) Stop() {
	r.stopOnce.Do(func() {
		if r.stats != nil {
			r.stats.MarkStopped(time.Now())
		}
		r.mu.Lock()
		cancel := r.cancel
		r.cancel = nil
		client := r.client
		r.client = nil
		r.resetSideLaneFairnessLocked()
		debugWriter := r.debugWriter
		r.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		if client != nil {
			_ = client.Close()
		}
		if debugWriter != nil {
			debugWriter.Close()
		}
		close(r.done)
		r.emit(Event{Kind: "ws_disconnected", Message: "已断开连接"})
	})
}

func (r *Runner) markSessionInvalidated(reason string) {
	r.handleSessionInvalidated(reason, babigame.IsSessionDisplacementReason(reason))
}

func (r *Runner) handleSessionInvalidated(reason string, sessionDisplaced bool) {
	if reason == "" {
		reason = "会话已过期，请重新登录"
	}
	autoRelogin := sessionDisplaced && r.Policy().GetBasic().GetDisplacedSessionReloginEnabled()
	ctx := context.Background()
	if r.db != nil {
		if err := r.db.DeleteSession(ctx, r.account.ID); err != nil {
			r.log.Warn("delete invalidated session failed", "err", err)
		}
	}
	r.mu.Lock()
	if r.sessionInvalidated {
		r.mu.Unlock()
		return
	}
	r.sessionInvalidated = true
	r.sessionInvalidatedReason = reason
	r.sessionAutoRelogin = autoRelogin
	client := r.client
	r.resetSideLaneFairnessLocked()
	r.mu.Unlock()
	if autoRelogin {
		wait := r.reloginInterval()
		r.emit(Event{
			Kind:    "session_expired",
			Message: fmt.Sprintf("检测到账号在其他设备登录，%s 后自动登录：%s", wait, reason),
			Level:   "warn",
		})
		if client != nil {
			_ = client.Close()
		}
		return
	}

	r.disableAutomationPreferenceForInvalidatedSession(ctx, reason)
	r.emit(Event{Kind: "session_expired", Message: fmt.Sprintf("检测到会话失效，已停止自动化：%s", reason)})
	r.Stop()
}

// failClosedPendingDisplacedRelogin cancels an already scheduled displaced-
// session login after the user disables the recovery switch. Marking the
// pending flag false before updating the policy prevents SetPolicy from
// recursively entering this path while automation_enabled is persisted off.
func (r *Runner) failClosedPendingDisplacedRelogin() bool {
	r.mu.Lock()
	if !r.sessionAutoRelogin {
		r.mu.Unlock()
		return false
	}
	reason := r.sessionInvalidatedReason
	r.sessionInvalidated = true
	r.sessionAutoRelogin = false
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if reason == "" {
		reason = "账号在其他设备登录"
	}

	ctx := context.Background()
	r.disableAutomationPreferenceForInvalidatedSession(ctx, reason)
	r.emit(Event{
		Kind:    "session_expired",
		Message: fmt.Sprintf("被挤号自动重登已关闭，已停止自动化：%s", reason),
		Level:   "warn",
	})
	r.Stop()
	return true
}

func (r *Runner) sessionInvalidatedError(message string) error {
	r.mu.RLock()
	reason := r.sessionInvalidatedReason
	r.mu.RUnlock()
	if reason == "" {
		return errors.New(message)
	}
	return fmt.Errorf("%s: %s", message, reason)
}

func (r *Runner) disableAutomationPreferenceForInvalidatedSession(ctx context.Context, reason string) {
	p := r.Policy()
	if !p.GetAutomationEnabled() {
		return
	}
	p.AutomationEnabled = false
	r.SetPolicy(p)
	if r.db != nil {
		raw, err := policycfg.ToJSON(p)
		if err != nil {
			r.log.Warn("marshal invalidated-session policy failed", "err", err, "reason", reason)
		} else if err := r.db.SavePolicyJSON(ctx, r.account.ID, raw); err != nil {
			r.log.Warn("persist invalidated-session policy failed", "err", err, "reason", reason)
		}
	}
	r.emit(policyDisabledBySessionInvalidatedEvent(reason))
}

func policyDisabledBySessionInvalidatedEvent(reason string) Event {
	payload, _ := json.Marshal(map[string]any{
		"automation_enabled": false,
		"reason":             reason,
	})
	return Event{
		Kind:        "policy_changed",
		Category:    "account",
		Domain:      "account.session",
		Action:      "blocked",
		Label:       "连接",
		Level:       "warn",
		Message:     "会话失效，已关闭自动恢复",
		PayloadJSON: string(payload),
	}
}

// Done returns a channel that closes once the runner has fully stopped.
func (r *Runner) Done() <-chan struct{} { return r.done }
