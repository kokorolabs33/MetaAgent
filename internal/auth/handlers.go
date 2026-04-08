package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/ctxutil"
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

// SimpleLogin handles POST /api/auth/login.
// MVP: email-only login (no password). Creates user if not exists.
func (h *Handler) SimpleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.JSONError(w, "invalid body", http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		httputil.JSONError(w, "email is required", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		parts := strings.Split(req.Email, "@")
		req.Name = parts[0]
	}

	// Find or create user
	var userID string
	err := h.DB.QueryRow(r.Context(),
		`SELECT id FROM users WHERE email = $1`, req.Email).Scan(&userID)
	if err != nil {
		// Create new user
		userID = uuid.New().String()
		_, err = h.DB.Exec(r.Context(),
			`INSERT INTO users (id, email, name, auth_provider, created_at)
			 VALUES ($1, $2, $3, 'email', NOW())`,
			userID, req.Email, req.Name)
		if err != nil {
			httputil.JSONError(w, "could not create user", http.StatusInternalServerError)
			return
		}
	}

	// Create session
	sessionID, err := h.Sessions.Create(r.Context(), userID)
	if err != nil {
		httputil.JSONError(w, "could not create session", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 7, // 7 days
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"user_id": userID,
	})
}

// GetMe handles GET /api/auth/me.
// Returns the current authenticated user.
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	user := ctxutil.UserFromCtx(r.Context())
	if user == nil {
		httputil.JSONError(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(user)
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
