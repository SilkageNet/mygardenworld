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
	UserID          int64
	Enabled         bool
	HasEndpoint     bool
	CooldownMinutes int
	Revision        int64
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
	s := NotificationSettings{UserID: userID, CooldownMinutes: 30}
	if userID <= 0 {
		return s, ErrNotificationSettings
	}
	err := d.QueryRowContext(ctx, `SELECT enabled, endpoint_enc <> '', cooldown_minutes, revision FROM user_notifications WHERE user_id = ?`, userID).Scan(&s.Enabled, &s.HasEndpoint, &s.CooldownMinutes, &s.Revision)
	if errors.Is(err, sql.ErrNoRows) {
		err = nil
	}
	return s, err
}

// SaveNotificationSettings retains the secret when endpoint is nil and clears it
// when explicitly empty. Enabling starts at the current log head, not historical
// failures. Changing the destination or enabled state cancels old pending
// deliveries/incidents; changing cooldown alone preserves them.
func (d *DB) SaveNotificationSettings(ctx context.Context, userID int64, enabled bool, endpoint *string, cooldownMinutes int) error {
	if userID <= 0 || cooldownMinutes < 1 || cooldownMinutes > 1440 {
		return ErrNotificationSettings
	}
	var encrypted string
	var err error
	if endpoint != nil && *endpoint != "" {
		encrypted, err = d.encodeNotificationEndpoint(userID, *endpoint)
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
	var previous string
	var previouslyEnabled bool
	if err := tx.QueryRowContext(ctx, `SELECT endpoint_enc, enabled FROM user_notifications n JOIN users u ON u.id = n.user_id WHERE n.user_id = ? AND u.status = 'active'`, userID).Scan(&previous, &previouslyEnabled); err != nil {
		return ErrNotificationSettings
	}
	if endpoint == nil {
		encrypted = previous
	}
	if enabled && encrypted == "" {
		return ErrNotificationSettings
	}
	routingChanged := previouslyEnabled != enabled || encrypted != previous
	if _, err := tx.ExecContext(ctx, `UPDATE user_notifications SET enabled = ?, endpoint_enc = ?, cooldown_minutes = ?, revision = revision + ?, event_cursor = CASE WHEN ? THEN (SELECT COALESCE(MAX(id), 0) FROM event_log) ELSE event_cursor END WHERE user_id = ?`, enabled, encrypted, cooldownMinutes, routingChanged, routingChanged, userID); err != nil {
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

func (d *DB) encodeNotificationEndpoint(userID int64, endpoint string) (string, error) {
	aead, err := d.credentialAEAD()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(endpoint), notificationAAD(userID))), nil
}

func (d *DB) decodeNotificationEndpoint(userID int64, encrypted string) (string, error) {
	aead, err := d.credentialAEAD()
	if err != nil {
		return "", err
	}
	b, err := base64.RawStdEncoding.DecodeString(encrypted)
	if err != nil || len(b) < aead.NonceSize() {
		return "", ErrNotificationSettings
	}
	plain, err := aead.Open(nil, b[:aead.NonceSize()], b[aead.NonceSize():], notificationAAD(userID))
	return string(plain), err
}

func notificationAAD(userID int64) []byte {
	return []byte(fmt.Sprintf("mygardenworld/notification/user/%d", userID))
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
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Acquire SQLite's write reservation before reading a snapshot.
	if _, err := tx.ExecContext(ctx, `UPDATE user_notifications SET event_cursor = event_cursor WHERE user_id = ?`, userID); err != nil {
		return err
	}
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
// number). Expired workers cannot acknowledge a newer attempt.
func (d *DB) ClaimNotification(ctx context.Context, now time.Time) (*NotificationDelivery, error) {
	if _, err := d.ExecContext(ctx, `UPDATE notification_outbox SET status = 'failed', last_error = '通知已过期或达到重试上限' WHERE status IN ('pending', 'sending') AND (created_ms < ? OR (attempts >= 5 AND lease_ms <= ?))`, now.Add(-24*time.Hour).UnixMilli(), now.UnixMilli()); err != nil {
		return nil, err
	}
	if _, err := d.ExecContext(ctx, `DELETE FROM notification_outbox WHERE status IN ('sent', 'failed', 'cancelled') AND created_ms < ?`, now.Add(-7*24*time.Hour).UnixMilli()); err != nil {
		return nil, err
	}
	var n NotificationDelivery
	err := d.QueryRowContext(ctx, `UPDATE notification_outbox SET status = 'sending', attempts = attempts + 1, lease_ms = ? WHERE id = (
SELECT o.id FROM notification_outbox o WHERE ((o.status = 'pending' AND o.next_ms <= ?) OR (o.status = 'sending' AND o.lease_ms <= ?))
AND NOT EXISTS (SELECT 1 FROM notification_outbox older WHERE older.user_id = o.user_id AND older.account_id IS o.account_id AND older.id < o.id AND older.status IN ('pending', 'sending'))
ORDER BY o.id LIMIT 1) RETURNING id, delivery_key, user_id, payload, revision, attempts, created_ms`, now.Add(time.Minute).UnixMilli(), now.UnixMilli(), now.UnixMilli()).Scan(&n.ID, &n.Key, &n.UserID, &n.Payload, &n.Revision, &n.Attempts, &n.CreatedMS)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return &n, err
}

// NotificationEndpoint rechecks ownership, user status and settings revision
// immediately before network delivery. There is intentionally no admin bypass.
func (d *DB) NotificationEndpoint(ctx context.Context, n *NotificationDelivery) (string, error) {
	var encrypted string
	err := d.QueryRowContext(ctx, `SELECT s.endpoint_enc FROM notification_outbox o JOIN user_notifications s ON s.user_id = o.user_id JOIN users u ON u.id = o.user_id WHERE o.id = ? AND o.user_id = ? AND o.status = 'sending' AND o.attempts = ? AND s.enabled = 1 AND s.revision = ? AND u.status = 'active' AND (o.account_id IS NULL OR EXISTS(SELECT 1 FROM accounts a WHERE a.id = o.account_id AND a.user_id = o.user_id))`, n.ID, n.UserID, n.Attempts, n.Revision).Scan(&encrypted)
	if err != nil {
		return "", err
	}
	return d.decodeNotificationEndpoint(n.UserID, encrypted)
}

func (d *DB) FinishNotification(ctx context.Context, n *NotificationDelivery, status, safeError string, next time.Time) error {
	_, err := d.ExecContext(ctx, `UPDATE notification_outbox SET status = ?, last_error = ?, next_ms = ?, lease_ms = 0 WHERE id = ? AND status = 'sending' AND attempts = ?`, status, safeError, next.UnixMilli(), n.ID, n.Attempts)
	return err
}
