package state

import (
	"encoding/json"
	"sort"
	"time"
)

func (s *State) applyZooLocked(raw json.RawMessage) {
	var ns33 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns33); err != nil {
		return
	}
	s.zooObserved = true
	if rawData, ok := ns33["0"]; ok {
		if zoo, ok := parseZooView(rawData, cloneZooView(s.zoo)); ok {
			s.zoo = zoo
		}
	}
	if rawPets, ok := ns33["1"]; ok {
		s.applyZooPetMapLocked(rawPets)
	}
}

func parseZooView(raw json.RawMessage, base ZooView) (ZooView, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return ZooView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ZooView{}, false
	}
	view := base
	view.Observed = true
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if rawPetIDs, ok := fields["3"]; ok {
		view.PetIDs = readInt32ListRaw(rawPetIDs)
	}
	if n, ok := readInt64JSONField(fields, "2"); ok {
		view.ReadLogTimeMs = n
	}
	if n, ok := readInt32JSONField(fields, "6"); ok {
		view.Comfort = n
	}
	if n, ok := readInt64JSONField(fields, "8"); ok {
		view.UpdatedAtMs = n
	}
	if rawRewards, ok := fields["13"]; ok {
		view.SouvenirRewardIDs = readInt32ListRaw(rawRewards)
	}
	return view, true
}

func (s *State) applyZooPetMapLocked(raw json.RawMessage) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var petMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &petMap); err != nil {
		return
	}
	if s.zooPets == nil {
		s.zooPets = make(map[int32]*ZooPetView)
	}
	for petIDStr, rawPet := range petMap {
		petID := atoi32(petIDStr)
		base := ZooPetView{PetID: petID}
		if old := s.zooPets[petID]; old != nil {
			base = cloneZooPetView(*old)
		}
		pet, ok := parseZooPetView(rawPet, base)
		if !ok || pet.PetID <= 0 {
			continue
		}
		cp := pet
		s.zooPets[pet.PetID] = &cp
	}
}

func parseZooPetView(raw json.RawMessage, base ZooPetView) (ZooPetView, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return ZooPetView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ZooPetView{}, false
	}
	pet := base
	if n, ok := readInt64JSONField(fields, "0"); ok {
		pet.UID = n
	}
	if n, ok := readInt32JSONField(fields, "1"); ok && n > 0 {
		pet.PetID = n
	}
	if rawMood, observed := fields["2"]; observed {
		if n, ok := readInt32Raw(rawMood); ok {
			pet.MoodObserved = true
			pet.MoodValue = n
		}
	}
	if rawSatiety, observed := fields["3"]; observed {
		if n, ok := readInt32Raw(rawSatiety); ok {
			pet.SatietyObserved = true
			pet.SatietyValue = n
		}
	}
	if rawFood, ok := fields["4"]; ok {
		pet.FoodstuffObserved = true
		pet.FoodstuffIDs = readInt32OrderedListRaw(rawFood)
	}
	if rawStatus, observed := fields["5"]; observed {
		if n, ok := readInt32Raw(rawStatus); ok {
			pet.StatusObserved = true
			pet.Status = n
		}
	}
	if n, ok := readInt32JSONField(fields, "9"); ok {
		pet.GoOutEventID = n
	}
	if rawEvents, ok := fields["10"]; ok {
		pet.SpecialEventIDs = readInt32ListRaw(rawEvents)
	}
	if rawStrokeCd, observed := fields["12"]; observed {
		pet.StrokeCdTimeObserved = true
		pet.StrokeCdTimeMs = 0
		if n, ok := readInt64Raw(rawStrokeCd); ok {
			pet.StrokeCdTimeMs = n
		}
	}
	if n, ok := readInt64JSONField(fields, "13"); ok {
		pet.GetHomeTimeMs = n
	}
	if rawStatusCd, observed := fields["14"]; observed {
		pet.StatusCdTimeObserved = true
		pet.StatusCdTimeMs = 0
		if n, ok := readInt64Raw(rawStatusCd); ok {
			pet.StatusCdTimeMs = n
		}
	}
	if n, ok := readInt64JSONField(fields, "15"); ok {
		pet.GoOutCdTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "19"); ok {
		pet.ReadLogTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "23"); ok {
		pet.UpdatedAtMs = n
	}
	if rawTimes, ok := fields["25"]; ok {
		pet.EventTriggerTimes = readInt64RawMap(rawTimes)
	}
	return pet, true
}

func cloneZooView(src ZooView) ZooView {
	out := src
	out.PetIDs = append([]int32(nil), src.PetIDs...)
	out.SouvenirRewardIDs = append([]int32(nil), src.SouvenirRewardIDs...)
	return out
}

func cloneZooPetView(src ZooPetView) ZooPetView {
	out := src
	out.FoodstuffIDs = append([]int32(nil), src.FoodstuffIDs...)
	out.SpecialEventIDs = append([]int32(nil), src.SpecialEventIDs...)
	if src.EventTriggerTimes != nil {
		out.EventTriggerTimes = make(map[int32]int64, len(src.EventTriggerTimes))
		for id, t := range src.EventTriggerTimes {
			out.EventTriggerTimes[id] = t
		}
	}
	return out
}

// ZooObserved reports whether namespace 33 has been observed.
func (s *State) ZooObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zooObserved
}

// Zoo returns the tracked animal-home state.
func (s *State) Zoo() ZooView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneZooView(s.zoo)
}

// ZooPets returns a defensive copy of the pet map.
func (s *State) ZooPets() map[int32]ZooPetView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]ZooPetView, len(s.zooPets))
	for id, pet := range s.zooPets {
		if pet == nil {
			continue
		}
		out[id] = cloneZooPetView(*pet)
	}
	return out
}

// ReadyZooStatusRefreshPetIDs returns pets whose observed status cooldown has
// expired. Missing cooldown fields are not interpreted as an expired zero.
func (s *State) ReadyZooStatusRefreshPetIDs(now time.Time) []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.zooPets))
	nowMs := now.UnixMilli()
	for petID, pet := range s.zooPets {
		if pet == nil || pet.PetID <= 0 || !pet.StatusCdTimeObserved {
			continue
		}
		if pet.StatusCdTimeMs > 0 && pet.StatusCdTimeMs <= nowMs {
			out = append(out, petID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// NextZooFoodstuffPlan returns the first deterministic, inventory-backed bowl
// stocking action. Food 1501 has priority over 1502 and a request never mixes
// food types.
func (s *State) NextZooFoodstuffPlan() (ZooFoodstuffPlan, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	capacity := ZooFoodBowlCapacity()
	satietyMax := ZooSatietyMax()
	if capacity <= 0 || satietyMax <= 0 {
		return ZooFoodstuffPlan{}, false
	}
	petIDs := make([]int32, 0, len(s.zooPets))
	for petID, pet := range s.zooPets {
		if pet == nil || pet.PetID <= 0 || !pet.FoodstuffObserved || !pet.StatusObserved || !pet.SatietyObserved {
			continue
		}
		if !zooPetCanEat(pet.Status) || pet.SatietyValue >= satietyMax {
			continue
		}
		petIDs = append(petIDs, petID)
	}
	sort.Slice(petIDs, func(i, j int) bool { return petIDs[i] < petIDs[j] })

	for _, petID := range petIDs {
		pet := s.zooPets[petID]
		empty := capacity - int32(len(pet.FoodstuffIDs))
		if empty <= 0 {
			continue
		}
		for _, foodstuffID := range []int32{1501, 1502} {
			count := s.inventory[foodstuffID]
			if count <= 0 {
				continue
			}
			if count > empty {
				count = empty
			}
			return ZooFoodstuffPlan{PetID: petID, FoodstuffID: foodstuffID, Count: count}, true
		}
	}
	return ZooFoodstuffPlan{}, false
}

// ReadyZooStrokePetIDs returns pets that match the client's touch red-dot gate.
func (s *State) ReadyZooStrokePetIDs(now time.Time) []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.zooPets))
	nowMs := now.UnixMilli()
	moodMax := ZooMoodMax()
	for petID, pet := range s.zooPets {
		if pet == nil || pet.PetID <= 0 || !pet.StatusObserved || !pet.MoodObserved || !pet.StrokeCdTimeObserved || pet.Status <= 0 {
			continue
		}
		if !zooPetTouchable(pet.Status) {
			continue
		}
		if moodMax > 0 && pet.MoodValue >= moodMax {
			continue
		}
		if pet.StrokeCdTimeMs > 0 && nowMs < pet.StrokeCdTimeMs {
			continue
		}
		out = append(out, petID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ZooEventActions intentionally returns no candidates until namespace 33.2
// server logs are modeled. Pet fields alone are not a reliable event source.
func (s *State) ZooEventActions() []ZooEventAction {
	return nil
}

// ZooMoodMax returns the client-configured pet mood cap.
func ZooMoodMax() int32 {
	raw, ok := StaticRow("c_zoo", -1)
	if !ok {
		return 100
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return 100
	}
	if n, ok := readInt32JSONField(fields, "$moodMax1", "$moodMax"); ok && n > 0 {
		return n
	}
	return 100
}

// ZooFoodBowlCapacity returns the decoded client-configured bowl capacity.
func ZooFoodBowlCapacity() int32 {
	raw, ok := StaticRow("c_zoo", -1)
	if !ok {
		return 0
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return 0
	}
	if n, ok := readInt32JSONField(fields, "$catBasinMax"); ok && n > 0 {
		return n
	}
	return 0
}

// ZooSatietyMax returns the decoded client-configured pet satiety cap.
func ZooSatietyMax() int32 {
	raw, ok := StaticRow("c_zoo", -1)
	if !ok {
		return 0
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return 0
	}
	if n, ok := readInt32JSONField(fields, "$satietyMax"); ok && n > 0 {
		return n
	}
	return 0
}

func zooPetTouchable(status int32) bool {
	fields, ok := zooStateRow(status)
	if !ok {
		return true
	}
	if n, ok := readInt32JSONField(fields, "isTouch"); ok {
		return n != 0
	}
	return true
}

func zooPetCanEat(status int32) bool {
	fields, ok := zooStateRow(status)
	if !ok {
		return false
	}
	if n, ok := readInt32JSONField(fields, "isEat"); ok {
		return n != 0
	}
	return false
}

func zooStateRow(status int32) (map[string]json.RawMessage, bool) {
	raw, ok := StaticRow("c_zooState", status)
	if !ok {
		return nil, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, false
	}
	return fields, true
}
