package dessertphysics

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"slices"
	"testing"
)

type replayFixture struct {
	Schema      string             `json:"schema"`
	Mode        int                `json:"mode"`
	Checkpoints []replayCheckpoint `json:"checkpoints"`
}

type replayCheckpoint struct {
	Sequence   int             `json:"sequence"`
	Operation  string          `json:"operation"`
	MergeLevel int             `json:"merge_level"`
	Submitted  replaySubmitted `json:"submitted"`
}

type replaySubmitted struct {
	Bodies []replayBody `json:"bodies"`
}

type replayBody struct {
	Level           int     `json:"level"`
	Position        Vec2    `json:"position"`
	LinearVelocity  Vec2    `json:"linear_velocity"`
	AngularVelocity float64 `json:"angular_velocity"`
	NodeAngleDeg    float64 `json:"node_angle_deg"`
	Scale           struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		Z float64 `json:"z"`
	} `json:"scale"`
	Awake   bool `json:"awake"`
	Falling bool `json:"falling"`
}

func TestSanitizedRoundCheckpointsReconstructDeterministically(t *testing.T) {
	// The checkpoints are authoritative request snapshots, not adjacent physics
	// frames. This test deliberately verifies schema/counts, units, bounds, and
	// deterministic reconstruction only; it does not claim trajectory replay or
	// live-autoplay equivalence.
	raw, err := os.ReadFile("testdata/mode1_round_100.json")
	if err != nil {
		t.Fatalf("read replay fixture: %v", err)
	}
	var fixture replayFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decode replay fixture: %v", err)
	}
	if fixture.Schema != "dessert-mode1-round-v1" || fixture.Mode != 1 || len(fixture.Checkpoints) != 181 {
		t.Fatalf("fixture identity/count = %q mode %d checkpoints %d", fixture.Schema, fixture.Mode, len(fixture.Checkpoints))
	}

	wantMergeLevels := []int{
		2, 3, 4, 3, 5, 4, 5, 6, 5, 2, 5, 6, 7, 3, 2, 3, 4, 5, 6, 4, 5, 6, 6, 2, 3, 4, 7,
		5, 3, 4, 5, 6, 5, 3, 4, 5, 6, 3, 4, 6, 7, 8, 3, 4, 5, 4, 6, 5, 2, 3, 6, 7, 3, 4,
		5, 6, 7, 8, 9, 4, 5, 3, 6, 3, 6, 3, 4, 5, 6, 7, 4, 6, 7, 3, 3, 4, 5, 5, 6, 7, 8,
	}
	drops := 0
	sawColliderIgnoreScaleZ := false
	mergeLevels := make([]int, 0, len(wantMergeLevels))
	config := DefaultConfig()
	for index, checkpoint := range fixture.Checkpoints {
		if checkpoint.Sequence != index+1 {
			t.Fatalf("checkpoint[%d] sequence = %d", index, checkpoint.Sequence)
		}
		switch checkpoint.Operation {
		case "drop":
			drops++
		case "merge":
			mergeLevels = append(mergeLevels, checkpoint.MergeLevel)
		default:
			t.Fatalf("checkpoint[%d] operation = %q", index, checkpoint.Operation)
		}

		states := make([]BodyState, 0, len(checkpoint.Submitted.Bodies))
		for bodyIndex, body := range checkpoint.Submitted.Bodies {
			if body.Scale.Z != body.Scale.X {
				sawColliderIgnoreScaleZ = true
			}
			state := BodyState{
				ID: uint64(bodyIndex + 1), Level: body.Level,
				PositionPX: body.Position, LinearVelocityMPS: body.LinearVelocity,
				AngleRad: body.NodeAngleDeg * math.Pi / 180, AngularVelocityRadPerS: body.AngularVelocity,
				ScaleX: body.Scale.X, ScaleY: body.Scale.Y, Awake: body.Awake, IsFallBall: body.Falling,
			}
			assertReplayBodyFiniteAndBounded(t, config, checkpoint.Sequence, state)
			states = append(states, state)
		}

		first, err := NewWorld(config, states)
		if err != nil {
			t.Fatalf("checkpoint %d first reconstruction: %v", checkpoint.Sequence, err)
		}
		reversed := slices.Clone(states)
		slices.Reverse(reversed)
		second, err := NewWorld(config, reversed)
		if err != nil {
			t.Fatalf("checkpoint %d second reconstruction: %v", checkpoint.Sequence, err)
		}
		firstSnapshot := first.Snapshot()
		secondSnapshot := second.Snapshot()
		if !reflect.DeepEqual(firstSnapshot, secondSnapshot) {
			t.Fatalf("checkpoint %d reconstruction depends on input order", checkpoint.Sequence)
		}
		for _, state := range firstSnapshot.Bodies {
			assertReplayBodyFiniteAndBounded(t, config, checkpoint.Sequence, state)
		}
	}
	if drops != 100 || len(mergeLevels) != 81 {
		t.Fatalf("fixture operations = %d drops/%d merges", drops, len(mergeLevels))
	}
	if !slices.Equal(mergeLevels, wantMergeLevels) {
		t.Fatalf("merge-level sequence = %v", mergeLevels)
	}
	if !sawColliderIgnoreScaleZ {
		t.Fatal("fixture did not exercise wire ScaleZ being independent of collider X/Y scale")
	}
}

func assertReplayBodyFiniteAndBounded(t *testing.T, config Config, sequence int, state BodyState) {
	t.Helper()
	values := []float64{
		state.PositionPX.X, state.PositionPX.Y,
		state.LinearVelocityMPS.X, state.LinearVelocityMPS.Y,
		state.AngleRad, state.AngularVelocityRadPerS,
		state.ScaleX, state.ScaleY,
	}
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			t.Fatalf("checkpoint %d body %d has non-finite value", sequence, state.ID)
		}
	}
	if state.Level < 1 || state.Level > len(config.RadiiPX) || state.ScaleX < config.MergeStartScale ||
		state.ScaleX > 1 || state.ScaleX != state.ScaleY {
		t.Fatalf("checkpoint %d body %d has invalid level/scale: %+v", sequence, state.ID, state)
	}
	radiusPX := config.RadiiPX[state.Level-1] * state.ScaleX
	if state.RadiusPX != 0 && math.Abs(state.RadiusPX-radiusPX) > floatEpsilon {
		t.Fatalf("checkpoint %d body %d radius=%g, want X/Y-scaled %g", sequence, state.ID, state.RadiusPX, radiusPX)
	}
	const tolerancePX = 2
	if state.PositionPX.X-radiusPX < config.LeftWallPX-tolerancePX ||
		state.PositionPX.X+radiusPX > config.RightWallPX+tolerancePX ||
		state.PositionPX.Y-radiusPX < config.FloorPX-tolerancePX ||
		state.PositionPX.Y > config.WaitingYPX+tolerancePX {
		t.Fatalf("checkpoint %d body %d is out of observed bounds: %+v radius=%g", sequence, state.ID, state, radiusPX)
	}
}
