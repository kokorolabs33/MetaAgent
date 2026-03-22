package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/ctxutil"
	"taskhub/internal/models"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

type OrgHandler struct {
	DB *pgxpool.Pool
}

// orgListItemWithRole is the response shape for List — lightweight org info plus the user's role.
type orgListItemWithRole struct {
	models.OrgListItem
	Role string `json:"role"`
}

// List returns the organisations the authenticated user belongs to.
func (h *OrgHandler) List(w http.ResponseWriter, r *http.Request) {
	user := ctxutil.UserFromCtx(r.Context())

	rows, err := h.DB.Query(r.Context(),
		`SELECT o.id, o.name, o.slug, o.plan, o.created_at, om.role
		 FROM organizations o
		 JOIN org_members om ON om.org_id = o.id
		 WHERE om.user_id = $1
		 ORDER BY o.created_at DESC`, user.ID)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := make([]orgListItemWithRole, 0)
	for rows.Next() {
		var item orgListItemWithRole
		if err := rows.Scan(&item.ID, &item.Name, &item.Slug, &item.Plan, &item.CreatedAt, &item.Role); err != nil {
			jsonError(w, "scan failed", http.StatusInternalServerError)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		jsonError(w, "rows iteration failed", http.StatusInternalServerError)
		return
	}

	jsonOK(w, items)
}

// createOrgRequest is the expected body for POST /api/orgs.
type createOrgRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// Create creates a new organisation and makes the caller the owner.
func (h *OrgHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createOrgRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)

	if req.Name == "" || req.Slug == "" {
		jsonError(w, "name and slug are required", http.StatusBadRequest)
		return
	}
	if !slugRe.MatchString(req.Slug) {
		jsonError(w, "slug must match ^[a-z0-9][a-z0-9-]*$", http.StatusBadRequest)
		return
	}

	user := ctxutil.UserFromCtx(r.Context())
	now := time.Now().UTC()
	org := models.Organization{
		ID:                   uuid.New().String(),
		Name:                 req.Name,
		Slug:                 req.Slug,
		Plan:                 "free",
		Settings:             json.RawMessage(`{}`),
		BudgetAlertThreshold: 0.8,
		CreatedAt:            now,
	}

	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		jsonError(w, "could not begin transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	_, err = tx.Exec(r.Context(),
		`INSERT INTO organizations (id, name, slug, plan, settings, budget_alert_threshold, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		org.ID, org.Name, org.Slug, org.Plan, org.Settings, org.BudgetAlertThreshold, org.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			jsonError(w, "slug already taken", http.StatusConflict)
			return
		}
		jsonError(w, "could not create organization", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(r.Context(),
		`INSERT INTO org_members (org_id, user_id, role, joined_at)
		 VALUES ($1, $2, 'owner', $3)`,
		org.ID, user.ID, now)
	if err != nil {
		jsonError(w, "could not add owner", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		jsonError(w, "could not commit transaction", http.StatusInternalServerError)
		return
	}

	jsonCreated(w, org)
}

// Get returns the organisation already loaded by the RequireOrg middleware.
func (h *OrgHandler) Get(w http.ResponseWriter, r *http.Request) {
	org := ctxutil.OrgFromCtx(r.Context())
	jsonOK(w, org)
}

// updateOrgRequest holds the optional fields for PUT /api/orgs/:org_id.
type updateOrgRequest struct {
	Name                 *string  `json:"name"`
	BudgetUSDMonthly     *float64 `json:"budget_usd_monthly"`
	BudgetAlertThreshold *float64 `json:"budget_alert_threshold"`
}

// Update modifies an organisation's mutable fields (owner-only, enforced by route middleware).
func (h *OrgHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req updateOrgRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	org := ctxutil.OrgFromCtx(r.Context())

	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			jsonError(w, "name cannot be empty", http.StatusBadRequest)
			return
		}
		org.Name = name
	}
	if req.BudgetUSDMonthly != nil {
		org.BudgetUSDMonthly = req.BudgetUSDMonthly
	}
	if req.BudgetAlertThreshold != nil {
		org.BudgetAlertThreshold = *req.BudgetAlertThreshold
	}

	_, err := h.DB.Exec(r.Context(),
		`UPDATE organizations
		 SET name = $1, budget_usd_monthly = $2, budget_alert_threshold = $3
		 WHERE id = $4`,
		org.Name, org.BudgetUSDMonthly, org.BudgetAlertThreshold, org.ID)
	if err != nil {
		jsonError(w, "could not update organization", http.StatusInternalServerError)
		return
	}

	jsonOK(w, org)
}
