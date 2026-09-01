package apiserver

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/buildinfo"
	redeemsvc "github.com/SilkageNet/mygardenworld/internal/redeem"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"
)

func TestWorkspaceSocketAuthenticatesAndNegotiatesProtocol(t *testing.T) {
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
	jwt := auth.NewJWT("workspace-test-secret")
	token, _, err := jwt.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Services{DB: db, JWT: jwt}
	server := httptest.NewServer(svc.WorkspaceHandler(WorkspaceHandlerOptions{}))
	defer server.Close()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	writeClientWorkspaceFrame(t, ctx, conn, &pb.WorkspaceClientFrame{
		RequestId: 9,
		Payload: &pb.WorkspaceClientFrame_Open{Open: &pb.OpenWorkspace{
			ProtocolVersion: WorkspaceProtocolVersion,
			AccessToken:     token,
		}},
	})
	ready := readServerWorkspaceFrame(t, ctx, conn)
	if ready.GetRequestId() != 9 || ready.GetReady().GetProtocolVersion() != WorkspaceProtocolVersion {
		t.Fatalf("ready=%+v", ready)
	}
	if len(ready.GetReady().GetFeatureCapabilities()) == 0 {
		t.Fatal("ready frame is missing feature capabilities")
	}
	if got, want := ready.GetReady().GetServerVersion(), buildinfo.GetVersion(); got != want {
		t.Fatalf("ready server version=%q, want %q", got, want)
	}
}

func TestWorkspaceSocketRejectsProtocolMismatch(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	jwt := auth.NewJWT("workspace-test-secret")
	svc := &Services{DB: db, JWT: jwt}
	server := httptest.NewServer(svc.WorkspaceHandler(WorkspaceHandlerOptions{}))
	defer server.Close()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()

	writeClientWorkspaceFrame(t, ctx, conn, &pb.WorkspaceClientFrame{
		Payload: &pb.WorkspaceClientFrame_Open{Open: &pb.OpenWorkspace{ProtocolVersion: 99}},
	})
	frame := readServerWorkspaceFrame(t, ctx, conn)
	if frame.GetError().GetCode() != "protocol_version_mismatch" || frame.GetError().GetRetryable() {
		t.Fatalf("error=%+v", frame.GetError())
	}
}

func TestWorkspaceSocketClosesWhenOwnerContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	jwt := auth.NewJWT("workspace-test-secret")
	token, _, err := jwt.GenerateAccessToken(user.ID, user.Role)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Services{DB: db, JWT: jwt}
	server := httptest.NewServer(svc.WorkspaceHandler(WorkspaceHandlerOptions{Context: ctx}))
	defer server.Close()
	conn, _, err := websocket.Dial(context.Background(), "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()

	writeClientWorkspaceFrame(t, context.Background(), conn, &pb.WorkspaceClientFrame{
		Payload: &pb.WorkspaceClientFrame_Open{Open: &pb.OpenWorkspace{
			ProtocolVersion: WorkspaceProtocolVersion,
			AccessToken:     token,
		}},
	})
	_ = readServerWorkspaceFrame(t, context.Background(), conn)
	cancel()
	readCtx, cancelRead := context.WithTimeout(context.Background(), time.Second)
	defer cancelRead()
	if _, _, err := conn.Read(readCtx); err == nil {
		t.Fatal("workspace connection remained open after owner context cancellation")
	}
}

func TestWorkspaceTokenExpiryIsAnExpectedReconnect(t *testing.T) {
	if !workspaceNormalClose(auth.ErrTokenExpired) {
		t.Fatal("access-token expiry should be treated as an expected reconnect")
	}
}

func TestWorkspaceSocketScopesAccountsSnapshotsAndLogPagesToIdentity(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.RedeemInstanceID(ctx); err != nil {
		t.Fatal(err)
	}
	owner, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	other, err := db.CreateUser(ctx, "other", "other@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	ownedAccount, err := db.CreateAccount(ctx, owner.ID, "owned", "ios", "owned", "secret")
	if err != nil {
		t.Fatal(err)
	}
	otherAccount, err := db.CreateAccount(ctx, other.ID, "other", "ios", "other", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := db.UpsertRedeemCode(ctx, store.RedeemCodeInput{
		Code: "OWNER-CODE", Channel: "ios", SourceKey: "test:owner",
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureRedeemAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `
UPDATE redeem_attempts
SET status = 'success', message = '兑换成功', attempt_count = 1, attempted_at = CURRENT_TIMESTAMP
WHERE account_id = ?`, ownedAccount.ID); err != nil {
		t.Fatal(err)
	}
	jwt := auth.NewJWT("workspace-test-secret")
	token, _, err := jwt.GenerateAccessToken(owner.ID, owner.Role)
	if err != nil {
		t.Fatal(err)
	}
	manager := runner.NewManager(db, runner.NewBus(), nil)
	svc := &Services{DB: db, JWT: jwt, Manager: manager}
	server := httptest.NewServer(svc.WorkspaceHandler(WorkspaceHandlerOptions{}))
	defer server.Close()
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.CloseNow() }()

	writeClientWorkspaceFrame(t, ctx, conn, &pb.WorkspaceClientFrame{
		RequestId: 1,
		Payload: &pb.WorkspaceClientFrame_Open{Open: &pb.OpenWorkspace{
			ProtocolVersion: WorkspaceProtocolVersion,
			AccessToken:     token,
		}},
	})
	ready := readServerWorkspaceFrame(t, ctx, conn)
	if len(ready.GetReady().GetAccounts()) != 1 || ready.GetReady().GetAccounts()[0].GetAccountId() != ownedAccount.ID {
		t.Fatalf("ready accounts=%+v, want only the authenticated owner's account", ready.GetReady().GetAccounts())
	}

	writeClientWorkspaceFrame(t, ctx, conn, &pb.WorkspaceClientFrame{
		RequestId: 2,
		Payload: &pb.WorkspaceClientFrame_SelectAccount{SelectAccount: &pb.SelectWorkspaceAccount{
			AccountId: ownedAccount.ID,
		}},
	})
	snapshot := readServerWorkspaceFrame(t, ctx, conn)
	if snapshot.GetRequestId() != 2 || snapshot.GetSnapshot().GetState().GetAccountId() != ownedAccount.ID || snapshot.GetSnapshot().GetState().GetPolicy() == nil {
		t.Fatalf("snapshot=%+v, want owned offline state and policy", snapshot.GetSnapshot())
	}

	writeClientWorkspaceFrame(t, ctx, conn, &pb.WorkspaceClientFrame{
		RequestId: 3,
		Payload: &pb.WorkspaceClientFrame_LoadRedeemAttempts{LoadRedeemAttempts: &pb.LoadAccountRedeemAttempts{
			AccountId: ownedAccount.ID,
			Filter:    pb.AccountRedeemAttemptFilter_ACCOUNT_REDEEM_ATTEMPT_FILTER_REDEEMED,
		}},
	})
	redeemPage := readServerWorkspaceFrame(t, ctx, conn)
	if redeemPage.GetRequestId() != 3 || redeemPage.GetRedeemAttempts().GetAccountId() != ownedAccount.ID ||
		len(redeemPage.GetRedeemAttempts().GetEntries()) != 1 ||
		redeemPage.GetRedeemAttempts().GetEntries()[0].GetCode() != "OWNER-CODE" ||
		redeemPage.GetRedeemAttempts().GetSummary().GetSuccess() != 1 {
		t.Fatalf("redeem page=%+v", redeemPage.GetRedeemAttempts())
	}

	writeClientWorkspaceFrame(t, ctx, conn, &pb.WorkspaceClientFrame{
		RequestId: 4,
		Payload: &pb.WorkspaceClientFrame_LoadLogs{LoadLogs: &pb.LoadWorkspaceLogs{
			AccountId: otherAccount.ID,
		}},
	})
	denied := readServerWorkspaceFrame(t, ctx, conn)
	if denied.GetRequestId() != 4 || denied.GetError().GetCode() != "request_failed" {
		t.Fatalf("cross-owner response=%+v, want request_failed", denied)
	}

	writeClientWorkspaceFrame(t, ctx, conn, &pb.WorkspaceClientFrame{
		RequestId: 5,
		Payload: &pb.WorkspaceClientFrame_LoadRedeemAttempts{LoadRedeemAttempts: &pb.LoadAccountRedeemAttempts{
			AccountId: otherAccount.ID,
		}},
	})
	denied = readServerWorkspaceFrame(t, ctx, conn)
	if denied.GetRequestId() != 5 || denied.GetError().GetCode() != "request_failed" {
		t.Fatalf("cross-owner redeem response=%+v, want request_failed", denied)
	}

	var attemptID int64
	if err := db.QueryRowContext(ctx, `SELECT id FROM redeem_attempts WHERE account_id = ?`, ownedAccount.ID).Scan(&attemptID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE redeem_attempts SET status = 'running' WHERE id = ?`, attemptID); err != nil {
		t.Fatal(err)
	}
	if err := db.CompleteRedeemAttempt(ctx, attemptID, store.RedeemValidationInvalid, "无效兑换码", nil); err != nil {
		t.Fatal(err)
	}
	manager.Bus().PublishTransient(runner.Event{
		TS:        time.Now().UTC(),
		AccountID: ownedAccount.ID,
		Kind:      redeemsvc.EventKindRedeemAttemptsUpdated,
	})
	push := readServerWorkspaceFrame(t, ctx, conn)
	if push.GetRequestId() != 0 || !push.GetRedeemAttempts().GetReplace() ||
		len(push.GetRedeemAttempts().GetEntries()) != 0 ||
		push.GetRedeemAttempts().GetSummary().GetInvalid() != 1 {
		t.Fatalf("live redeem page=%+v, want replacement after committed result", push.GetRedeemAttempts())
	}
}

func writeClientWorkspaceFrame(t *testing.T, ctx context.Context, conn *websocket.Conn, frame *pb.WorkspaceClientFrame) {
	t.Helper()
	data, err := proto.Marshal(frame)
	if err != nil {
		t.Fatal(err)
	}
	writeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	if err := conn.Write(writeCtx, websocket.MessageBinary, data); err != nil {
		t.Fatal(err)
	}
}

func readServerWorkspaceFrame(t *testing.T, ctx context.Context, conn *websocket.Conn) *pb.WorkspaceServerFrame {
	t.Helper()
	readCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	typ, data, err := conn.Read(readCtx)
	if err != nil {
		t.Fatal(err)
	}
	if typ != websocket.MessageBinary {
		t.Fatalf("message type=%v, want binary", typ)
	}
	frame := new(pb.WorkspaceServerFrame)
	if err := proto.Unmarshal(data, frame); err != nil {
		t.Fatal(err)
	}
	return frame
}
