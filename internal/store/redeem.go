package store

import (
	"cmp"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
)

const (
	RedeemValidationPending         = "pending"
	RedeemAttemptStatusRunning      = "running"
	RedeemValidationSuccess         = "success"
	RedeemValidationAlreadyRedeemed = "already_redeemed"
	RedeemValidationExpired         = "expired"
	RedeemValidationInvalid         = "invalid"
	RedeemValidationRetryable       = "retryable"
	RedeemValidationUnknown         = "unknown"

	RedeemSourceMyGardenWorld = "mygardenworld"
	RedeemSourceCustomHTTP    = "custom_http"

	RedeemAttemptFilterAll         = "all"
	RedeemAttemptFilterRedeemed    = "redeemed"
	RedeemAttemptFilterUnavailable = "unavailable"
	RedeemAttemptFilterAttention   = "attention"

	// RedeemAttemptLeaseDuration exceeds the bounded account-start and game-RPC
	// windows while still allowing abandoned work to recover during runtime.
	RedeemAttemptLeaseDuration = 3 * time.Minute
)

type RedeemCode struct {
	ID                  int64
	Fingerprint         string
	Code                string
	NormalizedCode      string
	Channel             string
	ExpiresAt           *time.Time
	Validation          string
	PropagationState    string
	LocalVerifiedAt     *time.Time
	CommunityVerifiedAt *time.Time
	OriginInstanceID    string
	LastMessage         string
	Revision            int64
	FirstSeenAt         time.Time
	UpdatedAt           time.Time
	ExpiryOverridden    bool
}

type RedeemCodeInput struct {
	Code               string
	Channel            string
	ExpiresAt          *time.Time
	ReportedValidation string
	OriginInstanceID   string
	SourceID           *int64
	SourceKey          string
}

type RedeemSource struct {
	ID                   int64
	Name                 string
	Type                 string
	BaseURL              string
	Channel              string
	ParserConfigJSON     string
	Enabled              bool
	PushEnabled          bool
	PollIntervalSeconds  int
	RemoteInstanceID     string
	Cursor               string
	LastSyncAt           *time.Time
	LastError            string
	ObservedCount        int64
	TrustedCount         int64
	SuccessCount         int64
	AlreadyRedeemedCount int64
	ExpiredCount         int64
	InvalidCount         int64
	PendingCount         int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type RedeemSourceInput struct {
	ID                  int64
	Name                string
	Type                string
	BaseURL             string
	Channel             string
	ParserConfigJSON    string
	Enabled             bool
	PushEnabled         bool
	PollIntervalSeconds int
}

type RedeemAttempt struct {
	ID           int64
	CodeID       int64
	AccountID    int64
	AccountName  string
	Channel      string
	Code         string
	Fingerprint  string
	ExpiresAt    *time.Time
	AttemptCount int
	RunToken     string
	LeaseUntil   time.Time
}

type RedeemAttemptRecord struct {
	ID           int64
	AccountID    int64
	Channel      string
	Code         string
	Status       string
	Message      string
	AttemptCount int
	AttemptedAt  *time.Time
	ExpiresAt    *time.Time
	UpdatedAt    time.Time
}

type RedeemAttemptSummary struct {
	Total           int64
	Success         int64
	AlreadyRedeemed int64
	Expired         int64
	Invalid         int64
	Pending         int64
	Running         int64
	Retryable       int64
	Unknown         int64
}

type ListRedeemAttemptsOptions struct {
	AccountID int64
	BeforeID  int64
	Limit     int
	Filter    string
}

type RedeemOutboxItem struct {
	ID           int64
	SourceID     int64
	SourceName   string
	BaseURL      string
	Code         RedeemCode
	AttemptCount int
}

func NormalizeRedeemCode(code string) string {
	return norm.NFC.String(strings.TrimSpace(code))
}

func RedeemFingerprint(code, channel string) string {
	normalized := NormalizeRedeemCode(code)
	sum := sha256.Sum256([]byte("mygardenworld-redeem-v1\x00" + channel + "\x00" + normalized))
	return hex.EncodeToString(sum[:])
}

func (d *DB) RedeemInstanceID(ctx context.Context) (string, error) {
	var instanceID string
	err := d.QueryRowContext(ctx, `SELECT instance_id FROM redeem_node_state WHERE id = 1`).Scan(&instanceID)
	if err == nil {
		return instanceID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("read redeem instance id: %w", err)
	}
	instanceID = uuid.NewString()
	if _, err := d.ExecContext(ctx, `INSERT OR IGNORE INTO redeem_node_state(id, instance_id) VALUES (1, ?)`, instanceID); err != nil {
		return "", fmt.Errorf("create redeem instance id: %w", err)
	}
	if err := d.QueryRowContext(ctx, `SELECT instance_id FROM redeem_node_state WHERE id = 1`).Scan(&instanceID); err != nil {
		return "", fmt.Errorf("reload redeem instance id: %w", err)
	}
	return instanceID, nil
}

func (d *DB) UpsertRedeemCode(ctx context.Context, in RedeemCodeInput) (*RedeemCode, bool, error) {
	in.Code = strings.TrimSpace(in.Code)
	in.Channel = strings.ToLower(strings.TrimSpace(in.Channel))
	in.ReportedValidation = normalizeRedeemValidation(in.ReportedValidation)
	if in.Code == "" || len([]rune(in.Code)) > 128 {
		return nil, false, errors.New("redeem code must contain 1 to 128 characters")
	}
	if strings.ContainsFunc(in.Code, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return nil, false, errors.New("redeem code contains control characters")
	}
	if in.Channel != "ios" && in.Channel != "alipay" {
		return nil, false, errors.New("redeem channel must be ios or alipay")
	}
	if in.ExpiresAt != nil {
		utc := in.ExpiresAt.UTC()
		in.ExpiresAt = &utc
	}
	if in.SourceKey == "" {
		in.SourceKey = "public"
	}
	normalized := NormalizeRedeemCode(in.Code)
	fingerprint := RedeemFingerprint(normalized, in.Channel)
	now := time.Now().UTC()

	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()

	existing, err := getRedeemCodeByFingerprint(ctx, tx, fingerprint)
	created := false
	switch {
	case errors.Is(err, sql.ErrNoRows):
		revision, revErr := nextRedeemRevision(ctx, tx)
		if revErr != nil {
			return nil, false, revErr
		}
		res, insertErr := tx.ExecContext(ctx, `
INSERT INTO redeem_codes(
    fingerprint, code, normalized_code, channel, expires_at, validation,
    community_verified_at, origin_instance_id, revision, first_seen_at, updated_at
) VALUES (?, ?, ?, ?, ?, 'pending', ?, ?, ?, ?, ?)`,
			fingerprint, in.Code, normalized, in.Channel, nullableTime(in.ExpiresAt),
			communityVerifiedTime(in.ReportedValidation, now), strings.TrimSpace(in.OriginInstanceID), revision, now, now,
		)
		if insertErr != nil {
			return nil, false, fmt.Errorf("insert redeem code: %w", insertErr)
		}
		id, idErr := res.LastInsertId()
		if idErr != nil {
			return nil, false, idErr
		}
		existing, err = getRedeemCodeByID(ctx, tx, id)
		created = true
	case err != nil:
		return nil, false, err
	default:
		expiresAt := existing.ExpiresAt
		if !existing.ExpiryOverridden {
			expiresAt = mergeRedeemExpiry(existing.ExpiresAt, in.ExpiresAt)
		}
		communityAt := existing.CommunityVerifiedAt
		if communityAt == nil && communityVerifiedTime(in.ReportedValidation, now) != nil {
			communityAt = &now
		}
		if !equalRedeemTime(existing.ExpiresAt, expiresAt) || !equalRedeemTime(existing.CommunityVerifiedAt, communityAt) {
			revision, revErr := nextRedeemRevision(ctx, tx)
			if revErr != nil {
				return nil, false, revErr
			}
			if _, err := tx.ExecContext(ctx, `
UPDATE redeem_codes
SET expires_at = ?, community_verified_at = ?, revision = ?, updated_at = ?
WHERE id = ?`, nullableTime(expiresAt), nullableTime(communityAt), revision, now, existing.ID); err != nil {
				return nil, false, fmt.Errorf("refresh redeem code: %w", err)
			}
			existing, err = getRedeemCodeByID(ctx, tx, existing.ID)
		}
	}
	if err != nil {
		return nil, false, err
	}
	var sourceID any
	if in.SourceID != nil && *in.SourceID > 0 {
		sourceID = *in.SourceID
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO redeem_code_observations(
    redeem_code_id, source_id, source_key, origin_instance_id, expires_at, validation, observed_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(redeem_code_id, source_key) DO UPDATE SET
    source_id = excluded.source_id,
    origin_instance_id = excluded.origin_instance_id,
    expires_at = excluded.expires_at,
    validation = excluded.validation,
    observed_at = excluded.observed_at`,
		existing.ID, sourceID, in.SourceKey, strings.TrimSpace(in.OriginInstanceID), nullableTime(in.ExpiresAt), in.ReportedValidation, now,
	); err != nil {
		return nil, false, fmt.Errorf("record redeem observation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return existing, created, nil
}

// BrowseRedeemCodes returns one newest-first page for the human-facing
// registry. It is deliberately separate from ListRedeemCodes, whose ascending
// revision cursor is a synchronization change feed and must not be overloaded
// with presentation semantics.
func (d *DB) BrowseRedeemCodes(ctx context.Context, offset, limit int, history bool) ([]*RedeemCode, int64, int64, error) {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	now := time.Now().UTC()
	tx, err := d.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	const historical = `validation IN ('expired', 'invalid') OR (expires_at IS NOT NULL AND expires_at <= ?)`
	var activeTotal, historyTotal int64
	if err := tx.QueryRowContext(ctx, `
SELECT COALESCE(SUM(CASE WHEN NOT (`+historical+`) THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN `+historical+` THEN 1 ELSE 0 END), 0)
FROM redeem_codes`, now, now).Scan(&activeTotal, &historyTotal); err != nil {
		return nil, 0, 0, err
	}

	predicate := `NOT (` + historical + `)`
	if history {
		predicate = historical
	}
	rows, err := tx.QueryContext(ctx, `
SELECT id, fingerprint, code, normalized_code, channel, expires_at, validation,
       propagation_state, local_verified_at, community_verified_at,
       origin_instance_id, last_message, revision, first_seen_at, updated_at,
       expiry_overridden
FROM redeem_codes
WHERE `+predicate+`
ORDER BY first_seen_at DESC, id DESC
LIMIT ? OFFSET ?`, now, limit, offset)
	if err != nil {
		return nil, 0, 0, err
	}
	entries := make([]*RedeemCode, 0, limit)
	for rows.Next() {
		entry, scanErr := scanRedeemCode(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, 0, 0, scanErr
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, 0, 0, err
	}
	if err := rows.Close(); err != nil {
		return nil, 0, 0, err
	}
	if err := tx.Commit(); err != nil {
		return nil, 0, 0, err
	}
	return entries, activeTotal, historyTotal, nil
}

// SetRedeemCodeExpiryOverride makes an administrator-selected expiry
// authoritative on this node. A nil expiry means permanent. Subsequent source
// observations are still retained, but cannot silently replace the override.
func (d *DB) SetRedeemCodeExpiryOverride(ctx context.Context, fingerprint string, expiresAt *time.Time) (*RedeemCode, error) {
	return d.updateRedeemCodeExpiryOverride(ctx, strings.TrimSpace(fingerprint), expiresAt, true)
}

// ClearRedeemCodeExpiryOverride restores the aggregate source-reported expiry.
func (d *DB) ClearRedeemCodeExpiryOverride(ctx context.Context, fingerprint string) (*RedeemCode, error) {
	return d.updateRedeemCodeExpiryOverride(ctx, strings.TrimSpace(fingerprint), nil, false)
}

func (d *DB) updateRedeemCodeExpiryOverride(ctx context.Context, fingerprint string, expiresAt *time.Time, override bool) (*RedeemCode, error) {
	if fingerprint == "" {
		return nil, errors.New("redeem code fingerprint required")
	}
	if expiresAt != nil {
		utc := expiresAt.UTC()
		expiresAt = &utc
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	entry, err := getRedeemCodeByFingerprint(ctx, tx, fingerprint)
	if err != nil {
		return nil, err
	}

	effective := expiresAt
	if !override {
		var observations, permanent int64
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(SUM(CASE WHEN expires_at IS NULL THEN 1 ELSE 0 END), 0)
FROM redeem_code_observations WHERE redeem_code_id = ?`, entry.ID).Scan(&observations, &permanent); err != nil {
			return nil, err
		}
		switch {
		case observations == 0:
			effective = entry.ExpiresAt
		case permanent > 0:
			effective = nil
		default:
			var latest time.Time
			if err := tx.QueryRowContext(ctx, `
SELECT expires_at FROM redeem_code_observations
WHERE redeem_code_id = ? AND expires_at IS NOT NULL
ORDER BY expires_at DESC LIMIT 1`, entry.ID).Scan(&latest); err != nil {
				return nil, err
			}
			effective = &latest
		}
	}
	revision, err := nextRedeemRevision(ctx, tx)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `
UPDATE redeem_codes
SET expires_at = ?, expiry_overridden = ?, revision = ?, updated_at = ?
WHERE id = ?`, nullableTime(effective), override, revision, now, entry.ID); err != nil {
		return nil, fmt.Errorf("update redeem code expiry: %w", err)
	}
	updated, err := getRedeemCodeByID(ctx, tx, entry.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated, nil
}

func nextRedeemRevision(ctx context.Context, tx *sql.Tx) (int64, error) {
	var revision int64
	err := tx.QueryRowContext(ctx, `
UPDATE redeem_node_state SET next_revision = next_revision + 1 WHERE id = 1
RETURNING next_revision`).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, errors.New("redeem node state is not initialized")
	}
	if err != nil {
		return 0, fmt.Errorf("advance redeem revision: %w", err)
	}
	return revision, nil
}

func mergeRedeemExpiry(current, incoming *time.Time) *time.Time {
	if current == nil || incoming == nil {
		return nil
	}
	if incoming.After(*current) {
		copy := incoming.UTC()
		return &copy
	}
	copy := current.UTC()
	return &copy
}

func equalRedeemTime(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func communityVerifiedTime(validation string, now time.Time) *time.Time {
	if validation != RedeemValidationSuccess && validation != RedeemValidationAlreadyRedeemed {
		return nil
	}
	copy := now.UTC()
	return &copy
}

func normalizeRedeemValidation(value string) string {
	switch value {
	case RedeemValidationSuccess, RedeemValidationAlreadyRedeemed, RedeemValidationExpired,
		RedeemValidationInvalid, RedeemValidationRetryable, RedeemValidationUnknown:
		return value
	default:
		return RedeemValidationPending
	}
}

func (d *DB) ListRedeemCodes(ctx context.Context, afterRevision int64, limit int, includeExpired, onlyPropagatable bool, channels []string) ([]*RedeemCode, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	where := `revision > ? AND (? OR expires_at IS NULL OR expires_at > ?)`
	args := []any{afterRevision, includeExpired, time.Now().UTC()}
	if onlyPropagatable {
		where += ` AND validation IN ('success', 'already_redeemed') AND propagation_state = 'eligible'`
	}
	if len(channels) > 0 {
		placeholders := make([]string, 0, len(channels))
		for _, channel := range channels {
			channel = strings.ToLower(strings.TrimSpace(channel))
			if channel != "ios" && channel != "alipay" {
				continue
			}
			placeholders = append(placeholders, "?")
			args = append(args, channel)
		}
		if len(placeholders) > 0 {
			where += ` AND channel IN (` + strings.Join(placeholders, ",") + `)`
		}
	}
	args = append(args, limit)
	rows, err := d.QueryContext(ctx, `
SELECT id, fingerprint, code, normalized_code, channel, expires_at, validation,
       propagation_state, local_verified_at, community_verified_at,
       origin_instance_id, last_message, revision, first_seen_at, updated_at,
       expiry_overridden
FROM redeem_codes WHERE `+where+` ORDER BY revision ASC LIMIT ?`, args...)
	if err != nil {
		return nil, afterRevision, err
	}
	defer func() { _ = rows.Close() }()
	var out []*RedeemCode
	next := afterRevision
	for rows.Next() {
		item, err := scanRedeemCode(rows)
		if err != nil {
			return nil, next, err
		}
		out = append(out, item)
		if item.Revision > next {
			next = item.Revision
		}
	}
	return out, next, rows.Err()
}

func (d *DB) EnsureRedeemAttempts(ctx context.Context) error {
	now := time.Now().UTC()
	_, err := d.ExecContext(ctx, `
INSERT OR IGNORE INTO redeem_attempts(redeem_code_id, account_id, status, created_at, updated_at)
SELECT c.id, a.id, 'pending', ?, ?
FROM redeem_codes c
JOIN accounts a ON a.channel = c.channel
JOIN users u ON u.id = a.user_id AND u.status = 'active'
WHERE c.validation NOT IN ('expired', 'invalid')
  AND (c.expires_at IS NULL OR c.expires_at > ?)`, now, now, now)
	return err
}

// DueRedeemAttemptAccountIDs returns only accounts that currently have work
// eligible by time and code state. The redeem service evaluates live-session
// policy for this small set instead of loading every account policy on each
// worker tick.
func (d *DB) DueRedeemAttemptAccountIDs(ctx context.Context) ([]int64, error) {
	now := time.Now().UTC()
	rows, err := d.QueryContext(ctx, `
SELECT DISTINCT a.account_id
FROM redeem_attempts a
JOIN redeem_codes c ON c.id = a.redeem_code_id
JOIN accounts ac ON ac.id = a.account_id
JOIN users u ON u.id = ac.user_id AND u.status = 'active'
WHERE a.status IN ('pending', 'retryable')
  AND (a.retry_at IS NULL OR a.retry_at <= ?)
  AND c.validation NOT IN ('expired', 'invalid')
  AND (c.expires_at IS NULL OR c.expires_at > ?)
ORDER BY a.account_id`, now, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var accountIDs []int64
	for rows.Next() {
		var accountID int64
		if err := rows.Scan(&accountID); err != nil {
			return nil, err
		}
		accountIDs = append(accountIDs, accountID)
	}
	return accountIDs, rows.Err()
}

func (d *DB) ListRedeemAttempts(ctx context.Context, opts ListRedeemAttemptsOptions) ([]RedeemAttemptRecord, RedeemAttemptSummary, error) {
	var summary RedeemAttemptSummary
	tx, err := d.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, summary, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*),
       COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status = 'already_redeemed' THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status = 'expired' THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status = 'invalid' THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status = 'running' THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status = 'retryable' THEN 1 ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status = 'unknown' THEN 1 ELSE 0 END), 0)
FROM redeem_attempts
WHERE account_id = ?`, opts.AccountID).Scan(
		&summary.Total, &summary.Success, &summary.AlreadyRedeemed, &summary.Expired,
		&summary.Invalid, &summary.Pending, &summary.Running, &summary.Retryable, &summary.Unknown,
	); err != nil {
		return nil, summary, err
	}

	query := `
SELECT a.id, a.account_id, c.channel, c.code, a.status, a.message, a.attempt_count,
       a.attempted_at, c.expires_at, a.updated_at
FROM redeem_attempts a
JOIN redeem_codes c ON c.id = a.redeem_code_id
WHERE a.account_id = ?`
	args := []any{opts.AccountID}
	switch opts.Filter {
	case RedeemAttemptFilterRedeemed:
		query += ` AND a.status IN ('success', 'already_redeemed')`
	case RedeemAttemptFilterUnavailable:
		query += ` AND a.status IN ('expired', 'invalid')`
	case RedeemAttemptFilterAttention:
		query += ` AND a.status IN ('pending', 'running', 'retryable', 'unknown')`
	}
	if opts.BeforeID > 0 {
		query += ` AND a.id < ?`
		args = append(args, opts.BeforeID)
	}
	query += ` ORDER BY a.id DESC LIMIT ?`
	args = append(args, opts.Limit)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, summary, err
	}
	records := make([]RedeemAttemptRecord, 0, opts.Limit)
	for rows.Next() {
		var record RedeemAttemptRecord
		var attemptedAt, expiresAt sql.NullTime
		if err := rows.Scan(&record.ID, &record.AccountID, &record.Channel, &record.Code,
			&record.Status, &record.Message, &record.AttemptCount, &attemptedAt, &expiresAt,
			&record.UpdatedAt); err != nil {
			return nil, summary, err
		}
		record.AttemptedAt = nullTimePtr(attemptedAt)
		record.ExpiresAt = nullTimePtr(expiresAt)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, summary, err
	}
	if err := rows.Close(); err != nil {
		return nil, summary, err
	}
	if err := tx.Commit(); err != nil {
		return nil, summary, err
	}
	return records, summary, nil
}

func (d *DB) RecoverRedeemWork(ctx context.Context) error {
	now := time.Now().UTC()
	_, err := d.ExecContext(ctx, `
UPDATE redeem_attempts
SET status = 'retryable', retry_at = ?, run_token = '', lease_until = NULL,
    message = 'daemon restarted during attempt', updated_at = ?
WHERE status = 'running'`, now, now)
	if err != nil {
		return err
	}
	_, err = d.ExecContext(ctx, `
UPDATE redeem_exchange_outbox
SET status = 'pending', next_attempt_at = ?, last_error = 'daemon restarted during delivery', updated_at = ?
WHERE status = 'sending'`, now, now)
	return err
}

// RecoverExpiredRedeemAttempts requeues work whose owner disappeared without
// completing or releasing its lease. Completion tokens prevent a late owner
// from overwriting the subsequently retried result.
func (d *DB) RecoverExpiredRedeemAttempts(ctx context.Context, now time.Time) ([]int64, error) {
	now = now.UTC()
	rows, err := d.QueryContext(ctx, `
UPDATE redeem_attempts
SET status = 'retryable', retry_at = ?, run_token = '', lease_until = NULL,
    message = '兑换任务执行超时，已自动重新排队', updated_at = ?
WHERE status = 'running' AND (lease_until IS NULL OR lease_until <= ?)
RETURNING account_id`, now, now, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	seen := make(map[int64]struct{})
	var accountIDs []int64
	for rows.Next() {
		var accountID int64
		if err := rows.Scan(&accountID); err != nil {
			return nil, err
		}
		if _, ok := seen[accountID]; !ok {
			seen[accountID] = struct{}{}
			accountIDs = append(accountIDs, accountID)
		}
	}
	return accountIDs, rows.Err()
}

func (d *DB) NextRedeemAttempt(ctx context.Context) (*RedeemAttempt, error) {
	return d.nextRedeemAttempt(ctx, nil)
}

// NextRedeemAttemptForAccounts claims the oldest due attempt belonging to one
// of accountIDs. The redeem worker uses this to keep offline ONLINE_ONLY
// accounts pending without repeatedly claiming them or creating a game
// session. An empty account list is intentionally not equivalent to an
// unrestricted claim.
func (d *DB) NextRedeemAttemptForAccounts(ctx context.Context, accountIDs []int64) (*RedeemAttempt, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}
	return d.nextRedeemAttempt(ctx, accountIDs)
}

func (d *DB) nextRedeemAttempt(ctx context.Context, accountIDs []int64) (*RedeemAttempt, error) {
	now := time.Now().UTC()
	leaseUntil := now.Add(RedeemAttemptLeaseDuration)
	runToken := uuid.NewString()
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	query := `
SELECT a.id, a.redeem_code_id, a.account_id, ac.name, c.channel, c.code,
       c.fingerprint, c.expires_at, a.attempt_count
FROM redeem_attempts a
JOIN redeem_codes c ON c.id = a.redeem_code_id
JOIN accounts ac ON ac.id = a.account_id
JOIN users u ON u.id = ac.user_id AND u.status = 'active'
WHERE a.status IN ('pending', 'retryable')
  AND (a.retry_at IS NULL OR a.retry_at <= ?)
  AND c.validation NOT IN ('expired', 'invalid')
  AND (c.expires_at IS NULL OR c.expires_at > ?)`
	args := []any{now, now}
	if accountIDs != nil {
		query += ` AND a.account_id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(accountIDs)), ",") + `)`
		for _, accountID := range accountIDs {
			args = append(args, accountID)
		}
	}
	query += ` ORDER BY a.id ASC LIMIT 1`
	row := tx.QueryRowContext(ctx, query, args...)
	var item RedeemAttempt
	var expires sql.NullTime
	if err := row.Scan(&item.ID, &item.CodeID, &item.AccountID, &item.AccountName, &item.Channel,
		&item.Code, &item.Fingerprint, &expires, &item.AttemptCount); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	item.ExpiresAt = nullTimePtr(expires)
	res, err := tx.ExecContext(ctx, `
UPDATE redeem_attempts
SET status = 'running', message = '', retry_at = NULL, run_token = ?, lease_until = ?, updated_at = ?
WHERE id = ? AND status IN ('pending', 'retryable')`, runToken, leaseUntil, now, item.ID)
	if err != nil {
		return nil, err
	}
	if count, _ := res.RowsAffected(); count != 1 {
		return nil, nil
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	item.RunToken = runToken
	item.LeaseUntil = leaseUntil
	return &item, nil
}

// ReleaseRedeemAttempt returns a claimed attempt to the pending queue without
// counting a game RPC attempt. It is used when an account changed to
// ONLINE_ONLY between eligibility selection and processing.
func (d *DB) ReleaseRedeemAttempt(ctx context.Context, attemptID int64, runToken, message string, retryAt *time.Time) error {
	if strings.TrimSpace(runToken) == "" {
		return errors.New("redeem attempt run token required")
	}
	result, err := d.ExecContext(ctx, `
UPDATE redeem_attempts
SET status = 'pending', message = ?, retry_at = ?, run_token = '', lease_until = NULL, updated_at = ?
WHERE id = ? AND status = 'running' AND run_token = ?`, strings.TrimSpace(message), nullableTime(retryAt), time.Now().UTC(), attemptID, runToken)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return errors.New("redeem attempt lease is no longer active")
	}
	return nil
}

// WakeRedeemAttemptsForAccount makes transport-retryable attempts immediately
// due after the account establishes a live session. Pending attempts already
// have no retry deadline and need no update.
func (d *DB) WakeRedeemAttemptsForAccount(ctx context.Context, accountID int64) (int64, error) {
	now := time.Now().UTC()
	result, err := d.ExecContext(ctx, `
UPDATE redeem_attempts
SET retry_at = ?, updated_at = ?
WHERE account_id = ? AND status = 'retryable' AND retry_at > ?`, now, now, accountID, now)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *DB) CompleteRedeemAttempt(ctx context.Context, attemptID int64, runToken, status, message string, retryAt *time.Time) error {
	if strings.TrimSpace(runToken) == "" {
		return errors.New("redeem attempt run token required")
	}
	status = normalizeRedeemValidation(status)
	if status == RedeemValidationPending {
		return errors.New("terminal or retryable redeem status required")
	}
	now := time.Now().UTC()
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var codeID int64
	if err := tx.QueryRowContext(ctx, `SELECT redeem_code_id FROM redeem_attempts WHERE id = ?`, attemptID).Scan(&codeID); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
UPDATE redeem_attempts
SET status = ?, message = ?, retry_at = ?, attempt_count = attempt_count + 1,
    attempted_at = ?, run_token = '', lease_until = NULL, updated_at = ?
WHERE id = ? AND status = 'running' AND run_token = ?`, status, strings.TrimSpace(message), nullableTime(retryAt), now, now, attemptID, runToken)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count != 1 {
		return errors.New("redeem attempt lease is no longer active")
	}
	current, err := getRedeemCodeByID(ctx, tx, codeID)
	if err != nil {
		return err
	}
	revision, err := nextRedeemRevision(ctx, tx)
	if err != nil {
		return err
	}
	switch status {
	case RedeemValidationSuccess:
		if _, err := tx.ExecContext(ctx, `
UPDATE redeem_codes SET validation = 'success', propagation_state = 'eligible',
    local_verified_at = COALESCE(local_verified_at, ?), last_message = ?, revision = ?, updated_at = ? WHERE id = ?`,
			now, message, revision, now, codeID); err != nil {
			return err
		}
		if err := enqueueRedeemPropagation(ctx, tx, codeID, current.OriginInstanceID, now); err != nil {
			return err
		}
	case RedeemValidationAlreadyRedeemed:
		if current.Validation != RedeemValidationSuccess {
			if _, err := tx.ExecContext(ctx, `
UPDATE redeem_codes SET validation = 'already_redeemed', propagation_state = 'eligible',
    local_verified_at = COALESCE(local_verified_at, ?), last_message = ?, revision = ?, updated_at = ? WHERE id = ?`,
				now, message, revision, now, codeID); err != nil {
				return err
			}
		}
		if err := enqueueRedeemPropagation(ctx, tx, codeID, current.OriginInstanceID, now); err != nil {
			return err
		}
	case RedeemValidationExpired, RedeemValidationInvalid:
		propagation := "suppressed_expired"
		if status == RedeemValidationInvalid {
			propagation = "suppressed_invalid"
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE redeem_codes SET validation = ?, propagation_state = ?, last_message = ?, revision = ?, updated_at = ? WHERE id = ?`,
			status, propagation, message, revision, now, codeID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE redeem_attempts SET status = ?, message = ?, retry_at = NULL, updated_at = ?
WHERE redeem_code_id = ? AND status IN ('pending', 'retryable')`, status, message, now, codeID); err != nil {
			return err
		}
	case RedeemValidationRetryable:
		if _, err := tx.ExecContext(ctx, `
UPDATE redeem_codes SET validation = 'retryable', last_message = ?, revision = ?, updated_at = ? WHERE id = ?`,
			message, revision, now, codeID); err != nil {
			return err
		}
	case RedeemValidationUnknown:
		if _, err := tx.ExecContext(ctx, `
UPDATE redeem_codes SET validation = 'unknown', last_message = ?, revision = ?, updated_at = ? WHERE id = ?`,
			message, revision, now, codeID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func enqueueRedeemPropagation(ctx context.Context, tx *sql.Tx, codeID int64, originInstanceID string, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO redeem_exchange_outbox(source_id, redeem_code_id, status, next_attempt_at, created_at, updated_at)
SELECT id, ?, 'pending', ?, ?, ? FROM redeem_sources
WHERE type = 'mygardenworld' AND enabled = 1 AND push_enabled = 1
  AND (remote_instance_id = '' OR remote_instance_id <> ?)
  AND id NOT IN (
      SELECT source_id FROM redeem_code_observations
      WHERE redeem_code_id = ? AND source_id IS NOT NULL
  )`, codeID, now, now, now, originInstanceID, codeID)
	return err
}

func (d *DB) UpsertRedeemSource(ctx context.Context, in RedeemSourceInput) (*RedeemSource, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Type = strings.TrimSpace(in.Type)
	in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	in.Channel = strings.TrimSpace(in.Channel)
	if in.ParserConfigJSON == "" {
		in.ParserConfigJSON = "{}"
	}
	if in.Name == "" || in.BaseURL == "" {
		return nil, errors.New("redeem source name and url required")
	}
	if in.Type != RedeemSourceMyGardenWorld && in.Type != RedeemSourceCustomHTTP {
		return nil, errors.New("invalid redeem source type")
	}
	if in.Type == RedeemSourceCustomHTTP && in.Channel != "ios" && in.Channel != "alipay" {
		return nil, errors.New("custom redeem source channel required")
	}
	if in.Type == RedeemSourceMyGardenWorld {
		in.Channel = ""
	}
	if in.PollIntervalSeconds < 60 {
		in.PollIntervalSeconds = 300
	}
	if in.Type != RedeemSourceMyGardenWorld {
		in.PushEnabled = false
	}
	now := time.Now().UTC()
	if in.ID <= 0 {
		res, err := d.ExecContext(ctx, `
INSERT INTO redeem_sources(name, type, base_url, channel, parser_config_json, enabled, push_enabled,
    poll_interval_seconds, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, in.Name, in.Type, in.BaseURL, in.Channel, in.ParserConfigJSON,
			in.Enabled, in.PushEnabled, in.PollIntervalSeconds, now, now)
		if err != nil {
			return nil, err
		}
		in.ID, err = res.LastInsertId()
		if err != nil {
			return nil, err
		}
	} else {
		res, err := d.ExecContext(ctx, `
UPDATE redeem_sources SET name = ?, type = ?, base_url = ?, channel = ?, parser_config_json = ?,
	    enabled = ?, push_enabled = ?, poll_interval_seconds = ?,
	    cursor = CASE WHEN type <> ? OR base_url <> ? THEN '' ELSE cursor END,
	    remote_instance_id = CASE WHEN type <> ? OR base_url <> ? THEN '' ELSE remote_instance_id END,
	    updated_at = ? WHERE id = ?`,
			in.Name, in.Type, in.BaseURL, in.Channel, in.ParserConfigJSON, in.Enabled, in.PushEnabled,
			in.PollIntervalSeconds, in.Type, in.BaseURL, in.Type, in.BaseURL,
			now, in.ID)
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil, sql.ErrNoRows
		}
	}
	return d.GetRedeemSource(ctx, in.ID)
}

func (d *DB) GetRedeemSource(ctx context.Context, id int64) (*RedeemSource, error) {
	return scanRedeemSource(d.QueryRowContext(ctx, redeemSourceSelect+` WHERE s.id = ?`, id))
}

func (d *DB) ListRedeemSources(ctx context.Context) ([]*RedeemSource, error) {
	rows, err := d.QueryContext(ctx, redeemSourceSelect+` ORDER BY s.id ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*RedeemSource
	for rows.Next() {
		item, err := scanRedeemSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (d *DB) DueRedeemSources(ctx context.Context, now time.Time) ([]*RedeemSource, error) {
	rows, err := d.QueryContext(ctx, redeemSourceScheduleSelect)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var sources []*RedeemSource
	for rows.Next() {
		source, err := scanRedeemSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	now = now.UTC()
	due := make([]*RedeemSource, 0, len(sources))
	for _, source := range sources {
		if !source.Enabled {
			continue
		}
		if source.LastSyncAt == nil || !source.LastSyncAt.Add(time.Duration(source.PollIntervalSeconds)*time.Second).After(now) {
			due = append(due, source)
		}
	}
	slices.SortStableFunc(due, func(left, right *RedeemSource) int {
		if left.LastSyncAt == nil {
			if right.LastSyncAt == nil {
				return cmp.Compare(left.ID, right.ID)
			}
			return -1
		}
		if right.LastSyncAt == nil {
			return 1
		}
		if order := left.LastSyncAt.Compare(*right.LastSyncAt); order != 0 {
			return order
		}
		return cmp.Compare(left.ID, right.ID)
	})
	return due, nil
}

func (d *DB) UpdateRedeemSourceSync(ctx context.Context, id int64, cursor, remoteInstanceID, lastError string) error {
	now := time.Now().UTC()
	_, err := d.ExecContext(ctx, `
UPDATE redeem_sources SET cursor = ?,
    remote_instance_id = CASE WHEN ? = '' THEN remote_instance_id ELSE ? END,
    last_sync_at = ?, last_error = ?, updated_at = ? WHERE id = ?`,
		cursor, remoteInstanceID, remoteInstanceID, now, strings.TrimSpace(lastError), now, id)
	return err
}

func (d *DB) DeleteRedeemSource(ctx context.Context, id int64) error {
	res, err := d.ExecContext(ctx, `DELETE FROM redeem_sources WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (d *DB) NextRedeemOutbox(ctx context.Context) (*RedeemOutboxItem, error) {
	now := time.Now().UTC()
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	row := tx.QueryRowContext(ctx, `
	SELECT o.id, s.id, s.name, s.base_url, o.attempt_count,
	       c.id, c.fingerprint, c.code, c.normalized_code, c.channel, c.expires_at, c.validation,
	       c.propagation_state, c.local_verified_at, c.community_verified_at,
	       c.origin_instance_id, c.last_message, c.revision, c.first_seen_at, c.updated_at,
	       c.expiry_overridden
FROM redeem_exchange_outbox o
JOIN redeem_sources s ON s.id = o.source_id AND s.enabled = 1 AND s.push_enabled = 1
JOIN redeem_codes c ON c.id = o.redeem_code_id
WHERE o.status = 'pending' AND (o.next_attempt_at IS NULL OR o.next_attempt_at <= ?)
  AND (c.expires_at IS NULL OR c.expires_at > ?)
  AND c.validation IN ('success', 'already_redeemed')
ORDER BY o.id ASC LIMIT 1`, now, now)
	var item RedeemOutboxItem
	var expiryOverridden int
	var expires, localVerified, communityVerified sql.NullTime
	if err := row.Scan(&item.ID, &item.SourceID, &item.SourceName, &item.BaseURL, &item.AttemptCount,
		&item.Code.ID, &item.Code.Fingerprint, &item.Code.Code, &item.Code.NormalizedCode, &item.Code.Channel,
		&expires, &item.Code.Validation, &item.Code.PropagationState, &localVerified, &communityVerified,
		&item.Code.OriginInstanceID, &item.Code.LastMessage, &item.Code.Revision, &item.Code.FirstSeenAt, &item.Code.UpdatedAt,
		&expiryOverridden); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	item.Code.ExpiresAt = nullTimePtr(expires)
	item.Code.LocalVerifiedAt = nullTimePtr(localVerified)
	item.Code.CommunityVerifiedAt = nullTimePtr(communityVerified)
	item.Code.ExpiryOverridden = expiryOverridden != 0
	res, err := tx.ExecContext(ctx, `UPDATE redeem_exchange_outbox SET status = 'sending', attempt_count = attempt_count + 1, updated_at = ? WHERE id = ? AND status = 'pending'`, now, item.ID)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return nil, nil
	}
	item.AttemptCount++
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &item, nil
}

func (d *DB) CompleteRedeemOutbox(ctx context.Context, id int64, sent bool, retryAt *time.Time, message string) error {
	status := "pending"
	if sent {
		status = "sent"
	}
	_, err := d.ExecContext(ctx, `
UPDATE redeem_exchange_outbox SET status = ?, next_attempt_at = ?, last_error = ?, updated_at = ? WHERE id = ?`,
		status, nullableTime(retryAt), strings.TrimSpace(message), time.Now().UTC(), id)
	return err
}

const redeemSourceSelect = `
WITH source_stats AS (
    SELECT o.source_id,
           COUNT(DISTINCT o.redeem_code_id) AS observed_count,
           COUNT(DISTINCT CASE WHEN c.validation IN ('success', 'already_redeemed', 'expired') THEN c.id END) AS trusted_count,
           COUNT(DISTINCT CASE WHEN c.validation = 'success' THEN c.id END) AS success_count,
           COUNT(DISTINCT CASE WHEN c.validation = 'already_redeemed' THEN c.id END) AS already_redeemed_count,
           COUNT(DISTINCT CASE WHEN c.validation = 'expired' THEN c.id END) AS expired_count,
           COUNT(DISTINCT CASE WHEN c.validation = 'invalid' THEN c.id END) AS invalid_count,
           COUNT(DISTINCT CASE WHEN c.validation IN ('pending', 'retryable', 'unknown') THEN c.id END) AS pending_count
    FROM redeem_code_observations o
    JOIN redeem_codes c ON c.id = o.redeem_code_id
    WHERE o.source_id IS NOT NULL
    GROUP BY o.source_id
)
SELECT s.id, s.name, s.type, s.base_url, s.channel, s.parser_config_json, s.enabled, s.push_enabled,
       s.poll_interval_seconds, s.remote_instance_id, s.cursor, s.last_sync_at, s.last_error,
       COALESCE(ss.observed_count, 0), COALESCE(ss.trusted_count, 0),
       COALESCE(ss.success_count, 0), COALESCE(ss.already_redeemed_count, 0),
       COALESCE(ss.expired_count, 0), COALESCE(ss.invalid_count, 0), COALESCE(ss.pending_count, 0),
       s.created_at, s.updated_at
FROM redeem_sources s
LEFT JOIN source_stats ss ON ss.source_id = s.id`

const redeemSourceScheduleSelect = `
SELECT s.id, s.name, s.type, s.base_url, s.channel, s.parser_config_json, s.enabled, s.push_enabled,
       s.poll_interval_seconds, s.remote_instance_id, s.cursor, s.last_sync_at, s.last_error,
       0, 0, 0, 0, 0, 0, 0,
       s.created_at, s.updated_at
FROM redeem_sources s
WHERE s.enabled = 1
ORDER BY s.id ASC`

func scanRedeemSource(scanner interface{ Scan(...any) error }) (*RedeemSource, error) {
	var item RedeemSource
	var enabled, pushEnabled int
	var lastSync sql.NullTime
	if err := scanner.Scan(&item.ID, &item.Name, &item.Type, &item.BaseURL, &item.Channel,
		&item.ParserConfigJSON, &enabled, &pushEnabled, &item.PollIntervalSeconds,
		&item.RemoteInstanceID, &item.Cursor, &lastSync, &item.LastError, &item.ObservedCount,
		&item.TrustedCount, &item.SuccessCount, &item.AlreadyRedeemedCount, &item.ExpiredCount,
		&item.InvalidCount, &item.PendingCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	item.Enabled = enabled != 0
	item.PushEnabled = pushEnabled != 0
	item.LastSyncAt = nullTimePtr(lastSync)
	return &item, nil
}

func getRedeemCodeByFingerprint(ctx context.Context, tx *sql.Tx, fingerprint string) (*RedeemCode, error) {
	return scanRedeemCode(tx.QueryRowContext(ctx, redeemCodeSelect+` WHERE fingerprint = ?`, fingerprint))
}

func getRedeemCodeByID(ctx context.Context, tx *sql.Tx, id int64) (*RedeemCode, error) {
	return scanRedeemCode(tx.QueryRowContext(ctx, redeemCodeSelect+` WHERE id = ?`, id))
}

const redeemCodeSelect = `
SELECT id, fingerprint, code, normalized_code, channel, expires_at, validation,
       propagation_state, local_verified_at, community_verified_at,
       origin_instance_id, last_message, revision, first_seen_at, updated_at,
       expiry_overridden
FROM redeem_codes`

func scanRedeemCode(scanner interface{ Scan(...any) error }) (*RedeemCode, error) {
	var item RedeemCode
	var expiryOverridden int
	var expires, localVerified, communityVerified sql.NullTime
	if err := scanner.Scan(&item.ID, &item.Fingerprint, &item.Code, &item.NormalizedCode,
		&item.Channel, &expires, &item.Validation, &item.PropagationState, &localVerified,
		&communityVerified, &item.OriginInstanceID, &item.LastMessage, &item.Revision,
		&item.FirstSeenAt, &item.UpdatedAt, &expiryOverridden); err != nil {
		return nil, err
	}
	item.ExpiresAt = nullTimePtr(expires)
	item.LocalVerifiedAt = nullTimePtr(localVerified)
	item.CommunityVerifiedAt = nullTimePtr(communityVerified)
	item.ExpiryOverridden = expiryOverridden != 0
	return &item, nil
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	copy := value.Time.UTC()
	return &copy
}

func ParseRedeemCursor(cursor string) (int64, error) {
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || value < 0 {
		return 0, errors.New("invalid redeem cursor")
	}
	return value, nil
}
