package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestLifecycleWaitCancellationDoesNotExecuteLater(t *testing.T) {
	for _, command := range []string{"pause", "start", "reload"} {
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
	if done, _, _, err := m.cleanAccountDeletion(t.Context(), r.account.ID, 250); err != nil || !done {
		t.Fatal(done, err)
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
	if err := m.DeleteAccount(t.Context(), r.account.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := m.cleanAccountDeletion(t.Context(), r.account.ID, 250); err == nil {
		t.Fatal("failed cleanup reported success")
	}
	if !client.Closed() {
		t.Fatal("failed deletion left game connection active")
	}
	if _, err := m.db.GetAccountIncludingDeleting(t.Context(), r.account.ID); err != nil {
		t.Fatal("failed deletion lost account", err)
	}
	if _, err := m.db.LoadPolicyJSON(t.Context(), r.account.ID); err != nil {
		t.Fatal("failed deletion lost policy", err)
	}
}

func TestDeletionIntentCancelsPendingGameWorkAndWaitsForDrain(t *testing.T) {
	m, r, client := startupCommitFixture(t)
	r.accountGameGate = &m.accountLock(r.account.ID).game
	m.runners[r.account.ID] = r
	workCtx, release, err := r.beginGameWork(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	lock := m.accountLock(r.account.ID)
	lock.Lock()
	if err := m.DeleteAccount(t.Context(), r.account.ID); err != nil {
		lock.Unlock()
		t.Fatal(err)
	}
	if workCtx.Err() == nil {
		lock.Unlock()
		t.Fatal("accepted deletion did not cancel work")
	}
	lock.Unlock()
	short, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if done, _, phase, err := m.cleanAccountDeletion(short, r.account.ID, 250); done || phase != "drain_game_work" || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(done, phase, err)
	}
	if !client.Closed() {
		t.Fatal("socket not closed")
	}
	if _, err := m.db.GetAccountIncludingDeleting(t.Context(), r.account.ID); err != nil {
		t.Fatal("rows deleted before drain", err)
	}
	if _, _, err := r.beginGameWork(t.Context()); !errors.Is(err, store.ErrAccountDeleting) {
		t.Fatal("new game work accepted", err)
	}
	for _, source := range []StartSource{StartSourceControlPanel, StartSourceDaemonRestore, StartSourceRedeemAutoConnect} {
		if _, err := m.StartWithSource(t.Context(), r.account.ID, source); !errors.Is(err, store.ErrAccountDeleting) {
			t.Fatal(source, err)
		}
	}
	release()
	if done, _, _, err := m.cleanAccountDeletion(t.Context(), r.account.ID, 250); !done || err != nil {
		t.Fatal(done, err)
	}
}

func TestDeletionResumesWithNewManagerAndFailureDoesNotRestoreAccount(t *testing.T) {
	m, r, _ := startupCommitFixture(t)
	if err := m.DeleteAccount(t.Context(), r.account.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.ExecContext(t.Context(), `CREATE TRIGGER fail_cleanup BEFORE DELETE ON accounts BEGIN SELECT RAISE(ABORT,'disk fixture'); END;`); err != nil {
		t.Fatal(err)
	}
	restarted := NewManager(m.db, NewBus(), m.log)
	defer restarted.Shutdown()
	if _, err := restarted.StartWithSource(t.Context(), r.account.ID, StartSourceDaemonRestore); !errors.Is(err, store.ErrAccountDeleting) {
		t.Fatal(err)
	}
	if _, _, _, err := restarted.cleanAccountDeletion(t.Context(), r.account.ID, 250); err == nil {
		t.Fatal("injected failure hidden")
	}
	if err := m.db.SetAccountDeletionFailed(t.Context(), r.account.ID, true); err != nil {
		t.Fatal(err)
	}
	a, err := m.db.GetAccountIncludingDeleting(t.Context(), r.account.ID)
	if err != nil || !a.DeletionPending || !a.DeletionFailed {
		t.Fatal(a, err)
	}
	if _, err := m.db.ExecContext(t.Context(), `DROP TRIGGER fail_cleanup`); err != nil {
		t.Fatal(err)
	}
	if done, _, _, err := restarted.cleanAccountDeletion(t.Context(), r.account.ID, 250); !done || err != nil {
		t.Fatal(done, err)
	}
}

func TestDeletionWorkerSkipsFailureAndCompletesOtherAccount(t *testing.T) {
	m, r, _ := startupCommitFixture(t)
	other, err := m.db.CreateAccount(t.Context(), r.account.UserID, "other", "ios", "other", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.ExecContext(t.Context(), `CREATE TRIGGER fail_first_cleanup BEFORE DELETE ON accounts WHEN OLD.name='fixture' BEGIN SELECT RAISE(ABORT,'disk fixture'); END;`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{r.account.ID, other.ID} {
		if err := m.DeleteAccount(t.Context(), id); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); m.RunAccountDeletions(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		a, err := m.db.GetAccountIncludingDeleting(t.Context(), r.account.ID)
		if err != nil {
			t.Fatal(err)
		}
		_, otherErr := m.db.GetAccountIncludingDeleting(t.Context(), other.ID)
		if a.DeletionFailed && a.DeletionPending && errors.Is(otherErr, store.ErrAccountNotFound) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("failed account starved another deletion", a, otherErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
