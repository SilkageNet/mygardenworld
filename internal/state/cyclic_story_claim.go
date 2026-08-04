package state

import "time"

// CyclicStoryEnterSnapshot returns the exact dynamically selected batch only
// when an enter request is safe and still needed. Fresh batches often lack
// score/bag until enter succeeds, so EnterReady (not Valid) gates this path.
// Invalid previously-observed orders also re-qualify for enter to resync.
func (s *State) CyclicStoryEnterSnapshot(now time.Time) (CyclicStoryEnterSnapshot, bool) {
	view, ok := s.CyclicStoryView(now)
	if !ok || !view.EnterReady || view.BatchID <= 0 {
		return CyclicStoryEnterSnapshot{}, false
	}
	if view.OrdersObserved && view.OrdersValid {
		return CyclicStoryEnterSnapshot{}, false
	}
	return CyclicStoryEnterSnapshot{At: now, BatchID: view.BatchID, Phase: view.Phase}, true
}

// CyclicStoryEnterApplied confirms that the same batch now carries observed
// order info.
func (s *State) CyclicStoryEnterApplied(snapshot CyclicStoryEnterSnapshot) bool {
	if snapshot.BatchID <= 0 || (snapshot.Phase != 2 && snapshot.Phase != 3) {
		return false
	}
	view, ok := s.CyclicStoryView(snapshot.At)
	return ok && view.EnterReady && view.BatchID == snapshot.BatchID && view.OrdersObserved && view.OrdersValid
}

// CyclicStoryOrderClaimSnapshot validates one active order that can be
// submitted with flowers already in ordinary inventory. It never invents
// plant demands; callers must already hold Cost of FlowerID.
func (s *State) CyclicStoryOrderClaimSnapshot(now time.Time, batchID, orderIdx int32) (CyclicStoryOrderClaimSnapshot, bool) {
	if batchID <= 0 || orderIdx < 0 {
		return CyclicStoryOrderClaimSnapshot{}, false
	}
	view, ok := s.CyclicStoryView(now)
	if !ok || !view.Valid || view.BatchID != batchID || view.Phase != 2 || !view.OrdersObserved {
		return CyclicStoryOrderClaimSnapshot{}, false
	}
	var selected *CyclicStoryOrderView
	occurrences := 0
	for i := range view.Orders {
		order := &view.Orders[i]
		if order.OrderIdx == orderIdx {
			selected = order
			occurrences++
		}
	}
	if occurrences != 1 || selected == nil || selected.OrderID <= 0 || selected.FlowerID <= 0 ||
		!selected.CatalogKnown || selected.Cost <= 0 || selected.OnCooldown {
		return CyclicStoryOrderClaimSnapshot{}, false
	}
	inventory := s.Inventory()
	if inventory[selected.FlowerID] < selected.Cost {
		return CyclicStoryOrderClaimSnapshot{}, false
	}
	return CyclicStoryOrderClaimSnapshot{
		At: now, BatchID: batchID, OrderIdx: orderIdx, OrderID: selected.OrderID,
		FlowerID: selected.FlowerID, Cost: selected.Cost, FinishCount: view.FinishCount,
		FinishCountObserved: view.FinishCountObserved, Score: view.Score,
	}, true
}

// CyclicStoryOrderClaimApplied accepts an exact order-slot replacement or a
// finish-count increase that removes the previous ready order.
func (s *State) CyclicStoryOrderClaimApplied(snapshot CyclicStoryOrderClaimSnapshot) bool {
	if snapshot.BatchID <= 0 || snapshot.OrderIdx < 0 || snapshot.OrderID <= 0 ||
		snapshot.FlowerID <= 0 || snapshot.Cost <= 0 {
		return false
	}
	view, ok := s.CyclicStoryView(snapshot.At)
	if !ok || !view.Valid || view.BatchID != snapshot.BatchID || !view.OrdersObserved {
		return false
	}
	var current *CyclicStoryOrderView
	for i := range view.Orders {
		if view.Orders[i].OrderIdx == snapshot.OrderIdx {
			current = &view.Orders[i]
			break
		}
	}
	if current == nil {
		return true
	}
	if current.OrderID != snapshot.OrderID {
		return true
	}
	return snapshot.FinishCountObserved && view.FinishCountObserved && view.FinishCount > snapshot.FinishCount
}

// CyclicStoryMilestoneClaimSnapshot validates a server-configured milestone
// idx against the raw score. Reward-grace phase (3) remains claimable.
func (s *State) CyclicStoryMilestoneClaimSnapshot(now time.Time, batchID, index int32) (CyclicStoryMilestoneClaimSnapshot, bool) {
	if batchID <= 0 || index <= 0 {
		return CyclicStoryMilestoneClaimSnapshot{}, false
	}
	view, ok := s.CyclicStoryView(now)
	if !ok || !view.Valid || view.BatchID != batchID || (view.Phase != 2 && view.Phase != 3) ||
		!view.MilestoneReceiptsObserved {
		return CyclicStoryMilestoneClaimSnapshot{}, false
	}
	var selected *CyclicNoteMilestoneView
	occurrences := 0
	for i := range view.Milestones {
		milestone := &view.Milestones[i]
		if milestone.Index == index {
			selected = milestone
			occurrences++
		}
	}
	if occurrences != 1 || selected == nil || selected.Target <= 0 || selected.Received || view.Score < selected.Target {
		return CyclicStoryMilestoneClaimSnapshot{}, false
	}
	return CyclicStoryMilestoneClaimSnapshot{
		At: now, BatchID: batchID, MilestoneIndex: index, Target: selected.Target, Score: view.Score,
	}, true
}

// CyclicStoryMilestoneClaimApplied requires the exact milestone idx to appear
// in the authoritative claimed-box list of the same batch.
func (s *State) CyclicStoryMilestoneClaimApplied(snapshot CyclicStoryMilestoneClaimSnapshot) bool {
	if snapshot.BatchID <= 0 || snapshot.MilestoneIndex <= 0 || snapshot.Target <= 0 || snapshot.Score < snapshot.Target {
		return false
	}
	view, ok := s.CyclicStoryView(snapshot.At)
	if !ok || !view.Valid || view.BatchID != snapshot.BatchID || !view.MilestoneReceiptsObserved {
		return false
	}
	for _, milestone := range view.Milestones {
		if milestone.Index == snapshot.MilestoneIndex {
			return milestone.Target == snapshot.Target && milestone.Received
		}
	}
	return false
}
