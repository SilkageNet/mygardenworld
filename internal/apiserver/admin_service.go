package apiserver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	connect "connectrpc.com/connect"
	"golang.org/x/crypto/bcrypt"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/runner"
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
