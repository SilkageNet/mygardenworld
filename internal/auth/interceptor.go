package auth

import (
	"context"
	"strings"

	"connectrpc.com/connect"
)

var publicProcedures = map[string]bool{
	"/mygardenworld.v1.AuthService/Login":   true,
	"/mygardenworld.v1.AuthService/Refresh": true,
	"/mygardenworld.v1.AuthService/Logout":  true,
}

type Interceptor struct {
	jwtSvc *JWT
}

func NewInterceptor(jwtSvc *JWT) *Interceptor {
	return &Interceptor{jwtSvc: jwtSvc}
}

func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if publicProcedures[req.Spec().Procedure] {
			return next(ctx, req)
		}
		id, err := extractIdentity(i.jwtSvc, req.Header())
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}
		ctx = ContextWithIdentity(ctx, id)
		return next(ctx, req)
	}
}

func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if publicProcedures[conn.Spec().Procedure] {
			return next(ctx, conn)
		}
		id, err := extractIdentity(i.jwtSvc, conn.RequestHeader())
		if err != nil {
			return connect.NewError(connect.CodeUnauthenticated, err)
		}
		ctx = ContextWithIdentity(ctx, id)
		return next(ctx, conn)
	}
}

func extractIdentity(jwtSvc *JWT, headers interface{ Get(string) string }) (*Identity, error) {
	auth := headers.Get("Authorization")
	if auth == "" {
		return nil, ErrTokenInvalid
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if token == auth {
		return nil, ErrTokenInvalid
	}
	claims, err := jwtSvc.ValidateAccessToken(token)
	if err != nil {
		return nil, err
	}
	return &Identity{UserID: claims.UserID, Role: claims.Role}, nil
}

// ExtractIdentityFromHeader is exported for use in streaming handlers.
func ExtractIdentityFromHeader(jwtSvc *JWT, authHeader string) (*Identity, error) {
	if authHeader == "" {
		return nil, ErrTokenInvalid
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return nil, ErrTokenInvalid
	}
	claims, err := jwtSvc.ValidateAccessToken(token)
	if err != nil {
		return nil, err
	}
	return &Identity{UserID: claims.UserID, Role: claims.Role}, nil
}
