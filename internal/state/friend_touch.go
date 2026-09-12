package state

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const friendCoinItemID int32 = 1305

// FriendStealMaxExtra is the policy/UI ceiling for friendship-coin buys
// (extra pick attempts) per friend. Catalog $pickMax is only the default when
// max_buy_per_friend is 0; settings may raise extras up to this value.
const FriendStealMaxExtra int32 = 75

// FriendCoinItemID is the friendship coin used to buy extra pick quota.
func FriendCoinItemID() int32 { return friendCoinItemID }

// FriendOtherInfoView is namespace 110.1 frdOtherInfoMap entry.
type FriendOtherInfoView struct {
	IsSteal    bool  `json:"is_steal,omitempty"`
	IsAid      bool  `json:"is_aid,omitempty"`
	ObservedAt int64 `json:"observed_at_ms,omitempty"`
}

// FriendTouchConfig is parsed from catalog c_frd defaults.
type FriendTouchConfig struct {
	StealMax    int32
	PickMax     int32
	PickAddCost int32
}

// ResolveFriendStealMaxBuy returns the allowed extra (bought) attempts per friend.
// Policy 0 keeps the catalog $pickMax default; explicit values may exceed the
// catalog up to FriendStealMaxExtra.
func ResolveFriendStealMaxBuy(maxBuyPerFriend int32, cfg FriendTouchConfig) int32 {
	if maxBuyPerFriend <= 0 {
		if cfg.PickMax > 0 && cfg.PickMax <= FriendStealMaxExtra {
			return cfg.PickMax
		}
		if cfg.PickMax > FriendStealMaxExtra {
			return FriendStealMaxExtra
		}
		return 10
	}
	if maxBuyPerFriend > FriendStealMaxExtra {
		return FriendStealMaxExtra
	}
	return maxBuyPerFriend
}

// FriendStealMaxTarget is free daily attempts plus the max allowed extras.
func FriendStealMaxTarget(cfg FriendTouchConfig, maxBuyPerFriend int32) int32 {
	return cfg.StealMax + ResolveFriendStealMaxBuy(maxBuyPerFriend, cfg)
}

// FriendTouchFriendView is one friend row for policy UI and planners.
type FriendTouchFriendView struct {
	UID                  int64  `json:"uid"`
	Name                 string `json:"name,omitempty"`
	StolenCount          int32  `json:"stolen_count,omitempty"`
	StealMax             int32  `json:"steal_max,omitempty"`
	StealLeft            int32  `json:"steal_left,omitempty"`
	CanSteal             bool   `json:"can_steal,omitempty"`
	ProfileObserved      bool   `json:"profile_observed,omitempty"`
	BaseStealMax         int32  `json:"base_steal_max,omitempty"`
	BoughtCount          int32  `json:"bought_count,omitempty"`
	QuotaObserved        bool   `json:"quota_observed,omitempty"`
	AvailabilityObserved bool   `json:"availability_observed,omitempty"`
}

// FriendTouchView is the session-scoped friend-touch state snapshot.
type FriendTouchView struct {
	StealObserved       bool
	StealRTimeMs        int64
	StealMap            map[int64]int32
	StealCntBuyMap      map[int64]int32
	StealCntBuyRTimeMs  int64
	StealCntBuyObserved bool
	OtherInfo           map[int64]FriendOtherInfoView
	OtherInfoObserved   bool
	FriendUIDs          []int64
	FriendsObserved     bool
	Profiles            map[int64]PearlCandidateProfile
	VisitUID            int64
	VisitObservedAtMs   int64
	VisitLands          map[int32]LandView
}

func FriendTouchConfigFromCatalog() (FriendTouchConfig, bool) {
	raw, ok := StaticRow("c_frd", -1)
	if !ok {
		return FriendTouchConfig{}, false
	}
	var base struct {
		StealMax    int32 `json:"$stealMax"`
		PickMax     int32 `json:"$pickMax"`
		PickAddCost int32 `json:"$pickAddCost"`
	}
	if json.Unmarshal(raw, &base) != nil || base.StealMax <= 0 {
		return FriendTouchConfig{}, false
	}
	if base.PickMax <= 0 {
		base.PickMax = 10
	}
	if base.PickAddCost <= 0 {
		base.PickAddCost = 1
	}
	return FriendTouchConfig{
		StealMax:    base.StealMax,
		PickMax:     base.PickMax,
		PickAddCost: base.PickAddCost,
	}, true
}

func (s *State) applyFrdStealLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return
	}
	if rawSteal, ok := fields["0"]; ok {
		s.applyFrdStealObjectLocked(rawSteal)
	}
	if rawUsrLand, ok := fields["1"]; ok {
		s.applyFrdVisitUsrLandLocked(rawUsrLand)
	}
	if rawChg, ok := fields["2"]; ok {
		s.applyFrdVisitChgLandLocked(rawChg)
	}
	if rawRcd, ok := fields["3"]; ok {
		s.applyFrdStealRcdListLocked(rawRcd)
	}
}

// applyFrdHomeLocked applies namespace 133 (IFrdHomeTot). Client loads a
// friend's garden via frdHome.getFrdHomeInfo {frdUid}; field 0 is IUsrLand.
func (s *State) applyFrdHomeLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return
	}
	if rawUsrLand, ok := fields["0"]; ok {
		s.applyFrdVisitUsrLandLocked(rawUsrLand)
	}
}

func (s *State) applyFrdExtTotLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return
	}
	if rawOther, ok := fields["1"]; ok {
		s.applyFrdExtOtherInfoLocked(rawOther)
	}
}

func (s *State) applyFrdStealObjectLocked(raw json.RawMessage) {
	if isJSONNull(raw) {
		s.frdStealObserved = false
		s.frdStealRTimeMs = 0
		s.frdStealMap = nil
		s.frdStealElvesCnt = 0
		s.frdStealHasUnReadRcd = false
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return
	}
	s.frdStealObserved = true
	if rawRTime, ok := fields["3"]; ok {
		if ms, ok := readExactInt64Raw(rawRTime); ok {
			s.frdStealRTimeMs = ms
		}
	}
	if rawMap, ok := fields["1"]; ok {
		if isJSONNull(rawMap) {
			s.frdStealMap = nil
		} else {
			parsed := parseInt64Int32Map(rawMap)
			if parsed != nil {
				// IFrdSteal.stealMap is an authoritative current-day map when
				// present. Replacement prevents yesterday's omitted UIDs from
				// surviving a reset as non-zero usage.
				s.frdStealMap = parsed
			}
		}
	}
	if rawUnread, ok := fields["6"]; ok {
		if v, ok := readExactInt32Raw(rawUnread); ok {
			s.frdStealHasUnReadRcd = v != 0
		}
	}
	if rawElvesCnt, ok := fields["7"]; ok {
		if v, ok := readExactInt32Raw(rawElvesCnt); ok {
			s.frdStealElvesCnt = v
		}
	}
}

func (s *State) applyFrdStealRcdListLocked(raw json.RawMessage) {
	if isJSONNull(raw) {
		s.frdStealRcdList = nil
		s.frdStealRcdObserved = true
		return
	}
	var asArray []json.RawMessage
	if json.Unmarshal(raw, &asArray) != nil {
		return
	}
	out := make([]FrdStealRcdView, 0, len(asArray))
	for _, rawRcd := range asArray {
		var fields map[string]json.RawMessage
		if json.Unmarshal(rawRcd, &fields) != nil {
			continue
		}
		view := FrdStealRcdView{}
		if uid, ok := readExactInt64Raw(fields["2"]); ok {
			view.FrdUID = uid
		}
		if ms, ok := readExactInt64Raw(fields["5"]); ok {
			view.CTimeMs = ms
		}
		if rawMap, ok := fields["3"]; ok && !isJSONNull(rawMap) {
			view.StealMap = parseInt32Int32Map(rawMap)
		}
		out = append(out, view)
	}
	s.frdStealRcdList = out
	s.frdStealRcdObserved = true
	s.frdStealHasUnReadRcd = false
}

func (s *State) applyFrdVisitUsrLandLocked(raw json.RawMessage) {
	if isJSONNull(raw) {
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return
	}
	uid, ok := readExactInt64Raw(fields["0"])
	if !ok || uid <= 0 {
		return
	}
	rawMap, hasMap := fields["1"]
	if !hasMap || isJSONNull(rawMap) {
		s.frdVisitUID = uid
		s.frdVisitAtMs = s.lastApplyMs
		s.frdVisitLands = map[int32]LandView{}
		return
	}
	parsed := parseFriendLandMap(rawMap)
	if parsed == nil {
		return
	}
	s.frdVisitUID = uid
	s.frdVisitAtMs = s.lastApplyMs
	s.frdVisitLands = parsed
}

func (s *State) applyFrdVisitChgLandLocked(raw json.RawMessage) {
	if isJSONNull(raw) || s.frdVisitUID <= 0 {
		return
	}
	parsed := parseFriendLandMap(raw)
	if parsed == nil {
		return
	}
	if s.frdVisitLands == nil {
		s.frdVisitLands = make(map[int32]LandView)
	}
	for landID, land := range parsed {
		if land.FlowerID == 0 && land.State == 0 {
			delete(s.frdVisitLands, landID)
			continue
		}
		s.frdVisitLands[landID] = land
	}
	s.frdVisitAtMs = s.lastApplyMs
}

func parseFriendLandMap(raw json.RawMessage) map[int32]LandView {
	var asObject map[string]json.RawMessage
	if json.Unmarshal(raw, &asObject) != nil {
		return nil
	}
	out := make(map[int32]LandView, len(asObject))
	for key, rawLand := range asObject {
		landID64, err := parseInt64Key(key)
		if err != nil || landID64 <= 0 || landID64 > int64(^uint32(0)>>1) {
			continue
		}
		if isJSONNull(rawLand) {
			continue
		}
		var landFields map[string]any
		if json.Unmarshal(rawLand, &landFields) != nil {
			continue
		}
		out[int32(landID64)] = FromPrimary(landFields)
	}
	return out
}

func (s *State) applyFrdExtOtherInfoLocked(raw json.RawMessage) {
	if isJSONNull(raw) {
		return
	}
	parsed := make(map[int64]FriendOtherInfoView)
	var asObject map[string]json.RawMessage
	if json.Unmarshal(raw, &asObject) != nil {
		return
	}
	for key, rawInfo := range asObject {
		uid, err := parseInt64Key(key)
		if err != nil || uid <= 0 {
			continue
		}
		var infoFields map[string]json.RawMessage
		if json.Unmarshal(rawInfo, &infoFields) != nil {
			continue
		}
		view := FriendOtherInfoView{ObservedAt: s.lastApplyMs}
		if rawSteal, ok := infoFields["0"]; ok {
			if v, ok := readExactInt32Raw(rawSteal); ok {
				view.IsSteal = v != 0
			}
		}
		if rawAid, ok := infoFields["1"]; ok {
			if v, ok := readInt64Raw(rawAid); ok {
				view.IsAid = v != 0
			} else if v, ok := readExactInt32Raw(rawAid); ok {
				view.IsAid = v != 0
			}
		}
		parsed[uid] = view
	}
	if len(parsed) == 0 {
		return
	}
	if s.frdOtherInfo == nil {
		s.frdOtherInfo = make(map[int64]FriendOtherInfoView)
	}
	for uid, view := range parsed {
		s.frdOtherInfo[uid] = view
	}
	s.frdOtherInfoObserved = true
}

func (s *State) applyFrdStealCntBuyLocked(raw json.RawMessage) {
	if isJSONNull(raw) {
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return
	}
	// This function is called only for a full namespace-24 base object. An
	// omitted/null field 104 therefore means an observed empty purchase map,
	// not unknown state.
	s.frdStealCntBuyObserved = true
	s.frdStealCntBuyMap = nil
	if rawRTime, ok := fields["9"]; ok {
		if ms, ok := readExactInt64Raw(rawRTime); ok {
			s.frdStealCntBuyRTimeMs = ms
		}
	}
	if rawMap, ok := fields["104"]; ok {
		if !isJSONNull(rawMap) {
			if parsed := parseInt64Int32Map(rawMap); parsed != nil {
				s.frdStealCntBuyMap = parsed
			}
		}
	}
}

func parseInt64Int32Map(raw json.RawMessage) map[int64]int32 {
	var asObject map[string]json.RawMessage
	if json.Unmarshal(raw, &asObject) != nil {
		return nil
	}
	out := make(map[int64]int32, len(asObject))
	for key, rawValue := range asObject {
		uid, err := parseInt64Key(key)
		if err != nil || uid <= 0 {
			continue
		}
		count, ok := readExactInt32Raw(rawValue)
		if !ok || count < 0 {
			continue
		}
		out[uid] = count
	}
	return out
}

func parseInt32Int32Map(raw json.RawMessage) map[int32]int32 {
	var asObject map[string]json.RawMessage
	if json.Unmarshal(raw, &asObject) != nil {
		return nil
	}
	out := make(map[int32]int32, len(asObject))
	for key, rawValue := range asObject {
		id64, err := parseInt64Key(key)
		if err != nil || id64 <= 0 || id64 > int64(^uint32(0)>>1) {
			continue
		}
		count, ok := readExactInt32Raw(rawValue)
		if !ok || count < 0 {
			continue
		}
		out[int32(id64)] = count
	}
	return out
}

func parseInt64Key(key string) (int64, error) {
	return strconv.ParseInt(key, 10, 64)
}

func frdStealMapFresh(rTimeMs int64, now time.Time) bool {
	if rTimeMs <= 0 {
		return false
	}
	return calendarDayID(now) == calendarDayID(time.UnixMilli(rTimeMs))
}

// FriendTouch returns a defensive snapshot for planners and query APIs.
func (s *State) FriendTouch(now time.Time) FriendTouchView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	view := FriendTouchView{
		StealObserved:       s.frdStealObserved,
		StealRTimeMs:        s.frdStealRTimeMs,
		StealCntBuyRTimeMs:  s.frdStealCntBuyRTimeMs,
		StealCntBuyObserved: s.frdStealCntBuyObserved,
		OtherInfoObserved:   s.frdOtherInfoObserved,
		FriendsObserved:     s.pearlFriendsObserved,
		FriendUIDs:          s.pearlFriendUIDsLocked(),
		VisitUID:            s.frdVisitUID,
		VisitObservedAtMs:   s.frdVisitAtMs,
	}
	if frdStealMapFresh(s.frdStealRTimeMs, now) && len(s.frdStealMap) > 0 {
		view.StealMap = cloneInt64Int32Map(s.frdStealMap)
	}
	if frdStealMapFresh(s.frdStealCntBuyRTimeMs, now) && len(s.frdStealCntBuyMap) > 0 {
		view.StealCntBuyMap = cloneInt64Int32Map(s.frdStealCntBuyMap)
	}
	if len(s.frdOtherInfo) > 0 {
		view.OtherInfo = make(map[int64]FriendOtherInfoView, len(s.frdOtherInfo))
		for uid, info := range s.frdOtherInfo {
			view.OtherInfo[uid] = info
		}
	}
	if len(s.pearlProfiles) > 0 {
		view.Profiles = make(map[int64]PearlCandidateProfile, len(s.pearlProfiles))
		for uid, profile := range s.pearlProfiles {
			if profile != nil {
				view.Profiles[uid] = *profile
			}
		}
	}
	if len(s.frdVisitLands) > 0 {
		view.VisitLands = make(map[int32]LandView, len(s.frdVisitLands))
		for landID, land := range s.frdVisitLands {
			view.VisitLands[landID] = land
		}
	}
	return view
}

// ReadyFriendStealLandID picks one stealable plot on the currently opened friend garden.
// New policy-aware code should use PickFriendStealLandIDWithSelection.
func ReadyFriendStealLandID(lands map[int32]LandView, now time.Time) (int32, bool) {
	return PickFriendStealLandID(lands, nil, 0, now)
}

// PickFriendStealLandID prefers higher flower quality, then lower inventory stock,
// then lower land level, then lower land id. Lands already stolen by selfUID are skipped.
func PickFriendStealLandID(lands map[int32]LandView, inventory map[int32]int32, selfUID int64, now time.Time) (int32, bool) {
	return PickFriendStealLandIDWithSelection(lands, inventory, selfUID, now, FriendStealSelection{})
}

// FriendStealSelection is a catalog-level flower filter. At most one allow
// list is populated by policy normalization; the exclude list is independent.
type FriendStealSelection struct {
	Qualities        []int32
	FlowerIDs        []int32
	ExcludeFlowerIDs []int32
}

// PickFriendStealLandIDWithSelection applies policy flower filters before the
// deterministic quality/stock ordering used by PickFriendStealLandID.
func PickFriendStealLandIDWithSelection(lands map[int32]LandView, inventory map[int32]int32, selfUID int64, now time.Time, selection FriendStealSelection) (int32, bool) {
	if len(lands) == 0 {
		return 0, false
	}
	qualities := positiveInt32Set(selection.Qualities)
	flowerIDs := positiveInt32Set(selection.FlowerIDs)
	excludedFlowerIDs := positiveInt32Set(selection.ExcludeFlowerIDs)
	nowMs := now.UnixMilli()
	type candidate struct {
		landID   int32
		quality  int32
		stock    int32
		lvl      int
		flowerID int32
	}
	candidates := make([]candidate, 0, len(lands))
	for landID, land := range lands {
		if !friendLandStealable(land, selfUID, nowMs) {
			continue
		}
		flowerID := int32(land.FlowerID)
		quality := friendFlowerQuality(flowerID)
		if len(qualities) > 0 && !qualities[quality] {
			continue
		}
		if len(flowerIDs) > 0 && !flowerIDs[flowerID] {
			continue
		}
		if excludedFlowerIDs[flowerID] {
			continue
		}
		stock := int32(0)
		if inventory != nil {
			stock = inventory[flowerID]
		}
		candidates = append(candidates, candidate{
			landID:   landID,
			quality:  quality,
			stock:    stock,
			lvl:      land.Lvl,
			flowerID: flowerID,
		})
	}
	if len(candidates) == 0 {
		return 0, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		switch {
		case a.quality != b.quality:
			return a.quality > b.quality
		case a.stock != b.stock:
			return a.stock < b.stock
		case a.lvl != b.lvl:
			return a.lvl < b.lvl
		case a.flowerID != b.flowerID:
			return a.flowerID < b.flowerID
		default:
			return a.landID < b.landID
		}
	})
	return candidates[0].landID, true
}

func positiveInt32Set(values []int32) map[int32]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[int32]bool, len(values))
	for _, value := range values {
		if value > 0 {
			out[value] = true
		}
	}
	return out
}

func friendLandStealable(land LandView, selfUID int64, nowMs int64) bool {
	if !land.IsPlanted() {
		return false
	}
	if selfUID > 0 && int64SliceContains(land.StealUIDs, selfUID) {
		return false
	}
	switch land.State {
	case 3:
		return true
	case 2:
		return land.NextTimeMs > 0 && land.NextTimeMs <= nowMs
	default:
		return false
	}
}

func friendFlowerQuality(flowerID int32) int32 {
	item, ok := ItemInfoByID(flowerID)
	if !ok {
		return 0
	}
	return item.Color
}

func int64SliceContains(values []int64, target int64) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

// MarkFriendTouchSkipEnter suppresses enterFrdSteal for a friend until cooldown elapses.
func (s *State) MarkFriendTouchSkipEnter(uid int64, until time.Time) {
	if s == nil || uid <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.frdStealSkipEnterUntil == nil {
		s.frdStealSkipEnterUntil = make(map[int64]int64)
	}
	s.frdStealSkipEnterUntil[uid] = until.UnixMilli()
}

// FriendTouchSkipEnter reports whether enterFrdSteal should be deferred for uid.
func (s *State) FriendTouchSkipEnter(uid int64, now time.Time) bool {
	if s == nil || uid <= 0 {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	untilMs, ok := s.frdStealSkipEnterUntil[uid]
	return ok && untilMs > now.UnixMilli()
}

// ClearFriendTouchSkipEnter removes an enter cooldown after a successful steal.
func (s *State) ClearFriendTouchSkipEnter(uid int64) {
	if s == nil || uid <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.frdStealSkipEnterUntil, uid)
}

// FriendStealCounters returns current-day counters and independent observation
// flags for the used-attempt and bought-attempt maps.
func (s *State) FriendStealCounters(uid int64, now time.Time) (used, bought int32, usedObserved, boughtObserved bool) {
	if s == nil || uid <= 0 {
		return 0, 0, false, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	usedObserved = s.frdStealObserved && frdStealMapFresh(s.frdStealRTimeMs, now)
	boughtObserved = s.frdStealCntBuyObserved && frdStealMapFresh(s.frdStealCntBuyRTimeMs, now)
	if usedObserved {
		used = s.frdStealMap[uid]
	}
	if boughtObserved {
		bought = s.frdStealCntBuyMap[uid]
	}
	return used, bought, usedObserved, boughtObserved
}

// NoteFriendStealSuccess reconciles successful responses that omit their
// namespace-111 delta. It never lowers an authoritative counter.
func (s *State) NoteFriendStealSuccess(uid int64, landID int32, usedBefore int32, usedBeforeObserved bool, now time.Time) {
	if s == nil || uid <= 0 || landID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.noteFriendStealUsedLocked(uid, usedBefore, usedBeforeObserved, now)
	if s.frdVisitUID != uid {
		return
	}
	land, exists := s.frdVisitLands[landID]
	if !exists || s.roleID <= 0 || int64SliceContains(land.StealUIDs, s.roleID) {
		return
	}
	land.StealUIDs = append(land.StealUIDs, s.roleID)
	s.frdVisitLands[landID] = land
}

// NoteFriendStealElvesSuccess reconciles a stealElves=1 success that may omit
// IFrdSteal.stealElvesCnt / land.elvesStealUids. Client still consumes the
// per-friend flower-steal quota (getStealCntLeftNumByFrdUid).
func (s *State) NoteFriendStealElvesSuccess(uid int64, landID int32, usedBefore int32, usedBeforeObserved bool, elvesCntBefore int32, elvesCntBeforeObserved bool, now time.Time) {
	if s == nil || uid <= 0 || landID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.noteFriendStealUsedLocked(uid, usedBefore, usedBeforeObserved, now)
	if elvesCntBeforeObserved && s.frdStealObserved && frdStealMapFresh(s.frdStealRTimeMs, now) {
		if minimum := elvesCntBefore + 1; s.frdStealElvesCnt < minimum {
			s.frdStealElvesCnt = minimum
		}
	}
	if s.frdVisitUID != uid {
		return
	}
	land, exists := s.frdVisitLands[landID]
	if !exists || s.roleID <= 0 || int64SliceContains(land.ElvesStealUIDs, s.roleID) {
		return
	}
	land.ElvesStealUIDs = append(append([]int64(nil), land.ElvesStealUIDs...), s.roleID)
	s.frdVisitLands[landID] = land
}

// MarkFriendStealElvesLandUnavailable sticky-skips a friend land for elf steals
// until that plot's plantTime advances (replant). frdHome refreshes that still
// show elvesId with empty elvesStealUids must not revive a server-rejected plot.
func (s *State) MarkFriendStealElvesLandUnavailable(friendUID int64, landID int32) {
	if s == nil || friendUID <= 0 || landID <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	plantTime := int64(0)
	if s.frdVisitUID == friendUID {
		if land, ok := s.frdVisitLands[landID]; ok {
			plantTime = land.PlantTimeMs
			// Also hide it on the current visit snapshot so the next plan tick
			// can advance without waiting for another enter.
			if s.roleID > 0 && !int64SliceContains(land.ElvesStealUIDs, s.roleID) {
				land.ElvesStealUIDs = append(append([]int64(nil), land.ElvesStealUIDs...), s.roleID)
				s.frdVisitLands[landID] = land
			} else if s.roleID <= 0 {
				land.ElvesID = 0
				s.frdVisitLands[landID] = land
			}
		}
	}
	if s.frdStealElvesSkipPlantTime == nil {
		s.frdStealElvesSkipPlantTime = make(map[int64]map[int32]int64)
	}
	byLand := s.frdStealElvesSkipPlantTime[friendUID]
	if byLand == nil {
		byLand = make(map[int32]int64)
		s.frdStealElvesSkipPlantTime[friendUID] = byLand
	}
	byLand[landID] = plantTime
}

// FriendStealElvesLandSkipped reports a sticky elf-steal skip for friend+land
// that still matches the observed plantTime (0 matches an unknown plantTime).
func (s *State) FriendStealElvesLandSkipped(friendUID int64, landID int32, plantTimeMs int64) bool {
	if s == nil || friendUID <= 0 || landID <= 0 {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	byLand := s.frdStealElvesSkipPlantTime[friendUID]
	if byLand == nil {
		return false
	}
	skippedAt, ok := byLand[landID]
	if !ok {
		return false
	}
	return skippedAt == plantTimeMs
}

// FriendStealElvesActuallyStolen reports whether a stealElves=1 response landed
// an elf (cnt bump, elvesStealUids, or elves inventory gain) rather than an
// ordinary flower steal that still returned ok.
func (s *State) FriendStealElvesActuallyStolen(friendUID int64, landID, elvesItemID, elvesCntBefore int32, elvesCntBeforeObserved bool, inventoryBefore int32, now time.Time) bool {
	if s == nil {
		return false
	}
	if elvesCntBeforeObserved && s.StealElvesCntAt(now) > elvesCntBefore {
		return true
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.frdVisitUID == friendUID {
		if land, ok := s.frdVisitLands[landID]; ok {
			if s.roleID > 0 && int64SliceContains(land.ElvesStealUIDs, s.roleID) {
				return true
			}
			if len(land.ElvesStealUIDs) > 0 && land.ElvesID == 0 {
				return true
			}
		}
	}
	if elvesItemID > 0 && s.inventory[elvesItemID] > inventoryBefore {
		return true
	}
	return false
}

// NoteFriendStealUsed reconciles per-friend flower-steal quota after a steal
// that consumed an attempt without confirming an elf gain (false flower steal).
func (s *State) NoteFriendStealUsed(uid int64, usedBefore int32, usedBeforeObserved bool, now time.Time) {
	if s == nil || uid <= 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.noteFriendStealUsedLocked(uid, usedBefore, usedBeforeObserved, now)
}

func (s *State) noteFriendStealUsedLocked(uid int64, usedBefore int32, usedBeforeObserved bool, now time.Time) {
	if !usedBeforeObserved || !s.frdStealObserved || !frdStealMapFresh(s.frdStealRTimeMs, now) {
		return
	}
	if s.frdStealMap == nil {
		s.frdStealMap = make(map[int64]int32)
	}
	if minimum := usedBefore + 1; s.frdStealMap[uid] < minimum {
		s.frdStealMap[uid] = minimum
	}
}

// NoteFriendStealPurchase reconciles a successful purchase response that omits
// namespace-24 field 104, preventing a duplicate friendship-coin spend.
func (s *State) NoteFriendStealPurchase(uid int64, boughtBefore int32, boughtBeforeObserved bool, now time.Time) {
	if s == nil || uid <= 0 || !boughtBeforeObserved {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.frdStealCntBuyObserved || !frdStealMapFresh(s.frdStealCntBuyRTimeMs, now) {
		return
	}
	if s.frdStealCntBuyMap == nil {
		s.frdStealCntBuyMap = make(map[int64]int32)
	}
	if minimum := boughtBefore + 1; s.frdStealCntBuyMap[uid] < minimum {
		s.frdStealCntBuyMap[uid] = minimum
	}
}

// FriendZoneFromUID extracts the zone id embedded in a role uid (uid % 100000).
// Observed labels use the same trailing zone: s2489 / s3424 / s2297.
func FriendZoneFromUID(uid int64) int32 {
	if uid <= 0 {
		return 0
	}
	return int32(uid % 100000)
}

// FriendDisplayName prefers the synced oppt name; otherwise falls back to s{zone}.
func FriendDisplayName(uid int64, name string) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	if zone := FriendZoneFromUID(uid); zone > 0 {
		return fmt.Sprintf("s%d", zone)
	}
	return ""
}

// FriendTouchFriends builds UI rows for all known friends.
func (s *State) FriendTouchFriends(now time.Time) []FriendTouchFriendView {
	cfg, ok := FriendTouchConfigFromCatalog()
	if !ok {
		cfg = FriendTouchConfig{StealMax: 10, PickMax: 10, PickAddCost: 1}
	}
	view := s.FriendTouch(now)
	out := make([]FriendTouchFriendView, 0, len(view.FriendUIDs))
	for _, uid := range view.FriendUIDs {
		profile := view.Profiles[uid]
		row := FriendTouchFriendView{
			UID:             uid,
			Name:            FriendDisplayName(uid, profile.Name),
			ProfileObserved: profile.ObservedAtMs > 0 && strings.TrimSpace(profile.Name) != "",
			StolenCount:     view.StealMap[uid],
			BaseStealMax:    cfg.StealMax,
			BoughtCount:     view.StealCntBuyMap[uid],
			StealMax:        cfg.StealMax + view.StealCntBuyMap[uid],
			StealLeft:       0,
			QuotaObserved:   friendTouchQuotaFresh(view, now),
		}
		if !row.QuotaObserved {
			row.StolenCount = 0
			row.StealMax = cfg.StealMax
			row.BoughtCount = 0
		}
		row.StealLeft = row.StealMax - row.StolenCount
		if row.StealLeft < 0 {
			row.StealLeft = 0
		}
		if info, exists := view.OtherInfo[uid]; exists {
			row.AvailabilityObserved = info.ObservedAt > 0 && now.UnixMilli()-info.ObservedAt <= (5*time.Minute).Milliseconds()
			row.CanSteal = row.QuotaObserved && row.AvailabilityObserved && info.IsSteal && row.StealLeft > 0
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		ni, nj := out[i].Name, out[j].Name
		if ni == "" {
			ni = "\xff"
		}
		if nj == "" {
			nj = "\xff"
		}
		if ni != nj {
			return ni < nj
		}
		return out[i].UID < out[j].UID
	})
	return out
}

func friendTouchQuotaFresh(view FriendTouchView, now time.Time) bool {
	return view.StealObserved && view.StealCntBuyObserved &&
		frdStealMapFresh(view.StealRTimeMs, now) && frdStealMapFresh(view.StealCntBuyRTimeMs, now)
}

func cloneInt64Int32Map(in map[int64]int32) map[int64]int32 {
	if len(in) == 0 {
		return nil
	}
	out := make(map[int64]int32, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
