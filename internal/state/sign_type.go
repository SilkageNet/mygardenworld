package state

import (
	"encoding/json"
	"strconv"
	"time"
)

func (s *State) applyBaseRewardsLocked(ns7 map[string]json.RawMessage) {
	rawMap, present := ns7["7"]
	if !present {
		return
	}
	s.baseRewardObserved = true
	if isJSONNull(rawMap) {
		s.invalidateBaseRewardMapLocked()
		return
	}
	var records map[string]json.RawMessage
	if json.Unmarshal(rawMap, &records) != nil {
		s.invalidateBaseRewardMapLocked()
		return
	}
	parsed := make(map[int32]json.RawMessage, len(records))
	for rawType, rawRecord := range records {
		parsedType, err := strconv.ParseInt(rawType, 10, 32)
		typeID := int32(parsedType)
		if err != nil || typeID <= 0 || strconv.FormatInt(parsedType, 10) != rawType {
			s.invalidateBaseRewardMapLocked()
			return
		}
		parsed[typeID] = rawRecord
	}
	if s.baseRewards == nil {
		s.baseRewards = make(map[int32]*BaseRewardView)
	}
	s.baseRewardMapValid = true
	for typeID, rawRecord := range parsed {
		s.applyBaseRewardRecordLocked(typeID, rawRecord)
	}
}

func (s *State) applyBaseRewardRecordLocked(typeID int32, raw json.RawMessage) {
	view := BaseRewardView{Observed: true, Type: typeID}
	if previous := s.baseRewards[typeID]; previous != nil {
		view = *previous
		view.Observed = true
		view.Type = typeID
	}
	if isJSONNull(raw) {
		view.Valid = false
		s.baseRewards[typeID] = &view
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		view.Valid = false
		s.baseRewards[typeID] = &view
		return
	}
	validFields := true
	if rawValue, present := fields["0"]; present {
		view.UID = 0
		view.UIDObserved = false
		if value, ok := readExactInt64Raw(rawValue); ok && value > 0 {
			view.UID = value
			view.UIDObserved = true
		} else {
			validFields = false
		}
	}
	if rawValue, present := fields["1"]; present {
		view.TypeObserved = false
		if value, ok := readExactInt32Raw(rawValue); ok && value == typeID {
			view.Type = value
			view.TypeObserved = true
		} else {
			validFields = false
		}
	}
	if rawValue, present := fields["2"]; present {
		view.Status = 0
		view.StatusObserved = false
		if value, ok := readExactInt32Raw(rawValue); ok && value >= 0 && value <= BaseRewardStatusReceived {
			view.Status = value
			view.StatusObserved = true
		} else {
			validFields = false
		}
	}
	if rawValue, present := fields["3"]; present {
		view.UpdatedAtMs = 0
		view.UpdatedAtObserved = false
		if value, ok := readExactInt64Raw(rawValue); ok && value > 0 {
			view.UpdatedAtMs = value
			view.UpdatedAtObserved = true
		} else {
			validFields = false
		}
	}
	if rawValue, present := fields["4"]; present {
		view.CreatedAtMs = 0
		view.CreatedAtObserved = false
		if value, ok := readExactInt64Raw(rawValue); ok && value > 0 {
			view.CreatedAtMs = value
			view.CreatedAtObserved = true
		} else {
			validFields = false
		}
	}
	view.Valid = validFields && view.TypeObserved && view.Type == typeID && view.StatusObserved &&
		view.UpdatedAtObserved && baseRewardDefinitionKnown(typeID)
	s.baseRewards[typeID] = &view
}

func baseRewardDefinitionKnown(typeID int32) bool {
	raw, ok := StaticRow("c_rwd", typeID)
	if !ok {
		return false
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return false
	}
	id, idOK := readStoryMainInt32(row["id"])
	trigger, triggerOK := readStoryMainInt32(row["triggerByClient"])
	reward, rewardOK := parseStoryMainCost(row["items"])
	return idOK && id == typeID && triggerOK && trigger == 1 && rewardOK && len(reward) > 0
}

func (s *State) invalidateBaseRewardMapLocked() {
	s.baseRewardMapValid = false
	for typeID, previous := range s.baseRewards {
		view := BaseRewardView{Observed: true, Type: typeID}
		if previous != nil {
			view = *previous
		}
		view.Valid = false
		s.baseRewards[typeID] = &view
	}
}

func (s *State) applySignTypesLocked(raw json.RawMessage) {
	s.signTypeObserved = true

	var namespace map[string]json.RawMessage
	if json.Unmarshal(raw, &namespace) != nil {
		s.invalidateSignTypeMapLocked()
		return
	}
	rawMap, present := namespace["0"]
	if !present {
		return
	}
	if isJSONNull(rawMap) {
		s.invalidateSignTypeMapLocked()
		return
	}
	var records map[string]json.RawMessage
	if json.Unmarshal(rawMap, &records) != nil {
		s.invalidateSignTypeMapLocked()
		return
	}

	parsed := make(map[int32]json.RawMessage, len(records))
	for rawType, rawRecord := range records {
		parsedType, err := strconv.ParseInt(rawType, 10, 32)
		typeID := int32(parsedType)
		if err != nil || typeID <= 0 || strconv.FormatInt(parsedType, 10) != rawType {
			s.invalidateSignTypeMapLocked()
			return
		}
		parsed[typeID] = rawRecord
	}

	if s.signTypes == nil {
		s.signTypes = make(map[int32]*SignTypeView)
	}
	s.signTypeMapValid = true
	for typeID, rawRecord := range parsed {
		s.applySignTypeRecordLocked(typeID, rawRecord)
	}
}

func (s *State) applySignTypeRecordLocked(typeID int32, raw json.RawMessage) {
	view := SignTypeView{Observed: true, Type: typeID}
	if previous := s.signTypes[typeID]; previous != nil {
		view = *previous
		view.Observed = true
		view.Type = typeID
	}
	if isJSONNull(raw) {
		view.Valid = false
		s.signTypes[typeID] = &view
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		view.Valid = false
		s.signTypes[typeID] = &view
		return
	}

	validFields := true
	if rawValue, present := fields["0"]; present {
		view.UID = 0
		view.UIDObserved = false
		if value, ok := readExactInt64Raw(rawValue); ok && value > 0 {
			view.UID = value
			view.UIDObserved = true
		} else {
			validFields = false
		}
	}
	if rawValue, present := fields["1"]; present {
		view.TypeObserved = false
		if value, ok := readExactInt32Raw(rawValue); ok && value == typeID {
			view.Type = value
			view.TypeObserved = true
		} else {
			validFields = false
		}
	}
	if rawValue, present := fields["2"]; present {
		view.LastTimeMs = 0
		view.LastTimeObserved = false
		if value, ok := readExactInt64Raw(rawValue); ok && value >= 0 {
			view.LastTimeMs = value
			view.LastTimeObserved = true
		} else {
			validFields = false
		}
	}
	if rawValue, present := fields["3"]; present {
		view.SignID = 0
		view.SignIDObserved = false
		if value, ok := readExactInt32Raw(rawValue); ok && value > 0 {
			view.SignID = value
			view.SignIDObserved = true
		} else {
			validFields = false
		}
	}
	if rawValue, present := fields["4"]; present {
		view.Status = 0
		view.StatusObserved = false
		if value, ok := readExactInt32Raw(rawValue); ok && value >= SignTypeStatusCanSign && value <= SignTypeStatusReceived {
			view.Status = value
			view.StatusObserved = true
		} else {
			validFields = false
		}
	}
	if rawValue, present := fields["5"]; present {
		view.UpdatedAtMs = 0
		view.UpdatedAtObserved = false
		if value, ok := readExactInt64Raw(rawValue); ok && value >= 0 {
			view.UpdatedAtMs = value
			view.UpdatedAtObserved = true
		} else {
			validFields = false
		}
	}
	if rawValue, present := fields["6"]; present {
		view.CreatedAtMs = 0
		view.CreatedAtObserved = false
		if value, ok := readExactInt64Raw(rawValue); ok && value >= 0 {
			view.CreatedAtMs = value
			view.CreatedAtObserved = true
		} else {
			validFields = false
		}
	}

	reward, rewardKnown := SignTypeRewardByID(view.SignID)
	view.Valid = validFields && view.TypeObserved && view.Type == typeID &&
		view.SignIDObserved && rewardKnown && reward.Type == typeID && view.StatusObserved
	s.signTypes[typeID] = &view
}

func (s *State) invalidateSignTypeMapLocked() {
	s.signTypeMapValid = false
	for typeID := range s.signTypes {
		s.invalidateSignTypeLocked(typeID)
	}
}

func (s *State) invalidateSignTypeLocked(typeID int32) {
	view := SignTypeView{Observed: true, Type: typeID}
	if previous := s.signTypes[typeID]; previous != nil {
		view = *previous
		view.Observed = true
		view.Type = typeID
	}
	view.Valid = false
	s.signTypes[typeID] = &view
}

// SignType returns one typed sign record and whether namespace 140 itself has
// been observed. A missing or malformed entry is deliberately returned as an
// invalid view so callers can fail closed without probing signType.sign.
func (s *State) SignType(typeID int32) (SignTypeView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.signTypeObserved {
		return SignTypeView{Type: typeID}, false
	}
	view := SignTypeView{Type: typeID}
	if current := s.signTypes[typeID]; current != nil {
		view = *current
	}
	if !s.signTypeMapValid {
		view.Valid = false
	}
	return view, true
}

// BaseReward returns one namespace 7.7 G.IRwd record and whether the reward
// map itself has been observed.
func (s *State) BaseReward(typeID int32) (BaseRewardView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.baseRewardObserved {
		return BaseRewardView{Type: typeID}, false
	}
	view := BaseRewardView{Type: typeID}
	if current := s.baseRewards[typeID]; current != nil {
		view = *current
	}
	if !s.baseRewardMapValid {
		view.Valid = false
	}
	return view, true
}

// UpdatedToday and UpdatedBeforeToday provide the exact day gates used by the
// client around signType type=1. Future timestamps return false for both.
func (v BaseRewardView) UpdatedToday(now time.Time) bool {
	return v.UpdatedAtObserved && timestampSameLocalDay(v.UpdatedAtMs, now)
}

func (v BaseRewardView) UpdatedBeforeToday(now time.Time) bool {
	return v.UpdatedAtObserved && timestampBeforeLocalDay(v.UpdatedAtMs, now)
}

func (v SignTypeView) UpdatedToday(now time.Time) bool {
	return v.UpdatedAtObserved && timestampSameLocalDay(v.UpdatedAtMs, now)
}

func (v SignTypeView) UpdatedBeforeToday(now time.Time) bool {
	return v.UpdatedAtObserved && timestampBeforeLocalDay(v.UpdatedAtMs, now)
}

func timestampSameLocalDay(raw int64, now time.Time) bool {
	value, ok := timestampTime(raw, now.Location())
	if !ok {
		return false
	}
	y1, m1, d1 := value.Date()
	y2, m2, d2 := now.In(now.Location()).Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func timestampBeforeLocalDay(raw int64, now time.Time) bool {
	value, ok := timestampTime(raw, now.Location())
	if !ok {
		return false
	}
	localNow := now.In(now.Location())
	year, month, day := localNow.Date()
	start := time.Date(year, month, day, 0, 0, 0, 0, now.Location())
	return value.Before(start)
}

func timestampTime(raw int64, location *time.Location) (time.Time, bool) {
	if raw <= 0 {
		return time.Time{}, false
	}
	if location == nil {
		location = time.Local
	}
	if raw >= 19000101 && raw <= 29991231 {
		value, err := time.ParseInLocation("20060102", strconv.FormatInt(raw, 10), location)
		return value, err == nil
	}
	if raw > 1_000_000_000_000 {
		return time.UnixMilli(raw).In(location), true
	}
	return time.Unix(raw, 0).In(location), true
}

// SignTypeEnterAttemptedToday is a local de-duplication marker for an enter
// request that legitimately returned an empty payload. It does not mutate or
// infer server status.
func (s *State) SignTypeEnterAttemptedToday(typeID int32, now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return timestampSameLocalDay(s.signTypeEnterAtMs[typeID], now)
}

func (s *State) MarkSignTypeEnterAttempt(typeID int32, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.signTypeEnterAtMs == nil {
		s.signTypeEnterAtMs = make(map[int32]int64)
	}
	s.signTypeEnterAtMs[typeID] = now.UnixMilli()
}

// InvalidateSignType blocks further automatic calls for one type until a new
// valid namespace 140 delta is observed. It is used after server code 3500 or
// a missing postcondition instead of treating the error as retryable.
func (s *State) InvalidateSignType(typeID int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.signTypes == nil {
		s.signTypes = make(map[int32]*SignTypeView)
	}
	s.invalidateSignTypeLocked(typeID)
}
