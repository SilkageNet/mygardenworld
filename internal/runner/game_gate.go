package runner

import (
	"context"
	"errors"
	"sync"
)

var ErrMaintenance = errors.New("系统维护中，游戏连接与操作已暂停，用户配置保持不变")

// gameGate admits and tracks complete game I/O lifetimes, not just runners
// already inserted into Manager. Cancellation also covers identity probes,
// QR polling, pending starts, manual RPCs and reconnect HTTP requests.
type gameGate struct {
	mu      sync.Mutex
	blocked bool
	next    uint64
	work    map[uint64]context.CancelFunc
	idle    chan struct{}
}

func (g *gameGate) begin(ctx context.Context) (context.Context, func(), error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.blocked {
		return nil, nil, ErrMaintenance
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if len(g.work) == 0 {
		g.idle = make(chan struct{})
	}
	if g.work == nil {
		g.work = make(map[uint64]context.CancelFunc)
	}
	g.next++
	id := g.next
	workCtx, cancel := context.WithCancel(ctx)
	g.work[id] = cancel
	var once sync.Once
	return workCtx, func() {
		once.Do(func() {
			cancel()
			g.mu.Lock()
			defer g.mu.Unlock()
			delete(g.work, id)
			if len(g.work) == 0 {
				close(g.idle)
			}
		})
	}, nil
}

func (g *gameGate) block() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.blocked = true
	for _, cancel := range g.work {
		cancel()
	}
}

func (g *gameGate) wait(ctx context.Context) error {
	g.mu.Lock()
	idle, count := g.idle, len(g.work)
	g.mu.Unlock()
	if count == 0 {
		return nil
	}
	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *gameGate) status() (bool, int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.blocked, len(g.work)
}

func (g *gameGate) open() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.blocked = false
}

// BeginGameWork is for daemon-owned external game I/O. Every successful begin
// must be paired with release after the I/O has returned, including failures.
func (m *Manager) BeginGameWork(ctx context.Context) (context.Context, func(), error) {
	return m.gameGate.begin(ctx)
}

func (r *Runner) beginGameWork(ctx context.Context) (context.Context, func(), error) {
	if r.gameGate == nil {
		return ctx, func() {}, ctx.Err()
	}
	return r.gameGate.begin(ctx)
}
