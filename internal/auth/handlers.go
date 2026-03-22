package auth

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/httputil"
	"taskhub/internal/models"
)

type Handler struct {
	DB           *pgxpool.Pool
	Sessions     *SessionStore
	GoogleID     string
	GoogleSecret string
	FrontendURL  string
}

// GoogleLogin redirects to Google OAuth consent screen.
func (h *Handler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	// TODO: generate state param, store in cookie, build Google OAuth URL
	httputil.JSONError(w, "Google OAuth not yet configured", http.StatusNotImplemented)
}

// GoogleCallback handles the OAuth callback from Google.
func (h *Handler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	// TODO: validate state, exchange code for token, fetch user info
	_ = r.URL.Query().Get("code")
	_ = r.URL.Query().Get("state")
	httputil.JSONError(w, "Google OAuth callback not yet implemented", http.StatusNotImplemented)
}

// Logout deletes session and clears cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := h.Sessions.Delete(r.Context(), cookie.Value); err != nil {
		httputil.JSONError(w, "logout failed", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// upsertUser creates a user if they don't exist, or returns the existing one.
func (h *Handler) upsertUser(ctx context.Context, email, name, avatarURL, provider, providerID string) (*models.User, error) {
	var u models.User
	err := h.DB.QueryRow(ctx,
		`SELECT id, email, name, avatar_url, auth_provider, auth_provider_id, created_at
		 FROM users WHERE email = $1`, email).
		Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.AuthProvider, &u.AuthProviderID, &u.CreatedAt)
	if err == nil {
		return &u, nil
	}

	u = models.User{
		ID:             uuid.New().String(),
		Email:          email,
		Name:           name,
		AvatarURL:      avatarURL,
		AuthProvider:   provider,
		AuthProviderID: providerID,
		CreatedAt:      time.Now(),
	}
	_, err = h.DB.Exec(ctx,
		`INSERT INTO users (id, email, name, avatar_url, auth_provider, auth_provider_id)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		u.ID, u.Email, u.Name, u.AvatarURL, u.AuthProvider, u.AuthProviderID)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}
