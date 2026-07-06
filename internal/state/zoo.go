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
		if zoo, ok := parseZooView(rawData); ok {
			s.zoo = zoo
		}
	}
	if rawPets, ok := ns33["1"]; ok {
		s.applyZooPetMapLocked(rawPets)
	}
}

func parseZooView(raw json.RawMessage) (ZooView, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return ZooView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ZooView{}, false
	}
	view := ZooView{Observed: true}
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
	if n, ok := readInt32JSONField(fields, "2"); ok {
		pet.MoodValue = n
	}
	if n, ok := readInt32JSONField(fields, "3"); ok {
		pet.SatietyValue = n
	}
	if rawFood, ok := fields["4"]; ok {
		pet.FoodstuffIDs = readInt32OrderedListRaw(rawFood)
	}
	if n, ok := readInt32JSONField(fields, "5"); ok {
		pet.Status = n
	}
	if n, ok := readInt32JSONField(fields, "9"); ok {
		pet.GoOutEventID = n
	}
	if rawEvents, ok := fields["10"]; ok {
		pet.SpecialEventIDs = readInt32ListRaw(rawEvents)
	}
	if n, ok := readInt64JSONField(fields, "12"); ok {
		pet.StrokeCdTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "13"); ok {
		pet.GetHomeTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "14"); ok {
		pet.StatusCdTimeMs = n
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

// ReadyZooFeedPetIDs returns pets with bowl food that can currently eat.
func (s *State) ReadyZooFeedPetIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.zooPets))
	for petID, pet := range s.zooPets {
		if pet == nil || pet.PetID <= 0 || len(pet.FoodstuffIDs) == 0 {
			continue
		}
		if zooPetCanEat(pet.Status) {
			out = append(out, petID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyZooStrokePetIDs returns pets that match the client's touch red-dot gate.
func (s *State) ReadyZooStrokePetIDs(now time.Time) []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.zooPets))
	nowMs := now.UnixMilli()
	moodMax := ZooMoodMax()
	for petID, pet := range s.zooPets {
		if pet == nil || pet.PetID <= 0 || pet.Status <= 0 {
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

// ZooEventActions returns conservative animal-event action candidates. Events
// that require share/video, contain ambiguous costs, or lack a safe action are
// returned as blocked markers instead of runnable operations.
func (s *State) ZooEventActions() []ZooEventAction {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ZooEventAction, 0, len(s.zooPets))
	for petID, pet := range s.zooPets {
		if pet == nil || pet.PetID <= 0 {
			continue
		}
		if pet.GoOutEventID > 0 {
			action := zooEventActionForPet(petID, pet.GoOutEventID)
			out = append(out, action)
		}
		for _, eventID := range pet.SpecialEventIDs {
			if eventID <= 0 || eventID == pet.GoOutEventID {
				continue
			}
			action := zooEventActionForPet(petID, eventID)
			out = append(out, action)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Blocked != out[j].Blocked {
			return !out[i].Blocked
		}
		if out[i].PetID != out[j].PetID {
			return out[i].PetID < out[j].PetID
		}
		return out[i].EventID < out[j].EventID
	})
	return out
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

func zooEventActionForPet(petID, eventID int32) ZooEventAction {
	action := ZooEventAction{
		PetID:        petID,
		EventID:      eventID,
		TableID:      eventID,
		IsShareVideo: 0,
	}
	info, ok := ZooEventInfoByID(eventID)
	if ok {
		action.Name = info.Name
	}
	if eventID == 4001 || eventID == 5001 {
		action.Action = "find_pet"
		if ok && info.SharedID > 0 {
			action.Blocked = true
			action.BlockedReason = "寻回宠物事件关联分享/广告路径，保守策略不自动执行"
		}
		return action
	}
	action.Action = "handle_event"
	action.Agree = true
	if !ok {
		action.Blocked = true
		action.BlockedReason = "宠物事件静态配置未识别，保守策略不自动处理"
		return action
	}
	if info.SharedID > 0 {
		action.Blocked = true
		action.BlockedReason = "宠物事件关联分享/广告路径，保守策略不自动执行"
		return action
	}
	if info.HasReward2 || len(info.Reward2) > 0 || info.NoHandle || info.Result {
		action.Blocked = true
		action.BlockedReason = "宠物事件存在选择或成本结果，需人工确认"
		return action
	}
	if info.Type != 2 || (!info.HasReward1 && len(info.Reward1) == 0) {
		action.Blocked = true
		action.BlockedReason = "宠物事件收益/成本不明确，保守策略不自动处理"
		return action
	}
	return action
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
