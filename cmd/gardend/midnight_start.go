package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/apiserver"
)

func runMidnightAccountEnsureLoop(ctx context.Context, svc *apiserver.Services, log *slog.Logger) {
	loc := apiserver.ShanghaiMidnightLocation()
	for {
		now := time.Now()
		next := apiserver.NextMidnightAfter(now, loc)
		timer := time.NewTimer(next.Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			timer.Stop()
			runMidnightAccountEnsure(ctx, svc, log, next)
		}
	}
}

func runMidnightAccountEnsure(ctx context.Context, svc *apiserver.Services, log *slog.Logger, at time.Time) {
	if log != nil {
		log.Info("midnight account ensure starting", "at", apiserver.FormatShanghaiMidnight(at))
	}
	report := svc.EnsureRunningAccounts(ctx)
	if log == nil {
		return
	}
	if report.Eligible == 0 && report.Failed == 0 && report.Skipped == 0 {
		log.Info("midnight account ensure finished", "result", "all_running")
		return
	}
	log.Info("midnight account ensure finished",
		"eligible", report.Eligible,
		"started", report.Started,
		"failed", report.Failed,
		"skipped", report.Skipped,
	)
}
