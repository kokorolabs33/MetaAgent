package models

import (
	"encoding/json"
	"time"
)

type Organization struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name"`
	Slug                 string          `json:"slug"`
	Plan                 string          `json:"plan"`
	Settings             json.RawMessage `json:"settings"`
	BudgetUSDMonthly     *float64        `json:"budget_usd_monthly,omitempty"`
	BudgetAlertThreshold float64         `json:"budget_alert_threshold"`
	CreatedAt            time.Time       `json:"created_at"`
}

type OrgListItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	Name           string    `json:"name"`
	AvatarURL      string    `json:"avatar_url"`
	AuthProvider   string    `json:"auth_provider,omitempty"`
	AuthProviderID string    `json:"auth_provider_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type OrgMember struct {
	OrgID    string    `json:"org_id"`
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// OrgMemberWithUser — explicit fields, no embedded User (avoids leaking auth fields)
type OrgMemberWithUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	AvatarURL string    `json:"avatar_url"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
}

type PageRequest struct {
	Cursor string
	Limit  int
}

type PageResponse[T any] struct {
	Items      []T     `json:"items"`
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

func NewPageRequest(cursor string, limit int) PageRequest {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return PageRequest{Cursor: cursor, Limit: limit}
}
