package runner

import (
	"context"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

// hydratePearlHireTicketUsage loads today's durable hire-ticket spend count into
// memory. When the row is missing, it bootstraps once from event_log so a
// restart mid-day does not forget spends that already happened after 00:00.
func (r *Runner) hydratePearlHireTicketUsage(ctx context.Context) {
	if r == nil || r.db == nil || r.account == nil || r.state == nil {
		return
	}
	now := time.Now()
	dayID := state.PearlHireTicketDayID(now)
	used, err := r.db.PearlHireTicketUsed(ctx, r.account.ID, dayID)
	if err != nil {
		r.log.Warn("load pearl hire daily ticket usage failed", "err", err)
		return
	}
	if used == 0 {
		recovered, countErr := r.db.CountPearlHireTicketSpendsSince(ctx, r.account.ID, state.PearlHireTicketDayStart(now))
		if countErr != nil {
			r.log.Warn("recover pearl hire daily ticket usage failed", "err", countErr)
		} else if recovered > 0 {
			used = recovered
			if setErr := r.db.SetPearlHireTicketUsed(ctx, r.account.ID, dayID, used); setErr != nil {
				r.log.Warn("persist recovered pearl hire daily ticket usage failed", "err", setErr)
			}
		}
	}
	r.state.SetPearlHireTicketUsed(dayID, used)
}

// notePearlHireTicketUsed increments the durable calendar-day counter first,
// then mirrors the authoritative total into memory and clears world-empty wait.
func (r *Runner) notePearlHireTicketUsed(at time.Time) {
	if r == nil || r.state == nil {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	dayID := state.PearlHireTicketDayID(at)
	if r.db == nil || r.account == nil {
		r.state.NotePearlHireTicketUsed(at)
		return
	}
	used, err := r.db.IncrementPearlHireTicketUsed(context.Background(), r.account.ID, dayID)
	if err != nil {
		r.log.Warn("persist pearl hire daily ticket usage failed", "err", err)
		r.state.NotePearlHireTicketUsed(at)
		return
	}
	r.state.SetPearlHireTicketUsed(dayID, used)
	r.state.ClearPearlHireWorldEmptyWait()
}
