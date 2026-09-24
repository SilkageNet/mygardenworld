package runner

import (
	"errors"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestFriendElvesStealRequestAndUnavailableError(t *testing.T) {
	op := &automation.PlannedOp{Kind: clientproto.RPCFrdStealSteal.String(), Action: "steal_elves", TargetUID: 2001, TargetID: 21, ItemID: 110132, Count: 1}
	req, err := friendTouchStealRequest(op)
	if err != nil || req.StealElves != 1 {
		t.Fatalf("elf request=%+v err=%v", req, err)
	}
	if !isFriendStealElvesUnavailableError(op, errors.New("已摘取过该鲜花")) {
		t.Fatal("elf rejection not recognized")
	}
	op.Action = "steal"
	if _, err := friendTouchStealRequest(op); err == nil {
		t.Fatal("ordinary flower steal accepted elf metadata")
	}
	if isFriendStealElvesUnavailableError(op, errors.New("已摘取过该鲜花")) {
		t.Fatal("ordinary flower rejection classified as elf")
	}
}
