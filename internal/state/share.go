package state

import (
	"encoding/json"
	"strconv"
	"time"
)

// applyShareTotLocked merges namespace 31 (IShareTot.map). Share responses are
// sparse, so omitted IDs and fields preserve their last authoritative values.
func (s *State) applyShareTotLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return
	}
	rawMap, ok := fields["0"]
	if !ok {
		return
	}
	if s.shareUsages == nil {
		s.shareUsages = make(map[int32]ShareUsageView)
	}
	if isJSONNull(rawMap) {
		clear(s.shareUsages)
		s.shareTotObserved = true
		return
	}
	var rows map[string]json.RawMessage
	if json.Unmarshal(rawMap, &rows) != nil {
		return
	}
	for idText, rawUsage := range rows {
		parsed, err := strconv.ParseInt(idText, 10, 32)
		if err != nil || parsed <= 0 {
			continue
		}
		shareID := int32(parsed)
		if isJSONNull(rawUsage) {
			delete(s.shareUsages, shareID)
			continue
		}
		var usageFields map[string]json.RawMessage
		if json.Unmarshal(rawUsage, &usageFields) != nil {
			continue
		}
		view := s.shareUsages[shareID]
		view.Observed = true
		view.ShareID = shareID
		if n, valid := readInt64JSONField(usageFields, "0"); valid {
			view.UID = n
		}
		if n, valid := readInt32JSONField(usageFields, "1"); valid && n > 0 {
			view.ShareID = n
		}
		if n, valid := readInt32JSONField(usageFields, "2"); valid {
			view.ShareCount = n
		}
		if n, valid := readInt32JSONField(usageFields, "3"); valid {
			view.ReceiveCount = n
		}
		if n, valid := readInt64JSONField(usageFields, "4"); valid {
			view.ReceiveTimeMs = n
		}
		if n, valid := readInt32JSONField(usageFields, "5"); valid {
			view.TotalCount = n
		}
		if n, valid := readInt64JSONField(usageFields, "6"); valid {
			view.UpdatedAtMs = n
		}
		if n, valid := readInt64JSONField(usageFields, "7"); valid {
			view.CreatedAtMs = n
		}
		s.shareUsages[shareID] = view
	}
	s.shareTotObserved = true
}

// ShareUsageAt returns one normalized c_share usage counter. An observed total
// map with no row means zero usage, not unknown. Daily rows reset at calendar
// midnight in the game timezone, matching ShareMgr.getMb in the mini client.
func (s *State) ShareUsageAt(shareID int32, now time.Time) (ShareUsageView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.shareTotObserved || shareID <= 0 {
		return ShareUsageView{}, false
	}
	view, exists := s.shareUsages[shareID]
	if !exists {
		view = ShareUsageView{Observed: true, ShareID: shareID}
	}
	if cfg, ok := ShareRewardConfigByID(shareID); ok && cfg.LimitType == 1 {
		stamp := view.UpdatedAtMs
		if stamp <= 0 {
			stamp = view.CreatedAtMs
		}
		if stamp > 0 && calendarDayID(time.UnixMilli(stamp)) < calendarDayID(now) {
			view.ShareCount = 0
			view.ReceiveCount = 0
			view.ReceiveTimeMs = 0
		}
	}
	return view, true
}

// ShareTotObserved reports whether namespace 31 supplied its usage map.
func (s *State) ShareTotObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shareTotObserved
}
