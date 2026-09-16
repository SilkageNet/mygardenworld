package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func zooAddFoodstuffRequest(op *automation.PlannedOp) (clientproto.ZooAddFoodstuffRequest, error) {
	if op == nil || op.TargetID <= 0 {
		return clientproto.ZooAddFoodstuffRequest{}, fmt.Errorf("addFoodstuff missing pet id")
	}
	if op.ItemID != 1501 && op.ItemID != 1502 {
		return clientproto.ZooAddFoodstuffRequest{}, fmt.Errorf("addFoodstuff unsupported food id %d", op.ItemID)
	}
	if op.Count <= 0 || op.Count > state.ZooFoodBowlCapacity() {
		return clientproto.ZooAddFoodstuffRequest{}, fmt.Errorf("addFoodstuff invalid count %d", op.Count)
	}
	if len(op.ItemCost) != 1 || op.ItemCost[op.ItemID] != op.Count || op.GoldCost != 0 || op.DiamondCost != 0 {
		return clientproto.ZooAddFoodstuffRequest{}, fmt.Errorf("addFoodstuff requires exact item cost %d:%d only", op.ItemID, op.Count)
	}
	if plannedOpHasCyclicNoteTargets(op) {
		return clientproto.ZooAddFoodstuffRequest{}, fmt.Errorf("addFoodstuff carries unexpected activity targets")
	}
	ids := make(clientproto.RPCIDList, op.Count)
	for i := range ids {
		ids[i] = op.ItemID
	}
	return clientproto.ZooAddFoodstuffRequest{PetId: op.TargetID, FoodstuffIds: ids}, nil
}

// A 301 rejects this quantity, not necessarily every remaining unit. Conservatively
// invalidate its usable local balance, then refresh the bowl on the SAME session.
// enterZoo is not an inventory sync: only namespace-7 observations restore stock.
// Never retry a paid mutation here; let the normal planner select stock or a
// budgeted ordinary-food purchase on its next turn.
type zooFoodRejectedError struct {
	cause                                             error
	PetID, ItemID, Requested, StockBefore, BowlBefore int32
	RefreshError                                      string
}

func (e *zooFoodRejectedError) Error() string { return e.cause.Error() }
func (e *zooFoodRejectedError) Unwrap() error { return e.cause }

func runZooAddFoodstuff(ctx context.Context, rt operationRuntime, op *automation.PlannedOp) (json.RawMessage, error) {
	if rt.runner == nil || rt.rpc == nil {
		return nil, fmt.Errorf("addFoodstuff runtime unavailable")
	}
	if err := automation.ValidateZooFoodStock(rt.runner.state, rt.runner.Policy().GetBasic().GetZoo(), op); err != nil {
		return nil, err
	}
	return executeZooFoodStock(ctx, rt.runner.state, op,
		func(ctx context.Context, req clientproto.ZooAddFoodstuffRequest) (json.RawMessage, error) {
			return checkedStateDelta(rt.rpc.Zoo().AddFoodstuff(ctx, req))
		},
		func(ctx context.Context) error {
			_, err := checkedStateDelta(rt.rpc.Zoo().EnterZoo(ctx, clientproto.ZooEnterZooRequest{}))
			return err
		})
}

func executeZooFoodStock(ctx context.Context, st *state.State, op *automation.PlannedOp,
	stock func(context.Context, clientproto.ZooAddFoodstuffRequest) (json.RawMessage, error),
	refresh func(context.Context) error,
) (json.RawMessage, error) {
	req, err := zooAddFoodstuffRequest(op)
	if err != nil {
		return nil, err
	}
	if st == nil || stock == nil || refresh == nil {
		return nil, fmt.Errorf("addFoodstuff execution incomplete")
	}
	before := st.Inventory()[op.ItemID]
	bowl := int32(len(st.ZooPets()[op.TargetID].FoodstuffIDs))
	raw, err := stock(ctx, req)
	if err == nil || resourceRejectedItemID(err) != op.ItemID {
		return raw, err
	}
	rejected := &zooFoodRejectedError{cause: err, PetID: op.TargetID, ItemID: op.ItemID, Requested: op.Count, StockBefore: before, BowlBefore: bowl}
	st.RejectZooFoodStock(op.ItemID)
	st.InvalidateZooFoodBowl(op.TargetID)
	if refreshErr := refresh(ctx); refreshErr != nil {
		rejected.RefreshError = refreshErr.Error()
	} else if pet, exists := st.ZooPets()[op.TargetID]; exists && !pet.FoodstuffObserved {
		rejected.RefreshError = "响应未包含目标食盆，继续等待同步"
	}
	return nil, rejected
}
