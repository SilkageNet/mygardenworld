package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"modernc.org/sqlite"
)

// LegacySessionUpgrade converts a v0 plaintext session into the current
// payload format. Returning nil without an error deliberately drops a stale or
// invalid session while preserving the account and its fresh-login grant.
type LegacySessionUpgrade func([]byte) ([]byte, error)

// LegacyV0Inspection is the non-sensitive preflight summary for the one-time
// unversioned-database migration.
type LegacyV0Inspection struct {
	Accounts int
	Sessions int
}

// LegacyV0MigrationResult describes what the one-time migration preserved.
type LegacyV0MigrationResult struct {
	BackupPath       string
	BackupKeyPath    string
	Accounts         int
	SessionsMigrated int
	SessionsDropped  int
}

type legacySession struct {
	accountID int64
	payload   []byte
	expiresAt any
	updatedAt any
}

// InspectLegacyV0 verifies that path has the exact historical shape supported
// by MigrateLegacyV0ToV1. It never creates a credential key or modifies SQLite.
func InspectLegacyV0(ctx context.Context, path string) (LegacyV0Inspection, error) {
	db, err := openLegacySQLite(ctx, path, true)
	if err != nil {
		return LegacyV0Inspection{}, err
	}
	defer func() { _ = db.Close() }()
	return inspectLegacyV0(ctx, db, path)
}

// MigrateLegacyV0ToV1 is the explicit, one-time bridge from the final
// unversioned schema to the clean v1 baseline. Normal Open deliberately does
// not call this function or carry any v0 compatibility path.
//
// The caller must stop gardend before invoking it. A consistent SQLite backup
// and matching credential-key backup are completed before the source database
// is changed.
func MigrateLegacyV0ToV1(ctx context.Context, path, backupPath string, upgrade LegacySessionUpgrade) (LegacyV0MigrationResult, error) {
	if strings.TrimSpace(path) == "" {
		return LegacyV0MigrationResult{}, errors.New("database path is empty")
	}
	if strings.TrimSpace(backupPath) == "" {
		return LegacyV0MigrationResult{}, errors.New("backup path is empty")
	}
	dbAbs, err := filepath.Abs(path)
	if err != nil {
		return LegacyV0MigrationResult{}, fmt.Errorf("resolve database path: %w", err)
	}
	backupAbs, err := filepath.Abs(backupPath)
	if err != nil {
		return LegacyV0MigrationResult{}, fmt.Errorf("resolve backup path: %w", err)
	}
	if sameFilesystemPath(dbAbs, backupAbs) {
		return LegacyV0MigrationResult{}, errors.New("backup path must differ from database path")
	}
	backupKeyPath := backupAbs + ".key"
	for _, candidate := range []string{backupAbs, backupKeyPath} {
		if _, statErr := os.Stat(candidate); statErr == nil {
			return LegacyV0MigrationResult{}, fmt.Errorf("backup target already exists: %s", candidate)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return LegacyV0MigrationResult{}, fmt.Errorf("inspect backup target %s: %w", candidate, statErr)
		}
	}

	db, err := openLegacySQLite(ctx, dbAbs, false)
	if err != nil {
		return LegacyV0MigrationResult{}, err
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	inspection, err := inspectLegacyV0(ctx, db, dbAbs)
	if err != nil {
		return LegacyV0MigrationResult{}, err
	}
	key, err := readCredentialKey(dbAbs + ".key")
	if err != nil {
		return LegacyV0MigrationResult{}, fmt.Errorf("read existing credential key: %w", err)
	}
	legacyDB := &DB{DB: db, credentialKey: key}
	if err := verifyLegacyPasswords(ctx, legacyDB); err != nil {
		return LegacyV0MigrationResult{}, err
	}
	sessions, dropped, err := prepareLegacySessions(ctx, db, upgrade)
	if err != nil {
		return LegacyV0MigrationResult{}, err
	}

	if err := backupSQLite(ctx, db, backupAbs); err != nil {
		return LegacyV0MigrationResult{}, fmt.Errorf("backup database: %w", err)
	}
	if err := copyFileExclusive(dbAbs+".key", backupKeyPath, 0o600); err != nil {
		_ = os.Remove(backupAbs)
		_ = os.Remove(backupKeyPath)
		return LegacyV0MigrationResult{}, fmt.Errorf("backup credential key: %w", err)
	}

	if err := applyLegacyV0Migration(ctx, legacyDB, sessions); err != nil {
		return LegacyV0MigrationResult{}, fmt.Errorf("migrate database (backup retained at %s): %w", backupAbs, err)
	}

	return LegacyV0MigrationResult{
		BackupPath:       backupAbs,
		BackupKeyPath:    backupKeyPath,
		Accounts:         inspection.Accounts,
		SessionsMigrated: len(sessions),
		SessionsDropped:  dropped,
	}, nil
}

func openLegacySQLite(ctx context.Context, path string, readOnly bool) (*sql.DB, error) {
	if info, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("stat database: %w", err)
	} else if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("database is not a regular file: %s", path)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(1000)&_pragma=foreign_keys(0)", path)
	if readOnly {
		dsn += "&mode=ro"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open legacy sqlite: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping legacy sqlite: %w", err)
	}
	return db, nil
}

func inspectLegacyV0(ctx context.Context, db *sql.DB, path string) (LegacyV0Inspection, error) {
	version, err := databaseVersion(ctx, db)
	if err != nil {
		return LegacyV0Inspection{}, err
	}
	if version != 0 {
		return LegacyV0Inspection{}, fmt.Errorf("database %s is schema v%d, want unversioned v0", path, version)
	}
	required := map[string][]string{
		"users":            {"id", "username", "email", "password_hash"},
		"refresh_tokens":   {"id", "user_id", "token_hash"},
		"accounts":         {"id", "user_id", "name", "channel", "username", "password_enc", "aid", "gs_idx", "ws_url"},
		"sessions":         {"account_id", "payload_json", "expires_at", "updated_at"},
		"account_policies": {"account_id", "policy_json", "updated_at"},
		"operation_log":    {"id", "account_id", "ts", "kind", "args_json", "result_json"},
		"event_log":        {"id", "account_id", "ts", "kind", "message", "payload_json", "category", "domain", "action", "label", "level"},
	}
	for table, columns := range required {
		actual, err := tableColumns(ctx, db, table)
		if err != nil {
			return LegacyV0Inspection{}, err
		}
		for _, column := range columns {
			if !actual[column] {
				return LegacyV0Inspection{}, fmt.Errorf("unsupported legacy schema: %s.%s is missing", table, column)
			}
		}
	}

	var invalidChannel string
	if err := db.QueryRowContext(ctx, `SELECT channel FROM accounts WHERE channel NOT IN ('ios', 'alipay') LIMIT 1`).Scan(&invalidChannel); err == nil {
		return LegacyV0Inspection{}, fmt.Errorf("unsupported account channel %q remains in legacy database", invalidChannel)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return LegacyV0Inspection{}, fmt.Errorf("validate account channels: %w", err)
	}
	var duplicate string
	err = db.QueryRowContext(ctx, `
SELECT printf('user_id=%d channel=%s username=%s', user_id, channel, username)
FROM accounts GROUP BY user_id, channel, username HAVING COUNT(*) > 1 LIMIT 1`).Scan(&duplicate)
	if err == nil {
		return LegacyV0Inspection{}, fmt.Errorf("duplicate channel identity prevents v1 migration: %s", duplicate)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return LegacyV0Inspection{}, fmt.Errorf("validate account identities: %w", err)
	}

	var out LegacyV0Inspection
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&out.Accounts); err != nil {
		return LegacyV0Inspection{}, fmt.Errorf("count accounts: %w", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&out.Sessions); err != nil {
		return LegacyV0Inspection{}, fmt.Errorf("count sessions: %w", err)
	}
	return out, nil
}

func tableColumns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, fmt.Errorf("inspect table %s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("read table %s columns: %w", table, err)
		}
		out[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate table %s columns: %w", table, err)
	}
	return out, nil
}

func verifyLegacyPasswords(ctx context.Context, db *DB) error {
	rows, err := db.QueryContext(ctx, `SELECT id, password_enc FROM accounts ORDER BY id`)
	if err != nil {
		return fmt.Errorf("read legacy credentials: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var encrypted string
		if err := rows.Scan(&id, &encrypted); err != nil {
			return fmt.Errorf("read legacy credential: %w", err)
		}
		if _, err := db.decodePassword(encrypted); err != nil {
			return fmt.Errorf("account %d credential does not match garden.db.key: %w", id, err)
		}
	}
	return rows.Err()
}

func prepareLegacySessions(ctx context.Context, db *sql.DB, upgrade LegacySessionUpgrade) ([]legacySession, int, error) {
	rows, err := db.QueryContext(ctx, `SELECT account_id, payload_json, expires_at, updated_at FROM sessions ORDER BY account_id`)
	if err != nil {
		return nil, 0, fmt.Errorf("read legacy sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []legacySession
	dropped := 0
	for rows.Next() {
		var row legacySession
		var payload string
		if err := rows.Scan(&row.accountID, &payload, &row.expiresAt, &row.updatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan legacy session: %w", err)
		}
		if upgrade == nil {
			dropped++
			continue
		}
		upgraded, err := upgrade([]byte(payload))
		if err != nil {
			return nil, 0, fmt.Errorf("upgrade account %d session: %w", row.accountID, err)
		}
		if len(upgraded) == 0 {
			dropped++
			continue
		}
		row.payload = upgraded
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate legacy sessions: %w", err)
	}
	return out, dropped, nil
}

func backupSQLite(ctx context.Context, db *sql.DB, destination string) error {
	reserved, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := reserved.Close(); err != nil {
		_ = os.Remove(destination)
		return err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(destination)
		}
	}()

	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	err = conn.Raw(func(driverConn any) error {
		backuper, supported := driverConn.(interface {
			NewBackup(string) (*sqlite.Backup, error)
		})
		if !supported {
			return errors.New("sqlite driver does not support online backup")
		}
		backup, err := backuper.NewBackup(destination)
		if err != nil {
			return err
		}
		for {
			if err := ctx.Err(); err != nil {
				_ = backup.Finish()
				return err
			}
			more, err := backup.Step(2048)
			if err != nil {
				_ = backup.Finish()
				return err
			}
			if !more {
				break
			}
		}
		destinationConn, err := backup.Commit()
		if err != nil {
			return err
		}
		return destinationConn.Close()
	})
	if err != nil {
		return err
	}
	if err := os.Chmod(destination, 0o600); err != nil {
		return err
	}
	ok = true
	return nil
}

func copyFileExclusive(source, destination string, mode os.FileMode) error {
	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	dst, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		_ = dst.Close()
		if !ok {
			_ = os.Remove(destination)
		}
	}()
	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	if err := dst.Sync(); err != nil {
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}

func applyLegacyV0Migration(ctx context.Context, db *DB, sessions []legacySession) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	rollback := func(cause error) error {
		_ = tx.Rollback()
		return cause
	}
	if _, err := tx.ExecContext(ctx, legacyAccountsV1SQL); err != nil {
		return rollback(fmt.Errorf("create v1 accounts table: %w", err))
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO accounts_v1(id, user_id, name, channel, username, password_enc, aid, gs_idx, ws_url, last_login_at, created_at, updated_at)
SELECT id, user_id, name, channel, username, password_enc, COALESCE(aid, 0), COALESCE(gs_idx, 0), COALESCE(ws_url, ''), last_login_at, created_at, updated_at
FROM accounts`); err != nil {
		return rollback(fmt.Errorf("copy accounts: %w", err))
	}
	if _, err := tx.ExecContext(ctx, legacySessionsV1SQL); err != nil {
		return rollback(fmt.Errorf("create v1 sessions table: %w", err))
	}
	for _, session := range sessions {
		encoded, err := db.encodeSession(session.accountID, session.payload)
		if err != nil {
			return rollback(fmt.Errorf("encrypt account %d session: %w", session.accountID, err))
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO sessions_v1(account_id, payload_enc, expires_at, updated_at) VALUES (?, ?, ?, ?)`,
			session.accountID, encoded, session.expiresAt, session.updatedAt); err != nil {
			return rollback(fmt.Errorf("insert account %d session: %w", session.accountID, err))
		}
	}
	for _, statement := range []string{
		`DROP TABLE sessions`,
		`DROP TABLE accounts`,
		`ALTER TABLE accounts_v1 RENAME TO accounts`,
		`ALTER TABLE sessions_v1 RENAME TO sessions`,
		`CREATE INDEX idx_accounts_user ON accounts(user_id)`,
		fmt.Sprintf("PRAGMA user_version = %d", currentSchemaVersion),
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return rollback(fmt.Errorf("apply %q: %w", statement, err))
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	version, err := databaseVersion(ctx, db.DB)
	if err != nil {
		return err
	}
	if version != currentSchemaVersion {
		return fmt.Errorf("schema version=%d, want %d", version, currentSchemaVersion)
	}
	rows, err := db.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("run foreign-key check: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if rows.Next() {
		return errors.New("foreign-key check failed after migration")
	}
	return rows.Err()
}

const legacyAccountsV1SQL = `
CREATE TABLE accounts_v1 (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT    NOT NULL,
    channel         TEXT    NOT NULL,
    username        TEXT    NOT NULL,
    password_enc    TEXT    NOT NULL,
    aid             INTEGER NOT NULL DEFAULT 0,
    gs_idx          INTEGER NOT NULL DEFAULT 0,
    ws_url          TEXT    NOT NULL DEFAULT '',
    last_login_at   DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, name),
    UNIQUE(user_id, channel, username),
    CHECK(channel IN ('ios', 'alipay'))
);`

const legacySessionsV1SQL = `
CREATE TABLE sessions_v1 (
    account_id  INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    payload_enc TEXT NOT NULL,
    expires_at  DATETIME,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

func sameFilesystemPath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
