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
	jwt := auth.NewJWT("workspace-test-secret")
	token, _, err := jwt.GenerateAccessToken(owner.ID, owner.Role)
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
		Payload: &pb.WorkspaceClientFrame_LoadLogs{LoadLogs: &pb.LoadWorkspaceLogs{
			AccountId: otherAccount.ID,
		}},
	})
	denied := readServerWorkspaceFrame(t, ctx, conn)
	if denied.GetRequestId() != 3 || denied.GetError().GetCode() != "request_failed" {
		t.Fatalf("cross-owner response=%+v, want request_failed", denied)
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
