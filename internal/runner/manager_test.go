package runner

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestAccountsWithAutomationEnabledUsesPersistedPolicy(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	enabled, err := db.CreateAccount(ctx, user.ID, "enabled", "ios", "game1", "pw1")
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := db.CreateAccount(ctx, user.ID, "disabled", "ios", "game2", "pw2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateAccount(ctx, user.ID, "default", "ios", "game3", "pw3"); err != nil {
		t.Fatal(err)
	}

	enabledPolicy := automation.DefaultPolicy()
	enabledPolicy.AutomationEnabled = true
	enabledRaw, err := policycfg.ToJSON(enabledPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SavePolicyJSON(ctx, enabled.ID, enabledRaw); err != nil {
		t.Fatal(err)
	}

	disabledPolicy := automation.DefaultPolicy()
	disabledRaw, err := policycfg.ToJSON(disabledPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SavePolicyJSON(ctx, disabled.ID, disabledRaw); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(db, NewBus(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	got, err := mgr.accountsWithAutomationEnabled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != enabled.ID {
		t.Fatalf("accountsWithAutomationEnabled()=%+v, want only account %d", got, enabled.ID)
	}
}
