package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/models"
)

// PolicyHandler provides CRUD operations for policies.
type PolicyHandler struct {
	DB *pgxpool.Pool
}

// List handles GET /api/policies
func (h *PolicyHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(),
		`SELECT id, name, rules, priority, is_active, created_at
		 FROM policies ORDER BY priority DESC, created_at DESC`)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	policies := make([]models.Policy, 0)
	for rows.Next() {
		var p models.Policy
		var rules []byte
		if err := rows.Scan(&p.ID, &p.Name, &rules, &p.Priority, &p.IsActive, &p.CreatedAt); err != nil {
			jsonError(w, "scan failed", http.StatusInternalServerError)
			return
		}
		if rules != nil {
			p.Rules = json.RawMessage(rules)
		}
		policies = append(policies, p)
	}
	if rows.Err() != nil {
		jsonError(w, "rows error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, policies)
}

type createPolicyRequest struct {
	Name     string          `json:"name"`
	Rules    json.RawMessage `json:"rules"`
	Priority int             `json:"priority"`
}

// Create handles POST /api/policies
func (h *PolicyHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createPolicyRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Rules == nil {
		req.Rules = json.RawMessage(`{}`)
	}

	p := models.Policy{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Rules:     req.Rules,
		Priority:  req.Priority,
		IsActive:  true,
		CreatedAt: time.Now().UTC(),
	}

	_, err := h.DB.Exec(r.Context(),
		`INSERT INTO policies (id, name, rules, priority, is_active, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		p.ID, p.Name, p.Rules, p.Priority, p.IsActive, p.CreatedAt)
	if err != nil {
		jsonError(w, "could not create policy", http.StatusInternalServerError)
		return
	}
	jsonCreated(w, p)
}

type updatePolicyRequest struct {
	Name     *string          `json:"name"`
	Rules    *json.RawMessage `json:"rules"`
	Priority *int             `json:"priority"`
	IsActive *bool            `json:"is_active"`
}

// Update handles PUT /api/policies/{id}
func (h *PolicyHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updatePolicyRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Name != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE policies SET name = $1 WHERE id = $2`, *req.Name, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	if req.Rules != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE policies SET rules = $1 WHERE id = $2`, *req.Rules, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	if req.Priority != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE policies SET priority = $1 WHERE id = $2`, *req.Priority, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	if req.IsActive != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE policies SET is_active = $1 WHERE id = $2`, *req.IsActive, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}

	// Read back and return
	var p models.Policy
	var rules []byte
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, name, rules, priority, is_active, created_at FROM policies WHERE id = $1`, id).
		Scan(&p.ID, &p.Name, &rules, &p.Priority, &p.IsActive, &p.CreatedAt)
	if err != nil {
		jsonError(w, "policy not found", http.StatusNotFound)
		return
	}
	p.Rules = json.RawMessage(rules)
	jsonOK(w, p)
}

// Delete handles DELETE /api/policies/{id}
func (h *PolicyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tag, err := h.DB.Exec(r.Context(), `DELETE FROM policies WHERE id = $1`, id)
	if err != nil {
		jsonError(w, "could not delete", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
