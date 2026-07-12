package state

import "time"

// DessertEnterSnapshot returns a target only when a valid tmpType-5601 batch
// is active/rewarding and its authoritative bag or dessert extension has not
// yet been observed. Malformed observed state is not repaired speculatively.
func (s *State) DessertEnterSnapshot(now time.Time) (DessertEnterSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	batch, phase, _, _, _ := s.preferredDessertBatchLocked(now.UnixMilli())
	config, catalogOK := DessertCatalogConfig()
	if batch == nil || !catalogOK || batch.BatchID <= 0 || batch.TmpType != config.TmpType || !batch.IdentityValid ||
		(phase != 2 && phase != 3) || (batch.BagObserved && !batch.BagValid) ||
		(batch.Dessert.ExtensionObserved && !batch.Dessert.ExtensionValid) ||
		(batch.Dessert.ModesObserved && !batch.Dessert.ModesValid) {
		return DessertEnterSnapshot{}, false
	}
	incomplete := !batch.BagObserved || !batch.Dessert.ExtensionObserved || !batch.Dessert.ModesObserved
	if !incomplete {
		return DessertEnterSnapshot{}, false
	}
	return DessertEnterSnapshot{At: now, BatchID: batch.BatchID, Phase: phase}, true
}

// DessertEnterApplied verifies that enter filled the activity-local bag and
// ext121 mode map for the same dynamically selected batch.
func (s *State) DessertEnterApplied(before DessertEnterSnapshot) bool {
	view, ok := s.DessertView(before.At)
	return ok && view.BatchID == before.BatchID && (view.Phase == 2 || view.Phase == 3) &&
		view.BagObserved && view.ExtensionObserved && view.ExtensionValid && view.ModeMapObserved && view.ModeMapValid
}

// DessertTaskClaimSnapshot returns the exact, uniquely identified fixed task
// that is ready and unclaimed. The reward invariant is intentionally repeated
// here so API readiness alone can never authorize an RPC.
func (s *State) DessertTaskClaimSnapshot(now time.Time, batchID, taskIndex, taskID int32) (DessertTaskClaimSnapshot, bool) {
	view, ok := s.DessertView(now)
	config, catalogOK := DessertCatalogConfig()
	if !ok || !catalogOK || !view.Valid || view.BatchID != batchID || batchID <= 0 || taskIndex != 0 || taskID <= 0 ||
		(view.Phase != 2 && view.Phase != 3) || !view.BagObserved || view.EnergyItemID != config.EnergyItemID {
		return DessertTaskClaimSnapshot{}, false
	}
	var matched *DessertTaskView
	for i := range view.Tasks {
		task := &view.Tasks[i]
		if task.TaskIndex != taskIndex || task.TaskID != taskID {
			continue
		}
		if matched != nil {
			return DessertTaskClaimSnapshot{}, false
		}
		matched = task
	}
	if matched == nil || matched.TaskType != config.TaskType || !matched.CatalogKnown || matched.Target <= 0 ||
		!matched.ProgressObserved || matched.Progress < matched.Target || !matched.ReceiptObserved || matched.Received ||
		len(matched.Reward) != 1 || matched.Reward[0].ItemID != config.EnergyItemID || matched.Reward[0].Count != config.InitialEnergy {
		return DessertTaskClaimSnapshot{}, false
	}
	return DessertTaskClaimSnapshot{
		At: now, BatchID: batchID, TaskIndex: taskIndex, TaskID: taskID, Target: matched.Target, Progress: matched.Progress,
		EnergyItemID: config.EnergyItemID, EnergyBefore: view.EnergyBalance, RewardCount: matched.Reward[0].Count,
	}, true
}

// DessertTaskClaimApplied requires both the authoritative receipt and the
// configured activity-local energy increase. Claimed progress may disappear.
func (s *State) DessertTaskClaimApplied(before DessertTaskClaimSnapshot) bool {
	view, ok := s.DessertView(before.At)
	if !ok || view.BatchID != before.BatchID || !view.BagObserved || view.EnergyItemID != before.EnergyItemID ||
		view.EnergyBalance < before.EnergyBefore+before.RewardCount {
		return false
	}
	for _, task := range view.Tasks {
		if task.TaskIndex == before.TaskIndex && task.TaskID == before.TaskID {
			return task.ReceiptObserved && task.Received
		}
	}
	return false
}

// DessertCelebritySyncSnapshot authorizes only a phase-two controlled sync.
func (s *State) DessertCelebritySyncSnapshot(now time.Time) (DessertCelebritySyncSnapshot, bool) {
	view, ok := s.DessertView(now)
	config, catalogOK := DessertCatalogConfig()
	if !ok || !catalogOK || !view.Valid || view.BatchID <= 0 || view.TmpType != config.TmpType || view.Phase != 2 {
		return DessertCelebritySyncSnapshot{}, false
	}
	s.mu.RLock()
	alreadySynced := s.dessertCelebritySyncedBatch == view.BatchID
	s.mu.RUnlock()
	if alreadySynced {
		return DessertCelebritySyncSnapshot{}, false
	}
	return DessertCelebritySyncSnapshot{At: now, BatchID: view.BatchID, Phase: view.Phase}, true
}

// DessertCelebritySyncApplied accepts a controlled full sync only when the
// complete type, ranking, and (possibly observed-empty) like maps are valid.
func (s *State) DessertCelebritySyncApplied(before DessertCelebritySyncSnapshot) bool {
	view, ok := s.DessertView(before.At)
	celebrity := view.Celebrity
	return ok && view.BatchID == before.BatchID && view.Phase == 2 && celebrity.Observed && celebrity.Valid &&
		celebrity.TypesObserved && celebrity.RankingsObserved && celebrity.LikesObserved && celebrity.TypeListed &&
		celebrity.RankingObserved
}

// MarkDessertCelebritySynced records a successful controlled full sync. It is
// deliberately not persisted and is cleared on each fresh login session.
func (s *State) MarkDessertCelebritySynced(batchID int32) {
	if batchID <= 0 {
		return
	}
	s.mu.Lock()
	s.dessertCelebritySyncedBatch = batchID
	s.mu.Unlock()
}

func (s *State) DessertCelebritySynced(batchID int32) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return batchID > 0 && s.dessertCelebritySyncedBatch == batchID
}

func (s *State) ResetDessertSession() {
	s.mu.Lock()
	s.dessertCelebritySyncedBatch = 0
	s.mu.Unlock()
}

// DessertCelebrityLikeSnapshot authorizes only the capture-confirmed free
// type-5601 like with the exact catalog reward.
func (s *State) DessertCelebrityLikeSnapshot(now time.Time, batchID int32) (DessertCelebrityLikeSnapshot, bool) {
	view, ok := s.DessertView(now)
	config, catalogOK := DessertCatalogConfig()
	celebrity := view.Celebrity
	if !ok || !catalogOK || !view.Valid || view.BatchID != batchID || batchID <= 0 || !s.DessertCelebritySynced(batchID) || view.Phase != 2 || !view.BagObserved ||
		config.CelebrityReward.ItemID != config.EnergyItemID || config.CelebrityReward.Count != 20 ||
		!celebrity.Observed || !celebrity.Valid || !celebrity.TypesObserved || !celebrity.RankingsObserved || !celebrity.LikesObserved ||
		!celebrity.TypeListed || !celebrity.RankingObserved || celebrity.RankingCount <= 0 || celebrity.LikedThisBatch {
		return DessertCelebrityLikeSnapshot{}, false
	}
	return DessertCelebrityLikeSnapshot{
		At: now, BatchID: batchID, BatchBeginMs: view.BeginMs, EnergyItemID: config.EnergyItemID,
		EnergyBefore: view.EnergyBalance, ExpectedReward: config.CelebrityReward.Count,
	}, true
}

func (s *State) DessertCelebrityLikeApplied(before DessertCelebrityLikeSnapshot) bool {
	view, ok := s.DessertView(before.At)
	return ok && view.BatchID == before.BatchID && view.Phase == 2 && view.BagObserved && view.EnergyItemID == before.EnergyItemID &&
		view.EnergyBalance >= before.EnergyBefore+before.ExpectedReward && view.Celebrity.LikedThisBatch &&
		view.Celebrity.LastLikeTimeMs > before.BatchBeginMs
}
