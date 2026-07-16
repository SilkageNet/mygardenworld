package dessertphysics

import (
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/ByteArena/box2d"
)

type contactPair struct {
	left  uint64
	right uint64
}

type contactListener struct {
	world       *World
	beginPairs  []contactPair
	fallBodyIDs []uint64
}

func (listener *contactListener) BeginContact(contact box2d.B2ContactInterface) {
	if listener == nil || listener.world == nil || contact == nil {
		return
	}
	recordA, _ := contact.GetFixtureA().GetBody().GetUserData().(*bodyRecord)
	recordB, _ := contact.GetFixtureB().GetBody().GetUserData().(*bodyRecord)
	boundaryA, _ := contact.GetFixtureA().GetBody().GetUserData().(boundaryKind)
	boundaryB, _ := contact.GetFixtureB().GetBody().GetUserData().(boundaryKind)
	if recordA != nil && (recordB != nil || boundaryB == boundaryFloor) {
		listener.fallBodyIDs = append(listener.fallBodyIDs, recordA.state.ID)
	}
	if recordB != nil && (recordA != nil || boundaryA == boundaryFloor) {
		listener.fallBodyIDs = append(listener.fallBodyIDs, recordB.state.ID)
	}
	if recordA == nil || recordB == nil {
		return
	}
	left, right := recordA.state.ID, recordB.state.ID
	if left > right {
		left, right = right, left
	}
	listener.beginPairs = append(listener.beginPairs, contactPair{left: left, right: right})
}

func (*contactListener) EndContact(box2d.B2ContactInterface) {}

func (*contactListener) PreSolve(box2d.B2ContactInterface, box2d.B2Manifold) {}

func (*contactListener) PostSolve(box2d.B2ContactInterface, *box2d.B2ContactImpulse) {}

// Step advances exactly one observed client frame (1/60 second). It reports
// merge contacts but never applies them optimistically.
func (w *World) Step() (StepResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	box2dRuntimeMu.Lock()
	defer box2dRuntimeMu.Unlock()
	if err := w.usableLocked(); err != nil {
		return StepResult{}, err
	}
	w.latestMerges = nil

	w.advanceGrowthLocked()
	w.listener.beginPairs = w.listener.beginPairs[:0]
	w.listener.fallBodyIDs = w.listener.fallBodyIDs[:0]
	w.physics.Step(w.config.StepSeconds, w.config.VelocityIterations, w.config.PositionIterations)
	w.elapsedSeconds += w.config.StepSeconds
	w.revision++
	if err := w.validateFiniteLocked(); err != nil {
		w.lockFailureLocked(err)
		return StepResult{Revision: w.revision, Elapsed: w.elapsedLocked()}, err
	}

	merges, err := w.contactsLocked()
	if err != nil {
		w.lockFailureLocked(err)
		return StepResult{Revision: w.revision, Elapsed: w.elapsedLocked()}, err
	}
	touchingDanger, terminalDanger := w.updateDangerLocked()
	result := StepResult{
		Revision:           w.revision,
		Elapsed:            w.elapsedLocked(),
		MergeCandidates:    merges,
		Stable:             len(merges) == 0 && w.stableLocked(),
		TouchingDangerLine: touchingDanger,
		TerminalDanger:     terminalDanger,
	}
	if len(merges) > 0 {
		w.latestMerges = make(map[contactPair]Merge, len(merges))
		for _, merge := range merges {
			w.latestMerges[contactPair{left: merge.BodyAID, right: merge.BodyBID}] = merge
		}
	}
	return result, nil
}

// AdvanceUntil runs a bounded number of fixed steps and stops on the first
// merge, dangerous-line contact, or stable board. The limit is mandatory so a
// malformed or chaotic board can never consume an unbounded tick.
func (w *World) AdvanceUntil(maxSteps int) (AdvanceResult, error) {
	if maxSteps <= 0 {
		return AdvanceResult{}, errors.New("dessert physics advance limit must be positive")
	}
	result := AdvanceResult{Reason: AdvanceStepLimit}
	for step := 1; step <= maxSteps; step++ {
		current, err := w.Step()
		result.Steps = step
		result.Last = current
		if err != nil {
			return result, err
		}
		switch {
		case len(current.MergeCandidates) > 0:
			result.Reason = AdvanceMerge
			return result, nil
		case current.TouchingDangerLine:
			result.Reason = AdvanceDanger
			return result, nil
		case current.Stable:
			result.Reason = AdvanceStable
			return result, nil
		}
	}
	return result, nil
}

func (w *World) advanceGrowthLocked() {
	stepDuration := time.Duration(w.config.StepSeconds * float64(time.Second))
	for _, record := range w.sortedRecordsLocked() {
		if record.state.GrowthRemaining <= 0 {
			continue
		}
		remaining := record.state.GrowthRemaining - stepDuration
		if remaining < 0 {
			remaining = 0
		}
		fractionRemaining := float64(remaining) / float64(w.config.MergeGrowthDuration)
		scale := 1 - (1-w.config.MergeStartScale)*fractionRemaining
		if remaining == 0 {
			scale = 1
		}
		radiusPX := w.config.RadiiPX[record.state.Level-1] * scale

		velocity := record.body.GetLinearVelocity()
		angularVelocity := record.body.GetAngularVelocity()
		wasAwake := record.body.IsAwake()
		record.body.DestroyFixture(record.fixture)
		record.fixture = w.createCircleFixtureLocked(record.body, radiusPX)
		record.body.SetLinearVelocity(velocity)
		record.body.SetAngularVelocity(angularVelocity)
		if !wasAwake {
			record.body.SetAwake(false)
		}

		record.state.ScaleX = scale
		record.state.ScaleY = scale
		record.state.RadiusPX = radiusPX
		record.state.GrowthRemaining = remaining
	}
}

func (w *World) contactsLocked() ([]Merge, error) {
	// isFallBall is cleared by another dessert or the floor. The left/right
	// walls deliberately do not clear it.
	for _, id := range w.listener.fallBodyIDs {
		if record := w.bodies[id]; record != nil {
			record.state.IsFallBall = false
		}
	}
	for contact := w.physics.GetContactList(); contact != nil; contact = contact.GetNext() {
		if !contact.IsEnabled() || !contact.IsTouching() {
			continue
		}
		bodyA := contact.GetFixtureA().GetBody()
		bodyB := contact.GetFixtureB().GetBody()
		recordA, _ := bodyA.GetUserData().(*bodyRecord)
		recordB, _ := bodyB.GetUserData().(*bodyRecord)
		boundaryA, _ := bodyA.GetUserData().(boundaryKind)
		boundaryB, _ := bodyB.GetUserData().(boundaryKind)
		if recordA != nil && (recordB != nil || boundaryB == boundaryFloor) {
			recordA.state.IsFallBall = false
		}
		if recordB != nil && (recordA != nil || boundaryA == boundaryFloor) {
			recordB.state.IsFallBall = false
		}
	}

	// Merge eligibility is based exclusively on BeginContact callbacks. Looking
	// at only the final contact list can miss a short-lived contact in this frame,
	// while treating persistent contacts as new would repeatedly re-plan them.
	pairs := make(map[contactPair]Merge)
	for _, pair := range w.listener.beginPairs {
		recordA := w.bodies[pair.left]
		recordB := w.bodies[pair.right]
		if recordA == nil || recordB == nil || recordA.state.IsSyn || recordB.state.IsSyn ||
			recordA.state.Level != recordB.state.Level || recordA.state.Level >= levelCount {
			continue
		}
		pairs[pair] = Merge{BodyAID: pair.left, BodyBID: pair.right, Level: recordA.state.Level, Revision: w.revision}
	}
	merges := make([]Merge, 0, len(pairs))
	for _, merge := range pairs {
		merges = append(merges, merge)
	}
	sortMerges(merges)
	used := make(map[uint64]struct{}, len(merges)*2)
	for _, merge := range merges {
		if _, exists := used[merge.BodyAID]; exists {
			return nil, fmt.Errorf("%w: body %d", ErrAmbiguousMerge, merge.BodyAID)
		}
		if _, exists := used[merge.BodyBID]; exists {
			return nil, fmt.Errorf("%w: body %d", ErrAmbiguousMerge, merge.BodyBID)
		}
		used[merge.BodyAID] = struct{}{}
		used[merge.BodyBID] = struct{}{}
	}
	return merges, nil
}

func (w *World) updateDangerLocked() (touching bool, terminal bool) {
	w.dangerAccumulatorSeconds += w.config.StepSeconds
	const dangerTickSeconds = 0.25
	terminalMS := float64(w.config.DangerTerminalDuration) / float64(time.Millisecond)
	for _, record := range w.sortedRecordsLocked() {
		state := w.bodyStateLocked(record)
		if w.touchesDangerLocked(state) {
			touching = true
		}
	}
	for w.dangerAccumulatorSeconds+floatEpsilon >= dangerTickSeconds {
		w.dangerAccumulatorSeconds -= dangerTickSeconds
		if w.dangerAccumulatorSeconds < 0 && w.dangerAccumulatorSeconds > -floatEpsilon {
			w.dangerAccumulatorSeconds = 0
		}
		for _, record := range w.sortedRecordsLocked() {
			velocity := record.body.GetLinearVelocity()
			angularVelocity := record.body.GetAngularVelocity()
			if velocity.X != 0 && math.Abs(velocity.X) < 0.1 && angularVelocity != 0 && math.Abs(angularVelocity) < 0.1 {
				record.body.SetLinearVelocity(box2d.MakeB2Vec2(0, velocity.Y))
				record.body.SetAngularVelocity(0)
			}
			state := w.bodyStateLocked(record)
			if w.touchesDangerLocked(state) {
				if record.state.DangerLineTimeMS > 0 {
					record.state.DangerLineTimeMS += dangerTickSeconds * 1000
				} else {
					record.state.DangerLineTimeMS = 1
				}
			} else {
				record.state.DangerLineTimeMS = 0
			}
		}
	}
	for _, record := range w.sortedRecordsLocked() {
		if record.state.DangerLineTimeMS >= terminalMS {
			terminal = true
			break
		}
	}
	return touching, terminal
}

func (w *World) validateFiniteLocked() error {
	for _, record := range w.sortedRecordsLocked() {
		position := record.body.GetPosition()
		velocity := record.body.GetLinearVelocity()
		values := []float64{
			position.X, position.Y, velocity.X, velocity.Y,
			record.body.GetAngle(), record.body.GetAngularVelocity(),
			record.state.ScaleX, record.state.ScaleY, record.state.RadiusPX,
		}
		for _, value := range values {
			if !isFinite(value) {
				return fmt.Errorf("%w for body %d", ErrNonFiniteState, record.state.ID)
			}
		}
	}
	return nil
}

func (w *World) elapsedLocked() time.Duration {
	return time.Duration(w.elapsedSeconds * float64(time.Second))
}
