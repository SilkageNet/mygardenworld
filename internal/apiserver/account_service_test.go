package apiserver

import (
	"context"
	"fmt"
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

func TestLogoutAccountDisablesAutomationPreference(t *testing.T) {
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
	_, err = svc.LogoutAccount(userCtx, connect.NewRequest(&pb.LogoutAccountRequest{Id: fmt.Sprintf("%d", acc.ID)}))
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
		t.Fatal("automation_enabled=true after LogoutAccount, want false")
	}
}

func TestRedeemResultMessageReportsOutcomeAndGains(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   runner.RedeemResult
		want string
	}{
		{
			name: "success with items",
			in: runner.RedeemResult{
				Outcome: runner.RedeemOutcomeSuccess,
				Items: []runner.RedeemItemGain{
					{Name: "金币", Count: 12888},
					{Name: "花坊币", Count: 66},
				},
			},
			want: "金币x12888、花坊币x66",
		},
		{
			name: "already redeemed",
			in: runner.RedeemResult{
				Outcome: runner.RedeemOutcomeAlreadyRedeemed,
				Message: "已领取过该奖励",
			},
			want: "已领取过该奖励",
		},
		{
			name: "invalid without message",
			in:   runner.RedeemResult{Outcome: runner.RedeemOutcomeInvalid},
			want: "无效兑换码",
		},
		{
			name: "success mail only",
			in: runner.RedeemResult{
				Outcome: runner.RedeemOutcomeSuccess,
				MailNew: 2,
			},
			want: "奖励已入邮件（2 封待领取）",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := redeemResultMessage(tc.in); got != tc.want {
				t.Fatalf("redeemResultMessage() = %q, want %q", got, tc.want)
			}
		})
	}
}
