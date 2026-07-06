package apiserver

import (
	"context"
	"errors"
	"log/slog"
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

var dummyPasswordHash = []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")

func (svc *Services) Login(ctx context.Context, req *connect.Request[pb.LoginRequest]) (*connect.Response[pb.AuthResponse], error) {
	in := req.Msg
	username := strings.TrimSpace(in.GetUsername())
	password := in.GetPassword()
	if username == "" || password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("username/password required"))
	}
	remote := req.Peer().Addr
	if dec, limited := svc.LoginLimiter.Check(username, remote); limited {
		svc.logAuth("warn", "auth_login_limited", username, remote, 0, slog.String("scope", dec.Scope), slog.Time("locked_until", dec.Until))
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("登录尝试过多，请稍后再试"))
	}
	user, err := svc.DB.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
			return svc.rejectInvalidLogin(username, remote, 0)
		}
		return nil, mapErr(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return svc.rejectInvalidLogin(username, remote, user.ID)
	}
	if user.Status != "active" {
		svc.logAuth("warn", "auth_login_disabled", username, remote, user.ID)
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
	svc.LoginLimiter.RecordSuccess(username)
	svc.logAuth("info", "auth_login_success", username, remote, user.ID)
	return resp, nil
}

func (svc *Services) rejectInvalidLogin(username, remote string, userID int64) (*connect.Response[pb.AuthResponse], error) {
	if dec, limited := svc.LoginLimiter.RecordFailure(username, remote); limited {
		svc.logAuth("warn", "auth_login_limited", username, remote, userID, slog.String("scope", dec.Scope), slog.Time("locked_until", dec.Until))
		return nil, connect.NewError(connect.CodeResourceExhausted, errors.New("登录尝试过多，请稍后再试"))
	}
	svc.logAuth("warn", "auth_login_failed", username, remote, userID)
	return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
}

func (svc *Services) logAuth(level, event, username, remote string, userID int64, attrs ...slog.Attr) {
	if svc.Log == nil {
		return
	}
	args := []any{
		"event", event,
		"username", strings.ToLower(strings.TrimSpace(username)),
		"remote_ip", remoteIP(remote),
	}
	if userID > 0 {
		args = append(args, "user_id", userID)
	}
	for _, attr := range attrs {
		args = append(args, attr)
	}
	switch level {
	case "warn":
		svc.Log.Warn("auth login", args...)
	default:
		svc.Log.Info("auth login", args...)
	}
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
		SameSite: http.SameSiteStrictMode,
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
		SameSite: http.SameSiteStrictMode,
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
