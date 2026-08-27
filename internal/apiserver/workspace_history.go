package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	workspaceRecentHistoryLimit = 20
	workspaceHistoryPageLimit   = 50
	workspaceHistoryPageMax     = 200
)

func (svc *Services) workspaceHistorySummary(
	ctx context.Context,
	accountID int64,
	runtime *pb.RuntimeStatisticsView,
	business *pb.BusinessStatisticsView,
) (*pb.WorkspaceHistorySummary, error) {
	rows, err := svc.DB.ListOperationHistory(ctx, store.ListOperationHistoryOptions{
		AccountID: accountID,
		Limit:     workspaceRecentHistoryLimit + 1,
	})
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > workspaceRecentHistoryLimit
	if hasMore {
		rows = rows[:workspaceRecentHistoryLimit]
	}
	summary := &pb.WorkspaceHistorySummary{
		RuntimeStatistics:  runtime,
		BusinessStatistics: business,
		RecentOperations:   operationHistoryProto(rows),
		HasMore:            hasMore,
	}
	if len(rows) > 0 {
		summary.NextBeforeId = rows[len(rows)-1].ID
	}
	return summary, nil
}

func (svc *Services) workspaceHistoryPage(ctx context.Context, accountID, beforeID int64, requestedLimit int32) (*pb.WorkspaceHistoryPage, error) {
	acc, err := svc.resolveAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	limit := int(requestedLimit)
	if limit <= 0 {
		limit = workspaceHistoryPageLimit
	}
	if limit > workspaceHistoryPageMax {
		limit = workspaceHistoryPageMax
	}
	rows, err := svc.DB.ListOperationHistory(ctx, store.ListOperationHistoryOptions{
		AccountID: acc.ID,
		BeforeID:  beforeID,
		Limit:     limit + 1,
	})
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	page := &pb.WorkspaceHistoryPage{
		AccountId: acc.ID,
		Items:     operationHistoryProto(rows),
		HasMore:   hasMore,
	}
	if len(rows) > 0 {
		page.NextBeforeId = rows[len(rows)-1].ID
	}
	return page, nil
}

func operationHistoryProto(rows []store.OperationHistory) []*pb.WorkspaceHistoryItem {
	out := make([]*pb.WorkspaceHistoryItem, 0, len(rows))
	for _, row := range rows {
		domain, action := splitOperationKind(row.Kind)
		outcome, detail := operationOutcome(row.ResultJSON)
		message := fmt.Sprintf("%s %s", row.Kind, outcomeLabel(outcome))
		if detail != "" {
			message += ": " + detail
		}
		out = append(out, &pb.WorkspaceHistoryItem{
			Id:       row.ID,
			Ts:       timestamppb.New(row.TS),
			Category: operationCategory(domain),
			Domain:   domain,
			Action:   action,
			Label:    row.Kind,
			Outcome:  outcome,
			Message:  message,
		})
	}
	return out
}

func splitOperationKind(kind string) (string, string) {
	domain, action, found := strings.Cut(kind, ".")
	if !found {
		return kind, kind
	}
	return domain, action
}

func operationOutcome(resultJSON string) (string, string) {
	var result map[string]any
	if json.Unmarshal([]byte(resultJSON), &result) != nil {
		return "success", ""
	}
	raw, ok := result["error"]
	if !ok {
		return "success", ""
	}
	detail := strings.TrimSpace(fmt.Sprint(raw))
	if detail == "" || detail == "<nil>" {
		return "success", ""
	}
	return "failed", detail
}

func outcomeLabel(outcome string) string {
	if outcome == "failed" {
		return "失败"
	}
	return "完成"
}

func operationCategory(domain string) string {
	switch {
	case strings.HasPrefix(domain, "fml"):
		return "union"
	case strings.Contains(domain, "Land"), strings.Contains(domain, "flower"), strings.Contains(domain, "cultivate"):
		return "plant"
	case strings.Contains(domain, "Order"), strings.Contains(domain, "order"), strings.Contains(domain, "vase"):
		return "order"
	case strings.Contains(domain, "act"), strings.Contains(domain, "cyclic"), strings.Contains(domain, "dessert"):
		return "activity"
	default:
		return "basic"
	}
}
