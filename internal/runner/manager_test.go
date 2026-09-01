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

func TestStartSourceIsConsumedOnceAndHasOperatorLabel(t *testing.T) {
	sources := []StartSource{
		StartSourceDaemonRestore,
		StartSourceAccountCreate,
		StartSourceControlPanel,
		StartSourceAutomationEnable,
		StartSourceManualOperation,
		StartSourceAlipayLogin,
		StartSourceRedeemAutoConnect,
	}
	for _, source := range sources {
		if label := startSourceLabel(source); label == "" || label == "未标记" {
			t.Fatalf("start source %q label=%q", source, label)
		}
	}
	r := &Runner{startSource: StartSourceRedeemAutoConnect}
	if got := r.consumeStartSource(); got != StartSourceRedeemAutoConnect {
		t.Fatalf("consumeStartSource()=%q, want %q", got, StartSourceRedeemAutoConnect)
	}
	if got := r.consumeStartSource(); got != StartSourceUnspecified {
		t.Fatalf("second consumeStartSource()=%q, want empty", got)
	}
}

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
	alipay, err := db.CreateAccount(ctx, user.ID, "alipay", "alipay", "game4", "grant")
	if err != nil {
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
	if err := db.SavePolicyJSON(ctx, alipay.ID, enabledRaw); err != nil {
		t.Fatal(err)
	}
	disabledOwner, err := db.CreateUser(ctx, "disabled-owner", "disabled@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	disabledStatus := "disabled"
	if _, err := db.UpdateUser(ctx, disabledOwner.ID, nil, nil, &disabledStatus); err != nil {
		t.Fatal(err)
	}
	disabledOwnerAccount, err := db.CreateAccount(ctx, disabledOwner.ID, "disabled-owner-account", "ios", "game5", "pw5")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SavePolicyJSON(ctx, disabledOwnerAccount.ID, enabledRaw); err != nil {
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
	if len(got) != 2 || got[0].ID != enabled.ID || got[1].ID != alipay.ID {
		t.Fatalf("accountsWithAutomationEnabled()=%+v, want accounts %d and %d", got, enabled.ID, alipay.ID)
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
	policy.Basic.DisplacedSessionReloginEnabled = true
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
	policy.Basic.DisplacedSessionReloginEnabled = true
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

func TestDisplacedSessionReloginDisabledUsesFailClosedInvalidation(t *testing.T) {
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
	if policy.GetBasic().GetDisplacedSessionReloginEnabled() {
		t.Fatal("default displaced-session relogin setting=true, want false")
	}
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

	if r.autoReloginPending() {
		t.Fatal("auto relogin was scheduled while displaced-session relogin was disabled")
	}
	if r.Policy().GetAutomationEnabled() {
		t.Fatal("live policy automation_enabled=true after fail-closed displacement, want false")
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
		t.Fatal("persisted automation_enabled=true after fail-closed displacement, want false")
	}
	select {
	case <-r.Done():
	default:
		t.Fatal("runner kept running while displaced-session relogin was disabled")
	}
}

func TestDisplacedSessionReloginDoesNotDependOnAutomationEnabled(t *testing.T) {
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = false
	policy.Basic.DisplacedSessionReloginEnabled = true
	r := newSessionLifecycleTestRunner(policy)

	r.markSessionInvalidated("账号已在其他设备登录，当前会话被替换")

	if !r.autoReloginPending() {
		t.Fatal("displaced-session relogin was not scheduled while automation_enabled=false")
	}
	if r.Policy().GetAutomationEnabled() {
		t.Fatal("displaced-session lifecycle unexpectedly enabled automation")
	}
	select {
	case <-r.Done():
		t.Fatal("runner stopped instead of waiting for delayed relogin")
	default:
	}
}

func TestDisablingDisplacedSessionReloginCancelsPendingWait(t *testing.T) {
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.DisplacedSessionReloginEnabled = true
	r := newSessionLifecycleTestRunner(policy)
	r.markSessionInvalidated("账号已在其他设备登录，当前会话被替换")

	updated := r.Policy()
	updated.Basic.DisplacedSessionReloginEnabled = false
	r.SetPolicy(updated)

	if r.autoReloginPending() {
		t.Fatal("auto relogin remained pending after its switch was disabled")
	}
	if r.Policy().GetAutomationEnabled() {
		t.Fatal("fail-closed cancellation left automation enabled")
	}
	select {
	case <-r.Done():
	default:
		t.Fatal("disabling the switch did not stop the pending runner immediately")
	}
}

func TestDisablingDisplacedSessionReloginCancelsRetryWait(t *testing.T) {
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.DisplacedSessionReloginEnabled = true
	r := newSessionLifecycleTestRunner(policy)
	r.markSessionInvalidated("账号已在其他设备登录，当前会话被替换")
	if !r.beginAutoReloginAttempt() {
		t.Fatal("could not enter displaced-session relogin attempt")
	}
	if r.autoReloginPending() {
		t.Fatal("initial pending state remained after the relogin attempt began")
	}

	updated := r.Policy()
	updated.Basic.DisplacedSessionReloginEnabled = false
	r.SetPolicy(updated)

	if r.Policy().GetAutomationEnabled() {
		t.Fatal("retry-wait cancellation left automation enabled")
	}
	select {
	case <-r.Done():
	default:
		t.Fatal("disabling the switch did not stop an active retry wait")
	}
}

func TestCompletedDisplacedSessionReloginClearsCancellationState(t *testing.T) {
	policy := automation.DefaultPolicy()
	policy.Basic.DisplacedSessionReloginEnabled = true
	r := newSessionLifecycleTestRunner(policy)
	r.markSessionInvalidated("账号已在其他设备登录，当前会话被替换")
	if !r.beginAutoReloginAttempt() {
		t.Fatal("could not enter displaced-session relogin attempt")
	}
	if !r.completeAutoRelogin() {
		t.Fatal("enabled displaced-session relogin could not complete")
	}

	updated := r.Policy()
	updated.Basic.DisplacedSessionReloginEnabled = false
	r.SetPolicy(updated)
	select {
	case <-r.Done():
		t.Fatal("turning off the switch stopped a runner after relogin completed")
	default:
	}
}

func TestAutoReloginCompletionCannotOverrideConcurrentSwitchOff(t *testing.T) {
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.DisplacedSessionReloginEnabled = true
	r := newSessionLifecycleTestRunner(policy)
	r.markSessionInvalidated("账号已在其他设备登录，当前会话被替换")
	if !r.beginAutoReloginAttempt() {
		t.Fatal("could not enter displaced-session relogin attempt")
	}

	updated := r.Policy()
	updated.Basic.DisplacedSessionReloginEnabled = false
	r.SetPolicy(updated)
	if r.completeAutoRelogin() {
		t.Fatal("late login success overrode concurrent fail-closed switch-off")
	}
	if !r.isSessionInvalidated() {
		t.Fatal("late login completion cleared the fail-closed invalidation state")
	}
	select {
	case <-r.Done():
	default:
		t.Fatal("concurrent switch-off did not stop the runner")
	}
}

func TestAutoReloginCompletionPreservesSecondDisplacement(t *testing.T) {
	policy := automation.DefaultPolicy()
	policy.Basic.DisplacedSessionReloginEnabled = true
	r := newSessionLifecycleTestRunner(policy)
	r.markSessionInvalidated("首次挤号：账号已在其他设备登录")
	if !r.beginAutoReloginAttempt() {
		t.Fatal("could not enter displaced-session relogin attempt")
	}

	r.handleSessionInvalidated("再次挤号：账号已在其他设备登录", true)
	if r.completeAutoRelogin() {
		t.Fatal("late login success overrode a second displacement")
	}
	if !r.autoReloginPending() {
		t.Fatal("second displacement was not preserved for another delayed login")
	}
	select {
	case <-r.Done():
		t.Fatal("second displacement stopped the runner instead of scheduling another wait")
	default:
	}
}

func TestAutoReloginPreflightRechecksSwitchBeforeFreshLogin(t *testing.T) {
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.DisplacedSessionReloginEnabled = true
	r := newSessionLifecycleTestRunner(policy)
	r.markSessionInvalidated("账号已在其他设备登录，当前会话被替换")

	// Bypass SetPolicy's immediate cancellation to simulate the switch racing
	// with timer expiry. The final preflight must still fail closed.
	r.mu.Lock()
	r.policy.Basic.DisplacedSessionReloginEnabled = false
	r.mu.Unlock()
	if r.prepareAutoReloginAttempt() {
		t.Fatal("auto relogin preflight allowed a fresh login after switch-off")
	}
	select {
	case <-r.Done():
	default:
		t.Fatal("failed auto relogin preflight did not stop the runner")
	}
}

func newSessionLifecycleTestRunner(policy *pb.Policy) *Runner {
	account := &store.Account{ID: 1, Name: "main", Channel: "ios"}
	r := New(
		babigame.Config{},
		nil,
		account,
		NewBus(),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	r.SetPolicy(policy)
	return r
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

func TestManagerRetainsSessionInvalidationAfterFailClosedStop(t *testing.T) {
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
	if policy.GetBasic().GetDisplacedSessionReloginEnabled() {
		t.Fatal("default displaced-session relogin setting=true, want false")
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := NewManager(db, NewBus(), log)
	r := New(babigame.Config{}, db, acc, mgr.Bus(), log)
	r.SetPolicy(policy)

	mgr.mu.Lock()
	mgr.runners[acc.ID] = r
	mgr.mu.Unlock()
	go mgr.forgetWhenDone(acc.ID, r)

	reason := "账号已在其他设备登录，当前会话被替换"
	r.markSessionInvalidated(reason)
	select {
	case <-r.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not stop after fail-closed displacement")
	}
	deadline := time.Now().Add(2 * time.Second)
	for mgr.Get(acc.ID) != nil {
		if time.Now().After(deadline) {
			t.Fatal("manager still tracks runner after Done")
		}
		time.Sleep(10 * time.Millisecond)
	}

	diag, ok := mgr.LastDiagnostics(acc.ID)
	if !ok {
		t.Fatal("LastDiagnostics missing after phone-login kick")
	}
	if diag.SessionInvalidatedReason != reason {
		t.Fatalf("SessionInvalidatedReason=%q, want %q", diag.SessionInvalidatedReason, reason)
	}
}

func TestManagerStopClearsRetainedSessionInvalidation(t *testing.T) {
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
	policy.Basic.DisplacedSessionReloginEnabled = true

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := NewManager(db, NewBus(), log)
	r := New(babigame.Config{}, db, acc, mgr.Bus(), log)
	r.SetPolicy(policy)

	mgr.mu.Lock()
	mgr.runners[acc.ID] = r
	mgr.mu.Unlock()
	go mgr.forgetWhenDone(acc.ID, r)

	r.markSessionInvalidated("账号已在其他设备登录，当前会话被替换")
	if !r.autoReloginPending() {
		t.Fatal("expected pending auto relogin before intentional stop")
	}
	if err := mgr.Stop(acc.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-r.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not stop")
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		_, ok := mgr.LastDiagnostics(acc.ID)
		if !ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("LastDiagnostics still retained after intentional Manager.Stop")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
