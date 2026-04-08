package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/ctxutil"
	"taskhub/internal/executor"
	"taskhub/internal/models"
	"taskhub/internal/webhook"
)

// InboundWebhookHandler provides CRUD for inbound webhook configurations
// and the public ingestion endpoint for receiving external webhook deliveries.
type InboundWebhookHandler struct {
	DB       *pgxpool.Pool
	Executor *executor.DAGExecutor
}

type inboundWebhookResponse struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Provider       string `json:"provider"`
	Secret         string `json:"secret"`          // pragma: allowlist secret
	PreviousSecret string `json:"previous_secret"` // pragma: allowlist secret
	IsActive       bool   `json:"is_active"`
	CreatedBy      string `json:"created_by"`
	EndpointURL    string `json:"endpoint_url"`
	CreatedAt      string `json:"created_at"`
}

func maskSecret(s string) string { // pragma: allowlist secret
	if s != "" {
		return "***"
	}
	return ""
}

func toResponse(wh models.InboundWebhook, masked bool) inboundWebhookResponse {
	resp := inboundWebhookResponse{
		ID:          wh.ID,
		Name:        wh.Name,
		Provider:    wh.Provider,
		Secret:      wh.Secret, // pragma: allowlist secret
		IsActive:    wh.IsActive,
		CreatedBy:   wh.CreatedBy,
		EndpointURL: "/api/webhooks/inbound/" + wh.ID,
		CreatedAt:   wh.CreatedAt.Format(time.RFC3339),
	}
	if masked {
		resp.Secret = maskSecret(wh.Secret)                 // pragma: allowlist secret
		resp.PreviousSecret = maskSecret(wh.PreviousSecret) // pragma: allowlist secret
	} else {
		resp.PreviousSecret = maskSecret(wh.PreviousSecret) // pragma: allowlist secret
	}
	return resp
}

func generateSecret() (string, error) { // pragma: allowlist secret
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate secret: %w", err) // pragma: allowlist secret
	}
	return hex.EncodeToString(b), nil
}

// List handles GET /api/inbound-webhooks
func (h *InboundWebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(),
		`SELECT id, name, provider, secret, previous_secret, is_active, created_by, created_at
		 FROM inbound_webhooks ORDER BY created_at DESC`)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	webhooks := make([]inboundWebhookResponse, 0)
	for rows.Next() {
		var wh models.InboundWebhook
		if err := rows.Scan(&wh.ID, &wh.Name, &wh.Provider, &wh.Secret, &wh.PreviousSecret, &wh.IsActive, &wh.CreatedBy, &wh.CreatedAt); err != nil {
			jsonError(w, "scan failed", http.StatusInternalServerError)
			return
		}
		webhooks = append(webhooks, toResponse(wh, true))
	}
	if rows.Err() != nil {
		jsonError(w, "rows error", http.StatusInternalServerError)
		return
	}
	jsonOK(w, webhooks)
}

type createInboundWebhookRequest struct {
	Name     string `json:"name"`
	Provider string `json:"provider"` // github, slack, generic
}

// Create handles POST /api/inbound-webhooks
func (h *InboundWebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createInboundWebhookRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = "generic"
	}
	if provider != "github" && provider != "slack" && provider != "generic" {
		jsonError(w, "provider must be github, slack, or generic", http.StatusBadRequest)
		return
	}

	secret, err := generateSecret() // pragma: allowlist secret
	if err != nil {
		jsonError(w, "could not generate secret", http.StatusInternalServerError) // pragma: allowlist secret
		return
	}

	user := ctxutil.UserFromCtx(r.Context())
	now := time.Now().UTC()
	wh := models.InboundWebhook{
		ID:        uuid.New().String(),
		Name:      strings.TrimSpace(req.Name),
		Provider:  provider,
		Secret:    secret, // pragma: allowlist secret
		IsActive:  true,
		CreatedBy: user.ID,
		CreatedAt: now,
	}

	_, err = h.DB.Exec(r.Context(),
		`INSERT INTO inbound_webhooks (id, name, provider, secret, is_active, created_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		wh.ID, wh.Name, wh.Provider, wh.Secret, wh.IsActive, wh.CreatedBy, wh.CreatedAt)
	if err != nil {
		jsonError(w, "could not create webhook", http.StatusInternalServerError)
		return
	}

	// Show secret once on create (not masked)
	jsonCreated(w, toResponse(wh, false))
}

// Get handles GET /api/inbound-webhooks/{id}
func (h *InboundWebhookHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var wh models.InboundWebhook
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, name, provider, secret, previous_secret, is_active, created_by, created_at
		 FROM inbound_webhooks WHERE id = $1`, id).
		Scan(&wh.ID, &wh.Name, &wh.Provider, &wh.Secret, &wh.PreviousSecret, &wh.IsActive, &wh.CreatedBy, &wh.CreatedAt)
	if err != nil {
		jsonError(w, "webhook not found", http.StatusNotFound)
		return
	}
	jsonOK(w, toResponse(wh, true))
}

type updateInboundWebhookRequest struct {
	Name         *string `json:"name"`
	IsActive     *bool   `json:"is_active"`
	RotateSecret *bool   `json:"rotate_secret"` // pragma: allowlist secret
}

// Update handles PUT /api/inbound-webhooks/{id}
func (h *InboundWebhookHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req updateInboundWebhookRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}

	if req.Name != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE inbound_webhooks SET name = $1 WHERE id = $2`, strings.TrimSpace(*req.Name), id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	if req.IsActive != nil {
		if _, err := h.DB.Exec(r.Context(), `UPDATE inbound_webhooks SET is_active = $1 WHERE id = $2`, *req.IsActive, id); err != nil {
			jsonError(w, "update failed", http.StatusInternalServerError)
			return
		}
	}
	if req.RotateSecret != nil && *req.RotateSecret { // pragma: allowlist secret
		// D-03: move current secret to previous_secret, generate new secret
		newSecret, err := generateSecret() // pragma: allowlist secret
		if err != nil {
			jsonError(w, "could not generate new secret", http.StatusInternalServerError) // pragma: allowlist secret
			return
		}
		_, err = h.DB.Exec(r.Context(),
			`UPDATE inbound_webhooks SET previous_secret = secret, secret = $1 WHERE id = $2`, // pragma: allowlist secret
			newSecret, id)
		if err != nil {
			jsonError(w, "rotate failed", http.StatusInternalServerError)
			return
		}
	}

	// Read back
	var wh models.InboundWebhook
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, name, provider, secret, previous_secret, is_active, created_by, created_at
		 FROM inbound_webhooks WHERE id = $1`, id).
		Scan(&wh.ID, &wh.Name, &wh.Provider, &wh.Secret, &wh.PreviousSecret, &wh.IsActive, &wh.CreatedBy, &wh.CreatedAt)
	if err != nil {
		jsonError(w, "webhook not found", http.StatusNotFound)
		return
	}
	jsonOK(w, toResponse(wh, true))
}

// Delete handles DELETE /api/inbound-webhooks/{id}
func (h *InboundWebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tag, err := h.DB.Exec(r.Context(), `DELETE FROM inbound_webhooks WHERE id = $1`, id)
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

// Ingest handles POST /api/webhooks/inbound/{id} -- the public ingestion endpoint.
// This is outside auth middleware; authentication is purely via HMAC signature (D-02).
func (h *InboundWebhookHandler) Ingest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Look up webhook
	var wh models.InboundWebhook
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, name, provider, secret, previous_secret, is_active, created_by, created_at
		 FROM inbound_webhooks WHERE id = $1`, id).
		Scan(&wh.ID, &wh.Name, &wh.Provider, &wh.Secret, &wh.PreviousSecret, &wh.IsActive, &wh.CreatedBy, &wh.CreatedAt)
	if err != nil || !wh.IsActive {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}

	// Read body with 1MB limit (T-10-05)
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		jsonError(w, "body too large or unreadable", http.StatusBadRequest)
		return
	}

	// Extract signature based on provider
	signature := extractSignature(r, wh.Provider)

	// Verify HMAC signature (T-10-01, T-10-02)
	if !webhook.VerifyHMAC(body, signature, wh.Secret, wh.PreviousSecret) { // pragma: allowlist secret
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Idempotency check (T-10-07, D-13, D-14)
	deliveryID := extractDeliveryID(r, wh.Provider)
	if deliveryID != "" {
		tag, err := h.DB.Exec(r.Context(),
			`INSERT INTO webhook_deliveries (delivery_id, webhook_id) VALUES ($1, $2) ON CONFLICT (delivery_id) DO NOTHING`,
			deliveryID, wh.ID)
		if err != nil {
			jsonError(w, "delivery check failed", http.StatusInternalServerError)
			return
		}
		if tag.RowsAffected() == 0 {
			// Duplicate delivery
			jsonOK(w, map[string]string{"status": "duplicate", "message": "already processed"})
			return
		}
	}

	// Slack URL verification challenge
	if wh.Provider == "slack" {
		var challenge struct {
			Type      string `json:"type"`
			Challenge string `json:"challenge"`
		}
		if json.Unmarshal(body, &challenge) == nil && challenge.Type == "url_verification" {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(challenge.Challenge))
			return
		}
	}

	// Parse payload based on provider
	var parsed *webhook.ParsedPayload
	switch wh.Provider {
	case "github":
		parsed, err = webhook.ParseGitHubPayload(r.Header.Get("X-GitHub-Event"), body)
	case "slack":
		parsed, err = webhook.ParseSlackPayload(body)
		// Handle Slack challenge that came through the parser
		if parsed != nil && parsed.Title == "__slack_challenge__" {
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(parsed.Metadata["challenge"]))
			return
		}
	default:
		parsed, err = webhook.ParseGenericPayload(body)
	}

	if err != nil {
		log.Printf("webhook ingest: parse error for %s/%s: %v", wh.Provider, wh.ID, err)
		jsonError(w, "parse error", http.StatusBadRequest)
		return
	}

	// Nil payload means unsupported event type -- ack without creating task (D-05)
	if parsed == nil {
		jsonOK(w, map[string]string{"status": "ignored"})
		return
	}

	// Create task (mirror TaskHandler.Create pattern)
	now := time.Now().UTC()
	task := models.Task{
		ID:          uuid.New().String(),
		Title:       parsed.Title,
		Description: parsed.Description,
		Status:      "pending",
		CreatedBy:   wh.CreatedBy,
		CreatedAt:   now,
	}

	_, err = h.DB.Exec(r.Context(),
		`INSERT INTO tasks (id, title, description, status, created_by, replan_count, created_at)
		 VALUES ($1, $2, $3, $4, $5, 0, $6)`,
		task.ID, task.Title, task.Description, task.Status, task.CreatedBy, task.CreatedAt)
	if err != nil {
		jsonError(w, "could not create task", http.StatusInternalServerError)
		return
	}

	// Spawn executor in background (ack-first pattern, pitfall 9)
	go func() { //nolint:contextcheck // context.Background is intentional as execution outlives the HTTP request
		if execErr := h.Executor.Execute(context.Background(), task); execErr != nil {
			log.Printf("executor: webhook task %s failed: %v", task.ID, execErr)
		}
	}()

	// Return 200 OK immediately
	jsonOK(w, map[string]string{"status": "accepted", "task_id": task.ID})
}

// extractSignature gets the HMAC signature from the appropriate provider-specific header.
func extractSignature(r *http.Request, provider string) string {
	switch provider {
	case "github":
		return r.Header.Get("X-Hub-Signature-256")
	case "slack":
		// Slack uses a different signing scheme: v0:timestamp:body
		// For simplicity, we support X-Slack-Signature header directly
		return r.Header.Get("X-Slack-Signature")
	default:
		return r.Header.Get("X-Webhook-Signature")
	}
}

// extractDeliveryID gets the delivery ID from provider-specific headers for idempotency.
func extractDeliveryID(r *http.Request, provider string) string {
	switch provider {
	case "github":
		return r.Header.Get("X-GitHub-Delivery")
	case "slack":
		// Use request timestamp as delivery ID for Slack
		ts := r.Header.Get("X-Slack-Request-Timestamp")
		if ts != "" {
			return "slack-" + ts
		}
		return ""
	default:
		return r.Header.Get("X-Webhook-Delivery-ID")
	}
}

// CleanupDeliveries removes webhook delivery records older than 7 days (D-15).
func (h *InboundWebhookHandler) CleanupDeliveries(ctx context.Context) {
	tag, err := h.DB.Exec(ctx,
		`DELETE FROM webhook_deliveries WHERE created_at < NOW() - INTERVAL '7 days'`)
	if err != nil {
		log.Printf("webhook cleanup: %v", err)
		return
	}
	if tag.RowsAffected() > 0 {
		log.Printf("webhook cleanup: removed %d old deliveries", tag.RowsAffected())
	}
}

// StartCleanupLoop runs delivery cleanup on startup and then every 24 hours.
func (h *InboundWebhookHandler) StartCleanupLoop(ctx context.Context) {
	// Run once immediately
	h.CleanupDeliveries(ctx)

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			h.CleanupDeliveries(ctx)
		}
	}
}
