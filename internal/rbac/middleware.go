package rbac

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/ctxutil"
	"taskhub/internal/httputil"
	"taskhub/internal/models"
)

type Middleware struct {
	DB *pgxpool.Pool
}

func (m *Middleware) RequireOrg(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		orgID := chi.URLParam(r, "org_id")
		if orgID == "" {
			httputil.JSONError(w, "missing org_id", http.StatusBadRequest)
			return
		}

		user := ctxutil.UserFromCtx(r.Context())
		if user == nil {
			httputil.JSONError(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var org models.Organization
		var role string
		err := m.DB.QueryRow(r.Context(),
			`SELECT o.id, o.name, o.slug, o.plan, o.settings, o.budget_usd_monthly, o.budget_alert_threshold, o.created_at, om.role
			 FROM organizations o
			 JOIN org_members om ON om.org_id = o.id
			 WHERE o.id = $1 AND om.user_id = $2`,
			orgID, user.ID).Scan(
			&org.ID, &org.Name, &org.Slug, &org.Plan,
			&org.Settings, &org.BudgetUSDMonthly, &org.BudgetAlertThreshold, &org.CreatedAt,
			&role)
		if err != nil {
			httputil.JSONError(w, "forbidden", http.StatusForbidden)
			return
		}

		if org.Settings == nil {
			org.Settings = json.RawMessage(`{}`)
		}

		ctx := ctxutil.SetOrg(r.Context(), &org)
		ctx = ctxutil.SetOrgRole(ctx, role)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireRole(minRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := ctxutil.OrgRoleFromCtx(r.Context())
			if !HasRole(role, minRole) {
				httputil.JSONError(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
