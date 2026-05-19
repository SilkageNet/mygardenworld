package auth

import "context"

type ctxKey struct{}

type Identity struct {
	UserID int64
	Role   string
}

func ContextWithIdentity(ctx context.Context, id *Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func IdentityFromContext(ctx context.Context) *Identity {
	id, _ := ctx.Value(ctxKey{}).(*Identity)
	return id
}

func IsAdmin(ctx context.Context) bool {
	id := IdentityFromContext(ctx)
	return id != nil && id.Role == "admin"
}

func UserIDFromContext(ctx context.Context) int64 {
	id := IdentityFromContext(ctx)
	if id == nil {
		return 0
	}
	return id.UserID
}
