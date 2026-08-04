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
	"net/url"
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
		Short:         "小云朵 local automation daemon",
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
		authWindow    time.Duration
		authLockout   time.Duration
		authUserFails int
		authIPFails   int
		maxReqBytes   int
		insecureCORS  bool
		insecureDebug bool
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
				AuthWindow:    authWindow,
				AuthLockout:   authLockout,
				AuthUserFails: authUserFails,
				AuthIPFails:   authIPFails,
				MaxReqBytes:   maxReqBytes,
				InsecureCORS:  insecureCORS,
				InsecureDebug: insecureDebug,
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
	cmd.Flags().DurationVar(&authWindow, "auth-login-window", 10*time.Minute, "login failure counting window")
	cmd.Flags().IntVar(&authUserFails, "auth-user-failures", 5, "failed logins per username before temporary lockout")
	cmd.Flags().IntVar(&authIPFails, "auth-ip-failures", 30, "failed logins per remote IP before temporary lockout")
	cmd.Flags().DurationVar(&authLockout, "auth-lockout", 15*time.Minute, "login lockout duration after too many failures")
	cmd.Flags().IntVar(&maxReqBytes, "max-request-bytes", 1048576, "maximum Connect request message size in bytes (0=unlimited)")
	cmd.Flags().BoolVar(&insecureCORS, "allow-insecure-cors", false, "allow --cors-origins '*'")
	cmd.Flags().BoolVar(&insecureDebug, "allow-insecure-debug", false, "allow --debug-dir while listening on a non-loopback address")
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
	AuthWindow    time.Duration
	AuthLockout   time.Duration
	AuthUserFails int
	AuthIPFails   int
	MaxReqBytes   int
	InsecureCORS  bool
	InsecureDebug bool
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
	originPolicy, err := newOriginPolicy(opts.CORSOrigins, opts.InsecureCORS)
	if err != nil {
		return err
	}
	if err := validateServeSecurity(opts); err != nil {
		return err
	}
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
	maintenanceCtx, cancelMaintenance := context.WithCancel(ctx)
	maintenanceDone := make(chan struct{})
	go func() {
		defer close(maintenanceDone)
		runLogCleanupLoop(maintenanceCtx, db, log)
	}()
	defer func() {
		cancelMaintenance()
		<-maintenanceDone
	}()

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
		LoginLimiter: apiserver.NewLoginLimiter(apiserver.LoginLimiterConfig{
			Window:       opts.AuthWindow,
			UserFailures: opts.AuthUserFails,
			IPFailures:   opts.AuthIPFails,
			Lockout:      opts.AuthLockout,
		}),
	}

	authInterceptor := auth.NewInterceptor(jwtSvc)
	protectedOpts := []connect.HandlerOption{
		connect.WithInterceptors(authInterceptor),
		connect.WithReadMaxBytes(opts.MaxReqBytes),
	}

	mux := http.NewServeMux()

	// AuthService uses the same interceptor: login/refresh/logout are
	// explicitly public, while get-me still receives identity context.
	path, handler := mygardenworldv1connect.NewAuthServiceHandler(svc, protectedOpts...)
	mux.Handle(path, handler)

	// All other services: protected
	for _, mounter := range []func() (string, http.Handler){
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewAccountServiceHandler(svc, protectedOpts...)
		},
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewAutomationServiceHandler(svc, protectedOpts...)
		},
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewPolicyServiceHandler(svc, protectedOpts...)
		},
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewQueryServiceHandler(svc, protectedOpts...)
		},
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewAdminServiceHandler(svc, protectedOpts...)
		},
	} {
		p, h := mounter()
		mux.Handle(p, h)
	}
	if opts.WebEnabled {
		mux.Handle("/", webui.Handler())
	}

	var handler2 http.Handler = mux
	handler2 = corsMiddleware(handler2, originPolicy)
	handler2 = originGuardMiddleware(handler2, originPolicy)
	handler2 = securityHeadersMiddleware(handler2)

	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	server := &http.Server{
		Handler:           handler2,
		Protocols:         protocols,
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    64 << 10,
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

	restoreCtx, cancelRestore := context.WithCancel(ctx)
	defer cancelRestore()
	restoreDone := make(chan struct{})
	go func() {
		defer close(restoreDone)
		report := mgr.RestoreEnabledRunners(restoreCtx)
		if report.Eligible > 0 || report.Failed > 0 || report.Skipped > 0 {
			log.Info("auto-start restore finished",
				"eligible", report.Eligible,
				"started", report.Started,
				"failed", report.Failed,
				"skipped", report.Skipped,
			)
		}
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

	cancelRestore()
	<-restoreDone

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
	if err := apiserver.ValidatePassword(opts.AdminPassword); err != nil {
		return err
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

func validateServeSecurity(opts serveOpts) error {
	if opts.MaxReqBytes < 0 {
		return errors.New("--max-request-bytes cannot be negative")
	}
	if opts.DebugDir != "" && !opts.InsecureDebug && !isLoopbackListenAddr(opts.ListenAddr) {
		return errors.New("--debug-dir cannot be used with a non-loopback --listen address unless --allow-insecure-debug is set")
	}
	return nil
}

type originPolicy struct {
	allowedOrigins map[string]struct{}
	allowAnyOrigin bool
}

func newOriginPolicy(origins string, allowInsecureAny bool) (originPolicy, error) {
	policy := originPolicy{allowedOrigins: make(map[string]struct{})}
	for origin := range strings.SplitSeq(origins, ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			if !allowInsecureAny {
				return originPolicy{}, errors.New("--cors-origins '*' requires --allow-insecure-cors")
			}
			policy.allowAnyOrigin = true
			continue
		}
		normalized, err := canonicalOrigin(origin)
		if err != nil {
			return originPolicy{}, fmt.Errorf("invalid CORS origin %q: %w", origin, err)
		}
		policy.allowedOrigins[normalized] = struct{}{}
	}
	return policy, nil
}

func corsMiddleware(next http.Handler, policy originPolicy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			if policy.allows(r, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
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

func originGuardMiddleware(next http.Handler, policy originPolicy) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && !policy.allows(r, origin) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'self'; object-src 'none'")
		next.ServeHTTP(w, r)
	})
}

func (p originPolicy) allows(r *http.Request, origin string) bool {
	if p.allowAnyOrigin {
		return true
	}
	normalized, err := canonicalOrigin(origin)
	if err != nil {
		return false
	}
	if _, ok := p.allowedOrigins[normalized]; ok {
		return true
	}
	return isSameOrigin(r, normalized)
}

func canonicalOrigin(origin string) (string, error) {
	u, err := url.Parse(origin)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("origin scheme must be http or https")
	}
	if u.Host == "" {
		return "", errors.New("origin host is required")
	}
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("origin must not include path, query, or fragment")
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host), nil
}

func isSameOrigin(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if !strings.EqualFold(u.Scheme, requestScheme(r)) {
		return false
	}
	originHost, originPort := splitHostPortForCompare(u.Scheme, u.Host)
	requestHost, requestPort := splitHostPortForCompare(requestScheme(r), r.Host)
	return originHost != "" && originHost == requestHost && originPort == requestPort
}

func splitHostPortForCompare(scheme, hostport string) (string, string) {
	host, port, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
		port = ""
	}
	host = strings.ToLower(strings.Trim(host, "[]"))
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return host, port
}

func requestScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return "https"
	}
	if forwarded := strings.ToLower(r.Header.Get("Forwarded")); strings.Contains(forwarded, "proto=https") {
		return "https"
	}
	return "http"
}

func isLoopbackListenAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.TrimSpace(strings.Trim(host, "[]"))
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
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
