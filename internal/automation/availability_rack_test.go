package automation

import (
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func TestBestRackArtByCountPrefersHighestStock(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"300208": 5,
			"301612": 20,
		}}},
		"102": map[string]any{"0": map[string]any{
			"3002": map[string]any{"1": 3002},
			"3016": map[string]any{"1": 3016},
		}},
	})
	ledger := NewInventoryLedger(s.Inventory())

	artID, count, ok := bestRackArtByCount(s, &pb.FlowerArtPolicy{}, ledger)
	if !ok {
		t.Fatal("expected rack art")
	}
	if artID != 301612 || count != 12 {
		t.Fatalf("bestRackArtByCount()=%d,%d want 301612,12", artID, count)
	}
}

func TestBestRackArtByCountComparesUncappedStock(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"300208": 12,
			"301612": 20,
		}}},
		"102": map[string]any{"0": map[string]any{
			"3002": map[string]any{"1": 3002},
			"3016": map[string]any{"1": 3016},
		}},
	})
	ledger := NewInventoryLedger(s.Inventory())

	artID, count, ok := bestRackArtByCount(s, &pb.FlowerArtPolicy{}, ledger)
	if !ok {
		t.Fatal("expected rack art")
	}
	if artID != 301612 || count != flowerRackPerSlotCount {
		t.Fatalf("bestRackArtByCount()=%d,%d want highest-stock 301612,%d", artID, count, flowerRackPerSlotCount)
	}
}

func TestBestRackArtByCountRespectsSellArtIDs(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"300208": 20,
			"301612": 5,
		}}},
		"102": map[string]any{"0": map[string]any{
			"3002": map[string]any{"1": 3002},
			"3016": map[string]any{"1": 3016},
		}},
	})
	ledger := NewInventoryLedger(s.Inventory())
	policy := &pb.FlowerArtPolicy{SellArtIds: []int32{300208}}

	artID, count, ok := bestRackArtByCount(s, policy, ledger)
	if !ok {
		t.Fatal("expected rack art")
	}
	if artID != 300208 || count != 12 {
		t.Fatalf("bestRackArtByCount()=%d,%d want selected 300208,12", artID, count)
	}
}

func TestBestRackArtByCountFallsBackWhenPreferredEmpty(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"300208": 20,
		}}},
		"102": map[string]any{"0": map[string]any{
			"3002": map[string]any{"1": 3002},
			"3016": map[string]any{"1": 3016},
		}},
	})
	ledger := NewInventoryLedger(s.Inventory())
	policy := &pb.FlowerArtPolicy{SellArtIds: []int32{301612}}

	artID, count, ok := bestRackArtByCount(s, policy, ledger)
	if !ok {
		t.Fatal("expected fallback rack art")
	}
	if artID != 300208 || count != 12 {
		t.Fatalf("bestRackArtByCount()=%d,%d want fallback 300208,12", artID, count)
	}
}

func TestBestRackArtByCountFiltersLockedVase(t *testing.T) {
	s := state.New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"300208": 5,
			"301612": 20,
		}}},
		"102": map[string]any{"0": map[string]any{
			"3002": map[string]any{"1": 3002},
		}},
	})
	ledger := NewInventoryLedger(s.Inventory())

	artID, count, ok := bestRackArtByCount(s, &pb.FlowerArtPolicy{}, ledger)
	if !ok {
		t.Fatal("expected rack art")
	}
	if artID != 300208 || count != 5 {
		t.Fatalf("bestRackArtByCount()=%d,%d want unlocked-vase art 300208,5", artID, count)
	}
}
