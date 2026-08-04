package state

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

const cyclicStoryFixtureNowMs int64 = 1783696000000

func TestCyclicStoryCatalogConfig(t *testing.T) {
	cfg, ok := CyclicStoryCatalogConfig()
	if !ok {
		t.Fatal("CyclicStoryCatalogConfig ok=false")
	}
	if cfg.TmpType != 4003 || cfg.Name != "莳花纪闻" || cfg.CurrencyItemID != 1108 {
		t.Fatalf("CyclicStoryCatalogConfig=%+v", cfg)
	}
}

func TestCyclicStoryOrderInfoByID(t *testing.T) {
	info := CyclicStoryOrderInfoByID(1)
	if !info.CatalogKnown || info.OrderID != 1 || info.Group != 1 || info.Cost != 80 ||
		len(info.Reward) != 1 || info.Reward[0] != (ItemCount{ItemID: 1108, Count: 8}) {
		t.Fatalf("CyclicStoryOrderInfoByID(1)=%+v", info)
	}
	unknown := CyclicStoryOrderInfoByID(999999)
	if unknown.CatalogKnown || unknown.OrderID != 999999 {
		t.Fatalf("unknown=%+v", unknown)
	}
}

func TestCyclicStoryCaptureFixture(t *testing.T) {
	s := applyCyclicStoryCaptureFixture(t)
	view, ok := s.CyclicStoryView(time.UnixMilli(cyclicStoryFixtureNowMs))
	if !ok || !view.Observed || !view.Found || !view.Valid {
		t.Fatalf("CyclicStoryView=(%+v,%t), want observed valid activity", view, ok)
	}
	if view.BatchID != 9101 || view.TmpID != 40030001 || view.TmpType != 4003 || view.Status != 1 || view.Phase != 2 {
		t.Fatalf("activity identity/phase=%+v", view)
	}
	if view.Name != "莳花纪闻1期" || view.Description != "九色灵鹿(7.7-7.20)" || view.Score != 45 ||
		view.CurrencyItemID != 1108 || view.CurrencyBalance != 12 || view.FinishCount != 3 ||
		view.ExpOrderNum != 1 || view.LastRefreshTimeMs != 1783695955911 {
		t.Fatalf("activity summary=%+v", view)
	}
	if !view.OrdersObserved || !view.MilestoneReceiptsObserved || !view.FinishCountObserved || !view.ExpOrderNumObserved {
		t.Fatalf("observed flags=%+v", view)
	}
	if !reflect.DeepEqual(view.ClaimedMilestoneIndexes, []int32{1}) || view.Bag[1108] != 12 {
		t.Fatalf("bag/claimed=%v/%v", view.Bag, view.ClaimedMilestoneIndexes)
	}
	if len(view.Orders) != 3 {
		t.Fatalf("orders=%+v, want 3 slots", view.Orders)
	}
	if view.Orders[0].OrderIdx != 0 || view.Orders[0].OrderID != 1 || view.Orders[0].FlowerID != 23001 ||
		!view.Orders[0].CatalogKnown || view.Orders[0].Cost != 80 || view.Orders[0].OnCooldown {
		t.Fatalf("order0=%+v", view.Orders[0])
	}
	if view.Orders[1].OrderID != 2 || view.Orders[1].Cost != 90 || !view.Orders[1].CatalogKnown {
		t.Fatalf("order1=%+v", view.Orders[1])
	}
	if view.Orders[2].OrderID != 0 || !view.Orders[2].OnCooldown {
		t.Fatalf("order2 cooldown=%+v", view.Orders[2])
	}
	if len(view.Milestones) != 3 || view.Milestones[0].Index != 1 || view.Milestones[0].Target != 40 ||
		!view.Milestones[0].Received || view.Milestones[1].Target != 80 || view.Milestones[1].Received ||
		view.Milestones[2].Target != 160 || view.Milestones[2].Received {
		t.Fatalf("milestones=%+v", view.Milestones)
	}
}

func TestCyclicStorySparseMergePreservesAbsentFields(t *testing.T) {
	s := applyCyclicStoryCaptureFixture(t)
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"11":50,"14":{"106":{"1":4}}}},"1":{"40030001":{"2":"更新后的活动说明"}}}}`))

	view, ok := s.CyclicStoryView(time.UnixMilli(cyclicStoryFixtureNowMs))
	if !ok || !view.Valid {
		t.Fatalf("CyclicStoryView=(%+v,%t), want valid after sparse delta", view, ok)
	}
	if view.Score != 50 || view.FinishCount != 4 || view.LastRefreshTimeMs != 1783695955911 ||
		view.Name != "莳花纪闻1期" || view.Description != "更新后的活动说明" || len(view.Orders) != 3 {
		t.Fatalf("sparse merge lost fields: %+v", view)
	}
}

func TestCyclicStoryOrderTimestampsAcceptMilliseconds(t *testing.T) {
	now := time.UnixMilli(cyclicStoryFixtureNowMs)
	raw := json.RawMessage(`{"0":1,"1":23001,"2":1783695000000,"3":0}`)
	order, ok := decodeCyclicStoryOrder(raw)
	if !ok || order.OrderTime != 1783695000000 || order.ValidTime != 0 {
		t.Fatalf("decodeCyclicStoryOrder=%+v ok=%t", order, ok)
	}
	if cyclicStoryTimestampAfter(1783695000, now) { // seconds in the past relative to fixture now
		t.Fatal("past second timestamp should not be after now")
	}
	if !cyclicStoryTimestampAfter(1783698000000, now) {
		t.Fatal("future millisecond timestamp should be after now")
	}
}

func TestCyclicStoryEnterWithoutScoreBag(t *testing.T) {
	raw, err := os.ReadFile("testdata/cyclic_story_activity.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	batch := payload["23"].(map[string]any)["0"].(map[string]any)["9101"].(map[string]any)
	delete(batch, "11")
	delete(batch, "12")
	delete(batch, "13")
	delete(batch, "14")
	stripped, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal stripped fixture: %v", err)
	}
	s := New()
	s.ApplyV(stripped)
	now := time.UnixMilli(cyclicStoryFixtureNowMs)
	view, ok := s.CyclicStoryView(now)
	if !ok || view.Valid || !view.EnterReady || view.BatchID != 9101 {
		t.Fatalf("CyclicStoryView=(%+v,%t), want EnterReady without Valid", view, ok)
	}
	snapshot, ready := s.CyclicStoryEnterSnapshot(now)
	if !ready || snapshot.BatchID != 9101 || snapshot.Phase != 2 {
		t.Fatalf("CyclicStoryEnterSnapshot=(%+v,%t)", snapshot, ready)
	}
}

func TestCyclicStoryEnterAndMilestoneSnapshots(t *testing.T) {
	s := applyCyclicStoryCaptureFixture(t)
	now := time.UnixMilli(cyclicStoryFixtureNowMs)
	if snapshot, ok := s.CyclicStoryEnterSnapshot(now); ok {
		t.Fatalf("enter should be blocked when orders already observed: %+v", snapshot)
	}

	batch := s.activityBatches[9101]
	batch.Story = cyclicStoryActivityState{}
	enterSnapshot, ok := s.CyclicStoryEnterSnapshot(now)
	if !ok || enterSnapshot.BatchID != 9101 || enterSnapshot.Phase != 2 {
		t.Fatalf("CyclicStoryEnterSnapshot=(%+v,%t)", enterSnapshot, ok)
	}
	if s.CyclicStoryEnterApplied(enterSnapshot) {
		t.Fatal("enter applied before orders observed")
	}
	batch.Story.OrdersObserved = true
	batch.Story.OrdersValid = true
	batch.Story.Orders = map[int32]cyclicStoryOrderState{}
	if !s.CyclicStoryEnterApplied(enterSnapshot) {
		t.Fatal("enter applied expected after orders observed")
	}

	s = applyCyclicStoryCaptureFixture(t)
	if _, ready := s.CyclicStoryMilestoneClaimSnapshot(now, 9101, 2); ready {
		t.Fatal("milestone 2 should not be ready at score 45")
	}
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"11":80}}}}`))
	snapshot, ok := s.CyclicStoryMilestoneClaimSnapshot(now, 9101, 2)
	if !ok || snapshot.MilestoneIndex != 2 || snapshot.Target != 80 {
		t.Fatalf("CyclicStoryMilestoneClaimSnapshot=(%+v,%t)", snapshot, ok)
	}
	if s.CyclicStoryMilestoneClaimApplied(snapshot) {
		t.Fatal("milestone applied before receipt")
	}
	s.ApplyV(json.RawMessage(`{"23":{"0":{"9101":{"13":[1,2]}}}}`))
	if !s.CyclicStoryMilestoneClaimApplied(snapshot) {
		t.Fatal("milestone applied expected after receipt")
	}
}

func TestCyclicStoryOrderClaimRequiresInventory(t *testing.T) {
	s := applyCyclicStoryCaptureFixture(t)
	now := time.UnixMilli(cyclicStoryFixtureNowMs)
	if got, ready := s.CyclicStoryOrderClaimSnapshot(now, 9101, 0); ready {
		t.Fatalf("order claim without inventory must fail: %+v", got)
	}
	s.ApplyV(json.RawMessage(`{"7":{"0":{"32":{"23001":80}}}}`))
	snapshot, ok := s.CyclicStoryOrderClaimSnapshot(now, 9101, 0)
	if !ok || snapshot.OrderID != 1 || snapshot.FlowerID != 23001 || snapshot.Cost != 80 {
		t.Fatalf("CyclicStoryOrderClaimSnapshot=(%+v,%t)", snapshot, ok)
	}
	if s.CyclicStoryOrderClaimApplied(snapshot) {
		t.Fatal("order claim applied before slot change")
	}
	batch := s.activityBatches[9101]
	batch.Story.Orders[0] = cyclicStoryOrderState{OrderID: 3, FlowerID: 23003, OrderTime: 1, ValidTime: 0, Valid: true}
	batch.Story.FinishCount = 4
	if !s.CyclicStoryOrderClaimApplied(snapshot) {
		t.Fatal("order claim applied expected after order replacement")
	}
}

func TestCyclicStoryActiveOrderRespectsValidTimeCooldown(t *testing.T) {
	s := applyCyclicStoryCaptureFixture(t)
	now := time.UnixMilli(cyclicStoryFixtureNowMs)
	s.ApplyV(json.RawMessage(`{"7":{"0":{"32":{"23001":80}}}}`))

	// Fresh active orders carry refreshCd as a future validTime even with orderId>0.
	futureValid := now.UnixMilli() + 25*60*1000
	s.ApplyV(json.RawMessage(fmt.Sprintf(
		`{"23":{"0":{"9101":{"14":{"106":{"0":{"0":{"0":1,"1":23001,"2":%d,"3":%d}}}}}}}}`,
		now.UnixMilli(), futureValid,
	)))

	view, ok := s.CyclicStoryView(now)
	if !ok || len(view.Orders) == 0 {
		t.Fatalf("CyclicStoryView=(%+v,%t)", view, ok)
	}
	order0 := view.Orders[0]
	if order0.OrderID != 1 || !order0.OnCooldown {
		t.Fatalf("active order with future validTime must be on cooldown: %+v", order0)
	}
	if got, ready := s.CyclicStoryOrderClaimSnapshot(now, 9101, 0); ready {
		t.Fatalf("claim must wait for validTime: %+v", got)
	}

	later := time.UnixMilli(futureValid + 1)
	viewLater, ok := s.CyclicStoryView(later)
	if !ok || len(viewLater.Orders) == 0 || viewLater.Orders[0].OnCooldown {
		t.Fatalf("order should leave cooldown after validTime: %+v ok=%t", viewLater, ok)
	}
	if _, ready := s.CyclicStoryOrderClaimSnapshot(later, 9101, 0); !ready {
		t.Fatal("claim should be ready once validTime elapsed")
	}
}

func applyCyclicStoryCaptureFixture(t *testing.T) *State {
	t.Helper()
	raw, err := os.ReadFile("testdata/cyclic_story_activity.json")
	if err != nil {
		t.Fatalf("read cyclic-story fixture: %v", err)
	}
	s := New()
	s.ApplyV(raw)
	return s
}
