package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/a2a"
	"taskhub/internal/ctxutil"
	"taskhub/internal/models"
)

// AgentHandler provides HTTP handlers for agent CRUD operations.
type AgentHandler struct {
	DB       *pgxpool.Pool
	Resolver *a2a.Resolver
}

// agentColumns is the SELECT column list for agents (order must match scanAgent).
const agentColumns = `id, org_id, name, version, description, endpoint,
	agent_card_url, agent_card, card_fetched_at,
	capabilities, skills,
	status, created_at, updated_at`

// scanAgent scans a row into an Agent, handling JSONB columns via []byte intermediaries.
func scanAgent(scan func(dest ...any) error) (models.Agent, error) {
	var a models.Agent
	var agentCard, capabilitiesJSON, skillsJSON []byte

	err := scan(
		&a.ID, &a.OrgID, &a.Name, &a.Version, &a.Description, &a.Endpoint,
		&a.AgentCardURL, &agentCard, &a.CardFetchedAt,
		&capabilitiesJSON, &skillsJSON,
		&a.Status, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return a, err
	}

	if agentCard != nil {
		a.AgentCard = json.RawMessage(agentCard)
	}
	if skillsJSON != nil {
		a.Skills = json.RawMessage(skillsJSON)
	}

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
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Endpoint     string   `json:"endpoint"`
	Capabilities []string `json:"capabilities"`
}

// Create adds a new agent to the organization.
// It fetches the AgentCard from the endpoint to populate metadata.
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

	org := ctxutil.OrgFromCtx(r.Context())
	now := time.Now().UTC()

	// Try to discover agent card from endpoint
	var agentCardURL string
	var agentCard json.RawMessage
	var skills json.RawMessage
	var cardFetchedAt *time.Time

	discovered, err := h.Resolver.Discover(r.Context(), req.Endpoint)
	if err == nil {
		agentCardURL = strings.TrimRight(req.Endpoint, "/") + "/.well-known/agent-card.json"
		agentCard = discovered.RawCard
		fetchTime := now
		cardFetchedAt = &fetchTime

		skillsJSON, _ := json.Marshal(discovered.Skills)
		skills = json.RawMessage(skillsJSON)

		// Use card values as defaults if not provided
		if req.Name == "" && discovered.Name != "" {
			req.Name = discovered.Name
		}
		if req.Description == "" && discovered.Description != "" {
			req.Description = discovered.Description
		}
		if req.Version == "" && discovered.Version != "" {
			req.Version = discovered.Version
		}
	}

	if skills == nil {
		skills = json.RawMessage("[]")
	}
	if agentCard == nil {
		agentCard = json.RawMessage("{}")
	}

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
		AgentCardURL:  agentCardURL,
		AgentCard:     agentCard,
		CardFetchedAt: cardFetchedAt,
		Capabilities:  caps,
		Skills:        skills,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	_, err = h.DB.Exec(r.Context(),
		`INSERT INTO agents (id, org_id, name, version, description, endpoint,
			agent_card_url, agent_card, card_fetched_at,
			capabilities, skills,
			status, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		agent.ID, agent.OrgID, agent.Name, agent.Version, agent.Description, agent.Endpoint,
		agent.AgentCardURL, agentCard, cardFetchedAt,
		capsJSON, skills,
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
	Name         *string   `json:"name"`
	Version      *string   `json:"version"`
	Description  *string   `json:"description"`
	Endpoint     *string   `json:"endpoint"`
	Capabilities *[]string `json:"capabilities"`
	Status       *string   `json:"status"`
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

	// Track whether endpoint is changing for agent card refresh.
	oldEndpoint := agent.Endpoint

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
	if req.Capabilities != nil {
		agent.Capabilities = *req.Capabilities
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

	// If endpoint changed, re-discover the agent card.
	endpointChanged := agent.Endpoint != oldEndpoint
	agentCard := agent.AgentCard
	agentCardURL := agent.AgentCardURL
	skills := agent.Skills
	var cardFetchedAt *time.Time

	if endpointChanged {
		discovered, discoverErr := h.Resolver.Discover(r.Context(), agent.Endpoint)
		if discoverErr != nil {
			log.Printf("agents: re-discover agent card for %s after endpoint change: %v", id, discoverErr)
			// Still allow the update; just clear stale card data.
			agentCard = json.RawMessage("{}")
			agentCardURL = ""
			skills = json.RawMessage("[]")
			cardFetchedAt = nil
		} else {
			agentCard = discovered.RawCard
			agentCardURL = strings.TrimRight(agent.Endpoint, "/") + "/.well-known/agent-card.json"
			fetchTime := time.Now().UTC()
			cardFetchedAt = &fetchTime
			skillsJSON, _ := json.Marshal(discovered.Skills)
			skills = json.RawMessage(skillsJSON)
		}
		agent.AgentCard = agentCard
		agent.AgentCardURL = agentCardURL
		agent.Skills = skills
		agent.CardFetchedAt = cardFetchedAt
	}

	if endpointChanged {
		_, err = h.DB.Exec(r.Context(),
			`UPDATE agents SET
				name = $1, version = $2, description = $3, endpoint = $4,
				capabilities = $5, status = $6,
				agent_card_url = $7, agent_card = $8, card_fetched_at = $9, skills = $10,
				updated_at = NOW()
			 WHERE id = $11 AND org_id = $12`,
			agent.Name, agent.Version, agent.Description, agent.Endpoint,
			capsJSON, agent.Status,
			agentCardURL, agentCard, cardFetchedAt, skills,
			id, org.ID)
	} else {
		_, err = h.DB.Exec(r.Context(),
			`UPDATE agents SET
				name = $1, version = $2, description = $3, endpoint = $4,
				capabilities = $5, status = $6, updated_at = NOW()
			 WHERE id = $7 AND org_id = $8`,
			agent.Name, agent.Version, agent.Description, agent.Endpoint,
			capsJSON, agent.Status, id, org.ID)
	}
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

// Healthcheck probes an agent's health by fetching its AgentCard.
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

	start := time.Now()
	_, discoverErr := h.Resolver.Discover(r.Context(), agent.Endpoint)
	latency := time.Since(start).Milliseconds()

	if discoverErr != nil {
		jsonOK(w, healthcheckResponse{Status: 0, LatencyMs: latency})
		return
	}

	jsonOK(w, healthcheckResponse{Status: 200, LatencyMs: latency})
}

// TestEndpoint tests connectivity to an arbitrary endpoint by trying AgentCard discovery.
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

	start := time.Now()
	_, discoverErr := h.Resolver.Discover(r.Context(), endpoint)
	latency := time.Since(start).Milliseconds()

	if discoverErr != nil {
		jsonOK(w, healthcheckResponse{Status: 0, LatencyMs: latency})
		return
	}

	jsonOK(w, healthcheckResponse{Status: 200, LatencyMs: latency})
}

// Discover handles POST /agents/discover.
// Fetches the AgentCard from the given URL and returns the discovered agent info.
func (h *AgentHandler) Discover(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	url := strings.TrimSpace(req.URL)
	if url == "" {
		jsonError(w, "url is required", http.StatusBadRequest)
		return
	}

	discovered, err := h.Resolver.Discover(r.Context(), url)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}

	jsonOK(w, discovered)
}

// defaultJSON returns raw if it is non-nil and non-empty, otherwise returns the fallback as RawMessage.
func defaultJSON(raw json.RawMessage, fallback string) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(fallback)
	}
	return raw
}
