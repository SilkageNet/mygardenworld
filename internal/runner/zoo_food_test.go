package runner

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func foodTestState() *state.State {
	s := state.New()
	s.ApplyVMap(map[string]any{
		"7":  map[string]any{"0": map[string]any{"44": 5000, "32": map[string]any{"1501": 0, "1502": 3}}},
		"33": map[string]any{"1": map[string]any{"1": map[string]any{"1": 1, "4": []int32{}}}},
		"20": map[string]any{"0": map[string]any{"9": map[string]any{"1": 9, "3": time.Now().UnixMilli(), "12": map[string]any{"90001": 0}}}},
	})
	return s
}

func TestZooFoodRejectionRecovery(t *testing.T) {
	for _, tt := range []struct {
		name, serverError string
		recover           bool
		refreshFails      bool
		refreshSparse     bool
	}{
		{"short fish stock", `{"code":301,"param":{"iid":1502}}`, true, false, false},
		{"refresh failure", `{"code":301,"param":{"iid":1502}}`, true, true, false},
		{"refresh missing bowl", `{"code":301,"param":{"iid":1502}}`, true, false, true},
		{"unrelated item", `{"code":301,"param":{"iid":1501}}`, false, false, false},
		{"ambiguous timeout", "timeout", false, false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := foodTestState()
			op := &automation.PlannedOp{Kind: clientproto.RPCZooAddFoodstuff.String(), TargetID: 1, ItemID: 1502, Count: 3, ItemCost: map[int32]int32{1502: 3}}
			calls, refreshes := 0, 0
			_, err := executeZooFoodStock(context.Background(), s, op,
				func(context.Context, clientproto.ZooAddFoodstuffRequest) (json.RawMessage, error) {
					calls++
					return nil, errors.New(tt.serverError)
				},
				func(context.Context) error {
					refreshes++
					if tt.refreshFails {
						return errors.New("refresh unavailable")
					}
					if tt.refreshSparse {
						return nil
					}
					s.ApplyVMap(map[string]any{"33": map[string]any{"1": map[string]any{"1": map[string]any{"4": []int32{}}}}})
					return nil
				})
			var rejected *zooFoodRejectedError
			if errors.As(err, &rejected) != tt.recover || calls != 1 {
				t.Fatalf("err=%v calls=%d", err, calls)
			}
			if !tt.recover {
				if refreshes != 0 || s.Inventory()[1502] != 3 {
					t.Fatal("unrelated/ambiguous rejection mutated state")
				}
				return
			}
			if refreshes != 1 || s.Inventory()[1502] != 3 || s.ZooFoodUsableCount(1502) != 0 || rejected.StockBefore != 3 || rejected.Requested != 3 {
				t.Fatalf("recovery=%+v", rejected)
			}
			bowlUnknown := tt.refreshFails || tt.refreshSparse
			if (rejected.RefreshError != "") != bowlUnknown {
				t.Fatalf("bowl unknown=%v refresh error=%q", bowlUnknown, rejected.RefreshError)
			}
			r := newOperationEventTestRunner()
			r.state = s
			if err := r.handleOperationError(context.Background(), operationResult{operationAttempt: operationAttempt{op: op}, err: rejected, finishedAt: time.Now()}); err != nil {
				t.Fatalf("rejection should be deferred, not fatal: %v", err)
			}
			p := automation.DefaultPolicy()
			p.AutomationEnabled = true
			p.Basic.Zoo.Enabled, p.Basic.Zoo.AutoFeed, p.Basic.Zoo.AutoBuyFood = true, true, true
			p.Basic.Zoo.MaxSpendGold = 200
			buy := false
			for _, planned := range automation.BuildPlan(s, p, time.Now()).Operations {
				if planned.Kind == clientproto.RPCZooAddFoodstuff.String() && planned.Executable {
					t.Fatal("repeated rejected stock")
				}
				if planned.Kind == clientproto.RPCShopBuy.String() && planned.Executable {
					buy = true
					if planned.ItemID != 90001 || planned.Count != 2 || planned.GoldCost != 200 {
						t.Fatalf("unbudgeted purchase %+v", planned)
					}
				}
			}
			if buy == bowlUnknown {
				t.Fatalf("buy=%v bowlUnknown=%v", buy, bowlUnknown)
			}
			p.Basic.Zoo.AutoBuyFood = false
			for _, planned := range automation.BuildPlan(s, p, time.Now()).Operations {
				if planned.Kind == clientproto.RPCShopBuy.String() && planned.Executable {
					t.Fatal("recovery enabled buying without permission")
				}
			}
			if bowlUnknown {
				s.ApplyVMap(map[string]any{"33": map[string]any{"1": map[string]any{"1": map[string]any{"2": 12}}}})
				if s.ZooPets()[1].FoodstuffObserved {
					t.Fatal("unrelated pet delta restored bowl")
				}
				return
			}
			// A purchase/other authoritative inventory update restores feeding.
			s.ApplyVMap(map[string]any{"7": map[string]any{"2": map[string]any{"2": map[string]any{"1501": 2}}}})
			food, ok := s.NextZooFoodstuffPlan()
			if !ok || food.FoodstuffID != 1501 || food.Count != 2 {
				t.Fatalf("restored plan=%+v", food)
			}
		})
	}
}

func TestZooFoodPreflightRejectsStaleBowlAndPolicy(t *testing.T) {
	s := foodTestState()
	p := automation.DefaultPolicy()
	p.Basic.Zoo.Enabled, p.Basic.Zoo.AutoFeed = true, true
	op := &automation.PlannedOp{Kind: clientproto.RPCZooAddFoodstuff.String(), TargetID: 1, ItemID: 1502, Count: 3, ItemCost: map[int32]int32{1502: 3}}
	if err := automation.ValidateZooFoodStock(s, p.Basic.Zoo, op); err != nil {
		t.Fatal(err)
	}
	s.InvalidateZooFoodBowl(1)
	if err := automation.ValidateZooFoodStock(s, p.Basic.Zoo, op); err == nil {
		t.Fatal("unobserved bowl allowed")
	}
	s.ApplyVMap(map[string]any{"33": map[string]any{"1": map[string]any{"1": map[string]any{"4": make([]int32, state.ZooFoodBowlCapacity())}}}})
	if err := automation.ValidateZooFoodStock(s, p.Basic.Zoo, op); err == nil {
		t.Fatal("full bowl allowed")
	}
	p.Basic.Zoo.AutoFeed = false
	if err := automation.ValidateZooFoodStock(foodTestState(), p.Basic.Zoo, op); err == nil {
		t.Fatal("disabled feeding allowed")
	}
}
