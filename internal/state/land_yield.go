package state

import "time"

// LandRemainingYield is flowers still expected from this planting after
// completed harvest rounds and friend steals (each steal UID removes one).
// remaining = max(0, (frequencys - harvestCnt) * cropGets - len(stealUids)).
func LandRemainingYield(land LandView) int32 {
	yield, ok := landFlowerYield(land)
	if !ok {
		return 0
	}
	harvested := int32(land.HarvestCnt)
	if harvested < 0 {
		harvested = 0
	}
	if harvested > yield.Frequencys {
		harvested = yield.Frequencys
	}
	roundsLeft := yield.Frequencys - harvested
	if roundsLeft <= 0 {
		return 0
	}
	stolen := stealCountCapped(land, yield.CropGets)
	rem := roundsLeft*yield.CropGets - stolen
	if rem < 0 {
		return 0
	}
	return rem
}

// LandCanTouch is how many flowers friends can still steal from the current
// mature round: max(0, cropGets - len(stealUids)). Zero while growing.
func LandCanTouch(land LandView, now time.Time) int32 {
	if !landMatureForSteal(land, now.UnixMilli()) {
		return 0
	}
	yield, ok := landFlowerYield(land)
	if !ok {
		return 0
	}
	left := yield.CropGets - stealCountCapped(land, yield.CropGets)
	if left < 0 {
		return 0
	}
	return left
}

func landFlowerYield(land LandView) (FlowerLvlYield, bool) {
	if !land.IsPlanted() {
		return FlowerLvlYield{}, false
	}
	yield, ok := FlowerLvlYieldByID(int32(land.FlowerID), int32(land.Lvl))
	if !ok || yield.CropGets <= 0 || yield.Frequencys <= 0 {
		return FlowerLvlYield{}, false
	}
	return yield, true
}

func stealCountCapped(land LandView, cropGets int32) int32 {
	stolen := int32(len(land.StealUIDs))
	if stolen < 0 {
		stolen = 0
	}
	if stolen > cropGets {
		stolen = cropGets
	}
	return stolen
}

func landMatureForSteal(land LandView, nowMs int64) bool {
	if !land.IsPlanted() {
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
