package apiserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	connect "connectrpc.com/connect"
	"golang.org/x/crypto/bcrypt"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestLoginUnknownUserAndWrongPasswordUseSameError(t *testing.T) {
	ctx := context.Background()
	svc := newAuthTestService(t, LoginLimiterConfig{UserFailures: 100, IPFailures: 100})
	createTestUser(t, ctx, svc.DB, "owner", "owner@example.test", "ValidPass123!", "active")

	for _, tc := range []struct {
		name     string
		username string
		password string
	}{
		{name: "unknown", username: "missing", password: "whatever"},
		{name: "wrong password", username: "owner", password: "wrong"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: tc.username, Password: tc.password}))
			if connect.CodeOf(err) != connect.CodeUnauthenticated {
				t.Fatalf("Login code=%s err=%v, want Unauthenticated", connect.CodeOf(err), err)
			}
			if !strings.Contains(err.Error(), "账号或密码不正确") {
				t.Fatalf("Login error=%q, want localized invalid credentials", err)
			}
		})
	}
}

func TestLoginLimiterReturnsResourceExhausted(t *testing.T) {
	ctx := context.Background()
	svc := newAuthTestService(t, LoginLimiterConfig{
		Window:       time.Hour,
		UserFailures: 2,
		IPFailures:   100,
		Lockout:      time.Hour,
		MaxEntries:   16,
	})
	createTestUser(t, ctx, svc.DB, "owner", "owner@example.test", "ValidPass123!", "active")

	_, err := svc.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "owner", Password: "wrong"}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("first Login code=%s err=%v, want Unauthenticated", connect.CodeOf(err), err)
	}
	_, err = svc.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "owner", Password: "wrong"}))
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatalf("second Login code=%s err=%v, want ResourceExhausted", connect.CodeOf(err), err)
	}
	_, err = svc.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "owner", Password: "ValidPass123!"}))
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatalf("locked Login code=%s err=%v, want ResourceExhausted", connect.CodeOf(err), err)
	}
}

func TestDisabledUserOnlyRevealsStatusAfterCorrectPassword(t *testing.T) {
	ctx := context.Background()
	svc := newAuthTestService(t, LoginLimiterConfig{UserFailures: 100, IPFailures: 100})
	createTestUser(t, ctx, svc.DB, "owner", "owner@example.test", "ValidPass123!", "disabled")

	_, err := svc.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "owner", Password: "wrong"}))
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("wrong password code=%s err=%v, want Unauthenticated", connect.CodeOf(err), err)
	}
	_, err = svc.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "owner", Password: "ValidPass123!"}))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("correct disabled code=%s err=%v, want PermissionDenied", connect.CodeOf(err), err)
	}
}

func TestLoginSetsStrictRefreshCookie(t *testing.T) {
	ctx := context.Background()
	svc := newAuthTestService(t, LoginLimiterConfig{UserFailures: 100, IPFailures: 100})
	svc.ListenAddr = "127.0.0.1:50051"
	createTestUser(t, ctx, svc.DB, "owner", "owner@example.test", "ValidPass123!", "active")

	resp, err := svc.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "owner", Password: "ValidPass123!"}))
	if err != nil {
		t.Fatal(err)
	}
	cookie := resp.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "SameSite=Strict") {
		t.Fatalf("Set-Cookie=%q, want SameSite=Strict", cookie)
	}
	if !strings.Contains(cookie, "mgw_refresh_token_50051=") {
		t.Fatalf("Set-Cookie=%q, want port-scoped cookie name", cookie)
	}
}

func TestRefreshCookiesAreScopedByListenPort(t *testing.T) {
	ctx := context.Background()
	svcA := newAuthTestService(t, LoginLimiterConfig{UserFailures: 100, IPFailures: 100})
	svcA.ListenAddr = "127.0.0.1:50051"
	svcB := newAuthTestService(t, LoginLimiterConfig{UserFailures: 100, IPFailures: 100})
	svcB.ListenAddr = "127.0.0.1:50052"
	createTestUser(t, ctx, svcA.DB, "owner", "owner@example.test", "ValidPass123!", "active")
	createTestUser(t, ctx, svcB.DB, "owner", "owner@example.test", "ValidPass123!", "active")

	respA, err := svcA.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "owner", Password: "ValidPass123!"}))
	if err != nil {
		t.Fatal(err)
	}
	respB, err := svcB.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "owner", Password: "ValidPass123!"}))
	if err != nil {
		t.Fatal(err)
	}
	cookiesA := respA.Header().Values("Set-Cookie")
	cookiesB := respB.Header().Values("Set-Cookie")
	joinedA := strings.Join(cookiesA, "\n")
	joinedB := strings.Join(cookiesB, "\n")
	if !strings.Contains(joinedA, "mgw_refresh_token_50051=") {
		t.Fatalf("stack A cookies=%q, want _50051", joinedA)
	}
	if !strings.Contains(joinedB, "mgw_refresh_token_50052=") {
		t.Fatalf("stack B cookies=%q, want _50052", joinedB)
	}
	if strings.Contains(joinedA, "mgw_refresh_token_50052=") || strings.Contains(joinedB, "mgw_refresh_token_50051=") {
		t.Fatalf("cookie names collided across stacks:\nA=%q\nB=%q", joinedA, joinedB)
	}
}

func TestCreateUserRejectsWeakPassword(t *testing.T) {
	ctx := auth.ContextWithIdentity(context.Background(), &auth.Identity{UserID: 1, Role: "admin"})
	svc := newAuthTestService(t, LoginLimiterConfig{})

	_, err := svc.CreateUser(ctx, connect.NewRequest(&pb.CreateUserRequest{
		Username: "weak",
		Email:    "weak@example.test",
		Password: "change-me-first",
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateUser code=%s err=%v, want InvalidArgument", connect.CodeOf(err), err)
	}
}

func TestCreateUserValidatesOptionsBeforeInsert(t *testing.T) {
	ctx := auth.ContextWithIdentity(context.Background(), &auth.Identity{UserID: 1, Role: "admin"})
	svc := newAuthTestService(t, LoginLimiterConfig{})

	_, err := svc.CreateUser(ctx, connect.NewRequest(&pb.CreateUserRequest{
		Username: " partial ",
		Email:    "partial@example.test",
		Password: "ValidPass123!",
		Role:     userRolePtr(pb.UserRole(99)),
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("CreateUser code=%s err=%v, want InvalidArgument", connect.CodeOf(err), err)
	}
	if _, err := svc.DB.GetUserByUsername(context.Background(), "partial"); !errors.Is(err, store.ErrUserNotFound) {
		t.Fatalf("invalid CreateUser left a user behind: %v", err)
	}
}

func TestDisableUserStopsRestoreAndRevokesRefreshTokens(t *testing.T) {
	ctx := context.Background()
	adminCtx := auth.ContextWithIdentity(ctx, &auth.Identity{UserID: 1, Role: "admin"})
	svc := newAuthTestService(t, LoginLimiterConfig{})
	svc.Manager = runner.NewManager(svc.DB, runner.NewBus(), svc.Log)
	user, err := svc.DB.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	account, err := svc.DB.CreateAccount(ctx, user.ID, "main", "ios", "game", "password")
	if err != nil {
		t.Fatal(err)
	}
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	if err := svc.persistPolicy(ctx, account.ID, policy); err != nil {
		t.Fatal(err)
	}
	if err := svc.DB.SaveRefreshToken(ctx, user.ID, "refresh-token", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	status := pb.UserStatus_USER_STATUS_DISABLED
	if _, err := svc.UpdateUser(adminCtx, connect.NewRequest(&pb.UpdateUserRequest{UserId: user.ID, Status: &status})); err != nil {
		t.Fatal(err)
	}
	storedPolicy, err := svc.DB.LoadPolicyJSON(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	effective, err := policycfg.FromJSON(storedPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if effective.GetAutomationEnabled() {
		t.Fatal("disabled user's persisted automation remains enabled")
	}
	if _, err := svc.DB.ValidateRefreshToken(ctx, "refresh-token"); !errors.Is(err, store.ErrTokenInvalid) {
		t.Fatalf("disabled user's refresh token error = %v, want ErrTokenInvalid", err)
	}
}

func userRolePtr(value pb.UserRole) *pb.UserRole { return &value }

func newAuthTestService(t *testing.T, limiterCfg LoginLimiterConfig) *Services {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &Services{
		DB:           db,
		JWT:          auth.NewJWT("test-secret-test-secret-test-secret"),
		Log:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		LoginLimiter: NewLoginLimiter(limiterCfg),
	}
}

func createTestUser(t *testing.T, ctx context.Context, db *store.DB, username, email, password, status string) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.CreateUser(ctx, username, email, string(hash))
	if err != nil {
		t.Fatal(err)
	}
	if status != "" && status != "active" {
		if _, err := db.UpdateUser(ctx, user.ID, nil, nil, &status); err != nil {
			t.Fatal(err)
		}
	}
}
