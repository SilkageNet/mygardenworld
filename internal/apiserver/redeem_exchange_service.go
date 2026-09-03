package apiserver

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	redeemsvc "github.com/SilkageNet/mygardenworld/internal/redeem"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	redeemSubmitWindow    = 10 * time.Minute
	redeemSubmitPerSource = 120
	redeemSubmitGlobal    = 1000
)

type RedeemSubmitLimiter struct {
	mu          sync.Mutex
	windowStart time.Time
	global      int
	bySource    map[string]int
}

func NewRedeemSubmitLimiter() *RedeemSubmitLimiter {
	return &RedeemSubmitLimiter{bySource: make(map[string]int)}
}

func (l *RedeemSubmitLimiter) Allow(source string, count int) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if l.windowStart.IsZero() || now.Sub(l.windowStart) >= redeemSubmitWindow {
		l.windowStart = now
		l.global = 0
		l.bySource = make(map[string]int)
	}
	if count <= 0 || l.global+count > redeemSubmitGlobal || l.bySource[source]+count > redeemSubmitPerSource {
		return false
	}
	l.global += count
	l.bySource[source] += count
	return true
}

func (svc *Services) GetExchangeInfo(_ context.Context, _ *connect.Request[pb.GetExchangeInfoRequest]) (*connect.Response[pb.GetExchangeInfoResponse], error) {
	if svc.Redeem == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("redeem exchange unavailable"))
	}
	return connect.NewResponse(&pb.GetExchangeInfoResponse{
		InstanceId:   svc.Redeem.InstanceID(),
		ServerTime:   timestamppb.Now(),
		MaxBatchSize: redeemsvc.MaxSubmitBatch,
		Channels:     []pb.Channel{pb.Channel_CHANNEL_IOS, pb.Channel_CHANNEL_ALIPAY},
	}), nil
}

func (svc *Services) ListRedeemCodes(ctx context.Context, req *connect.Request[pb.ListRedeemCodesRequest]) (*connect.Response[pb.ListRedeemCodesResponse], error) {
	if svc.Redeem == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("redeem exchange unavailable"))
	}
	channels := make([]string, 0, len(req.Msg.GetChannels()))
	for _, channel := range req.Msg.GetChannels() {
		if value := redeemsvc.ChannelFromProto(channel); value != "" {
			channels = append(channels, value)
		}
	}
	entries, cursor, err := svc.Redeem.List(ctx, req.Msg.GetCursor(), int(req.Msg.GetPageSize()), req.Msg.GetIncludeExpired(), req.Msg.GetOnlyPropagatable(), channels)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	resp := &pb.ListRedeemCodesResponse{NextCursor: cursor, Entries: make([]*pb.RedeemCode, 0, len(entries))}
	for _, entry := range entries {
		resp.Entries = append(resp.Entries, redeemCodeToProto(entry))
	}
	return connect.NewResponse(resp), nil
}

func (svc *Services) BrowseRedeemCodes(ctx context.Context, req *connect.Request[pb.BrowseRedeemCodesRequest]) (*connect.Response[pb.BrowseRedeemCodesResponse], error) {
	if svc.Redeem == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("redeem exchange unavailable"))
	}
	page := int(req.Msg.GetPage())
	pageSize := int(req.Msg.GetPageSize())
	if page < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("page must be non-negative"))
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	if page > int(^uint(0)>>1)/pageSize {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("page is too large"))
	}
	history := false
	switch req.Msg.GetFilter() {
	case pb.RedeemBrowseFilter_REDEEM_BROWSE_FILTER_UNSPECIFIED,
		pb.RedeemBrowseFilter_REDEEM_BROWSE_FILTER_ACTIVE:
	case pb.RedeemBrowseFilter_REDEEM_BROWSE_FILTER_HISTORY:
		history = true
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid redeem browse filter required"))
	}
	entries, activeTotal, historyTotal, err := svc.Redeem.Browse(ctx, page, pageSize, history)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	resp := &pb.BrowseRedeemCodesResponse{
		Page:         int32(page),
		PageSize:     int32(pageSize),
		ActiveTotal:  activeTotal,
		HistoryTotal: historyTotal,
		Entries:      make([]*pb.RedeemCode, 0, len(entries)),
	}
	for _, entry := range entries {
		resp.Entries = append(resp.Entries, redeemCodeToProto(entry))
	}
	return connect.NewResponse(resp), nil
}

func (svc *Services) SubmitRedeemCodes(ctx context.Context, req *connect.Request[pb.SubmitRedeemCodesRequest]) (*connect.Response[pb.SubmitRedeemCodesResponse], error) {
	if svc.Redeem == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("redeem exchange unavailable"))
	}
	entries := req.Msg.GetEntries()
	if len(entries) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("at least one redeem code required"))
	}
	if len(entries) > redeemsvc.MaxSubmitBatch {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("at most %d redeem codes per request", redeemsvc.MaxSubmitBatch))
	}
	senderID := req.Msg.GetSenderInstanceId()
	limiterKey := senderID
	if limiterKey == "" {
		limiterKey = remoteIP(req.Peer().Addr)
	}
	if !svc.RedeemLimiter.Allow(limiterKey, len(entries)) {
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("redeem submissions are temporarily rate limited"))
	}
	submissions := make([]redeemsvc.Submission, 0, len(entries))
	for _, entry := range entries {
		channel := redeemsvc.ChannelFromProto(entry.GetChannel())
		if channel == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("redeem channel required"))
		}
		if entry.GetPermanent() && entry.GetExpiresAt() != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("permanent redeem codes must not include expires_at"))
		}
		if !entry.GetPermanent() && entry.GetExpiresAt() == nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("finite redeem codes require expires_at"))
		}
		var expiresAt *time.Time
		if entry.GetExpiresAt() != nil {
			if !entry.GetExpiresAt().IsValid() {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid redeem expiry"))
			}
			value := entry.GetExpiresAt().AsTime().UTC()
			expiresAt = &value
		}
		submissions = append(submissions, redeemsvc.Submission{
			Code: entry.GetCode(), Channel: channel, ExpiresAt: expiresAt,
			ReportedValidation: redeemsvc.ValidationFromProto(entry.GetReportedValidation()),
			OriginInstanceID:   entry.GetOriginInstanceId(),
		})
	}
	sourceKey := "public:" + remoteIP(req.Peer().Addr)
	if senderID != "" {
		sourceKey = "peer:" + senderID
	}
	results := svc.Redeem.Submit(ctx, submissions, senderID, sourceKey)
	resp := &pb.SubmitRedeemCodesResponse{Results: make([]*pb.RedeemSubmitResult, 0, len(results))}
	for _, result := range results {
		item := &pb.RedeemSubmitResult{}
		if result.Code != nil {
			item.Fingerprint = result.Code.Fingerprint
		}
		switch {
		case result.Err != nil:
			item.Disposition = pb.RedeemSubmitDisposition_REDEEM_SUBMIT_DISPOSITION_REJECTED
			item.Message = result.Err.Error()
		case result.Created:
			item.Disposition = pb.RedeemSubmitDisposition_REDEEM_SUBMIT_DISPOSITION_ACCEPTED
		default:
			item.Disposition = pb.RedeemSubmitDisposition_REDEEM_SUBMIT_DISPOSITION_DUPLICATE
		}
		resp.Results = append(resp.Results, item)
	}
	return connect.NewResponse(resp), nil
}

func redeemCodeToProto(entry *store.RedeemCode) *pb.RedeemCode {
	if entry == nil {
		return nil
	}
	out := &pb.RedeemCode{
		Fingerprint:      entry.Fingerprint,
		Code:             entry.Code,
		Channel:          redeemsvc.ChannelToProto(entry.Channel),
		Permanent:        entry.ExpiresAt == nil,
		Validation:       redeemsvc.ValidationToProto(entry.Validation),
		FirstSeenAt:      timestamppb.New(entry.FirstSeenAt),
		UpdatedAt:        timestamppb.New(entry.UpdatedAt),
		OriginInstanceId: entry.OriginInstanceID,
		LastMessage:      entry.LastMessage,
		ExpiryOverridden: entry.ExpiryOverridden,
	}
	if entry.ExpiresAt != nil {
		out.ExpiresAt = timestamppb.New(*entry.ExpiresAt)
	}
	if entry.LocalVerifiedAt != nil {
		out.LocalVerifiedAt = timestamppb.New(*entry.LocalVerifiedAt)
	}
	if entry.CommunityVerifiedAt != nil {
		out.CommunityVerifiedAt = timestamppb.New(*entry.CommunityVerifiedAt)
	}
	return out
}
