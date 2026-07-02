package store

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// ErrAccountExists is returned by CreateAccount when the name is taken.
var ErrAccountExists = errors.New("account already exists")

// ErrAccountNotFound is returned when a lookup by id or name has no match.
var ErrAccountNotFound = errors.New("account not found")

// ErrAccountAmbiguous is returned when a global name lookup matches multiple
// users. Callers should retry with an id or a user-scoped lookup.
var ErrAccountAmbiguous = errors.New("account name is ambiguous")

// scannable is satisfied by both *sql.Row and *sql.Rows.
type scannable interface {
	Scan(dest ...any) error
}

func scanAccount(s scannable) (*Account, error) {
	var (
		acc       Account
		lastLogin sql.NullTime
		createdAt time.Time
		updatedAt time.Time
	)
	if err := s.Scan(
		&acc.ID,
		&acc.UserID,
		&acc.Name,
		&acc.Channel,
		&acc.Username,
		&acc.AID,
		&acc.GsIdx,
		&acc.WSURL,
		&lastLogin,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}
	if lastLogin.Valid {
		t := lastLogin.Time
		acc.LastLoginAt = &t
	}
	acc.CreatedAt = createdAt
	acc.UpdatedAt = updatedAt
	return &acc, nil
}

// isUniqueErr returns true when err looks like a UNIQUE constraint violation
// from modernc/sqlite. The driver doesn't expose typed sqlite error codes
// across its package boundary, so we string-match here.
func isUniqueErr(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "constraint failed: UNIQUE")
}

func timeToProto(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}
