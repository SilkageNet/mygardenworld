package apiserver

import (
	"context"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

func (svc *Services) probeAccountIdentity(ctx context.Context, channel, username, password string) (session *babigame.Session, err error) {
	ctx, release, err := svc.beginGameWork(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	cfg, err := babigame.ConfigForChannel(babigame.Channel(channel))
	if err != nil {
		return nil, err
	}
	userID, err := requireUserID(ctx)
	if err != nil {
		return nil, err
	}
	finish, err := svc.identityProbes.begin(identityProbeKey{userID, channel, username}, time.Now())
	if err != nil {
		return nil, err
	}
	defer func() { finish(err, time.Now()) }()
	httpc, err := svc.loginHTTPClient(ctx, cfg, userID, channel, username)
	if err != nil {
		return nil, err
	}
	defer httpc.HTTPClient.CloseIdleConnections()
	if pkg, err := httpc.QueryPackageConfig(ctx); err == nil && pkg.GameVersion != "" {
		httpc.Cfg.GameVersion = pkg.GameVersion
		httpc.Cfg.ClientVersion = pkg.GameVersion
	}
	return babigame.PerformLoginWithPassword(ctx, httpc, username, password, 1)
}

// loginHTTPClient shares the durable identity lifecycle across native password
// probes and Alipay QR grants. Only the device identifier survives fresh login;
// package bootstrap still owns launch UUIDs and authentication session values.
func (svc *Services) loginHTTPClient(ctx context.Context, cfg babigame.Config, userID int64, channel, username string) (*babigame.HTTPClient, error) {
	identity, err := svc.DB.EnsureDeviceIdentity(ctx, userID, channel, username)
	if err != nil {
		return nil, mapErr(err)
	}
	if svc.Log != nil {
		svc.Log.Info("using durable device identity", "user_id", userID, "channel", channel, "source", identity.Source, "device_ref", identity.Summary())
	}
	return babigame.NewHTTPClient(cfg, identity.DeviceID, "", ""), nil
}

func (svc *Services) beginGameWork(ctx context.Context) (context.Context, func(), error) {
	if svc.Manager == nil {
		return ctx, func() {}, ctx.Err()
	}
	return svc.Manager.BeginGameWork(ctx)
}

func (svc *Services) saveLoginProbe(ctx context.Context, accountID int64, session *babigame.Session) {
	if session == nil {
		return
	}
	if blob, err := babigame.MarshalSessionJSON(session); err != nil {
		if svc.Log != nil {
			svc.Log.Warn("marshal probed login session failed", "account_id", accountID, "err", err)
		}
	} else if err := svc.DB.SaveSession(ctx, accountID, blob, nil); err != nil && svc.Log != nil {
		svc.Log.Warn("persist probed login session failed", "account_id", accountID, "err", err)
	}
	if err := svc.DB.UpdateLogin(ctx, accountID, session.AID, int32(session.GsIdx), session.WSURL(), time.Now().UTC()); err != nil && svc.Log != nil {
		svc.Log.Warn("persist probed login metadata failed", "account_id", accountID, "err", err)
	}
}
