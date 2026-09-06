package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientrpc"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	harvestRetryWait      = 30 * time.Second
	harvestRPCInterval    = 120 * time.Millisecond
	waterSourceSyncPeriod = 60 * time.Second
)

const minDecisionWake = 5 * time.Millisecond

func (r *Runner) decisionLoop(ctx context.Context) {
	for {
		interval := r.nextTickInterval(time.Now())
		r.setNextDecisionAt(time.Now().Add(interval))
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			r.tick(ctx)
		}
	}
}

func (r *Runner) tickInterval() time.Duration {
	r.mu.RLock()
	p := r.policy
	r.mu.RUnlock()
	d := time.Duration(0)
	if p != nil {
		d = time.Duration(p.GetDecisionIntervalSeconds() * float64(time.Second))
	}
	if d <= 0 {
		d = 4 * time.Second
	}
	return d
}

// nextTickInterval shortens the default decision interval so a filter-passing
// race CD task is planned at AppearTime-lead, and a take that hit server CD
// retries at AppearTime rather than waiting out the next 4s tick.
func (r *Runner) nextTickInterval(now time.Time) time.Duration {
	interval := r.tickInterval()
	if r.restrictionError() != nil {
		return max(interval, time.Second)
	}
	soonest := interval
	consider := func(at time.Time) {
		if at.IsZero() {
			return
		}
		d := at.Sub(now)
		if d > 0 && d < soonest {
			soonest = d
		}
	}
	r.mu.RLock()
	policy := r.policy
	st := r.state
	r.mu.RUnlock()
	consider(automation.RaceTakeWakeAt(st, policy, now))
	raceCooldownUntil := r.soonestRaceOpCooldownUntil(now)
	consider(raceCooldownUntil)
	// A failed bootstrap op carries a short runner cooldown. Respect it here;
	// otherwise RaceBootstrapDue would force a 5ms loop until the cooldown
	// expires even though selectRunnableOperation cannot run the race op.
	if automation.RaceBootstrapDue(st, policy, now) && raceCooldownUntil.IsZero() {
		return minDecisionWake
	}
	if soonest < minDecisionWake {
		return minDecisionWake
	}
	return soonest
}

func (r *Runner) soonestRaceOpCooldownUntil(now time.Time) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	var soonest time.Time
	for _, cd := range r.operationCooldowns {
		if !cd.Until.After(now) {
			continue
		}
		switch cd.Domain {
		case "union.race.take", "union.race.sync", "union.race.enter":
		default:
			continue
		}
		if soonest.IsZero() || cd.Until.Before(soonest) {
			soonest = cd.Until
		}
	}
	return soonest
}

func (r *Runner) tick(ctx context.Context) {
	snapshot := r.readTickSnapshot()
	if r.restrictionError() == nil {
		r.emitPearlHireDiagnostic(snapshot, time.Now())
	}
	if snapshot.sessionInvalidated || snapshot.client == nil || snapshot.session == nil {
		r.resetSideLaneFairness()
		return
	}
	if r.recoverAccountRestriction(snapshot.client, time.Now()) {
		return
	}

	now := time.Now()
	var selected *automation.PlannedOp
	if snapshot.policy != nil && snapshot.policy.GetAutomationEnabled() &&
		automation.RaceBootstrapDue(r.state, snapshot.policy, now) {
		selected = r.nextRunnableOperation(snapshot.policy, now)
		if selected != nil && automation.IsUrgentRaceOp(*selected) {
			r.runOperationTick(ctx, snapshot.client, snapshot.session, selected, now)
			return
		}
	}

	r.state.RefreshWaterDrops(now)
	r.tickWaterSourceSync(ctx, snapshot.client, snapshot.session)
	if r.isSessionInvalidated() {
		return
	}
	if err := r.enforceReputationGuard(ctx, snapshot.client, snapshot.session, "tick", now); err != nil {
		if isReputationGuardError(err) {
			r.Stop()
		}
		return
	}
	if snapshot.policy == nil || !snapshot.policy.AutomationEnabled {
		r.resetSideLaneFairness()
		return
	}
	r.tickResidentOrderSync(ctx, snapshot.client, snapshot.session, snapshot.policy)
	if r.isSessionInvalidated() {
		return
	}

	r.emitCustomerOrderInfo()
	r.emitResidentOrderLimitInfo(snapshot.policy, now)

	// Reuse a non-urgent operation selected during the bootstrap check. Calling
	// the stateful fairness selector twice in one tick can consume a required
	// Farm turn and then immediately select the same urgent sync it was meant to
	// yield, recreating starvation despite the scheduler safety net.
	if selected == nil {
		selected = r.nextRunnableOperation(snapshot.policy, now)
	}
	if selected == nil {
		return
	}
	r.runOperationTick(ctx, snapshot.client, snapshot.session, selected, now)
}

type tickSnapshot struct {
	policy             *pb.Policy
	client             *babigame.Client
	session            *babigame.Session
	sessionInvalidated bool
}

func (r *Runner) readTickSnapshot() tickSnapshot {
	r.mu.RLock()
	snapshot := tickSnapshot{
		policy:             r.policy,
		client:             r.client,
		session:            r.session,
		sessionInvalidated: r.sessionInvalidated,
	}
	r.mu.RUnlock()
	return snapshot
}

func (r *Runner) runOperationTick(ctx context.Context, client *babigame.Client, session *babigame.Session, op *automation.PlannedOp, now time.Time) {
	_ = r.executeOperation(ctx, client, session, op, now)
}

// executeOperation runs one operation through the same serialization,
// resource gates, state reconciliation, logging, and diagnostics used by the
// scheduler. It returns the original request error to explicit callers while
// keeping handled/transient errors out of runtime failure diagnostics.
func (r *Runner) executeOperation(ctx context.Context, client *babigame.Client, session *babigame.Session, op *automation.PlannedOp, now time.Time) error {
	r.operationMu.Lock()
	defer r.operationMu.Unlock()

	var opErr error
	finishOperation := r.beginOperation(op.Kind)
	defer func() { finishOperation(opErr) }()
	if err := r.restrictionError(); err != nil {
		opErr = err
		return err
	}

	if err := r.checkOperationResources(op, now); err != nil {
		opErr = err
		r.handleResourceGateFailure(ctx, op, err)
		return err
	}

	// Reserve before any delete preflight or verification RPC. Failures keep
	// the reservation, and a manual request uses this same serialized gate.
	if err := r.reserveRaceDelete(ctx, op, time.Now()); err != nil {
		opErr = err
		return err
	}
	if err := r.ensurePlannedOperationRqst(ctx, op); err != nil {
		opErr = fmt.Errorf("rqst: %w", err)
		r.handleRqstFailure(ctx, op, err, opErr)
		return opErr
	}

	if op.Kind == clientproto.RPCFmlRaceTakeTask.String() || op.Kind == clientproto.RPCFmlRaceDelTask.String() {
		rawRPC := babigame.NewRPCClient(
			client,
			session,
			babigame.WithDefaultTimeout(30*time.Second),
			babigame.WithApplyV(r.state.ApplyV),
		)
		err := preflightFmlRaceTaskMutation(ctx, operationRuntime{runner: r, rpc: clientrpc.NewClient(rawRPC)}, op)
		if err != nil {
			opErr = err
			// Preflight failures return before ordinary RPC error handling. Give
			// the rejected task its own retry delay so an unchanged snapshot or
			// failed refresh cannot monopolize the next decision.
			op = r.cooldownSideOperation(op, time.Now(), err, "竞赛任务执行前校验未通过", 5*time.Second)
			r.emit(Event{
				Kind:        "operation_deferred",
				Category:    op.Category,
				Domain:      op.Domain,
				Action:      "blocked",
				Label:       operationEventLabel(op),
				Message:     fmt.Sprintf("%s 已跳过: %v", opDesc(op), err),
				PayloadJSON: operationPayload(op, nil, nil, err),
				Level:       "warn",
			})
			r.logOperation(ctx, op.Kind, nil, map[string]any{"error": err.Error(), "stage": "race_preflight"})
			return err
		}
	}

	releaseWaterLock, err := r.lockOperationWaterDrops(op, now)
	if err != nil {
		opErr = err
		return err
	}
	defer releaseWaterLock()

	args, err := operationArgs(op)
	if err != nil {
		opErr = err
		r.handleOperationArgsFailure(ctx, op, err)
		return err
	}

	attempt := operationAttempt{op: op, args: args, startedAt: time.Now()}
	if op.Kind == clientproto.RPCFrdStealSteal.String() || op.Kind == clientproto.RPCFrdExtBuyStealCnt.String() {
		used, bought, usedObserved, boughtObserved := r.state.FriendStealCounters(op.TargetUID, attempt.startedAt)
		attempt.friendStealUsedBefore = used
		attempt.friendStealUsedBeforeSet = usedObserved
		attempt.friendStealBoughtBefore = bought
		attempt.friendStealBoughtBeforeSet = boughtObserved
	}
	if op.Kind == clientproto.RPCFlowerRackRecvSellMoney.String() {
		attempt.goldBefore = r.state.Gold()
	}
	if op.Kind == clientproto.RPCCultivateUpgrade.String() && op.FlowerID > 0 {
		if cv, ok := r.state.Cultivations()[op.FlowerID]; ok {
			attempt.levelBefore = cv.Lvl
		}
	}
	if op.Kind == clientproto.RPCWaterwheelRecv.String() || op.Kind == clientproto.RPCFreeWaterRecv.String() {
		waterDrops, _, _ := r.state.WaterDrops()
		attempt.waterDropsBefore = waterDrops
	}
	if op.Kind == clientproto.RPCActCyclicStoryRecvOrderRwd.String() {
		if view, ok := r.state.CyclicStoryView(attempt.startedAt); ok && view.Valid {
			attempt.scoreBefore = view.Score
			attempt.scoreBeforeSet = true
		}
	}
	if op.Kind == clientproto.RPCFmlRaceTakeTask.String() && !r.state.FmlRace().TasksObserved {
		r.emit(Event{
			Kind:        "operation_deferred",
			Category:    op.Category,
			Domain:      op.Domain,
			Action:      "blocked",
			Label:       operationEventLabel(op),
			Message:     fmt.Sprintf("%s 已跳过: 任务池尚未同步", opDesc(op)),
			PayloadJSON: operationPayload(op, args, nil, nil),
			Level:       "warn",
		})
		return fmt.Errorf("竞赛任务池尚未同步")
	}
	r.emitOperationPlanned(attempt)

	raw, err := r.executePlannedOp(ctx, client, session, op)
	if isRaceTakeOnCooldownError(op.Kind, err) {
		raw, err = r.retryRaceTakeUntilAppear(ctx, client, session, op)
	}
	result := operationResult{
		operationAttempt: attempt,
		raw:              raw,
		err:              err,
		finishedAt:       time.Now(),
	}
	if result.err != nil {
		opErr = r.handleOperationError(ctx, result)
		return result.err
	}
	r.handleOperationSuccess(ctx, result)
	return nil
}

// TakeUnionRaceTask validates and immediately executes a user-selected race
// task. It intentionally reuses the scheduler's operation pipeline, so a
// manual click cannot race another mutation or bypass resource/state handling.
func (r *Runner) TakeUnionRaceTask(ctx context.Context, taskMsID int64) error {
	snapshot := r.readTickSnapshot()
	if snapshot.sessionInvalidated || snapshot.client == nil || snapshot.session == nil {
		return fmt.Errorf("账号当前未连接游戏服务")
	}
	op, err := automation.ManualRaceTakeOperation(r.state, snapshot.policy, taskMsID, time.Now())
	if err != nil {
		return err
	}
	return r.executeOperation(ctx, snapshot.client, snapshot.session, &op, time.Now())
}

// DeleteUnionRaceTask validates and immediately executes a user-selected race
// task deletion through the runner's serialized mutation pipeline.
func (r *Runner) DeleteUnionRaceTask(ctx context.Context, taskMsID int64) error {
	snapshot := r.readTickSnapshot()
	if snapshot.sessionInvalidated || snapshot.client == nil || snapshot.session == nil {
		return fmt.Errorf("账号当前未连接游戏服务")
	}
	op, err := automation.ManualRaceDeleteOperation(r.state, snapshot.policy, taskMsID, time.Now())
	if err != nil {
		return err
	}
	return r.executeOperation(ctx, snapshot.client, snapshot.session, &op, time.Now())
}

const (
	raceTakeCDRetryPad = 80 * time.Millisecond
	raceTakeCDRetryGap = 10 * time.Millisecond
	raceTakeCDRetryMax = 8
)

// retryRaceTakeUntilAppear re-sends takeTask at AppearTime after a preemptive
// lead-window CD rejection, instead of waiting for the next 4s decision tick.
func (r *Runner) retryRaceTakeUntilAppear(ctx context.Context, client *babigame.Client, session *babigame.Session, op *automation.PlannedOp) (json.RawMessage, error) {
	appear := raceTakeAppearTime(r.state, op)
	deadline := time.Now().Add(raceTakeCDRetryPad)
	if !appear.IsZero() {
		deadline = appear.Add(raceTakeCDRetryPad)
	}
	var raw json.RawMessage
	var err error
	for i := 0; i < raceTakeCDRetryMax; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		now := time.Now()
		if sleep := raceTakeRetrySleep(now, appear); sleep > 0 {
			timer := time.NewTimer(sleep)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		raw, err = r.executePlannedOp(ctx, client, session, op)
		if err == nil || !isRaceTakeOnCooldownError(op.Kind, err) {
			return raw, err
		}
		if !time.Now().Before(deadline) {
			break
		}
	}
	return raw, err
}

func raceTakeAppearTime(st *state.State, op *automation.PlannedOp) time.Time {
	if st == nil || op == nil || op.TaskMsID == 0 {
		return time.Time{}
	}
	for _, t := range st.FmlRace().Tasks {
		if t.MsId == op.TaskMsID && t.AppearTime > 0 {
			return time.UnixMilli(t.AppearTime)
		}
	}
	return time.Time{}
}

func raceTakeRetrySleep(now, appear time.Time) time.Duration {
	if appear.IsZero() {
		return raceTakeCDRetryGap
	}
	if now.Before(appear) {
		return appear.Sub(now)
	}
	return raceTakeCDRetryGap
}
