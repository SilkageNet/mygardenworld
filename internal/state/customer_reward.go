package state

import (
	"encoding/json"
	"math"
)

// CustomerOrderFloralCoinReward matches the mini client's calOrderCustomerRwd:
// use a nonempty c_flowerArt.cPrice or the table's $cPrice fallback, then
// multiply by art count. Gold/experience bonuses and video doubling do not
// change this ordinary item-1002 reward. Unknown data is not a zero reward.
func CustomerOrderFloralCoinReward(order *CustomerOrder) (int64, bool) {
	if order == nil || len(order.Requires) > 0 || len(order.ItemRequires) == 0 {
		return 0, false
	}
	var total int64
	for _, req := range order.ItemRequires {
		if req.ItemID <= 0 || req.Count <= 0 {
			return 0, false
		}
		raw, ok := StaticRow("c_flowerArt", req.ItemID)
		if !ok {
			return 0, false
		}
		fallback, _ := StaticRow("c_flowerArt", -1)
		perArt, ok := customerArtFloralCoin(raw, fallback)
		if !ok || perArt > (math.MaxInt64-total)/int64(req.Count) {
			return 0, false
		}
		total += perArt * int64(req.Count)
	}
	return total, true
}

func customerArtFloralCoin(raw, fallback json.RawMessage) (int64, bool) {
	var row struct {
		Prices [][]int64 `json:"cPrice"`
	}
	if json.Unmarshal(raw, &row) != nil {
		return 0, false
	}
	prices := row.Prices
	if len(prices) == 0 {
		var defaults struct {
			Price []int64 `json:"$cPrice"`
		}
		if json.Unmarshal(fallback, &defaults) != nil || len(defaults.Price) != 2 {
			return 0, false
		}
		prices = [][]int64{defaults.Price}
	}
	var coins int64
	for _, price := range prices {
		if len(price) != 2 || price[0] <= 0 || price[1] < 0 {
			return 0, false
		}
		if price[0] == 1002 {
			if price[1] > math.MaxInt64-coins {
				return 0, false
			}
			coins += price[1]
		}
	}
	return coins, true
}
