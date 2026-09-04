package runner

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"github.com/SilkageNet/mygardenworld/internal/store"
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

func TestLandSuffixSuppressesEmptyLandIDs(t *testing.T) {
	if got := landSuffix(nil); got != "" {
		t.Fatalf("landSuffix(nil)=%q, want empty", got)
	}
	if got := landSuffix([]int32{}); got != "" {
		t.Fatalf("landSuffix(empty)=%q, want empty", got)
	}
	if got := landSuffix([]int32{1001, 1002}); got != " (田地=[1001 1002])" {
		t.Fatalf("landSuffix(ids)=%q", got)
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

func TestStoryAchievementAndMapOperationArgs(t *testing.T) {
	cases := []struct {
		name string
		op   automation.PlannedOp
		want any
	}{
		{name: "story enter", op: automation.PlannedOp{Kind: clientproto.RPCStoryMainEnter.String()}, want: clientproto.StoryMainEnterRequest{}},
		{name: "story unlock", op: automation.PlannedOp{Kind: clientproto.RPCStoryMainUnlock.String(), TargetID: 4101, ItemCost: map[int32]int32{56: 85}}, want: clientproto.StoryMainUnlockRequest{}},
		{name: "achievement recv", op: automation.PlannedOp{Kind: clientproto.RPCTaskAchRecv.String(), TargetID: 10001}, want: clientproto.TaskAchRecvRequest{ID: 10001}},
		{name: "random event enter", op: automation.PlannedOp{Kind: clientproto.RPCRandomEventEnter.String()}, want: clientproto.RandomEventEnterRequest{}},
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

func TestOrderPalaceAndTeamOperationSpecs(t *testing.T) {
	cases := []struct {
		name string
		op   automation.PlannedOp
		want any
	}{
		{name: "palace enter", op: automation.PlannedOp{Kind: clientproto.RPCOrderPalaceEnter.String()}, want: clientproto.OrderPalaceEnterRequest{}},
		{name: "palace finish", op: automation.PlannedOp{Kind: clientproto.RPCOrderPalaceFinishOrder.String()}, want: clientproto.OrderPalaceFinishOrderRequest{}},
		{name: "satin finish", op: automation.PlannedOp{Kind: clientproto.RPCOrderFlowerFinishSatinOrder.String()}, want: clientproto.OrderFlowerFinishSatinOrderRequest{}},
		{name: "decorate finish", op: automation.PlannedOp{Kind: clientproto.RPCOrderFlowerFinishDecorateOrder.String()}, want: clientproto.OrderFlowerFinishDecorateOrderRequest{}},
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

func TestSignTypeOperationArgsAndStagePreflight(t *testing.T) {
	cases := []struct {
		kind string
		want any
	}{
		{kind: clientproto.RPCSignTypeEnter.String(), want: clientproto.SignTypeEnterRequest{Type: state.SignTypeAntiFraud}},
		{kind: clientproto.RPCSignTypeSign.String(), want: clientproto.SignTypeSignRequest{Type: state.SignTypeAntiFraud}},
		{kind: clientproto.RPCSignTypeRecv.String(), want: clientproto.SignTypeRecvRequest{Type: state.SignTypeAntiFraud}},
	}
	for _, tc := range cases {
		args, err := operationArgs(&automation.PlannedOp{Kind: tc.kind, TargetID: state.SignTypeAntiFraud})
		if err != nil {
			t.Fatalf("operationArgs(%s): %v", tc.kind, err)
		}
		if got, want := jsonString(t, args), jsonString(t, tc.want); got != want {
			t.Fatalf("operationArgs(%s)=%s, want %s", tc.kind, got, want)
		}
	}
	if _, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCSignTypeSign.String(), TargetID: 2}); err == nil {
		t.Fatal("signType type=2 should not be executable")
	}

	st := state.New()
	baseOld := time.Now().Add(-24 * time.Hour).UnixMilli()
	st.ApplyVMap(map[string]any{
		"7":   map[string]any{"7": map[string]any{"2": map[string]any{"0": 123, "1": 2, "2": 2, "3": baseOld, "4": int64(1)}}},
		"140": map[string]any{"0": map[string]any{"1": map[string]any{"0": 123, "1": 1, "3": 1, "4": 0}}},
	})
	rt := operationRuntime{runner: &Runner{state: st}}
	if done, err := signTypeStagePreflight(rt, state.SignTypeAntiFraud, state.SignTypeStatusCanSign); err != nil || done {
		t.Fatalf("status 0 preflight = done:%t err:%v", done, err)
	}
	if ready, err := signTypeStageReadyAfterEnter(rt, state.SignTypeAntiFraud, state.SignTypeStatusCanReceive, time.Now()); err != nil || ready {
		t.Fatalf("recv enter reset to status 0 = ready:%t err:%v, want successful replan", ready, err)
	}
	st.ApplyV(json.RawMessage(`{"140":{"0":{"1":{"4":1}}}}`))
	if done, err := signTypeStagePreflight(rt, state.SignTypeAntiFraud, state.SignTypeStatusCanSign); err != nil || !done {
		t.Fatalf("stale sign preflight = done:%t err:%v", done, err)
	}
	if done, err := signTypeStagePreflight(rt, state.SignTypeAntiFraud, state.SignTypeStatusCanReceive); err != nil || done {
		t.Fatalf("status 1 recv preflight = done:%t err:%v", done, err)
	}

	now := time.Date(2026, 7, 11, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	old := now.Add(-24 * time.Hour).UnixMilli()
	st = state.New()
	st.ApplyVMap(map[string]any{
		"7":   map[string]any{"7": map[string]any{"2": map[string]any{"0": 123, "1": 2, "2": 2, "3": old, "4": int64(1)}}},
		"140": map[string]any{"0": map[string]any{"1": map[string]any{"0": 123, "1": 1, "3": 1, "4": 2, "5": old}}},
	})
	rt = operationRuntime{runner: &Runner{state: st}}
	if needed, err := signTypeEnterSyncNeeded(rt, state.SignTypeAntiFraud, now); err != nil || !needed {
		t.Fatalf("cross-day enter preflight = needed:%t err:%v", needed, err)
	}
	st.MarkSignTypeEnterAttempt(state.SignTypeAntiFraud, now)
	if needed, err := signTypeEnterSyncNeeded(rt, state.SignTypeAntiFraud, now); err != nil || needed {
		t.Fatalf("deduplicated enter preflight = needed:%t err:%v", needed, err)
	}
}

func TestSignTypeBusinessStateErrorsAreNonRetryable(t *testing.T) {
	want := map[int]string{
		3500: "条件已达成，无需重复操作",
		3501: "今日奖励已领取",
		3502: "未达成领取条件，无法获取奖励",
		3503: "功能暂未解锁",
	}
	for code, description := range want {
		got, ok := signTypeNonRetryableCode(code)
		if !ok || got != description {
			t.Fatalf("signTypeNonRetryableCode(%d)=(%q,%t), want (%q,true)", code, got, ok, description)
		}
	}
	if got, ok := signTypeNonRetryableCode(3499); ok || got != "" {
		t.Fatalf("signTypeNonRetryableCode(3499)=(%q,%t), want non-business error", got, ok)
	}

	st := state.New()
	st.ApplyV(json.RawMessage(`{"140":{"0":{"1":{"0":123,"1":1,"3":1,"4":0}}}}`))
	rt := operationRuntime{runner: &Runner{state: st}}
	d := babigame.WSResponseD{M: json.RawMessage(`{"code":3501,"args":[]}`)}
	if err := invalidateSignTypeServerStateError(rt, state.SignTypeAntiFraud, "signType.recv", d); err == nil {
		t.Fatal("3501 did not produce a fail-closed error")
	}
	view, observed := st.SignType(state.SignTypeAntiFraud)
	if !observed || view.Valid {
		t.Fatalf("3501 did not invalidate sign state: observed=%t view=%+v", observed, view)
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

	args, err = operationArgs(&automation.PlannedOp{Kind: clientproto.RPCShopCultivateRefresh.String()})
	if err != nil {
		t.Fatalf("operationArgs(shopCultivate.refresh): %v", err)
	}
	if _, ok := args.(clientproto.ShopCultivateRefreshRequest); !ok {
		t.Fatalf("operationArgs(shopCultivate.refresh)=%T, want ShopCultivateRefreshRequest", args)
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

func TestZooFoodShopOperationArgs(t *testing.T) {
	args, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCShopEnter.String(), TargetID: state.ZooFoodShopTempID})
	if err != nil {
		t.Fatalf("operationArgs(shop.enter): %v", err)
	}
	enter, ok := args.(clientproto.ShopEnterRequest)
	if !ok || enter.TempId != state.ZooFoodShopTempID {
		t.Fatalf("operationArgs(shop.enter)=%+v, want tempId=%d", args, state.ZooFoodShopTempID)
	}

	args, err = operationArgs(&automation.PlannedOp{
		Kind:     clientproto.RPCShopBuy.String(),
		TargetID: state.ZooFoodShopTempID,
		ItemID:   state.ZooNormalFoodShopItemID,
		Count:    3,
		GoldCost: 300,
	})
	if err != nil {
		t.Fatalf("operationArgs(shop.buy): %v", err)
	}
	buy, ok := args.(clientproto.ShopBuyRequest)
	if !ok || buy.TempId != state.ZooFoodShopTempID || buy.ItemId != state.ZooNormalFoodShopItemID || buy.Count != 3 {
		t.Fatalf("operationArgs(shop.buy)=%+v", args)
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

func TestSDKAdOperationsRejectedBeforeExecution(t *testing.T) {
	r := &Runner{state: state.New()}
	now := time.Now()
	tests := []struct {
		name string
		op   automation.PlannedOp
	}{
		{
			name: "video giftbag",
			op: automation.PlannedOp{
				Kind:     clientproto.RPCShopGiftbagBuy.String(),
				TargetID: 1,
				Count:    1,
			},
		},
		{
			name: "video union build",
			op: automation.PlannedOp{
				Kind:     clientproto.RPCFmlBld.String(),
				TargetID: 1,
				Count:    1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := r.checkOperationResources(&tt.op, now)
			if err == nil || !strings.Contains(err.Error(), "SDK 广告") || !strings.Contains(err.Error(), "不会编造") {
				t.Fatalf("checkOperationResources() error = %v, want explicit SDK ad rejection", err)
			}
		})
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
		{name: "refresh", op: automation.PlannedOp{Kind: clientproto.RPCZooRefreshPetStatus.String(), TargetID: 1}, want: clientproto.ZooRefreshPetStatusRequest{PetIdList: clientproto.RPCIDList{1}}},
		{name: "stock", op: automation.PlannedOp{Kind: clientproto.RPCZooAddFoodstuff.String(), TargetID: 1, ItemID: 1501, Count: 3, ItemCost: map[int32]int32{1501: 3}}, want: clientproto.ZooAddFoodstuffRequest{PetId: 1, FoodstuffIds: clientproto.RPCIDList{1501, 1501, 1501}}},
		{name: "stroke", op: automation.PlannedOp{Kind: clientproto.RPCZooStrokePet.String(), TargetID: 1}, want: clientproto.ZooStrokePetRequest{PetId: 1}},
		{name: "handle log index", op: automation.PlannedOp{Kind: clientproto.RPCZooHandleEvent.String(), TargetID: 1, ItemID: 42, Count: 1}, want: clientproto.ZooHandleEventRequest{PetId: 1, TableId: 42, Agree: true, IsShareVideo: 0}},
		{name: "read log", op: automation.PlannedOp{Kind: clientproto.RPCZooReadLog.String(), TargetID: 1, ItemID: 42}, want: clientproto.ZooReadLogRequest{PetId: 1}},
		{name: "recv souvenir rewards", op: automation.PlannedOp{Kind: clientproto.RPCZooRecvSouvenirRwd.String(), SlotIDs: []int32{2, 3}, Count: 2}, want: clientproto.ZooRecvSouvenirRwdRequest{IdxList: clientproto.RPCIDList{2, 3}}},
		{name: "read souvenirs", op: automation.PlannedOp{Kind: clientproto.RPCZooReadSouvenir.String(), SlotIDs: []int32{30201, 32901}, Count: 2}, want: clientproto.ZooReadSouvenirRequest{SouvenirIds: clientproto.RPCIDList{30201, 32901}}},
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

func TestZooOperationRegistryExcludesUnsafeRPCs(t *testing.T) {
	for _, rpc := range []clientproto.RPCName{
		clientproto.RPCZooFeedPets,
		clientproto.RPCZooFindPet,
	} {
		if _, ok := operationSpecFor(rpc.String()); ok {
			t.Fatalf("unsafe zoo RPC %s remains executable", rpc)
		}
	}
	for _, rpc := range []clientproto.RPCName{
		clientproto.RPCZooRecvSouvenirRwd,
		clientproto.RPCZooReadSouvenir,
	} {
		if _, ok := operationSpecFor(rpc.String()); !ok {
			t.Fatalf("safe zoo souvenir RPC %s is not executable", rpc)
		}
	}
}

func TestZooSouvenirArgsRequireStrictCostFreeBatches(t *testing.T) {
	for _, kind := range []string{clientproto.RPCZooRecvSouvenirRwd.String(), clientproto.RPCZooReadSouvenir.String()} {
		base := automation.PlannedOp{Kind: kind, SlotIDs: []int32{2, 3}, Count: 2}
		if _, err := operationArgs(&base); err != nil {
			t.Fatalf("safe %s args: %v", kind, err)
		}
		for name, op := range map[string]automation.PlannedOp{
			"empty":          {Kind: kind},
			"count mismatch": {Kind: kind, SlotIDs: []int32{2, 3}, Count: 1},
			"zero":           {Kind: kind, SlotIDs: []int32{0}, Count: 1},
			"duplicate":      {Kind: kind, SlotIDs: []int32{2, 2}, Count: 2},
			"unsorted":       {Kind: kind, SlotIDs: []int32{3, 2}, Count: 2},
			"gold":           {Kind: kind, SlotIDs: []int32{2}, Count: 1, GoldCost: 1},
			"diamond":        {Kind: kind, SlotIDs: []int32{2}, Count: 1, DiamondCost: 1},
			"item":           {Kind: kind, SlotIDs: []int32{2}, Count: 1, ItemCost: map[int32]int32{11: 1}},
		} {
			if _, err := operationArgs(&op); err == nil {
				t.Fatalf("unsafe %s %s args accepted: %+v", kind, name, op)
			}
		}
	}
}

func TestZooHandleEventRejectsAnyCostOrAmbiguousResult(t *testing.T) {
	base := automation.PlannedOp{Kind: clientproto.RPCZooHandleEvent.String(), TargetID: 1, ItemID: 42, Count: 1}
	if _, err := operationArgs(&base); err != nil {
		t.Fatalf("safe handleEvent args: %v", err)
	}
	for _, op := range []automation.PlannedOp{
		{Kind: base.Kind, TargetID: 1, ItemID: 42, Count: 0},
		{Kind: base.Kind, TargetID: 1, ItemID: 42, Count: 1, GoldCost: 1},
		{Kind: base.Kind, TargetID: 1, ItemID: 42, Count: 1, DiamondCost: 1},
		{Kind: base.Kind, TargetID: 1, ItemID: 42, Count: 1, ItemCost: map[int32]int32{11: 1}},
	} {
		if _, err := operationArgs(&op); err == nil {
			t.Fatalf("unsafe handleEvent unexpectedly accepted: %+v", op)
		}
	}
}

func TestZooAddFoodstuffRequiresExactObservedCost(t *testing.T) {
	base := automation.PlannedOp{Kind: clientproto.RPCZooAddFoodstuff.String(), TargetID: 1, ItemID: 1501, Count: 2}
	if _, err := operationArgs(&base); err == nil {
		t.Fatal("addFoodstuff without ItemCost unexpectedly accepted")
	}
	base.ItemCost = map[int32]int32{1501: 1}
	if _, err := operationArgs(&base); err == nil {
		t.Fatal("addFoodstuff with mismatched ItemCost unexpectedly accepted")
	}
	base.ItemCost[1501] = 2
	args, err := operationArgs(&base)
	if err != nil {
		t.Fatalf("addFoodstuff with exact ItemCost: %v", err)
	}
	request, ok := args.(clientproto.ZooAddFoodstuffRequest)
	if !ok || request.PetId != 1 || len(request.FoodstuffIds) != 2 || request.FoodstuffIds[0] != 1501 || request.FoodstuffIds[1] != 1501 {
		t.Fatalf("addFoodstuff args=%T %+v, want typed repeated IDs", args, args)
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
		{name: "place recv one key", op: automation.PlannedOp{Kind: clientproto.RPCPearlPlaceRecvOneKey.String()}, want: clientproto.PearlPlaceRecvOneKeyRequest{}},
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

func TestFmlBldOperationArgs(t *testing.T) {
	args, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCFmlBld.String(), TargetID: 2})
	if err != nil {
		t.Fatalf("operationArgs(fml.bld): %v", err)
	}
	build, ok := args.(clientproto.FmlBldRequest)
	if !ok {
		t.Fatalf("operationArgs(fml.bld)=%T, want FmlBldRequest", args)
	}
	if build.ID != 2 {
		t.Fatalf("FmlBldRequest.ID=%d, want 2", build.ID)
	}
	if _, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCFmlBld.String(), TargetID: 0}); err == nil {
		t.Fatal("operationArgs(fml.bld id=0) should fail")
	}
}

func TestFmlEnterOperationArgs(t *testing.T) {
	args, err := operationArgs(&automation.PlannedOp{Kind: clientproto.RPCFmlEnter.String()})
	if err != nil {
		t.Fatalf("operationArgs(fml.enter): %v", err)
	}
	enter, ok := args.(clientproto.FmlEnterRequest)
	if !ok {
		t.Fatalf("operationArgs(fml.enter)=%T, want FmlEnterRequest", args)
	}
	if enter.Fml != 1 || enter.Mb != 1 || enter.MbL != 1 {
		t.Fatalf("FmlEnterRequest=%+v, want fml=1 mb=1 mbL=1", enter)
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

func TestFmlLandPlantOperationArgs(t *testing.T) {
	args, err := operationArgs(&automation.PlannedOp{
		Kind:     clientproto.RPCFmlLandPlant.String(),
		LandIDs:  []int32{1, 2},
		FlowerID: 23005,
	})
	if err != nil {
		t.Fatalf("operationArgs(fmlLand.plant): %v", err)
	}
	plant, ok := args.(clientproto.FmlLandPlantRequest)
	if !ok {
		t.Fatalf("operationArgs(fmlLand.plant)=%T, want FmlLandPlantRequest", args)
	}
	if len(plant.LandIds) != 2 || plant.LandIds[0] != 1 || plant.LandIds[1] != 2 || plant.FlwId != 23005 {
		t.Fatalf("FmlLandPlantRequest=%+v, want landIds=[1 2] flwId=23005", plant)
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
	if !refresh.IsAutoCollect {
		t.Fatal("FmlForestRefreshRequest.IsAutoCollect=false, want true")
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
		return
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
	policy.Plant.Planting.AutoEnabled = true
	policy.Union.Race.Enabled = false
	r := &Runner{
		state:               st,
		harvestBlockedUntil: map[int32]time.Time{1001: now.Add(time.Minute)},
	}

	op := r.nextRunnableOperation(policy, now)
	if op == nil || op.Kind != clientproto.RPCUsrLandWater.String() || len(op.LandIDs) != 1 || op.LandIDs[0] != 1002 {
		t.Fatalf("nextRunnableOperation()=%+v, want water op for 1002", op)
	}
}

func TestNextRunnableOperationSkipsCoolingSideOperationAndKeepsFarmRunnable(t *testing.T) {
	now := time.Date(2026, 7, 5, 11, 45, 0, 0, time.UTC)
	st := state.New()
	st.ApplyVMap(map[string]any{
		"22": map[string]any{
			"1": map[string]any{
				"1": map[string]any{"4": 569},
				"3": map[string]any{},
				"100": map[string]any{
					"40001": map[string]any{"0": 40001, "1": 569, "2": 0},
				},
			},
		},
	})
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Task.DailyEnabled = true
	policy.Union.Race.Enabled = false
	// Land monitor syncs via fml.enter whenever 25.102 is unseen; keep it quiet
	// so this test only asserts side cooldown + farm fallthrough.
	st.ApplyVMap(map[string]any{"25": map[string]any{"102": map[string]any{"0": map[string]any{}}}})
	r := &Runner{
		state:              st,
		operationCooldowns: map[string]operationCooldown{},
	}

	side := r.nextRunnableOperation(policy, now)
	if side == nil || side.Kind != clientproto.RPCTaskDlyRecv.String() || side.Lane != automation.LaneSide {
		t.Fatalf("nextRunnableOperation()=%+v, want side daily task", side)
	}
	r.setSideOperationCooldown(side, now, errors.New("server busy"), "", 0)
	if op := r.nextRunnableOperation(policy, now.Add(time.Second)); op != nil {
		t.Fatalf("nextRunnableOperation()=%+v, want nil while only side op is cooling", op)
	}

	st.ApplyVMap(map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 3},
		}},
	})
	op := r.nextRunnableOperation(policy, now.Add(2*time.Second))
	if op == nil || op.Kind != clientproto.RPCUsrLandHarvest.String() || op.Lane != automation.LaneFarm {
		t.Fatalf("nextRunnableOperation()=%+v, want farm harvest while side op is cooling", op)
	}
}

func TestNextRunnableOperationWaitsForLocalWaterwheelBucket(t *testing.T) {
	now := time.Date(2026, 7, 6, 11, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
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
	policy.Basic.Reputation.Enabled = false
	policy.Basic.WaterwheelEnabled = true
	policy.Basic.FreeWaterEnabled = true
	r := &Runner{state: st}

	op := r.nextRunnableOperation(policy, now)
	if op == nil || op.Kind != clientproto.RPCFreeWaterRecv.String() {
		t.Fatalf("nextRunnableOperation()=%+v, want free water while waterwheel waits for a local bucket", op)
	}
}

func TestNextRunnableOperationSkipsWaterwheelAfterDailyLimit(t *testing.T) {
	now := time.Date(2026, 7, 6, 11, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	st := state.New()
	st.ApplyVMap(map[string]any{
		"114": map[string]any{
			"1": 1,
		},
		"117": map[string]any{
			"1": 2,
			"2": now.UnixMilli(),
		},
	})
	st.MarkWaterwheelEntered(now.Add(-time.Hour))
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Reputation.Enabled = false
	policy.Basic.WaterwheelEnabled = true
	policy.Basic.FreeWaterEnabled = true
	r := &Runner{state: st}

	op := r.nextRunnableOperation(policy, now)
	if op == nil || op.Kind != clientproto.RPCWaterwheelRecv.String() {
		t.Fatalf("nextRunnableOperation()=%+v, want waterwheel before daily limit is recorded", op)
	}

	st.MarkWaterwheelDailyLimitReached(now)
	op = r.nextRunnableOperation(policy, now)
	if op == nil || op.Kind != clientproto.RPCFreeWaterRecv.String() {
		t.Fatalf("nextRunnableOperation()=%+v, want free water after waterwheel daily limit", op)
	}
}

func TestEnforceReputationGuardDisablesAutomationOnLowScore(t *testing.T) {
	now := time.Now()
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{
			"17": map[string]any{"0": map[string]any{"1": 79}},
		},
	})
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Reputation.Enabled = true
	policy.Basic.Reputation.Threshold = 80
	r := &Runner{
		account:                &store.Account{ID: 1, Name: "test"},
		log:                    slog.New(slog.NewTextHandler(io.Discard, nil)),
		state:                  st,
		policy:                 policy,
		lastReputationSyncTick: now,
		harvestBlockedUntil:    map[int32]time.Time{},
		unknownRPCCounts:       map[string]int32{},
	}

	err := r.enforceReputationGuard(context.Background(), nil, nil, "test", now)
	if !isReputationGuardError(err) {
		t.Fatalf("enforceReputationGuard() error=%v, want reputation guard error", err)
	}
	if r.Policy().GetAutomationEnabled() {
		t.Fatal("automation enabled = true, want disabled after low reputation score")
	}
}

func TestEnforceReputationGuardAllowsSafeScore(t *testing.T) {
	now := time.Now()
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{
			"17": map[string]any{"0": map[string]any{"1": 90}},
		},
	})
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Reputation.Enabled = true
	policy.Basic.Reputation.Threshold = 80
	r := &Runner{
		account:                &store.Account{ID: 1, Name: "test"},
		log:                    slog.New(slog.NewTextHandler(io.Discard, nil)),
		state:                  st,
		policy:                 policy,
		lastReputationSyncTick: now,
		harvestBlockedUntil:    map[int32]time.Time{},
		unknownRPCCounts:       map[string]int32{},
	}

	if err := r.enforceReputationGuard(context.Background(), nil, nil, "test", now); err != nil {
		t.Fatalf("enforceReputationGuard() error=%v, want nil", err)
	}
	if !r.Policy().GetAutomationEnabled() {
		t.Fatal("automation enabled = false, want unchanged for safe reputation score")
	}
}

func TestEnforceReputationGuardDoesNotTreatUnobservedScoreAsZero(t *testing.T) {
	now := time.Now()
	st := state.New()
	st.ApplyVMap(map[string]any{
		"7": map[string]any{
			"17": map[string]any{"0": map[string]any{"0": 77900091102482}},
		},
	})
	policy := automation.DefaultPolicy()
	policy.AutomationEnabled = true
	policy.Basic.Reputation.Enabled = true
	policy.Basic.Reputation.Threshold = 80
	r := &Runner{
		account:                &store.Account{ID: 1, Name: "test"},
		log:                    slog.New(slog.NewTextHandler(io.Discard, nil)),
		state:                  st,
		policy:                 policy,
		lastReputationSyncTick: now,
		harvestBlockedUntil:    map[int32]time.Time{},
		unknownRPCCounts:       map[string]int32{},
	}

	err := r.enforceReputationGuard(context.Background(), nil, nil, "startup", now)
	if err == nil {
		t.Fatal("enforceReputationGuard() error=nil, want unobserved-score error")
	}
	if isReputationGuardError(err) {
		t.Fatalf("enforceReputationGuard() error=%v, must not classify unobserved score as low reputation", err)
	}
	if !r.Policy().GetAutomationEnabled() {
		t.Fatal("automation enabled = false, want unchanged while reputation is unobserved")
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

func TestIsWaterwheelDailyLimitError(t *testing.T) {
	err := errors.New("rpc waterwheel.recv: server: 已达到领取上限")
	if !isWaterwheelDailyLimitError(clientproto.RPCWaterwheelRecv.String(), err) {
		t.Fatal("isWaterwheelDailyLimitError = false, want true")
	}
	if isWaterwheelDailyLimitError(clientproto.RPCFreeWaterRecv.String(), err) {
		t.Fatal("isWaterwheelDailyLimitError matched the wrong rpc")
	}
}

func TestIsFmlFlowerTakeDailyLimitError(t *testing.T) {
	err := errors.New(`rpc fmlFlowerShare.take: server: {"code":"fmlShare_tips8","msg":"今日拿取次数已达上限","args":[]}`)
	if !isFmlFlowerTakeDailyLimitError(clientproto.RPCFmlFlowerShareTake.String(), err) {
		t.Fatal("isFmlFlowerTakeDailyLimitError = false, want true")
	}
	if isFmlFlowerTakeDailyLimitError(clientproto.RPCFmlFlowerShareRecvRwd.String(), err) {
		t.Fatal("isFmlFlowerTakeDailyLimitError matched the wrong rpc")
	}
	if got := classifyOperationError(clientproto.RPCFmlFlowerShareTake.String(), err); got != operationErrorFmlFlowerTakeDailyLimit {
		t.Fatalf("classifyOperationError=%q, want %q", got, operationErrorFmlFlowerTakeDailyLimit)
	}

	rpcErr := &babigame.RPCServerError{
		Name:     clientproto.RPCFmlFlowerShareTake,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"code":"fmlShare_tips8","msg":"今日拿取次数已达上限","args":[]}`)},
	}
	if !isFmlFlowerTakeDailyLimitError(clientproto.RPCFmlFlowerShareTake.String(), rpcErr) {
		t.Fatal("RPCServerError tips8 should classify as daily limit")
	}

	r := newOperationEventTestRunner()
	now := time.Now()
	op := &automation.PlannedOp{
		Kind:      clientproto.RPCFmlFlowerShareTake.String(),
		Category:  automation.CategoryUnion,
		Domain:    "union.flower.take",
		Action:    "take",
		Lane:      automation.LaneSide,
		TargetUID: 662505100059,
		TargetID:  3,
	}
	if err := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              rpcErr,
		finishedAt:       now,
	}); err != nil {
		t.Fatalf("handleOperationError=%v, want nil", err)
	}
	if _, ok := r.state.FmlFlowerTakeDailyLimitReached(now); !ok {
		t.Fatal("daily limit should be marked")
	}
	if !r.state.FmlFlowerTakeExhausted(now) {
		t.Fatal("take should be exhausted after tips8")
	}
	if _, cooling := r.operationCoolingDown(&automation.PlannedOp{
		Kind:        clientproto.RPCFmlFlowerShareTake.String(),
		Lane:        automation.LaneSide,
		Domain:      "union.flower.take",
		CooldownKey: "union.flower.take",
		TargetUID:   999,
		TargetID:    1,
	}, now.Add(time.Minute)); !cooling {
		t.Fatal("shared union.flower.take cooldown should block other take targets")
	}
}

func TestIsResidentOrderDailyLimitError(t *testing.T) {
	err := errors.New("rpc orderFlower.finishOrder: server: 今日完成订单次数已达上限")
	if !isResidentOrderDailyLimitError(clientproto.RPCOrderFlowerFinishOrder.String(), err) {
		t.Fatal("isResidentOrderDailyLimitError = false, want true")
	}
	if !isResidentOrderDailyLimitError(clientproto.RPCOrderFlowerFinishSatinOrder.String(), err) {
		t.Fatal("isResidentOrderDailyLimitError = false for satin, want true")
	}
	if !isResidentOrderDailyLimitError(clientproto.RPCOrderFlowerFinishDecorateOrder.String(), err) {
		t.Fatal("isResidentOrderDailyLimitError = false for decorate, want true")
	}
	if isResidentOrderDailyLimitError(clientproto.RPCOrderCustomerFinishOrder.String(), err) {
		t.Fatal("isResidentOrderDailyLimitError matched the wrong rpc")
	}
}

func TestHandleResidentSatinDecorateDailyLimitMarksState(t *testing.T) {
	err := errors.New("rpc orderFlower.finishSatinOrder: server: 今日完成订单次数已达上限")
	now := time.Date(2026, 7, 5, 20, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	wantUntil := state.NextCalendarDayReset(now)

	r := newOperationEventTestRunner()
	satinKey := clientproto.RPCOrderFlowerFinishSatinOrder.String() + "|order.resident.satin|finish"
	r.operationCooldowns[satinKey] = operationCooldown{
		Until:  now.Add(61 * time.Second),
		Reason: "服务端提示订单冷却中",
	}
	satin := &automation.PlannedOp{
		Kind:     clientproto.RPCOrderFlowerFinishSatinOrder.String(),
		Category: automation.CategoryOrder,
		Domain:   "order.resident.satin",
		Action:   "finish",
		Lane:     automation.LaneSide,
	}
	if err := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: satin},
		err:              err,
		finishedAt:       now,
	}); err != nil {
		t.Fatalf("handleOperationError satin=%v, want nil", err)
	}
	until, ok := r.state.ResidentSatinDailyLimitReached(now)
	if !ok {
		t.Fatal("satin daily limit should be marked in state")
	}
	if !until.Equal(wantUntil) {
		t.Fatalf("satin limit until=%s, want next calendar day %s", until, wantUntil)
	}
	if _, ok := r.operationCooldowns[satinKey]; ok {
		t.Fatal("satin retry timer should be closed after daily limit")
	}
	if _, cooling := r.operationCoolingDown(satin, now.Add(time.Minute)); cooling {
		t.Fatal("satin should not keep a retry cooldown after daily limit")
	}

	r = newOperationEventTestRunner()
	decorate := &automation.PlannedOp{
		Kind:     clientproto.RPCOrderFlowerFinishDecorateOrder.String(),
		Category: automation.CategoryOrder,
		Domain:   "order.resident.decorate",
		Action:   "finish",
		Lane:     automation.LaneSide,
	}
	if err := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: decorate},
		err:              err,
		finishedAt:       now,
	}); err != nil {
		t.Fatalf("handleOperationError decorate=%v, want nil", err)
	}
	if _, ok := r.state.ResidentDecorateDailyLimitReached(now); !ok {
		t.Fatal("decorate daily limit should be marked in state")
	}
	if _, cooling := r.operationCoolingDown(decorate, now.Add(time.Minute)); cooling {
		t.Fatal("decorate should not keep a retry cooldown after daily limit")
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

func TestFlowerArtMaterialRejectedItemID(t *testing.T) {
	err := errors.New(`rpc flowerArt.makeFlowerArt: server: {"code":301,"param":{"iid":23022}}`)
	if got := flowerArtMaterialRejectedItemID(clientproto.RPCFlowerArtMakeFlowerArt.String(), err); got != 23022 {
		t.Fatalf("flowerArtMaterialRejectedItemID=%d, want 23022", got)
	}
	if !isFlowerArtMaterialRejectedError(clientproto.RPCFlowerArtMakeFlowerArt.String(), err) {
		t.Fatal("isFlowerArtMaterialRejectedError = false, want true")
	}
	if isFlowerArtMaterialRejectedError(clientproto.RPCFlowerRackSell.String(), err) {
		t.Fatal("isFlowerArtMaterialRejectedError matched the wrong rpc")
	}
	if isFlowerArtMaterialRejectedError(clientproto.RPCFlowerArtMakeFlowerArt.String(), errors.New(`{"code":301,"param":{"iid":0}}`)) {
		t.Fatal("isFlowerArtMaterialRejectedError matched iid 0")
	}
	rpcErr := &babigame.RPCServerError{
		Name:     clientproto.RPCFlowerArtMakeFlowerArt,
		Envelope: babigame.WSResponseD{M: json.RawMessage(`{"code":301,"param":{"iid":23022}}`)},
	}
	if got := flowerArtMaterialRejectedItemID(clientproto.RPCFlowerArtMakeFlowerArt.String(), rpcErr); got != 23022 {
		t.Fatalf("RPCServerError itemID=%d, want 23022", got)
	}
}

func TestClassifyOperationError(t *testing.T) {
	cases := []struct {
		name string
		kind string
		err  error
		want operationErrorKind
	}{
		{
			name: "pearl hire candidate gold fallback",
			kind: clientproto.RPCPearlPlaceHire.String(),
			err:  &pearlHireCandidateFallbackError{TicketSpent: true},
			want: operationErrorPearlHireCandidateFallback,
		},
		{
			name: "harvest not mature",
			kind: clientproto.RPCUsrLandHarvest.String(),
			err:  errors.New("rpc usrLand.harvest: server: 鲜花尚未成熟"),
			want: operationErrorHarvestNotMature,
		},
		{
			name: "resident order cooldown",
			kind: clientproto.RPCOrderFlowerFinishOrder.String(),
			err:  errors.New("rpc orderFlower.finishOrder: server: 冷却中"),
			want: operationErrorResidentOrderCooldown,
		},
		{
			name: "resident order daily limit",
			kind: clientproto.RPCOrderFlowerFinishOrder.String(),
			err:  errors.New("rpc orderFlower.finishOrder: server: 今日完成订单次数已达上限"),
			want: operationErrorResidentOrderDailyLimit,
		},
		{
			name: "waterwheel invalid data",
			kind: clientproto.RPCWaterwheelRecv.String(),
			err:  errors.New("rpc waterwheel.recv: server: 数据有误"),
			want: operationErrorWaterwheelInvalidData,
		},
		{
			name: "waterwheel daily limit",
			kind: clientproto.RPCWaterwheelRecv.String(),
			err:  errors.New("rpc waterwheel.recv: server: 已达到领取上限"),
			want: operationErrorWaterwheelDailyLimit,
		},
		{
			name: "water drop rejected",
			kind: clientproto.RPCUsrLandWaterBatch.String(),
			err:  errors.New(`rpc usrLand.waterBatch: server: {"code":301,"param":{"iid":7}}`),
			want: operationErrorWaterDropRejected,
		},
		{
			name: "flower art material rejected",
			kind: clientproto.RPCFlowerArtMakeFlowerArt.String(),
			err:  errors.New(`rpc flowerArt.makeFlowerArt: server: {"code":301,"param":{"iid":23022}}`),
			want: operationErrorFlowerArtMaterialRejected,
		},
		{
			name: "customer finish material rejected",
			kind: clientproto.RPCOrderCustomerFinishOrder.String(),
			err:  errors.New(`rpc orderCustomer.finishOrder: server: {"code":301,"param":{"iid":300505}}`),
			want: operationErrorFlowerArtMaterialRejected,
		},
		{
			name: "task group finished",
			kind: clientproto.RPCTaskDlyRecv.String(),
			err:  errors.New("rpc taskDly.recv: server: 本组任务已经完结"),
			want: operationErrorTaskGroupFinished,
		},
		{
			name: "fml flower take daily limit",
			kind: clientproto.RPCFmlFlowerShareTake.String(),
			err:  errors.New(`rpc fmlFlowerShare.take: server: {"code":"fmlShare_tips8","msg":"今日拿取次数已达上限","args":[]}`),
			want: operationErrorFmlFlowerTakeDailyLimit,
		},
		{
			name: "fml enter account has no guild",
			kind: clientproto.RPCFmlEnter.String(),
			err:  errors.New(`rpc fml.enter: server: {"code":109,"args":[]}`),
			want: operationErrorFmlNotJoined,
		},
		{
			name: "race account has no guild",
			kind: clientproto.RPCFmlRaceGetTaskList.String(),
			err:  errors.New("rpc fmlRace.getTaskList: server: 您还未加入任何公会"),
			want: operationErrorFmlNotJoined,
		},
		{
			name: "mail already picked",
			kind: clientproto.RPCMailPick.String(),
			err:  errors.New("rpc mail.pick: server: 邮件附件已领取"),
			want: operationErrorMailAlreadyPicked,
		},
		{
			name: "mail has nothing left to pick",
			kind: clientproto.RPCMailPick.String(),
			err:  errors.New("rpc mail.pick: server: 不存在可以领取的邮件"),
			want: operationErrorMailAlreadyPicked,
		},
		{
			name: "mail non-to-pick error code",
			kind: clientproto.RPCMailPick.String(),
			err:  errors.New("rpc mail.pick: server: mail_nonToPick"),
			want: operationErrorMailAlreadyPicked,
		},
		{
			name: "mail already-picked error code",
			kind: clientproto.RPCMailPick.String(),
			err:  errors.New("rpc mail.pick: server: mail_alreadyPick"),
			want: operationErrorMailAlreadyPicked,
		},
		{
			name: "ordinary failure",
			kind: clientproto.RPCFreeWaterRecv.String(),
			err:  errors.New("rpc freeWater.recv: server busy"),
			want: operationErrorOrdinary,
		},
		{
			name: "nil error",
			kind: clientproto.RPCFreeWaterRecv.String(),
			err:  nil,
			want: operationErrorOrdinary,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyOperationError(tc.kind, tc.err); got != tc.want {
				t.Fatalf("classifyOperationError()=%s, want %s", got, tc.want)
			}
		})
	}
}

func TestHandleOperationErrorFlowerArtMaterialRejected(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	r := newOperationEventTestRunner()
	r.state.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"23022": 5}}},
	})
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCFlowerArtMakeFlowerArt.String(),
		Lane:     automation.LaneSide,
		Category: automation.CategoryFlowerArt,
		Domain:   automation.GoalFlowerArt,
		Action:   "craft",
		ItemID:   300504,
		Count:    1,
	}
	err := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              errors.New(`rpc flowerArt.makeFlowerArt: server: {"code":301,"param":{"iid":23022}}`),
		finishedAt:       now,
	})
	if err != nil {
		t.Fatalf("handleOperationError=%v, want nil", err)
	}
	if got := r.state.Inventory()[23022]; got != 0 {
		t.Fatalf("Inventory[23022]=%d, want 0 after material rejection", got)
	}
}

func TestHandleOperationErrorCustomerFinishMaterialRejected(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	r := newOperationEventTestRunner()
	r.state.ApplyVMap(map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"300505": 1}}},
	})
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCOrderCustomerFinishOrder.String(),
		Lane:     automation.LaneSide,
		Category: automation.CategoryOrder,
		Domain:   automation.GoalCustomerOrder,
		Action:   "finish",
		TargetID: 7,
	}
	err := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              errors.New(`rpc orderCustomer.finishOrder: server: {"code":301,"param":{"iid":300505}}`),
		finishedAt:       now,
	})
	if err != nil {
		t.Fatalf("handleOperationError=%v, want nil", err)
	}
	if got := r.state.Inventory()[300505]; got != 0 {
		t.Fatalf("Inventory[300505]=%d, want 0 after finish shortage", got)
	}
}

func TestHandleOperationErrorMailAlreadyPicked(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	r := newOperationEventTestRunner()
	r.state.ApplyVMap(map[string]any{
		"19": map[string]any{
			"1": []any{
				map[string]any{"1": 101, "2": 201, "13": [][]int32{{1, 5}}, "20": 0},
			},
		},
	})
	op := &automation.PlannedOp{
		Kind:     clientproto.RPCMailPick.String(),
		Lane:     automation.LaneSide,
		Category: automation.CategoryBasic,
		Domain:   "basic.mail",
		Action:   "claim",
		TargetID: 101,
		ItemID:   201,
	}
	err := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              errors.New("rpc mail.pick: server: 邮件附件已领取"),
		finishedAt:       now,
	})
	if err != nil {
		t.Fatalf("handleOperationError=%v, want nil", err)
	}
	if got := r.state.ReadyMailPickTargets(); len(got) != 0 {
		t.Fatalf("ReadyMailPickTargets=%+v, want none after already-picked recovery", got)
	}
}

func TestHandleOperationErrorPearlHireCandidateFallback(t *testing.T) {
	now := time.Date(2026, 9, 4, 21, 51, 36, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	r := newOperationEventTestRunner()
	r.bus = NewBus()
	events, cancel := r.bus.SubscribeLive(1)
	defer cancel()
	op := &automation.PlannedOp{
		Kind:      clientproto.RPCPearlPlaceHire.String(),
		Lane:      automation.LaneSide,
		Category:  automation.CategoryBasic,
		Domain:    "basic.pearl",
		Action:    "hire",
		TargetID:  1,
		TargetUID: 2001,
		Count:     1,
		ItemCost:  map[int32]int32{1003: 1},
	}
	r.setSideOperationCooldown(op, now, errors.New("old failure"), "", time.Minute)
	err := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: op},
		err:              &pearlHireCandidateFallbackError{TicketSpent: true},
		finishedAt:       now,
	})
	if err != nil {
		t.Fatalf("handleOperationError=%v, want nil", err)
	}
	if _, coolingDown := r.operationCoolingDown(op, now.Add(time.Second)); coolingDown {
		t.Fatal("recognized candidate fallback retained an operation cooldown")
	}
	select {
	case event := <-events:
		if event.Kind != "operation_deferred" || event.Action != "blocked" || event.Level != "warn" {
			t.Fatalf("event=%+v, want warning deferred event", event)
		}
		if !strings.Contains(event.Message, "已消耗 1 张雇佣券") ||
			!strings.Contains(event.Message, "继续检查其他候选") ||
			!strings.Contains(event.Message, "未自动使用金币") {
			t.Fatalf("message=%q, want handled fallback details", event.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("missing pearl candidate fallback event")
	}
}

func TestHandleOperationErrorOutcomes(t *testing.T) {
	now := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	r := newOperationEventTestRunner()
	harvestErr := errors.New("rpc usrLand.harvest: server: 鲜花尚未成熟")
	harvestOp := &automation.PlannedOp{
		Kind:     clientproto.RPCUsrLandHarvest.String(),
		Lane:     automation.LaneFarm,
		Category: "plant",
		Domain:   "plant",
		Action:   "harvest",
		LandIDs:  []int32{1001},
	}

	err := r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: harvestOp},
		err:              harvestErr,
		finishedAt:       now,
	})
	if err != nil {
		t.Fatalf("handleOperationError(harvest)=%v, want nil", err)
	}
	if until := r.harvestBlockedUntil[1001]; !until.Equal(now.Add(harvestRetryWait)) {
		t.Fatalf("harvest blocked until=%s, want %s", until, now.Add(harvestRetryWait))
	}

	ordinaryErr := errors.New("server busy")
	sideOp := &automation.PlannedOp{
		Kind:     clientproto.RPCFreeWaterRecv.String(),
		Lane:     automation.LaneSide,
		Category: "basic",
		Domain:   "basic",
		Action:   "free_water",
	}
	err = r.handleOperationError(context.Background(), operationResult{
		operationAttempt: operationAttempt{op: sideOp},
		err:              ordinaryErr,
		finishedAt:       now,
	})
	if !errors.Is(err, ordinaryErr) {
		t.Fatalf("handleOperationError(ordinary)=%v, want original error", err)
	}
	if _, ok := r.operationCoolingDown(sideOp, now.Add(time.Second)); !ok {
		t.Fatal("ordinary side operation should enter cooldown")
	}
}

func newOperationEventTestRunner() *Runner {
	return &Runner{
		account:             &store.Account{ID: 1, Name: "test"},
		log:                 slog.New(slog.NewTextHandler(io.Discard, nil)),
		state:               state.New(),
		harvestBlockedUntil: map[int32]time.Time{},
		operationCooldowns:  map[string]operationCooldown{},
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

func TestEmitResidentOrderLimitInfoLogsOnce(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	st := state.New()
	st.NoteResidentOrderFinished(now, nil)
	st.NoteResidentOrderFinished(now, nil)
	policy := automation.DefaultPolicy()
	policy.Order.Resident.NormalEnabled = true
	policy.Order.Resident.NormalDailyLimit = 2

	bus := NewBus()
	ch, cancel := bus.SubscribeLive(8)
	defer cancel()
	r := &Runner{
		account: &store.Account{ID: 1, Name: "test"},
		log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		state:   st,
		policy:  policy,
		bus:     bus,
	}

	r.emitResidentOrderLimitInfo(policy, now)
	r.emitResidentOrderLimitInfo(policy, now)

	select {
	case event := <-ch:
		if event.Kind != "operation_deferred" || event.Domain != "order.resident" {
			t.Fatalf("event=%+v, want deferred resident limit", event)
		}
		if !strings.Contains(event.Message, "普通居民订单今日已完成") || !strings.Contains(event.Message, "2/2") {
			t.Fatalf("message=%q, want policy limit details", event.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("missing resident limit log event")
	}
	select {
	case event := <-ch:
		t.Fatalf("unexpected duplicate limit log: %+v", event)
	case <-time.After(50 * time.Millisecond):
	}
}
