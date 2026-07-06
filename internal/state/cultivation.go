package state

import "encoding/json"

func (s *State) applyCultivationsLocked(raw json.RawMessage) {
	var ns101 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns101); err != nil {
		return
	}
	raw0, ok := ns101["0"]
	if !ok {
		return
	}
	var flowers map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &flowers); err != nil {
		return
	}
	for fid, rawEntry := range flowers {
		id := atoi32(fid)
		if id == 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawEntry, &fields); err != nil {
			continue
		}
		cv := s.cultivations[id]
		if cv == nil {
			cv = &CultivateView{FlowerID: id}
			s.cultivations[id] = cv
		}
		if v, ok := fields["2"]; ok {
			var n int32
			if json.Unmarshal(v, &n) == nil {
				cv.Lvl = n
			}
		}
		if v, ok := fields["3"]; ok {
			var n int64
			if json.Unmarshal(v, &n) == nil {
				cv.CulTimeMs = n
			}
		}
		if v, ok := fields["4"]; ok {
			var n int32
			if json.Unmarshal(v, &n) == nil {
				cv.Status = n
			}
		}
		if v, ok := fields["5"]; ok {
			var n int64
			if json.Unmarshal(v, &n) == nil {
				cv.UTimeMs = n
			}
		}
	}
}

// Cultivations returns a copy of the cultivation state map.
func (s *State) Cultivations() map[int32]CultivateView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]CultivateView, len(s.cultivations))
	for k, v := range s.cultivations {
		out[k] = *v
	}
	return out
}
