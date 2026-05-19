package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

// Manager is the daemon-wide collection of runners, keyed by account id.
//
// Manager intentionally has no global babigame.Config: each runner resolves
// its protocol Config from the account row's platform via
// babigame.ConfigForPlatform. That keeps host pinning a per-account
// decision (an iOS account and an Android account can coexist) and
// guarantees we fail fast in Start when an account's platform is not
// supported by this build.
type Manager struct {
	db  *store.DB
	bus *Bus
	log *slog.Logger

	DebugDir string // when non-empty, runners write debug JSONL here

	mu      sync.RWMutex
	runners map[int64]*Runner
	opLocks map[int64]*sync.Mutex
}

// NewManager wires up the registry. The daemon serves all platforms; the
// platform → Config mapping is resolved per-account in Start.
func NewManager(db *store.DB, bus *Bus, log *slog.Logger) *Manager {
	return &Manager{
		db:      db,
		bus:     bus,
		log:     log,
		runners: make(map[int64]*Runner),
		opLocks: make(map[int64]*sync.Mutex),
	}
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

// Start either reuses an existing runner or creates+starts a new one for the
// account. Login is performed on first start; subsequent calls are no-ops.
//
// Returns an error when the account's platform is not supported by this
// build. We never fall back to a "default" platform - that would silently
// hit the wrong host fronts.
func (m *Manager) Start(ctx context.Context, accountID int64) (*Runner, error) {
	lock := m.accountLock(accountID)
	lock.Lock()
	defer lock.Unlock()
	return m.start(ctx, accountID)
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

func (m *Manager) start(ctx context.Context, accountID int64) (*Runner, error) {
	m.mu.Lock()
	if r, ok := m.runners[accountID]; ok {
		m.mu.Unlock()
		return r, nil
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
	if entries, err := m.db.LoadPolicyValues(ctx, acc.ID); err == nil {
		r.SetPolicy(policycfg.FromEntries(entries))
	} else {
		m.log.Warn("load policy failed", "account", acc.Name, "err", err)
	}
	if m.DebugDir != "" {
		path := fmt.Sprintf("%s/%s_debug.jsonl", m.DebugDir, acc.Name)
		dw, err := babigame.NewDebugFrameWriter(path)
		if err != nil {
			m.log.Error("debug writer failed", "path", path, "err", err)
		} else {
			r.debugWriter = dw
		}
	}
	if err := r.Start(ctx); err != nil {
		return nil, err
	}
	m.mu.Lock()
	m.runners[accountID] = r
	m.mu.Unlock()
	go m.forgetWhenDone(accountID, r)
	return r, nil
}

func (m *Manager) forgetWhenDone(accountID int64, r *Runner) {
	<-r.Done()
	m.mu.Lock()
	if m.runners[accountID] == r {
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
	m.mu.Unlock()
	if r == nil {
		return errors.New("no active runner")
	}
	r.Stop()
	return nil
}

// Reload tears down and re-spins the runner. Used to apply config drift.
func (m *Manager) Reload(ctx context.Context, accountID int64) (*Runner, error) {
	lock := m.accountLock(accountID)
	lock.Lock()
	defer lock.Unlock()
	_ = m.stop(accountID)
	return m.start(ctx, accountID)
}

// Shutdown stops every runner. Used at daemon exit.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	runners := m.runners
	m.runners = make(map[int64]*Runner)
	m.opLocks = make(map[int64]*sync.Mutex)
	m.mu.Unlock()
	for _, r := range runners {
		r.Stop()
	}
}
