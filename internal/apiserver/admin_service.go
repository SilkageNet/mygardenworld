package apiserver

import (
	"context"
	"errors"

	connect "connectrpc.com/connect"
	"golang.org/x/crypto/bcrypt"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
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
	if len(in.GetPassword()) < 6 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("password must be at least 6 characters"))
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

func (svc *Services) GetUser(ctx context.Context, req *connect.Request[pb.GetUserRequest]) (*connect.Response[pb.GetUserResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	user, err := svc.DB.GetUserByID(ctx, req.Msg.GetUserId())
	if err != nil {
		return nil, mapErr(err)
	}
	count, _ := svc.DB.CountAccountsByUser(ctx, user.ID)
	return connect.NewResponse(&pb.GetUserResponse{User: userToProto(user, count)}), nil
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

func (svc *Services) DeleteUser(ctx context.Context, req *connect.Request[pb.DeleteUserRequest]) (*connect.Response[pb.DeleteUserResponse], error) {
	if err := svc.requireAdmin(ctx); err != nil {
		return nil, err
	}
	if err := svc.DB.DeleteUser(ctx, req.Msg.GetUserId()); err != nil {
		return nil, mapErr(err)
	}
	return connect.NewResponse(&pb.DeleteUserResponse{}), nil
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
