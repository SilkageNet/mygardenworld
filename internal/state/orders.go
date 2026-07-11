package state

import (
	"encoding/json"
	"sort"
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
				if rawMode, ok := fields["0"]; ok {
					_ = json.Unmarshal(rawMode, &order.Mode)
				}
				// field "2" = [[flowerId, count], ...]
				if rawReqs, ok := fields["2"]; ok {
					order.Requires = parseFlowerRequires(rawReqs)
				}
				if rawCdTime, ok := fields["4"]; ok {
					_ = json.Unmarshal(rawCdTime, &order.CdTimeMs)
				}
				if rawCTime, ok := fields["5"]; ok {
					_ = json.Unmarshal(rawCTime, &order.CTimeMs)
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
	if n, ok := readInt32JSONField(fields, "0"); ok {
		view.Flowers = n
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
	if n, ok := readInt32JSONField(fields, "5"); ok {
		view.VideoRwd = n
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
	if json.Unmarshal(raw, &ns129) != nil {
		return
	}
	s.randomEventObserved = true
	raw0, ok := ns129["0"]
	if !ok {
		return
	}
	var inner map[string]json.RawMessage
	if json.Unmarshal(raw0, &inner) != nil {
		return
	}
	rawEvents, ok := inner["1"]
	if !ok {
		return
	}
	var events map[string]json.RawMessage
	if json.Unmarshal(rawEvents, &events) != nil {
		return
	}
	s.randomEvents = make(map[int32]*RandomEventView, len(events))
	for idStr, rawEvent := range events {
		id := atoi32(idStr)
		if id == 0 {
			continue
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(rawEvent, &fields) != nil {
			continue
		}
		event := &RandomEventView{EventID: id}
		if rawID, ok := fields["0"]; ok {
			_ = json.Unmarshal(rawID, &event.EventID)
		}
		if rawStatus, ok := fields["1"]; ok {
			_ = json.Unmarshal(rawStatus, &event.Status)
		}
		if rawAffair, ok := fields["2"]; ok {
			_ = json.Unmarshal(rawAffair, &event.Affair)
		}
		s.randomEvents[id] = event
	}
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
	s.residentOrderLimitUntilMs = nextGameDayReset(now).UnixMilli()
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

// ResidentOrderNormalDailyMax returns c_orderFlower.$dailyMax, the mini
// client's hard daily cap for normal resident orders.
func ResidentOrderNormalDailyMax() int32 {
	raw, ok := catalog.Tables["c_orderFlower"].Rows["-1"]
	if !ok {
		return 0
	}
	var row map[string]any
	if json.Unmarshal(raw, &row) != nil {
		return 0
	}
	return readInt32Any(row["$dailyMax"])
}

func nextGameDayReset(now time.Time) time.Time {
	local := now.In(gameDayLocation())
	y, m, d := local.Date()
	return time.Date(y, m, d+1, 0, 5, 0, 0, local.Location())
}

func gameDayID(now time.Time) int32 {
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
		out[k] = &cp
	}
	return out
}

// ResidentSatinOrder returns the latest observed satin resident order state.
func (s *State) ResidentSatinOrder() ResidentSpecialOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.residentSatinOrder
}

// ResidentDecorateOrder returns the latest observed decorate resident order state.
func (s *State) ResidentDecorateOrder() ResidentSpecialOrder {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.residentDecorateOrder
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

// ReadyFlowerOrderAdBoxIDs returns resident-order boxes that currently present
// the client as a video/share reward before a concrete order is generated.
func (s *State) ReadyFlowerOrderAdBoxIDs() []int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]int32, 0)
	for id, order := range s.flowerOrders {
		if order != nil && order.Mode == 8 && len(order.Requires) == 0 {
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
