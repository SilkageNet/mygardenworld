package apiserver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const alipayQRLoginTTL = 2 * time.Minute

type alipayLoginFlow struct {
	OwnerID   int64
	QRToken   string
	ExpiresAt time.Time
	Status    pb.AlipayLoginStatus
	Account   *store.Account
	Error     string
	UpdatedAt time.Time
}

type alipayLoginSnapshot struct {
	Status    pb.AlipayLoginStatus
	Account   *store.Account
	Error     string
	ExpiresAt time.Time
}

// AlipayLoginCoordinator owns short-lived QR state in memory. Raw QR tokens
// and the resulting PC web credential never enter logs or API responses.
type AlipayLoginCoordinator struct {
	provider babigame.AlipayAuthProvider
	now      func() time.Time

	mu    sync.Mutex
	flows map[string]*alipayLoginFlow
}

func NewAlipayLoginCoordinator(provider babigame.AlipayAuthProvider) *AlipayLoginCoordinator {
	return &AlipayLoginCoordinator{
		provider: provider,
		now:      time.Now,
		flows:    make(map[string]*alipayLoginFlow),
	}
}

func (c *AlipayLoginCoordinator) start(ctx context.Context, ownerID int64) (string, string, time.Time, error) {
	if c == nil || c.provider == nil {
		return "", "", time.Time{}, errors.New("alipay QR login is unavailable")
	}
	challenge, err := c.provider.BeginQR(ctx)
	if err != nil {
		return "", "", time.Time{}, err
	}
	now := c.now().UTC()
	loginID := babigame.RandomUUID()
	flow := &alipayLoginFlow{
		OwnerID:   ownerID,
		QRToken:   challenge.Token,
		ExpiresAt: now.Add(alipayQRLoginTTL),
		Status:    pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_WAITING_FOR_SCAN,
		UpdatedAt: now,
	}
	c.mu.Lock()
	c.cleanupLocked(now)
	c.flows[loginID] = flow
	c.mu.Unlock()
	return loginID, challenge.URL, flow.ExpiresAt, nil
}

func (c *AlipayLoginCoordinator) poll(ctx context.Context, ownerID int64, loginID string, createAccount func(context.Context, babigame.AlipayWebGrant) (*store.Account, error)) alipayLoginSnapshot {
	now := c.now().UTC()
	c.mu.Lock()
	flow := c.flows[loginID]
	if flow == nil || flow.OwnerID != ownerID {
		c.mu.Unlock()
		return alipayLoginSnapshot{Status: pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_FAILED, Error: "扫码登录会话不存在"}
	}
	if now.After(flow.ExpiresAt) && flow.Status != pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_COMPLETE {
		flow.Status = pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_EXPIRED
		flow.Error = "二维码已过期，请重新获取"
		flow.UpdatedAt = now
	}
	if flow.Status != pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_WAITING_FOR_SCAN {
		snapshot := snapshotAlipayFlow(flow)
		c.mu.Unlock()
		return snapshot
	}
	flow.Status = pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_PROCESSING
	flow.UpdatedAt = now
	qrToken := flow.QRToken
	c.mu.Unlock()

	grant, authorized, err := c.provider.PollQR(ctx, qrToken)
	if err != nil {
		if errors.Is(err, babigame.ErrAlipayQRExpired) {
			return c.finish(loginID, pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_EXPIRED, nil, "二维码已过期，请重新获取")
		}
		return c.finish(loginID, pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_FAILED, nil, formatLoginErr(err))
	}
	if !authorized {
		return c.finish(loginID, pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_WAITING_FOR_SCAN, nil, "")
	}
	account, err := createAccount(ctx, grant)
	if err != nil {
		return c.finish(loginID, pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_FAILED, nil, formatLoginErr(err))
	}
	return c.finish(loginID, pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_COMPLETE, account, "")
}

func (c *AlipayLoginCoordinator) finish(loginID string, status pb.AlipayLoginStatus, account *store.Account, message string) alipayLoginSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	flow := c.flows[loginID]
	if flow == nil {
		return alipayLoginSnapshot{Status: pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_FAILED, Error: "扫码登录会话不存在"}
	}
	flow.Status = status
	flow.Account = account
	flow.Error = message
	flow.UpdatedAt = c.now().UTC()
	if status == pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_COMPLETE || status == pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_FAILED || status == pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_EXPIRED {
		flow.QRToken = ""
	}
	return snapshotAlipayFlow(flow)
}

func (c *AlipayLoginCoordinator) cleanupLocked(now time.Time) {
	for id, flow := range c.flows {
		if now.Sub(flow.UpdatedAt) > 10*time.Minute {
			delete(c.flows, id)
		}
	}
}

func snapshotAlipayFlow(flow *alipayLoginFlow) alipayLoginSnapshot {
	return alipayLoginSnapshot{Status: flow.Status, Account: flow.Account, Error: flow.Error, ExpiresAt: flow.ExpiresAt}
}

func (svc *Services) StartAlipayLogin(ctx context.Context, req *connect.Request[pb.StartAlipayLoginRequest]) (*connect.Response[pb.StartAlipayLoginResponse], error) {
	if svc.AlipayLogins == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("alipay QR login is unavailable"))
	}
	userID := auth.UserIDFromContext(ctx)
	if userID <= 0 {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	loginID, qrContent, expiresAt, err := svc.AlipayLogins.start(ctx, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.StartAlipayLoginResponse{
		LoginId:   loginID,
		QrContent: qrContent,
		ExpiresAt: timestamppb.New(expiresAt),
		Status:    pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_WAITING_FOR_SCAN,
	}), nil
}

func (svc *Services) pollAlipayLogin(ctx context.Context, loginID string) (alipayLoginSnapshot, error) {
	if svc.AlipayLogins == nil {
		return alipayLoginSnapshot{}, errors.New("alipay QR login is unavailable")
	}
	loginID = strings.TrimSpace(loginID)
	if loginID == "" {
		return alipayLoginSnapshot{}, errors.New("login_id required")
	}
	userID := auth.UserIDFromContext(ctx)
	if userID <= 0 {
		return alipayLoginSnapshot{}, errors.New("not authenticated")
	}
	snapshot := svc.AlipayLogins.poll(ctx, userID, loginID, func(ctx context.Context, grant babigame.AlipayWebGrant) (*store.Account, error) {
		return svc.createAlipayAccount(ctx, userID, grant)
	})
	return snapshot, nil
}

func (svc *Services) createAlipayAccount(ctx context.Context, userID int64, grant babigame.AlipayWebGrant) (*store.Account, error) {
	if grant.UserID == "" || grant.Token == "" {
		return nil, errors.New("alipay grant missing user identity")
	}
	existing, err := svc.DB.GetAccountByChannelUsername(ctx, userID, string(babigame.ChannelAlipay), grant.UserID)
	if err != nil && !errors.Is(err, store.ErrAccountNotFound) {
		return nil, err
	}
	if existing == nil {
		if err := svc.checkAccountQuota(ctx, userID); err != nil {
			return nil, err
		}
	}
	cfg, err := babigame.ConfigForChannel(babigame.ChannelAlipay)
	if err != nil {
		return nil, err
	}
	gameHTTP := babigame.NewHTTPClient(cfg, "", "", "")
	if pkg, err := gameHTTP.QueryPackageConfig(ctx); err == nil && pkg.GameVersion != "" {
		gameHTTP.Cfg.GameVersion = pkg.GameVersion
		gameHTTP.Cfg.ClientVersion = pkg.GameVersion
	}
	session, err := svc.AlipayLogins.provider.LoginWithWebGrant(ctx, gameHTTP, grant)
	if err != nil {
		return nil, fmt.Errorf("verify Alipay game login: %w", err)
	}
	if existing != nil {
		if err := svc.DB.UpdateAccountCredentials(ctx, existing.ID, grant.UserID, grant.Token); err != nil {
			return nil, err
		}
		svc.saveLoginProbe(ctx, existing.ID, session)
		r, startErr := svc.Manager.ReloadWithSource(ctx, existing.ID, runner.StartSourceAlipayLogin)
		if startErr != nil {
			return nil, fmt.Errorf("restart Alipay account: %w", startErr)
		}
		if err := svc.enableAutomation(ctx, existing.ID, r); err != nil {
			return nil, fmt.Errorf("enable Alipay account automation: %w", err)
		}
		return r.Account(), nil
	}
	name, err := svc.DB.UniqueAccountName(ctx, userID, 0, babigame.DisplayNameFromSession(session, "Alipay 账号"))
	if err != nil {
		return nil, err
	}
	account, err := svc.DB.CreateAccount(ctx, userID, name, string(babigame.ChannelAlipay), grant.UserID, grant.Token)
	if err != nil {
		return nil, err
	}
	svc.saveLoginProbe(ctx, account.ID, session)
	r, err := svc.Manager.StartWithSource(ctx, account.ID, runner.StartSourceAlipayLogin)
	if err != nil {
		return nil, fmt.Errorf("start Alipay account: %w", err)
	}
	if err := svc.enableAutomation(ctx, account.ID, r); err != nil {
		return nil, fmt.Errorf("enable Alipay account automation: %w", err)
	}
	return r.Account(), nil
}

func (svc *Services) checkAccountQuota(ctx context.Context, userID int64) error {
	user, err := svc.DB.GetUserByID(ctx, userID)
	if err != nil {
		return mapErr(err)
	}
	count, err := svc.DB.CountAccountsByUser(ctx, userID)
	if err != nil {
		return mapErr(err)
	}
	if count >= user.MaxAccounts {
		return connect.NewError(connect.CodeResourceExhausted, fmt.Errorf("account quota reached (%d/%d)", count, user.MaxAccounts))
	}
	return nil
}

func alipayTimestampOrNil(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
