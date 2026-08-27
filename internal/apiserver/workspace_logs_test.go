package apiserver

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func TestWorkspaceLogPagesAreLosslessAndNewestFirst(t *testing.T) {
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
	acc, err := db.CreateAccount(ctx, user.ID, "main", "ios", "game", "secret")
	if err != nil {
		t.Fatal(err)
	}
	for index := 1; index <= 450; index++ {
		if _, err := db.LogEvent(ctx, store.EventLog{AccountID: acc.ID, AccountName: acc.Name, TS: time.Unix(int64(index), 0), Kind: fmt.Sprintf("event_%d", index)}); err != nil {
			t.Fatal(err)
		}
	}

	svc := &Services{DB: db}
	recent, highWater, err := svc.workspaceLogPageForAccount(ctx, acc, 0, 200)
	if err != nil {
		t.Fatal(err)
	}
	assertWorkspaceLogPage(t, recent, pb.WorkspaceLogPageKind_WORKSPACE_LOG_PAGE_KIND_RECENT, 450, 251, true, false)
	if highWater != 450 || recent.GetNextBeforeId() != 251 {
		t.Fatalf("recent high_water=%d next_before=%d", highWater, recent.GetNextBeforeId())
	}

	older, _, err := svc.workspaceLogPageForAccount(ctx, acc, recent.GetNextBeforeId(), 200)
	if err != nil {
		t.Fatal(err)
	}
	assertWorkspaceLogPage(t, older, pb.WorkspaceLogPageKind_WORKSPACE_LOG_PAGE_KIND_BEFORE, 250, 51, true, false)

	catchup, catchupHighWater, err := svc.workspaceLogsAfterForAccount(ctx, acc, 50, 200)
	if err != nil {
		t.Fatal(err)
	}
	assertWorkspaceLogPage(t, catchup, pb.WorkspaceLogPageKind_WORKSPACE_LOG_PAGE_KIND_AFTER, 250, 51, false, true)
	if catchupHighWater != 250 {
		t.Fatalf("first catch-up high_water=%d, want 250", catchupHighWater)
	}
	final, finalHighWater, err := svc.workspaceLogsAfterForAccount(ctx, acc, catchupHighWater, 200)
	if err != nil {
		t.Fatal(err)
	}
	assertWorkspaceLogPage(t, final, pb.WorkspaceLogPageKind_WORKSPACE_LOG_PAGE_KIND_AFTER, 450, 251, false, false)
	if finalHighWater != 450 {
		t.Fatalf("final catch-up high_water=%d, want 450", finalHighWater)
	}
}

func assertWorkspaceLogPage(t *testing.T, page *pb.WorkspaceLogPage, kind pb.WorkspaceLogPageKind, first, last int64, moreBefore, moreAfter bool) {
	t.Helper()
	if page.GetKind() != kind || len(page.GetEvents()) != 200 || page.GetEvents()[0].GetId() != first || page.GetEvents()[199].GetId() != last ||
		page.GetHasMoreBefore() != moreBefore || page.GetHasMoreAfter() != moreAfter {
		t.Fatalf("page kind=%s len=%d bounds=%d..%d more_before=%t more_after=%t", page.GetKind(), len(page.GetEvents()), page.GetEvents()[0].GetId(), page.GetEvents()[len(page.GetEvents())-1].GetId(), page.GetHasMoreBefore(), page.GetHasMoreAfter())
	}
}
