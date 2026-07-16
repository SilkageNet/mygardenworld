package dessertphysics

import (
	"errors"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/ByteArena/box2d"
)

func bodyAt(id uint64, level int, xPX, yPX float64) BodyState {
	return BodyState{
		ID: id, Level: level, PositionPX: Vec2{X: xPX, Y: yPX},
		ScaleX: 1, ScaleY: 1, Awake: true,
	}
}

func requireWorld(t *testing.T, states ...BodyState) *World {
	t.Helper()
	world, err := NewWorld(DefaultConfig(), states)
	if err != nil {
		t.Fatalf("NewWorld: %v", err)
	}
	return world
}

func TestWorldPreservesWireUnitsAndSortsBodies(t *testing.T) {
	t.Parallel()
	second := bodyAt(2, 2, 64, -32)
	second.LinearVelocityMPS = Vec2{X: 2, Y: -3}
	second.AngleRad = 0.25
	second.AngularVelocityRadPerS = -0.5
	first := bodyAt(1, 1, -16, 32)

	world := requireWorld(t, second, first)
	snapshot := world.Snapshot()
	if len(snapshot.Bodies) != 2 || snapshot.Bodies[0].ID != 1 || snapshot.Bodies[1].ID != 2 {
		t.Fatalf("snapshot order = %+v", snapshot.Bodies)
	}
	got := snapshot.Bodies[1]
	if got.PositionPX != second.PositionPX || got.LinearVelocityMPS != second.LinearVelocityMPS ||
		got.AngleRad != second.AngleRad || got.AngularVelocityRadPerS != second.AngularVelocityRadPerS {
		t.Fatalf("wire units changed: got %+v want %+v", got, second)
	}
	if got.RadiusPX != DefaultConfig().RadiiPX[1] {
		t.Fatalf("level-2 radius = %g", got.RadiusPX)
	}
}

func TestAddDropUsesObservedVelocityAndLegalWaitingLane(t *testing.T) {
	t.Parallel()
	world := requireWorld(t)
	drop, err := world.AddDrop(0, 1, Vec2{X: 0, Y: 360})
	if err != nil {
		t.Fatalf("AddDrop: %v", err)
	}
	if drop.ID != 1 || drop.LinearVelocityMPS != (Vec2{Y: -10}) || !drop.IsFallBall {
		t.Fatalf("drop = %+v", drop)
	}
	snapshot := world.Snapshot()
	if got := snapshot.Bodies[0].LinearVelocityMPS.Y; got != -10 {
		t.Fatalf("snapshot drop velocity Y = %g", got)
	}
	if _, err := world.AddDrop(0, 1, Vec2{X: 0, Y: 359}); err == nil {
		t.Fatal("AddDrop accepted non-waiting Y")
	}
	if _, err := world.AddDrop(0, 1, Vec2{X: 250, Y: 360}); err == nil {
		t.Fatal("AddDrop accepted a center outside radius-adjusted walls")
	}
}

func TestFloorAndWallsContainBodies(t *testing.T) {
	t.Parallel()
	falling := bodyAt(1, 1, 0, 100)
	falling.IsFallBall = true
	wallHit := bodyAt(2, 2, 220, -200)
	wallHit.LinearVelocityMPS = Vec2{X: 20}
	world := requireWorld(t, falling, wallHit)
	for range 600 {
		if _, err := world.Step(); err != nil {
			t.Fatalf("Step: %v", err)
		}
	}
	snapshot := world.Snapshot()
	for _, body := range snapshot.Bodies {
		if body.PositionPX.X-body.RadiusPX < DefaultConfig().LeftWallPX-1 ||
			body.PositionPX.X+body.RadiusPX > DefaultConfig().RightWallPX+1 {
			t.Fatalf("body escaped side walls: %+v", body)
		}
		if body.PositionPX.Y-body.RadiusPX < DefaultConfig().FloorPX-1 {
			t.Fatalf("body escaped floor: %+v", body)
		}
	}
	if snapshot.Bodies[0].IsFallBall {
		t.Fatal("floor contact did not clear isFallBall")
	}
}

func TestSideWallDoesNotClearFallBall(t *testing.T) {
	t.Parallel()
	state := bodyAt(1, 1, 240, 0)
	state.LinearVelocityMPS = Vec2{X: 10}
	state.IsFallBall = true
	world := requireWorld(t, state)
	for range 5 {
		if _, err := world.Step(); err != nil {
			t.Fatalf("Step: %v", err)
		}
	}
	if !world.Snapshot().Bodies[0].IsFallBall {
		t.Fatal("side-wall contact cleared isFallBall")
	}
}

func TestBeginContactMergeEligibility(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		left  BodyState
		right BodyState
		want  bool
	}{
		{name: "same level", left: bodyAt(2, 1, -10, 0), right: bodyAt(1, 1, 10, 0), want: true},
		{name: "different level", left: bodyAt(1, 1, -10, 0), right: bodyAt(2, 2, 10, 0)},
		{name: "level eleven", left: bodyAt(1, 11, -50, 0), right: bodyAt(2, 11, 50, 0)},
		{name: "already syncing", left: func() BodyState { state := bodyAt(1, 1, -10, 0); state.IsSyn = true; return state }(), right: bodyAt(2, 1, 10, 0)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			world := requireWorld(t, test.left, test.right)
			result, err := world.Step()
			if err != nil {
				t.Fatalf("Step: %v", err)
			}
			if got := len(result.MergeCandidates) == 1; got != test.want {
				t.Fatalf("merge candidates = %+v, want eligible %v", result.MergeCandidates, test.want)
			}
			if test.want {
				merge := result.MergeCandidates[0]
				if merge.BodyAID != 1 || merge.BodyBID != 2 || merge.Level != 1 {
					t.Fatalf("merge is not stably ordered: %+v", merge)
				}
				for _, body := range world.Snapshot().Bodies {
					if body.IsFallBall {
						t.Fatalf("ball-ball contact left fall guard set: %+v", body)
					}
				}
			}
		})
	}
}

func TestDisjointContactsAreStableAndSharedBodyFailsClosed(t *testing.T) {
	t.Parallel()
	disjoint := requireWorld(t,
		bodyAt(4, 1, 125, 0), bodyAt(2, 1, -125, 0), bodyAt(3, 1, 105, 0), bodyAt(1, 1, -145, 0),
	)
	result, err := disjoint.Step()
	if err != nil {
		t.Fatalf("disjoint Step: %v", err)
	}
	want := []Merge{{BodyAID: 1, BodyBID: 2, Level: 1, Revision: 1}, {BodyAID: 3, BodyBID: 4, Level: 1, Revision: 1}}
	if !reflect.DeepEqual(result.MergeCandidates, want) {
		t.Fatalf("merges = %+v, want %+v", result.MergeCandidates, want)
	}

	ambiguous := requireWorld(t, bodyAt(3, 1, 18, 0), bodyAt(1, 1, -18, 0), bodyAt(2, 1, 0, 0))
	if _, err := ambiguous.Step(); !errors.Is(err, ErrAmbiguousMerge) {
		t.Fatalf("ambiguous Step error = %v", err)
	}
	if _, err := ambiguous.Step(); !errors.Is(err, ErrWorldLocked) {
		t.Fatalf("locked Step error = %v", err)
	}
	if ambiguous.Snapshot().Failure == "" {
		t.Fatal("ambiguous world snapshot did not expose failure")
	}
}

func TestApplyMergeUsesMidpointAndGrowsOverEightyMilliseconds(t *testing.T) {
	t.Parallel()
	world := requireWorld(t, bodyAt(1, 1, -10, 0), bodyAt(2, 1, 10, 0))
	result, err := world.Step()
	if err != nil || len(result.MergeCandidates) != 1 {
		t.Fatalf("Step = %+v, %v", result, err)
	}
	before := world.Snapshot()
	wantMidpoint := Vec2{
		X: (before.Bodies[0].PositionPX.X + before.Bodies[1].PositionPX.X) / 2,
		Y: (before.Bodies[0].PositionPX.Y + before.Bodies[1].PositionPX.Y) / 2,
	}
	merged, err := world.ApplyMerge(result.MergeCandidates[0])
	if err != nil {
		t.Fatalf("ApplyMerge: %v", err)
	}
	if merged.Level != 2 || merged.PositionPX != wantMidpoint || merged.ScaleX != 0.5 || merged.ScaleY != 0.5 ||
		merged.RadiusPX != DefaultConfig().RadiiPX[1]*0.5 || merged.GrowthRemaining != 80*time.Millisecond {
		t.Fatalf("merged body = %+v, midpoint %+v", merged, wantMidpoint)
	}
	for range 5 {
		if _, err := world.Step(); err != nil {
			t.Fatalf("growth Step: %v", err)
		}
	}
	grown := world.Snapshot().Bodies[0]
	if grown.ScaleX != 1 || grown.ScaleY != 1 || grown.RadiusPX != DefaultConfig().RadiiPX[1] || grown.GrowthRemaining != 0 {
		t.Fatalf("grown body = %+v", grown)
	}
}

func TestApplyMergeRejectsForgedNonContact(t *testing.T) {
	t.Parallel()
	world := requireWorld(t, bodyAt(1, 1, -100, 0), bodyAt(2, 1, 100, 0))
	result, err := world.Step()
	if err != nil || len(result.MergeCandidates) != 0 {
		t.Fatalf("Step = %+v, %v", result, err)
	}
	forged := Merge{BodyAID: 1, BodyBID: 2, Level: 1, Revision: result.Revision}
	if _, err := world.ApplyMerge(forged); err == nil {
		t.Fatal("ApplyMerge accepted a forged non-contact pair")
	}
	if got := len(world.Snapshot().Bodies); got != 2 {
		t.Fatalf("forged merge changed body count to %d", got)
	}
}

func TestStableAndDangerRules(t *testing.T) {
	t.Parallel()
	empty := requireWorld(t)
	result, err := empty.Step()
	if err != nil || !result.Stable {
		t.Fatalf("empty Step = %+v, %v", result, err)
	}

	falling := bodyAt(1, 1, 0, 260.5)
	falling.IsFallBall = true
	fallingWorld := requireWorld(t, falling)
	fallResult, err := fallingWorld.Step()
	if err != nil || fallResult.Stable || fallResult.TouchingDangerLine {
		t.Fatalf("falling Step = %+v, %v", fallResult, err)
	}

	danger := bodyAt(1, 1, 0, 260.5)
	danger.Awake = false
	danger.DangerLineTimeMS = 4990
	dangerWorld := requireWorld(t, danger)
	for index := range 14 {
		result, err = dangerWorld.Step()
		if err != nil {
			t.Fatalf("danger Step %d: %v", index, err)
		}
		if result.TerminalDanger {
			t.Fatalf("terminal danger advanced before the 250ms client timer: %+v", result)
		}
	}
	result, err = dangerWorld.Step()
	if err != nil || !result.TouchingDangerLine || !result.TerminalDanger {
		t.Fatalf("danger timer Step = %+v, %v", result, err)
	}
}

func TestStableThresholdUsesMPSAndRadiansPerSecond(t *testing.T) {
	t.Parallel()
	slow := bodyAt(1, 1, 0, 0)
	slow.LinearVelocityMPS = Vec2{X: 0.5}
	slow.AngularVelocityRadPerS = 0.5
	slowWorld := requireWorld(t, slow)
	result, err := slowWorld.Step()
	if err != nil || !result.Stable {
		t.Fatalf("slow Step = %+v, %v", result, err)
	}

	fast := bodyAt(1, 1, 0, 0)
	fast.LinearVelocityMPS = Vec2{X: 1.01}
	fastWorld := requireWorld(t, fast)
	result, err = fastWorld.Step()
	if err != nil || result.Stable {
		t.Fatalf("fast Step = %+v, %v", result, err)
	}
}

func TestDangerUsesFullLevelRadiusDuringMergeGrowth(t *testing.T) {
	t.Parallel()
	state := bodyAt(1, 11, 0, 100)
	state.ScaleX = 0.5
	state.ScaleY = 0.5
	state.Awake = false
	world := requireWorld(t, state)
	result, err := world.Step()
	if err != nil || !result.TouchingDangerLine {
		t.Fatalf("growing danger Step = %+v, %v", result, err)
	}
}

func TestClientTimerSnapsSmallHorizontalAndAngularVelocity(t *testing.T) {
	t.Parallel()
	state := bodyAt(1, 1, 0, 100)
	state.LinearVelocityMPS = Vec2{X: 0.05}
	state.AngularVelocityRadPerS = 0.05
	world := requireWorld(t, state)
	for index := range 14 {
		if _, err := world.Step(); err != nil {
			t.Fatalf("Step %d: %v", index, err)
		}
	}
	before := world.Snapshot().Bodies[0]
	if before.LinearVelocityMPS.X == 0 || before.AngularVelocityRadPerS == 0 {
		t.Fatalf("velocity snapped before 250ms: %+v", before)
	}
	if _, err := world.Step(); err != nil {
		t.Fatalf("timer Step: %v", err)
	}
	after := world.Snapshot().Bodies[0]
	if after.LinearVelocityMPS.X != 0 || after.AngularVelocityRadPerS != 0 {
		t.Fatalf("velocity not snapped at 250ms: %+v", after)
	}
}

func TestEvolutionIsDeterministicAcrossInputOrder(t *testing.T) {
	t.Parallel()
	growing := bodyAt(7, 2, -100, -250)
	growing.ScaleX = 0.5
	growing.ScaleY = 0.5
	growing.LinearVelocityMPS = Vec2{X: 1.25, Y: 0.5}
	other := bodyAt(3, 4, 100, -100)
	other.LinearVelocityMPS = Vec2{X: -0.75, Y: -0.25}
	other.AngularVelocityRadPerS = 0.2
	forward := requireWorld(t, growing, other)
	reverse := requireWorld(t, other, growing)
	for frame := range 120 {
		forwardStep, forwardErr := forward.Step()
		reverseStep, reverseErr := reverse.Step()
		if !reflect.DeepEqual(forwardErr, reverseErr) || !reflect.DeepEqual(forwardStep, reverseStep) {
			t.Fatalf("frame %d Step differs: forward=%+v/%v reverse=%+v/%v", frame, forwardStep, forwardErr, reverseStep, reverseErr)
		}
		if !reflect.DeepEqual(forward.Snapshot(), reverse.Snapshot()) {
			t.Fatalf("frame %d snapshot depends on input order", frame)
		}
		if forwardErr != nil {
			break
		}
	}
}

func TestAdvanceUntilReasons(t *testing.T) {
	t.Parallel()
	mergeWorld := requireWorld(t, bodyAt(1, 1, -10, 0), bodyAt(2, 1, 10, 0))
	merge, err := mergeWorld.AdvanceUntil(5)
	if err != nil || merge.Reason != AdvanceMerge || merge.Steps != 1 {
		t.Fatalf("merge advance = %+v, %v", merge, err)
	}
	stableWorld := requireWorld(t)
	stable, err := stableWorld.AdvanceUntil(5)
	if err != nil || stable.Reason != AdvanceStable {
		t.Fatalf("stable advance = %+v, %v", stable, err)
	}
	dangerBody := bodyAt(1, 1, 0, 260.5)
	dangerBody.Awake = false
	dangerWorld := requireWorld(t, dangerBody)
	danger, err := dangerWorld.AdvanceUntil(5)
	if err != nil || danger.Reason != AdvanceDanger {
		t.Fatalf("danger advance = %+v, %v", danger, err)
	}
}

func TestStepFailsClosedOnNonFiniteBox2DOutput(t *testing.T) {
	t.Parallel()
	world := requireWorld(t, bodyAt(1, 1, 0, 0))
	world.bodies[1].body.SetLinearVelocity(box2d.MakeB2Vec2(math.NaN(), 0))
	if _, err := world.Step(); !errors.Is(err, ErrNonFiniteState) {
		t.Fatalf("Step error = %v", err)
	}
	if _, err := world.Step(); !errors.Is(err, ErrWorldLocked) {
		t.Fatalf("locked Step error = %v", err)
	}
	if snapshot := world.Snapshot(); snapshot.Failure == "" || len(snapshot.Bodies) != 0 {
		t.Fatalf("failed snapshot leaked unsafe bodies: %+v", snapshot)
	}
}
