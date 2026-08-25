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
var ErrLastActiveAdmin = errors.New("cannot remove the last active admin")

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
	return d.CreateUserWithOptions(ctx, username, email, passwordHash, "user", 5, "active")
}

// CreateUserWithOptions inserts the complete user record in one statement so
// API validation failures or database errors cannot leave a partially
// configured user behind.
func (d *DB) CreateUserWithOptions(ctx context.Context, username, email, passwordHash, role string, maxAccounts int, status string) (*User, error) {
	now := time.Now().UTC()
	res, err := d.ExecContext(ctx,
		`INSERT INTO users(username, email, password_hash, role, max_accounts, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		username, email, passwordHash, role, maxAccounts, status, now, now,
	)
	if err != nil {
		if isUniqueErr(err) {
			return nil, ErrUserExists
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read inserted user id: %w", err)
	}
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
	sets := make([]string, 0, 4)
	args := make([]any, 0, 5)
	if role != nil {
		sets = append(sets, "role = ?")
		args = append(args, *role)
	}
	if maxAccounts != nil {
		sets = append(sets, "max_accounts = ?")
		args = append(args, *maxAccounts)
	}
	if status != nil {
		sets = append(sets, "status = ?")
		args = append(args, *status)
	}
	if len(sets) > 0 {
		sets = append(sets, "updated_at = ?")
		args = append(args, time.Now().UTC(), id)
		query := `UPDATE users SET ` + strings.Join(sets, ", ") + ` WHERE id = ?`
		removesActiveAdmin := (role != nil && *role != "admin") || (status != nil && *status != "active")
		if removesActiveAdmin {
			query += ` AND NOT (
				role = 'admin' AND status = 'active'
				AND (SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active') <= 1
			)`
		}
		res, err := d.ExecContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		updated, err := res.RowsAffected()
		if err != nil {
			return nil, err
		}
		if updated == 0 {
			user, err := d.GetUserByID(ctx, id)
			if err != nil {
				return nil, err
			}
			if removesActiveAdmin && user.Role == "admin" && user.Status == "active" {
				return nil, ErrLastActiveAdmin
			}
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
