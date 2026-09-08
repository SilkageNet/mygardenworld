package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func rejectedRestore(code int) error {
	return fmt.Errorf("启动恢复登录失败: %w", &babigame.RPCServerError{
		Name:     clientproto.RPCIndexReLogin,
		Envelope: babigame.WSResponseD{M: json.RawMessage(fmt.Sprintf(`{"code":%d}`, code))},
	})
}

func TestCachedSessionRecoveryFallback(t *testing.T) {
	for _, tc := range []struct {
		name          string
		code          int
		active        bool
		lateResponse  bool
		cancelled     bool
		invalidated   bool
		err           error
		wantPreserved bool
	}{
		{name: "expired 97777 permits rejected cache replacement", code: 97777, err: rejectedRestore(91102)},
		{name: "expired 97778 permits rejected cache replacement", code: 97778, err: rejectedRestore(91102)},
		{name: "bare numeric cache expiry", code: 97777, err: &babigame.RPCServerError{Name: clientproto.RPCIndexReLogin, Envelope: babigame.WSResponseD{M: json.RawMessage(`91102`)}}},
		{name: "ordinary cache expiry", err: rejectedRestore(91102)},
		{name: "cooldown still active", code: 97777, active: true, err: rejectedRestore(91102), wantPreserved: true},
		{name: "late restriction response wins", code: 97777, lateResponse: true, err: rejectedRestore(91102), wantPreserved: true},
		{name: "unknown server code", code: 97777, err: rejectedRestore(12345), wantPreserved: true},
		{name: "new 97777", code: 97777, err: rejectedRestore(97777), wantPreserved: true},
		{name: "new 97778", code: 97777, err: rejectedRestore(97778), wantPreserved: true},
		{name: "RPC timeout", code: 97777, err: context.DeadlineExceeded, wantPreserved: true},
		{name: "ordinary timeout also preserves token", err: context.DeadlineExceeded, wantPreserved: true},
		{name: "transport startup failure", code: 97777, err: fmt.Errorf("gateway: %w", errWebSocketSessionStart), wantPreserved: true},
		{name: "untyped error text is not evidence", code: 97777, err: errors.New("rpc index.reLogin: server: 91102"), wantPreserved: true},
		{name: "durable clear failed", code: 97777, err: errors.New("database unavailable"), wantPreserved: true},
		{name: "context cancelled during restore", code: 97777, cancelled: true, err: rejectedRestore(91102), wantPreserved: true},
		{name: "session invalidated", code: 97777, invalidated: true, err: rejectedRestore(91102), wantPreserved: true},
		{name: "not a cache reLogin rejection", code: 97777, err: &babigame.RPCServerError{Name: clientproto.RPCIndexLogin, Envelope: babigame.WSResponseD{M: json.RawMessage(`{"code":91102}`)}}, wantPreserved: true},
		{name: "explicit displacement is not cache expiry", code: 97777, err: &babigame.RPCServerError{Name: clientproto.RPCIndexReLogin, Envelope: babigame.WSResponseD{M: json.RawMessage(`{"code":91102,"msg":"账号已在其他设备登录"}`)}}, wantPreserved: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newOperationEventTestRunner()
			r.safety = store.AccountRequestSafety{RestrictionCode: tc.code, RestrictedUntilMS: time.Now().Add(-time.Minute).UnixMilli()}
			if tc.active {
				r.safety.RestrictedUntilMS = time.Now().Add(time.Minute).UnixMilli()
			}
			if tc.lateResponse {
				r.safetyRevision++
			}
			r.sessionInvalidated = tc.invalidated
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			before, revision := r.accountSafetySnapshot()
			if got := r.preserveCachedSession(ctx, tc.err, 0); got != tc.wantPreserved {
				t.Fatalf("preserve=%v want %v", got, tc.wantPreserved)
			}
			if after, afterRevision := r.accountSafetySnapshot(); after != before || afterRevision != revision {
				t.Fatal("cache decision changed request protection")
			}
		})
	}
}

func TestExpiredRestrictionRetainsRPCGateUntilFreshBaseline(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	u, err := db.CreateUser(ctx, "test", "test@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(ctx, u.ID, "test", "ios", "u", "p")
	if err != nil {
		t.Fatal(err)
	}
	paused := store.AccountRequestSafety{RestrictionCode: 97777, RestrictedUntilMS: time.Now().Add(-time.Minute).UnixMilli(), RestrictionAttempts: 1}
	if err := db.SaveAccountRestriction(ctx, a.ID, paused); err != nil {
		t.Fatal(err)
	}
	r := newOperationEventTestRunner()
	r.db, r.account = db, a
	if err := r.loadAccountSafety(ctx); err != nil {
		t.Fatal(err)
	}
	_, revision := r.accountSafetySnapshot()
	if r.preserveCachedSession(ctx, rejectedRestore(91102), revision) {
		t.Fatal("expired cache would be retried forever after restart")
	}
	for _, method := range []string{"shopCultivate.buy", "usr.heartTick", "usr.lazySync"} {
		if r.beforeGameRPC(ctx, method) == nil {
			t.Fatalf("%s resumed before fresh baseline", method)
		}
	}
	if err := r.beforeGameRPC(ctx, "index.login"); err != nil {
		t.Fatal("fresh login probe blocked", err)
	}
	if stored, err := db.LoadAccountRequestSafety(ctx, a.ID); err != nil || stored.RestrictionCode != 97777 {
		t.Fatal("protection cleared before successful login", err)
	}
	// connectSession clears only after a successful complete login baseline.
	if err := r.clearAccountRestriction(revision); err != nil {
		t.Fatal(err)
	}
	if err := r.beforeGameRPC(ctx, "shopCultivate.buy"); err != nil {
		t.Fatal("successful baseline did not restore requests", err)
	}
	if stored, err := db.LoadAccountRequestSafety(ctx, a.ID); err != nil || stored.RestrictionCode != 0 {
		t.Fatal("successful recovery was not persisted", err)
	}
}
