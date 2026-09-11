package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func startupCommitFixture(t *testing.T) (*Manager, *Runner, *babigame.Client) {
	t.Helper()
	m, db := maintenanceTestManager(t)
	u, err := db.CreateUser(t.Context(), "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccountWithPolicy(t.Context(), u.ID, "fixture", "ios", "fixture", "fixture", maintenanceTestPolicy(t, false))
	if err != nil {
		t.Fatal(err)
	}
	r := New(babigame.Config{}, db, a, m.bus, m.log)
	_, r.cancel = context.WithCancel(context.Background())
	client := babigame.NewClient(&babigame.Session{})
	r.client = client // Model the connection already installed during bootstrap.
	t.Cleanup(r.Stop)
	return m, r, client
}

func assertStartupIntent(t *testing.T, r *Runner, enabled bool) {
	t.Helper()
	raw, err := r.db.LoadPolicyJSON(t.Context(), r.account.ID)
	if err != nil {
		t.Fatal(err)
	}
	p, err := policycfg.FromJSON(raw)
	if err != nil || p.GetAutomationEnabled() != enabled || r.Policy().GetAutomationEnabled() != enabled {
		t.Fatalf("durable=%v live=%v want=%v err=%v", p.GetAutomationEnabled(), r.Policy().GetAutomationEnabled(), enabled, err)
	}
}

func TestStartupCommitFailureClosesUnfinishedConnection(t *testing.T) {
	for _, failure := range []string{"cancel", "deadline", "policy write", "session invalidated", "stopped", "preserve intent cancelled"} {
		t.Run(failure, func(t *testing.T) {
			_, r, client := startupCommitFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			activate := true
			switch failure {
			case "cancel":
				cancel()
			case "deadline":
				var deadlineCancel context.CancelFunc
				ctx, deadlineCancel = context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
				defer deadlineCancel()
			case "policy write":
				if _, err := r.db.ExecContext(ctx, `CREATE TRIGGER reject_policy BEFORE UPDATE ON account_policies BEGIN SELECT RAISE(ABORT, 'fixture write failed'); END`); err != nil {
					t.Fatal(err)
				}
			case "session invalidated":
				r.sessionInvalidated = true
			case "stopped":
				r.Stop()
			case "preserve intent cancelled":
				activate = false
				cancel()
			}
			if err := r.completeStartup(ctx, activate); err == nil {
				t.Fatal("unfinished start reported success")
			}
			if !client.Closed() || r.cancel != nil {
				t.Fatal("failed start retained a connection or background runtime")
			}
			select {
			case <-r.Done():
			default:
				t.Fatal("failed start did not finish cleanup")
			}
			assertStartupIntent(t, r, false)
		})
	}
}

func TestStartupCommitKeepsDurableAndLiveIntentTogether(t *testing.T) {
	for _, activate := range []bool{false, true} {
		t.Run(map[bool]string{false: "reauth preserves pause", true: "explicit activation"}[activate], func(t *testing.T) {
			m, r, client := startupCommitFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if err := r.completeStartup(ctx, activate); err != nil {
				t.Fatal(err)
			}
			cancel() // Cancellation after the successful commit cannot undo it.
			assertStartupIntent(t, r, activate)
			if client.Closed() {
				t.Fatal("completed start was closed")
			}
			if activate {
				m.runners[r.account.ID] = r
				if got, err := m.StartAutomation(t.Context(), r.account.ID, StartSourceAutomationEnable, false); err != nil || got != r {
					t.Fatalf("reuse existing runner=%v err=%v", got == r, err)
				}
				var events int
				if err := r.db.QueryRowContext(t.Context(), `SELECT COUNT(*) FROM event_log WHERE message = '自动化已启动'`).Scan(&events); err != nil || events != 1 {
					t.Fatalf("activation events=%d err=%v", events, err)
				}
			}
		})
	}
}

func TestActivationRejectsCancelledOrMaintenanceWithoutStoppingExistingRunner(t *testing.T) {
	for _, maintenance := range []bool{false, true} {
		m, r, client := startupCommitFixture(t)
		m.runners[r.account.ID] = r
		ctx, cancel := context.WithCancel(t.Context())
		want := context.Canceled
		if maintenance {
			m.gameGate.block()
			want = ErrMaintenance
		} else {
			cancel()
		}
		_, err := m.StartAutomation(ctx, r.account.ID, StartSourceControlPanel, true)
		cancel()
		if !errors.Is(err, want) || client.Closed() || m.Get(r.account.ID) != r {
			t.Fatalf("err=%v want=%v closed=%v", err, want, client.Closed())
		}
		assertStartupIntent(t, r, false)
	}
}

func TestLogoutWaitsForPendingStartupAndClearsRestoreIntent(t *testing.T) {
	m, r, client := startupCommitFixture(t)
	lock := m.accountLock(r.account.ID)
	lock.Lock() // A start admitted but not yet registered.
	finished := make(chan error, 1)
	go func() { finished <- m.PauseAutomation(t.Context(), r.account.ID, true) }()
	if err := r.completeStartup(t.Context(), true); err != nil {
		lock.Unlock()
		t.Fatal(err)
	}
	m.mu.Lock()
	m.runners[r.account.ID] = r
	m.mu.Unlock()
	lock.Unlock()
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	assertStartupIntent(t, r, false)
	if !client.Closed() || m.Get(r.account.ID) != nil {
		t.Fatal("logout left the newly started runner alive")
	}
}

func TestManagerDoesNotRegisterUncommittedStartup(t *testing.T) {
	m, r, _ := startupCommitFixture(t)
	// A persisted protection window exercises the real startup/fallback path
	// without contacting a game server. It must still commit explicit intent
	// before publishing a background recovery worker.
	if err := m.db.SaveAccountRestriction(t.Context(), r.account.ID, store.AccountRequestSafety{
		RestrictionCode: 97777, RestrictedUntilMS: time.Now().Add(time.Hour).UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.ExecContext(t.Context(), `CREATE TRIGGER reject_policy BEFORE UPDATE ON account_policies BEGIN SELECT RAISE(ABORT, 'fixture write failed'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := m.StartAutomation(t.Context(), r.account.ID, StartSourceControlPanel, true); err == nil {
		t.Fatal("failed activation registered a recovery worker")
	}
	if m.Get(r.account.ID) != nil {
		t.Fatal("unfinished runner remains registered")
	}
	assertStartupIntent(t, r, false)
	if _, err := m.db.ExecContext(t.Context(), `DROP TRIGGER reject_policy`); err != nil {
		t.Fatal(err)
	}
	started, err := m.StartAutomation(t.Context(), r.account.ID, StartSourceControlPanel, true)
	if err != nil {
		t.Fatal(err)
	}
	assertStartupIntent(t, started, true)
	if m.Get(r.account.ID) != started || started.restrictionError() == nil {
		t.Fatal("successful start lost registration or request protection")
	}
	if err := m.PauseAutomation(t.Context(), r.account.ID, true); err != nil {
		t.Fatal(err)
	}
}
