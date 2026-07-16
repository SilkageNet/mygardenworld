package dessertphysics

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/ByteArena/box2d"
)

type boundaryKind uint8

const (
	boundaryLeft boundaryKind = iota + 1
	boundaryRight
	boundaryFloor
)

type bodyRecord struct {
	state   BodyState
	body    *box2d.B2Body
	fixture *box2d.B2Fixture
}

// ByteArena/box2d keeps contact factories and time-of-impact counters in
// package-level mutable variables. Different B2World values therefore cannot
// safely enter mutating Box2D calls concurrently, even though their bodies are
// otherwise independent. Keep this lock package-wide so multiple accounts and
// candidate simulations cannot race inside the dependency.
var box2dRuntimeMu sync.Mutex

// World owns a Box2D world and its authoritative dessert metadata. A World is
// safe for concurrent inspection, although callers should still serialize
// gameplay decisions so every authoritative response can re-baseline it.
type World struct {
	mu sync.Mutex

	config       Config
	physics      box2d.B2World
	bodies       map[uint64]*bodyRecord
	nextID       uint64
	listener     contactListener
	latestMerges map[contactPair]Merge

	revision                 uint64
	elapsedSeconds           float64
	dangerAccumulatorSeconds float64
	failure                  error
}

// NewWorld reconstructs a physics world from authoritative client-wire state.
func NewWorld(config Config, states []BodyState) (*World, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	box2dRuntimeMu.Lock()
	defer box2dRuntimeMu.Unlock()
	world := &World{
		config:  config,
		physics: box2d.MakeB2World(box2d.MakeB2Vec2(0, config.GravityMPS2)),
		bodies:  make(map[uint64]*bodyRecord, len(states)),
		nextID:  1,
	}
	world.listener.world = world
	world.physics.SetContactListener(&world.listener)
	world.createBoundariesLocked()
	normalized := make([]BodyState, 0, len(states))
	for _, input := range states {
		state, err := normalizeBodyState(config, input)
		if err != nil {
			return nil, err
		}
		normalized = append(normalized, state)
	}
	sort.Slice(normalized, func(left, right int) bool {
		return normalized[left].ID < normalized[right].ID
	})
	for index, state := range normalized {
		if index > 0 && normalized[index-1].ID == state.ID {
			return nil, fmt.Errorf("duplicate dessert body ID %d", state.ID)
		}
		if err := world.addBodyLocked(state); err != nil {
			return nil, err
		}
	}
	return world, nil
}

// Config returns the exact observed client configuration used by this World.
func (w *World) Config() Config {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.config
}

func (w *World) createBoundariesLocked() {
	// The client walls are taller than the playable area. Extending the finite
	// Box2D edges well beyond the danger line reproduces their effective extent
	// without introducing a second gameplay boundary.
	topPX := w.config.DangerLinePX + 2048
	w.createBoundaryLocked(boundaryLeft, Vec2{X: w.config.LeftWallPX, Y: w.config.FloorPX}, Vec2{X: w.config.LeftWallPX, Y: topPX})
	w.createBoundaryLocked(boundaryRight, Vec2{X: w.config.RightWallPX, Y: topPX}, Vec2{X: w.config.RightWallPX, Y: w.config.FloorPX})
	w.createBoundaryLocked(boundaryFloor, Vec2{X: w.config.RightWallPX, Y: w.config.FloorPX}, Vec2{X: w.config.LeftWallPX, Y: w.config.FloorPX})
}

func (w *World) createBoundaryLocked(kind boundaryKind, startPX, endPX Vec2) {
	bodyDef := box2d.MakeB2BodyDef()
	bodyDef.UserData = kind
	body := w.physics.CreateBody(&bodyDef)
	shape := box2d.MakeB2EdgeShape()
	shape.Set(w.toMeters(startPX), w.toMeters(endPX))
	fixtureDef := box2d.MakeB2FixtureDef()
	fixtureDef.Shape = &shape
	fixtureDef.Friction = w.config.Friction
	fixtureDef.Restitution = w.config.Restitution
	fixtureDef.Filter = box2d.MakeB2Filter()
	body.CreateFixtureFromDef(&fixtureDef)
}

func (w *World) addBodyLocked(state BodyState) error {
	if _, exists := w.bodies[state.ID]; exists {
		return fmt.Errorf("duplicate dessert body ID %d", state.ID)
	}
	bodyDef := box2d.MakeB2BodyDef()
	bodyDef.Type = box2d.B2BodyType.B2_dynamicBody
	bodyDef.Position = w.toMeters(state.PositionPX)
	bodyDef.Angle = state.AngleRad
	bodyDef.LinearVelocity = box2d.MakeB2Vec2(state.LinearVelocityMPS.X, state.LinearVelocityMPS.Y)
	bodyDef.AngularVelocity = state.AngularVelocityRadPerS
	bodyDef.AllowSleep = true
	bodyDef.Awake = state.Awake
	bodyDef.Active = true
	bodyDef.GravityScale = w.config.GravityScale

	record := &bodyRecord{state: state}
	bodyDef.UserData = record
	record.body = w.physics.CreateBody(&bodyDef)
	record.fixture = w.createCircleFixtureLocked(record.body, state.RadiusPX)
	w.bodies[state.ID] = record
	if state.ID >= w.nextID {
		w.nextID = state.ID + 1
	}
	return nil
}

func (w *World) createCircleFixtureLocked(body *box2d.B2Body, radiusPX float64) *box2d.B2Fixture {
	shape := box2d.MakeB2CircleShape()
	shape.M_radius = radiusPX / w.config.PTM
	fixtureDef := box2d.MakeB2FixtureDef()
	fixtureDef.Shape = &shape
	fixtureDef.Density = w.config.Density
	fixtureDef.Friction = w.config.Friction
	fixtureDef.Restitution = w.config.Restitution
	fixtureDef.Filter = box2d.MakeB2Filter()
	return body.CreateFixtureFromDef(&fixtureDef)
}

// AddDrop inserts a full-size released waiting dessert. It is primarily used
// for bounded candidate simulation; authoritative live operation still has to
// be re-baselined from the server response.
func (w *World) AddDrop(id uint64, level int, positionPX Vec2) (BodyState, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	box2dRuntimeMu.Lock()
	defer box2dRuntimeMu.Unlock()
	if err := w.usableLocked(); err != nil {
		return BodyState{}, err
	}
	if id == 0 {
		id = w.nextID
	}
	if level < 1 || level > levelCount {
		return BodyState{}, fmt.Errorf("invalid dessert drop level %d", level)
	}
	radiusPX := w.config.RadiiPX[level-1]
	if !isFinite(positionPX.X) || !isFinite(positionPX.Y) ||
		positionPX.X < w.config.LeftWallPX+radiusPX-floatEpsilon ||
		positionPX.X > w.config.RightWallPX-radiusPX+floatEpsilon ||
		!sameFloat(positionPX.Y, w.config.WaitingYPX) {
		return BodyState{}, fmt.Errorf("dessert drop position (%g,%g) is outside the legal waiting lane", positionPX.X, positionPX.Y)
	}
	state := BodyState{
		ID: id, Level: level, PositionPX: positionPX,
		LinearVelocityMPS: w.config.DropInitialVelocityMPS,
		ScaleX:            1, ScaleY: 1, Awake: true, IsFallBall: true,
	}
	state, err := normalizeBodyState(w.config, state)
	if err != nil {
		return BodyState{}, err
	}
	if err := w.addBodyLocked(state); err != nil {
		return BodyState{}, err
	}
	w.revision++
	w.latestMerges = nil
	return state, nil
}

// ApplyMerge removes the two contacted bodies and creates their next-level
// dessert at the exact midpoint. The new collider grows linearly from 0.5 to
// 1.0 over 80ms, matching the client's merge animation.
func (w *World) ApplyMerge(merge Merge) (BodyState, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	box2dRuntimeMu.Lock()
	defer box2dRuntimeMu.Unlock()
	if err := w.usableLocked(); err != nil {
		return BodyState{}, err
	}
	if merge.Revision != w.revision {
		return BodyState{}, fmt.Errorf("stale dessert merge revision %d (world %d)", merge.Revision, w.revision)
	}
	if merge.BodyAID == 0 || merge.BodyBID == 0 || merge.BodyAID == merge.BodyBID {
		return BodyState{}, errors.New("invalid dessert merge body IDs")
	}
	pair := contactPair{left: merge.BodyAID, right: merge.BodyBID}
	expected, observed := w.latestMerges[pair]
	if !observed || expected != merge {
		return BodyState{}, errors.New("dessert merge was not reported by the latest physics step")
	}
	left := w.bodies[merge.BodyAID]
	right := w.bodies[merge.BodyBID]
	if left == nil || right == nil {
		return BodyState{}, errors.New("dessert merge references a missing body")
	}
	if left.state.Level != right.state.Level || left.state.Level != merge.Level || merge.Level >= levelCount {
		return BodyState{}, errors.New("dessert merge level is no longer eligible")
	}
	leftPosition := w.positionPXLocked(left)
	rightPosition := w.positionPXLocked(right)
	position := Vec2{X: (leftPosition.X + rightPosition.X) / 2, Y: (leftPosition.Y + rightPosition.Y) / 2}

	w.physics.DestroyBody(left.body)
	w.physics.DestroyBody(right.body)
	delete(w.bodies, left.state.ID)
	delete(w.bodies, right.state.ID)

	state := BodyState{
		ID: w.nextID, Level: merge.Level + 1, PositionPX: position,
		ScaleX: w.config.MergeStartScale, ScaleY: w.config.MergeStartScale,
		Awake: true, GrowthRemaining: w.config.MergeGrowthDuration,
	}
	state, err := normalizeBodyState(w.config, state)
	if err != nil {
		w.lockFailureLocked(err)
		return BodyState{}, err
	}
	if err := w.addBodyLocked(state); err != nil {
		w.lockFailureLocked(err)
		return BodyState{}, err
	}
	w.revision++
	w.latestMerges = nil
	return state, nil
}

// Snapshot returns a deterministic copy of the current world.
func (w *World) Snapshot() Snapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.snapshotLocked()
}

func (w *World) snapshotLocked() Snapshot {
	snapshot := Snapshot{
		Revision: w.revision,
		Elapsed:  time.Duration(w.elapsedSeconds * float64(time.Second)),
		Bodies:   make([]BodyState, 0, len(w.bodies)),
	}
	if w.failure != nil {
		snapshot.Failure = w.failure.Error()
		return snapshot
	}
	for _, record := range w.bodies {
		state := w.bodyStateLocked(record)
		snapshot.Bodies = append(snapshot.Bodies, state)
		if w.touchesDangerLocked(state) {
			snapshot.TouchingDangerLine = true
		}
		if time.Duration(state.DangerLineTimeMS*float64(time.Millisecond)) >= w.config.DangerTerminalDuration {
			snapshot.TerminalDanger = true
		}
	}
	sort.Slice(snapshot.Bodies, func(left, right int) bool {
		return snapshot.Bodies[left].ID < snapshot.Bodies[right].ID
	})
	snapshot.Stable = w.stableLocked()
	return snapshot
}

func (w *World) bodyStateLocked(record *bodyRecord) BodyState {
	state := record.state
	state.PositionPX = w.positionPXLocked(record)
	velocity := record.body.GetLinearVelocity()
	state.LinearVelocityMPS = Vec2{X: velocity.X, Y: velocity.Y}
	state.AngleRad = record.body.GetAngle()
	state.AngularVelocityRadPerS = record.body.GetAngularVelocity()
	state.Awake = record.body.IsAwake()
	state.RadiusPX = w.config.RadiiPX[state.Level-1] * state.ScaleX
	return state
}

func (w *World) positionPXLocked(record *bodyRecord) Vec2 {
	position := record.body.GetPosition()
	return Vec2{X: position.X * w.config.PTM, Y: position.Y * w.config.PTM}
}

func (w *World) stableLocked() bool {
	if w.failure != nil {
		return false
	}
	threshold := w.config.StableThreshold
	for _, record := range w.bodies {
		if record.state.GrowthRemaining > 0 {
			return false
		}
		if record.state.IsFallBall || record.state.IsSyn {
			return false
		}
		velocity := record.body.GetLinearVelocity()
		if math.Abs(velocity.X) > threshold || math.Abs(velocity.Y) > threshold ||
			math.Abs(record.body.GetAngularVelocity()) > threshold {
			return false
		}
	}
	return true
}

func (w *World) touchesDangerLocked(state BodyState) bool {
	// The client checks data.radius, which is the full level radius even while
	// a newly merged collider is visually/physically growing from half scale.
	return !state.IsFallBall && state.PositionPX.Y+w.config.RadiiPX[state.Level-1] >= w.config.DangerLinePX
}

func (w *World) sortedRecordsLocked() []*bodyRecord {
	ids := make([]uint64, 0, len(w.bodies))
	for id := range w.bodies {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(left, right int) bool { return ids[left] < ids[right] })
	records := make([]*bodyRecord, 0, len(ids))
	for _, id := range ids {
		records = append(records, w.bodies[id])
	}
	return records
}

func (w *World) usableLocked() error {
	if w.failure != nil {
		return fmt.Errorf("%w: %v", ErrWorldLocked, w.failure)
	}
	return nil
}

func (w *World) lockFailureLocked(err error) {
	if w.failure == nil {
		w.failure = err
	}
}

func (w *World) toMeters(vectorPX Vec2) box2d.B2Vec2 {
	return box2d.MakeB2Vec2(vectorPX.X/w.config.PTM, vectorPX.Y/w.config.PTM)
}
