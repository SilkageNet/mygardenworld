package state

import "testing"

func TestRejectedZooFoodUsesOnlyConfirmedStock(t *testing.T) {
	for _, tt := range []struct {
		name, payload     string
		usable, inventory int32
	}{
		{"no update", `{}`, 0, 8},
		{"unrelated", `{"7":{"2":{"2":{"1501":4}}}}`, 0, 8},
		{"partial actual stock", `{"7":{"2":{"2":{"1502":2}}}}`, 2, 2},
		{"confirmed same count", `{"7":{"2":{"2":{"1502":8}}}}`, 8, 8},
		{"only positive delta", `{"7":{"2":{"0":{"1502":2}}}}`, 2, 10},
		{"negative delta is not refreshed stock", `{"7":{"2":{"0":{"1502":-3}}}}`, 0, 5},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := New()
			s.ApplyV([]byte(`{"7":{"0":{"32":{"1502":8}}},"33":{"1":{"1":{"1":1,"4":[]}}}}`))
			changes := 0
			s.SetOnInventoryChange(func(InventorySnapshot) { changes++ })
			s.RejectZooFoodStock(1502)
			if changes != 0 || s.Inventory()[1502] != 8 {
				t.Fatal("rejection fabricated consumption")
			}
			s.ApplyV([]byte(tt.payload))
			if s.ZooFoodUsableCount(1502) != tt.usable || s.Inventory()[1502] != tt.inventory {
				t.Fatalf("usable=%d inventory=%d", s.ZooFoodUsableCount(1502), s.Inventory()[1502])
			}
		})
	}
}
