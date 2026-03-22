package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/ctxutil"
	"taskhub/internal/models"
	"taskhub/internal/rbac"
)

type MemberHandler struct {
	DB *pgxpool.Pool
}

// List returns all members of the organisation with user info.
func (h *MemberHandler) List(w http.ResponseWriter, r *http.Request) {
	org := ctxutil.OrgFromCtx(r.Context())

	rows, err := h.DB.Query(r.Context(),
		`SELECT u.id, u.email, u.name, u.avatar_url, om.role, om.joined_at
		 FROM org_members om
		 JOIN users u ON u.id = om.user_id
		 WHERE om.org_id = $1
		 ORDER BY om.joined_at`, org.ID)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := make([]models.OrgMemberWithUser, 0)
	for rows.Next() {
		var m models.OrgMemberWithUser
		if err := rows.Scan(&m.ID, &m.Email, &m.Name, &m.AvatarURL, &m.Role, &m.JoinedAt); err != nil {
			jsonError(w, "scan failed", http.StatusInternalServerError)
			return
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		jsonError(w, "rows iteration failed", http.StatusInternalServerError)
		return
	}

	jsonOK(w, items)
}

// inviteRequest is the expected body for POST /api/orgs/:org_id/members.
type inviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Invite adds a user to the organisation by email.
func (h *MemberHandler) Invite(w http.ResponseWriter, r *http.Request) {
	var req inviteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Role == "" {
		jsonError(w, "email and role are required", http.StatusBadRequest)
		return
	}
	if !rbac.ValidRole(req.Role) {
		jsonError(w, "invalid role", http.StatusBadRequest)
		return
	}
	if req.Role == "owner" {
		jsonError(w, "cannot assign owner role", http.StatusBadRequest)
		return
	}

	org := ctxutil.OrgFromCtx(r.Context())

	// Find user by email.
	var userID string
	err := h.DB.QueryRow(r.Context(),
		`SELECT id FROM users WHERE email = $1`, req.Email).Scan(&userID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	tag, err := h.DB.Exec(r.Context(),
		`INSERT INTO org_members (org_id, user_id, role, joined_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT DO NOTHING`,
		org.ID, userID, req.Role)
	if err != nil {
		jsonError(w, "could not add member", http.StatusInternalServerError)
		return
	}

	if tag.RowsAffected() == 0 {
		jsonError(w, "user is already a member", http.StatusConflict)
		return
	}

	jsonCreated(w, map[string]string{"status": "invited"})
}

// updateRoleRequest is the expected body for PUT /api/orgs/:org_id/members/:uid.
type updateRoleRequest struct {
	Role string `json:"role"`
}

// UpdateRole changes a member's role within the organisation.
func (h *MemberHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	var req updateRoleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if !rbac.ValidRole(req.Role) {
		jsonError(w, "invalid role", http.StatusBadRequest)
		return
	}
	if req.Role == "owner" {
		jsonError(w, "cannot assign owner role", http.StatusBadRequest)
		return
	}

	org := ctxutil.OrgFromCtx(r.Context())
	uid := chi.URLParam(r, "uid")

	tag, err := h.DB.Exec(r.Context(),
		`UPDATE org_members SET role = $1 WHERE org_id = $2 AND user_id = $3`,
		req.Role, org.ID, uid)
	if err != nil {
		jsonError(w, "could not update role", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, "member not found", http.StatusNotFound)
		return
	}

	jsonOK(w, map[string]string{"status": "updated"})
}

// Remove deletes a member from the organisation.
func (h *MemberHandler) Remove(w http.ResponseWriter, r *http.Request) {
	user := ctxutil.UserFromCtx(r.Context())
	uid := chi.URLParam(r, "uid")

	if uid == user.ID {
		jsonError(w, "cannot remove yourself", http.StatusBadRequest)
		return
	}

	org := ctxutil.OrgFromCtx(r.Context())

	tag, err := h.DB.Exec(r.Context(),
		`DELETE FROM org_members WHERE org_id = $1 AND user_id = $2`,
		org.ID, uid)
	if err != nil {
		jsonError(w, "could not remove member", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, "member not found", http.StatusNotFound)
		return
	}

	jsonOK(w, map[string]string{"status": "removed"})
}
