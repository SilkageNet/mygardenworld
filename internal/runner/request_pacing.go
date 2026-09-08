package runner

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

// RequestPacing is a local precaution, not a discovered server rate limit.
// Decisions can still run every four seconds; actual requests, including
// executor-internal loops and explicit commands, share these account limits.
type RequestPacing struct {
	RequestInterval  time.Duration
	RepeatInterval   time.Duration
	PurchaseInterval time.Duration
}

func (p RequestPacing) defaults() RequestPacing {
	if p.RequestInterval == 0 {
		p.RequestInterval = 2 * time.Second
	}
	if p.RepeatInterval == 0 {
		p.RepeatInterval = 8 * time.Second
	}
	if p.PurchaseInterval == 0 {
		p.PurchaseInterval = 30 * time.Second
	}
	return p
}

func (p RequestPacing) Validate() error {
	p = p.defaults()
	if p.RequestInterval < time.Second || p.RequestInterval > 30*time.Second ||
		p.RepeatInterval < p.RequestInterval || p.RepeatInterval > 5*time.Minute ||
		p.PurchaseInterval < p.RepeatInterval || p.PurchaseInterval > 30*time.Minute {
		return fmt.Errorf("game pacing requires request 1s..30s, repeat >= request and <=5m, purchase >= repeat and <=30m")
	}
	return nil
}

type requestPacer struct {
	mu          sync.Mutex
	config      RequestPacing
	lastRequest time.Time
	lastScope   map[string]time.Time
	lastSpacing map[string]time.Duration
}

func newRequestPacer(p RequestPacing) *requestPacer {
	return &requestPacer{config: p.defaults(), lastScope: make(map[string]time.Time), lastSpacing: make(map[string]time.Duration)}
}

func (p *requestPacer) scope(name string) (string, time.Duration) {
	// All purchase targets in a shop share one namespace reservation. Changing
	// shopId/itemId, response success/failure, or runner reload cannot bypass it.
	group, method, _ := strings.Cut(name, ".")
	if strings.HasPrefix(strings.ToLower(method), "buy") || name == clientproto.RPCPearlPlaceHire.String() {
		return group + ".purchase", p.config.PurchaseInterval
	}
	return name, p.config.RepeatInterval
}

func (p *requestPacer) delayLocked(name string, now time.Time) time.Duration {
	scope, interval := p.scope(name)
	return max(0, p.lastRequest.Add(p.config.RequestInterval).Sub(now), p.lastScope[scope].Add(interval).Sub(now))
}

func (p *requestPacer) delay(name string, now time.Time) time.Duration {
	if p == nil || name == clientproto.RPCUsrHeartTick.String() {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.delayLocked(name, now)
}

func (p *requestPacer) wait(ctx context.Context, name string, guard func() error) error {
	// Keepalive has its own timer and must not queue behind a purchase. The
	// account protection/maintenance guards still apply to heartbeats.
	if p == nil || name == clientproto.RPCUsrHeartTick.String() {
		return guard()
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := guard(); err != nil {
			return err
		}
		p.mu.Lock()
		now := time.Now()
		delay := p.delayLocked(name, now)
		if delay <= 0 {
			scope, _ := p.scope(name)
			if previous := p.lastScope[scope]; !previous.IsZero() {
				p.lastSpacing[scope] = now.Sub(previous)
			}
			p.lastRequest, p.lastScope[scope] = now, now
			p.mu.Unlock()
			return guard() // Recheck protection after waiting, before sending.
		}
		p.mu.Unlock()
		if !sleepOrDone(ctx, delay) {
			return ctx.Err()
		}
	}
}

func (p *requestPacer) diagnostic(name string) map[string]any {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	scope, minimum := p.scope(name)
	return map[string]any{"scope": scope, "minimum_interval_ms": minimum.Milliseconds(), "last_admitted_at": p.lastScope[scope], "previous_interval_ms": p.lastSpacing[scope].Milliseconds()}
}
