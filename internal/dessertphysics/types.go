package dessertphysics

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"
)

var (
	// ErrAmbiguousMerge indicates that two otherwise valid contacts in one
	// physics frame share a body. The adapter locks after this condition rather
	// than guessing which client callback would win.
	ErrAmbiguousMerge = errors.New("ambiguous dessert merge contacts share a body")
	// ErrNonFiniteState indicates that Box2D produced a value unsafe to return
	// to a gameplay planner.
	ErrNonFiniteState = errors.New("dessert physics produced non-finite state")
	// ErrWorldLocked is returned after a fail-closed simulation error.
	ErrWorldLocked = errors.New("dessert physics world is fail-closed")
)

// BodyState is the authoritative input/output representation of one dessert.
// PositionPX and RadiusPX are client pixels. LinearVelocityMPS is Box2D metres
// per second. AngleRad and AngularVelocityRadPerS use radians.
//
// ScaleX and ScaleY are kept separately because the wire object contains both;
// NewWorld requires them to be equal. ScaleZ is intentionally absent because
// it does not affect the circle collider. RadiusPX may be zero on input, in
// which case it is derived from level and ScaleX.
type BodyState struct {
	ID                     uint64
	Level                  int
	PositionPX             Vec2
	RadiusPX               float64
	LinearVelocityMPS      Vec2
	AngleRad               float64
	AngularVelocityRadPerS float64
	ScaleX                 float64
	ScaleY                 float64
	Awake                  bool
	IsSyn                  bool
	IsFallBall             bool
	DangerLineTimeMS       float64
	GrowthRemaining        time.Duration
}

// Merge identifies a same-level contact that can be applied. IDs are always
// sorted, making contact selection stable across Box2D iteration order.
type Merge struct {
	BodyAID uint64
	BodyBID uint64
	// Level is the two contacted bodies' source level. The wire mergeLvl and
	// resulting dessert level are Level+1.
	Level    int
	Revision uint64
}

// StepResult describes one fixed 1/60-second physics step.
type StepResult struct {
	Revision           uint64
	Elapsed            time.Duration
	MergeCandidates    []Merge
	Stable             bool
	TouchingDangerLine bool
	TerminalDanger     bool
}

// AdvanceReason explains why AdvanceUntil stopped.
type AdvanceReason uint8

const (
	AdvanceStepLimit AdvanceReason = iota
	AdvanceMerge
	AdvanceStable
	AdvanceDanger
)

// AdvanceResult is the bounded result of AdvanceUntil.
type AdvanceResult struct {
	Reason AdvanceReason
	Steps  int
	Last   StepResult
}

// Snapshot is a deterministic copy of the current simulated world. Bodies are
// ordered by ID and retain the same units as BodyState.
type Snapshot struct {
	Revision           uint64
	Elapsed            time.Duration
	Bodies             []BodyState
	Stable             bool
	TouchingDangerLine bool
	TerminalDanger     bool
	Failure            string
}

func normalizeBodyState(config Config, state BodyState) (BodyState, error) {
	if state.ID == 0 {
		return BodyState{}, errors.New("dessert body ID must be positive")
	}
	if state.Level < 1 || state.Level > levelCount {
		return BodyState{}, fmt.Errorf("dessert body %d has invalid level %d", state.ID, state.Level)
	}
	values := []float64{
		state.PositionPX.X, state.PositionPX.Y,
		state.RadiusPX,
		state.LinearVelocityMPS.X, state.LinearVelocityMPS.Y,
		state.AngleRad, state.AngularVelocityRadPerS,
		state.ScaleX, state.ScaleY, state.DangerLineTimeMS,
	}
	for _, value := range values {
		if !isFinite(value) {
			return BodyState{}, fmt.Errorf("dessert body %d contains a non-finite value", state.ID)
		}
	}
	if state.ScaleX < config.MergeStartScale-floatEpsilon || state.ScaleX > 1+floatEpsilon ||
		state.ScaleY < config.MergeStartScale-floatEpsilon || state.ScaleY > 1+floatEpsilon ||
		!sameFloat(state.ScaleX, state.ScaleY) {
		return BodyState{}, fmt.Errorf("dessert body %d has invalid collider scale (%g,%g)", state.ID, state.ScaleX, state.ScaleY)
	}
	if math.Abs(state.ScaleX-config.MergeStartScale) <= floatEpsilon {
		state.ScaleX = config.MergeStartScale
		state.ScaleY = config.MergeStartScale
	} else if math.Abs(state.ScaleX-1) <= floatEpsilon {
		state.ScaleX = 1
		state.ScaleY = 1
	}
	if state.DangerLineTimeMS < 0 || state.GrowthRemaining < 0 || state.GrowthRemaining > config.MergeGrowthDuration {
		return BodyState{}, fmt.Errorf("dessert body %d has invalid temporal state", state.ID)
	}
	expectedRadius := config.RadiiPX[state.Level-1] * state.ScaleX
	if state.RadiusPX == 0 {
		state.RadiusPX = expectedRadius
	} else if !sameFloat(state.RadiusPX, expectedRadius) {
		return BodyState{}, fmt.Errorf("dessert body %d radius %g does not match level/scale radius %g", state.ID, state.RadiusPX, expectedRadius)
	}
	if state.ScaleX < 1-floatEpsilon {
		expectedRemaining := time.Duration((1 - state.ScaleX) / (1 - config.MergeStartScale) * float64(config.MergeGrowthDuration))
		if state.GrowthRemaining == 0 {
			state.GrowthRemaining = expectedRemaining
		} else if state.GrowthRemaining != expectedRemaining {
			return BodyState{}, fmt.Errorf("dessert body %d growth state does not match collider scale", state.ID)
		}
	} else {
		state.ScaleX = 1
		state.ScaleY = 1
		state.RadiusPX = config.RadiiPX[state.Level-1]
		state.GrowthRemaining = 0
	}
	return state, nil
}

func sortMerges(merges []Merge) {
	for index := range merges {
		if merges[index].BodyAID > merges[index].BodyBID {
			merges[index].BodyAID, merges[index].BodyBID = merges[index].BodyBID, merges[index].BodyAID
		}
	}
	sort.Slice(merges, func(left, right int) bool {
		if merges[left].BodyAID != merges[right].BodyAID {
			return merges[left].BodyAID < merges[right].BodyAID
		}
		return merges[left].BodyBID < merges[right].BodyBID
	})
}
