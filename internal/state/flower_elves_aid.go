package state

import (
	"encoding/json"
	"strconv"
	"time"
)

// FlowerElvesAidView is namespace 132.5 (IFlowerElvesAid).
//
// Observed wire samples carry partial updates of aidMap + reqAid; preReqAidTime
// and effEndTime appear when non-zero. Merge fields when applying deltas.
type FlowerElvesAidView struct {
	UID           int64
	AidMap        map[int64]int64 // frdUid -> help time ms
	PreReqAidTime int64
	EffEndTime    int64
	ReqAid        int64 // non-zero while a request is open
	UTime         int64
	CTime         int64
}

// FlowerElvesAid returns a copy of the observed aid snapshot.
func (s *State) FlowerElvesAid() FlowerElvesAidView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.copyFlowerElvesAidLocked()
}

// FlowerElvesAidObserved reports whether namespace 132.5 has been applied.
func (s *State) FlowerElvesAidObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.flowerElvesAidObserved
}

func (s *State) copyFlowerElvesAidLocked() FlowerElvesAidView {
	out := s.flowerElvesAid
	if len(s.flowerElvesAid.AidMap) > 0 {
		out.AidMap = make(map[int64]int64, len(s.flowerElvesAid.AidMap))
		for uid, ts := range s.flowerElvesAid.AidMap {
			out.AidMap[uid] = ts
		}
	}
	return out
}

// CanRecvFlowerElvesAid reports whether recvAidEff is safe to call.
//
// Observed: once $friendHelpNum friends have helped, the server may clear
// reqAid (field 4→0) while aidMap still holds helpers and the buff is not yet
// claimed. Requiring reqAid!=0 misses that window. Gate on helpers + no active
// buff + a prior request, and skip when this PreReqAidTime was already claimed.
func (s *State) CanRecvFlowerElvesAid(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.canRecvFlowerElvesAidLocked(now)
}

func (s *State) canRecvFlowerElvesAidLocked(now time.Time) bool {
	if !s.flowerElvesAidObserved {
		return false
	}
	if s.flowerElvesAid.PreReqAidTime <= 0 && s.flowerElvesAid.ReqAid == 0 {
		return false
	}
	if s.flowerElvesAid.PreReqAidTime > 0 && s.flowerElvesAid.PreReqAidTime == s.flowerElvesAidClaimedPreReq {
		return false
	}
	if s.flowerElvesAid.EffEndTime > 0 && now.UnixMilli() < s.flowerElvesAid.EffEndTime {
		return false
	}
	g, ok := FlowerElvesGlobalsFromCatalog()
	need := int32(1)
	if ok && g.FriendHelpNum > 0 {
		need = g.FriendHelpNum
	}
	return int32(len(s.flowerElvesAid.AidMap)) >= need
}

// CanReqFlowerElvesAid reports whether reqAid is safe to call.
func (s *State) CanReqFlowerElvesAid(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.flowerElvesAidObserved {
		return false
	}
	if s.flowerElvesAid.ReqAid != 0 {
		return false
	}
	// Pending claim: server rejects a new request with "已发起过协助请求".
	if s.canRecvFlowerElvesAidLocked(now) {
		return false
	}
	if s.flowerElvesAid.EffEndTime > 0 && now.UnixMilli() < s.flowerElvesAid.EffEndTime {
		return false
	}
	if _, blocked := s.flowerElvesAidReqReadyAtLocked(now); blocked {
		return false
	}
	return true
}

// flowerElvesAidReqReadyAtLocked returns the next reqAid-ready instant and
// whether the cooldown currently blocks. readyAt is 0 when not blocked.
func (s *State) flowerElvesAidReqReadyAtLocked(now time.Time) (readyAt int64, blocked bool) {
	if s.flowerElvesAid.PreReqAidTime <= 0 {
		return 0, false
	}
	g, ok := FlowerElvesGlobalsFromCatalog()
	cooldownMin := int32(150)
	if ok && g.FriendTimeMin > 0 {
		cooldownMin = g.FriendTimeMin
	}
	readyAt = s.flowerElvesAid.PreReqAidTime + int64(cooldownMin)*60_000
	if now.UnixMilli() < readyAt {
		return readyAt, true
	}
	return 0, false
}

// FlowerElvesAidHelpCountToday is the durable calendar-day count of successful helpFrd.
func (s *State) FlowerElvesAidHelpCountToday(now time.Time) int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.elvesAidHelpDayID != calendarDayID(now) {
		return 0
	}
	return int32(len(s.elvesAidHelped))
}

// FlowerElvesAidHelpedToday reports whether dstUid was already helped today.
func (s *State) FlowerElvesAidHelpedToday(dstUID int64, now time.Time) bool {
	if dstUID <= 0 {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.elvesAidHelpDayID != calendarDayID(now) {
		return false
	}
	_, ok := s.elvesAidHelped[dstUID]
	return ok
}

// NoteFlowerElvesAidHelped records a successful helpFrd and clears cached isAid.
func (s *State) NoteFlowerElvesAidHelped(dstUID int64, at time.Time) {
	if dstUID <= 0 || at.IsZero() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	day := calendarDayID(at)
	if s.elvesAidHelpDayID != day {
		s.elvesAidHelpDayID = day
		s.elvesAidHelped = make(map[int64]struct{})
	}
	if s.elvesAidHelped == nil {
		s.elvesAidHelped = make(map[int64]struct{})
	}
	s.elvesAidHelped[dstUID] = struct{}{}
	if info, ok := s.frdOtherInfo[dstUID]; ok {
		info.IsAid = false
		s.frdOtherInfo[dstUID] = info
	}
}

// SetFlowerElvesAidHelped replaces the in-memory calendar-day helpFrd targets.
// Callers hydrate this from durable storage (and operation_log recovery) on start.
func (s *State) SetFlowerElvesAidHelped(dayID int32, uids []int64) {
	if dayID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.elvesAidHelpDayID = dayID
	s.elvesAidHelped = make(map[int64]struct{}, len(uids))
	for _, uid := range uids {
		if uid <= 0 {
			continue
		}
		s.elvesAidHelped[uid] = struct{}{}
		if info, ok := s.frdOtherInfo[uid]; ok {
			info.IsAid = false
			s.frdOtherInfo[uid] = info
		}
	}
}

func (s *State) applyFlowerElvesAidLocked(raw json.RawMessage) {
	if len(raw) == 0 || isJSONNull(raw) {
		// Explicit null clears the blob; still mark observed so planners do not
		// spin on sync forever when the server omits an empty aid object.
		s.flowerElvesAid = FlowerElvesAidView{}
		s.flowerElvesAidObserved = true
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	view := s.flowerElvesAid
	if view.AidMap == nil {
		view.AidMap = make(map[int64]int64)
	}
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if rawMap, ok := fields["1"]; ok {
		view.AidMap = decodeFlowerElvesAidMap(rawMap)
	}
	if raw, ok := fields["2"]; ok {
		if isJSONNull(raw) {
			view.PreReqAidTime = 0
		} else if n, ok := readInt64JSONField(fields, "2"); ok {
			view.PreReqAidTime = n
		}
	}
	if raw, ok := fields["3"]; ok {
		// Server sends null when the buff is inactive; clear so stale end times
		// from a prior session do not keep showing as active.
		if isJSONNull(raw) {
			view.EffEndTime = 0
		} else if n, ok := readInt64JSONField(fields, "3"); ok {
			view.EffEndTime = n
		}
	}
	if raw, ok := fields["4"]; ok {
		if isJSONNull(raw) {
			view.ReqAid = 0
		} else if n, ok := readInt64JSONField(fields, "4"); ok {
			view.ReqAid = n
		}
	}
	if n, ok := readInt64JSONField(fields, "5"); ok {
		view.UTime = n
	}
	if n, ok := readInt64JSONField(fields, "6"); ok {
		view.CTime = n
	}
	s.flowerElvesAid = view
	s.flowerElvesAidObserved = true
	// An observed active buff means this preReq cycle was already claimed.
	if view.EffEndTime > 0 && view.PreReqAidTime > 0 {
		s.flowerElvesAidClaimedPreReq = view.PreReqAidTime
	}
}

func decodeFlowerElvesAidMap(raw json.RawMessage) map[int64]int64 {
	out := make(map[int64]int64)
	if len(raw) == 0 || isJSONNull(raw) {
		return out
	}
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil {
		return out
	}
	for key, rawTS := range entries {
		uid, err := strconv.ParseInt(key, 10, 64)
		if err != nil || uid <= 0 {
			continue
		}
		if isJSONNull(rawTS) {
			continue
		}
		if ts, ok := readInt64Raw(rawTS); ok && ts > 0 {
			out[uid] = ts
		}
	}
	return out
}
