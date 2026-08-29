package state

import (
	"encoding/json"
	"time"
)

const (
	ZooFoodShopTempID       int32 = 9
	ZooNormalFoodShopItemID int32 = 90001
)

type shopState struct {
	tempID              int32
	resetMs             int64
	dailyBought         map[int32]int32
	dailyRecordObserved bool
	observed            bool
	fullSyncedAtMs      int64
}

// applyShopsLocked consumes namespace 20 (IShopTot.map). Login/enter rows are
// full IShop snapshots, while shop.buy commonly patches only dRecord. Full
// rows replace the daily record; sparse rows merge it like updateMbMap.
func (s *State) applyShopsLocked(raw json.RawMessage, appliedAtMs int64) {
	var total map[string]json.RawMessage
	if err := json.Unmarshal(raw, &total); err != nil {
		return
	}
	rawMap, ok := total["0"]
	if !ok {
		return
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(rawMap, &entries); err != nil {
		return
	}
	if s.shops == nil {
		s.shops = make(map[int32]*shopState)
	}
	for key, rawShop := range entries {
		tempID := atoi32(key)
		if tempID <= 0 {
			continue
		}
		if isJSONNull(rawShop) {
			delete(s.shops, tempID)
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(rawShop, &fields); err != nil {
			continue
		}
		shop := s.shops[tempID]
		if shop == nil {
			shop = &shopState{tempID: tempID, dailyBought: make(map[int32]int32)}
			s.shops[tempID] = shop
		}
		if observedTempID, ok := readInt32JSONField(fields, "1"); ok && observedTempID > 0 {
			shop.tempID = observedTempID
		}
		fullSnapshot := shopFullSnapshot(fields)
		if resetMs, ok := readInt64JSONField(fields, "3"); ok {
			if shop.resetMs > 0 && resetMs > 0 && gameDayID(time.UnixMilli(resetMs)) > gameDayID(time.UnixMilli(shop.resetMs)) {
				shop.dailyBought = make(map[int32]int32)
			}
			shop.resetMs = resetMs
		}
		if rawDaily, ok := fields["12"]; ok {
			if fullSnapshot || isJSONNull(rawDaily) || !shop.dailyRecordObserved {
				shop.dailyBought = readInt32RawMap(rawDaily)
			} else {
				for itemID, count := range readInt32RawMap(rawDaily) {
					shop.dailyBought[itemID] = count
				}
			}
			shop.dailyRecordObserved = true
		} else if fullSnapshot {
			// An omitted dRecord on an authoritative login/enter row means the
			// daily record is empty; do not spin on shop.enter waiting for {}.
			shop.dailyBought = make(map[int32]int32)
			shop.dailyRecordObserved = true
		}
		if fullSnapshot {
			shop.fullSyncedAtMs = appliedAtMs
		}
		shop.observed = true
	}
}

func shopFullSnapshot(fields map[string]json.RawMessage) bool {
	// tempId/list/reset/refresh counters are row-level data sent by login and
	// shop.enter. A shop.buy dRecord delta does not carry these fields.
	for _, key := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "16"} {
		if _, ok := fields[key]; ok {
			return true
		}
	}
	return false
}

// ZooFoodShop returns the normal gold-food offer and current daily quota.
// Diamond food is deliberately excluded from automation.
func (s *State) ZooFoodShop(now time.Time) ZooFoodShopView {
	view := zooNormalFoodStatic()
	s.mu.RLock()
	defer s.mu.RUnlock()
	shop := s.shops[ZooFoodShopTempID]
	if shop == nil || !shop.observed {
		view.NeedsEnter = true
		return view
	}
	view.Observed = true
	view.DailyBought = shop.dailyBought[ZooNormalFoodShopItemID]
	if view.DailyLimit > 0 {
		view.DailyRemaining = view.DailyLimit - view.DailyBought
		if view.DailyRemaining < 0 {
			view.DailyRemaining = 0
		}
	}
	view.NeedsEnter = !shop.dailyRecordObserved || shop.fullSyncedAtMs <= 0 ||
		gameDayID(now) > gameDayID(time.UnixMilli(shop.fullSyncedAtMs))
	return view
}

func zooNormalFoodStatic() ZooFoodShopView {
	view := ZooFoodShopView{
		ShopTempID: ZooFoodShopTempID,
		ShopItemID: ZooNormalFoodShopItemID,
	}
	rawRow, ok := StaticRow("c_shop_item_9", ZooNormalFoodShopItemID)
	if !ok {
		return view
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(rawRow, &row) != nil {
		return view
	}
	if rawItems, ok := row["items"]; ok {
		var stacks []json.RawMessage
		if json.Unmarshal(rawItems, &stacks) == nil && len(stacks) > 0 {
			parts := readInt32OrderedListRaw(stacks[0])
			if len(parts) >= 2 {
				view.FoodstuffID, view.FoodstuffCount = parts[0], parts[1]
			}
		}
	}
	if rawCosts, ok := row["costs"]; ok {
		var stacks []json.RawMessage
		if json.Unmarshal(rawCosts, &stacks) == nil && len(stacks) > 0 {
			parts := readInt32OrderedListRaw(stacks[0])
			if len(parts) >= 2 && parts[0] == 11 {
				view.GoldCost = parts[1]
			}
		}
	}
	if rawLimit, ok := row["dLimit"]; ok {
		parts := readInt32OrderedListRaw(rawLimit)
		if len(parts) > 0 {
			view.DailyLimit = parts[0]
			view.DailyRemaining = parts[0]
		}
	}
	return view
}
