// Package dessertphysics provides a deterministic, pure-Go adapter for the
// normal-mode physics used by the dessert activity.
//
// Client positions and collider radii are expressed in pixels. Linear
// velocities use Box2D metres per second, while angles and angular velocities
// use radians and radians per second. Keeping those units explicit is
// important: the Cocos client divides positions by PTM, but passes its public
// linearVelocity value directly to Box2D.
package dessertphysics

import (
	"errors"
	"math"
	"time"
)

const (
	levelCount   = 11
	floatEpsilon = 1e-9
)

var defaultRadiiPX = [levelCount]float64{
	19.5, 26.5, 36.5, 45.5, 56, 63.5, 79, 105, 111.5, 135.5, 187,
}

// Vec2 is a two-dimensional vector. The field using it states the vector's
// unit explicitly (for example PositionPX or LinearVelocityMPS).
type Vec2 struct {
	X float64
	Y float64
}

// Config contains the observed client constants. NewWorld rejects modified
// values so a caller cannot accidentally run a materially different physics
// model and treat its output as client-compatible.
type Config struct {
	StepSeconds            float64
	VelocityIterations     int
	PositionIterations     int
	PTM                    float64
	GravityMPS2            float64
	GravityScale           float64
	Density                float64
	Friction               float64
	Restitution            float64
	LeftWallPX             float64
	RightWallPX            float64
	FloorPX                float64
	DangerLinePX           float64
	WaitingYPX             float64
	StableThreshold        float64
	MergeStartScale        float64
	MergeGrowthDuration    time.Duration
	DangerTerminalDuration time.Duration
	DropInitialVelocityMPS Vec2
	RadiiPX                [levelCount]float64
}

// DefaultConfig returns the immutable normal-mode client constants observed in
// the unpacked mini-program.
func DefaultConfig() Config {
	return Config{
		StepSeconds:            1.0 / 60.0,
		VelocityIterations:     10,
		PositionIterations:     10,
		PTM:                    32,
		GravityMPS2:            -10,
		GravityScale:           2.5,
		Density:                0.8,
		Friction:               0.2,
		Restitution:            0.2,
		LeftWallPX:             -262,
		RightWallPX:            262,
		FloorPX:                -400,
		DangerLinePX:           279,
		WaitingYPX:             360,
		StableThreshold:        1,
		MergeStartScale:        0.5,
		MergeGrowthDuration:    80 * time.Millisecond,
		DangerTerminalDuration: 5 * time.Second,
		DropInitialVelocityMPS: Vec2{Y: -10},
		RadiiPX:                defaultRadiiPX,
	}
}

func (c Config) validate() error {
	want := DefaultConfig()
	if !sameFloat(c.StepSeconds, want.StepSeconds) ||
		c.VelocityIterations != want.VelocityIterations ||
		c.PositionIterations != want.PositionIterations ||
		!sameFloat(c.PTM, want.PTM) ||
		!sameFloat(c.GravityMPS2, want.GravityMPS2) ||
		!sameFloat(c.GravityScale, want.GravityScale) ||
		!sameFloat(c.Density, want.Density) ||
		!sameFloat(c.Friction, want.Friction) ||
		!sameFloat(c.Restitution, want.Restitution) ||
		!sameFloat(c.LeftWallPX, want.LeftWallPX) ||
		!sameFloat(c.RightWallPX, want.RightWallPX) ||
		!sameFloat(c.FloorPX, want.FloorPX) ||
		!sameFloat(c.DangerLinePX, want.DangerLinePX) ||
		!sameFloat(c.WaitingYPX, want.WaitingYPX) ||
		!sameFloat(c.StableThreshold, want.StableThreshold) ||
		!sameFloat(c.MergeStartScale, want.MergeStartScale) ||
		c.MergeGrowthDuration != want.MergeGrowthDuration ||
		c.DangerTerminalDuration != want.DangerTerminalDuration ||
		!sameVec(c.DropInitialVelocityMPS, want.DropInitialVelocityMPS) {
		return errors.New("dessert physics config differs from observed client constants")
	}
	for index := range c.RadiiPX {
		if !sameFloat(c.RadiiPX[index], want.RadiiPX[index]) {
			return errors.New("dessert physics radii differ from observed client constants")
		}
	}
	return nil
}

func sameVec(left, right Vec2) bool {
	return sameFloat(left.X, right.X) && sameFloat(left.Y, right.Y)
}

func sameFloat(left, right float64) bool {
	return isFinite(left) && isFinite(right) && math.Abs(left-right) <= floatEpsilon
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
