package runner

import (
	"math"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/dessertphysics"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	dessertAutoPlayPolicy            = "auto_play"
	dessertResumeExistingRoundPolicy = "resume_existing_round"
	dessertModePolicy                = "mode"
	dessertMaxEnergyPolicy           = "max_energy_per_session"
	dessertMinEnergyReservePolicy    = "min_energy_reserve"
	dessertWaitingDuration           = 800 * time.Millisecond
	dessertStaticVelocityThreshold   = 1.0
)

// DessertRuntimeSnapshot is a sanitized, session-local shadow-controller
// diagnostic. It contains no physical object list, account identity, or live
// RPC payload and is safe for the public snapshot mapper to consume.
type DessertRuntimeSnapshot struct {
	Observed           bool
	ShadowOnly         bool
	PolicyEnabled      bool
	SessionEpoch       uint64
	BatchID            int32
	Mode               int32
	AuthorityRevision  uint64
	BoardHash          string
	BoardOwned         bool
	TakeoverRequested  bool
	Waiting            bool
	WaitingRemainingMS int64
	FrozenWaitingLevel int32
	SessionEnergyUsed  int32
	Suggestion         string
	BlockedReason      string
	FailureLocked      bool
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

// DessertRuntimeSnapshot returns a copy of the current login-epoch runtime.
func (r *Runner) DessertRuntimeSnapshot() DessertRuntimeSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dessertRound.DessertRuntimeSnapshot
}

func dessertPolicyConfig(policy *pb.Policy) dessertAutoplayPolicy {
	config := dessertAutoplayPolicy{mode: 1}
	if policy == nil || !policy.GetAutomationEnabled() || !policy.GetActivity().GetEnabled() {
		return config
	}
	module := policy.GetActivity().GetModules()[dessertModuleID]
	if module == nil || !module.GetEnabled() || !module.GetBoolParams()[dessertAutoPlayPolicy] {
		return config
	}
	config.enabled = true
	config.resumeExisting = module.GetBoolParams()[dessertResumeExistingRoundPolicy]
	if value, present := module.GetIntParams()[dessertModePolicy]; present {
		if value < math.MinInt32 || value > math.MaxInt32 {
			config.configurationError = "甜糕模式参数超出范围"
			return config
		}
		config.mode = int32(value)
	}
	if value, present := module.GetIntParams()[dessertMaxEnergyPolicy]; present {
		if value < math.MinInt32 || value > math.MaxInt32 {
			config.configurationError = "甜糕会话体力预算超出范围"
			return config
		}
		config.maxSessionEnergy = int32(value)
	}
	if value, present := module.GetIntParams()[dessertMinEnergyReservePolicy]; present {
		if value < math.MinInt32 || value > math.MaxInt32 {
			config.configurationError = "甜糕最低体力保留值超出范围"
			return config
		}
		config.minEnergyReserve = int32(value)
	}
	if config.mode != 1 {
		config.configurationError = "当前只支持普通模式（mode=1）的影子诊断"
	} else if config.maxSessionEnergy < 0 || config.maxSessionEnergy > 100 {
		config.configurationError = "每次登录的甜糕体力预算必须在 0～100 之间"
	} else if config.minEnergyReserve < 0 {
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

	// The evidence gate is intentionally checked before takeover/static-board
	// decisions. Today the reviewed replay proves topology and deterministic
	// reconstruction, but not a causal trajectory or terminal lifecycle.
	if !babigame.DessertLiveAutoplayEvidenceGate() {
		runtime.BlockedReason = "抓包尚未证明因果轨迹和自然终态，影子建议与实时执行保持阻塞"
		return
	}
	if config.maxSessionEnergy == 0 {
		runtime.BlockedReason = "max_energy_per_session=0，实时自动游戏已禁用"
		return
	}
	if len(mode.Objects) > 0 && !config.resumeExisting {
		runtime.BlockedReason = "检测到非空棋盘，未启用接管现有回合"
		return
	}
	if len(mode.Objects) > 0 {
		if reason := dessertStaticBoardBlockReason(mode); reason != "" {
			runtime.BlockedReason = reason
			return
		}
	}
	// Commit 11 is shadow-only by construction. Passing future gates still
	// cannot manufacture a live action until the bounded controller lands.
	runtime.BlockedReason = "影子控制器尚未产生经验证的安全建议"
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
