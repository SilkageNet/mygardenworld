package runner

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestWaterResponseIncludesDrops(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want bool
	}{
		{
			name: "water batch current total",
			raw:  json.RawMessage(`{"7":{"2":{"1":{"7":8},"2":{"7":5}}}}`),
			want: true,
		},
		{
			name: "cold snapshot inventory",
			raw:  json.RawMessage(`{"7":{"0":{"32":{"7":12}}}}`),
			want: true,
		},
		{
			name: "spend count only is not remaining drops",
			raw:  json.RawMessage(`{"7":{"2":{"1":{"7":8}}}}`),
			want: false,
		},
		{
			name: "no water namespace",
			raw:  json.RawMessage(`{"100":{"1":{"1001":{"1":3}}}}`),
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := waterResponseIncludesDrops(tc.raw); got != tc.want {
				t.Fatalf("waterResponseIncludesDrops()=%t, want %t", got, tc.want)
			}
		})
	}
}

func TestWaterBatchUsesWaterOperationPath(t *testing.T) {
	if !isWaterOp(clientproto.RPCUsrLandWaterBatch.String()) {
		t.Fatal("waterBatch should share water verification/reservation path")
	}
	if isWaterOp(clientproto.RPCUsrLandWaterOneKey.String()) {
		t.Fatal("waterOneKey should not be part of the automation water path")
	}
	args, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCUsrLandWaterBatch.String(), LandIDs: []int32{1001, 1002}})
	if err != nil {
		t.Fatalf("operationArgs(waterBatch): %v", err)
	}
	waterBatch, ok := args.(clientproto.UsrLandWaterBatchRequest)
	if !ok {
		t.Fatalf("operationArgs(waterBatch)=%T, want UsrLandWaterBatchRequest", args)
	}
	if len(waterBatch.LandIds) != 2 || waterBatch.LandIds[0] != 1001 || waterBatch.LandIds[1] != 1002 {
		t.Fatalf("UsrLandWaterBatchRequest.LandIds=%v, want [1001 1002]", waterBatch.LandIds)
	}
}

func TestHarvestOperationArgsAllowsBatch(t *testing.T) {
	args, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCUsrLandHarvest.String(), LandIDs: []int32{1001, 1002}})
	if err != nil {
		t.Fatalf("operationArgs(harvest batch): %v", err)
	}
	reqs, ok := args.([]clientproto.UsrLandHarvestRequest)
	if !ok {
		t.Fatalf("operationArgs(harvest batch)=%T, want []UsrLandHarvestRequest", args)
	}
	if len(reqs) != 2 || reqs[0].LandId != 1001 || reqs[1].LandId != 1002 {
		t.Fatalf("harvest requests=%+v, want land IDs [1001 1002]", reqs)
	}

	args, err = operationArgs(&automation.PlannedOp{Kind: clientproto.RPCUsrLandHarvest.String(), LandIDs: []int32{1003}})
	if err != nil {
		t.Fatalf("operationArgs(harvest single): %v", err)
	}
	req, ok := args.(clientproto.UsrLandHarvestRequest)
	if !ok || req.LandId != 1003 {
		t.Fatalf("operationArgs(harvest single)=%T %+v, want landId 1003", args, args)
	}
}

func TestCollectRewardOperationArgs(t *testing.T) {
	args, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCCollectRwdRecv.String(), TargetID: 11})
	if err != nil {
		t.Fatalf("operationArgs(collectRwd.recv): %v", err)
	}
	recv, ok := args.(clientproto.CollectRwdRecvRequest)
	if !ok {
		t.Fatalf("operationArgs(collectRwd.recv)=%T, want CollectRwdRecvRequest", args)
	}
	if recv.Type != 11 {
		t.Fatalf("CollectRwdRecvRequest.Type=%d, want 11", recv.Type)
	}

	args, err = operationArgs(&automation.PlannedOp{Kind: clientproto.RPCCollectRwdRecvArtCreateRwdByVase.String(), TargetID: 3001})
	if err != nil {
		t.Fatalf("operationArgs(collectRwd.recvArtCreateRwdByVase): %v", err)
	}
	byVase, ok := args.(clientproto.CollectRwdRecvArtCreateRwdByVaseRequest)
	if !ok {
		t.Fatalf("operationArgs(collectRwd.recvArtCreateRwdByVase)=%T, want CollectRwdRecvArtCreateRwdByVaseRequest", args)
	}
	if byVase["flowerArtId"] != int32(3001) {
		t.Fatalf("flowerArtId=%v, want 3001", byVase["flowerArtId"])
	}
}

func TestOrderPalaceAndTeamOperationSpecs(t *testing.T) {
	cases := []struct {
		name string
		op   automation.PlannedOp
		want any
	}{
		{name: "palace enter", op: automation.PlannedOp{Kind: clientproto.RPCOrderPalaceEnter.String()}, want: clientproto.OrderPalaceEnterRequest{}},
		{name: "palace finish", op: automation.PlannedOp{Kind: clientproto.RPCOrderPalaceFinishOrder.String()}, want: clientproto.OrderPalaceFinishOrderRequest{}},
		{name: "team refresh", op: automation.PlannedOp{Kind: clientproto.RPCOrderTeamRefreshOrder.String()}, want: clientproto.OrderTeamRefreshOrderRequest{}},
		{name: "team submit", op: automation.PlannedOp{Kind: clientproto.RPCOrderTeamSubmitOrder.String()}, want: clientproto.OrderTeamSubmitOrderRequest{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, ok := operationSpecFor(tc.op.Kind)
			if !ok {
				t.Fatalf("operationSpecFor(%s) not found", tc.op.Kind)
			}
			if spec.run == nil {
				t.Fatalf("operationSpecFor(%s).run is nil", tc.op.Kind)
			}
			args, err := operationArgs(&tc.op)
			if err != nil {
				t.Fatalf("operationArgs(%s): %v", tc.op.Kind, err)
			}
			if got, want := jsonString(t, args), jsonString(t, tc.want); got != want {
				t.Fatalf("operationArgs(%s)=%s, want %s", tc.op.Kind, got, want)
			}
		})
	}
}

func TestOrderRackAndMailOperationArgs(t *testing.T) {
	cases := []struct {
		name string
		op   automation.PlannedOp
		want any
	}{
		{name: "customer gen", op: automation.PlannedOp{Kind: clientproto.RPCOrderCustomerGenOrder.String()}, want: clientproto.OrderCustomerGenOrderRequest{GuestNpcIdList: clientproto.RPCIDList{}}},
		{name: "customer reject", op: automation.PlannedOp{Kind: clientproto.RPCOrderCustomerRejectOrder.String(), TargetID: 7}, want: clientproto.OrderCustomerRejectOrderRequest{NPCId: 7}},
		{name: "rack recv money", op: automation.PlannedOp{Kind: clientproto.RPCFlowerRackRecvSellMoney.String(), TargetID: 3}, want: clientproto.FlowerRackRecvSellMoneyRequest{RackId: 3}},
		{name: "mail get list", op: automation.PlannedOp{Kind: clientproto.RPCMailGetList.String()}, want: clientproto.MailGetListRequest{}},
		{name: "mail pick", op: automation.PlannedOp{Kind: clientproto.RPCMailPick.String(), TargetID: 101, ItemID: 202}, want: clientproto.MailPickRequest{MsId: 101, AllId: 202}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, ok := operationSpecFor(tc.op.Kind)
			if !ok {
				t.Fatalf("operationSpecFor(%s) not found", tc.op.Kind)
			}
			if spec.run == nil {
				t.Fatalf("operationSpecFor(%s).run is nil", tc.op.Kind)
			}
			args, err := operationArgs(&tc.op)
			if err != nil {
				t.Fatalf("operationArgs(%s): %v", tc.op.Kind, err)
			}
			if got, want := jsonString(t, args), jsonString(t, tc.want); got != want {
				t.Fatalf("operationArgs(%s)=%s, want %s", tc.op.Kind, got, want)
			}
		})
	}
}

func TestShopCultivateOperationArgs(t *testing.T) {
	args, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCShopCultivateEnter.String()})
	if err != nil {
		t.Fatalf("operationArgs(shopCultivate.enter): %v", err)
	}
	if _, ok := args.(clientproto.ShopCultivateEnterRequest); !ok {
		t.Fatalf("operationArgs(shopCultivate.enter)=%T, want ShopCultivateEnterRequest", args)
	}

	args, err = operationArgs(&automation.PlannedOp{Kind: clientproto.RPCShopCultivateBuy.String(), TargetID: 10001})
	if err != nil {
		t.Fatalf("operationArgs(shopCultivate.buy): %v", err)
	}
	buy, ok := args.(clientproto.ShopCultivateBuyRequest)
	if !ok {
		t.Fatalf("operationArgs(shopCultivate.buy)=%T, want ShopCultivateBuyRequest", args)
	}
	if buy.ShopId != 10001 {
		t.Fatalf("ShopCultivateBuyRequest.ShopId=%d, want 10001", buy.ShopId)
	}
}

func TestShopGiftbagOperationArgs(t *testing.T) {
	args, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCShopGiftbagEnter.String()})
	if err != nil {
		t.Fatalf("operationArgs(shopGiftbag.enter): %v", err)
	}
	if _, ok := args.(clientproto.ShopGiftbagEnterRequest); !ok {
		t.Fatalf("operationArgs(shopGiftbag.enter)=%T, want ShopGiftbagEnterRequest", args)
	}

	args, err = operationArgs(&automation.PlannedOp{Kind: clientproto.RPCShopGiftbagBuy.String(), TargetID: 1, Count: 1})
	if err != nil {
		t.Fatalf("operationArgs(shopGiftbag.buy): %v", err)
	}
	buy, ok := args.(clientproto.ShopGiftbagBuyRequest)
	if !ok {
		t.Fatalf("operationArgs(shopGiftbag.buy)=%T, want ShopGiftbagBuyRequest", args)
	}
	if buy.ShopId != 1 || buy.Num != 1 {
		t.Fatalf("ShopGiftbagBuyRequest=%+v, want shopId=1 num=1", buy)
	}
}

func TestUsrExtraAntiFraudOperationArgs(t *testing.T) {
	cases := []struct {
		name string
		op   automation.PlannedOp
		want any
	}{
		{name: "update status", op: automation.PlannedOp{Kind: clientproto.RPCUsrExtraUpdateAntiFraudQAStatus.String()}, want: clientproto.UsrExtraUpdateAntiFraudQAStatusRequest{}},
		{name: "recv reward", op: automation.PlannedOp{Kind: clientproto.RPCUsrExtraRecvAntiFraudQARwd.String()}, want: clientproto.UsrExtraRecvAntiFraudQARwdRequest{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args, err := operationArgs(&tc.op)
			if err != nil {
				t.Fatalf("operationArgs(%s): %v", tc.op.Kind, err)
			}
			if got, want := jsonString(t, args), jsonString(t, tc.want); got != want {
				t.Fatalf("operationArgs(%s)=%s, want %s", tc.op.Kind, got, want)
			}
		})
	}
}

func TestZooOperationArgs(t *testing.T) {
	cases := []struct {
		name string
		op   automation.PlannedOp
		want any
	}{
		{name: "enter", op: automation.PlannedOp{Kind: clientproto.RPCZooEnterZoo.String()}, want: clientproto.ZooEnterZooRequest{}},
		{name: "feed", op: automation.PlannedOp{Kind: clientproto.RPCZooFeedPets.String(), TargetID: 1}, want: map[string]any{"petIdList": []int32{1}}},
		{name: "stroke", op: automation.PlannedOp{Kind: clientproto.RPCZooStrokePet.String(), TargetID: 1}, want: map[string]any{"petId": int32(1)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args, err := operationArgs(&tc.op)
			if err != nil {
				t.Fatalf("operationArgs(%s): %v", tc.op.Kind, err)
			}
			if got, want := jsonString(t, args), jsonString(t, tc.want); got != want {
				t.Fatalf("operationArgs(%s)=%s, want %s", tc.op.Kind, got, want)
			}
		})
	}
}

func TestPearlOperationArgs(t *testing.T) {
	cases := []struct {
		name string
		op   automation.PlannedOp
		want any
	}{
		{name: "refresh", op: automation.PlannedOp{Kind: clientproto.RPCPearlRefresh.String()}, want: clientproto.PearlRefreshRequest{}},
		{name: "daily free", op: automation.PlannedOp{Kind: clientproto.RPCPearlRecvDailyFree.String()}, want: clientproto.PearlRecvDailyFreeRequest{}},
		{name: "place recv", op: automation.PlannedOp{Kind: clientproto.RPCPearlPlaceRecv.String(), TargetID: 2}, want: clientproto.PearlPlaceRecvRequest{PlaceId: 2}},
		{name: "protect", op: automation.PlannedOp{Kind: clientproto.RPCPearlSetProtectState.String(), TargetID: 1}, want: clientproto.PearlSetProtectStateRequest{ProtectState: 1}},
		{name: "draw", op: automation.PlannedOp{Kind: clientproto.RPCPearlDraw.String(), Count: 1}, want: clientproto.PearlDrawRequest{Count: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args, err := operationArgs(&tc.op)
			if err != nil {
				t.Fatalf("operationArgs(%s): %v", tc.op.Kind, err)
			}
			if got, want := jsonString(t, args), jsonString(t, tc.want); got != want {
				t.Fatalf("operationArgs(%s)=%s, want %s", tc.op.Kind, got, want)
			}
		})
	}
}

func TestFmlBuildOperationArgs(t *testing.T) {
	args, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCFmlBuild.String(), TargetID: 2})
	if err != nil {
		t.Fatalf("operationArgs(Fml.build): %v", err)
	}
	build, ok := args.(clientproto.FmlBuildRequest)
	if !ok {
		t.Fatalf("operationArgs(Fml.build)=%T, want FmlBuildRequest", args)
	}
	if build.ID != 2 {
		t.Fatalf("FmlBuildRequest.ID=%d, want 2", build.ID)
	}
}

func TestFmlLandHarvestOperationArgs(t *testing.T) {
	args, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCFmlLandHarvest.String(), LandIDs: []int32{1, 3}})
	if err != nil {
		t.Fatalf("operationArgs(fmlLand.harvest): %v", err)
	}
	harvest, ok := args.(clientproto.FmlLandHarvestRequest)
	if !ok {
		t.Fatalf("operationArgs(fmlLand.harvest)=%T, want FmlLandHarvestRequest", args)
	}
	if len(harvest.LandIds) != 2 || harvest.LandIds[0] != 1 || harvest.LandIds[1] != 3 {
		t.Fatalf("FmlLandHarvestRequest.LandIds=%v, want [1 3]", harvest.LandIds)
	}
}

func TestFmlForestRefreshOperationArgs(t *testing.T) {
	args, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCFmlForestRefresh.String(), TargetID: 1})
	if err != nil {
		t.Fatalf("operationArgs(fmlForest.refresh): %v", err)
	}
	refresh, ok := args.(clientproto.FmlForestRefreshRequest)
	if !ok {
		t.Fatalf("operationArgs(fmlForest.refresh)=%T, want FmlForestRefreshRequest", args)
	}
	if refresh.IsAutoCollect != 1 {
		t.Fatalf("FmlForestRefreshRequest.IsAutoCollect=%d, want 1", refresh.IsAutoCollect)
	}
}

func TestFmlFlowerShareOperationArgs(t *testing.T) {
	cases := []struct {
		name string
		op   automation.PlannedOp
		want any
	}{
		{name: "refresh", op: automation.PlannedOp{Kind: clientproto.RPCFmlFlowerShareRefresh.String()}, want: clientproto.FmlFlowerShareRefreshRequest{}},
		{name: "other list", op: automation.PlannedOp{Kind: clientproto.RPCFmlFlowerShareGetFmlOtherShareList.String()}, want: clientproto.FmlFlowerShareGetFmlOtherShareListRequest{}},
		{name: "recv reward", op: automation.PlannedOp{Kind: clientproto.RPCFmlFlowerShareRecvRwd.String(), SlotIDs: []int32{1, 3}}, want: clientproto.FmlFlowerShareRecvRwdRequest{SlotIds: []int32{1, 3}}},
		{name: "take", op: automation.PlannedOp{Kind: clientproto.RPCFmlFlowerShareTake.String(), TargetUID: 77900091102484, TargetID: 2}, want: clientproto.FmlFlowerShareTakeRequest{DstUid: 77900091102484, SlotId: 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args, err := operationArgs(&tc.op)
			if err != nil {
				t.Fatalf("operationArgs(%s): %v", tc.op.Kind, err)
			}
			if got, want := jsonString(t, args), jsonString(t, tc.want); got != want {
				t.Fatalf("operationArgs(%s)=%s, want %s", tc.op.Kind, got, want)
			}
		})
	}
}

func jsonString(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	return string(raw)
}

func TestApplyHarvestBlocksSkipsBlockedSingleLand(t *testing.T) {
	now := time.Now()
	r := &Runner{harvestBlockedUntil: map[int32]time.Time{1002: now.Add(time.Minute)}}
	op := &automation.PlannedOp{
		Kind:    "usrLand.harvest",
		LandIDs: []int32{1002},
	}

	if got := r.applyHarvestBlocks(op, now); got != nil {
		t.Fatalf("applyHarvestBlocks()=%+v, want nil", got)
	}
}

func TestApplyHarvestBlocksFiltersBlockedLandFromBatch(t *testing.T) {
	now := time.Now()
	r := &Runner{harvestBlockedUntil: map[int32]time.Time{1002: now.Add(time.Minute)}}
	op := &automation.PlannedOp{
		Kind:    "usrLand.harvest",
		LandIDs: []int32{1001, 1002, 1003},
	}

	got := r.applyHarvestBlocks(op, now)
	if got == nil {
		t.Fatal("applyHarvestBlocks()=nil, want remaining harvest lands")
	}
	if len(got.LandIDs) != 2 || got.LandIDs[0] != 1001 || got.LandIDs[1] != 1003 {
		t.Fatalf("filtered LandIDs=%v, want [1001 1003]", got.LandIDs)
	}
}

func TestOneKeyOperationSpecsRemoved(t *testing.T) {
	for _, kind := range []string{
		clientproto.RPCUsrLandHarvestOneKey.String(),
		clientproto.RPCUsrLandWaterOneKey.String(),
		clientproto.RPCUsrLandPlantOneKey.String(),
		clientproto.RPCFlowerRackRecvOneKey.String(),
		clientproto.RPCMailPickOneKey.String(),
	} {
		if _, ok := operationSpecFor(kind); ok {
			t.Fatalf("operationSpecFor(%s) should not be registered", kind)
		}
	}
}

func TestNextRunnableOperationFallsThroughBlockedHarvest(t *testing.T) {
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"7": 6}}},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 3},
			"1002": map[string]any{"0": 23002, "1": 1},
		}},
	})
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Plant.Flower.AutoEnabled = true
	r := &Runner{
		state:               st,
		harvestBlockedUntil: map[int32]time.Time{1001: now.Add(time.Minute)},
	}

	op := r.nextRunnableOperation(policy, now)
	if op == nil || op.Kind != clientproto.RPCUsrLandWater.String() || len(op.LandIDs) != 1 || op.LandIDs[0] != 1002 {
		t.Fatalf("nextRunnableOperation()=%+v, want water op for 1002", op)
	}
}

func TestNextRunnableOperationWaitsForLocalWaterwheelBucket(t *testing.T) {
	now := time.Now()
	st := state.New()
	st.ApplyVMap(map[string]any{
		"114": map[string]any{
			"1": 1,
			"4": now.Add(-time.Hour).UnixMilli(),
		},
		"117": map[string]any{
			"1": 2,
			"2": now.UnixMilli(),
		},
	})
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.WaterwheelEnabled = true
	policy.Basic.FreeWaterEnabled = true
	r := &Runner{state: st}

	op := r.nextRunnableOperation(policy, now)
	if op == nil || op.Kind != clientproto.RPCFreeWaterRecv.String() {
		t.Fatalf("nextRunnableOperation()=%+v, want free water while waterwheel waits for a local bucket", op)
	}
}

func TestApplyHarvestBlocksIgnoresExpiredBlock(t *testing.T) {
	now := time.Now()
	r := &Runner{harvestBlockedUntil: map[int32]time.Time{1002: now.Add(-time.Second)}}
	op := &automation.PlannedOp{
		Kind:    "usrLand.harvest",
		LandIDs: []int32{1002},
	}

	if got := r.applyHarvestBlocks(op, now); got != op {
		t.Fatalf("applyHarvestBlocks()=%+v, want original op", got)
	}
}

func TestIsWaterwheelInvalidDataError(t *testing.T) {
	err := errors.New("rpc waterwheel.recv: server: 数据有误")
	if !isWaterwheelInvalidDataError(clientproto.RPCWaterwheelRecv.String(), err) {
		t.Fatal("isWaterwheelInvalidDataError = false, want true")
	}
	if isWaterwheelInvalidDataError(clientproto.RPCFreeWaterRecv.String(), err) {
		t.Fatal("isWaterwheelInvalidDataError matched the wrong rpc")
	}
}

func TestIsWaterDropResourceRejectedError(t *testing.T) {
	err := errors.New(`rpc usrLand.waterBatch: server: {"code":301,"param":{"iid":7}}`)
	if !isWaterDropResourceRejectedError(clientproto.RPCUsrLandWaterBatch.String(), err) {
		t.Fatal("isWaterDropResourceRejectedError = false, want true")
	}
	if isWaterDropResourceRejectedError(clientproto.RPCWaterwheelRecv.String(), err) {
		t.Fatal("isWaterDropResourceRejectedError matched the wrong rpc")
	}
	if isWaterDropResourceRejectedError(clientproto.RPCUsrLandWaterBatch.String(), errors.New(`{"code":301,"param":{"iid":1001}}`)) {
		t.Fatal("isWaterDropResourceRejectedError matched a non-water-drop resource")
	}
}

func TestCheckOperationResourcesUsesCostGates(t *testing.T) {
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"1001": 1}}},
	})
	r := &Runner{state: st}
	op := &automation.PlannedOp{
		Kind: clientproto.RPCUsrLandSpeedUpBatch.String(),
		CostGates: []automation.CostGate{{
			ID:           "item:1001",
			ResourceKind: automation.GateResourceItem,
			Label:        "加速券",
			ItemID:       1001,
			Required:     2,
			Status:       automation.PlanStatusReady,
		}},
	}

	if err := r.checkOperationResources(op, time.Now()); err == nil {
		t.Fatal("checkOperationResources() error=nil, want insufficient item gate error")
	}
}
