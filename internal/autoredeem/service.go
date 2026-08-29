package autoredeem

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

// Service manages automatic redeem code fetching and redemption.
type Service struct {
	db      *store.DB
	manager *runner.Manager
	fetcher CodeFetcher
	log     *slog.Logger
}

// NewService constructs an auto-redeem service.
func NewService(db *store.DB, manager *runner.Manager, fetcher CodeFetcher, log *slog.Logger) *Service {
	return &Service{
		db:      db,
		manager: manager,
		fetcher: fetcher,
		log:     log.With("component", "auto_redeem"),
	}
}

const (
	maxAttempts       = 5
	redeemInterval    = 2 * time.Second
	pollCheckInterval = 1 * time.Minute
)

// Run starts the auto-redeem loop. It blocks until ctx is cancelled.
// On startup it checks whether codes have been synced within the last hour;
// if not it triggers an immediate poll. The loop then checks the enabled flag
// each cycle.
func (s *Service) Run(ctx context.Context) {
	s.log.Info("auto-redeem service started")
	defer s.log.Info("auto-redeem service stopped")

	// pollGuard is a buffered channel used as a semaphore to prevent
	// overlapping polls. A successful send means the goroutine owns the
	// exclusive right to poll; a full channel means a poll is already
	// running and the current trigger is skipped.
	pollGuard := make(chan struct{}, 1)

	// Startup check: if last sync was more than 1 hour ago, poll now.
	if s.needsStartupFetch(ctx) {
		pollGuard <- struct{}{}
		go func() {
			defer func() { <-pollGuard }()
			s.poll(ctx)
		}()
	}

	var (
		lastFetch   = time.Now()
		wasEnabled  = true
		pollSkipped bool
	)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollCheckInterval):
		}

		enabled, err := s.db.GetAutoRedeemEnabled(ctx)
		if err != nil {
			s.log.Error("check auto-redeem enabled", "err", err)
			continue
		}
		if !enabled {
			wasEnabled = false
			continue
		}

		// When the switch transitions to enabled, force an immediate poll.
		if !wasEnabled {
			lastFetch = time.Time{}
			wasEnabled = true
		}

		now := time.Now()
		interval := s.pollInterval(now)
		if now.Sub(lastFetch) < interval {
			continue
		}

		select {
		case pollGuard <- struct{}{}:
			lastFetch = now
			pollSkipped = false
			go func() {
				defer func() { <-pollGuard }()
				s.poll(ctx)
			}()
		default:
			if !pollSkipped {
				s.log.Info("previous poll still running, deferring next poll")
				pollSkipped = true
			}
		}
	}
}

// needsStartupFetch returns true when the last sync was more than 1 hour ago.
func (s *Service) needsStartupFetch(ctx context.Context) bool {
	lastSync, err := s.db.GetLastSyncAt(ctx)
	if err != nil {
		s.log.Error("check last sync time", "err", err)
		return true
	}
	if lastSync.IsZero() {
		s.log.Info("no previous sync recorded, running startup poll")
		return true
	}
	if time.Since(lastSync) > time.Hour {
		s.log.Info("last sync older than 1 hour, running startup poll", "last_sync", lastSync)
		return true
	}
	s.log.Info("codes recently synced, skipping startup poll", "last_sync", lastSync)
	return false
}

func (s *Service) pollInterval(now time.Time) time.Duration {
	h, m := now.Hour(), now.Minute()
	if (h >= 19 && h <= 21) || (h == 22 && m <= 30) {
		return 10 * time.Minute
	}
	return 1 * time.Hour
}

// poll fetches codes from the external API, overwrites them in the DB, and
// redeems them for all accounts.
func (s *Service) poll(ctx context.Context) {
	s.log.Info("fetching redeem codes")
	entries, err := s.fetcher.FetchCodes(ctx)
	if err != nil {
		s.log.Error("fetch redeem codes failed", "err", err)
		return
	}
	if len(entries) == 0 {
		s.log.Info("no redeem codes returned")
		return
	}

	// Overwrite codes in DB and record sync time.
	codeEntries := make([]store.RedeemCodeEntry, 0, len(entries))
	for _, e := range entries {
		codeEntries = append(codeEntries, store.RedeemCodeEntry{
			Code:       e.Code,
			SourceTime: e.CreatedTime,
		})
	}
	if err := s.db.SaveRedeemCodes(ctx, codeEntries); err != nil {
		s.log.Error("save redeem codes", "err", err)
	}

	// Get all accounts.
	accounts, err := s.db.ListAccounts(ctx, 0)
	if err != nil {
		s.log.Error("list accounts", "err", err)
		return
	}
	if len(accounts) == 0 {
		s.log.Info("no accounts registered, codes saved for viewing only")
		return
	}

	s.redeemForAccounts(ctx, entries, accounts)
}

// RedeemForNewAccount redeems all known codes for a single newly-created account.
// If no codes exist in the DB it fetches them first.
func (s *Service) RedeemForNewAccount(ctx context.Context, accountID int64) {
	codes, err := s.db.ListRedeemCodes(ctx, 0, 0)
	if err != nil {
		s.log.Error("list codes for new account", "err", err)
		return
	}

	// If DB has no codes yet, fetch them first.
	if len(codes) == 0 {
		s.log.Info("no codes in DB, fetching before redeeming for new account")
		entries, fetchErr := s.fetcher.FetchCodes(ctx)
		if fetchErr != nil {
			s.log.Error("fetch codes for new account", "err", fetchErr)
			return
		}
		codeEntries := make([]store.RedeemCodeEntry, 0, len(entries))
		for _, e := range entries {
			codeEntries = append(codeEntries, store.RedeemCodeEntry{
				Code:       e.Code,
				SourceTime: e.CreatedTime,
			})
		}
		if saveErr := s.db.SaveRedeemCodes(ctx, codeEntries); saveErr != nil {
			s.log.Error("save codes for new account", "err", saveErr)
		}
		// Re-load from DB to get proper entries.
		codes, err = s.db.ListRedeemCodes(ctx, 0, 0)
		if err != nil {
			s.log.Error("reload codes for new account", "err", err)
			return
		}
	}

	if len(codes) == 0 {
		return
	}

	acc, err := s.db.GetAccountByID(ctx, accountID)
	if err != nil {
		s.log.Error("get new account", "err", err)
		return
	}

	// Build CodeEntry slice from DB codes.
	entries := make([]CodeEntry, 0, len(codes))
	for _, c := range codes {
		entries = append(entries, CodeEntry{Code: c.Code, CreatedTime: c.SourceTime})
	}

	s.redeemForAccounts(ctx, entries, []*store.Account{acc})
}

// redeemForAccounts iterates entries × accounts, skipping already-handled pairs.
func (s *Service) redeemForAccounts(ctx context.Context, entries []CodeEntry, accounts []*store.Account) {
	s.log.Info("redeeming codes", "codes", len(entries), "accounts", len(accounts))

	redeemed := 0
	failed := 0
	for _, entry := range entries {
		for _, acc := range accounts {
			select {
			case <-ctx.Done():
				s.log.Info("auto-redeem cancelled", "redeemed", redeemed, "failed", failed)
				return
			default:
			}

			attempts, status, found, err := s.db.GetRedeemHistoryStatus(ctx, acc.ID, entry.Code)
			if err != nil {
				s.log.Error("check redeem history", "account", acc.Name, "code", entry.Code, "err", err)
				continue
			}
			if found {
				switch status {
				case "redeemed", "expired", "already_claimed", "invalid":
					continue
				case "failed":
					if attempts >= maxAttempts {
						continue
					}
				}
			}

			r := s.manager.Get(acc.ID)
			if r == nil || !r.Connected() {
				r, err = s.manager.Start(ctx, acc.ID)
				if err != nil {
					s.log.Warn("start runner failed", "account", acc.Name, "err", err)
					_ = s.db.UpsertRedeemHistory(ctx, acc.ID, entry.Code, "failed", err.Error())
					failed++
					continue
				}
			}

			result, err := r.RedeemCode(ctx, entry.Code)
			if err != nil {
				errMsg := err.Error()
				newStatus := classifyRedeemError(errMsg)
				_ = s.db.UpsertRedeemHistory(ctx, acc.ID, entry.Code, newStatus, errMsg)
				switch newStatus {
				case "already_claimed":
					s.log.Info("code already claimed", "account", acc.Name, "code", entry.Code)
				case "expired":
					s.log.Info("code expired", "account", acc.Name, "code", entry.Code)
				default:
					s.log.Warn("redeem failed", "account", acc.Name, "code", entry.Code, "err", err)
					failed++
				}
			} else {
				_ = s.db.UpsertRedeemHistory(ctx, acc.ID, entry.Code, "redeemed", "success")
				redeemed++
				s.log.Info("redeem success", "account", acc.Name, "code", entry.Code, "items", len(result.Items), "mails", result.MailNew)
			}

			select {
			case <-ctx.Done():
				s.log.Info("auto-redeem cancelled during interval", "redeemed", redeemed, "failed", failed)
				return
			case <-time.After(redeemInterval):
			}
		}
	}
	s.log.Info("auto-redeem cycle done", "redeemed", redeemed, "failed", failed)
}

// ForceSync fetches the latest redeem codes from the external API, saves them
// to the database, and redeems them for all registered accounts. It returns
// the new sync timestamp.
func (s *Service) ForceSync(ctx context.Context) (time.Time, error) {
	s.log.Info("force-sync redeem codes requested")
	s.poll(ctx)
	t, err := s.db.GetLastSyncAt(ctx)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

// classifyRedeemError determines the redeem failure status.
// Returns "already_claimed" when the code was already used by this account,
// "expired" when the code is permanently invalid, and "failed" otherwise.
func classifyRedeemError(msg string) string {
	// "已领取" specifically means this account already claimed this code.
	if strings.Contains(msg, "已领取") {
		return "already_claimed"
	}
	if strings.Contains(msg, "已过期") ||
		strings.Contains(msg, "已失效") ||
		strings.Contains(msg, "已使用") {
		return "expired"
	}
	if strings.Contains(msg, "不存在") ||
		strings.Contains(msg, "无效") {
		return "invalid"
	}
	for _, code := range []string{"335", "336", "337"} {
		if strings.Contains(msg, `"code":`+code) || strings.Contains(msg, `"code": `+code) {
			return "expired"
		}
	}
	return "failed"
}
