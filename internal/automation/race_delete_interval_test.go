package automation

import (
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
)

func TestRaceDeleteInterval(t *testing.T) {
	for _, tt := range []struct{ input, want int32 }{{0, 120}, {-1, 120}, {1, 30}, {30, 30}, {180, 180}, {3600, 3600}, {3601, 3600}} {
		if got := RaceDeleteInterval(&pb.UnionRacePolicy{DeleteIntervalSeconds: tt.input}); got != time.Duration(tt.want)*time.Second {
			t.Errorf("interval(%d)=%s, want %ds", tt.input, got, tt.want)
		}
	}
	if RaceDeleteInterval(nil) != 120*time.Second || DefaultPolicy().Union.Race.DeleteIntervalSeconds != 120 {
		t.Fatal("missing policy must use 120 seconds")
	}
}
