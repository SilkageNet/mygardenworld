package store

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestListOperationHistoryIsAccountScopedAndPaginated(t *testing.T) {
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
	first, err := db.CreateAccount(ctx, user.ID, "first", "ios", "first", "secret")
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.CreateAccount(ctx, user.ID, "second", "ios", "second", "secret")
	if err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 3; index++ {
		if err := db.LogOperation(ctx, first.ID, fmt.Sprintf("farm.step%d", index), nil, map[string]any{"ok": true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.LogOperation(ctx, second.ID, "other.step", nil, nil); err != nil {
		t.Fatal(err)
	}

	page, err := db.ListOperationHistory(ctx, ListOperationHistoryOptions{AccountID: first.ID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Kind != "farm.step3" || page[1].Kind != "farm.step2" {
		t.Fatalf("first page=%+v", page)
	}
	next, err := db.ListOperationHistory(ctx, ListOperationHistoryOptions{AccountID: first.ID, BeforeID: page[1].ID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || next[0].Kind != "farm.step1" {
		t.Fatalf("second page=%+v", next)
	}
}
