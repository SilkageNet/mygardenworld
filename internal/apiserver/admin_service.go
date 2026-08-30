package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	connect "connectrpc.com/connect"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	redeemsvc "github.com/SilkageNet/mygardenworld/internal/redeem"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

func (svc *Services) requireAdmin(ctx context.Context) error {
	if !auth.IsAdmin(ctx) {
		return connect.NewError(connect.CodePermissionDenied, errors.New("admin required"))
	}
	return nil
}

func (svc *Services) CreateUser(ctx context.Context, req *connect.Request[pb.CreateUserRequest]) (*connect.Response[pb.CreateUserResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	in := req.Msg
	if in.GetUsername() == "" || in.GetEmail() == "" || in.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("username/email/password required"))
	}
	if err := ValidatePassword(in.GetPassword()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.GetPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	user, err := svc.DB.CreateUser(ctx, in.GetUsername(), in.GetEmail(), string(hash))
	if err != nil {
		return nil, mapErr(err)
	}
	var role *string
	var maxAccounts *int
	var status *string
	if in.Role != nil {
		v := in.GetRole()
		if err := validateRole(v); err != nil {
			return nil, err
		}
		role = &v
	}
	if in.MaxAccounts != nil {
		v := int(in.GetMaxAccounts())
		if err := validateMaxAccounts(v); err != nil {
			return nil, err
		}
		maxAccounts = &v
	}
	if in.Status != nil {
		v := in.GetStatus()
		if err := validateStatus(v); err != nil {
			return nil, err
		}
		status = &v
	}
	if role != nil || maxAccounts != nil || status != nil {
		user, err = svc.DB.UpdateUser(ctx, user.ID, role, maxAccounts, status)
		if err != nil {
			return nil, mapErr(err)
		}
	}
	return connect.NewResponse(&pb.CreateUserResponse{User: userToProto(user, 0)}), nil
}

func (svc *Services) ListUsers(ctx context.Context, req *connect.Request[pb.ListUsersRequest]) (*connect.Response[pb.ListUsersResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	page := int(req.Msg.GetPage())
	pageSize := int(req.Msg.GetPageSize())
	if pageSize <= 0 {
		pageSize = 50
	}
	offset := page * pageSize
	users, total, err := svc.DB.ListUsers(ctx, offset, pageSize)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &pb.ListUsersResponse{Total: int32(total)}
	for _, u := range users {
		count, _ := svc.DB.CountAccountsByUser(ctx, u.ID)
		resp.Users = append(resp.Users, userToProto(u, count))
	}
	return connect.NewResponse(resp), nil
}

func (svc *Services) UpdateUser(ctx context.Context, req *connect.Request[pb.UpdateUserRequest]) (*connect.Response[pb.UpdateUserResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	in := req.Msg
	var role *string
	var maxAccounts *int
	var status *string
	if in.Role != nil {
		r := *in.Role
		if err := validateRole(r); err != nil {
			return nil, err
		}
		role = &r
	}
	if in.MaxAccounts != nil {
		m := int(*in.MaxAccounts)
		if err := validateMaxAccounts(m); err != nil {
			return nil, err
		}
		maxAccounts = &m
	}
	if in.Status != nil {
		s := *in.Status
		if err := validateStatus(s); err != nil {
			return nil, err
		}
		status = &s
	}
	if status != nil && *status == "disabled" {
		target, err := svc.DB.GetUserByID(ctx, in.GetUserId())
		if err != nil {
			return nil, mapErr(err)
		}
		effectiveRole := target.Role
		if role != nil {
			effectiveRole = *role
		}
		if effectiveRole == "admin" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("admin users cannot be disabled"))
		}
	}
	if maxAccounts != nil {
		count, err := svc.DB.CountAccountsByUser(ctx, in.GetUserId())
		if err != nil {
			return nil, mapErr(err)
		}
		if *maxAccounts < count {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("max_accounts cannot be below current account count"))
		}
	}
	user, err := svc.DB.UpdateUser(ctx, in.GetUserId(), role, maxAccounts, status)
	if err != nil {
		return nil, mapErr(err)
	}
	count, _ := svc.DB.CountAccountsByUser(ctx, user.ID)
	return connect.NewResponse(&pb.UpdateUserResponse{User: userToProto(user, count)}), nil
}

func (svc *Services) GetSystemStats(ctx context.Context, _ *connect.Request[pb.GetSystemStatsRequest]) (*connect.Response[pb.GetSystemStatsResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	users, total, err := svc.DB.ListUsers(ctx, 0, 1)
	if err != nil {
		return nil, mapErr(err)
	}
	_ = users
	allAccounts, err := svc.DB.ListAccounts(ctx, 0)
	if err != nil {
		return nil, mapErr(err)
	}
	var active, connected int32
	for _, acc := range allAccounts {
		if r := svc.Manager.Get(acc.ID); r != nil {
			active++
			if r.Connected() {
				connected++
			}
		}
	}
	return connect.NewResponse(&pb.GetSystemStatsResponse{
		TotalUsers:        int32(total),
		TotalGameAccounts: int32(len(allAccounts)),
		ActiveRunners:     active,
		ConnectedRunners:  connected,
	}), nil
}

func validateRole(role string) error {
	switch role {
	case "admin", "user":
		return nil
	default:
		return connect.NewError(connect.CodeInvalidArgument, errors.New("role must be admin or user"))
	}
}

func validateStatus(status string) error {
	switch status {
	case "active", "disabled":
		return nil
	default:
		return connect.NewError(connect.CodeInvalidArgument, errors.New("status must be active or disabled"))
	}
}

func validateMaxAccounts(maxAccounts int) error {
	if maxAccounts < 0 {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("max_accounts must be non-negative"))
	}
	return nil
}

func (svc *Services) ListRedeemSources(ctx context.Context, _ *connect.Request[pb.ListRedeemSourcesRequest]) (*connect.Response[pb.ListRedeemSourcesResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	sources, err := svc.DB.ListRedeemSources(ctx)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &pb.ListRedeemSourcesResponse{Sources: make([]*pb.RedeemSource, 0, len(sources))}
	for _, source := range sources {
		resp.Sources = append(resp.Sources, redeemSourceToProto(source))
	}
	return connect.NewResponse(resp), nil
}

func (svc *Services) UpsertRedeemSource(ctx context.Context, req *connect.Request[pb.UpsertRedeemSourceRequest]) (*connect.Response[pb.UpsertRedeemSourceResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	typeName := redeemSourceTypeStore(req.Msg.GetType())
	if typeName == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid redeem source type required"))
	}
	parserJSON := strings.TrimSpace(req.Msg.GetParserConfigJson())
	if parserJSON == "" {
		parserJSON = "{}"
	}
	if !json.Valid([]byte(parserJSON)) {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("parser_config_json must be valid JSON"))
	}
	if err := redeemsvc.ValidateSourceEndpoint(req.Msg.GetBaseUrl(), typeName == store.RedeemSourceMyGardenWorld); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if typeName == store.RedeemSourceCustomHTTP {
		if err := redeemsvc.ValidateCustomParserConfig(parserJSON); err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	source, err := svc.DB.UpsertRedeemSource(ctx, store.RedeemSourceInput{
		ID: req.Msg.GetId(), Name: req.Msg.GetName(), Type: typeName,
		BaseURL: req.Msg.GetBaseUrl(), Channel: redeemsvc.ChannelFromProto(req.Msg.GetChannel()),
		ParserConfigJSON: parserJSON, Enabled: req.Msg.GetEnabled(), PushEnabled: req.Msg.GetPushEnabled(),
		PollIntervalSeconds: int(req.Msg.GetPollIntervalSeconds()),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&pb.UpsertRedeemSourceResponse{Source: redeemSourceToProto(source)}), nil
}

func (svc *Services) DeleteRedeemSource(ctx context.Context, req *connect.Request[pb.DeleteRedeemSourceRequest]) (*connect.Response[pb.DeleteRedeemSourceResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if req.Msg.GetId() <= 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid redeem source id required"))
	}
	if err := svc.DB.DeleteRedeemSource(ctx, req.Msg.GetId()); err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.DeleteRedeemSourceResponse{}), nil
}

func (svc *Services) SyncRedeemSource(ctx context.Context, req *connect.Request[pb.SyncRedeemSourceRequest]) (*connect.Response[pb.SyncRedeemSourceResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if svc.Redeem == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("redeem exchange unavailable"))
	}
	if err := svc.Redeem.SyncSource(ctx, req.Msg.GetId()); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	source, err := svc.DB.GetRedeemSource(ctx, req.Msg.GetId())
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.SyncRedeemSourceResponse{Source: redeemSourceToProto(source)}), nil
}

func redeemSourceTypeStore(value pb.RedeemSourceType) string {
	switch value {
	case pb.RedeemSourceType_REDEEM_SOURCE_TYPE_MYGARDENWORLD:
		return store.RedeemSourceMyGardenWorld
	case pb.RedeemSourceType_REDEEM_SOURCE_TYPE_CUSTOM_HTTP:
		return store.RedeemSourceCustomHTTP
	default:
		return ""
	}
}

func redeemSourceTypeProto(value string) pb.RedeemSourceType {
	if value == store.RedeemSourceMyGardenWorld {
		return pb.RedeemSourceType_REDEEM_SOURCE_TYPE_MYGARDENWORLD
	}
	if value == store.RedeemSourceCustomHTTP {
		return pb.RedeemSourceType_REDEEM_SOURCE_TYPE_CUSTOM_HTTP
	}
	return pb.RedeemSourceType_REDEEM_SOURCE_TYPE_UNSPECIFIED
}

func redeemSourceToProto(source *store.RedeemSource) *pb.RedeemSource {
	if source == nil {
		return nil
	}
	out := &pb.RedeemSource{
		Id: source.ID, Name: source.Name, Type: redeemSourceTypeProto(source.Type),
		BaseUrl: source.BaseURL, Channel: redeemsvc.ChannelToProto(source.Channel),
		ParserConfigJson: source.ParserConfigJSON, Enabled: source.Enabled,
		PushEnabled: source.PushEnabled, PollIntervalSeconds: int32(source.PollIntervalSeconds),
		RemoteInstanceId: source.RemoteInstanceID, Cursor: source.Cursor, LastError: source.LastError,
		ObservedCount: source.ObservedCount, TrustedCount: source.TrustedCount,
		SuccessCount: source.SuccessCount, AlreadyRedeemedCount: source.AlreadyRedeemedCount,
		ExpiredCount: source.ExpiredCount, InvalidCount: source.InvalidCount,
		PendingCount: source.PendingCount,
	}
	if source.LastSyncAt != nil {
		out.LastSyncAt = timestamppb.New(*source.LastSyncAt)
	}
	return out
}
