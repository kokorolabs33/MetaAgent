# Plan 2: A2A Server + AgentCard Aggregation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make TaskHub expose itself as an A2A agent — serve an aggregated AgentCard and accept tasks via JSON-RPC 2.0.

**Architecture:** Add A2A server-side components to the existing `internal/a2a/` package. The server receives external tasks via JSON-RPC, creates internal tasks, runs the existing decomposition/execution pipeline, and returns results as A2A artifacts. AgentCard is auto-aggregated from all connected agents' skills.

**Tech Stack:** Go (chi router, pgx/v5), JSON-RPC 2.0

**Key constraint:** Every task must end with `go build ./...` passing.

---

### Task 1: AgentCard aggregator

**Files:**
- Create: `internal/a2a/aggregator.go`

Build a service that reads all active/online agents from the DB, collects their skills, and produces a single aggregated AgentCard JSON with ETag.

```go
// aggregator.go
package a2a

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AggregatedCard is the full AgentCard that TaskHub exposes.
type AggregatedCard struct {
	Name                string         `json:"name"`
	Description         string         `json:"description"`
	Version             string         `json:"version"`
	SupportedInterfaces []CardInterface `json:"supportedInterfaces"`
	Capabilities        CardCapability `json:"capabilities"`
	Skills              []CardSkill    `json:"skills"`
}

type CardInterface struct {
	URL             string `json:"url"`
	ProtocolBinding string `json:"protocolBinding"`
}

// Aggregator builds and caches the aggregated AgentCard.
type Aggregator struct {
	DB *pgxpool.Pool

	mu       sync.RWMutex
	card     *AggregatedCard
	cardJSON []byte
	etag     string
	builtAt  time.Time
}

// NewAggregator creates an Aggregator.
func NewAggregator(db *pgxpool.Pool) *Aggregator {
	return &Aggregator{DB: db}
}

// GetCard returns the cached card JSON and ETag. Rebuilds if stale or empty.
func (a *Aggregator) GetCard(ctx context.Context, baseURL string) ([]byte, string, error) {
	a.mu.RLock()
	if a.cardJSON != nil {
		data, etag := a.cardJSON, a.etag
		a.mu.RUnlock()
		return data, etag, nil
	}
	a.mu.RUnlock()

	return a.Rebuild(ctx, baseURL)
}

// Rebuild rebuilds the aggregated card from DB.
func (a *Aggregator) Rebuild(ctx context.Context, baseURL string) ([]byte, string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Load a2a_server_config for name/description overrides
	var nameOverride, descOverride *string
	_ = a.DB.QueryRow(ctx,
		`SELECT name_override, description_override FROM a2a_server_config WHERE id = 1`).
		Scan(&nameOverride, &descOverride)

	// Load all online agents' skills
	rows, err := a.DB.Query(ctx,
		`SELECT skills FROM agents WHERE status = 'active' AND is_online = true AND skills IS NOT NULL AND skills != '[]'::jsonb`)
	if err != nil {
		return nil, "", fmt.Errorf("query agent skills: %w", err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	var allSkills []CardSkill

	for rows.Next() {
		var skillsJSON []byte
		if err := rows.Scan(&skillsJSON); err != nil {
			continue
		}
		var skills []CardSkill
		if err := json.Unmarshal(skillsJSON, &skills); err != nil {
			continue
		}
		for _, s := range skills {
			key := s.ID
			if key == "" {
				key = s.Name
			}
			if !seen[key] {
				seen[key] = true
				allSkills = append(allSkills, s)
			}
		}
	}

	name := "TaskHub Agent"
	if nameOverride != nil && *nameOverride != "" {
		name = *nameOverride
	}

	desc := fmt.Sprintf("Orchestration agent with %d skills from connected agents", len(allSkills))
	if descOverride != nil && *descOverride != "" {
		desc = *descOverride
	}

	card := &AggregatedCard{
		Name:        name,
		Description: desc,
		Version:     "1.0.0",
		SupportedInterfaces: []CardInterface{
			{URL: baseURL + "/a2a", ProtocolBinding: "HTTP+JSON"},
		},
		Capabilities: CardCapability{Streaming: false, PushNotifications: false},
		Skills:       allSkills,
	}

	data, err := json.Marshal(card)
	if err != nil {
		return nil, "", fmt.Errorf("marshal card: %w", err)
	}

	hash := sha256.Sum256(data)
	etag := fmt.Sprintf(`"%x"`, hash[:8])

	a.card = card
	a.cardJSON = data
	a.etag = etag
	a.builtAt = time.Now()

	// Also update the cached card in DB
	_, _ = a.DB.Exec(ctx,
		`UPDATE a2a_server_config SET aggregated_card = $1, card_updated_at = NOW() WHERE id = 1`,
		data)

	return data, etag, nil
}

// Invalidate clears the cache so next GetCard triggers a rebuild.
func (a *Aggregator) Invalidate() {
	a.mu.Lock()
	a.cardJSON = nil
	a.mu.Unlock()
}
```

- [ ] **Step 1: Create the file above**
- [ ] **Step 2: `go build ./...` — verify it compiles**
- [ ] **Step 3: Commit**: `feat(a2a): add AgentCard aggregator service`

---

### Task 2: A2A JSON-RPC server handler

**Files:**
- Create: `internal/a2a/server.go`

Build the JSON-RPC 2.0 server that handles `tasks/send`, `tasks/get`, `tasks/cancel`.

```go
// server.go
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Server handles incoming A2A JSON-RPC requests.
type Server struct {
	DB         *pgxpool.Pool
	Aggregator *Aggregator
	BaseURL    string

	// TaskExecutor is called to execute a newly created task.
	// The server creates the DB record; this function starts execution.
	TaskExecutor func(ctx context.Context, taskID string) error
}

// a2aServerTask maps internal task state to A2A protocol task.
type a2aServerTask struct {
	ID        string           `json:"id"`
	Status    a2aStatus        `json:"status"`
	Artifacts []artifact       `json:"artifacts,omitempty"`
}

// HandleJSONRPC is the HTTP handler for POST /a2a.
func (s *Server) HandleJSONRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeRPCError(w, "", -32700, "parse error")
		return
	}

	var req jsonRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeRPCError(w, "", -32700, "parse error")
		return
	}

	if req.JSONRPC != "2.0" {
		s.writeRPCError(w, req.ID, -32600, "invalid request: jsonrpc must be 2.0")
		return
	}

	var result any
	var rpcErr *jsonRPCError

	switch req.Method {
	case "tasks/send", "message/send":
		result, rpcErr = s.handleTasksSend(r.Context(), req.Params)
	case "tasks/get":
		result, rpcErr = s.handleTasksGet(r.Context(), req.Params)
	case "tasks/cancel":
		result, rpcErr = s.handleTasksCancel(r.Context(), req.Params)
	default:
		rpcErr = &jsonRPCError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)}
	}

	if rpcErr != nil {
		s.writeRPCError(w, req.ID, rpcErr.Code, rpcErr.Message)
		return
	}

	s.writeRPCResult(w, req.ID, result)
}

// handleTasksSend creates an internal task and starts execution.
func (s *Server) handleTasksSend(ctx context.Context, params any) (any, *jsonRPCError) {
	// Parse params
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "invalid params"}
	}

	var sp sendMessageParams
	if err := json.Unmarshal(paramsJSON, &sp); err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "invalid params: " + err.Error()}
	}

	// Extract text from message parts
	var instruction string
	for _, part := range sp.Message.Parts {
		if part.Text != "" {
			if instruction != "" {
				instruction += "\n"
			}
			instruction += part.Text
		}
	}
	if instruction == "" {
		return nil, &jsonRPCError{Code: -32602, Message: "message must contain at least one text part"}
	}

	// Create internal task
	taskID := uuid.New().String()
	callerTaskID := sp.TaskID
	now := time.Now().UTC()

	_, err = s.DB.Exec(ctx,
		`INSERT INTO tasks (id, title, description, status, created_by, source, caller_task_id, replan_count, created_at)
		 VALUES ($1, $2, $3, 'pending', 'a2a', 'a2a', $4, 0, $5)`,
		taskID, instruction, instruction, callerTaskID, now)
	if err != nil {
		log.Printf("a2a server: create task: %v", err)
		return nil, &jsonRPCError{Code: -32000, Message: "internal error creating task"}
	}

	// Start execution in background
	if s.TaskExecutor != nil {
		go func() {
			if err := s.TaskExecutor(context.Background(), taskID); err != nil {
				log.Printf("a2a server: execute task %s: %v", taskID, err)
			}
		}()
	}

	// Return immediately with "working" state
	return &a2aServerTask{
		ID:     taskID,
		Status: a2aStatus{State: "working"},
	}, nil
}

// handleTasksGet returns the current state of an internal task as an A2A task.
func (s *Server) handleTasksGet(ctx context.Context, params any) (any, *jsonRPCError) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "invalid params"}
	}

	var tp taskIDParams
	if err := json.Unmarshal(paramsJSON, &tp); err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "invalid params"}
	}

	// Query internal task
	var status, result string
	var resultJSON []byte
	var taskError *string
	err = s.DB.QueryRow(ctx,
		`SELECT status, result, error FROM tasks WHERE id = $1`, tp.ID).
		Scan(&status, &resultJSON, &taskError)
	if err != nil {
		return nil, &jsonRPCError{Code: -32001, Message: "task not found"}
	}

	// Map internal status to A2A state
	state := mapStatusToA2AState(status)

	task := &a2aServerTask{
		ID:     tp.ID,
		Status: a2aStatus{State: state},
	}

	// Add error message if failed
	if state == "failed" && taskError != nil && *taskError != "" {
		task.Status.Message = &a2aMessage{
			Role:  "agent",
			Parts: []MessagePart{TextPart(*taskError)},
		}
	}

	// Add result as artifact if completed
	if state == "completed" && resultJSON != nil {
		task.Artifacts = []artifact{
			{
				ArtifactID: "result",
				Parts:      []MessagePart{DataPart(json.RawMessage(resultJSON))},
			},
		}
	}

	return task, nil
}

// handleTasksCancel cancels an internal task.
func (s *Server) handleTasksCancel(ctx context.Context, params any) (any, *jsonRPCError) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "invalid params"}
	}

	var tp taskIDParams
	if err := json.Unmarshal(paramsJSON, &tp); err != nil {
		return nil, &jsonRPCError{Code: -32602, Message: "invalid params"}
	}

	// Mark as canceled
	tag, err := s.DB.Exec(ctx,
		`UPDATE tasks SET status = 'canceled' WHERE id = $1 AND status NOT IN ('completed', 'failed', 'canceled')`,
		tp.ID)
	if err != nil {
		return nil, &jsonRPCError{Code: -32000, Message: "internal error"}
	}
	if tag.RowsAffected() == 0 {
		return nil, &jsonRPCError{Code: -32001, Message: "task not found or already terminal"}
	}

	return &a2aServerTask{
		ID:     tp.ID,
		Status: a2aStatus{State: "canceled"},
	}, nil
}

// mapStatusToA2AState maps internal task status to A2A protocol state.
func mapStatusToA2AState(status string) string {
	switch status {
	case "pending", "planning":
		return "submitted"
	case "running":
		return "working"
	case "completed":
		return "completed"
	case "failed":
		return "failed"
	case "canceled", "cancelled":
		return "canceled"
	default:
		return "unknown"
	}
}

// writeRPCResult writes a successful JSON-RPC response.
func (s *Server) writeRPCResult(w http.ResponseWriter, id string, result any) {
	resultJSON, _ := json.Marshal(result)
	resp := jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultJSON,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// writeRPCError writes an error JSON-RPC response.
func (s *Server) writeRPCError(w http.ResponseWriter, id string, code int, message string) {
	resp := struct {
		JSONRPC string        `json:"jsonrpc"`
		ID      string        `json:"id"`
		Error   jsonRPCError  `json:"error"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Error:   jsonRPCError{Code: code, Message: message},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
```

- [ ] **Step 1: Create the file above**
- [ ] **Step 2: `go build ./...` — verify it compiles**
- [ ] **Step 3: Commit**: `feat(a2a): add JSON-RPC server handler for tasks/send, tasks/get, tasks/cancel`

---

### Task 3: Agent health check worker

**Files:**
- Create: `internal/a2a/health.go`

Background worker that periodically checks agents and updates `is_online`, `last_health_check`, `skill_hash`.

```go
// health.go
package a2a

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthChecker periodically checks agent health and updates online status.
type HealthChecker struct {
	DB         *pgxpool.Pool
	Resolver   *Resolver
	Aggregator *Aggregator
	Interval   time.Duration
}

// Start runs the health checker in a background goroutine. Cancel the context to stop.
func (h *HealthChecker) Start(ctx context.Context) {
	if h.Interval <= 0 {
		h.Interval = 2 * time.Minute
	}

	go func() {
		// Run immediately on start
		h.checkAll(ctx)

		ticker := time.NewTicker(h.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.checkAll(ctx)
			}
		}
	}()
}

func (h *HealthChecker) checkAll(ctx context.Context) {
	rows, err := h.DB.Query(ctx,
		`SELECT id, endpoint, skill_hash FROM agents WHERE status = 'active'`)
	if err != nil {
		log.Printf("health: query agents: %v", err)
		return
	}
	defer rows.Close()

	type agentInfo struct {
		id       string
		endpoint string
		oldHash  string
	}

	var agents []agentInfo
	for rows.Next() {
		var a agentInfo
		if err := rows.Scan(&a.id, &a.endpoint, &a.oldHash); err != nil {
			continue
		}
		agents = append(agents, a)
	}

	changed := false
	for _, a := range agents {
		online, newHash := h.checkOne(ctx, a.endpoint)

		_, err := h.DB.Exec(ctx,
			`UPDATE agents SET is_online = $1, last_health_check = NOW(), skill_hash = $2 WHERE id = $3`,
			online, newHash, a.id)
		if err != nil {
			log.Printf("health: update agent %s: %v", a.id, err)
		}

		if a.oldHash != newHash {
			changed = true
			if a.oldHash != "" {
				log.Printf("health: agent %s skill drift detected (hash %s -> %s)", a.id, a.oldHash[:8], newHash[:8])
			}
		}
	}

	if changed && h.Aggregator != nil {
		h.Aggregator.Invalidate()
	}
}

func (h *HealthChecker) checkOne(ctx context.Context, endpoint string) (online bool, skillHash string) {
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	discovered, err := h.Resolver.Discover(checkCtx, endpoint)
	if err != nil {
		return false, ""
	}

	// Compute skill hash from discovered skills
	skillsJSON, _ := json.Marshal(discovered.Skills)
	hash := sha256.Sum256(skillsJSON)
	return true, fmt.Sprintf("%x", hash)
}
```

- [ ] **Step 1: Create the file above**
- [ ] **Step 2: `go build ./...` — verify it compiles**
- [ ] **Step 3: Commit**: `feat(a2a): add agent health check worker with drift detection`

---

### Task 4: AgentCard endpoint + A2A config handler

**Files:**
- Create: `internal/handlers/a2aconfig.go`

Handler for `GET /.well-known/agent-card.json` and the config management API.

```go
// a2aconfig.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/a2a"
)

// A2AConfigHandler provides handlers for AgentCard serving and A2A server configuration.
type A2AConfigHandler struct {
	DB         *pgxpool.Pool
	Aggregator *a2a.Aggregator
	BaseURL    string
}

// ServeAgentCard handles GET /.well-known/agent-card.json.
// Public endpoint, no auth required.
func (h *A2AConfigHandler) ServeAgentCard(w http.ResponseWriter, r *http.Request) {
	// Check if A2A server is enabled
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

	// Check If-None-Match for caching
	if match := r.Header.Get("If-None-Match"); match != "" {
		if strings.Contains(match, etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Write(data)
}

// GetConfig handles GET /api/a2a-config.
func (h *A2AConfigHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	var cfg struct {
		Enabled             bool            `json:"enabled"`
		NameOverride        *string         `json:"name_override"`
		DescriptionOverride *string         `json:"description_override"`
		SecurityScheme      json.RawMessage `json:"security_scheme"`
		AggregatedCard      json.RawMessage `json:"aggregated_card"`
		CardUpdatedAt       *string         `json:"card_updated_at"`
	}

	err := h.DB.QueryRow(r.Context(),
		`SELECT enabled, name_override, description_override, security_scheme, aggregated_card, card_updated_at
		 FROM a2a_server_config WHERE id = 1`).
		Scan(&cfg.Enabled, &cfg.NameOverride, &cfg.DescriptionOverride,
			&cfg.SecurityScheme, &cfg.AggregatedCard, &cfg.CardUpdatedAt)
	if err != nil {
		jsonError(w, "could not load config", http.StatusInternalServerError)
		return
	}

	jsonOK(w, cfg)
}

// updateA2AConfigRequest is the body for PUT /api/a2a-config.
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

	if req.Enabled != nil {
		_, _ = h.DB.Exec(r.Context(),
			`UPDATE a2a_server_config SET enabled = $1 WHERE id = 1`, *req.Enabled)
	}
	if req.NameOverride != nil {
		_, _ = h.DB.Exec(r.Context(),
			`UPDATE a2a_server_config SET name_override = $1 WHERE id = 1`, *req.NameOverride)
	}
	if req.DescriptionOverride != nil {
		_, _ = h.DB.Exec(r.Context(),
			`UPDATE a2a_server_config SET description_override = $1 WHERE id = 1`, *req.DescriptionOverride)
	}

	// Invalidate cached card
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
		"etag":       etag,
		"card":       json.RawMessage(data),
		"rebuilt_at": "now",
	})
}
```

- [ ] **Step 1: Create the file above**
- [ ] **Step 2: `go build ./...` — verify it compiles**
- [ ] **Step 3: Commit**: `feat(handlers): add AgentCard endpoint and A2A config API`

---

### Task 5: Wire everything in main.go

**Files:**
- Modify: `cmd/server/main.go`

Add initialization and route registration for all new A2A server components.

**Changes to main.go:**

1. Create the aggregator:
```go
aggregator := a2a.NewAggregator(pool)
```

2. Create the A2A server:
```go
a2aServer := &a2a.Server{
    DB:         pool,
    Aggregator: aggregator,
    BaseURL:    "http://localhost:" + cfg.Port,
    TaskExecutor: func(ctx context.Context, taskID string) error {
        // Load task from DB and execute
        var task models.Task
        err := pool.QueryRow(ctx,
            `SELECT id, title, description, status, created_by, replan_count, created_at
             FROM tasks WHERE id = $1`, taskID).
            Scan(&task.ID, &task.Title, &task.Description, &task.Status,
                &task.CreatedBy, &task.ReplanCount, &task.CreatedAt)
        if err != nil {
            return fmt.Errorf("load task %s: %w", taskID, err)
        }
        return exec.Execute(ctx, task)
    },
}
```

3. Create the A2A config handler:
```go
a2aConfigH := &handlers.A2AConfigHandler{
    DB:         pool,
    Aggregator: aggregator,
    BaseURL:    "http://localhost:" + cfg.Port,
}
```

4. Start health checker:
```go
healthChecker := &a2a.HealthChecker{
    DB:         pool,
    Resolver:   a2aResolver,
    Aggregator: aggregator,
    Interval:   2 * time.Minute,
}
healthChecker.Start(ctx)
```

5. Add routes (OUTSIDE the auth group for public endpoints):
```go
// A2A public endpoints (no auth)
r.Get("/.well-known/agent-card.json", a2aConfigH.ServeAgentCard)
r.Post("/a2a", a2aServer.HandleJSONRPC)
```

6. Add routes INSIDE the auth group:
```go
// A2A config (admin)
r.Get("/api/a2a-config", a2aConfigH.GetConfig)
r.Put("/api/a2a-config", a2aConfigH.UpdateConfig)
r.Post("/api/a2a-config/refresh-card", a2aConfigH.RefreshCard)
```

7. Add needed imports: `"fmt"`, `"taskhub/internal/models"` if not already imported.

- [ ] **Step 1: Apply all changes to main.go**
- [ ] **Step 2: `go build ./...` — verify it compiles**
- [ ] **Step 3: Run `go test ./...` to verify nothing broke**
- [ ] **Step 4: Commit**: `feat(server): wire A2A server, aggregator, health checker, and config routes`

---

### Task 6: Invalidate AgentCard on agent CRUD

**Files:**
- Modify: `internal/handlers/agents.go`

When agents are created, updated, or deleted, the aggregated card should be invalidated.

1. Add `Aggregator *a2a.Aggregator` field to `AgentHandler` struct.
2. In `Create`, after successful INSERT, call `h.Aggregator.Invalidate()` (if non-nil).
3. In `Update`, after successful UPDATE, call `h.Aggregator.Invalidate()`.
4. In `Delete`, after successful DELETE, call `h.Aggregator.Invalidate()`.
5. Update `cmd/server/main.go` to pass the aggregator: `agentH := &handlers.AgentHandler{DB: pool, Resolver: a2aResolver, Aggregator: aggregator}`

- [ ] **Step 1: Apply changes to agents.go**
- [ ] **Step 2: Update main.go to wire aggregator into AgentHandler**
- [ ] **Step 3: `go build ./...` — verify it compiles**
- [ ] **Step 4: Commit**: `feat(agents): invalidate aggregated card on agent CRUD`

---

### Task 7: Final verification

- [ ] **Step 1: `go build ./...`**
- [ ] **Step 2: `go test ./...`**
- [ ] **Step 3: Review commit log**: `git log --oneline feat/meta-agent-foundation ^main`
