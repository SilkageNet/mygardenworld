package apiserver

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestWorkspaceHistorySummaryAndPageShareOneCursorContract(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	user, err := db.CreateUser(ctx, "owner", "owner@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.CreateAccount(ctx, user.ID, "garden", "ios", "login", "secret")
	if err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= workspaceRecentHistoryLimit+2; index++ {
		if err := db.LogOperation(ctx, account.ID, fmt.Sprintf("usrLand.step%d", index), nil, map[string]any{"ok": true}); err != nil {
			t.Fatal(err)
		}
	}

	svc := &Services{DB: db}
	summary, err := svc.workspaceHistorySummary(ctx, account.ID, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(summary.GetRecentOperations()) != workspaceRecentHistoryLimit || !summary.GetHasMore() || summary.GetNextBeforeId() <= 0 {
		t.Fatalf("summary=%+v, want full page with continuation", summary)
	}

	userCtx := auth.ContextWithIdentity(ctx, &auth.Identity{UserID: user.ID, Role: user.Role})
	page, err := svc.workspaceHistoryPage(userCtx, account.ID, summary.GetNextBeforeId(), workspaceHistoryPageLimit)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.GetItems()) != 2 || page.GetHasMore() {
		t.Fatalf("continuation=%+v, want two terminal rows", page)
	}
	lastSummaryID := summary.GetRecentOperations()[len(summary.GetRecentOperations())-1].GetId()
	if page.GetItems()[0].GetId() >= lastSummaryID {
		t.Fatalf("continuation overlaps summary: %+v", page.GetItems())
	}
}

func TestOperationHistoryMarksPersistedErrorsAsFailures(t *testing.T) {
	if outcome, detail := operationOutcome(`{"error":"insufficient gold"}`); outcome != "failed" || detail != "insufficient gold" {
		t.Fatalf("outcome=%q detail=%q", outcome, detail)
	}
	if outcome, detail := operationOutcome(`{"ok":true}`); outcome != "success" || detail != "" {
		t.Fatalf("outcome=%q detail=%q", outcome, detail)
	}
}
