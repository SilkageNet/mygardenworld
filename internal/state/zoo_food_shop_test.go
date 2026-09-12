package state

import (
	"encoding/json"
	"testing"
	"time"
)

func TestZooFoodShopCatFoodOfferAndBuyPlan(t *testing.T) {
	offer, ok := zooFoodShopCatFoodStatic()
	if !ok || offer.ShopItemID != ZooFoodShopItemCatFood || offer.ItemID != ZooFoodItemCatFood || offer.CostItemID != 11 || offer.CostCount != 100 || offer.DailyLimit != 30 {
		t.Fatalf("static offer=%+v ok=%t", offer, ok)
	}

	s := New()
	s.ApplyV(json.RawMessage(`{
		"7":{"0":{"32":{},"44":1000}},
		"20":{"0":{"9":{"1":9,"3":1700000000000,"12":{"90001":2}}}},
		"33":{"0":{"0":1},"1":{"1":{"1":1,"3":0,"4":[],"5":2}}}
	}`))
	if !s.ZooFoodShopObserved() {
		t.Fatal("shop not observed")
	}
	view, ok := s.ZooFoodShopCatFoodOffer()
	if !ok || view.Bought != 2 || view.Remaining != 28 {
		t.Fatalf("offer view=%+v ok=%t", view, ok)
	}
	plan, ok := s.NextZooFoodBuyPlan()
	if !ok || plan.ShopItemID != ZooFoodShopItemCatFood || plan.Count <= 0 || plan.GoldCost != 100*plan.Count {
		t.Fatalf("buy plan=%+v ok=%t", plan, ok)
	}
	if plan.Count > 10 {
		t.Fatalf("buy count=%d exceeds per-tick cap", plan.Count)
	}
}

func TestZooFoodBuyPlanSkippedWhenInventoryCoversBowls(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{
		"7":{"0":{"32":{"1501":40},"44":1000}},
		"20":{"0":{"9":{"1":9,"12":{}}}},
		"33":{"0":{"0":1},"1":{"1":{"1":1,"3":0,"4":[],"5":2}}}
	}`))
	if plan, ok := s.NextZooFoodBuyPlan(); ok {
		t.Fatalf("unexpected buy plan=%+v", plan)
	}
}

func TestRandomEventNeedsEnterAfterRefreshHour(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"129":{"0":{"1":{}}}}`))
	loc := gameDayLocation()
	now := time.Now().In(loc)
	if s.RandomEventNeedsEnter(now) {
		t.Fatal("fresh sync should not need enter")
	}
	y, m, d := now.Date()
	next := time.Date(y, m, d, 20, 0, 0, 0, loc)
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	for _, hour := range []int{9, 14, 20} {
		candidate := time.Date(y, m, d, hour, 0, 0, 0, loc)
		if candidate.After(now) {
			next = candidate
			break
		}
	}
	if !s.RandomEventNeedsEnter(next.Add(time.Second)) {
		t.Fatalf("expected NeedsEnter after refresh at %s", next)
	}
}
