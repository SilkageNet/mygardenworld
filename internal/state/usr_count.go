package state

import (
	"encoding/json"
	"time"
)

// applyUsrCountLocked merges the observed daily counters in namespace 7.4.
func (s *State) applyUsrCountLocked(ns7 map[string]json.RawMessage) {
	raw, ok := ns7["4"]
	if !ok {
		return
	}
	if isJSONNull(raw) {
		s.usrCount = nil
		return
	}
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil {
		return
	}
	if s.usrCount == nil {
		s.usrCount = make(map[int32]UsrCountView)
	}
	for key, entry := range entries {
		id := atoiCatalogID(key)
		if id <= 0 {
			continue
		}
		if isJSONNull(entry) {
			delete(s.usrCount, id)
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(entry, &fields) != nil {
			continue
		}
		view := s.usrCount[id]
		view.Type, view.Observed = id, true
		if n, ok := readExactInt32Raw(fields["2"]); ok {
			view.TdyCnt = n
		}
		if n, ok := readExactInt32Raw(fields["3"]); ok {
			view.TotCnt = n
		}
		if n, ok := readExactInt64Raw(fields["4"]); ok {
			view.RTimeMs = n
		}
		s.usrCount[id] = view
	}
}

func (s *State) getTdyCountLocked(countType int32, now time.Time) (int32, bool) {
	view, ok := s.usrCount[countType]
	if !ok || !view.Observed {
		return 0, false
	}
	if view.RTimeMs > 0 && !frdStealMapFresh(view.RTimeMs, now) {
		return 0, true
	}
	if view.TdyCnt < 0 {
		return 0, true
	}
	return view.TdyCnt, true
}
