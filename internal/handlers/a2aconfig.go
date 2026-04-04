package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/a2a"
)

// A2AConfigHandler serves the AgentCard and manages A2A server configuration.
type A2AConfigHandler struct {
	DB         *pgxpool.Pool
	Aggregator *a2a.Aggregator
	BaseURL    string
}

// ServeAgentCard handles GET /.well-known/agent-card.json (public, no auth).
func (h *A2AConfigHandler) ServeAgentCard(w http.ResponseWriter, r *http.Request) {
	var enabled bool
	err := h.DB.QueryRow(r.Context(),
		`SELECT enabled FROM a2a_server_config WHERE id = 1`).Scan(&enabled)
	if err != nil || !enabled {
		jsonError(w, "A2A server is not enabled", http.StatusNotFound)
		return
	}

	data, etag, err := h.Aggregator.GetCard(r.Context(), h.BaseURL)
	if err != nil {
		jsonError(w, "could not generate agent card", http.StatusInternalServerError)
		return
	}

	if match := r.Header.Get("If-None-Match"); match != "" {
		if strings.Contains(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=60")
	_, _ = w.Write(data)
}

// GetConfig handles GET /api/a2a-config.
func (h *A2AConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	var (
		enabled       bool
		nameOverride  *string
		descOverride  *string
		secScheme     []byte
		aggCard       []byte
		cardUpdatedAt *time.Time
	)

	err := h.DB.QueryRow(r.Context(),
		`SELECT enabled, name_override, description_override, security_scheme, aggregated_card, card_updated_at
		 FROM a2a_server_config WHERE id = 1`).
		Scan(&enabled, &nameOverride, &descOverride, &secScheme, &aggCard, &cardUpdatedAt)
	if err != nil {
		jsonError(w, "could not load config", http.StatusInternalServerError)
		return
	}

	var cardUpdatedAtStr *string
	if cardUpdatedAt != nil {
		s := cardUpdatedAt.Format(time.RFC3339)
		cardUpdatedAtStr = &s
	}

	cfg := struct {
		Enabled             bool            `json:"enabled"`
		NameOverride        *string         `json:"name_override"`
		DescriptionOverride *string         `json:"description_override"`
		SecurityScheme      json.RawMessage `json:"security_scheme"`
		AggregatedCard      json.RawMessage `json:"aggregated_card"`
		CardUpdatedAt       *string         `json:"card_updated_at"`
	}{
		Enabled:             enabled,
		NameOverride:        nameOverride,
		DescriptionOverride: descOverride,
		SecurityScheme:      json.RawMessage(secScheme),
		AggregatedCard:      json.RawMessage(aggCard),
		CardUpdatedAt:       cardUpdatedAtStr,
	}

	jsonOK(w, cfg)
}

type updateA2AConfigRequest struct {
	Enabled             *bool   `json:"enabled"`
	NameOverride        *string `json:"name_override"`
	DescriptionOverride *string `json:"description_override"`
}

// UpdateConfig handles PUT /api/a2a-config.
func (h *A2AConfigHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req updateA2AConfigRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	if req.Enabled != nil {
		_, _ = h.DB.Exec(ctx,
			`UPDATE a2a_server_config SET enabled = $1 WHERE id = 1`, *req.Enabled)
	}
	if req.NameOverride != nil {
		_, _ = h.DB.Exec(ctx,
			`UPDATE a2a_server_config SET name_override = $1 WHERE id = 1`, *req.NameOverride)
	}
	if req.DescriptionOverride != nil {
		_, _ = h.DB.Exec(ctx,
			`UPDATE a2a_server_config SET description_override = $1 WHERE id = 1`, *req.DescriptionOverride)
	}

	h.Aggregator.Invalidate()
	h.GetConfig(w, r)
}

// RefreshCard handles POST /api/a2a-config/refresh-card.
func (h *A2AConfigHandler) RefreshCard(w http.ResponseWriter, r *http.Request) {
	data, etag, err := h.Aggregator.Rebuild(r.Context(), h.BaseURL)
	if err != nil {
		jsonError(w, "could not rebuild card", http.StatusInternalServerError)
		return
	}

	jsonOK(w, map[string]any{
		"etag": etag,
		"card": json.RawMessage(data),
	})
}
