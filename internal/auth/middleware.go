package auth

import (
	"net/http"

	"taskhub/internal/ctxutil"
	"taskhub/internal/httputil"
)

const sessionCookieName = "taskhub_session"

type Middleware struct {
	Sessions *SessionStore
}

func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
