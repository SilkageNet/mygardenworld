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
	// Sparse namespace-124 deltas often carry only the changed counters. Merge
	// present fields into the current day so orderFlowerFinishNum is not wiped
	// by an unrelated patch (e.g. flowerArtSellNum-only), which would disable
	// the ordinary resident-order daily limit.
	days, view, countersSeen, ok := parseStatisticsDays(s.statisticsByDay, s.statistics, raw0)
	if ok {
		s.statisticsByDay = days
		prevDay := s.statistics.DayID
		s.statistics = view
		dayChanged := view.DayID != 0 && prevDay != 0 && view.DayID != prevDay
		switch {
		case countersSeen.normal:
			// Authoritative field 9 replaces the local high-water for this day.
			s.residentOrderFinishBias = 0
			if view.DayID != 0 {
				s.residentOrderFinishBiasDayID = view.DayID
			}
		case view.DayID != 0 && prevDay != 0 && view.DayID != prevDay:
			// New game-day stats without field 9: drop bias that belonged to the
			// previous day, but keep bias already accumulated for view.DayID
			// (finishes after 00:05 before the server published today's counters).
			if s.residentOrderFinishBiasDayID != view.DayID {
				s.residentOrderFinishBias = 0
				s.residentOrderFinishBiasDayID = view.DayID
			}
		}
		if countersSeen.satin || dayChanged {
			s.residentSatinFinishBias = 0
			if view.DayID != 0 {
				s.residentSatinFinishBiasDayID = view.DayID
			}
		}
		if countersSeen.decorate || dayChanged {
			s.residentDecorateFinishBias = 0
			if view.DayID != 0 {
				s.residentDecorateFinishBiasDayID = view.DayID
			}
		}
		s.clearResidentOrderLimitIfStatisticsResetLocked(view)
	}
}

type statisticsCountersSeen struct {
	normal   bool
	satin    bool
	decorate bool
}

func parseStatisticsViewMerged(prev StatisticsView, raw json.RawMessage) (StatisticsView, statisticsCountersSeen, bool) {
	_, latest, seen, ok := parseStatisticsDays(nil, prev, raw)
	return latest, seen, ok
}

func parseStatisticsDays(prevDays map[int32]StatisticsView, prevLatest StatisticsView, raw json.RawMessage) (map[int32]StatisticsView, StatisticsView, statisticsCountersSeen, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, StatisticsView{}, statisticsCountersSeen{}, false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, StatisticsView{}, statisticsCountersSeen{}, false
	}
	days := cloneStatisticsDays(prevDays)
	if hasStatisticsField(fields) {
		base := statisticsDayBase(days, prevLatest, prevLatest.DayID)
		view, countersSeen, ok := mergeStatisticsFields(base, fields, 0)
		if !ok {
			return nil, StatisticsView{}, statisticsCountersSeen{}, false
		}
		storeStatisticsDay(days, view)
		return days, view, countersSeen, true
	}
	var best StatisticsView
	seen := false
	countersSeen := statisticsCountersSeen{}
	for dayIDStr, rawEntry := range fields {
		var entryFields map[string]json.RawMessage
		if err := json.Unmarshal(rawEntry, &entryFields); err != nil {
			continue
		}
		// Live finishOrder patches key days by Asia/Shanghai midnight ms
		// (e.g. "1785686400000"); atoi32 overflows and previously made
		// ResidentOrderFinishNum treat today's counter as a prior-day 0.
		dayHint := normalizeStatisticsDayID(atoi64(dayIDStr))
		base := statisticsDayBase(days, prevLatest, dayHint)
		entry, entryCounters, ok := mergeStatisticsFields(base, entryFields, dayHint)
		if !ok {
			continue
		}
		storeStatisticsDay(days, entry)
		if !seen || entry.DayID >= best.DayID {
			best = entry
			countersSeen = entryCounters
			seen = true
		}
	}
	if seen {
		return days, best, countersSeen, true
	}
	return nil, StatisticsView{}, statisticsCountersSeen{}, false
}

func cloneStatisticsDays(in map[int32]StatisticsView) map[int32]StatisticsView {
	out := make(map[int32]StatisticsView, len(in))
	for dayID, view := range in {
		out[dayID] = view
	}
	return out
}

func statisticsDayBase(days map[int32]StatisticsView, prevLatest StatisticsView, dayID int32) StatisticsView {
	if dayID != 0 {
		if stored, ok := days[dayID]; ok {
			return stored
		}
	}
	if prevLatest.Observed && (dayID == 0 || prevLatest.DayID == 0 || prevLatest.DayID == dayID) {
		return prevLatest
	}
	return StatisticsView{}
}

func storeStatisticsDay(days map[int32]StatisticsView, view StatisticsView) {
	if view.DayID == 0 {
		return
	}
	days[view.DayID] = view
}

func hasStatisticsField(fields map[string]json.RawMessage) bool {
	for _, key := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15", "16", "17"} {
		if _, ok := fields[key]; ok {
			return true
		}
	}
	return false
}

func mergeStatisticsFields(prev StatisticsView, fields map[string]json.RawMessage, dayHint int32) (StatisticsView, statisticsCountersSeen, bool) {
	view := prev
	seen := false
	countersSeen := statisticsCountersSeen{}
	if rawDay, ok := readInt64JSONField(fields, "1"); ok {
		n := normalizeStatisticsDayID(rawDay)
		if n == 0 {
			n = int32(rawDay)
		}
		if prev.Observed && prev.DayID != 0 && n != prev.DayID {
			// New game day: drop prior-day counters rather than mixing them.
			view = StatisticsView{}
		}
		view.DayID = n
		seen = true
	} else if view.DayID == 0 && dayHint != 0 {
		view.DayID = dayHint
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.Gold = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "3"); ok {
		view.Experience = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "4"); ok {
		view.Diamonds = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "5"); ok {
		view.SpeedUpCard = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "6"); ok {
		view.FlowerShopCoin = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "7"); ok {
		view.FlowerHarvestNum = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "8"); ok {
		view.FlowerArtSellNum = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "9"); ok {
		view.OrderFlowerFinishNum = n
		countersSeen.normal = true
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
		countersSeen.satin = true
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "15"); ok {
		view.Satin = n
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "16"); ok {
		view.OrderDecorateFinishNum = n
		countersSeen.decorate = true
		seen = true
	}
	if n, ok := readInt32JSONField(fields, "17"); ok {
		view.Wood = n
		seen = true
	}
	if !seen {
		return StatisticsView{}, statisticsCountersSeen{}, false
	}
	view.Observed = true
	return view, countersSeen, true
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

// normalizeStatisticsDayID converts namespace-124 day identifiers to YYYYMMDD.
// Clients may send either catalog-style YYYYMMDD or Asia/Shanghai midnight
// unix milliseconds; the latter must not be forced through int32.
func normalizeStatisticsDayID(v int64) int32 {
	if v <= 0 {
		return 0
	}
	if v >= 20000101 && v <= 21001231 {
		return int32(v)
	}
	switch {
	case v >= 1_000_000_000_000:
		return calendarDayID(time.UnixMilli(v))
	case v >= 1_000_000_000:
		return calendarDayID(time.Unix(v, 0))
	default:
		return 0
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

// BenefitBoxDrawsRemaining mirrors G.BenefitBoxCtrl.getBenefitBoxInfo:
// drawCnt is the synced baseline; when below c_benefitBox.$boxMax, boxes
// refill every $boxCd seconds relative to resetCntTime without a push.
func (s *State) BenefitBoxDrawsRemaining(now time.Time) int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.benefitBoxDrawsRemainingLocked(now)
}

// BenefitBoxReady reports whether at least one free draw is available at now.
func (s *State) BenefitBoxReady(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.benefitBoxObserved && s.benefitBoxDrawsRemainingLocked(now) > 0
}

// BenefitBoxObserved reports whether namespace 116 has been seen at least once.
func (s *State) BenefitBoxObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.benefitBoxObserved
}

func (s *State) benefitBoxDrawsRemainingLocked(now time.Time) int32 {
	max := benefitBoxMax()
	cnt := s.benefitBoxDrawCnt
	if cnt >= max {
		return max
	}
	if s.benefitBoxResetCntMs <= 0 {
		return cnt
	}
	nowMs := now.UnixMilli()
	if s.benefitBoxResetCntMs > nowMs {
		return cnt
	}
	cdMs := int64(benefitBoxCD() / time.Millisecond)
	if cdMs <= 0 {
		return cnt
	}
	gained := (nowMs - s.benefitBoxResetCntMs) / cdMs
	if gained <= 0 {
		return cnt
	}
	total := cnt + int32(gained)
	if total > max {
		return max
	}
	return total
}

func benefitBoxCD() time.Duration {
	rawRow, ok := StaticRow("c_benefitBox", -1)
	if !ok {
		return 3600 * time.Second
	}
	var row map[string]json.RawMessage
	if err := json.Unmarshal(rawRow, &row); err != nil {
		return 3600 * time.Second
	}
	n, ok := readInt32JSONField(row, "$boxCd")
	if !ok || n <= 0 {
		return 3600 * time.Second
	}
	return time.Duration(n) * time.Second
}

func benefitBoxMax() int32 {
	rawRow, ok := StaticRow("c_benefitBox", -1)
	if !ok {
		return 8
	}
	var row map[string]json.RawMessage
	if err := json.Unmarshal(rawRow, &row); err != nil {
		return 8
	}
	n, ok := readInt32JSONField(row, "$boxMax")
	if !ok || n <= 0 {
		return 8
	}
	return n
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
// Catalog windows are 11:00–14:00 and 17:00–21:00 Asia/Shanghai; automation
// claims any time the active window is open and that slot is still unclaimed.
// When namespace 117 has never been observed, unclaimed slots are assumed so
// the first in-window recv can bootstrap server state.
func (s *State) NextFreeWaterIndex(now time.Time) (int32, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	received := s.freeWaterReceivedIdxLocked(now)
	local := now.In(gameDayLocation())
	if periods := freeWaterClaimPeriods(); len(periods) > 0 {
		minute := local.Hour()*60 + local.Minute()
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
	return 0, false
}

func (s *State) freeWaterReceivedIdxLocked(now time.Time) map[int32]bool {
	received := make(map[int32]bool, len(s.freeWaterRecvIdx))
	if !s.freeWaterObserved {
		return received
	}
	if s.freeWaterResetMs > 0 && gameDayID(now) > gameDayID(time.UnixMilli(s.freeWaterResetMs)) {
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

// contains reports whether minute is inside the catalog window. It supports
// both ordinary and overnight windows; the start is inclusive and end exclusive.
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
