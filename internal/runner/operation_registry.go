package runner

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientrpc"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

type operationSpec struct {
	args func(*automation.PlannedOp) (any, error)
	run  func(context.Context, operationRuntime, *automation.PlannedOp) (json.RawMessage, error)
}

var plannedOperationSpecs = map[string]operationSpec{
	clientproto.RPCUsrLandHarvest.String(): {
		args: harvestOperationArgs,
		run:  runUsrLandHarvest,
	},
	clientproto.RPCUsrLandPlantBatch.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.UsrLandPlantBatchRequest, error) {
			if op.FlowerID == 0 {
				return clientproto.UsrLandPlantBatchRequest{}, fmt.Errorf("plantBatch missing flower id")
			}
			return clientproto.UsrLandPlantBatchRequest{LandIds: op.LandIDs, FlowerId: op.FlowerID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrLandPlantBatchRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrLand().PlantBatch(ctx, req)
		},
	),
	clientproto.RPCUsrLandPlant.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.UsrLandPlantRequest, error) {
			if op.FlowerID == 0 {
				return clientproto.UsrLandPlantRequest{}, fmt.Errorf("plant missing flower id")
			}
			landID, err := plannedOpSingleLandID(op)
			if err != nil {
				return clientproto.UsrLandPlantRequest{}, err
			}
			return clientproto.UsrLandPlantRequest{LandId: landID, FlowerId: op.FlowerID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrLandPlantRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrLand().Plant(ctx, req)
		},
	),
	clientproto.RPCUsrLandWaterBatch.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.UsrLandWaterBatchRequest, error) {
			return clientproto.UsrLandWaterBatchRequest{LandIds: op.LandIDs}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrLandWaterBatchRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrLand().WaterBatch(ctx, req)
		},
	),
	clientproto.RPCUsrLandWater.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.UsrLandWaterRequest, error) {
			landID, err := plannedOpSingleLandID(op)
			if err != nil {
				return clientproto.UsrLandWaterRequest{}, err
			}
			return clientproto.UsrLandWaterRequest{LandId: landID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrLandWaterRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrLand().Water(ctx, req)
		},
	),
	clientproto.RPCUsrLandUnlockLand.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.UsrLandUnlockLandRequest, error) {
			return clientproto.UsrLandUnlockLandRequest{LandId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrLandUnlockLandRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrLand().UnlockLand(ctx, req)
		},
	),
	clientproto.RPCUsrLandSpeedUpBatch.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.UsrLandSpeedUpBatchRequest, error) {
			return clientproto.UsrLandSpeedUpBatchRequest{LandIds: op.LandIDs}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrLandSpeedUpBatchRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrLand().SpeedUpBatch(ctx, req)
		},
	),
	clientproto.RPCCultivateRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.CultivateRecvRequest, error) {
			return clientproto.CultivateRecvRequest{FlowerId: op.FlowerID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.CultivateRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Cultivate().Recv(ctx, req)
		},
	),
	clientproto.RPCCultivateUpgrade.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.CultivateUpgradeRequest, error) {
			return clientproto.CultivateUpgradeRequest{FlowerId: op.FlowerID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.CultivateUpgradeRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Cultivate().Upgrade(ctx, req)
		},
	),
	clientproto.RPCCultivateCultivate.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.CultivateCultivateRequest, error) {
			return clientproto.CultivateCultivateRequest{FlowerId: op.FlowerID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.CultivateCultivateRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Cultivate().Cultivate(ctx, req)
		},
	),
	clientproto.RPCOrderFlowerFinishOrder.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.OrderFlowerFinishOrderRequest, error) {
			return clientproto.OrderFlowerFinishOrderRequest{BoxId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderFlowerFinishOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderFlower().FinishOrder(ctx, req)
		},
	),
	clientproto.RPCOrderFlowerFinishSatinOrder.String(): stateDeltaOperation(
		staticRequest(clientproto.OrderFlowerFinishSatinOrderRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderFlowerFinishSatinOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderFlower().FinishSatinOrder(ctx, req)
		},
	),
	clientproto.RPCOrderFlowerFinishDecorateOrder.String(): stateDeltaOperation(
		staticRequest(clientproto.OrderFlowerFinishDecorateOrderRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderFlowerFinishDecorateOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderFlower().FinishDecorateOrder(ctx, req)
		},
	),
	clientproto.RPCOrderFlowerRecvOrderRwd.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.OrderFlowerRecvOrderRwdRequest, error) {
			return clientproto.OrderFlowerRecvOrderRwdRequest{Target: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderFlowerRecvOrderRwdRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderFlower().RecvOrderRwd(ctx, req)
		},
	),
	clientproto.RPCOrderCustomerFinishOrder.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.OrderCustomerFinishOrderRequest, error) {
			return clientproto.OrderCustomerFinishOrderRequest{NPCId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderCustomerFinishOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderCustomer().FinishOrder(ctx, req)
		},
	),
	clientproto.RPCOrderCustomerGenOrder.String(): stateDeltaOperation(
		staticRequest(clientproto.OrderCustomerGenOrderRequest{GuestNpcIdList: clientproto.RPCIDList{}}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderCustomerGenOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderCustomer().GenOrder(ctx, req)
		},
	),
	clientproto.RPCOrderCustomerRejectOrder.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.OrderCustomerRejectOrderRequest, error) {
			return clientproto.OrderCustomerRejectOrderRequest{NPCId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderCustomerRejectOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderCustomer().RejectOrder(ctx, req)
		},
	),
	clientproto.RPCOrderPalaceEnter.String(): stateDeltaOperation(
		staticRequest(clientproto.OrderPalaceEnterRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderPalaceEnterRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderPalace().Enter(ctx, req)
		},
	),
	clientproto.RPCOrderPalaceFinishOrder.String(): stateDeltaOperation(
		staticRequest(clientproto.OrderPalaceFinishOrderRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderPalaceFinishOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderPalace().FinishOrder(ctx, req)
		},
	),
	clientproto.RPCOrderTeamRefreshOrder.String(): stateDeltaOperation(
		staticRequest(clientproto.OrderTeamRefreshOrderRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderTeamRefreshOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderTeam().RefreshOrder(ctx, req)
		},
	),
	clientproto.RPCOrderTeamSubmitOrder.String(): stateDeltaOperation(
		staticRequest(clientproto.OrderTeamSubmitOrderRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OrderTeamSubmitOrderRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.OrderTeam().SubmitOrder(ctx, req)
		},
	),
	clientproto.RPCFlowerArtMakeFlowerArt.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FlowerArtMakeFlowerArtRequest, error) {
			if op.VaseID <= 0 || len(op.FlowerIDs) == 0 || op.Count <= 0 {
				return clientproto.FlowerArtMakeFlowerArtRequest{}, fmt.Errorf("flower art craft missing vase/flowers/count")
			}
			return clientproto.FlowerArtMakeFlowerArtRequest{VaseId: op.VaseID, FlowersIds: op.FlowerIDs, Num: op.Count}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FlowerArtMakeFlowerArtRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FlowerArt().MakeFlowerArt(ctx, req)
		},
	),
	clientproto.RPCCollectRwdRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.CollectRwdRecvRequest, error) {
			return clientproto.CollectRwdRecvRequest{Type: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.CollectRwdRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.CollectRwd().Recv(ctx, req)
		},
	),
	clientproto.RPCCollectRwdRecvArtCreateRwdByVase.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.CollectRwdRecvArtCreateRwdByVaseRequest, error) {
			return clientproto.CollectRwdRecvArtCreateRwdByVaseRequest{"flowerArtId": op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.CollectRwdRecvArtCreateRwdByVaseRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.CollectRwd().RecvArtCreateRwdByVase(ctx, req)
		},
	),
	clientproto.RPCFlowerRackSell.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FlowerRackSellRequest, error) {
			return clientproto.FlowerRackSellRequest{RackId: op.TargetID, Iid: op.ItemID, Num: op.Count}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FlowerRackSellRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FlowerRack().Sell(ctx, req)
		},
	),
	clientproto.RPCFlowerRackCancelSell.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FlowerRackCancelSellRequest, error) {
			if op.TargetID <= 0 {
				return clientproto.FlowerRackCancelSellRequest{}, fmt.Errorf("flower rack cancel missing rackId")
			}
			return clientproto.FlowerRackCancelSellRequest{RackId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FlowerRackCancelSellRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FlowerRack().CancelSell(ctx, req)
		},
	),
	clientproto.RPCFlowerRackRecvSellMoney.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FlowerRackRecvSellMoneyRequest, error) {
			return clientproto.FlowerRackRecvSellMoneyRequest{RackId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FlowerRackRecvSellMoneyRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FlowerRack().RecvSellMoney(ctx, req)
		},
	),
	clientproto.RPCWaterwheelRecv.String(): {
		args: staticAnyRequest(clientproto.WaterwheelRecvRequest{}),
		run:  runWaterwheelRecv,
	},
	clientproto.RPCFreeWaterRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FreeWaterRecvRequest, error) {
			return clientproto.FreeWaterRecvRequest{Idx: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FreeWaterRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FreeWater().Recv(ctx, req)
		},
	),
	clientproto.RPCBenefitBoxDraw.String(): {
		args: staticAnyRequest(clientproto.BenefitBoxDrawRequest{}),
		run:  runBenefitBoxDraw,
	},
	clientproto.RPCZooEnterZoo.String(): stateDeltaOperation(
		staticRequest(clientproto.ZooEnterZooRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ZooEnterZooRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Zoo().EnterZoo(ctx, req)
		},
	),
	clientproto.RPCZooRefreshPetStatus.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.ZooRefreshPetStatusRequest, error) {
			if op.TargetID <= 0 {
				return clientproto.ZooRefreshPetStatusRequest{}, fmt.Errorf("refreshPetStatus missing pet id")
			}
			return clientproto.ZooRefreshPetStatusRequest{PetIdList: clientproto.RPCIDList{op.TargetID}}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ZooRefreshPetStatusRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Zoo().RefreshPetStatus(ctx, req)
		},
	),
	clientproto.RPCZooAddFoodstuff.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.ZooAddFoodstuffRequest, error) {
			if op.TargetID <= 0 {
				return clientproto.ZooAddFoodstuffRequest{}, fmt.Errorf("addFoodstuff missing pet id")
			}
			if op.ItemID != 1501 && op.ItemID != 1502 {
				return clientproto.ZooAddFoodstuffRequest{}, fmt.Errorf("addFoodstuff unsupported food id %d", op.ItemID)
			}
			if op.Count <= 0 {
				return clientproto.ZooAddFoodstuffRequest{}, fmt.Errorf("addFoodstuff invalid count %d", op.Count)
			}
			if len(op.ItemCost) != 1 || op.ItemCost[op.ItemID] != op.Count {
				return clientproto.ZooAddFoodstuffRequest{}, fmt.Errorf("addFoodstuff requires exact item cost %d:%d", op.ItemID, op.Count)
			}
			if plannedOpHasCyclicNoteTargets(op) {
				return clientproto.ZooAddFoodstuffRequest{}, fmt.Errorf("addFoodstuff carries unexpected activity targets")
			}
			foodstuffIDs := make(clientproto.RPCIDList, op.Count)
			for i := range foodstuffIDs {
				foodstuffIDs[i] = op.ItemID
			}
			return clientproto.ZooAddFoodstuffRequest{PetId: op.TargetID, FoodstuffIds: foodstuffIDs}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ZooAddFoodstuffRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Zoo().AddFoodstuff(ctx, req)
		},
	),
	clientproto.RPCZooStrokePet.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.ZooStrokePetRequest, error) {
			if op.TargetID <= 0 {
				return clientproto.ZooStrokePetRequest{}, fmt.Errorf("strokePet missing pet id")
			}
			return clientproto.ZooStrokePetRequest{PetId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ZooStrokePetRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Zoo().StrokePet(ctx, req)
		},
	),
	clientproto.RPCZooHandleEvent.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return zooHandleEventRequest(op) },
		run:  runZooHandleEvent,
	},
	clientproto.RPCZooReadLog.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return zooReadLogRequest(op) },
		run:  runZooReadLog,
	},
	clientproto.RPCZooRecvSouvenirRwd.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return zooRecvSouvenirRewardRequest(op) },
		run:  runZooRecvSouvenirReward,
	},
	clientproto.RPCZooReadSouvenir.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return zooReadSouvenirRequest(op) },
		run:  runZooReadSouvenir,
	},
	clientproto.RPCUsrExtraUpdateAntiFraudQAStatus.String(): stateDeltaOperation(
		staticRequest(clientproto.UsrExtraUpdateAntiFraudQAStatusRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrExtraUpdateAntiFraudQAStatusRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrExtra().UpdateAntiFraudQAStatus(ctx, req)
		},
	),
	clientproto.RPCUsrExtraRecvAntiFraudQARwd.String(): stateDeltaOperation(
		staticRequest(clientproto.UsrExtraRecvAntiFraudQARwdRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.UsrExtraRecvAntiFraudQARwdRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.UsrExtra().RecvAntiFraudQARwd(ctx, req)
		},
	),
	clientproto.RPCShopEnter.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.ShopEnterRequest, error) {
			if op == nil || op.TargetID != state.ZooFoodShopTempID || op.ItemID != 0 || op.Count != 0 || op.GoldCost != 0 || op.DiamondCost != 0 || len(op.ItemCost) != 0 {
				return clientproto.ShopEnterRequest{}, fmt.Errorf("shop.enter automation only supports cost-free zoo food shop %d sync", state.ZooFoodShopTempID)
			}
			return clientproto.ShopEnterRequest{TempId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ShopEnterRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Shop().Enter(ctx, req)
		},
	),
	clientproto.RPCShopBuy.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.ShopBuyRequest, error) {
			if op == nil || op.TargetID != state.ZooFoodShopTempID || op.ItemID != state.ZooNormalFoodShopItemID || op.Count <= 0 {
				return clientproto.ShopBuyRequest{}, fmt.Errorf("shop.buy automation only supports positive-count normal zoo food")
			}
			return clientproto.ShopBuyRequest{TempId: op.TargetID, ItemId: op.ItemID, Count: op.Count}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ShopBuyRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Shop().Buy(ctx, req)
		},
	),
	clientproto.RPCShopCultivateEnter.String(): stateDeltaOperation(
		staticRequest(clientproto.ShopCultivateEnterRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ShopCultivateEnterRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.ShopCultivate().Enter(ctx, req)
		},
	),
	clientproto.RPCShopCultivateRefresh.String(): stateDeltaOperation(
		staticRequest(clientproto.ShopCultivateRefreshRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ShopCultivateRefreshRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.ShopCultivate().Refresh(ctx, req)
		},
	),
	clientproto.RPCShopCultivateBuy.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.ShopCultivateBuyRequest, error) {
			return clientproto.ShopCultivateBuyRequest{ShopId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ShopCultivateBuyRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.ShopCultivate().Buy(ctx, req)
		},
	),
	clientproto.RPCShopGiftbagEnter.String(): stateDeltaOperation(
		staticRequest(clientproto.ShopGiftbagEnterRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ShopGiftbagEnterRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.ShopGiftbag().Enter(ctx, req)
		},
	),
	clientproto.RPCShopGiftbagBuy.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.ShopGiftbagBuyRequest, error) {
			return clientproto.ShopGiftbagBuyRequest{ShopId: op.TargetID, Num: op.Count}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.ShopGiftbagBuyRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.ShopGiftbag().Buy(ctx, req)
		},
	),
	clientproto.RPCPearlRefresh.String(): stateDeltaOperation(
		staticRequest(clientproto.PearlRefreshRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.PearlRefreshRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Pearl().Refresh(ctx, req)
		},
	),
	clientproto.RPCFrdEnter.String(): stateDeltaOperation(
		pearlFriendSyncRequest,
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FrdEnterRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Frd().Enter(ctx, req)
		},
	),
	clientproto.RPCFrdExtGetFrdOtherInfoByUids.String(): stateDeltaOperation(
		friendTouchOtherInfoRequest,
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FrdExtGetFrdOtherInfoByUidsRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FrdExt().GetFrdOtherInfoByUids(ctx, req)
		},
	),
	clientproto.RPCFrdExtBuyStealCnt.String(): stateDeltaOperation(
		friendTouchBuyRequest,
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FrdExtBuyStealCntRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FrdExt().BuyStealCnt(ctx, req)
		},
	),
	clientproto.RPCFrdStealEnterFrdSteal.String(): {
		args: friendTouchVerificationArgs,
		run:  runFriendTouchVerification,
	},
	clientproto.RPCFrdHomeGetFrdHomeInfo.String(): stateDeltaOperation(
		friendTouchGardenRequest,
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FrdHomeGetFrdHomeInfoRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FrdHome().GetFrdHomeInfo(ctx, req)
		},
	),
	clientproto.RPCFrdStealSteal.String(): stateDeltaOperation(
		friendTouchStealRequest,
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FrdStealStealRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FrdSteal().Steal(ctx, req)
		},
	),
	clientproto.RPCOpptGetDetailOppts.String(): stateDeltaOperation(
		pearlCandidateDetailRequest,
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.OpptGetDetailOpptsRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Oppt().GetDetailOppts(ctx, req)
		},
	),
	clientproto.RPCPearlGetHireStateByUids.String(): stateDeltaOperation(
		pearlCandidateHireStateRequest,
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.PearlGetHireStateByUidsRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Pearl().GetHireStateByUids(ctx, req)
		},
	),
	clientproto.RPCPearlGetRecommendList.String(): stateDeltaOperation(
		pearlRecommendRequest,
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.PearlGetRecommendListRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Pearl().GetRecommendList(ctx, req)
		},
	),
	clientproto.RPCPearlPlaceHire.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return pearlHireRequest(op) },
		run:  runPearlHire,
	},
	clientproto.RPCPearlRecvDailyFree.String(): stateDeltaOperation(
		staticRequest(clientproto.PearlRecvDailyFreeRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.PearlRecvDailyFreeRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Pearl().RecvDailyFree(ctx, req)
		},
	),
	clientproto.RPCPearlPlaceRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.PearlPlaceRecvRequest, error) {
			return clientproto.PearlPlaceRecvRequest{PlaceId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.PearlPlaceRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.PearlPlace().Recv(ctx, req)
		},
	),
	clientproto.RPCPearlPlaceRecvOneKey.String(): {
		args: func(op *automation.PlannedOp) (any, error) {
			return pearlRecvOneKeyRequest(op)
		},
		run: runPearlRecvOneKey,
	},
	clientproto.RPCPearlSetProtectState.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.PearlSetProtectStateRequest, error) {
			return clientproto.PearlSetProtectStateRequest{ProtectState: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.PearlSetProtectStateRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Pearl().SetProtectState(ctx, req)
		},
	),
	clientproto.RPCPearlDraw.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.PearlDrawRequest, error) {
			return clientproto.PearlDrawRequest{Count: op.Count}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.PearlDrawRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Pearl().Draw(ctx, req)
		},
	),
	clientproto.RPCFmlBld.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlBldRequest, error) {
			if op.TargetID <= 0 {
				return clientproto.FmlBldRequest{}, fmt.Errorf("fml.bld missing build option id")
			}
			return clientproto.FmlBldRequest{ID: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlBldRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Fml().Bld(ctx, req)
		},
	),
	clientproto.RPCFmlEnter.String(): {
		args: func(_ *automation.PlannedOp) (any, error) {
			return fmlEnterSyncRequest(), nil
		},
		run: runFmlEnter,
	},
	clientproto.RPCFmlLandHarvest.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlLandHarvestRequest, error) {
			return clientproto.FmlLandHarvestRequest{LandIds: op.LandIDs}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlLandHarvestRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlLand().Harvest(ctx, req)
		},
	),
	clientproto.RPCFmlLandPlant.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlLandPlantRequest, error) {
			if op.FlowerID <= 0 {
				return clientproto.FmlLandPlantRequest{}, fmt.Errorf("fmlLand.plant missing flower id")
			}
			return clientproto.FmlLandPlantRequest{LandIds: op.LandIDs, FlwId: op.FlowerID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlLandPlantRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlLand().Plant(ctx, req)
		},
	),
	clientproto.RPCFmlForestRefresh.String(): {
		args: func(op *automation.PlannedOp) (any, error) {
			return fmlForestRefreshRequest(op), nil
		},
		run: runFmlForestRefresh,
	},
	clientproto.RPCFmlFlowerShareRefresh.String(): stateDeltaOperation(
		staticRequest(clientproto.FmlFlowerShareRefreshRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlFlowerShareRefreshRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlFlowerShare().Refresh(ctx, req)
		},
	),
	clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String(): stateDeltaOperation(
		staticRequest(clientproto.FmlFlowerShareGetFmlOtherShareListRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlFlowerShareGetFmlOtherShareListRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlFlowerShare().GetFmlOtherShareList(ctx, req)
		},
	),
	clientproto.RPCFmlFlowerShareRecvRwd.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlFlowerShareRecvRwdRequest, error) {
			return clientproto.FmlFlowerShareRecvRwdRequest{SlotIds: op.SlotIDs}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlFlowerShareRecvRwdRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlFlowerShare().RecvRwd(ctx, req)
		},
	),
	clientproto.RPCFmlFlowerShareTake.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlFlowerShareTakeRequest, error) {
			return clientproto.FmlFlowerShareTakeRequest{DstUid: op.TargetUID, SlotId: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlFlowerShareTakeRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlFlowerShare().Take(ctx, req)
		},
	),
	// Guild race operations.
	clientproto.RPCFmlRaceTakeTask.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlRaceTakeTaskRequest, error) {
			return clientproto.FmlRaceTakeTaskRequest{TaskMsId: op.TaskMsID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlRaceTakeTaskRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlRace().TakeTask(ctx, req)
		},
	),
	clientproto.RPCFmlRaceFinishTask.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlRaceFinishTaskRequest, error) {
			return clientproto.FmlRaceFinishTaskRequest{TaskMsId: op.TaskMsID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlRaceFinishTaskRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlRace().FinishTask(ctx, req)
		},
	),
	clientproto.RPCFmlRaceDelTask.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlRaceDelTaskRequest, error) {
			return clientproto.FmlRaceDelTaskRequest{TaskMsId: op.TaskMsID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlRaceDelTaskRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlRace().DelTask(ctx, req)
		},
	),
	clientproto.RPCFmlRaceUpgradeTask.String(): {
		args: func(op *automation.PlannedOp) (any, error) {
			return clientproto.FmlRaceUpgradeTaskRequest{}, nil
		},
		run: runFmlRaceUpgrade,
	},
	clientproto.RPCFmlRaceEnter.String(): {
		args: func(op *automation.PlannedOp) (any, error) {
			return clientproto.FmlRaceEnterRequest{}, nil
		},
		run: runFmlRaceEnter,
	},
	clientproto.RPCFmlRaceGetTaskList.String(): {
		args: func(op *automation.PlannedOp) (any, error) {
			return clientproto.FmlRaceGetTaskListRequest{}, nil
		},
		run: runFmlRaceGetTaskList,
	},
	clientproto.RPCFmlRaceGetFmlRaceUsrRankList.String(): {
		args: func(op *automation.PlannedOp) (any, error) {
			if op.TaskMsID <= 0 {
				return nil, fmt.Errorf("fmlRace.getFmlRaceUsrRankList requires batchId")
			}
			// Generated request uses int32 RPCID; race batchId is a ms timestamp.
			return map[string]any{"batchId": op.TaskMsID}, nil
		},
		run: runFmlRaceGetUsrRankList,
	},
	clientproto.RPCFmlRaceGiveUpTask.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.FmlRaceGiveUpTaskRequest, error) {
			return clientproto.FmlRaceGiveUpTaskRequest{}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.FmlRaceGiveUpTaskRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.FmlRace().GiveUpTask(ctx, req)
		},
	),
	clientproto.RPCTaskMainRecv.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return mainTaskClaimRequest(op) },
		run:  runMainTaskClaim,
	},
	clientproto.RPCActCyclicNoteEnter.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return cyclicNoteEnterRequest(op) },
		run:  runCyclicNoteEnter,
	},
	clientproto.RPCActCyclicNoteRecvTaskRwd.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return cyclicNoteTaskClaimRequest(op) },
		run:  runCyclicNoteTaskClaim,
	},
	clientproto.RPCActCyclicNoteRecv.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return cyclicNoteMilestoneClaimRequest(op) },
		run:  runCyclicNoteMilestoneClaim,
	},
	clientproto.RPCActCyclicStoryEnter.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return cyclicStoryEnterRequest(op) },
		run:  runCyclicStoryEnter,
	},
	clientproto.RPCActCyclicStoryRecvOrderRwd.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return cyclicStoryOrderClaimRequest(op) },
		run:  runCyclicStoryOrderClaim,
	},
	clientproto.RPCActCyclicStoryRecv.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return cyclicStoryMilestoneClaimRequest(op) },
		run:  runCyclicStoryMilestoneClaim,
	},
	clientproto.RPCTaskDlyRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.TaskDlyRecvRequest, error) {
			return clientproto.TaskDlyRecvRequest{ID: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.TaskDlyRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.TaskDly().Recv(ctx, req)
		},
	),
	clientproto.RPCTaskWeekRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.TaskWeekRecvRequest, error) {
			return clientproto.TaskWeekRecvRequest{ID: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.TaskWeekRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.TaskWeek().Recv(ctx, req)
		},
	),
	clientproto.RPCStoryMainEnter.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return storyEnterRequest(op) },
		run:  runStoryEnter,
	},
	clientproto.RPCStoryMainUnlock.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return storyUnlockRequest(op) },
		run:  runStoryUnlock,
	},
	clientproto.RPCTaskAchRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.TaskAchRecvRequest, error) {
			return clientproto.TaskAchRecvRequest{ID: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.TaskAchRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.TaskAch().Recv(ctx, req)
		},
	),
	clientproto.RPCRoadGrowRecv.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.RoadGrowRecvRequest, error) {
			return clientproto.RoadGrowRecvRequest{ID: op.TargetID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.RoadGrowRecvRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.RoadGrow().Recv(ctx, req)
		},
	),
	clientproto.RPCRandomEventEnter.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return randomEventEnterRequest(op) },
		run:  runRandomEventEnter,
	},
	clientproto.RPCRandomEventDoAffair.String(): {
		args: func(op *automation.PlannedOp) (any, error) { return randomEventClaimRequest(op) },
		run:  runRandomEventClaim,
	},
	clientproto.RPCMailGetList.String(): stateDeltaOperation(
		staticRequest(clientproto.MailGetListRequest{}),
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.MailGetListRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Mail().GetList(ctx, req)
		},
	),
	clientproto.RPCMailPick.String(): stateDeltaOperation(
		func(op *automation.PlannedOp) (clientproto.MailPickRequest, error) {
			return clientproto.MailPickRequest{MsId: int64(op.TargetID), AllId: op.ItemID}, nil
		},
		func(ctx context.Context, rpc *clientrpc.Client, req clientproto.MailPickRequest) (babigame.RPCResponse[clientproto.StateDelta], error) {
			return rpc.Mail().Pick(ctx, req)
		},
	),
	clientproto.RPCSignTypeEnter.String(): {
		args: func(op *automation.PlannedOp) (any, error) {
			typeID, err := plannedSignTypeID(op)
			return clientproto.SignTypeEnterRequest{Type: typeID}, err
		},
		run: runSignTypeEnter,
	},
	clientproto.RPCSignTypeSign.String(): {
		args: func(op *automation.PlannedOp) (any, error) {
			typeID, err := plannedSignTypeID(op)
			return clientproto.SignTypeSignRequest{Type: typeID}, err
		},
		run: runSignTypeSign,
	},
	clientproto.RPCSignTypeRecv.String(): {
		args: func(op *automation.PlannedOp) (any, error) {
			typeID, err := plannedSignTypeID(op)
			return clientproto.SignTypeRecvRequest{Type: typeID}, err
		},
		run: runSignTypeRecv,
	},
}

func operationSpecFor(kind string) (operationSpec, bool) {
	spec, ok := plannedOperationSpecs[kind]
	return spec, ok
}

func staticRequest[Req any](req Req) func(*automation.PlannedOp) (Req, error) {
	return func(*automation.PlannedOp) (Req, error) {
		return req, nil
	}
}

func staticAnyRequest(req any) func(*automation.PlannedOp) (any, error) {
	return func(*automation.PlannedOp) (any, error) {
		return req, nil
	}
}

func harvestRequests(op *automation.PlannedOp) ([]clientproto.UsrLandHarvestRequest, error) {
	if op == nil || len(op.LandIDs) == 0 {
		return nil, fmt.Errorf("operation %s requires at least one land id", clientproto.RPCUsrLandHarvest.String())
	}
	reqs := make([]clientproto.UsrLandHarvestRequest, 0, len(op.LandIDs))
	for _, landID := range op.LandIDs {
		if landID == 0 {
			return nil, fmt.Errorf("operation %s has empty land id", clientproto.RPCUsrLandHarvest.String())
		}
		reqs = append(reqs, clientproto.UsrLandHarvestRequest{LandId: landID})
	}
	return reqs, nil
}

func harvestOperationArgs(op *automation.PlannedOp) (any, error) {
	reqs, err := harvestRequests(op)
	if err != nil {
		return nil, err
	}
	if len(reqs) == 1 {
		return reqs[0], nil
	}
	return reqs, nil
}

func operationArgs(op *automation.PlannedOp) (any, error) {
	if op == nil {
		return nil, fmt.Errorf("nil planned operation")
	}
	spec, ok := operationSpecFor(op.Kind)
	if !ok {
		return nil, fmt.Errorf("unsupported planned operation %s", op.Kind)
	}
	return spec.args(op)
}

func plannedOpSingleLandID(op *automation.PlannedOp) (int32, error) {
	if op == nil || len(op.LandIDs) != 1 || op.LandIDs[0] == 0 {
		return 0, fmt.Errorf("operation %s requires exactly one land id", op.Kind)
	}
	return op.LandIDs[0], nil
}
