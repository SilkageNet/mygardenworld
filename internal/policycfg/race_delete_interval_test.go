package policycfg

import (
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
)

func TestNormalizeRaceDeleteInterval(t *testing.T) {
	for _, tt := range []struct{ input, want int32 }{{0, 120}, {-1, 120}, {1, 30}, {180, 180}, {4000, 3600}} {
		input := &pb.Policy{Union: &pb.UnionPolicy{Race: &pb.UnionRacePolicy{DeleteIntervalSeconds: tt.input}}}
		got := Normalize(input).GetUnion().GetRace().GetDeleteIntervalSeconds()
		if got != tt.want || input.Union.Race.DeleteIntervalSeconds != tt.input {
			t.Errorf("Normalize(%d)=%d, want %d without mutating input", tt.input, got, tt.want)
		}
	}
}
