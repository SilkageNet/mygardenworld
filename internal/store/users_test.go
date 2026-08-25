package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestUpdateUserProtectsLastActiveAdmin(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	admin, err := db.CreateUserWithOptions(ctx, "admin", "admin@example.test", "hash", "admin", 5, "active")
	if err != nil {
		t.Fatal(err)
	}
	userRole := "user"
	if _, err := db.UpdateUser(ctx, admin.ID, &userRole, nil, nil); !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("demote only admin error = %v, want ErrLastActiveAdmin", err)
	}

	second, err := db.CreateUserWithOptions(ctx, "admin2", "admin2@example.test", "hash", "admin", 5, "active")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.UpdateUser(ctx, admin.ID, &userRole, nil, nil); err != nil {
		t.Fatalf("demote with another active admin: %v", err)
	}
	if _, err := db.UpdateUser(ctx, second.ID, &userRole, nil, nil); !errors.Is(err, ErrLastActiveAdmin) {
		t.Fatalf("demote new last admin error = %v, want ErrLastActiveAdmin", err)
	}
}
