package apiserver

import (
	"net"
	"strings"
	"sync"
	"time"
)

const (
	defaultLoginWindow       = 10 * time.Minute
	defaultLoginUserFailures = 5
	defaultLoginIPFailures   = 30
	defaultLoginLockout      = 15 * time.Minute
	defaultLoginMaxEntries   = 4096
)

type LoginLimiterConfig struct {
	Window       time.Duration
	UserFailures int
	IPFailures   int
	Lockout      time.Duration
	MaxEntries   int
	Clock        func() time.Time
}

type LoginLimitDecision struct {
	Scope string
	Until time.Time
}

type LoginLimiter struct {
	mu    sync.Mutex
	cfg   LoginLimiterConfig
	users map[string]*loginBucket
	ips   map[string]*loginBucket
}

type loginBucket struct {
	failures     int
	firstFailure time.Time
	lockedUntil  time.Time
	lastSeen     time.Time
}

func NewLoginLimiter(cfg LoginLimiterConfig) *LoginLimiter {
	cfg = normalizeLoginLimiterConfig(cfg)
	return &LoginLimiter{
		cfg:   cfg,
		users: make(map[string]*loginBucket),
		ips:   make(map[string]*loginBucket),
	}
}

func DefaultLoginLimiterConfig() LoginLimiterConfig {
	return normalizeLoginLimiterConfig(LoginLimiterConfig{})
}

func normalizeLoginLimiterConfig(cfg LoginLimiterConfig) LoginLimiterConfig {
	if cfg.Window <= 0 {
		cfg.Window = defaultLoginWindow
	}
	if cfg.UserFailures <= 0 {
		cfg.UserFailures = defaultLoginUserFailures
	}
	if cfg.IPFailures <= 0 {
		cfg.IPFailures = defaultLoginIPFailures
	}
	if cfg.Lockout <= 0 {
		cfg.Lockout = defaultLoginLockout
	}
	if cfg.MaxEntries <= 0 {
		cfg.MaxEntries = defaultLoginMaxEntries
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return cfg
}

func (l *LoginLimiter) Check(username, remoteAddr string) (LoginLimitDecision, bool) {
	if l == nil {
		return LoginLimitDecision{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.cfg.Clock()
	l.pruneExpired(now)
	userKey := normalizeLoginKey(username)
	ipKey := remoteIP(remoteAddr)
	if dec, ok := l.checkBucket(l.users[userKey], "username", now); ok {
		return dec, true
	}
	if dec, ok := l.checkBucket(l.ips[ipKey], "ip", now); ok {
		return dec, true
	}
	return LoginLimitDecision{}, false
}

func (l *LoginLimiter) RecordFailure(username, remoteAddr string) (LoginLimitDecision, bool) {
	if l == nil {
		return LoginLimitDecision{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.cfg.Clock()
	l.pruneExpired(now)
	userKey := normalizeLoginKey(username)
	ipKey := remoteIP(remoteAddr)

	var userDecision LoginLimitDecision
	userLimited := false
	if userKey != "" {
		userDecision, userLimited = l.recordFailure(l.users, userKey, "username", l.cfg.UserFailures, now)
	}
	ipDecision, ipLimited := l.recordFailure(l.ips, ipKey, "ip", l.cfg.IPFailures, now)
	if userLimited {
		return userDecision, true
	}
	if ipLimited {
		return ipDecision, true
	}
	return LoginLimitDecision{}, false
}

func (l *LoginLimiter) RecordSuccess(username string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.users, normalizeLoginKey(username))
}

func (l *LoginLimiter) recordFailure(buckets map[string]*loginBucket, key, scope string, threshold int, now time.Time) (LoginLimitDecision, bool) {
	if threshold <= 0 {
		return LoginLimitDecision{}, false
	}
	l.makeRoom(buckets, now)
	b := buckets[key]
	if b == nil {
		b = &loginBucket{}
		buckets[key] = b
	}
	b.lastSeen = now
	if dec, ok := l.checkBucket(b, scope, now); ok {
		return dec, true
	}
	if b.firstFailure.IsZero() || now.Sub(b.firstFailure) > l.cfg.Window {
		b.failures = 0
		b.firstFailure = now
	}
	b.failures++
	if b.failures >= threshold {
		b.failures = 0
		b.firstFailure = time.Time{}
		b.lockedUntil = now.Add(l.cfg.Lockout)
		return LoginLimitDecision{Scope: scope, Until: b.lockedUntil}, true
	}
	return LoginLimitDecision{}, false
}

func (l *LoginLimiter) checkBucket(b *loginBucket, scope string, now time.Time) (LoginLimitDecision, bool) {
	if b == nil {
		return LoginLimitDecision{}, false
	}
	if !b.lockedUntil.IsZero() {
		if now.Before(b.lockedUntil) {
			b.lastSeen = now
			return LoginLimitDecision{Scope: scope, Until: b.lockedUntil}, true
		}
		b.lockedUntil = time.Time{}
	}
	return LoginLimitDecision{}, false
}

func (l *LoginLimiter) pruneExpired(now time.Time) {
	l.pruneMap(l.users, now)
	l.pruneMap(l.ips, now)
}

func (l *LoginLimiter) pruneMap(buckets map[string]*loginBucket, now time.Time) {
	for key, bucket := range buckets {
		if bucket == nil {
			delete(buckets, key)
			continue
		}
		if !bucket.lockedUntil.IsZero() {
			if !now.Before(bucket.lockedUntil) {
				delete(buckets, key)
			}
			continue
		}
		if bucket.firstFailure.IsZero() || now.Sub(bucket.firstFailure) > l.cfg.Window {
			delete(buckets, key)
		}
	}
}

func (l *LoginLimiter) makeRoom(buckets map[string]*loginBucket, now time.Time) {
	if len(buckets) < l.cfg.MaxEntries {
		return
	}
	l.pruneMap(buckets, now)
	for len(buckets) >= l.cfg.MaxEntries {
		var (
			oldestKey  string
			oldestSeen time.Time
			found      bool
		)
		for key, bucket := range buckets {
			seen := time.Time{}
			if bucket != nil {
				seen = bucket.lastSeen
			}
			if !found || seen.Before(oldestSeen) {
				oldestKey = key
				oldestSeen = seen
				found = true
			}
		}
		if !found {
			return
		}
		delete(buckets, oldestKey)
	}
}

func normalizeLoginKey(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func remoteIP(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return "unknown"
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		remoteAddr = host
	}
	remoteAddr = strings.Trim(remoteAddr, "[]")
	if ip := net.ParseIP(remoteAddr); ip != nil {
		return ip.String()
	}
	return strings.ToLower(remoteAddr)
}
