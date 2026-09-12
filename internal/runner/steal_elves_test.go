package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestStealElvesFlag(t *testing.T) {
	if stealElvesFlag(&automation.PlannedOp{Action: "steal_elves"}) != 1 {
		t.Fatal("expected 1")
	}
	if stealElvesFlag(&automation.PlannedOp{Action: "steal"}) != 0 {
		t.Fatal("expected 0")
	}
}

func TestFriendTouchStealRequestAllowsElvesItemID(t *testing.T) {
	req, err := friendTouchStealRequest(&automation.PlannedOp{
		TargetUID: 75485026103424,
		TargetID:  1009,
		Count:     1,
		ItemID:    110156,
		Action:    "steal_elves",
	})
	if err != nil {
		t.Fatalf("steal_elves with ItemID metadata: %v", err)
	}
	want := clientproto.FrdStealStealRequest{FrdUid: 75485026103424, LandId: 1009, StealElves: 1}
	if req != want {
		t.Fatalf("got %+v want %+v", req, want)
	}

	if _, err := friendTouchStealRequest(&automation.PlannedOp{
		TargetUID: 1,
		TargetID:  2,
		Count:     1,
		ItemID:    110156,
		Action:    "steal",
	}); err == nil {
		t.Fatal("ordinary steal must still reject ItemID")
	}
}

func TestFriendStealElvesUnavailableSkipsLand(t *testing.T) {
	r := newOperationEventTestRunner()
	r.state.ApplyV([]byte(`{"7":{"0":{"0":9001}}}`))
	r.state.ApplyV([]byte(`{"111":{"1":{"0":2001,"1":{"1020":{"0":23404,"1":3,"6":110156,"7":100,"8":[]},"1040":{"0":23404,"1":3,"6":110156,"7":100,"8":[]}}}}}`))
	op := &automation.PlannedOp{
		Kind:      clientproto.RPCFrdStealSteal.String(),
		Action:    "steal_elves",
		FeatureID: "plant.friend_steal_elves",
		TargetUID: 2001,
		TargetID:  1020,
		ItemID:    110156,
		Count:     1,
		Category:  automation.CategoryElves,
		Domain:    "farm.elves_steal",
	}
	if err := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op, args: map[string]any{}},
		err:              errors.New("rpc frdSteal.steal: server: 花灵已被他人摘取"),
		finishedAt:       time.Now(),
	}); err != nil {
		t.Fatalf("handleOperationError=%v, want nil", err)
	}
	if !r.state.FriendStealElvesLandSkipped(2001, 1020, 100) {
		t.Fatal("expected sticky skip after unavailable error")
	}
	if !isFriendStealElvesUnavailableError(op, errors.New("server: 已摘取过该鲜花")) {
		t.Fatal("已摘取过该鲜花 should be unavailable for steal_elves")
	}
	ordinary := &automation.PlannedOp{Kind: clientproto.RPCFrdStealSteal.String(), Action: "steal"}
	if isFriendStealElvesUnavailableError(ordinary, errors.New("server: 已摘取过该鲜花")) {
		t.Fatal("ordinary flower steal must not use elves unavailable path")
	}
}
