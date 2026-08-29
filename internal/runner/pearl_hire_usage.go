package runner

import (
	"context"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

// hydratePearlHireTicketUsage restores today's structured counter before the
// first automation decision. Runtime logs are intentionally not a data source.
func (r *Runner) hydratePearlHireTicketUsage(ctx context.Context, at time.Time) {
	if r == nil || r.db == nil || r.account == nil || r.state == nil || at.IsZero() {
		return
	}
	dayID := state.PearlHireTicketDayID(at)
	used, err := r.db.PearlHireTicketUsed(ctx, r.account.ID, dayID)
	if err != nil {
		r.log.Warn("load pearl hire daily ticket usage failed", "err", err)
		return
	}
	r.state.SetPearlHireTicketUsed(dayID, used)
}

// notePearlHireTicketUsed persists an exact observed ticket decrement and
// mirrors the atomic total into memory. If persistence is unavailable, the
// in-memory counter still advances so the running process remains conservative.
func (r *Runner) notePearlHireTicketUsed(ctx context.Context, at time.Time) {
	if r == nil || r.state == nil || at.IsZero() {
		return
	}
	dayID := state.PearlHireTicketDayID(at)
	if r.db == nil || r.account == nil {
		r.state.NotePearlHireTicketUsed(at)
		return
	}
	r.state.NotePearlHireTicketUsed(at)
	used, err := r.db.IncrementPearlHireTicketUsed(ctx, r.account.ID, dayID)
	if err != nil {
		r.log.Warn("persist pearl hire daily ticket usage failed", "err", err)
		return
	}
	r.state.MergePearlHireTicketUsed(dayID, used)
}
