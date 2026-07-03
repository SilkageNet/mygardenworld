package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	connect "connectrpc.com/connect"
	"golang.org/x/crypto/bcrypt"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/store"

	"google.golang.org/protobuf/types/known/timestamppb"
)

const refreshCookieName = "mgw_refresh_token"

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
	resp := connect.NewResponse(&pb.AuthResponse{
		AccessToken: pair.AccessToken,
		User:        userToProto(user, count),
	})
	setRefreshCookie(resp.Header(), pair.RefreshToken, req.Header())
	return resp, nil
}

func (svc *Services) Refresh(ctx context.Context, req *connect.Request[pb.RefreshRequest]) (*connect.Response[pb.AuthResponse], error) {
	token := refreshTokenFromRequest(req.Header(), req.Msg.GetRefreshToken())
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
	resp := connect.NewResponse(&pb.AuthResponse{
		AccessToken: pair.AccessToken,
		User:        userToProto(user, count),
	})
	setRefreshCookie(resp.Header(), pair.RefreshToken, req.Header())
	return resp, nil
}

func (svc *Services) Logout(ctx context.Context, req *connect.Request[pb.LogoutRequest]) (*connect.Response[pb.LogoutResponse], error) {
	if token := refreshTokenFromRequest(req.Header(), req.Msg.GetRefreshToken()); token != "" {
		_ = svc.DB.RevokeRefreshToken(ctx, token)
	}
	resp := connect.NewResponse(&pb.LogoutResponse{})
	clearRefreshCookie(resp.Header(), req.Header())
	return resp, nil
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

func refreshTokenFromRequest(headers http.Header, explicit string) string {
	if explicit != "" {
		return explicit
	}
	req := http.Request{Header: headers}
	cookie, err := req.Cookie(refreshCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func setRefreshCookie(headers http.Header, token string, reqHeaders http.Header) {
	http.SetCookie(&headerResponseWriter{headers: headers}, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/mygardenworld.v1.AuthService",
		Expires:  time.Now().Add(auth.RefreshTokenDuration),
		MaxAge:   int(auth.RefreshTokenDuration.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestLooksHTTPS(reqHeaders),
	})
}

func clearRefreshCookie(headers http.Header, reqHeaders http.Header) {
	http.SetCookie(&headerResponseWriter{headers: headers}, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/mygardenworld.v1.AuthService",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestLooksHTTPS(reqHeaders),
	})
}

func requestLooksHTTPS(headers http.Header) bool {
	if strings.EqualFold(headers.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	if forwarded := headers.Get("Forwarded"); strings.Contains(strings.ToLower(forwarded), "proto=https") {
		return true
	}
	return false
}

type headerResponseWriter struct {
	headers http.Header
}

func (w *headerResponseWriter) Header() http.Header {
	return w.headers
}

func (w *headerResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("headerResponseWriter does not write bodies")
}

func (w *headerResponseWriter) WriteHeader(int) {}
