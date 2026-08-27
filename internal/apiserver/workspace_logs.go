package apiserver

import (
	"context"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultWorkspaceLogPageLimit = 200
	maxWorkspaceLogPageLimit     = 200
)

func (svc *Services) workspaceRecentLogsForAccount(ctx context.Context, acc *store.Account, afterID int64) (*pb.WorkspaceLogPage, int64, error) {
	if afterID <= 0 {
		return svc.workspaceLogPageForAccount(ctx, acc, 0, defaultWorkspaceLogPageLimit)
	}
	if afterID > 0 {
		oldest, newest, err := svc.DB.EventLogBounds(ctx, acc.ID)
		if err != nil {
			return nil, afterID, err
		}
		if newest == 0 || afterID < oldest || afterID > newest {
			page, highWater, pageErr := svc.workspaceLogPageForAccount(ctx, acc, 0, defaultWorkspaceLogPageLimit)
			if page != nil {
				page.GapDetected = true
			}
			return page, highWater, pageErr
		}
	}
	return svc.workspaceLogsAfterForAccount(ctx, acc, afterID, defaultWorkspaceLogPageLimit)
}

func (svc *Services) workspaceLogPage(ctx context.Context, accountID, beforeID int64, requestedLimit int32) (*pb.WorkspaceLogPage, error) {
	acc, err := svc.resolveAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	page, _, err := svc.workspaceLogPageForAccount(ctx, acc, beforeID, normalizeWorkspaceLogPageLimit(requestedLimit))
	return page, err
}

func (svc *Services) workspaceLogPageForAccount(ctx context.Context, acc *store.Account, beforeID int64, limit int) (*pb.WorkspaceLogPage, int64, error) {
	rows, err := svc.DB.ListEventLogs(ctx, store.ListEventLogsOptions{
		AccountIDs: []int64{acc.ID},
		BeforeID:   beforeID,
		Limit:      limit + 1,
	})
	if err != nil {
		return nil, 0, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	page := &pb.WorkspaceLogPage{AccountId: acc.ID, Kind: pb.WorkspaceLogPageKind_WORKSPACE_LOG_PAGE_KIND_RECENT, HasMoreBefore: hasMore}
	if beforeID > 0 {
		page.Kind = pb.WorkspaceLogPageKind_WORKSPACE_LOG_PAGE_KIND_BEFORE
	}
	highWater := int64(0)
	for _, row := range rows {
		page.Events = append(page.Events, eventLogToProto(row))
		if row.ID > highWater {
			highWater = row.ID
		}
	}
	if len(rows) > 0 {
		page.NextBeforeId = rows[len(rows)-1].ID
	}
	return page, highWater, nil
}

func (svc *Services) workspaceLogsAfterForAccount(ctx context.Context, acc *store.Account, afterID int64, limit int) (*pb.WorkspaceLogPage, int64, error) {
	rows, err := svc.DB.ListEventLogs(ctx, store.ListEventLogsOptions{AccountIDs: []int64{acc.ID}, AfterID: afterID, Limit: limit + 1})
	if err != nil {
		return nil, afterID, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	page := &pb.WorkspaceLogPage{AccountId: acc.ID, Kind: pb.WorkspaceLogPageKind_WORKSPACE_LOG_PAGE_KIND_AFTER, HasMoreAfter: hasMore}
	highWater := afterID
	for i := len(rows) - 1; i >= 0; i-- {
		page.Events = append(page.Events, eventLogToProto(rows[i]))
	}
	for _, row := range rows {
		if row.ID > highWater {
			highWater = row.ID
		}
	}
	return page, highWater, nil
}

func normalizeWorkspaceLogPageLimit(requested int32) int {
	limit := int(requested)
	if limit <= 0 {
		return defaultWorkspaceLogPageLimit
	}
	if limit > maxWorkspaceLogPageLimit {
		return maxWorkspaceLogPageLimit
	}
	return limit
}

func eventToProto(e runner.Event) *pb.Event {
	return e.ToProto()
}

func eventLogToProto(e store.EventLog) *pb.Event {
	return &pb.Event{
		Id:          e.ID,
		Ts:          timestamppb.New(e.TS),
		AccountId:   e.AccountID,
		AccountName: e.AccountName,
		Kind:        e.Kind,
		Message:     e.Message,
		PayloadJson: e.PayloadJSON,
		Category:    runner.WorkspaceLogCategory(e.Category, e.Domain),
		Domain:      e.Domain,
		Action:      e.Action,
		Label:       e.Label,
		Level:       e.Level,
	}
}
