package state

import (
	"encoding/json"
	"sort"
	"strconv"
	"time"
)

const cyclicStoryTmpType int32 = 4003

type cyclicStoryActivityState struct {
	Orders              map[int32]cyclicStoryOrderState
	OrdersObserved      bool
	OrdersValid         bool
	FinishCount         int32
	FinishCountObserved bool
	FinishCountValid    bool
	LastRefreshTimeMs   int64
	ExpOrderNum         int32
	ExpOrderNumObserved bool
	ExpOrderNumValid    bool
}

type cyclicStoryOrderState struct {
	OrderID   int32
	FlowerID  int32
	OrderTime int64
	ValidTime int64
	Valid     bool
}

func mergeCyclicStoryExtension(batch *activityBatchState, raw json.RawMessage) {
	var ext map[string]json.RawMessage
	if json.Unmarshal(raw, &ext) != nil || ext == nil {
		return
	}
	rawStory, present := ext["106"]
	if !present {
		return
	}
	var story map[string]json.RawMessage
	if json.Unmarshal(rawStory, &story) != nil || story == nil {
		return
	}
	if rawOrders, present := story["0"]; present {
		orders, parsed, valid := decodeCyclicStoryOrders(rawOrders)
		batch.Story.Orders = orders
		batch.Story.OrdersObserved = true
		batch.Story.OrdersValid = parsed && valid
	}
	if rawFinishCount, present := story["1"]; present {
		value, valid := readActivityInt32Raw(rawFinishCount)
		batch.Story.FinishCountObserved = true
		batch.Story.FinishCountValid = valid && value >= 0
		if batch.Story.FinishCountValid {
			batch.Story.FinishCount = value
		} else {
			batch.Story.FinishCount = 0
		}
	}
	if value, ok := readActivityInt64Field(story, "2"); ok {
		batch.Story.LastRefreshTimeMs = value
	}
	if rawExp, present := story["3"]; present {
		value, valid := readActivityInt32Raw(rawExp)
		batch.Story.ExpOrderNumObserved = true
		batch.Story.ExpOrderNumValid = valid && value >= 0
		if batch.Story.ExpOrderNumValid {
			batch.Story.ExpOrderNum = value
		} else {
			batch.Story.ExpOrderNum = 0
		}
	}
}

func decodeCyclicStoryOrders(raw json.RawMessage) (map[int32]cyclicStoryOrderState, bool, bool) {
	if isJSONNull(raw) {
		return map[int32]cyclicStoryOrderState{}, true, true
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return nil, false, false
	}
	out := make(map[int32]cyclicStoryOrderState, len(fields))
	valid := true
	for key, rawOrder := range fields {
		idx64, err := strconv.ParseInt(key, 10, 32)
		if err != nil || idx64 < 0 || strconv.FormatInt(idx64, 10) != key {
			valid = false
			continue
		}
		if isJSONNull(rawOrder) {
			continue
		}
		order, orderOK := decodeCyclicStoryOrder(rawOrder)
		if !orderOK {
			valid = false
			out[int32(idx64)] = cyclicStoryOrderState{Valid: false}
			continue
		}
		out[int32(idx64)] = order
	}
	return out, true, valid
}

func decodeCyclicStoryOrder(raw json.RawMessage) (cyclicStoryOrderState, bool) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || fields == nil {
		return cyclicStoryOrderState{}, false
	}
	orderID, orderOK := readActivityInt32Field(fields, "0")
	flowerID, flowerOK := readActivityInt32Field(fields, "1")
	orderTime, timeOK := readActivityInt64Field(fields, "2")
	validTime, validOK := readActivityInt64Field(fields, "3")
	if !orderOK || !flowerOK || !timeOK || !validOK || orderID < 0 || flowerID < 0 || orderTime < 0 || validTime < 0 {
		return cyclicStoryOrderState{}, false
	}
	return cyclicStoryOrderState{
		OrderID: orderID, FlowerID: flowerID, OrderTime: orderTime, ValidTime: validTime, Valid: true,
	}, true
}

// cyclicStoryTimestampAfter reports whether ts is strictly after now.
// Newer batches send millisecond timestamps; older captures used seconds.
func cyclicStoryTimestampAfter(ts int64, now time.Time) bool {
	until := CyclicStoryValidUntil(ts)
	return !until.IsZero() && until.After(now)
}

// CyclicStoryValidUntil converts an order validTime (ms or sec) into a wall
// clock. Zero/negative inputs mean "already eligible".
func CyclicStoryValidUntil(ts int64) time.Time {
	if ts <= 0 {
		return time.Time{}
	}
	if ts >= 1_000_000_000_000 {
		return time.UnixMilli(ts)
	}
	return time.Unix(ts, 0)
}

// CyclicStoryView returns the currently preferred 莳花纪闻 activity snapshot.
func (s *State) CyclicStoryView(now time.Time) (CyclicStoryView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := CyclicStoryView{Observed: s.activityObserved}
	batch, phase, visibleStart, graceEnd, phaseEnd := s.preferredCyclicStoryBatchLocked(now.UnixMilli())
	if batch == nil {
		return out, false
	}
	out.Found = true
	out.BatchID = batch.BatchID
	out.TmpID = batch.TmpID
	out.TmpType = batch.TmpType
	out.Status = batch.Status
	out.Phase = phase
	out.VisibleStartMs = visibleStart
	out.BeginMs = batch.BeginMs
	out.EndMs = batch.EndMs
	out.GraceEndMs = graceEnd
	out.PhaseEndMs = phaseEnd
	out.Score = batch.Score
	out.Bag = cloneInt32Map(batch.Bag)
	out.FinishCount = batch.Story.FinishCount
	out.FinishCountObserved = batch.Story.FinishCountObserved && batch.Story.FinishCountValid
	out.ExpOrderNum = batch.Story.ExpOrderNum
	out.ExpOrderNumObserved = batch.Story.ExpOrderNumObserved && batch.Story.ExpOrderNumValid
	out.LastRefreshTimeMs = batch.Story.LastRefreshTimeMs
	out.OrdersObserved = batch.Story.OrdersObserved
	out.OrdersValid = batch.Story.OrdersValid
	out.MilestoneReceiptsObserved = batch.BoxesObserved && batch.BoxesValid
	out.ClaimedMilestoneIndexes = append([]int32(nil), batch.ClaimedBoxes...)

	config, catalogOK := CyclicStoryCatalogConfig()
	if catalogOK {
		out.CurrencyItemID = config.CurrencyItemID
		out.CurrencyBalance = out.Bag[config.CurrencyItemID]
		out.Name = config.Name
	}
	template := s.activityTemplates[batch.TmpID]
	if template != nil {
		if template.Name != "" {
			out.Name = template.Name
		}
		out.Description = template.Description
	}

	identityReady := catalogOK && config.TmpType == batch.TmpType && batch.BatchID > 0 && batch.IdentityValid && batch.TmpID > 0 &&
		batch.Status == 1 && batch.BeginMs > 0 && batch.EndMs > batch.BeginMs && batch.DurationBeforeMs >= 0 &&
		batch.DurationAfterMs >= 0 &&
		template != nil && template.IdentityValid && template.TmpID == batch.TmpID && template.TmpType == config.TmpType &&
		template.BoxesObserved && template.BoxesValid
	// Enter can bootstrap a fresh batch before score/bag/orders arrive.
	out.EnterReady = identityReady && (phase == 2 || phase == 3)
	out.Valid = identityReady && batch.ScoreObserved && batch.ScoreValid && batch.BagObserved && batch.BagValid &&
		(!batch.Story.FinishCountObserved || batch.Story.FinishCountValid) &&
		(!batch.Story.ExpOrderNumObserved || batch.Story.ExpOrderNumValid) &&
		(!batch.BoxesObserved || batch.BoxesValid) &&
		(!batch.Story.OrdersObserved || batch.Story.OrdersValid)
	if batch.Story.OrdersObserved && batch.Story.OrdersValid {
		indexes := make([]int32, 0, len(batch.Story.Orders))
		for idx := range batch.Story.Orders {
			indexes = append(indexes, idx)
		}
		sort.Slice(indexes, func(i, j int) bool { return indexes[i] < indexes[j] })
		out.Orders = make([]CyclicStoryOrderView, 0, len(indexes))
		for _, idx := range indexes {
			raw := batch.Story.Orders[idx]
			order := CyclicStoryOrderView{
				OrderIdx: idx, OrderID: raw.OrderID, FlowerID: raw.FlowerID,
				OrderTime: raw.OrderTime, ValidTime: raw.ValidTime,
			}
			if !raw.Valid {
				out.Valid = false
				out.Orders = append(out.Orders, order)
				continue
			}
			if raw.OrderID > 0 {
				info := CyclicStoryOrderInfoByID(raw.OrderID)
				order.Cost = info.Cost
				order.CatalogKnown = info.CatalogKnown
				order.Reward = cloneCyclicNoteItems(info.Reward)
			}
			// Official client gates claim with validTime <= now for every slot.
			// Empty slots use a future validTime as refresh cooldown; freshly
			// rolled active orders also carry refreshCd before they are claimable
			// (c_actCyclicStory.$refreshCd, observed as orderTime+1500s).
			order.OnCooldown = cyclicStoryTimestampAfter(raw.ValidTime, now)
			out.Orders = append(out.Orders, order)
		}
	}
	if template != nil && template.BoxesObserved && template.BoxesValid {
		claimed := make(map[int32]struct{}, len(batch.ClaimedBoxes))
		for _, index := range batch.ClaimedBoxes {
			claimed[index] = struct{}{}
		}
		out.Milestones = make([]CyclicNoteMilestoneView, 0, len(template.Milestones))
		for _, milestone := range template.Milestones {
			_, received := claimed[milestone.Index]
			out.Milestones = append(out.Milestones, CyclicNoteMilestoneView{
				Index: milestone.Index, Target: milestone.Target, Received: received,
				Reward: cloneCyclicNoteItems(milestone.Reward),
			})
		}
	}
	return out, true
}

func (s *State) preferredCyclicStoryBatchLocked(nowMs int64) (*activityBatchState, int32, int64, int64, int64) {
	var selected *activityBatchState
	var selectedPhase int32
	var selectedVisibleStart, selectedGraceEnd, selectedPhaseEnd int64
	for _, batch := range s.activityBatches {
		if batch == nil || batch.TmpType != cyclicStoryTmpType || batch.Status != 1 {
			continue
		}
		phase, visibleStart, graceEnd, phaseEnd, ok := cyclicNotePhase(batch, nowMs)
		if !ok || (phase != 1 && phase != 2 && phase != 3) {
			continue
		}
		if selected != nil && !preferCyclicNoteBatch(batch, phase, selected, selectedPhase) {
			continue
		}
		selected = batch
		selectedPhase = phase
		selectedVisibleStart = visibleStart
		selectedGraceEnd = graceEnd
		selectedPhaseEnd = phaseEnd
	}
	return selected, selectedPhase, selectedVisibleStart, selectedGraceEnd, selectedPhaseEnd
}
