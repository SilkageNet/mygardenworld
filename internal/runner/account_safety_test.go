package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestRaceDeleteReservationSurvivesFailurePolicyChangeAndRunnerReplacement(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	u, err := db.CreateUser(ctx, "test", "test@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a, err := db.CreateAccount(ctx, u.ID, "test", "ios", "u", "p")
	if err != nil {
		t.Fatal(err)
	}
	r := newOperationEventTestRunner()
	r.db, r.account = db, a
	now := time.Now().Truncate(time.Millisecond)
	op := automation.PlannedOp{Kind: clientproto.RPCFmlRaceDelTask.String(), TaskMsID: 1, Executable: true}
	if err := r.reserveRaceDelete(ctx, &op, now); err != nil {
		t.Fatal(err)
	}
	r.clearOperationCooldown(&op) // Ordinary success must not remove the slot.
	op.TaskMsID = 2
	if err := r.reserveRaceDelete(ctx, &op, now.Add(119*time.Second)); err == nil {
		t.Fatal("another task bypassed account slot")
	}
	r2 := newOperationEventTestRunner()
	r2.db, r2.account = db, a
	if err := r2.loadAccountSafety(ctx); err != nil {
		t.Fatal(err)
	}
	if r2.raceDeleteWait(now.Add(time.Second)) != 119*time.Second {
		t.Fatal("restart forgot deletion")
	}
	r2.policy = automation.DefaultPolicy()
	r2.policy.Union.Race.DeleteIntervalSeconds = 180
	if err := r2.reserveRaceDelete(ctx, &op, now.Add(120*time.Second)); err == nil {
		t.Fatal("policy change reset reservation")
	}
	if err := r2.reserveRaceDelete(ctx, &op, now.Add(180*time.Second)); err != nil {
		t.Fatal(err)
	}
	// The restriction itself must hydrate on Start without attempting even
	// HTTP configuration/login. Invalid empty network config makes an
	// accidental network/login path fail this test rather than contact a game.
	r2.recordAccountRestriction("farm.harvest", babigame.WSResponseD{M: json.RawMessage(`{"code":97778}`)}, time.Now())
	r3 := New(babigame.Config{}, db, a, NewBus(), r.log)
	if err := r3.Start(ctx); err != nil {
		t.Fatalf("pending restriction did not defer startup: %v", err)
	}
	if r3.Connected() || r3.restrictionError() == nil {
		t.Fatal("restart lost account pause")
	}
	r3.Stop()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := r2.reserveRaceDelete(ctx, &op, now.Add(time.Hour)); err == nil {
		t.Fatal("delete allowed after persistence failure")
	}
}

func TestRestrictionRecoveryRequiresDurableClear(t *testing.T) {
	r := newOperationEventTestRunner()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	r.db = db
	r.safety = store.AccountRequestSafety{RestrictionCode: 97777, RestrictedUntilMS: time.Now().Add(-time.Second).UnixMilli()}
	if err := r.clearAccountRestriction(0); err == nil || r.restrictionError() == nil {
		t.Fatal("failed persistence resumed account")
	}
	r.deferRestrictionProbe(0, fmt.Errorf("test probe failure"))
	s, _ := r.accountSafetySnapshot()
	if s.RestrictedUntilMS <= time.Now().UnixMilli() {
		t.Fatal("persistence failure caused an immediate probe loop")
	}
}

func TestRaceDeleteSpacingDoesNotStarveOtherOperations(t *testing.T) {
	r := newOperationEventTestRunner()
	now := time.Now()
	r.safety.LastRaceDeleteMS = now.UnixMilli()
	deleteOp := automation.PlannedOp{Kind: "fmlRace.delTask", Executable: true, Lane: automation.LaneSide}
	farmOp := automation.PlannedOp{Kind: "farm.plantBatch", Executable: true, Lane: automation.LaneFarm}
	got := r.selectRunnableOperation([]automation.PlannedOp{deleteOp, farmOp}, now)
	if got == nil || got.Kind != farmOp.Kind {
		t.Fatalf("delete spacing starved farming: %+v", got)
	}
}

func TestAccountRestrictionBackoffAndServerDeadline(t *testing.T) {
	now := time.UnixMilli(1800000000000)
	unknown := babigame.WSResponseD{M: json.RawMessage(`{"code":97777}`)}
	s := store.AccountRequestSafety{}
	for _, wait := range []time.Duration{5 * time.Minute, 10 * time.Minute, 20 * time.Minute, 30 * time.Minute, 30 * time.Minute} {
		s = nextAccountRestriction(s, unknown, now)
		if got := time.UnixMilli(s.RestrictedUntilMS).Sub(now); got != wait {
			t.Fatalf("backoff=%s want %s", got, wait)
		}
		duplicate := nextAccountRestriction(s, unknown, now.Add(time.Second))
		if duplicate != s {
			t.Fatalf("duplicate extended restriction: %+v -> %+v", s, duplicate)
		}
		now = time.UnixMilli(s.RestrictedUntilMS)
	}
	for _, tt := range []struct {
		name, args string
		want       time.Duration
	}{
		{"server_ms", fmt.Sprint(now.Add(time.Hour).UnixMilli()), time.Hour + 30*time.Second},
		{"server_string_ms", fmt.Sprintf("%q", fmt.Sprint(now.Add(time.Hour).UnixMilli())), time.Hour + 30*time.Second},
		{"server_iso", fmt.Sprintf("%q", now.Add(time.Hour).Format(time.RFC3339)), time.Hour + 30*time.Second},
		{"past", fmt.Sprint(now.Add(-time.Hour).UnixMilli()), 30 * time.Minute},
		{"unknown", `"unknown"`, 30 * time.Minute},
		{"seconds_not_ms", fmt.Sprint(now.Add(time.Hour).Unix()), 30 * time.Minute},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := babigame.WSResponseD{M: json.RawMessage(`{"code":97778,"args":[` + tt.args + `]}`)}
			got := nextAccountRestriction(store.AccountRequestSafety{}, d, now)
			if time.UnixMilli(got.RestrictedUntilMS).Sub(now) != tt.want {
				t.Fatalf("deadline: %+v want %s", got, tt.want)
			}
		})
	}
	first := nextAccountRestriction(store.AccountRequestSafety{}, unknown, now)
	escalated := nextAccountRestriction(first, babigame.WSResponseD{M: json.RawMessage(`{"code":97778}`)}, now.Add(time.Second))
	if escalated.RestrictionCode != 97778 || escalated.RestrictedUntilMS != now.Add(30*time.Minute+time.Second).UnixMilli() {
		t.Fatalf("escalation: %+v", escalated)
	}
	if got := nextAccountRestriction(escalated, unknown, now.Add(2*time.Second)); got != escalated {
		t.Fatal("late 97777 downgraded 97778")
	}
}

func TestAccountRestrictionGatesAllRequestsAndLateErrorsWin(t *testing.T) {
	r := newOperationEventTestRunner()
	r.bus = NewBus()
	events, cancel := r.bus.SubscribeLive(10)
	defer cancel()
	d := babigame.WSResponseD{M: json.RawMessage(`{"code":97777}`)}
	r.recordAccountRestriction("fmlRace.delTask", d, time.Now())
	for _, name := range []string{"farm.plantBatch", "farm.harvest", "fmlRace.delTask", "usr.lazySync", "usr.heartTick", "index.login", "index.reLogin"} {
		if err := r.beforeGameRPC(context.Background(), name); err == nil {
			t.Errorf("%s bypassed pause", name)
		}
	}
	other := newOperationEventTestRunner()
	if err := other.beforeGameRPC(context.Background(), "farm.harvest"); err != nil {
		t.Fatal("another account was paused")
	}
	_, revision := r.accountSafetySnapshot()
	r.recordAccountRestriction("farm.harvest", d, time.Now())
	if len(events) != 1 {
		t.Fatal("duplicate restriction spammed logs")
	}
	if err := r.clearAccountRestriction(revision); err == nil {
		t.Fatal("stale recovery cleared a newer failure")
	}
	r.safetyMu.Lock()
	r.safety.RestrictedUntilMS = time.Now().Add(-time.Second).UnixMilli()
	r.safetyMu.Unlock()
	if err := r.beforeGameRPC(context.Background(), "farm.harvest"); err == nil {
		t.Fatal("deadline alone resumed mutations")
	}
	if err := r.beforeGameRPC(context.Background(), "index.reLogin"); err != nil {
		t.Fatalf("recovery probe blocked: %v", err)
	}
	_, revision = r.accountSafetySnapshot()
	if err := r.clearAccountRestriction(revision); err != nil {
		t.Fatal(err)
	}
	if err := r.beforeGameRPC(context.Background(), "farm.harvest"); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatal("missing single recovery event")
	}
}

func TestRestrictionWaitHonorsCancellation(t *testing.T) {
	r := newOperationEventTestRunner()
	r.safety = store.AccountRequestSafety{RestrictionCode: 97778, RestrictedUntilMS: time.Now().Add(time.Hour).UnixMilli()}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r.waitAccountRestriction(ctx) {
		t.Fatal("cancelled reconnect wait continued")
	}
	if err := r.executeOperation(ctx, nil, nil, &automation.PlannedOp{Kind: "farm.harvest"}, time.Now()); err == nil {
		t.Fatal("manual operation bypassed account pause")
	}
}
