package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/ctxutil"
	"taskhub/internal/models"
)

// AgentHandler provides HTTP handlers for agent CRUD operations.
type AgentHandler struct {
	DB *pgxpool.Pool
}

// agentColumns is the SELECT column list for agents (order must match scanAgent).
const agentColumns = `id, org_id, name, version, description, endpoint,
	adapter_type, adapter_config, auth_type, auth_config,
	capabilities, input_schema, output_schema, config,
	status, created_at, updated_at`

// scanAgent scans a row into an Agent, handling JSONB columns via []byte intermediaries.
func scanAgent(scan func(dest ...any) error) (models.Agent, error) {
	var a models.Agent
	var adapterConfig, authConfig, inputSchema, outputSchema, agentConfig []byte
	var capabilitiesJSON []byte

	err := scan(
		&a.ID, &a.OrgID, &a.Name, &a.Version, &a.Description, &a.Endpoint,
		&a.AdapterType, &adapterConfig, &a.AuthType, &authConfig,
		&capabilitiesJSON, &inputSchema, &outputSchema, &agentConfig,
		&a.Status, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return a, err
	}

	a.AdapterConfig = adapterConfig
	a.AuthConfig = authConfig
	a.InputSchema = inputSchema
	a.OutputSchema = outputSchema
	a.Config = agentConfig

	if err := json.Unmarshal(capabilitiesJSON, &a.Capabilities); err != nil {
		a.Capabilities = []string{}
	}

	return a, nil
}

// List returns all agents for the current organization.
func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	org := ctxutil.OrgFromCtx(r.Context())

	rows, err := h.DB.Query(r.Context(),
		`SELECT `+agentColumns+`
		 FROM agents
		 WHERE org_id = $1
		 ORDER BY created_at DESC`, org.ID)
	if err != nil {
		jsonError(w, "query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	agents := make([]models.Agent, 0)
	for rows.Next() {
		a, err := scanAgent(rows.Scan)
		if err != nil {
			jsonError(w, "scan failed", http.StatusInternalServerError)
			return
		}
		agents = append(agents, a)
	}
	if err := rows.Err(); err != nil {
		jsonError(w, "rows iteration failed", http.StatusInternalServerError)
		return
	}

	jsonOK(w, agents)
}

// createAgentRequest is the expected body for POST /agents.
type createAgentRequest struct {
	Name          string          `json:"name"`
	Version       string          `json:"version"`
	Description   string          `json:"description"`
	Endpoint      string          `json:"endpoint"`
	AdapterType   string          `json:"adapter_type"`
	AdapterConfig json.RawMessage `json:"adapter_config,omitempty"`
	AuthType      string          `json:"auth_type"`
	AuthConfig    json.RawMessage `json:"auth_config,omitempty"`
	Capabilities  []string        `json:"capabilities"`
	InputSchema   json.RawMessage `json:"input_schema,omitempty"`
	OutputSchema  json.RawMessage `json:"output_schema,omitempty"`
	Config        json.RawMessage `json:"config,omitempty"`
}

// Create adds a new agent to the organization.
func (h *AgentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createAgentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Endpoint = strings.TrimSpace(req.Endpoint)

	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Endpoint == "" {
		jsonError(w, "endpoint is required", http.StatusBadRequest)
		return
	}
	if req.AdapterType != "http_poll" && req.AdapterType != "native" {
		jsonError(w, "adapter_type must be http_poll or native", http.StatusBadRequest)
		return
	}

	org := ctxutil.OrgFromCtx(r.Context())
	now := time.Now().UTC()

	// Marshal capabilities to JSON for DB storage.
	caps := req.Capabilities
	if caps == nil {
		caps = []string{}
	}
	capsJSON, err := json.Marshal(caps)
	if err != nil {
		jsonError(w, "invalid capabilities", http.StatusBadRequest)
		return
	}

	// Default JSONB fields to empty objects.
	adapterConfig := defaultJSON(req.AdapterConfig, "{}")
	authConfig := defaultJSON(req.AuthConfig, "{}")
	inputSchema := defaultJSON(req.InputSchema, "{}")
	outputSchema := defaultJSON(req.OutputSchema, "{}")
	config := defaultJSON(req.Config, "{}")

	version := req.Version
	if version == "" {
		version = "1.0.0"
	}

	agent := models.Agent{
		ID:            uuid.New().String(),
		OrgID:         org.ID,
		Name:          req.Name,
		Version:       version,
		Description:   req.Description,
		Endpoint:      req.Endpoint,
		AdapterType:   req.AdapterType,
		AdapterConfig: adapterConfig,
		AuthType:      req.AuthType,
		AuthConfig:    authConfig,
		Capabilities:  caps,
		InputSchema:   inputSchema,
		OutputSchema:  outputSchema,
		Config:        config,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_, err = h.DB.Exec(r.Context(),
		`INSERT INTO agents (id, org_id, name, version, description, endpoint,
			adapter_type, adapter_config, auth_type, auth_config,
			capabilities, input_schema, output_schema, config,
			status, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		agent.ID, agent.OrgID, agent.Name, agent.Version, agent.Description, agent.Endpoint,
		agent.AdapterType, adapterConfig, agent.AuthType, authConfig,
		capsJSON, inputSchema, outputSchema, config,
		agent.Status, agent.CreatedAt, agent.UpdatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			jsonError(w, "agent name already exists in this organization", http.StatusConflict)
			return
		}
		jsonError(w, "could not create agent", http.StatusInternalServerError)
		return
	}

	jsonCreated(w, agent)
}

// Get returns a single agent by ID within the current organization.
func (h *AgentHandler) Get(w http.ResponseWriter, r *http.Request) {
	org := ctxutil.OrgFromCtx(r.Context())
	id := chi.URLParam(r, "id")

	agent, err := scanAgent(
		h.DB.QueryRow(r.Context(),
			`SELECT `+agentColumns+`
			 FROM agents
			 WHERE id = $1 AND org_id = $2`, id, org.ID).Scan,
	)
	if err != nil {
		jsonError(w, "agent not found", http.StatusNotFound)
		return
	}

	jsonOK(w, agent)
}

// updateAgentRequest holds optional fields for PUT /agents/{id}.
type updateAgentRequest struct {
	Name          *string          `json:"name"`
	Version       *string          `json:"version"`
	Description   *string          `json:"description"`
	Endpoint      *string          `json:"endpoint"`
	AdapterType   *string          `json:"adapter_type"`
	AdapterConfig *json.RawMessage `json:"adapter_config"`
	AuthType      *string          `json:"auth_type"`
	AuthConfig    *json.RawMessage `json:"auth_config"`
	Capabilities  *[]string        `json:"capabilities"`
	InputSchema   *json.RawMessage `json:"input_schema"`
	OutputSchema  *json.RawMessage `json:"output_schema"`
	Config        *json.RawMessage `json:"config"`
	Status        *string          `json:"status"`
}

// Update modifies an existing agent's fields (partial update).
func (h *AgentHandler) Update(w http.ResponseWriter, r *http.Request) {
	org := ctxutil.OrgFromCtx(r.Context())
	id := chi.URLParam(r, "id")

	// Read current agent from DB first.
	agent, err := scanAgent(
		h.DB.QueryRow(r.Context(),
			`SELECT `+agentColumns+`
			 FROM agents
			 WHERE id = $1 AND org_id = $2`, id, org.ID).Scan,
	)
	if err != nil {
		jsonError(w, "agent not found", http.StatusNotFound)
		return
	}

	var req updateAgentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Apply changes.
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			jsonError(w, "name cannot be empty", http.StatusBadRequest)
			return
		}
		agent.Name = name
	}
	if req.Version != nil {
		agent.Version = *req.Version
	}
	if req.Description != nil {
		agent.Description = *req.Description
	}
	if req.Endpoint != nil {
		endpoint := strings.TrimSpace(*req.Endpoint)
		if endpoint == "" {
			jsonError(w, "endpoint cannot be empty", http.StatusBadRequest)
			return
		}
		agent.Endpoint = endpoint
	}
	if req.AdapterType != nil {
		if *req.AdapterType != "http_poll" && *req.AdapterType != "native" {
			jsonError(w, "adapter_type must be http_poll or native", http.StatusBadRequest)
			return
		}
		agent.AdapterType = *req.AdapterType
	}
	if req.AdapterConfig != nil {
		agent.AdapterConfig = *req.AdapterConfig
	}
	if req.AuthType != nil {
		agent.AuthType = *req.AuthType
	}
	if req.AuthConfig != nil {
		agent.AuthConfig = *req.AuthConfig
	}
	if req.Capabilities != nil {
		agent.Capabilities = *req.Capabilities
	}
	if req.InputSchema != nil {
		agent.InputSchema = *req.InputSchema
	}
	if req.OutputSchema != nil {
		agent.OutputSchema = *req.OutputSchema
	}
	if req.Config != nil {
		agent.Config = *req.Config
	}
	if req.Status != nil {
		agent.Status = *req.Status
	}

	// Marshal capabilities for DB.
	capsJSON, err := json.Marshal(agent.Capabilities)
	if err != nil {
		jsonError(w, "invalid capabilities", http.StatusBadRequest)
		return
	}

	_, err = h.DB.Exec(r.Context(),
		`UPDATE agents SET
			name = $1, version = $2, description = $3, endpoint = $4,
			adapter_type = $5, adapter_config = $6, auth_type = $7, auth_config = $8,
			capabilities = $9, input_schema = $10, output_schema = $11, config = $12,
			status = $13, updated_at = NOW()
		 WHERE id = $14 AND org_id = $15`,
		agent.Name, agent.Version, agent.Description, agent.Endpoint,
		agent.AdapterType, agent.AdapterConfig, agent.AuthType, agent.AuthConfig,
		capsJSON, agent.InputSchema, agent.OutputSchema, agent.Config,
		agent.Status, id, org.ID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			jsonError(w, "agent name already exists in this organization", http.StatusConflict)
			return
		}
		jsonError(w, "could not update agent", http.StatusInternalServerError)
		return
	}

	// Re-read updated_at from DB.
	_ = h.DB.QueryRow(r.Context(),
		`SELECT updated_at FROM agents WHERE id = $1`, id).Scan(&agent.UpdatedAt)

	jsonOK(w, agent)
}

// Delete removes an agent from the organization.
func (h *AgentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	org := ctxutil.OrgFromCtx(r.Context())
	id := chi.URLParam(r, "id")

	tag, err := h.DB.Exec(r.Context(),
		`DELETE FROM agents WHERE id = $1 AND org_id = $2`, id, org.ID)
	if err != nil {
		jsonError(w, "could not delete agent", http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		jsonError(w, "agent not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// healthcheckResponse is returned by the Healthcheck handler.
type healthcheckResponse struct {
	Status    int   `json:"status"`
	LatencyMs int64 `json:"latency_ms"`
}

// Healthcheck probes an agent's health endpoint.
func (h *AgentHandler) Healthcheck(w http.ResponseWriter, r *http.Request) {
	org := ctxutil.OrgFromCtx(r.Context())
	id := chi.URLParam(r, "id")

	agent, err := scanAgent(
		h.DB.QueryRow(r.Context(),
			`SELECT `+agentColumns+`
			 FROM agents
			 WHERE id = $1 AND org_id = $2`, id, org.ID).Scan,
	)
	if err != nil {
		jsonError(w, "agent not found", http.StatusNotFound)
		return
	}

	healthURL := strings.TrimRight(agent.Endpoint, "/") + "/health"

	client := &http.Client{Timeout: 5 * time.Second}
	start := time.Now()
	resp, err := client.Get(healthURL)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		jsonOK(w, healthcheckResponse{Status: 0, LatencyMs: latency})
		return
	}
	defer resp.Body.Close()

	jsonOK(w, healthcheckResponse{Status: resp.StatusCode, LatencyMs: latency})
}

// TestEndpoint tests connectivity to an arbitrary endpoint (no saved agent required).
// Used by the register form to validate before saving.
func (h *AgentHandler) TestEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	endpoint := strings.TrimSpace(req.Endpoint)
	if endpoint == "" {
		jsonError(w, "endpoint is required", http.StatusBadRequest)
		return
	}

	healthURL := strings.TrimRight(endpoint, "/") + "/health"
	client := &http.Client{Timeout: 5 * time.Second}
	start := time.Now()
	resp, err := client.Get(healthURL)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		jsonOK(w, healthcheckResponse{Status: 0, LatencyMs: latency})
		return
	}
	defer resp.Body.Close()

	jsonOK(w, healthcheckResponse{Status: resp.StatusCode, LatencyMs: latency})
}

// defaultJSON returns raw if it is non-nil and non-empty, otherwise returns the fallback as RawMessage.
func defaultJSON(raw json.RawMessage, fallback string) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(fallback)
	}
	return raw
}
