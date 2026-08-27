package automation

import (
	"sort"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func maintenanceOperations(s *state.State, policy *pb.Policy, ledger *InventoryLedger, now time.Time) []PlannedOp {
	plant := policy.GetPlant()
	planting := plant.GetPlanting()
	cultivate := plant.GetCultivate()
	var ops []PlannedOp
	goal := Goal{ID: "farm.maintenance", Category: CategoryPlant, Domain: "farm.maintenance", Label: "农场维护", Priority: 55}
	if planting.GetAutoUnlockLand() {
		if landID, goldCost, ok := nextLandUnlockCandidate(s); ok {
			unlock := op(clientproto.RPCUsrLandUnlockLand.String(), goal, "unlock", "有可开垦土地", 7600, landID, 0, 0)
			unlock.GoldCost = goldCost
			ops = append(ops, unlock)
		}
	}
	if planting.GetUseSpeedUpTicket() || raceSpeedupEnabledAt(s, policy.GetUnion().GetRace(), now) {
		// Global planting speedup accelerates every growing land. Race-only
		// "种植任务使用加速卡" must only hit the taken plant-harvest flower.
		// When both are on, prefer the race flower first so limited tickets
		// still serve guild competition before ordinary crops.
		flowerFilter := int32(0)
		preferFlower := int32(0)
		raceOn := raceSpeedupEnabledAt(s, policy.GetUnion().GetRace(), now)
		if raceOn {
			preferFlower = s.FmlRace().Taken.ParamID
		}
		if !planting.GetUseSpeedUpTicket() {
			flowerFilter = preferFlower
			preferFlower = 0
		}
		if lands, count := speedUpCandidates(s, now, flowerFilter, preferFlower); count > 0 {
			reason := "存在可加速土地"
			if !planting.GetUseSpeedUpTicket() {
				reason = "公会竞赛种植任务使用加速卡"
				if raceExpireUrgentSpeedup(s.FmlRace().Taken, now) &&
					!policy.GetUnion().GetRace().GetUseSpeedupTicketInTask() {
					reason = "公会竞赛任务即将过期，使用加速卡"
				}
			}
			speed := op(clientproto.RPCUsrLandSpeedUpBatch.String(), goal, "speed_up", reason, 7400, 0, 0, count)
			speed.LandIDs = lands
			speed.ItemCost = map[int32]int32{1001: count}
			ops = append(ops, speed)
		}
	}
	if cultivate.GetEnabled() || cultivate.GetUpgradeEnabled() {
		if cultivate, ok := cultivateOperation(s, plant, ledger, now); ok {
			ops = append(ops, cultivate)
		}
	}
	return ops
}

func blockedUnknownOperations(policy *pb.Policy) []PlannedOp {
	var ops []PlannedOp
	add := func(enabled bool, category, domain, label string) {
		if !enabled {
			return
		}
		op := markerOp(category, domain, "blocked", "协议或状态不明确，已按计划阻塞", 100)
		op.Label = label
		op.Status = PlanStatusAdapterMissing
		op.Executable = false
		op.BlockedReasons = []string{"该领域尚未完成协议确认，先记录文档，不自动执行"}
		ops = append(ops, op)
	}
	union := policy.GetUnion()
	unionFlower := union.GetFlower()
	// Race and guild land plant/harvest are fully managed; do not mark them unknown.
	add(unionFlower.GetShareEnabled() ||
		union.GetRedPacketEnabled(), CategoryUnion, "union.unknown", "公会扩展功能")
	return ops
}

// speedUpCandidates returns growing lands that still need tickets.
// When flowerID > 0, only lands planted with that flower are eligible
// (used by guild-race plant-harvest speedup).
// When preferFlower > 0, matching lands are ordered first so scarce tickets
// serve the race crop before other growing lands.
func speedUpCandidates(s *state.State, now time.Time, flowerID, preferFlower int32) ([]int32, int32) {
	available := s.Inventory()[1001]
	if available <= 0 {
		return nil, 0
	}
	lands := s.Lands()
	var ids []int32
	for id, land := range lands {
		if land.State != 2 || land.NextTimeMs <= now.UnixMilli() {
			continue
		}
		if flowerID > 0 && int32(land.FlowerID) != flowerID {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if preferFlower > 0 {
			li := int32(lands[ids[i]].FlowerID) == preferFlower
			lj := int32(lands[ids[j]].FlowerID) == preferFlower
			if li != lj {
				return li
			}
		}
		return ids[i] < ids[j]
	})
	want := int32(len(ids))
	if want > available {
		want = available
	}
	if want > 5 {
		want = 5
	}
	return ids[:want], want
}

func cultivateOperation(s *state.State, policy *pb.PlantPolicy, ledger *InventoryLedger, now time.Time) (PlannedOp, bool) {
	goal := Goal{ID: "farm.cultivate", Category: CategoryPlant, Domain: "farm.cultivate", Label: "培育", Priority: 55}
	cultivatePolicy := policy.GetCultivate()
	cultivations := s.Cultivations()
	ids := make([]int32, 0, len(cultivations))
	for id := range cultivations {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if cultivatePolicy.GetEnabled() {
		nowMs := now.UnixMilli()
		for _, id := range ids {
			cv := cultivations[id]
			if cv.Status == 1 && cv.CulTimeMs > 0 && cv.CulTimeMs <= nowMs {
				op := op(clientproto.RPCCultivateRecv.String(), goal, "recv", "培育完成可领取", 7200, 0, 0, 0)
				op.FlowerID = id
				return op, true
			}
		}
	}
	if cultivatePolicy.GetUpgradeEnabled() {
		targetLevel := cultivatePolicy.GetTargetLevel()
		for _, id := range ids {
			cv := cultivations[id]
			if cv.Status != 2 || cv.Lvl <= 0 {
				continue
			}
			if targetLevel > 0 && cv.Lvl >= targetLevel {
				continue
			}
			cost, ok := state.FlowerUpgradeCostForLevel(id, cv.Lvl)
			if !ok || s.Inventory()[cost.ItemID] < cost.Count || s.Gold() < cost.Gold {
				continue
			}
			// Domain must be farm.upgrade so feature enrichment marks the op
			// executable; farm.cultivate+upgrade is unregistered and blocked.
			upgrade := domainOp(clientproto.RPCCultivateUpgrade.String(), goal, "farm.upgrade", "upgrade", "鲜花培育等级可升级", 7100, 0, 0, 0)
			upgrade.FlowerID = id
			upgrade.Count = cv.Lvl // from-level for operator logs (lvN-lvN+1)
			upgrade.GoldCost = cost.Gold
			upgrade.ItemCost = map[int32]int32{cost.ItemID: cost.Count}
			return upgrade, true
		}
	}
	if cultivatePolicy.GetEnabled() {
		for _, flower := range s.PlantableFlowers(nil, nil) {
			if _, exists := cultivations[flower.FlowerID]; exists {
				continue
			}
			costs, ok := state.CultivateCost(flower.FlowerID)
			if !ok {
				blocked := op(clientproto.RPCCultivateCultivate.String(), goal, "cultivate", "培育材料配置未确认", 7050, 0, 0, 0)
				blocked.FlowerID = flower.FlowerID
				blocked.Status = PlanStatusAdapterMissing
				blocked.Executable = false
				blocked.BlockedReasons = []string{"缺少培育材料静态配置，已阻塞等待确认"}
				return blocked, true
			}
			itemCost := map[int32]int32{}
			for _, cost := range costs {
				if cost.ItemID > 0 && cost.Count > 0 {
					itemCost[cost.ItemID] += cost.Count
				}
			}
			if !ledger.CanSpendItems(itemCost) {
				continue
			}
			op := op(clientproto.RPCCultivateCultivate.String(), goal, "cultivate", "有未培育花朵", 7050, 0, 0, 0)
			op.FlowerID = flower.FlowerID
			op.ItemCost = itemCost
			return op, true
		}
	}
	return PlannedOp{}, false
}

func nextLandUnlockCandidate(st *state.State) (int32, int32, bool) {
	if !st.LandRosterObserved() || !st.FarmLandConfigObserved() {
		return 0, 0, false
	}
	opened := st.Lands()
	reclaimable := 0
	for _, land := range st.FarmLands() {
		if _, ok := opened[land.ID]; ok {
			continue
		}
		reclaimable++
		if reclaimable > 6 {
			break
		}
		if len(land.Cost) < 2 {
			continue
		}
		actualCost := land.Cost[1] - land.Cost[0] + 11
		if st.Level() >= land.OpenLevel && st.Gold() >= actualCost {
			return land.ID, actualCost, true
		}
	}
	return 0, 0, false
}
