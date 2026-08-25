package store

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRotateRefreshTokenConsumesTokenOnce(t *testing.T) {
	db, userID := newTokenTestDB(t)
	ctx := context.Background()
	if err := db.SaveRefreshToken(ctx, userID, "old-token", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}

	var successes atomic.Int32
	var wg sync.WaitGroup
	for _, next := range []string{"new-token-a", "new-token-b"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := db.RotateRefreshToken(ctx, "old-token", next, time.Now().Add(time.Hour)); err == nil {
				successes.Add(1)
			} else if !errors.Is(err, ErrTokenInvalid) {
				t.Errorf("RotateRefreshToken() error = %v, want ErrTokenInvalid", err)
			}
		}()
	}
	wg.Wait()
	if got := successes.Load(); got != 1 {
		t.Fatalf("successful rotations = %d, want 1", got)
	}
	if _, err := db.ValidateRefreshToken(ctx, "old-token"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("old token validation error = %v, want ErrTokenInvalid", err)
	}
}

func TestRotateRefreshTokenRollsBackOnReplacementFailure(t *testing.T) {
	db, userID := newTokenTestDB(t)
	ctx := context.Background()
	if err := db.SaveRefreshToken(ctx, userID, "old-token", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	newHash := hashToken("rejected-token")
	trigger := fmt.Sprintf(`CREATE TRIGGER reject_rotated_token BEFORE INSERT ON refresh_tokens
		WHEN NEW.token_hash = '%s' BEGIN SELECT RAISE(ABORT, 'forced insert failure'); END`, newHash)
	if _, err := db.ExecContext(ctx, trigger); err != nil {
		t.Fatal(err)
	}

	if err := db.RotateRefreshToken(ctx, "old-token", "rejected-token", time.Now().Add(time.Hour)); err == nil {
		t.Fatal("RotateRefreshToken() succeeded, want forced replacement failure")
	}
	gotUserID, err := db.ValidateRefreshToken(ctx, "old-token")
	if err != nil {
		t.Fatalf("old token should remain valid after rollback: %v", err)
	}
	if gotUserID != userID {
		t.Fatalf("old token user id = %d, want %d", gotUserID, userID)
	}
}

func newTokenTestDB(t *testing.T) (*DB, int64) {
	t.Helper()
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	user, err := db.CreateUser(ctx, "token-owner", "token-owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	return db, user.ID
}
