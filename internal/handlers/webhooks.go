package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/webhook"
)

// WebhookHandler provides CRUD for webhook configurations.
type WebhookHandler struct {
	DB     *pgxpool.Pool
	Sender *webhook.Sender
}

type webhookConfig struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	IsActive  bool     `json:"is_active"`
	Secret    string   `json:"secret"`
	CreatedAt string   `json:"created_at"`
}

// List handles GET /api/webhooks
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(),
		`SELECT id, name, url, events, is_active, secret, created_at
		 FROM webhook_configs ORDER BY created_at DESC`)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	configs := make([]webhookConfig, 0)
	for rows.Next() {
		var c webhookConfig
		var createdAt time.Time
		if err := rows.Scan(&c.ID, &c.Name, &c.URL, &c.Events, &c.IsActive, &c.Secret, &createdAt); err != nil {
			jsonError(w, "scan failed", http.StatusInternalServerError)
			return
		}
		c.CreatedAt = createdAt.Format(time.RFC3339)
		// Mask secret in list responses
		if c.Secret != "" {
			c.Secret = "***"
		}
		configs = append(configs, c)
	}
	if rows.Err() != nil {
		jsonError(w, "rows error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, configs)
}

type createWebhookRequest struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Secret string   `json:"secret"`
}

// Create handles POST /api/webhooks
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createWebhookRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.URL == "" {
		jsonError(w, "name and url are required", http.StatusBadRequest)
		return
	}
	if len(req.Events) == 0 {
		jsonError(w, "at least one event is required", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	c := webhookConfig{
		ID:        uuid.New().String(),
		Name:      req.Name,
		URL:       req.URL,
		Events:    req.Events,
		IsActive:  true,
		Secret:    req.Secret,
		CreatedAt: now.Format(time.RFC3339),
	}

	_, err := h.DB.Exec(r.Context(),
		`INSERT INTO webhook_configs (id, name, url, events, is_active, secret, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		c.ID, c.Name, c.URL, c.Events, c.IsActive, c.Secret, now)
	if err != nil {
		jsonError(w, "could not create webhook", http.StatusInternalServerError)
		return
	}

	// Mask secret in response
	if c.Secret != "" {
		c.Secret = "***"
	}
	jsonCreated(w, c)
}

type updateWebhookRequest struct {
	Name     *string   `json:"name"`
	URL      *string   `json:"url"`
	Events   *[]string `json:"events"`
	IsActive *bool     `json:"is_active"`
	Secret   *string   `json:"secret"`
}

// Update handles PUT /api/webhooks/{id}
func (h *WebhookHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateWebhookRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Name != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE webhook_configs SET name = $1 WHERE id = $2`, *req.Name, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	if req.URL != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE webhook_configs SET url = $1 WHERE id = $2`, *req.URL, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	if req.Events != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE webhook_configs SET events = $1 WHERE id = $2`, *req.Events, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	if req.IsActive != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE webhook_configs SET is_active = $1 WHERE id = $2`, *req.IsActive, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	if req.Secret != nil { // pragma: allowlist secret
		if _, err := h.DB.Exec(r.Context(), `UPDATE webhook_configs SET secret = $1 WHERE id = $2`, *req.Secret, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}

	// Read back
	var c webhookConfig
	var createdAt time.Time
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, name, url, events, is_active, secret, created_at
		 FROM webhook_configs WHERE id = $1`, id).
		Scan(&c.ID, &c.Name, &c.URL, &c.Events, &c.IsActive, &c.Secret, &createdAt)
	if err != nil {
		jsonError(w, "webhook not found", http.StatusNotFound)
		return
	}
	c.CreatedAt = createdAt.Format(time.RFC3339)
	if c.Secret != "" {
		c.Secret = "***"
	}
	jsonOK(w, c)
}

// Delete handles DELETE /api/webhooks/{id}
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tag, err := h.DB.Exec(r.Context(), `DELETE FROM webhook_configs WHERE id = $1`, id)
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

// Test handles POST /api/webhooks/{id}/test — sends a test payload.
func (h *WebhookHandler) Test(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var url, secret string
	err := h.DB.QueryRow(r.Context(),
		`SELECT url, secret FROM webhook_configs WHERE id = $1`, id).
		Scan(&url, &secret)
	if err != nil {
		jsonError(w, "webhook not found", http.StatusNotFound)
		return
	}

	payload := webhook.WebhookPayload{
		Event:     "test",
		TaskID:    "",
		SubtaskID: "",
		Data:      map[string]string{"message": "This is a test webhook from TaskHub"},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		jsonError(w, "marshal failed", http.StatusInternalServerError)
		return
	}

	// Deliver synchronously for test so we can report result
	result := h.Sender.DeliverTest(url, secret, body)
	jsonOK(w, result)
}
