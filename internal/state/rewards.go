package state

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"time"
)

func (s *State) applyCollectRewardsLocked(raw json.RawMessage) {
	var ns103 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns103); err != nil {
		return
	}
	raw0, ok := ns103["0"]
	if !ok {
		return
	}
	var rewards map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &rewards); err != nil {
		return
	}
	for typeStr, rawReward := range rewards {
		typeID := atoi32(typeStr)
		if typeID == 0 {
			continue
		}
		if isJSONNull(rawReward) {
			delete(s.collectRewards, typeID)
			continue
		}
		view := s.collectRewards[typeID]
		if view == nil {
			view = &CollectRewardView{Type: typeID}
			s.collectRewards[typeID] = view
		}
		// collectRwd responses are sparse updateMbMap deltas. Merge only the
		// fields carried by this response so a catalog-reward claim cannot erase
		// type 13's artCreateRwdList (field 7) from the login snapshot.
		if len(rawReward) > 0 && string(rawReward) != "{}" {
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(rawReward, &fields); err == nil {
				if n, ok := readInt32JSONField(fields, "1"); ok && n > 0 {
					view.Type = n
				}
				if n, ok := readInt32JSONField(fields, "2"); ok {
					view.Lvl = n
				}
				if n, ok := readInt32JSONField(fields, "3"); ok {
					view.Exp = n
				}
				if rawRecv, ok := fields["4"]; ok {
					view.RecvIDs = readInt32ListRaw(rawRecv)
				}
				if n, ok := readInt64JSONField(fields, "5"); ok {
					view.UTimeMs = n
				}
				if n, ok := readInt64JSONField(fields, "6"); ok {
					view.CTimeMs = n
				}
				if rawArtCreate, ok := fields["7"]; ok {
					view.ArtCreateRewardIDs = readInt32ListRaw(rawArtCreate)
				}
			}
		}
		if view.Type != typeID {
			delete(s.collectRewards, typeID)
			s.collectRewards[view.Type] = view
		}
	}
	s.collectRewardObserved = true
}

func (s *State) applyShopCultivateLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if s.shopCultivateCosts == nil {
		s.shopCultivateCosts = make(map[int32]ItemCount)
	}
	if s.shopCultivateBought == nil {
		s.shopCultivateBought = make(map[int32]int32)
	}
	hadResetField := false
	if rawReset, ok := fields["2"]; ok {
		var n int64
		if json.Unmarshal(rawReset, &n) == nil {
			hadResetField = true
			if s.shopCultivateResetMs > 0 && n > 0 &&
				gameDayID(time.UnixMilli(n)) > gameDayID(time.UnixMilli(s.shopCultivateResetMs)) {
				s.shopCultivateBought = make(map[int32]int32)
			}
			s.shopCultivateResetMs = n
		}
	}
	if rawLar, ok := fields["3"]; ok {
		var n int64
		if json.Unmarshal(rawLar, &n) == nil {
			s.shopCultivateLarMs = n
		}
	}
	if rawMr, ok := fields["4"]; ok {
		var n int32
		if json.Unmarshal(rawMr, &n) == nil {
			s.shopCultivateMrCount = n
		}
	}
	fullSnapshot := false
	if rawInfo, ok := fields["1"]; ok {
		fullSnapshot = true
		if isJSONNull(rawInfo) {
			s.shopCultivateCosts = make(map[int32]ItemCount)
		} else {
			var costs map[string]json.RawMessage
			if err := json.Unmarshal(rawInfo, &costs); err == nil {
				next := make(map[int32]ItemCount, len(costs))
				for shopIDStr, rawCost := range costs {
					shopID := atoi32(shopIDStr)
					if shopID == 0 {
						continue
					}
					parts := readInt32OrderedListRaw(rawCost)
					if len(parts) < 2 || parts[0] <= 0 || parts[1] <= 0 {
						continue
					}
					next[shopID] = ItemCount{ItemID: parts[0], Count: parts[1]}
				}
				s.shopCultivateCosts = next
			}
		}
	}
	if rawBought, ok := fields["6"]; ok {
		if isJSONNull(rawBought) || fullSnapshot {
			s.shopCultivateBought = readInt32RawMap(rawBought)
		} else {
			for shopID, count := range readInt32RawMap(rawBought) {
				s.shopCultivateBought[shopID] = count
			}
		}
	}
	// Re-enter / refresh often send a full infoMap + larTime but omit
	// lResetTime. Keep resetMs aligned with the sync marker so
	// ShopCultivateNeedsEnter does not spin on a stale prior-day value.
	if fullSnapshot && !hadResetField {
		syncMs := s.shopCultivateLarMs
		if rawU, ok := fields["7"]; ok {
			var n int64
			if json.Unmarshal(rawU, &n) == nil && n > syncMs {
				syncMs = n
			}
		}
		if syncMs > 0 && (s.shopCultivateResetMs <= 0 ||
			gameDayID(time.UnixMilli(syncMs)) > gameDayID(time.UnixMilli(s.shopCultivateResetMs))) {
			s.shopCultivateResetMs = syncMs
		}
	}
	s.shopCultivateObserved = true
}

func (s *State) applyShopGiftbagLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if rawDRecord, ok := fields["1"]; ok {
		s.shopGiftbagDRecord = readInt32RawMap(rawDRecord)
	}
	if rawWRecord, ok := fields["2"]; ok {
		s.shopGiftbagWRecord = readInt32RawMap(rawWRecord)
	}
	if rawMRecord, ok := fields["3"]; ok {
		s.shopGiftbagMRecord = readInt32RawMap(rawMRecord)
	}
	if rawTRecord, ok := fields["4"]; ok {
		s.shopGiftbagTRecord = readInt32RawMap(rawTRecord)
	}
	if rawBuyTimes, ok := fields["5"]; ok {
		s.shopGiftbagBuyTimeRecord = readInt64RawMap(rawBuyTimes)
	}
	if n, ok := readInt64JSONField(fields, "6"); ok {
		s.shopGiftbagResetMs = n
	}
	if n, ok := readInt64JSONField(fields, "7"); ok {
		s.shopGiftbagUpdatedAtMs = n
	}
	if n, ok := readInt64JSONField(fields, "8"); ok {
		s.shopGiftbagCreatedAtMs = n
	}
	s.shopGiftbagObserved = true
}

func (s *State) applyPearlLocked(raw json.RawMessage) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if s.pearlPlaces == nil {
		s.pearlPlaces = make(map[int32]*PearlPlaceView)
	}
	if rawPlaces, ok := fields["0"]; ok {
		// PearlPlaceMgr ignores a null/falsy placeMap. Only null entries inside
		// an observed map carry deletion semantics.
		if !isJSONNull(rawPlaces) {
			var places map[string]json.RawMessage
			if err := json.Unmarshal(rawPlaces, &places); err == nil {
				for placeIDStr, rawPlace := range places {
					parsedID, err := strconv.ParseInt(placeIDStr, 10, 32)
					if err != nil || parsedID <= 0 {
						continue
					}
					placeID := int32(parsedID)
					if isJSONNull(rawPlace) {
						delete(s.pearlPlaces, placeID)
						continue
					}
					view := s.pearlPlaces[placeID]
					if view == nil {
						view = &PearlPlaceView{PlaceID: placeID}
						s.pearlPlaces[placeID] = view
					}
					applyPearlPlaceFields(view, rawPlace)
				}
			}
		}
	}
	if rawPearl, ok := fields["1"]; ok {
		var pearlFields map[string]json.RawMessage
		if json.Unmarshal(rawPearl, &pearlFields) == nil {
			if uid, valid := readExactInt64Raw(pearlFields["0"]); valid && uid > 0 && (s.roleID == 0 || s.roleID == uid) {
				s.roleID = uid
			}
		}
		applyPearlFields(&s.pearl, rawPearl)
		s.applyPearlEnemiesLocked(rawPearl)
	}
	if rawDraw, ok := fields["2"]; ok {
		s.pearlDrawRaw = cloneRaw(rawDraw)
		s.pearlDrawCount = rawCollectionCount(rawDraw)
	}
	if rawHireStates, ok := fields["5"]; ok {
		s.applyPearlHireStatesLocked(rawHireStates)
	}
	if rawRecommend, ok := fields["6"]; ok {
		s.replacePearlRecommendationsLocked(rawRecommend)
	}
	s.pearlObserved = true
}

func applyPearlFields(view *PearlView, raw json.RawMessage) {
	if view == nil {
		return
	}
	if len(raw) == 0 || isJSONNull(raw) {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	view.Observed = true
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.ProtectState = n
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.ProtectNum = n
	}
	if n, ok := readInt64JSONField(fields, "3"); ok {
		view.OwnerUID = n
	}
	if n, ok := readInt64JSONField(fields, "4"); ok {
		view.LaborEndTime = n
	}
	if n, ok := readInt64JSONField(fields, "6"); ok {
		view.RecvDailyDate = n
	}
	if n, ok := readInt32JSONField(fields, "7"); ok {
		view.HireState = n
	}
	if n, ok := readInt32JSONField(fields, "8"); ok {
		view.SmallDrawCnt = n
	}
	if n, ok := readInt64JSONField(fields, "9"); ok {
		view.UTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "10"); ok {
		view.CTimeMs = n
	}
}

func applyPearlPlaceFields(view *PearlPlaceView, raw json.RawMessage) {
	if view == nil || len(raw) == 0 || string(raw) == "{}" {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return
	}
	if n, ok := readInt32JSONField(fields, "1"); ok && n > 0 {
		view.PlaceID = n
	}
	if rawLaborUID, exists := fields["2"]; exists {
		view.LaborUIDObserved = false
		if isJSONNull(rawLaborUID) {
			view.LaborUID = 0
			view.LaborUIDObserved = true
		} else if n, ok := readExactInt64Raw(rawLaborUID); ok && n >= 0 {
			view.LaborUID = n
			view.LaborUIDObserved = true
		}
	}
	if rawEnd, exists := fields["3"]; exists {
		view.LaborEndTimeObserved = false
		if isJSONNull(rawEnd) {
			view.LaborEndTime = 0
			view.LaborEndTimeObserved = true
		} else {
			if n, ok := readExactInt64Raw(rawEnd); ok && n >= 0 {
				view.LaborEndTime = n
				view.LaborEndTimeObserved = true
			}
		}
	}
	if rawFailCount, exists := fields["4"]; exists {
		view.HireFailCntObserved = false
		if isJSONNull(rawFailCount) {
			view.HireFailCnt = 0
			view.HireFailCntObserved = true
		} else if n, ok := readExactInt32Raw(rawFailCount); ok && n >= 0 {
			view.HireFailCnt = n
			view.HireFailCntObserved = true
		}
	}
	if n, ok := readInt32JSONField(fields, "5"); ok {
		view.EventID = n
	}
	if rawEvery, exists := fields["6"]; exists {
		view.EveryMakeNumObserved = false
		var n int32
		if !isJSONNull(rawEvery) && json.Unmarshal(rawEvery, &n) == nil {
			view.EveryMakeNum = n
			view.EveryMakeNumObserved = true
		}
	}
	if rawRecv, exists := fields["7"]; exists {
		view.RecvCntObserved = false
		var n int32
		if !isJSONNull(rawRecv) && json.Unmarshal(rawRecv, &n) == nil {
			view.RecvCnt = n
			view.RecvCntObserved = true
		}
	}
	if rawSurplus, exists := fields["8"]; exists {
		view.SurplusRecvNumObserved = false
		var n int32
		if !isJSONNull(rawSurplus) && json.Unmarshal(rawSurplus, &n) == nil {
			view.SurplusRecvNum = n
			view.SurplusRecvNumObserved = true
		}
	}
	if n, ok := readInt64JSONField(fields, "9"); ok {
		view.UTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "10"); ok {
		view.CTimeMs = n
	}
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// CollectRewards returns the currently observed namespace 103 reward state.
func (s *State) CollectRewards() map[int32]CollectRewardView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]CollectRewardView, len(s.collectRewards))
	for k, v := range s.collectRewards {
		if v == nil {
			continue
		}
		cp := *v
		cp.RecvIDs = cloneInt32s(v.RecvIDs)
		cp.ArtCreateRewardIDs = cloneInt32s(v.ArtCreateRewardIDs)
		out[k] = cp
	}
	return out
}

// CollectRewardObserved reports whether namespace 103 has been observed.
func (s *State) CollectRewardObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.collectRewardObserved
}

// ReadyCollectRewardTypes returns collectRwd.recv types that have at least
// one unclaimed c_flowerCollect reward at or below the observed exp.
func (s *State) ReadyCollectRewardTypes(types ...int32) []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.collectRewards) == 0 {
		return nil
	}
	filter := setOf(types)
	out := make([]int32, 0, len(s.collectRewards))
	for typeID, reward := range s.collectRewards {
		if reward == nil {
			continue
		}
		if len(filter) > 0 {
			if _, ok := filter[typeID]; !ok {
				continue
			}
		}
		if collectRewardReady(*reward) {
			out = append(out, typeID)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyArtCreateRewardVaseIDs returns vase ids whose flower-art creation
// reward can be claimed through collectRwd.recvArtCreateRwdByVase.
func (s *State) ReadyArtCreateRewardVaseIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	reward := s.collectRewards[13]
	if reward == nil || len(s.flowerArt.MakeList) == 0 {
		return nil
	}
	received := setOf(reward.ArtCreateRewardIDs)
	ready := map[int32]struct{}{}
	for _, artID := range s.flowerArt.MakeList {
		if artID <= 0 {
			continue
		}
		if _, ok := received[artID]; ok {
			continue
		}
		vaseID := (artID - 1) / 100
		if vaseID <= 0 {
			continue
		}
		ready[vaseID] = struct{}{}
	}
	out := make([]int32, 0, len(ready))
	for vaseID := range ready {
		out = append(out, vaseID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ShopCultivateObserved reports whether namespace 113 has been observed.
func (s *State) ShopCultivateObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shopCultivateObserved
}

// MarkShopCultivateOfferExhausted reconciles a material-shop offer after the
// server authoritatively rejects shopCultivate.buy with code 312 ("cannot buy
// this item again"). The next enter/refresh snapshot may replace the record;
// until then, the planner must not keep retrying the same stale offer.
func (s *State) MarkShopCultivateOfferExhausted(shopID int32) bool {
	if shopID <= 0 {
		return false
	}
	_, _, buyLimit, _, ok := shopCultivateStatic(shopID)
	if !ok || buyLimit <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shopCultivateBought == nil {
		s.shopCultivateBought = make(map[int32]int32)
	}
	if s.shopCultivateBought[shopID] >= buyLimit {
		return false
	}
	s.shopCultivateBought[shopID] = buyLimit
	return true
}

// ShopCultivateNeedsEnter reports whether material-shop state must be synced
// via shopCultivate.enter (never observed, incomplete timing fields, or daily
// reset boundary passed).
//
// Buy ACKs often patch only 113.6 (bRecord) while leaving larTime unset. Without
// larTime, AutoRefreshReady can never become true, so an emptied shelf stays
// stuck for the rest of the day unless we force enter to reload full shop
// state (infoMap + larTime + mrCount + lResetTime).
//
// Enter/refresh responses also frequently omit 113.2 (lResetTime) while still
// updating larTime / infoMap. Prefer the newer of resetMs and larMs when
// deciding whether today's shelf has already been synced; otherwise a stale
// prior-day resetMs keeps NeedsEnter true forever and blocks buy behind an
// enter spin loop.
func (s *State) ShopCultivateNeedsEnter(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.shopCultivateObserved {
		return true
	}
	if s.shopCultivateLarMs <= 0 {
		return true
	}
	markerMs := shopCultivateSyncMarkerMs(s.shopCultivateResetMs, s.shopCultivateLarMs)
	if markerMs <= 0 {
		return true
	}
	return gameDayID(now) > gameDayID(time.UnixMilli(markerMs))
}

// shopCultivateSyncMarkerMs returns the best "synced for game day" timestamp.
// larTime alone is enough after a successful enter/refresh that omitted
// lResetTime; when both exist, the later game day wins.
func shopCultivateSyncMarkerMs(resetMs, larMs int64) int64 {
	switch {
	case resetMs <= 0:
		return larMs
	case larMs <= 0:
		return resetMs
	case gameDayID(time.UnixMilli(larMs)) > gameDayID(time.UnixMilli(resetMs)):
		return larMs
	default:
		return resetMs
	}
}

// ShopCultivateAutoRefreshDue reports whether the client's automatic shelf
// rotation countdown has elapsed. The mini client responds by calling enter;
// shopCultivate.refresh is a distinct user action that consumes refresh quota.
func (s *State) ShopCultivateAutoRefreshDue(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.shopCultivateObserved || s.shopCultivateLarMs <= 0 {
		return false
	}
	cd := shopCultivateAutoRefreshCD()
	if cd <= 0 {
		return false
	}
	return !now.Before(time.UnixMilli(s.shopCultivateLarMs).Add(cd))
}

// ShopCultivateOffers returns current material-shop offers enriched with the
// static item/limit metadata from c_shop_cultivate.
func (s *State) ShopCultivateOffers() []ShopCultivateOfferView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ShopCultivateOfferView, 0, len(s.shopCultivateCosts))
	for shopID, cost := range s.shopCultivateCosts {
		view := ShopCultivateOfferView{
			ShopID:     shopID,
			CostItemID: cost.ItemID,
			CostCount:  cost.Count,
			Bought:     s.shopCultivateBought[shopID],
		}
		if itemID, itemCount, buyLimit, sortOrder, ok := shopCultivateStatic(shopID); ok {
			view.ItemID = itemID
			view.ItemCount = itemCount
			view.BuyLimit = buyLimit
			view.Sort = sortOrder
		}
		if view.BuyLimit > 0 {
			view.Remaining = view.BuyLimit - view.Bought
			if view.Remaining < 0 {
				view.Remaining = 0
			}
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sort != out[j].Sort {
			return out[i].Sort < out[j].Sort
		}
		return out[i].ShopID < out[j].ShopID
	})
	return out
}

// ShopGiftbagObserved reports whether namespace 112 has been observed.
func (s *State) ShopGiftbagObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shopGiftbagObserved
}

// ShopGiftbagOffers returns status normalized at the current wall clock.
func (s *State) ShopGiftbagOffers() []ShopGiftbagOfferView {
	return s.ShopGiftbagOffersAt(time.Now())
}

// ShopGiftbagOffersAt returns static gift-bag rows enriched with observed
// purchase records, daily reset semantics, the current reward tier, and the
// client-visible cooldown deadline.
func (s *State) ShopGiftbagOffersAt(now time.Time) []ShopGiftbagOfferView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	table, ok := StaticTableByName("c_shop_giftbag")
	if !ok {
		return nil
	}
	out := make([]ShopGiftbagOfferView, 0, len(table.Rows))
	for idStr, rawRow := range table.Rows {
		shopID := atoi32(idStr)
		if shopID <= 0 {
			continue
		}
		view, ok := shopGiftbagStatic(shopID, rawRow)
		if !ok {
			continue
		}
		view.DailyBought = s.shopGiftbagDRecord[shopID]
		if s.shopGiftbagResetMs > 0 && calendarDayID(time.UnixMilli(s.shopGiftbagResetMs)) < calendarDayID(now) {
			view.DailyBought = 0
		}
		view.WeekBought = s.shopGiftbagWRecord[shopID]
		view.MonthBought = s.shopGiftbagMRecord[shopID]
		view.TotalBought = s.shopGiftbagTRecord[shopID]
		view.Remaining = giftbagRemaining(view)
		if len(view.Rewards) > 0 {
			idx := int(view.DailyBought)
			if idx < 0 {
				idx = 0
			}
			if idx >= len(view.Rewards) {
				idx = len(view.Rewards) - 1
			}
			view.NextReward = view.Rewards[idx]
		}
		if boughtAt := s.shopGiftbagBuyTimeRecord[shopID]; boughtAt > 0 && view.CooldownSec > 0 {
			availableAt := boughtAt + int64(view.CooldownSec)*1000
			if availableAt > now.UnixMilli() {
				view.AvailableAtMs = availableAt
			}
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sort != out[j].Sort {
			return out[i].Sort < out[j].Sort
		}
		return out[i].ShopID < out[j].ShopID
	})
	return out
}

// PearlObserved reports whether namespace 115 has been observed.
func (s *State) PearlObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pearlObserved
}

// Pearl returns the currently observed pearl summary state.
func (s *State) Pearl() PearlView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pearl
}

// PearlPlaces returns a defensive copy of observed pearl production slots.
func (s *State) PearlPlaces() map[int32]PearlPlaceView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]PearlPlaceView, len(s.pearlPlaces))
	for id, place := range s.pearlPlaces {
		if place == nil {
			continue
		}
		out[id] = *place
	}
	return out
}

// PearlReceivableCount mirrors PearlPlaceCtrl.canRecvNum from the observed
// client. The boolean is false when required slot fields or catalog timing are
// unknown, so automation never probes with an ambiguous state.
func PearlReceivableCount(place PearlPlaceView, now time.Time) (int64, bool) {
	if !place.LaborEndTimeObserved || !place.EveryMakeNumObserved ||
		!place.RecvCntObserved || !place.SurplusRecvNumObserved {
		return 0, false
	}
	if place.LaborEndTime < 0 || place.EveryMakeNum < 0 ||
		place.RecvCnt < 0 || place.SurplusRecvNum < 0 {
		return 0, false
	}
	// The client returns before adding surplus when either of these fields is
	// zero. Explicit null/zero therefore represents a known empty slot.
	if place.LaborEndTime == 0 || place.EveryMakeNum == 0 {
		return 0, true
	}
	timing, ok := PearlProductionTimingFromCatalog()
	if !ok {
		return 0, false
	}
	hireTimeMs := timing.HireTimeSeconds * int64(time.Second/time.Millisecond)
	gatherCDMs := timing.GatherCDSeconds * int64(time.Second/time.Millisecond)
	if hireTimeMs <= 0 || gatherCDMs <= 0 {
		return 0, false
	}

	startMs := place.LaborEndTime - hireTimeMs
	effectiveMs := now.UnixMilli()
	if effectiveMs > place.LaborEndTime {
		effectiveMs = place.LaborEndTime
	}
	elapsedMs := effectiveMs - startMs
	if elapsedMs < 0 {
		elapsedMs = 0
	}
	cycles := elapsedMs / gatherCDMs
	unreceivedCycles := cycles - int64(place.RecvCnt)
	if unreceivedCycles < 0 {
		unreceivedCycles = 0
	}
	count := unreceivedCycles*int64(place.EveryMakeNum) + int64(place.SurplusRecvNum)
	if count < 0 {
		return 0, false
	}
	return count, true
}

// ReadyPearlPlaceIDs returns time-matured pearl slots at the current instant.
// Call ReadyPearlPlaceIDsAt when a planner already owns a fixed clock value.
func (s *State) ReadyPearlPlaceIDs() []int32 {
	return s.ReadyPearlPlaceIDsAt(time.Now())
}

// ReadyPearlPlaceIDsAt returns time-matured pearl slots in stable order.
func (s *State) ReadyPearlPlaceIDsAt(now time.Time) []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readyPearlPlaceIDsLocked(now)
}

func (s *State) readyPearlPlaceIDsLocked(now time.Time) []int32 {
	out := make([]int32, 0, len(s.pearlPlaces))
	for id, place := range s.pearlPlaces {
		if place == nil {
			continue
		}
		if count, known := PearlReceivableCount(*place, now); known && count > 0 {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// PearlClaimSnapshot captures the exact slots ready at now for a one-key RPC.
func (s *State) PearlClaimSnapshot(now time.Time) (PearlClaimSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.readyPearlPlaceIDsLocked(now)
	if len(ids) == 0 {
		return PearlClaimSnapshot{}, false
	}
	return PearlClaimSnapshot{At: now, PlaceIDs: ids}, true
}

// PearlClaimApplied reports whether every slot captured by preflight is no
// longer receivable at that same instant. A deleted slot counts as cleared.
func (s *State) PearlClaimApplied(snapshot PearlClaimSnapshot) bool {
	if snapshot.At.IsZero() || len(snapshot.PlaceIDs) == 0 {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, id := range snapshot.PlaceIDs {
		place, exists := s.pearlPlaces[id]
		if !exists || place == nil {
			continue
		}
		count, known := PearlReceivableCount(*place, snapshot.At)
		if !known || count > 0 {
			return false
		}
	}
	return true
}

// PearlDrawCount returns the number of pearl draw entries currently observed.
func (s *State) PearlDrawCount() int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pearlDrawCount
}

// PearlDailyFreeReady reports whether the daily free pearl has not been
// observed as received for the local day represented by now.
func (s *State) PearlDailyFreeReady(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.pearlObserved || !s.pearl.Observed {
		return false
	}
	return !sameLocalDay(s.pearl.RecvDailyDate, now)
}

func collectRewardReady(view CollectRewardView) bool {
	if view.Type <= 0 || view.Exp <= 0 {
		return false
	}
	table, ok := StaticTableByName("c_flowerCollect")
	if !ok {
		return false
	}
	received := setOf(view.RecvIDs)
	for idStr, rawRow := range table.Rows {
		rowID := atoi32(idStr)
		if rowID <= 0 || rowID/10000 != view.Type {
			continue
		}
		if _, ok := received[rowID]; ok {
			continue
		}
		var row map[string]json.RawMessage
		if err := json.Unmarshal(rawRow, &row); err != nil {
			continue
		}
		exp, ok := readInt32JSONField(row, "exp")
		if !ok || exp <= 0 {
			continue
		}
		if view.Exp >= exp {
			return true
		}
	}
	return false
}

func shopCultivateStatic(shopID int32) (itemID, itemCount, buyLimit, sortOrder int32, ok bool) {
	rawRow, ok := StaticRow("c_shop_cultivate", shopID)
	if !ok {
		return 0, 0, 0, 0, false
	}
	var row map[string]json.RawMessage
	if err := json.Unmarshal(rawRow, &row); err != nil {
		return 0, 0, 0, 0, false
	}
	if rawItems, ok := row["items"]; ok {
		var stacks []json.RawMessage
		if err := json.Unmarshal(rawItems, &stacks); err == nil && len(stacks) > 0 {
			parts := readInt32OrderedListRaw(stacks[0])
			if len(parts) >= 2 {
				itemID = parts[0]
				itemCount = parts[1]
			}
		}
	}
	if rawLimit, ok := row["bLimit"]; ok {
		parts := readInt32OrderedListRaw(rawLimit)
		if len(parts) > 0 {
			buyLimit = parts[0]
		}
	}
	if n, ok := readInt32JSONField(row, "sort"); ok {
		sortOrder = n
	}
	return itemID, itemCount, buyLimit, sortOrder, itemID > 0
}

// shopCultivateAutoRefreshCD is c_shop_cultivate[-1].$autoRefreshCd (seconds).
// Catalog value is 9000 (2h30m); clients often display the remaining countdown
// rounded up by one second.
func shopCultivateAutoRefreshCD() time.Duration {
	rawRow, ok := StaticRow("c_shop_cultivate", -1)
	if !ok {
		return 9000 * time.Second
	}
	var row map[string]json.RawMessage
	if err := json.Unmarshal(rawRow, &row); err != nil {
		return 9000 * time.Second
	}
	n, ok := readInt32JSONField(row, "$autoRefreshCd")
	if !ok || n <= 0 {
		return 9000 * time.Second
	}
	return time.Duration(n) * time.Second
}

func shopGiftbagStatic(shopID int32, rawRow json.RawMessage) (ShopGiftbagOfferView, bool) {
	var row map[string]json.RawMessage
	if err := json.Unmarshal(rawRow, &row); err != nil {
		return ShopGiftbagOfferView{}, false
	}
	view := ShopGiftbagOfferView{ShopID: shopID}
	if n, ok := readInt32JSONField(row, "type"); ok {
		view.Type = n
	}
	if n, ok := readInt32JSONField(row, "shareId"); ok {
		view.ShareID = n
	}
	if n, ok := readInt32JSONField(row, "rchgId"); ok {
		view.RchgID = n
	}
	if n, ok := readInt32JSONField(row, "moneyId"); ok {
		view.MoneyID = n
	}
	if n, ok := readInt32JSONField(row, "price"); ok {
		view.Price = n
	}
	if n, ok := readInt32JSONField(row, "priceMax"); ok {
		view.PriceMax = n
	}
	if n, ok := readInt32JSONField(row, "sort"); ok {
		view.Sort = n
	}
	if n, ok := readInt32JSONField(row, "cd"); ok {
		view.CooldownSec = n
	}
	view.DailyLimit = firstInt32ListValue(row["dLimit"])
	view.WeeklyLimit = firstInt32ListValue(row["wLimit"])
	view.MonthLimit = firstInt32ListValue(row["mLimit"])
	view.TotalLimit = firstInt32ListValue(row["tLimit"])
	if rawItems, ok := row["items"]; ok {
		view.Rewards = readItemCountsRaw(rawItems)
	}
	return view, true
}

func firstInt32ListValue(raw json.RawMessage) int32 {
	parts := readInt32OrderedListRaw(raw)
	if len(parts) == 0 {
		return 0
	}
	return parts[0]
}

func giftbagRemaining(view ShopGiftbagOfferView) int32 {
	remaining := int32(0)
	applyLimit := func(limit, bought int32) {
		if limit <= 0 {
			return
		}
		left := limit - bought
		if left < 0 {
			left = 0
		}
		if remaining == 0 || left < remaining {
			remaining = left
		}
	}
	applyLimit(view.DailyLimit, view.DailyBought)
	applyLimit(view.WeeklyLimit, view.WeekBought)
	applyLimit(view.MonthLimit, view.MonthBought)
	applyLimit(view.TotalLimit, view.TotalBought)
	return remaining
}
