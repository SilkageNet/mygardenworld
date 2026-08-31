package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"
)

func (s *State) applyCustomerOrdersLocked(raw json.RawMessage) {
	var ns109 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns109); err != nil {
		return
	}
	raw0, ok := ns109["0"]
	if !ok {
		return
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &outer); err != nil {
		return
	}
	s.customerOrderSummary.Observed = true
	if n, ok := readInt64JSONField(outer, "2"); ok {
		s.customerOrderSummary.NextGenTimeMs = n
	}
	if n, ok := readInt64JSONField(outer, "3"); ok {
		s.customerOrderSummary.UpdatedAtMs = n
	}
	if n, ok := readInt64JSONField(outer, "4"); ok {
		s.customerOrderSummary.CreatedAtMs = n
	}
	if n, ok := readInt32JSONField(outer, "5"); ok {
		s.customerOrderSummary.CreateCount = n
	}
	raw1, ok := outer["1"]
	if !ok {
		s.customerOrderSummary.ActiveCount = int32(len(s.customerOrders))
		return
	}
	var orders map[string]json.RawMessage
	if err := json.Unmarshal(raw1, &orders); err != nil {
		s.customerOrderSummary.ActiveCount = int32(len(s.customerOrders))
		return
	}
	// Replace the full order set.
	// Older captures used fields 0=[[flowerId,count],...], 1=npcId, 3=finishCnt.
	// Current captures use fields 0=dialogId, 1=artId, 2=num, 3=pathId.
	s.customerOrders = make(map[int32]*CustomerOrder, len(orders))
	for npcID, rawOrder := range orders {
		id := atoi32(npcID)
		if id <= 0 {
			continue
		}
		order := &CustomerOrder{NPCID: id}
		storeID := id
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawOrder, &fields); err == nil {
			oldShape := false
			if rawReqs, ok := fields["0"]; ok {
				flowers, items := parseOrderRequires(rawReqs)
				order.Requires = append(order.Requires, flowers...)
				order.ItemRequires = append(order.ItemRequires, items...)
				oldShape = len(flowers) > 0 || len(items) > 0
			}
			if oldShape {
				if rawNPCID, ok := fields["1"]; ok {
					var n int32
					if json.Unmarshal(rawNPCID, &n) == nil && n > 0 {
						order.NPCID = n
						storeID = n
					}
				}
				if rawFinishCnt, ok := fields["3"]; ok {
					var n int32
					if json.Unmarshal(rawFinishCnt, &n) == nil {
						order.FinishCnt = n
					}
				}
				if rawItemID, rawItemOK := fields["1"]; rawItemOK {
					var itemID int32
					var count int32
					_ = json.Unmarshal(rawItemID, &itemID)
					if rawCount, ok := fields["2"]; ok {
						_ = json.Unmarshal(rawCount, &count)
					}
					if itemID > 0 && count > 0 && itemID != order.NPCID {
						order.ItemRequires = append(order.ItemRequires, ItemRequire{ItemID: itemID, Count: count})
					}
				}
			} else if rawItemID, ok := fields["1"]; ok {
				var itemID int32
				var count int32
				_ = json.Unmarshal(rawItemID, &itemID)
				if rawCount, ok := fields["2"]; ok {
					_ = json.Unmarshal(rawCount, &count)
				}
				if itemID > 0 && count > 0 {
					order.ItemRequires = []ItemRequire{{ItemID: itemID, Count: count}}
				}
			}
		}
		s.customerOrders[storeID] = order
	}
	s.customerOrderSummary.ActiveCount = int32(len(s.customerOrders))
}

func (s *State) applyFlowerRackLocked(raw json.RawMessage) {
	var ns104 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns104); err != nil {
		return
	}
	raw0, ok := ns104["0"]
	if !ok {
		return
	}
	var slots map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &slots); err != nil {
		return
	}
	for rackIDStr, rawSlot := range slots {
		rackID := atoi32(rackIDStr)
		if rackID <= 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawSlot, &fields); err != nil {
			continue
		}
		slot := s.flowerRack[rackID]
		if slot == nil {
			slot = &FlowerRackSlot{RackID: rackID}
			s.flowerRack[rackID] = slot
		}
		if rawRackID, ok := fields["1"]; ok {
			var n int32
			if json.Unmarshal(rawRackID, &n) == nil && n > 0 {
				slot.RackID = n
			}
		}
		if rawItemID, ok := fields["2"]; ok {
			var n int32
			if json.Unmarshal(rawItemID, &n) == nil {
				slot.ItemID = n
			}
		}
		if rawCount, ok := fields["3"]; ok {
			var n int32
			if json.Unmarshal(rawCount, &n) == nil {
				slot.Count = n
			}
		}
		if rawListedAt, ok := fields["4"]; ok {
			var n int64
			if json.Unmarshal(rawListedAt, &n) == nil {
				slot.ListedAtMs = n
			}
		}
		if rawUpdatedAt, ok := fields["5"]; ok {
			var n int64
			if json.Unmarshal(rawUpdatedAt, &n) == nil {
				slot.UpdatedAtMs = n
			}
		}
		if slot.ItemID == 0 || slot.Count == 0 {
			slot.ItemID = 0
			slot.Count = 0
			slot.SellReadyAtMs = 0
		} else if sellDurationMs := FlowerRackSellDurationMs(); sellDurationMs > 0 && slot.ListedAtMs > 0 {
			slot.SellReadyAtMs = slot.ListedAtMs + int64(slot.Count)*sellDurationMs
		}
	}
}

func (s *State) applyFlowerOrdersLocked(raw json.RawMessage) {
	// NS105 structure: {"0": {"1": {boxId: {order...}}, ...}}
	var ns105 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns105); err != nil {
		return
	}
	raw0, ok := ns105["0"]
	if !ok {
		return
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &inner); err != nil {
		return
	}
	if raw1, ok := inner["1"]; ok {
		var boxes map[string]json.RawMessage
		if err := json.Unmarshal(raw1, &boxes); err == nil {
			s.flowerOrders = make(map[int32]*FlowerOrder, len(boxes))
			for boxIDStr, rawBox := range boxes {
				boxID := atoi32(boxIDStr)
				if boxID == 0 {
					continue
				}
				var fields map[string]json.RawMessage
				if err := json.Unmarshal(rawBox, &fields); err != nil {
					continue
				}
				order := &FlowerOrder{BoxID: boxID}
				if n, ok := readInt32JSONField(fields, "0"); ok {
					order.NPCID = n
				}
				if n, ok := readInt32JSONField(fields, "1"); ok {
					order.DialogID = n
				}
				// field "2" = [[flowerId, count], ...]
				if rawReqs, ok := fields["2"]; ok {
					order.Requires = parseFlowerRequires(rawReqs)
				}
				if n, ok := readInt32JSONField(fields, "3"); ok {
					order.IsVideo = n
				}
				if rawCdTime, ok := fields["4"]; ok {
					_ = json.Unmarshal(rawCdTime, &order.CdTimeMs)
				}
				if rawCTime, ok := fields["5"]; ok {
					_ = json.Unmarshal(rawCTime, &order.CTimeMs)
				}
				if n, ok := readInt32JSONField(fields, "6"); ok {
					order.PlaceIdx = n
				}
				if rawRewards, ok := fields["7"]; ok {
					order.VideoRewards = readItemCountsRaw(rawRewards)
				}
				s.flowerOrders[boxID] = order
			}
		}
	}
	if rawReceived, ok := inner["2"]; ok {
		var ids []int32
		if json.Unmarshal(rawReceived, &ids) == nil {
			s.flowerOrderRewardsReceived = make(map[int32]bool, len(ids))
			for _, id := range ids {
				if id > 0 {
					s.flowerOrderRewardsReceived[id] = true
				}
			}
		}
	}
	if rawSatin, ok := inner["6"]; ok {
		s.residentSatinOrder = parseResidentSpecialOrder(rawSatin)
	}
	if rawDecorate, ok := inner["7"]; ok {
		s.residentDecorateOrder = parseResidentSpecialOrder(rawDecorate)
	}
}

func parseResidentSpecialOrder(raw json.RawMessage) ResidentSpecialOrder {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return ResidentSpecialOrder{}
	}
	view := ResidentSpecialOrder{Observed: true}
	if rawReqs, ok := fields["0"]; ok {
		view.Requires = parseFlowerRequires(rawReqs)
	}
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.NPCID = n
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.DialogID = n
	}
	if n, ok := readInt32JSONField(fields, "3"); ok {
		view.FinishCnt = n
	}
	if n, ok := readInt32JSONField(fields, "4"); ok {
		view.IsVideo = n
	}
	if rawRewards, ok := fields["5"]; ok {
		view.VideoRewards = readItemCountsRaw(rawRewards)
	}
	if n, ok := readInt64JSONField(fields, "6"); ok {
		view.CdTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "7"); ok {
		view.CTimeMs = n
	}
	return view
}

func (s *State) applyTeamOrderLocked(raw json.RawMessage) {
	var ns107 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns107); err != nil {
		return
	}
	raw0, ok := ns107["0"]
	if !ok {
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &fields); err != nil {
		return
	}
	view := TeamOrderView{Observed: true}
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.Status = n
	}
	if n, ok := readInt64JSONField(fields, "2"); ok {
		view.StartTimeMs = n
	}
	if n, ok := readInt32JSONField(fields, "3"); ok {
		view.OrderNum = n
	}
	if n, ok := readInt32JSONField(fields, "4"); ok {
		view.FlowerID = n
	}
	if n, ok := readInt32JSONField(fields, "5"); ok {
		view.Reward = n
	}
	if n, ok := readInt32JSONField(fields, "6"); ok {
		view.RemainingNum = n
	}
	if n, ok := readInt32JSONField(fields, "7"); ok {
		view.RefreshNotCnt = n
	}
	if n, ok := readInt64JSONField(fields, "8"); ok {
		view.UTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "9"); ok {
		view.CTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "10"); ok {
		view.ActiveTimeMs = n
	}
	if n, ok := readInt32JSONField(fields, "11"); ok {
		view.ActiveCnt = n
	}
	if n, ok := readInt32JSONField(fields, "14"); ok {
		view.NPCID = n
	}
	s.teamOrder = view
}

func (s *State) applyPalaceOrderLocked(raw json.RawMessage) {
	var ns108 map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ns108); err != nil {
		return
	}
	raw0, ok := ns108["0"]
	if !ok {
		return
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw0, &outer); err != nil {
		return
	}
	rawOrder := raw0
	if nested, ok := outer["0"]; ok {
		rawOrder = nested
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rawOrder, &fields); err != nil {
		return
	}
	view := PalaceOrderView{Observed: true}
	if n, ok := readInt64JSONField(fields, "0"); ok {
		view.UID = n
	}
	if n, ok := readInt32JSONField(fields, "1"); ok {
		view.FlowerID = n
	}
	if n, ok := readInt32JSONField(fields, "2"); ok {
		view.Num = n
	}
	if n, ok := readInt32JSONField(fields, "3"); ok {
		view.IsFinish = n
	}
	if n, ok := readInt64JSONField(fields, "4"); ok {
		view.LTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "5"); ok {
		view.UTimeMs = n
	}
	if n, ok := readInt64JSONField(fields, "6"); ok {
		view.CTimeMs = n
	}
	s.palaceOrder = view
}

func (s *State) applyRoadGrowLocked(raw json.RawMessage) {
	var ns119 map[string]json.RawMessage
	if json.Unmarshal(raw, &ns119) != nil {
		return
	}
	rawRecv, ok := ns119["3"]
	if !ok {
		return
	}
	var recv map[string]int32
	if json.Unmarshal(rawRecv, &recv) != nil {
		return
	}
	s.roadGrowReceived = make(map[int32]bool, len(recv))
	for id, v := range recv {
		if v != 0 {
			s.roadGrowReceived[atoi32(id)] = true
		}
	}
}

func (s *State) applyRandomEventsLocked(raw json.RawMessage) {
	var ns129 map[string]json.RawMessage
	if json.Unmarshal(raw, &ns129) != nil || ns129 == nil {
		s.invalidateRandomEventMapLocked("namespace 129 不是对象")
		return
	}
	raw0, ok := ns129["0"]
	if !ok {
		return
	}
	var inner map[string]json.RawMessage
	if json.Unmarshal(raw0, &inner) != nil || inner == nil {
		s.invalidateRandomEventMapLocked("namespace 129.0 不是对象")
		return
	}
	rawEvents, ok := inner["1"]
	if !ok {
		return
	}
	s.randomEventObserved = true
	trimmed := bytes.TrimSpace(rawEvents)
	if bytes.Equal(trimmed, []byte("null")) {
		s.randomEvents = make(map[int32]*RandomEventView)
		s.randomEventMapValid = true
		s.randomEventMapError = ""
		return
	}
	var events map[string]json.RawMessage
	if json.Unmarshal(rawEvents, &events) != nil || events == nil {
		s.invalidateRandomEventMapLocked("namespace 129.0.1 不是事件对象")
		return
	}
	keys := make([]string, 0, len(events))
	for key := range events {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	replacement := make(map[int32]*RandomEventView, len(events))
	for _, idStr := range keys {
		rawEvent := events[idStr]
		parsedID, err := strconv.ParseInt(idStr, 10, 32)
		if err != nil || parsedID <= 0 {
			s.invalidateRandomEventMapLocked(fmt.Sprintf("事件 key %q 不是正整数", idStr))
			return
		}
		mapID := int32(parsedID)
		// Whole-table replacements may mark removed events as null. Treat those
		// keys as absent instead of failing the authoritative map closed.
		if bytes.Equal(bytes.TrimSpace(rawEvent), []byte("null")) {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(rawEvent, &fields) != nil || fields == nil {
			s.invalidateRandomEventMapLocked(fmt.Sprintf("事件 %d 不是对象", mapID))
			return
		}
		eventID, idOK := strictRandomEventInt32(fields, "0")
		positionIndex, positionOK := strictRandomEventInt32(fields, "1")
		dialogID, dialogOK := strictRandomEventInt32(fields, "2")
		if !idOK || !positionOK || !dialogOK {
			s.invalidateRandomEventMapLocked(fmt.Sprintf("事件 %d 缺少有效的 eventId/posIdx/dialogId", mapID))
			return
		}
		event := validateRandomEvent(mapID, eventID, positionIndex, dialogID)
		replacement[mapID] = &event
	}
	s.randomEvents = replacement
	s.randomEventMapValid = true
	s.randomEventMapError = ""
}

func (s *State) invalidateRandomEventMapLocked(reason string) {
	s.randomEventObserved = true
	s.randomEvents = make(map[int32]*RandomEventView)
	s.randomEventMapValid = false
	s.randomEventMapError = reason
}

func strictRandomEventInt32(fields map[string]json.RawMessage, key string) (int32, bool) {
	raw, ok := fields[key]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return 0, false
	}
	var value int32
	if json.Unmarshal(raw, &value) != nil {
		return 0, false
	}
	return value, true
}

func validateRandomEvent(mapID, eventID, positionIndex, dialogID int32) RandomEventView {
	event := RandomEventView{EventID: eventID, PositionIndex: positionIndex, DialogID: dialogID}
	if mapID <= 0 || eventID <= 0 || mapID != eventID {
		event.BlockedReason = fmt.Sprintf("事件 key=%d 与 eventId=%d 不一致", mapID, eventID)
		return event
	}
	definition, known := RandomEventDefinition(eventID)
	event.CatalogKnown = known
	if !known {
		event.BlockedReason = "c_randomEvent 未找到完整配置"
		return event
	}
	event.CostFree = definition.CostFree
	if !definition.CostFree {
		event.BlockedReason = "事件配置包含消耗"
		return event
	}
	if positionIndex < 0 || positionIndex >= definition.PlaceCount {
		event.BlockedReason = fmt.Sprintf("posIdx=%d 超出位置范围 [0,%d)", positionIndex, definition.PlaceCount)
		return event
	}
	dialogKnown := false
	for _, configured := range definition.DialogIDs {
		if dialogID == configured {
			dialogKnown = true
			break
		}
	}
	if !dialogKnown {
		event.BlockedReason = fmt.Sprintf("dialogId=%d 不属于事件配置", dialogID)
		return event
	}
	event.Valid = true
	return event
}

func parseFlowerRequires(raw json.RawMessage) []FlowerRequire {
	flowers, _ := parseOrderRequires(raw)
	return flowers
}

func parseOrderRequires(raw json.RawMessage) ([]FlowerRequire, []ItemRequire) {
	var reqs [][]int32
	if json.Unmarshal(raw, &reqs) != nil {
		return nil, nil
	}
	flowers := make([]FlowerRequire, 0, len(reqs))
	items := make([]ItemRequire, 0, len(reqs))
	for _, req := range reqs {
		if len(req) >= 2 && req[0] > 0 && req[1] > 0 {
			if isFlowerItemID(req[0]) {
				flowers = append(flowers, FlowerRequire{FlowerID: req[0], Count: req[1]})
			} else {
				items = append(items, ItemRequire{ItemID: req[0], Count: req[1]})
			}
		}
	}
	return flowers, items
}

// MarkResidentOrderDailyLimitReached records the server-side normal resident
// order daily cap so the planner stops selecting orderFlower.finishOrder until
// the next observed game day.
func (s *State) MarkResidentOrderDailyLimitReached(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.residentOrderLimitUntilMs = NextGameDayReset(now).UnixMilli()
	s.residentOrderLimitDayID = s.statistics.DayID
	if s.residentOrderLimitDayID == 0 {
		s.residentOrderLimitDayID = gameDayID(now)
	}
}

// ResidentOrderDailyLimitReached reports a locally recorded server-side normal
// resident order daily cap.
func (s *State) ResidentOrderDailyLimitReached(now time.Time) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.residentOrderLimitUntilMs <= 0 {
		return time.Time{}, false
	}
	until := time.UnixMilli(s.residentOrderLimitUntilMs)
	if !until.After(now) {
		s.residentOrderLimitUntilMs = 0
		s.residentOrderLimitDayID = 0
		return time.Time{}, false
	}
	return until, true
}

// MarkResidentSatinDailyLimitReached records the server-side satin resident
// order daily cap so the planner stops selecting finishSatinOrder until the
// next 00:00 Asia/Shanghai reset.
func (s *State) MarkResidentSatinDailyLimitReached(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.residentSatinLimitUntilMs = NextCalendarDayReset(now).UnixMilli()
}

// ResidentSatinDailyLimitReached reports a locally recorded server-side satin
// resident order daily cap.
func (s *State) ResidentSatinDailyLimitReached(now time.Time) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.residentSatinLimitUntilMs <= 0 {
		return time.Time{}, false
	}
	until := time.UnixMilli(s.residentSatinLimitUntilMs)
	if !until.After(now) {
		s.residentSatinLimitUntilMs = 0
		return time.Time{}, false
	}
	return until, true
}

// MarkResidentDecorateDailyLimitReached records the server-side decorate
// resident order daily cap so the planner stops selecting finishDecorateOrder
// until the next 00:00 Asia/Shanghai reset.
func (s *State) MarkResidentDecorateDailyLimitReached(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.residentDecorateLimitUntilMs = NextCalendarDayReset(now).UnixMilli()
}

// ResidentDecorateDailyLimitReached reports a locally recorded server-side
// decorate resident order daily cap.
func (s *State) ResidentDecorateDailyLimitReached(now time.Time) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.residentDecorateLimitUntilMs <= 0 {
		return time.Time{}, false
	}
	until := time.UnixMilli(s.residentDecorateLimitUntilMs)
	if !until.After(now) {
		s.residentDecorateLimitUntilMs = 0
		return time.Time{}, false
	}
	return until, true
}

// NoteResidentOrderFinished records one successful ordinary resident-order
// finish when the response did not already carry an authoritative field-9
// statistics update (ApplyV clears the bias in that case).
func (s *State) NoteResidentOrderFinished(now time.Time, raw json.RawMessage) {
	if responseStatisticsCounters(raw).normal {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	noteResidentOrderFinish(&s.residentOrderFinishBias, &s.residentOrderFinishBiasDayID, gameDayID(now))
}

// NoteResidentSatinOrderFinished records a successful satin resident order
// whose response omitted the authoritative namespace-124 satin counter.
func (s *State) NoteResidentSatinOrderFinished(now time.Time, raw json.RawMessage) {
	if responseStatisticsCounters(raw).satin {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	noteResidentOrderFinish(&s.residentSatinFinishBias, &s.residentSatinFinishBiasDayID, calendarDayID(now))
}

// NoteResidentDecorateOrderFinished records a successful decorate resident
// order whose response omitted the authoritative namespace-124 counter.
func (s *State) NoteResidentDecorateOrderFinished(now time.Time, raw json.RawMessage) {
	if responseStatisticsCounters(raw).decorate {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	noteResidentOrderFinish(&s.residentDecorateFinishBias, &s.residentDecorateFinishBiasDayID, calendarDayID(now))
}

// NoteCustomerOrderFinished records one successful customer-order finish when
// the response did not already carry an authoritative field-11 statistics
// update (ApplyV clears the bias in that case).
func (s *State) NoteCustomerOrderFinished(now time.Time, raw json.RawMessage) {
	if responseStatisticsCounters(raw).customer {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	noteResidentOrderFinish(&s.customerOrderFinishBias, &s.customerOrderFinishBiasDayID, gameDayID(now))
}

func noteResidentOrderFinish(bias, biasDayID *int32, day int32) {
	if *biasDayID != day {
		*biasDayID = day
		*bias = 0
	}
	if *bias < math.MaxInt32 {
		*bias++
	}
}

func responseStatisticsCounters(raw json.RawMessage) statisticsCountersSeen {
	if len(raw) == 0 || string(raw) == "null" {
		return statisticsCountersSeen{}
	}
	var top map[string]json.RawMessage
	if json.Unmarshal(raw, &top) != nil {
		return statisticsCountersSeen{}
	}
	raw124, ok := top["124"]
	if !ok {
		return statisticsCountersSeen{}
	}
	var ns map[string]json.RawMessage
	if json.Unmarshal(raw124, &ns) != nil {
		return statisticsCountersSeen{}
	}
	raw0, ok := ns["0"]
	if !ok {
		return statisticsCountersSeen{}
	}
	_, countersSeen, ok := parseStatisticsViewMerged(StatisticsView{}, raw0)
	if !ok {
		return statisticsCountersSeen{}
	}
	return countersSeen
}

// ResidentOrderFinishNum returns today's ordinary resident finish count from
// namespace 124 plus successful local finishes not reflected by that counter.
func (s *State) ResidentOrderFinishNum(now time.Time) int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return residentOrderFinishCount(s.statistics, s.statistics.OrderFlowerFinishNum, s.residentOrderFinishBias, s.residentOrderFinishBiasDayID, gameDayID(now))
}

// ResidentSatinOrderFinishNum returns today's satin resident finish count.
func (s *State) ResidentSatinOrderFinishNum(now time.Time) int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return residentOrderFinishCount(s.statistics, s.statistics.OrderSatinFinishNum, s.residentSatinFinishBias, s.residentSatinFinishBiasDayID, calendarDayID(now))
}

// ResidentDecorateOrderFinishNum returns today's decorate resident finish count.
func (s *State) ResidentDecorateOrderFinishNum(now time.Time) int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return residentOrderFinishCount(s.statistics, s.statistics.OrderDecorateFinishNum, s.residentDecorateFinishBias, s.residentDecorateFinishBiasDayID, calendarDayID(now))
}

// CustomerOrderFinishNum returns today's customer-order finish count from
// namespace 124 plus successful local finishes not reflected by that counter.
func (s *State) CustomerOrderFinishNum(now time.Time) int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return residentOrderFinishCount(s.statistics, s.statistics.OrderCustomerFinishNum, s.customerOrderFinishBias, s.customerOrderFinishBiasDayID, gameDayID(now))
}

func residentOrderFinishCount(stats StatisticsView, observed, bias, biasDayID, day int32) int32 {
	var count int64
	if stats.Observed && (stats.DayID == 0 || stats.DayID >= day) {
		count = int64(observed)
	}
	if biasDayID == day && (!stats.Observed || stats.DayID == 0 || stats.DayID <= day) {
		count += int64(bias)
	}
	if count < 0 {
		return 0
	}
	if count > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(count)
}

// ResidentSatinFinishNum returns today's satin resident finish count. Counts
// reset at 00:00 Asia/Shanghai; a prior-day statistics snapshot is treated as 0
// until the server publishes the new day's counters.
func (s *State) ResidentSatinFinishNum(now time.Time) int32 {
	return s.ResidentSatinOrderFinishNum(now)
}

// ResidentDecorateFinishNum returns today's decorate resident finish count.
// Counts reset at 00:00 Asia/Shanghai like satin orders.
func (s *State) ResidentDecorateFinishNum(now time.Time) int32 {
	return s.ResidentDecorateOrderFinishNum(now)
}

// ResidentOrderNormalDailyMax returns c_orderFlower.$dailyMax, the mini
// client's hard daily cap for normal resident orders.
func ResidentOrderNormalDailyMax() int32 {
	return residentOrderCatalogDailyMax("$dailyMax")
}

// ResidentOrderSatinDailyMax returns c_orderFlower.$dailyMax2, the mini
// client's hard daily cap for satin resident orders.
func ResidentOrderSatinDailyMax() int32 {
	return residentOrderCatalogDailyMax("$dailyMax2")
}

// ResidentOrderDecorateDailyMax returns c_orderFlower.$dailyMax3, the mini
// client's hard daily cap for decorate resident orders.
func ResidentOrderDecorateDailyMax() int32 {
	return residentOrderCatalogDailyMax("$dailyMax3")
}

func residentOrderCatalogDailyMax(key string) int32 {
	raw, ok := catalog.Tables["c_orderFlower"].Rows["-1"]
	if !ok {
		return 0
	}
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil {
		return 0
	}
	return readInt32Any(row[key])
}

// NextGameDayReset returns the next 00:05 Asia/Shanghai game-day boundary.
func NextGameDayReset(now time.Time) time.Time {
	local := now.In(gameDayLocation())
	y, m, d := local.Date()
	reset := time.Date(y, m, d, 0, 5, 0, 0, local.Location())
	if !reset.After(local) {
		reset = time.Date(y, m, d+1, 0, 5, 0, 0, local.Location())
	}
	return reset
}

// NextCalendarDayReset is the next 00:00 Asia/Shanghai boundary. Satin/decorate
// resident-order daily counters reset here.
func NextCalendarDayReset(now time.Time) time.Time {
	local := now.In(gameDayLocation())
	y, m, d := local.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, local.Location())
}

func gameDayID(now time.Time) int32 {
	local := now.In(gameDayLocation())
	// Game day rolls at 00:05 Asia/Shanghai; before that keep the previous day.
	if local.Hour() == 0 && local.Minute() < 5 {
		local = local.Add(-5 * time.Minute)
	}
	y, m, d := local.Date()
	return int32(y*10000 + int(m)*100 + d)
}

func calendarDayID(now time.Time) int32 {
	local := now.In(gameDayLocation())
	y, m, d := local.Date()
	return int32(y*10000 + int(m)*100 + d)
}

func gameDayLocation() *time.Location {
	return time.FixedZone("Asia/Shanghai", 8*60*60)
}

// FlowerOrders returns the current resident order requirements.
func (s *State) FlowerOrders() map[int32]*FlowerOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]*FlowerOrder, len(s.flowerOrders))
	for k, v := range s.flowerOrders {
		cp := *v
		cp.Requires = append([]FlowerRequire(nil), v.Requires...)
		cp.VideoRewards = append([]ItemCount(nil), v.VideoRewards...)
		out[k] = &cp
	}
	return out
}

// ResidentSatinOrder returns the latest observed satin resident order state.
func (s *State) ResidentSatinOrder() ResidentSpecialOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.residentSatinOrder
	out.Requires = append([]FlowerRequire(nil), s.residentSatinOrder.Requires...)
	out.VideoRewards = append([]ItemCount(nil), s.residentSatinOrder.VideoRewards...)
	return out
}

// ResidentDecorateOrder returns the latest observed decorate resident order state.
func (s *State) ResidentDecorateOrder() ResidentSpecialOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := s.residentDecorateOrder
	out.Requires = append([]FlowerRequire(nil), s.residentDecorateOrder.Requires...)
	out.VideoRewards = append([]ItemCount(nil), s.residentDecorateOrder.VideoRewards...)
	return out
}

// PalaceOrder returns the current palace order state.
func (s *State) PalaceOrder() PalaceOrderView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.palaceOrder
}

// TeamOrder returns the current team order state.
func (s *State) TeamOrder() TeamOrderView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.teamOrder
}

// Statistics returns the latest observed daily statistics snapshot.
func (s *State) Statistics() StatisticsView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statistics
}

// StatisticsDays returns observed daily statistics, newest day first.
func (s *State) StatisticsDays() []StatisticsView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.statisticsByDay) == 0 {
		if s.statistics.Observed {
			return []StatisticsView{s.statistics}
		}
		return nil
	}
	ids := make([]int32, 0, len(s.statisticsByDay))
	for dayID := range s.statisticsByDay {
		ids = append(ids, dayID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
	out := make([]StatisticsView, 0, len(ids))
	for _, dayID := range ids {
		out = append(out, s.statisticsByDay[dayID])
	}
	return out
}

// FlowerRackSlots returns the current flower-art shelf slots.
func (s *State) FlowerRackSlots() map[int32]FlowerRackSlot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]FlowerRackSlot, len(s.flowerRack))
	for k, v := range s.flowerRack {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}

// FlowerRackClaimableSlotIDs returns listed rack slots whose configured sale
// window has elapsed. The client treats a rack as sold when:
// now - sellStartTime >= num * c_flowerRack.$sellTime.
func (s *State) FlowerRackClaimableSlotIDs(now time.Time) []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nowMs := now.UnixMilli()
	out := make([]int32, 0)
	for rackID, slot := range s.flowerRack {
		if slot == nil || slot.ItemID <= 0 || slot.Count <= 0 || slot.SellReadyAtMs <= 0 || nowMs < slot.SellReadyAtMs {
			continue
		}
		out = append(out, rackID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// EmptyFlowerRackSlotIDs returns observed rack slots with no listed art.
func (s *State) EmptyFlowerRackSlotIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0)
	for rackID, slot := range s.flowerRack {
		if slot == nil || slot.ItemID != 0 || slot.Count != 0 {
			continue
		}
		out = append(out, rackID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyFlowerOrderAdBoxIDs returns ordinary resident-order slots whose current
// IFlowerOrder.isVideo flag requires a manual video action.
func (s *State) ReadyFlowerOrderAdBoxIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0)
	for id, order := range s.flowerOrders {
		if order != nil && order.IsVideo != 0 {
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ReadyFlowerOrderRewardTargets returns resident-order milestone rewards that
// are claimable from observed daily progress.
func (s *State) ReadyFlowerOrderRewardTargets() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var finished int32
	for _, task := range s.dailyTasks {
		if task != nil && task.TaskID == 30060001 && task.Finished > finished {
			finished = task.Finished
		}
	}
	if finished <= 0 {
		return nil
	}
	thresholds := []int32{15, 30, 45, 60}
	out := make([]int32, 0, len(thresholds))
	for idx, threshold := range thresholds {
		target := int32(idx + 1)
		if finished >= threshold && !s.flowerOrderRewardsReceived[target] {
			out = append(out, target)
		}
	}
	return out
}

// FlowerOrderDeficits returns flower ids whose long-lived requirements are not
// yet covered by current inventory. Customer orders are intentionally excluded:
// they should be completed from current stock/craft capacity or refreshed.
func (s *State) FlowerOrderDeficits() map[int32]int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	needed := make(map[int32]int32)
	addRequires := func(reqs []FlowerRequire) {
		for _, req := range reqs {
			if req.FlowerID == 0 || req.Count <= 0 {
				continue
			}
			needed[req.FlowerID] += req.Count
		}
	}
	for _, order := range s.flowerOrders {
		if order != nil {
			addRequires(order.Requires)
		}
	}
	if s.residentSatinOrder.Observed && s.residentSatinOrder.IsVideo == 0 {
		addRequires(s.residentSatinOrder.Requires)
	}
	if s.residentDecorateOrder.Observed && s.residentDecorateOrder.IsVideo == 0 {
		addRequires(s.residentDecorateOrder.Requires)
	}
	if s.mainTask != nil && s.mainTask.Valid && !s.mainTask.Complete && s.mainTask.ProgressObserved {
		if flowerID, missing, ok := MainTaskFlowerRequirement(s.mainTask.TaskID, s.mainTask.Finished); ok {
			needed[flowerID] += missing
		}
	}
	out := make(map[int32]int32)
	for flowerID, count := range needed {
		if have := s.inventory[flowerID]; have < count {
			out[flowerID] = count - have
		}
	}
	return out
}

// CustomerOrders returns the set of active customer order npcIds.
func (s *State) CustomerOrders() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0, len(s.customerOrders))
	for id := range s.customerOrders {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// CustomerOrderSummary returns namespace 109 metadata.
func (s *State) CustomerOrderSummary() CustomerOrderSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	summary := s.customerOrderSummary
	summary.ActiveCount = int32(len(s.customerOrders))
	return summary
}

// CustomerOrderGenerationReady reports whether ordinary customer orders can be
// requested now based on the observed client cooldown.
func (s *State) CustomerOrderGenerationReady(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.customerOrderSummary.Observed || len(s.customerOrders) > 0 {
		return false
	}
	next := s.customerOrderSummary.NextGenTimeMs
	return next <= 0 || now.UnixMilli() >= next+1000
}

// CustomerOrderDetails returns the active customer order requirements.
func (s *State) CustomerOrderDetails() map[int32]*CustomerOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int32]*CustomerOrder, len(s.customerOrders))
	for k, v := range s.customerOrders {
		if v == nil {
			continue
		}
		cp := *v
		cp.Requires = append([]FlowerRequire(nil), v.Requires...)
		cp.ItemRequires = append([]ItemRequire(nil), v.ItemRequires...)
		out[k] = &cp
	}
	return out
}
