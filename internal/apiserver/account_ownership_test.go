package apiserver

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestAccountOwnershipHasNoAdministratorBypass(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	owner, err := db.CreateUser(ctx, "owner", "owner@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.CreateUser(ctx, "other", "other@test.invalid", "hash")
	if err != nil {
		t.Fatal(err)
	}
	owned, err := db.CreateAccount(ctx, owner.ID, "owned", "ios", "owned", "secret")
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := db.CreateAccount(ctx, other.ID, "foreign", "ios", "foreign", "secret")
	if err != nil {
		t.Fatal(err)
	}
	svc := &Services{DB: db, Manager: runner.NewManager(db, runner.NewBus(), nil)}
	for _, role := range []string{"user", "admin", "anonymous"} {
		t.Run(role, func(t *testing.T) {
			userCtx := ctx
			want := connect.CodePermissionDenied
			if role == "anonymous" {
				want = connect.CodeUnauthenticated
			} else {
				userCtx = auth.ContextWithIdentity(ctx, &auth.Identity{UserID: owner.ID, Role: role})
			}
			list, err := svc.ListAccounts(userCtx, connect.NewRequest(&pb.ListAccountsRequest{}))
			statuses, statusErr := svc.accountStatuses(userCtx)
			if role == "anonymous" {
				if connect.CodeOf(err) != want || connect.CodeOf(statusErr) != want {
					t.Fatalf("anonymous lists: %v, %v", err, statusErr)
				}
			} else if err != nil || statusErr != nil || len(list.Msg.Accounts) != 1 || list.Msg.Accounts[0].Id != owned.ID || len(statuses) != 1 || statuses[0].AccountId != owned.ID {
				t.Fatalf("lists leaked or failed: %v %v %v %v", list, statuses, err, statusErr)
			}
			checks := map[string]func() error{
				"resolve":     func() error { _, err := svc.resolveAccount(userCtx, foreign.ID); return err },
				"logs":        func() error { _, err := svc.workspaceLogPage(userCtx, foreign.ID, 0, 5); return err },
				"policy copy": func() error { _, err := svc.initialAccountPolicy(userCtx, foreign.ID); return err },
				"policy save": func() error {
					_, err := svc.SetPolicy(userCtx, connect.NewRequest(&pb.SetPolicyRequest{AccountId: foreign.ID}))
					return err
				},
				"connect": func() error {
					_, err := svc.ConnectAccount(userCtx, connect.NewRequest(&pb.ConnectAccountRequest{Id: foreign.ID}))
					return err
				},
				"disconnect": func() error {
					_, err := svc.DisconnectAccount(userCtx, connect.NewRequest(&pb.DisconnectAccountRequest{Id: foreign.ID}))
					return err
				},
				"delete": func() error {
					_, err := svc.DeleteAccount(userCtx, connect.NewRequest(&pb.DeleteAccountRequest{Id: foreign.ID}))
					return err
				},
			}
			for name, check := range checks {
				t.Run(name, func(t *testing.T) {
					if err := check(); connect.CodeOf(err) != want {
						t.Fatalf("got %v, want %v", err, want)
					}
				})
			}
		})
	}
	if _, err := db.GetAccountByID(ctx, foreign.ID); err != nil {
		t.Fatal("foreign account was modified", err)
	}
}
