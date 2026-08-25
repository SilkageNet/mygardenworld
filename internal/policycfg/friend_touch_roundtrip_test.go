package policycfg

import (
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
)

func TestFriendStealRoundTrip(t *testing.T) {
	in := &pb.Policy{Plant: &pb.PlantPolicy{FriendSteal: &pb.FriendStealPolicy{
		Enabled:         true,
		Mode:            pb.SelectionMode_SELECTION_MODE_QUALITY,
		Qualities:       []int32{4, 5},
		FriendMode:      pb.SelectionMode_SELECTION_MODE_SPECIFIC,
		FriendCounts:    map[int64]int32{2001: 3},
		ExcludeUids:     []int64{2002},
		AutoBuyTimes:    true,
		MaxBuyPerFriend: 2,
	}}}
	raw, err := ToJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := FromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	policy := out.GetPlant().GetFriendSteal()
	if !policy.GetEnabled() || policy.GetFriendMode() != pb.SelectionMode_SELECTION_MODE_SPECIFIC || policy.GetFriendCounts()[2001] != 3 {
		t.Fatalf("roundtrip=%+v raw=%s", policy, raw)
	}
	if policy.GetMode() != pb.SelectionMode_SELECTION_MODE_QUALITY || len(policy.GetQualities()) != 2 || policy.GetMaxBuyPerFriend() != 2 {
		t.Fatalf("flower/buy policy lost: %+v", policy)
	}
}

func TestNormalizeMigratesLegacyFriendTouch(t *testing.T) {
	got := Normalize(&pb.Policy{Basic: &pb.BasicPolicy{FriendTouch: &pb.FriendTouchPolicy{
		Enabled:         true,
		Mode:            pb.SelectionMode_SELECTION_MODE_SPECIFIC,
		FriendCounts:    map[int64]int32{2001: 3},
		ExcludeUids:     []int64{2002},
		AutoBuyTimes:    true,
		MaxBuyPerFriend: 2,
	}}})
	if got.GetBasic().GetFriendTouch() != nil {
		t.Fatal("legacy basic.friend_touch should be cleared after migration")
	}
	policy := got.GetPlant().GetFriendSteal()
	if !policy.GetEnabled() || policy.GetFriendMode() != pb.SelectionMode_SELECTION_MODE_SPECIFIC || policy.GetFriendCounts()[2001] != 3 {
		t.Fatalf("legacy policy not migrated: %+v", policy)
	}
	if !policy.GetAutoBuyTimes() || policy.GetMaxBuyPerFriend() != 2 || len(policy.GetExcludeUids()) != 1 {
		t.Fatalf("legacy limits not migrated: %+v", policy)
	}
}

func TestNormalizeFriendStealFailClosedLimits(t *testing.T) {
	got := Normalize(&pb.Policy{Plant: &pb.PlantPolicy{FriendSteal: &pb.FriendStealPolicy{
		FriendMode:      pb.SelectionMode_SELECTION_MODE_QUALITY,
		FriendCounts:    map[int64]int32{-1: 3, 2001: 999},
		ExcludeUids:     []int64{0, 2002, 2002},
		MaxBuyPerFriend: 999,
		BuyCount:        5,
		MaxSpendDiamond: 100,
	}}})
	policy := got.GetPlant().GetFriendSteal()
	if policy.GetFriendMode() != pb.SelectionMode_SELECTION_MODE_SPECIFIC {
		t.Fatalf("invalid friend mode should normalize to specific with targets: %v", policy.GetFriendMode())
	}
	if _, exists := policy.GetFriendCounts()[-1]; exists || policy.GetFriendCounts()[2001] > 20 {
		t.Fatalf("friend counts not cleaned: %+v", policy.GetFriendCounts())
	}
	if len(policy.GetExcludeUids()) != 1 || policy.GetMaxBuyPerFriend() > 10 {
		t.Fatalf("friend limits not cleaned: %+v", policy)
	}
	if policy.GetBuyCount() != 0 || policy.GetMaxSpendDiamond() != 0 {
		t.Fatalf("speculative diamond fields must be cleared: %+v", policy)
	}
}
