package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrUserNotFound = errors.New("user not found")
var ErrUserExists = errors.New("user already exists")

type User struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	Role         string
	MaxAccounts  int
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (d *DB) CreateUser(ctx context.Context, username, email, passwordHash string) (*User, error) {
	now := time.Now().UTC()
	res, err := d.ExecContext(ctx,
		`INSERT INTO users(username, email, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		username, email, passwordHash, now, now,
	)
	if err != nil {
		if isUniqueErr(err) {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	id, _ := res.LastInsertId()
	return d.GetUserByID(ctx, id)
}

func (d *DB) GetUserByID(ctx context.Context, id int64) (*User, error) {
	row := d.QueryRowContext(ctx,
		`SELECT id, username, email, password_hash, role, max_accounts, status, created_at, updated_at
		 FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return u, err
}

func (d *DB) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row := d.QueryRowContext(ctx,
		`SELECT id, username, email, password_hash, role, max_accounts, status, created_at, updated_at
		 FROM users WHERE username = ? OR email = ?`, username, username)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return u, err
}

func (d *DB) ListUsers(ctx context.Context, offset, limit int) ([]*User, int, error) {
	var total int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 50
	}
	rows, err := d.QueryContext(ctx,
		`SELECT id, username, email, password_hash, role, max_accounts, status, created_at, updated_at
		 FROM users ORDER BY id ASC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()
	var out []*User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, u)
	}
	return out, total, rows.Err()
}

func (d *DB) UpdateUser(ctx context.Context, id int64, role *string, maxAccounts *int, status *string) (*User, error) {
	if role != nil {
		if _, err := d.ExecContext(ctx, `UPDATE users SET role = ?, updated_at = ? WHERE id = ?`, *role, time.Now().UTC(), id); err != nil {
			return nil, err
		}
	}
	if maxAccounts != nil {
		if _, err := d.ExecContext(ctx, `UPDATE users SET max_accounts = ?, updated_at = ? WHERE id = ?`, *maxAccounts, time.Now().UTC(), id); err != nil {
			return nil, err
		}
	}
	if status != nil {
		if _, err := d.ExecContext(ctx, `UPDATE users SET status = ?, updated_at = ? WHERE id = ?`, *status, time.Now().UTC(), id); err != nil {
			return nil, err
		}
	}
	return d.GetUserByID(ctx, id)
}

// UpdateUserPasswordHash replaces the stored bcrypt hash for a user.
func (d *DB) UpdateUserPasswordHash(ctx context.Context, id int64, passwordHash string) error {
	if strings.TrimSpace(passwordHash) == "" {
		return fmt.Errorf("password hash is empty")
	}
	_, err := d.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?`,
		passwordHash, time.Now().UTC(), id,
	)
	return err
}

func scanUser(s scannable) (*User, error) {
	var u User
	if err := s.Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash,
		&u.Role, &u.MaxAccounts, &u.Status,
		&u.CreatedAt, &u.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &u, nil
}
