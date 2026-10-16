package store

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

func TestDeviceIdentityConcurrentAllocationAndScope(t *testing.T) {
	db, owner, account, other, _ := notificationFixture(t)
	var wg sync.WaitGroup
	results := make(chan DeviceIdentity, 24)
	for range 24 {
		wg.Go(func() {
			identity, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, account.Channel, account.Username)
			if err != nil {
				t.Error(err)
				return
			}
			results <- identity
		})
	}
	wg.Wait()
	close(results)
	var first DeviceIdentity
	for identity := range results {
		if first.DeviceID == "" {
			first = identity
		}
		if identity != first {
			t.Fatal("concurrent allocation rotated identity")
		}
	}
	if first.DeviceID == "" || first.Source != "generated" || len(first.Summary()) != 12 || strings.Contains(first.Summary(), first.DeviceID) {
		t.Fatal("invalid identity diagnostics")
	}
	seen := map[string]bool{first.DeviceID: true}
	for _, scope := range []struct {
		owner             int64
		channel, username string
	}{
		{other.ID, "ios", account.Username}, {owner.ID, "alipay", account.Username}, {owner.ID, "ios", "another"},
	} {
		identity, err := db.EnsureDeviceIdentity(t.Context(), scope.owner, scope.channel, scope.username)
		if err != nil || seen[identity.DeviceID] {
			t.Fatal("identity crossed owner/channel/account", err)
		}
		seen[identity.DeviceID] = true
	}
}

func TestDeviceIdentityPendingBindingAndDeletion(t *testing.T) {
	db, owner, _, _, _ := notificationFixture(t)
	identity, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, "ios", "pending")
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.CreateAccountWithPolicy(t.Context(), owner.ID, "pending", "ios", "pending", "password", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	var bound int64
	if err := db.QueryRowContext(t.Context(), `SELECT account_id FROM login_device_identities WHERE user_id=? AND username='pending'`, owner.ID).Scan(&bound); err != nil || bound != account.ID {
		t.Fatal("creation did not bind identity", err)
	}
	if err := db.DeleteSession(t.Context(), account.ID); err != nil {
		t.Fatal(err)
	}
	again, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, "ios", "pending")
	if err != nil || again != identity {
		t.Fatal("session deletion rotated identity", err)
	}
	policy, err := db.LoadPolicyJSON(t.Context(), account.ID)
	if err != nil || policy != `{}` {
		t.Fatal("identity leaked into policy", err)
	}
	if err := db.RequestAccountDeletion(t.Context(), account.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, "ios", "pending"); !errors.Is(err, ErrAccountDeleting) {
		t.Fatal("deleting account admitted", err)
	}
	if _, err := db.CleanAccountDeletionBatch(t.Context(), account.ID, 250); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM login_device_identities WHERE user_id=? AND username='pending'`, owner.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("orphan bound identity", err)
	}
}

func TestDeviceIdentityPendingQuotaAndReclamation(t *testing.T) {
	db, owner, account, _, _ := notificationFixture(t)
	var first DeviceIdentity
	for i := range pendingDeviceLimit {
		identity, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, "ios", fmt.Sprint(i))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = identity
		}
	}
	if _, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, "ios", "overflow"); !errors.Is(err, ErrDeviceIdentityQuota) {
		t.Fatal("pending identities unbounded", err)
	}
	bound, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, account.Channel, account.Username)
	if err != nil {
		t.Fatal("pending quota blocked existing account", err)
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE login_device_identities SET touched_ms=0`); err != nil {
		t.Fatal(err)
	}
	again, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, "ios", "0")
	if err != nil || again != first {
		t.Fatal("active retry rotated identity at retention boundary", err)
	}
	if _, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, "ios", "overflow"); err != nil {
		t.Fatal("abandoned identities not reclaimed", err)
	}
	if again, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, account.Channel, account.Username); err != nil || again != bound {
		t.Fatal("bound identity expired", err)
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE users SET status='disabled' WHERE id=?`, owner.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, "ios", "0"); !errors.Is(err, ErrUserInactive) {
		t.Fatal("inactive owner admitted", err)
	}
}

func TestV20DeviceIdentityAdoptionAndRestart(t *testing.T) {
	for _, cache := range []string{"valid", "expired", "missing", "malformed"} {
		t.Run(cache, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "garden.db")
			db, err := Open(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			owner, err := db.CreateUser(t.Context(), "owner", "owner@test.invalid", "hash")
			if err != nil {
				t.Fatal(err)
			}
			account, err := db.CreateAccount(t.Context(), owner.ID, "account", "ios", "game", "secret")
			if err != nil {
				t.Fatal(err)
			}
			raw, err := babigame.MarshalSessionJSON(&babigame.Session{DeviceID: "existing-device", UUID: "launch", Session0: "session", GameLogin: babigame.GameLoginResult{Token: "token"}, AID: 1, GsIdx: 1, RouteToken: "route"})
			if err != nil {
				t.Fatal(err)
			}
			if cache == "malformed" {
				raw = []byte(`{`)
			}
			if cache != "missing" {
				var expiry *time.Time
				if cache == "expired" {
					past := time.Now().Add(-time.Hour)
					expiry = &past
				}
				if err := db.SaveSession(t.Context(), account.ID, raw, expiry); err != nil {
					t.Fatal(err)
				}
			}
			before := AccountRequestSafety{RestrictionCode: 5000, FreshLoginAttempts: 2, RestrictedUntilMS: 1234}
			if err := db.SaveAccountRestriction(t.Context(), account.ID, before); err != nil {
				t.Fatal(err)
			}
			// Reconstruct v19; test real Open ordering, including decryption.
			if _, err := db.ExecContext(t.Context(), `DROP TABLE login_device_identities; PRAGMA user_version=19`); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = Open(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			identity, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, "ios", "game")
			if err != nil {
				t.Fatal(err)
			}
			if cache == "valid" || cache == "expired" {
				if identity.DeviceID != "existing-device" || identity.Source != "session" {
					t.Fatal("migration rotated existing identity")
				}
			} else if identity.DeviceID == "" || identity.Source != "generated" {
				t.Fatal("missing identity not initialized")
			}
			if got, err := db.LoadAccountRequestSafety(t.Context(), account.ID); err != nil || got != before {
				t.Fatal("migration changed recovery safety", err)
			}
			if _, err := db.LoadSession(t.Context(), account.ID); err != nil {
				t.Fatal(err)
			}
			if err := db.DeleteSession(t.Context(), account.ID); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = Open(t.Context(), path)
			if err != nil {
				t.Fatal(err)
			}
			if again, err := db.EnsureDeviceIdentity(t.Context(), owner.ID, "ios", "game"); err != nil || again != identity {
				t.Fatal("restart rotated identity", err)
			}
			if version, err := databaseVersion(t.Context(), db.writer); err != nil || version != 20 {
				t.Fatal("migration version", version, err)
			}
		})
	}
}
