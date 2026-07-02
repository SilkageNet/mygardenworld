package state

import (
	"encoding/json"
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

func TestReserveWaterDropsReducesAvailableUntilReleased(t *testing.T) {
	s := New()
	now := time.Now()
	applyMap(t, s, map[string]any{
		"7": map[string]any{"0": map[string]any{
			"32": map[string]any{"7": 6},
			"33": map[string]any{"7": map[string]any{"1": 130}},
		}},
	})

	if !s.ReserveWaterDrops(4, now) {
		t.Fatal("ReserveWaterDrops returned false, want true")
	}
	available, _, _ := s.AvailableWaterDrops(now)
	if available != 2 {
		t.Fatalf("available after reserve = %d, want 2", available)
	}
	if s.ReserveWaterDrops(3, now) {
		t.Fatal("ReserveWaterDrops allowed spending reserved drops")
	}
	s.ReleaseWaterDropsReservation(4)
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

func TestWaterwheelCooldownUsesBucketCreateInterval(t *testing.T) {
	interval := waterwheelBucketCreateInterval()
	if interval <= 0 || interval >= time.Hour {
		t.Fatalf("waterwheelBucketCreateInterval = %s, want configured short interval", interval)
	}
	s := New()
	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": 1,
			"4": time.Now().Add(-interval - time.Second).UnixMilli(),
		},
	})
	if !s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = false, want true after configured bucket interval")
	}

	applyMap(t, s, map[string]any{
		"114": map[string]any{
			"1": 1,
			"4": time.Now().UnixMilli(),
		},
	})
	if s.WaterwheelCooldownReady() {
		t.Fatal("WaterwheelCooldownReady = true, want false before next bucket interval")
	}
}

func TestApplyV_FreeWaterTracksNextIndex(t *testing.T) {
	s := New()
	if _, ok := s.NextFreeWaterIndex(); ok {
		t.Fatal("free water should be unavailable before namespace 117 is observed")
	}

	applyMap(t, s, map[string]any{
		"117": map[string]any{
			"1": 2,
			"2": 1778914415000,
		},
	})
	idx, ok := s.NextFreeWaterIndex()
	if !ok || idx != 2 {
		t.Fatalf("NextFreeWaterIndex got (%d,%t), want (2,true)", idx, ok)
	}
}

func TestApplyV_ReadyDailyTaskIDs(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"22": map[string]any{
			"1": map[string]any{
				"1": map[string]any{"3084": 3},
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

func TestApplyV_ReadyWeeklyTaskIDs(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"22": map[string]any{
			"100": map[string]any{
				"1": map[string]any{
					"3088": 163,
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
	empty := s.EmptyFlowerRackSlotIDs()
	if len(empty) != 1 || empty[0] != 1 {
		t.Fatalf("EmptyFlowerRackSlotIDs=%v, want [1]", empty)
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

func TestApplyV_ResidentOrderRewardPartialUpdatePreservesOrders(t *testing.T) {
	s := New()
	applyMap(t, s, map[string]any{
		"105": map[string]any{"0": map[string]any{
			"1": map[string]any{
				"1": map[string]any{"0": 8, "1": 1},
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
	if deficits[23058] != 3 {
		t.Fatalf("main task deficit for 23058 = %d, want 3", deficits[23058])
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
