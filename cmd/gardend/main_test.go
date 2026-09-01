package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	connect "connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1/mygardenworldv1connect"
	"github.com/SilkageNet/mygardenworld/internal/apiserver"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestCORSMiddlewareEchoesAllowedOrigin(t *testing.T) {
	policy, err := newOriginPolicy("http://localhost:3000,http://127.0.0.1:3000", false)
	if err != nil {
		t.Fatal(err)
	}
	handler := originGuardMiddleware(corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), policy), policy)

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://127.0.0.1:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:3000" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want current origin", got)
	}
	if got := rec.Code; got != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", got, http.StatusNoContent)
	}
}

func TestOriginGuardRejectsUnknownOrigin(t *testing.T) {
	policy, err := newOriginPolicy("http://localhost:3000,http://127.0.0.1:3000", false)
	if err != nil {
		t.Fatal(err)
	}
	handler := originGuardMiddleware(corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), policy), policy)

	req := httptest.NewRequest(http.MethodOptions, "http://127.0.0.1:50051/test", nil)
	req.Host = "127.0.0.1:50051"
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty for disallowed origin", got)
	}
	if got := rec.Code; got != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", got, http.StatusForbidden)
	}
}

func TestOriginGuardAllowsSameOrigin(t *testing.T) {
	policy, err := newOriginPolicy("", false)
	if err != nil {
		t.Fatal(err)
	}
	handler := originGuardMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), policy)

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:50051/test", nil)
	req.Host = "127.0.0.1:50051"
	req.Header.Set("Origin", "http://127.0.0.1:50051")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Code; got != http.StatusOK {
		t.Fatalf("status = %d, want %d", got, http.StatusOK)
	}
}

func TestOriginPolicyRejectsWildcardUnlessExplicitlyAllowed(t *testing.T) {
	if _, err := newOriginPolicy("*", false); err == nil {
		t.Fatal("newOriginPolicy(*) succeeded without insecure flag, want error")
	}
	policy, err := newOriginPolicy("*", true)
	if err != nil {
		t.Fatalf("newOriginPolicy(*) with insecure flag returned error: %v", err)
	}
	handler := originGuardMiddleware(corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), policy), policy)

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://example.com" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want echoed wildcard origin", got)
	}
	if got := rec.Code; got != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", got, http.StatusNoContent)
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	handler := securityHeadersMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := rec.Header().Get("X-Frame-Options"); got != "DENY" {
		t.Fatalf("X-Frame-Options = %q, want DENY", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors 'none'") {
		t.Fatalf("Content-Security-Policy = %q, want frame-ancestors 'none'", got)
	}
}

func TestValidateServeSecurityRejectsNetworkDebug(t *testing.T) {
	err := validateServeSecurity(serveOpts{
		ListenAddr:  "0.0.0.0:50051",
		DebugDir:    "debug",
		MaxReqBytes: 1048576,
	})
	if err == nil {
		t.Fatal("validateServeSecurity succeeded, want non-loopback debug error")
	}
	if err := validateServeSecurity(serveOpts{
		ListenAddr:    "0.0.0.0:50051",
		DebugDir:      "debug",
		MaxReqBytes:   1048576,
		InsecureDebug: true,
	}); err != nil {
		t.Fatalf("validateServeSecurity with insecure debug returned error: %v", err)
	}
}

func TestSeedAdminRejectsWeakPassword(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	err = seedAdmin(ctx, db, slog.New(slog.NewTextHandler(io.Discard, nil)), serveOpts{
		AdminUsername: "admin",
		AdminEmail:    "admin@example.test",
		AdminPassword: "change-me-first",
	})
	if err == nil {
		t.Fatal("seedAdmin succeeded with weak password, want error")
	}
}

func TestConnectReadMaxBytesRejectsOversizedRequest(t *testing.T) {
	svc := &apiserver.Services{}
	path, handler := mygardenworldv1connect.NewAuthServiceHandler(svc, connect.WithReadMaxBytes(8))
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	defer server.Close()

	client := mygardenworldv1connect.NewAuthServiceClient(server.Client(), server.URL)
	_, err := client.Login(context.Background(), connect.NewRequest(&pb.LoginRequest{
		Username: strings.Repeat("a", 128),
		Password: "p",
	}))
	if connect.CodeOf(err) != connect.CodeResourceExhausted {
		t.Fatalf("Login code=%s err=%v, want ResourceExhausted", connect.CodeOf(err), err)
	}
}

func TestCleanDataDirPathRejectsDangerousTargets(t *testing.T) {
	root := filepath.VolumeName(os.TempDir()) + string(os.PathSeparator)
	if _, err := cleanDataDirPath(root); err == nil {
		t.Fatalf("cleanDataDirPath(%q) succeeded, want error", root)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cleanDataDirPath(cwd); err == nil {
		t.Fatalf("cleanDataDirPath(%q) succeeded, want error", cwd)
	}
}

func TestRemoveDataDirDeletesDirectory(t *testing.T) {
	parent := t.TempDir()
	dataDir := filepath.Join(parent, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "garden.db"), []byte("db"), 0o644); err != nil {
		t.Fatal(err)
	}

	removed, err := removeDataDir(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if !removed {
		t.Fatal("removeDataDir removed=false, want true")
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("dataDir still exists or stat failed unexpectedly: %v", err)
	}
}

func TestCompactDBCommandRequiresConfirmationAndCompacts(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	db, err := store.Open(ctx, filepath.Join(dataDir, "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	var preview bytes.Buffer
	cmd := newCompactDBCmd()
	cmd.SetOut(&preview)
	cmd.SetArgs([]string{"--data-dir", dataDir})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(preview.String(), "Would compact") || !strings.Contains(preview.String(), "Stop gardend") {
		t.Fatalf("compact preview=%q", preview.String())
	}

	var output bytes.Buffer
	cmd = newCompactDBCmd()
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--data-dir", dataDir, "--yes"})
	if err := cmd.ExecuteContext(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Compacted SQLite database") {
		t.Fatalf("compact output=%q", output.String())
	}
}
