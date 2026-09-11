package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var ErrNotificationSettings = errors.New("通知设置无效或用户不可用")
var ErrNotificationQueueFull = errors.New("通知队列已满，请稍后重试")
var ErrNotificationTestCooldown = errors.New("测试通知每分钟只能发送一次")

type NotificationSettings struct {
	UserID           int64
	Enabled          bool
	HasEndpoint      bool
	CooldownMinutes  int
	Revision         int64
	Provider         string
	HasSigningSecret bool
}

// NotificationUpdate is scoped to a system user, not a game account.
type NotificationUpdate struct {
	Enabled         bool
	Endpoint        *string
	CooldownMinutes int
	Provider        string
	SigningSecret   *string
}

type NotificationTarget struct {
	Provider      string
	Endpoint      string
	SigningSecret string
}

// NotificationSignal contains only reviewed, non-secret summaries. In particular,
// callers must never copy raw upstream errors or PayloadJSON into Message.
type NotificationSignal struct {
	Kind      string
	Message   string
	Severity  int
	Recovered bool
}

// NotificationPayload is the generic JSON webhook contract. ID remains stable
// across retries: receivers can deduplicate at-least-once deliveries with it.
type NotificationPayload struct {
	ID              string    `json:"id"`
	Kind            string    `json:"kind"`
	Level           string    `json:"level"`
	Message         string    `json:"message"`
	AccountID       int64     `json:"account_id,omitempty"`
	AccountName     string    `json:"account_name,omitempty"`
	TS              time.Time `json:"ts"`
	Recovered       bool      `json:"recovered"`
	Occurrences     int64     `json:"occurrences"`
	DurationSeconds int64     `json:"duration_seconds"`
}

type NotificationDelivery struct {
	ID        int64
	UserID    int64
	Key       string
	Title     string
	Status    string
	Attempts  int
	CreatedMS int64
	LastError string
	Payload   string
	Revision  int64
}

func (d *DB) NotificationSettings(ctx context.Context, userID int64) (NotificationSettings, error) {
	s := NotificationSettings{UserID: userID, CooldownMinutes: 30, Provider: "custom"}
	if userID <= 0 {
		return s, ErrNotificationSettings
	}
	err := d.QueryRowContext(ctx, `SELECT enabled, endpoint_enc <> '', cooldown_minutes, revision, provider, signing_secret_enc <> '' FROM user_notifications WHERE user_id = ?`, userID).Scan(&s.Enabled, &s.HasEndpoint, &s.CooldownMinutes, &s.Revision, &s.Provider, &s.HasSigningSecret)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return s, err
}

// SaveNotificationSettings retains omitted credentials only for an unchanged
// route. Replacing the URL clears the signing key unless explicitly supplied.
// Enabling starts at the current log head, not historical failures. Changing
// credentials, provider or enabled state cancels old pending deliveries and
// incidents; changing cooldown alone preserves them.
func (d *DB) SaveNotificationSettings(ctx context.Context, userID int64, update NotificationUpdate) error {
	enabled, endpoint, cooldownMinutes := update.Enabled, update.Endpoint, update.CooldownMinutes
	if userID <= 0 || cooldownMinutes < 1 || cooldownMinutes > 1440 {
		return ErrNotificationSettings
	}
	switch update.Provider {
	case "custom", "wecom", "dingtalk", "feishu":
	default:
		return ErrNotificationSettings
	}
	if update.SigningSecret != nil && (len(*update.SigningSecret) > 1024 || (*update.SigningSecret != "" && update.Provider != "dingtalk" && update.Provider != "feishu")) {
		return ErrNotificationSettings
	}
	var encrypted string
	var err error
	if endpoint != nil && *endpoint != "" {
		encrypted, err = d.encodeNotificationCredential(userID, "endpoint", *endpoint)
		if err != nil {
			return err
		}
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `INSERT INTO user_notifications(user_id) SELECT id FROM users WHERE id = ? AND status = 'active' ON CONFLICT(user_id) DO NOTHING`, userID); err != nil {
		return err
	}
	var previous, previousProvider, previousSecret string
	var previouslyEnabled bool
	if err := tx.QueryRowContext(ctx, `SELECT endpoint_enc, enabled, provider, signing_secret_enc FROM user_notifications n JOIN users u ON u.id = n.user_id WHERE n.user_id = ? AND u.status = 'active'`, userID).Scan(&previous, &previouslyEnabled, &previousProvider, &previousSecret); err != nil {
		return ErrNotificationSettings
	}
	// Never silently send old credentials to a newly selected provider.
	if update.Provider != previousProvider && endpoint == nil {
		return ErrNotificationSettings
	}
	if endpoint == nil {
		encrypted = previous
	}
	secret := previousSecret
	if endpoint != nil || update.Provider != previousProvider {
		secret = ""
	}
	if update.SigningSecret != nil {
		secret = ""
		if *update.SigningSecret != "" {
			if encrypted == "" {
				return ErrNotificationSettings
			}
			secret, err = d.encodeNotificationCredential(userID, "signing", *update.SigningSecret)
			if err != nil {
				return err
			}
		}
	}
	if enabled && encrypted == "" {
		return ErrNotificationSettings
	}
	routingChanged := previouslyEnabled != enabled || encrypted != previous || update.Provider != previousProvider || secret != previousSecret
	if _, err := tx.ExecContext(ctx, `UPDATE user_notifications SET enabled = ?, endpoint_enc = ?, cooldown_minutes = ?, provider = ?, signing_secret_enc = ?, revision = revision + ?, retry_after_ms = CASE WHEN ? THEN 0 ELSE retry_after_ms END, event_cursor = CASE WHEN ? THEN (SELECT COALESCE(MAX(id), 0) FROM event_log) ELSE event_cursor END WHERE user_id = ?`, enabled, encrypted, cooldownMinutes, update.Provider, secret, routingChanged, routingChanged, routingChanged, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE notification_outbox SET status = 'cancelled', last_error = '通知设置已更改' WHERE user_id = ? AND ? AND status IN ('pending', 'sending')`, userID, routingChanged); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM notification_incidents WHERE user_id = ? AND ?`, userID, routingChanged); err != nil {
		return err
	}
	return tx.Commit()
}

func (d *DB) encodeNotificationCredential(userID int64, purpose, value string) (string, error) {
	aead, err := d.credentialAEAD()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(value), notificationAAD(userID, purpose))), nil
}

func (d *DB) decodeNotificationCredential(userID int64, purpose, encrypted string) (string, error) {
	aead, err := d.credentialAEAD()
	if err != nil {
		return "", err
	}
	b, err := base64.RawStdEncoding.DecodeString(encrypted)
	if err != nil || len(b) < aead.NonceSize() {
		return "", ErrNotificationSettings
	}
	plain, err := aead.Open(nil, b[:aead.NonceSize()], b[aead.NonceSize():], notificationAAD(userID, purpose))
	return string(plain), err
}

func notificationAAD(userID int64, purpose string) []byte {
	aad := fmt.Sprintf("mygardenworld/notification/user/%d", userID)
	// Preserve the endpoint's existing encryption domain when migrating v10.
	if purpose != "endpoint" {
		aad += "/" + purpose
	}
	return []byte(aad)
}

func (d *DB) NotificationUsers(ctx context.Context) ([]int64, error) {
	rows, err := d.QueryContext(ctx, `SELECT n.user_id FROM user_notifications n JOIN users u ON u.id = n.user_id WHERE n.enabled = 1 AND u.status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ConsumeNotificationEvents advances a user's cursor and builds its outbox in one
// transaction. A dropped Bus event or daemon restart cannot lose a retained log.
// The callback is pure; no network or runner actions are permitted here.
func (d *DB) ConsumeNotificationEvents(ctx context.Context, userID int64, now time.Time, classify func(EventLog) *NotificationSignal) error {
	if userID <= 0 {
		return ErrNotificationSettings
	}
	// An idle subscriber must not reserve the writer every two seconds. This
	// is only a fast path: settings, ownership and cursor are reread inside the
	// transaction so concurrent consumers/settings changes remain atomic.
	var pending bool
	if err := d.QueryRowContext(ctx, `SELECT EXISTS (
SELECT 1 FROM user_notifications n JOIN users u ON u.id = n.user_id
JOIN accounts a ON a.user_id = n.user_id JOIN event_log e ON e.account_id = a.id
WHERE n.user_id = ? AND n.enabled = 1 AND u.status = 'active' AND e.id > n.event_cursor
)`, userID).Scan(&pending); err != nil || !pending {
		return err
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var cursor, revision int64
	var cooldown int
	err = tx.QueryRowContext(ctx, `SELECT event_cursor, revision, cooldown_minutes FROM user_notifications n JOIN users u ON u.id = n.user_id WHERE n.user_id = ? AND n.enabled = 1 AND u.status = 'active'`, userID).Scan(&cursor, &revision, &cooldown)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.id, e.account_id, a.name, e.ts, e.kind, e.level, e.action FROM event_log e JOIN accounts a ON a.id = e.account_id WHERE a.user_id = ? AND e.id > ? ORDER BY e.id LIMIT 200`, userID, cursor)
	if err != nil {
		return err
	}
	var events []EventLog
	for rows.Next() {
		var e EventLog
		if err := rows.Scan(&e.ID, &e.AccountID, &e.AccountName, &e.TS, &e.Kind, &e.Level, &e.Action); err != nil {
			_ = rows.Close()
			return err
		}
		events = append(events, e)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, e := range events {
		cursor = e.ID
		signal := classify(e)
		if signal == nil {
			continue
		}
		if err := recordNotificationSignal(ctx, tx, userID, revision, cooldown, e, *signal, now); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE user_notifications SET event_cursor = ? WHERE user_id = ?`, cursor, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func recordNotificationSignal(ctx context.Context, tx *sql.Tx, userID, revision int64, cooldown int, e EventLog, s NotificationSignal, now time.Time) error {
	var opened, last, count int64
	var severity int
	err := tx.QueryRowContext(ctx, `SELECT opened_ms, last_sent_ms, occurrences, severity FROM notification_incidents WHERE user_id = ? AND account_id = ? AND kind = ?`, userID, e.AccountID, s.Kind).Scan(&opened, &last, &count, &severity)
	fresh := errors.Is(err, sql.ErrNoRows)
	if err != nil && !fresh {
		return err
	}
	if s.Recovered && fresh {
		return nil
	}
	if fresh {
		opened = e.TS.UnixMilli()
	}
	if !s.Recovered {
		count++
	}
	send := fresh || s.Recovered || s.Severity > severity || now.UnixMilli()-last >= int64(cooldown)*60_000
	if send {
		level := "error"
		if s.Severity == 1 {
			level = "warn"
		}
		if s.Recovered {
			level = "info"
		}
		p := NotificationPayload{ID: rand.Text(), Kind: s.Kind, Level: level, Message: s.Message, AccountID: e.AccountID, AccountName: e.AccountName, TS: e.TS, Recovered: s.Recovered, Occurrences: count, DurationSeconds: max(0, (e.TS.UnixMilli()-opened)/1000)}
		if _, err := insertNotification(ctx, tx, userID, revision, p, now); err != nil {
			return err
		}
		last = now.UnixMilli()
	}
	if s.Recovered {
		_, err = tx.ExecContext(ctx, `DELETE FROM notification_incidents WHERE user_id = ? AND account_id = ? AND kind = ?`, userID, e.AccountID, s.Kind)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO notification_incidents(user_id, account_id, kind, opened_ms, last_sent_ms, occurrences, severity) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT(user_id, account_id, kind) DO UPDATE SET last_sent_ms = excluded.last_sent_ms, occurrences = excluded.occurrences, severity = excluded.severity`, userID, e.AccountID, s.Kind, opened, last, count, max(severity, s.Severity))
	}
	return err
}

func insertNotification(ctx context.Context, tx *sql.Tx, userID, revision int64, p NotificationPayload, now time.Time) (int64, error) {
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_outbox WHERE user_id = ? AND status IN ('pending', 'sending')`, userID).Scan(&count); err != nil {
		return 0, err
	}
	if count >= 200 {
		return 0, ErrNotificationQueueFull
	}
	data, err := json.Marshal(p)
	if err != nil {
		return 0, err
	}
	var accountID any
	if p.AccountID > 0 {
		accountID = p.AccountID
	}
	title := p.Message
	if p.AccountName != "" {
		title = p.AccountName + " · " + p.Message
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO notification_outbox(delivery_key, user_id, account_id, revision, payload, title, next_ms, created_ms) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, p.ID, userID, accountID, revision, string(data), title, now.UnixMilli(), now.UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (d *DB) QueueNotificationTest(ctx context.Context, userID int64, now time.Time) (int64, error) {
	if userID <= 0 {
		return 0, ErrNotificationSettings
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE user_notifications SET last_test_ms = ? WHERE user_id = ? AND enabled = 1 AND endpoint_enc <> '' AND last_test_ms <= ? AND EXISTS(SELECT 1 FROM users WHERE id = ? AND status = 'active')`, now.UnixMilli(), userID, now.Add(-time.Minute).UnixMilli(), userID)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected == 0 {
		var active bool
		if err := tx.QueryRowContext(ctx, `SELECT n.enabled AND n.endpoint_enc <> '' AND u.status = 'active' FROM user_notifications n JOIN users u ON u.id = n.user_id WHERE n.user_id = ?`, userID).Scan(&active); err != nil || !active {
			return 0, ErrNotificationSettings
		}
		return 0, ErrNotificationTestCooldown
	}
	var revision int64
	if err := tx.QueryRowContext(ctx, `SELECT revision FROM user_notifications WHERE user_id = ?`, userID).Scan(&revision); err != nil {
		return 0, err
	}
	id, err := insertNotification(ctx, tx, userID, revision, NotificationPayload{ID: rand.Text(), Kind: "test", Level: "info", Message: "这是你的个人 Webhook 测试通知", TS: now, Occurrences: 1}, now)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (d *DB) NotificationDeliveries(ctx context.Context, userID, beforeID int64) ([]NotificationDelivery, error) {
	if userID <= 0 {
		return nil, ErrNotificationSettings
	}
	rows, err := d.QueryContext(ctx, `SELECT id, delivery_key, title, status, attempts, created_ms, last_error FROM notification_outbox WHERE user_id = ? AND (? = 0 OR id < ?) ORDER BY id DESC LIMIT 6`, userID, beforeID, beforeID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []NotificationDelivery
	for rows.Next() {
		var n NotificationDelivery
		if err := rows.Scan(&n.ID, &n.Key, &n.Title, &n.Status, &n.Attempts, &n.CreatedMS, &n.LastError); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// ClaimNotification uses a lease and attempt token (the incremented attempt
// number). Expired workers cannot acknowledge a newer attempt. Native robots
// are paced to one attempt per user per four seconds, atomically across workers
// and restarts, below the 20/minute DingTalk and WeCom limits. External senders
// sharing the same bot can still cause provider throttling.
func (d *DB) ClaimNotification(ctx context.Context, now time.Time) (*NotificationDelivery, error) {
	args := []any{now.UnixMilli(), now.UnixMilli(), now.Add(-24 * time.Hour).UnixMilli(), now.UnixMilli(), now.Add(-4 * time.Second).UnixMilli()}
	var pending bool
	if err := d.QueryRowContext(ctx, `SELECT EXISTS (`+notificationClaimSelect+`)`, args...).Scan(&pending); err != nil || !pending {
		return nil, err
	}
	// Repeat the entire eligibility predicate at the write boundary. The read
	// above avoids idle writes; it is not a lease or a pacing reservation.
	var n NotificationDelivery
	writeArgs := append([]any{now.Add(time.Minute).UnixMilli(), now.UnixMilli()}, args...)
	err := d.writeRowContext(ctx, `UPDATE notification_outbox SET status = 'sending', attempts = attempts + 1, lease_ms = ?, last_attempt_ms = ? WHERE id = (`+notificationClaimSelect+`) RETURNING id, delivery_key, user_id, payload, revision, attempts, created_ms`, writeArgs...).Scan(&n.ID, &n.Key, &n.UserID, &n.Payload, &n.Revision, &n.Attempts, &n.CreatedMS)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &n, err
}

const notificationClaimSelect = `SELECT o.id FROM notification_outbox o WHERE ((o.status = 'pending' AND o.next_ms <= ?) OR (o.status = 'sending' AND o.lease_ms <= ?))
AND o.created_ms >= ? AND o.attempts < 5
AND NOT EXISTS (SELECT 1 FROM notification_outbox older WHERE older.user_id = o.user_id AND older.account_id IS o.account_id AND older.id < o.id AND older.status IN ('pending', 'sending'))
AND NOT EXISTS (SELECT 1 FROM user_notifications settings WHERE settings.user_id = o.user_id AND settings.retry_after_ms > ?)
AND NOT EXISTS (SELECT 1 FROM user_notifications settings JOIN notification_outbox recent ON recent.user_id = settings.user_id WHERE settings.user_id = o.user_id AND settings.provider <> 'custom' AND recent.last_attempt_ms > ?)
ORDER BY o.id LIMIT 1`

// CleanNotificationOutbox runs independently of delivery workers. Bounded
// batches release the writer between maintenance passes; claims enforce expiry
// and attempt limits even before the next cleanup pass marks a row failed.
func (d *DB) CleanNotificationOutbox(ctx context.Context, now time.Time) error {
	if _, err := d.ExecContext(ctx, `UPDATE notification_outbox SET status = 'failed', last_error = '通知已过期或达到重试上限' WHERE id IN (
SELECT id FROM notification_outbox WHERE status IN ('pending', 'sending') AND (created_ms < ? OR (attempts >= 5 AND lease_ms <= ?)) ORDER BY id LIMIT 1000
)`, now.Add(-24*time.Hour).UnixMilli(), now.UnixMilli()); err != nil {
		return err
	}
	_, err := d.ExecContext(ctx, `DELETE FROM notification_outbox WHERE id IN (
SELECT id FROM notification_outbox WHERE status IN ('sent', 'failed', 'cancelled') AND created_ms < ? ORDER BY id LIMIT 1000
)`, now.Add(-7*24*time.Hour).UnixMilli())
	return err
}

// NotificationDestination rechecks ownership, user status and settings revision
// immediately before network delivery. There is intentionally no admin bypass.
func (d *DB) NotificationDestination(ctx context.Context, n *NotificationDelivery) (NotificationTarget, error) {
	var encrypted, secret string
	var target NotificationTarget
	err := d.QueryRowContext(ctx, `SELECT s.endpoint_enc, s.provider, s.signing_secret_enc FROM notification_outbox o JOIN user_notifications s ON s.user_id = o.user_id JOIN users u ON u.id = o.user_id WHERE o.id = ? AND o.user_id = ? AND o.status = 'sending' AND o.attempts = ? AND s.enabled = 1 AND s.revision = ? AND u.status = 'active' AND (o.account_id IS NULL OR EXISTS(SELECT 1 FROM accounts a WHERE a.id = o.account_id AND a.user_id = o.user_id))`, n.ID, n.UserID, n.Attempts, n.Revision).Scan(&encrypted, &target.Provider, &secret)
	if err != nil {
		return target, err
	}
	target.Endpoint, err = d.decodeNotificationCredential(n.UserID, "endpoint", encrypted)
	if err == nil && secret != "" {
		target.SigningSecret, err = d.decodeNotificationCredential(n.UserID, "signing", secret)
	}
	return target, err
}

func (d *DB) FinishNotification(ctx context.Context, n *NotificationDelivery, status, safeError string, next time.Time) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx, `UPDATE notification_outbox SET status = ?, last_error = ?, next_ms = ?, lease_ms = 0 WHERE id = ? AND status = 'sending' AND attempts = ?`, status, safeError, next.UnixMilli(), n.ID, n.Attempts)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	// Backoff belongs to the destination, not just this account's message.
	// A stale worker or changed settings must not postpone a new route.
	if affected > 0 && status == "pending" {
		if _, err := tx.ExecContext(ctx, `UPDATE user_notifications SET retry_after_ms = MAX(retry_after_ms, ?) WHERE user_id = ? AND revision = ?`, next.UnixMilli(), n.UserID, n.Revision); err != nil {
			return err
		}
	}
	return tx.Commit()
}
