package state

import (
	"encoding/json"
	"sort"
)

func (s *State) applyLandsLocked(ns100 map[string]json.RawMessage) []LandChange {
	var changes []LandChange
	if raw0, ok := ns100["0"]; ok {
		var s0 map[string]json.RawMessage
		if err := json.Unmarshal(raw0, &s0); err == nil {
			if rawRole, ok := s0["0"]; ok {
				_ = json.Unmarshal(rawRole, &s.roleID)
			}
			if raw1, ok := s0["1"]; ok {
				var roster map[string]json.RawMessage
				if err := json.Unmarshal(raw1, &roster); err == nil {
					s.landRosterObserved = true
					next := make(map[int32]LandView, len(roster))
					for lidStr, rawEntry := range roster {
						lid := atoi32(lidStr)
						if lid < 1000 {
							continue
						}
						var entry map[string]any
						if len(rawEntry) > 0 && string(rawEntry) != "{}" {
							if err := json.Unmarshal(rawEntry, &entry); err != nil {
								continue
							}
						}
						var view LandView
						if len(entry) > 0 {
							view = FromPrimary(entry)
						} else {
							view = EmptyObserved()
						}
						next[lid] = view
						before, existed := s.lands[lid]
						if !existed || !landViewEqual(before, view) {
							changes = append(changes, LandChange{LandID: lid, Before: before, After: view})
						}
					}
					for lid, before := range s.lands {
						if _, ok := next[lid]; !ok {
							changes = append(changes, LandChange{LandID: lid, Before: before, After: LandView{}})
						}
					}
					s.lands = next
				}
			}
		}
	}
	if raw1, ok := ns100["1"]; ok {
		var sub1 map[string]json.RawMessage
		if err := json.Unmarshal(raw1, &sub1); err == nil {
			for lidStr, rawEntry := range sub1 {
				lid := atoi32(lidStr)
				if lid < 1000 {
					continue
				}
				var entry map[string]any
				view := EmptyObserved()
				if len(rawEntry) > 0 && string(rawEntry) != "{}" {
					if err := json.Unmarshal(rawEntry, &entry); err == nil {
						view = FromPrimary(entry)
					}
				}
				if change, ok := s.upsertLandLocked(lid, view, "primary"); ok {
					changes = append(changes, change)
				}
			}
		}
	}
	return changes
}

func (s *State) upsertLandLocked(lid int32, next LandView, _ string) (LandChange, bool) {
	prev, existed := s.lands[lid]
	if existed && landViewEqual(prev, next) {
		return LandChange{}, false
	}
	s.lands[lid] = next
	return LandChange{LandID: lid, Before: prev, After: next}, true
}

func landViewEqual(a, b LandView) bool {
	if a.FlowerID != b.FlowerID || a.State != b.State || a.Lvl != b.Lvl ||
		a.HarvestCnt != b.HarvestCnt || a.NextTimeMs != b.NextTimeMs ||
		a.ElvesID != b.ElvesID || a.PlantTimeMs != b.PlantTimeMs || a.Observed != b.Observed {
		return false
	}
	if len(a.StealUIDs) != len(b.StealUIDs) {
		return false
	}
	for i := range a.StealUIDs {
		if a.StealUIDs[i] != b.StealUIDs[i] {
			return false
		}
	}
	if len(a.ElvesStealUIDs) != len(b.ElvesStealUIDs) {
		return false
	}
	for i := range a.ElvesStealUIDs {
		if a.ElvesStealUIDs[i] != b.ElvesStealUIDs[i] {
			return false
		}
	}
	return true
}

// Lands returns a copy of the land map.
func (s *State) Lands() map[int32]LandView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]LandView, len(s.lands))
	for k, v := range s.lands {
		out[k] = v
	}
	return out
}

// LandRosterObserved reports whether the cold-start `100.0.1` land roster has
// arrived. Once true, absence from Lands means the server did not include that
// land in the player's opened/owned land list.
func (s *State) LandRosterObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.landRosterObserved
}

// SetFarmLands replaces the per-account runtime c_farmLand view loaded from
// the current client resource pack. This intentionally does not fall back to
// embedded static data, because stale land tables cause wrong unlock decisions.
func (s *State) SetFarmLands(lands []FarmLandInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.farmLands = make(map[int32]FarmLandInfo, len(lands))
	for _, land := range lands {
		if land.ID <= 0 {
			continue
		}
		s.farmLands[land.ID] = cloneFarmLandInfo(land)
	}
	s.farmLandObserved = len(s.farmLands) > 0
	s.bumpRevisionLocked()
}

// FarmLandConfigObserved reports whether the current client-side land table has
// been loaded for this running account session.
func (s *State) FarmLandConfigObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.farmLandObserved
}

// FarmLands returns the runtime c_farmLand rows loaded for this account,
// sorted by id. It returns nil until SetFarmLands succeeds.
func (s *State) FarmLands() []FarmLandInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.farmLandObserved {
		return nil
	}
	out := make([]FarmLandInfo, 0, len(s.farmLands))
	for _, land := range s.farmLands {
		if land.ID > 0 {
			out = append(out, cloneFarmLandInfo(land))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// FarmLand returns one runtime c_farmLand row for this account.
func (s *State) FarmLand(id int32) (FarmLandInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.farmLandObserved {
		return FarmLandInfo{}, false
	}
	land, ok := s.farmLands[id]
	if !ok {
		return FarmLandInfo{}, false
	}
	return cloneFarmLandInfo(land), true
}

// MarkLandsWatered forces the given lands to state=2 (growing) locally and
// spends one local water drop per land when item 7 is tracked. Some successful
// water RPC responses omit inventory deltas, so this keeps the next plan from
// reusing a stale water balance.

// MaxLandID returns the highest land ID currently tracked.
func (s *State) MaxLandID() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var max int32
	for id := range s.lands {
		if id > max {
			max = id
		}
	}
	return max
}
