package runner

import (
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

func TestFreshLoginKeepsDeviceNotAuthenticationSession(t *testing.T) {
	for _, channel := range []string{"ios", "alipay"} {
		t.Run(channel, func(t *testing.T) {
			_, r, _ := startupCommitFixture(t)
			if _, err := r.db.ExecContext(t.Context(), `UPDATE accounts SET channel=? WHERE id=?`, channel, r.account.ID); err != nil {
				t.Fatal(err)
			}
			r.account.Channel = channel
			// Disable native package fetching in this factory-only test. No game
			// network is used; real channel bootstrap is tested in babigame.
			r.cfg = babigame.Config{}
			first, err := r.prepareFreshHTTPClient(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer first.HTTPClient.CloseIdleConnections()
			if err := r.db.DeleteSession(t.Context(), r.account.ID); err != nil {
				t.Fatal(err)
			}
			for range 3 {
				next, err := r.prepareFreshHTTPClient(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				next.HTTPClient.CloseIdleConnections()
				if next.DeviceID != first.DeviceID {
					t.Fatal("fresh retry rotated device")
				}
				if next.UUID == first.UUID || next.Session0 == first.Session0 {
					t.Fatal("fresh retry reused authentication lifecycle values")
				}
			}
		})
	}
}
