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
	"time"
)

func TestOpenCreatesVersionedBaseline(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "garden.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	version, err := databaseVersion(ctx, db.DB)
	if err != nil {
		t.Fatal(err)
	}
	if version != currentSchemaVersion {
		t.Fatalf("schema version=%d, want %d", version, currentSchemaVersion)
	}
	var sessionColumn string
	if err := db.QueryRowContext(ctx, `SELECT name FROM pragma_table_info('sessions') WHERE name = 'payload_enc'`).Scan(&sessionColumn); err != nil {
		t.Fatalf("encrypted session column: %v", err)
	}
}

func TestOpenRejectsUnversionedDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "garden.db")
	legacy, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.ExecContext(ctx, `CREATE TABLE accounts (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = Open(ctx, path)
	if !errors.Is(err, ErrUnversionedDatabase) {
		t.Fatalf("Open() error=%v, want ErrUnversionedDatabase", err)
	}
	if _, statErr := os.Stat(path + ".key"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("rejected database unexpectedly created a key: %v", statErr)
	}
}

func TestOpenMigratesVersionThreeThroughRedeemSchema(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "garden.db")
	previous, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := previous.ExecContext(ctx, `CREATE TABLE accounts (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if _, err := previous.ExecContext(ctx, `PRAGMA user_version = 3`); err != nil {
		t.Fatal(err)
	}
	if err := previous.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if version, err := databaseVersion(ctx, db.DB); err != nil || version != 7 {
		t.Fatalf("schema version=%d err=%v, want 7", version, err)
	}
	var column string
	if err := db.QueryRowContext(ctx, `SELECT name FROM pragma_table_info('account_pearl_hire_usage') WHERE name = 'used_count'`).Scan(&column); err != nil {
		t.Fatalf("pearl hire usage migration: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT name FROM pragma_table_info('redeem_codes') WHERE name = 'fingerprint'`).Scan(&column); err != nil {
		t.Fatalf("redeem exchange migration: %v", err)
	}
	var retiredColumns int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('redeem_sources') WHERE name IN ('accepted_count', 'invalid_count')`).Scan(&retiredColumns); err != nil {
		t.Fatalf("inspect retired redeem source counters: %v", err)
	}
	if retiredColumns != 0 {
		t.Fatalf("retired redeem source counters=%d, want 0", retiredColumns)
	}
	if err := db.QueryRowContext(ctx, `SELECT name FROM pragma_table_info('redeem_codes') WHERE name = 'expiry_overridden'`).Scan(&column); err != nil {
		t.Fatalf("redeem expiry override migration: %v", err)
	}
}

func TestOpenMigratesVersionFiveRedeemSourcesWithoutLosingConfiguration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "garden.db")
	baseline, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := baseline.Close(); err != nil {
		t.Fatal(err)
	}

	previous, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`DROP INDEX idx_redeem_codes_browse`,
		`ALTER TABLE redeem_codes DROP COLUMN expiry_overridden`,
		`ALTER TABLE redeem_sources ADD COLUMN accepted_count INTEGER NOT NULL DEFAULT 0 CHECK(accepted_count >= 0)`,
		`ALTER TABLE redeem_sources ADD COLUMN invalid_count INTEGER NOT NULL DEFAULT 0 CHECK(invalid_count >= 0)`,
		`INSERT INTO redeem_sources(name, type, base_url, channel, accepted_count, invalid_count) VALUES ('source', 'custom_http', 'https://example.test/codes.json', 'ios', 7, 2)`,
		`PRAGMA user_version = 5`,
	} {
		if _, err := previous.ExecContext(ctx, statement); err != nil {
			_ = previous.Close()
			t.Fatal(err)
		}
	}
	if err := previous.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if version, err := databaseVersion(ctx, db.DB); err != nil || version != 7 {
		t.Fatalf("schema version=%d err=%v, want 7", version, err)
	}
	var name string
	if err := db.QueryRowContext(ctx, `SELECT name FROM redeem_sources WHERE name = 'source'`).Scan(&name); err != nil {
		t.Fatalf("preserved source: %v", err)
	}
	var retiredColumns int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('redeem_sources') WHERE name IN ('accepted_count', 'invalid_count')`).Scan(&retiredColumns); err != nil {
		t.Fatal(err)
	}
	if retiredColumns != 0 {
		t.Fatalf("retired redeem source counters=%d, want 0", retiredColumns)
	}
}

func TestOpenMigratesVersionSixRedeemCodesWithoutLosingData(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "garden.db")
	baseline, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := baseline.RedeemInstanceID(ctx); err != nil {
		_ = baseline.Close()
		t.Fatal(err)
	}
	expires := time.Now().UTC().Add(time.Hour)
	created, _, err := baseline.UpsertRedeemCode(ctx, RedeemCodeInput{
		Code: "PRESERVED", Channel: "ios", ExpiresAt: &expires, SourceKey: "test:migration",
	})
	if err != nil {
		_ = baseline.Close()
		t.Fatal(err)
	}
	if err := baseline.Close(); err != nil {
		t.Fatal(err)
	}

	previous, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`DROP INDEX idx_redeem_codes_browse`,
		`ALTER TABLE redeem_codes DROP COLUMN expiry_overridden`,
		`PRAGMA user_version = 6`,
	} {
		if _, err := previous.ExecContext(ctx, statement); err != nil {
			_ = previous.Close()
			t.Fatal(err)
		}
	}
	if err := previous.Close(); err != nil {
		t.Fatal(err)
	}

	migrated, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = migrated.Close() }()
	entries, _, err := migrated.ListRedeemCodes(ctx, 0, 10, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Fingerprint != created.Fingerprint {
		t.Fatalf("migrated redeem entries=%+v", entries)
	}
	entry := entries[0]
	if entry.Code != "PRESERVED" || entry.ExpiryOverridden || entry.ExpiresAt == nil || !entry.ExpiresAt.Equal(expires) {
		t.Fatalf("migrated redeem code=%+v", entry)
	}
}

func TestOpenRejectsNewerDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "garden.db")
	newer, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newer.ExecContext(ctx, `PRAGMA user_version = 999`); err != nil {
		t.Fatal(err)
	}
	if err := newer.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = Open(ctx, path)
	if !errors.Is(err, ErrNewerDatabase) {
		t.Fatalf("Open() error=%v, want ErrNewerDatabase", err)
	}
}

func TestSessionIsEncryptedAndExpires(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.CreateAccount(ctx, user.ID, "main", "ios", "game", "secret")
	if err != nil {
		t.Fatal(err)
	}

	payload := []byte(`{"token":"session-secret"}`)
	expiresAt := time.Now().Add(time.Hour)
	if err := db.SaveSession(ctx, account.ID, payload, &expiresAt); err != nil {
		t.Fatal(err)
	}
	var stored string
	if err := db.QueryRowContext(ctx, `SELECT payload_enc FROM sessions WHERE account_id = ?`, account.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "session-secret") || !strings.HasPrefix(stored, sessionVersionV1) {
		t.Fatalf("session was not encrypted: %q", stored)
	}
	loaded, err := db.LoadSession(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded, payload) {
		t.Fatalf("loaded session=%s, want %s", loaded, payload)
	}
	other, err := db.CreateAccount(ctx, user.ID, "other", "ios", "game-other", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO sessions(account_id, payload_enc, expires_at) VALUES (?, ?, ?)`, other.ID, stored, expiresAt); err != nil {
		t.Fatal(err)
	}
	if _, err := db.LoadSession(ctx, other.ID); err == nil {
		t.Fatal("session ciphertext copied to another account decrypted successfully")
	}

	expired := time.Now().Add(-time.Second)
	if err := db.SaveSession(ctx, account.ID, payload, &expired); err != nil {
		t.Fatal(err)
	}
	loaded, err = db.LoadSession(ctx, account.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Fatalf("expired session=%s, want nil", loaded)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE account_id = ?`, account.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expired session row count=%d, want 0", count)
	}
}

func TestAccountNamesAreScopedByUser(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	u1, err := db.CreateUser(ctx, "owner1", "owner1@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := db.CreateUser(ctx, "owner2", "owner2@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	a1, err := db.CreateAccount(ctx, u1.ID, "main", "ios", "game1", "pw1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateAccount(ctx, u2.ID, "main", "ios", "game2", "pw2"); err != nil {
		t.Fatalf("same account name for another user should be allowed: %v", err)
	}

	got, err := db.GetAccountByName(ctx, u1.ID, "main")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != a1.ID {
		t.Fatalf("GetAccountByName(user1, main) id=%d, want %d", got.ID, a1.ID)
	}
	if _, err := db.GetAccountByName(ctx, 0, "main"); err == nil {
		t.Fatal("GetAccountByName accepted a missing user id")
	}
}

func TestUniqueAccountNameAndRename(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	acc, err := db.CreateAccount(ctx, user.ID, "茉莉 · 第3区", "ios", "game1", "pw1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.CreateAccount(ctx, user.ID, "茉莉 · 第3区 #2", "ios", "game2", "pw2"); err != nil {
		t.Fatal(err)
	}

	name, err := db.UniqueAccountName(ctx, user.ID, 0, " 茉莉   ·   第3区 ")
	if err != nil {
		t.Fatal(err)
	}
	if name != "茉莉 · 第3区 #3" {
		t.Fatalf("unique name=%q, want 茉莉 · 第3区 #3", name)
	}

	same, err := db.UniqueAccountName(ctx, user.ID, acc.ID, "茉莉 · 第3区")
	if err != nil {
		t.Fatal(err)
	}
	if same != "茉莉 · 第3区" {
		t.Fatalf("same-account unique name=%q, want original", same)
	}

	renamed, err := db.RenameAccount(ctx, acc.ID, "  海棠   ·   第4区 ")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Name != "海棠 · 第4区" {
		t.Fatalf("renamed name=%q, want 海棠 · 第4区", renamed.Name)
	}
}

func TestAccountPasswordIsEncryptedAtRest(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	acc, err := db.CreateAccount(ctx, user.ID, "main", "ios", "game", "secret-password")
	if err != nil {
		t.Fatal(err)
	}

	var stored string
	if err := db.QueryRowContext(ctx, `SELECT password_enc FROM accounts WHERE id = ?`, acc.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, passwordVersionV1) {
		t.Fatalf("stored password prefix=%q, want %q", stored[:min(len(stored), 3)], passwordVersionV1)
	}
	if strings.Contains(stored, "secret-password") {
		t.Fatalf("stored password contains plaintext: %q", stored)
	}
	username, password, err := db.GetCredentials(ctx, acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if username != "game" || password != "secret-password" {
		t.Fatalf("credentials=(%q,%q), want game/secret-password", username, password)
	}

	if err := db.UpdateAccountCredentials(ctx, acc.ID, "game-refreshed", "refreshed-secret"); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT password_enc FROM accounts WHERE id = ?`, acc.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stored, passwordVersionV1) || strings.Contains(stored, "refreshed-secret") {
		t.Fatalf("updated password is not encrypted: %q", stored)
	}
	username, password, err = db.GetCredentials(ctx, acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if username != "game-refreshed" || password != "refreshed-secret" {
		t.Fatalf("updated credentials=(%q,%q), want game-refreshed/refreshed-secret", username, password)
	}
}

func TestEventLogPersistsAndFilters(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	acc1, err := db.CreateAccount(ctx, user.ID, "main", "ios", "game1", "pw1")
	if err != nil {
		t.Fatal(err)
	}
	acc2, err := db.CreateAccount(ctx, user.ID, "alt", "ios", "game2", "pw2")
	if err != nil {
		t.Fatal(err)
	}

	base := time.Unix(100, 0).UTC()
	id1, err := db.LogEvent(ctx, EventLog{AccountID: acc1.ID, AccountName: acc1.Name, TS: base, Kind: "session", Message: "connected"})
	if err != nil {
		t.Fatal(err)
	}
	id2, err := db.LogEvent(ctx, EventLog{AccountID: acc1.ID, AccountName: acc1.Name, TS: base.Add(time.Second), Kind: "operation_ack", Message: "done"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.LogEvent(ctx, EventLog{AccountID: acc2.ID, AccountName: acc2.Name, TS: base.Add(2 * time.Second), Kind: "operation_ack", Message: "other"}); err != nil {
		t.Fatal(err)
	}

	got, err := db.ListEventLogs(ctx, ListEventLogsOptions{AccountIDs: []int64{acc1.ID}, Kinds: []string{"operation_ack"}, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != id2 || got[0].Message != "done" {
		t.Fatalf("filtered events=%+v, want only account1 operation_ack", got)
	}

	got, err = db.ListEventLogs(ctx, ListEventLogsOptions{AfterID: id1, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != id2 {
		t.Fatalf("after-id events=%+v, want chronological ids after %d", got, id1)
	}
}
