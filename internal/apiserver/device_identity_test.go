package apiserver

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestLoginClientReusesDeviceAcrossServiceRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "garden.db")
	db, err := store.Open(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	owner, err := db.CreateUser(t.Context(), "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	for _, channel := range []string{"ios", "alipay"} {
		t.Run(channel, func(t *testing.T) {
			cfg, err := babigame.ConfigForChannel(babigame.Channel(channel))
			if err != nil {
				t.Fatal(err)
			}
			svc := &Services{DB: db}
			first, err := svc.loginHTTPClient(t.Context(), cfg, owner.ID, channel, "player")
			if err != nil {
				t.Fatal(err)
			}
			defer first.HTTPClient.CloseIdleConnections()
			// Admission guard is process-local; device identity must not be.
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = store.Open(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			svc = &Services{DB: db}
			next, err := svc.loginHTTPClient(t.Context(), cfg, owner.ID, channel, "player")
			if err != nil {
				t.Fatal(err)
			}
			defer next.HTTPClient.CloseIdleConnections()
			if next.DeviceID != first.DeviceID || next.UUID == first.UUID || next.Session0 == first.Session0 {
				t.Fatal("incorrect device/authentication lifetimes")
			}
		})
	}
}

type failingIdentityAlipayProvider struct {
	fakeAlipayProvider
	deviceIDs []string
}

func (p *failingIdentityAlipayProvider) LoginWithWebGrant(_ context.Context, client *babigame.HTTPClient, _ babigame.AlipayWebGrant) (*babigame.Session, error) {
	p.deviceIDs = append(p.deviceIDs, client.DeviceID)
	return nil, errors.New("fixture authentication failure")
}

func TestAlipayReauthorizationFailurePreservesDevice(t *testing.T) {
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	owner, err := db.CreateUser(t.Context(), "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.CreateAccount(t.Context(), owner.ID, "player", "alipay", "player", "old-grant")
	if err != nil {
		t.Fatal(err)
	}
	identity, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, "alipay", "player")
	if err != nil {
		t.Fatal(err)
	}
	provider := &failingIdentityAlipayProvider{}
	ctx := auth.ContextWithIdentity(t.Context(), &auth.Identity{UserID: owner.ID})
	for range 3 {
		svc := &Services{DB: db, AlipayLogins: NewAlipayLoginCoordinator(provider)}
		if _, err := svc.createAlipayAccount(ctx, owner.ID, babigame.AlipayWebGrant{UserID: "player", Token: "new-grant"}, alipayLoginOptions{AccountID: account.ID}); err == nil {
			t.Fatal("fixture failure ignored")
		}
	}
	if len(provider.deviceIDs) != 3 {
		t.Fatal("grant provider not reached")
	}
	for _, deviceID := range provider.deviceIDs {
		if deviceID != identity.DeviceID {
			t.Fatal("QR reauthorization rotated identity")
		}
	}
}
