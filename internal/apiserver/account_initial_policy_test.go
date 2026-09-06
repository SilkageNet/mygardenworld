package apiserver

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestInitialAccountPolicyAndReauthenticationScope(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	owner, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	acc, err := db.CreateAccount(ctx, owner.ID, "existing", "alipay", "original-user", "old-grant")
	if err != nil {
		t.Fatal(err)
	}
	p := automation.DefaultPolicy()
	p.Union.Race.UpgradeTask = true
	p.Union.Race.MaxSpendDiamond = 73
	p.AutomationEnabled = false
	raw, err := policycfg.ToJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SavePolicyJSON(ctx, acc.ID, raw); err != nil {
		t.Fatal(err)
	}
	svc := &Services{DB: db, AlipayLogins: NewAlipayLoginCoordinator(&fakeAlipayProvider{})}
	userCtx := auth.ContextWithIdentity(ctx, &auth.Identity{UserID: owner.ID, Role: "user"})
	for _, tc := range []struct {
		name     string
		userID   int64
		sourceID int64
		allowed  bool
	}{
		{"own source", owner.ID, acc.ID, true},
		{"other owner", owner.ID + 100, acc.ID, false},
		{"missing", owner.ID, acc.ID + 100, false},
		{"negative", owner.ID, -1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			context := auth.ContextWithIdentity(ctx, &auth.Identity{UserID: tc.userID, Role: "user"})
			got, err := svc.initialAccountPolicy(context, tc.sourceID)
			if (err == nil) != tc.allowed {
				t.Fatalf("policy access: %v", err)
			}
			if tc.allowed && got != raw {
				t.Fatal("initial policy was not copied in full")
			}
		})
	}
	// Account re-authorization must remain available when no new accounts fit.
	if _, err := db.ExecContext(ctx, `UPDATE users SET max_accounts = 1 WHERE id = ?`, owner.ID); err != nil {
		t.Fatal(err)
	}
	response, err := svc.StartAlipayLogin(userCtx, connect.NewRequest(&pb.StartAlipayLoginRequest{AccountId: acc.ID}))
	if err != nil || response.Msg.GetLoginId() == "" {
		t.Fatalf("reauth at quota: %v", err)
	}
	_, err = svc.createAlipayAccount(userCtx, owner.ID, babigame.AlipayWebGrant{UserID: "wrong-user", Token: "new-grant"}, alipayLoginOptions{AccountID: acc.ID})
	if err == nil || !strings.Contains(err.Error(), "不一致") {
		t.Fatalf("mismatched QR identity accepted: %v", err)
	}
	stored, err := db.LoadPolicyJSON(ctx, acc.ID)
	if err != nil || stored != raw {
		t.Fatal("failed reauth changed policy")
	}
	count, err := db.CountAccountsByUser(ctx, owner.ID)
	if err != nil || count != 1 {
		t.Fatal("reauth created a new account")
	}
	_, err = svc.StartAlipayLogin(userCtx, connect.NewRequest(&pb.StartAlipayLoginRequest{AccountId: acc.ID, InitialPolicyAccountId: acc.ID}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("reauth allowed policy overwrite: %v", err)
	}
}

func TestAlipayInitialPolicySnapshotSurvivesPolling(t *testing.T) {
	provider := &fakeAlipayProvider{polls: 1}
	coordinator := NewAlipayLoginCoordinator(provider)
	options := alipayLoginOptions{AccountID: 123, InitialPolicy: "immutable policy snapshot"}
	id, _, _, err := coordinator.start(context.Background(), 7, options)
	if err != nil {
		t.Fatal(err)
	}
	got := coordinator.poll(context.Background(), 7, id, func(_ context.Context, _ babigame.AlipayWebGrant, received alipayLoginOptions) (*store.Account, error) {
		if received != options {
			t.Fatal("QR binding or initial policy lost")
		}
		return &store.Account{ID: 123}, nil
	})
	if got.Status != pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_COMPLETE {
		t.Fatalf("status: %v", got.Status)
	}
}
