package runner

import (
	"io"
	"log/slog"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func testRiskPauseRunner(policy *pb.Policy) *Runner {
	return &Runner{
		account: &store.Account{ID: 1, Name: "risk-pause-test"},
		policy:  policy,
		state:   state.New(),
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func shanghaiAt(hour, min int) time.Time {
	return time.Date(2026, 9, 21, hour, min, 0, 0, shanghai)
}

func TestRiskPausePhaseAlignedToMidnight(t *testing.T) {
	// 120 run + 12 pause: 00:00-02:00 run, 02:00-02:12 pause, 02:12-04:12 run.
	cases := []struct {
		name    string
		at      time.Time
		pausing bool
		untilH  int
		untilM  int
		nextH   int
		nextM   int
	}{
		{name: "midnight running", at: shanghaiAt(0, 0), nextH: 2, nextM: 0},
		{name: "just before stop", at: shanghaiAt(1, 59), nextH: 2, nextM: 0},
		{name: "stop instant", at: shanghaiAt(2, 0), pausing: true, untilH: 2, untilM: 12},
		{name: "mid pause", at: shanghaiAt(2, 6), pausing: true, untilH: 2, untilM: 12},
		{name: "resume instant", at: shanghaiAt(2, 12), nextH: 4, nextM: 12},
		{name: "second stop", at: shanghaiAt(4, 12), pausing: true, untilH: 4, untilM: 24},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			phase, ok := riskPausePhaseAt(tt.at, 120, 12)
			if !ok {
				t.Fatal("phase not ok")
			}
			if phase.pausing != tt.pausing {
				t.Fatalf("pausing=%v, want %v", phase.pausing, tt.pausing)
			}
			if tt.pausing {
				want := shanghaiAt(tt.untilH, tt.untilM)
				if !phase.pauseUntil.Equal(want) {
					t.Fatalf("pause until=%v, want %v", phase.pauseUntil, want)
				}
				return
			}
			want := shanghaiAt(tt.nextH, tt.nextM)
			if !phase.nextPauseAt.Equal(want) {
				t.Fatalf("next pause=%v, want %v", phase.nextPauseAt, want)
			}
		})
	}
}

func TestMaybeEnterRiskPauseFollowsWallClockNotLogin(t *testing.T) {
	policy := automation.DefaultPolicy()
	policy.Basic.RunPauseEnabled = true
	policy.Basic.RunDurationMinutes = 120
	policy.Basic.PauseDurationMinutes = 12
	r := testRiskPauseRunner(policy)

	if r.maybeEnterRiskPause(shanghaiAt(1, 0)) {
		t.Fatal("entered pause during the run window")
	}
	if !r.maybeEnterRiskPause(shanghaiAt(2, 1)) {
		t.Fatal("did not enter pause at 02:01")
	}
	until, active := r.riskPauseDeadline()
	if !active || !until.Equal(shanghaiAt(2, 12)) {
		t.Fatalf("pause until=%v, want 02:12", until)
	}
}

func TestRiskPauseSnapshotIgnoresSessionClock(t *testing.T) {
	policy := automation.DefaultPolicy()
	policy.Basic.RunPauseEnabled = true
	policy.Basic.RunDurationMinutes = 120
	policy.Basic.PauseDurationMinutes = 12
	r := testRiskPauseRunner(policy)

	running := r.RiskPauseSnapshot(shanghaiAt(1, 30))
	if running.Pausing || !running.NextPauseAt.Equal(shanghaiAt(2, 0)) {
		t.Fatalf("running snapshot=%+v", running)
	}
	pausing := r.RiskPauseSnapshot(shanghaiAt(2, 5))
	if !pausing.Pausing || !pausing.PauseUntil.Equal(shanghaiAt(2, 12)) {
		t.Fatalf("pausing snapshot=%+v", pausing)
	}
}

func TestMaybeEnterRiskPauseDisabledDoesNothing(t *testing.T) {
	r := testRiskPauseRunner(automation.DefaultPolicy())
	if r.maybeEnterRiskPause(shanghaiAt(2, 5)) {
		t.Fatal("disabled run-pause entered pause")
	}
}

func TestSetPolicyDisablingRunPauseClearsDeadline(t *testing.T) {
	policy := automation.DefaultPolicy()
	policy.Basic.RunPauseEnabled = true
	r := testRiskPauseRunner(policy)
	r.riskPauseUntil = shanghaiAt(2, 12)

	disabled := automation.DefaultPolicy()
	disabled.Basic.RunPauseEnabled = false
	r.SetPolicy(disabled)
	if _, active := r.riskPauseDeadline(); active {
		t.Fatal("disabling run-pause left deadline active")
	}
}

func TestDiagnosticsOmitsDisconnectedDuringRiskPause(t *testing.T) {
	policy := automation.DefaultPolicy()
	policy.Basic.RunPauseEnabled = true
	r := testRiskPauseRunner(policy)
	now := time.Now()
	r.riskPauseUntil = now.Add(time.Minute)
	diag := r.Diagnostics(now)
	for _, reason := range diag.BlockedReasons {
		if reason == "WebSocket 未连接" {
			t.Fatal("risk pause should not report WebSocket disconnected as blocked")
		}
	}
}
