// Package runner owns the per-account lifecycle: HTTP login + WebSocket
// connection + state tracker + automation loop + event broadcast. The
// gRPC server creates one runner per account on demand and keeps them in
// a Manager.
package runner

import (
	"context"
	"log/slog"
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
// stable; gRPC subscribers receive it directly via QueryService.StreamEvents.
type Event struct {
	TS          time.Time
	AccountID   string
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
		Ts:          timestamppb.New(e.TS),
		AccountId:   e.AccountID,
		AccountName: e.AccountName,
		Kind:        e.Kind,
		Message:     e.Message,
		PayloadJson: e.PayloadJSON,
		Category:    e.Category,
		Label:       e.Label,
		Level:       e.Level,
		Domain:      e.Domain,
		Action:      e.Action,
	}
}

// Runner owns the live game session for a single account.
//
// The protocol Config is per-account: it's resolved from `account.Platform`
// at start time so iOS and Android sessions can run side-by-side and we
// don't smuggle a "default" host map into something that should be a hard
// per-account choice.
type Runner struct {
	cfg     babigame.Config
	db      *store.DB
	account *store.Account
	log     *slog.Logger

	mu      sync.RWMutex
	client  *babigame.Client
	httpc   *babigame.HTTPClient
	session *babigame.Session
	state   *state.State
	policy  *pb.Policy

	lastWaterSyncTick time.Time // 节流水资源状态刷新
	lastEventAt       time.Time // latest event emitted by this runner
	nextDecisionAt    time.Time // next scheduled decision-loop tick

	currentOperation          string
	currentOperationStartedAt time.Time
	lastOperation             string
	lastOperationAt           time.Time
	lastOperationError        string
	lastOperationErrorAt      time.Time

	harvestBlockedUntil map[int32]time.Time // 服务端提示未成熟后，按田地短期冷却
	rqst                rqstState           // 反作弊验证状态
	unknownRPCCounts    map[string]int32    // runtime RPC names missing from the catalog

	debugWriter *babigame.DebugFrameWriter

	cancel   context.CancelFunc
	done     chan struct{}
	stopOnce sync.Once

	sessionInvalidated       bool
	sessionInvalidatedReason string

	bus *Bus
}

// New constructs a runner. cfg must already be resolved from the account's
// channel via babigame.ConfigForChannel; the daemon does that in
// Manager.Start.
func New(cfg babigame.Config, db *store.DB, account *store.Account, bus *Bus, log *slog.Logger) *Runner {
	return &Runner{
		cfg:                 cfg,
		db:                  db,
		account:             account,
		log:                 log.With("account", account.Name, "channel", account.Channel),
		state:               state.New(),
		policy:              automation.DefaultPolicy(),
		harvestBlockedUntil: make(map[int32]time.Time),
		unknownRPCCounts:    make(map[string]int32),
		done:                make(chan struct{}),
		bus:                 bus,
	}
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

// SetPolicy replaces the live policy. Caller is responsible for persisting.
func (r *Runner) SetPolicy(p *pb.Policy) {
	r.mu.Lock()
	r.policy = policycfg.Normalize(p)
	r.mu.Unlock()
}
