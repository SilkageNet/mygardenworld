package runner

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestFriendStealElvesSuccessMessageIncludesElves(t *testing.T) {
	st := state.New()
	now := time.Now()
	st.ApplyV([]byte(fmt.Sprintf(`{"28":{"5":[{"0":2001,"1":"榴莲","4":20}]},"111":{"0":{"0":100,"3":%d}}}`, now.UnixMilli())))
	msg := friendStealElvesSuccessMessage(&automation.PlannedOp{
		TargetUID: 2001,
		TargetID:  21,
		ItemID:    110132,
		Action:    "steal_elves",
	}, st, now)
	if !strings.Contains(msg, "摸取好友 榴莲") || !strings.Contains(msg, "田地 #21") {
		t.Fatalf("message=%q", msg)
	}
	if label := state.ItemLabel(110132); label != "" && !strings.Contains(msg, label) {
		t.Fatalf("want elves label %q in %q", label, msg)
	}
}

func TestOpDescStealElvesUses摸取(t *testing.T) {
	got := opDesc(&automation.PlannedOp{Action: "steal_elves", FeatureID: "plant.friend_steal_elves"})
	if got != "摸取花灵" {
		t.Fatalf("opDesc=%q", got)
	}
}
