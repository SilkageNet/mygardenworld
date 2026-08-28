//go:build ignore

package main

import (
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

func main() {
	dur := state.FlowerRackSellDurationMs()
	fmt.Printf("FlowerRackSellDurationMs = %d ms (%v per item)\n", dur, time.Duration(dur)*time.Millisecond)
	// Last sell at 07:52:43 Shanghai on racks 1-6 with num=12
	listed := time.Date(2026, 8, 25, 7, 52, 43, 0, time.FixedZone("CST", 8*3600))
	ready := listed.Add(time.Duration(12*dur) * time.Millisecond)
	fmt.Printf("Listed all 6 at %v\n", listed)
	fmt.Printf("Earliest claim (12*dur) at %v\n", ready)
	fmt.Printf("Now %v claimable=%v\n", time.Now().In(listed.Location()), time.Now().After(ready))
}
