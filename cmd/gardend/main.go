// gardend is the long-running daemon that owns per-account WebSocket sessions
// and exposes a Connect-based control plane with JWT authentication.
//
// Subcommands:
//
//	gardend serve   --data-dir <dir> --listen <addr> --jwt-secret <secret>
//	gardend reset-data --data-dir <dir> --yes
//	gardend version
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/crypto/bcrypt"

	"github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1/mygardenworldv1connect"
	"github.com/SilkageNet/mygardenworld/internal/apiserver"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/buildinfo"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"github.com/SilkageNet/mygardenworld/internal/updatecmd"
	"github.com/SilkageNet/mygardenworld/internal/webui"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:           "gardend",
		Short:         "mygardenworld auto-farmer daemon",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.AddCommand(newServeCmd(), newResetDataCmd(), newVersionCmd(), updatecmd.New("gardend"))
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newServeCmd() *cobra.Command {
	var (
		dataDir       string
		listenAddr    string
		logFormat     string
		logLevel      string
		jwtSecret     string
		adminUsername string
		adminPassword string
		adminEmail    string
		corsOrigins   string
		debugDir      string
		webEnabled    bool
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the API daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jwtSecret == "" {
				jwtSecret = os.Getenv("JWT_SECRET")
			}
			if jwtSecret == "" {
				jwtSecret = generateRandomSecret(32)
			}
			if adminPassword == "" {
				adminPassword = os.Getenv("ADMIN_PASSWORD")
			}
			return runServe(cmd.Context(), serveOpts{
				DataDir:       dataDir,
				ListenAddr:    listenAddr,
				LogFormat:     logFormat,
				LogLevel:      logLevel,
				JWTSecret:     jwtSecret,
				AdminUsername: adminUsername,
				AdminPassword: adminPassword,
				AdminEmail:    adminEmail,
				CORSOrigins:   corsOrigins,
				DebugDir:      debugDir,
				WebEnabled:    webEnabled,
			})
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", defaultAppDir("data"), "directory for SQLite + state files")
	cmd.Flags().StringVar(&listenAddr, "listen", "127.0.0.1:50051", "API listen address (host:port)")
	cmd.Flags().StringVar(&logFormat, "log-format", "text", "log format: text|json")
	cmd.Flags().StringVar(&logLevel, "log-level", "info", "log level: debug|info|warn|error")
	cmd.Flags().StringVar(&jwtSecret, "jwt-secret", "", "JWT signing secret (or JWT_SECRET env)")
	cmd.Flags().StringVar(&adminUsername, "admin-username", "admin", "initial admin username")
	cmd.Flags().StringVar(&adminPassword, "admin-password", "", "initial admin password (or ADMIN_PASSWORD env)")
	cmd.Flags().StringVar(&adminEmail, "admin-email", "admin@localhost", "initial admin email")
	cmd.Flags().StringVar(&corsOrigins, "cors-origins", "http://localhost:3000,http://127.0.0.1:3000", "allowed CORS origins (comma-separated)")
	cmd.Flags().StringVar(&debugDir, "debug-dir", "", "directory for debug JSONL logs (empty=disabled)")
	cmd.Flags().BoolVar(&webEnabled, "web", true, "serve the embedded web console")
	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("gardend %s (commit=%s date=%s)\n", buildinfo.GetVersion(), buildinfo.Commit, buildinfo.Date)
		},
	}
}

func newResetDataCmd() *cobra.Command {
	var (
		dataDir string
		yes     bool
	)
	cmd := &cobra.Command{
		Use:   "reset-data",
		Short: "Delete local SQLite and state data",
		RunE: func(cmd *cobra.Command, args []string) error {
			absDataDir, err := cleanDataDirPath(dataDir)
			if err != nil {
				return err
			}
			if !yes {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Would delete local data directory: %s\n", absDataDir)
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Re-run with --yes to confirm.")
				return nil
			}
			removed, err := removeDataDir(absDataDir)
			if err != nil {
				return err
			}
			if removed {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted local data directory: %s\n", absDataDir)
			} else {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Local data directory did not exist: %s\n", absDataDir)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", defaultAppDir("data"), "directory to delete (same default as serve)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deletion without prompting")
	return cmd
}

type serveOpts struct {
	DataDir       string
	ListenAddr    string
	LogFormat     string
	LogLevel      string
	JWTSecret     string
	AdminUsername string
	AdminPassword string
	AdminEmail    string
	CORSOrigins   string
	DebugDir      string
	WebEnabled    bool
}

func cleanDataDirPath(dataDir string) (string, error) {
	if strings.TrimSpace(dataDir) == "" {
		return "", errors.New("--data-dir cannot be empty")
	}
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return "", fmt.Errorf("resolve data-dir: %w", err)
	}
	absDataDir = filepath.Clean(absDataDir)
	volumeRoot := filepath.VolumeName(absDataDir) + string(os.PathSeparator)
	if samePath(absDataDir, volumeRoot) {
		return "", fmt.Errorf("refusing to reset filesystem root: %s", absDataDir)
	}
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" && samePath(absDataDir, homeDir) {
		return "", fmt.Errorf("refusing to reset home directory: %s", absDataDir)
	}
	if cwd, err := os.Getwd(); err == nil && samePath(absDataDir, cwd) {
		return "", fmt.Errorf("refusing to reset current working directory: %s", absDataDir)
	}
	return absDataDir, nil
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	absA, err := filepath.Abs(a)
	if err != nil {
		absA = a
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		absB = b
	}
	return strings.EqualFold(filepath.Clean(absA), filepath.Clean(absB))
}

// defaultAppDir returns the platform-appropriate default directory for app data.
// Windows: %LOCALAPPDATA%\mygardenworld\<sub>
// macOS:   ~/Library/Application Support/mygardenworld/<sub>
// Linux:   ~/.config/mygardenworld/<sub>
func defaultAppDir(sub string) string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", sub)
	}
	return filepath.Join(dir, "mygardenworld", sub)
}

func generateRandomSecret(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func removeDataDir(absDataDir string) (bool, error) {
	info, err := os.Stat(absDataDir)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat data-dir: %w", err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("data-dir is not a directory: %s", absDataDir)
	}
	if err := os.RemoveAll(absDataDir); err != nil {
		return false, fmt.Errorf("delete data-dir: %w", err)
	}
	return true, nil
}

func runServe(ctx context.Context, opts serveOpts) error {
	log := buildLogger(opts.LogFormat, opts.LogLevel)
	if err := os.MkdirAll(opts.DataDir, 0o755); err != nil {
		return fmt.Errorf("mkdir data-dir: %w", err)
	}
	if opts.DebugDir != "" {
		if err := os.MkdirAll(opts.DebugDir, 0o755); err != nil {
			return fmt.Errorf("mkdir debug-dir: %w", err)
		}
	}
	dbPath := filepath.Join(opts.DataDir, "garden.db")
	db, err := store.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() { _ = db.Close() }()
	log.Info("opened sqlite", "path", dbPath)

	if err := seedAdmin(ctx, db, log, opts); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}

	jwtSvc := auth.NewJWT(opts.JWTSecret)

	bus := runner.NewBus()
	mgr := runner.NewManager(db, bus, log)
	mgr.DebugDir = opts.DebugDir
	defer mgr.Shutdown()

	svc := &apiserver.Services{
		DB:      db,
		Manager: mgr,
		JWT:     jwtSvc,
		Log:     log,
	}

	authInterceptor := auth.NewInterceptor(jwtSvc)
	protectedOpts := connect.WithInterceptors(authInterceptor)

	mux := http.NewServeMux()

	// AuthService uses the same interceptor: login/refresh are
	// explicitly public, while logout/get-me still receive identity context.
	path, handler := mygardenworldv1connect.NewAuthServiceHandler(svc, protectedOpts)
	mux.Handle(path, handler)

	// All other services: protected
	for _, mounter := range []func() (string, http.Handler){
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewAccountServiceHandler(svc, protectedOpts)
		},
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewAutomationServiceHandler(svc, protectedOpts)
		},
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewPolicyServiceHandler(svc, protectedOpts)
		},
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewQueryServiceHandler(svc, protectedOpts)
		},
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewAdminServiceHandler(svc, protectedOpts)
		},
	} {
		p, h := mounter()
		mux.Handle(p, h)
	}
	if opts.WebEnabled {
		mux.Handle("/", webui.Handler())
	}

	var handler2 http.Handler = mux
	handler2 = corsMiddleware(handler2, opts.CORSOrigins)

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	server := &http.Server{
		Handler:           handler2,
		Protocols:         protocols,
		ReadHeaderTimeout: 10 * time.Second,
	}

	lis, err := net.Listen("tcp", opts.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", opts.ListenAddr, err)
	}
	log.Info("gardend listening", "addr", opts.ListenAddr, "data_dir", opts.DataDir)

	errCh := make(chan error, 1)
	go func() {
		err := server.Serve(lis)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-signalCh:
		log.Info("shutdown signal", "signal", sig.String())
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		log.Info("context done")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	return nil
}

func seedAdmin(ctx context.Context, db *store.DB, log *slog.Logger, opts serveOpts) error {
	_, err := db.GetUserByUsername(ctx, opts.AdminUsername)
	if err == nil {
		return nil
	}
	if !errors.Is(err, store.ErrUserNotFound) {
		return err
	}
	if opts.AdminPassword == "" {
		return errors.New("initial admin user is missing; set --admin-password or ADMIN_PASSWORD")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(opts.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user, err := db.CreateUser(ctx, opts.AdminUsername, opts.AdminEmail, string(hash))
	if err != nil {
		return err
	}
	role := "admin"
	maxAccounts := 100
	_, err = db.UpdateUser(ctx, user.ID, &role, &maxAccounts, nil)
	if err != nil {
		return err
	}
	log.Info("seeded admin user", "username", opts.AdminUsername)
	return nil
}

func corsMiddleware(next http.Handler, origins string) http.Handler {
	allowedOrigins := make(map[string]struct{})
	allowAnyOrigin := false
	for _, origin := range strings.Split(origins, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			allowAnyOrigin = true
			continue
		}
		allowedOrigins[origin] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			if allowAnyOrigin {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else if _, ok := allowedOrigins[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Connect-Protocol-Version")
		w.Header().Set("Access-Control-Expose-Headers", "Connect-Protocol-Version")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func buildLogger(format, level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(h)
}
