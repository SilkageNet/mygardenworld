package apiserver

import (
	"context"
	"errors"
	"time"

	connect "connectrpc.com/connect"
	"golang.org/x/crypto/bcrypt"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/store"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (svc *Services) Login(ctx context.Context, req *connect.Request[pb.LoginRequest]) (*connect.Response[pb.AuthResponse], error) {
	in := req.Msg
	if in.GetUsername() == "" || in.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("username/password required"))
	}
	user, err := svc.DB.GetUserByUsername(ctx, in.GetUsername())
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
		}
		return nil, mapErr(err)
	}
	if user.Status != "active" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("account disabled"))
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.GetPassword())); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}
	pair, err := svc.JWT.GenerateTokenPair(user.ID, user.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := svc.DB.SaveRefreshToken(ctx, user.ID, pair.RefreshToken, time.Now().Add(auth.RefreshTokenDuration)); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	count, _ := svc.DB.CountAccountsByUser(ctx, user.ID)
	return connect.NewResponse(&pb.AuthResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		User:         userToProto(user, count),
	}), nil
}

func (svc *Services) Refresh(ctx context.Context, req *connect.Request[pb.RefreshRequest]) (*connect.Response[pb.AuthResponse], error) {
	token := req.Msg.GetRefreshToken()
	if token == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("refresh_token required"))
	}
	userID, err := svc.DB.ValidateRefreshToken(ctx, token)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired refresh token"))
	}
	_ = svc.DB.RevokeRefreshToken(ctx, token)
	user, err := svc.DB.GetUserByID(ctx, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	if user.Status != "active" {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("account disabled"))
	}
	pair, err := svc.JWT.GenerateTokenPair(user.ID, user.Role)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := svc.DB.SaveRefreshToken(ctx, user.ID, pair.RefreshToken, time.Now().Add(auth.RefreshTokenDuration)); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	count, _ := svc.DB.CountAccountsByUser(ctx, user.ID)
	return connect.NewResponse(&pb.AuthResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		User:         userToProto(user, count),
	}), nil
}

func (svc *Services) Logout(ctx context.Context, req *connect.Request[pb.LogoutRequest]) (*connect.Response[pb.LogoutResponse], error) {
	if token := req.Msg.GetRefreshToken(); token != "" {
		_ = svc.DB.RevokeRefreshToken(ctx, token)
	}
	return connect.NewResponse(&pb.LogoutResponse{}), nil
}

func (svc *Services) GetMe(ctx context.Context, _ *connect.Request[pb.GetMeRequest]) (*connect.Response[pb.GetMeResponse], error) {
	userID := auth.UserIDFromContext(ctx)
	if userID == 0 {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	user, err := svc.DB.GetUserByID(ctx, userID)
	if err != nil {
		return nil, mapErr(err)
	}
	count, _ := svc.DB.CountAccountsByUser(ctx, user.ID)
	return connect.NewResponse(&pb.GetMeResponse{User: userToProto(user, count)}), nil
}

func userToProto(u *store.User, accountCount int) *pb.User {
	if u == nil {
		return nil
	}
	return &pb.User{
		Id:              u.ID,
		Username:        u.Username,
		Email:           u.Email,
		Role:            u.Role,
		MaxAccounts:     int32(u.MaxAccounts),
		CurrentAccounts: int32(accountCount),
		Status:          u.Status,
		CreatedAt:       timestamppb.New(u.CreatedAt),
		UpdatedAt:       timestamppb.New(u.UpdatedAt),
	}
}
