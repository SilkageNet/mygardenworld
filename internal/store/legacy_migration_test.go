package store

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateLegacyV0ToV1BacksUpAndPreservesData(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "garden.db")
	legacy, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(ctx, legacyV0TestSchema); err != nil {
		t.Fatal(err)
	}
	key, err := loadOrCreateCredentialKey(path)
	if err != nil {
		t.Fatal(err)
	}
	legacyHandle := &DB{DB: legacy, credentialKey: key}
	password, err := legacyHandle.encodePassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(ctx, `INSERT INTO users(id, username, email, password_hash) VALUES (1, 'owner', 'owner@example.test', 'hash')`); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(ctx, `INSERT INTO accounts(id, user_id, name, channel, username, password_enc) VALUES (10, 1, 'main', 'ios', 'game', ?)`, password); err != nil {
		t.Fatal(err)
	}
	plainSession := []byte(`{"version":1,"token":"session-secret"}`)
	if _, err := legacy.ExecContext(ctx, `INSERT INTO sessions(account_id, payload_json) VALUES (10, ?)`, string(plainSession)); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(ctx, `INSERT INTO account_policies(account_id, policy_json) VALUES (10, '{"basic":{}}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(ctx, `INSERT INTO operation_log(account_id, kind) VALUES (10, 'farm.harvest')`); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(ctx, `INSERT INTO event_log(account_id, kind, message) VALUES (10, 'operation', 'kept')`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	backup := filepath.Join(dir, "garden.db.pre-v1.bak")
	result, err := MigrateLegacyV0ToV1(ctx, path, backup, func(data []byte) ([]byte, error) {
		return append([]byte(nil), data...), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Accounts != 1 || result.SessionsMigrated != 1 || result.SessionsDropped != 0 {
		t.Fatalf("migration result=%+v", result)
	}
	if result.BackupPath != backup || result.BackupKeyPath != backup+".key" {
		t.Fatalf("backup result=%+v", result)
	}

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	loaded, err := db.LoadSession(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded, plainSession) {
		t.Fatalf("migrated session=%s, want %s", loaded, plainSession)
	}
	username, decodedPassword, err := db.GetCredentials(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if username != "game" || decodedPassword != "secret" {
		t.Fatalf("credentials=(%q,%q)", username, decodedPassword)
	}
	for table := range map[string]struct{}{"account_policies": {}, "operation_log": {}, "event_log": {}} {
		var count int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("%s count=%d, want 1", table, count)
		}
	}
	var stored string
	if err := db.QueryRowContext(ctx, `SELECT payload_enc FROM sessions WHERE account_id = 10`).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, sessionVersionV1) || strings.Contains(stored, "session-secret") {
		t.Fatalf("session was not encrypted: %q", stored)
	}

	backupDB, err := sql.Open("sqlite", "file:"+backup+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = backupDB.Close() }()
	var backupVersion int
	if err := backupDB.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&backupVersion); err != nil {
		t.Fatal(err)
	}
	if backupVersion != 0 {
		t.Fatalf("backup schema version=%d, want 0", backupVersion)
	}
	var backupSession string
	if err := backupDB.QueryRowContext(ctx, `SELECT payload_json FROM sessions WHERE account_id = 10`).Scan(&backupSession); err != nil {
		t.Fatal(err)
	}
	if backupSession != string(plainSession) {
		t.Fatalf("backup session=%s", backupSession)
	}
	backupKey, err := os.ReadFile(backup + ".key")
	if err != nil {
		t.Fatal(err)
	}
	originalKey, err := os.ReadFile(path + ".key")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(backupKey, originalKey) {
		t.Fatal("backup credential key differs from source key")
	}
}

func TestMigrateLegacyV0ToV1RejectsUnsupportedChannelBeforeBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "garden.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, legacyV0TestSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO users(id, username, email, password_hash) VALUES (1, 'owner', 'owner@example.test', 'hash')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO accounts(id, user_id, name, channel, username, password_enc) VALUES (10, 1, 'old', 'android', 'game', 'v1:invalid')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := loadOrCreateCredentialKey(path); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(dir, "backup.db")
	_, err = MigrateLegacyV0ToV1(ctx, path, backup, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported account channel") {
		t.Fatalf("migration error=%v, want unsupported channel", err)
	}
	if _, statErr := os.Stat(backup); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("preflight failure created backup: %v", statErr)
	}
}

const legacyV0TestSchema = `
CREATE TABLE users (
 id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL UNIQUE, email TEXT NOT NULL UNIQUE,
 password_hash TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'user', max_accounts INTEGER NOT NULL DEFAULT 5,
 status TEXT NOT NULL DEFAULT 'active', created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE refresh_tokens (
 id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, token_hash TEXT NOT NULL UNIQUE,
 expires_at DATETIME NOT NULL, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE accounts (
 id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE, name TEXT NOT NULL,
 channel TEXT NOT NULL DEFAULT 'ios', username TEXT NOT NULL, password_enc TEXT NOT NULL,
 aid INTEGER DEFAULT 0, gs_idx INTEGER DEFAULT 0, ws_url TEXT DEFAULT '', last_login_at DATETIME,
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 UNIQUE(user_id, name)
);
CREATE INDEX idx_accounts_user ON accounts(user_id);
CREATE TABLE sessions (
 account_id INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE, payload_json TEXT NOT NULL, expires_at DATETIME,
 updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE account_policies (
 account_id INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE, policy_json TEXT NOT NULL, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE operation_log (
 id INTEGER PRIMARY KEY AUTOINCREMENT, account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE, ts DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 kind TEXT NOT NULL, args_json TEXT NOT NULL DEFAULT '{}', result_json TEXT NOT NULL DEFAULT '{}'
);
CREATE TABLE event_log (
 id INTEGER PRIMARY KEY AUTOINCREMENT, account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE, account_name TEXT NOT NULL DEFAULT '',
 ts DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, kind TEXT NOT NULL, message TEXT NOT NULL DEFAULT '',
 payload_json TEXT NOT NULL DEFAULT '{}', category TEXT NOT NULL DEFAULT '', domain TEXT NOT NULL DEFAULT '',
 action TEXT NOT NULL DEFAULT '', label TEXT NOT NULL DEFAULT '', level TEXT NOT NULL DEFAULT 'info'
);`
