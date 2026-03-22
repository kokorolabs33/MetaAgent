package ctxutil

import (
	"context"
	"taskhub/internal/models"
)

type contextKey string

const (
	ctxKeyUser    contextKey = "user"
	ctxKeyOrg     contextKey = "org"
	ctxKeyOrgRole contextKey = "org_role"
)

func UserFromCtx(ctx context.Context) *models.User {
	u, _ := ctx.Value(ctxKeyUser).(*models.User)
	return u
}

func OrgFromCtx(ctx context.Context) *models.Organization {
	o, _ := ctx.Value(ctxKeyOrg).(*models.Organization)
	return o
}

func OrgRoleFromCtx(ctx context.Context) string {
	r, _ := ctx.Value(ctxKeyOrgRole).(string)
	return r
}

func SetUser(ctx context.Context, u *models.User) context.Context {
	return context.WithValue(ctx, ctxKeyUser, u)
}

func SetOrg(ctx context.Context, o *models.Organization) context.Context {
	return context.WithValue(ctx, ctxKeyOrg, o)
}

func SetOrgRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, ctxKeyOrgRole, role)
}
