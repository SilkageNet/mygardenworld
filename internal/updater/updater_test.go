package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestShouldUpdateLatestOnlyWhenNewer(t *testing.T) {
	if !shouldUpdate("v1.2.3", "v1.2.4", "latest") {
		t.Fatal("expected newer latest release to update")
	}
	if shouldUpdate("v1.2.3", "v1.2.3", "latest") {
		t.Fatal("expected same latest release to be skipped")
	}
	if !shouldUpdate("dev", "v1.2.3", "latest") {
		t.Fatal("expected dev build to update")
	}
}

func TestPickAssetUsesCurrentPlatformShape(t *testing.T) {
	release := &githubRelease{
		TagName: "v1.0.0",
		Assets: []githubAsset{
			{Name: "mygardenworld_v1.0.0_linux_amd64.tar.gz"},
			{Name: "mygardenworld_v1.0.0_windows_amd64.zip"},
		},
	}
	asset, err := pickAsset(release, "windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if asset.Name != "mygardenworld_v1.0.0_windows_amd64.zip" {
		t.Fatalf("asset=%q", asset.Name)
	}
}

func TestExtractBinaryFromTarGz(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "mygardenworld_v1.0.0_linux_amd64.tar.gz")
	if err := os.WriteFile(archivePath, tarGzArchive(t, "mygardenworld_v1.0.0_linux_amd64/gardend", "new-binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := extractBinary(archivePath, dir, "gardend")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-binary" {
		t.Fatalf("extracted %q", data)
	}
}

func TestExtractBinaryFromZip(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "mygardenworld_v1.0.0_windows_amd64.zip")
	if err := os.WriteFile(archivePath, zipArchive(t, "mygardenworld_v1.0.0_windows_amd64/gardend.exe", "new-binary"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := extractBinary(archivePath, dir, "gardend.exe")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-binary" {
		t.Fatalf("extracted %q", data)
	}
}

func TestRunDryRunFetchesReleaseAndSelectsAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/SilkageNet/mygardenworld/releases/latest" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		ext := ".tar.gz"
		if runtime.GOOS == "windows" {
			ext = ".zip"
		}
		_ = json.NewEncoder(w).Encode(githubRelease{
			TagName: "v9.9.9",
			Assets: []githubAsset{{
				Name:               "mygardenworld_v9.9.9_" + runtime.GOOS + "_" + runtime.GOARCH + ext,
				BrowserDownloadURL: serverURLPlaceholder,
			}},
		})
	}))
	defer server.CloseClientConnections()
	defer server.Close()

	result, err := Run(context.Background(), Options{
		APIBaseURL:     server.URL,
		BinaryName:     "gardend",
		CurrentVersion: "v1.0.0",
		TargetPath:     filepath.Join(t.TempDir(), "gardend"),
		DryRun:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.TargetVersion != "v9.9.9" || result.AssetName == "" {
		t.Fatalf("result=%+v", result)
	}
}

func TestChecksumForAsset(t *testing.T) {
	got := checksumForAsset("abc123  mygardenworld_v1.0.0_windows_amd64.zip\n", "mygardenworld_v1.0.0_windows_amd64.zip")
	if got != "abc123" {
		t.Fatalf("checksum=%q", got)
	}
}

func TestRunVerifiesChecksumWhenPresent(t *testing.T) {
	dir := t.TempDir()
	binaryName := "gardend"
	target := filepath.Join(dir, binaryName)
	if runtime.GOOS == "windows" {
		target += ".exe"
	}
	if err := os.WriteFile(target, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	archiveName := "mygardenworld_v9.9.9_" + runtime.GOOS + "_" + runtime.GOARCH
	var archiveBody []byte
	var archiveFile string
	if runtime.GOOS == "windows" {
		archiveFile = archiveName + ".zip"
		archiveBody = zipArchive(t, archiveName+"/"+filepath.Base(target), "new-binary")
	} else {
		archiveFile = archiveName + ".tar.gz"
		archiveBody = tarGzArchive(t, archiveName+"/"+filepath.Base(target), "new-binary")
	}
	sum := sha256.Sum256(archiveBody)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/SilkageNet/mygardenworld/releases/latest":
			_ = json.NewEncoder(w).Encode(githubRelease{
				TagName: "v9.9.9",
				Assets: []githubAsset{
					{Name: archiveFile, BrowserDownloadURL: "http://" + r.Host + "/download/" + archiveFile},
					{Name: "checksums.txt", BrowserDownloadURL: "http://" + r.Host + "/download/checksums.txt"},
				},
			})
		case "/download/" + archiveFile:
			_, _ = w.Write(archiveBody)
		case "/download/checksums.txt":
			_, _ = fmt.Fprintf(w, "%x  %s\n", sum, archiveFile)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.CloseClientConnections()
	defer server.Close()

	result, err := Run(context.Background(), Options{
		APIBaseURL:     server.URL,
		BinaryName:     binaryName,
		CurrentVersion: "v1.0.0",
		TargetPath:     target,
	})
	if runtime.GOOS == "windows" && result != nil && result.Scheduled {
		t.Skip("windows self replacement is scheduled asynchronously")
	}
	if err != nil {
		t.Fatal(err)
	}
	if !result.ChecksumOK {
		t.Fatalf("checksum not verified: %+v", result)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-binary" {
		t.Fatalf("target=%q", data)
	}
}

const serverURLPlaceholder = "http://example.invalid/archive"

func tarGzArchive(t *testing.T, name, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipArchive(t *testing.T, name, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
