package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCORSMiddlewareEchoesAllowedOrigin(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "http://localhost:3000,http://127.0.0.1:3000")

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

func TestCORSMiddlewareRejectsUnknownOrigin(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}), "http://localhost:3000,http://127.0.0.1:3000")

	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty for disallowed origin", got)
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
