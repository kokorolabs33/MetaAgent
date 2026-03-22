package rbac

import "testing"

func TestHasRole(t *testing.T) {
	tests := []struct {
		userRole     string
		requiredRole string
		want         bool
	}{
		{"owner", "owner", true},
		{"owner", "admin", true},
		{"owner", "member", true},
		{"owner", "viewer", true},
		{"admin", "owner", false},
		{"admin", "admin", true},
		{"admin", "member", true},
		{"admin", "viewer", true},
		{"member", "owner", false},
		{"member", "admin", false},
		{"member", "member", true},
		{"member", "viewer", true},
		{"viewer", "owner", false},
		{"viewer", "admin", false},
		{"viewer", "member", false},
		{"viewer", "viewer", true},
		{"unknown", "viewer", false},
		{"viewer", "unknown", true},
	}
	for _, tt := range tests {
		t.Run(tt.userRole+"_needs_"+tt.requiredRole, func(t *testing.T) {
			got := HasRole(tt.userRole, tt.requiredRole)
			if got != tt.want {
				t.Errorf("HasRole(%q, %q) = %v, want %v", tt.userRole, tt.requiredRole, got, tt.want)
			}
		})
	}
}

func TestValidRole(t *testing.T) {
	valid := []string{"owner", "admin", "member", "viewer"}
	for _, role := range valid {
		if !ValidRole(role) {
			t.Errorf("ValidRole(%q) = false, want true", role)
		}
	}
	invalid := []string{"", "superadmin", "moderator", "Owner"}
	for _, role := range invalid {
		if ValidRole(role) {
			t.Errorf("ValidRole(%q) = true, want false", role)
		}
	}
}
