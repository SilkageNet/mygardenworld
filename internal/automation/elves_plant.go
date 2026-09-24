package automation

import (
	"sort"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

const (
	elvesPlantPriority        = int32(9800)
	elvesPlantHarvestPriority = int32(10100)
	elvesNightHarvestPriority = int32(10200)
	elvesNightHarvestHour     = 22
	elvesNightHarvestGoal     = "elves_night_harvest"
)

func elvesPlantPolicy(plant *pb.PlantPolicy) *pb.ElvesPlantPolicy {
	if plant == nil {
		return nil
	}
	return plant.GetElvesPlant()
}

func elvesPlantActive(p *pb.ElvesPlantPolicy) bool {
	return p != nil && p.GetEnabled() && p.GetMainFlowerId() > 0 && p.GetSecondaryFlowerId() > 0
}

func elvesPlantMainLandCount(p *pb.ElvesPlantPolicy, totalLands int) int {
	if p == nil {
		return 0
	}
	n := int(p.GetMainLandCount())
	if n <= 0 {
		n = 4
	}
	if totalLands > 0 && n >= totalLands {
		n = totalLands - 1
		if n < 0 {
			n = 0
		}
	}
	return n
}

// elvesPlantDelayedHarvest reports whether secondary harvest is owned by the
// elves-plant module (independent of auto_harvest).
func elvesPlantDelayedHarvest(p *pb.ElvesPlantPolicy) bool {
	return elvesPlantActive(p) && p.GetHarvestDelaySeconds() > 0
}

// elvesNightHarvestEnabled is the 22:00 own-land elf harvest switch. It does
// not require ElvesPlantPolicy.enabled.
func elvesNightHarvestEnabled(p *pb.ElvesPlantPolicy) bool {
	return p != nil && p.GetNightHarvestEnabled()
}

// elvesNightHarvestOpen is 22:00–24:00 Asia/Shanghai. A late tick still
// collects; after midnight the window closes until the next evening.
func elvesNightHarvestOpen(now time.Time) bool {
	return now.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Hour() >= elvesNightHarvestHour
}

// elvesNightHarvestLandIDs lists own lands that currently have a flower elf
// and are harvestable immediately (configured harvest delay is ignored).
func elvesNightHarvestLandIDs(s *state.State, p *pb.ElvesPlantPolicy, now time.Time) []int32 {
	if s == nil || !elvesNightHarvestEnabled(p) || !elvesNightHarvestOpen(now) {
		return nil
	}
	lands := s.Lands()
	ids := make([]int32, 0)
	for id, land := range lands {
		if land.ElvesID == 0 {
			continue
		}
		kind, _ := Recommend(land, now, 0)
		if kind != KindHarvest {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func filterOutLandIDs(landIDs, drop []int32) []int32 {
	if len(drop) == 0 || len(landIDs) == 0 {
		return landIDs
	}
	skip := make(map[int32]struct{}, len(drop))
	for _, id := range drop {
		skip[id] = struct{}{}
	}
	out := make([]int32, 0, len(landIDs))
	for _, id := range landIDs {
		if _, ok := skip[id]; ok {
			continue
		}
		out = append(out, id)
	}
	return out
}

// elvesSecondaryFirstBloom is the post-water initial mature round. That bloom
// does not spawn flower elves; elves appear on the second (regrow) maturity.
func elvesSecondaryFirstBloom(land state.LandView) bool {
	return land.State == 3 && land.HarvestCnt == 0 && land.ElvesID == 0
}

// elvesPlantHarvestDelayForLand picks the harvest delay for a secondary land.
// First watering-round bloom: immediate. Later maturity (elves round): configured delay.
func elvesPlantHarvestDelayForLand(p *pb.ElvesPlantPolicy, land state.LandView) time.Duration {
	if !elvesPlantDelayedHarvest(p) {
		return 0
	}
	if elvesSecondaryFirstBloom(land) {
		return 0
	}
	return time.Duration(p.GetHarvestDelaySeconds()) * time.Second
}

func filterOutFlowerLandIDs(s *state.State, landIDs []int32, flowerID int32) []int32 {
	if flowerID <= 0 || len(landIDs) == 0 {
		return landIDs
	}
	lands := s.Lands()
	out := make([]int32, 0, len(landIDs))
	for _, id := range landIDs {
		if int32(lands[id].FlowerID) == flowerID {
			continue
		}
		out = append(out, id)
	}
	return out
}

func filterOnlyFlowerLandIDs(s *state.State, landIDs []int32, flowerID int32) []int32 {
	if flowerID <= 0 || len(landIDs) == 0 {
		return nil
	}
	lands := s.Lands()
	out := make([]int32, 0, len(landIDs))
	for _, id := range landIDs {
		if int32(lands[id].FlowerID) == flowerID {
			out = append(out, id)
		}
	}
	return out
}

// elvesPlantPlantOps fills empty lands: keep main_land_count main flowers, rest secondary.
func elvesPlantPlantOps(s *state.State, p *pb.ElvesPlantPolicy, empty []int32) []PlannedOp {
	if !elvesPlantActive(p) || len(empty) == 0 {
		return nil
	}
	lands := s.Lands()
	mainID := p.GetMainFlowerId()
	secID := p.GetSecondaryFlowerId()
	mainCount := 0
	for _, land := range lands {
		if int32(land.FlowerID) == mainID {
			mainCount++
		}
	}
	wantMain := elvesPlantMainLandCount(p, len(lands))
	needMain := wantMain - mainCount
	if needMain < 0 {
		needMain = 0
	}
	sort.Slice(empty, func(i, j int) bool { return empty[i] < empty[j] })
	var ops []PlannedOp
	cursor := 0
	if needMain > 0 && cursor < len(empty) {
		n := needMain
		if n > len(empty)-cursor {
			n = len(empty) - cursor
		}
		picks := append([]int32(nil), empty[cursor:cursor+n]...)
		cursor += n
		ops = append(ops, elvesPlantLandOp(picks, mainID, "种植花灵主花"))
	}
	if cursor < len(empty) {
		picks := append([]int32(nil), empty[cursor:]...)
		ops = append(ops, elvesPlantLandOp(picks, secID, "种植花灵副花"))
	}
	return ops
}

func elvesPlantLandOp(landIDs []int32, flowerID int32, reason string) PlannedOp {
	kind := clientproto.RPCUsrLandPlant.String()
	if len(landIDs) > 1 {
		kind = clientproto.RPCUsrLandPlantBatch.String()
	}
	return landOp(kind, "farm.plant", "plant", reason, elvesPlantPriority, landIDs, flowerID, "elves_plant", "elves_plant")
}

func elvesPlantSpeedUpOps(s *state.State, policy *pb.Policy, now time.Time) []PlannedOp {
	plant := policy.GetPlant()
	p := elvesPlantPolicy(plant)
	if !elvesPlantActive(p) || !p.GetUseSpeedUpTicket() {
		return nil
	}
	if elvesPlantBlockedByPendingAid(s, now) {
		return nil
	}
	capN := state.ResolveElvesSpawnCap(p.GetElvesSpawnCap())
	if s.ElvesProducedCount() >= capN {
		return nil
	}
	planting := plant.GetPlanting()
	batchMax := planting.GetSpeedUpTicketMax()
	if batchMax > 0 {
		remaining := batchMax - s.SpeedUpTicketsReservedToday(now)
		if remaining <= 0 {
			return nil
		}
		batchMax = remaining
	}
	secID := p.GetSecondaryFlowerId()
	lands, count := speedUpCandidates(s, now, secID, 0, batchMax)
	if count <= 0 {
		return nil
	}
	goal := Goal{ID: "farm.elves_plant", Category: CategoryPlant, Domain: "farm.speed_up", Label: "种植花灵加速", Priority: 55}
	speed := op(clientproto.RPCUsrLandSpeedUpBatch.String(), goal, "speed_up", "种植花灵副花加速", 7450, 0, 0, count)
	speed.LandIDs = lands
	speed.ItemCost = map[int32]int32{1001: count}
	speed.FeatureID = "plant.elves_plant_speed_up"
	return []PlannedOp{speed}
}
