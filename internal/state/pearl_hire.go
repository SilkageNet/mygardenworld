package state

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
)

const pearlHireTicketItemID int32 = 1003

type pearlFriendRelation struct {
	UID0 int64
	UID1 int64
}

func (s *State) applyPearlFriendsLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return
	}
	rawRelations, hasRelations := fields["1"]
	if !hasRelations {
		return
	}
	if isJSONNull(rawRelations) {
		return
	}
	var relations []json.RawMessage
	if json.Unmarshal(rawRelations, &relations) != nil {
		return
	}
	parsed := make([]pearlFriendRelation, 0, len(relations))
	for _, rawRelation := range relations {
		var relationFields map[string]json.RawMessage
		if json.Unmarshal(rawRelation, &relationFields) != nil {
			return
		}
		uid0, ok0 := readExactInt64Raw(relationFields["0"])
		uid1, ok1 := readExactInt64Raw(relationFields["1"])
		if !ok0 || !ok1 || uid0 <= 0 || uid1 <= 0 {
			return
		}
		parsed = append(parsed, pearlFriendRelation{UID0: uid0, UID1: uid1})
	}
	rawBase, hasBase := fields["0"]
	if hasBase {
		s.applyFrdStealCntBuyLocked(rawBase)
		var baseFields map[string]json.RawMessage
		if json.Unmarshal(rawBase, &baseFields) != nil {
			return
		}
		baseUID, validBaseUID := readExactInt64Raw(baseFields["0"])
		if !validBaseUID || baseUID <= 0 {
			return
		}
	}
	if s.pearlFriendRelations == nil || hasBase {
		s.pearlFriendRelations = make(map[string]pearlFriendRelation)
		s.pearlFriendOrder = nil
	}
	if hasBase {
		s.pearlFriendsObserved = true
	}
	for _, relation := range parsed {
		uid0, uid1 := relation.UID0, relation.UID1
		key := pearlRelationKey(uid0, uid1)
		if _, exists := s.pearlFriendRelations[key]; !exists {
			s.pearlFriendOrder = append(s.pearlFriendOrder, key)
		}
		s.pearlFriendRelations[key] = pearlFriendRelation{UID0: uid0, UID1: uid1}
	}
}

func (s *State) applyPearlProfilesLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return
	}
	rawProfiles, ok := fields["5"]
	if !ok || isJSONNull(rawProfiles) {
		return
	}
	var profiles []json.RawMessage
	if json.Unmarshal(rawProfiles, &profiles) != nil {
		return
	}
	if s.pearlProfiles == nil {
		s.pearlProfiles = make(map[int64]*PearlCandidateProfile)
	}
	for _, rawProfile := range profiles {
		var profileFields map[string]json.RawMessage
		if json.Unmarshal(rawProfile, &profileFields) != nil {
			continue
		}
		uid, validUID := readExactInt64Raw(profileFields["0"])
		if !validUID || uid <= 0 {
			continue
		}
		profile := s.pearlProfiles[uid]
		if profile == nil {
			profile = &PearlCandidateProfile{UID: uid}
			s.pearlProfiles[uid] = profile
		}
		if rawName, exists := profileFields["1"]; exists {
			if isJSONNull(rawName) {
				profile.Name = ""
			} else {
				_ = json.Unmarshal(rawName, &profile.Name)
			}
		}
		if rawLevel, exists := profileFields["4"]; exists {
			level, validLevel := readExactInt32Raw(rawLevel)
			profile.LevelObserved = validLevel && level >= 0
			if profile.LevelObserved {
				profile.Level = level
				profile.ObservedAtMs = s.lastApplyMs
			} else {
				profile.Level = 0
				profile.ObservedAtMs = 0
			}
		}
	}
}

func (s *State) applyPearlHireStatesLocked(raw json.RawMessage) {
	if isJSONNull(raw) {
		return
	}
	var entries map[string]json.RawMessage
	if json.Unmarshal(raw, &entries) != nil {
		return
	}
	if s.pearlHireStates == nil {
		s.pearlHireStates = make(map[int64]*PearlCandidateHireState)
	}
	for uidText, rawEndTime := range entries {
		uid, validUID := parseExactPositiveInt64(uidText)
		if !validUID {
			continue
		}
		if isJSONNull(rawEndTime) {
			delete(s.pearlHireStates, uid)
			continue
		}
		endTime, validEndTime := readExactInt64Raw(rawEndTime)
		if !validEndTime || endTime < 0 {
			delete(s.pearlHireStates, uid)
			continue
		}
		s.pearlHireStates[uid] = &PearlCandidateHireState{
			UID:            uid,
			LaborEndTimeMs: endTime,
			ObservedAtMs:   s.lastApplyMs,
		}
	}
}

func (s *State) replacePearlRecommendationsLocked(raw json.RawMessage) {
	if isJSONNull(raw) {
		return
	}
	var entries []json.RawMessage
	if json.Unmarshal(raw, &entries) != nil {
		return
	}
	seen := make(map[int64]struct{}, len(entries))
	parsed := make([]int64, 0, len(entries))
	for _, rawUID := range entries {
		uid, ok := readExactInt64Raw(rawUID)
		if !ok || uid <= 0 {
			return
		}
		if _, exists := seen[uid]; exists {
			continue
		}
		seen[uid] = struct{}{}
		parsed = append(parsed, uid)
	}
	s.pearlRecommendUIDs = parsed
	s.pearlRecommendObserved = true
	s.pearlRecommendAtMs = s.lastApplyMs
}

func (s *State) applyPearlEnemiesLocked(rawPearl json.RawMessage) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(rawPearl, &fields) != nil {
		return
	}
	rawEnemies, ok := fields["5"]
	if !ok {
		return
	}
	if isJSONNull(rawEnemies) {
		return
	}
	var entries map[string]json.RawMessage
	if json.Unmarshal(rawEnemies, &entries) != nil {
		return
	}
	type enemyUpdate struct {
		uid       int64
		eventTime int64
		delete    bool
	}
	updates := make([]enemyUpdate, 0, len(entries))
	for uidText, rawEventTime := range entries {
		uid, validUID := parseExactPositiveInt64(uidText)
		if !validUID {
			return
		}
		if isJSONNull(rawEventTime) {
			updates = append(updates, enemyUpdate{uid: uid, delete: true})
			continue
		}
		eventTime, validEventTime := readExactInt64Raw(rawEventTime)
		if !validEventTime || eventTime <= 0 {
			return
		}
		updates = append(updates, enemyUpdate{uid: uid, eventTime: eventTime})
	}
	if s.pearlEnemies == nil {
		s.pearlEnemies = make(map[int64]int64)
	}
	for _, update := range updates {
		if update.delete {
			delete(s.pearlEnemies, update.uid)
			continue
		}
		s.pearlEnemies[update.uid] = update.eventTime
	}
	s.pearlEnemiesObserved = true
}

// PearlHire returns a defensive view evaluated at the current clock.
func (s *State) PearlHire() PearlHireView {
	return s.PearlHireAt(time.Now())
}

// PearlHireAt returns a defensive view of all candidate state and evaluates
// the durable daily ticket counter against the supplied calendar day.
func (s *State) PearlHireAt(now time.Time) PearlHireView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	view := PearlHireView{
		RoleID:                s.roleID,
		TicketCount:           s.inventory[pearlHireTicketItemID],
		TicketUsedToday:       s.pearlHireTicketUsedTodayLocked(now),
		NobleEligible:         s.nobleEligibleLocked(),
		Places:                make(map[int32]PearlPlaceView, len(s.pearlPlaces)),
		FriendUIDs:            s.pearlFriendUIDsLocked(),
		FriendsObserved:       s.pearlFriendsObserved,
		Profiles:              make(map[int64]PearlCandidateProfile, len(s.pearlProfiles)),
		HireStates:            make(map[int64]PearlCandidateHireState, len(s.pearlHireStates)),
		RecommendUIDs:         append([]int64(nil), s.pearlRecommendUIDs...),
		RecommendObserved:     s.pearlRecommendObserved,
		RecommendObservedAtMs: s.pearlRecommendAtMs,
		EnemiesObserved:       s.pearlEnemiesObserved,
		FailedUntilMs:         make(map[int64]int64, len(s.pearlHireFailedUntil)),
		SkippedUIDs:           make(map[int64]struct{}, len(s.pearlHireSkippedUIDs)),
		SessionLocked:         s.pearlHireSessionLocked,
		SessionLockReason:     s.pearlHireLockReason,
	}
	for id, place := range s.pearlPlaces {
		if place != nil {
			view.Places[id] = *place
		}
	}
	for uid, profile := range s.pearlProfiles {
		if profile != nil {
			view.Profiles[uid] = *profile
		}
	}
	for uid, hireState := range s.pearlHireStates {
		if hireState != nil {
			view.HireStates[uid] = *hireState
		}
	}
	for uid, eventTime := range s.pearlEnemies {
		view.Enemies = append(view.Enemies, PearlEnemyView{UID: uid, EventTimeMs: eventTime})
	}
	sort.Slice(view.Enemies, func(i, j int) bool {
		if view.Enemies[i].EventTimeMs != view.Enemies[j].EventTimeMs {
			return view.Enemies[i].EventTimeMs > view.Enemies[j].EventTimeMs
		}
		return view.Enemies[i].UID < view.Enemies[j].UID
	})
	for uid, until := range s.pearlHireFailedUntil {
		view.FailedUntilMs[uid] = until
	}
	for uid := range s.pearlHireSkippedUIDs {
		view.SkippedUIDs[uid] = struct{}{}
	}
	return view
}

// SetPearlHireTicketUsed hydrates the durable counter for one Shanghai
// calendar day. Invalid values are rejected rather than normalized.
func (s *State) SetPearlHireTicketUsed(dayID, used int32) {
	if dayID <= 0 || used < 0 {
		return
	}
	s.mu.Lock()
	s.pearlHireTicketUsedDayID = dayID
	s.pearlHireTicketUsedToday = used
	s.mu.Unlock()
}

// MergePearlHireTicketUsed applies an atomic persistence result without ever
// reducing a more conservative in-memory count accumulated after an earlier
// database failure.
func (s *State) MergePearlHireTicketUsed(dayID, used int32) {
	if dayID <= 0 || used < 0 {
		return
	}
	s.mu.Lock()
	if s.pearlHireTicketUsedDayID != dayID {
		s.pearlHireTicketUsedDayID = dayID
		s.pearlHireTicketUsedToday = used
	} else if used > s.pearlHireTicketUsedToday {
		s.pearlHireTicketUsedToday = used
	}
	s.mu.Unlock()
}

// NotePearlHireTicketUsed records one observed ticket decrement in memory and
// returns the new conservative day total. The runner supplies this high-water
// mark to persistence so a later successful write repairs earlier failures.
func (s *State) NotePearlHireTicketUsed(at time.Time) int32 {
	if at.IsZero() {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dayID := PearlHireTicketDayID(at)
	if s.pearlHireTicketUsedDayID != dayID {
		s.pearlHireTicketUsedDayID = dayID
		s.pearlHireTicketUsedToday = 0
	}
	s.pearlHireTicketUsedToday++
	return s.pearlHireTicketUsedToday
}

func (s *State) pearlHireTicketUsedTodayLocked(now time.Time) int32 {
	if now.IsZero() || s.pearlHireTicketUsedDayID != PearlHireTicketDayID(now) {
		return 0
	}
	return s.pearlHireTicketUsedToday
}

// PearlHireTicketDayID is the Asia/Shanghai calendar day used by the daily
// hire-ticket policy. The official limit resets at 00:00, not at 00:05.
func PearlHireTicketDayID(now time.Time) int32 {
	return calendarDayID(now)
}

// PearlHireAttemptSnapshot captures the live ticket balance and slot revision
// immediately before one hire request.
func (s *State) PearlHireAttemptSnapshot(placeID int32, targetUID int64, at time.Time) (PearlHireAttemptSnapshot, bool) {
	if placeID <= 0 || targetUID <= 0 || at.IsZero() {
		return PearlHireAttemptSnapshot{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	place, exists := s.pearlPlaces[placeID]
	if !exists || place == nil {
		return PearlHireAttemptSnapshot{}, false
	}
	return PearlHireAttemptSnapshot{
		At:                   at,
		PlaceID:              placeID,
		TargetUID:            targetUID,
		TicketCount:          s.inventory[pearlHireTicketItemID],
		PreviousPlaceUTimeMs: place.UTimeMs,
	}, true
}

// PearlHireAttemptApplied classifies an authoritative response after it has
// been merged. known=true with failCount>0 is a contested hire; success=true
// requires an updated slot, explicit hireFailCnt=0, the requested labor UID,
// a future end timestamp, and an exact one-ticket decrement.
func (s *State) PearlHireAttemptApplied(snapshot PearlHireAttemptSnapshot) (success bool, failCount int32, known bool) {
	if snapshot.At.IsZero() || snapshot.PlaceID <= 0 || snapshot.TargetUID <= 0 {
		return false, 0, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	place, exists := s.pearlPlaces[snapshot.PlaceID]
	if !exists || place == nil || place.UTimeMs == snapshot.PreviousPlaceUTimeMs || !place.HireFailCntObserved {
		return false, 0, false
	}
	if place.HireFailCnt > 0 {
		return false, place.HireFailCnt, true
	}
	if place.LaborUID != snapshot.TargetUID || place.LaborEndTime <= snapshot.At.UnixMilli() ||
		s.inventory[pearlHireTicketItemID] != snapshot.TicketCount-1 {
		return false, 0, false
	}
	return true, 0, true
}

// PearlHireTicketDecreased reports only the exact observed one-ticket spend
// associated with an attempt snapshot. It is independent of slot outcome so
// contested attempts that consume a ticket still count toward the daily cap.
func (s *State) PearlHireTicketDecreased(snapshot PearlHireAttemptSnapshot) bool {
	if snapshot.At.IsZero() || snapshot.TicketCount <= 0 {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.inventory[pearlHireTicketItemID] == snapshot.TicketCount-1
}

func (s *State) pearlFriendUIDsLocked() []int64 {
	seen := make(map[int64]struct{}, len(s.pearlFriendOrder))
	out := make([]int64, 0, len(s.pearlFriendOrder))
	for _, key := range s.pearlFriendOrder {
		relation, ok := s.pearlFriendRelations[key]
		if !ok {
			continue
		}
		uid := relation.UID0
		if relation.UID0 != s.roleID && relation.UID1 != s.roleID {
			continue
		}
		if uid == s.roleID && relation.UID1 != s.roleID {
			uid = relation.UID1
		} else if relation.UID1 == s.roleID && relation.UID0 != s.roleID {
			uid = relation.UID0
		}
		if uid <= 0 || uid == s.roleID {
			continue
		}
		if _, exists := seen[uid]; exists {
			continue
		}
		seen[uid] = struct{}{}
		out = append(out, uid)
	}
	return out
}

// MarkPearlHireFailed protects a contested UID from being retried for the
// observed client cooldown window. Calls at the exact expiry instant are
// eligible again because planners compare with time.Time.After.
func (s *State) MarkPearlHireFailed(uid int64, at time.Time) {
	if uid <= 0 || at.IsZero() {
		return
	}
	s.mu.Lock()
	if s.pearlHireFailedUntil == nil {
		s.pearlHireFailedUntil = make(map[int64]int64)
	}
	s.pearlHireFailedUntil[uid] = at.Add(time.Minute).UnixMilli()
	delete(s.pearlHireStates, uid)
	delete(s.pearlProfiles, uid)
	s.mu.Unlock()
}

// SkipPearlHireCandidate excludes one candidate for the remainder of the
// current connection session. The official client treats pearlPlace.hire's
// gold alternative as a candidate-level result and returns to candidate
// selection without issuing a second payment request, even when the rejected
// attempt has already consumed its submitted hire ticket.
func (s *State) SkipPearlHireCandidate(uid int64) {
	if uid <= 0 {
		return
	}
	s.mu.Lock()
	if s.pearlHireSkippedUIDs == nil {
		s.pearlHireSkippedUIDs = make(map[int64]struct{})
	}
	s.pearlHireSkippedUIDs[uid] = struct{}{}
	s.mu.Unlock()
}

// LockPearlHireSession disables every later automatic hire in this connection
// session after an ambiguous response makes another ticket spend unsafe.
func (s *State) LockPearlHireSession(reason string) {
	s.mu.Lock()
	s.pearlHireSessionLocked = true
	s.pearlHireLockReason = strings.TrimSpace(reason)
	if s.pearlHireLockReason == "" {
		s.pearlHireLockReason = "珍珠雇佣结果不明确，当前会话已停止自动雇佣"
	}
	s.mu.Unlock()
}

// ResetPearlHireSession drops all short-lived candidate state, failure
// cooldowns, candidate skips, and safety locks when a genuinely new session
// starts.
func (s *State) ResetPearlHireSession() {
	s.mu.Lock()
	s.pearlFriendRelations = make(map[string]pearlFriendRelation)
	s.pearlFriendOrder = nil
	s.pearlFriendsObserved = false
	s.pearlProfiles = make(map[int64]*PearlCandidateProfile)
	s.pearlHireStates = make(map[int64]*PearlCandidateHireState)
	s.pearlRecommendUIDs = nil
	s.pearlRecommendAtMs = 0
	s.pearlRecommendObserved = false
	s.pearlEnemies = make(map[int64]int64)
	s.pearlEnemiesObserved = false
	s.pearlHireFailedUntil = make(map[int64]int64)
	s.pearlHireSkippedUIDs = make(map[int64]struct{})
	s.pearlHireSessionLocked = false
	s.pearlHireLockReason = ""
	s.mu.Unlock()
}

func pearlRelationKey(uid0, uid1 int64) string {
	if uid1 < uid0 {
		uid0, uid1 = uid1, uid0
	}
	return strconv.FormatInt(uid0, 10) + ":" + strconv.FormatInt(uid1, 10)
}

func parseExactPositiveInt64(value string) (int64, bool) {
	if value == "" || strings.TrimSpace(value) != value {
		return 0, false
	}
	n, err := strconv.ParseInt(value, 10, 64)
	return n, err == nil && n > 0
}

func readExactInt32Raw(raw json.RawMessage) (int32, bool) {
	n, ok := readExactInt64Raw(raw)
	if !ok || n < -1<<31 || n > 1<<31-1 {
		return 0, false
	}
	return int32(n), true
}

func readExactInt64Raw(raw json.RawMessage) (int64, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0, false
	}
	if raw[0] == '"' {
		var value string
		if json.Unmarshal(raw, &value) != nil || value == "" || strings.TrimSpace(value) != value {
			return 0, false
		}
		n, err := strconv.ParseInt(value, 10, 64)
		return n, err == nil
	}
	var value json.Number
	if json.Unmarshal(raw, &value) != nil {
		return 0, false
	}
	n, err := strconv.ParseInt(value.String(), 10, 64)
	return n, err == nil
}
