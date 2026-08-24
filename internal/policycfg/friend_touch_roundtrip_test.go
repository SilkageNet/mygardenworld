package policycfg

import (
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
)

func TestFriendTouchRoundTrip(t *testing.T) {
	in := &pb.Policy{Basic: &pb.BasicPolicy{FriendTouch: &pb.FriendTouchPolicy{
		Enabled:      true,
		Mode:         pb.SelectionMode_SELECTION_MODE_SPECIFIC,
		FriendCounts: map[int64]int32{2001: 3},
		ExcludeUids:  []int64{2002},
	}}}
	raw, err := ToJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := FromJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	ft := out.GetBasic().GetFriendTouch()
	if !ft.GetEnabled() || ft.GetMode() != pb.SelectionMode_SELECTION_MODE_SPECIFIC || ft.GetFriendCounts()[2001] != 3 {
		t.Fatalf("roundtrip=%+v raw=%s", ft, raw)
	}
}

func TestNormalizeFillsFriendTouch(t *testing.T) {
	got := Normalize(&pb.Policy{Basic: &pb.BasicPolicy{}})
	if got.GetBasic().GetFriendTouch() == nil {
		t.Fatal("expected friend_touch default")
	}
	if got.GetBasic().GetFriendTouch().GetMode() != pb.SelectionMode_SELECTION_MODE_ALL {
		t.Fatalf("mode=%v", got.GetBasic().GetFriendTouch().GetMode())
	}
	preserved := Normalize(&pb.Policy{Basic: &pb.BasicPolicy{FriendTouch: &pb.FriendTouchPolicy{
		Enabled: true,
		Mode:    pb.SelectionMode_SELECTION_MODE_SPECIFIC,
	}}})
	if !preserved.GetBasic().GetFriendTouch().GetEnabled() {
		t.Fatal("enabled should be preserved")
	}
}
