package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	reconnectInitialWait = 2 * time.Second
	reconnectMaxWait     = 30 * time.Second
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
	client, err := r.connectFresh(rctx, username, password)
	if err != nil {
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

	r.mu.Lock()
	r.session = session
	r.httpc = httpc
	r.client = client
	r.mu.Unlock()

	// reLogin to populate full 100.0.1.
	if v, err := client.ReLogin(ctx, 1); err == nil {
		r.state.ApplyV(v)
	} else {
		r.log.Warn("relogin failed", "err", err)
	}
	if r.isSessionInvalidated() {
		return nil, errors.New("session invalidated during startup")
	}
	if v, err := client.LazySync(ctx); err == nil {
		r.state.ApplyV(v)
	}
	if r.isSessionInvalidated() {
		return nil, errors.New("session invalidated during startup")
	}

	r.emit(Event{Kind: "session", Message: fmt.Sprintf("已连接 (服务器=%s 区=%d)", session.GsHost, session.GsIdx)})
	return client, nil
}

func (r *Runner) newClient(session *babigame.Session) *babigame.Client {
	client := babigame.NewClient(session)
	client.DebugWriter = r.debugWriter
	client.OnSessionExpired(func(d babigame.WSResponseD) {
		r.markSessionInvalidated(d.ErrorMsg())
	})
	for _, ns := range []string{"7", "22", "100", "101", "105", "109", "114", "117"} {
		ns := ns
		client.OnNamespace(ns, func(_ string, raw json.RawMessage, _ babigame.WSResponseD) {
			fragment, _ := json.Marshal(map[string]json.RawMessage{ns: raw})
			r.state.ApplyV(fragment)
		})
	}
	return client
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
		raw, _ := json.Marshal(snap)
		r.emit(Event{
			Kind:        "resource_changed",
			Message:     fmt.Sprintf("资源更新: 金币=%d 水滴=%d/%d Lv.%d", snap.Gold, snap.WaterDrops, snap.WaterDropsTotal, snap.Level),
			PayloadJSON: string(raw),
		})
		// Reset blocked flags when the relevant condition changes.
		r.mu.Lock()
		if snap.Level > r.prevLevel && r.prevLevel > 0 {
			r.taskRecvBlocked = false
			r.storyUnlockBlocked = false
			r.landUnlockBlocked = false
		}
		if snap.WaterDrops > 0 {
			r.waterBlocked = false
			r.waterBlockedUntil = time.Time{}
		}
		r.flowerUpgradeBlocked = make(map[int32]flowerUpgradeBlock)
		r.cultivateBlocked = make(map[int32]time.Time)
		r.prevLevel = snap.Level
		r.mu.Unlock()
	})
	r.state.SetOnInventoryChange(func(snap state.InventorySnapshot) {
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
	for current != nil {
		select {
		case <-ctx.Done():
			return
		case <-current.Done():
		}
		if ctx.Err() != nil || r.isSessionInvalidated() {
			return
		}
		r.clearDisconnectedClient(current)
		r.emit(Event{Kind: "ws_disconnected", Message: "网络连接断开，准备重连", Level: "warn"})

		wait := reconnectInitialWait
		for {
			if !sleepOrDone(ctx, wait) || r.isSessionInvalidated() {
				return
			}
			next, err := r.connectFresh(ctx, username, password)
			if err == nil {
				current = next
				break
			}
			if ctx.Err() != nil || r.isSessionInvalidated() {
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

func (r *Runner) clearDisconnectedClient(client *babigame.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client == client {
		r.client = nil
		r.session = nil
		r.httpc = nil
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
		r.mu.Lock()
		cancel := r.cancel
		r.cancel = nil
		client := r.client
		r.client = nil
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
	if reason == "" {
		reason = "会话已过期，请重新登录"
	}
	if err := r.db.DeleteSession(context.Background(), r.account.ID); err != nil {
		r.log.Warn("delete invalidated session failed", "err", err)
	}
	r.mu.Lock()
	if r.sessionInvalidated {
		r.mu.Unlock()
		return
	}
	r.sessionInvalidated = true
	r.sessionInvalidatedReason = reason
	r.mu.Unlock()

	r.emit(Event{Kind: "session_expired", Message: fmt.Sprintf("检测到会话失效，已停止自动化：%s", reason)})
	r.Stop()
}

// Done returns a channel that closes once the runner has fully stopped.
func (r *Runner) Done() <-chan struct{} { return r.done }
