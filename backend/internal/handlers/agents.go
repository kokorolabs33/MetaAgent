package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"taskhub/internal/models"
)

type AgentHandler struct {
	DB *sql.DB
}

func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.QueryContext(r.Context(),
		`SELECT id, name, description, system_prompt, capabilities, color, created_at FROM agents ORDER BY created_at`)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	agents := []models.Agent{}
	for rows.Next() {
		var a models.Agent
		var caps string
		if err := rows.Scan(&a.ID, &a.Name, &a.Description, &a.SystemPrompt, &caps, &a.Color, &a.CreatedAt); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		a.Capabilities = models.JSONToCapabilities(caps)
		agents = append(agents, a)
	}
	jsonOK(w, agents)
}

func (h *AgentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		SystemPrompt string   `json:"system_prompt"`
		Capabilities []string `json:"capabilities"`
		Color        string   `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	a := models.Agent{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Description:  req.Description,
		SystemPrompt: req.SystemPrompt,
		Capabilities: req.Capabilities,
		Color:        req.Color,
		CreatedAt:    time.Now(),
	}
	_, err := h.DB.ExecContext(r.Context(),
		`INSERT INTO agents (id, name, description, system_prompt, capabilities, color) VALUES ($1,$2,$3,$4,$5,$6)`,
		a.ID, a.Name, a.Description, a.SystemPrompt, models.CapabilitiesToJSON(a.Capabilities), a.Color,
	)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	jsonOK(w, a)
}

// Shared JSON helpers — used by all handlers in this package
func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
