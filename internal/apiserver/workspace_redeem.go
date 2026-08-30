package apiserver

import (
	"context"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultWorkspaceRedeemPageLimit = 20
	maxWorkspaceRedeemPageLimit     = 100
)

func (svc *Services) workspaceRedeemAttemptPage(
	ctx context.Context,
	accountID, beforeID int64,
	requestedLimit int32,
	requestedFilter pb.AccountRedeemAttemptFilter,
) (*pb.AccountRedeemAttemptPage, error) {
	acc, err := svc.resolveAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	filter, storeFilter := normalizeWorkspaceRedeemFilter(requestedFilter)
	limit := normalizeWorkspaceRedeemPageLimit(requestedLimit)
	records, summary, err := svc.DB.ListRedeemAttempts(ctx, store.ListRedeemAttemptsOptions{
		AccountID: acc.ID,
		BeforeID:  beforeID,
		Limit:     limit + 1,
		Filter:    storeFilter,
	})
	if err != nil {
		return nil, err
	}
	hasMore := len(records) > limit
	if hasMore {
		records = records[:limit]
	}
	page := &pb.AccountRedeemAttemptPage{
		AccountId:     acc.ID,
		Filter:        filter,
		HasMoreBefore: hasMore,
		Summary:       redeemAttemptSummaryToProto(summary),
		Entries:       make([]*pb.AccountRedeemAttempt, 0, len(records)),
		Replace:       beforeID <= 0,
	}
	for _, record := range records {
		page.Entries = append(page.Entries, redeemAttemptRecordToProto(record))
	}
	if len(records) > 0 {
		page.NextBeforeId = records[len(records)-1].ID
	}
	return page, nil
}

func normalizeWorkspaceRedeemPageLimit(requested int32) int {
	limit := int(requested)
	if limit <= 0 {
		return defaultWorkspaceRedeemPageLimit
	}
	if limit > maxWorkspaceRedeemPageLimit {
		return maxWorkspaceRedeemPageLimit
	}
	return limit
}

func normalizeWorkspaceRedeemFilter(requested pb.AccountRedeemAttemptFilter) (pb.AccountRedeemAttemptFilter, string) {
	switch requested {
	case pb.AccountRedeemAttemptFilter_ACCOUNT_REDEEM_ATTEMPT_FILTER_REDEEMED:
		return requested, store.RedeemAttemptFilterRedeemed
	case pb.AccountRedeemAttemptFilter_ACCOUNT_REDEEM_ATTEMPT_FILTER_UNAVAILABLE:
		return requested, store.RedeemAttemptFilterUnavailable
	case pb.AccountRedeemAttemptFilter_ACCOUNT_REDEEM_ATTEMPT_FILTER_ATTENTION:
		return requested, store.RedeemAttemptFilterAttention
	default:
		return pb.AccountRedeemAttemptFilter_ACCOUNT_REDEEM_ATTEMPT_FILTER_ALL, store.RedeemAttemptFilterAll
	}
}

func redeemAttemptRecordToProto(record store.RedeemAttemptRecord) *pb.AccountRedeemAttempt {
	return &pb.AccountRedeemAttempt{
		Id:           record.ID,
		Code:         record.Code,
		Channel:      redeemChannelToProto(record.Channel),
		Status:       redeemAttemptStatusToProto(record.Status),
		Message:      record.Message,
		AttemptCount: int32(record.AttemptCount),
		AttemptedAt:  redeemTimeToProto(record.AttemptedAt),
		ExpiresAt:    redeemTimeToProto(record.ExpiresAt),
		UpdatedAt:    timestamppb.New(record.UpdatedAt),
	}
}

func redeemAttemptSummaryToProto(summary store.RedeemAttemptSummary) *pb.AccountRedeemAttemptSummary {
	return &pb.AccountRedeemAttemptSummary{
		Total:           summary.Total,
		Success:         summary.Success,
		AlreadyRedeemed: summary.AlreadyRedeemed,
		Expired:         summary.Expired,
		Invalid:         summary.Invalid,
		Pending:         summary.Pending,
		Running:         summary.Running,
		Retryable:       summary.Retryable,
		Unknown:         summary.Unknown,
	}
}

func redeemAttemptStatusToProto(status string) pb.AccountRedeemAttemptStatus {
	switch status {
	case store.RedeemValidationPending:
		return pb.AccountRedeemAttemptStatus_ACCOUNT_REDEEM_ATTEMPT_STATUS_PENDING
	case store.RedeemAttemptStatusRunning:
		return pb.AccountRedeemAttemptStatus_ACCOUNT_REDEEM_ATTEMPT_STATUS_RUNNING
	case store.RedeemValidationSuccess:
		return pb.AccountRedeemAttemptStatus_ACCOUNT_REDEEM_ATTEMPT_STATUS_SUCCESS
	case store.RedeemValidationAlreadyRedeemed:
		return pb.AccountRedeemAttemptStatus_ACCOUNT_REDEEM_ATTEMPT_STATUS_ALREADY_REDEEMED
	case store.RedeemValidationExpired:
		return pb.AccountRedeemAttemptStatus_ACCOUNT_REDEEM_ATTEMPT_STATUS_EXPIRED
	case store.RedeemValidationInvalid:
		return pb.AccountRedeemAttemptStatus_ACCOUNT_REDEEM_ATTEMPT_STATUS_INVALID
	case store.RedeemValidationRetryable:
		return pb.AccountRedeemAttemptStatus_ACCOUNT_REDEEM_ATTEMPT_STATUS_RETRYABLE
	case store.RedeemValidationUnknown:
		return pb.AccountRedeemAttemptStatus_ACCOUNT_REDEEM_ATTEMPT_STATUS_UNKNOWN
	default:
		return pb.AccountRedeemAttemptStatus_ACCOUNT_REDEEM_ATTEMPT_STATUS_UNSPECIFIED
	}
}

func redeemChannelToProto(channel string) pb.Channel {
	switch channel {
	case "ios":
		return pb.Channel_CHANNEL_IOS
	case "alipay":
		return pb.Channel_CHANNEL_ALIPAY
	default:
		return pb.Channel_CHANNEL_UNSPECIFIED
	}
}

func redeemTimeToProto(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamppb.New(*value)
}
