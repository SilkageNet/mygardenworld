package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

// Manager is the daemon-wide collection of runners, keyed by account id.
//
// Manager intentionally has no global babigame.Config: each runner resolves
// its protocol Config from the account row's platform via
// babigame.ConfigForChannel. That keeps host pinning a per-account
// decision (an iOS account and an Alipay account can coexist) and
// guarantees we fail fast in StartWithSource when an account's platform is not
// supported by this build.
type Manager struct {
	db  *store.DB
	bus *Bus
	log *slog.Logger

	DebugDir            string // when non-empty, runners write debug JSONL here
	gameGate            gameGate
	maintenanceMu       sync.Mutex
	maintenanceInitErr  error
	maintenanceRestores sync.WaitGroup
	maintenanceDraining atomic.Bool
	// Pacing is configured once by the daemon before any account starts.
	Pacing RequestPacing
	pacers map[int64]*requestPacer

	mu        sync.RWMutex
	runners   map[int64]*Runner
	opLocks   map[int64]*sync.Mutex
	lastStats map[int64]RuntimeStatsSnapshot
	// lastDiag keeps the final diagnostics after a runner exits so status can
	// still surface session-invalidation (e.g. phone login kick) as 异常
	// instead of a bare offline badge.
	lastDiag             map[int64]Diagnostics
	manualResumeRequired bool
}

const restoreAccountTimeout = 90 * time.Second

var errRestoreIneligible = errors.New("account no longer eligible for automatic restoration")

// StartSource records which product path created a game session. It is carried
// into the first successful session event so operators can distinguish an
// explicit control-panel connection from daemon restore and background redeem
// processing.
type StartSource string

const (
	StartSourceUnspecified       StartSource = ""
	StartSourceDaemonRestore     StartSource = "daemon_restore"
	StartSourceAccountCreate     StartSource = "account_create"
	StartSourceControlPanel      StartSource = "control_panel"
	StartSourceAutomationEnable  StartSource = "automation_enable"
	StartSourceManualOperation   StartSource = "manual_operation"
	StartSourceAlipayLogin       StartSource = "alipay_login"
	StartSourceRedeemAutoConnect StartSource = "redeem_auto_connect"
)

// RestoreReport summarizes one daemon startup auto-restore pass.
type RestoreReport struct {
	Eligible int
	Started  int
	Failed   int
	Skipped  int
}

// NewManager wires up the registry. The daemon serves all platforms; the
// platform → Config mapping is resolved per-account in StartWithSource.
func NewManager(db *store.DB, bus *Bus, log *slog.Logger) *Manager {
	m := &Manager{
		db:        db,
		bus:       bus,
		log:       log,
		runners:   make(map[int64]*Runner),
		opLocks:   make(map[int64]*sync.Mutex),
		lastStats: make(map[int64]RuntimeStatsSnapshot),
		lastDiag:  make(map[int64]Diagnostics),
		pacers:    make(map[int64]*requestPacer),
	}
	if db != nil {
		s, err := db.Maintenance(context.Background())
		m.maintenanceInitErr = err
		if err != nil || s.Enabled {
			m.gameGate.block()
			m.manualResumeRequired = true
		}
	}
	return m
}

// Get returns the runner for an account, or nil when no runner is currently
// active.
func (m *Manager) Get(accountID int64) *Runner {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.runners[accountID]
}

// Bus returns the in-process event bus shared with every runner. Subscribers
// get the union of every runner's events.
func (m *Manager) Bus() *Bus { return m.bus }

// All returns a snapshot of every active runner.
func (m *Manager) All() []*Runner {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*Runner, 0, len(m.runners))
	for _, r := range m.runners {
		out = append(out, r)
	}
	return out
}

// RuntimeStats returns the active run statistics, or the latest stopped
// in-memory snapshot if the runner has already been stopped.
func (m *Manager) RuntimeStats(accountID int64) (RuntimeStatsSnapshot, bool) {
	m.mu.RLock()
	if r := m.runners[accountID]; r != nil {
		m.mu.RUnlock()
		return r.RuntimeStats(), true
	}
	stats, ok := m.lastStats[accountID]
	m.mu.RUnlock()
	return stats, ok
}

// LastDiagnostics returns diagnostics retained after a runner stopped. Used
// when Get returns nil so kick/expiry reasons are not lost.
func (m *Manager) LastDiagnostics(accountID int64) (Diagnostics, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	diag, ok := m.lastDiag[accountID]
	return diag, ok
}

// ClearLastDiagnostics drops any retained stop reason (intentional logout/stop).
func (m *Manager) ClearLastDiagnostics(accountID int64) {
	m.mu.Lock()
	delete(m.lastDiag, accountID)
	m.mu.Unlock()
}

// RestoreEnabledRunners starts every account whose persisted policy says
// automation should be running. It is intended for daemon startup: a normal
// shutdown stops in-memory runners; DisableAutomation, DisconnectAccount, or a
// non-recoverable session invalidation persists automation_enabled=false and
// therefore opts the account out. A displaced live session is recovered inside
// its existing runner and does not participate in daemon-start restoration.
func (m *Manager) RestoreEnabledRunners(ctx context.Context) RestoreReport {
	report := RestoreReport{}
	if !m.BackgroundStartsAllowed() {
		return report
	}
	accounts, err := m.accountsWithAutomationEnabled(ctx)
	if err != nil {
		report.Failed = 1
		m.log.Error("scan auto-start accounts failed", "err", err)
		return report
	}
	report.Eligible = len(accounts)
	for i, acc := range accounts {
		if !m.BackgroundStartsAllowed() {
			report.Skipped += len(accounts) - i
			break
		}
		if err := ctx.Err(); err != nil {
			report.Skipped += len(accounts) - i
			m.log.Info("auto-start restore cancelled", "skipped", report.Skipped, "err", err)
			break
		}
		startCtx, cancel := context.WithTimeout(ctx, restoreAccountTimeout)
		_, err := m.StartWithSource(startCtx, acc.ID, StartSourceDaemonRestore)
		cancel()
		if err != nil {
			if errors.Is(err, errRestoreIneligible) || errors.Is(err, ErrMaintenance) || errors.Is(err, ErrManualResumeRequired) {
				report.Skipped++
				continue
			}
			report.Failed++
			m.log.Warn("auto-start account failed", "account_id", acc.ID, "account", acc.Name, "err", err)
			continue
		}
		report.Started++
		m.log.Info("auto-started account", "account_id", acc.ID, "account", acc.Name)
	}
	return report
}

func (m *Manager) accountsWithAutomationEnabled(ctx context.Context) ([]*store.Account, error) {
	accounts, err := m.db.ListAccounts(ctx, 0)
	if err != nil {
		return nil, err
	}
	out := make([]*store.Account, 0, len(accounts))
	for _, acc := range accounts {
		user, err := m.db.GetUserByID(ctx, acc.UserID)
		if err != nil {
			return nil, fmt.Errorf("load owner for %q: %w", acc.Name, err)
		}
		if user.Status != "active" {
			continue
		}
		rawPolicy, err := m.db.LoadPolicyJSON(ctx, acc.ID)
		if err != nil {
			return nil, fmt.Errorf("load policy for %q: %w", acc.Name, err)
		}
		policy, err := policycfg.FromJSON(rawPolicy)
		if err != nil {
			return nil, fmt.Errorf("load policy for %q: %w", acc.Name, err)
		}
		if policy.GetAutomationEnabled() {
			out = append(out, acc)
		}
	}
	return out, nil
}

// StartWithSource either reuses an existing runner or creates and starts one
// for the account. Login is performed on first start; subsequent calls are
// no-ops and do not emit a second start source. Every caller must identify its
// product path so successful and failed connection attempts remain auditable.
//
// It returns an error when the account's channel is not supported by this
// build. We never fall back to a "default" channel because that would silently
// hit the wrong host fronts.
func (m *Manager) StartWithSource(ctx context.Context, accountID int64, source StartSource) (*Runner, error) {
	ctx, release, err := m.BeginGameWork(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	lock := m.accountLock(accountID)
	lock.Lock()
	defer lock.Unlock()
	return m.start(ctx, accountID, source)
}

func (m *Manager) accountLock(accountID int64) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	lock := m.opLocks[accountID]
	if lock == nil {
		lock = &sync.Mutex{}
		m.opLocks[accountID] = lock
	}
	return lock
}

func (m *Manager) start(ctx context.Context, accountID int64, source StartSource) (*Runner, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	if r, ok := m.runners[accountID]; ok {
		m.mu.Unlock()
		return r, nil
	}
	if m.manualResumeRequired && (source == StartSourceRedeemAutoConnect || source == StartSourceDaemonRestore) {
		m.mu.Unlock()
		return nil, ErrManualResumeRequired
	}
	m.mu.Unlock()

	acc, err := m.db.GetAccountByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	cfg, err := babigame.ConfigForChannel(babigame.Channel(acc.Channel))
	if err != nil {
		return nil, fmt.Errorf("account %q: %w", acc.Name, err)
	}
	r := New(cfg, m.db, acc, m.bus, m.log)
	r.gameGate = &m.gameGate
	m.mu.Lock()
	if m.pacers[accountID] == nil {
		m.pacers[accountID] = newRequestPacer(m.Pacing)
	}
	r.pacer = m.pacers[accountID]
	m.mu.Unlock()
	r.startSource = source
	rawPolicy, err := m.db.LoadPolicyJSON(ctx, acc.ID)
	if err != nil {
		return nil, fmt.Errorf("load policy for %q: %w", acc.Name, err)
	}
	policy, err := policycfg.FromJSON(rawPolicy)
	if err != nil {
		return nil, fmt.Errorf("load policy for %q: %w", acc.Name, err)
	}
	// Restoration candidates may have been listed well before their turn.
	// Re-read intent and owner eligibility immediately before starting I/O.
	if source == StartSourceDaemonRestore && !policy.GetAutomationEnabled() {
		return nil, errRestoreIneligible
	}
	owner, err := m.db.GetUserByID(ctx, acc.UserID)
	if err != nil {
		return nil, err
	}
	if owner.Status != "active" {
		return nil, store.ErrUserInactive
	}
	r.SetPolicy(policy)
	if m.DebugDir != "" {
		path := fmt.Sprintf("%s/%s_debug.jsonl", m.DebugDir, acc.Name)
		dw, err := babigame.NewDebugFrameWriter(path)
		if err != nil {
			m.log.Error("debug writer failed", "path", path, "err", err)
		} else {
			r.debugWriter = dw
		}
	}
	m.log.Info("starting account session", "account_id", acc.ID, "account", acc.Name, "source", source)
	if err := r.Start(ctx); err != nil {
		m.log.Warn("start account session failed", "account_id", acc.ID, "account", acc.Name, "source", source, "err", err)
		return nil, err
	}
	m.mu.Lock()
	m.runners[accountID] = r
	delete(m.lastStats, accountID)
	delete(m.lastDiag, accountID)
	m.mu.Unlock()
	go m.forgetWhenDone(accountID, r)
	return r, nil
}

func (m *Manager) forgetWhenDone(accountID int64, r *Runner) {
	<-r.Done()
	diag := r.Diagnostics(time.Now())
	m.mu.Lock()
	if m.runners[accountID] == r {
		m.lastStats[accountID] = r.RuntimeStats()
		if diag.SessionInvalidatedReason != "" {
			m.lastDiag[accountID] = diag
		}
		delete(m.runners, accountID)
	}
	m.mu.Unlock()
}

// Stop terminates the runner for the account, returning an error when no
// runner is active.
func (m *Manager) Stop(accountID int64) error {
	lock := m.accountLock(accountID)
	lock.Lock()
	defer lock.Unlock()
	return m.stop(accountID)
}

func (m *Manager) stop(accountID int64) error {
	m.mu.Lock()
	r := m.runners[accountID]
	delete(m.runners, accountID)
	delete(m.lastDiag, accountID)
	m.mu.Unlock()
	if r == nil {
		return errors.New("no active runner")
	}
	// Intentional stop/logout should not keep a kick reason as 异常.
	r.discardSessionInvalidation()
	r.Stop()
	m.rememberStats(accountID, r)
	return nil
}

// ReloadWithSource replaces the runner and attributes the new session to
// source.
func (m *Manager) ReloadWithSource(ctx context.Context, accountID int64, source StartSource) (*Runner, error) {
	ctx, release, err := m.BeginGameWork(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	lock := m.accountLock(accountID)
	lock.Lock()
	defer lock.Unlock()
	_ = m.stop(accountID)
	return m.start(ctx, accountID, source)
}

// Shutdown stops every runner. Used at daemon exit.
func (m *Manager) Shutdown() {
	m.gameGate.block()
	m.stopMaintenanceRunners()
	_ = m.gameGate.wait(context.Background())
	m.maintenanceRestores.Wait()
	m.mu.Lock()
	runners := m.runners
	m.runners = make(map[int64]*Runner)
	m.opLocks = make(map[int64]*sync.Mutex)
	m.mu.Unlock()
	for _, r := range runners {
		r.Stop()
	}
}

func (m *Manager) rememberStats(accountID int64, r *Runner) {
	if r == nil {
		return
	}
	m.mu.Lock()
	m.lastStats[accountID] = r.RuntimeStats()
	m.mu.Unlock()
}
