// Package updater implements the GitHub Releases based self-update flow used
// by the CLI binaries.
package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultRepo       = "SilkageNet/mygardenworld"
	DefaultAPIBaseURL = "https://api.github.com"
)

type Options struct {
	Repo           string
	Version        string
	BinaryName     string
	CurrentVersion string
	TargetPath     string
	APIBaseURL     string
	HTTPClient     *http.Client
	Force          bool
	DryRun         bool
}

type Result struct {
	Updated        bool
	Scheduled      bool
	CurrentVersion string
	TargetVersion  string
	AssetName      string
	TargetPath     string
	BackupPath     string
	ChecksumOK     bool
	Message        string
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func Run(ctx context.Context, opts Options) (*Result, error) {
	opts = normalizeOptions(opts)
	release, err := fetchRelease(ctx, opts)
	if err != nil {
		return nil, err
	}
	if release.TagName == "" {
		return nil, errors.New("release has no tag_name")
	}
	if !opts.Force && !shouldUpdate(opts.CurrentVersion, release.TagName, opts.Version) {
		return &Result{
			CurrentVersion: opts.CurrentVersion,
			TargetVersion:  release.TagName,
			TargetPath:     opts.TargetPath,
			Message:        fmt.Sprintf("%s is already up to date", opts.BinaryName),
		}, nil
	}

	asset, err := pickAsset(release, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	result := &Result{
		CurrentVersion: opts.CurrentVersion,
		TargetVersion:  release.TagName,
		AssetName:      asset.Name,
		TargetPath:     opts.TargetPath,
	}
	if opts.DryRun {
		result.Message = fmt.Sprintf("would update %s from %s to %s using %s", opts.BinaryName, opts.CurrentVersion, release.TagName, asset.Name)
		return result, nil
	}

	tmpDir, err := os.MkdirTemp("", "mygardenworld-update-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, asset.Name)
	if err := downloadFile(ctx, opts, asset.BrowserDownloadURL, archivePath); err != nil {
		return nil, err
	}
	checksumOK, err := verifyChecksum(ctx, opts, release, asset.Name, archivePath)
	if err != nil {
		return nil, err
	}
	result.ChecksumOK = checksumOK
	extractedPath, err := extractBinary(archivePath, tmpDir, opts.BinaryName)
	if err != nil {
		return nil, err
	}

	stagedPath := opts.TargetPath + ".new"
	if err := copyFile(extractedPath, stagedPath, 0o755); err != nil {
		return nil, fmt.Errorf("stage new binary: %w", err)
	}
	backupPath := opts.TargetPath + ".bak"
	result.BackupPath = backupPath

	scheduled, err := replaceBinary(opts.TargetPath, stagedPath, backupPath)
	if err != nil {
		_ = os.Remove(stagedPath)
		return nil, err
	}
	result.Updated = !scheduled
	result.Scheduled = scheduled
	if scheduled {
		result.Message = fmt.Sprintf("scheduled %s update to %s after this process exits", opts.BinaryName, release.TagName)
	} else {
		result.Message = fmt.Sprintf("updated %s from %s to %s", opts.BinaryName, opts.CurrentVersion, release.TagName)
	}
	return result, nil
}

func normalizeOptions(opts Options) Options {
	if opts.Repo == "" {
		opts.Repo = DefaultRepo
	}
	if opts.APIBaseURL == "" {
		opts.APIBaseURL = DefaultAPIBaseURL
	}
	opts.APIBaseURL = strings.TrimRight(opts.APIBaseURL, "/")
	if opts.Version == "" {
		opts.Version = "latest"
	}
	if opts.CurrentVersion == "" {
		opts.CurrentVersion = "dev"
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
	if opts.BinaryName == "" {
		opts.BinaryName = filepath.Base(os.Args[0])
	}
	opts.BinaryName = normalizeExeName(opts.BinaryName)
	if opts.TargetPath == "" {
		if exe, err := os.Executable(); err == nil {
			opts.TargetPath = exe
		}
	}
	if abs, err := filepath.Abs(opts.TargetPath); err == nil {
		opts.TargetPath = abs
	}
	return opts
}

func fetchRelease(ctx context.Context, opts Options) (*githubRelease, error) {
	version := strings.TrimSpace(opts.Version)
	var url string
	if version == "" || strings.EqualFold(version, "latest") {
		url = fmt.Sprintf("%s/repos/%s/releases/latest", opts.APIBaseURL, opts.Repo)
	} else {
		url = fmt.Sprintf("%s/repos/%s/releases/tags/%s", opts.APIBaseURL, opts.Repo, normalizeTag(version))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "mygardenworld-updater")
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("fetch release: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &release, nil
}

func pickAsset(release *githubRelease, goos, goarch string) (githubAsset, error) {
	suffix := fmt.Sprintf("_%s_%s", goos, goarch)
	wantExt := ".tar.gz"
	if goos == "windows" {
		wantExt = ".zip"
	}
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, suffix) && strings.HasSuffix(asset.Name, wantExt) {
			return asset, nil
		}
	}
	return githubAsset{}, fmt.Errorf("no release asset for %s/%s in %s", goos, goarch, release.TagName)
}

func shouldUpdate(current, target, requested string) bool {
	current = strings.TrimSpace(current)
	target = strings.TrimSpace(target)
	if current == "" || current == "dev" || current == "unknown" {
		return true
	}
	if strings.EqualFold(requested, "latest") || strings.TrimSpace(requested) == "" {
		return compareVersions(target, current) > 0
	}
	return normalizeTag(current) != normalizeTag(target)
}

func compareVersions(a, b string) int {
	ap := versionParts(a)
	bp := versionParts(b)
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = strings.TrimPrefix(normalizeTag(v), "v")
	v = strings.SplitN(v, "-", 2)[0]
	pieces := strings.Split(v, ".")
	out := make([]int, 0, len(pieces))
	for _, piece := range pieces {
		n, err := strconv.Atoi(piece)
		if err != nil {
			break
		}
		out = append(out, n)
	}
	return out
}

func normalizeTag(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return version
	}
	if strings.HasPrefix(version, "v") {
		return version
	}
	return "v" + version
}

func downloadFile(ctx context.Context, opts Options, url, dst string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "mygardenworld-updater")
	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write archive: %w", err)
	}
	return nil
}

func verifyChecksum(ctx context.Context, opts Options, release *githubRelease, assetName, archivePath string) (bool, error) {
	var checksumAsset githubAsset
	for _, asset := range release.Assets {
		if asset.Name == "checksums.txt" {
			checksumAsset = asset
			break
		}
	}
	if checksumAsset.Name == "" || checksumAsset.BrowserDownloadURL == "" {
		return false, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumAsset.BrowserDownloadURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "mygardenworld-updater")
	resp, err := opts.HTTPClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("download checksums: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("download checksums: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, fmt.Errorf("read checksums: %w", err)
	}
	want := checksumForAsset(string(body), assetName)
	if want == "" {
		return false, fmt.Errorf("checksums.txt has no entry for %s", assetName)
	}
	got, err := fileSHA256(archivePath)
	if err != nil {
		return false, err
	}
	if !strings.EqualFold(got, want) {
		return false, fmt.Errorf("checksum mismatch for %s: got %s want %s", assetName, got, want)
	}
	return true, nil
}

func checksumForAsset(checksums, assetName string) string {
	for _, line := range strings.Split(checksums, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[len(fields)-1] == assetName || pathBase(fields[len(fields)-1]) == assetName {
			return fields[0]
		}
	}
	return ""
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open archive for checksum: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("hash archive: %w", err)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func extractBinary(archivePath, dstDir, binaryName string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractBinaryFromZip(archivePath, dstDir, binaryName)
	}
	if strings.HasSuffix(archivePath, ".tar.gz") {
		return extractBinaryFromTarGz(archivePath, dstDir, binaryName)
	}
	return "", fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
}

func extractBinaryFromZip(archivePath, dstDir, binaryName string) (string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if !entryMatchesBinary(f.Name, binaryName) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", fmt.Errorf("open zip entry: %w", err)
		}
		defer rc.Close()
		return writeExtractedBinary(rc, dstDir, binaryName)
	}
	return "", fmt.Errorf("%s not found in %s", binaryName, filepath.Base(archivePath))
}

func extractBinaryFromTarGz(archivePath, dstDir, binaryName string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", fmt.Errorf("open tar.gz: %w", err)
	}
	defer file.Close()
	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("open gzip: %w", err)
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar: %w", err)
		}
		if header.Typeflag != tar.TypeReg || !entryMatchesBinary(header.Name, binaryName) {
			continue
		}
		return writeExtractedBinary(tr, dstDir, binaryName)
	}
	return "", fmt.Errorf("%s not found in %s", binaryName, filepath.Base(archivePath))
}

func entryMatchesBinary(entryName, binaryName string) bool {
	entryName = filepath.ToSlash(entryName)
	base := pathBase(entryName)
	return base == binaryName || normalizeExeName(base) == normalizeExeName(binaryName)
}

func pathBase(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

func normalizeExeName(name string) string {
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		return name + ".exe"
	}
	return name
}

func writeExtractedBinary(src io.Reader, dstDir, binaryName string) (string, error) {
	dst := filepath.Join(dstDir, "extracted-"+binaryName)
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return "", fmt.Errorf("create extracted binary: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, src); err != nil {
		return "", fmt.Errorf("write extracted binary: %w", err)
	}
	return dst, nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, perm)
}

func replaceBinary(targetPath, stagedPath, backupPath string) (bool, error) {
	if runtime.GOOS == "windows" && samePath(targetPath, currentExecutable()) {
		return true, scheduleWindowsReplacement(targetPath, stagedPath, backupPath)
	}
	_ = os.Remove(backupPath)
	if err := os.Rename(targetPath, backupPath); err != nil {
		return false, fmt.Errorf("backup current binary: %w", err)
	}
	if err := os.Rename(stagedPath, targetPath); err != nil {
		_ = os.Rename(backupPath, targetPath)
		return false, fmt.Errorf("install new binary: %w", err)
	}
	return false, nil
}

func currentExecutable() string {
	exe, _ := os.Executable()
	if abs, err := filepath.Abs(exe); err == nil {
		return abs
	}
	return exe
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA == nil {
		a = aa
	}
	if errB == nil {
		b = bb
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func scheduleWindowsReplacement(targetPath, stagedPath, backupPath string) error {
	scriptPath := stagedPath + ".ps1"
	script := fmt.Sprintf(`
$ErrorActionPreference = "Stop"
$pidToWait = %d
$target = %q
$staged = %q
$backup = %q
Wait-Process -Id $pidToWait -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 300
if (Test-Path -LiteralPath $backup) { Remove-Item -LiteralPath $backup -Force }
if (Test-Path -LiteralPath $target) { Move-Item -LiteralPath $target -Destination $backup -Force }
Move-Item -LiteralPath $staged -Destination $target -Force
Remove-Item -LiteralPath $MyInvocation.MyCommand.Path -Force
`, os.Getpid(), targetPath, stagedPath, backupPath)
	if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
		return fmt.Errorf("write update script: %w", err)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", scriptPath)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Start()
}

func ContextWithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	return context.WithTimeout(parent, timeout)
}
