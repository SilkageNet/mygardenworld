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
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1/mygardenworldv1connect"
	"github.com/SilkageNet/mygardenworld/internal/apiserver"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	redeemsvc "github.com/SilkageNet/mygardenworld/internal/redeem"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"github.com/SilkageNet/mygardenworld/internal/webui"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

func newServeCmd() *cobra.Command {
	var (
		dataDir          string
		listenAddr       string
		logFormat        string
		logLevel         string
		jwtSecret        string
		adminUsername    string
		adminPassword    string
		adminEmail       string
		corsOrigins      string
		debugDir         string
		authWindow       time.Duration
		authLockout      time.Duration
		authUserFails    int
		authIPFails      int
		maxReqBytes      int
		insecureCORS     bool
		insecureDebug    bool
		webEnabled       bool
		logRetentionDays int
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
				DataDir:          dataDir,
				ListenAddr:       listenAddr,
				LogFormat:        logFormat,
				LogLevel:         logLevel,
				JWTSecret:        jwtSecret,
				AdminUsername:    adminUsername,
				AdminPassword:    adminPassword,
				AdminEmail:       adminEmail,
				CORSOrigins:      corsOrigins,
				DebugDir:         debugDir,
				AuthWindow:       authWindow,
				AuthLockout:      authLockout,
				AuthUserFails:    authUserFails,
				AuthIPFails:      authIPFails,
				MaxReqBytes:      maxReqBytes,
				InsecureCORS:     insecureCORS,
				InsecureDebug:    insecureDebug,
				WebEnabled:       webEnabled,
				LogRetentionDays: logRetentionDays,
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
	cmd.Flags().IntVar(&logRetentionDays, "log-retention-days", defaultLogRetentionDays, "days to retain event and operation logs (0=keep forever)")
	return cmd
}

type serveOpts struct {
	DataDir          string
	ListenAddr       string
	LogFormat        string
	LogLevel         string
	JWTSecret        string
	AdminUsername    string
	AdminPassword    string
	AdminEmail       string
	CORSOrigins      string
	DebugDir         string
	AuthWindow       time.Duration
	AuthLockout      time.Duration
	AuthUserFails    int
	AuthIPFails      int
	MaxReqBytes      int
	InsecureCORS     bool
	InsecureDebug    bool
	WebEnabled       bool
	LogRetentionDays int
}

func generateRandomSecret(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func runServe(ctx context.Context, opts serveOpts) error {
	log := buildLogger(opts.LogFormat, opts.LogLevel)
	logRetention, err := logRetentionDuration(opts.LogRetentionDays)
	if err != nil {
		return err
	}
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
		runLogCleanupLoop(maintenanceCtx, db, log, logRetention)
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
	redeemService, err := redeemsvc.NewService(ctx, db, mgr, log)
	if err != nil {
		return fmt.Errorf("initialize redeem exchange: %w", err)
	}
	redeemCtx, cancelRedeem := context.WithCancel(ctx)
	redeemDone := make(chan struct{})
	go func() {
		defer close(redeemDone)
		redeemService.Run(redeemCtx)
	}()
	defer func() {
		cancelRedeem()
		<-redeemDone
	}()
	alipayCfg, err := babigame.ConfigForChannel(babigame.ChannelAlipay)
	if err != nil {
		return fmt.Errorf("configure Alipay channel: %w", err)
	}

	svc := &apiserver.Services{
		DB:            db,
		Manager:       mgr,
		JWT:           jwtSvc,
		Log:           log,
		Redeem:        redeemService,
		RedeemLimiter: apiserver.NewRedeemSubmitLimiter(),
		AlipayLogins: apiserver.NewAlipayLoginCoordinator(
			babigame.NewAlipayClient(alipayCfg),
		),
		LoginLimiter: apiserver.NewLoginLimiter(apiserver.LoginLimiterConfig{
			Window:       opts.AuthWindow,
			UserFailures: opts.AuthUserFails,
			IPFailures:   opts.AuthIPFails,
			Lockout:      opts.AuthLockout,
		}),
	}
	handlers := apiserver.NewHandlers(svc)

	authInterceptor := auth.NewInterceptor(jwtSvc, func(ctx context.Context, userID int64) (*auth.Identity, error) {
		user, err := db.GetUserByID(ctx, userID)
		if err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				return nil, auth.ErrTokenInvalid
			}
			return nil, err
		}
		if user.Status != "active" {
			return nil, auth.ErrIdentityDisabled
		}
		return &auth.Identity{UserID: user.ID, Role: user.Role}, nil
	})
	protectedOpts := []connect.HandlerOption{
		connect.WithInterceptors(authInterceptor),
		connect.WithReadMaxBytes(opts.MaxReqBytes),
	}

	mux := http.NewServeMux()
	workspaceOrigins := make([]string, 0, len(originPolicy.allowedOrigins))
	for origin := range originPolicy.allowedOrigins {
		workspaceOrigins = append(workspaceOrigins, origin)
	}
	mux.Handle("/api/workspace", svc.WorkspaceHandler(apiserver.WorkspaceHandlerOptions{
		Context:                ctx,
		OriginPatterns:         workspaceOrigins,
		InsecureAllowAnyOrigin: originPolicy.allowAnyOrigin,
	}))

	// AuthService uses the same interceptor: login/refresh/logout are
	// explicitly public, while get-me still receives identity context.
	path, handler := mygardenworldv1connect.NewAuthServiceHandler(handlers.Auth, protectedOpts...)
	mux.Handle(path, handler)

	// Redeem exchange is intentionally public: it contains only community
	// codes and aggregate verification evidence, never account data.
	path, handler = mygardenworldv1connect.NewRedeemExchangeServiceHandler(
		handlers.Redeem,
		connect.WithReadMaxBytes(opts.MaxReqBytes),
	)
	mux.Handle(path, handler)

	// All other services: protected
	for _, mounter := range []func() (string, http.Handler){
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewAccountServiceHandler(handlers.Account, protectedOpts...)
		},
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewAutomationServiceHandler(handlers.Automation, protectedOpts...)
		},
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewPolicyServiceHandler(handlers.Policy, protectedOpts...)
		},
		func() (string, http.Handler) {
			return mygardenworldv1connect.NewAdminServiceHandler(handlers.Admin, protectedOpts...)
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
	user, err := db.GetUserByUsername(ctx, opts.AdminUsername)
	if err == nil {
		if opts.AdminPassword == "" {
			return nil
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(opts.AdminPassword)); err == nil {
			return nil
		}
		if err := apiserver.ValidatePassword(opts.AdminPassword); err != nil {
			return err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(opts.AdminPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if err := db.UpdateUserPasswordHash(ctx, user.ID, string(hash)); err != nil {
			return err
		}
		log.Info("updated admin password from serve flags", "username", opts.AdminUsername)
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
	user, err = db.CreateUser(ctx, opts.AdminUsername, opts.AdminEmail, string(hash))
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
