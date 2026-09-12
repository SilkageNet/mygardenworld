package state

import "time"

// SpeedUpTicketItemID is the inventory item consumed by usrLand.speedUpBatch.
const SpeedUpTicketItemID int32 = 1001

// SpeedUpTicketsUsedToday returns tickets spent since 00:00 Asia/Shanghai.
func (s *State) SpeedUpTicketsUsedToday(now time.Time) int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.speedUpTicketsUsedTodayLocked(now)
}

// NoteSpeedUpTicketsUsed records count tickets spent on the calendar day that
// contains at. count <= 0 is ignored.
func (s *State) NoteSpeedUpTicketsUsed(at time.Time, count int32) {
	if at.IsZero() || count <= 0 {
		return
	}
	s.mu.Lock()
	day := calendarDayID(at)
	if s.speedUpTicketUsedDayID != day {
		s.speedUpTicketUsedDayID = day
		s.speedUpTicketUsedToday = 0
	}
	s.speedUpTicketUsedToday += count
	s.mu.Unlock()
}

// SetSpeedUpTicketsUsed replaces the in-memory calendar-day counter. Callers
// hydrate this from durable storage (and event_log recovery) on runner start.
func (s *State) SetSpeedUpTicketsUsed(dayID, used int32) {
	if dayID <= 0 || used < 0 {
		return
	}
	s.mu.Lock()
	s.speedUpTicketUsedDayID = dayID
	s.speedUpTicketUsedToday = used
	s.mu.Unlock()
}

func (s *State) speedUpTicketsUsedTodayLocked(now time.Time) int32 {
	if s.speedUpTicketUsedDayID != calendarDayID(now) {
		return 0
	}
	return s.speedUpTicketUsedToday
}
