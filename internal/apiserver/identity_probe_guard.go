package apiserver

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

const (
	identityProbeTTL        = 10 * time.Minute
	identityProbeRetryWait  = 30 * time.Second
	identityProbeRiskWait   = 5 * time.Minute
	identityProbeMaxEntries = 1024
)

type identityProbeKey struct {
	userID   int64
	channel  string
	username string
}

type probeDevice struct{ deviceID, uuid string }

type identityProbeEntry struct {
	device           probeDevice
	active           bool
	expires, retryAt time.Time
}

// identityProbeGuard bounds in-flight password probes and retains only device
// identifiers after failure. It never caches credentials or successful sessions,
// and never shares an identity across system users or channels. The local wait
// is anti-repeat protection, NOT a server risk expiry or an automatic retry.
// This process-local guard is deliberately not a durable IP-wide risk policy.
type identityProbeGuard struct {
	mu      sync.Mutex
	entries map[identityProbeKey]*identityProbeEntry
}

func (g *identityProbeGuard) begin(key identityProbeKey, now time.Time) (probeDevice, func(error, time.Time), error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for k, e := range g.entries {
		if !e.active && !now.Before(e.expires) {
			delete(g.entries, k)
		}
	}
	e := g.entries[key]
	if e != nil {
		if e.active {
			return probeDevice{}, nil, probeLimitError("该账号正在验证登录，请勿重复提交")
		}
		if now.Before(e.retryAt) {
			seconds := int(e.retryAt.Sub(now).Seconds()) + 1
			return probeDevice{}, nil, probeLimitError(fmt.Sprintf("登录失败后的本地防重复保护中，请至少 %d 秒后再手动尝试；等待结束不代表游戏风控已解除", seconds))
		}
	} else {
		if len(g.entries) >= identityProbeMaxEntries {
			return probeDevice{}, nil, probeLimitError("登录验证请求过多，请稍后再试")
		}
		if g.entries == nil {
			g.entries = make(map[identityProbeKey]*identityProbeEntry)
		}
		e = &identityProbeEntry{device: probeDevice{babigame.RandomDeviceID(), babigame.RandomUUID()}}
		g.entries[key] = e
	}
	e.active = true
	var once sync.Once
	finish := func(err error, completed time.Time) {
		once.Do(func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			if err == nil {
				delete(g.entries, key)
				return
			}
			e.active = false
			e.expires = completed.Add(identityProbeTTL)
			wait := identityProbeRetryWait
			if babigame.IsLoginRiskError(err) {
				wait = identityProbeRiskWait
			}
			e.retryAt = completed.Add(wait)
		})
	}
	return e.device, finish, nil
}

func probeLimitError(message string) error {
	return connect.NewError(connect.CodeResourceExhausted, errors.New(message))
}

func accountLoginError(err error) error {
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce
	}
	return connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("login: %s", formatLoginErr(err)))
}
