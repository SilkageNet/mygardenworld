// Package notification delivers user-owned, curated account alerts independently
// of game runners. SQLite event cursors and an outbox survive process restarts;
// the live Bus is deliberately not used as a durable event source.
package notification

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/outbound"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

type Service struct {
	db     *store.DB
	log    *slog.Logger
	client *http.Client
}

func New(db *store.DB, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{db: db, log: log, client: outbound.NewClient(10 * time.Second)}
}

// Run has separate bounded ingestion and delivery loops. No network call can
// hold a store transaction, block a game runner, or stall event ingestion.
func (s *Service) Run(ctx context.Context) {
	var workers sync.WaitGroup
	workers.Go(func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			if err := s.db.CleanNotificationOutbox(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
				s.log.Warn("notification cleanup deferred", "error", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	})
	for range 4 {
		workers.Go(func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := s.deliverNext(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
						s.log.Warn("notification delivery deferred")
					}
				}
			}
		})
	}
	workers.Go(func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				ids, err := s.db.NotificationUsers(ctx)
				if err != nil {
					s.log.Warn("notification users unavailable")
					continue
				}
				for _, id := range ids {
					if err := s.db.ConsumeNotificationEvents(ctx, id, time.Now().UTC(), Classify); err != nil && !errors.Is(err, store.ErrNotificationQueueFull) && ctx.Err() == nil {
						s.log.Warn("notification ingestion deferred", "user_id", id)
					}
				}
			}
		}
	})
	workers.Wait()
	s.client.CloseIdleConnections()
}

// Classify uses typed event kind/action, never message substring matching or raw
// payload forwarding. Routine failures, disconnects and diagnostic waits are not
// incidents. An intentional disconnect must not become a webhook alert.
func Classify(e store.EventLog) *store.NotificationSignal {
	switch e.Kind {
	case "connection_unavailable":
		return &store.NotificationSignal{Kind: "connection", Message: "账号连接异常，自动化暂不可用，请查看控制台", Severity: 2}
	case "connection_recovered":
		return &store.NotificationSignal{Kind: "connection", Message: "账号连接已恢复，自动化将按当前配置运行", Recovered: true}
	case "account_request_paused":
		return &store.NotificationSignal{Kind: "account_request", Message: "账号进入请求保护，游戏请求已暂停，请查看控制台", Severity: 2}
	case "account_request_resumed":
		return &store.NotificationSignal{Kind: "account_request", Message: "账号请求保护已解除", Recovered: true}
	case "session_expired":
		if e.Action == "retry_scheduled" {
			return &store.NotificationSignal{Kind: "session", Message: "账号在其他设备登录，正在等待自动重登", Severity: 1}
		}
		return &store.NotificationSignal{Kind: "session", Message: "会话失效，自动化已停止，请查看控制台", Severity: 2}
	case "session":
		return &store.NotificationSignal{Kind: "session", Message: "账号会话已重新建立", Recovered: true}
	case "reputation_guard":
		if e.Action == "blocked" {
			return &store.NotificationSignal{Kind: "reputation", Message: "礼仪分保护触发，自动化已停止", Severity: 2}
		}
	case "pearl_hire_locked":
		return &store.NotificationSignal{Kind: "pearl_hire", Message: "珍珠雇佣结果不明确，本次会话已停止继续雇佣，请查看控制台", Severity: 2}
	}
	return nil
}

func (s *Service) deliverNext(ctx context.Context, now time.Time) error {
	n, err := s.db.ClaimNotification(ctx, now)
	if err != nil || n == nil {
		return err
	}
	target, err := s.db.NotificationDestination(ctx, n)
	if errors.Is(err, sql.ErrNoRows) {
		return s.db.FinishNotification(ctx, n, "cancelled", "通知设置或账号归属已变化", now)
	}
	if err != nil {
		return s.db.FinishNotification(ctx, n, "failed", "无法读取通知凭据，请重新保存设置", now)
	}
	req, err := providerRequest(ctx, target, n.Payload, now)
	if err != nil {
		return s.db.FinishNotification(ctx, n, "failed", "通知地址或内容无效，请检查渠道与地址", now)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "mygardenworld-webhook/1")
	req.Header.Set("X-Notification-ID", n.Key)
	resp, sendErr := s.client.Do(req)
	status, safeError := "pending", "网络请求失败或超时"
	if errors.Is(sendErr, outbound.ErrUnsafeEndpoint) {
		status, safeError = "failed", "通知地址解析到受限网络，已阻止发送"
	}
	next := now.Add(retryDelay(n.Attempts))
	if sendErr == nil {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 65537))
		_ = resp.Body.Close()
		code := resp.StatusCode
		safeError = "接收端返回 HTTP " + strconv.Itoa(code)
		switch {
		case code >= 200 && code < 300:
			if target.Provider != "custom" && (readErr != nil || len(body) > 65536) {
				status, safeError = "failed", "平台响应不完整或过大，未确认接收"
			} else {
				status, safeError = providerAcknowledgement(target.Provider, body)
				// DingTalk documents a ten-minute ban after exceeding its limit.
				// Other native providers get at least a minute before retrying.
				if status == "pending" {
					next = now.Add(max(time.Minute, retryDelay(n.Attempts)))
					if target.Provider == "dingtalk" {
						next = now.Add(10 * time.Minute)
					}
				}
			}
		case code == 408 || code == 429 || code >= 500:
			if delay := retryAfter(resp.Header.Get("Retry-After"), now); delay > next.Sub(now) {
				next = now.Add(delay)
			}
		default:
			status = "failed"
		}
	}
	if status == "pending" && n.Attempts >= 5 {
		status = "failed"
	}
	// Cancellation leaves the lease recoverable; a potentially delivered request
	// is retried with the same delivery key after restart.
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return s.db.FinishNotification(ctx, n, status, safeError, next)
}

func retryDelay(attempt int) time.Duration {
	return min(30*time.Minute, 30*time.Second*time.Duration(1<<min(max(attempt-1, 0), 6)))
}

func retryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(value, 10, 32); err == nil {
		return min(24*time.Hour, max(0, time.Duration(seconds)*time.Second))
	}
	if at, err := http.ParseTime(value); err == nil {
		return min(24*time.Hour, max(0, at.Sub(now)))
	}
	return 0
}
