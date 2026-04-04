package ctxutil

import (
	"context"

	"taskhub/internal/models"
)

type contextKey string

const (
	ctxKeyUser contextKey = "user"
	ctxKeyRole contextKey = "role"
)

func UserFromCtx(ctx context.Context) *models.User {
	u, _ := ctx.Value(ctxKeyUser).(*models.User)
	return u
}

func SetUser(ctx context.Context, u *models.User) context.Context {
	return context.WithValue(ctx, ctxKeyUser, u)
}

func RoleFromCtx(ctx context.Context) string {
	r, _ := ctx.Value(ctxKeyRole).(string)
	return r
}

func SetRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, ctxKeyRole, role)
}
