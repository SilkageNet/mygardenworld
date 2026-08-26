package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
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
	var (
		session *babigame.Session
		err     error
	)
	switch babigame.Channel(r.account.Channel) {
	case babigame.ChannelIOS:
		session, err = babigame.PerformLoginWithPassword(ctx, httpc, username, password, r.cfg.IsSimulator)
	case babigame.ChannelAlipay:
		session, err = babigame.NewAlipayClient(r.cfg).LoginWithWebGrant(ctx, httpc, babigame.AlipayWebGrant{
			Token:  password,
			UserID: username,
		})
	default:
		err = fmt.Errorf("unsupported channel %q", r.account.Channel)
	}
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}

	if blob, err := babigame.MarshalSessionJSON(session); err != nil {
		r.log.Warn("marshal login session failed", "err", err)
	} else if err := r.db.SaveSession(ctx, r.account.ID, blob, nil); err != nil {
		r.log.Warn("persist login session failed", "err", err)
	}
	now := time.Now().UTC()
	if err := r.db.UpdateLogin(ctx, r.account.ID, session.AID, int32(session.GsIdx), session.WSURL(), now); err != nil {
		r.log.Warn("persist login metadata failed", "err", err)
	}

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
	if v, err := client.Login(ctx, r.cfg.IsSimulator); err == nil {
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
	} else {
		r.log.Warn("ws lazy sync failed", "err", err)
	}
	// index.login + lazySync form the authoritative startup baseline. If neither
	// supplied IFmlTot.mb (25.1), this account has no current guild membership;
	// stale IFml/race records must not wake any guild planner.
	r.state.FinalizeFmlMembershipSnapshot()
	// Only now may the shadow controller observe activity state. During a
	// reconnecting fresh login the State still contains the previous epoch's
	// board until index.login/lazySync have supplied this epoch's baseline.
	r.markDessertSessionStateReady()
	if r.isSessionInvalidated() {
		_ = client.Close()
		r.clearDisconnectedClient(client)
		return nil, r.sessionInvalidatedError("session invalidated during startup")
	}
	// Refresh satin/decorate order slots + daily counters after every
	// start/reconnect so a user-watched ad or midnight reset is observed.
	r.syncResidentOrderState(ctx, client, session, "startup")
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
	r.resetResidentOrderSession()
	if r.state != nil {
		r.state.ResetDessertSession()
		// Contest window: every login/reconnect must re-fetch the task pool
		// before farm/order work so takeable rows are claimed immediately.
		r.state.MarkFmlRaceTasksUnobserved()
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
