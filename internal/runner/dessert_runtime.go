package runner

import (
	"encoding/hex"
	"math"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/dessertphysics"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	dessertWaitingDuration         = 800 * time.Millisecond
	dessertStaticVelocityThreshold = 1.0
	dessertEvidenceBlockedReason   = "抓包已证明游戏生命周期，但因果轨迹与物理回放尚未验证，有界控制器与实时执行保持阻塞"
	// dessertLiveControllerCompiled is a compile-time fuse. Capture evidence
	// and a successful bounded preflight are necessary but never sufficient
	// while this fuse remains false.
	dessertLiveControllerCompiled = false
)

// DessertRuntimeSnapshot is a sanitized, session-local shadow-controller
// diagnostic. It contains no physical object list, account identity, or live
// RPC payload and is safe for the public snapshot mapper to consume.
type DessertRuntimeSnapshot struct {
	Observed             bool
	ShadowOnly           bool
	PolicyEnabled        bool
	LiveEvidenceReady    bool
	LiveExecutionAllowed bool
	SessionEpoch         uint64
	BatchID              int32
	Mode                 int32
	AuthorityRevision    uint64
	BoardHash            string
	BoardOwned           bool
	TakeoverRequested    bool
	Waiting              bool
	WaitingRemainingMS   int64
	FrozenWaitingLevel   int32
	SessionEnergyUsed    int32
	MaxSessionEnergy     int32
	MinEnergyReserve     int32
	Suggestion           string
	BlockedReason        string
	FailureLocked        bool
}

type dessertRoundRuntime struct {
	DessertRuntimeSnapshot
	waitingStartedAt          time.Time
	lastStep                  int32
	failureReason             string
	pendingDropFingerprint    string
	simulatedTime             time.Duration
	baselineAuthorityRevision uint64
	baselineBoardHash         string
	createdBySession          bool
	takenOverBySession        bool
	stateReady                bool
	authorityFloor            map[int32]uint64
}

type dessertAutoplayPolicy struct {
	enabled            bool
	resumeExisting     bool
	mode               int32
	maxSessionEnergy   int32
	minEnergyReserve   int32
	configurationError string
}

// dessertControllerReadiness deliberately describes only controller-local
// readiness. It cannot name an RPC, carry a payload, or be converted into a
// PlannedOp. Live transport registration remains a separate, absent layer.
type dessertControllerReadiness uint8

const (
	dessertControllerBlocked dessertControllerReadiness = iota
	dessertControllerIdleReady
	dessertControllerEmptyRoundReady
	dessertControllerOwnedRoundReady
)

type dessertBoundedControllerDecision struct {
	readiness     dessertControllerReadiness
	blockedReason string
}

// dessertBoundedControllerInput is an immutable projection used by the pure
// safety gate. In particular, board ownership is cross-checked against its
// internal provenance instead of trusting the public diagnostic bit alone.
type dessertBoundedControllerInput struct {
	config                    dessertAutoplayPolicy
	view                      state.DessertView
	mode                      state.DessertModeView
	runtimeObserved           bool
	runtimeBatchID            int32
	runtimeMode               int32
	runtimeAuthorityRevision  uint64
	runtimeBoardHash          string
	baselineAuthorityRevision uint64
	baselineBoardHash         string
	boardOwned                bool
	createdBySession          bool
	takenOverBySession        bool
	failureLocked             bool
	failureReason             string
	sessionEnergyUsed         int32
	pendingDropFingerprint    string
}

// DessertRuntimeSnapshot returns a copy of the current login-epoch runtime.
func (r *Runner) DessertRuntimeSnapshot() DessertRuntimeSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dessertRound.DessertRuntimeSnapshot
}

func dessertPolicyConfig(policy *pb.Policy) dessertAutoplayPolicy {
	config := dessertAutoplayPolicy{mode: 1}
	if policy == nil || !policy.GetAutomationEnabled() {
		return config
	}
	module := policy.GetActivity().GetDessert()
	if module == nil || !module.GetEnabled() || !module.GetAutoPlay() {
		return config
	}
	config.enabled = true
	config.resumeExisting = module.GetResumeExistingRound()
	config.mode = module.GetMode()
	config.maxSessionEnergy = module.GetMaxEnergyPerSession()
	config.minEnergyReserve = module.GetMinEnergyReserve()
	switch {
	case config.mode != 1:
		config.configurationError = "当前只支持普通模式（mode=1）的影子诊断"
	case config.maxSessionEnergy < 0 || config.maxSessionEnergy > 100:
		config.configurationError = "每次登录的甜糕体力预算必须在 0～100 之间"
	case config.minEnergyReserve < 0:
		config.configurationError = "甜糕最低体力保留值不能为负数"
	}
	return config
}

func dessertPolicyEnabled(policy *pb.Policy) bool {
	return dessertPolicyConfig(policy).enabled
}

func (r *Runner) resetDessertRoundSession() {
	authorityFloor := map[int32]uint64(nil)
	if r.state != nil {
		authorityFloor = r.state.DessertAuthorityRevisions()
	}
	r.mu.Lock()
	r.dessertSessionEpoch++
	r.dessertRound = dessertRoundRuntime{DessertRuntimeSnapshot: DessertRuntimeSnapshot{
		ShadowOnly: true, SessionEpoch: r.dessertSessionEpoch, PolicyEnabled: dessertPolicyEnabled(r.policy),
	}, authorityFloor: authorityFloor}
	r.mu.Unlock()
}

func (r *Runner) markDessertSessionStateReady() {
	r.mu.Lock()
	r.dessertRound.stateReady = true
	r.mu.Unlock()
}

func (r *Runner) resetDessertRoundForPolicyLocked(enabled bool) {
	energyUsed := r.dessertRound.SessionEnergyUsed
	stateReady := r.dessertRound.stateReady
	authorityFloor := cloneDessertAuthorityFloor(r.dessertRound.authorityFloor)
	r.dessertRound = dessertRoundRuntime{DessertRuntimeSnapshot: DessertRuntimeSnapshot{
		ShadowOnly: true, SessionEpoch: r.dessertSessionEpoch, PolicyEnabled: enabled, SessionEnergyUsed: energyUsed,
	}, stateReady: stateReady, authorityFloor: authorityFloor}
}

// refreshDessertShadowRuntime observes the authoritative typed board once per
// decision tick. It never creates a PlannedOp and never calls an RPC. Until
// both causal trajectory evidence and every live lifecycle fixture pass the
// hard gate, even the suggested action remains empty.
func (r *Runner) refreshDessertShadowRuntime(now time.Time) {
	view, found := r.state.DessertView(now)

	r.mu.Lock()
	defer r.mu.Unlock()
	// SetPolicy can race a decision tick after its outer snapshot was read.
	// Re-read the authoritative live policy under the runner lock so a disable
	// always wins over a stale tick.
	config := dessertPolicyConfig(r.policy)

	if r.dessertRound.PolicyEnabled != config.enabled {
		r.resetDessertRoundForPolicyLocked(config.enabled)
	}
	runtime := &r.dessertRound
	runtime.ShadowOnly = true
	runtime.PolicyEnabled = config.enabled
	runtime.LiveEvidenceReady = babigame.DessertLiveAutoplayEvidenceGate()
	runtime.LiveExecutionAllowed = false
	runtime.MaxSessionEnergy = config.maxSessionEnergy
	runtime.MinEnergyReserve = config.minEnergyReserve
	runtime.TakeoverRequested = config.resumeExisting
	runtime.Suggestion = ""
	runtime.BlockedReason = ""
	if !config.enabled {
		return
	}
	if !runtime.stateReady {
		clearDessertRuntimeObservation(runtime)
		runtime.TakeoverRequested = config.resumeExisting
		runtime.BlockedReason = "等待本次登录会话的初始状态同步"
		return
	}
	if !found || !view.Found {
		clearDessertRuntimeObservation(runtime)
		runtime.TakeoverRequested = config.resumeExisting
		runtime.BlockedReason = "尚未观察到香卉甜糕活动状态"
		return
	}

	identityChanged := runtime.BatchID != view.BatchID || runtime.Mode != config.mode
	if identityChanged {
		resetDessertRuntimeBoard(runtime)
	}
	runtime.Observed = true
	runtime.BatchID = view.BatchID
	runtime.Mode = config.mode
	runtime.AuthorityRevision = view.AuthorityRevision
	runtime.BoardHash = view.BoardHash
	if view.AuthorityRevision <= runtime.authorityFloor[view.BatchID] {
		clearDessertRuntimeObservation(runtime)
		runtime.TakeoverRequested = config.resumeExisting
		runtime.BlockedReason = "本次登录会话尚未收到新的权威甜糕棋盘"
		return
	}
	if config.configurationError != "" {
		resetDessertRuntimeBoard(runtime)
		runtime.BlockedReason = config.configurationError
		return
	}
	if !view.Valid || !view.ModeMapObserved || !view.ModeMapValid {
		resetDessertRuntimeBoard(runtime)
		if view.ModeMapObserved && !view.ModeMapValid {
			runtime.FailureLocked = true
			runtime.failureReason = "服务端下发了无法解码的权威甜糕棋盘"
		}
		if runtime.FailureLocked {
			runtime.BlockedReason = runtime.failureReason
		} else {
			runtime.BlockedReason = "甜糕活动或模式状态不完整，影子诊断保持阻塞"
		}
		return
	}
	mode, ok := dessertModeByID(view.Modes, config.mode)
	if !ok || !mode.Observed || !mode.Valid {
		resetDessertRuntimeBoard(runtime)
		runtime.BlockedReason = "目标甜糕模式没有完整的权威状态"
		return
	}

	if identityChanged {
		runtime.lastStep = mode.Step
	}
	if !identityChanged && mode.AuthorityRevision < runtime.baselineAuthorityRevision {
		runtime.FailureLocked = true
		runtime.failureReason = "甜糕权威棋盘 revision 发生回退"
	}
	if !identityChanged && mode.AuthorityRevision == runtime.baselineAuthorityRevision && runtime.baselineBoardHash != "" &&
		runtime.baselineBoardHash != mode.BoardHash {
		runtime.FailureLocked = true
		runtime.failureReason = "同一甜糕棋盘 revision 对应了不同 typed hash"
	}
	if identityChanged || mode.AuthorityRevision != runtime.baselineAuthorityRevision {
		if identityChanged || mode.Step > runtime.lastStep {
			runtime.waitingStartedAt = now
			runtime.FrozenWaitingLevel = 0
		}
		if !identityChanged && mode.Step < runtime.lastStep {
			runtime.FailureLocked = true
			runtime.failureReason = "甜糕投放步数发生回退"
		}
		runtime.lastStep = mode.Step
		runtime.pendingDropFingerprint = ""
		runtime.simulatedTime = 0
	}
	runtime.AuthorityRevision = mode.AuthorityRevision
	runtime.BoardHash = mode.BoardHash
	runtime.baselineAuthorityRevision = mode.AuthorityRevision
	runtime.baselineBoardHash = mode.BoardHash
	runtime.BoardOwned = runtime.createdBySession || runtime.takenOverBySession

	updateDessertWaiting(runtime, now, mode)
	if runtime.FailureLocked {
		runtime.BlockedReason = runtime.failureReason
		return
	}

	decision := dessertBoundedControllerDecisionForRuntime(config, view, mode, runtime)
	runtime.BlockedReason = decision.blockedReason
	preflightReady := decision.readiness != dessertControllerBlocked
	runtime.LiveExecutionAllowed = dessertLiveControllerCompiled && runtime.LiveEvidenceReady && preflightReady
	if preflightReady && !runtime.LiveExecutionAllowed {
		// Even a fully eligible result is controller-local diagnostics only.
		// There is intentionally no game RPC operation spec or executor to
		// consume it in this evidence-incomplete build.
		runtime.BlockedReason = "有界控制器安全门槛已满足，但实时游戏 RPC 仍未注册"
	}
}

// dessertBoundedControllerDecisionForRuntime is the only production entry
// point. The embedded evidence gate is read here and cannot be supplied by a
// policy, environment variable, database value, or caller.
func dessertBoundedControllerDecisionForRuntime(
	config dessertAutoplayPolicy,
	view state.DessertView,
	mode state.DessertModeView,
	runtime *dessertRoundRuntime,
) dessertBoundedControllerDecision {
	input := dessertBoundedControllerInput{
		config:                    config,
		view:                      view,
		mode:                      mode,
		runtimeObserved:           runtime.Observed,
		runtimeBatchID:            runtime.BatchID,
		runtimeMode:               runtime.Mode,
		runtimeAuthorityRevision:  runtime.AuthorityRevision,
		runtimeBoardHash:          runtime.BoardHash,
		baselineAuthorityRevision: runtime.baselineAuthorityRevision,
		baselineBoardHash:         runtime.baselineBoardHash,
		boardOwned:                runtime.BoardOwned,
		createdBySession:          runtime.createdBySession,
		takenOverBySession:        runtime.takenOverBySession,
		failureLocked:             runtime.FailureLocked,
		failureReason:             runtime.failureReason,
		sessionEnergyUsed:         runtime.SessionEnergyUsed,
		pendingDropFingerprint:    runtime.pendingDropFingerprint,
	}
	return evaluateDessertBoundedController(babigame.DessertLiveAutoplayEvidenceGate(), input)
}

// evaluateDessertBoundedController is pure so the post-evidence safety gates
// can be exhaustively tested. Production never calls it directly: the wrapper
// above always supplies the verified embedded evidence result. Most
// importantly, evidence=false returns before inspecting any later field, so it
// cannot leak whether a particular board would otherwise be executable.
func evaluateDessertBoundedController(
	evidenceVerified bool,
	input dessertBoundedControllerInput,
) dessertBoundedControllerDecision {
	block := func(reason string) dessertBoundedControllerDecision {
		return dessertBoundedControllerDecision{readiness: dessertControllerBlocked, blockedReason: reason}
	}
	if !evidenceVerified {
		return block(dessertEvidenceBlockedReason)
	}

	config := input.config
	if !config.enabled {
		return block("甜糕实时自动游戏策略未启用")
	}
	if config.configurationError != "" {
		return block(config.configurationError)
	}
	if config.mode != 1 {
		return block("当前只支持普通模式（mode=1）")
	}
	if config.maxSessionEnergy < 1 || config.maxSessionEnergy > 100 {
		return block("max_energy_per_session 必须在 1～100 之间；0 表示禁用")
	}
	if config.minEnergyReserve < 0 {
		return block("甜糕最低体力保留值不能为负数")
	}
	if input.sessionEnergyUsed < 0 || input.sessionEnergyUsed >= config.maxSessionEnergy {
		return block("本次登录会话的甜糕体力预算已用尽或无效")
	}
	if input.failureLocked {
		if input.failureReason != "" {
			return block(input.failureReason)
		}
		return block("甜糕自动游戏会话已锁定")
	}
	if input.pendingDropFingerprint != "" {
		return block("上一笔甜糕投放仍在等待权威响应")
	}

	view := input.view
	mode := input.mode
	if !input.runtimeObserved || !view.Found || !view.Valid || view.BatchID <= 0 ||
		input.runtimeBatchID != view.BatchID || input.runtimeMode != config.mode || mode.Mode != config.mode ||
		!mode.Observed || !mode.Valid {
		return block("甜糕活动、模式或运行时身份不完整")
	}
	if view.Phase != 2 {
		return block("甜糕游戏只允许在活动进行阶段运行")
	}
	if mode.GameStatus < 0 || mode.GameStatus > 2 {
		return block("甜糕 gameStatus 未经客户端语义确认")
	}
	if !view.BagObserved || view.EnergyItemID <= 0 || view.EnergyBalance < 0 {
		return block("甜糕活动体力余额尚未完整同步")
	}
	bagEnergy, energyObserved := view.Bag[view.EnergyItemID]
	if !energyObserved || bagEnergy != view.EnergyBalance {
		return block("甜糕活动体力余额与权威背包不一致")
	}
	if view.EnergyBalance <= config.minEnergyReserve {
		return block("甜糕体力不足以在保留值之上安全消耗 1 点")
	}

	if input.runtimeAuthorityRevision == 0 || view.AuthorityRevision == 0 || mode.AuthorityRevision == 0 ||
		input.baselineAuthorityRevision == 0 ||
		input.runtimeAuthorityRevision != view.AuthorityRevision ||
		input.runtimeAuthorityRevision != mode.AuthorityRevision ||
		input.runtimeAuthorityRevision != input.baselineAuthorityRevision {
		return block("甜糕权威 revision 不一致，疑似存在并发操作")
	}
	if !validDessertBoardHash(input.runtimeBoardHash) || !validDessertBoardHash(input.baselineBoardHash) ||
		!validDessertBoardHash(mode.BoardHash) || input.runtimeBoardHash != mode.BoardHash ||
		input.runtimeBoardHash != input.baselineBoardHash {
		return block("甜糕 typed hash 不一致，疑似存在并发操作")
	}

	ownedBySession := input.createdBySession || input.takenOverBySession
	if input.boardOwned != ownedBySession {
		return block("甜糕棋盘归属标记与会话来源不一致")
	}
	if !mode.IsRunning {
		if ownedBySession || len(mode.Objects) != 0 || mode.CurID != 0 {
			return block("非运行态甜糕棋盘仍含对象、waiting ball 或会话归属")
		}
		return dessertBoundedControllerDecision{readiness: dessertControllerIdleReady}
	}
	if mode.CurID <= 0 {
		return block("运行中的甜糕棋盘缺少合法 waiting ball")
	}
	if ownedBySession {
		return dessertBoundedControllerDecision{readiness: dessertControllerOwnedRoundReady}
	}
	if len(mode.Objects) == 0 {
		return dessertBoundedControllerDecision{readiness: dessertControllerEmptyRoundReady}
	}
	if !config.resumeExisting {
		return block("检测到非空棋盘，未启用接管现有回合")
	}
	if reason := dessertStaticBoardBlockReason(mode); reason != "" {
		return block(reason)
	}
	// dessertStaticBoardBlockReason currently fails every non-empty board at
	// the warm-up gate. Keep this fail-closed fallback in case that helper is
	// later refactored without a corresponding ownership transition.
	return block("现有甜糕棋盘尚未建立可验证的会话归属")
}

func validDessertBoardHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func cloneDessertAuthorityFloor(values map[int32]uint64) map[int32]uint64 {
	if values == nil {
		return nil
	}
	out := make(map[int32]uint64, len(values))
	for batchID, revision := range values {
		out[batchID] = revision
	}
	return out
}

func clearDessertRuntimeObservation(runtime *dessertRoundRuntime) {
	runtime.Observed = false
	runtime.BatchID = 0
	runtime.Mode = 0
	runtime.AuthorityRevision = 0
	runtime.BoardHash = ""
	resetDessertRuntimeBoard(runtime)
}

func resetDessertRuntimeBoard(runtime *dessertRoundRuntime) {
	runtime.BoardOwned = false
	runtime.lastStep = 0
	runtime.pendingDropFingerprint = ""
	runtime.simulatedTime = 0
	runtime.baselineAuthorityRevision = 0
	runtime.baselineBoardHash = ""
	runtime.createdBySession = false
	runtime.takenOverBySession = false
	clearDessertRuntimeWaiting(runtime)
}

func clearDessertRuntimeWaiting(runtime *dessertRoundRuntime) {
	runtime.Waiting = false
	runtime.WaitingRemainingMS = 0
	runtime.FrozenWaitingLevel = 0
	runtime.waitingStartedAt = time.Time{}
}

func dessertModeByID(modes []state.DessertModeView, modeID int32) (state.DessertModeView, bool) {
	for _, mode := range modes {
		if mode.Mode == modeID {
			return mode, true
		}
	}
	return state.DessertModeView{}, false
}

func updateDessertWaiting(runtime *dessertRoundRuntime, now time.Time, mode state.DessertModeView) {
	runtime.Waiting = mode.IsRunning && mode.CurID > 0
	if !runtime.Waiting {
		runtime.waitingStartedAt = time.Time{}
		runtime.WaitingRemainingMS = 0
		runtime.FrozenWaitingLevel = 0
		return
	}
	if runtime.waitingStartedAt.IsZero() {
		runtime.waitingStartedAt = now
	}
	elapsed := now.Sub(runtime.waitingStartedAt)
	if elapsed < 0 {
		elapsed = 0
	}
	remaining := dessertWaitingDuration - elapsed
	if remaining > 0 {
		runtime.WaitingRemainingMS = remaining.Milliseconds()
		if runtime.WaitingRemainingMS == 0 {
			runtime.WaitingRemainingMS = 1
		}
		return
	}
	runtime.WaitingRemainingMS = 0
	if runtime.FrozenWaitingLevel == 0 {
		runtime.FrozenWaitingLevel = mode.CurID
	}
}

// dessertStaticBoardBlockReason is deliberately conservative. A non-empty
// board is rejected unless every body is demonstrably still, outside the
// danger/spawn line, and free of unresolved same-level contact. The final
// warm-up convergence remains a separate future physics gate.
func dessertStaticBoardBlockReason(mode state.DessertModeView) string {
	config := dessertphysics.DefaultConfig()
	for _, object := range mode.Objects {
		if object.Level <= 0 || int(object.Level) > len(config.RadiiPX) {
			return "现有甜糕棋盘包含未知等级，拒绝接管"
		}
		if object.IsSyn || object.IsFallBall || math.Abs(object.LinearVelocity.X) > dessertStaticVelocityThreshold ||
			math.Abs(object.LinearVelocity.Y) > dessertStaticVelocityThreshold ||
			math.Abs(object.AngularVelocity) > dessertStaticVelocityThreshold || object.IsAwake {
			return "现有甜糕棋盘并非可证明静止状态，拒绝接管"
		}
		fullRadius := config.RadiiPX[object.Level-1]
		if object.Position.Y+fullRadius >= config.DangerLinePX {
			return "现有甜糕棋盘进入出生区或危险线，拒绝接管"
		}
	}
	if len(mode.Objects) > 0 {
		return "现有甜糕棋盘尚未通过物理 warm-up 收敛验证，拒绝接管"
	}
	return ""
}
