package automation

import (
	"fmt"
	"sort"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// raceTakeLeadWindow is how early automation may emit takeTask for a CD pool
// row that already meets filter rules. The decision loop wakes at
// AppearTime-lead so the default 4s tick cannot miss this window.
const raceTakeLeadWindow = 300 * time.Millisecond

// raceTaskPoolRefreshInterval is how often automation re-fetches the task pool
// when idle of giveUp/finish/take.
const raceTaskPoolRefreshInterval = 30 * time.Second

// raceTaskPoolBootstrapRetryInterval bounds successful getTaskList probes that
// still do not yield field 114. The first probe remains immediate and urgent;
// later probes allow ordinary work to proceed between attempts.
const raceTaskPoolBootstrapRetryInterval = 30 * time.Second

// raceNearCDSyncSuppressWindow is how close a filter-passing CD task must be
// before periodic getTaskList is deferred. Keeping this much shorter than
// raceTaskPoolRefreshInterval lets long CD waits still refresh upgrade/claim
// state; only the final approach skips sync to favor take timing.
const raceNearCDSyncSuppressWindow = 10 * time.Second

// raceFinishProgressSyncInterval caps getTaskList retries when LocalFinishCnt
// already meets the target but server FinishCnt still lags. A successful
// getTaskList that still leaves FinishCnt short clamps LocalFinishCnt (see
// state.reconcileFmlRaceLocalFinishAfterFullPool), so this is a short nudge
// rather than an unbounded poll.
const raceFinishProgressSyncInterval = 30 * time.Second

// raceInactiveEnterRetryInterval is how often enter may re-probe after an
// inactive batch once the weekly session (or a published start window) is open.
const raceInactiveEnterRetryInterval = 30 * time.Second

// fmlMemberPositionSyncInterval backs off fml.enter when a channel front still
// omits IFmlTot.mb after it was explicitly requested. Guild positions change
// rarely, while an initial zero timestamp still schedules the sync immediately.
const fmlMemberPositionSyncInterval = 10 * time.Minute

// fmlFlowerTakeListRefreshInterval is how often automation re-fetches the
// guild other-share list while take quota remains.
const fmlFlowerTakeListRefreshInterval = time.Hour

// unionRaceOperations emits PlannedOps for the guild race task pool.
// Lifecycle:
//  1. enter + getTaskList (sync) — runs when Enabled, even if AutoEnableModules is off
//  2. delTask (删除低分任务) — independently enabled; requires current guild
//     position permission and never acts on occupied or unscored rows
//  3. takeTask (接取) — requires AutoEnableModules; supports 种植收获、顾客订单、
//     珍珠采集雇佣、花艺制作/售卖、花种培育
//  4. progress — raceTaskProgressDemands drives plant/harvest for 种植收获;
//     顾客订单 / 珍珠雇佣 / 花艺 reuse ordinary (or race-owned flower-art) ops.
//     花种培育 is take/finish only: no progress demand; FinishCnt advances
//     outside race automation (manual / ordinary cultivate) and is synced via
//     getTaskList before finishTask
//  5. finishTask when TargetCnt > 0 && FinishCnt >= TargetCnt (完成并领取积分;
//     TargetCnt<=0 means unknown progress and must not auto-finish)
//
// useSpeedupTicketInTask is honored by maintenanceOperations via
// raceSpeedupEnabled while an unfinished plant-harvest task is held.
// Near ExpireTime (last 10 minutes), speedup tickets are always allowed as a
// forced completion guarantee even when the regular toggle is off.
func unionRaceOperations(s *state.State, policy *pb.UnionRacePolicy, uid int64, now time.Time, gates RaceModuleGates) []PlannedOp {
	if policy == nil || !policy.GetEnabled() {
		return nil
	}
	view := s.FmlRace()
	build := s.FmlBuild()
	if build.MembershipObserved && build.MemberFmlID <= 0 {
		return nil
	}
	// A race batch is itself proof of guild membership for sparse deltas. Until
	// one has been observed, require the authoritative guild ID from namespace
	// 25.0 before sending any race bootstrap RPC.
	if !view.Observed && build.FmlID <= 0 {
		return nil
	}

	goal := Goal{ID: "union.race", Category: CategoryRace, Domain: "union.race", Label: "公会竞赛", Priority: 43}

	// Race enter/task-list responses carry the pool but commonly omit the
	// current IFmlMb record. Low-score deletion needs mb.pos to prove permission,
	// so explicitly enter the guild with mb=1 instead of leaving the setting in
	// a permanent "待同步" state. The runner records every attempt for backoff.
	if policy.GetDeleteLowScoreTask() && !build.MemberPositionObserved &&
		memberPositionSyncDue(build, now) {
		op := domainOp(
			clientproto.RPCFmlEnter.String(), goal, "union.race.sync", "sync",
			"公会竞赛删除权限未同步，拉取当前公会职位", 4490, 0, 0, 0,
		)
		op.CooldownKey = "union.race.delete.member_position"
		// This is a one-shot permission bootstrap. Letting ordinary guild syncs
		// rank ahead can leave both the setting and task pool permanently pending.
		op.PreemptFarm = true
		return []PlannedOp{op}
	}

	// Enter pushes CurFmlRaceBatch (111) and CurFmlRaceRcd (117, raceLvl).
	// Login may already carry 111 without 117; re-enter once while raceLvl is
	// unknown so task-quota totals use the correct guild tier (甲=18/乙=15/…).
	// Task pool / taken-task (114/110) require a follow-up getTaskList.
	// Neither sync step is gated by autoEnableModules.
	// Also re-fetch when a plant-harvest row is missing flower ParamID (once per
	// pool msId set — state.MissingParamRefreshFP prevents tight loops).
	if !view.Observed {
		// Some channel fronts acknowledge enter with an empty delta when no
		// meaningful race snapshot is available. Back off after that successful
		// probe so race bootstrap cannot starve ordinary farm/order work.
		if raceEnterProbeCoolingDown(view, now) {
			return nil
		}
		// Outside an active/calendar contest, enter stays a normal side op so
		// farm/order work is not delayed. During the weekly window, login with
		// no batch yet must still enter before farm so the pool can be claimed.
		op := domainOp(clientproto.RPCFmlRaceEnter.String(), goal, "union.race.enter", "enter", "公会竞赛进入同步", 4400, 0, 0, 0)
		if state.FmlRaceCalendarInSession(now) {
			op.PreemptFarm = true
		}
		return []PlannedOp{op}
	}
	// Re-evaluate the published start/end window at planner time. Apply-time
	// BatchActive stays false if enter ran before Tuesday 09:00.
	view.BatchActive = view.ActiveAt(now)
	if view.BatchStatus != 1 {
		if raceShouldEnterInactiveBatch(view, now) {
			op := domainOp(clientproto.RPCFmlRaceEnter.String(), goal, "union.race.enter", "enter", "公会竞赛开赛同步批次", 4400, 0, 0, 0)
			op.CooldownKey = "union.race.enter.batch"
			op.PreemptFarm = true
			return []PlannedOp{op}
		}
		if !view.BatchActive {
			return nil
		}
	}
	if view.BatchActive && view.RaceLvl <= 0 && s.FmlBuild().RaceLvl <= 0 {
		synced := view.RaceLvlSyncAtMs
		if synced == 0 || !now.Before(time.UnixMilli(synced).Add(raceEnterProbeInterval)) {
			op := domainOp(clientproto.RPCFmlRaceEnter.String(), goal, "union.race.enter", "enter", "公会竞赛同步段位与任务配额", 4399, 0, 0, 0)
			op.CooldownKey = "union.race.enter.race_lvl"
			op.PreemptFarm = true
			return []PlannedOp{op}
		}
	}
	if !view.BatchActive {
		return nil
	}
	// enter/getTaskList may omit field 110; recover fTaskNum from member rank list
	// so UI「已做」and AutoStopOnQuotaDone work after restart.
	if !view.TaskQuotaObserved && view.BatchID > 0 {
		const raceQuotaSyncInterval = 10 * time.Minute
		synced := view.RaceQuotaSyncAtMs
		if synced == 0 || !now.Before(time.UnixMilli(synced).Add(raceQuotaSyncInterval)) {
			op := domainOp(
				clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String(), goal, "union.race.sync", "sync",
				"公会竞赛同步已做次数", 4398, 0, 0, 0,
			)
			op.TaskMsID = view.BatchID
			op.CooldownKey = "union.race.usr_rank"
			return []PlannedOp{op}
		}
	}
	if view.Taken.HasTask && raceTakenExpired(view.Taken, now) {
		if !view.TasksObserved || view.TaskPoolStale || view.TasksSyncedAtMs <= 0 ||
			!now.Before(time.UnixMilli(view.TasksSyncedAtMs).Add(raceExpiredTaskSyncInterval)) {
			op := domainOp(
				clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
				"公会竞赛已接任务过期，重新同步任务池", 4399, 0, 0, 0,
			)
			op.PreemptFarm = true
			return []PlannedOp{op}
		}
		return nil
	}

	// Finish a completed held task before pool sync. Harvest ACKs update
	// FinishCnt via field 134; waiting for getTaskList here risks expire.
	if policy.GetAutoEnableModules() &&
		view.Taken.HasTask && view.Taken.TargetCnt > 0 && view.Taken.FinishCnt >= view.Taken.TargetCnt {
		return []PlannedOp{raceFinishOperation(goal, view.Taken)}
	}

	// Local harvest high-water already covers the target but server FinishCnt
	// still lags (missing/delayed field 134). Force getTaskList so pool field 8
	// can advance FinishCnt and finishTask can fire on the next tick.
	if policy.GetAutoEnableModules() && raceNeedsFinishProgressSync(view, now) {
		op := domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			"公会竞赛本地收获已达标，同步进度以便提交", 4397, 0, 0, 0,
		)
		op.PreemptFarm = true
		return []PlannedOp{op}
	}

	syncPrio := int32(4398)
	switch {
	case RaceHoldsUnfinishedCustomerOrder(view) && gates.Customer:
		syncPrio = raceCustomerSyncPriority
	case RaceHoldsUnfinishedPearlHire(view) && gates.Pearl:
		syncPrio = racePearlSyncPriority
	case RaceHoldsUnfinishedFlowerArtSell(view) || RaceHoldsUnfinishedFlowerArtCraft(view):
		syncPrio = raceFlowerArtSyncPriority
	// Cultivate holds (especially sticky score-36) may advance manually while
	// plant.cultivate is off; still elevate getTaskList so FinishCnt can catch up.
	case RaceHoldsUnfinishedFlowerCultivate(view):
		syncPrio = raceCultivateSyncPriority
	}
	if view.BatchActive && view.TaskPoolStale {
		op := domainOp(clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync", "公会竞赛任务池状态已变化，重新同步", syncPrio, 0, 0, 0)
		op.PreemptFarm = true
		return []PlannedOp{op}
	}
	if view.BatchActive && !view.TasksObserved {
		if !raceTaskPoolBootstrapSyncDue(view, now) {
			return nil
		}
		op := domainOp(clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync", "公会竞赛任务池尚未返回，执行有界重试", syncPrio, 0, 0, 0)
		op.PreemptFarm = true
		return []PlannedOp{op}
	}
	if view.BatchActive && raceTaskPoolNeedsParamRefresh(view) {
		op := domainOp(clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync", "公会竞赛任务目标参数缺失，尝试补全", syncPrio, 0, 0, 0)
		op.PreemptFarm = true
		return []PlannedOp{op}
	}

	// Giving up is destructive user intent and has its own explicit opt-in. It
	// may release a task taken manually outside gardend, so auto-complete alone
	// must never authorize it. Evaluate it after authoritative task sync but
	// independently of AutoEnableModules.
	if view.Taken.HasTask && (view.Taken.TargetCnt <= 0 || view.Taken.FinishCnt < view.Taken.TargetCnt) {
		if reason := raceTakenAbandonReason(s, policy, view, gates); reason != "" {
			op := domainOp(clientproto.RPCFmlRaceGiveUpTask.String(), goal, "union.race.giveUp", "giveUp", reason, 4395, 0, 0, 0)
			op.TaskMsID = view.Taken.TaskMsId
			op.TaskID = view.Taken.TaskType
			if op.TaskID == 0 {
				op.TaskID = view.Taken.TaskId
			}
			op.FlowerID = view.Taken.ParamID
			op.PreemptFarm = true
			return []PlannedOp{op}
		}
	}

	// autoEnableModules gates take/finish/upgrade. Low-score deletion and
	// explicitly enabled give-up are independent policies. When auto-complete is
	// off, the race module still syncs (enter/getTaskList + TTL refresh) so the
	// task pool remains visible and may delete eligible low-score rows, but does
	// not auto-execute task completion flows.
	if !policy.GetAutoEnableModules() {
		if raceTaskPoolTTLStale(view, now) {
			op := domainOp(
				clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
				"公会竞赛定时刷新任务池", 4398, 0, 0, 0,
			)
			op.PreemptFarm = true
			return []PlannedOp{op}
		}
		ops := raceLowScoreDeleteOperations(s, view, policy, goal)
		if op, ok := raceUsrRankScoreSyncOp(view, goal, now); ok {
			ops = append(ops, op)
		}
		return ops
	}
	var ops []PlannedOp

	// Module-backed race progress needs getTaskList after ordinary finishes.
	// Customer/pearl sync only while those modules can still advance counters
	// (module-off holds are abandoned). Flower-art sync has no ordinary toggle
	// gate (race owns those ops). Flower-cultivate is take/finish only: sync
	// FinishCnt from manual / ordinary cultivate without driving progress.
	if len(ops) == 0 && gates.Customer && raceNeedsCustomerProgressSync(view, now) {
		return []PlannedOp{domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			"公会竞赛顾客订单进度同步", raceCustomerSyncPriority, 0, 0, 0,
		)}
	}
	if len(ops) == 0 && gates.Pearl && raceNeedsPearlProgressSync(view, now) {
		return []PlannedOp{domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			"公会竞赛珍珠雇佣进度同步", racePearlSyncPriority, 0, 0, 0,
		)}
	}
	if len(ops) == 0 && raceNeedsFlowerArtProgressSync(view, now) {
		reason := "公会竞赛花艺制作进度同步"
		if RaceHoldsUnfinishedFlowerArtSell(view) {
			reason = "公会竞赛花艺售卖进度同步"
		}
		return []PlannedOp{domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			reason, raceFlowerArtSyncPriority, 0, 0, 0,
		)}
	}
	if len(ops) == 0 && raceNeedsCultivateProgressSync(view, now) {
		return []PlannedOp{domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			"公会竞赛花种培育进度同步", raceCultivateSyncPriority, 0, 0, 0,
		)}
	}

	// 1b. Finish the current taken task if complete.
	// Require TargetCnt>0 so unknown progress (0/0) is never auto-finished.
	if view.Taken.HasTask && view.Taken.TargetCnt > 0 && view.Taken.FinishCnt >= view.Taken.TargetCnt {
		ops = append(ops, raceFinishOperation(goal, view.Taken))
	}

	// 2. Select a task to take (only if not currently holding one).
	// TakeQuotaExhausted is sticky for this batch after the server reports
	// 「任务接取次数已达上限」— do not keep retrying exhausted takes.
	// AutoStopOnQuotaDone also stops take when free-task quota is already used
	// (finished >= total), without waiting for a take rejection.
	if !view.Taken.HasTask && !view.TakeQuotaExhausted && !raceFreeTaskQuotaDone(s, view, policy) {
		selected := selectRaceTasks(s, view.Tasks, policy, uid, now, gates)
		if len(selected) > 0 {
			best := selected[0]
			op := domainOp(clientproto.RPCFmlRaceTakeTask.String(), goal, "union.race.take", "take", "公会竞赛选择最优任务接取", 4380, 0, 0, 0)
			op.TaskMsID = best.MsId
			op.TaskID = best.TaskType
			if op.TaskID == 0 {
				op.TaskID = best.TaskId
			}
			op.FlowerID = best.ParamID
			op.PreemptFarm = true
			ops = append(ops, op)
		}
	}

	hasPrimary := false
	for _, op := range ops {
		if isRacePrimaryMutatingOp(op) {
			hasPrimary = true
			break
		}
	}

	if !hasPrimary && raceTaskPoolTTLStale(view, now) && !raceHasNearTakeableCD(s, view.Tasks, policy, uid, now, gates) {
		op := domainOp(
			clientproto.RPCFmlRaceGetTaskList.String(), goal, "union.race.sync", "sync",
			"公会竞赛定时刷新任务池", 4398, 0, 0, 0,
		)
		op.PreemptFarm = true
		return []PlannedOp{op}
	}

	// 3. Optional: upgrade the currently held task. The observed client sends
	// an empty upgradeTask request, so this RPC cannot target an arbitrary pool
	// row. Its diamond cost must be known and pass the configured budget; the
	// global diamond gate still blocks automatic execution by default.
	if policy.GetUpgradeTask() && view.Taken.HasTask {
		if task, ok := raceTaskByMsID(view.Tasks, view.Taken.TaskMsId); ok && task.IsUpgrade == 0 {
			op := domainOp(clientproto.RPCFmlRaceUpgradeTask.String(), goal, "union.race.upgrade", "upgrade", "公会竞赛当前任务可升级", 4370, 0, 0, 0)
			op.TaskMsID = task.MsId
			op.TaskID = task.TaskType
			if op.TaskID == 0 {
				op.TaskID = task.TaskId
			}
			op.FlowerID = task.ParamID
			cost, costKnown := state.FmlRaceTaskUpgradeCost(task.TaskId, task.Score)
			switch {
			case !costKnown:
				op.Status = PlanStatusAdapterMissing
				op.Executable = false
				op.BlockedReasons = []string{"公会竞赛任务升级成本无法从客户端配置确认"}
			case policy.GetMaxSpendDiamond() <= 0:
				op.DiamondCost = cost
				op.Status = PlanStatusBlocked
				op.Executable = false
				op.BlockedReasons = []string{"公会竞赛任务升级元宝预算未设置"}
			case int64(cost) > policy.GetMaxSpendDiamond():
				op.DiamondCost = cost
				op.Status = PlanStatusBlocked
				op.Executable = false
				op.BlockedReasons = []string{"公会竞赛任务升级元宝成本超过策略上限"}
			default:
				op.DiamondCost = cost
			}
			ops = append(ops, op)
		}
	}

	// 4. Optional: delete low-score tasks. A stale pool is never mutated; when
	// a primary take is concurrently due, the take wins and deletion waits for
	// the resulting fresh task delta / next pool sync.
	if !raceTaskPoolTTLStale(view, now) {
		ops = append(ops, raceLowScoreDeleteOperations(s, view, policy, goal)...)
	}

	// Idle: sync personal score/rank without preempting take/finish/giveUp.
	// getTaskList also piggybacks a member-rank fetch for the common path.
	if len(ops) == 0 {
		if op, ok := raceUsrRankScoreSyncOp(view, goal, now); ok {
			return []PlannedOp{op}
		}
	}

	return ops
}

func memberPositionSyncDue(build state.FmlBuildView, now time.Time) bool {
	if build.MemberPositionObserved {
		return false
	}
	if build.MemberPositionSyncAtMs <= 0 {
		return true
	}
	return !now.Before(time.UnixMilli(build.MemberPositionSyncAtMs).Add(fmlMemberPositionSyncInterval))
}

func raceLowScoreDeleteOperations(s *state.State, view state.FmlRaceView, policy *pb.UnionRacePolicy, goal Goal) []PlannedOp {
	if s == nil || policy == nil || !policy.GetDeleteLowScoreTask() || policy.GetDeleteTaskMaxScore() <= 0 || !view.TasksObserved || view.TaskPoolStale {
		return nil
	}
	maxScore := policy.GetDeleteTaskMaxScore()
	candidates := make([]state.FmlRaceTaskView, 0, len(view.Tasks))
	for _, task := range view.Tasks {
		// Score zero is indistinguishable from an omitted field in the observed
		// client shape. Destructive maintenance must not treat missing data as a
		// real zero-point task. AppearTime controls when a replacement task can
		// be taken; it is not an observed deletion precondition. If the server
		// still rejects deletion during refresh, the runner defers this task by
		// its task-scoped cooldown without blocking other candidates.
		if task.MsId <= 0 || task.UID != 0 || task.Score <= 0 || task.Score > maxScore {
			continue
		}
		candidates = append(candidates, task)
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score < candidates[j].Score
		}
		return candidates[i].MsId < candidates[j].MsId
	})

	build := s.FmlBuild()
	blockedReason := ""
	switch {
	case !build.MemberPositionObserved:
		blockedReason = "公会职位尚未同步，无法确认竞赛任务删除权限"
	case !state.FmlPositionAllowsRaceDelete(build.MemberPosition):
		blockedReason = "当前公会职位没有删除竞赛任务的权限"
	}

	ops := make([]PlannedOp, 0, len(candidates))
	for _, task := range candidates {
		op := domainOp(clientproto.RPCFmlRaceDelTask.String(), goal, "union.race.delete", "delete", "公会竞赛低分任务清理", 4360, 0, 0, 0)
		op.TaskMsID = task.MsId
		op.TaskID = task.TaskType
		if op.TaskID == 0 {
			op.TaskID = task.TaskId
		}
		op.FlowerID = task.ParamID
		// Global operation ordering uses OperationID as its final stable key.
		// Preserve score-first deletion after the complete plan is sorted while
		// keeping retry cooldowns scoped to the mutable task instance.
		op.OperationID = fmt.Sprintf("%s|score=%010d|task=%020d", op.OperationID, task.Score, task.MsId)
		op.CooldownKey = fmt.Sprintf("union.race.delete:%d", task.MsId)
		if blockedReason != "" {
			op.Status = PlanStatusBlocked
			op.Executable = false
			op.BlockingStage = "permission"
			op.BlockedReasons = []string{blockedReason}
			return []PlannedOp{op}
		}
		ops = append(ops, op)
	}
	return ops
}

const raceEnterProbeInterval = 10 * time.Minute

func raceEnterProbeCoolingDown(view state.FmlRaceView, now time.Time) bool {
	return view.RaceLvlSyncAtMs > 0 && now.Before(time.UnixMilli(view.RaceLvlSyncAtMs).Add(raceEnterProbeInterval))
}

// raceShouldEnterInactiveBatch re-probes fmlRace.enter when the stored batch is
// not in-progress. Without this, an ended last-week snapshot (status==2) or a
// published future window (status==0) never discovers Tuesday 09:00 opening.
func raceShouldEnterInactiveBatch(view state.FmlRaceView, now time.Time) bool {
	if view.BatchStatus != 2 && view.BatchStartMs > 0 {
		start := time.UnixMilli(view.BatchStartMs)
		if now.Before(start) {
			return false
		}
		if view.BatchEndMs <= 0 || now.Before(time.UnixMilli(view.BatchEndMs)) {
			return raceInactiveEnterRetryDue(view, now, start)
		}
	}
	if !state.FmlRaceCalendarInSession(now) {
		return false
	}
	return raceInactiveEnterRetryDue(view, now, state.FmlRaceCalendarSessionStart(now))
}

func raceInactiveEnterRetryDue(view state.FmlRaceView, now, sessionStart time.Time) bool {
	if view.RaceLvlSyncAtMs <= 0 {
		return true
	}
	last := time.UnixMilli(view.RaceLvlSyncAtMs)
	if !sessionStart.IsZero() && last.Before(sessionStart) {
		return true
	}
	return !now.Before(last.Add(raceInactiveEnterRetryInterval))
}

func raceTaskPoolTTLStale(view state.FmlRaceView, now time.Time) bool {
	if !view.BatchActive || !view.TasksObserved || view.TaskPoolStale {
		return false
	}
	if view.TasksSyncedAtMs <= 0 {
		return true
	}
	return !now.Before(time.UnixMilli(view.TasksSyncedAtMs).Add(raceTaskPoolRefreshInterval))
}

func raceTaskPoolBootstrapSyncDue(view state.FmlRaceView, now time.Time) bool {
	if view.TasksObserved {
		return false
	}
	if view.TaskPoolSyncAttemptAtMs <= 0 {
		return true
	}
	return !now.Before(time.UnixMilli(view.TaskPoolSyncAttemptAtMs).Add(raceTaskPoolBootstrapRetryInterval))
}

// raceNeedsFinishProgressSync reports that plant-harvest LocalFinishCnt already
// meets TargetCnt while authoritative FinishCnt has not, so the planner must
// refresh the task pool instead of idling until the regular pool refresh.
// Recent successful getTaskList (within raceFinishProgressSyncInterval) is
// respected so a lagging server FinishCnt cannot re-plan sync every decision tick.
func raceNeedsFinishProgressSync(view state.FmlRaceView, now time.Time) bool {
	taken := view.Taken
	if !taken.HasTask || taken.TargetCnt <= 0 || taken.FinishCnt >= taken.TargetCnt {
		return false
	}
	if view.LocalFinishTaskMsId != taken.TaskMsId || view.LocalFinishCnt < taken.TargetCnt {
		return false
	}
	if view.TasksObserved && view.TasksSyncedAtMs > 0 &&
		now.Before(time.UnixMilli(view.TasksSyncedAtMs).Add(raceFinishProgressSyncInterval)) {
		return false
	}
	return true
}

func raceHasNearTakeableCD(s *state.State, tasks []state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64, now time.Time, gates RaceModuleGates) bool {
	nowMs := now.UnixMilli()
	for _, t := range tasks {
		if t.AppearTime <= 0 || t.AppearTime <= nowMs {
			continue
		}
		if t.UID != 0 {
			continue
		}
		rem := time.Duration(t.AppearTime-nowMs) * time.Millisecond
		if rem >= raceNearCDSyncSuppressWindow {
			continue
		}
		if raceTakeNonCDSkipReason(s, t, policy, uid, gates) == "" {
			return true
		}
	}
	return false
}

// RaceTakeWakeAt is when the decision loop should next tick to emit takeTask
// for a filter-passing CD pool row: AppearTime minus raceTakeLeadWindow.
// Zero means there is nothing to wait for (already holding, already takeable
// this tick, auto-complete off, or no eligible CD task).
func RaceTakeWakeAt(s *state.State, policy *pb.Policy, now time.Time) time.Time {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() {
		return time.Time{}
	}
	race := policy.GetUnion().GetRace()
	if race == nil || !race.GetEnabled() || !race.GetAutoEnableModules() {
		return time.Time{}
	}
	view := s.FmlRace()
	view.BatchActive = view.ActiveAt(now)
	if !view.BatchActive || !view.TasksObserved || view.TaskPoolStale || view.Taken.HasTask ||
		view.TakeQuotaExhausted || raceFreeTaskQuotaDone(s, view, race) {
		return time.Time{}
	}
	uid := s.RoleID()
	gates := raceModuleGatesFromPolicy(policy)
	nowMs := now.UnixMilli()
	leadMs := raceTakeLeadWindow.Milliseconds()
	var bestAppear int64
	anyTakeableNow := false
	for _, t := range view.Tasks {
		if t.UID != 0 {
			continue
		}
		if raceTakeNonCDSkipReason(s, t, race, uid, gates) != "" {
			continue
		}
		if t.AppearTime <= 0 || t.AppearTime <= nowMs || t.AppearTime-nowMs <= leadMs {
			anyTakeableNow = true
			continue
		}
		if bestAppear == 0 || t.AppearTime < bestAppear {
			bestAppear = t.AppearTime
		}
	}
	if anyTakeableNow || bestAppear == 0 {
		return time.Time{}
	}
	return time.UnixMilli(bestAppear - leadMs)
}

// RaceTakeDue reports that takeTask should fire on this tick (ready now or
// inside raceTakeLeadWindow). The decision loop uses this to skip water /
// resident / reputation preamble so the 300ms lead is not eaten before RPC.
func RaceTakeDue(s *state.State, policy *pb.Policy, now time.Time) bool {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() {
		return false
	}
	race := policy.GetUnion().GetRace()
	if race == nil || !race.GetEnabled() || !race.GetAutoEnableModules() {
		return false
	}
	view := s.FmlRace()
	view.BatchActive = view.ActiveAt(now)
	if !view.BatchActive || !view.TasksObserved || view.TaskPoolStale || view.Taken.HasTask ||
		view.TakeQuotaExhausted || raceFreeTaskQuotaDone(s, view, race) {
		return false
	}
	return len(selectRaceTasks(s, view.Tasks, race, s.RoleID(), now, raceModuleGatesFromPolicy(policy))) > 0
}

// RaceBootstrapDue reports that race enter / getTaskList / take / finish must
// run before farm or order work: unobserved pool after login, takeable rows, or
// a held task ready to submit.
func RaceBootstrapDue(s *state.State, policy *pb.Policy, now time.Time) bool {
	if s == nil || policy == nil || !policy.GetAutomationEnabled() {
		return false
	}
	race := policy.GetUnion().GetRace()
	if race == nil || !race.GetEnabled() {
		return false
	}
	view := s.FmlRace()
	build := s.FmlBuild()
	if !build.MembershipObserved || build.MemberFmlID <= 0 {
		return false
	}
	if !view.Observed && build.FmlID <= 0 {
		return false
	}
	if !view.Observed {
		if raceEnterProbeCoolingDown(view, now) {
			return false
		}
		// First enter only preempts during the weekly contest window.
		return state.FmlRaceCalendarInSession(now)
	}
	view.BatchActive = view.ActiveAt(now)
	if !view.BatchActive {
		// Still allow calendar-window enter probes (status may still be 0).
		return raceShouldEnterInactiveBatch(view, now)
	}
	if view.TaskPoolStale {
		return true
	}
	if !view.TasksObserved {
		return raceTaskPoolBootstrapSyncDue(view, now)
	}
	if view.Taken.HasTask && view.Taken.TargetCnt > 0 && view.Taken.FinishCnt >= view.Taken.TargetCnt {
		return race.GetAutoEnableModules()
	}
	if !race.GetAutoEnableModules() {
		return false
	}
	return RaceTakeDue(s, policy, now)
}

// IsUrgentRaceOp reports ops that must preempt farm/order lanes (login/pool
// sync, take/giveUp/finish). Bare off-week enter remains normal.
func IsUrgentRaceOp(op PlannedOp) bool {
	return op.PreemptFarm
}

// IsUrgentRaceDomain reports domains that may preempt farm/order when the
// planned op also carries PreemptFarm (see IsUrgentRaceOp).
func IsUrgentRaceDomain(domain string) bool {
	switch domain {
	case "union.race.enter", "union.race.sync", "union.race.take",
		"union.race.giveUp", "union.race.finish":
		return true
	default:
		return false
	}
}

func isRacePrimaryMutatingOp(op PlannedOp) bool {
	switch op.Kind {
	case clientproto.RPCFmlRaceGiveUpTask.String(),
		clientproto.RPCFmlRaceFinishTask.String(),
		clientproto.RPCFmlRaceTakeTask.String():
		return true
	default:
		return false
	}
}

// raceUsrRankScoreSyncOp plans getFmlRaceUsrRankList for personal score/rank
// when missing, or periodically after a dedicated sync. Callers must only use
// this when it would not preempt primary race take/finish/giveUp work.
func raceUsrRankScoreSyncOp(view state.FmlRaceView, goal Goal, now time.Time) (PlannedOp, bool) {
	if view.BatchID <= 0 {
		return PlannedOp{}, false
	}
	needScoreRank := !view.ScoreObserved || !view.RankObserved
	const raceScoreRankSyncInterval = 10 * time.Minute
	synced := view.RaceQuotaSyncAtMs
	backoffOK := synced == 0 || !now.Before(time.UnixMilli(synced).Add(raceScoreRankSyncInterval))
	periodic := !needScoreRank && synced > 0 && !now.Before(time.UnixMilli(synced).Add(raceScoreRankSyncInterval))
	if (!needScoreRank || !backoffOK) && !periodic {
		return PlannedOp{}, false
	}
	op := domainOp(
		clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String(), goal, "union.race.sync", "sync",
		"公会竞赛同步个人得分与排名", 4398, 0, 0, 0,
	)
	op.TaskMsID = view.BatchID
	op.CooldownKey = "union.race.usr_rank"
	return op, true
}

// raceFreeTaskQuotaDone reports that AutoStopOnQuotaDone should block further
// takeTask planning: usr-rcd quota was observed and finished_task_num already
// covers the free tier total (c_fmlRace(raceLvl).taskNum). Purchased extras
// (buyTaskNum) are intentionally ignored so automation stops at the UI「已做」
// free quota. Unknown raceLvl / unobserved quota returns false.
func raceFreeTaskQuotaDone(s *state.State, view state.FmlRaceView, policy *pb.UnionRacePolicy) bool {
	if policy == nil || !policy.GetAutoStopOnQuotaDone() {
		return false
	}
	if !view.TaskQuotaObserved {
		return false
	}
	raceLvl := view.RaceLvl
	if raceLvl <= 0 && s != nil {
		raceLvl = s.FmlBuild().RaceLvl
	}
	total := state.FmlRaceTotalTaskNum(raceLvl, view.BuyTaskNum)
	if total <= 0 {
		return false
	}
	return view.FinishedTaskNum >= total
}

func raceFinishOperation(goal Goal, taken state.FmlRaceTakenView) PlannedOp {
	prio := int32(4390)
	taskType := taken.TaskType
	if taskType == 0 {
		taskType = taken.TaskId
	}
	switch taskType {
	case raceTaskTypeCustomerOrder:
		prio = raceCustomerFinishPriority
	case raceTaskTypePearlHire:
		prio = racePearlFinishPriority
	case raceTaskTypeFlowerArtSell, raceTaskTypeFlowerArtCraft:
		prio = raceFlowerArtFinishPriority
	case raceTaskTypeFlowerCultivate:
		prio = raceCultivateFinishPriority
	}
	op := domainOp(clientproto.RPCFmlRaceFinishTask.String(), goal, "union.race.finish", "finish", "公会竞赛任务已完成，提交领取积分", prio, 0, 0, 0)
	op.TaskMsID = taken.TaskMsId
	op.TaskID = taskType
	if op.TaskID == 0 {
		op.TaskID = taken.TaskId
	}
	op.FlowerID = taken.ParamID
	op.PreemptFarm = true
	return op
}

// RaceTakeSkipReason returns the primary reason automation will not take this
// pool task, or "" if it is takeable (including preemptive CD within raceTakeLeadWindow).
// Keep priority and CD copy branching aligned with the table-driven tests.
func RaceTakeSkipReason(s *state.State, t state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64, now time.Time, gates RaceModuleGates) string {
	if t.UID != 0 {
		return "已被接取"
	}
	leadUntil := now.Add(raceTakeLeadWindow).UnixMilli()
	if t.AppearTime > 0 && t.AppearTime > leadUntil {
		hhmmss := time.UnixMilli(t.AppearTime).Local().Format("15:04:05")
		if raceTakeNonCDSkipReason(s, t, policy, uid, gates) != "" {
			return hhmmss + " 后刷新"
		}
		return "冷却中，" + hhmmss + " 后可接"
	}
	return raceTakeNonCDSkipReason(s, t, policy, uid, gates)
}

// ManualRaceTakeOperation validates a user-selected task against the same
// observed state, quota, cooldown, module, and policy gates as automatic
// selection. AutoEnableModules is intentionally not required: the race module
// switch controls visibility/availability, while the explicit click supplies
// the execution intent.
func ManualRaceTakeOperation(s *state.State, policy *pb.Policy, taskMsID int64, now time.Time) (PlannedOp, error) {
	if s == nil || policy == nil {
		return PlannedOp{}, fmt.Errorf("公会竞赛状态不可用")
	}
	if taskMsID <= 0 {
		return PlannedOp{}, fmt.Errorf("竞赛任务标识无效")
	}
	race := policy.GetUnion().GetRace()
	if race == nil || !race.GetEnabled() {
		return PlannedOp{}, fmt.Errorf("请先开启公会竞赛")
	}
	view := s.FmlRace()
	view.BatchActive = view.ActiveAt(now)
	if !view.Observed || !view.BatchActive {
		return PlannedOp{}, fmt.Errorf("当前不在有效的公会竞赛批次中")
	}
	if !view.TasksObserved || view.TaskPoolStale {
		return PlannedOp{}, fmt.Errorf("竞赛任务池尚未同步")
	}
	if view.Taken.HasTask {
		return PlannedOp{}, fmt.Errorf("已有竞赛任务，请先完成或放弃当前任务")
	}
	if view.TakeQuotaExhausted || raceFreeTaskQuotaDone(s, view, race) {
		return PlannedOp{}, fmt.Errorf("竞赛任务接取次数已用完")
	}
	task, ok := raceTaskByMsID(view.Tasks, taskMsID)
	if !ok {
		return PlannedOp{}, fmt.Errorf("任务已不在当前任务池，请等待列表刷新")
	}
	if reason := RaceTakeSkipReason(s, task, race, s.RoleID(), now, raceModuleGatesFromPolicy(policy)); reason != "" {
		return PlannedOp{}, fmt.Errorf("当前不可接取：%s", reason)
	}
	goal := Goal{ID: "union.race", Category: CategoryRace, Domain: "union.race", Label: "公会竞赛", Priority: 43}
	op := domainOp(clientproto.RPCFmlRaceTakeTask.String(), goal, "union.race.take", "take", "手动接取公会竞赛任务", 4380, 0, 0, 0)
	op.TaskMsID = task.MsId
	op.TaskID = task.TaskType
	if op.TaskID == 0 {
		op.TaskID = task.TaskId
	}
	op.FlowerID = task.ParamID
	op.PreemptFarm = true
	return op, nil
}

// ManualRaceDeleteOperation validates an explicit task-pool deletion without
// applying the automatic low-score threshold. Human intent supplies task
// selection, while current-cycle membership, permission, task freshness, and
// server cooldown evidence still fail closed.
func ManualRaceDeleteOperation(s *state.State, policy *pb.Policy, taskMsID int64, now time.Time) (PlannedOp, error) {
	if s == nil || policy == nil {
		return PlannedOp{}, fmt.Errorf("公会竞赛状态不可用")
	}
	if taskMsID <= 0 {
		return PlannedOp{}, fmt.Errorf("竞赛任务标识无效")
	}
	race := policy.GetUnion().GetRace()
	if race == nil || !race.GetEnabled() {
		return PlannedOp{}, fmt.Errorf("请先开启公会竞赛")
	}
	view := s.FmlRace()
	if !view.Observed || !view.ActiveAt(now) {
		return PlannedOp{}, fmt.Errorf("当前不在有效的公会竞赛批次中")
	}
	if !view.TasksObserved || view.TaskPoolStale {
		return PlannedOp{}, fmt.Errorf("竞赛任务池尚未同步")
	}
	task, ok := raceTaskByMsID(view.Tasks, taskMsID)
	if !ok {
		return PlannedOp{}, fmt.Errorf("任务已不在当前任务池，请等待列表刷新")
	}
	if reason := RaceDeleteSkipReason(s, task, now); reason != "" {
		return PlannedOp{}, fmt.Errorf("当前不可删除：%s", reason)
	}
	goal := Goal{ID: "union.race", Category: CategoryRace, Domain: "union.race", Label: "公会竞赛", Priority: 43}
	op := domainOp(clientproto.RPCFmlRaceDelTask.String(), goal, "union.race.delete", "delete", "手动删除公会竞赛任务", 4385, 0, 0, 0)
	op.TaskMsID = task.MsId
	op.TaskID = task.TaskType
	if op.TaskID == 0 {
		op.TaskID = task.TaskId
	}
	op.FlowerID = task.ParamID
	op.CooldownKey = fmt.Sprintf("union.race.delete:%d", task.MsId)
	return op, nil
}

// RaceDeleteSkipReason describes the state-derived gate for a manual delete.
// Empty means the task can be offered to the current user.
func RaceDeleteSkipReason(s *state.State, task state.FmlRaceTaskView, now time.Time) string {
	if s == nil {
		return "公会状态不可用"
	}
	build := s.FmlBuild()
	switch {
	case !build.MembershipObserved:
		return "公会成员身份尚未同步"
	case build.MemberFmlID <= 0:
		return "当前账号未加入公会"
	case !build.MemberPositionObserved:
		return "公会职位尚未同步"
	case !state.FmlPositionAllowsRaceDelete(build.MemberPosition):
		return "当前公会职位没有删除权限"
	case task.MsId <= 0:
		return "竞赛任务标识无效"
	case task.UID != 0:
		return "任务已被成员接取"
	case task.AppearTime > now.UnixMilli():
		return "任务槽位冷却中"
	default:
		return ""
	}
}

// raceTakeNonCDSkipReason evaluates take filters other than far-CD AppearTime.
// Empty means those filters would allow take (ready / within-lead still apply outside).
func raceTakeNonCDSkipReason(s *state.State, t state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64, gates RaceModuleGates) string {
	if policy.GetMinTaskScore() > 0 && t.Score <= policy.GetMinTaskScore() {
		return fmt.Sprintf("分数不足（≤%d）", policy.GetMinTaskScore())
	}
	if policy.GetAvoidProgressedTasks() && t.FinishCnt > 0 {
		if t.TargetCnt > 0 {
			return fmt.Sprintf("已有进度（%d/%d）", t.FinishCnt, t.TargetCnt)
		}
		return fmt.Sprintf("已有进度（%d）", t.FinishCnt)
	}
	if policy.GetOnlyUpgradeTask() && t.IsUpgrade == 0 {
		return "仅接已升级任务"
	}
	// Only member upgrades carry UpgradeUid. UpgradeUid==0 is system upgrade
	// and stays takeable when exclude-others is on.
	if policy.GetExcludeOthersUpgradeTask() && t.UpgradeUid != 0 && t.UpgradeUid != uid {
		return "他人已升级"
	}
	taskType := t.TaskType
	if taskType == 0 {
		taskType = t.TaskId
	}
	if raceTaskTypePriority(policy, taskType) <= 0 {
		return "优先级为0"
	}
	if !raceTaskTypeAutoCompletable(taskType) {
		return "暂不支持自动完成"
	}
	switch taskType {
	case raceTaskTypePlantHarvest:
		if t.ParamID <= 0 || !flowerCultivated(s, t.ParamID) {
			return "目标花卉未培养"
		}
	case raceTaskTypeCustomerOrder:
		if !gates.Customer {
			return "顾客订单模块未开启"
		}
	case raceTaskTypePearlHire:
		if !gates.Pearl {
			return "珍珠雇佣模块未开启"
		}
	case raceTaskTypeFlowerArtCraft:
		if t.ParamID <= 0 {
			return "目标花瓶未知"
		}
		if s == nil || !s.VaseObserved() {
			return "未观察到花瓶状态"
		}
		if !s.HasVase(t.ParamID) {
			return "目标花瓶未解锁"
		}
	case raceTaskTypeFlowerCultivate:
		// Take does not require plant.cultivate. Race does not drive cultivate
		// ops — only take / sync FinishCnt / finishTask.
		if t.Score != raceFlowerCultivateRequiredScore {
			return fmt.Sprintf("仅接%d分花种培育", raceFlowerCultivateRequiredScore)
		}
	}
	return ""
}

func raceTaskByMsID(tasks []state.FmlRaceTaskView, msID int64) (state.FmlRaceTaskView, bool) {
	for _, task := range tasks {
		if task.MsId == msID {
			return task, true
		}
	}
	return state.FmlRaceTaskView{}, false
}

// raceTaskPoolNeedsParamRefresh reports whether getTaskList should run again
// because a plant-harvest or flower-art-craft task has no target ParamID and
// this incomplete pool identity has not yet been refresh-attempted.
func raceTaskPoolNeedsParamRefresh(view state.FmlRaceView) bool {
	if !state.FmlRacePoolMissingParam(view.Tasks) {
		return false
	}
	return state.FmlRaceMissingParamFingerprint(view.Tasks) != view.MissingParamRefreshFP
}

// raceTaskTypePriority returns the configured priority for a race task type.
// Missing map entries fall back to defaultUnionRacePriority (0 = do not take).
func raceTaskTypePriority(policy *pb.UnionRacePolicy, taskType int32) int32 {
	if m := policy.GetTaskTypePriority(); m != nil {
		if p, ok := m[taskType]; ok {
			return p
		}
	}
	return defaultUnionRacePriority()[taskType]
}

// selectRaceTasks filters the available task pool via RaceTakeSkipReason, then
// sorts by configured priority.
//
// min_task_score is a lower bound: tasks with Score <= minScore are skipped.
// 0 means no score filtering. Combined with only_upgrade_task, only upgraded tasks
// above the threshold are eligible.
//
// task_type_priority: 0 (or missing → default 0) means do not take that type.
// Positive values rank candidates (higher first), then Score descending.
//
// Plant-harvest (3036): skip when ParamID is missing or the flower is not yet
// cultivated (Status==2 && Lvl>0). Seed stock / empty land are not required.
//
// Flower-art sell (3030) / craft (3034): race auto-complete drives
// flowerRack.sell / makeFlowerArt itself; order.flower_art toggles are not
// required for take or progress.
//
// avoid_progressed_tasks applies to every type before type-specific gates and
// only affects future takes; an already-held task is still completed normally.
//
// Flower-cultivate (3044): only Score==36; plant.cultivate is not required.
// Race does not drive cultivate ops — only take, progress sync, and finishTask
// once FinishCnt catches up.
//
// AppearTime gating: ready tasks (appearTime already due) are preferred. CD tasks
// within raceTakeLeadWindow may be selected preemptively when no ready candidate
// remains; farther CD tasks are skipped.
func selectRaceTasks(s *state.State, tasks []state.FmlRaceTaskView, policy *pb.UnionRacePolicy, uid int64, now time.Time, gates RaceModuleGates) []state.FmlRaceTaskView {
	nowMs := now.UnixMilli()

	var ready, upcoming []state.FmlRaceTaskView
	for _, t := range tasks {
		if RaceTakeSkipReason(s, t, policy, uid, now, gates) != "" {
			continue
		}
		if t.AppearTime > 0 && t.AppearTime > nowMs {
			// Within lead (otherwise skip reason would be non-empty).
			upcoming = append(upcoming, t)
			continue
		}
		ready = append(ready, t)
	}

	sortRaceTasks := func(list []state.FmlRaceTaskView) {
		sort.SliceStable(list, func(i, j int) bool {
			pi := int(raceTaskTypePriority(policy, list[i].TaskType))
			pj := int(raceTaskTypePriority(policy, list[j].TaskType))
			if pi != pj {
				return pi > pj
			}
			return list[i].Score > list[j].Score
		})
	}
	sortRaceTasks(ready)
	sortRaceTasks(upcoming)
	if len(ready) > 0 {
		return ready
	}
	return upcoming
}

// raceTakenAbandonReason returns a non-empty give-up reason when a held
// unfinished task should not be kept. Callers must only invoke this for
// unfinished taken tasks (unknown progress TargetCnt<=0 counts as unfinished).
//
// Order (auto-give-up on):
//  1. Flower-cultivate at the required score (36): never give up once held
//     (manual take, priority 0, module off, min_task_score, missing pool row).
//     Race does not drive cultivate progress — only sync + finishTask.
//  2. Score <= min_task_score (pool score fills Taken.Score==0; still-unresolved
//     Score==0 → do not give up for score alone). Fires even when FinishCnt>0 so
//     a sub-threshold hold is dropped instead of planted to completion.
//  3. Flower-cultivate with known score other than 36 → give up.
//  4. Plant-harvest uncompletable / customer / pearl module off / priority 0
//     (also with progress). Flower-cultivate is never abandoned for a missing
//     plant.cultivate toggle. Flower-art sell/craft never abandons for a
//     missing ordinary sell/craft toggle.
//  5. Pool observed and TaskMsId missing from pool → give up only when FinishCnt==0
//     (mid-progress keep avoids dropping a live task on a transient pool gap)
func raceTakenAbandonReason(s *state.State, policy *pb.UnionRacePolicy, view state.FmlRaceView, gates RaceModuleGates) string {
	taken := view.Taken
	if policy == nil || !policy.GetAutoGiveUpTask() || !taken.HasTask {
		return ""
	}
	score := raceTakenScore(view)
	minScore := policy.GetMinTaskScore()
	taskType := taken.TaskType
	if taskType == 0 {
		taskType = taken.TaskId
	}
	// 36-point flower-cultivate: never give up once held (including manually
	// taken holds and priority 0). Race only takes/finishes; progress is external.
	if taskType == raceTaskTypeFlowerCultivate && score == raceFlowerCultivateRequiredScore {
		return ""
	}
	switch {
	case minScore > 0 && score > 0 && score <= minScore:
		return "公会竞赛放弃不符合分数要求的已接任务"
	case taskType == raceTaskTypeFlowerCultivate && score > 0 && score != raceFlowerCultivateRequiredScore:
		return fmt.Sprintf("公会竞赛放弃非%d分花种培育任务", raceFlowerCultivateRequiredScore)
	case raceTakenUncompletable(s, taken, gates):
		switch taskType {
		case raceTaskTypeCustomerOrder:
			return "公会竞赛放弃无法完成的顾客订单任务"
		case raceTaskTypePearlHire:
			return "公会竞赛放弃无法完成的珍珠雇佣任务"
		case raceTaskTypeFlowerArtCraft:
			return "公会竞赛放弃无法完成的花艺制作任务"
		default:
			return "公会竞赛放弃无法完成的种植收获任务"
		}
	case raceTakenPriorityZero(policy, taken):
		return "公会竞赛放弃优先级为0的已接任务"
	}
	if taken.FinishCnt > 0 {
		return ""
	}
	if view.TasksObserved {
		if _, ok := raceTaskByMsID(view.Tasks, taken.TaskMsId); !ok {
			return "公会竞赛放弃不在任务池中的已接任务"
		}
	}
	return ""
}

// raceTakenScore prefers the held-task score, then the matching pool row when
// field 110 omitted score and enrichment has not filled it yet.
func raceTakenScore(view state.FmlRaceView) int32 {
	if view.Taken.Score > 0 {
		return view.Taken.Score
	}
	if task, ok := raceTaskByMsID(view.Tasks, view.Taken.TaskMsId); ok && task.Score > 0 {
		return task.Score
	}
	return 0
}

// raceTakenBlocksProgress reports whether farm modules must not advance a held
// race task — either it is about to be given up, or its score is still unknown
// while a min_task_score gate is active (planting before score resolves caused
// full-field race plants of sub-threshold tasks). Started tasks (FinishCnt>0)
// are never blocked by the score-unresolved gate alone.
func raceTakenBlocksProgress(s *state.State, policy *pb.UnionRacePolicy, view state.FmlRaceView, gates RaceModuleGates) bool {
	taken := view.Taken
	if policy == nil || !policy.GetAutoGiveUpTask() || !taken.HasTask {
		return false
	}
	if raceTakenAbandonReason(s, policy, view, gates) != "" {
		return true
	}
	if taken.FinishCnt > 0 {
		return false
	}
	return policy.GetMinTaskScore() > 0 && raceTakenScore(view) == 0
}

// raceTakenUncompletable reports whether a held unfinished task can never be
// progressed by automation — plant-harvest with a missing/unplantable target,
// flower-art craft without its required vase, or customer/pearl while the
// ordinary module is off. Flower-art sell is race-driven without a vase target.
// Flower-cultivate is take/finish only and is never treated as uncompletable for
// a missing plant.cultivate toggle.
func raceTakenUncompletable(s *state.State, taken state.FmlRaceTakenView, gates RaceModuleGates) bool {
	taskType := taken.TaskType
	if taskType == 0 {
		taskType = taken.TaskId
	}
	switch taskType {
	case raceTaskTypePlantHarvest:
		return taken.ParamID <= 0 || !flowerCultivated(s, taken.ParamID)
	case raceTaskTypeFlowerArtCraft:
		return raceFlowerArtCraftVaseUnavailable(s, taken.ParamID)
	case raceTaskTypeCustomerOrder:
		return !gates.Customer
	case raceTaskTypePearlHire:
		return !gates.Pearl
	default:
		return false
	}
}

func raceFlowerArtCraftVaseUnavailable(s *state.State, vaseID int32) bool {
	if vaseID <= 0 || s == nil || !s.VaseObserved() {
		return true
	}
	return !s.HasVase(vaseID)
}

// raceTakenPriorityZero reports whether a held task's type is configured at
// priority 0 (do not take / should give up).
func raceTakenPriorityZero(policy *pb.UnionRacePolicy, taken state.FmlRaceTakenView) bool {
	taskType := taken.TaskType
	if taskType == 0 {
		taskType = taken.TaskId
	}
	return raceTaskTypePriority(policy, taskType) <= 0
}
