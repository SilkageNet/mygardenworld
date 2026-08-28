package apiserver

import (
	"context"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/runner"
)

const ensureAccountTimeout = 90 * time.Second

// EnsureRunningAccounts starts every account that does not currently have a
// live connection. Displaced-session auto-relogin waits are left alone.
// Successful starts persist automation_enabled=true like LoginAccount.
func (svc *Services) EnsureRunningAccounts(ctx context.Context) runner.RestoreReport {
	report := runner.RestoreReport{}
	accounts, err := svc.DB.ListAccounts(ctx, 0)
	if err != nil {
		report.Failed = 1
		if svc.Log != nil {
			svc.Log.Error("list accounts for ensure-running failed", "err", err)
		}
		return report
	}
	for i, acc := range accounts {
		if err := ctx.Err(); err != nil {
			report.Skipped = len(accounts) - i
			if svc.Log != nil {
				svc.Log.Info("ensure-running cancelled", "skipped", report.Skipped, "err", err)
			}
			break
		}
		r := svc.Manager.Get(acc.ID)
		if r != nil && r.Connected() {
			continue
		}
		if r != nil && r.DisplacedReloginPending() {
			report.Skipped++
			continue
		}
		report.Eligible++
		startCtx, cancel := context.WithTimeout(ctx, ensureAccountTimeout)
		var started *runner.Runner
		var startErr error
		if r == nil {
			started, startErr = svc.Manager.Start(startCtx, acc.ID)
		} else {
			started, startErr = svc.Manager.Reload(startCtx, acc.ID)
		}
		cancel()
		if startErr != nil {
			report.Failed++
			if svc.Log != nil {
				svc.Log.Warn("ensure-running account failed",
					"account_id", acc.ID,
					"account", acc.Name,
					"err", startErr,
				)
			}
			continue
		}
		svc.enableAutomation(ctx, acc.ID, started)
		report.Started++
		if svc.Log != nil {
			svc.Log.Info("ensure-running started account",
				"account_id", acc.ID,
				"account", acc.Name,
			)
		}
	}
	return report
}

// ShanghaiMidnightLocation is the calendar boundary used for nightly maintenance.
func ShanghaiMidnightLocation() *time.Location {
	return time.FixedZone("Asia/Shanghai", 8*60*60)
}

// NextMidnightAfter returns the next 00:00 at loc strictly after now.
func NextMidnightAfter(now time.Time, loc *time.Location) time.Time {
	if loc == nil {
		loc = ShanghaiMidnightLocation()
	}
	local := now.In(loc)
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	if now.Before(midnight) {
		return midnight
	}
	return midnight.Add(24 * time.Hour)
}

// FormatShanghaiMidnight renders a midnight boundary for logs.
func FormatShanghaiMidnight(t time.Time) string {
	return t.In(ShanghaiMidnightLocation()).Format("2006-01-02 15:04:05 MST")
}
