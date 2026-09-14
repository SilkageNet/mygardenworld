package apiserver

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestWorkspaceDeselectReleasesDeletedAccountBuffers(t *testing.T) {
	s := &workspaceSession{
		selectedID: 42, selectedAccount: &store.Account{ID: 42}, lastState: &pb.WorkspaceState{AccountId: 42},
		logHighWater: 123, catchingUp: true, dirtyState: true, redeemSubscribed: true, dirtyRedeem: true,
	}
	if err := s.selectAccount(1, 0, 0); err != nil {
		t.Fatal(err)
	}
	if s.selectedID != 0 || s.selectedAccount != nil || s.lastState != nil || s.logHighWater != 0 || s.catchingUp || s.dirtyState || s.redeemSubscribed || s.dirtyRedeem {
		t.Fatal("empty selection kept the old account subscription")
	}
	s.selectedID, s.selectedAccount = 42, &store.Account{ID: 42}
	s.setStatuses([]*pb.AccountStatus{{AccountId: 43}})
	if s.selectedID != 0 || s.selectedAccount != nil {
		t.Fatal("account removed in another tab retained a subscription")
	}
}

func TestDeleteAccountReportsWriterTimeoutAndCanRetry(t *testing.T) {
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	u, err := db.CreateUser(t.Context(), "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(t.Context(), u.ID, "fixture", "ios", "fixture", "fixture")
	if err != nil {
		t.Fatal(err)
	}
	svc := &Services{DB: db, Manager: runner.NewManager(db, runner.NewBus(), nil)}
	ctx := auth.ContextWithIdentity(t.Context(), &auth.Identity{UserID: u.ID, Role: "user"})
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	short, cancel := context.WithTimeout(ctx, 20*time.Millisecond)
	defer cancel()
	if _, err := svc.DeleteAccount(short, connect.NewRequest(&pb.DeleteAccountRequest{Id: a.ID})); connect.CodeOf(err) != connect.CodeDeadlineExceeded {
		t.Fatalf("writer timeout was hidden: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetAccountByID(ctx, a.ID); err != nil {
		t.Fatal("timed-out delete committed", err)
	}
	if _, err := svc.DeleteAccount(ctx, connect.NewRequest(&pb.DeleteAccountRequest{Id: a.ID})); err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListAccounts(ctx, connect.NewRequest(&pb.ListAccountsRequest{}))
	if err != nil || len(list.Msg.Accounts) != 0 {
		t.Fatalf("deleted account remained: %v %v", list, err)
	}
	if _, err := svc.DeleteAccount(ctx, connect.NewRequest(&pb.DeleteAccountRequest{Id: a.ID})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("missing account did not report not-found: %v", err)
	}
}
