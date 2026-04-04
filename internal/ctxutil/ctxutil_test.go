package ctxutil

import (
	"context"
	"testing"

	"taskhub/internal/models"
)

func TestSetUser_UserFromCtx_RoundTrip(t *testing.T) {
	u := &models.User{
		ID:    "user-1",
		Email: "alice@example.com",
		Name:  "Alice",
	}

	ctx := SetUser(context.Background(), u)
	got := UserFromCtx(ctx)

	if got == nil {
		t.Fatal("expected non-nil user")
	}
	if got.ID != u.ID {
		t.Errorf("user ID = %q, want %q", got.ID, u.ID)
	}
	if got.Email != u.Email {
		t.Errorf("user Email = %q, want %q", got.Email, u.Email)
	}
	if got.Name != u.Name {
		t.Errorf("user Name = %q, want %q", got.Name, u.Name)
	}
}

func TestUserFromCtx_EmptyContext(t *testing.T) {
	got := UserFromCtx(context.Background())
	if got != nil {
		t.Errorf("expected nil user from empty context, got %+v", got)
	}
}

func TestSetRole_RoleFromCtx_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		role string
	}{
		{"admin role", "admin"},
		{"viewer role", "viewer"},
		{"empty role", ""},
		{"custom role", "org-manager"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := SetRole(context.Background(), tt.role)
			got := RoleFromCtx(ctx)
			if got != tt.role {
				t.Errorf("RoleFromCtx = %q, want %q", got, tt.role)
			}
		})
	}
}

func TestRoleFromCtx_EmptyContext(t *testing.T) {
	got := RoleFromCtx(context.Background())
	if got != "" {
		t.Errorf("expected empty string from empty context, got %q", got)
	}
}

func TestSetUser_OverwritesPrevious(t *testing.T) {
	u1 := &models.User{ID: "user-1", Name: "Alice"}
	u2 := &models.User{ID: "user-2", Name: "Bob"}

	ctx := SetUser(context.Background(), u1)
	ctx = SetUser(ctx, u2)

	got := UserFromCtx(ctx)
	if got.ID != u2.ID {
		t.Errorf("expected overwritten user ID %q, got %q", u2.ID, got.ID)
	}
}

func TestSetRole_OverwritesPrevious(t *testing.T) {
	ctx := SetRole(context.Background(), "admin")
	ctx = SetRole(ctx, "viewer")

	got := RoleFromCtx(ctx)
	if got != "viewer" {
		t.Errorf("expected overwritten role %q, got %q", "viewer", got)
	}
}

func TestUserAndRole_Independent(t *testing.T) {
	u := &models.User{ID: "user-1", Name: "Alice"}
	ctx := SetUser(context.Background(), u)
	ctx = SetRole(ctx, "admin")

	// Both values should be retrievable
	gotUser := UserFromCtx(ctx)
	gotRole := RoleFromCtx(ctx)

	if gotUser == nil || gotUser.ID != "user-1" {
		t.Errorf("expected user-1, got %+v", gotUser)
	}
	if gotRole != "admin" {
		t.Errorf("expected admin role, got %q", gotRole)
	}
}
