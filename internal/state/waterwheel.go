package state

import (
	"encoding/json"
	"time"
)

func (s *State) applyWaterwheelLocked(raw json.RawMessage) {
	var ns114 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns114); err != nil {
		return
	}
	now := time.Now()
	nowMs := now.UnixMilli()
	wasObserved := s.wwObserved
	prevCount := s.wwClaimedCount
	nextCount := s.wwClaimedCount
	countObserved := false
	if v, ok := ns114["1"]; ok {
		var n int32
		if json.Unmarshal(v, &n) == nil {
			nextCount = n
			countObserved = true
		}
	}
	if v, ok := ns114["2"]; ok {
		s.wwAdvList = readInt32ListRaw(v)
	}
	if v, ok := ns114["4"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.wwLastRecvTs = n
		}
	}
	if v, ok := ns114["5"]; ok {
		var n int64
		if json.Unmarshal(v, &n) == nil {
			s.wwCTimeMs = n
		}
	}
	s.wwObserved = true
	if s.wwEntered && (!wasObserved || s.wwLocalGenMs == 0 || (countObserved && nextCount < prevCount)) {
		s.wwLocalGenMs = nowMs
	}
	if countObserved {
		if nextCount < prevCount {
			s.wwBackoffUntil = 0
		}
		if nextCount > prevCount {
			available := s.waterwheelLocalBucketCountAtLocked(now, prevCount)
			remaining := available - (nextCount - prevCount)
			if remaining < 0 {
				remaining = 0
			}
			s.wwLocalGenMs = nowMs - int64(waterwheelBucketCreateInterval()/time.Millisecond)*int64(remaining)
			s.wwBackoffUntil = 0
		}
		s.wwClaimedCount = nextCount
	}
}

// WaterwheelClaimedCount returns the total number of waterwheel claims made.
func (s *State) WaterwheelClaimedCount() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.wwClaimedCount
}

// WaterwheelNextClaimRequiresSkip reports whether the next bucket index is in
// namespace 114.2 advList. The mini client calls waterwheel.skip before recv
// for these buckets when it cannot or should not play the ad.
func (s *State) WaterwheelNextClaimRequiresSkip() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	next := s.wwClaimedCount + 1
	for _, id := range s.wwAdvList {
		if id == next {
			return true
		}
	}
	return false
}

// WaterwheelReady is a compatibility accessor used by older diagnostics. It
// returns 1 when the local cooldown view says a claim can be attempted, else 0.
func (s *State) WaterwheelReady() int32 {
	if s.WaterwheelCooldownReady() {
		return 1
	}
	return 0
}

// WaterwheelEnterDue reports whether automation should call waterwheel.enter
// to initialize the same client-side bucket lifecycle used by the mini client.
func (s *State) WaterwheelEnterDue(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.wwEntered {
		return false
	}
	nowMs := now.UnixMilli()
	if s.wwBackoffUntil > nowMs {
		return false
	}
	if max := waterwheelBucketDailyMax(); max > 0 && s.wwObserved && s.wwClaimedCount >= max {
		return false
	}
	return true
}

// MarkWaterwheelEntered starts the local bucket-generation lifecycle after a
// successful waterwheel.enter RPC.
func (s *State) MarkWaterwheelEntered(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wwEntered = true
	s.wwLocalGenMs = now.UnixMilli()
	s.wwBackoffUntil = 0
}

// MarkWaterwheelUnavailable temporarily suppresses local waterwheel claim
// attempts when the server says the generated bucket state is invalid.
func (s *State) MarkWaterwheelUnavailable(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wwEntered = false
	s.wwLocalGenMs = 0
	s.wwBackoffUntil = now.Add(waterwheelBucketCreateInterval()).UnixMilli()
}

// MarkWaterwheelDailyLimitReached records the server-side daily cap so the
// planner stops selecting waterwheel.recv until namespace 114 reports a reset.
func (s *State) MarkWaterwheelDailyLimitReached(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wwObserved = true
	if max := waterwheelBucketDailyMax(); max > 0 {
		if s.wwClaimedCount < max {
			s.wwClaimedCount = max
		}
		s.wwBackoffUntil = 0
	} else {
		s.wwBackoffUntil = now.Add(24 * time.Hour).UnixMilli()
	}
	s.wwEntered = false
	s.wwLocalGenMs = 0
}

func waterwheelBucketCreateInterval() time.Duration {
	raw, ok := catalog.Tables["c_waterwheel"].Rows["-1"]
	if !ok {
		return time.Hour
	}
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil {
		return time.Hour
	}
	seconds := readInt32Any(row["$bucketCreateCd"])
	if seconds <= 0 {
		return time.Hour
	}
	return time.Duration(seconds) * time.Second
}

func waterwheelBucketDailyMax() int32 {
	raw, ok := catalog.Tables["c_waterwheel"].Rows["-1"]
	if !ok {
		return 0
	}
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil {
		return 0
	}
	return readInt32Any(row["$bucketGetMax"])
}

func waterwheelBucketExistMax() int32 {
	raw, ok := catalog.Tables["c_waterwheel"].Rows["-1"]
	if !ok {
		return 0
	}
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil {
		return 0
	}
	return readInt32Any(row["$bucketExistMax"])
}

// WaterwheelCooldownReady returns true if the emulated client-side bucket
// timer says a waterwheel claim can be attempted. The mini client generates
// buckets locally after waterwheel.enter using c_waterwheel.$bucketCreateCd,
// keeps positions in BucketPosUsed_<uid>, and only then calls waterwheel.recv.
func (s *State) WaterwheelCooldownReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nowMs := time.Now().UnixMilli()
	if !s.wwObserved || !s.wwEntered {
		return false
	}
	if s.wwBackoffUntil > nowMs {
		return false
	}
	if max := waterwheelBucketDailyMax(); max > 0 && s.wwClaimedCount >= max {
		return false
	}
	return s.waterwheelLocalBucketCountAtLocked(time.Now(), s.wwClaimedCount) > 0
}

func (s *State) waterwheelLocalBucketCountAtLocked(now time.Time, claimedCount int32) int32 {
	if s.wwLocalGenMs <= 0 {
		return 0
	}
	interval := waterwheelBucketCreateInterval()
	if interval <= 0 {
		return 0
	}
	elapsed := time.Duration(now.UnixMilli()-s.wwLocalGenMs) * time.Millisecond
	if elapsed < 0 {
		return 0
	}
	count := int32(elapsed / interval)
	if existMax := waterwheelBucketExistMax(); existMax > 0 && count > existMax {
		count = existMax
	}
	if max := waterwheelBucketDailyMax(); max > 0 {
		remaining := max - claimedCount
		if remaining < 0 {
			remaining = 0
		}
		if count > remaining {
			count = remaining
		}
	}
	return count
}
