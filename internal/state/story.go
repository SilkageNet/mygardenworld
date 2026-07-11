package state

// StoryMainReady reports whether an observed story state is either a valid
// next section or the catalog-derived completed state.
func (s *State) StoryMainReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.storyMain.Observed && s.storyMain.Valid
}

// StoryUnlockSnapshot captures the exact current target, decoded cost, and
// inventory before one empty-argument storyMain.unlock request.
func (s *State) StoryUnlockSnapshot() (StoryUnlockSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	story := s.storyMain
	if !story.Observed || !story.Valid || story.Complete || story.SectionID <= 0 || len(story.Cost) == 0 {
		return StoryUnlockSnapshot{}, false
	}
	definition, valid, complete := ResolveStoryMainProgress(story.Chapter, story.SectionIdx)
	if !valid || complete || definition.SectionID != story.SectionID || !sameStoryCost(definition.Cost, story.Cost) {
		return StoryUnlockSnapshot{}, false
	}
	for _, cost := range story.Cost {
		if cost.ItemID <= 0 || cost.Count <= 0 || s.inventory[cost.ItemID] < cost.Count {
			return StoryUnlockSnapshot{}, false
		}
	}
	return StoryUnlockSnapshot{
		Chapter:    story.Chapter,
		SectionIdx: story.SectionIdx,
		SectionID:  story.SectionID,
		Cost:       append([]ItemCount(nil), story.Cost...),
		Inventory:  cloneInt32Map(s.inventory),
	}, true
}

// StoryUnlockApplied verifies exact one-section progress and exact catalog
// cost consumption after an authoritative response has been merged.
func (s *State) StoryUnlockApplied(snapshot StoryUnlockSnapshot) bool {
	if snapshot.Chapter <= 0 || snapshot.SectionIdx < 0 || snapshot.SectionID <= 0 || len(snapshot.Cost) == 0 {
		return false
	}
	nextChapter, nextSection, expectedComplete, ok := NextStoryMainProgress(snapshot.Chapter, snapshot.SectionIdx)
	if !ok {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	story := s.storyMain
	if !story.Observed || !story.Valid || story.Complete != expectedComplete ||
		story.Chapter != nextChapter || story.SectionIdx != nextSection {
		return false
	}
	for _, cost := range snapshot.Cost {
		if cost.ItemID <= 0 || cost.Count <= 0 {
			return false
		}
		before, observed := snapshot.Inventory[cost.ItemID]
		if !observed || before < cost.Count || s.inventory[cost.ItemID] != before-cost.Count {
			return false
		}
	}
	return true
}

func sameStoryCost(a, b []ItemCount) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
