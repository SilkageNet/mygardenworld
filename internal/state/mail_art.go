package state

import (
	"encoding/json"
	"sort"
	"strconv"
)

func (s *State) applyMailLocked(raw json.RawMessage) {
	var ns19 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns19); err != nil {
		return
	}
	s.mailObserved = true
	rawList, ok := ns19["1"]
	if !ok || len(rawList) == 0 || string(rawList) == "null" {
		return
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(rawList, &entries); err != nil {
		return
	}
	if s.mails == nil {
		s.mails = make(map[string]*MailView)
	}
	for _, rawEntry := range entries {
		mail, present, ok := parseMailView(rawEntry)
		if !ok {
			continue
		}
		key := mailKey(mail.MsID, mail.AllID)
		if key == "" {
			continue
		}
		if prev, exists := s.mails[key]; exists && prev != nil {
			mail = mergeMailView(*prev, mail, present)
		}
		next := mail
		s.mails[key] = &next
	}
}

type mailFieldsPresent struct {
	isDel  bool
	isRead bool
	isPick bool
	items  bool
}

func parseMailView(raw json.RawMessage) (MailView, mailFieldsPresent, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return MailView{}, mailFieldsPresent{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return MailView{}, mailFieldsPresent{}, false
	}
	view := MailView{}
	var present mailFieldsPresent
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.MsID = n
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.AllID = n
	}
	if n, ok := readInt32JSONField(fields, "17"); ok {
		view.IsDel = n
		present.isDel = true
	}
	if n, ok := readInt32JSONField(fields, "18"); ok {
		view.IsRead = n
		present.isRead = true
	}
	if n, ok := readInt32JSONField(fields, "20"); ok {
		view.IsPick = n
		present.isPick = true
	}
	if rawItems, ok := fields["13"]; ok {
		view.ItemsRaw = append(json.RawMessage(nil), rawItems...)
		present.items = true
	}
	return view, present, view.MsID > 0 || view.AllID > 0
}

// mergeMailView keeps previously observed fields when a sparse ns19 delta omits
// them. mail.pick often returns a partial row without isPick=1; replacing the
// whole entry would leave ReadyMailPickTargets retrying and hit「附件已领取」.
func mergeMailView(prev, incoming MailView, present mailFieldsPresent) MailView {
	out := prev
	if incoming.MsID > 0 {
		out.MsID = incoming.MsID
	}
	if incoming.AllID > 0 {
		out.AllID = incoming.AllID
	}
	if present.isDel {
		out.IsDel = incoming.IsDel
	}
	if present.isRead {
		out.IsRead = incoming.IsRead
	}
	if present.isPick {
		out.IsPick = incoming.IsPick
	}
	if present.items {
		out.ItemsRaw = append(json.RawMessage(nil), incoming.ItemsRaw...)
	}
	return out
}

// MarkMailPicked marks one mail as claimed so automation stops retrying pick
// when the server already applied the claim but local ns19 lagged.
func (s *State) MarkMailPicked(msID, allID int32) {
	key := mailKey(msID, allID)
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.mails == nil {
		return
	}
	mail, ok := s.mails[key]
	if !ok || mail == nil {
		return
	}
	mail.IsPick = 1
}

func mailKey(msID, allID int32) string {
	if msID <= 0 && allID <= 0 {
		return ""
	}
	return strconv.FormatInt(int64(msID), 10) + ":" + strconv.FormatInt(int64(allID), 10)
}

func mailHasItems(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		return len(arr) > 0
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		return len(obj) > 0
	}
	return true
}

func (s *State) applyVasesLocked(raw json.RawMessage) {
	var ns102 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns102); err != nil {
		return
	}
	raw0, ok := ns102["0"]
	if !ok {
		return
	}
	var vaseMap map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &vaseMap); err != nil {
		return
	}
	s.vaseObserved = true
	next := make(map[int32]*VaseView, len(vaseMap))
	for vaseIDStr, rawVase := range vaseMap {
		vaseID := atoi32(vaseIDStr)
		if vaseID <= 0 {
			continue
		}
		view := &VaseView{VaseID: vaseID}
		if len(rawVase) > 0 && string(rawVase) != "{}" {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(rawVase, &fields); err == nil {
				if rawID, ok := fields["1"]; ok {
					var n int32
					if json.Unmarshal(rawID, &n) == nil && n > 0 {
						view.VaseID = n
					}
				}
				if rawUTime, ok := fields["2"]; ok {
					_ = json.Unmarshal(rawUTime, &view.UTimeMs)
				}
				if rawCTime, ok := fields["3"]; ok {
					_ = json.Unmarshal(rawCTime, &view.CTimeMs)
				}
			}
		}
		next[view.VaseID] = view
	}
	s.vases = next
}

func (s *State) applyFlowerArtLocked(raw json.RawMessage) {
	var ns106 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns106); err != nil {
		return
	}
	raw0, ok := ns106["0"]
	if !ok {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &fields); err != nil {
		return
	}
	s.flowerArt.Observed = true
	if rawExp, ok := fields["1"]; ok {
		_ = json.Unmarshal(rawExp, &s.flowerArt.Exp)
	}
	if rawMakeList, ok := fields["2"]; ok {
		s.flowerArt.MakeListRaw = cloneRaw(rawMakeList)
		s.flowerArt.MakeList = readInt32ListRaw(rawMakeList)
	}
	if rawSRecvList, ok := fields["3"]; ok {
		s.flowerArt.SRecvListRaw = cloneRaw(rawSRecvList)
		s.flowerArt.SRecvList = readInt32ListRaw(rawSRecvList)
	}
	if rawUTime, ok := fields["4"]; ok {
		_ = json.Unmarshal(rawUTime, &s.flowerArt.UTimeMs)
	}
	if rawCTime, ok := fields["5"]; ok {
		_ = json.Unmarshal(rawCTime, &s.flowerArt.CTimeMs)
	}
}

// MailObserved reports whether namespace 19 has been observed at least once.
func (s *State) MailObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mailObserved
}

// Mails returns the currently tracked ordinary mail list.
func (s *State) Mails() []MailView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MailView, 0, len(s.mails))
	for _, mail := range s.mails {
		if mail == nil {
			continue
		}
		cp := *mail
		cp.ItemsRaw = append(json.RawMessage(nil), mail.ItemsRaw...)
		out = append(out, cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MsID != out[j].MsID {
			return out[i].MsID < out[j].MsID
		}
		return out[i].AllID < out[j].AllID
	})
	return out
}

// ReadyMailPickTargets returns unpicked mail entries that carry rewards.
func (s *State) ReadyMailPickTargets() []MailPickTarget {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MailPickTarget, 0)
	for _, mail := range s.mails {
		if mail == nil || mail.IsDel != 0 || mail.IsPick != 0 || !mailHasItems(mail.ItemsRaw) {
			continue
		}
		if mail.MsID <= 0 && mail.AllID <= 0 {
			continue
		}
		out = append(out, MailPickTarget{MsID: mail.MsID, AllID: mail.AllID})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MsID != out[j].MsID {
			return out[i].MsID < out[j].MsID
		}
		return out[i].AllID < out[j].AllID
	})
	return out
}

// Vases returns the currently observed unlocked vase set.
func (s *State) Vases() map[int32]VaseView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]VaseView, len(s.vases))
	for k, v := range s.vases {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}

// VaseObserved reports whether namespace 102 has been observed at least once.
func (s *State) VaseObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.vaseObserved
}

// HasVase reports whether the account has the requested vase unlocked.
func (s *State) HasVase(vaseID int32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.vases[vaseID]
	return ok
}

// FlowerArt returns the tracked namespace 106 flower-art state.
func (s *State) FlowerArt() FlowerArtView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.flowerArt
	out.MakeList = cloneInt32s(out.MakeList)
	out.MakeListRaw = cloneRaw(out.MakeListRaw)
	out.SRecvList = cloneInt32s(out.SRecvList)
	out.SRecvListRaw = cloneRaw(out.SRecvListRaw)
	return out
}
