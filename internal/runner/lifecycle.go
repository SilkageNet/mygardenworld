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

var errWebSocketSessionStart = errors.New("websocket session start failed")

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
	r.hydratePearlHireTicketUsage(ctx, time.Now())
	client, err := r.connectStoredOrFresh(ctx, username, password)
	if err != nil {
		if r.autoReloginPending() {
			go r.decisionLoop(rctx)
			go r.connectionLoop(rctx, username, password, nil)
			return nil
		}
		if errors.Is(err, errWebSocketSessionStart) {
			// Credentials and the HTTP login path were good enough to reach the
			// game WebSocket. Keep the runner alive so transient DNS, gateway, or
			// handshake failures recover with the normal reconnect backoff.
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

// connectStoredOrFresh first tries the encrypted session captured during
// account creation or the previous successful login. A rejected/corrupt cache
// is deleted and the channel-specific fresh login becomes the fallback.
func (r *Runner) connectStoredOrFresh(ctx context.Context, username, password string) (*babigame.Client, error) {
	blob, err := r.db.LoadSession(ctx, r.account.ID)
	if err != nil {
		r.log.Warn("load cached session failed; using fresh login", "err", err)
		if deleteErr := r.db.DeleteSession(ctx, r.account.ID); deleteErr != nil {
			r.log.Warn("delete unreadable cached session failed", "err", deleteErr)
		}
	} else if len(blob) > 0 {
		session, decodeErr := babigame.UnmarshalSessionJSON(blob, r.cfg)
		if decodeErr == nil {
			httpc := r.prepareHTTPClient(ctx, session.DeviceID, session.UUID, session.Session0)
			session.Cfg = httpc.Cfg
			client, resumeErr := r.connectSession(ctx, httpc, session, true)
			if resumeErr == nil {
				return client, nil
			}
			// A transport outage does not prove that the cached route token is
			// invalid. Preserve it for the reconnect loop instead of replacing a
			// reusable session with repeated fresh HTTP logins.
			if errors.Is(resumeErr, errWebSocketSessionStart) {
				return nil, resumeErr
			}
			decodeErr = resumeErr
		}
		r.log.Info("cached session rejected; using fresh login", "err", decodeErr)
		if deleteErr := r.db.DeleteSession(ctx, r.account.ID); deleteErr != nil {
			r.log.Warn("delete rejected cached session failed", "err", deleteErr)
		}
	}
	return r.connectFresh(ctx, username, password)
}

func (r *Runner) connectFresh(ctx context.Context, username, password string) (*babigame.Client, error) {
	httpc := r.prepareHTTPClient(ctx, "", "", "")
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
	return r.connectSession(ctx, httpc, session, false)
}

func (r *Runner) prepareHTTPClient(ctx context.Context, deviceID, uuid, session0 string) *babigame.HTTPClient {
	httpc := babigame.NewHTTPClient(r.cfg, deviceID, uuid, session0)
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
	return httpc
}

func (r *Runner) connectSession(ctx context.Context, httpc *babigame.HTTPClient, session *babigame.Session, resume bool) (*babigame.Client, error) {
	client := babigame.NewClient(session)
	client.DebugWriter = r.debugWriter
	if err := client.Connect(ctx); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("%w: ws connect: %w", errWebSocketSessionStart, err)
	}

	// A cached route token resumes with index.reLogin; a freshly issued route
	// token uses index.login, matching the official client lifecycle.
	r.state.BeginFmlMembershipSnapshot()
	var (
		v   json.RawMessage
		err error
	)
	if resume {
		v, err = client.ReLogin(ctx, r.cfg.IsSimulator)
	} else {
		v, err = client.Login(ctx, r.cfg.IsSimulator)
	}
	if err == nil {
		r.state.ApplyV(v)
		r.syncAccountDisplayName(ctx, v, session)
	} else {
		_ = client.Close()
		return nil, fmt.Errorf("启动%s失败: %w", map[bool]string{true: "恢复登录", false: "登录"}[resume], err)
	}

	// Do not install invalidation callbacks until a cached session has proved
	// usable. Otherwise an expired cache could stop the runner before its fresh
	// login fallback gets a chance to run.
	r.attachClientHandlers(client)
	r.resetFreshSessionAutomationState()
	r.mu.Lock()
	r.session = session
	r.httpc = httpc
	r.client = client
	r.rqst = rqstState{}
	r.mu.Unlock()

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
	// index.login + lazySync form the startup membership baseline. Some channel
	// fronts omit IFmlTot.mb (25.1) for joined accounts, so finalization also
	// accepts the guild ID in IFmlTot.fml (25.0) as positive membership evidence.
	r.state.FinalizeFmlMembershipSnapshot()
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
	// The giftbag namespace is lazy-loaded by the official client. Refresh it
	// once per connection so the manual cooldown/quota reminder is accurate
	// without turning the four-second automation loop into a sync poller.
	r.syncGiftbagState(ctx, client, session)
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

	if blob, err := babigame.MarshalSessionJSON(session); err != nil {
		r.log.Warn("marshal login session failed", "err", err)
	} else if err := r.db.SaveSession(ctx, r.account.ID, blob, nil); err != nil {
		r.log.Warn("persist login session failed", "err", err)
	}
	if err := r.db.UpdateLogin(ctx, r.account.ID, session.AID, int32(session.GsIdx), session.WSURL(), time.Now().UTC()); err != nil {
		r.log.Warn("persist login metadata failed", "err", err)
	}
	message := "已连接"
	if resume {
		message = "已恢复缓存会话"
	}
	event := Event{Kind: "session"}
	if source := r.consumeStartSource(); source != StartSourceUnspecified {
		payload, _ := json.Marshal(map[string]any{"start_source": source})
		event.PayloadJSON = string(payload)
		event.Message = fmt.Sprintf("%s（来源=%s，服务器=%s 区=%d）", message, startSourceLabel(source), session.GsHost, session.GsIdx)
	} else {
		event.Message = fmt.Sprintf("%s (服务器=%s 区=%d)", message, session.GsHost, session.GsIdx)
	}
	r.emit(event)
	return client, nil
}

func (r *Runner) consumeStartSource() StartSource {
	r.mu.Lock()
	defer r.mu.Unlock()
	source := r.startSource
	r.startSource = StartSourceUnspecified
	return source
}

func startSourceLabel(source StartSource) string {
	switch source {
	case StartSourceDaemonRestore:
		return "服务启动恢复"
	case StartSourceAccountCreate:
		return "新增账号"
	case StartSourceControlPanel:
		return "控制面板"
	case StartSourceAutomationEnable:
		return "启用自动化"
	case StartSourceManualOperation:
		return "手动操作"
	case StartSourceAlipayLogin:
		return "支付宝授权"
	case StartSourceRedeemAutoConnect:
		return "兑换码自动上线"
	default:
		return "未标记"
	}
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
	r.resetResidentOrderSession()
	r.mu.Lock()
	clear(r.cultivateUpgradeRejects)
	r.mu.Unlock()
	if r.state != nil {
		// Contest window: every login/reconnect must re-fetch the task pool
		// before farm/order work so takeable rows are claimed immediately.
		r.state.MarkFmlRaceTaskPoolStale()
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

func (r *Runner) attachClientHandlers(client *babigame.Client) {
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
