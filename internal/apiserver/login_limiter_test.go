package apiserver

import (
	"testing"
	"time"
)

func TestLoginLimiterLocksUsername(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := NewLoginLimiter(LoginLimiterConfig{
		Window:       time.Minute,
		UserFailures: 2,
		IPFailures:   100,
		Lockout:      5 * time.Minute,
		MaxEntries:   16,
		Clock:        func() time.Time { return now },
	})

	if _, limited := limiter.RecordFailure(" Alice@example.test ", "10.0.0.1:1"); limited {
		t.Fatal("first failure limited, want allowed")
	}
	decision, limited := limiter.RecordFailure("alice@EXAMPLE.test", "10.0.0.2:1")
	if !limited || decision.Scope != "username" {
		t.Fatalf("second failure limited=(%v,%q), want username limit", limited, decision.Scope)
	}
	if _, limited := limiter.Check("alice@example.test", "10.0.0.3:1"); !limited {
		t.Fatal("username is not limited after threshold")
	}

	now = now.Add(5*time.Minute + time.Nanosecond)
	if _, limited := limiter.Check("alice@example.test", "10.0.0.3:1"); limited {
		t.Fatal("username still limited after lockout expiry")
	}
}

func TestLoginLimiterLocksIP(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := NewLoginLimiter(LoginLimiterConfig{
		Window:       time.Minute,
		UserFailures: 100,
		IPFailures:   2,
		Lockout:      5 * time.Minute,
		MaxEntries:   16,
		Clock:        func() time.Time { return now },
	})

	if _, limited := limiter.RecordFailure("a", "192.0.2.10:5000"); limited {
		t.Fatal("first IP failure limited, want allowed")
	}
	decision, limited := limiter.RecordFailure("b", "192.0.2.10:6000")
	if !limited || decision.Scope != "ip" {
		t.Fatalf("second IP failure limited=(%v,%q), want ip limit", limited, decision.Scope)
	}
	if _, limited := limiter.Check("c", "192.0.2.10:7000"); !limited {
		t.Fatal("IP is not limited after threshold")
	}
}

func TestLoginLimiterWindowExpiryRestartsFailures(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := NewLoginLimiter(LoginLimiterConfig{
		Window:       time.Minute,
		UserFailures: 2,
		IPFailures:   100,
		Lockout:      5 * time.Minute,
		MaxEntries:   16,
		Clock:        func() time.Time { return now },
	})

	if _, limited := limiter.RecordFailure("alice", "10.0.0.1:1"); limited {
		t.Fatal("first failure limited, want allowed")
	}
	now = now.Add(time.Minute + time.Nanosecond)
	if _, limited := limiter.RecordFailure("alice", "10.0.0.1:1"); limited {
		t.Fatal("failure after window limited, want fresh window")
	}
}

func TestLoginLimiterSuccessClearsUsernameBucket(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := NewLoginLimiter(LoginLimiterConfig{
		Window:       time.Minute,
		UserFailures: 2,
		IPFailures:   100,
		Lockout:      5 * time.Minute,
		MaxEntries:   16,
		Clock:        func() time.Time { return now },
	})

	if _, limited := limiter.RecordFailure("alice", "10.0.0.1:1"); limited {
		t.Fatal("first failure limited, want allowed")
	}
	limiter.RecordSuccess(" Alice ")
	if _, limited := limiter.RecordFailure("alice", "10.0.0.2:1"); limited {
		t.Fatal("failure after success limited, want username bucket cleared")
	}
}

func TestLoginLimiterMaxEntriesEvictsOldest(t *testing.T) {
	now := time.Unix(100, 0)
	limiter := NewLoginLimiter(LoginLimiterConfig{
		Window:       time.Hour,
		UserFailures: 100,
		IPFailures:   100,
		Lockout:      5 * time.Minute,
		MaxEntries:   2,
		Clock:        func() time.Time { return now },
	})

	_, _ = limiter.RecordFailure("a", "10.0.0.1:1")
	now = now.Add(time.Second)
	_, _ = limiter.RecordFailure("b", "10.0.0.2:1")
	now = now.Add(time.Second)
	_, _ = limiter.RecordFailure("c", "10.0.0.3:1")

	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	if len(limiter.users) > 2 || len(limiter.ips) > 2 {
		t.Fatalf("bucket sizes users=%d ips=%d, want both <= 2", len(limiter.users), len(limiter.ips))
	}
	if _, ok := limiter.users["a"]; ok {
		t.Fatal("oldest username bucket was not evicted")
	}
}
