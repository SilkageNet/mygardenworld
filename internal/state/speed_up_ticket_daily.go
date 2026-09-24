package state

import "time"

// SpeedUpTicketsReservedToday includes requests whose server outcome was
// uncertain. Counting them conservatively prevents a restart from exceeding
// the configured daily allowance.
func (s *State) SpeedUpTicketsReservedToday(now time.Time) int32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.speedUpTicketDayID != calendarDayID(now) {
		return 0
	}
	return s.speedUpTicketsReserved
}

func (s *State) SetSpeedUpTicketsReserved(dayID, count int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if dayID <= 0 || count < 0 {
		return
	}
	s.speedUpTicketDayID = dayID
	s.speedUpTicketsReserved = count
	s.bumpRevisionLocked()
}
