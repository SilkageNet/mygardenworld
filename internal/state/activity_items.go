package state

// ActivityItemCount returns one item from the authoritative namespace-23 bag
// for an exact batch. Missing keys are valid zero balances; an unobserved,
// malformed, or mismatched batch fails closed.
func (s *State) ActivityItemCount(batchID, itemID int32) (int32, bool) {
	if batchID <= 0 || itemID <= 0 {
		return 0, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	batch := s.activityBatches[batchID]
	if batch == nil || !batch.IdentityValid || batch.BatchID != batchID || !batch.BagObserved || !batch.BagValid ||
		!nonnegativeInt32Map(batch.Bag) {
		return 0, false
	}
	count := batch.Bag[itemID]
	if count < 0 {
		return 0, false
	}
	return count, true
}
