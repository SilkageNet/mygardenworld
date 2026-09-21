package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestDeletionScheduleOnlyShrinksExecutionTimeout(t *testing.T) {
	for _, tc := range []struct {
		phase string
		err   error
		want  int
	}{
		{"wait_writer", context.DeadlineExceeded, 250}, {"wait_lifecycle", context.DeadlineExceeded, 250},
		{"drain_game_work", context.DeadlineExceeded, 250}, {"begin_transaction", context.DeadlineExceeded, 250},
		{"event_log", context.DeadlineExceeded, 125}, {"commit", context.DeadlineExceeded, 125},
		{"finalize", context.DeadlineExceeded, 250}, {"event_log", errors.New("disk is full private path"), 250},
	} {
		t.Run(tc.phase+tc.err.Error(), func(t *testing.T) {
			s := deletionSchedule{limit: 250}
			s.observe(store.DeletionBatch{DeletionAttempt: store.DeletionAttempt{Phase: tc.phase}}, tc.err, time.Now())
			if s.limit != tc.want || s.failures != 1 || s.after.IsZero() {
				t.Fatal(s)
			}
		})
	}
}

func TestDeletionScheduleRecoversAfterIntermittentSlowDisk(t *testing.T) {
	s := deletionSchedule{limit: 250}
	now := time.Now()
	for range 7 {
		s.observe(store.DeletionBatch{DeletionAttempt: store.DeletionAttempt{Phase: "operation_log"}}, context.DeadlineExceeded, now)
	}
	if s.limit != 1 {
		t.Fatal(s)
	}
	for range 24 {
		// Writer admission may be slow; fast DELETE+COMMIT must still recover.
		s.observe(store.DeletionBatch{Removed: int64(s.limit), DeletionAttempt: store.DeletionAttempt{Phase: "operation_log", WaitMS: 1900, WorkMS: 50}}, nil, now)
	}
	if s.limit != 250 || s.failures != 0 || !s.after.IsZero() {
		t.Fatalf("permanently degraded: %+v", s)
	}
	s.observe(store.DeletionBatch{Removed: 250, DeletionAttempt: store.DeletionAttempt{WorkMS: 1500}}, nil, now)
	if s.limit != 125 {
		t.Fatal("slow committed batches should yield a smaller next batch", s)
	}
}

func TestDeletionAttemptVisibleWhenWriterCannotPersistFailure(t *testing.T) {
	m, r, _ := startupCommitFixture(t)
	if err := m.DeleteAccount(t.Context(), r.account.ID); err != nil {
		t.Fatal(err)
	}
	tx, err := m.db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); m.RunAccountDeletions(ctx) }()
	defer func() { cancel(); <-done }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if a, ok := m.LatestDeletionAttempt(r.account.ID); ok {
			if a.Phase != "wait_writer" || a.ErrorKind != "timeout" || a.BatchSize != 250 || a.Failures != 1 {
				t.Fatal(a)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("writer wait remained invisible")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
