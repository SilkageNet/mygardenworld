package state

import "time"

// Missing membership fields are not negative evidence. The observed flag and
// guild ID encode unknown, confirmed absence, and confirmed membership.
func (s *State) setFmlMembershipLocked(id int32) {
	wasAbsent := s.fmlBuild.MembershipObserved && s.fmlBuild.MemberFmlID == 0
	if id > 0 && s.lastConfirmedFmlID > 0 && id != s.lastConfirmedFmlID {
		// Retain no executable facts from a different guild, including across
		// reconnects. Current namespace facts are applied after this reset.
		s.unionState = unionState{}
	}
	if s.fmlBuild.MemberFmlID != id || id == 0 {
		s.fmlBuild.MemberPositionObserved = false
		s.fmlBuild.MemberPosition = 0
		s.fmlBuild.MemberPositionSyncAtMs = 0
	}
	s.fmlBuild.MembershipObserved = true
	s.fmlBuild.MemberFmlID = id
	s.fmlBuild.MembershipSyncAttempts = 0
	if id > 0 {
		s.lastConfirmedFmlID = id
		s.fmlBuild.FmlID = id
	} else if !wasAbsent || s.fmlBuild.MembershipSyncAtMs == 0 {
		s.fmlBuild.MembershipSyncAtMs = s.lastApplyMs
	}
}

// MarkFmlMembershipUncertainAt closes execution after a conflicting guild RPC
// response. It does not claim that a failed subsystem read proves a departure.
func (s *State) MarkFmlMembershipUncertainAt(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlBuild.MembershipObserved = false
	s.fmlBuild.MemberFmlID = 0
	s.fmlBuild.MemberPositionObserved = false
	s.fmlBuild.MemberPosition = 0
	s.fmlBuild.MembershipSyncAtMs = at.UnixMilli()
	s.fmlBuild.MembershipSyncAttempts = max(1, s.fmlBuild.MembershipSyncAttempts)
	s.bumpRevisionLocked()
}

// MarkFmlMembershipSyncAttemptAt records empty responses and failures as well
// as successes, so a missing member payload cannot create a four-second loop.
func (s *State) MarkFmlMembershipSyncAttemptAt(at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fmlBuild.MembershipSyncAtMs = at.UnixMilli()
	s.fmlBuild.MembershipSyncAttempts = min(3, s.fmlBuild.MembershipSyncAttempts+1)
	s.bumpRevisionLocked()
}
