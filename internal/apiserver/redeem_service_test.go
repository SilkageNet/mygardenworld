package apiserver

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	connect "connectrpc.com/connect"

	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestResolveRedeemAccountsRequiresExplicitSingleChannelTargets(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	owner, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.CreateUser(ctx, "other", "other@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	iosOne, err := db.CreateAccount(ctx, owner.ID, "ios-one", "ios", "ios-one", "pw")
	if err != nil {
		t.Fatal(err)
	}
	iosTwo, err := db.CreateAccount(ctx, owner.ID, "ios-two", "ios", "ios-two", "pw")
	if err != nil {
		t.Fatal(err)
	}
	alipay, err := db.CreateAccount(ctx, owner.ID, "alipay", "alipay", "alipay", "pw")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := db.CreateAccount(ctx, other.ID, "foreign", "ios", "foreign", "pw")
	if err != nil {
		t.Fatal(err)
	}

	svc := &Services{DB: db}
	userCtx := auth.ContextWithIdentity(ctx, &auth.Identity{UserID: owner.ID, Role: "user"})

	for name, ids := range map[string][]string{
		"missing": nil,
		"blank":   {"  "},
		"mixed":   {fmt.Sprint(iosOne.ID), fmt.Sprint(alipay.ID)},
	} {
		t.Run(name, func(t *testing.T) {
			_, resolveErr := svc.resolveRedeemAccounts(userCtx, ids)
			if connect.CodeOf(resolveErr) != connect.CodeInvalidArgument {
				t.Fatalf("resolveRedeemAccounts() error = %v, want invalid_argument", resolveErr)
			}
		})
	}

	accounts, err := svc.resolveRedeemAccounts(userCtx, []string{
		fmt.Sprint(iosOne.ID),
		fmt.Sprint(iosOne.ID),
		fmt.Sprint(iosTwo.ID),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(accounts) != 2 || accounts[0].ID != iosOne.ID || accounts[1].ID != iosTwo.ID {
		t.Fatalf("resolveRedeemAccounts() = %+v, want both unique iOS accounts", accounts)
	}

	_, err = svc.resolveRedeemAccounts(userCtx, []string{fmt.Sprint(foreign.ID)})
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("foreign account error = %v, want permission_denied", err)
	}
}
