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
	consider(r.soonestRaceOpCooldownUntil(now))
	if automation.RaceBootstrapDue(st, policy, now) {
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
	if snapshot.sessionInvalidated || snapshot.client == nil || snapshot.session == nil {
		r.resetSideLaneFairness()
		return
	}

	now := time.Now()
	if snapshot.policy != nil && snapshot.policy.GetAutomationEnabled() &&
		automation.RaceBootstrapDue(r.state, snapshot.policy, now) {
		if op := r.nextRunnableOperation(snapshot.policy, now); op != nil && automation.IsUrgentRaceOp(*op) {
			r.runOperationTick(ctx, snapshot.client, snapshot.session, op, now)
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
	r.refreshDessertShadowRuntime(now)
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

	op := r.nextRunnableOperation(snapshot.policy, now)
	if op == nil {
		return
	}
	r.runOperationTick(ctx, snapshot.client, snapshot.session, op, now)
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
	r.operationMu.Lock()
	defer r.operationMu.Unlock()

	var opErr error
	finishOperation := r.beginOperation(op.Kind)
	defer func() { finishOperation(opErr) }()

	if err := r.checkOperationResources(op, now); err != nil {
		opErr = err
		r.handleResourceGateFailure(ctx, op, err)
		return
	}

	if err := r.ensurePlannedOperationRqst(ctx, op); err != nil {
		opErr = fmt.Errorf("rqst: %w", err)
		r.handleRqstFailure(ctx, op, err, opErr)
		return
	}

	releaseWaterLock, err := r.lockOperationWaterDrops(op, now)
	if err != nil {
		opErr = err
		return
	}
	defer releaseWaterLock()

	args, err := operationArgs(op)
	if err != nil {
		opErr = err
		r.handleOperationArgsFailure(ctx, op, err)
		return
	}

	attempt := operationAttempt{op: op, args: args, startedAt: time.Now()}
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
		return
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
		return
	}
	r.handleOperationSuccess(ctx, result)
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
