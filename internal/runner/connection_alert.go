package runner

import (
	"context"
	"sync"
	"time"
)

const connectionAlertDelay = time.Minute

// watchConnectionRecovery observes one recovery episode independently of slow
// network probes. Its stop function joins the observer before a recovery event
// can be emitted, so a late timer cannot reopen a recovered incident.
func (r *Runner) watchConnectionRecovery(ctx context.Context) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		nextAlert := time.Now().Add(connectionAlertDelay)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if ctx.Err() != nil || r.isSessionInvalidated() {
					return // These have dedicated alerts, not a transport incident.
				}
				if r.restrictionError() != nil {
					nextAlert = time.Now().Add(connectionAlertDelay)
					continue
				}
				if r.gameGate != nil {
					if blocked, _ := r.gameGate.status(); blocked {
						return
					}
				}
				if time.Now().Before(nextAlert) {
					continue
				}
				nextAlert = time.Now().Add(connectionAlertDelay)
				r.emit(Event{Kind: "connection_unavailable", Category: "account", Domain: "account.connection", Action: "unavailable",
					Label: "连接异常", Level: "warn", Message: "游戏连接持续不可用超过 60 秒，正在自动重连"})
				// Durable notification incidents apply the user's cooldown. Keep
				// observing so a long outage can remind without every retry alerting.
			}
		}
	}()
	return sync.OnceFunc(func() { cancel(); <-done })
}

func (r *Runner) emitConnectionRecovered() {
	// Emit after every successful startup as well: an incident persisted before
	// a daemon restart must close, but the outbox ignores unmatched recoveries.
	r.emit(Event{Kind: "connection_recovered", Category: "account", Domain: "account.connection", Action: "recovered",
		Label: "连接恢复", Level: "info", Message: "游戏连接已建立，按当前配置运行"})
}
