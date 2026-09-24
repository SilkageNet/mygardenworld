package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/state"
)

func (r *Runner) hydrateSpeedUpTicketReservations(ctx context.Context, at time.Time) {
	if r == nil || r.db == nil || r.account == nil || r.state == nil {
		return
	}
	dayID := state.PearlHireTicketDayID(at)
	count, err := r.db.SpeedUpTicketsReserved(ctx, r.account.ID, dayID)
	if err != nil {
		r.log.Warn("load daily speed-up ticket reservations failed", "err", err)
		return
	}
	r.state.SetSpeedUpTicketsReserved(dayID, count)
}

func (r *Runner) reserveSpeedUpTickets(ctx context.Context, at time.Time, count int32) error {
	limit := r.Policy().GetPlant().GetPlanting().GetSpeedUpTicketMax()
	if limit <= 0 {
		return nil
	}
	if r.db == nil || r.account == nil || count <= 0 {
		return fmt.Errorf("加速券每日限额无法核验")
	}
	dayID := state.PearlHireTicketDayID(at)
	total, ok, err := r.db.ReserveSpeedUpTickets(ctx, r.account.ID, dayID, count, limit)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("今日加速券额度不足：上限 %d，当前请求 %d", limit, count)
	}
	r.state.SetSpeedUpTicketsReserved(dayID, total)
	return nil
}
