package state

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// helper: build a v-fragment from a Go map and apply it.
func applyMap(t *testing.T, s *State, top map[string]any) {
	t.Helper()
	raw, err := json.Marshal(top)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s.ApplyV(raw)
}

func TestApplyV_DiagnosticsTrackNamespacesAndNoble(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{
			"0": map[string]any{
				"36": 1,
				"37": 240,
			},
		},
		"777": map[string]any{"0": map[string]any{"1": "raw-only"}},
	})

	vip, vipExp := s.Vip()
	if vip != 1 || vipExp != 240 {
		t.Fatalf("Vip() = (%d,%d), want (1,240)", vip, vipExp)
	}
	if !s.NobleEligible() {
		t.Fatal("NobleEligible() = false, want true when vip is observed")
	}
	if got := s.ObservedNamespaces(); len(got) != 2 || got[0] != "7" || got[1] != "777" {
		t.Fatalf("ObservedNamespaces() = %v, want [7 777]", got)
	}
	if got := s.UnknownNamespaceCount(); got != 1 {
		t.Fatalf("UnknownNamespaceCount() = %d, want 1", got)
	}
}

func TestSpendableDiamondsUsesVisibleBalanceOnly(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{
			"0": map[string]any{
				"41": 598,
				"42": 3239,
			},
		},
	})

	visible, secondary := s.Diamonds()
	if visible != 598 || secondary != 3239 {
		t.Fatalf("Diamonds() = (%d,%d), want (598,3239)", visible, secondary)
	}
	if got := s.SpendableDiamonds(); got != 598 {
		t.Fatalf("SpendableDiamonds() = %d, want 598", got)
	}
}

func TestApplyV_UsrExtraTracksAntiFraudQA(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{
			"13": map[string]any{
				"1": map[string]any{
					"104": 1,
					"105": 1779290172000,
				},
			},
		},
	})

	extra := s.UsrExtra()
	if !extra.Observed || extra.AntiFraudQAStatus != 1 || extra.LastAntiFraudQATimeMs != 1779290172000 {
		t.Fatalf("UsrExtra()=%+v, want observed status=1 time=1779290172000", extra)
	}
	status, ok := s.AntiFraudQAStatus()
	if !ok || status != 1 {
		t.Fatalf("AntiFraudQAStatus()=(%d,%t), want (1,true)", status, ok)
	}

	applyMap(t, s, map[string]any{
		"7": map[string]any{
			"13": map[string]any{
				"1": map[string]any{"104": 2},
			},
		},
	})
	extra = s.UsrExtra()
	if extra.AntiFraudQAStatus != AntiFraudQAStatusClaimed || extra.LastAntiFraudQATimeMs != 1779290172000 {
		t.Fatalf("UsrExtra delta=%+v, want status=2 and preserved time", extra)
	}
}

func TestApplyV_ReputationTracksOwnScore(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{
			"17": map[string]any{
				"0": map[string]any{
					"0": 77900091102482,
					"1": 79,
					"3": 1779290000000,
					"4": 1779290100000,
					"5": 1779290200000,
					"6": 1779290300000,
				},
			},
		},
	})

	rep, ok := s.Reputation()
	if !ok || !rep.Observed || rep.UID != 77900091102482 || rep.Score != 79 ||
		rep.LastSyncTimeMs != 1779290000000 || rep.LastViewTimeMs != 1779290100000 ||
		rep.UTimeMs != 1779290200000 || rep.CTimeMs != 1779290300000 {
		t.Fatalf("Reputation()=(%+v,%t), want observed score=79 with timestamps", rep, ok)
	}

	applyMap(t, s, map[string]any{
		"7": map[string]any{
			"17": map[string]any{
				"0": map[string]any{"1": 100},
			},
		},
	})
	rep, ok = s.Reputation()
	if !ok || rep.Score != 100 || rep.UID != 77900091102482 {
		t.Fatalf("Reputation() after delta=(%+v,%t), want score=100 and preserved uid", rep, ok)
	}
}

func TestApplyV_ReputationRequiresObservedScore(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{
			"17": map[string]any{
				"0": map[string]any{"0": 77900091102482},
			},
		},
	})

	if rep, ok := s.Reputation(); ok || rep.Observed {
		t.Fatalf("Reputation()=(%+v,%t), want unobserved without score field", rep, ok)
	}
}

func TestApplyV_VideoDoubleState(t *testing.T) {
	s := New()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	applyMap(t, s, map[string]any{
		"118": map[string]any{
			"0": 77900091102482,
			"1": 2,
			"2": now.Add(time.Hour).UnixMilli(),
			"3": now.UnixMilli(),
			"4": now.Add(-time.Hour).UnixMilli(),
		},
	})

	view := s.VideoDouble()
	if !view.Observed || view.UID != 77900091102482 || view.VideoCount != 2 || view.EndTimeMs != now.Add(time.Hour).UnixMilli() {
		t.Fatalf("VideoDouble()=%+v", view)
	}
	if !s.VideoDoubleObserved() {
		t.Fatal("VideoDoubleObserved()=false, want true")
	}
	if !s.VideoDoubleActive(now) {
		t.Fatal("VideoDoubleActive()=false, want true before eTime")
	}
	if s.VideoDoubleActive(now.Add(2 * time.Hour)) {
		t.Fatal("VideoDoubleActive()=true, want false after eTime")
	}

	applyMap(t, s, map[string]any{
		"118": map[string]any{"1": 3},
	})
	view = s.VideoDouble()
	if view.VideoCount != 3 || view.EndTimeMs != now.Add(time.Hour).UnixMilli() {
		t.Fatalf("VideoDouble delta=%+v, want count updated and eTime preserved", view)
	}
}

func TestApplyV_ZooState(t *testing.T) {
	s := New()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	applyMap(t, s, map[string]any{
		"33": map[string]any{
			"0": map[string]any{
				"0": 77900091102482,
				"3": []int32{1, 2},
				"6": 120,
				"8": 1779290172000,
			},
			"1": map[string]any{
				"1": map[string]any{
					"0":  77900091102482,
					"1":  1,
					"2":  50,
					"3":  20,
					"4":  []int32{1501},
					"5":  2,
					"12": now.Add(-time.Minute).UnixMilli(),
				},
				"2": map[string]any{
					"1":  2,
					"2":  ZooMoodMax(),
					"3":  80,
					"4":  []int32{1502},
					"5":  5,
					"12": now.Add(time.Minute).UnixMilli(),
				},
			},
		},
	})

	if !s.ZooObserved() {
		t.Fatal("ZooObserved()=false, want true")
	}
	zoo := s.Zoo()
	if !zoo.Observed || zoo.UID != 77900091102482 || zoo.Comfort != 120 || len(zoo.PetIDs) != 2 {
		t.Fatalf("Zoo()=%+v", zoo)
	}
	pets := s.ZooPets()
	if len(pets) != 2 {
		t.Fatalf("ZooPets len=%d, want 2", len(pets))
	}
	if pets[1].MoodValue != 50 || pets[1].SatietyValue != 20 || len(pets[1].FoodstuffIDs) != 1 || pets[1].FoodstuffIDs[0] != 1501 {
		t.Fatalf("Zoo pet 1 = %+v", pets[1])
	}
	if !pets[1].MoodObserved || !pets[1].SatietyObserved || !pets[1].FoodstuffObserved || !pets[1].StatusObserved || !pets[1].StrokeCdTimeObserved {
		t.Fatalf("Zoo pet 1 observed fields = %+v", pets[1])
	}
	stroke := s.ReadyZooStrokePetIDs(now)
	if len(stroke) != 1 || stroke[0] != 1 {
		t.Fatalf("ReadyZooStrokePetIDs()=%v, want [1]", stroke)
	}

	applyMap(t, s, map[string]any{
		"33": map[string]any{
			"1": map[string]any{
				"1": map[string]any{"1": 1, "2": 70, "5": 2},
			},
		},
	})
	pets = s.ZooPets()
	if len(pets) != 2 || pets[1].MoodValue != 70 || len(pets[1].FoodstuffIDs) != 1 || pets[2].PetID != 2 {
		t.Fatalf("ZooPets after delta=%+v, want pet 2 preserved and pet 1 updated", pets)
	}
}

func TestZooSafeStockAndStatusRefresh(t *testing.T) {
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"1501": 40, "1502": 12}}},
		"33": map[string]any{
			"1": map[string]any{
				"2": map[string]any{"1": 2, "3": 20, "4": make([]int32, 29), "5": 2, "14": now.Add(time.Minute).UnixMilli()},
				"1": map[string]any{"1": 1, "3": 20, "4": []int32{}, "5": 2, "14": now.Add(-time.Minute).UnixMilli()},
				"3": map[string]any{"1": 3, "14": now.Add(-2 * time.Minute).UnixMilli()},
			},
		},
	})

	refresh := s.ReadyZooStatusRefreshPetIDs(now)
	if len(refresh) != 2 || refresh[0] != 1 || refresh[1] != 3 {
		t.Fatalf("ReadyZooStatusRefreshPetIDs()=%v, want [1 3]", refresh)
	}
	stock, ok := s.NextZooFoodstuffPlan()
	if !ok || stock.PetID != 1 || stock.FoodstuffID != 1501 || stock.Count != ZooFoodBowlCapacity() {
		t.Fatalf("NextZooFoodstuffPlan()=%+v ok=%t", stock, ok)
	}
	applyMap(t, s, map[string]any{"33": map[string]any{"1": map[string]any{
		"1": map[string]any{"1": 1, "4": make([]int32, ZooFoodBowlCapacity()-1)},
	}}})
	stock, ok = s.NextZooFoodstuffPlan()
	if !ok || stock.PetID != 1 || stock.FoodstuffID != 1501 || stock.Count != 1 {
		t.Fatalf("NextZooFoodstuffPlan() with one empty bowl slot=%+v ok=%t", stock, ok)
	}
}

func TestZooSafeStockUsesObservedBowlWithoutStatusOrSatiety(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"1501": 0, "1502": 4}}},
		"33": map[string]any{"1": map[string]any{
			"1": map[string]any{"1": 1, "4": []int32{}},
		}},
	})
	stock, ok := s.NextZooFoodstuffPlan()
	if !ok || stock != (ZooFoodstuffPlan{PetID: 1, FoodstuffID: 1502, Count: 4}) {
		t.Fatalf("NextZooFoodstuffPlan()=%+v ok=%t, want food 1502 x4 without status/satiety gate", stock, ok)
	}
	if got := s.ReadyZooStrokePetIDs(time.Now()); len(got) != 0 {
		t.Fatalf("ReadyZooStrokePetIDs()=%v from sparse zero values", got)
	}

	applyMap(t, s, map[string]any{"33": map[string]any{"1": map[string]any{
		"1": map[string]any{"1": 1, "3": 20, "5": 2},
	}}})
	stock, ok = s.NextZooFoodstuffPlan()
	if !ok || stock != (ZooFoodstuffPlan{PetID: 1, FoodstuffID: 1502, Count: 4}) {
		t.Fatalf("NextZooFoodstuffPlan()=%+v ok=%t, want food 1502 x4", stock, ok)
	}
}

func TestZooRootSparseDeltaPreservesObservedCollections(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{"33": map[string]any{"0": map[string]any{
		"0": 77900091102482, "3": []int32{1, 2}, "6": 120, "8": int64(100), "13": []int32{1, 2},
	}}})
	applyMap(t, s, map[string]any{"33": map[string]any{"0": map[string]any{"8": int64(200)}}})

	zoo := s.Zoo()
	if zoo.UID != 77900091102482 || zoo.Comfort != 120 || zoo.UpdatedAtMs != 200 {
		t.Fatalf("Zoo() after sparse root delta=%+v", zoo)
	}
	if len(zoo.PetIDs) != 2 || zoo.PetIDs[0] != 1 || zoo.PetIDs[1] != 2 || len(zoo.SouvenirRewardIDs) != 2 || zoo.SouvenirRewardIDs[0] != 1 || zoo.SouvenirRewardIDs[1] != 2 {
		t.Fatalf("Zoo() collections cleared by sparse root delta: %+v", zoo)
	}
}

func TestApplyV_FmlBuildState(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"0": map[string]any{
				"0":  88,
				"19": 2,
				"20": 1779290172000,
				"30": map[string]any{"1": 1, "2": 1},
			},
			"133": map[string]any{
				"1": 88,
				"4": 1779291172000,
				"5": map[string]any{"1": 1, "2": 2, "3": 0},
			},
		},
	})

	got := s.FmlBuild()
	if !got.Observed || !got.BuildCountsObserved {
		t.Fatalf("FmlBuild observed flags = %+v, want observed build counts", got)
	}
	if got.FmlID != 88 || got.TodayBuildNum != 2 || got.LastBuildTimeMs != 1779291172000 {
		t.Fatalf("FmlBuild scalar fields = %+v", got)
	}
	if got.BuildCounts[1] != 1 || got.BuildCounts[2] != 2 || got.BuildCounts[3] != 0 {
		t.Fatalf("FmlBuild counts = %+v", got.BuildCounts)
	}
}

func TestApplyV_FmlMembershipUsesCurrentMemberRecord(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"0": map[string]any{"0": 88},
			"1": map[string]any{"0": 77900091102482, "1": 88, "2": 2},
		},
	})
	got := s.FmlBuild()
	if !got.MembershipObserved || got.MemberFmlID != 88 || !got.MemberPositionObserved || got.MemberPosition != 2 {
		t.Fatalf("joined membership=%+v, want observed fid 88", got)
	}

	// Cached IFml data can remain after leaving. A null current-member record
	// is authoritative and must clear membership without trusting 25.0.
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"0": map[string]any{"0": 88},
			"1": nil,
		},
	})
	got = s.FmlBuild()
	if !got.MembershipObserved || got.MemberFmlID != 0 || got.MemberPositionObserved || got.MemberPosition != 0 {
		t.Fatalf("left membership=%+v, want observed no guild", got)
	}
}

func TestFinalizeFmlMembershipSnapshotMarksMissingMemberAsNoGuild(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"0":{},"111":{"0":1787658000000,"1":1}}}`))
	if got := s.FmlBuild(); got.MembershipObserved {
		t.Fatalf("sparse state must remain unknown before startup finalization: %+v", got)
	}
	s.FinalizeFmlMembershipSnapshot()
	got := s.FmlBuild()
	if !got.MembershipObserved || got.MemberFmlID != 0 {
		t.Fatalf("finalized membership=%+v, want observed no guild", got)
	}
}

func TestFinalizeFmlMembershipSnapshotUsesGuildRecordWhenMemberRecordMissing(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"0":{"0":88},"111":{"0":1787658000000,"1":1}}}`))
	if got := s.FmlBuild(); got.MembershipObserved {
		t.Fatalf("guild-only startup state must remain unknown before finalization: %+v", got)
	}

	s.FinalizeFmlMembershipSnapshot()
	got := s.FmlBuild()
	if !got.MembershipObserved || got.MemberFmlID != 88 || got.FmlID != 88 {
		t.Fatalf("finalized membership=%+v, want observed guild 88", got)
	}
}

func TestFmlMembershipSnapshotDoesNotReusePreviousConnectionGuild(t *testing.T) {
	s := New()
	s.ApplyV(json.RawMessage(`{"25":{"0":{"0":88},"1":{"0":77900091102482,"1":88,"2":1},"111":{"0":1787658000000,"1":1}}}`))
	if got := s.FmlBuild(); !got.MembershipObserved || got.MemberFmlID != 88 {
		t.Fatalf("initial membership=%+v", got)
	}

	s.BeginFmlMembershipSnapshot()
	if got := s.FmlBuild(); got.MembershipObserved || got.FmlID != 0 || got.MemberFmlID != 0 || got.MemberPositionObserved || got.MemberPosition != 0 {
		t.Fatalf("new connection retained old membership evidence: %+v", got)
	}
	// A no-guild baseline can still omit 25.1; finalization must not revive the
	// stale race snapshot or old guild ID from the preceding connection.
	s.ApplyV(json.RawMessage(`{"25":{"0":{}}}`))
	s.FinalizeFmlMembershipSnapshot()
	if got := s.FmlBuild(); !got.MembershipObserved || got.MemberFmlID != 0 || got.FmlID != 0 {
		t.Fatalf("new no-guild baseline=%+v", got)
	}
}

func TestApplyV_FmlLandState(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{"0": 2, "1": 23005, "2": 1779290000000, "3": 5, "4": 2, "5": 1779290100000},
					"2": map[string]any{"0": 1, "1": 23007, "3": 3, "4": 3},
				},
			},
		},
	})

	if !s.FmlLandObserved() {
		t.Fatal("FmlLandObserved()=false, want true")
	}
	lands := s.FmlLands()
	if len(lands) != 2 {
		t.Fatalf("FmlLands len=%d, want 2", len(lands))
	}
	if lands[1].FlowerID != 23005 || lands[1].MatureFlowerCnt != 5 || lands[1].HarvestedCnt != 2 {
		t.Fatalf("FmlLand #1 = %+v", lands[1])
	}
	now := time.UnixMilli(1_800_000_000_000) // after fixture startTime, stock-capped
	ready := s.ReadyFmlLandHarvestIDs(now)
	if len(ready) != 1 || ready[0] != 1 {
		t.Fatalf("ReadyFmlLandHarvestIDs()=%v, want [1]", ready)
	}
	reason := FormatFmlLandHarvestReason(lands, ready, now)
	// Stored mature=5 harvested=2, but startTime+c_fmlLandLvl yields stock-capped pending.
	if !strings.Contains(reason, "土地#1") || !strings.Contains(reason, "×8") {
		t.Fatalf("FormatFmlLandHarvestReason()=%q, want land and computed pending count", reason)
	}
}

func TestApplyV_FmlLandSparseStubMarksObserved(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"102": map[string]any{
				"0": 12345,
				"2": 1779290000000,
			},
		},
	})
	if !s.FmlLandObserved() {
		t.Fatal("FmlLandObserved()=false after sparse 25.102 stub, want true")
	}
	if len(s.FmlLands()) != 0 {
		t.Fatalf("sparse stub should keep empty land map, got %d", len(s.FmlLands()))
	}
}

func TestReadyFmlLandHarvestIDs_ComputesFromStartTimeWhenMatureCountStale(t *testing.T) {
	// Protocol often pushes matureFlwCnt=0 until the client recalculates from
	// startTime + c_fmlLandLvl.time/stock. Automation must not wait for that.
	now := time.UnixMilli(1_800_000_000_000)
	s := New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"102": map[string]any{
				"1": map[string]any{
					"1": map[string]any{
						"0": 0,
						"1": 23005,
						"2": now.Add(-45 * time.Minute).UnixMilli(), // 2700s / 900s = 3 flowers
						"3": 0,
						"4": 0,
					},
				},
			},
		},
	})

	ready := s.ReadyFmlLandHarvestIDs(now)
	if len(ready) != 1 || ready[0] != 1 {
		t.Fatalf("ReadyFmlLandHarvestIDs()=%v, want [1] with computed maturity", ready)
	}
	pending := FmlLandPendingHarvest(s.FmlLands()[1], now)
	if pending != 3 {
		t.Fatalf("FmlLandPendingHarvest()=%d, want 3", pending)
	}
}

func TestFmlLandNextMatureMs(t *testing.T) {
	now := time.UnixMilli(1_800_000_000_000)
	land := FmlLandView{
		LandID:      1,
		Level:       0,
		FlowerID:    23005,
		StartTimeMs: now.Add(-10 * time.Minute).UnixMilli(), // 600s / 900s = 0 pending
	}
	next := FmlLandNextMatureMs(land, now)
	want := land.StartTimeMs + 900_000
	if next != want {
		t.Fatalf("FmlLandNextMatureMs()=%d, want %d", next, want)
	}
	if FmlLandNextMatureMs(FmlLandView{LandID: 2}, now) != 0 {
		t.Fatal("empty land should report next=0")
	}
}

func TestApplyV_FmlForestEnergyState(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"127": map[string]any{
				"0": 77900091102482,
				"1": 88,
				"2": map[string]any{"1": 10, "2": "7"},
				"4": 1779290100000,
				"6": map[string]any{"1": 3},
				"7": 1779290000000,
				"8": map[string]any{
					"88": map[string]any{"1": 5, "2": 0},
					"99": map[string]any{"1": 4, "3": 2},
				},
			},
		},
	})

	if !s.FmlForestEnergyObserved() {
		t.Fatal("FmlForestEnergyObserved()=false, want true")
	}
	got := s.FmlForestEnergy()
	if got.UID != 77900091102482 || got.FmlID != 88 || got.UpdatedAtMs != 1779290100000 {
		t.Fatalf("FmlForestEnergy scalar fields = %+v", got)
	}
	if got.EnergyByType[1] != 10 || got.EnergyByType[2] != 7 || got.DailyEnergyByType[1] != 3 {
		t.Fatalf("FmlForestEnergy maps = %+v daily=%+v", got.EnergyByType, got.DailyEnergyByType)
	}
	if got.PendingTempEnergyByType[1] != 9 || got.PendingTempEnergyByType[3] != 2 || got.PendingTempEnergyTotal != 11 {
		t.Fatalf("FmlForestEnergy pending = %+v total=%d", got.PendingTempEnergyByType, got.PendingTempEnergyTotal)
	}
	ready := s.ReadyFmlForestEnergyTypes()
	if len(ready) != 2 || ready[0] != 1 || ready[1] != 3 {
		t.Fatalf("ReadyFmlForestEnergyTypes()=%v, want [1 3]", ready)
	}
}

func TestApplyV_FmlFlowerShareState(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"107": map[string]any{
				"0": 77900091102482,
				"1": map[string]any{
					"1": map[string]any{"0": 23005, "1": 10, "2": 3, "3": 1779290000000},
					"2": map[string]any{"0": 23007, "1": 10, "2": 0},
				},
				"2": 4,
				"3": 1779290100000,
			},
			"108": []any{
				map[string]any{
					"0": 77900091102483,
					"1": map[string]any{
						"1": map[string]any{"0": 23009, "1": 8, "2": 7},
						"2": map[string]any{"0": 23010, "1": 4, "2": 4},
					},
				},
				map[string]any{
					"0": 77900091102484,
					"1": map[string]any{
						"3": map[string]any{"0": 23011, "1": 6, "2": 1},
					},
				},
			},
		},
	})

	if !s.FmlFlowerShareObserved() || !s.OtherFmlFlowerSharesObserved() {
		t.Fatal("flower share observed flags = false, want true")
	}
	share := s.FmlFlowerShare()
	if share.UID != 77900091102482 || share.TdyTakeCnt != 4 || share.LastTakeTimeMs != 1779290100000 {
		t.Fatalf("FmlFlowerShare scalar fields = %+v", share)
	}
	if share.Slots[1].FlowerID != 23005 || share.Slots[1].TakeNum != 3 {
		t.Fatalf("FmlFlowerShare slot 1 = %+v", share.Slots[1])
	}
	rewards := s.ReadyFmlFlowerShareRewardSlotIDs()
	if len(rewards) != 1 || rewards[0] != 1 {
		t.Fatalf("ReadyFmlFlowerShareRewardSlotIDs()=%v, want [1]", rewards)
	}
	candidates := s.FmlFlowerTakeCandidates()
	if len(candidates) != 2 {
		t.Fatalf("FmlFlowerTakeCandidates len=%d, want 2: %+v", len(candidates), candidates)
	}
	if candidates[0].UID != 77900091102483 || candidates[0].SlotID != 1 || candidates[0].FlowerID != 23009 || candidates[0].Available != 1 {
		t.Fatalf("candidate[0]=%+v", candidates[0])
	}
	if candidates[1].UID != 77900091102484 || candidates[1].SlotID != 3 || candidates[1].FlowerID != 23011 || candidates[1].Available != 5 {
		t.Fatalf("candidate[1]=%+v", candidates[1])
	}
}

func TestFmlFlowerTakeDailyLimitSurvivesSparseShareDelta(t *testing.T) {
	s := New()
	now := time.Now()
	s.MarkFmlFlowerTakeDailyLimitReached(now)
	if !s.FmlFlowerTakeExhausted(now) {
		t.Fatal("expected exhausted after MarkFmlFlowerTakeDailyLimitReached")
	}
	if _, ok := s.FmlFlowerTakeDailyLimitReached(now); !ok {
		t.Fatal("expected daily limit mark")
	}
	before := s.FmlFlowerShare().TdyTakeCnt

	// Sparse 25.107 without field 2 must not wipe tdyTakeCnt / clear the cap.
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"107": map[string]any{
				"0": 77900091102482,
				"1": map[string]any{
					"1": map[string]any{"0": 23005, "1": 10, "2": 3},
				},
			},
		},
	})
	share := s.FmlFlowerShare()
	if share.TdyTakeCnt != before {
		t.Fatalf("tdyTakeCnt=%d after sparse delta, want preserved %d", share.TdyTakeCnt, before)
	}
	if _, ok := s.FmlFlowerTakeDailyLimitReached(now); !ok {
		t.Fatal("daily limit mark cleared by sparse 107 delta")
	}
	if !s.FmlFlowerTakeExhausted(now) {
		t.Fatal("exhausted cleared by sparse 107 delta")
	}
}

func TestFmlFlowerTakeExhausted_IgnoresInitTakeNumWhenGuildLimitUnobserved(t *testing.T) {
	s := New()
	now := FmlFlowerTakeWindowStart(time.Now()).Add(time.Hour)
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"107": map[string]any{
				"0": 77900091102482,
				"1": map[string]any{},
				"2": 1, // already took once today
				"3": now.UnixMilli(),
			},
		},
	})
	if s.FmlBuild().FlowerTakeCnt != 0 {
		t.Fatalf("FlowerTakeCnt=%d, want 0 (unobserved)", s.FmlBuild().FlowerTakeCnt)
	}
	if s.FmlFlowerTakeExhausted(now) {
		t.Fatal("must not treat tdyTakeCnt>=$initTakeNum as exhausted when FlowerTakeCnt is unobserved")
	}
}

func TestFmlFlowerTakeExhausted_UsesObservedGuildLimit(t *testing.T) {
	s := New()
	now := FmlFlowerTakeWindowStart(time.Now()).Add(time.Hour)
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"0": map[string]any{
				"0":   1001,
				"102": 3,
			},
			"107": map[string]any{
				"0": 1,
				"1": map[string]any{},
				"2": 2,
				"3": now.UnixMilli(),
			},
		},
	})
	if s.FmlFlowerTakeExhausted(now) {
		t.Fatal("tdy=2 limit=3 should not be exhausted")
	}
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"107": map[string]any{"0": 1, "2": 3, "3": now.UnixMilli()},
		},
	})
	if !s.FmlFlowerTakeExhausted(now) {
		t.Fatal("tdy=3 limit=3 should be exhausted")
	}
}

func TestNoteFmlFlowerShareTake_AdvancesDepletedSlot(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"25": map[string]any{
			"108": []any{
				map[string]any{
					"0": 77900091102484,
					"1": map[string]any{
						"2": map[string]any{"0": 23011, "1": 1, "2": 0},
					},
				},
			},
		},
	})
	if n := len(s.FmlFlowerTakeCandidates()); n != 1 {
		t.Fatalf("candidates=%d, want 1", n)
	}
	s.NoteFmlFlowerShareTake(77900091102484, 2)
	if n := len(s.FmlFlowerTakeCandidates()); n != 0 {
		t.Fatalf("after note candidates=%d, want 0", n)
	}
}

func TestApplyV_RosterPopulatesLands(t *testing.T) {
	// Cold-start `index.reLogin` shape: 100.0.1.<id> carries the full
	// per-land state for every land in the player's roster. We verify both
	// branches: an entry with content (-> populates LandView) and an empty
	// entry (-> observed-empty).
	s := New()
	applyMap(t, s, map[string]any{
		"100": map[string]any{
			"0": map[string]any{
				"0": 77900091102482, // role id
				"1": map[string]any{
					"1001": map[string]any{
						"0": 23001, "1": 2, "2": 4, "3": 1, "5": 1778914414973, "7": 1778914245642,
					},
					"1002": map[string]any{}, // empty -> observed-empty
				},
			},
		},
	})
	got := s.Lands()
	if !s.LandRosterObserved() {
		t.Fatalf("LandRosterObserved()=false, want true after 100.0.1")
	}
	if len(got) != 2 {
		t.Fatalf("want 2 lands, got %d", len(got))
	}
	l1 := got[1001]
	if !l1.Observed || l1.FlowerID != 23001 || l1.State != 2 || l1.NextTimeMs != 1778914414973 {
		t.Errorf("1001 mismatch: %+v", l1)
	}
	l2 := got[1002]
	if !l2.Observed || l2.FlowerID != 0 || l2.State != 0 {
		t.Errorf("1002 should be observed-empty, got %+v", l2)
	}
}

func TestApplyV_RosterReplacesStaleLands(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{},
			"1025": map[string]any{},
		}},
	})
	applyMap(t, s, map[string]any{
		"100": map[string]any{"0": map[string]any{"1": map[string]any{
			"1001": map[string]any{},
			"1002": map[string]any{},
		}}},
	})

	got := s.Lands()
	if _, ok := got[1025]; ok {
		t.Fatalf("stale land 1025 survived full roster replace: %+v", got)
	}
	if len(got) != 2 {
		t.Fatalf("len(Lands())=%d, want 2 after roster replace", len(got))
	}
}

func TestApplyV_PrimaryDeltaOverwrites(t *testing.T) {
	// usrLand.plant response: 100.1.<id> carries the new state for the
	// affected land. Existing roster state for the same land is replaced.
	s := New()
	applyMap(t, s, map[string]any{
		"100": map[string]any{
			"0": map[string]any{"1": map[string]any{
				"1001": map[string]any{"0": 23001, "1": 2, "5": 100},
			}},
		},
	})
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23005, "1": 1, "2": 1, "5": 0, "7": 999},
		}},
	})
	l := s.Lands()[1001]
	if l.FlowerID != 23005 || l.State != 1 || l.NextTimeMs != 0 || l.PlantTimeMs != 999 {
		t.Errorf("primary delta not applied: %+v", l)
	}
}

func TestApplyV_HarvestClearsToObservedEmpty(t *testing.T) {
	// usrLand.harvest response: 100.1.<id> = {}
	// We must keep the land in the map but flip flower/state to zero with
	// observed=true (so the automation engine knows it's plantable).
	s := New()
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 2, "5": 100, "7": 1},
		}},
	})
	applyMap(t, s, map[string]any{
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{},
		}},
	})
	l := s.Lands()[1001]
	if !l.Observed {
		t.Errorf("cleared land must remain observed: %+v", l)
	}
	if l.FlowerID != 0 || l.State != 0 || l.NextTimeMs != 0 {
		t.Errorf("cleared land has stale fields: %+v", l)
	}
}

func TestApplyV_OnChangeFiresOnDiffOnly(t *testing.T) {
	s := New()
	var seen []LandChange
	s.SetOnChange(func(c []LandChange) { seen = append(seen, c...) })

	apply := func(state int) {
		applyMap(t, s, map[string]any{
			"100": map[string]any{"1": map[string]any{
				"1001": map[string]any{"0": 23001, "1": state, "5": 0, "7": 1},
			}},
		})
	}
	apply(2)
	apply(2) // identical -> no callback
	apply(3) // diff -> callback fires
	if len(seen) != 2 {
		t.Errorf("expected 2 changes (initial + state 2->3), got %d: %+v", len(seen), seen)
	}
	if seen[1].After.State != 3 || seen[1].Before.State != 2 {
		t.Errorf("second change wrong: %+v", seen[1])
	}
}

func TestApplyV_InventoryOnlyTouchesCell32(t *testing.T) {
	// 7.0.32 is the resource map (flowers + materials). Other 7.0.<n>
	// fields are untouched. We pin the assertion to *known seed ids* so
	// future schema changes don't regress us into pulling in 7.0.41 etc.
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{
			"0": map[string]any{
				"32": map[string]any{"23001": 50, "23005": 12, "1001": 999}, // flower + non-flower
				"41": 522,                                                   // unrelated cell - must NOT show up
			},
		},
	})
	inv := s.Inventory()
	if inv[23001] != 50 {
		t.Errorf("23001 missing: %v", inv)
	}
	if inv[1001] != 999 {
		t.Errorf("non-flower 1001 dropped: %v", inv)
	}
	flowers := s.FlowerInventory()
	if flowers[23001] != 50 || flowers[23005] != 12 {
		t.Errorf("flower inventory wrong: %v", flowers)
	}
	if _, has := flowers[1001]; has {
		t.Errorf("FlowerInventory leaked non-seed item: %v", flowers)
	}
}

func TestApplyV_InventoryChangeCallback(t *testing.T) {
	s := New()
	var seen []InventorySnapshot
	s.SetOnInventoryChange(func(snap InventorySnapshot) {
		seen = append(seen, snap)
	})
	applyMap(t, s, map[string]any{
		"7": map[string]any{
			"0": map[string]any{
				"32": map[string]any{"23001": 50, "23005": 12},
			},
		},
	})
	if len(seen) != 1 {
		t.Fatalf("inventory callback count = %d, want 1", len(seen))
	}
	if seen[0].Inventory[23001] != 50 || seen[0].Inventory[23005] != 12 {
		t.Fatalf("inventory snapshot wrong: %+v", seen[0])
	}
	if len(seen[0].Changes) != 2 {
		t.Fatalf("inventory changes = %+v, want 2 entries", seen[0].Changes)
	}

	applyMap(t, s, map[string]any{
		"7": map[string]any{
			"2": map[string]any{
				"0": map[string]any{"23001": -5},
			},
		},
	})
	if len(seen) != 2 {
		t.Fatalf("inventory callback count after delta = %d, want 2", len(seen))
	}
	if seen[1].Inventory[23001] != 45 {
		t.Fatalf("inventory delta snapshot wrong: %+v", seen[1])
	}
}

func TestApplyV_WaterDropsTracksCurrentAndTotal(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"33": map[string]any{"7": map[string]any{"1": 130, "5": 1778914414973}},
		}},
	})
	waterDrops, total, nextMs := s.WaterDrops()
	if waterDrops != 0 || total != 130 || nextMs != 1778914414973 {
		t.Fatalf("metadata-only water drops got (%d,%d,%d), want (0,130,1778914414973)", waterDrops, total, nextMs)
	}

	applyMap(t, s, map[string]any{
		"7": map[string]any{"1": map[string]any{
			"13": 5,
			"14": 5,
		}},
	})
	waterDrops, total, nextMs = s.WaterDrops()
	if waterDrops != 5 || total != 130 || nextMs != 1778914414973 {
		t.Fatalf("cold fallback water drops got (%d,%d,%d), want (5,130,1778914414973)", waterDrops, total, nextMs)
	}

	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 12},
			"33": map[string]any{"7": map[string]any{"1": 130, "5": 1778914414999}},
		}},
	})
	waterDrops, total, nextMs = s.WaterDrops()
	if waterDrops != 12 || total != 130 || nextMs != 1778914414999 {
		t.Fatalf("inventory water drops got (%d,%d,%d), want (12,130,1778914414999)", waterDrops, total, nextMs)
	}

	applyMap(t, s, map[string]any{
		"7": map[string]any{"2": map[string]any{
			"0": map[string]any{"7": 65},
			"2": map[string]any{"7": 80},
		}},
	})
	waterDrops, total, nextMs = s.WaterDrops()
	if waterDrops != 80 || total != 130 || nextMs != 1778914414999 {
		t.Fatalf("delta water drops got (%d,%d,%d), want (80,130,1778914414999)", waterDrops, total, nextMs)
	}

	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 0},
			"33": map[string]any{"7": map[string]any{"1": 130, "5": 1778914415000}},
		}},
	})
	waterDrops, total, nextMs = s.WaterDrops()
	if waterDrops != 0 || total != 130 || nextMs != 1778914415000 {
		t.Fatalf("zero inventory water drops got (%d,%d,%d), want (0,130,1778914415000)", waterDrops, total, nextMs)
	}
}

func TestAvailableWaterDropsAfterRecoveryTimestamp(t *testing.T) {
	s := New()
	now := time.Now()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 3},
			"33": map[string]any{"7": map[string]any{"1": 130, "5": now.Add(-time.Minute).UnixMilli()}},
		}},
	})
	current, total, _ := s.WaterDrops()
	if current != 3 || total != 130 {
		t.Fatalf("WaterDrops got (%d,%d), want (3,130)", current, total)
	}
	available, total, _ := s.AvailableWaterDrops(now)
	if available != 4 || total != 130 {
		t.Fatalf("AvailableWaterDrops got (%d,%d), want (4,130)", available, total)
	}
}

func TestAvailableWaterDropsKeepsInventoryAboveCapacity(t *testing.T) {
	s := New()
	now := time.Now()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 10000},
			"33": map[string]any{"7": map[string]any{"1": 130}},
		}},
	})
	current, total, _ := s.WaterDrops()
	if current != 10000 || total != 130 {
		t.Fatalf("WaterDrops got (%d,%d), want (10000,130)", current, total)
	}
	available, total, _ := s.AvailableWaterDrops(now)
	if available != 10000 || total != 130 {
		t.Fatalf("AvailableWaterDrops got (%d,%d), want (10000,130)", available, total)
	}
	if !s.LockWaterDrops(5000, now) {
		t.Fatal("LockWaterDrops returned false for over-capacity inventory")
	}
	available, _, _ = s.AvailableWaterDrops(now)
	if available != 5000 {
		t.Fatalf("available after lock = %d, want 5000", available)
	}
}

func TestLockWaterDropsReducesAvailableUntilReleased(t *testing.T) {
	s := New()
	now := time.Now()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 6},
			"33": map[string]any{"7": map[string]any{"1": 130}},
		}},
	})

	if !s.LockWaterDrops(4, now) {
		t.Fatal("LockWaterDrops returned false, want true")
	}
	available, _, _ := s.AvailableWaterDrops(now)
	if available != 2 {
		t.Fatalf("available after reserve = %d, want 2", available)
	}
	if s.LockWaterDrops(3, now) {
		t.Fatal("LockWaterDrops allowed spending in-flight drops")
	}
	s.ReleaseWaterDropsLock(4)
	available, _, _ = s.AvailableWaterDrops(now)
	if available != 6 {
		t.Fatalf("available after release = %d, want 6", available)
	}
}

func TestRefreshWaterDropsMaterializesOneRecovery(t *testing.T) {
	s := New()
	now := time.Now()
	nextMs := now.Add(-time.Minute).UnixMilli()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 0},
			"33": map[string]any{"7": map[string]any{"1": 130, "5": nextMs}},
		}},
	})

	var resources []ResourceSnapshot
	var inventories []InventorySnapshot
	s.SetOnResourceChange(func(snap ResourceSnapshot) {
		resources = append(resources, snap)
	})
	s.SetOnInventoryChange(func(snap InventorySnapshot) {
		inventories = append(inventories, snap)
	})

	if !s.RefreshWaterDrops(now) {
		t.Fatal("RefreshWaterDrops returned false, want true")
	}
	waterDrops, total, gotNext := s.WaterDrops()
	wantNext := nextMs + waterDropRestoreIntervalMs()
	if waterDrops != 1 || total != 130 || gotNext != wantNext {
		t.Fatalf("WaterDrops after refresh got (%d,%d,%d), want (1,130,%d)", waterDrops, total, gotNext, wantNext)
	}
	if len(resources) != 1 || resources[0].WaterDrops != 1 {
		t.Fatalf("resource callbacks = %+v, want one snapshot with WaterDrops=1", resources)
	}
	if len(inventories) != 1 || len(inventories[0].Changes) != 1 || inventories[0].Changes[0].ItemID != 7 {
		t.Fatalf("inventory callbacks = %+v, want one item-7 change", inventories)
	}
	if s.RefreshWaterDrops(now.Add(time.Second)) {
		t.Fatal("RefreshWaterDrops applied the same server timestamp twice")
	}
}

func TestRefreshWaterDropsAdvancesContinuousRecovery(t *testing.T) {
	s := New()
	now := time.Now()
	interval := waterDropRestoreIntervalMs()
	nextMs := now.Add(-time.Duration(3*interval+1) * time.Millisecond).UnixMilli()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 0},
			"33": map[string]any{"7": map[string]any{"1": 5, "5": nextMs}},
		}},
	})

	if !s.RefreshWaterDrops(now) {
		t.Fatal("RefreshWaterDrops returned false, want true")
	}
	waterDrops, total, gotNext := s.WaterDrops()
	wantNext := nextMs + 4*interval
	if waterDrops != 4 || total != 5 || gotNext != wantNext {
		t.Fatalf("WaterDrops after continuous refresh got (%d,%d,%d), want (4,5,%d)", waterDrops, total, gotNext, wantNext)
	}
}

func TestMarkLandsWateredSpendsTrackedWaterDrops(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 3},
			"33": map[string]any{"7": map[string]any{"1": 130, "5": int64(0)}}}},
		"100": map[string]any{"1": map[string]any{
			"1001": map[string]any{"0": 23001, "1": 1},
			"1002": map[string]any{"0": 23001, "1": 1},
		}},
	})

	var changed ResourceSnapshot
	var landChanges []LandChange
	var inventoryChanged InventorySnapshot
	s.SetOnResourceChange(func(snap ResourceSnapshot) {
		changed = snap
	})
	s.SetOnChange(func(changes []LandChange) {
		landChanges = append(landChanges, changes...)
	})
	s.SetOnInventoryChange(func(snap InventorySnapshot) {
		inventoryChanged = snap
	})
	s.MarkLandsWatered([]int32{1001, 1002})

	waterDrops, _, _ := s.WaterDrops()
	if waterDrops != 1 {
		t.Fatalf("WaterDrops after local spend = %d, want 1", waterDrops)
	}
	if changed.WaterDrops != 1 {
		t.Fatalf("resource callback WaterDrops = %d, want 1", changed.WaterDrops)
	}
	lands := s.Lands()
	if lands[1001].State != 2 || lands[1002].State != 2 {
		t.Fatalf("lands were not marked watered: %+v", lands)
	}
	if len(landChanges) != 2 {
		t.Fatalf("land changes = %d, want 2", len(landChanges))
	}
	if inventoryChanged.Inventory[7] != 1 {
		t.Fatalf("inventory callback water item = %d, want 1", inventoryChanged.Inventory[7])
	}
}

func TestMarkWaterDropsExhaustedClearsStaleDrops(t *testing.T) {
	now := time.Now()
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 3},
			"33": map[string]any{"7": map[string]any{"1": 130, "5": now.Add(-time.Minute).UnixMilli()}}}},
	})
	if !s.LockWaterDrops(2, now) {
		t.Fatal("LockWaterDrops = false, want true before server rejection")
	}

	var changed ResourceSnapshot
	var inventoryChanged InventorySnapshot
	s.SetOnResourceChange(func(snap ResourceSnapshot) {
		changed = snap
	})
	s.SetOnInventoryChange(func(snap InventorySnapshot) {
		inventoryChanged = snap
	})
	s.MarkWaterDropsExhausted(now)

	waterDrops, _, nextMs := s.WaterDrops()
	if waterDrops != 0 {
		t.Fatalf("WaterDrops after exhausted = %d, want 0", waterDrops)
	}
	if nextMs <= now.UnixMilli() {
		t.Fatalf("WaterDrops nextMs = %d, want future timestamp after %d", nextMs, now.UnixMilli())
	}
	available, _, _ := s.AvailableWaterDrops(now)
	if available != 0 {
		t.Fatalf("AvailableWaterDrops = %d, want 0", available)
	}
	if changed.WaterDrops != 0 {
		t.Fatalf("resource callback WaterDrops = %d, want 0", changed.WaterDrops)
	}
	if inventoryChanged.Inventory[7] != 0 {
		t.Fatalf("inventory callback water item = %d, want 0", inventoryChanged.Inventory[7])
	}
}

func TestMarkInventoryItemExhaustedClearsStaleStock(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{"23022": 4, "23001": 2}}},
	})
	if got := s.Inventory()[23022]; got != 4 {
		t.Fatalf("Inventory[23022]=%d, want 4", got)
	}

	var inventoryChanged InventorySnapshot
	s.SetOnInventoryChange(func(snap InventorySnapshot) {
		inventoryChanged = snap
	})
	s.MarkInventoryItemExhausted(23022)

	inv := s.Inventory()
	if inv[23022] != 0 {
		t.Fatalf("Inventory[23022]=%d, want 0", inv[23022])
	}
	if inv[23001] != 2 {
		t.Fatalf("Inventory[23001]=%d, want 2 (untouched)", inv[23001])
	}
	if len(inventoryChanged.Changes) != 1 || inventoryChanged.Changes[0].ItemID != 23022 ||
		inventoryChanged.Changes[0].Before != 4 || inventoryChanged.Changes[0].After != 0 {
		t.Fatalf("inventory callback changes=%+v", inventoryChanged.Changes)
	}

	inventoryChanged = InventorySnapshot{}
	s.MarkInventoryItemExhausted(23022)
	if len(inventoryChanged.Changes) != 0 {
		t.Fatalf("second exhaustion should be a no-op, got changes=%+v", inventoryChanged.Changes)
	}
	s.MarkInventoryItemExhausted(0)
	if len(inventoryChanged.Changes) != 0 {
		t.Fatalf("itemID 0 should be a no-op, got changes=%+v", inventoryChanged.Changes)
	}
}

func TestWaterwheelCooldownUsesBucketCreateInterval(t *testing.T) {
	interval := waterwheelBucketCreateInterval()
	if interval <= 0 || interval >= time.Hour {
		t.Fatalf("waterwheelBucketCreateInterval = %s, want configured short interval", interval)
	}
	s := New()
	now := time.Now()
	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": 1,
			"4": now.Add(-interval - time.Second).UnixMilli(),
			"5": now.Add(-10 * interval).UnixMilli(),
		},
	})
	if s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = true immediately after observe, want false until local bucket is generated")
	}

	s.mu.Lock()
	s.wwEntered = true
	s.wwLocalGenMs = now.Add(-interval - time.Second).UnixMilli()
	s.mu.Unlock()
	if !s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = false, want true after local bucket interval")
	}

	s.mu.Lock()
	s.wwEntered = true
	s.wwLocalGenMs = now.UnixMilli()
	s.mu.Unlock()
	if s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = true, want false before next bucket interval")
	}
}

func TestWaterwheelUnavailableBackoffSuppressesReady(t *testing.T) {
	interval := waterwheelBucketCreateInterval()
	if interval <= 0 || interval >= time.Hour {
		t.Fatalf("waterwheelBucketCreateInterval = %s, want configured short interval", interval)
	}
	s := New()
	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": 1,
		},
	})
	s.mu.Lock()
	s.wwEntered = true
	s.wwLocalGenMs = time.Now().Add(-interval - time.Second).UnixMilli()
	s.mu.Unlock()
	if !s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = false, want initially ready")
	}

	s.MarkWaterwheelUnavailable(time.Now())
	if s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = true, want false during local backoff")
	}
	if s.WaterwheelEnterDue(time.Now()) {
		t.Fatal("WaterwheelEnterDue = true, want false during local backoff")
	}
	if !s.WaterwheelEnterDue(time.Now().Add(interval + time.Second)) {
		t.Fatal("WaterwheelEnterDue = false, want true after local backoff")
	}
}

func TestWaterwheelEnterCatchesUpFromLastRecvTimestamp(t *testing.T) {
	interval := waterwheelBucketCreateInterval()
	if interval <= 0 || interval >= time.Hour {
		t.Fatalf("waterwheelBucketCreateInterval = %s, want configured short interval", interval)
	}
	existMax := waterwheelBucketExistMax()
	if existMax <= 1 {
		t.Fatalf("waterwheelBucketExistMax = %d, want configured positive limit", existMax)
	}

	now := time.Now()
	s := New()
	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": 3,
			// Long idle since last claim — same signal BucketMgr uses via leaveSceneTime.
			"4": now.Add(-time.Duration(existMax+2) * interval).UnixMilli(),
		},
	})
	if s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = true before enter, want false")
	}

	s.MarkWaterwheelEntered(now)
	if !s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = false after enter with accrued idle time, want catch-up ready")
	}
	s.mu.RLock()
	got := s.waterwheelLocalBucketCountAtLocked(now, s.wwClaimedCount)
	s.mu.RUnlock()
	if got != existMax {
		t.Fatalf("local bucket count = %d, want existMax catch-up %d", got, existMax)
	}
}

func TestWaterwheelEnterWithoutPriorTimestampWaitsFullCreateCd(t *testing.T) {
	interval := waterwheelBucketCreateInterval()
	if interval <= 0 || interval >= time.Hour {
		t.Fatalf("waterwheelBucketCreateInterval = %s, want configured short interval", interval)
	}
	now := time.Now()
	s := New()
	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": 0,
		},
	})
	s.MarkWaterwheelEntered(now)
	if s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = true immediately after fresh enter, want false until create CD")
	}
	s.mu.Lock()
	s.wwLocalGenMs = now.Add(-interval - time.Second).UnixMilli()
	s.mu.Unlock()
	if !s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = false after create CD, want true")
	}
}

func TestWaterwheelReadyRequiresEnterLifecycle(t *testing.T) {
	interval := waterwheelBucketCreateInterval()
	if interval <= 0 || interval >= time.Hour {
		t.Fatalf("waterwheelBucketCreateInterval = %s, want configured short interval", interval)
	}
	now := time.Now()
	s := New()
	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": 1,
		},
	})
	s.mu.Lock()
	s.wwLocalGenMs = now.Add(-2 * interval).UnixMilli()
	s.mu.Unlock()
	if s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = true before waterwheel.enter, want false")
	}

	if !s.WaterwheelEnterDue(now) {
		t.Fatal("WaterwheelEnterDue = false before enter, want true")
	}
	s.MarkWaterwheelEntered(now)
	if s.WaterwheelEnterDue(now) {
		t.Fatal("WaterwheelEnterDue = true after enter, want false")
	}
	if s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = true immediately after enter, want false")
	}

	s.mu.Lock()
	s.wwLocalGenMs = now.Add(-interval - time.Second).UnixMilli()
	s.mu.Unlock()
	if !s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = false after entered local interval, want true")
	}
}

func TestWaterwheelNextClaimRequiresSkipUsesAdvList(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": 2,
			"2": []any{3, 8},
		},
	})
	if !s.WaterwheelNextClaimRequiresSkip() {
		t.Fatal("WaterwheelNextClaimRequiresSkip = false, want true for next advertised bucket")
	}

	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": 3,
		},
	})
	if s.WaterwheelNextClaimRequiresSkip() {
		t.Fatal("WaterwheelNextClaimRequiresSkip = true, want false when next bucket is not advertised")
	}
}

func TestWaterwheelCooldownUsesLocalBucketAndDailyLimit(t *testing.T) {
	interval := waterwheelBucketCreateInterval()
	if interval <= 0 || interval >= time.Hour {
		t.Fatalf("waterwheelBucketCreateInterval = %s, want configured short interval", interval)
	}
	now := time.Now()
	s := New()
	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": 2,
		},
	})
	s.mu.Lock()
	s.wwEntered = true
	s.wwLocalGenMs = now.Add(-2 * interval).UnixMilli()
	s.mu.Unlock()
	if !s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = false, want true with locally generated bucket")
	}

	max := waterwheelBucketDailyMax()
	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": max,
		},
	})
	s.mu.Lock()
	s.wwEntered = true
	s.wwLocalGenMs = now.Add(-2 * interval).UnixMilli()
	s.mu.Unlock()
	if s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = true, want false after daily max is reached")
	}
}

func TestWaterwheelDailyLimitErrorSuppressesClaimsUntilReset(t *testing.T) {
	interval := waterwheelBucketCreateInterval()
	if interval <= 0 || interval >= time.Hour {
		t.Fatalf("waterwheelBucketCreateInterval = %s, want configured short interval", interval)
	}
	max := waterwheelBucketDailyMax()
	if max <= 1 {
		t.Fatalf("waterwheelBucketDailyMax = %d, want configured positive limit", max)
	}
	now := time.Now()
	s := New()
	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": max - 1,
		},
	})
	s.MarkWaterwheelEntered(now.Add(-interval - time.Second))
	if !s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = false, want ready before the limit error")
	}

	s.MarkWaterwheelDailyLimitReached(now)
	if got := s.WaterwheelClaimedCount(); got != max {
		t.Fatalf("WaterwheelClaimedCount = %d, want daily max %d", got, max)
	}
	if s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = true, want false after daily limit error")
	}
	if s.WaterwheelEnterDue(now.Add(interval + time.Second)) {
		t.Fatal("WaterwheelEnterDue = true, want false while daily limit is recorded")
	}

	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": 0,
		},
	})
	if !s.WaterwheelEnterDue(now.Add(interval + time.Second)) {
		t.Fatal("WaterwheelEnterDue = false, want true after server count reset")
	}
}

func TestWaterwheelLocalDailyResetAfterStaleServerPush(t *testing.T) {
	interval := waterwheelBucketCreateInterval()
	if interval <= 0 || interval >= time.Hour {
		t.Fatalf("waterwheelBucketCreateInterval = %s, want configured short interval", interval)
	}

	// Use time.Now() as the base so WaterwheelCooldownReady's internal
	// time.Now() call is consistent with the test timeline.
	now := time.Now()
	s := New()
	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": 1,
		},
	})
	s.MarkWaterwheelEntered(now)

	// Simulate the daily limit being reached locally.
	s.MarkWaterwheelDailyLimitReached(now)
	if s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = true, want false at daily limit")
	}
	if s.WaterwheelEnterDue(now) {
		t.Fatal("WaterwheelEnterDue = true, want false at daily limit")
	}

	// Less than 24h later — still blocked.
	stillBlocked := now.Add(23 * time.Hour)
	if s.WaterwheelEnterDue(stillBlocked) {
		t.Fatal("WaterwheelEnterDue = true, want false before 24h window")
	}

	// More than 24h later — local reset kicks in even without a server push.
	afterReset := now.Add(25 * time.Hour)
	if !s.WaterwheelEnterDue(afterReset) {
		t.Fatal("WaterwheelEnterDue = false after 24h, want local daily reset")
	}
	if s.WaterwheelClaimedCount() != 0 {
		t.Fatalf("WaterwheelClaimedCount = %d after local reset, want 0", s.WaterwheelClaimedCount())
	}

	// Re-enter and backdate wwLocalGenMs so the bucket timer is ready
	// relative to the real time.Now() used inside WaterwheelCooldownReady.
	s.MarkWaterwheelEntered(time.Now())
	s.mu.Lock()
	s.wwLocalGenMs = time.Now().Add(-interval - time.Second).UnixMilli()
	s.mu.Unlock()
	if !s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = false after re-enter + bucket interval, want true")
	}
}

func TestResidentOrderDailyLimitErrorSuppressesUntilNextGameDay(t *testing.T) {
	now := time.Date(2026, 7, 5, 20, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s := New()
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{"20260705": map[string]any{"1": 20260705, "9": 1259}}},
	})

	s.MarkResidentOrderDailyLimitReached(now)
	until, ok := s.ResidentOrderDailyLimitReached(now)
	if !ok {
		t.Fatal("ResidentOrderDailyLimitReached = false, want true after server limit error")
	}
	if !until.After(now) {
		t.Fatalf("resident order limit until = %s, want future time", until)
	}

	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{"20260706": map[string]any{"1": 20260706, "9": 0}}},
	})
	if _, ok := s.ResidentOrderDailyLimitReached(now.Add(6 * time.Hour)); ok {
		t.Fatal("ResidentOrderDailyLimitReached = true, want false after statistics day reset")
	}
}

func TestNextGameDayResetUsesCurrentBoundaryBeforeZeroFive(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	beforeReset := time.Date(2026, 7, 6, 0, 2, 0, 0, shanghai)
	wantToday := time.Date(2026, 7, 6, 0, 5, 0, 0, shanghai)
	if got := NextGameDayReset(beforeReset); !got.Equal(wantToday) {
		t.Fatalf("NextGameDayReset before boundary=%s, want %s", got, wantToday)
	}
	afterReset := time.Date(2026, 7, 6, 0, 6, 0, 0, shanghai)
	wantTomorrow := time.Date(2026, 7, 7, 0, 5, 0, 0, shanghai)
	if got := NextGameDayReset(afterReset); !got.Equal(wantTomorrow) {
		t.Fatalf("NextGameDayReset after boundary=%s, want %s", got, wantTomorrow)
	}
}

func TestResidentSatinDecorateDailyLimitErrorSuppressesUntilNextCalendarDay(t *testing.T) {
	now := time.Date(2026, 7, 5, 20, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	s := New()

	s.MarkResidentSatinDailyLimitReached(now)
	until, ok := s.ResidentSatinDailyLimitReached(now)
	if !ok {
		t.Fatal("ResidentSatinDailyLimitReached = false, want true after server limit error")
	}
	wantUntil := NextCalendarDayReset(now)
	if !until.Equal(wantUntil) {
		t.Fatalf("satin limit until = %s, want %s", until, wantUntil)
	}
	if _, ok := s.ResidentSatinDailyLimitReached(wantUntil); ok {
		t.Fatal("ResidentSatinDailyLimitReached = true at reset boundary, want false")
	}

	s.MarkResidentDecorateDailyLimitReached(now)
	until, ok = s.ResidentDecorateDailyLimitReached(now)
	if !ok {
		t.Fatal("ResidentDecorateDailyLimitReached = false, want true after server limit error")
	}
	if !until.Equal(wantUntil) {
		t.Fatalf("decorate limit until = %s, want %s", until, wantUntil)
	}
	if _, ok := s.ResidentDecorateDailyLimitReached(wantUntil); ok {
		t.Fatal("ResidentDecorateDailyLimitReached = true at reset boundary, want false")
	}
}

func TestApplyV_FreeWaterTracksClaimedSlots(t *testing.T) {
	s := New()
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	openMorning := time.Date(2026, 7, 6, 11, 0, 30, 0, shanghai)
	idx, ok := s.NextFreeWaterIndex(openMorning)
	if !ok || idx != 0 {
		t.Fatalf("NextFreeWaterIndex before observe got (%d,%t), want (0,true) at window start", idx, ok)
	}

	applyMap(t, s, map[string]any{
		"117": map[string]any{
			"1": []int32{0},
			"2": openMorning.UnixMilli(),
		},
	})
	if idx, ok := s.NextFreeWaterIndex(openMorning); ok {
		t.Fatalf("NextFreeWaterIndex got (%d,true), want unavailable for already claimed slot 0", idx)
	}
	openEvening := time.Date(2026, 7, 6, 17, 0, 0, 0, shanghai)
	idx, ok = s.NextFreeWaterIndex(openEvening)
	if !ok || idx != 1 {
		t.Fatalf("NextFreeWaterIndex got (%d,%t), want (1,true) at evening window start", idx, ok)
	}
	if idx, ok := s.NextFreeWaterIndex(time.Date(2026, 7, 6, 12, 0, 0, 0, shanghai)); ok {
		t.Fatalf("NextFreeWaterIndex got (%d,true), want unavailable mid-morning after slot 0 claimed", idx)
	}
	unobserved := New()
	if idx, ok := unobserved.NextFreeWaterIndex(time.Date(2026, 7, 6, 11, 1, 0, 0, shanghai)); !ok || idx != 0 {
		t.Fatalf("NextFreeWaterIndex got (%d,%t), want (0,true) after first claim minute", idx, ok)
	}
	if idx, ok := unobserved.NextFreeWaterIndex(time.Date(2026, 7, 6, 12, 0, 0, 0, shanghai)); !ok || idx != 0 {
		t.Fatalf("NextFreeWaterIndex got (%d,%t), want (0,true) mid-window", idx, ok)
	}
	if idx, ok := s.NextFreeWaterIndex(time.Date(2026, 7, 6, 10, 50, 0, 0, shanghai)); ok {
		t.Fatalf("NextFreeWaterIndex got (%d,true), want unavailable before free-water window", idx)
	}
	if idx, ok := unobserved.NextFreeWaterIndex(time.Date(2026, 7, 6, 14, 0, 0, 0, shanghai)); ok {
		t.Fatalf("NextFreeWaterIndex got (%d,true), want unavailable at exclusive morning window end", idx)
	}
}

func TestBenefitBoxDrawsAccrueLocallyUntilMax(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	resetAt := time.Date(2026, 7, 29, 20, 0, 0, 0, shanghai)
	s := New()
	applyMap(t, s, map[string]any{
		"116": map[string]any{
			"0": map[string]any{
				"1": 0,
				"2": resetAt.UnixMilli(),
			},
		},
	})
	if got := s.BenefitBoxDrawsRemaining(resetAt); got != 0 {
		t.Fatalf("BenefitBoxDrawsRemaining at reset = %d, want 0", got)
	}
	if got := s.BenefitBoxDrawsRemaining(resetAt.Add(3 * time.Hour)); got != 3 {
		t.Fatalf("BenefitBoxDrawsRemaining after 3h = %d, want 3", got)
	}
	morning := time.Date(2026, 7, 30, 4, 30, 0, 0, shanghai)
	if got := s.BenefitBoxDrawsRemaining(morning); got != 8 {
		t.Fatalf("BenefitBoxDrawsRemaining at morning = %d, want capped 8", got)
	}
	if !s.BenefitBoxReady(morning) {
		t.Fatal("BenefitBoxReady at morning = false, want true after overnight accrual from drawCnt=0")
	}
}

func TestApplyV_ReadyDailyTaskIDs(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"22": map[string]any{
			"1": map[string]any{
				"1": map[string]any{"3016": 3},
				"3": map[string]any{"102": 1},
				"100": map[string]any{
					"101": map[string]any{"0": 101, "1": 10, "2": 10, "4": 1},
					"102": map[string]any{"0": 102, "1": 5, "2": 5, "4": 1},
					"103": map[string]any{"0": 30160001, "1": 8, "2": 0, "4": 0},
				},
			},
		},
	})
	ready := s.ReadyDailyTaskIDs()
	if len(ready) != 1 || ready[0] != 101 {
		t.Fatalf("ReadyDailyTaskIDs got %v, want [101]", ready)
	}
	tasks := s.DailyTasks()
	if tasks[102].Receipted != 1 || tasks[103].Finished != 3 {
		t.Fatalf("daily task copy mismatch: %+v", tasks)
	}
}

func TestApplyV_DailyTaskRecvMapUsesProgressTypeAndMergesPartialDelta(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
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
	ready := s.ReadyDailyTaskIDs()
	if len(ready) != 1 || ready[0] != 40001 {
		t.Fatalf("ReadyDailyTaskIDs got %v, want [40001]", ready)
	}

	applyMap(t, s, map[string]any{
		"22": map[string]any{
			"1": map[string]any{
				"3": map[string]any{"4": 1},
			},
		},
	})
	if ready := s.ReadyDailyTaskIDs(); len(ready) != 0 {
		t.Fatalf("ReadyDailyTaskIDs after recv delta got %v, want empty", ready)
	}
	tasks := s.DailyTasks()
	task := tasks[40001]
	if task.ProgressType != 4 || task.Receipted != 1 || task.Status != 3 {
		t.Fatalf("daily task after recv delta=%+v, want progress type 4 receipted status", task)
	}
}

func TestApplyV_ReadyWeeklyTaskIDs(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"22": map[string]any{
			"100": map[string]any{
				"1": map[string]any{
					"3026": 163,
					"3089": 999,
				},
				"3": map[string]any{
					"30260002": 1,
				},
			},
		},
	})
	ready := s.ReadyWeeklyTaskIDs()
	if len(ready) != 1 || ready[0] != 30260001 {
		t.Fatalf("ReadyWeeklyTaskIDs got %v, want [30260001]", ready)
	}
	tasks := s.WeeklyTasks()
	if tasks[30260001].Finished != 163 || tasks[30260002].Receipted != 1 || tasks[30260002].Status != 3 {
		t.Fatalf("weekly task copy mismatch: %+v", tasks)
	}

	applyMap(t, s, map[string]any{
		"22": map[string]any{
			"100": map[string]any{
				"3": map[string]any{
					"30260001": 1,
					"30260002": 1,
				},
			},
		},
	})
	tasks = s.WeeklyTasks()
	if tasks[30260001].Finished != 163 || tasks[30260001].Receipted != 1 || tasks[30260001].Status != 3 {
		t.Fatalf("weekly partial update lost progress: %+v", tasks[30260001])
	}
}

func TestApplyV_CustomerOrdersDoNotCreatePlantingDeficits(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 3,
			"23005": 10,
		}}},
		"109": map[string]any{
			"0": map[string]any{
				"1": map[string]any{
					"7": map[string]any{
						"0": [][]int32{{23001, 8}, {23005, 4}},
						"1": 7,
						"3": 1,
					},
				},
			},
		},
	})
	orders := s.CustomerOrders()
	if len(orders) != 1 || orders[0] != 7 {
		t.Fatalf("CustomerOrders got %v, want [7]", orders)
	}
	deficits := s.FlowerOrderDeficits()
	if len(deficits) != 0 {
		t.Fatalf("customer orders should not create planting deficits, got %v", deficits)
	}
}

func TestApplyV_CustomerOrderGenerationMetadata(t *testing.T) {
	s := New()
	now := time.Date(2026, 7, 3, 9, 0, 0, 0, time.Local)
	nextGen := now.Add(-2 * time.Second).UnixMilli()
	applyMap(t, s, map[string]any{
		"109": map[string]any{
			"0": map[string]any{
				"1": map[string]any{},
				"2": nextGen,
				"3": now.UnixMilli(),
				"4": now.Add(-time.Hour).UnixMilli(),
				"5": 3,
			},
		},
	})

	summary := s.CustomerOrderSummary()
	if !summary.Observed || summary.NextGenTimeMs != nextGen || summary.ActiveCount != 0 || summary.CreateCount != 3 {
		t.Fatalf("CustomerOrderSummary mismatch: %+v", summary)
	}
	if !s.CustomerOrderGenerationReady(now) {
		t.Fatalf("CustomerOrderGenerationReady()=false, want true")
	}

	applyMap(t, s, map[string]any{
		"109": map[string]any{"0": map[string]any{
			"1": map[string]any{},
			"2": now.Add(time.Minute).UnixMilli(),
		}},
	})
	if s.CustomerOrderGenerationReady(now) {
		t.Fatalf("CustomerOrderGenerationReady()=true during cooldown, want false")
	}

	applyMap(t, s, map[string]any{
		"109": map[string]any{"0": map[string]any{"1": map[string]any{
			"7": map[string]any{"0": 2, "1": 300208, "2": 1, "3": 1},
		}}},
	})
	if s.CustomerOrderGenerationReady(now) {
		t.Fatalf("CustomerOrderGenerationReady()=true with active order, want false")
	}
}

func TestApplyV_CustomerOrderMixedRequirementDeficits(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 3,
			"1402":  0,
		}}},
		"109": map[string]any{
			"0": map[string]any{
				"1": map[string]any{
					"7": map[string]any{
						"0": [][]int32{{23001, 8}, {1402, 2}},
						"1": 7,
					},
				},
			},
		},
	})
	details := s.CustomerOrderDetails()
	if got := details[7].Requires; len(got) != 1 || got[0].FlowerID != 23001 || got[0].Count != 8 {
		t.Fatalf("flower requirements = %+v, want 23001 x8", got)
	}
	if got := details[7].ItemRequires; len(got) != 1 || got[0].ItemID != 1402 || got[0].Count != 2 {
		t.Fatalf("item requirements = %+v, want 1402 x2", got)
	}
	deficits := s.FlowerOrderDeficits()
	if len(deficits) != 0 {
		t.Fatalf("mixed customer orders should not create planting deficits, got %v", deficits)
	}
}

func TestApplyV_CustomerOrderArtRequirements(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"300208": 3,
			"300207": 1,
		}}},
		"109": map[string]any{
			"0": map[string]any{
				"1": map[string]any{
					"1": map[string]any{"0": 2, "1": 300208, "2": 3, "3": 1, "4": 1778919337499},
					"3": map[string]any{"0": 2, "1": 300207, "2": 2, "3": 3, "4": 1778984660957},
				},
			},
		},
	})
	orders := s.CustomerOrders()
	if len(orders) != 2 || orders[0] != 1 || orders[1] != 3 {
		t.Fatalf("CustomerOrders got %v, want [1 3]", orders)
	}
	details := s.CustomerOrderDetails()
	if got := details[1].ItemRequires; len(got) != 1 || got[0].ItemID != 300208 || got[0].Count != 3 {
		t.Fatalf("NPC 1 item requirements = %+v, want 300208 x3", got)
	}
	if got := details[3].ItemRequires; len(got) != 1 || got[0].ItemID != 300207 || got[0].Count != 2 {
		t.Fatalf("NPC 3 item requirements = %+v, want 300207 x2", got)
	}
	deficits := s.FlowerOrderDeficits()
	if len(deficits) != 0 {
		t.Fatalf("art customer orders should not create planting deficits, got %v", deficits)
	}
}

func TestApplyV_FlowerRackTracksSlots(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"104": map[string]any{
			"0": map[string]any{
				"1": map[string]any{"1": 1, "2": 0, "3": 0, "4": nil, "5": 100},
				"3": map[string]any{"1": 3, "2": 300207, "3": 6, "4": 1779290145874, "5": 1779290145874},
			},
		},
	})
	slots := s.FlowerRackSlots()
	if len(slots) != 2 {
		t.Fatalf("FlowerRackSlots len=%d, want 2: %+v", len(slots), slots)
	}
	if slots[3].ItemID != 300207 || slots[3].Count != 6 || slots[3].ListedAtMs != 1779290145874 {
		t.Fatalf("rack 3 mismatch: %+v", slots[3])
	}
	if slots[3].SellReadyAtMs != 1779290145874+int64(slots[3].Count)*FlowerRackSellDurationMs() {
		t.Fatalf("rack 3 sell ready mismatch: %+v", slots[3])
	}
	if got := s.FlowerRackClaimableSlotIDs(time.UnixMilli(slots[3].ListedAtMs + FlowerRackSellDurationMs())); len(got) != 0 {
		t.Fatalf("FlowerRackClaimableSlotIDs after one item window=%v, want none for count %d", got, slots[3].Count)
	}
	empty := s.EmptyFlowerRackSlotIDs()
	if len(empty) != 1 || empty[0] != 1 {
		t.Fatalf("EmptyFlowerRackSlotIDs=%v, want [1]", empty)
	}
	if got := s.FlowerRackClaimableSlotIDs(time.UnixMilli(slots[3].SellReadyAtMs - 1)); len(got) != 0 {
		t.Fatalf("FlowerRackClaimableSlotIDs before ready=%v, want none", got)
	}
	if got := s.FlowerRackClaimableSlotIDs(time.UnixMilli(slots[3].SellReadyAtMs)); len(got) != 1 || got[0] != 3 {
		t.Fatalf("FlowerRackClaimableSlotIDs at ready=%v, want [3]", got)
	}

	applyMap(t, s, map[string]any{
		"104": map[string]any{
			"0": map[string]any{
				"1": map[string]any{"2": 300208, "3": 1, "4": 1779290172297, "5": 1779290172297},
			},
		},
	})
	slots = s.FlowerRackSlots()
	if slots[1].ItemID != 300208 || slots[1].Count != 1 {
		t.Fatalf("rack 1 delta mismatch: %+v", slots[1])
	}
	if got := s.EmptyFlowerRackSlotIDs(); len(got) != 0 {
		t.Fatalf("empty slots after listing=%v, want none", got)
	}
}

func TestApplyV_MailTracksPickTargets(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"19": map[string]any{
			"1": []any{
				map[string]any{"1": 101, "2": 201, "13": [][]int32{{1, 5}}, "17": 0, "18": 0, "20": 0},
				map[string]any{"1": 102, "2": 202, "13": [][]int32{{7, 3}}, "17": 0, "18": 1, "20": 1},
				map[string]any{"1": 103, "2": 203, "13": []any{}, "17": 0, "20": 0},
			},
		},
	})
	if !s.MailObserved() {
		t.Fatal("MailObserved=false, want true")
	}
	mails := s.Mails()
	if len(mails) != 3 {
		t.Fatalf("Mails len=%d, want 3: %+v", len(mails), mails)
	}
	if mails[0].MsID != 101 || mails[0].AllID != 201 || mails[0].IsPick != 0 || len(mails[0].ItemsRaw) == 0 {
		t.Fatalf("first mail mismatch: %+v", mails[0])
	}
	targets := s.ReadyMailPickTargets()
	if len(targets) != 1 || targets[0].MsID != 101 || targets[0].AllID != 201 {
		t.Fatalf("ReadyMailPickTargets=%+v, want msId=101 allId=201", targets)
	}
}

func TestApplyV_MailSparseDeltaPreservesIsPick(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"19": map[string]any{
			"1": []any{
				map[string]any{"1": 101, "2": 201, "13": [][]int32{{1, 5}}, "17": 0, "18": 0, "20": 1},
			},
		},
	})
	// Sparse pick/follow-up delta often omits field 20; must not wipe isPick.
	applyMap(t, s, map[string]any{
		"19": map[string]any{
			"1": []any{
				map[string]any{"1": 101, "2": 201, "18": 1},
			},
		},
	})
	mails := s.Mails()
	if len(mails) != 1 || mails[0].IsPick != 1 || mails[0].IsRead != 1 || len(mails[0].ItemsRaw) == 0 {
		t.Fatalf("sparse merge wiped mail fields: %+v", mails)
	}
	if got := s.ReadyMailPickTargets(); len(got) != 0 {
		t.Fatalf("ReadyMailPickTargets=%+v, want none after preserved isPick", got)
	}
}

func TestMarkMailPickedStopsReadyTargets(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"19": map[string]any{
			"1": []any{
				map[string]any{"1": 101, "2": 201, "13": [][]int32{{1, 5}}, "20": 0},
			},
		},
	})
	if got := s.ReadyMailPickTargets(); len(got) != 1 {
		t.Fatalf("ReadyMailPickTargets=%+v, want one", got)
	}
	s.MarkMailPicked(101, 201)
	if got := s.ReadyMailPickTargets(); len(got) != 0 {
		t.Fatalf("ReadyMailPickTargets=%+v after MarkMailPicked, want none", got)
	}
}

func TestApplyV_VasesAndFlowerArtState(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"102": map[string]any{
			"0": map[string]any{
				"3001": map[string]any{"0": 1, "1": 3001, "2": 1779290172000, "3": 1779290171000},
				"3002": map[string]any{},
			},
		},
		"106": map[string]any{
			"0": map[string]any{
				"1": 25,
				"2": []int32{300208},
				"3": map[string]any{"300208": 1},
				"4": 1779290173000,
				"5": 1779290170000,
			},
		},
	})
	if !s.VaseObserved() {
		t.Fatal("VaseObserved=false, want true")
	}
	vases := s.Vases()
	if len(vases) != 2 || vases[3001].UTimeMs != 1779290172000 || !s.HasVase(3002) {
		t.Fatalf("Vases mismatch: %+v", vases)
	}
	art := s.FlowerArt()
	if !art.Observed || art.Exp != 25 || len(art.MakeListRaw) == 0 || len(art.SRecvListRaw) == 0 {
		t.Fatalf("FlowerArt mismatch: %+v", art)
	}
	if len(art.MakeList) != 1 || art.MakeList[0] != 300208 {
		t.Fatalf("FlowerArt MakeList=%v, want [300208]", art.MakeList)
	}
	if len(art.SRecvList) != 1 || art.SRecvList[0] != 300208 {
		t.Fatalf("FlowerArt SRecvList=%v, want [300208]", art.SRecvList)
	}
}

func TestApplyV_CollectRewardsAndFlowerArtRewardReadiness(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"103": map[string]any{
			"0": map[string]any{
				"11": map[string]any{"1": 11, "2": 0, "3": 5, "4": []int32{}},
				"13": map[string]any{"1": 13, "2": 0, "3": 70, "4": []int32{}, "7": []int32{300101}},
			},
		},
		"106": map[string]any{
			"0": map[string]any{
				"2": []int32{300101, 300102, 300201},
			},
		},
	})
	if !s.CollectRewardObserved() {
		t.Fatal("CollectRewardObserved=false, want true")
	}
	rewards := s.CollectRewards()
	if rewards[13].Exp != 70 || len(rewards[13].ArtCreateRewardIDs) != 1 || rewards[13].ArtCreateRewardIDs[0] != 300101 {
		t.Fatalf("collect reward 13 mismatch: %+v", rewards[13])
	}
	if got := s.ReadyCollectRewardTypes(11, 12, 13); len(got) != 2 || got[0] != 11 || got[1] != 13 {
		t.Fatalf("ReadyCollectRewardTypes=%v, want [11 13]", got)
	}
	if got := s.ReadyArtCreateRewardVaseIDs(); len(got) != 2 || got[0] != 3001 || got[1] != 3002 {
		t.Fatalf("ReadyArtCreateRewardVaseIDs=%v, want [3001 3002]", got)
	}

	// collectRwd.recv returns only the changed fields of one reward record.
	// Missing field 7 must retain the authoritative art-create receipt list.
	applyMap(t, s, map[string]any{
		"103": map[string]any{
			"0": map[string]any{
				"13": map[string]any{"2": 2, "4": []int32{130001, 130002}, "5": int64(1783744303000)},
			},
		},
	})
	rewards = s.CollectRewards()
	if len(rewards) != 2 || rewards[13].Lvl != 2 || len(rewards[13].ArtCreateRewardIDs) != 1 || rewards[13].ArtCreateRewardIDs[0] != 300101 {
		t.Fatalf("sparse collect reward merge lost prior state: %+v", rewards)
	}
	if got := s.ReadyArtCreateRewardVaseIDs(); len(got) != 2 || got[0] != 3001 || got[1] != 3002 {
		t.Fatalf("ReadyArtCreateRewardVaseIDs after sparse merge=%v, want [3001 3002]", got)
	}

	applyMap(t, s, map[string]any{
		"103": map[string]any{
			"0": map[string]any{
				"11": map[string]any{"1": 11, "2": 1, "3": 5, "4": []int32{110001}},
			},
		},
	})
	if got := s.ReadyCollectRewardTypes(11); len(got) != 0 {
		t.Fatalf("ReadyCollectRewardTypes after recv=%v, want none", got)
	}
	if rewards := s.CollectRewards(); len(rewards) != 2 || len(rewards[13].ArtCreateRewardIDs) != 1 {
		t.Fatalf("sparse type 11 update replaced other reward records: %+v", rewards)
	}
}

func TestApplyV_CollectRewardSparseDeltaDoesNotCreateFalseArtReward(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"103": map[string]any{"0": map[string]any{
			"13": map[string]any{
				"1": 13,
				"2": 1,
				"3": 24,
				"4": []int32{130001},
				"7": []int32{300101, 300102},
			},
		}},
		"106": map[string]any{"0": map[string]any{
			"2": []int32{300101, 300102},
		}},
	})
	if got := s.ReadyArtCreateRewardVaseIDs(); len(got) != 0 {
		t.Fatalf("initial ReadyArtCreateRewardVaseIDs=%v, want none", got)
	}

	// This is the observed shape returned after collectRwd.recv: only the
	// changed catalog-reward fields are present, while field 7 is omitted.
	applyMap(t, s, map[string]any{
		"103": map[string]any{"0": map[string]any{
			"13": map[string]any{
				"2": 2,
				"4": []int32{130001, 130002},
				"5": int64(1783744303000),
			},
		}},
	})
	if got := s.ReadyArtCreateRewardVaseIDs(); len(got) != 0 {
		t.Fatalf("ReadyArtCreateRewardVaseIDs after sparse delta=%v, want none", got)
	}
	reward := s.CollectRewards()[13]
	if reward.Lvl != 2 || len(reward.ArtCreateRewardIDs) != 2 {
		t.Fatalf("sparse merge reward=%+v", reward)
	}
}

func TestApplyV_ShopCultivateOffers(t *testing.T) {
	s := New()
	day := time.Date(2026, 7, 6, 0, 5, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	applyMap(t, s, map[string]any{
		"113": map[string]any{
			"1": map[string]any{
				"10001": []int32{11, 3214},
				"10002": []int32{11, 4215},
			},
			"2": day.UnixMilli(),
			"3": day.UnixMilli(),
			"6": map[string]any{"10001": 0, "10002": 1},
		},
	})
	if !s.ShopCultivateObserved() {
		t.Fatal("ShopCultivateObserved=false, want true")
	}
	offers := s.ShopCultivateOffers()
	if len(offers) != 2 {
		t.Fatalf("ShopCultivateOffers len=%d, want 2: %+v", len(offers), offers)
	}
	first := offers[0]
	if first.ShopID != 10001 || first.ItemID != 1401 || first.ItemCount != 1 || first.CostItemID != 11 || first.CostCount != 3214 || first.BuyLimit != 1 || first.Remaining != 1 {
		t.Fatalf("first shop cultivate offer mismatch: %+v", first)
	}
	if offers[1].Remaining != 0 {
		t.Fatalf("second offer remaining=%d, want 0: %+v", offers[1].Remaining, offers[1])
	}

	applyMap(t, s, map[string]any{
		"113": map[string]any{
			"6": map[string]any{"10001": 1},
		},
	})
	offers = s.ShopCultivateOffers()
	if len(offers) != 2 {
		t.Fatalf("partial bRecord update should preserve infoMap, got %+v", offers)
	}
	if offers[0].Remaining != 0 {
		t.Fatalf("remaining after bRecord update=%d, want 0: %+v", offers[0].Remaining, offers[0])
	}
	if offers[1].Remaining != 0 {
		t.Fatalf("sparse buy delta wiped sibling bought count: %+v", offers[1])
	}

	sameDay := time.Date(2026, 7, 6, 18, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	if s.ShopCultivateNeedsEnter(sameDay) {
		t.Fatal("ShopCultivateNeedsEnter=true same day with lar/reset, want false")
	}

	nextDay := time.Date(2026, 7, 7, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	if !s.ShopCultivateNeedsEnter(nextDay) {
		t.Fatal("ShopCultivateNeedsEnter=false after daily reset boundary, want true")
	}
}

func TestMarkShopCultivateOfferExhausted(t *testing.T) {
	s := New()
	now := time.Date(2026, 8, 29, 10, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))
	applyMap(t, s, map[string]any{
		"113": map[string]any{
			"1": map[string]any{
				"10001": []int32{11, 3214},
				"10002": []int32{11, 4215},
			},
			"2": now.UnixMilli(),
			"3": now.UnixMilli(),
			"6": map[string]any{},
		},
	})

	if !s.MarkShopCultivateOfferExhausted(10001) {
		t.Fatal("MarkShopCultivateOfferExhausted=false, want state correction")
	}
	offers := s.ShopCultivateOffers()
	if len(offers) != 2 || offers[0].ShopID != 10001 || offers[0].Remaining != 0 {
		t.Fatalf("offers after exhausted correction=%+v, want shop 10001 unavailable", offers)
	}
	if offers[1].Remaining != 1 {
		t.Fatalf("sibling offer remaining=%d, want 1", offers[1].Remaining)
	}
	if s.MarkShopCultivateOfferExhausted(10001) {
		t.Fatal("second MarkShopCultivateOfferExhausted=true, want no change")
	}
	if s.MarkShopCultivateOfferExhausted(99999) {
		t.Fatal("unknown offer correction=true, want false")
	}
}

func TestShopCultivateNeedsEnterIncompleteTiming(t *testing.T) {
	now := time.Date(2026, 7, 6, 18, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))

	missingLar := New()
	applyMap(t, missingLar, map[string]any{
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{11, 1000}},
			"2": now.UnixMilli(),
			"6": map[string]any{"10001": 1},
		},
	})
	if !missingLar.ShopCultivateNeedsEnter(now) {
		t.Fatal("NeedsEnter=false when larTime missing after buy-only patch, want true")
	}

	missingReset := New()
	applyMap(t, missingReset, map[string]any{
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{11, 1000}},
			"3": now.UnixMilli(),
			"6": map[string]any{"10001": 1},
		},
	})
	// Enter/refresh often omit lResetTime; same-day larTime means already synced.
	if missingReset.ShopCultivateNeedsEnter(now) {
		t.Fatal("NeedsEnter=true when lResetTime missing but larTime is same day, want false")
	}
}

func TestShopCultivateNeedsEnterStaleResetFreshLar(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 8, 10, 15, 0, 0, 0, shanghai)
	staleReset := time.Date(2026, 8, 4, 0, 5, 0, 0, shanghai)
	freshLar := time.Date(2026, 8, 10, 14, 55, 0, 0, shanghai)

	s := New()
	applyMap(t, s, map[string]any{
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{11, 1000}},
			"2": staleReset.UnixMilli(),
			"3": freshLar.UnixMilli(),
			"6": map[string]any{},
		},
	})
	if s.ShopCultivateNeedsEnter(now) {
		t.Fatal("NeedsEnter=true with stale lResetTime but same-day larTime, want false")
	}

	// Cross-day: both markers yesterday → need enter.
	yesterday := time.Date(2026, 8, 9, 18, 0, 0, 0, shanghai)
	s2 := New()
	applyMap(t, s2, map[string]any{
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{11, 1000}},
			"2": yesterday.UnixMilli(),
			"3": yesterday.UnixMilli(),
			"6": map[string]any{},
		},
	})
	if !s2.ShopCultivateNeedsEnter(now) {
		t.Fatal("NeedsEnter=false when both markers are prior game day, want true")
	}
}

func TestApplyV_ShopCultivateFullSnapshotOmitsResetBumpsMarker(t *testing.T) {
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	staleReset := time.Date(2026, 8, 4, 0, 5, 0, 0, shanghai)
	now := time.Date(2026, 8, 10, 14, 55, 0, 0, shanghai)

	s := New()
	applyMap(t, s, map[string]any{
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{11, 1000}},
			"2": staleReset.UnixMilli(),
			"3": staleReset.UnixMilli(),
		},
	})
	// Re-enter style patch: full infoMap + lar/uTime, no field 2.
	applyMap(t, s, map[string]any{
		"113": map[string]any{
			"1": map[string]any{"10004": []int32{11, 5001}},
			"3": now.UnixMilli(),
			"6": map[string]any{},
			"7": now.UnixMilli(),
		},
	})
	if s.ShopCultivateNeedsEnter(now.Add(time.Minute)) {
		t.Fatal("NeedsEnter still true after full snapshot omitted lResetTime, want false")
	}
	offers := s.ShopCultivateOffers()
	if len(offers) != 1 || offers[0].ShopID != 10004 {
		t.Fatalf("offers after re-enter snapshot=%+v, want shop 10004", offers)
	}
}

func TestApplyV_ShopCultivateAutoRefreshDueIgnoresManualRefreshQuota(t *testing.T) {
	s := New()
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	lar := time.Date(2026, 7, 26, 12, 0, 0, 0, shanghai)
	applyMap(t, s, map[string]any{
		"113": map[string]any{
			"1": map[string]any{"10001": []int32{11, 3214}},
			"2": lar.UnixMilli(),
			"3": lar.UnixMilli(),
			"4": 0,
			"6": map[string]any{"10001": 1},
		},
	})
	if s.ShopCultivateAutoRefreshDue(lar.Add(8999 * time.Second)) {
		t.Fatal("ShopCultivateAutoRefreshDue=true before $autoRefreshCd, want false")
	}
	if !s.ShopCultivateAutoRefreshDue(lar.Add(9000 * time.Second)) {
		t.Fatal("ShopCultivateAutoRefreshDue=false at $autoRefreshCd, want true")
	}
	if !s.ShopCultivateAutoRefreshDue(lar.Add(9001 * time.Second)) {
		t.Fatal("ShopCultivateAutoRefreshDue=false after $autoRefreshCd, want true")
	}

	applyMap(t, s, map[string]any{
		"113": map[string]any{"4": 3},
	})
	if !s.ShopCultivateAutoRefreshDue(lar.Add(9001 * time.Second)) {
		t.Fatal("automatic shelf rotation must not depend on manual refresh quota")
	}
}

func TestApplyV_ZooFoodShopTracksDailyRecordAndFreshness(t *testing.T) {
	now := time.Now()
	s := New()
	applyMap(t, s, map[string]any{
		"20": map[string]any{"0": map[string]any{
			"9": map[string]any{"1": 9, "3": now.UnixMilli(), "12": map[string]any{"90001": 4}},
		}},
	})
	view := s.ZooFoodShop(now)
	if !view.Observed || view.NeedsEnter || view.ShopTempID != 9 || view.ShopItemID != 90001 ||
		view.FoodstuffID != 1501 || view.FoodstuffCount != 1 || view.GoldCost != 100 ||
		view.DailyBought != 4 || view.DailyLimit != 30 || view.DailyRemaining != 26 {
		t.Fatalf("ZooFoodShop()=%+v", view)
	}

	applyMap(t, s, map[string]any{
		"20": map[string]any{"0": map[string]any{
			"9": map[string]any{"12": map[string]any{"90001": 5}},
		}},
	})
	view = s.ZooFoodShop(now)
	if view.DailyBought != 5 || view.DailyRemaining != 25 {
		t.Fatalf("ZooFoodShop() after sparse dRecord=%+v", view)
	}
	if !s.ZooFoodShop(now.Add(25 * time.Hour)).NeedsEnter {
		t.Fatal("ZooFoodShop should require enter on a later game day")
	}

	empty := New()
	applyMap(t, empty, map[string]any{
		"20": map[string]any{"0": map[string]any{
			"9": map[string]any{"1": 9, "3": now.UnixMilli()},
		}},
	})
	if view := empty.ZooFoodShop(now); view.NeedsEnter || view.DailyBought != 0 || view.DailyRemaining != 30 {
		t.Fatalf("full shop row with omitted dRecord must mean empty daily record: %+v", view)
	}

	preserve := New()
	applyMap(t, preserve, map[string]any{
		"20": map[string]any{"0": map[string]any{
			"9": map[string]any{"1": 9, "3": now.UnixMilli(), "12": map[string]any{"90001": 4, "90002": 3}},
		}},
	})
	applyMap(t, preserve, map[string]any{
		"20": map[string]any{"0": map[string]any{
			"9": map[string]any{"12": map[string]any{"90001": 5}, "17": now.Add(time.Minute).UnixMilli()},
		}},
	})
	if got := preserve.shops[9].dailyBought[90002]; got != 3 {
		t.Fatalf("sparse dRecord update erased another item: got %d, want 3", got)
	}
}

func TestApplyV_ShopGiftbagOffers(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"112": map[string]any{
			"1": map[string]any{"1": 2},
		},
	})
	if !s.ShopGiftbagObserved() {
		t.Fatal("ShopGiftbagObserved=false, want true")
	}
	var freeGift ShopGiftbagOfferView
	for _, offer := range s.ShopGiftbagOffers() {
		if offer.ShopID == 1 {
			freeGift = offer
			break
		}
	}
	if freeGift.ShopID != 1 {
		t.Fatalf("missing free giftbag offer")
	}
	if freeGift.Type != 1 || freeGift.ShareID != 8 || freeGift.DailyLimit != 4 || freeGift.DailyBought != 2 || freeGift.Remaining != 2 {
		t.Fatalf("free giftbag offer mismatch: %+v", freeGift)
	}
	if len(freeGift.Rewards) != 4 || freeGift.Rewards[0].ItemID != 1 || freeGift.Rewards[0].Count != 5 {
		t.Fatalf("free giftbag rewards mismatch: %+v", freeGift.Rewards)
	}

	applyMap(t, s, map[string]any{
		"112": map[string]any{
			"1": map[string]any{"1": 4},
		},
	})
	for _, offer := range s.ShopGiftbagOffers() {
		if offer.ShopID == 1 && offer.Remaining != 0 {
			t.Fatalf("remaining after dRecord update=%d, want 0: %+v", offer.Remaining, offer)
		}
	}
}

func TestApplyV_PearlState(t *testing.T) {
	s := New()
	now := time.Date(2026, 7, 2, 10, 0, 0, 0, time.Local)
	yesterday := now.Add(-24 * time.Hour).UnixMilli()
	applyMap(t, s, map[string]any{
		"115": map[string]any{
			"0": map[string]any{
				"1": map[string]any{"1": 1, "2": 10001, "3": now.Add(2 * time.Hour).UnixMilli(), "6": 1, "7": 0, "8": 2},
				"2": map[string]any{"1": 2, "3": nil, "6": 0, "7": 0, "8": 0},
			},
			"1": map[string]any{"1": 0, "2": 3, "6": yesterday, "8": 1},
			"2": []int32{101, 102},
		},
	})
	if !s.PearlObserved() {
		t.Fatal("PearlObserved=false, want true")
	}
	if !s.PearlDailyFreeReady(now) {
		t.Fatal("PearlDailyFreeReady=false, want true for yesterday recv date")
	}
	if got := s.PearlDrawCount(); got != 2 {
		t.Fatalf("PearlDrawCount=%d, want 2", got)
	}
	if got := s.ReadyPearlPlaceIDsAt(now); len(got) != 1 || got[0] != 1 {
		t.Fatalf("ReadyPearlPlaceIDs=%v, want [1]", got)
	}
	pearl := s.Pearl()
	if pearl.ProtectState != 0 || pearl.ProtectNum != 3 || pearl.SmallDrawCnt != 1 {
		t.Fatalf("Pearl() mismatch: %+v", pearl)
	}

	applyMap(t, s, map[string]any{
		"115": map[string]any{
			"0": map[string]any{"1": map[string]any{"8": 0}},
			"1": map[string]any{"1": 1, "6": now.UnixMilli()},
		},
	})
	if s.PearlDailyFreeReady(now) {
		t.Fatal("PearlDailyFreeReady=true, want false for same-day recv date")
	}
	if got := s.ReadyPearlPlaceIDsAt(now); len(got) != 0 {
		t.Fatalf("ReadyPearlPlaceIDs after partial update=%v, want none", got)
	}
	places := s.PearlPlaces()
	if len(places) != 2 {
		t.Fatalf("partial pearl update should preserve other places, got %+v", places)
	}
}

func TestApplyV_ResidentOrderRewardPartialUpdatePreservesOrders(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"105": map[string]any{"0": map[string]any{
			"1": map[string]any{
				"1": map[string]any{"0": 8001, "1": 91, "3": 1, "6": 4, "7": map[string]any{"1": 8}},
				"2": map[string]any{"0": 3, "1": 1, "2": [][]int32{{23004, 7}}},
			},
		}},
	})
	if got := s.ReadyFlowerOrderAdBoxIDs(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("ReadyFlowerOrderAdBoxIDs before reward=%v, want [1]", got)
	}

	applyMap(t, s, map[string]any{
		"105": map[string]any{"0": map[string]any{
			"2": []int32{1},
		}},
	})
	if got := s.ReadyFlowerOrderAdBoxIDs(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("ReadyFlowerOrderAdBoxIDs after partial reward=%v, want preserved [1]", got)
	}
	orders := s.FlowerOrders()
	if orders[2] == nil || len(orders[2].Requires) != 1 {
		t.Fatalf("partial reward update should preserve existing orders, got %+v", orders)
	}
	if orders[1] == nil || orders[1].NPCID != 8001 || orders[1].DialogID != 91 || orders[1].PlaceIdx != 4 || len(orders[1].VideoRewards) != 1 || orders[1].VideoRewards[0] != (ItemCount{ItemID: 1, Count: 8}) {
		t.Fatalf("video order metadata mismatch: %+v", orders[1])
	}
}

func TestApplyV_ShareUsageSparseMergeAndDailyReset(t *testing.T) {
	s := New()
	loc := gameDayLocation()
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, loc)
	applyMap(t, s, map[string]any{"31": map[string]any{"0": map[string]any{
		"10": map[string]any{"0": 99, "1": 10, "2": 1, "3": 1, "4": now.UnixMilli(), "6": now.UnixMilli()},
	}}})
	applyMap(t, s, map[string]any{"31": map[string]any{"0": map[string]any{
		"10": map[string]any{"3": 2},
		"11": map[string]any{"1": 11, "2": 3, "6": now.UnixMilli()},
	}}})
	usage, ok := s.ShareUsageAt(10, now)
	if !ok || !usage.Observed || usage.ShareCount != 1 || usage.ReceiveCount != 2 || usage.UID != 99 {
		t.Fatalf("ShareUsageAt today=%+v ok=%t", usage, ok)
	}
	nextDay, ok := s.ShareUsageAt(10, now.Add(24*time.Hour))
	if !ok || nextDay.ShareCount != 0 || nextDay.ReceiveCount != 0 || nextDay.TotalCount != usage.TotalCount {
		t.Fatalf("ShareUsageAt next day=%+v ok=%t", nextDay, ok)
	}
}

func TestShopGiftbagOffersAtExposesCooldownResetAndNextReward(t *testing.T) {
	s := New()
	loc := gameDayLocation()
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, loc)
	applyMap(t, s, map[string]any{"112": map[string]any{
		"1": map[string]any{"1": 1},
		"5": map[string]any{"1": now.Add(-time.Hour).UnixMilli()},
		"6": now.UnixMilli(),
	}})
	var offer ShopGiftbagOfferView
	for _, candidate := range s.ShopGiftbagOffersAt(now) {
		if candidate.ShopID == 1 {
			offer = candidate
			break
		}
	}
	if offer.DailyBought != 1 || offer.Remaining != 3 || offer.AvailableAtMs != now.Add(time.Hour).UnixMilli() || offer.NextReward != (ItemCount{ItemID: 1, Count: 6}) {
		t.Fatalf("giftbag status=%+v", offer)
	}
	nextDay := s.ShopGiftbagOffersAt(now.Add(24 * time.Hour))
	for _, candidate := range nextDay {
		if candidate.ShopID == 1 && (candidate.DailyBought != 0 || candidate.Remaining != 4 || candidate.AvailableAtMs != 0 || candidate.NextReward != (ItemCount{ItemID: 1, Count: 5})) {
			t.Fatalf("next-day giftbag status=%+v", candidate)
		}
	}
}

func TestFmlBuildOptionUsageAtResetsStaleCounts(t *testing.T) {
	s := New()
	loc := gameDayLocation()
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, loc)
	applyMap(t, s, map[string]any{"25": map[string]any{"133": map[string]any{
		"1": 5, "4": now.UnixMilli(), "5": map[string]any{"1": 1, "2": 2},
	}}})
	today := s.FmlBuildOptionUsageAt(1, now)
	if !today.Observed || today.Count != 1 || today.GroupCount != 1 {
		t.Fatalf("today build usage=%+v", today)
	}
	tomorrow := s.FmlBuildOptionUsageAt(1, now.Add(24*time.Hour))
	if !tomorrow.Observed || tomorrow.Count != 0 || tomorrow.GroupCount != 0 {
		t.Fatalf("tomorrow build usage=%+v", tomorrow)
	}
}

func TestApplyV_ResidentOrderTracksCooldownMetadata(t *testing.T) {
	s := New()
	now := time.Date(2026, 7, 4, 9, 0, 0, 0, time.UTC)
	cdTime := now.Add(42 * time.Second).UnixMilli()
	applyMap(t, s, map[string]any{
		"105": map[string]any{"0": map[string]any{
			"1": map[string]any{
				"1": map[string]any{"0": 7, "1": 3, "2": [][]int32{{23010, 8}}, "4": cdTime, "5": now.UnixMilli()},
			},
		}},
	})

	orders := s.FlowerOrders()
	order := orders[1]
	if order == nil {
		t.Fatalf("missing flower order: %+v", orders)
		return
	}
	if order.CdTimeMs != cdTime || order.CTimeMs != now.UnixMilli() {
		t.Fatalf("cooldown metadata = (%d,%d), want (%d,%d)", order.CdTimeMs, order.CTimeMs, cdTime, now.UnixMilli())
	}
	if order.CooldownReady(now) {
		t.Fatalf("CooldownReady()=true before cd time")
	}
	if !order.CooldownReady(now.Add(42 * time.Second)) {
		t.Fatalf("CooldownReady()=false at cd time")
	}
}

func TestApplyV_StatisticsTracksOrderFinishCounts(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			"20260701": map[string]any{"1": 20260701, "9": 3, "11": 4, "14": 5, "16": 6},
			"20260702": map[string]any{"1": 20260702, "9": 7, "10": 8, "12": 1779290172000},
		}},
	})

	stats := s.Statistics()
	if !stats.Observed {
		t.Fatal("Statistics.Observed=false, want true")
	}
	if stats.DayID != 20260702 || stats.OrderFlowerFinishNum != 7 || stats.OrderPalaceFinishNum != 8 || stats.UTimeMs != 1779290172000 {
		t.Fatalf("Statistics mismatch: %+v", stats)
	}
	days := s.StatisticsDays()
	if len(days) != 2 || days[0].DayID != 20260702 || days[1].DayID != 20260701 || days[1].OrderFlowerFinishNum != 3 {
		t.Fatalf("StatisticsDays mismatch: %+v", days)
	}
}

func TestApplyV_StatisticsSparseDeltaPreservesOrderFlowerFinishNum(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			"20260702": map[string]any{"1": 20260702, "9": 42, "10": 3, "12": 1779290172000},
		}},
	})
	// Unrelated counter patch must not wipe ordinary resident finish count,
	// otherwise the policy daily limit never trips.
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			"20260702": map[string]any{"8": 9},
		}},
	})

	stats := s.Statistics()
	if !stats.Observed || stats.DayID != 20260702 {
		t.Fatalf("Statistics day mismatch: %+v", stats)
	}
	if stats.OrderFlowerFinishNum != 42 {
		t.Fatalf("OrderFlowerFinishNum=%d, want 42 after sparse delta", stats.OrderFlowerFinishNum)
	}
	if stats.FlowerArtSellNum != 9 || stats.OrderPalaceFinishNum != 3 {
		t.Fatalf("sparse merge mismatch: %+v", stats)
	}
}

func TestApplyV_StatisticsNewDayReplacesPriorCounters(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			"20260702": map[string]any{"1": 20260702, "9": 42, "14": 11, "16": 12},
		}},
	})
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			"20260703": map[string]any{"1": 20260703, "9": 1},
		}},
	})

	stats := s.Statistics()
	if stats.DayID != 20260703 || stats.OrderFlowerFinishNum != 1 {
		t.Fatalf("Statistics new day mismatch: %+v", stats)
	}
	if stats.OrderSatinFinishNum != 0 || stats.OrderDecorateFinishNum != 0 {
		t.Fatalf("satin/decorate should reset on new day: %+v", stats)
	}
	days := s.StatisticsDays()
	if len(days) != 2 || days[1].DayID != 20260702 || days[1].OrderFlowerFinishNum != 42 {
		t.Fatalf("prior-day history should be kept: %+v", days)
	}
}

func TestApplyV_StatisticsMillisDayKeyKeepsResidentFinishLimit(t *testing.T) {
	// Live orderFlower.finishOrder patches use Asia/Shanghai midnight ms as the
	// day map key and omit field 1. atoi32 used to overflow that key so
	// ResidentOrderFinishNum treated today's field 9 as a prior-day 0.
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 8, 3, 22, 0, 0, 0, loc)
	dayStartMs := time.Date(2026, 8, 3, 0, 0, 0, 0, loc).UnixMilli() // 1785686400000
	s := New()
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			fmt.Sprintf("%d", dayStartMs): map[string]any{
				"2": 1540590, "3": 623894, "9": 846, "12": dayStartMs + 8*time.Hour.Milliseconds(),
			},
		}},
	})

	stats := s.Statistics()
	if !stats.Observed || stats.DayID != 20260803 {
		t.Fatalf("Statistics day mismatch: %+v (dayStartMs=%d)", stats, dayStartMs)
	}
	if stats.OrderFlowerFinishNum != 846 {
		t.Fatalf("OrderFlowerFinishNum=%d, want 846", stats.OrderFlowerFinishNum)
	}
	if stats.Gold != 1540590 || stats.Experience != 623894 {
		t.Fatalf("business counters mismatch: %+v", stats)
	}
	if got := s.ResidentOrderFinishNum(now); got != 846 {
		t.Fatalf("ResidentOrderFinishNum=%d, want 846 so policy daily limit can trip", got)
	}
}

func TestApplyV_StatisticsMillisField1NormalizesToYYYYMMDD(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, loc)
	dayStartMs := time.Date(2026, 8, 3, 0, 0, 0, 0, loc).UnixMilli()
	s := New()
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			fmt.Sprintf("%d", dayStartMs): map[string]any{"1": dayStartMs, "9": 600},
		}},
	})
	if got := s.Statistics().DayID; got != 20260803 {
		t.Fatalf("DayID=%d, want 20260803 from ms field 1", got)
	}
	if got := s.ResidentOrderFinishNum(now); got != 600 {
		t.Fatalf("ResidentOrderFinishNum=%d, want 600", got)
	}
}

func TestApplyV_StatisticsTracksBusinessCountersAndHistory(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			"20260817": map[string]any{
				"1": 20260817, "2": 100, "3": 200, "7": 30, "9": 4,
			},
			"20260818": map[string]any{
				"1": 20260818, "2": 1540590, "3": 623894, "4": 12, "5": 3, "6": 88,
				"7": 410, "8": 9, "9": 846, "10": 2, "11": 5, "14": 7, "15": 11, "16": 8, "17": 19,
			},
		}},
	})
	stats := s.Statistics()
	if stats.DayID != 20260818 || stats.Gold != 1540590 || stats.Experience != 623894 || stats.Diamonds != 12 {
		t.Fatalf("today business stats mismatch: %+v", stats)
	}
	if stats.SpeedUpCard != 3 || stats.FlowerShopCoin != 88 || stats.FlowerHarvestNum != 410 || stats.Wood != 19 || stats.Satin != 11 {
		t.Fatalf("today resource counters mismatch: %+v", stats)
	}
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			"20260818": map[string]any{"2": 1541590, "7": 411},
		}},
	})
	stats = s.Statistics()
	if stats.Gold != 1541590 || stats.FlowerHarvestNum != 411 || stats.Experience != 623894 || stats.OrderFlowerFinishNum != 846 {
		t.Fatalf("sparse business merge mismatch: %+v", stats)
	}
	days := s.StatisticsDays()
	if len(days) != 2 || days[1].DayID != 20260817 || days[1].Gold != 100 || days[1].FlowerHarvestNum != 30 {
		t.Fatalf("business history mismatch: %+v", days)
	}
}

func TestResidentOrderFinishNumKeepsNewDayBiasAcrossSparseDayRollover(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, 7, 25, 0, 10, 0, 0, loc)
	s := New()
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			"20260724": map[string]any{"1": 20260724, "9": 80},
		}},
	})
	s.NoteResidentOrderFinished(now, nil)
	s.NoteResidentOrderFinished(now, nil)
	if got := s.ResidentOrderFinishNum(now); got != 2 {
		t.Fatalf("before sparse rollover FinishNum=%d, want 2", got)
	}
	// New-day DayID without field 9 must not wipe bias already counted for 20260725.
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			"20260725": map[string]any{"1": 20260725, "8": 1},
		}},
	})
	if got := s.ResidentOrderFinishNum(now); got != 2 {
		t.Fatalf("after sparse rollover FinishNum=%d, want 2 kept bias", got)
	}
	stats := s.Statistics()
	if stats.DayID != 20260725 || stats.OrderFlowerFinishNum != 0 {
		t.Fatalf("stats after sparse rollover=%+v, want day 20260725 finish 0", stats)
	}
}

func TestResidentSatinDecorateFinishNumResetsAtMidnight(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	s := New()
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			"20260725": map[string]any{"1": 20260725, "14": 31, "16": 28},
		}},
	})

	before := time.Date(2026, 7, 25, 23, 59, 0, 0, loc)
	if got := s.ResidentSatinFinishNum(before); got != 31 {
		t.Fatalf("satin before midnight=%d, want 31", got)
	}
	if got := s.ResidentDecorateFinishNum(before); got != 28 {
		t.Fatalf("decorate before midnight=%d, want 28", got)
	}

	after := time.Date(2026, 7, 26, 0, 0, 0, 0, loc)
	if got := s.ResidentSatinFinishNum(after); got != 0 {
		t.Fatalf("satin after midnight=%d, want 0 until new-day stats", got)
	}
	if got := s.ResidentDecorateFinishNum(after); got != 0 {
		t.Fatalf("decorate after midnight=%d, want 0 until new-day stats", got)
	}
	if reset := NextCalendarDayReset(before); !reset.Equal(after) {
		t.Fatalf("NextCalendarDayReset=%s, want %s", reset, after)
	}
}

func TestResidentSpecialFinishBiasResetsAtCalendarMidnight(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*60*60)
	s := New()
	applyMap(t, s, map[string]any{
		"124": map[string]any{"0": map[string]any{
			"20260725": map[string]any{"1": 20260725, "14": 1, "16": 2},
		}},
	})

	before := time.Date(2026, 7, 25, 23, 59, 0, 0, loc)
	s.NoteResidentSatinOrderFinished(before, nil)
	s.NoteResidentDecorateOrderFinished(before, nil)
	if got := s.ResidentSatinOrderFinishNum(before); got != 2 {
		t.Fatalf("satin before midnight=%d, want observed 1 + local 1", got)
	}
	if got := s.ResidentDecorateOrderFinishNum(before); got != 3 {
		t.Fatalf("decorate before midnight=%d, want observed 2 + local 1", got)
	}

	after := time.Date(2026, 7, 26, 0, 1, 0, 0, loc)
	if got := s.ResidentSatinOrderFinishNum(after); got != 0 {
		t.Fatalf("satin after midnight=%d, want 0", got)
	}
	if got := s.ResidentDecorateOrderFinishNum(after); got != 0 {
		t.Fatalf("decorate after midnight=%d, want 0", got)
	}
}

func TestApplyV_OrderPalaceTeamAndResidentSpecialOrders(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"105": map[string]any{"0": map[string]any{
			"6": map[string]any{
				"0": []any{[]any{23001, 2}, []any{23002, 1}},
				"1": 201, "2": 301, "3": 2, "6": 1779290100000,
			},
			"7": map[string]any{
				"0": []any{[]any{23003, 4}},
				"1": 202, "2": 302, "3": 1, "7": 1779290200000,
			},
		}},
		"107": map[string]any{"0": map[string]any{
			"0": 77900091102482, "1": 1, "3": 5, "4": 23005, "6": 2, "14": 99,
		}},
		"108": map[string]any{"0": map[string]any{"0": map[string]any{
			"0": 77900091102482, "1": 23007, "2": 3, "3": 0, "5": 1779290300000,
		}}},
	})

	satin := s.ResidentSatinOrder()
	if !satin.Observed || satin.NPCID != 201 || satin.FinishCnt != 2 || len(satin.Requires) != 2 ||
		satin.Requires[0] != (FlowerRequire{FlowerID: 23001, Count: 2}) ||
		satin.Requires[1] != (FlowerRequire{FlowerID: 23002, Count: 1}) {
		t.Fatalf("ResidentSatinOrder mismatch: %+v", satin)
	}
	decorate := s.ResidentDecorateOrder()
	if !decorate.Observed || decorate.NPCID != 202 || decorate.FinishCnt != 1 || len(decorate.Requires) != 1 ||
		decorate.Requires[0] != (FlowerRequire{FlowerID: 23003, Count: 4}) {
		t.Fatalf("ResidentDecorateOrder mismatch: %+v", decorate)
	}
	team := s.TeamOrder()
	if !team.Observed || team.FlowerID != 23005 || team.OrderNum != 5 || team.RemainingNum != 2 || team.NPCID != 99 {
		t.Fatalf("TeamOrder mismatch: %+v", team)
	}
	palace := s.PalaceOrder()
	if !palace.Observed || palace.FlowerID != 23007 || palace.Num != 3 || palace.IsFinish != 0 {
		t.Fatalf("PalaceOrder mismatch: %+v", palace)
	}
}

func TestApplyV_MainTaskFlowerDeficit(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"22": map[string]any{
			"0": map[string]any{
				"1": 10001,
				"2": 1,
			},
		},
	})
	deficits := s.FlowerOrderDeficits()
	if deficits[23001] != 3 {
		t.Fatalf("main task deficit for 23001 = %d, want 3", deficits[23001])
	}
}

func TestApplyV_RoadGrowAndRandomEventReady(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"34": 14}},
		"119": map[string]any{
			"3": map[string]any{"20001": 1, "20002": 1, "20003": 1},
		},
		"129": map[string]any{
			"0": map[string]any{
				"1": map[string]any{
					"6002": map[string]any{"0": 6002, "1": 0, "2": 60020601},
					"6005": map[string]any{"0": 6005, "1": 1, "2": 60050301},
				},
			},
		},
	})
	road := s.ReadyRoadGrowTaskIDs()
	if len(road) != 1 || road[0] != 20004 {
		t.Fatalf("ReadyRoadGrowTaskIDs=%v, want [20004]", road)
	}
	events := s.ReadyRandomEventIDs()
	if len(events) != 2 || events[0] != 6002 || events[1] != 6005 {
		t.Fatalf("ReadyRandomEventIDs=%v, want [6002 6005]", events)
	}
	if !s.RandomEventObserved() {
		t.Fatal("RandomEventObserved=false, want true")
	}
}

func TestApplyV_StoryAchievementAndZooEvents(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{
			"0":   map[string]any{"32": map[string]any{"56": 120}},
			"101": map[string]any{"0": 1, "1": 19, "2": 5, "3": int64(10), "4": int64(20)},
		},
		"22": map[string]any{
			"2": map[string]any{
				"1": map[string]any{"1": 3},
				"3": map[string]any{},
			},
		},
		"33": map[string]any{
			"0": map[string]any{"0": 1, "3": []int32{1}},
			"1": map[string]any{
				"1": map[string]any{
					"1":  1,
					"5":  5,
					"9":  4001,
					"10": []int32{3012},
					"19": int64(30),
					"25": map[string]any{"4001": int64(40)},
				},
			},
		},
	})
	story, ok := s.StoryMain()
	if !ok || story.Chapter != 19 || story.SectionIdx != 5 || story.SectionID != 2806 || len(story.Cost) == 0 {
		t.Fatalf("StoryMain=%+v ok=%t, want chapter 19 section 2806 with cost", story, ok)
	}
	readyAch := s.ReadyAchievementTaskIDs()
	if len(readyAch) != 1 || readyAch[0] != 10001 {
		t.Fatalf("ReadyAchievementTaskIDs=%v, want [10001]", readyAch)
	}
	pets := s.ZooPets()
	pet := pets[1]
	if pet.GoOutEventID != 4001 || len(pet.SpecialEventIDs) != 1 || pet.ReadLogTimeMs != 30 || pet.EventTriggerTimes[4001] != 40 {
		t.Fatalf("ZooPet fields=%+v, want parsed event fields", pet)
	}
	if events := s.ZooEventActions(); len(events) != 0 {
		t.Fatalf("ZooEventActions=%+v, want pet-field inference disabled", events)
	}
}

func TestZooEventActionsDoesNotInferFromPetFields(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"33": map[string]any{
			"1": map[string]any{
				"1": map[string]any{
					"1":  1,
					"10": []int32{2096},
				},
			},
		},
	})
	events := s.ZooEventActions()
	if len(events) != 0 {
		t.Fatalf("ZooEventActions=%+v, want no field-inferred event", events)
	}
}

func TestApplyV_AchievementRecvMapUsesGroupCursor(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"22": map[string]any{
			"2": map[string]any{
				"1": map[string]any{"1001": 20},
				"3": map[string]any{},
			},
		},
	})
	ready := s.ReadyAchievementTaskIDs()
	if len(ready) != 1 || ready[0] != 60001 {
		t.Fatalf("ReadyAchievementTaskIDs got %v, want [60001]", ready)
	}

	applyMap(t, s, map[string]any{
		"22": map[string]any{
			"2": map[string]any{
				"3": map[string]any{"6": 1},
			},
		},
	})
	if ready := s.ReadyAchievementTaskIDs(); len(ready) != 0 {
		t.Fatalf("ReadyAchievementTaskIDs after recv got %v, want empty", ready)
	}
	tasks := s.AchievementTasks()
	task := tasks[60001]
	if task.GroupID != 6 || task.ProgressType != 1001 || task.GroupReceived != 1 || task.Receipted != 1 || task.Status != 3 || task.Current {
		t.Fatalf("achievement task after recv=%+v, want completed group cursor", task)
	}
}

func TestLeastInventoryFlower_RespectsAllowAndBlock(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23001": 5,
			"23002": 2,
			"23003": 8,
		}}},
	})

	// No filter: lowest count overall -> 23002.
	id, count := s.LeastInventoryFlower(nil, nil)
	if id != 23002 || count != 2 {
		t.Errorf("got (%d,%d), want (23002,2)", id, count)
	}

	// Allow only 23001 + 23003 -> 23001 (count 5).
	id, count = s.LeastInventoryFlower([]int32{23001, 23003}, nil)
	if id != 23001 || count != 5 {
		t.Errorf("allow-list got (%d,%d), want (23001,5)", id, count)
	}

	// Block 23002 -> next-lowest is 23001.
	id, count = s.LeastInventoryFlower(nil, []int32{23002})
	if id != 23001 || count != 5 {
		t.Errorf("block-list got (%d,%d), want (23001,5)", id, count)
	}

	// Empty inventory -> (0,0).
	empty := New()
	id, count = empty.LeastInventoryFlower(nil, nil)
	if id != 0 || count != 0 {
		t.Errorf("empty got (%d,%d), want (0,0)", id, count)
	}
}

func TestPlantableFlowers_IncludesCultivatedZeroStockFlowers(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{"32": map[string]any{
			"23007": 3000,
		}}},
		"101": map[string]any{"0": map[string]any{
			"23007": map[string]any{"2": 1, "4": 2},
			"23008": map[string]any{"2": 1, "4": 2},
			"23009": map[string]any{"2": 1, "4": 1},
		}},
	})

	flowers := s.PlantableFlowers(nil, nil)
	got := map[int32]int32{}
	for _, flower := range flowers {
		got[flower.FlowerID] = flower.Stock
	}
	if got[23007] != 3000 {
		t.Fatalf("23007 stock = %d, want 3000; all=%v", got[23007], got)
	}
	if stock, ok := got[23008]; !ok || stock != 0 {
		t.Fatalf("23008 zero-stock cultivated flower missing: all=%v", got)
	}
	if _, ok := got[23009]; ok {
		t.Fatalf("uncultivated/in-progress flower leaked into plantables: all=%v", got)
	}
}

func TestApplyV_StringInputIsNoop(t *testing.T) {
	// Some legacy server responses serialize v as a JSON-stringified blob
	// instead of an object. We must tolerate this without panicking.
	s := New()
	s.ApplyV(json.RawMessage(`"some-string"`))
	if len(s.Lands()) != 0 {
		t.Errorf("string v should not populate lands")
	}
}
