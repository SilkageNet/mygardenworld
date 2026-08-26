package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func friendTouchOtherInfoRequest(op *automation.PlannedOp) (clientproto.FrdExtGetFrdOtherInfoByUidsRequest, error) {
	if err := validateFriendTouchSyncOperation(op, true); err != nil {
		return clientproto.FrdExtGetFrdOtherInfoByUidsRequest{}, err
	}
	return clientproto.FrdExtGetFrdOtherInfoByUidsRequest{
		UIDs:  append(clientproto.RPCUIDList(nil), op.TargetUIDs...),
		Steal: 1,
	}, nil
}

func friendTouchBuyRequest(op *automation.PlannedOp) (clientproto.FrdExtBuyStealCntRequest, error) {
	if op == nil || op.TargetUID <= 0 || op.Count != 1 {
		return clientproto.FrdExtBuyStealCntRequest{}, fmt.Errorf("frdExt.buyStealCnt requires frdUid and buyCnt=1")
	}
	if len(op.TargetUIDs) != 0 || op.TargetID != 0 || op.FlowerID != 0 || op.ItemID != 0 ||
		len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0 ||
		plannedOpHasCyclicNoteTargets(op) {
		return clientproto.FrdExtBuyStealCntRequest{}, fmt.Errorf("frdExt.buyStealCnt carries unexpected fields")
	}
	if op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 1 {
		return clientproto.FrdExtBuyStealCntRequest{}, fmt.Errorf("frdExt.buyStealCnt requires ItemCost only")
	}
	return clientproto.FrdExtBuyStealCntRequest{FrdUid: op.TargetUID, BuyCnt: op.Count}, nil
}

func friendTouchVerificationArgs(op *automation.PlannedOp) (any, error) {
	if err := validateFriendTouchVerificationOperation(op); err != nil {
		return nil, err
	}
	// Do not put the account-bound fingerprint into operation logs.
	return map[string]any{"point": []any{int32(rqstPointFriendSteal), "<device-fingerprint>"}}, nil
}

func validateFriendTouchVerificationOperation(op *automation.PlannedOp) error {
	if op == nil {
		return fmt.Errorf("frdSteal.enterFrdSteal verification operation is nil")
	}
	if op.Count != 0 || op.TargetID != 0 || op.TargetUID != 0 || op.ItemID != 0 || op.FlowerID != 0 || op.VaseID != 0 ||
		len(op.TargetUIDs) != 0 || len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0 ||
		plannedOpHasCyclicNoteTargets(op) || op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
		return fmt.Errorf("frdSteal.enterFrdSteal verification carries unexpected fields")
	}
	return nil
}

func runFriendTouchVerification(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	if err := validateFriendTouchVerificationOperation(op); err != nil {
		return nil, err
	}
	if rt.runner == nil {
		return nil, fmt.Errorf("frdSteal.enterFrdSteal runner unavailable")
	}
	rt.runner.mu.RLock()
	session := rt.runner.session
	rt.runner.mu.RUnlock()
	if session == nil {
		return nil, fmt.Errorf("frdSteal.enterFrdSteal session unavailable")
	}
	req := clientproto.FrdStealEnterFrdStealRequest{
		"point": []any{int32(rqstPointFriendSteal), buildDeviceFingerprint(session.DeviceID)},
	}
	raw, err := checkedStateDelta(rt.rpc.FrdSteal().EnterFrdSteal(ctx, req))
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func friendTouchGardenRequest(op *automation.PlannedOp) (clientproto.FrdHomeGetFrdHomeInfoRequest, error) {
	if op == nil || op.TargetUID <= 0 {
		return clientproto.FrdHomeGetFrdHomeInfoRequest{}, fmt.Errorf("frdHome.getFrdHomeInfo requires frdUid")
	}
	if op.Count != 0 || op.TargetID != 0 || op.ItemID != 0 || op.FlowerID != 0 || op.VaseID != 0 ||
		len(op.TargetUIDs) != 0 || len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0 ||
		plannedOpHasCyclicNoteTargets(op) || op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
		return clientproto.FrdHomeGetFrdHomeInfoRequest{}, fmt.Errorf("frdHome.getFrdHomeInfo carries unexpected fields")
	}
	return clientproto.FrdHomeGetFrdHomeInfoRequest{FrdUid: op.TargetUID}, nil
}

func friendTouchStealRequest(op *automation.PlannedOp) (clientproto.FrdStealStealRequest, error) {
	if op == nil || op.TargetUID <= 0 || op.TargetID <= 0 || op.Count != 1 {
		return clientproto.FrdStealStealRequest{}, fmt.Errorf("frdSteal.steal requires frdUid, landId and count=1")
	}
	if len(op.TargetUIDs) != 0 || op.ItemID != 0 || op.FlowerID != 0 || op.VaseID != 0 || op.SlotID != 0 ||
		len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0 ||
		plannedOpHasCyclicNoteTargets(op) || op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
		return clientproto.FrdStealStealRequest{}, fmt.Errorf("frdSteal.steal carries unexpected fields")
	}
	return clientproto.FrdStealStealRequest{
		FrdUid:     op.TargetUID,
		LandId:     op.TargetID,
		StealElves: 0,
	}, nil
}

func validateFriendTouchSyncOperation(op *automation.PlannedOp, requireUIDs bool) error {
	if op == nil {
		return fmt.Errorf("friend touch sync operation is nil")
	}
	if op.TargetID != 0 || op.TargetUID != 0 || op.ItemID != 0 || op.Count != 0 ||
		op.FlowerID != 0 || op.VaseID != 0 || len(op.LandIDs) != 0 || len(op.SlotIDs) != 0 || len(op.FlowerIDs) != 0 ||
		plannedOpHasCyclicNoteTargets(op) {
		return fmt.Errorf("friend touch sync carries unexpected target fields")
	}
	if op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
		return fmt.Errorf("friend touch sync must be cost-free")
	}
	if requireUIDs && len(op.TargetUIDs) == 0 {
		return fmt.Errorf("friend touch sync requires at least one UID")
	}
	if !requireUIDs && len(op.TargetUIDs) != 0 {
		return fmt.Errorf("friend touch sync requires an empty UID list")
	}
	seen := make(map[int64]struct{}, len(op.TargetUIDs))
	for _, uid := range op.TargetUIDs {
		if uid <= 0 {
			return fmt.Errorf("friend touch sync contains an invalid UID")
		}
		if _, exists := seen[uid]; exists {
			return fmt.Errorf("friend touch sync contains a duplicate UID")
		}
		seen[uid] = struct{}{}
	}
	return nil
}
