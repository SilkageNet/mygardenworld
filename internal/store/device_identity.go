package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
)

const pendingDeviceLimit = 64
const pendingDeviceRetention = 30 * 24 * time.Hour

var ErrDeviceIdentityQuota = errors.New("未完成添加的账号设备记录过多，请稍后再试")

// DeviceIdentity is a durable client identifier, not an authentication token or
// an automation policy. Software versions and server-issued launch/session
// identifiers deliberately remain outside its lifecycle.
type DeviceIdentity struct {
	DeviceID string
	Source   string
}

// Summary is safe for diagnostic logs; never log the full account identifier.
func (identity DeviceIdentity) Summary() string {
	digest := sha256.Sum256([]byte(identity.DeviceID))
	return fmt.Sprintf("%x", digest[:6])
}

// EnsureDeviceIdentity commits before any login request. Bound identities live
// until account deletion; unsuccessful additions retain their identity for 30
// days. Quota, cleanup, adoption and allocation share the same writer transaction.
func (d *DB) EnsureDeviceIdentity(ctx context.Context, userID int64, channel, username string) (DeviceIdentity, error) {
	var identity DeviceIdentity
	if userID <= 0 || strings.TrimSpace(username) == "" || (channel != "ios" && channel != "alipay") {
		return identity, errors.New("invalid device identity scope")
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return identity, err
	}
	defer func() { _ = tx.Rollback() }()
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT status='active' FROM users WHERE id=?`, userID).Scan(&active); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return identity, ErrUserInactive
		}
		return identity, err
	}
	if !active {
		return identity, ErrUserInactive
	}
	var accountID sql.NullInt64
	var deleting bool
	err = tx.QueryRowContext(ctx, `SELECT id, deletion_pending FROM accounts WHERE user_id=? AND channel=? AND username=?`, userID, channel, username).Scan(&accountID, &deleting)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return identity, err
	}
	if deleting {
		return identity, ErrAccountDeleting
	}
	now := time.Now().UnixMilli()
	// Keep the current attempted identity even at the retention boundary. Only
	// abandoned, unbound entries may be reclaimed; accounts never expire here.
	if _, err := tx.ExecContext(ctx, `DELETE FROM login_device_identities WHERE user_id=? AND account_id IS NULL AND touched_ms<? AND NOT(channel=? AND username=?)`, userID, now-pendingDeviceRetention.Milliseconds(), channel, username); err != nil {
		return identity, err
	}
	err = tx.QueryRowContext(ctx, `SELECT device_id, source FROM login_device_identities WHERE user_id=? AND channel=? AND username=?`, userID, channel, username).Scan(&identity.DeviceID, &identity.Source)
	if err == nil {
		if _, err := tx.ExecContext(ctx, `UPDATE login_device_identities SET account_id=?, touched_ms=? WHERE user_id=? AND channel=? AND username=?`, accountID, now, userID, channel, username); err != nil {
			return identity, err
		}
		return identity, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return identity, err
	}
	if !accountID.Valid {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM login_device_identities WHERE user_id=? AND account_id IS NULL`, userID).Scan(&count); err != nil {
			return identity, err
		}
		if count >= pendingDeviceLimit {
			return identity, ErrDeviceIdentityQuota
		}
	} else {
		// Inspect even expired sessions without LoadSession's expiry deletion.
		// This is a local encrypted cache read, never a game snapshot fetch.
		var encrypted string
		err := tx.QueryRowContext(ctx, `SELECT payload_enc FROM sessions WHERE account_id=?`, accountID.Int64).Scan(&encrypted)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return identity, err
		}
		if err == nil {
			if raw, err := d.decodeSession(accountID.Int64, encrypted); err == nil {
				cfg, _ := babigame.ConfigForChannel(babigame.Channel(channel))
				if session, err := babigame.UnmarshalSessionJSON(raw, cfg); err == nil && len(session.DeviceID) <= 256 {
					identity = DeviceIdentity{DeviceID: session.DeviceID, Source: "session"}
				}
			}
		}
	}
	if identity.DeviceID == "" {
		identity = DeviceIdentity{DeviceID: babigame.RandomDeviceID(), Source: "generated"}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO login_device_identities(user_id,channel,username,account_id,device_id,source,touched_ms) VALUES(?,?,?,?,?,?,?)`, userID, channel, username, accountID, identity.DeviceID, identity.Source, now); err != nil {
		return identity, err
	}
	return identity, tx.Commit()
}

// initializeDeviceIdentities runs before the daemon can delete an old session.
// Adoption is restartable: each identity commits once, without replacing any
// already initialized identity after a partially completed startup.
func (d *DB) initializeDeviceIdentities(ctx context.Context) error {
	rows, err := d.QueryContext(ctx, `SELECT a.user_id,a.channel,a.username FROM accounts a JOIN users u ON u.id=a.user_id
LEFT JOIN login_device_identities i ON i.user_id=a.user_id AND i.channel=a.channel AND i.username=a.username
WHERE a.deletion_pending=0 AND u.status='active' AND i.user_id IS NULL`)
	if err != nil {
		return err
	}
	type scope struct {
		userID            int64
		channel, username string
	}
	var scopes []scope
	for rows.Next() {
		var s scope
		if err := rows.Scan(&s.userID, &s.channel, &s.username); err != nil {
			_ = rows.Close()
			return err
		}
		scopes = append(scopes, s)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	for _, s := range scopes {
		if _, err := d.EnsureDeviceIdentity(ctx, s.userID, s.channel, s.username); err != nil {
			return err
		}
	}
	return nil
}
