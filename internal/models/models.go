package models

import (
	"time"
)

type User struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	Name           string    `json:"name"`
	AvatarURL      string    `json:"avatar_url"`
	AuthProvider   string    `json:"auth_provider,omitempty"`
	AuthProviderID string    `json:"auth_provider_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
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
