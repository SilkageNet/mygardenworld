package automation

import (
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
)

// PearlCollectInterval batches production instead of collecting every mature
// slot immediately. This local precaution is not a known server rate limit.
func PearlCollectInterval(p *pb.PearlPolicy) time.Duration {
	seconds := p.GetCollectIntervalSeconds()
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(min(3600, max(60, seconds))) * time.Second
}
