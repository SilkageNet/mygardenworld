package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func guildServerError(name clientproto.RPCName, payload string) error {
	return &babigame.RPCServerError{Name: name, Envelope: babigame.WSResponseD{M: json.RawMessage(payload)}}
}

func TestFmlNotJoinedRequiresTypedGuildServerEvidence(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"enter_code", guildServerError(clientproto.RPCFmlEnter, `{"code":109}`), true},
		{"subsystem_message", guildServerError(clientproto.RPCFmlLandHarvest, `{"msg":"您还未加入任何公会"}`), true},
		{"wrapped_enter", fmt.Errorf("preflight: %w", guildServerError(clientproto.RPCFmlEnter, `{"code":109}`)), true},
		{"local_message", errors.New("本地判断: 未加入公会"), false},
		{"untyped_code", errors.New(`rpc fml.enter: server: {"code":109}`), false},
		{"timeout", context.DeadlineExceeded, false},
		{"cancelled", context.Canceled, false},
		{"unrelated_rpc", guildServerError(clientproto.RPCShopEnter, `{"code":109,"msg":"未加入公会"}`), false},
		{"unproven_code", guildServerError(clientproto.RPCFmlRaceGetTaskList, `{"code":109}`), false},
		{"restriction", guildServerError(clientproto.RPCFmlEnter, `{"code":97777,"msg":"未加入公会"}`), false},
		{"restriction_97778", guildServerError(clientproto.RPCFmlEnter, `{"code":97778,"msg":"未加入公会"}`), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isFmlNotJoinedError(clientproto.RPCFmlRaceGetTaskList.String(), tc.err); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestFmlMembershipConflictingResponseAndRecovery(t *testing.T) {
	r := newOperationEventTestRunner()
	r.state.ApplyV(json.RawMessage(`{"25":{"1":{"1":88,"2":1}}}`))
	now := time.Now()
	op := &automation.PlannedOp{Kind: clientproto.RPCFmlRaceGetTaskList.String(), Category: automation.CategoryRace, Domain: "union.race.sync"}
	err := guildServerError(clientproto.RPCFmlRaceGetTaskList, `{"msg":"您还未加入任何公会"}`)
	if got := r.handleOperationError(t.Context(), operationResult{operationAttempt: operationAttempt{op: op}, err: err, finishedAt: now}); got != nil {
		t.Fatal(got)
	}
	if got := r.state.FmlBuild(); got.MembershipObserved || got.MemberFmlID != 0 || got.MemberPositionObserved {
		t.Fatalf("must pause as unknown: %+v", got)
	}
	if next := automation.FmlMembershipNextSyncAt(r.state.FmlBuild()); !next.Equal(time.UnixMilli(now.UnixMilli()).Add(30 * time.Second)) {
		t.Fatalf("retry=%v", next)
	}
	for _, payload := range []string{`{}`, `{"111":{"0":42,"1":1}}`} {
		if _, err := r.syncFmlMembership(func() (json.RawMessage, error) { return json.RawMessage(payload), nil }); err != nil {
			t.Fatal(err)
		}
		if r.state.FmlBuild().MembershipObserved {
			t.Fatal("empty/subsystem response reopened membership")
		}
	}
	if _, err := r.syncFmlMembership(func() (json.RawMessage, error) { return json.RawMessage(`{"0":{"0":88},"1":{"1":88,"2":2}}`), nil }); err != nil {
		t.Fatal(err)
	}
	got := r.state.FmlBuild()
	if !got.MembershipObserved || got.MemberFmlID != 88 || got.MemberPosition != 2 || got.MembershipSyncAttempts != 0 {
		t.Fatalf("not recovered: %+v", got)
	}
	if err := r.validateFmlMembershipBeforeSend(t.Context(), clientproto.RPCFmlLandHarvest.String()); err != nil {
		t.Fatal(err)
	}
	// Only the explicit membership read confirms absence; retry stays slow.
	op.Kind = clientproto.RPCFmlEnter.String()
	err = guildServerError(clientproto.RPCFmlEnter, `{"code":109}`)
	if got := r.handleOperationError(t.Context(), operationResult{operationAttempt: operationAttempt{op: op}, err: err, finishedAt: now}); got != nil {
		t.Fatal(got)
	}
	got = r.state.FmlBuild()
	if !got.MembershipObserved || got.MemberFmlID != 0 {
		t.Fatalf("missing confirmed absence: %+v", got)
	}
	if !automation.FmlMembershipNextSyncAt(got).Equal(time.UnixMilli(now.UnixMilli()).Add(5 * time.Minute)) {
		t.Fatal("confirmed absence must use slow retry")
	}
}

func TestFmlMembershipSyncFailureBacksOffWithoutLosingKnownIdentity(t *testing.T) {
	for _, joined := range []bool{false, true} {
		r := newOperationEventTestRunner()
		if joined {
			r.state.ApplyV(json.RawMessage(`{"25":{"1":{"1":88,"2":1}}}`))
		}
		_, err := r.syncFmlMembership(func() (json.RawMessage, error) { return nil, context.DeadlineExceeded })
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
		got := r.state.FmlBuild()
		if got.MembershipObserved != joined || got.MembershipSyncAttempts != 1 || got.MembershipSyncAtMs <= 0 {
			t.Fatalf("bad timeout state: %+v", got)
		}
		if joined && got.MemberFmlID != 88 {
			t.Fatal("timeout cleared membership")
		}
	}
}

func TestStartupLazySyncMembershipBaseline(t *testing.T) {
	for _, tc := range []struct {
		name, login, sync string
		err               error
		observed          bool
		guild             int32
	}{
		{"failed_empty", `{}`, `{}`, context.DeadlineExceeded, false, 0},
		{"failed_guild_only", `{"25":{"0":{"0":88}}}`, `{}`, context.DeadlineExceeded, false, 0},
		{"failed_with_member", `{"25":{"1":{"1":88}}}`, `{}`, context.DeadlineExceeded, true, 88},
		{"complete_empty", `{}`, `{}`, nil, true, 0},
		{"complete_guild_only", `{"25":{"0":{"0":88}}}`, `{}`, nil, true, 88},
		{"complete_member", `{}`, `{"25":{"1":{"1":88}}}`, nil, true, 88},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newOperationEventTestRunner()
			r.state.ApplyV(json.RawMessage(`{"25":{"1":{"1":99,"2":1},"111":{"0":42,"1":1}}}`))
			r.state.BeginFmlMembershipSnapshot()
			r.state.ApplyV(json.RawMessage(tc.login))
			r.applyStartupLazySync(json.RawMessage(tc.sync), tc.err)
			got := r.state.FmlBuild()
			if got.MembershipObserved != tc.observed || got.MemberFmlID != tc.guild || r.state.FmlRace().Observed {
				t.Fatalf("bad startup state: %+v", got)
			}
		})
	}
}

func TestFmlMembershipRecheckedAfterPacingWait(t *testing.T) {
	for _, change := range []string{"leave", "switch"} {
		t.Run(change, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				r := newOperationEventTestRunner()
				r.pacer = newRequestPacer(RequestPacing{})
				r.pacer.lastRequest = time.Now()
				r.state.ApplyV(json.RawMessage(`{"25":{"1":{"1":88}}}`))
				ctx := context.WithValue(t.Context(), fmlExecutionContextKey{}, int32(88))
				done := make(chan error, 1)
				go func() { done <- r.beforeGameRPC(ctx, clientproto.RPCFmlLandHarvest.String()) }()
				synctest.Wait()
				if change == "leave" {
					r.state.MarkNoFmlMembership()
				} else {
					r.state.ApplyV(json.RawMessage(`{"25":{"1":{"1":99}}}`))
				}
				time.Sleep(2 * time.Second)
				if err := <-done; err == nil {
					t.Fatal("stale operation passed pacing guard")
				}
			})
		})
	}
}

func TestFmlMembershipRecoveryHonorsPolicyAndRequestProtection(t *testing.T) {
	r := newOperationEventTestRunner()
	r.policy = automation.DefaultPolicy()
	r.policy.AutomationEnabled = false
	ctx := context.WithValue(t.Context(), scheduledOperationKey{}, true)
	if err := r.beforeGameRPC(ctx, clientproto.RPCFmlEnter.String()); err == nil || !strings.Contains(err.Error(), "自动化已关闭") {
		t.Fatalf("disabled sync: %v", err)
	}
	r.policy.AutomationEnabled = true
	r.policy.Union.Race.Enabled = true
	if err := r.beforeGameRPC(ctx, clientproto.RPCFmlEnter.String()); err != nil {
		t.Fatal(err)
	}
	r.safety = store.AccountRequestSafety{RestrictionCode: 97777, RestrictedUntilMS: time.Now().Add(time.Minute).UnixMilli()}
	if err := r.beforeGameRPC(ctx, clientproto.RPCFmlEnter.String()); err == nil {
		t.Fatal("recovery bypassed request protection")
	}
	if err := r.beforeGameRPC(ctx, clientproto.RPCUsrHeartTick.String()); err == nil {
		t.Fatal("heartbeat bypassed request protection")
	}
}
