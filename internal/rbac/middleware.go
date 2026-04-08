package rbac

import (
	"net/http"
	"taskhub/internal/ctxutil"
	"taskhub/internal/httputil"
)

func RequireRole(minRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := ctxutil.RoleFromCtx(r.Context())
			if !HasRole(role, minRole) {
				httputil.JSONError(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
