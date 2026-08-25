package auth

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
)

func TestInterceptorUsesCurrentResolvedIdentity(t *testing.T) {
	jwtSvc := NewJWT("test-secret-test-secret-test-secret")
	token, _, err := jwtSvc.GenerateAccessToken(7, "admin")
	if err != nil {
		t.Fatal(err)
	}
	interceptor := NewInterceptor(jwtSvc, func(_ context.Context, userID int64) (*Identity, error) {
		return &Identity{UserID: userID, Role: "user"}, nil
	})
	req := connect.NewRequest(&pb.GetMeRequest{})
	req.Header().Set("Authorization", "Bearer "+token)

	_, err = interceptor.WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		identity := IdentityFromContext(ctx)
		if identity == nil || identity.UserID != 7 || identity.Role != "user" {
			t.Fatalf("resolved identity = %+v, want current user role", identity)
		}
		return connect.NewResponse(&pb.GetMeResponse{}), nil
	})(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
}

func TestInterceptorRejectsDisabledResolvedIdentity(t *testing.T) {
	jwtSvc := NewJWT("test-secret-test-secret-test-secret")
	token, _, err := jwtSvc.GenerateAccessToken(7, "user")
	if err != nil {
		t.Fatal(err)
	}
	interceptor := NewInterceptor(jwtSvc, func(context.Context, int64) (*Identity, error) {
		return nil, ErrIdentityDisabled
	})
	req := connect.NewRequest(&pb.GetMeRequest{})
	req.Header().Set("Authorization", "Bearer "+token)

	_, err = interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("disabled identity reached protected handler")
		return nil, nil
	})(context.Background(), req)
	if connect.CodeOf(err) != connect.CodeUnauthenticated || !errors.Is(err, ErrIdentityDisabled) {
		t.Fatalf("disabled identity error = %v, want unauthenticated ErrIdentityDisabled", err)
	}
}

func TestInterceptorReturnsInternalForResolverFailure(t *testing.T) {
	jwtSvc := NewJWT("test-secret-test-secret-test-secret")
	token, _, err := jwtSvc.GenerateAccessToken(7, "user")
	if err != nil {
		t.Fatal(err)
	}
	resolverErr := errors.New("database unavailable")
	interceptor := NewInterceptor(jwtSvc, func(context.Context, int64) (*Identity, error) {
		return nil, resolverErr
	})
	req := connect.NewRequest(&pb.GetMeRequest{})
	req.Header().Set("Authorization", "Bearer "+token)

	_, err = interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatal("failed identity lookup reached protected handler")
		return nil, nil
	})(context.Background(), req)
	if connect.CodeOf(err) != connect.CodeInternal || !errors.Is(err, resolverErr) {
		t.Fatalf("resolver failure = %v, want internal error", err)
	}
}
