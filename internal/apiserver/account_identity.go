package apiserver

import (
	"context"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

func (svc *Services) probeAccountIdentity(ctx context.Context, channel, username, password string) (*babigame.Session, error) {
	cfg, err := babigame.ConfigForChannel(babigame.Channel(channel))
	if err != nil {
		return nil, err
	}
	httpc := babigame.NewHTTPClient(cfg, "", "", "")
	if pkg, err := httpc.QueryPackageConfig(ctx); err == nil && pkg.GameVersion != "" {
		httpc.Cfg.GameVersion = pkg.GameVersion
		httpc.Cfg.ClientVersion = pkg.GameVersion
	}
	return babigame.PerformLoginWithPassword(ctx, httpc, username, password, 1)
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
