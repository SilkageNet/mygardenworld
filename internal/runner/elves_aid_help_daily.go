package runner

import (
	"context"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

// hydrateFlowerElvesAidHelpUsage loads today's durable helpFrd targets into
// memory. Prefer operation_log recovery when it reports more UIDs so mid-day
// restarts and first deploys do not under-count past $helpMax.
func (r *Runner) hydrateFlowerElvesAidHelpUsage(ctx context.Context) {
	if r == nil || r.db == nil || r.account == nil || r.state == nil {
		return
	}
	now := time.Now()
	dayID := state.PearlHireTicketDayID(now)
	stored, err := r.db.ElvesAidHelpedUIDs(ctx, r.account.ID, dayID)
	if err != nil {
		r.log.Warn("load elves aid help daily usage failed", "err", err)
		return
	}
	recovered, countErr := r.db.ListElvesAidHelpTargetsSince(ctx, r.account.ID, state.PearlHireTicketDayStart(now))
	if countErr != nil {
		r.log.Warn("recover elves aid help daily usage failed", "err", countErr)
	}
	uids := mergeElvesAidHelpedUIDs(stored, recovered)
	if len(uids) > len(stored) {
		if setErr := r.db.SetElvesAidHelpedUIDs(ctx, r.account.ID, dayID, uids); setErr != nil {
			r.log.Warn("persist recovered elves aid help daily usage failed", "err", setErr)
		}
	}
	r.state.SetFlowerElvesAidHelped(dayID, uids)
}

// noteFlowerElvesAidHelped records a successful helpFrd in durable storage,
// then mirrors the authoritative UID list into memory.
func (r *Runner) noteFlowerElvesAidHelped(dstUID int64, at time.Time) {
	if r == nil || r.state == nil || dstUID <= 0 {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	dayID := state.PearlHireTicketDayID(at)
	if r.db == nil || r.account == nil {
		r.state.NoteFlowerElvesAidHelped(dstUID, at)
		return
	}
	uids, err := r.db.AddElvesAidHelpedUID(context.Background(), r.account.ID, dayID, dstUID)
	if err != nil {
		r.log.Warn("persist elves aid help daily usage failed", "err", err)
		r.state.NoteFlowerElvesAidHelped(dstUID, at)
		return
	}
	r.state.SetFlowerElvesAidHelped(dayID, uids)
}

func mergeElvesAidHelpedUIDs(parts ...[]int64) []int64 {
	seen := make(map[int64]struct{})
	var out []int64
	for _, part := range parts {
		for _, uid := range part {
			if uid <= 0 {
				continue
			}
			if _, ok := seen[uid]; ok {
				continue
			}
			seen[uid] = struct{}{}
			out = append(out, uid)
		}
	}
	return out
}
