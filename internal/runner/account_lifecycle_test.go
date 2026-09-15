package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestLifecycleWaitCancellationDoesNotExecuteLater(t *testing.T) {
	for _, command := range []string{"delete", "pause", "start", "reload"} {
		t.Run(command, func(t *testing.T) {
			m, r, client := startupCommitFixture(t)
			m.runners[r.account.ID] = r
			lock := m.accountLock(r.account.ID)
			lock.Lock()
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				var err error
				switch command {
				case "delete":
					err = m.DeleteAccount(ctx, r.account.ID)
				case "pause":
					err = m.PauseAutomation(ctx, r.account.ID, true)
				case "start":
					_, err = m.StartAutomation(ctx, r.account.ID, StartSourceControlPanel, true)
				case "reload":
					_, err = m.ReloadWithSource(ctx, r.account.ID, StartSourceControlPanel)
				}
				done <- err
			}()
			select {
			case err := <-done:
				lock.Unlock()
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("cancellation = %v", err)
				}
			case <-time.After(time.Second):
				lock.Unlock()
				<-done
				t.Fatal("command ignored cancellation while waiting for login")
			}
			if client.Closed() || m.Get(r.account.ID) != r {
				t.Fatal("cancelled command changed runner lifecycle")
			}
			if _, err := m.db.GetAccountByID(t.Context(), r.account.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeleteAccountSerializesPendingStartAndRemovesRuntime(t *testing.T) {
	m, r, client := startupCommitFixture(t)
	lock := m.accountLock(r.account.ID)
	lock.Lock()
	done := make(chan error, 1)
	go func() { done <- m.DeleteAccount(t.Context(), r.account.ID) }()
	// Complete an already admitted startup while deletion waits for ownership.
	m.mu.Lock()
	m.runners[r.account.ID] = r
	m.mu.Unlock()
	lock.Unlock()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if !client.Closed() || m.Get(r.account.ID) != nil {
		t.Fatal("deleted account retained a game runtime")
	}
	if _, err := m.db.GetAccountByID(t.Context(), r.account.ID); !errors.Is(err, store.ErrAccountNotFound) {
		t.Fatalf("deleted account still readable: %v", err)
	}
	if _, err := m.StartWithSource(t.Context(), r.account.ID, StartSourceControlPanel); err == nil {
		t.Fatal("start after deletion reused a runner")
	}
	if _, ok := m.RuntimeStats(r.account.ID); ok {
		t.Fatal("deleted account retained runtime statistics")
	}
}

func TestDeleteAccountFailureRetainsPersistedAccount(t *testing.T) {
	m, r, client := startupCommitFixture(t)
	m.runners[r.account.ID] = r
	if _, err := m.db.ExecContext(t.Context(), `CREATE TRIGGER reject_delete BEFORE DELETE ON accounts BEGIN SELECT RAISE(ABORT, 'fixture deletion failed'); END`); err != nil {
		t.Fatal(err)
	}
	if err := m.DeleteAccount(t.Context(), r.account.ID); err == nil {
		t.Fatal("failed deletion reported success")
	}
	if !client.Closed() {
		t.Fatal("failed deletion left game connection active")
	}
	if _, err := m.db.GetAccountByID(t.Context(), r.account.ID); err != nil {
		t.Fatal("failed deletion lost account", err)
	}
	if _, err := m.db.LoadPolicyJSON(t.Context(), r.account.ID); err != nil {
		t.Fatal("failed deletion lost policy", err)
	}
}
