package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	connect "connectrpc.com/connect"
	"golang.org/x/crypto/bcrypt"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	redeemsvc "github.com/SilkageNet/mygardenworld/internal/redeem"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
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
	username := strings.TrimSpace(in.GetUsername())
	email := strings.TrimSpace(in.GetEmail())
	if username == "" || email == "" || in.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("username/email/password required"))
	}
	if err := ValidatePassword(in.GetPassword()); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	role := "user"
	maxAccounts := 5
	status := "active"
	if in.Role != nil {
		var err error
		role, err = userRoleStore(in.GetRole())
		if err != nil {
			return nil, err
		}
	}
	if in.MaxAccounts != nil {
		maxAccounts = int(in.GetMaxAccounts())
		if err := validateMaxAccounts(maxAccounts); err != nil {
			return nil, err
		}
	}
	if in.Status != nil {
		var err error
		status, err = userStatusStore(in.GetStatus())
		if err != nil {
			return nil, err
		}
	}
	if role == "admin" && status == "disabled" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("admin users cannot be disabled"))
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.GetPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	user, err := svc.DB.CreateUserWithOptions(ctx, username, email, string(hash), role, maxAccounts, status)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.CreateUserResponse{User: userToProto(user, 0)}), nil
}

func (svc *Services) ListUsers(ctx context.Context, req *connect.Request[pb.ListUsersRequest]) (*connect.Response[pb.ListUsersResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	page := int(req.Msg.GetPage())
	pageSize := int(req.Msg.GetPageSize())
	if page < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("page must be non-negative"))
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	if page > int(^uint(0)>>1)/pageSize {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("page is too large"))
	}
	offset := page * pageSize
	users, total, err := svc.DB.ListUsers(ctx, offset, pageSize)
	if err != nil {
		return nil, mapErr(err)
	}
	resp := &pb.ListUsersResponse{Total: int32(total)}
	for _, u := range users {
		count, err := svc.DB.CountAccountsByUser(ctx, u.ID)
		if err != nil {
			return nil, mapErr(err)
		}
		resp.Users = append(resp.Users, userToProto(u, count))
	}
	return connect.NewResponse(resp), nil
}

func (svc *Services) UpdateUser(ctx context.Context, req *connect.Request[pb.UpdateUserRequest]) (*connect.Response[pb.UpdateUserResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	in := req.Msg
	target, err := svc.DB.GetUserByID(ctx, in.GetUserId())
	if err != nil {
		return nil, mapErr(err)
	}
	var role *string
	var maxAccounts *int
	var status *string
	if in.Role != nil {
		r, err := userRoleStore(*in.Role)
		if err != nil {
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
		s, err := userStatusStore(*in.Status)
		if err != nil {
			return nil, err
		}
		status = &s
	}
	if status != nil && *status == "disabled" {
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
	if status != nil && *status == "disabled" && user.Status == "disabled" {
		if err := svc.disableUserAccess(ctx, target.ID); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("user disabled but access cleanup failed: %w", err))
		}
	}
	count, err := svc.DB.CountAccountsByUser(ctx, user.ID)
	if err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.UpdateUserResponse{User: userToProto(user, count)}), nil
}

func userRoleStore(role pb.UserRole) (string, error) {
	switch role {
	case pb.UserRole_USER_ROLE_USER:
		return "user", nil
	case pb.UserRole_USER_ROLE_ADMIN:
		return "admin", nil
	default:
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("valid role required"))
	}
}

func userStatusStore(status pb.UserStatus) (string, error) {
	switch status {
	case pb.UserStatus_USER_STATUS_ACTIVE:
		return "active", nil
	case pb.UserStatus_USER_STATUS_DISABLED:
		return "disabled", nil
	default:
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("valid status required"))
	}
}

func (svc *Services) disableUserAccess(ctx context.Context, userID int64) error {
	accounts, err := svc.DB.ListAccounts(ctx, userID)
	if err != nil {
		return err
	}
	var firstErr error
	for _, account := range accounts {
		var runtime *runner.Runner
		if svc.Manager != nil {
			runtime = svc.Manager.Get(account.ID)
		}
		if err := svc.disableAutomation(ctx, account.ID, runtime); err != nil && firstErr == nil {
			firstErr = err
		}
		if svc.Manager != nil {
			_ = svc.Manager.Stop(account.ID)
			svc.Manager.ClearLastDiagnostics(account.ID)
		}
	}
	if err := svc.DB.RevokeAllRefreshTokens(ctx, userID); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

func (svc *Services) GetSystemStats(ctx context.Context, _ *connect.Request[pb.GetSystemStatsRequest]) (*connect.Response[pb.GetSystemStatsResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	_, total, err := svc.DB.ListUsers(ctx, 0, 1)
	if err != nil {
		return nil, mapErr(err)
	}
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

func (svc *Services) UpdateRedeemCodeExpiry(ctx context.Context, req *connect.Request[pb.UpdateRedeemCodeExpiryRequest]) (*connect.Response[pb.UpdateRedeemCodeExpiryResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if svc.Redeem == nil {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("redeem exchange unavailable"))
	}
	fingerprint := strings.TrimSpace(req.Msg.GetFingerprint())
	if fingerprint == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("redeem code fingerprint required"))
	}
	var expiresAt *time.Time
	clearOverride := false
	switch req.Msg.GetMode() {
	case pb.RedeemExpiryOverrideMode_REDEEM_EXPIRY_OVERRIDE_MODE_FINITE:
		if req.Msg.GetExpiresAt() == nil || !req.Msg.GetExpiresAt().IsValid() {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid expires_at required for finite expiry"))
		}
		value := req.Msg.GetExpiresAt().AsTime().UTC()
		if !value.After(time.Now().UTC()) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("redeem code expiry must be in the future"))
		}
		expiresAt = &value
	case pb.RedeemExpiryOverrideMode_REDEEM_EXPIRY_OVERRIDE_MODE_PERMANENT:
		if req.Msg.GetExpiresAt() != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("permanent expiry must not include expires_at"))
		}
	case pb.RedeemExpiryOverrideMode_REDEEM_EXPIRY_OVERRIDE_MODE_SOURCE:
		if req.Msg.GetExpiresAt() != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source expiry must not include expires_at"))
		}
		clearOverride = true
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("valid redeem expiry override mode required"))
	}
	entry, err := svc.Redeem.UpdateExpiry(ctx, fingerprint, expiresAt, clearOverride)
	if err != nil {
		return nil, mapErr(err)
	}
	if svc.Log != nil {
		identity := auth.IdentityFromContext(ctx)
		var userID int64
		if identity != nil {
			userID = identity.UserID
		}
		svc.Log.Info("administrator updated redeem code expiry",
			"user_id", userID,
			"fingerprint", fingerprint,
			"mode", req.Msg.GetMode().String(),
			"expires_at", expiresAt,
		)
	}
	return connect.NewResponse(&pb.UpdateRedeemCodeExpiryResponse{Code: redeemCodeToProto(entry)}), nil
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
