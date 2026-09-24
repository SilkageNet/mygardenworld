package state

import (
	"testing"
	"time"
)

func TestFriendElvesStealNeedsObservedEffect(t *testing.T) {
	s := New()
	now := time.Now()
	s.frdVisitUID = 2001
	s.roleID = 100
	s.frdVisitLands = map[int32]LandView{21: {FlowerID: 23331, State: 3, ElvesID: 110132, PlantTimeMs: 123}}
	s.inventory = map[int32]int32{110132: 2}
	if s.FriendStealElvesActuallyStolen(2001, 21, 110132, 0, false, 2, now) {
		t.Fatal("RPC acknowledgement without an elf effect counted as success")
	}
	s.MarkFriendStealElvesLandUnavailable(2001, 21)
	if !s.FriendStealElvesLandSkipped(2001, 21, 123) || s.FriendStealElvesLandSkipped(2001, 21, 124) {
		t.Fatal("rejected land skip must last until replant")
	}
	s.inventory[110132] = 3
	if !s.FriendStealElvesActuallyStolen(2001, 21, 110132, 0, false, 2, now) {
		t.Fatal("observed elf inventory gain was not recognized")
	}
}
