package dessertphysics

import (
	"testing"
	"time"
)

func TestDefaultConfigMatchesObservedClientConstants(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	if config.StepSeconds != 1.0/60.0 || config.VelocityIterations != 10 || config.PositionIterations != 10 {
		t.Fatalf("step config = %g/%d/%d", config.StepSeconds, config.VelocityIterations, config.PositionIterations)
	}
	if config.PTM != 32 || config.GravityMPS2 != -10 || config.GravityScale != 2.5 {
		t.Fatalf("world config = PTM %g gravity %g scale %g", config.PTM, config.GravityMPS2, config.GravityScale)
	}
	if config.Density != 0.8 || config.Friction != 0.2 || config.Restitution != 0.2 {
		t.Fatalf("fixture config = density %g friction %g restitution %g", config.Density, config.Friction, config.Restitution)
	}
	if config.LeftWallPX != -262 || config.RightWallPX != 262 || config.FloorPX != -400 ||
		config.DangerLinePX != 279 || config.WaitingYPX != 360 {
		t.Fatalf("bounds config = %+v", config)
	}
	if config.StableThreshold != 1 || config.MergeStartScale != 0.5 || config.MergeGrowthDuration != 80*time.Millisecond {
		t.Fatalf("runtime config = %+v", config)
	}
	if config.DropInitialVelocityMPS != (Vec2{Y: -10}) {
		t.Fatalf("drop velocity = %+v", config.DropInitialVelocityMPS)
	}
	wantRadii := [11]float64{19.5, 26.5, 36.5, 45.5, 56, 63.5, 79, 105, 111.5, 135.5, 187}
	if config.RadiiPX != wantRadii {
		t.Fatalf("radii = %v", config.RadiiPX)
	}
	if err := config.validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestNewWorldRejectsModifiedConfigAndAnisotropicScale(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	config.PTM = 64
	if _, err := NewWorld(config, nil); err == nil {
		t.Fatal("NewWorld accepted a modified PTM")
	}

	state := bodyAt(1, 1, 0, 0)
	state.ScaleY = 0.5
	if _, err := NewWorld(DefaultConfig(), []BodyState{state}); err == nil {
		t.Fatal("NewWorld accepted anisotropic X/Y collider scale")
	}
}
