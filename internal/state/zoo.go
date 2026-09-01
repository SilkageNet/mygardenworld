package state

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
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
	if rawLogs, ok := ns33["2"]; ok {
		s.applyZooLogMapLocked(rawLogs)
	}
	if rawSouvenirs, ok := ns33["4"]; ok {
		s.applyZooSouvenirMapLocked(rawSouvenirs)
	}
	if rawDecorates, ok := ns33["5"]; ok {
		s.applyZooDecorateMapLocked(rawDecorates)
	}
	if rawDecSuits, ok := ns33["6"]; ok {
		s.applyZooDecorateSuitMapLocked(rawDecSuits)
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
	if rawDecMap, ok := fields["1"]; ok {
		if decMap, valid := readZooDecorateMapRaw(rawDecMap); valid {
			view.ZooDecorateMap = decMap
			view.ZooDecorateMapObserved = true
		}
	}
	if n, ok := readInt64JSONField(fields, "2"); ok {
		view.ReadLogTimeMs = n
	}
	if rawPetIDs, ok := fields["3"]; ok {
		view.PetIDs = readInt32ListRaw(rawPetIDs)
	}
	if n, ok := readInt64JSONField(fields, "4"); ok {
		view.AsleepBeginTimeMs = n
	}
	if rawSleep, observed := fields["5"]; observed {
		view.LastSetSleepTimeObserved = false
		view.LastSetSleepTimeMs = 0
		if n, ok := readZooLogInt64Raw(bytes.TrimSpace(rawSleep)); ok {
			view.LastSetSleepTimeMs = n
			view.LastSetSleepTimeObserved = true
		}
	}
	if n, ok := readInt32JSONField(fields, "6"); ok {
		view.Comfort = n
	}
	if n, ok := readInt64JSONField(fields, "7"); ok {
		view.CreatedAtMs = n
	}
	if n, ok := readInt64JSONField(fields, "8"); ok {
		view.UpdatedAtMs = n
	}
	if rawGuides, ok := fields["9"]; ok {
		view.GuideIDs = readInt32ListRaw(rawGuides)
	}
	if n, ok := readInt32JSONField(fields, "10"); ok {
		view.HasChangeSleep = n
	}
	if rawRewards, ok := fields["13"]; ok {
		view.SouvenirRewardIDs = nil
		view.SouvenirRewardIDsObserved = false
		if ids, valid := readZooPositiveIDList(rawRewards, true); valid {
			view.SouvenirRewardIDs = ids
			view.SouvenirRewardIDsObserved = true
		}
	}
	return view, true
}

func readZooDecorateMapRaw(raw json.RawMessage) (map[int32][]int32, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, false
	}
	out := make(map[int32][]int32, len(m))
	for pidRaw, idsRaw := range m {
		pid := atoi32(pidRaw)
		if pid <= 0 {
			continue
		}
		out[pid] = readInt32ListRaw(idsRaw)
	}
	return out, true
}

func readZooPositiveIDList(raw json.RawMessage, nullIsEmpty bool) ([]int32, bool) {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, nullIsEmpty
	}
	var values []json.RawMessage
	if len(trimmed) == 0 || json.Unmarshal(trimmed, &values) != nil {
		return nil, false
	}
	ids := make([]int32, 0, len(values))
	for _, rawValue := range values {
		id, ok := readZooLogInt32Raw(rawValue)
		if !ok || id <= 0 {
			return nil, false
		}
		ids = append(ids, id)
	}
	return uniqueSortedInt32s(ids), true
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
	if rawName, observed := fields["6"]; observed {
		pet.NameObserved = false
		pet.Name = ""
		var s string
		if trimmed := bytes.TrimSpace(rawName); len(trimmed) > 0 && string(trimmed) != "null" && json.Unmarshal(trimmed, &s) == nil {
			pet.Name = s
			pet.NameObserved = true
		}
	}
	if n, ok := readInt32JSONField(fields, "7"); ok {
		pet.ConDozeCount = n
	}
	if rawStrokeCd, observed := fields["8"]; observed {
		pet.StrokeCdObserved = false
		pet.StrokeCd = 0
		if n, ok := readZooLogInt32Raw(bytes.TrimSpace(rawStrokeCd)); ok {
			pet.StrokeCd = n
			pet.StrokeCdObserved = true
		}
	}
	if n, ok := readInt32JSONField(fields, "9"); ok {
		pet.GoOutEventID = n
	}
	if rawEvents, ok := fields["10"]; ok {
		pet.SpecialEventIDs = readInt32ListRaw(rawEvents)
	}
	if rawLastStroke, observed := fields["11"]; observed {
		pet.LastStrokeTimeObserved = false
		pet.LastStrokeTimeMs = 0
		if n, ok := readZooLogInt64Raw(bytes.TrimSpace(rawLastStroke)); ok {
			pet.LastStrokeTimeMs = n
			pet.LastStrokeTimeObserved = true
		}
	}
	if rawStrokeCdTime, observed := fields["12"]; observed {
		pet.StrokeCdTimeObserved = true
		pet.StrokeCdTimeMs = 0
		if n, ok := readInt64Raw(rawStrokeCdTime); ok {
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
	if rawCalTime, observed := fields["16"]; observed {
		pet.CalTimeObserved = false
		pet.CalTimeMs = 0
		if n, ok := readZooLogInt64Raw(bytes.TrimSpace(rawCalTime)); ok {
			pet.CalTimeMs = n
			pet.CalTimeObserved = true
		}
	}
	if rawHunger, observed := fields["17"]; observed {
		pet.HungerTimeObserved = false
		pet.HungerTimeMs = 0
		if n, ok := readZooLogInt64Raw(bytes.TrimSpace(rawHunger)); ok {
			pet.HungerTimeMs = n
			pet.HungerTimeObserved = true
		}
	}
	if rawExt, observed := fields["18"]; observed {
		pet.Ext = nil
		pet.ExtObserved = false
		if trimmed := bytes.TrimSpace(rawExt); len(trimmed) > 0 && string(trimmed) != "null" && isZooLogJSONObject(rawExt) {
			pet.Ext = append(json.RawMessage(nil), rawExt...)
			pet.ExtObserved = true
		}
	}
	if rawReadLogTime, observed := fields["19"]; observed {
		pet.ReadLogTimeObserved = false
		pet.ReadLogTimeMs = 0
		if trimmed := bytes.TrimSpace(rawReadLogTime); len(trimmed) > 0 && string(trimmed) != "null" {
			if n, ok := readZooLogInt64Raw(rawReadLogTime); ok {
				pet.ReadLogTimeMs = n
				pet.ReadLogTimeObserved = true
			}
		}
	}
	if n, ok := readInt64JSONField(fields, "23"); ok {
		pet.UpdatedAtMs = n
	}
	if n, ok := readInt64JSONField(fields, "24"); ok {
		pet.CreatedAtMs = n
	}
	if rawTimes, ok := fields["25"]; ok {
		pet.EventTriggerTimes = readInt64RawMap(rawTimes)
	}
	return pet, true
}

func (s *State) applyZooLogMapLocked(raw json.RawMessage) {
	if !isZooLogJSONObject(raw) {
		s.invalidateZooLogsLocked("宠物日志集合不是对象，旧日志状态已失去可信度")
		return
	}
	var logMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &logMap); err != nil {
		s.invalidateZooLogsLocked("宠物日志集合解析失败，旧日志状态已失去可信度")
		return
	}
	s.zooLogsObserved = true
	s.zooLogsInvalidReason = ""
	if s.zooLogs == nil {
		s.zooLogs = make(map[string]*ZooLogView)
	}
	for key, rawLog := range logMap {
		if bytes.Equal(bytes.TrimSpace(rawLog), []byte("null")) {
			delete(s.zooLogs, key)
			continue
		}
		base := ZooLogView{Key: key}
		if old := s.zooLogs[key]; old != nil && !old.Malformed {
			base = cloneZooLogView(*old)
		}
		log, ok := parseZooLogView(rawLog, base)
		if !ok {
			base.Malformed = true
			base.MalformedReason = "宠物日志条目不是可解析对象，拒绝沿用旧状态"
			cp := base
			s.zooLogs[key] = &cp
			continue
		}
		log.Malformed = false
		log.MalformedReason = ""
		cp := log
		s.zooLogs[key] = &cp
	}
}

func (s *State) applyZooSouvenirMapLocked(raw json.RawMessage) {
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return
	}
	if !isZooLogJSONObject(trimmed) {
		s.invalidateZooSouvenirsLocked()
		return
	}
	var souvenirMap map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &souvenirMap); err != nil {
		s.invalidateZooSouvenirsLocked()
		return
	}
	if s.zooSouvenirs == nil {
		s.zooSouvenirs = make(map[int32]*ZooSouvenirView)
	}
	s.zooSouvenirsObserved = true
	for rawTempID, rawSouvenir := range souvenirMap {
		value, err := strconv.ParseInt(rawTempID, 10, 32)
		tempID := int32(value)
		if err != nil || tempID <= 0 {
			s.invalidateZooSouvenirsLocked()
			return
		}
		if bytes.Equal(bytes.TrimSpace(rawSouvenir), []byte("null")) {
			delete(s.zooSouvenirs, tempID)
			continue
		}
		base := ZooSouvenirView{MapTempID: tempID, TempID: tempID}
		if old := s.zooSouvenirs[tempID]; old != nil {
			base = *old
		}
		souvenir, ok := parseZooSouvenirView(rawSouvenir, base)
		if !ok {
			s.invalidateZooSouvenirsLocked()
			return
		}
		cp := souvenir
		s.zooSouvenirs[tempID] = &cp
	}
}

func (s *State) invalidateZooSouvenirsLocked() {
	s.zooSouvenirsObserved = false
	s.zooSouvenirs = make(map[int32]*ZooSouvenirView)
}

func parseZooSouvenirView(raw json.RawMessage, base ZooSouvenirView) (ZooSouvenirView, bool) {
	if !isZooLogJSONObject(raw) {
		return ZooSouvenirView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ZooSouvenirView{}, false
	}
	souvenir := base
	setInt64 := func(key string, value *int64, observed *bool) {
		if rawValue, present := fields[key]; present {
			*value = 0
			*observed = false
			if n, ok := readZooLogInt64Raw(rawValue); ok {
				*value = n
				*observed = true
			}
		}
	}
	setInt32 := func(key string, value *int32, observed *bool) {
		if rawValue, present := fields[key]; present {
			*value = 0
			*observed = false
			if n, ok := readZooLogInt32Raw(rawValue); ok {
				*value = n
				*observed = true
			}
		}
	}
	setInt64("0", &souvenir.UID, &souvenir.UIDObserved)
	setInt32("1", &souvenir.TempID, &souvenir.TempIDObserved)
	if rawIsRead, present := fields["2"]; present {
		souvenir.IsRead = 0
		souvenir.IsReadObserved = false
		if n, ok := readZooLogInt32Raw(rawIsRead); ok {
			souvenir.IsRead = n
			souvenir.IsReadObserved = true
		}
	}
	setInt64("3", &souvenir.UpdatedAtMs, &souvenir.UpdatedAtObserved)
	setInt64("4", &souvenir.CreatedAtMs, &souvenir.CreatedAtObserved)
	if souvenir.TempIDObserved && souvenir.TempID != souvenir.MapTempID {
		souvenir.TempID = souvenir.MapTempID
		souvenir.TempIDObserved = false
	}
	return souvenir, true
}

func isZooLogJSONObject(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) >= 2 && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}'
}

func (s *State) invalidateZooLogsLocked(reason string) {
	s.zooLogsObserved = false
	s.zooLogsInvalidReason = reason
}

func parseZooLogView(raw json.RawMessage, base ZooLogView) (ZooLogView, bool) {
	if !isZooLogJSONObject(raw) {
		return ZooLogView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ZooLogView{}, false
	}
	log := base
	valid := true
	setInt64 := func(key string, value *int64, observed *bool) {
		if rawValue, present := fields[key]; present {
			*observed = false
			trimmed := bytes.TrimSpace(rawValue)
			if len(trimmed) == 0 || string(trimmed) == "null" {
				return
			}
			if n, ok := readZooLogInt64Raw(rawValue); ok {
				*value = n
				*observed = true
			} else {
				valid = false
			}
		}
	}
	setInt32 := func(key string, value *int32, observed *bool) {
		if rawValue, present := fields[key]; present {
			*observed = false
			trimmed := bytes.TrimSpace(rawValue)
			if len(trimmed) == 0 || string(trimmed) == "null" {
				return
			}
			if n, ok := readZooLogInt32Raw(rawValue); ok {
				*value = n
				*observed = true
			} else {
				valid = false
			}
		}
	}
	setInt64("0", &log.UID, &log.UIDObserved)
	setInt32("1", &log.PetID, &log.PetIDObserved)
	setInt32("2", &log.Index, &log.IndexObserved)
	setInt32("3", &log.MoodChangeValue, &log.MoodChangeObserved)
	setInt32("4", &log.SatietyChangeValue, &log.SatietyChangeObserved)
	setInt32("5", &log.GoOutEventID, &log.GoOutEventIDObserved)
	setInt32("6", &log.EventType, &log.EventTypeObserved)
	setInt32("7", &log.ProType, &log.ProTypeObserved)
	if rawGain, present := fields["8"]; present {
		log.Gain = nil
		log.GainObserved = false
		if trimmed := bytes.TrimSpace(rawGain); len(trimmed) > 0 && string(trimmed) != "null" {
			log.Gain, log.GainObserved = readZooLogItemMap(rawGain)
			valid = valid && log.GainObserved
		}
	}
	if rawConsume, present := fields["9"]; present {
		log.Consume = nil
		log.ConsumeObserved = false
		if trimmed := bytes.TrimSpace(rawConsume); len(trimmed) > 0 && string(trimmed) != "null" {
			log.Consume, log.ConsumeObserved = readZooLogItemMap(rawConsume)
			valid = valid && log.ConsumeObserved
		}
	}
	if rawSouvenir, present := fields["10"]; present {
		log.Souvenir = nil
		log.SouvenirObserved = false
		if trimmed := bytes.TrimSpace(rawSouvenir); len(trimmed) > 0 && string(trimmed) != "null" {
			log.Souvenir, log.SouvenirObserved = readZooLogItemMap(rawSouvenir)
			valid = valid && log.SouvenirObserved
		}
	}
	if rawExt, present := fields["11"]; present {
		log.Ext = ZooLogExtView{}
		log.ExtObserved = false
		if trimmed := bytes.TrimSpace(rawExt); len(trimmed) > 0 && string(trimmed) != "null" {
			ext, ok := parseZooLogExtView(rawExt)
			if ok {
				log.Ext = ext
				log.ExtObserved = true
			} else {
				log.Ext = ZooLogExtView{}
				log.ExtObserved = false
				valid = false
			}
		}
	}
	setInt64("12", &log.UpdatedAtMs, &log.UpdatedAtObserved)
	setInt64("13", &log.CreatedAtMs, &log.CreatedAtObserved)
	setInt64("14", &log.InsertedAtMs, &log.InsertedAtObserved)
	return log, valid
}

func parseZooLogExtView(raw json.RawMessage) (ZooLogExtView, bool) {
	ext := ZooLogExtView{ConsumeObserved: true, Consume2Observed: true}
	if !isZooLogJSONObject(raw) {
		return ZooLogExtView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ZooLogExtView{}, false
	}
	if rawName, present := fields["0"]; present {
		if json.Unmarshal(rawName, &ext.UserName) != nil {
			return ZooLogExtView{}, false
		}
		ext.UserNameObserved = true
	}
	if rawName, present := fields["1"]; present {
		if json.Unmarshal(rawName, &ext.PetName) != nil {
			return ZooLogExtView{}, false
		}
		ext.PetNameObserved = true
	}
	if rawPetID, present := fields["2"]; present {
		n, ok := readZooLogInt32Raw(rawPetID)
		if !ok {
			return ZooLogExtView{}, false
		}
		ext.PetID = n
		ext.PetIDObserved = true
	}
	if rawConsume, present := fields["3"]; present {
		var ok bool
		ext.Consume, ok = readZooLogItemMap(rawConsume)
		if !ok {
			return ZooLogExtView{}, false
		}
	}
	if rawConsume, present := fields["4"]; present {
		var ok bool
		ext.Consume2, ok = readZooLogItemMap(rawConsume)
		if !ok {
			return ZooLogExtView{}, false
		}
	}
	if rawBack, present := fields["5"]; present {
		n, ok := readZooLogInt32Raw(rawBack)
		if !ok {
			return ZooLogExtView{}, false
		}
		ext.IsUserBack = n
		ext.IsUserBackObserved = true
	}
	return ext, true
}

func readZooLogItemMap(raw json.RawMessage) (map[int32]int32, bool) {
	if !isZooLogJSONObject(raw) {
		return nil, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, false
	}
	out := make(map[int32]int32, len(fields))
	for rawID, rawCount := range fields {
		itemValue, itemErr := strconv.ParseInt(rawID, 10, 32)
		itemID := int32(itemValue)
		count, ok := readZooLogInt32Raw(rawCount)
		if itemErr != nil || itemID <= 0 || !ok {
			return nil, false
		}
		out[itemID] = count
	}
	if len(out) == 0 {
		return nil, true
	}
	return out, true
}

func readZooLogInt32Raw(raw json.RawMessage) (int32, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var n int32
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, false
	}
	value, err := strconv.ParseInt(s, 10, 32)
	return int32(value), err == nil
}

func readZooLogInt64Raw(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false
	}
	var n int64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, false
	}
	value, err := strconv.ParseInt(s, 10, 64)
	return value, err == nil
}

func cloneZooView(src ZooView) ZooView {
	out := src
	out.PetIDs = append([]int32(nil), src.PetIDs...)
	out.SouvenirRewardIDs = append([]int32(nil), src.SouvenirRewardIDs...)
	if src.ZooDecorateMap != nil {
		out.ZooDecorateMap = make(map[int32][]int32, len(src.ZooDecorateMap))
		for k, v := range src.ZooDecorateMap {
			out.ZooDecorateMap[k] = append([]int32(nil), v...)
		}
	}
	return out
}

func cloneZooPetView(src ZooPetView) ZooPetView {
	out := src
	out.FoodstuffIDs = append([]int32(nil), src.FoodstuffIDs...)
	out.SpecialEventIDs = append([]int32(nil), src.SpecialEventIDs...)
	if src.Ext != nil {
		out.Ext = append(json.RawMessage(nil), src.Ext...)
	}
	if src.EventTriggerTimes != nil {
		out.EventTriggerTimes = make(map[int32]int64, len(src.EventTriggerTimes))
		for id, t := range src.EventTriggerTimes {
			out.EventTriggerTimes[id] = t
		}
	}
	return out
}

func cloneZooLogExtView(src ZooLogExtView) ZooLogExtView {
	out := src
	out.Consume = cloneInt32Map(src.Consume)
	out.Consume2 = cloneInt32Map(src.Consume2)
	return out
}

func cloneZooLogView(src ZooLogView) ZooLogView {
	out := src
	out.Gain = cloneInt32Map(src.Gain)
	out.Consume = cloneInt32Map(src.Consume)
	out.Souvenir = cloneInt32Map(src.Souvenir)
	out.Ext = cloneZooLogExtView(src.Ext)
	return out
}

func cloneZooSouvenirView(src ZooSouvenirView) ZooSouvenirView {
	return src
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

// ZooLogs returns a defensive copy of the namespace 33.2 log map.
func (s *State) ZooLogs() map[string]ZooLogView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]ZooLogView, len(s.zooLogs))
	for key, log := range s.zooLogs {
		if log != nil {
			out[key] = cloneZooLogView(*log)
		}
	}
	return out
}

// ZooLogsObserved reports whether namespace 33.2 has been observed as an object.
func (s *State) ZooLogsObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zooLogsObserved
}

// ZooLogsUnavailableReason describes an observed collection-level parse
// failure. An empty result means namespace 33.2 has simply not been observed.
func (s *State) ZooLogsUnavailableReason() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zooLogsInvalidReason
}

// ZooSouvenirs returns a defensive copy of namespace 33.4.
func (s *State) ZooSouvenirs() map[int32]ZooSouvenirView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]ZooSouvenirView, len(s.zooSouvenirs))
	for tempID, souvenir := range s.zooSouvenirs {
		if souvenir != nil {
			out[tempID] = cloneZooSouvenirView(*souvenir)
		}
	}
	return out
}

// ZooSouvenirsObserved reports whether namespace 33.4 has been observed as a
// valid object. A sparse object merges entries; a null entry deletes one.
func (s *State) ZooSouvenirsObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zooSouvenirsObserved
}

// ZooDecorates returns a defensive copy of namespace 33.5 decorate map.
func (s *State) ZooDecorates() map[int32]ZooDecorateView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]ZooDecorateView, len(s.zooDecorates))
	for id, dec := range s.zooDecorates {
		if dec != nil {
			out[id] = cloneZooDecorateView(*dec)
		}
	}
	return out
}

// ZooDecoratesObserved reports whether namespace 33.5 has been observed.
func (s *State) ZooDecoratesObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zooDecoratesObserved
}

// ZooDecorateSuits returns a defensive copy of namespace 33.6 decorate suit map.
func (s *State) ZooDecorateSuits() map[int32]ZooDecorateSuitView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]ZooDecorateSuitView, len(s.zooDecorateSuits))
	for id, suit := range s.zooDecorateSuits {
		if suit != nil {
			out[id] = cloneZooDecorateSuitView(*suit)
		}
	}
	return out
}

// ZooDecorateSuitsObserved reports whether namespace 33.6 has been observed.
func (s *State) ZooDecorateSuitsObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zooDecorateSuitsObserved
}

// ZooSouvenirCount is the client's collection progress: the number of
// distinct current namespace 33.4 map entries, independent of read status.
func (s *State) ZooSouvenirCount() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.zooSouvenirsObserved {
		return 0
	}
	return int32(len(s.zooSouvenirs))
}

// UnreadZooSouvenirIDs returns only explicitly observed unread entries.
func (s *State) UnreadZooSouvenirIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.unreadZooSouvenirIDsLocked()
}

func (s *State) unreadZooSouvenirIDsLocked() []int32 {
	if !s.zooSouvenirsObserved {
		return nil
	}
	out := make([]int32, 0, len(s.zooSouvenirs))
	for tempID, souvenir := range s.zooSouvenirs {
		if souvenir == nil || tempID <= 0 || !souvenir.IsReadObserved || souvenir.IsRead != 0 {
			continue
		}
		out = append(out, tempID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyZooSouvenirRewardIDs returns every achieved, unclaimed static
// collection milestone. Both the souvenir map and claimed-index list must be
// explicitly observed before an RPC is considered safe.
func (s *State) ReadyZooSouvenirRewardIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readyZooSouvenirRewardIDsLocked()
}

func (s *State) readyZooSouvenirRewardIDsLocked() []int32 {
	if !s.zooSouvenirsObserved || !s.zoo.SouvenirRewardIDsObserved {
		return nil
	}
	claimed := make(map[int32]struct{}, len(s.zoo.SouvenirRewardIDs))
	for _, index := range s.zoo.SouvenirRewardIDs {
		claimed[index] = struct{}{}
	}
	count := int32(len(s.zooSouvenirs))
	var out []int32
	for _, milestone := range ZooSouvenirCollectMilestones() {
		if milestone.Index <= 0 || milestone.Required <= 0 || count < milestone.Required {
			continue
		}
		if _, exists := claimed[milestone.Index]; exists {
			continue
		}
		out = append(out, milestone.Index)
	}
	return out
}

// ZooSouvenirRewardsReady validates that every requested reward is still
// currently claimable immediately before the RPC.
func (s *State) ZooSouvenirRewardsReady(indices []int32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return positiveIDSubset(indices, s.readyZooSouvenirRewardIDsLocked())
}

// ZooSouvenirRewardsClaimed is the recvSouvenirRwd postcondition.
func (s *State) ZooSouvenirRewardsClaimed(indices []int32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.zoo.SouvenirRewardIDsObserved || len(indices) == 0 {
		return false
	}
	return positiveIDSubset(indices, s.zoo.SouvenirRewardIDs)
}

// ZooSouvenirsUnread validates that every requested souvenir is still
// explicitly unread immediately before readSouvenir.
func (s *State) ZooSouvenirsUnread(ids []int32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return positiveIDSubset(ids, s.unreadZooSouvenirIDsLocked())
}

// ZooSouvenirsReadyToAcknowledge closes the planning/execution race: an
// unread batch may only be acknowledged while both source fields remain
// observed and no collection reward has become ready in the meantime.
func (s *State) ZooSouvenirsReadyToAcknowledge(ids []int32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.zooSouvenirsObserved || !s.zoo.SouvenirRewardIDsObserved || len(s.readyZooSouvenirRewardIDsLocked()) != 0 {
		return false
	}
	return positiveIDSubset(ids, s.unreadZooSouvenirIDsLocked())
}

// ZooSouvenirsAcknowledged is the readSouvenir postcondition. Explicit map
// deletion also clears the unread entry, matching the client's updateMbMap.
func (s *State) ZooSouvenirsAcknowledged(ids []int32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.zooSouvenirsObserved || len(ids) == 0 {
		return false
	}
	for _, id := range ids {
		if id <= 0 {
			return false
		}
		souvenir := s.zooSouvenirs[id]
		if souvenir != nil && (!souvenir.IsReadObserved || souvenir.IsRead == 0) {
			return false
		}
	}
	return true
}

func positiveIDSubset(requested, available []int32) bool {
	if len(requested) == 0 {
		return false
	}
	set := make(map[int32]struct{}, len(available))
	for _, id := range available {
		if id > 0 {
			set[id] = struct{}{}
		}
	}
	for _, id := range requested {
		if id <= 0 {
			return false
		}
		if _, ok := set[id]; !ok {
			return false
		}
	}
	return true
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

// NextZooFoodBowlNeed returns the first deterministic observed bowl deficit.
// Stocking a bowl is independent of whether the pet is currently eating or
// already satiated, matching ZooFoodPanel in the mini client.
func (s *State) NextZooFoodBowlNeed() (ZooFoodBowlNeed, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	capacity := ZooFoodBowlCapacity()
	if capacity <= 0 {
		return ZooFoodBowlNeed{}, false
	}
	petIDs := make([]int32, 0, len(s.zooPets))
	for petID, pet := range s.zooPets {
		if pet == nil || pet.PetID <= 0 || !pet.FoodstuffObserved {
			continue
		}
		petIDs = append(petIDs, petID)
	}
	sort.Slice(petIDs, func(i, j int) bool { return petIDs[i] < petIDs[j] })

	for _, petID := range petIDs {
		pet := s.zooPets[petID]
		empty := capacity - int32(len(pet.FoodstuffIDs))
		if empty > 0 {
			return ZooFoodBowlNeed{PetID: petID, Count: empty}, true
		}
	}
	return ZooFoodBowlNeed{}, false
}

// ZooFoodBowlsObserved reports whether every known pet has an authoritative
// foodstuffArr. It distinguishes "all bowls full" from sparse state.
func (s *State) ZooFoodBowlsObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	known := false
	for _, pet := range s.zooPets {
		if pet == nil || pet.PetID <= 0 {
			continue
		}
		known = true
		if !pet.FoodstuffObserved {
			return false
		}
	}
	return known
}

// NextZooFoodstuffPlan returns the first deterministic, inventory-backed bowl
// stocking action. Food 1501 has priority over 1502 and a request never mixes
// food types.
func (s *State) NextZooFoodstuffPlan() (ZooFoodstuffPlan, bool) {
	need, ok := s.NextZooFoodBowlNeed()
	if !ok {
		return ZooFoodstuffPlan{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, foodstuffID := range []int32{1501, 1502} {
		count := s.inventory[foodstuffID]
		if count <= 0 {
			continue
		}
		if count > need.Count {
			count = need.Count
		}
		return ZooFoodstuffPlan{PetID: need.PetID, FoodstuffID: foodstuffID, Count: count}, true
	}
	return ZooFoodstuffPlan{}, false
}

// ZooStrokeWaitReason explains why no pet currently matches the client's
// touch red-dot gate. It is presentation-safe planner evidence, not a new
// eligibility rule.
func (s *State) ZooStrokeWaitReason(now time.Time) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.zooPets) == 0 {
		return "尚未观测到可互动宠物"
	}
	moodMax := ZooMoodMax()
	nowMs := now.UnixMilli()
	var missing, notTouchable, moodFull, cooling int
	for _, pet := range s.zooPets {
		if pet == nil || pet.PetID <= 0 {
			continue
		}
		if !pet.StatusObserved || !pet.MoodObserved || !pet.StrokeCdTimeObserved || pet.Status <= 0 {
			missing++
			continue
		}
		if !zooPetTouchable(pet.Status) {
			notTouchable++
			continue
		}
		if moodMax > 0 && pet.MoodValue >= moodMax {
			moodFull++
			continue
		}
		if pet.StrokeCdTimeMs > 0 && nowMs < pet.StrokeCdTimeMs {
			cooling++
			continue
		}
		return ""
	}
	switch {
	case missing > 0:
		return "宠物互动所需的状态、心情或冷却字段尚未完整同步"
	case cooling > 0:
		return "宠物互动仍在冷却中"
	case moodFull > 0:
		return "宠物心情已满，无需互动"
	case notTouchable > 0:
		return "宠物当前状态不允许互动"
	default:
		return "当前没有达到互动条件的宠物"
	}
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

// ZooEventActions derives conservative actions exclusively from namespace
// 33.2 server logs. Completed unread logs are coalesced to one read per pet.
func (s *State) ZooEventActions() []ZooEventAction {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.zooLogsObserved && s.zooLogsInvalidReason != "" {
		return []ZooEventAction{{
			Name:          "宠物日志同步异常",
			Action:        "sync_logs",
			Blocked:       true,
			BlockedReason: s.zooLogsUnavailableReasonLocked(),
		}}
	}

	var actions []ZooEventAction
	readByPet := make(map[int32]ZooEventAction)
	identityCounts := make(map[[2]int32]int)
	for _, log := range s.zooLogs {
		if log == nil {
			continue
		}
		if petID, reason := validatedZooLogIdentity(*log); reason == "" {
			identityCounts[[2]int32{petID, log.Index}]++
		}
	}
	duplicateReported := make(map[[2]int32]bool)
	for _, log := range s.zooLogs {
		if log == nil {
			continue
		}
		if petID, reason := validatedZooLogIdentity(*log); reason == "" && identityCounts[[2]int32{petID, log.Index}] > 1 {
			identity := [2]int32{petID, log.Index}
			if !duplicateReported[identity] {
				action := "handle_event"
				if log.ProTypeObserved && log.ProType != 0 {
					action = "read_log"
				}
				actions = append(actions, blockedZooLogAction(*log, action, "宠物日志身份重复，拒绝自动处理"))
				duplicateReported[identity] = true
			}
			continue
		}
		if log.Malformed {
			reason := log.MalformedReason
			if reason == "" {
				reason = "宠物日志状态不可信，拒绝自动处理"
			}
			action := "handle_event"
			if log.ProTypeObserved && log.ProType != 0 {
				action = "read_log"
			}
			actions = append(actions, blockedZooLogAction(*log, action, reason))
			continue
		}
		if !log.ProTypeObserved {
			actions = append(actions, blockedZooLogAction(*log, "handle_event", "宠物日志缺少处理状态，无法判断是否待处理"))
			continue
		}
		if log.ProType == 0 {
			actions = append(actions, zooActiveLogAction(*log))
			continue
		}

		if !log.CreatedAtObserved || log.CreatedAtMs <= 0 {
			actions = append(actions, blockedZooLogAction(*log, "read_log", "已完成宠物日志缺少创建时间，无法判断是否未读"))
			continue
		}
		petID, reason := validatedZooLogIdentity(*log)
		if reason != "" {
			actions = append(actions, blockedZooLogAction(*log, "read_log", reason))
			continue
		}
		pet := s.zooPets[petID]
		if pet == nil || !pet.ReadLogTimeObserved {
			actions = append(actions, blockedZooLogAction(*log, "read_log", "宠物已读日志时间未观测，拒绝把历史日志推断为未读"))
			continue
		}
		if log.CreatedAtMs <= pet.ReadLogTimeMs {
			continue
		}
		action := ZooEventAction{
			PetID:       petID,
			EventID:     log.GoOutEventID,
			TableID:     log.Index,
			CreatedAtMs: log.CreatedAtMs,
			Name:        zooEventName(log.GoOutEventID),
			Action:      "read_log",
		}
		if current, ok := readByPet[petID]; !ok || zooEventActionNewer(action, current) {
			readByPet[petID] = action
		}
	}
	for _, action := range readByPet {
		actions = append(actions, action)
	}
	sort.Slice(actions, func(i, j int) bool {
		leftRank, rightRank := zooEventActionRank(actions[i]), zooEventActionRank(actions[j])
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if actions[i].CreatedAtMs != actions[j].CreatedAtMs {
			return actions[i].CreatedAtMs > actions[j].CreatedAtMs
		}
		if actions[i].PetID != actions[j].PetID {
			return actions[i].PetID < actions[j].PetID
		}
		return actions[i].TableID < actions[j].TableID
	})
	return actions
}

// ZooHandleEventAction re-evaluates one exact log against the current state.
// Runners use it immediately before the RPC to close the planning/execution gap.
func (s *State) ZooHandleEventAction(petID, index int32) (ZooEventAction, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	log, matches := s.zooLogByIdentityLocked(petID, index)
	if !s.zooLogsObserved {
		if log == nil {
			log = &ZooLogView{PetID: petID, PetIDObserved: true, Index: index, IndexObserved: true}
		}
		return blockedZooLogAction(*log, "handle_event", s.zooLogsUnavailableReasonLocked()), true
	}
	if log == nil {
		return ZooEventAction{}, false
	}
	if matches > 1 {
		return blockedZooLogAction(*log, "handle_event", "宠物日志身份重复，拒绝自动处理"), true
	}
	if log.Malformed {
		reason := log.MalformedReason
		if reason == "" {
			reason = "宠物日志状态不可信"
		}
		return blockedZooLogAction(*log, "handle_event", reason), true
	}
	if !log.ProTypeObserved || log.ProType != 0 {
		return blockedZooLogAction(*log, "handle_event", "宠物日志当前已不再是待处理状态"), true
	}
	return zooActiveLogAction(*log), true
}

// ZooLogHandled reports the required handleEvent postcondition: the exact log
// was removed, or the server explicitly changed proType away from zero.
func (s *State) ZooLogHandled(petID, index int32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.zooLogsObserved {
		return false
	}
	log, matches := s.zooLogByIdentityLocked(petID, index)
	if matches > 1 {
		return false
	}
	return log == nil || !log.Malformed && log.ProTypeObserved && log.ProType != 0
}

// ZooReadLogAction re-evaluates one completed unread log immediately before a
// standalone readLog RPC.
func (s *State) ZooReadLogAction(petID, index int32) (ZooEventAction, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	log, matches := s.zooLogByIdentityLocked(petID, index)
	if !s.zooLogsObserved {
		if log == nil {
			log = &ZooLogView{PetID: petID, PetIDObserved: true, Index: index, IndexObserved: true}
		}
		return blockedZooLogAction(*log, "read_log", s.zooLogsUnavailableReasonLocked()), true
	}
	if log == nil {
		return ZooEventAction{}, false
	}
	blocked := func(reason string) (ZooEventAction, bool) {
		return blockedZooLogAction(*log, "read_log", reason), true
	}
	if matches > 1 {
		return blocked("宠物日志身份重复，拒绝自动处理")
	}
	if log.Malformed {
		reason := log.MalformedReason
		if reason == "" {
			reason = "宠物日志状态不可信"
		}
		return blocked(reason)
	}
	if !log.ProTypeObserved || log.ProType == 0 {
		return blocked("宠物日志当前不是已完成状态")
	}
	if !log.CreatedAtObserved || log.CreatedAtMs <= 0 {
		return blocked("已完成宠物日志缺少创建时间")
	}
	validatedPetID, reason := validatedZooLogIdentity(*log)
	if reason != "" || validatedPetID != petID {
		if reason == "" {
			reason = "宠物日志身份与请求不一致"
		}
		return blocked(reason)
	}
	pet := s.zooPets[petID]
	if pet == nil || !pet.ReadLogTimeObserved {
		return blocked("宠物已读日志时间未观测")
	}
	if log.CreatedAtMs <= pet.ReadLogTimeMs {
		return blocked("宠物日志当前已不是未读状态")
	}
	return ZooEventAction{
		PetID:       petID,
		EventID:     log.GoOutEventID,
		TableID:     index,
		CreatedAtMs: log.CreatedAtMs,
		Name:        zooEventName(log.GoOutEventID),
		Action:      "read_log",
	}, true
}

// ZooLogRead reports the standalone readLog postcondition for the exact log.
func (s *State) ZooLogRead(petID, index int32, capturedCreatedAtMs int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.zooLogsObserved {
		return false
	}
	log, matches := s.zooLogByIdentityLocked(petID, index)
	if matches > 1 {
		return false
	}
	if log == nil {
		return true
	}
	if log.Malformed {
		return false
	}
	pet := s.zooPets[petID]
	return capturedCreatedAtMs > 0 && pet != nil && pet.ReadLogTimeObserved && pet.ReadLogTimeMs >= capturedCreatedAtMs
}

func zooActiveLogAction(log ZooLogView) ZooEventAction {
	action := ZooEventAction{
		PetID:       log.PetID,
		EventID:     log.GoOutEventID,
		TableID:     log.Index,
		CreatedAtMs: log.CreatedAtMs,
		Name:        zooEventName(log.GoOutEventID),
		Action:      "handle_event",
		Agree:       true,
	}
	var reasons []string
	if _, reason := validatedZooLogIdentity(log); reason != "" {
		reasons = append(reasons, reason)
	}
	if !log.GoOutEventIDObserved || log.GoOutEventID <= 0 {
		reasons = append(reasons, "宠物日志缺少事件配置 ID")
	}
	if !log.EventTypeObserved || log.EventType <= 0 {
		reasons = append(reasons, "宠物日志缺少事件类型")
	}
	if !log.CreatedAtObserved || log.CreatedAtMs <= 0 {
		reasons = append(reasons, "宠物日志缺少创建时间")
	}
	if !log.GainObserved {
		reasons = append(reasons, "宠物日志收益字段未完整观测")
	} else if len(log.Gain) != 0 {
		reasons = append(reasons, "宠物日志已包含收益结果，只允许确认已读")
	}
	if !log.ConsumeObserved {
		reasons = append(reasons, "宠物日志消耗字段未观测")
	} else if len(log.Consume) != 0 {
		reasons = append(reasons, "宠物事件存在物品或货币消耗")
	}
	if !log.ExtObserved || !log.Ext.ConsumeObserved || !log.Ext.Consume2Observed {
		reasons = append(reasons, "宠物日志扩展消耗字段未完整观测")
	} else if len(log.Ext.Consume) != 0 || len(log.Ext.Consume2) != 0 {
		reasons = append(reasons, "宠物事件扩展结果包含消耗")
	}
	if !log.SouvenirObserved {
		reasons = append(reasons, "宠物日志纪念品字段未完整观测")
	} else if len(log.Souvenir) != 0 {
		reasons = append(reasons, "宠物日志已包含纪念品结果，只允许确认已读")
	}
	info, ok := ZooEventInfoByID(log.GoOutEventID)
	if !ok {
		reasons = append(reasons, "宠物事件静态配置不存在")
	} else {
		action.Name = info.Name
		if log.EventTypeObserved && log.EventType > 0 && log.EventType != info.Type {
			reasons = append(reasons, "宠物日志事件类型与静态配置不一致")
		}
		// The observed client only calls handleEvent from its item-choice branch:
		// eventType is not 2, static mood/satiety handling is absent, gain is
		// empty, and ext.consume supplies the first choice. Type-2 rewards and
		// mood/satiety changes go directly to readLog instead. We still reject
		// every ext.consume entry below as a cost, so capture/catalog data do not
		// currently produce an executable handleEvent candidate.
		if !zooClientHandleBranch(log, info) {
			reasons = append(reasons, "宠物日志形状不符合已观测客户端 handleEvent 分支")
		}
		if info.SharedID != 0 || info.Code != "" {
			reasons = append(reasons, "宠物事件关联分享、视频或特殊客户端流程")
		}
		if info.HasReward2 || len(info.Reward2) > 0 || info.NoHandle || info.Result || strings.Contains(info.Text, "|") {
			reasons = append(reasons, "宠物事件存在二选一、稍后处理或多结果分支")
		}
		resultCount := 0
		for _, present := range []bool{info.HasReward1, info.MoodChange != 0, info.SatietyChange != 0, info.SouvenirID != 0} {
			if present {
				resultCount++
			}
		}
		if resultCount != 1 {
			reasons = append(reasons, "宠物事件结果类别不唯一")
		}
	}
	if len(reasons) > 0 {
		action.Blocked = true
		action.Agree = false
		action.BlockedReason = strings.Join(reasons, "；")
	}
	return action
}

func zooClientHandleBranch(log ZooLogView, info ZooEventInfo) bool {
	return log.EventTypeObserved && log.EventType > 0 && log.EventType != 2 &&
		info.MoodChange == 0 && info.SatietyChange == 0 &&
		log.GainObserved && len(log.Gain) == 0 &&
		log.ExtObserved && log.Ext.ConsumeObserved && len(log.Ext.Consume) > 0
}

func validatedZooLogIdentity(log ZooLogView) (int32, string) {
	if !log.PetIDObserved || log.PetID <= 0 {
		return 0, "宠物日志缺少宠物 ID"
	}
	if !log.IndexObserved || log.Index <= 0 {
		return 0, "宠物日志缺少日志序号"
	}
	return log.PetID, ""
}

func blockedZooLogAction(log ZooLogView, action, reason string) ZooEventAction {
	return ZooEventAction{
		PetID:         log.PetID,
		EventID:       log.GoOutEventID,
		TableID:       log.Index,
		CreatedAtMs:   log.CreatedAtMs,
		Name:          zooEventName(log.GoOutEventID),
		Action:        action,
		Blocked:       true,
		BlockedReason: reason,
	}
}

func (s *State) zooLogByIdentityLocked(petID, index int32) (*ZooLogView, int) {
	var found *ZooLogView
	matches := 0
	for _, log := range s.zooLogs {
		if log == nil || !log.PetIDObserved || !log.IndexObserved || log.PetID != petID || log.Index != index {
			continue
		}
		matches++
		if found == nil {
			found = log
		}
	}
	return found, matches
}

func (s *State) zooLogsUnavailableReasonLocked() string {
	if s.zooLogsInvalidReason != "" {
		return s.zooLogsInvalidReason
	}
	return "宠物服务端日志尚未同步，不使用宠物字段猜测事件"
}

func zooEventName(eventID int32) string {
	if info, ok := ZooEventInfoByID(eventID); ok {
		return info.Name
	}
	return ""
}

func zooEventActionNewer(left, right ZooEventAction) bool {
	return left.CreatedAtMs > right.CreatedAtMs || left.CreatedAtMs == right.CreatedAtMs && left.TableID > right.TableID
}

func zooEventActionRank(action ZooEventAction) int {
	if action.Blocked {
		return 2
	}
	if action.Action == "handle_event" {
		return 0
	}
	return 1
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

func (s *State) applyZooDecorateMapLocked(raw json.RawMessage) {
	if !isZooLogJSONObject(raw) {
		s.zooDecoratesObserved = false
		s.zooDecorates = make(map[int32]*ZooDecorateView)
		return
	}
	var decMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decMap); err != nil {
		s.zooDecoratesObserved = false
		s.zooDecorates = make(map[int32]*ZooDecorateView)
		return
	}
	s.zooDecoratesObserved = true
	if s.zooDecorates == nil {
		s.zooDecorates = make(map[int32]*ZooDecorateView)
	}
	for rawTempID, rawDec := range decMap {
		tempID := atoi32(rawTempID)
		if tempID <= 0 {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(rawDec), []byte("null")) {
			delete(s.zooDecorates, tempID)
			continue
		}
		base := ZooDecorateView{MapTempID: tempID, TempID: tempID}
		if old := s.zooDecorates[tempID]; old != nil {
			base = *old
		}
		dec, ok := parseZooDecorateView(rawDec, base)
		if !ok {
			continue
		}
		cp := dec
		s.zooDecorates[tempID] = &cp
	}
}

func parseZooDecorateView(raw json.RawMessage, base ZooDecorateView) (ZooDecorateView, bool) {
	if !isZooLogJSONObject(raw) {
		return ZooDecorateView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ZooDecorateView{}, false
	}
	dec := base
	if rawUID, present := fields["0"]; present {
		dec.UID = 0
		dec.UIDObserved = false
		if n, ok := readZooLogInt64Raw(bytes.TrimSpace(rawUID)); ok {
			dec.UID = n
			dec.UIDObserved = true
		}
	}
	if rawTempID, present := fields["1"]; present {
		dec.TempID = dec.MapTempID
		dec.TempIDObserved = false
		if n, ok := readZooLogInt32Raw(bytes.TrimSpace(rawTempID)); ok && n > 0 {
			dec.TempID = n
			dec.TempIDObserved = true
		}
	}
	if rawIsRead, present := fields["2"]; present {
		dec.IsRead = 0
		dec.IsReadObserved = false
		if trimmed := bytes.TrimSpace(rawIsRead); len(trimmed) > 0 && string(trimmed) != "null" {
			if n, ok := readZooLogInt32Raw(trimmed); ok {
				dec.IsRead = n
				dec.IsReadObserved = true
			}
		}
	}
	if rawComfort, present := fields["3"]; present {
		dec.Comfort = 0
		dec.ComfortObserved = false
		if trimmed := bytes.TrimSpace(rawComfort); len(trimmed) > 0 && string(trimmed) != "null" {
			if n, ok := readZooLogInt32Raw(trimmed); ok {
				dec.Comfort = n
				dec.ComfortObserved = true
			}
		}
	}
	if rawUpdatedAt, present := fields["4"]; present {
		dec.UpdatedAtMs = 0
		dec.UpdatedAtObserved = false
		if n, ok := readZooLogInt64Raw(bytes.TrimSpace(rawUpdatedAt)); ok {
			dec.UpdatedAtMs = n
			dec.UpdatedAtObserved = true
		}
	}
	if rawCreatedAt, present := fields["5"]; present {
		dec.CreatedAtMs = 0
		dec.CreatedAtObserved = false
		if n, ok := readZooLogInt64Raw(bytes.TrimSpace(rawCreatedAt)); ok {
			dec.CreatedAtMs = n
			dec.CreatedAtObserved = true
		}
	}
	if dec.TempIDObserved && dec.TempID != dec.MapTempID {
		dec.TempID = dec.MapTempID
		dec.TempIDObserved = false
	}
	return dec, true
}

func (s *State) applyZooDecorateSuitMapLocked(raw json.RawMessage) {
	if !isZooLogJSONObject(raw) {
		s.zooDecorateSuitsObserved = false
		s.zooDecorateSuits = make(map[int32]*ZooDecorateSuitView)
		return
	}
	var suitMap map[string]json.RawMessage
	if err := json.Unmarshal(raw, &suitMap); err != nil {
		s.zooDecorateSuitsObserved = false
		s.zooDecorateSuits = make(map[int32]*ZooDecorateSuitView)
		return
	}
	s.zooDecorateSuitsObserved = true
	if s.zooDecorateSuits == nil {
		s.zooDecorateSuits = make(map[int32]*ZooDecorateSuitView)
	}
	for rawTempID, rawSuit := range suitMap {
		tempID := atoi32(rawTempID)
		if tempID <= 0 {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(rawSuit), []byte("null")) {
			delete(s.zooDecorateSuits, tempID)
			continue
		}
		base := ZooDecorateSuitView{MapTempID: tempID, TempID: tempID}
		if old := s.zooDecorateSuits[tempID]; old != nil {
			base = *old
		}
		suit, ok := parseZooDecorateSuitView(rawSuit, base)
		if !ok {
			continue
		}
		cp := suit
		s.zooDecorateSuits[tempID] = &cp
	}
}

func parseZooDecorateSuitView(raw json.RawMessage, base ZooDecorateSuitView) (ZooDecorateSuitView, bool) {
	if !isZooLogJSONObject(raw) {
		return ZooDecorateSuitView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ZooDecorateSuitView{}, false
	}
	suit := base
	if rawUID, present := fields["0"]; present {
		suit.UID = 0
		suit.UIDObserved = false
		if n, ok := readZooLogInt64Raw(bytes.TrimSpace(rawUID)); ok {
			suit.UID = n
			suit.UIDObserved = true
		}
	}
	if rawTempID, present := fields["1"]; present {
		suit.TempID = suit.MapTempID
		suit.TempIDObserved = false
		if n, ok := readZooLogInt32Raw(bytes.TrimSpace(rawTempID)); ok && n > 0 {
			suit.TempID = n
			suit.TempIDObserved = true
		}
	}
	if rawActCount, present := fields["2"]; present {
		suit.ActCount = 0
		suit.ActCountObserved = false
		if trimmed := bytes.TrimSpace(rawActCount); len(trimmed) > 0 && string(trimmed) != "null" {
			if n, ok := readZooLogInt32Raw(trimmed); ok {
				suit.ActCount = n
				suit.ActCountObserved = true
			}
		}
	}
	if rawUpdatedAt, present := fields["3"]; present {
		suit.UpdatedAtMs = 0
		suit.UpdatedAtObserved = false
		if n, ok := readZooLogInt64Raw(bytes.TrimSpace(rawUpdatedAt)); ok {
			suit.UpdatedAtMs = n
			suit.UpdatedAtObserved = true
		}
	}
	if rawCreatedAt, present := fields["4"]; present {
		suit.CreatedAtMs = 0
		suit.CreatedAtObserved = false
		if n, ok := readZooLogInt64Raw(bytes.TrimSpace(rawCreatedAt)); ok {
			suit.CreatedAtMs = n
			suit.CreatedAtObserved = true
		}
	}
	if suit.TempIDObserved && suit.TempID != suit.MapTempID {
		suit.TempID = suit.MapTempID
		suit.TempIDObserved = false
	}
	return suit, true
}

func cloneZooDecorateView(src ZooDecorateView) ZooDecorateView {
	return src
}

func cloneZooDecorateSuitView(src ZooDecorateSuitView) ZooDecorateSuitView {
	return src
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
