package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

func (r *Runner) hydrateElvesAidHelpReservations(ctx context.Context, at time.Time) {
	if r == nil || r.db == nil || r.account == nil || r.state == nil {
		return
	}
	dayID := state.PearlHireTicketDayID(at)
	uids, err := r.db.ElvesAidHelpReservations(ctx, r.account.ID, dayID)
	if err != nil {
		r.log.Warn("load flower-elf aid reservations failed", "err", err)
		return
	}
	r.state.SetFlowerElvesAidHelped(dayID, uids)
}

func (r *Runner) reserveElvesAidHelp(ctx context.Context, at time.Time, uid int64) error {
	globals, ok := state.FlowerElvesGlobalsFromCatalog()
	if !ok || globals.HelpMax <= 0 || r.db == nil || r.account == nil {
		return fmt.Errorf("花灵协助每日额度无法核验")
	}
	dayID := state.PearlHireTicketDayID(at)
	reserved, err := r.db.ReserveElvesAidHelp(ctx, r.account.ID, dayID, uid, globals.HelpMax)
	if err != nil {
		return err
	}
	if !reserved {
		return fmt.Errorf("花灵协助今日已执行或额度已满")
	}
	r.state.NoteFlowerElvesAidHelped(uid, at)
	return nil
}
