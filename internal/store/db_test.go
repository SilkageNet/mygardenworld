package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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
	if _, err := db.GetAccountByName(ctx, 0, "main"); !errors.Is(err, ErrAccountAmbiguous) {
		t.Fatalf("global GetAccountByName(main) error=%v, want ErrAccountAmbiguous", err)
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
