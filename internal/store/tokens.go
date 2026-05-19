package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

var ErrTokenInvalid = errors.New("refresh token invalid or expired")

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (d *DB) SaveRefreshToken(ctx context.Context, userID int64, token string, expiresAt time.Time) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO refresh_tokens(user_id, token_hash, expires_at) VALUES (?, ?, ?)`,
		userID, hashToken(token), expiresAt.UTC(),
	)
	return err
}

func (d *DB) ValidateRefreshToken(ctx context.Context, token string) (int64, error) {
	var userID int64
	var expiresAt time.Time
	err := d.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM refresh_tokens WHERE token_hash = ?`,
		hashToken(token),
	).Scan(&userID, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrTokenInvalid
	}
	if err != nil {
		return 0, err
	}
	if time.Now().After(expiresAt) {
		_ = d.revokeTokenHash(ctx, hashToken(token))
		return 0, ErrTokenInvalid
	}
	return userID, nil
}

func (d *DB) RevokeRefreshToken(ctx context.Context, token string) error {
	return d.revokeTokenHash(ctx, hashToken(token))
}

func (d *DB) RevokeAllRefreshTokens(ctx context.Context, userID int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE user_id = ?`, userID)
	return err
}

func (d *DB) revokeTokenHash(ctx context.Context, hash string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token_hash = ?`, hash)
	return err
}

func (d *DB) CleanExpiredTokens(ctx context.Context) error {
	_, err := d.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE expires_at < ?`, time.Now().UTC())
	return err
}
