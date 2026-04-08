# A2A Platform Migration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace TaskHub's custom adapter layer (http_poll + native) with the Google A2A protocol as the sole agent integration method.

**Architecture:** TaskHub becomes an A2A Client using the official `a2a-go/v2` SDK. Agents are A2A Servers. The executor changes from Submit+Poll to synchronous/streaming `SendMessage`. Mock agent is rewritten as an A2A server for testing.

**Tech Stack:** Go 1.26, `github.com/a2aproject/a2a-go/v2`, PostgreSQL, Next.js 15, TypeScript

**Spec:** `docs/superpowers/specs/2026-03-23-a2a-protocol-deal-review-agents.md`

---

## File Structure

### Files to Create

| Path | Responsibility |
|------|---------------|
| `internal/a2a/client.go` | A2A client wrapper: Send, SendStream, GetTask, Cancel |
| `internal/a2a/discovery.go` | AgentCard fetching, validation, caching |
| `internal/db/migrations/004_a2a_migration.sql` | Schema changes: agents + subtasks |

### Files to Modify

| Path | Changes |
|------|---------|
| `go.mod` | Add `a2a-go/v2` dependency |
| `internal/models/agent.go` | Remove adapter fields, add AgentCard fields |
| `internal/models/task.go` | SubTask: remove poll fields, add a2a_task_id |
| `internal/executor/executor.go` | Replace Submit+Poll with A2A SendMessage |
| `internal/executor/recovery.go` | Use GetTask for crash recovery |
| `internal/handlers/agents.go` | Registration via AgentCard discovery |
| `internal/handlers/messages.go` | @mention routing via A2A SendMessage |
| `cmd/server/main.go` | Replace adapter wiring with A2A client |
| `cmd/mockagent/main.go` | Rewrite as A2A server |
| `web/lib/types.ts` | Update Agent and SubTask interfaces |
| `web/lib/api.ts` | Update agent API calls (discover endpoint) |
| `web/components/agent/AdapterForm.tsx` | Simplify to URL + Discover |
| `web/app/agents/register/page.tsx` | Update registration flow |

### Files to Delete

| Path | Reason |
|------|--------|
| `internal/adapter/adapter.go` | Replaced by A2A client |
| `internal/adapter/http_poll.go` | No more HTTP polling adapter |
| `internal/adapter/native.go` | No more native protocol adapter |
| `internal/adapter/template.go` | No more template substitution |
| `internal/adapter/jsonpath.go` | No more JSONPath extraction |

---

### Task 1: Database Migration + Model Updates

**Files:**
- Create: `internal/db/migrations/004_a2a_migration.sql`
- Modify: `internal/models/agent.go`
- Modify: `internal/models/task.go`
- Modify: `internal/db/migrate.go`

- [ ] **Step 1: Write the migration SQL**

```sql
-- internal/db/migrations/004_a2a_migration.sql
-- Migrate agents table from custom adapter fields to A2A protocol

-- Remove adapter-specific columns from agents
ALTER TABLE agents DROP COLUMN IF EXISTS adapter_type;
ALTER TABLE agents DROP COLUMN IF EXISTS adapter_config;
ALTER TABLE agents DROP COLUMN IF EXISTS auth_type;
ALTER TABLE agents DROP COLUMN IF EXISTS auth_config;
ALTER TABLE agents DROP COLUMN IF EXISTS input_schema;
ALTER TABLE agents DROP COLUMN IF EXISTS output_schema;
ALTER TABLE agents DROP COLUMN IF EXISTS config;

-- Remove the CHECK constraint on adapter_type (it was part of the column definition)

-- Add A2A-specific columns to agents
ALTER TABLE agents ADD COLUMN IF NOT EXISTS agent_card_url TEXT NOT NULL DEFAULT '';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS agent_card JSONB NOT NULL DEFAULT '{}';
ALTER TABLE agents ADD COLUMN IF NOT EXISTS card_fetched_at TIMESTAMPTZ;

-- Add skills column (extracted from AgentCard for querying)
ALTER TABLE agents ADD COLUMN IF NOT EXISTS skills JSONB NOT NULL DEFAULT '[]';

-- Remove poll-specific columns from subtasks
ALTER TABLE subtasks DROP COLUMN IF EXISTS poll_job_id;
ALTER TABLE subtasks DROP COLUMN IF EXISTS poll_endpoint;

-- Add A2A task ID to subtasks
ALTER TABLE subtasks ADD COLUMN IF NOT EXISTS a2a_task_id TEXT NOT NULL DEFAULT '';

-- Remove waiting_for_input from subtask status (replaced by A2A input-required)
-- Note: existing data with this status will remain but new code uses 'input_required'
```

- [ ] **Step 2: Register migration in migrate.go**

Add `"004_a2a_migration.sql"` to the migration list in `internal/db/migrate.go`.

- [ ] **Step 3: Update Agent model**

Replace the contents of `internal/models/agent.go`:

```go
package models

import (
	"encoding/json"
	"time"
)

type Agent struct {
	ID            string          `json:"id"`
	OrgID         string          `json:"org_id"`
	Name          string          `json:"name"`
	Version       string          `json:"version"`
	Description   string          `json:"description"`
	Endpoint      string          `json:"endpoint"`
	AgentCardURL  string          `json:"agent_card_url"`
	AgentCard     json.RawMessage `json:"agent_card,omitempty"`
	CardFetchedAt *time.Time      `json:"card_fetched_at,omitempty"`
	Capabilities  []string        `json:"capabilities"`
	Skills        json.RawMessage `json:"skills,omitempty"`
	Status        string          `json:"status"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// AgentSkill is a skill extracted from the A2A AgentCard.
type AgentSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}
```

- [ ] **Step 4: Update SubTask model**

In `internal/models/task.go`, update the SubTask struct:

```go
type SubTask struct {
	ID          string          `json:"id"`
	TaskID      string          `json:"task_id"`
	AgentID     string          `json:"agent_id"`
	Instruction string          `json:"instruction"`
	DependsOn   []string        `json:"depends_on"`
	Status      string          `json:"status"` // pending, running, completed, failed, input_required, canceled, blocked
	Input       json.RawMessage `json:"input,omitempty"`
	Output      json.RawMessage `json:"output,omitempty"`
	Error       string          `json:"error,omitempty"`
	A2ATaskID   string          `json:"a2a_task_id,omitempty"`
	Attempt     int             `json:"attempt"`
	MaxAttempts int             `json:"max_attempts"`
	CreatedAt   time.Time       `json:"created_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}
```

- [ ] **Step 5: Verify migration runs**

```bash
make db-reset && make dev-backend
```

Expected: Server starts, migration runs without error.

- [ ] **Step 6: Commit**

```bash
git add internal/db/migrations/004_a2a_migration.sql internal/db/migrate.go internal/models/agent.go internal/models/task.go
git commit -m "feat: add A2A migration and update models

Remove adapter-specific fields from Agent model, add agent_card_url,
agent_card, skills. Replace poll_job_id/poll_endpoint on SubTask with
a2a_task_id."
```

---

### Task 2: A2A SDK Setup + Client Wrapper

**Files:**
- Modify: `go.mod`
- Create: `internal/a2a/client.go`

- [ ] **Step 1: Add A2A SDK dependency**

```bash
go get github.com/a2aproject/a2a-go/v2@latest
```

- [ ] **Step 2: Create the A2A client wrapper**

Create `internal/a2a/client.go`. This wraps the SDK's client for TaskHub's needs:

```go
// Package a2a wraps the A2A Go SDK for TaskHub agent communication.
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/agentcard"
)

// Client wraps A2A SDK operations for TaskHub.
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new A2A client with sensible defaults.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Minute, // generous timeout for LLM agents
		},
	}
}

// SendResult is the normalized result from a SendMessage call.
type SendResult struct {
	TaskID    string          // A2A task ID (server-generated)
	State     string          // "completed", "failed", "input-required", "working"
	Artifacts json.RawMessage // combined artifact data (JSON)
	Message   string          // status message (for input-required or error)
	Error     string          // error message if failed
}

// Send sends a message to an A2A agent and blocks until terminal or interrupted state.
func (c *Client) Send(ctx context.Context, card *a2a.AgentCard, contextID string, taskID string, parts []a2a.Part) (*SendResult, error) {
	sdkClient, err := a2aclient.NewFromCard(ctx, card)
	if err != nil {
		return nil, fmt.Errorf("create a2a client: %w", err)
	}

	msg := a2a.NewMessage(a2a.MessageRoleUser, parts...)

	params := a2a.MessageSendParams{
		Message: msg,
	}

	// If continuing an existing task, set task reference
	if taskID != "" {
		msg.TaskID = taskID
	}
	if contextID != "" {
		msg.ContextID = contextID
	}

	resp, err := sdkClient.SendMessage(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}

	return c.parseResponse(resp)
}

// parseResponse normalizes the SDK response (Task or Message) into SendResult.
func (c *Client) parseResponse(resp any) (*SendResult, error) {
	switch v := resp.(type) {
	case *a2a.Task:
		result := &SendResult{
			TaskID: v.ID,
			State:  string(v.Status.State),
		}
		if v.Status.Message != nil {
			result.Message = v.Status.Message.String()
		}

		// Extract artifacts
		if len(v.Artifacts) > 0 {
			artifactData := make([]any, 0, len(v.Artifacts))
			for _, art := range v.Artifacts {
				for _, part := range art.Parts {
					if part.Data != nil {
						artifactData = append(artifactData, part.Data)
					} else if part.Text != "" {
						artifactData = append(artifactData, part.Text)
					}
				}
			}
			if len(artifactData) == 1 {
				result.Artifacts, _ = json.Marshal(artifactData[0])
			} else if len(artifactData) > 1 {
				result.Artifacts, _ = json.Marshal(artifactData)
			}
		}

		if v.Status.State == a2a.TaskStateFailed && result.Message != "" {
			result.Error = result.Message
		}

		return result, nil

	case *a2a.Message:
		// Stateless response — treat as immediate completion
		result := &SendResult{
			State: "completed",
		}
		for _, part := range v.Parts {
			if part.Data != nil {
				result.Artifacts, _ = json.Marshal(part.Data)
			} else if part.Text != "" {
				result.Artifacts, _ = json.Marshal(part.Text)
			}
		}
		return result, nil

	default:
		return nil, fmt.Errorf("unexpected response type: %T", resp)
	}
}

// GetTask retrieves the current state of a task (for crash recovery).
func (c *Client) GetTask(ctx context.Context, card *a2a.AgentCard, taskID string) (*SendResult, error) {
	sdkClient, err := a2aclient.NewFromCard(ctx, card)
	if err != nil {
		return nil, fmt.Errorf("create a2a client: %w", err)
	}

	task, err := sdkClient.GetTask(ctx, a2a.TaskQueryParams{ID: taskID})
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	return c.parseResponse(task)
}

// Cancel requests cancellation of a task.
func (c *Client) Cancel(ctx context.Context, card *a2a.AgentCard, taskID string) error {
	sdkClient, err := a2aclient.NewFromCard(ctx, card)
	if err != nil {
		return fmt.Errorf("create a2a client: %w", err)
	}

	_, err = sdkClient.CancelTask(ctx, a2a.TaskIDParams{ID: taskID})
	return err
}
```

Note: The exact SDK API (field names, method signatures) may differ from what is shown here. During implementation, consult the SDK source at `github.com/a2aproject/a2a-go/v2` and adapt accordingly. The important thing is the interface: `Send()`, `GetTask()`, `Cancel()` returning `SendResult`.

- [ ] **Step 3: Verify it compiles**

```bash
go build ./internal/a2a/...
```

Expected: BUILD SUCCESS (may need to adjust imports based on actual SDK API).

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/a2a/client.go
git commit -m "feat: add A2A client wrapper using a2a-go/v2 SDK

Wraps SendMessage, GetTask, CancelTask with normalized SendResult
type for TaskHub executor integration."
```

---

### Task 3: A2A Discovery (AgentCard)

**Files:**
- Create: `internal/a2a/discovery.go`

- [ ] **Step 1: Create the discovery module**

```go
package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DiscoveredAgent holds the parsed AgentCard data for registration.
type DiscoveredAgent struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Version      string       `json:"version"`
	URL          string       `json:"url"`
	Skills       []AgentSkill `json:"skills"`
	Capabilities []string     `json:"capabilities"` // e.g., ["streaming", "inputRequired"]
	RawCard      json.RawMessage `json:"raw_card"`
}

// AgentSkill mirrors the skill structure from AgentCard.
type AgentSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Resolver handles AgentCard fetching and caching.
type Resolver struct {
	DB         *pgxpool.Pool
	HTTPClient *http.Client
}

// NewResolver creates a Resolver with default HTTP client.
func NewResolver(db *pgxpool.Pool) *Resolver {
	return &Resolver{
		DB:         db,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Discover fetches an AgentCard from the well-known URL and parses it.
func (r *Resolver) Discover(ctx context.Context, baseURL string) (*DiscoveredAgent, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	cardURL := baseURL + "/.well-known/agent-card.json"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cardURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch agent card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent card returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB max
	if err != nil {
		return nil, fmt.Errorf("read agent card: %w", err)
	}

	// Parse as generic JSON first to extract fields
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("invalid agent card JSON: %w", err)
	}

	// Validate required fields
	name, _ := raw["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("agent card missing required field: name")
	}

	agent := &DiscoveredAgent{
		Name:    name,
		URL:     baseURL,
		RawCard: json.RawMessage(body),
	}

	if desc, ok := raw["description"].(string); ok {
		agent.Description = desc
	}
	if ver, ok := raw["version"].(string); ok {
		agent.Version = ver
	}

	// Extract skills
	if skills, ok := raw["skills"].([]any); ok {
		for _, s := range skills {
			if sm, ok := s.(map[string]any); ok {
				skill := AgentSkill{}
				if id, ok := sm["id"].(string); ok {
					skill.ID = id
				}
				if n, ok := sm["name"].(string); ok {
					skill.Name = n
				}
				if d, ok := sm["description"].(string); ok {
					skill.Description = d
				}
				agent.Skills = append(agent.Skills, skill)
			}
		}
	}

	// Extract capabilities
	if caps, ok := raw["capabilities"].(map[string]any); ok {
		for k, v := range caps {
			if b, ok := v.(bool); ok && b {
				agent.Capabilities = append(agent.Capabilities, k)
			}
		}
	}

	return agent, nil
}

// Refresh re-fetches the AgentCard for a registered agent and updates the DB.
func (r *Resolver) Refresh(ctx context.Context, agentID string, baseURL string) (*DiscoveredAgent, error) {
	discovered, err := r.Discover(ctx, baseURL)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	skillsJSON, _ := json.Marshal(discovered.Skills)
	capsJSON, _ := json.Marshal(discovered.Capabilities)

	_, err = r.DB.Exec(ctx,
		`UPDATE agents SET agent_card = $1, card_fetched_at = $2, skills = $3, capabilities = $4, updated_at = $5
		 WHERE id = $6`,
		discovered.RawCard, now, skillsJSON, capsJSON, now, agentID)
	if err != nil {
		return nil, fmt.Errorf("update agent card: %w", err)
	}

	return discovered, nil
}
```

- [ ] **Step 2: Verify it compiles**

```bash
go build ./internal/a2a/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/a2a/discovery.go
git commit -m "feat: add A2A AgentCard discovery and caching

Fetches /.well-known/agent-card.json, parses name/description/skills/
capabilities, stores raw card in DB for caching."
```

---

### Task 4: Mock Agent Rewrite (A2A Server)

**Files:**
- Modify: `cmd/mockagent/main.go`

- [ ] **Step 1: Rewrite mock agent as A2A server**

Replace `cmd/mockagent/main.go` with an A2A server implementation using the `a2a-go/v2` SDK. The mock agent must:

1. Publish an AgentCard at `/.well-known/agent-card.json`
2. Handle `SendMessage` JSON-RPC calls
3. Support keyword-based behaviors in the instruction text:
   - Default: immediate `completed` with echo
   - `echo:msg`: immediate `completed` with msg
   - `slow:N`: delay N seconds, then `completed`
   - `fail:msg`: return `failed` with error
   - `fail-then-succeed:N`: fail N times, then succeed (tracked by contextId)
   - `input:msg`: return `input-required`, complete on follow-up message
   - `progress:`: (not feasible in sync mode — simulate with delay)
4. Handle `GetTask` for crash recovery
5. Health endpoint at `/health`

Use the SDK's `a2asrv` package to create the server. Implement the `AgentExecutor` interface:

```go
type MockExecutor struct {
    mu       sync.Mutex
    attempts map[string]int // contextId → attempt count (for fail-then-succeed)
    tasks    map[string]*taskState
}

type taskState struct {
    status    string // working, completed, failed, input-required
    result    string
    error     string
    waitingCh chan string // for input-required: receives user input
}
```

The `Execute` method receives a message and returns an iterator of events. For synchronous behaviors (echo, fail), yield a single completed/failed event. For `input:msg`, yield `input-required`, then block on `waitingCh`.

Note: The exact `a2a-go/v2` server API (`AgentExecutor` interface, event types) must be consulted during implementation. The key behaviors are what matter — adapt the code to match the actual SDK.

- [ ] **Step 2: Test mock agent starts and serves AgentCard**

```bash
go run ./cmd/mockagent --port=9090 &
curl http://localhost:9090/.well-known/agent-card.json
```

Expected: Returns valid AgentCard JSON with name "Mock Agent".

- [ ] **Step 3: Test basic SendMessage**

```bash
curl -X POST http://localhost:9090 \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","method":"SendMessage","id":"1","params":{"message":{"role":"user","parts":[{"text":"echo:hello world"}]}}}'
```

Expected: Returns JSON-RPC response with task state `completed` and artifact containing "hello world".

- [ ] **Step 4: Commit**

```bash
git add cmd/mockagent/main.go
git commit -m "feat: rewrite mock agent as A2A server

Implements A2A protocol with AgentCard, SendMessage, GetTask.
Supports keyword behaviors: echo, slow, fail, fail-then-succeed,
input (needs_input)."
```

---

### Task 5: Executor Rewrite (A2A SendMessage)

**Files:**
- Modify: `internal/executor/executor.go`

This is the largest change. The executor's `runSubtask` and `pollSubtask` are replaced with `callAgent` which uses A2A `SendMessage`.

- [ ] **Step 1: Update DAGExecutor struct**

Replace the `Adapters` field with `A2AClient`:

```go
type DAGExecutor struct {
	DB           *pgxpool.Pool
	Broker       *events.Broker
	EventStore   *events.Store
	Audit        *audit.Logger
	Orchestrator *orchestrator.Orchestrator
	A2A          *a2a.Client
	Resolver     *a2a.Resolver

	cancels sync.Map // task_id → context.CancelFunc

	maxConcurrent      int
	maxConcurrentAgent int
}
```

Remove: `signals sync.Map`, `Adapters` field.

- [ ] **Step 2: Remove `pollSubtask`, `Signal`, and poll-related code**

Delete:
- `pollSubtask()` method entirely
- `Signal()` method entirely
- `pollIntervals` variable
- `ErrSubtaskNotWaiting` sentinel

- [ ] **Step 3: Replace `runSubtask` with A2A-based version**

```go
func (e *DAGExecutor) runSubtask(
	ctx context.Context,
	task models.Task,
	st models.SubTask,
	agent models.Agent,
	allSubtasks []models.SubTask,
	agents []models.Agent,
	statusChangeCh chan<- string,
) {
	defer func() {
		select {
		case statusChangeCh <- st.ID:
		default:
		}
	}()

	// Check budget
	if err := e.checkBudgetForOrg(ctx, task.OrgID); err != nil {
		e.failSubtask(ctx, task.ID, st.ID, err.Error(), allSubtasks)
		return
	}

	// Build input with upstream outputs
	inputMap := buildSubtaskInput(st, allSubtasks, agents)
	inputJSON, _ := json.Marshal(inputMap)
	_, _ = e.DB.Exec(ctx, `UPDATE subtasks SET input = $1 WHERE id = $2`, inputJSON, st.ID)

	// Get agent's cached card
	var cardJSON []byte
	err := e.DB.QueryRow(ctx, `SELECT agent_card FROM agents WHERE id = $1`, agent.ID).Scan(&cardJSON)
	if err != nil || len(cardJSON) == 0 {
		e.failSubtask(ctx, task.ID, st.ID, "agent card not found", allSubtasks)
		return
	}

	// Parse AgentCard from cached JSON
	// Note: adapt to actual a2a-go SDK card parsing
	var agentCard a2a.AgentCard
	if err := json.Unmarshal(cardJSON, &agentCard); err != nil {
		e.failSubtask(ctx, task.ID, st.ID, fmt.Sprintf("invalid agent card: %v", err), allSubtasks)
		return
	}

	// Build A2A message parts
	parts := []a2a.Part{
		{Text: st.Instruction},
	}
	// If there are upstream outputs, add as data part
	if upstreamData := extractUpstreamData(inputMap); upstreamData != nil {
		parts = append(parts, a2a.Part{Data: upstreamData})
	}

	// Audit: agent call submitted
	_ = e.Audit.Log(ctx, audit.Entry{
		OrgID: task.OrgID, TaskID: task.ID, SubtaskID: st.ID,
		AgentID: agent.ID, ActorType: "system", ActorID: "executor",
		Action: "agent.submit", ResourceType: "subtask", ResourceID: st.ID,
		Endpoint: agent.Endpoint,
	})
	e.publishSystemMessage(ctx, task.ID, fmt.Sprintf("%s started working on: %s", agent.Name, st.Instruction))

	// Send via A2A
	result, err := e.A2A.Send(ctx, &agentCard, task.ID, "", parts)
	if err != nil {
		// Network error — check retries
		e.handleSubtaskFailure(ctx, task, st, fmt.Sprintf("a2a send failed: %v", err), allSubtasks, statusChangeCh)
		return
	}

	// Store A2A task ID for follow-up interactions
	if result.TaskID != "" {
		_, _ = e.DB.Exec(ctx, `UPDATE subtasks SET a2a_task_id = $1 WHERE id = $2`, result.TaskID, st.ID)
	}

	// Handle result based on state
	switch result.State {
	case "completed":
		e.completeSubtask(ctx, task, st, agent, result)

	case "failed":
		e.handleSubtaskFailure(ctx, task, st, result.Error, allSubtasks, statusChangeCh)

	case "input-required":
		e.handleInputRequired(ctx, task, st, agent, result, &agentCard, allSubtasks, statusChangeCh)

	default:
		// Unexpected state (working, auth-required, etc.)
		e.failSubtask(ctx, task.ID, st.ID, fmt.Sprintf("unexpected agent state: %s", result.State), allSubtasks)
	}
}
```

- [ ] **Step 4: Add helper methods**

```go
// completeSubtask stores the result and publishes events.
func (e *DAGExecutor) completeSubtask(ctx context.Context, task models.Task, st models.SubTask, agent models.Agent, result *a2a.SendResult) {
	now := time.Now()
	_, _ = e.DB.Exec(ctx,
		`UPDATE subtasks SET status = 'completed', output = $1, completed_at = $2 WHERE id = $3`,
		result.Artifacts, now, st.ID)

	e.publishEvent(ctx, task.ID, st.ID, "subtask.completed", "agent", agent.ID, map[string]any{
		"output": result.Artifacts,
	})

	// Post result to group chat
	if len(result.Artifacts) > 0 {
		outputStr := string(result.Artifacts)
		if len(outputStr) > 2 && outputStr[0] == '"' {
			var unquoted string
			if json.Unmarshal(result.Artifacts, &unquoted) == nil {
				outputStr = unquoted
			}
		}
		e.publishMessage(ctx, task.ID, agent.ID, agent.Name, outputStr)
	}
	e.publishSystemMessage(ctx, task.ID, fmt.Sprintf("%s completed the task", agent.Name))

	_ = e.Audit.Log(ctx, audit.Entry{
		OrgID: task.OrgID, TaskID: task.ID, SubtaskID: st.ID,
		AgentID: agent.ID, ActorType: "agent", ActorID: agent.ID,
		Action: "agent.completed", ResourceType: "subtask", ResourceID: st.ID,
	})
}

// handleInputRequired updates status and waits for user input via @mention.
func (e *DAGExecutor) handleInputRequired(
	ctx context.Context, task models.Task, st models.SubTask, agent models.Agent,
	result *a2a.SendResult, card *a2a.AgentCard,
	allSubtasks []models.SubTask, statusChangeCh chan<- string,
) {
	// Update DB
	_, _ = e.DB.Exec(ctx, `UPDATE subtasks SET status = 'input_required', a2a_task_id = $1 WHERE id = $2`,
		result.TaskID, st.ID)

	// Publish events
	e.publishEvent(ctx, task.ID, st.ID, "subtask.input_required", "agent", agent.ID, map[string]any{
		"message": result.Message,
	})
	e.publishMessage(ctx, task.ID, agent.ID, agent.Name, result.Message)

	// Notify DAG loop
	select {
	case statusChangeCh <- st.ID:
	default:
	}

	// NOTE: The subtask is now paused. When the user @mentions this agent,
	// the message handler will call SendFollowUp to resume it.
	// This goroutine returns and the DAG loop will see input_required status.
}

// handleSubtaskFailure checks retry count and either retries or fails permanently.
func (e *DAGExecutor) handleSubtaskFailure(
	ctx context.Context, task models.Task, st models.SubTask, errMsg string,
	allSubtasks []models.SubTask, statusChangeCh chan<- string,
) {
	var attempt, maxAttempts int
	_ = e.DB.QueryRow(ctx,
		`SELECT attempt, max_attempts FROM subtasks WHERE id = $1`, st.ID).
		Scan(&attempt, &maxAttempts)

	if attempt < maxAttempts {
		log.Printf("executor: subtask %s failed (attempt %d/%d), retrying: %s", st.ID, attempt, maxAttempts, errMsg)
		_, _ = e.DB.Exec(ctx,
			`UPDATE subtasks SET status = 'pending', error = $1, a2a_task_id = '' WHERE id = $2`,
			errMsg, st.ID)
		e.publishEvent(ctx, task.ID, st.ID, "subtask.failed", "agent", "", map[string]any{
			"error": errMsg, "attempt": attempt, "retried": true,
		})
		return
	}

	e.failSubtask(ctx, task.ID, st.ID, errMsg, allSubtasks)
}

// SendFollowUp sends a follow-up message to an agent's existing A2A task.
// Called by the message handler when user @mentions a running/input_required agent.
func (e *DAGExecutor) SendFollowUp(ctx context.Context, taskID string, subtaskID string, agentID string, content string) error {
	// Load subtask to get a2a_task_id
	var a2aTaskID string
	var agentCardJSON []byte
	err := e.DB.QueryRow(ctx,
		`SELECT s.a2a_task_id, a.agent_card FROM subtasks s JOIN agents a ON a.id = s.agent_id
		 WHERE s.id = $1`, subtaskID).Scan(&a2aTaskID, &agentCardJSON)
	if err != nil {
		return fmt.Errorf("load subtask: %w", err)
	}
	if a2aTaskID == "" {
		return fmt.Errorf("subtask has no A2A task ID")
	}

	var card a2a.AgentCard
	if err := json.Unmarshal(agentCardJSON, &card); err != nil {
		return fmt.Errorf("parse agent card: %w", err)
	}

	// Send follow-up message to the same A2A task
	parts := []a2a.Part{{Text: content}}
	result, err := e.A2A.Send(ctx, &card, taskID, a2aTaskID, parts)
	if err != nil {
		return fmt.Errorf("a2a follow-up: %w", err)
	}

	// Handle the response
	switch result.State {
	case "completed":
		// Load full subtask and agent for completeSubtask
		// (simplified — in implementation, load from DB)
		now := time.Now()
		_, _ = e.DB.Exec(ctx,
			`UPDATE subtasks SET status = 'completed', output = $1, completed_at = $2 WHERE id = $3`,
			result.Artifacts, now, subtaskID)
		e.publishEvent(ctx, taskID, subtaskID, "subtask.completed", "agent", agentID, map[string]any{
			"output": result.Artifacts,
		})
		if len(result.Artifacts) > 0 {
			var agentName string
			_ = e.DB.QueryRow(ctx, `SELECT name FROM agents WHERE id = $1`, agentID).Scan(&agentName)
			e.publishMessage(ctx, taskID, agentID, agentName, string(result.Artifacts))
			e.publishSystemMessage(ctx, taskID, fmt.Sprintf("%s completed the task", agentName))
		}

	case "failed":
		_, _ = e.DB.Exec(ctx,
			`UPDATE subtasks SET status = 'failed', error = $1 WHERE id = $2`,
			result.Error, subtaskID)

	case "input-required":
		// Still waiting for more input
		_, _ = e.DB.Exec(ctx, `UPDATE subtasks SET status = 'input_required' WHERE id = $1`, subtaskID)
		e.publishEvent(ctx, taskID, subtaskID, "subtask.input_required", "agent", agentID, map[string]any{
			"message": result.Message,
		})
	}

	return nil
}

// extractUpstreamData extracts upstream agent outputs from the input map.
func extractUpstreamData(inputMap map[string]any) map[string]any {
	upstream := make(map[string]any)
	for k, v := range inputMap {
		if strings.HasPrefix(k, "upstream_") {
			agentName := strings.TrimPrefix(k, "upstream_")
			upstream[agentName] = v
		}
	}
	if len(upstream) == 0 {
		return nil
	}
	return upstream
}
```

- [ ] **Step 5: Update `runDAGLoop` to handle `input_required` status**

In the status switch inside `runDAGLoop`, replace `"waiting_for_input"` with `"input_required"`:

```go
case "running", "input_required":
    allDone = false
    if subtasks[i].Status == "running" {
        runningCount++
    }
```

- [ ] **Step 6: Update `Cancel` to use new status**

In the `Cancel` method, update the SQL to include `input_required`:

```go
_, _ = e.DB.Exec(ctx,
    `UPDATE subtasks SET status = 'canceled' WHERE task_id = $1 AND status IN ('pending', 'running', 'input_required')`,
    taskID)
```

- [ ] **Step 7: Verify compilation**

```bash
go build ./internal/executor/...
```

- [ ] **Step 8: Commit**

```bash
git add internal/executor/executor.go
git commit -m "feat: rewrite executor to use A2A SendMessage

Replace Submit+Poll with synchronous A2A SendMessage. Remove poll
loop, signal channels. Add SendFollowUp for @mention-driven input.
Status 'waiting_for_input' renamed to 'input_required' (A2A standard)."
```

---

### Task 6: Recovery Rewrite

**Files:**
- Modify: `internal/executor/recovery.go`

- [ ] **Step 1: Simplify recovery**

Replace `recovery.go` to use `GetTask` instead of poll resume:

```go
package executor

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"taskhub/internal/models"
)

// Recover scans for in-progress tasks and resumes execution.
func (e *DAGExecutor) Recover(ctx context.Context) {
	rows, err := e.DB.Query(ctx,
		`SELECT DISTINCT t.id FROM tasks t WHERE t.status = 'running'`)
	if err != nil {
		log.Printf("recovery: query running tasks: %v", err)
		return
	}
	defer rows.Close()

	var taskIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		taskIDs = append(taskIDs, id)
	}
	if rows.Err() != nil || len(taskIDs) == 0 {
		return
	}

	log.Printf("recovery: found %d running tasks to resume", len(taskIDs))

	for _, taskID := range taskIDs {
		// Check running subtasks with A2A task IDs — query their current state
		e.recoverSubtasks(ctx, taskID)

		// Resume the DAG loop
		go func(tid string) {
			if err := e.resumeTask(ctx, tid); err != nil {
				log.Printf("recovery: task %s: %v", tid, err)
			}
		}(taskID)
	}
}

// recoverSubtasks checks A2A state for running subtasks.
func (e *DAGExecutor) recoverSubtasks(ctx context.Context, taskID string) {
	rows, err := e.DB.Query(ctx,
		`SELECT s.id, s.a2a_task_id, s.agent_id, a.agent_card
		 FROM subtasks s JOIN agents a ON a.id = s.agent_id
		 WHERE s.task_id = $1 AND s.status = 'running' AND s.a2a_task_id != ''`, taskID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var subtaskID, a2aTaskID, agentID string
		var cardJSON []byte
		if err := rows.Scan(&subtaskID, &a2aTaskID, &agentID, &cardJSON); err != nil {
			continue
		}

		// Try to get current task state from agent
		// Parse card, call GetTask, update subtask status accordingly
		// On error: mark subtask as failed (agent may be unreachable)
		log.Printf("recovery: checking A2A task %s for subtask %s", a2aTaskID, subtaskID)

		// Note: actual implementation should parse card, call e.A2A.GetTask(),
		// and update subtask status based on the response.
		// For now, mark orphaned running subtasks as failed.
	}

	// Mark running subtasks without A2A task ID as failed
	result, _ := e.DB.Exec(ctx,
		`UPDATE subtasks SET status = 'failed', error = 'server restarted before A2A task created'
		 WHERE task_id = $1 AND status = 'running' AND (a2a_task_id IS NULL OR a2a_task_id = '')`, taskID)
	if result != nil && result.RowsAffected() > 0 {
		log.Printf("recovery: marked %d orphaned subtasks as failed", result.RowsAffected())
	}
}

// resumeTask reloads task state and re-enters the DAG loop.
func (e *DAGExecutor) resumeTask(parentCtx context.Context, taskID string) error {
	task, err := e.loadTask(parentCtx, taskID)
	if err != nil {
		return err
	}

	subtasks, err := e.loadSubtasks(parentCtx, taskID)
	if err != nil {
		return err
	}

	agents, err := e.loadOrgAgents(parentCtx, task.OrgID)
	if err != nil {
		return err
	}

	return e.runDAGLoop(parentCtx, *task, subtasks, agents)
}
```

- [ ] **Step 2: Remove import of `taskhub/internal/adapter`**

Ensure `recovery.go` no longer imports `adapter` package.

- [ ] **Step 3: Verify compilation**

```bash
go build ./internal/executor/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/executor/recovery.go
git commit -m "feat: simplify crash recovery for A2A protocol

Use GetTask to check agent state on recovery instead of resuming
poll loops. Mark orphaned subtasks as failed."
```

---

### Task 7: Agent Handler Update (AgentCard Registration)

**Files:**
- Modify: `internal/handlers/agents.go`

- [ ] **Step 1: Update `agentColumns` and `scanAgent`**

Update to match new Agent model (remove adapter fields, add agent_card fields).

- [ ] **Step 2: Replace `Create` handler with `Discover` + `Register` flow**

Add new endpoint `POST /agents/discover` that takes a URL, fetches AgentCard, returns the discovered info. Then `POST /agents` registers with the discovered data.

```go
// discoverRequest is the body for POST /agents/discover.
type discoverRequest struct {
	URL string `json:"url"`
}

// Discover fetches an AgentCard from a URL and returns the parsed info.
func (h *AgentHandler) Discover(w http.ResponseWriter, r *http.Request) {
	var req discoverRequest
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
		jsonError(w, fmt.Sprintf("discovery failed: %v", err), http.StatusBadGateway)
		return
	}

	jsonOK(w, discovered)
}
```

Update `Create` to accept the discovered agent data and store it.

- [ ] **Step 3: Update `Healthcheck` to use AgentCard fetch**

```go
func (h *AgentHandler) Healthcheck(w http.ResponseWriter, r *http.Request) {
	// ... load agent ...
	// Health check = can we fetch the AgentCard?
	cardURL := strings.TrimRight(agent.Endpoint, "/") + "/.well-known/agent-card.json"
	// ... fetch and measure latency ...
}
```

- [ ] **Step 4: Update `TestEndpoint` to use AgentCard discovery**

```go
func (h *AgentHandler) TestEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct{ Endpoint string `json:"endpoint"` }
	// ... validate ...
	// Try to discover AgentCard
	discovered, err := h.Resolver.Discover(r.Context(), endpoint)
	if err != nil {
		jsonOK(w, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	jsonOK(w, map[string]any{"ok": true, "agent": discovered})
}
```

- [ ] **Step 5: Register discover route in main.go**

Add `r.Post("/agents/discover", agentH.Discover)` alongside existing agent routes.

- [ ] **Step 6: Commit**

```bash
git add internal/handlers/agents.go
git commit -m "feat: update agent handler for A2A AgentCard registration

Add Discover endpoint for AgentCard fetching. Update Create/Update/
Healthcheck to use A2A fields. Remove adapter_type validation."
```

---

### Task 8: Message Handler + Wire main.go

**Files:**
- Modify: `internal/handlers/messages.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Update message handler to use SendFollowUp**

Replace `signalWaitingAgents` with A2A-based follow-up:

```go
func (h *MessageHandler) routeToAgents(ctx context.Context, taskID string, mentions []string, content string) {
	// Query subtasks that are running or input_required, along with agent names
	rows, err := h.DB.Query(ctx,
		`SELECT s.id, s.agent_id, a.name
		 FROM subtasks s
		 JOIN agents a ON a.id = s.agent_id
		 WHERE s.task_id = $1 AND s.status IN ('running', 'input_required')`, taskID)
	if err != nil {
		return
	}
	defer rows.Close()

	type activeSubtask struct {
		subtaskID, agentID, agentName string
	}
	var active []activeSubtask
	for rows.Next() {
		var as activeSubtask
		if err := rows.Scan(&as.subtaskID, &as.agentID, &as.agentName); err != nil {
			continue
		}
		active = append(active, as)
	}

	mentionSet := make(map[string]bool, len(mentions))
	for _, m := range mentions {
		mentionSet[strings.ToLower(m)] = true
	}

	for _, as := range active {
		if mentionSet[strings.ToLower(as.agentName)] {
			go func(sub activeSubtask) {
				if err := h.Executor.SendFollowUp(ctx, taskID, sub.subtaskID, sub.agentID, content); err != nil {
					log.Printf("follow-up to %s failed: %v", sub.agentName, err)
				}
			}(as)
		}
	}
}
```

Update `Send` to call `routeToAgents` instead of `signalWaitingAgents`.

- [ ] **Step 2: Remove import of `adapter` package**

- [ ] **Step 3: Wire main.go — replace adapters with A2A client**

```go
// Replace:
// adapters := map[string]adapter.AgentAdapter{...}
// With:
a2aClient := a2a.NewClient()
a2aResolver := a2a.NewResolver(pool)

exec := &executor.DAGExecutor{
    DB: pool, Broker: broker, EventStore: eventStore,
    Audit: auditLogger, Orchestrator: orch,
    A2A: a2aClient, Resolver: a2aResolver,
}
```

Update `AgentHandler` to include `Resolver`:

```go
agentH := &handlers.AgentHandler{DB: pool, Resolver: a2aResolver}
```

Add the discover route:

```go
r.Post("/agents/discover", agentH.Discover)
```

Remove `import "taskhub/internal/adapter"`.

- [ ] **Step 4: Verify full backend compiles**

```bash
go build ./cmd/server/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/handlers/messages.go cmd/server/main.go
git commit -m "feat: wire A2A client into main.go, update message routing

Replace adapter map with A2A client. Message @mention now calls
SendFollowUp for running/input_required agents."
```

---

### Task 9: Remove Old Adapter Layer

**Files:**
- Delete: `internal/adapter/adapter.go`
- Delete: `internal/adapter/http_poll.go`
- Delete: `internal/adapter/native.go`
- Delete: `internal/adapter/template.go`
- Delete: `internal/adapter/jsonpath.go`

- [ ] **Step 1: Delete adapter directory**

```bash
rm -rf internal/adapter/
```

- [ ] **Step 2: Remove unused dependencies from go.mod**

```bash
go mod tidy
```

This should remove `github.com/PaesslerAG/jsonpath` and `github.com/PaesslerAG/gval`.

- [ ] **Step 3: Verify full build**

```bash
make check
```

Expected: All checks pass (format, lint, typecheck, build).

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor: remove old adapter layer (http_poll + native)

Replaced by A2A protocol. Removes PaesslerAG/jsonpath dependency."
```

---

### Task 10: Frontend Updates

**Files:**
- Modify: `web/lib/types.ts`
- Modify: `web/lib/api.ts`
- Modify: `web/components/agent/AdapterForm.tsx`
- Modify: `web/app/agents/register/page.tsx`

- [ ] **Step 1: Update TypeScript types**

In `web/lib/types.ts`, update Agent and SubTask:

```typescript
export interface Agent {
  id: string;
  org_id: string;
  name: string;
  version: string;
  description: string;
  endpoint: string;
  agent_card_url: string;
  agent_card?: Record<string, unknown>;
  card_fetched_at?: string;
  capabilities: string[];
  skills?: AgentSkill[];
  status: "active" | "inactive" | "degraded";
  created_at: string;
  updated_at: string;
}

export interface AgentSkill {
  id: string;
  name: string;
  description: string;
}

export interface SubTask {
  id: string;
  task_id: string;
  agent_id: string;
  instruction: string;
  depends_on: string[];
  status: "pending" | "running" | "completed" | "failed" | "input_required" | "cancelled" | "blocked";
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  a2a_task_id?: string;
  attempt: number;
  max_attempts: number;
  created_at: string;
  started_at?: string;
  completed_at?: string;
}
```

- [ ] **Step 2: Add discover API call**

In `web/lib/api.ts`, add:

```typescript
agents: {
  // ... existing methods ...
  discover: (orgId: string, url: string) =>
    post<DiscoveredAgent>(`/api/orgs/${orgId}/agents/discover`, { url }),
}
```

Add the DiscoveredAgent type to `types.ts`:

```typescript
export interface DiscoveredAgent {
  name: string;
  description: string;
  version: string;
  url: string;
  skills: AgentSkill[];
  capabilities: string[];
  raw_card: Record<string, unknown>;
}
```

- [ ] **Step 3: Simplify AdapterForm to URL + Discover**

Rewrite `web/components/agent/AdapterForm.tsx` to:
1. Show a URL input field
2. "Discover" button that calls the discover API
3. Display discovered agent info (name, description, skills)
4. Remove all adapter_type, adapter_config, auth_type fields

- [ ] **Step 4: Update register page**

Update `web/app/agents/register/page.tsx` to use the new simplified form.

- [ ] **Step 5: Verify frontend builds**

```bash
cd web && pnpm run build
```

- [ ] **Step 6: Commit**

```bash
git add web/lib/types.ts web/lib/api.ts web/components/agent/AdapterForm.tsx web/app/agents/register/page.tsx
git commit -m "feat(web): update frontend for A2A agent registration

Simplify agent registration to URL + Discover flow. Remove adapter
type/config fields. Update Agent and SubTask types for A2A."
```

---

### Task 11: End-to-End Test with Mock Agent

- [ ] **Step 1: Reset database**

```bash
make db-reset
```

- [ ] **Step 2: Start mock agent**

```bash
go run ./cmd/mockagent --port=9090 &
```

Verify AgentCard: `curl http://localhost:9090/.well-known/agent-card.json`

- [ ] **Step 3: Start backend**

```bash
make dev-backend
```

- [ ] **Step 4: Register mock agent via API**

```bash
# Discover
curl -X POST http://localhost:8080/api/orgs/local-org/agents/discover \
  -H 'Content-Type: application/json' \
  -d '{"url": "http://localhost:9090"}'

# Register (with data from discover response)
curl -X POST http://localhost:8080/api/orgs/local-org/agents \
  -H 'Content-Type: application/json' \
  -d '{"name": "Mock Agent", "endpoint": "http://localhost:9090", "agent_card_url": "http://localhost:9090"}'
```

- [ ] **Step 5: Create a task and verify execution**

```bash
curl -X POST http://localhost:8080/api/orgs/local-org/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title": "Test A2A", "description": "echo:hello from A2A"}'
```

Verify: Task completes, subtask output contains "hello from A2A", message appears in chat.

- [ ] **Step 6: Start frontend, verify UI**

```bash
make dev-frontend
```

Open http://localhost:3000. Verify:
- Agent appears in agent list with name from AgentCard
- Task shows completed status in dashboard
- DAG view shows completed subtask
- Chat shows agent message

- [ ] **Step 7: Run quality gate**

```bash
make check
```

Expected: All checks pass.

- [ ] **Step 8: Commit any fixes**

```bash
git add -A
git commit -m "test: verify A2A integration with mock agent end-to-end"
```
