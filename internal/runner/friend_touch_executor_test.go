package runner

import (
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestFriendTouchRequestsMatchMiniClientFlow(t *testing.T) {
	verification, err := friendTouchVerificationArgs(&automation.PlannedOp{
		Kind: clientproto.RPCFrdStealEnterFrdSteal.String(),
	})
	if err != nil || jsonString(t, verification) != `{"point":[22,"\u003cdevice-fingerprint\u003e"]}` {
		t.Fatalf("verification=%s err=%v", jsonString(t, verification), err)
	}

	garden, err := friendTouchGardenRequest(&automation.PlannedOp{
		Kind:      clientproto.RPCFrdHomeGetFrdHomeInfo.String(),
		TargetUID: 2001,
	})
	if err != nil || jsonString(t, garden) != `{"frdUid":2001}` {
		t.Fatalf("garden=%s err=%v", jsonString(t, garden), err)
	}

	steal, err := friendTouchStealRequest(&automation.PlannedOp{
		Kind:      clientproto.RPCFrdStealSteal.String(),
		TargetUID: 2001,
		TargetID:  11,
		Count:     1,
	})
	if err != nil || jsonString(t, steal) != `{"frdUid":2001,"landId":11}` {
		t.Fatalf("steal=%s err=%v", jsonString(t, steal), err)
	}
}

func TestFriendTouchVerificationRejectsFriendTarget(t *testing.T) {
	op := &automation.PlannedOp{
		Kind:      clientproto.RPCFrdStealEnterFrdSteal.String(),
		TargetUID: 2001,
	}
	if _, err := friendTouchVerificationArgs(op); err == nil {
		t.Fatal("enterFrdSteal verification accepted a friend uid")
	}
}

func TestFriendTouchOperationRegistryUsesSeparateVerificationAndGardenRPCs(t *testing.T) {
	for _, rpc := range []clientproto.RPCName{
		clientproto.RPCFrdStealEnterFrdSteal,
		clientproto.RPCFrdHomeGetFrdHomeInfo,
		clientproto.RPCFrdStealSteal,
	} {
		spec, ok := operationSpecFor(rpc.String())
		if !ok || spec.args == nil || spec.run == nil {
			t.Fatalf("friend-touch RPC %s is not fully registered", rpc)
		}
	}
}
