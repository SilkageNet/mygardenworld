package automation

import (
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestRaceIdlePollingIsBoundedAndDoesNotAccelerateHeldTasks(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	for _, tc := range []struct {
		name                 string
		age                  time.Duration
		held, off, exhausted bool
		want                 bool
	}{
		{name: "idle before deadline", age: 9 * time.Second},
		{name: "idle at deadline", age: 10 * time.Second, want: true},
		{name: "held retains 30s", age: 20 * time.Second, held: true},
		{name: "maintenance before deadline", age: 299 * time.Second, off: true},
		{name: "maintenance at deadline", age: 300 * time.Second, off: true, want: true},
		{name: "held while auto off retains 30s", age: 30 * time.Second, off: true, held: true, want: true},
		{name: "quota exhausted retains 30s", age: 20 * time.Second, exhausted: true},
		{name: "held ordinary deadline", age: 30 * time.Second, held: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view := state.FmlRaceView{BatchActive: true, TasksObserved: true, TasksSyncedAtMs: now.UnixMilli(),
				TaskPoolSyncAttemptAtMs: now.Add(-tc.age).UnixMilli(), TakeQuotaExhausted: tc.exhausted,
				Taken: state.FmlRaceTakenView{HasTask: tc.held}}
			policy := testRacePolicy()
			policy.AutoEnableModules = !tc.off
			if got := raceTaskPoolRefreshDue(view, policy, now); got != tc.want {
				t.Fatalf("due=%v want=%v", got, tc.want)
			}
		})
	}
}
