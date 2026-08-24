package apiserver

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

type failClosedPolicyRuntime struct {
	policy *pb.Policy
	events []runner.Event
}

func (r *failClosedPolicyRuntime) SetPolicy(policy *pb.Policy) {
	r.policy = policycfg.Clone(policy)
	// Model the runner's fail-closed transition when a pending displaced-
	// session relogin is switched off.
	if !r.policy.GetBasic().GetDisplacedSessionReloginEnabled() {
		r.policy.AutomationEnabled = false
	}
}

func (r *failClosedPolicyRuntime) Policy() *pb.Policy {
	return policycfg.Clone(r.policy)
}

func (r *failClosedPolicyRuntime) Emit(event runner.Event) {
	r.events = append(r.events, event)
}

func TestPersistAndApplyPolicyReturnsAndStoresEffectiveRunnerPolicy(t *testing.T) {
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
	account, err := db.CreateAccount(ctx, user.ID, "main", "ios", "game", "pw")
	if err != nil {
		t.Fatal(err)
	}

	svc := &Services{DB: db}
	runtime := &failClosedPolicyRuntime{}
	requested := automation.DefaultPolicy()
	requested.AutomationEnabled = true
	requested.Basic.DisplacedSessionReloginEnabled = false
	effective, err := svc.persistAndApplyPolicy(ctx, account.ID, runtime, requested)
	if err != nil {
		t.Fatal(err)
	}
	if effective.GetAutomationEnabled() {
		t.Fatal("returned policy has automation_enabled=true, want effective false")
	}
	if runtime.Policy().GetAutomationEnabled() {
		t.Fatal("runtime policy has automation_enabled=true, want false")
	}

	raw, err := db.LoadPolicyJSON(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := policycfg.FromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if stored.GetAutomationEnabled() {
		t.Fatal("stored policy has automation_enabled=true, want effective false")
	}
	if len(runtime.events) != 1 || !strings.Contains(runtime.events[0].PayloadJSON, `"automation_enabled":false`) {
		t.Fatalf("policy event=%+v, want one effective automation_enabled=false event", runtime.events)
	}
}

func TestSetPolicyRejectsSDKAdAutomation(t *testing.T) {
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
	account, err := db.CreateAccount(ctx, user.ID, "main", "ios", "game", "pw")
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := &Services{DB: db, Manager: runner.NewManager(db, runner.NewBus(), log), Log: log}

	requested := automation.DefaultPolicy()
	requested.Basic.Shop.VideoFreeGiftEnabled = true
	_, err = svc.SetPolicy(ctx, connect.NewRequest(&pb.SetPolicyRequest{
		AccountId: fmt.Sprintf("%d", account.ID),
		Policy:    requested,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "SDK 广告") || !strings.Contains(err.Error(), "视频礼包") {
		t.Fatalf("SetPolicy() error = %v, want explicit invalid_argument SDK ad rejection", err)
	}

	raw, loadErr := db.LoadPolicyJSON(ctx, account.ID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	stored, parseErr := policycfg.FromJSON(raw)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	if stored.GetBasic().GetShop().GetVideoFreeGiftEnabled() {
		t.Fatal("rejected video gift policy was persisted")
	}
}
