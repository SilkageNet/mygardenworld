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

	lastMiscTick      time.Time // 节流 misc 操作
	lastCultivateTick time.Time // 节流培育操作
	lastWaterSyncTick time.Time // 节流水资源状态刷新
	lastEventAt       time.Time // latest event emitted by this runner

	landUnlockBlocked     bool // 上次尝试无效果，等待条件变化
	taskRecvBlocked       bool
	storyUnlockBlocked    bool
	waterBlocked          bool                         // 水滴不足，冷却后重试
	waterBlockedUntil     time.Time                    // 缺水后下一次允许试探浇水的时间
	harvestBlockedUntil   map[int32]time.Time          // 服务端提示未成熟后，按田地短期冷却
	freeWaterBlockedUntil time.Time                    // freeWater.recv 失败后的下一次试探时间
	dailyTaskBlockedUntil time.Time                    // taskDly.recv 失败后的下一次试探时间
	flowerUpgradeBlocked  map[int32]flowerUpgradeBlock // 升级材料不足，等待材料变化或短期冷却
	cultivateBlocked      map[int32]time.Time          // 培育材料不足或配置未知，短期冷却
	prevLevel             int32                        // 用于检测升级

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
		cfg:                  cfg,
		db:                   db,
		account:              account,
		log:                  log.With("account", account.Name, "channel", account.Channel),
		state:                state.New(),
		policy:               automation.DefaultPolicy(),
		harvestBlockedUntil:  make(map[int32]time.Time),
		flowerUpgradeBlocked: make(map[int32]flowerUpgradeBlock),
		cultivateBlocked:     make(map[int32]time.Time),
		done:                 make(chan struct{}),
		bus:                  bus,
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
