package apiserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestMaintenanceBlocksIdentityProbeAndQRBeforeExternalIO(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.RequestMaintenance(ctx, true, false); err != nil {
		t.Fatal(err)
	}
	m := runner.NewManager(db, runner.NewBus(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer m.Shutdown()
	provider := &fakeAlipayProvider{}
	svc := &Services{DB: db, Manager: m, AlipayLogins: NewAlipayLoginCoordinator(provider)}
	if _, err := svc.probeAccountIdentity(ctx, "ios", "not-a-real-user", "not-a-real-password"); !errors.Is(err, runner.ErrMaintenance) {
		t.Fatal(err)
	}
	if _, err := svc.StartAlipayLogin(ctx, connect.NewRequest(&pb.StartAlipayLoginRequest{})); connect.CodeOf(err) != connect.CodeUnavailable {
		t.Fatal(err)
	}
	if _, err := svc.pollAlipayLogin(ctx, "unused"); !errors.Is(err, runner.ErrMaintenance) {
		t.Fatal(err)
	}
	if provider.polls != 0 || len(svc.AlipayLogins.flows) != 0 {
		t.Fatal("maintenance reached QR provider")
	}
	view := svc.maintenanceView()
	if !view.Enabled || view.Draining {
		t.Fatal("wrong maintenance view")
	}
	if got := view.ProtoReflect().Descriptor().Fields().Len(); got != 2 {
		t.Fatalf("maintenance view must expose only enabled/draining, got %d fields", got)
	}
}
