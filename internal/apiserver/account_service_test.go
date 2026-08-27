package apiserver

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestDisconnectAccountDisablesAutomationPreference(t *testing.T) {
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
	acc, err := db.CreateAccount(ctx, user.ID, "main", "ios", "game", "pw")
	if err != nil {
		t.Fatal(err)
	}
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	raw, err := policycfg.ToJSON(policy)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SavePolicyJSON(ctx, acc.ID, raw); err != nil {
		t.Fatal(err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := &Services{
		DB:      db,
		Manager: runner.NewManager(db, runner.NewBus(), log),
		Log:     log,
	}
	userCtx := auth.ContextWithIdentity(ctx, &auth.Identity{UserID: user.ID, Role: "user"})
	_, err = svc.DisconnectAccount(userCtx, connect.NewRequest(&pb.DisconnectAccountRequest{Id: acc.ID}))
	if err != nil {
		t.Fatal(err)
	}

	stored, err := db.LoadPolicyJSON(ctx, acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, err := policycfg.FromJSON(stored)
	if err != nil {
		t.Fatal(err)
	}
	if got.GetAutomationEnabled() {
		t.Fatal("automation_enabled=true after DisconnectAccount, want false")
	}
}
