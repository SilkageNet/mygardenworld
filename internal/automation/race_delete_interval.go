package automation

import (
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
)

// RaceDeleteInterval is a local precaution, not an observed server threshold.
// Keep the lower bound even for policies supplied outside policycfg.Normalize.
func RaceDeleteInterval(p *pb.UnionRacePolicy) time.Duration {
	seconds := p.GetDeleteIntervalSeconds()
	if seconds <= 0 {
		seconds = 120
	}
	return time.Duration(max(30, min(3600, seconds))) * time.Second
}
