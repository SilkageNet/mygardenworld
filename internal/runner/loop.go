package runner

import (
	"context"
	"fmt"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
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
	consider(r.soonestRaceTakeCooldownUntil(now))
	if soonest < minDecisionWake {
		return minDecisionWake
	}
	return soonest
}

func (r *Runner) soonestRaceTakeCooldownUntil(now time.Time) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	var soonest time.Time
	for _, cd := range r.operationCooldowns {
		if cd.Domain != "union.race.take" || !cd.Until.After(now) {
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
	r.emitOperationPlanned(attempt)

	raw, err := r.executePlannedOp(ctx, client, session, op)
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
