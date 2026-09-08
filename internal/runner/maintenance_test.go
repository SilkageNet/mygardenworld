package runner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func maintenanceTestPolicy(t *testing.T, enabled bool) string {
	t.Helper()
	p := automation.DefaultPolicy()
	p.AutomationEnabled = enabled
	raw, err := policycfg.ToJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func maintenanceTestManager(t *testing.T) (*Manager, *store.DB) {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(db, NewBus(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { m.Shutdown(); _ = db.Close() })
	return m, db
}

func TestMaintenanceDrainsBeforeAcknowledgementAndBlocksEveryStartSource(t *testing.T) {
	m, db := maintenanceTestManager(t)
	ctx := context.Background()
	workCtx, release, err := m.BeginGameWork(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	request, err := db.RequestMaintenance(ctx, true, false)
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	go func() { finished <- m.ApplyMaintenance(ctx) }()
	select {
	case <-workCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("in-flight work not cancelled")
	}
	s, err := db.Maintenance(ctx)
	if err != nil || s.AppliedRevision == request.Revision {
		t.Fatalf("premature acknowledgement: %+v %v", s, err)
	}
	if !m.MaintenanceStatus().Draining {
		t.Fatal("missing draining status")
	}
	for _, source := range []StartSource{StartSourceUnspecified, StartSourceDaemonRestore, StartSourceAccountCreate, StartSourceControlPanel, StartSourceAutomationEnable, StartSourceManualOperation, StartSourceAlipayLogin, StartSourceRedeemAutoConnect} {
		if _, err := m.StartWithSource(ctx, 999, source); !errors.Is(err, ErrMaintenance) {
			t.Fatalf("source %s: %v", source, err)
		}
	}
	if _, err := m.ReloadWithSource(ctx, 999, StartSourceControlPanel); !errors.Is(err, ErrMaintenance) {
		t.Fatal(err)
	}
	release()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	s, _ = db.Maintenance(ctx)
	if s.AppliedRevision != request.Revision || m.MaintenanceStatus().Draining {
		t.Fatalf("not drained: %+v", s)
	}

	// Default exit does not silently recreate sessions through redeem AUTO.
	if _, err := db.RequestMaintenance(ctx, false, false); err != nil {
		t.Fatal(err)
	}
	if err := m.ApplyMaintenance(ctx); err != nil {
		t.Fatal(err)
	}
	if m.MaintenanceStatus().Enabled || m.BackgroundStartsAllowed() {
		t.Fatal("default exit semantics wrong")
	}
	if _, err := m.StartWithSource(ctx, 999, StartSourceRedeemAutoConnect); !errors.Is(err, ErrManualResumeRequired) {
		t.Fatal(err)
	}
	if _, err := m.StartWithSource(ctx, 999, StartSourceControlPanel); errors.Is(err, ErrMaintenance) || errors.Is(err, ErrManualResumeRequired) {
		t.Fatal("owner cannot reconnect")
	}
}

func TestMaintenanceSurvivesRestartWithoutChangingPolicies(t *testing.T) {
	m, db := maintenanceTestManager(t)
	ctx := context.Background()
	u, err := db.CreateUser(ctx, "owner", "owner@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccountWithPolicy(ctx, u.ID, "account", "ios", "u", "p", maintenanceTestPolicy(t, true))
	if err != nil {
		t.Fatal(err)
	}
	before, _ := db.LoadPolicyJSON(ctx, a.ID)
	if _, err := db.RequestMaintenance(ctx, true, false); err != nil {
		t.Fatal(err)
	}
	// Even before the first poll, a newly constructed manager fails closed.
	restarted := NewManager(db, NewBus(), m.log)
	defer restarted.Shutdown()
	if !restarted.MaintenanceStatus().Enabled {
		t.Fatal("startup ignored durable gate")
	}
	if got := restarted.RestoreEnabledRunners(ctx); got.Started != 0 || got.Failed != 0 {
		t.Fatalf("restore attempted: %+v", got)
	}
	if err := restarted.ApplyMaintenance(ctx); err != nil {
		t.Fatal(err)
	}
	after, _ := db.LoadPolicyJSON(ctx, a.ID)
	if after != before {
		t.Fatal("maintenance changed user policy")
	}
}

func TestGameGateConcurrentCancellationAndIdempotentRelease(t *testing.T) {
	var g gameGate
	var wg sync.WaitGroup
	var admitted sync.WaitGroup
	admitted.Add(20)
	for range 20 {
		wg.Go(func() {
			ctx, done, err := g.begin(context.Background())
			if err != nil {
				t.Error(err)
				admitted.Done()
				return
			}
			admitted.Done()
			<-ctx.Done()
			done()
			done()
		})
	}
	admitted.Wait()
	g.block()
	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := g.wait(waitCtx); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if _, _, err := g.begin(context.Background()); !errors.Is(err, ErrMaintenance) {
		t.Fatal(err)
	}
	g.open()
	_, done, err := g.begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	done()
}

func TestMaintenanceTimeoutLeavesGateClosedAndCommandUnacknowledged(t *testing.T) {
	m, db := maintenanceTestManager(t)
	_, release, _ := m.BeginGameWork(context.Background())
	defer release()
	s, err := db.RequestMaintenance(context.Background(), true, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := m.ApplyMaintenance(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	current, _ := db.Maintenance(context.Background())
	if !m.MaintenanceStatus().Enabled || current.AppliedRevision == s.Revision {
		t.Fatal("failed drain opened gate or acknowledged")
	}
	release()
	if err := m.ApplyMaintenance(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRestorationRechecksPolicyBeforeLogin(t *testing.T) {
	m, db := maintenanceTestManager(t)
	ctx := context.Background()
	u, err := db.CreateUser(ctx, "owner", "owner@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(ctx, u.ID, "disabled", "ios", "test-only", "test-only")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SavePolicyJSON(ctx, a.ID, maintenanceTestPolicy(t, false)); err != nil {
		t.Fatal(err)
	}
	if _, err := m.StartWithSource(ctx, a.ID, StartSourceDaemonRestore); !errors.Is(err, errRestoreIneligible) {
		t.Fatalf("disabled policy reached login: %v", err)
	}
}
