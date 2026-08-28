package runner

import (
	"sort"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

// cultivateUpgradeResourceObservation records the exact authoritative facts
// used for an upgrade attempt rejected by the server. Keeping the rejection
// tied to observations, rather than a timer, prevents a bad static cost from
// generating requests forever while still allowing an immediate retry after
// resources or cultivation state genuinely change.
type cultivateUpgradeResourceObservation struct {
	FlowerID      int32
	Level         int32
	Gold          int32
	GoldCost      int32
	EssenceItemID int32
	EssenceCount  int32
	EssenceCost   int32
}

func (r *Runner) cultivateUpgradeResourceObservation(op *automation.PlannedOp) (cultivateUpgradeResourceObservation, bool) {
	if r == nil || r.state == nil || op == nil || op.Kind != clientproto.RPCCultivateUpgrade.String() || op.FlowerID <= 0 {
		return cultivateUpgradeResourceObservation{}, false
	}
	cultivation, ok := r.state.Cultivations()[op.FlowerID]
	if !ok || cultivation.Lvl <= 0 {
		return cultivateUpgradeResourceObservation{}, false
	}
	itemIDs := make([]int32, 0, len(op.ItemCost))
	for itemID, count := range op.ItemCost {
		if itemID > 0 && count > 0 {
			itemIDs = append(itemIDs, itemID)
		}
	}
	sort.Slice(itemIDs, func(i, j int) bool { return itemIDs[i] < itemIDs[j] })
	var itemID, itemCost int32
	if len(itemIDs) > 0 {
		itemID = itemIDs[0]
		itemCost = op.ItemCost[itemID]
	}
	inventory := r.state.Inventory()
	return cultivateUpgradeResourceObservation{
		FlowerID:      op.FlowerID,
		Level:         cultivation.Lvl,
		Gold:          r.state.Gold(),
		GoldCost:      op.GoldCost,
		EssenceItemID: itemID,
		EssenceCount:  inventory[itemID],
		EssenceCost:   itemCost,
	}, true
}

func (r *Runner) markCultivateUpgradeResourceRejected(op *automation.PlannedOp) {
	observation, ok := r.cultivateUpgradeResourceObservation(op)
	if !ok {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cultivateUpgradeRejects == nil {
		r.cultivateUpgradeRejects = make(map[int32]cultivateUpgradeResourceObservation)
	}
	r.cultivateUpgradeRejects[observation.FlowerID] = observation
}

func (r *Runner) cultivateUpgradeResourceRejectedUnchanged(op *automation.PlannedOp) bool {
	observation, ok := r.cultivateUpgradeResourceObservation(op)
	if !ok {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rejected, exists := r.cultivateUpgradeRejects[observation.FlowerID]
	if !exists {
		return false
	}
	if rejected == observation {
		return true
	}
	delete(r.cultivateUpgradeRejects, observation.FlowerID)
	return false
}

func (r *Runner) clearCultivateUpgradeResourceRejection(op *automation.PlannedOp) {
	if op == nil || op.Kind != clientproto.RPCCultivateUpgrade.String() || op.FlowerID <= 0 {
		return
	}
	r.mu.Lock()
	delete(r.cultivateUpgradeRejects, op.FlowerID)
	r.mu.Unlock()
}
