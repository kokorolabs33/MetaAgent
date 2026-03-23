package auth

import (
	"net/http"
	"time"

	"taskhub/internal/ctxutil"
	"taskhub/internal/httputil"
	"taskhub/internal/models"
)

const sessionCookieName = "taskhub_session"

type Middleware struct {
	Sessions  *SessionStore
	LocalMode bool // if true, skip auth and inject local user
}

// localUser is the auto-injected user in local mode.
var localUser = &models.User{
	ID:        "local-user",
	Email:     "local@taskhub.local",
	Name:      "Local User",
	CreatedAt: time.Now(),
}

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Local mode: skip auth, inject local user
		if m.LocalMode {
			ctx := ctxutil.SetUser(r.Context(), localUser)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Cloud mode: validate session cookie
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			httputil.JSONError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := m.Sessions.Validate(r.Context(), cookie.Value)
		if err != nil {
			httputil.JSONError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := ctxutil.SetUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
