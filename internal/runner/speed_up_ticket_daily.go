package runner

import (
	"context"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

// hydrateSpeedUpTicketUsage loads today's durable speed-up ticket spend into
// memory. Prefer event_log recovery when it reports a higher total so mid-day
// restarts and first deploys do not under-count.
func (r *Runner) hydrateSpeedUpTicketUsage(ctx context.Context) {
	if r == nil || r.db == nil || r.account == nil || r.state == nil {
		return
	}
	now := time.Now()
	dayID := state.PearlHireTicketDayID(now)
	used, err := r.db.SpeedUpTicketUsed(ctx, r.account.ID, dayID)
	if err != nil {
		r.log.Warn("load speed-up ticket daily usage failed", "err", err)
		return
	}
	recovered, countErr := r.db.CountSpeedUpTicketSpendsSince(ctx, r.account.ID, state.PearlHireTicketDayStart(now))
	if countErr != nil {
		r.log.Warn("recover speed-up ticket daily usage failed", "err", countErr)
	} else if recovered > used {
		used = recovered
		if setErr := r.db.SetSpeedUpTicketUsed(ctx, r.account.ID, dayID, used); setErr != nil {
			r.log.Warn("persist recovered speed-up ticket daily usage failed", "err", setErr)
		}
	}
	r.state.SetSpeedUpTicketsUsed(dayID, used)
}

// noteSpeedUpTicketsUsed increments the durable calendar-day counter by count,
// then mirrors the authoritative total into memory.
func (r *Runner) noteSpeedUpTicketsUsed(at time.Time, count int32) {
	if r == nil || r.state == nil || count <= 0 {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	dayID := state.PearlHireTicketDayID(at)
	if r.db == nil || r.account == nil {
		r.state.NoteSpeedUpTicketsUsed(at, count)
		return
	}
	used, err := r.db.AddSpeedUpTicketUsed(context.Background(), r.account.ID, dayID, count)
	if err != nil {
		r.log.Warn("persist speed-up ticket daily usage failed", "err", err)
		r.state.NoteSpeedUpTicketsUsed(at, count)
		return
	}
	r.state.SetSpeedUpTicketsUsed(dayID, used)
}
