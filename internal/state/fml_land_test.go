package state

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestFmlLandHarvestUsesCurrentStockAndCalculationTime(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	// Level 0 produces one flower every 900 seconds, with capacity 6.
	for _, tt := range []struct {
		name    string
		land    FmlLandView
		pending int32
		next    int64
	}{
		{"stock is not lifetime yield", FmlLandView{FlowerID: 23001, MatureFlowerCnt: 6, HarvestedCnt: 100}, 6, 0},
		{"known stock without timing", FmlLandView{FlowerID: 23001, MatureFlowerCnt: 2, HarvestedCnt: 100}, 2, 0},
		{"production resumes after harvest", FmlLandView{FlowerID: 23001, StartTimeMs: now.Add(-24 * time.Hour).UnixMilli(), LastCalcTimeMs: now.Add(-15 * time.Minute).UnixMilli(), HarvestedCnt: 100}, 1, 0},
		{"recent harvest must not replay planting age", FmlLandView{FlowerID: 23001, StartTimeMs: now.Add(-24 * time.Hour).UnixMilli(), LastCalcTimeMs: now.Add(-5 * time.Minute).UnixMilli(), HarvestedCnt: 3}, 0, now.Add(10 * time.Minute).UnixMilli()},
		{"add new production to stored stock", FmlLandView{FlowerID: 23001, StartTimeMs: now.Add(-24 * time.Hour).UnixMilli(), LastCalcTimeMs: now.Add(-30 * time.Minute).UnixMilli(), MatureFlowerCnt: 3, HarvestedCnt: 100}, 5, 0},
		{"cap available stock", FmlLandView{FlowerID: 23001, LastCalcTimeMs: now.Add(-45 * time.Minute).UnixMilli(), MatureFlowerCnt: 5, HarvestedCnt: 100}, 6, 0},
		{"initial planting uses start time", FmlLandView{FlowerID: 23001, StartTimeMs: now.Add(-45 * time.Minute).UnixMilli()}, 3, 0},
		{"future anchor", FmlLandView{FlowerID: 23001, LastCalcTimeMs: now.Add(time.Minute).UnixMilli()}, 0, now.Add(16 * time.Minute).UnixMilli()},
		{"unknown catalog preserves stock", FmlLandView{Level: 99999, FlowerID: 23001, MatureFlowerCnt: 2, HarvestedCnt: 100}, 2, 0},
		{"empty land", FmlLandView{MatureFlowerCnt: 2}, 0, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := FmlLandPendingHarvest(tt.land, now); got != tt.pending {
				t.Fatalf("pending=%d, want %d", got, tt.pending)
			}
			if got := FmlLandNextMatureMs(tt.land, now); got != tt.next {
				t.Fatalf("next=%d, want %d", got, tt.next)
			}
		})
	}
}

func TestFmlLandHarvestResumesAcrossServerSnapshots(t *testing.T) {
	s := New()
	start := time.UnixMilli(1_800_000_000_000)
	for _, tt := range []struct {
		name             string
		at               time.Time
		stock, harvested int32
	}{
		{"first full harvest", start, 6, 0},
		{"second full harvest", start.Add(90 * time.Minute), 6, 6},
		{"many previous harvests", start.Add(24 * time.Hour), 6, 96},
	} {
		t.Run(tt.name, func(t *testing.T) {
			apply := func(stock, harvested int32) {
				s.ApplyV(json.RawMessage(fmt.Sprintf(`{"25":{"102":{"1":{"1":{"0":0,"1":23001,"2":%d,"3":%d,"4":%d,"5":%d}}}}}`, start.Add(-90*time.Minute).UnixMilli(), stock, harvested, tt.at.UnixMilli())))
			}
			apply(tt.stock, tt.harvested)
			if ids := s.ReadyFmlLandHarvestIDs(tt.at); len(ids) != 1 || ids[0] != 1 {
				t.Fatalf("full land not harvestable: %v", ids)
			}
			apply(0, tt.harvested+tt.stock)
			if ids := s.ReadyFmlLandHarvestIDs(tt.at); len(ids) != 0 {
				t.Fatalf("just-harvested land must wait: %v", ids)
			}
			if got := FmlLandNextMatureMs(s.FmlLands()[1], tt.at); got != tt.at.Add(15*time.Minute).UnixMilli() {
				t.Fatalf("next maturity=%d", got)
			}
			if ids := s.ReadyFmlLandHarvestIDs(tt.at.Add(15 * time.Minute)); len(ids) != 1 || ids[0] != 1 {
				t.Fatalf("next production not harvestable: %v", ids)
			}
		})
	}
}
