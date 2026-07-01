// Package automation is the decision engine: pure functions that, given
// (lands + inventory + policy), return what the runner should do next.
//
// Mirrors plan_next_operation from scripts/tools/garden_bot.py.
package automation

import (
	"fmt"
	"sort"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/state"
	"google.golang.org/protobuf/proto"
)

// Recommendation kinds. Stable strings; matched against `Policy` knobs and
// surfaced in events / status snapshots.
const (
	KindHarvest = "harvest"
	KindPlant   = "plant"
	KindWater   = "water"
	KindWait    = "wait"
	KindUnknown = "unknown"
)

const (
	PlantModeLowStock  = "low_stock"
	PlantModeHighValue = "high_value"
	PlantModeSelected  = "selected"
)

// Recommend returns a single-land decision. Pure; no side effects.
func Recommend(land state.LandView, now time.Time) (kind, reason string) {
	if !land.Observed {
		return KindUnknown, "no observed primary state"
	}
	if !land.IsPlanted() {
		return KindPlant, "land is empty"
	}
	// state=3 = initial bloom ready (harvestCnt is still 0 here).
	if land.State == 3 {
		return KindHarvest, "state=3 (initial bloom ready)"
	}
	// state=2 = regrowing; nextTime is the unlock timestamp.
	if land.State == 2 {
		if land.NextTimeMs > 0 && land.NextTimeMs <= now.UnixMilli() {
			return KindHarvest, fmt.Sprintf("state=2, nextTime(%d) elapsed", land.NextTimeMs)
		}
		return KindWait, fmt.Sprintf("state=2 regrowing; nextTime=%d", land.NextTimeMs)
	}
	if land.State == 1 {
		return KindWater, "state=1, awaiting first water"
	}
	return KindWait, fmt.Sprintf("state=%d not actionable", land.State)
}

// PlannedOp is the next operation the runner should send. The runner converts
// this semantic plan to the appropriate babigame WS RPC request.
type PlannedOp struct {
	// Kind is the babigame RPC name (e.g. "usrLand.harvestOneKey",
	// "usrLand.plantBatch", "usrLand.water").
	Kind string

	// LandIDs is the list this op covers; one element for single-land ops,
	// many for batch / one-key ops.
	LandIDs []int32

	// FlowerID, when non-zero, is the seed used by a plant op. Recorded for
	// logging.
	FlowerID int32
}

// Plan returns the highest-priority PlannedOp for the given state + policy,
// or nil when nothing is actionable.
//
// Priority: harvest > plant > water. Within a kind, batch RPCs are preferred
// when policy.prefer_one_key (harvest) or batch_max > 1 (plant/water) and
// more than one land qualifies.
func Plan(s *state.State, policy *pb.Policy, now time.Time) *PlannedOp {
	if policy == nil || !policy.AutomationEnabled {
		return nil
	}

	lands := s.Lands()
	type bucket struct {
		harvest []int32
		water   []int32
		plant   []int32
	}
	var b bucket
	ids := make([]int32, 0, len(lands))
	for id := range lands {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		kind, _ := Recommend(lands[id], now)
		switch kind {
		case KindHarvest:
			b.harvest = append(b.harvest, id)
		case KindWater:
			b.water = append(b.water, id)
		case KindPlant:
			b.plant = append(b.plant, id)
		}
	}

	if len(b.harvest) > 0 && policy.GetHarvest().GetEnabled() {
		if len(b.harvest) > 1 && policy.GetHarvest().GetPreferOneKey() {
			return &PlannedOp{
				Kind:    "usrLand.harvestOneKey",
				LandIDs: b.harvest,
			}
		}
		return &PlannedOp{
			Kind:    "usrLand.harvest",
			LandIDs: []int32{b.harvest[0]},
		}
	}

	if len(b.plant) > 0 && policy.GetPlant().GetEnabled() {
		flowerID, _, plantLimit := selectPlantFlower(s, policy.GetPlant())
		if flowerID == 0 {
			// No cultivated flower stock; can't plant this tick.
			goto waterPath
		}
		// Planting does not consume flower inventory; captures show plant RPCs
		// update land/progress state, not 230xx item counts.
		maxBatch := int32(policy.GetPlant().GetMaxBatch())
		if maxBatch <= 0 {
			maxBatch = 8
		}
		want := int32(len(b.plant))
		if want > maxBatch {
			want = maxBatch
		}
		if plantLimit > 0 && want > plantLimit {
			want = plantLimit
		}
		if want <= 0 {
			goto waterPath
		}
		picks := b.plant[:want]
		if len(picks) > 1 {
			return &PlannedOp{
				Kind:     "usrLand.plantBatch",
				LandIDs:  picks,
				FlowerID: flowerID,
			}
		}
		return &PlannedOp{
			Kind:     "usrLand.plant",
			LandIDs:  picks,
			FlowerID: flowerID,
		}
	}

waterPath:
	if len(b.water) > 0 && policy.GetWater().GetEnabled() {
		waterDrops, _, _ := s.AvailableWaterDrops(now)
		minDrops := policy.GetWater().GetMinDrops()
		if minDrops < 0 {
			minDrops = 0
		}
		usableDrops := waterDrops - minDrops
		if usableDrops <= 0 {
			return nil
		}
		maxBatch := int32(policy.GetWater().GetMaxBatch())
		if maxBatch <= 0 {
			maxBatch = 8
		}
		want := int32(len(b.water))
		if want > maxBatch {
			want = maxBatch
		}
		if want > usableDrops {
			want = usableDrops
		}
		if want <= 0 {
			return nil
		}
		picks := b.water[:want]
		if len(picks) > 1 {
			return &PlannedOp{
				Kind:    "usrLand.waterBatch",
				LandIDs: picks,
			}
		}
		return &PlannedOp{
			Kind:    "usrLand.water",
			LandIDs: picks,
		}
	}
	return nil
}

func selectPlantFlower(s *state.State, policy *pb.PlantPolicy) (int32, int32, int32) {
	if policy == nil {
		return 0, 0, 0
	}
	mode := normalizePlantMode(policy.GetMode())
	if mode == PlantModeSelected && len(policy.GetAllowedFlowerIds()) == 0 {
		return 0, 0, 0
	}
	candidates := s.PlantableFlowers(policy.GetAllowedFlowerIds(), policy.GetBlockedFlowerIds())
	if len(candidates) == 0 {
		return 0, 0, 0
	}

	if plantTaskPriorityEnabled(policy) {
		deficits := s.FlowerOrderDeficits()
		if candidate, deficit, ok := bestDeficitCandidate(candidates, deficits); ok {
			return candidate.FlowerID, candidate.Stock, deficit
		}
	}
	if mode == PlantModeHighValue || mode == PlantModeSelected {
		candidate := bestValueCandidate(candidates)
		return candidate.FlowerID, candidate.Stock, 0
	}
	candidate := lowestStockCandidate(candidates)
	return candidate.FlowerID, candidate.Stock, 0
}

func plantTaskPriorityEnabled(policy *pb.PlantPolicy) bool {
	if policy == nil || policy.TaskPriorityEnabled == nil {
		return true
	}
	return policy.GetTaskPriorityEnabled()
}

func bestDeficitCandidate(candidates []state.PlantableFlower, deficits map[int32]int32) (state.PlantableFlower, int32, bool) {
	var best state.PlantableFlower
	var bestDeficit int32
	for _, candidate := range candidates {
		deficit := deficits[candidate.FlowerID]
		if deficit <= 0 {
			continue
		}
		if best.FlowerID == 0 ||
			deficit > bestDeficit ||
			(deficit == bestDeficit && candidate.Gold > best.Gold) ||
			(deficit == bestDeficit && candidate.Gold == best.Gold && candidate.FlowerID < best.FlowerID) {
			best = candidate
			bestDeficit = deficit
		}
	}
	return best, bestDeficit, best.FlowerID != 0
}

func bestValueCandidate(candidates []state.PlantableFlower) state.PlantableFlower {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Gold > best.Gold ||
			(candidate.Gold == best.Gold && candidate.Experience > best.Experience) ||
			(candidate.Gold == best.Gold && candidate.Experience == best.Experience && candidate.FlowerID < best.FlowerID) {
			best = candidate
		}
	}
	return best
}

func lowestStockCandidate(candidates []state.PlantableFlower) state.PlantableFlower {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if candidate.Stock < best.Stock ||
			(candidate.Stock == best.Stock && candidate.Gold > best.Gold) ||
			(candidate.Stock == best.Stock && candidate.Gold == best.Gold && candidate.FlowerID < best.FlowerID) {
			best = candidate
		}
	}
	return best
}

func normalizePlantMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case PlantModeLowStock:
		return PlantModeLowStock
	case PlantModeHighValue:
		return PlantModeHighValue
	case PlantModeSelected:
		return PlantModeSelected
	case "":
		return PlantModeHighValue
	default:
		return PlantModeHighValue
	}
}

// DefaultPolicy returns a Policy with conservative safety defaults:
// automation off (caller must opt in), the core farming loop enabled, and
// resource/progression misc operations disabled until explicitly enabled.
func DefaultPolicy() *pb.Policy {
	return &pb.Policy{
		AutomationEnabled:       false,
		Harvest:                 &pb.HarvestPolicy{Enabled: true, PreferOneKey: true},
		Plant:                   &pb.PlantPolicy{Enabled: true, Mode: PlantModeHighValue, TaskPriorityEnabled: proto.Bool(true), MinStock: 0, MaxBatch: 8},
		Water:                   &pb.WaterPolicy{Enabled: true, MaxBatch: 8, MinDrops: 5},
		Misc:                    &pb.MiscPolicy{},
		DecisionIntervalSeconds: 4,
	}
}
