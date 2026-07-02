package store

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
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
}
