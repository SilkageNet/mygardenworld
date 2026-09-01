// Package runner owns the per-account lifecycle: HTTP login + WebSocket
// connection + state tracker + automation loop + event broadcast. The
// gRPC server creates one runner per account on demand and keeps them in
// a Manager.
package runner

import (
	"log/slog"
	"strings"
	"sync"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Event mirrors the proto Event for in-process broadcast. Stored shape is
// stable; workspace subscribers receive it through the protobuf WebSocket.
type Event struct {
	ID          int64
	TS          time.Time
	AccountID   int64
	AccountName string
	Kind        string
	Message     string
	PayloadJSON string
	Category    string
	Domain      string
	Action      string
	Label       string
	Level       string
}

// ToProto converts the in-process event to the wire format.
func (e Event) ToProto() *pb.Event {
	return &pb.Event{
		Id:          e.ID,
		Ts:          timestamppb.New(e.TS),
		AccountId:   e.AccountID,
		AccountName: e.AccountName,
		Kind:        e.Kind,
		Message:     e.Message,
		PayloadJson: e.PayloadJSON,
		Category:    WorkspaceLogCategory(e.Category, e.Domain),
		Label:       e.Label,
		Level:       e.Level,
		Domain:      e.Domain,
		Action:      e.Action,
	}
}

// WorkspaceLogCategory maps the runner's detailed implementation taxonomy to
// the stable product taxonomy exposed by the workspace protocol.
func WorkspaceLogCategory(category, domain string) pb.WorkspaceLogCategory {
	category = strings.ToLower(category)
	domain = strings.ToLower(domain)
	switch {
	case category == "account", strings.HasPrefix(domain, "account"), strings.HasPrefix(domain, "session"):
		return pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_ACCOUNT
	case category == "system", strings.HasPrefix(domain, "system"), strings.HasPrefix(domain, "policy"), strings.HasPrefix(domain, "redeem"):
		return pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_SYSTEM
	case category == "union", category == "race", strings.HasPrefix(domain, "fml"), strings.Contains(domain, "union"), strings.Contains(domain, "race"):
		return pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_UNION
	case category == "activity", strings.HasPrefix(domain, "act"), strings.Contains(domain, "cyclic"):
		return pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_ACTIVITIES
	case category == "order", category == "flower_art", strings.Contains(domain, "order"), strings.Contains(domain, "vase"), strings.Contains(domain, "flowerart"):
		return pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_ORDERS
	case strings.Contains(domain, "pearl"):
		return pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_BASIC
	case category == "plant", category == "water", strings.HasPrefix(domain, "farm"), strings.Contains(domain, "land"), strings.Contains(domain, "cultivat"), strings.Contains(domain, "friend"):
		return pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_GARDEN
	case strings.Contains(domain, "inventory"), strings.Contains(domain, "resource"), strings.Contains(domain, "warehouse"), strings.Contains(domain, "market"), strings.Contains(domain, "shop"), strings.Contains(domain, "zoo"), strings.Contains(domain, "mail"):
		return pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_WAREHOUSE
	default:
		return pb.WorkspaceLogCategory_WORKSPACE_LOG_CATEGORY_BASIC
	}
}

// Runner owns the live game session for a single account.
//
// The protocol Config is per-account: it's resolved from `account.Channel`
// at start time so iOS and Alipay sessions can run side-by-side and we
// don't smuggle a "default" host map into something that should be a hard
// per-account choice.
type Runner struct {
	cfg     babigame.Config
	db      *store.DB
	account *store.Account
	log     *slog.Logger

	mu          sync.RWMutex
	state       *state.State
	policy      *pb.Policy
	stats       *RuntimeStats
	lastEventAt time.Time
	bus         *Bus
	startSource StartSource

	sessionRuntimeState
	schedulerState
	executionState
}

// New constructs a runner. cfg must already be resolved from the account's
// channel via babigame.ConfigForChannel; the daemon does that in
// Manager.StartWithSource.
func New(cfg babigame.Config, db *store.DB, account *store.Account, bus *Bus, log *slog.Logger) *Runner {
	r := &Runner{
		cfg:     cfg,
		db:      db,
		account: account,
		log:     log.With("account", account.Name, "channel", account.Channel),
		state:   state.New(),
		policy:  automation.DefaultPolicy(),
		stats:   newRuntimeStats(time.Now()),
		bus:     bus,
	}
	r.harvestBlockedUntil = make(map[int32]time.Time)
	r.operationCooldowns = make(map[string]operationCooldown)
	r.cultivateUpgradeRejects = make(map[int32]cultivateUpgradeResourceObservation)
	r.sideLaneFirstWait = make(map[string]time.Time)
	r.unknownRPCCounts = make(map[string]int32)
	r.lastCustomerOrderInfo = make(map[int32]string)
	r.done = make(chan struct{})
	return r
}

// State returns the current per-account state tracker.
func (r *Runner) State() *state.State { return r.state }

// Account returns the cached account row.
func (r *Runner) Account() *store.Account { return r.account }

// Connected returns whether a live WebSocket is held.
func (r *Runner) Connected() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client != nil && !r.client.Closed() && !r.sessionInvalidated
}

func (r *Runner) LastEventAt() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastEventAt
}

func (r *Runner) RuntimeStats() RuntimeStatsSnapshot {
	r.mu.RLock()
	stats := r.stats
	r.mu.RUnlock()
	if stats == nil {
		return RuntimeStatsSnapshot{}
	}
	return stats.Snapshot()
}

func (r *Runner) Emit(e Event) {
	r.emit(e)
}

func (r *Runner) isSessionInvalidated() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessionInvalidated
}

// Policy returns a deep copy of the current effective policy.
func (r *Runner) Policy() *pb.Policy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.policy == nil {
		return automation.DefaultPolicy()
	}
	return policycfg.Clone(r.policy)
}

// SetPolicy replaces the live policy. Callers persist normal updates; a
// lifecycle fail-closed transition may additionally persist its effective
// automation-disabled policy.
func (r *Runner) SetPolicy(p *pb.Policy) {
	normalized := policycfg.Normalize(p)
	r.mu.Lock()
	r.policy = normalized
	if !normalized.GetAutomationEnabled() {
		r.resetSideLaneFairnessLocked()
	}
	stopPendingRelogin := r.sessionAutoRelogin &&
		!normalized.GetBasic().GetDisplacedSessionReloginEnabled()
	r.mu.Unlock()
	if stopPendingRelogin {
		r.failClosedPendingDisplacedRelogin()
	}
}
