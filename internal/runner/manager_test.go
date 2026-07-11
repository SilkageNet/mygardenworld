package runner

import (
	"context"
	"io"
	"log/slog"
	"math"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
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

func TestGenericSessionInvalidationDisablesAutomationRestorePreference(t *testing.T) {
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
	r := New(babigame.Config{}, db, acc, NewBus(), log)
	r.SetPolicy(policy)
	r.markSessionInvalidated("会话已过期，请重新登录")

	if r.Policy().GetAutomationEnabled() {
		t.Fatal("live policy automation_enabled=true after session invalidation, want false")
	}
	stored, err := db.LoadPolicyJSON(ctx, acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	gotPolicy, err := policycfg.FromJSON(stored)
	if err != nil {
		t.Fatal(err)
	}
	if gotPolicy.GetAutomationEnabled() {
		t.Fatal("persisted automation_enabled=true after session invalidation, want false")
	}

	mgr := NewManager(db, NewBus(), log)
	got, err := mgr.accountsWithAutomationEnabled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("accountsWithAutomationEnabled()=%+v, want no restored accounts after session invalidation", got)
	}
}

func TestDisplacedSessionKeepsAutomationEnabledForDelayedRelogin(t *testing.T) {
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
	policy.Basic.ReconnectIntervalSeconds = 17
	raw, err := policycfg.ToJSON(policy)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SavePolicyJSON(ctx, acc.ID, raw); err != nil {
		t.Fatal(err)
	}

	r := New(babigame.Config{}, db, acc, NewBus(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.SetPolicy(policy)
	r.markSessionInvalidated("账号已在其他设备登录，当前会话被替换")

	if !r.autoReloginPending() {
		t.Fatal("auto relogin was not scheduled for an explicit displaced-session reason")
	}
	if got := r.reloginInterval(); got != 17*time.Second {
		t.Fatalf("reloginInterval()=%s, want 17s", got)
	}
	if !r.Policy().GetAutomationEnabled() {
		t.Fatal("live policy automation_enabled=false after displacement, want preserved")
	}
	stored, err := db.LoadPolicyJSON(ctx, acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	gotPolicy, err := policycfg.FromJSON(stored)
	if err != nil {
		t.Fatal(err)
	}
	if !gotPolicy.GetAutomationEnabled() {
		t.Fatal("persisted automation_enabled=false after displacement, want preserved")
	}
	select {
	case <-r.Done():
		t.Fatal("runner stopped instead of waiting for delayed relogin")
	default:
	}
}

func TestDelayedReloginBackoffAndCancellation(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   float64
		want time.Duration
	}{
		{name: "nan defaults safely", in: math.NaN(), want: defaultReloginWait},
		{name: "infinity defaults safely", in: math.Inf(1), want: defaultReloginWait},
		{name: "subsecond clamps", in: 0.1, want: time.Second},
		{name: "huge clamps", in: 1e30, want: maxReloginWait},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{policy: &pb.Policy{Basic: &pb.BasicPolicy{ReconnectIntervalSeconds: tt.in}}}
			if got := r.reloginInterval(); got != tt.want {
				t.Fatalf("reloginInterval()=%s, want %s", got, tt.want)
			}
		})
	}

	if got := nextReloginWait(time.Second, time.Second); got != 2*time.Second {
		t.Fatalf("nextReloginWait(1s, 1s)=%s, want 2s", got)
	}
	if got := nextReloginWait(16*time.Second, time.Second); got != 30*time.Second {
		t.Fatalf("nextReloginWait(16s, 1s)=%s, want 30s cap", got)
	}
	if got := nextReloginWait(5*time.Minute, 5*time.Minute); got != 5*time.Minute {
		t.Fatalf("nextReloginWait(5m, 5m)=%s, want configured interval cap", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if sleepOrDone(ctx, time.Hour) {
		t.Fatal("sleepOrDone returned true after cancellation")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cancelled timer took %s to return", elapsed)
	}
}
