package state

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"time"
)

const (
	// ZooFoodShopTempID is c_shop id 9 / shop.enter tempId for pet food.
	ZooFoodShopTempID int32 = 9
	// ZooFoodShopItemCatFood is c_shop_item_9.90001 → inventory item 1501 (猫粮).
	ZooFoodShopItemCatFood int32 = 90001
	// ZooFoodItemCatFood is the inventory item granted by 90001.
	ZooFoodItemCatFood int32 = 1501
)

// ZooFoodShopOffer is the gold-only cat-food SKU from c_shop_item_9.
type ZooFoodShopOffer struct {
	ShopItemID int32
	ItemID     int32
	ItemCount  int32
	CostItemID int32
	CostCount  int32
	DailyLimit int32
	Bought     int32
	Remaining  int32
}

// ZooFoodBuyPlan is one shop.buy for cat food.
type ZooFoodBuyPlan struct {
	ShopTempID int32
	ShopItemID int32
	ItemID     int32
	Count      int32
	GoldCost   int32
}

func (s *State) applyShopTotLocked(raw json.RawMessage) {
	var ns20 map[string]json.RawMessage
	if json.Unmarshal(raw, &ns20) != nil || ns20 == nil {
		return
	}
	rawMap, ok := ns20["0"]
	if !ok {
		return
	}
	var shops map[string]json.RawMessage
	if json.Unmarshal(rawMap, &shops) != nil || shops == nil {
		return
	}
	rawShop, ok := shops[strconvFormatInt32(ZooFoodShopTempID)]
	if !ok {
		return
	}
	if bytesEqualTrimNull(rawShop) {
		s.zooFoodShopObserved = true
		s.zooFoodShopDRecord = make(map[int32]int32)
		s.zooFoodShopResetMs = 0
		return
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(rawShop, &fields) != nil || fields == nil {
		return
	}
	s.zooFoodShopObserved = true
	if rawReset, ok := fields["3"]; ok {
		if n, ok := readInt64Raw(rawReset); ok {
			s.zooFoodShopResetMs = n
		}
	}
	if rawDaily, ok := fields["12"]; ok {
		s.zooFoodShopDRecord = readInt32RawMap(rawDaily)
	}
}

// ZooFoodShopObserved reports whether shop tempId=9 has been entered/synced.
func (s *State) ZooFoodShopObserved() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.zooFoodShopObserved
}

// ZooFoodShopNeedsEnter is true when pet-food shop buy records are unknown or
// the observed daily reset marker is from a prior calendar day.
func (s *State) ZooFoodShopNeedsEnter(now time.Time) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.zooFoodShopObserved {
		return true
	}
	if s.zooFoodShopResetMs <= 0 {
		return false
	}
	return !sameLocalDay(s.zooFoodShopResetMs, now)
}

// ZooFoodShopCatFoodOffer returns the gold-only 猫粮 SKU enriched with daily
// remaining from observed dRecord. Diamond 小鱼干 (90002) is intentionally omitted.
func (s *State) ZooFoodShopCatFoodOffer() (ZooFoodShopOffer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	offer, ok := zooFoodShopCatFoodStatic()
	if !ok {
		return ZooFoodShopOffer{}, false
	}
	offer.Bought = s.zooFoodShopDRecord[offer.ShopItemID]
	if offer.DailyLimit > 0 {
		offer.Remaining = offer.DailyLimit - offer.Bought
		if offer.Remaining < 0 {
			offer.Remaining = 0
		}
	} else {
		offer.Remaining = 30
	}
	return offer, true
}

// NextZooFoodBuyPlan returns one gold-backed cat-food purchase when pets need
// bowl stocking and inventory cannot cover the empty slots.
func (s *State) NextZooFoodBuyPlan() (ZooFoodBuyPlan, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.zooFoodShopObserved {
		return ZooFoodBuyPlan{}, false
	}
	offer, ok := zooFoodShopCatFoodStatic()
	if !ok || offer.CostItemID != 11 || offer.CostCount <= 0 || offer.ItemID != ZooFoodItemCatFood {
		return ZooFoodBuyPlan{}, false
	}
	needed := s.zooFoodInventoryDeficitLocked()
	if needed <= 0 {
		return ZooFoodBuyPlan{}, false
	}
	bought := s.zooFoodShopDRecord[offer.ShopItemID]
	remaining := offer.DailyLimit - bought
	if offer.DailyLimit <= 0 {
		remaining = needed
	}
	if remaining <= 0 {
		return ZooFoodBuyPlan{}, false
	}
	count := needed
	if count > remaining {
		count = remaining
	}
	if count > 10 {
		count = 10
	}
	goldEach := offer.CostCount
	totalGold := goldEach * count
	if s.gold < totalGold {
		affordable := s.gold / goldEach
		if affordable <= 0 {
			return ZooFoodBuyPlan{}, false
		}
		count = affordable
		totalGold = goldEach * count
	}
	return ZooFoodBuyPlan{
		ShopTempID: ZooFoodShopTempID,
		ShopItemID: offer.ShopItemID,
		ItemID:     offer.ItemID,
		Count:      count,
		GoldCost:   totalGold,
	}, true
}

func (s *State) zooFoodInventoryDeficitLocked() int32 {
	capacity := ZooFoodBowlCapacity()
	satietyMax := ZooSatietyMax()
	if capacity <= 0 || satietyMax <= 0 {
		return 0
	}
	available := s.inventory[ZooFoodItemCatFood] + s.inventory[1502]
	var emptySlots int32
	petIDs := make([]int32, 0, len(s.zooPets))
	for petID, pet := range s.zooPets {
		if pet == nil || pet.PetID <= 0 || !pet.FoodstuffObserved || !pet.StatusObserved || !pet.SatietyObserved {
			continue
		}
		if !zooPetCanEat(pet.Status) || pet.SatietyValue >= satietyMax {
			continue
		}
		petIDs = append(petIDs, petID)
	}
	sort.Slice(petIDs, func(i, j int) bool { return petIDs[i] < petIDs[j] })
	for _, petID := range petIDs {
		pet := s.zooPets[petID]
		empty := capacity - int32(len(pet.FoodstuffIDs))
		if empty > 0 {
			emptySlots += empty
		}
	}
	if emptySlots <= available {
		return 0
	}
	return emptySlots - available
}

func zooFoodShopCatFoodStatic() (ZooFoodShopOffer, bool) {
	raw, ok := StaticRow("c_shop_item_9", ZooFoodShopItemCatFood)
	if !ok {
		return ZooFoodShopOffer{}, false
	}
	var row map[string]json.RawMessage
	if json.Unmarshal(raw, &row) != nil {
		return ZooFoodShopOffer{}, false
	}
	rowID, idOK := readInt32JSONField(row, "id")
	if !idOK || rowID != ZooFoodShopItemCatFood {
		return ZooFoodShopOffer{}, false
	}
	var itemStacks []json.RawMessage
	if json.Unmarshal(row["items"], &itemStacks) != nil || len(itemStacks) == 0 {
		return ZooFoodShopOffer{}, false
	}
	itemParts := readInt32OrderedListRaw(itemStacks[0])
	if len(itemParts) < 2 || itemParts[0] != ZooFoodItemCatFood || itemParts[1] <= 0 {
		return ZooFoodShopOffer{}, false
	}
	var costStacks []json.RawMessage
	if json.Unmarshal(row["costs"], &costStacks) != nil || len(costStacks) != 1 {
		return ZooFoodShopOffer{}, false
	}
	costParts := readInt32OrderedListRaw(costStacks[0])
	if len(costParts) < 2 || costParts[0] != 11 || costParts[1] <= 0 {
		return ZooFoodShopOffer{}, false
	}
	dailyLimit := int32(0)
	if rawLimit, ok := row["dLimit"]; ok {
		parts := readInt32OrderedListRaw(rawLimit)
		if len(parts) > 0 && parts[0] > 0 {
			dailyLimit = parts[0]
		}
	}
	return ZooFoodShopOffer{
		ShopItemID: ZooFoodShopItemCatFood,
		ItemID:     itemParts[0],
		ItemCount:  itemParts[1],
		CostItemID: costParts[0],
		CostCount:  costParts[1],
		DailyLimit: dailyLimit,
	}, true
}

func strconvFormatInt32(v int32) string {
	return strconv.FormatInt(int64(v), 10)
}

func bytesEqualTrimNull(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return bytes.Equal(trimmed, []byte("null"))
}
