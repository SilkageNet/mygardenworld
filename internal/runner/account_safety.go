package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/automation"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

type accountSafetyState struct {
	safetyMu       sync.Mutex
	safety         store.AccountRequestSafety
	safetyLoaded   bool
	safetyRevision uint64
}

func (r *Runner) loadAccountSafety(ctx context.Context) error {
	r.safetyMu.Lock()
	defer r.safetyMu.Unlock()
	if r.safetyLoaded {
		return nil
	}
	if r.db != nil {
		s, err := r.db.LoadAccountRequestSafety(ctx, r.account.ID)
		if err != nil {
			return fmt.Errorf("加载账号请求保护失败: %w", err)
		}
		r.safety = s
	}
	r.safetyLoaded = true
	return nil
}

func (r *Runner) accountSafetySnapshot() (store.AccountRequestSafety, uint64) {
	r.safetyMu.Lock()
	defer r.safetyMu.Unlock()
	return r.safety, r.safetyRevision
}

func (r *Runner) raceDeleteWait(now time.Time) time.Duration {
	interval := automation.RaceDeleteInterval(r.Policy().GetUnion().GetRace())
	s, _ := r.accountSafetySnapshot()
	if s.LastRaceDeleteMS == 0 {
		return 0
	}
	return max(0, time.UnixMilli(s.LastRaceDeleteMS).Add(interval).Sub(now))
}

func (r *Runner) reserveRaceDelete(ctx context.Context, op *automation.PlannedOp, now time.Time) error {
	if op.Kind != clientproto.RPCFmlRaceDelTask.String() {
		return nil
	}
	if err := r.loadAccountSafety(ctx); err != nil {
		return err
	}
	interval := automation.RaceDeleteInterval(r.Policy().GetUnion().GetRace())
	r.safetyMu.Lock()
	defer r.safetyMu.Unlock()
	until := time.UnixMilli(r.safety.LastRaceDeleteMS).Add(interval)
	if r.safety.LastRaceDeleteMS != 0 && now.Before(until) {
		return fmt.Errorf("竞赛删除间隔未到，请于 %s 后重试（自动与手动共用）", until.Local().Format("15:04:05"))
	}
	if r.db != nil {
		allowed, err := r.db.ReserveRaceDelete(ctx, r.account.ID, now.UnixMilli(), interval.Milliseconds())
		if err != nil {
			return fmt.Errorf("保存竞赛删除间隔失败，未发送删除请求: %w", err)
		}
		if !allowed {
			if s, err := r.db.LoadAccountRequestSafety(ctx, r.account.ID); err == nil {
				r.safety.LastRaceDeleteMS = s.LastRaceDeleteMS
			}
			return fmt.Errorf("竞赛删除间隔未到（自动与手动共用）")
		}
	}
	r.safety.LastRaceDeleteMS = now.UnixMilli()
	return nil
}

func (r *Runner) persistRestrictionLocked(s store.AccountRequestSafety) error {
	if r.db == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return r.db.SaveAccountRestriction(ctx, r.account.ID, s)
}

func (r *Runner) restrictionError() error {
	s, _ := r.accountSafetySnapshot()
	return accountRestrictionError(s)
}

func accountRestrictionError(s store.AccountRequestSafety) error {
	if s.RestrictionCode == 0 {
		return nil
	}
	return fmt.Errorf("账号请求已暂停（服务端 %d），%s 后验证恢复", s.RestrictionCode,
		time.UnixMilli(s.RestrictedUntilMS).Local().Format("01/02 15:04:05"))
}

// Reconnect waits without HTTP login/config fetches. The existing cached
// session is retained; a restriction is not evidence of an invalid token.
func (r *Runner) waitAccountRestriction(ctx context.Context) bool {
	for {
		s, _ := r.accountSafetySnapshot()
		wait := time.Until(time.UnixMilli(s.RestrictedUntilMS))
		if s.RestrictionCode == 0 || wait <= 0 {
			return ctx.Err() == nil
		}
		if !sleepOrDone(ctx, wait) {
			return false
		}
	}
}
