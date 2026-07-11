package state

import (
	"encoding/json"
	"time"
)

func (s *State) applyUsrExtraLocked(ns7 map[string]json.RawMessage) {
	raw13, ok := ns7["13"]
	if !ok {
		return
	}
	var usrExtTot map[string]json.RawMessage
	if err := json.Unmarshal(raw13, &usrExtTot); err != nil {
		return
	}
	rawExtra, ok := usrExtTot["1"]
	if !ok {
		return
	}
	var extra map[string]json.RawMessage
	if err := json.Unmarshal(rawExtra, &extra); err != nil {
		return
	}
	s.usrExtra.Observed = true
	if rawStatus, ok := extra["104"]; ok {
		var n int32
		if json.Unmarshal(rawStatus, &n) == nil {
			s.usrExtra.AntiFraudQAStatus = n
		}
	}
	if rawTime, ok := extra["105"]; ok {
		var n int64
		if json.Unmarshal(rawTime, &n) == nil {
			s.usrExtra.LastAntiFraudQATimeMs = n
		}
	}
}

func (s *State) applyReputationLocked(ns7 map[string]json.RawMessage) {
	raw17, ok := ns7["17"]
	if !ok {
		return
	}
	var reputationTot map[string]json.RawMessage
	if err := json.Unmarshal(raw17, &reputationTot); err != nil {
		return
	}
	rawData, ok := reputationTot["0"]
	if !ok {
		return
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal(rawData, &data); err != nil {
		return
	}
	s.reputation.Observed = true
	if n, ok := readInt64JSONField(data, "0"); ok {
		s.reputation.UID = n
	}
	if n, ok := readInt32JSONField(data, "1"); ok {
		s.reputation.Score = n
	}
	if n, ok := readInt64JSONField(data, "3"); ok {
		s.reputation.LastSyncTimeMs = n
	}
	if n, ok := readInt64JSONField(data, "4"); ok {
		s.reputation.LastViewTimeMs = n
	}
	if n, ok := readInt64JSONField(data, "5"); ok {
		s.reputation.UTimeMs = n
	}
	if n, ok := readInt64JSONField(data, "6"); ok {
		s.reputation.CTimeMs = n
	}
}

func (s *State) applyStoryMainLocked(ns7 map[string]json.RawMessage) {
	rawStory, ok := ns7["101"]
	if !ok {
		return
	}
	if isJSONNull(rawStory) {
		s.invalidateStoryMainLocked()
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawStory, &fields); err != nil {
		s.invalidateStoryMainLocked()
		return
	}
	view := s.storyMain
	view.Observed = true
	previousChapter := view.Chapter
	_, sectionPresent := fields["2"]
	if rawUID, exists := fields["0"]; exists {
		if n, valid := readStoryMainInt64(rawUID); valid && n > 0 {
			view.UID = n
		}
	}
	if rawChapter, exists := fields["1"]; exists {
		view.ChapterObserved = false
		if n, valid := readStoryMainInt32(rawChapter); valid && n > 0 {
			view.Chapter = n
			view.ChapterObserved = true
			if n != previousChapter && !sectionPresent {
				view.SectionObserved = false
			}
		}
	}
	if rawSection, exists := fields["2"]; exists {
		view.SectionObserved = false
		if n, valid := readStoryMainInt32(rawSection); valid && n >= 0 {
			view.SectionIdx = n
			view.SectionObserved = true
		}
	}
	if rawUTime, exists := fields["3"]; exists {
		if n, valid := readStoryMainInt64(rawUTime); valid && n >= 0 {
			view.UTimeMs = n
		}
	}
	if rawCTime, exists := fields["4"]; exists {
		if n, valid := readStoryMainInt64(rawCTime); valid && n >= 0 {
			view.CTimeMs = n
		}
	}
	view.Valid = false
	view.Complete = false
	view.SectionID = 0
	view.ChapterName = ""
	view.SectionName = ""
	view.Cost = nil
	if view.ChapterObserved && view.SectionObserved {
		if def, valid, complete := ResolveStoryMainProgress(view.Chapter, view.SectionIdx); valid {
			view.Valid = true
			view.Complete = complete
			if !complete {
				view.SectionID = def.SectionID
				view.ChapterName = def.ChapterName
				view.SectionName = def.SectionName
				view.Cost = append([]ItemCount(nil), def.Cost...)
			}
		}
	}
	s.storyMain = view
}

func (s *State) invalidateStoryMainLocked() {
	view := s.storyMain
	view.Observed = true
	view.Valid = false
	view.Complete = false
	view.ChapterObserved = false
	view.SectionObserved = false
	view.SectionID = 0
	view.ChapterName = ""
	view.SectionName = ""
	view.Cost = nil
	s.storyMain = view
}

func (s *State) applyStatisticsLocked(raw json.RawMessage) {
	var ns124 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns124); err != nil {
		return
	}
	raw0, ok := ns124["0"]
	if !ok {
		return
	}
	if view, ok := parseStatisticsView(raw0); ok {
		s.statistics = view
		s.clearResidentOrderLimitIfStatisticsResetLocked(view)
	}
}

func parseStatisticsView(raw json.RawMessage) (StatisticsView, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return StatisticsView{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return StatisticsView{}, false
	}
	if _, ok := fields["9"]; ok {
		return parseStatisticsFields(fields)
	}
	var best StatisticsView
	for dayIDStr, rawEntry := range fields {
		var entryFields map[string]json.RawMessage
		if err := json.Unmarshal(rawEntry, &entryFields); err != nil {
			continue
		}
		entry, ok := parseStatisticsFields(entryFields)
		if !ok {
			continue
		}
		if entry.DayID == 0 {
			entry.DayID = atoi32(dayIDStr)
		}
		if !best.Observed || entry.DayID >= best.DayID {
			best = entry
		}
	}
	if best.Observed {
		return best, true
	}
	return StatisticsView{}, false
}

func parseStatisticsFields(fields map[string]json.RawMessage) (StatisticsView, bool) {
	view := StatisticsView{Observed: true}
	seen := false
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.DayID = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "8"); ok {
		view.FlowerArtSellNum = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "9"); ok {
		view.OrderFlowerFinishNum = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "10"); ok {
		view.OrderPalaceFinishNum = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "11"); ok {
		view.OrderCustomerFinishNum = n
		seen = true
	}
	if n, ok := readInt64JSONField(fields, "12"); ok {
		view.UTimeMs = n
		seen = true
	}
	if n, ok := readInt64JSONField(fields, "13"); ok {
		view.CTimeMs = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "14"); ok {
		view.OrderSatinFinishNum = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "16"); ok {
		view.OrderDecorateFinishNum = n
		seen = true
	}
	return view, seen
}

func (s *State) clearResidentOrderLimitIfStatisticsResetLocked(stats StatisticsView) {
	if s.residentOrderLimitUntilMs <= 0 {
		return
	}
	if stats.DayID != 0 && s.residentOrderLimitDayID != 0 && stats.DayID != s.residentOrderLimitDayID {
		s.residentOrderLimitUntilMs = 0
		s.residentOrderLimitDayID = 0
	}
}

func (s *State) applyFreeWaterLocked(raw json.RawMessage) {
	var ns117 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns117); err != nil {
		return
	}
	s.freeWaterObserved = true
	if v, ok := ns117["1"]; ok {
		s.freeWaterRecvIdx = readInt32ListRawAllowZero(v)
	}
	if v, ok := ns117["2"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.freeWaterResetMs = n
		}
	}
}

func (s *State) applyBenefitBoxLocked(raw json.RawMessage) {
	// NS 116 client schema: {"0": {"1": drawCnt, "2": resetCntTime, "3": uTime, "4": cTime}}
	var ns116 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns116); err != nil {
		return
	}
	s.benefitBoxObserved = true
	raw0, ok := ns116["0"]
	if !ok {
		return
	}
	var sub map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &sub); err != nil {
		return
	}
	if v, ok := sub["1"]; ok {
		var n int32
		if json.Unmarshal(v, &n) == nil {
			s.benefitBoxDrawCnt = n
		}
	}
	if v, ok := sub["2"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.benefitBoxResetCntMs = n
		}
	}
	if v, ok := sub["3"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.benefitBoxUTimeMs = n
		}
	}
}

func (s *State) applyVideoDoubleLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	view := s.videoDouble
	view.Observed = true
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.VideoCount = n
	}
	if n, ok := readInt64JSONField(fields, "2"); ok {
		view.EndTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "3"); ok {
		view.UpdatedAtMs = n
	}
	if n, ok := readInt64JSONField(fields, "4"); ok {
		view.CreatedAtMs = n
	}
	s.videoDouble = view
}

// BenefitBoxDrawsRemaining returns the number of free draws available.
func (s *State) BenefitBoxDrawsRemaining() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.benefitBoxDrawCnt
}

// BenefitBoxReady returns true if there are draws available.
func (s *State) BenefitBoxReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.benefitBoxObserved && s.benefitBoxDrawCnt > 0
}

// UsrExtra returns the tracked account-extension state.
func (s *State) UsrExtra() UsrExtraView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.usrExtra
}

// AntiFraudQAStatus returns the observed anti-fraud QA reward status.
func (s *State) AntiFraudQAStatus() (int32, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.usrExtra.AntiFraudQAStatus, s.usrExtra.Observed
}

// Reputation returns the observed own-account 礼仪分/健康分 state.
func (s *State) Reputation() (ReputationView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reputation, s.reputation.Observed
}

// VideoDouble returns the tracked double-coin video reward state.
func (s *State) VideoDouble() VideoDoubleView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.videoDouble
}

// VideoDoubleObserved reports whether namespace 118 has been observed.
func (s *State) VideoDoubleObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.videoDouble.Observed
}

// VideoDoubleActive reports whether the client-observed double-coin timer is active.
func (s *State) VideoDoubleActive(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.videoDoubleActiveLocked(now)
}

func (s *State) videoDoubleActiveLocked(now time.Time) bool {
	return s.videoDouble.Observed && s.videoDouble.EndTimeMs > now.UnixMilli()
}

// NextFreeWaterIndex returns the currently claimable idx for freeWater.recv.
// The mini client treats IFreeWater.recvIdx as the list of slots already
// claimed today and only calls recv(idx) for the active free-water time window.
func (s *State) NextFreeWaterIndex(now time.Time) (int32, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.freeWaterObserved {
		return 0, false
	}
	received := s.freeWaterReceivedIdxLocked(now)
	if periods := freeWaterClaimPeriods(); len(periods) > 0 {
		minute := now.Hour()*60 + now.Minute()
		for _, period := range periods {
			if !period.contains(minute) {
				continue
			}
			if received[period.idx] {
				return 0, false
			}
			return period.idx, true
		}
		return 0, false
	}
	for idx := int32(0); idx < 2; idx++ {
		if !received[idx] {
			return idx, true
		}
	}
	return 0, false
}

func (s *State) freeWaterReceivedIdxLocked(now time.Time) map[int32]bool {
	received := make(map[int32]bool, len(s.freeWaterRecvIdx))
	if s.freeWaterResetMs > 0 && dailyRefreshTime(now).After(time.UnixMilli(s.freeWaterResetMs).In(now.Location())) {
		return received
	}
	for _, idx := range s.freeWaterRecvIdx {
		received[idx] = true
	}
	return received
}

type freeWaterClaimPeriod struct {
	idx      int32
	startMin int
	endMin   int
}

func (p freeWaterClaimPeriod) contains(minute int) bool {
	if p.startMin == p.endMin {
		return false
	}
	if p.startMin < p.endMin {
		return minute >= p.startMin && minute < p.endMin
	}
	return minute >= p.startMin || minute < p.endMin
}

func freeWaterClaimPeriods() []freeWaterClaimPeriod {
	table, ok := catalog.Tables["c_gameCfg"]
	if !ok {
		return nil
	}
	rawRow, ok := table.Rows["-1"]
	if !ok {
		return nil
	}
	var row map[string]json.RawMessage
	if err := json.Unmarshal(rawRow, &row); err != nil {
		return nil
	}
	rawPeriods, ok := row["$freeWaterTime"]
	if !ok {
		return nil
	}
	var pairs [][]int32
	if err := json.Unmarshal(rawPeriods, &pairs); err != nil {
		return nil
	}
	periods := make([]freeWaterClaimPeriod, 0, len(pairs))
	for idx, pair := range pairs {
		if len(pair) < 2 {
			continue
		}
		start, okStart := decodeFreeWaterConfigMinute(pair[0])
		end, okEnd := decodeFreeWaterConfigMinute(pair[1])
		if !okStart || !okEnd {
			continue
		}
		periods = append(periods, freeWaterClaimPeriod{idx: int32(idx), startMin: start, endMin: end})
	}
	return periods
}

func decodeFreeWaterConfigMinute(raw int32) (int, bool) {
	if raw >= 80 && raw < 104 {
		return int(raw-80) * 60, true
	}
	if raw >= 0 && raw < 24 {
		return int(raw) * 60, true
	}
	if raw >= 0 && raw < 2400 {
		hour := raw / 100
		minute := raw % 100
		if hour < 24 && minute < 60 {
			return int(hour*60 + minute), true
		}
	}
	return 0, false
}

func dailyRefreshTime(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
}
