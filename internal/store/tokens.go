package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
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

// RotateRefreshToken atomically consumes one refresh token and stores its
// replacement. If any part of the rotation fails, the old token remains
// valid; concurrent replays of the same token can only succeed once.
func (d *DB) RotateRefreshToken(ctx context.Context, oldToken, newToken string, expiresAt time.Time) error {
	if oldToken == "" || newToken == "" || oldToken == newToken {
		return ErrTokenInvalid
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin refresh token rotation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	oldHash := hashToken(oldToken)
	var userID int64
	var storedExpiry time.Time
	if err := tx.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM refresh_tokens WHERE token_hash = ?`,
		oldHash,
	).Scan(&userID, &storedExpiry); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTokenInvalid
		}
		return fmt.Errorf("load refresh token for rotation: %w", err)
	}
	if !time.Now().Before(storedExpiry) {
		if _, err := tx.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token_hash = ?`, oldHash); err != nil {
			return fmt.Errorf("delete expired refresh token: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit expired refresh token cleanup: %w", err)
		}
		return ErrTokenInvalid
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token_hash = ?`, oldHash)
	if err != nil {
		return fmt.Errorf("consume refresh token: %w", err)
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count consumed refresh tokens: %w", err)
	}
	if deleted != 1 {
		return ErrTokenInvalid
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO refresh_tokens(user_id, token_hash, expires_at) VALUES (?, ?, ?)`,
		userID, hashToken(newToken), expiresAt.UTC(),
	); err != nil {
		return fmt.Errorf("store rotated refresh token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit refresh token rotation: %w", err)
	}
	return nil
}

func (d *DB) revokeTokenHash(ctx context.Context, hash string) error {
	_, err := d.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token_hash = ?`, hash)
	return err
}

func (d *DB) CleanExpiredTokens(ctx context.Context) error {
	_, err := d.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE expires_at < ?`, time.Now().UTC())
	return err
}
