package runner

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const dessertRuntimeNowMs int64 = 1783819000000

func TestDessertRuntimePolicyIsFourLayeredAndBounded(t *testing.T) {
	policy := automation.DefaultPolicy()
	if got := dessertPolicyConfig(policy); got.enabled || got.mode != 1 || got.maxSessionEnergy != 0 {
		t.Fatalf("default dessert autoplay policy=%+v", got)
	}

	policy.AutomationEnabled = true
	//nolint:staticcheck // Exercise compatibility with persisted legacy policy JSON.
	policy.Activity.Enabled = true
	policy.Activity.Modules = map[string]*pb.ActivityModulePolicy{
		dessertModuleID: {
			Enabled: true,
			BoolParams: map[string]bool{
				dessertAutoPlayPolicy:            true,
				dessertResumeExistingRoundPolicy: true,
			},
			IntParams: map[string]int64{
				dessertModePolicy:             1,
				dessertMaxEnergyPolicy:        5,
				dessertMinEnergyReservePolicy: 10,
			},
		},
	}
	got := dessertPolicyConfig(policy)
	if !got.enabled || !got.resumeExisting || got.mode != 1 || got.maxSessionEnergy != 5 ||
		got.minEnergyReserve != 10 || got.configurationError != "" {
		t.Fatalf("enabled dessert autoplay policy=%+v", got)
	}

	policy.Activity.Modules[dessertModuleID].IntParams[dessertModePolicy] = 5
	if got := dessertPolicyConfig(policy); !got.enabled || got.configurationError == "" {
		t.Fatalf("unsupported mode did not fail closed: %+v", got)
	}
	policy.Activity.Modules[dessertModuleID].IntParams[dessertModePolicy] = 1
	policy.Activity.Modules[dessertModuleID].IntParams[dessertMaxEnergyPolicy] = 101
	if got := dessertPolicyConfig(policy); got.configurationError == "" {
		t.Fatalf("oversized energy budget did not fail closed: %+v", got)
	}
}

func TestDessertBoundedControllerEvidenceGateIsOpaqueAndOutermost(t *testing.T) {
	if dessertLiveControllerCompiled {
		t.Fatal("live dessert controller compile-time fuse unexpectedly enabled")
	}
	if babigame.DessertLiveAutoplayEvidenceGate() {
		t.Fatal("test corpus unexpectedly enables live dessert evidence")
	}

	cases := map[string]dessertBoundedControllerInput{
		"otherwise ready":               validDessertBoundedControllerInput(),
		"zero value":                    {},
		"locked with private detail":    {failureLocked: true, failureReason: "private-board-detail"},
		"forged ownership and revision": {boardOwned: true, runtimeAuthorityRevision: ^uint64(0)},
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			decision := evaluateDessertBoundedController(false, input)
			if decision.readiness != dessertControllerBlocked || decision.blockedReason != dessertEvidenceBlockedReason ||
				strings.Contains(decision.blockedReason, "private-board-detail") {
				t.Fatalf("evidence gate leaked later preflight state: %+v", decision)
			}
		})
	}
}

func TestDessertBoundedControllerFutureSafetyGates(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*dessertBoundedControllerInput)
		want   string
	}{
		{name: "policy disabled", mutate: func(in *dessertBoundedControllerInput) { in.config.enabled = false }, want: "策略未启用"},
		{name: "unsupported mode", mutate: func(in *dessertBoundedControllerInput) { in.config.mode = 2 }, want: "普通模式"},
		{name: "zero budget", mutate: func(in *dessertBoundedControllerInput) { in.config.maxSessionEnergy = 0 }, want: "1～100"},
		{name: "oversized budget", mutate: func(in *dessertBoundedControllerInput) { in.config.maxSessionEnergy = 101 }, want: "1～100"},
		{name: "negative reserve", mutate: func(in *dessertBoundedControllerInput) { in.config.minEnergyReserve = -1 }, want: "不能为负数"},
		{name: "budget exhausted", mutate: func(in *dessertBoundedControllerInput) { in.sessionEnergyUsed = in.config.maxSessionEnergy }, want: "预算"},
		{name: "negative used", mutate: func(in *dessertBoundedControllerInput) { in.sessionEnergyUsed = -1 }, want: "预算"},
		{name: "failure lock", mutate: func(in *dessertBoundedControllerInput) { in.failureLocked = true; in.failureReason = "并发失败锁" }, want: "并发失败锁"},
		{name: "pending drop", mutate: func(in *dessertBoundedControllerInput) { in.pendingDropFingerprint = "pending" }, want: "等待权威响应"},
		{name: "identity mismatch", mutate: func(in *dessertBoundedControllerInput) { in.runtimeBatchID++ }, want: "身份不完整"},
		{name: "reward phase", mutate: func(in *dessertBoundedControllerInput) { in.view.Phase = 3 }, want: "活动进行阶段"},
		{name: "unknown game status", mutate: func(in *dessertBoundedControllerInput) { in.mode.GameStatus = 3 }, want: "gameStatus"},
		{name: "bag unobserved", mutate: func(in *dessertBoundedControllerInput) { in.view.BagObserved = false }, want: "余额尚未完整同步"},
		{name: "bag mismatch", mutate: func(in *dessertBoundedControllerInput) { in.view.Bag[in.view.EnergyItemID]++ }, want: "背包不一致"},
		{name: "at reserve", mutate: func(in *dessertBoundedControllerInput) {
			in.view.EnergyBalance = in.config.minEnergyReserve
			in.view.Bag[in.view.EnergyItemID] = in.view.EnergyBalance
		}, want: "保留值"},
		{name: "runtime revision mismatch", mutate: func(in *dessertBoundedControllerInput) { in.runtimeAuthorityRevision++ }, want: "revision 不一致"},
		{name: "view revision mismatch", mutate: func(in *dessertBoundedControllerInput) { in.view.AuthorityRevision++ }, want: "revision 不一致"},
		{name: "mode revision mismatch", mutate: func(in *dessertBoundedControllerInput) { in.mode.AuthorityRevision++ }, want: "revision 不一致"},
		{name: "baseline revision mismatch", mutate: func(in *dessertBoundedControllerInput) { in.baselineAuthorityRevision++ }, want: "revision 不一致"},
		{name: "runtime hash invalid", mutate: func(in *dessertBoundedControllerInput) { in.runtimeBoardHash = "short" }, want: "typed hash 不一致"},
		{name: "mode hash mismatch", mutate: func(in *dessertBoundedControllerInput) { in.mode.BoardHash = strings.Repeat("b", 64) }, want: "typed hash 不一致"},
		{name: "baseline hash mismatch", mutate: func(in *dessertBoundedControllerInput) { in.baselineBoardHash = strings.Repeat("c", 64) }, want: "typed hash 不一致"},
		{name: "forged ownership", mutate: func(in *dessertBoundedControllerInput) { in.boardOwned = true }, want: "归属标记"},
		{name: "dirty idle board", mutate: func(in *dessertBoundedControllerInput) { in.mode.Objects = []state.DessertObjectView{{Level: 1}} }, want: "非运行态"},
		{name: "running without waiting ball", mutate: func(in *dessertBoundedControllerInput) { in.mode.IsRunning = true }, want: "waiting ball"},
		{name: "nonempty takeover disabled", mutate: func(in *dessertBoundedControllerInput) {
			in.mode.IsRunning = true
			in.mode.CurID = 1
			in.mode.Objects = []state.DessertObjectView{{Level: 1, Position: state.DessertVector2{Y: 0}}}
		}, want: "未启用接管"},
		{name: "takeover lacks warmup", mutate: func(in *dessertBoundedControllerInput) {
			in.config.resumeExisting = true
			in.mode.IsRunning = true
			in.mode.CurID = 1
			in.mode.Objects = []state.DessertObjectView{{Level: 1, Position: state.DessertVector2{Y: 0}}}
		}, want: "warm-up"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := validDessertBoundedControllerInput()
			tc.mutate(&input)
			decision := evaluateDessertBoundedController(true, input)
			if decision.readiness != dessertControllerBlocked || !strings.Contains(decision.blockedReason, tc.want) {
				t.Fatalf("decision=%+v, want blocked reason containing %q", decision, tc.want)
			}
		})
	}
}

func TestDessertBoundedControllerOnlyReportsNonTransportReadiness(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*dessertBoundedControllerInput)
		want   dessertControllerReadiness
	}{
		{name: "clean idle board", want: dessertControllerIdleReady},
		{name: "empty running board", mutate: func(in *dessertBoundedControllerInput) {
			in.mode.IsRunning = true
			in.mode.CurID = 1
		}, want: dessertControllerEmptyRoundReady},
		{name: "owned running board", mutate: func(in *dessertBoundedControllerInput) {
			in.mode.IsRunning = true
			in.mode.CurID = 1
			in.boardOwned = true
			in.createdBySession = true
		}, want: dessertControllerOwnedRoundReady},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := validDessertBoundedControllerInput()
			if tc.mutate != nil {
				tc.mutate(&input)
			}
			decision := evaluateDessertBoundedController(true, input)
			if decision.readiness != tc.want || decision.blockedReason != "" {
				t.Fatalf("decision=%+v, want readiness=%d", decision, tc.want)
			}
		})
	}
}

func TestDessertRuntimeWaitingFreezesLevelUntilNextDrop(t *testing.T) {
	now := time.UnixMilli(dessertRuntimeNowMs)
	r := newDessertRuntimeTestRunner(t, true, 1, 1)
	policy := dessertRuntimePolicy(true)
	r.SetPolicy(policy)

	r.refreshDessertShadowRuntime(now)
	first := r.DessertRuntimeSnapshot()
	if !first.Observed || !first.ShadowOnly || !first.PolicyEnabled || !first.Waiting ||
		first.WaitingRemainingMS != 800 || first.FrozenWaitingLevel != 0 || first.Suggestion != "" ||
		first.BlockedReason != dessertEvidenceBlockedReason || first.BoardOwned || first.LiveEvidenceReady ||
		first.LiveExecutionAllowed || first.MaxSessionEnergy != 5 || first.MinEnergyReserve != 0 {
		t.Fatalf("initial shadow runtime=%+v", first)
	}
	r.refreshDessertShadowRuntime(now.Add(799 * time.Millisecond))
	if got := r.DessertRuntimeSnapshot(); got.WaitingRemainingMS != 1 || got.FrozenWaitingLevel != 0 {
		t.Fatalf("waiting before freeze=%+v", got)
	}
	r.refreshDessertShadowRuntime(now.Add(800 * time.Millisecond))
	if got := r.DessertRuntimeSnapshot(); got.WaitingRemainingMS != 0 || got.FrozenWaitingLevel != 1 {
		t.Fatalf("waiting at freeze=%+v", got)
	}

	// A merge may replace curId without increasing step. The already-frozen
	// waiting ball remains level 1.
	applyDessertRuntimeFixture(t, r.state, true, 2, 1)
	r.refreshDessertShadowRuntime(now.Add(900 * time.Millisecond))
	merged := r.DessertRuntimeSnapshot()
	if merged.FrozenWaitingLevel != 1 || merged.WaitingRemainingMS != 0 || merged.AuthorityRevision <= first.AuthorityRevision {
		t.Fatalf("merge changed frozen waiting level: before=%+v after=%+v", first, merged)
	}

	// A drop is the only authoritative transition that starts a new 800ms
	// window and permits a new level to be frozen.
	applyDessertRuntimeFixture(t, r.state, true, 3, 2)
	r.refreshDessertShadowRuntime(now.Add(time.Second))
	dropped := r.DessertRuntimeSnapshot()
	if dropped.FrozenWaitingLevel != 0 || dropped.WaitingRemainingMS != 800 {
		t.Fatalf("drop did not reset waiting window: %+v", dropped)
	}
	r.refreshDessertShadowRuntime(now.Add(1800 * time.Millisecond))
	if got := r.DessertRuntimeSnapshot(); got.FrozenWaitingLevel != 3 || got.WaitingRemainingMS != 0 {
		t.Fatalf("new waiting level did not freeze: %+v", got)
	}
}

func TestDessertRuntimeFreshLoginAndPolicyToggleResetSessionFailures(t *testing.T) {
	now := time.UnixMilli(dessertRuntimeNowMs)
	r := newDessertRuntimeTestRunner(t, true, 1, 1)
	on := dessertRuntimePolicy(true)
	off := dessertRuntimePolicy(false)
	r.SetPolicy(on)
	r.refreshDessertShadowRuntime(now)

	r.mu.Lock()
	r.dessertRound.FailureLocked = true
	r.dessertRound.failureReason = "test failure"
	r.dessertRound.BlockedReason = "test failure"
	r.dessertRound.SessionEnergyUsed = 4
	r.mu.Unlock()

	// Explicitly turning auto_play off clears the session failure even if it
	// is turned back on before another decision tick runs.
	r.SetPolicy(off)
	if got := r.DessertRuntimeSnapshot(); got.PolicyEnabled || got.FailureLocked || got.SessionEnergyUsed != 4 {
		t.Fatalf("policy-off reset=%+v", got)
	}
	r.SetPolicy(on)
	r.refreshDessertShadowRuntime(now.Add(time.Second))
	if got := r.DessertRuntimeSnapshot(); !got.PolicyEnabled || got.FailureLocked || got.SessionEnergyUsed != 4 {
		t.Fatalf("policy re-enable retained failure=%+v", got)
	}

	beforeEpoch := r.DessertRuntimeSnapshot().SessionEpoch
	r.mu.Lock()
	r.dessertRound.SessionEnergyUsed = 5
	r.dessertRound.BoardOwned = true
	r.dessertRound.pendingDropFingerprint = "pending"
	r.dessertRound.simulatedTime = 5 * time.Second
	r.dessertRound.baselineAuthorityRevision = 99
	r.dessertRound.baselineBoardHash = "baseline"
	r.dessertRound.createdBySession = true
	r.dessertRound.takenOverBySession = true
	r.mu.Unlock()
	wantAuthorityFloor := r.state.DessertAuthorityRevisions()[9101]
	r.resetFreshSessionAutomationState()
	after := r.DessertRuntimeSnapshot()
	if after.SessionEpoch != beforeEpoch+1 || after.SessionEnergyUsed != 0 || after.BoardOwned || after.FailureLocked ||
		!after.ShadowOnly || !after.PolicyEnabled {
		t.Fatalf("fresh-login reset=%+v, before epoch=%d", after, beforeEpoch)
	}
	r.mu.RLock()
	internalReset := r.dessertRound.pendingDropFingerprint == "" && r.dessertRound.simulatedTime == 0 &&
		r.dessertRound.baselineAuthorityRevision == 0 && r.dessertRound.baselineBoardHash == "" &&
		!r.dessertRound.createdBySession && !r.dessertRound.takenOverBySession && !r.dessertRound.stateReady &&
		r.dessertRound.authorityFloor[9101] == wantAuthorityFloor
	r.mu.RUnlock()
	if !internalReset {
		t.Fatalf("fresh login retained internal dessert controller state: %+v", r.dessertRound)
	}
}

func TestDessertRuntimeEnergyBudgetSurvivesBoardIdentityChanges(t *testing.T) {
	now := time.UnixMilli(dessertRuntimeNowMs)
	r := newDessertRuntimeTestRunner(t, true, 1, 1)
	policy := dessertRuntimePolicy(true)
	r.SetPolicy(policy)
	r.refreshDessertShadowRuntime(now)
	r.mu.Lock()
	r.dessertRound.SessionEnergyUsed = 7
	r.mu.Unlock()

	r.state.ApplyV(json.RawMessage(`{"23":{"0":{"9101":null}}}`))
	applyDessertRuntimeFixtureBatch(t, r.state, 9102, true, 2, 1)
	r.refreshDessertShadowRuntime(now.Add(time.Second))
	got := r.DessertRuntimeSnapshot()
	if got.BatchID != 9102 || got.SessionEnergyUsed != 7 {
		t.Fatalf("board identity reset login-epoch energy budget: %+v", got)
	}
}

func TestDessertRuntimeWaitsForCurrentLoginEpochStateBaseline(t *testing.T) {
	now := time.UnixMilli(dessertRuntimeNowMs)
	r := newDessertRuntimeTestRunner(t, true, 1, 1)
	policy := dessertRuntimePolicy(true)
	r.SetPolicy(policy)
	r.refreshDessertShadowRuntime(now)
	before := r.DessertRuntimeSnapshot()
	if !before.Observed || before.BatchID == 0 {
		t.Fatalf("precondition runtime=%+v", before)
	}

	// State intentionally still contains the previous epoch's board here.
	// The runtime must hide it until connectFresh marks the new login baseline.
	r.resetDessertRoundSession()
	r.refreshDessertShadowRuntime(now.Add(time.Second))
	waitingForSync := r.DessertRuntimeSnapshot()
	if waitingForSync.Observed || waitingForSync.BatchID != 0 || waitingForSync.BoardHash != "" || waitingForSync.Waiting ||
		!strings.Contains(waitingForSync.BlockedReason, "初始状态同步") {
		t.Fatalf("fresh epoch reused stale dessert state before sync: %+v", waitingForSync)
	}

	r.markDessertSessionStateReady()
	r.refreshDessertShadowRuntime(now.Add(2 * time.Second))
	waitingForAuthority := r.DessertRuntimeSnapshot()
	if waitingForAuthority.Observed || waitingForAuthority.BatchID != 0 || waitingForAuthority.AuthorityRevision != 0 ||
		waitingForAuthority.BoardHash != "" || waitingForAuthority.Waiting ||
		!strings.Contains(waitingForAuthority.BlockedReason, "新的权威") {
		t.Fatalf("fresh epoch accepted old board after sync marker: %+v", waitingForAuthority)
	}
	r.state.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"14":{"121":{"0":2226}}}}}}`))
	r.refreshDessertShadowRuntime(now.Add(3 * time.Second))
	if scoreOnly := r.DessertRuntimeSnapshot(); scoreOnly.Observed || scoreOnly.BatchID != 0 ||
		!strings.Contains(scoreOnly.BlockedReason, "新的权威") {
		t.Fatalf("score-only delta bypassed login authority floor: %+v", scoreOnly)
	}

	// Even an identical authoritative field-1 payload advances the state
	// revision and proves it came from this login epoch.
	applyDessertRuntimeFixture(t, r.state, true, 1, 1)
	r.refreshDessertShadowRuntime(now.Add(4 * time.Second))
	current := r.DessertRuntimeSnapshot()
	if !current.Observed || current.BatchID != 9101 || current.AuthorityRevision <= before.AuthorityRevision ||
		!current.Waiting || strings.Contains(current.BlockedReason, "新的权威") {
		t.Fatalf("fresh authoritative board did not unlock diagnostics: %+v", current)
	}
}

func TestDessertRuntimeRefreshUsesLivePolicyNotStaleTickSnapshot(t *testing.T) {
	r := newDessertRuntimeTestRunner(t, true, 1, 1)
	on := dessertRuntimePolicy(true)
	off := dessertRuntimePolicy(false)
	r.SetPolicy(on)
	r.refreshDessertShadowRuntime(time.UnixMilli(dessertRuntimeNowMs))
	r.SetPolicy(off)

	// refresh no longer accepts a policy snapshot: it must re-read r.policy
	// under the same lock used by SetPolicy, so the disable wins.
	r.refreshDessertShadowRuntime(time.UnixMilli(dessertRuntimeNowMs + 1))
	if got := r.DessertRuntimeSnapshot(); got.PolicyEnabled || got.Observed || got.Suggestion != "" {
		t.Fatalf("stale tick re-enabled dessert shadow runtime: %+v", got)
	}
}

func TestDessertRuntimeMalformedAndMissingActivityCannotLeakPriorBoard(t *testing.T) {
	now := time.UnixMilli(dessertRuntimeNowMs)
	r := newDessertRuntimeTestRunner(t, true, 1, 1)
	policy := dessertRuntimePolicy(true)
	r.SetPolicy(policy)
	r.refreshDessertShadowRuntime(now)
	initial := r.DessertRuntimeSnapshot()

	r.state.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"14":{"121":{"1":[]}}}}}}`))
	r.refreshDessertShadowRuntime(now.Add(time.Second))
	malformed := r.DessertRuntimeSnapshot()
	if !malformed.Observed || malformed.BatchID != 9101 || malformed.Mode != 1 ||
		malformed.AuthorityRevision != initial.AuthorityRevision+1 || malformed.BoardHash == "" ||
		!malformed.FailureLocked || malformed.Waiting || malformed.FrozenWaitingLevel != 0 || malformed.Suggestion != "" {
		t.Fatalf("malformed authority diagnostics=%+v, initial=%+v", malformed, initial)
	}

	// A disappearing batch clears all board identity and waiting diagnostics,
	// while the session-local failure lock remains explicit until policy reset.
	r.state.ApplyV(json.RawMessage(`{"23":{"0":{"9101":null}}}`))
	r.refreshDessertShadowRuntime(now.Add(2 * time.Second))
	missing := r.DessertRuntimeSnapshot()
	if missing.Observed || missing.BatchID != 0 || missing.Mode != 0 || missing.AuthorityRevision != 0 ||
		missing.BoardHash != "" || missing.BoardOwned || missing.Waiting || missing.WaitingRemainingMS != 0 ||
		missing.FrozenWaitingLevel != 0 || missing.Suggestion != "" || !missing.FailureLocked ||
		!strings.Contains(missing.BlockedReason, "尚未观察") {
		t.Fatalf("missing activity leaked stale runtime=%+v", missing)
	}
}

func TestDessertStaticTakeoverUsesSynAndFullRadiusDangerGates(t *testing.T) {
	mode := state.DessertModeView{Objects: []state.DessertObjectView{{
		Level: 1, IsSyn: true, Position: state.DessertVector2{Y: -300}, Scale: state.DessertVector3{X: 1, Y: 1, Z: 1},
	}}}
	if reason := dessertStaticBoardBlockReason(mode); !strings.Contains(reason, "静止") {
		t.Fatalf("isSyn board was not rejected: %q", reason)
	}

	mode.Objects[0].IsSyn = false
	mode.Objects[0].Position.Y = 260 // level-1 full radius 19.5 crosses y=279.
	if reason := dessertStaticBoardBlockReason(mode); !strings.Contains(reason, "危险线") {
		t.Fatalf("full-radius danger check did not reject board: %q", reason)
	}
}

func TestDessertShadowRuntimeCannotRegisterLiveGameRPCs(t *testing.T) {
	for _, rpc := range []clientproto.RPCName{
		clientproto.RPCActDessertGameStart,
		clientproto.RPCActDessertGameSync,
		clientproto.RPCActDessertGameOver,
	} {
		if _, registered := plannedOperationSpecs[rpc.String()]; registered {
			t.Fatalf("shadow-only commit registered live dessert RPC %s", rpc)
		}
		if _, err := operationArgs(&automation.PlannedOp{Kind: rpc.String()}); err == nil || !strings.Contains(err.Error(), "compile-time blocked") {
			t.Fatalf("argument dispatch accepted live dessert RPC %s: %v", rpc, err)
		}
		if !isHardBlockedDessertGameOperation(rpc.String()) {
			t.Fatalf("live dessert RPC %s missing transport denylist", rpc)
		}
		if _, err := (&Runner{}).executePlannedOp(context.Background(), nil, nil, &automation.PlannedOp{Kind: rpc.String()}); err == nil || !strings.Contains(err.Error(), "compile-time blocked") {
			t.Fatalf("dispatch accepted live dessert RPC %s: %v", rpc, err)
		}
		func() {
			kind := rpc.String()
			original, existed := plannedOperationSpecs[kind]
			defer func() {
				if existed {
					plannedOperationSpecs[kind] = original
				} else {
					delete(plannedOperationSpecs, kind)
				}
			}()
			plannedOperationSpecs[kind] = operationSpec{args: func(*automation.PlannedOp) (any, error) { return struct{}{}, nil }}
			if _, registered := operationSpecFor(kind); registered {
				t.Fatalf("registry lookup exposed accidentally inserted live dessert RPC %s", rpc)
			}
			if _, err := operationArgs(&automation.PlannedOp{Kind: kind}); err == nil || !strings.Contains(err.Error(), "compile-time blocked") {
				t.Fatalf("argument dispatch bypassed denylist after accidental registration for %s: %v", rpc, err)
			}
		}()
	}
	if isHardBlockedDessertGameOperation(clientproto.RPCActDessertEnter.String()) {
		t.Fatal("safe dessert enter was caught by live game denylist")
	}
}

func validDessertBoundedControllerInput() dessertBoundedControllerInput {
	hash := strings.Repeat("a", 64)
	return dessertBoundedControllerInput{
		config: dessertAutoplayPolicy{
			enabled: true, mode: 1, maxSessionEnergy: 5, minEnergyReserve: 1,
		},
		view: state.DessertView{
			Found: true, Valid: true, BatchID: 9101, Phase: 2, BagObserved: true,
			AuthorityRevision: 7, EnergyItemID: 1342, EnergyBalance: 10, Bag: map[int32]int32{1342: 10},
		},
		mode: state.DessertModeView{
			Mode: 1, Observed: true, Valid: true, AuthorityRevision: 7, BoardHash: hash,
		},
		runtimeObserved: true, runtimeBatchID: 9101, runtimeMode: 1,
		runtimeAuthorityRevision: 7, runtimeBoardHash: hash,
		baselineAuthorityRevision: 7, baselineBoardHash: hash,
	}
}

func newDessertRuntimeTestRunner(t *testing.T, running bool, curID, step int32) *Runner {
	t.Helper()
	s := state.New()
	applyDessertRuntimeFixture(t, s, running, curID, step)
	return &Runner{
		state:              s,
		policy:             automation.DefaultPolicy(),
		operationCooldowns: make(map[string]operationCooldown),
		sideLaneFirstWait:  make(map[string]time.Time),
		dessertRound: dessertRoundRuntime{
			DessertRuntimeSnapshot: DessertRuntimeSnapshot{ShadowOnly: true},
			stateReady:             true,
		},
	}
}

func dessertRuntimePolicy(autoPlay bool) *pb.Policy {
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	//nolint:staticcheck // Exercise compatibility with persisted legacy policy JSON.
	policy.Activity.Enabled = true
	policy.Activity.Modules = map[string]*pb.ActivityModulePolicy{
		dessertModuleID: {
			Enabled:    true,
			BoolParams: map[string]bool{dessertAutoPlayPolicy: autoPlay},
			IntParams: map[string]int64{
				dessertModePolicy:             1,
				dessertMaxEnergyPolicy:        5,
				dessertMinEnergyReservePolicy: 0,
			},
		},
	}
	return policy
}

func applyDessertRuntimeFixture(t *testing.T, s *state.State, running bool, curID, step int32) {
	applyDessertRuntimeFixtureBatch(t, s, 9101, running, curID, step)
}

func applyDessertRuntimeFixtureBatch(t *testing.T, s *state.State, batchID int32, running bool, curID, step int32) {
	t.Helper()
	raw, err := os.ReadFile("../state/testdata/dessert_activity.json")
	if err != nil {
		t.Fatalf("read dessert state fixture: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("decode dessert state fixture: %v", err)
	}
	ns23 := document["23"].(map[string]any)
	batches := ns23["0"].(map[string]any)
	batch := batches["9101"].(map[string]any)
	if batchID != 9101 {
		delete(batches, "9101")
		batch["0"] = float64(batchID)
		batches[strconv.FormatInt(int64(batchID), 10)] = batch
		records := ns23["3"].(map[string]any)
		record := records["9101|0"]
		delete(records, "9101|0")
		recordMap := record.(map[string]any)
		recordMap["0"] = float64(batchID)
		records[strconv.FormatInt(int64(batchID), 10)+"|0"] = recordMap
	}
	ext121 := batch["14"].(map[string]any)["121"].(map[string]any)
	mode := ext121["1"].(map[string]any)["1"].(map[string]any)
	mode["0"] = float64(step)
	mode["5"] = running
	mode["7"] = float64(curID)
	updated, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode dessert state fixture: %v", err)
	}
	s.ApplyV(updated)
}
