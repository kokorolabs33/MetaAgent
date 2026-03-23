# Plan 3: Execution Engine

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the core execution engine — event store, SSE broker, LLM orchestrator, task CRUD, DAG executor with polling/signal/retry/blocked propagation, group chat messages, audit logging, and budget enforcement. After this plan, users can create tasks that get decomposed by an LLM orchestrator and executed by external agents with real-time streaming.

**Architecture:** The orchestrator uses an LLM to decompose tasks into a DAG of subtasks. The DAG executor runs subtasks in dependency order, polling external agents via adapters. Events are persisted to PostgreSQL and streamed via SSE. Group chat enables @mention-based human-in-the-loop interaction.

**Tech Stack:** Go 1.26, chi/v5, pgx/v5, Anthropic Claude API (via SDK)

**Spec:** `docs/superpowers/specs/2026-03-22-taskhub-v2-design.md` (Sections 3, 5, 6)

**Depends on:** Plan 1 (foundation) + Plan 2 (agent system)

---

## File Map

| File | Responsibility |
|------|----------------|
| `internal/models/task.go` | Task, SubTask, Plan domain structs |
| `internal/models/event.go` | Event, Message domain structs |
| `internal/events/store.go` | Event store — persist events to DB |
| `internal/events/broker.go` | SSE broker — in-memory fanout + DB catchup |
| `internal/audit/audit.go` | Audit logger — write audit_logs + cost tracking |
| `internal/orchestrator/orchestrator.go` | LLM-based task decomposition (Plan + Replan) |
| `internal/executor/executor.go` | DAG executor — polling loop, concurrency, retry, signal, cancel |
| `internal/executor/recovery.go` | Crash recovery — resume in-progress tasks on startup |
| `internal/handlers/tasks.go` | Task CRUD + cancel + cost handlers |
| `internal/handlers/messages.go` | Group chat message handlers |
| `internal/handlers/stream.go` | SSE streaming handler |

---

## Chunk 1: Task & Event Models

### Task 1: Task and SubTask models

**Files:**
- Create: `internal/models/task.go`

- [ ] **Step 1: Write task models**

```go
// internal/models/task.go
package models

import (
	"encoding/json"
	"time"
)

type Task struct {
	ID           string          `json:"id"`
	OrgID        string          `json:"org_id"`
	Title        string          `json:"title"`
	Description  string          `json:"description"`
	Status       string          `json:"status"` // pending, planning, running, completed, failed, cancelled
	CreatedBy    string          `json:"created_by"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Plan         json.RawMessage `json:"plan,omitempty"`
	Result       json.RawMessage `json:"result,omitempty"`
	Error        string          `json:"error,omitempty"`
	ReplanCount  int             `json:"replan_count"`
	CreatedAt    time.Time       `json:"created_at"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
}

type SubTask struct {
	ID           string          `json:"id"`
	TaskID       string          `json:"task_id"`
	AgentID      string          `json:"agent_id"`
	Instruction  string          `json:"instruction"`
	DependsOn    []string        `json:"depends_on"`
	Status       string          `json:"status"` // pending, running, completed, failed, waiting_for_input, cancelled, blocked
	Input        json.RawMessage `json:"input,omitempty"`
	Output       json.RawMessage `json:"output,omitempty"`
	Error        string          `json:"error,omitempty"`
	PollJobID    string          `json:"poll_job_id,omitempty"`
	PollEndpoint string          `json:"poll_endpoint,omitempty"`
	Attempt      int             `json:"attempt"`
	MaxAttempts  int             `json:"max_attempts"`
	CreatedAt    time.Time       `json:"created_at"`
	StartedAt    *time.Time      `json:"started_at,omitempty"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
}

// TaskWithSubtasks is the combined view for task detail API.
type TaskWithSubtasks struct {
	Task
	SubTasks []SubTask `json:"subtasks"`
}

// ExecutionPlan is what the orchestrator returns.
type ExecutionPlan struct {
	Summary  string        `json:"summary"`
	SubTasks []PlanSubTask `json:"subtasks"`
}

type PlanSubTask struct {
	ID          string   `json:"id"`       // temp ID for dependency references
	AgentID     string   `json:"agent_id"`
	AgentName   string   `json:"agent_name"`
	Instruction string   `json:"instruction"`
	DependsOn   []string `json:"depends_on"` // references to other PlanSubTask IDs
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/models/task.go
git commit -m "feat: add Task, SubTask, ExecutionPlan models"
```

---

### Task 2: Event and Message models

**Files:**
- Create: `internal/models/event.go`

- [ ] **Step 1: Write event and message models**

```go
// internal/models/event.go
package models

import (
	"encoding/json"
	"time"
)

type Event struct {
	ID        string          `json:"id"`
	TaskID    string          `json:"task_id"`
	SubtaskID string          `json:"subtask_id,omitempty"`
	Type      string          `json:"type"`
	ActorType string          `json:"actor_type"` // system, agent, user
	ActorID   string          `json:"actor_id,omitempty"`
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
}

type Message struct {
	ID         string          `json:"id"`
	TaskID     string          `json:"task_id"`
	SenderType string          `json:"sender_type"` // agent, user, system
	SenderID   string          `json:"sender_id,omitempty"`
	SenderName string          `json:"sender_name"`
	Content    string          `json:"content"`
	Mentions   []string        `json:"mentions"`
	Metadata   json.RawMessage `json:"metadata,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/models/event.go
git commit -m "feat: add Event and Message models"
```

---

## Chunk 2: Event Store & SSE Broker

### Task 3: Event store

**Files:**
- Create: `internal/events/store.go`

- [ ] **Step 1: Write event store**

```go
// internal/events/store.go
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"taskhub/internal/models"
)

type Store struct {
	DB *pgxpool.Pool
}

// Save persists an event to the database and returns the saved event with generated ID and timestamp.
func (s *Store) Save(ctx context.Context, taskID, subtaskID, eventType, actorType, actorID string, data any) (*models.Event, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal event data: %w", err)
	}

	evt := &models.Event{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		SubtaskID: subtaskID,
		Type:      eventType,
		ActorType: actorType,
		ActorID:   actorID,
		Data:      dataJSON,
		CreatedAt: time.Now(),
	}

	_, err = s.DB.Exec(ctx,
		`INSERT INTO events (id, task_id, subtask_id, type, actor_type, actor_id, data, created_at)
		 VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8)`,
		evt.ID, evt.TaskID, evt.SubtaskID, evt.Type, evt.ActorType, evt.ActorID, evt.Data, evt.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert event: %w", err)
	}

	return evt, nil
}

// ListByTask returns all events for a task, ordered by created_at.
func (s *Store) ListByTask(ctx context.Context, taskID string) ([]models.Event, error) {
	return s.listByTaskAfter(ctx, taskID, time.Time{}, "")
}

// ListByTaskAfter returns events after a given (created_at, id) pair for SSE catchup.
func (s *Store) ListByTaskAfter(ctx context.Context, taskID string, afterTime time.Time, afterID string) ([]models.Event, error) {
	return s.listByTaskAfter(ctx, taskID, afterTime, afterID)
}

func (s *Store) listByTaskAfter(ctx context.Context, taskID string, afterTime time.Time, afterID string) ([]models.Event, error) {
	var rows pgx.Rows
	var err error

	if afterID != "" {
		rows, err = s.DB.Query(ctx,
			`SELECT id, task_id, COALESCE(subtask_id, ''), type, actor_type, COALESCE(actor_id, ''), data, created_at
			 FROM events
			 WHERE task_id = $1 AND (created_at, id) > ($2, $3)
			 ORDER BY created_at, id`,
			taskID, afterTime, afterID)
	} else {
		rows, err = s.DB.Query(ctx,
			`SELECT id, task_id, COALESCE(subtask_id, ''), type, actor_type, COALESCE(actor_id, ''), data, created_at
			 FROM events
			 WHERE task_id = $1
			 ORDER BY created_at, id`,
			taskID)
	}
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var e models.Event
		if err := rows.Scan(&e.ID, &e.TaskID, &e.SubtaskID, &e.Type, &e.ActorType, &e.ActorID, &e.Data, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return events, nil
}

// GetByID returns a single event by ID (used for Last-Event-ID lookup).
func (s *Store) GetByID(ctx context.Context, eventID string) (*models.Event, error) {
	var e models.Event
	err := s.DB.QueryRow(ctx,
		`SELECT id, task_id, COALESCE(subtask_id, ''), type, actor_type, COALESCE(actor_id, ''), data, created_at
		 FROM events WHERE id = $1`, eventID).
		Scan(&e.ID, &e.TaskID, &e.SubtaskID, &e.Type, &e.ActorType, &e.ActorID, &e.Data, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
```

Note: add `"github.com/jackc/pgx/v5"` to the import for the `pgx.Rows` type.

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/events/store.go
git commit -m "feat: add event store — persist and query events from PostgreSQL"
```

---

### Task 4: SSE broker

**Files:**
- Create: `internal/events/broker.go`

- [ ] **Step 1: Write SSE broker with in-memory fanout**

```go
// internal/events/broker.go
package events

import (
	"sync"

	"taskhub/internal/models"
)

// Broker provides in-memory pub/sub for real-time event fanout.
// The event store handles persistence; the broker handles live delivery.
type Broker struct {
	mu          sync.RWMutex
	subscribers map[string][]chan *models.Event // task_id → channels
}

func NewBroker() *Broker {
	return &Broker{
		subscribers: make(map[string][]chan *models.Event),
	}
}

// Subscribe returns a channel that receives events for a given task.
// Call Unsubscribe when done.
func (b *Broker) Subscribe(taskID string) chan *models.Event {
	ch := make(chan *models.Event, 64)
	b.mu.Lock()
	b.subscribers[taskID] = append(b.subscribers[taskID], ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a channel from subscriptions.
func (b *Broker) Unsubscribe(taskID string, ch chan *models.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subscribers[taskID]
	for i, sub := range subs {
		if sub == ch {
			b.subscribers[taskID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
	if len(b.subscribers[taskID]) == 0 {
		delete(b.subscribers, taskID)
	}
}

// Publish sends an event to all subscribers of a task.
func (b *Broker) Publish(event *models.Event) {
	b.mu.RLock()
	subs := b.subscribers[event.TaskID]
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// Subscriber too slow, drop event (they can catch up from DB)
		}
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/events/broker.go
git commit -m "feat: add SSE broker — in-memory event fanout per task"
```

---

## Chunk 3: Audit Logger

### Task 5: Audit logger

**Files:**
- Create: `internal/audit/audit.go`

- [ ] **Step 1: Write audit logger**

```go
// internal/audit/audit.go
package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Logger struct {
	DB *pgxpool.Pool
}

type Entry struct {
	OrgID        string
	TaskID       string
	SubtaskID    string
	AgentID      string
	ActorType    string
	ActorID      string
	Action       string
	ResourceType string
	ResourceID   string
	Details      any
	Model        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	Endpoint     string
	LatencyMs    int
	StatusCode   int
}

func (l *Logger) Log(ctx context.Context, e Entry) error {
	detailsJSON, err := json.Marshal(e.Details)
	if err != nil {
		detailsJSON = []byte("{}")
	}

	_, err = l.DB.Exec(ctx,
		`INSERT INTO audit_logs (id, org_id, task_id, subtask_id, agent_id, actor_type, actor_id,
		 action, resource_type, resource_id, details, model, input_tokens, output_tokens, cost_usd,
		 endpoint_called, latency_ms, status_code)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), $6, $7,
		 $8, $9, NULLIF($10,''), $11, NULLIF($12,''), $13, $14, $15,
		 NULLIF($16,''), NULLIF($17,0), NULLIF($18,0))`,
		uuid.New().String(), e.OrgID, e.TaskID, e.SubtaskID, e.AgentID,
		e.ActorType, e.ActorID, e.Action, e.ResourceType, e.ResourceID,
		detailsJSON, e.Model, e.InputTokens, e.OutputTokens, e.CostUSD,
		e.Endpoint, e.LatencyMs, e.StatusCode)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

// GetTaskCost returns total cost for a task.
func (l *Logger) GetTaskCost(ctx context.Context, taskID string) (float64, int, int, error) {
	var totalCost float64
	var totalInput, totalOutput int
	err := l.DB.QueryRow(ctx,
		`SELECT COALESCE(SUM(cost_usd),0), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0)
		 FROM audit_logs WHERE task_id = $1`, taskID).
		Scan(&totalCost, &totalInput, &totalOutput)
	return totalCost, totalInput, totalOutput, err
}

// GetOrgMonthlySpend returns the current month's spend for an org.
func (l *Logger) GetOrgMonthlySpend(ctx context.Context, orgID string) (float64, error) {
	var spent float64
	err := l.DB.QueryRow(ctx,
		`SELECT COALESCE(SUM(cost_usd), 0) FROM audit_logs
		 WHERE org_id = $1 AND created_at >= date_trunc('month', NOW())`,
		orgID).Scan(&spent)
	return spent, err
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/audit/audit.go
git commit -m "feat: add audit logger — write audit entries + cost queries"
```

---

## Chunk 4: Orchestrator

### Task 6: LLM orchestrator

**Files:**
- Create: `internal/orchestrator/orchestrator.go`

- [ ] **Step 1: Write orchestrator**

The orchestrator calls Claude API to decompose tasks. It takes the task description + available agents and returns an ExecutionPlan (list of subtasks with dependencies).

```go
// internal/orchestrator/orchestrator.go
package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"taskhub/internal/models"
)

// Orchestrator decomposes tasks into subtask DAGs using an LLM.
type Orchestrator struct {
	// Future: use Anthropic Go SDK. MVP: use claude CLI.
}

const planPrompt = `You are a task decomposition engine. Given a task and available agents, break the task into subtasks.

RULES:
- Each subtask is assigned to exactly one agent by agent_id
- Subtasks can depend on other subtasks (DAG - no cycles)
- Use depends_on with subtask IDs to define execution order
- Subtasks with no dependencies can run in parallel
- Give each subtask a unique short ID like "s1", "s2", etc.
- Return ONLY valid JSON, no markdown or explanation

Available agents:
%s

Respond with ONLY this JSON:
{"summary":"brief plan summary","subtasks":[{"id":"s1","agent_id":"...","agent_name":"...","instruction":"specific instruction","depends_on":[]}]}`

const replanPrompt = `A subtask in the execution plan has failed. You need to create replacement subtasks.

Original task: %s
Current plan state: %s
Failed subtask: %s (agent: %s, instruction: %s)
Error: %s

Create replacement subtasks for the failed one and any dependents that were blocked.
Already-completed subtasks should NOT be regenerated.
Respond with ONLY valid JSON in the same format as the original plan.`

func (o *Orchestrator) Plan(ctx context.Context, task models.Task, agents []models.Agent) (*models.ExecutionPlan, error) {
	agentDesc := buildAgentDescription(agents)
	prompt := fmt.Sprintf(planPrompt, agentDesc)
	userMsg := fmt.Sprintf("Task: %s\n\nDescription: %s", task.Title, task.Description)

	response, err := callLLM(ctx, prompt, userMsg)
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	var plan models.ExecutionPlan
	if err := json.Unmarshal([]byte(response), &plan); err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}
	return &plan, nil
}

func (o *Orchestrator) Replan(ctx context.Context, task models.Task, failed models.SubTask, agents []models.Agent) (*models.ExecutionPlan, error) {
	planJSON, _ := json.Marshal(task.Plan)
	prompt := fmt.Sprintf(replanPrompt, task.Title, string(planJSON), failed.ID, failed.AgentID, failed.Instruction, failed.Error)

	response, err := callLLM(ctx, "You are a task replanning engine. Respond with ONLY valid JSON.", prompt)
	if err != nil {
		return nil, fmt.Errorf("replan llm call: %w", err)
	}

	var plan models.ExecutionPlan
	if err := json.Unmarshal([]byte(response), &plan); err != nil {
		return nil, fmt.Errorf("parse replan: %w", err)
	}
	return &plan, nil
}

func buildAgentDescription(agents []models.Agent) string {
	var sb strings.Builder
	for _, a := range agents {
		fmt.Fprintf(&sb, "- ID: %s | Name: %s | Capabilities: %v | Description: %s\n",
			a.ID, a.Name, a.Capabilities, a.Description)
	}
	return sb.String()
}

// callLLM uses claude CLI for MVP. Replace with Anthropic Go SDK later.
func callLLM(ctx context.Context, systemPrompt, userMsg string) (string, error) {
	cmd := exec.CommandContext(ctx, "claude", "--print", "-s", systemPrompt, userMsg)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("claude cli: %w", err)
	}
	// Strip markdown code fences if present
	result := strings.TrimSpace(string(out))
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	return strings.TrimSpace(result), nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/orchestrator/
git commit -m "feat: add LLM orchestrator — task decomposition via claude CLI"
```

---

## Chunk 5: DAG Executor

### Task 7: DAG executor

**Files:**
- Create: `internal/executor/executor.go`

- [ ] **Step 1: Write DAG executor**

This is the core of the system. The executor:
1. Creates subtask records from the plan
2. Runs a DAG loop finding ready subtasks
3. Spawns goroutines per subtask for polling
4. Handles needs_input via signal channels
5. Manages retry and blocked propagation
6. Checks budget before each submit

Key struct:

```go
type DAGExecutor struct {
    DB           *pgxpool.Pool
    Broker       *events.Broker
    EventStore   *events.Store
    Audit        *audit.Logger
    Orchestrator *orchestrator.Orchestrator
    Adapters     map[string]adapter.AgentAdapter // adapter_type → adapter

    signals  sync.Map // subtask_id → chan adapter.UserInput
    cancels  sync.Map // task_id → context.CancelFunc

    maxConcurrent      int // default 10
    maxConcurrentAgent int // default 3
}
```

Key methods:
- `Execute(ctx, task) error` — main entry: plan → create subtasks → run DAG loop
- `Cancel(ctx, taskID) error` — cancel a running task
- `Signal(ctx, taskID, input) error` — deliver user input to a waiting subtask
- `runSubtask(ctx, subtask, agent)` — goroutine per subtask: submit → poll loop → handle status
- `findReadySubtasks(subtasks) []SubTask` — find pending subtasks with all deps completed
- `propagateBlocked(ctx, failedID, subtasks)` — mark downstream subtasks as blocked
- `buildSubtaskInput(st, allSubtasks, agents) map[string]any` — inject upstream outputs
- `checkBudget(ctx, orgID) error` — verify monthly spend < budget

The executor publishes events via EventStore.Save() + Broker.Publish() for every state transition.

Implementation should follow the spec exactly (Section 3 of design doc). The polling loop uses exponential backoff: 1s, 2s, 5s, 10s, 30s cap.

- [ ] **Step 2: Verify compilation**

```bash
go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/executor/executor.go
git commit -m "feat: add DAG executor — polling, signal, retry, blocked propagation, budget check"
```

---

### Task 8: Crash recovery

**Files:**
- Create: `internal/executor/recovery.go`

- [ ] **Step 1: Write crash recovery**

```go
// internal/executor/recovery.go
package executor

import (
	"context"
	"log"
)

// Recover scans the database for in-progress tasks and resumes execution.
// Called once on server startup.
func (e *DAGExecutor) Recover(ctx context.Context) {
	// 1. Find subtasks with status='running' and poll_job_id IS NOT NULL
	//    → resume polling with existing job handle (do NOT re-submit)

	// 2. Find subtasks with status='running' and poll_job_id IS NULL
	//    → crashed between submit and storing job_id, mark as failed (retry will re-submit)

	// 3. Find subtasks with status='waiting_for_input'
	//    → re-register signal channels

	// 4. Find tasks with status='running'
	//    → create fresh cancel context, re-enter DAG loop

	rows, err := e.DB.Query(ctx,
		`SELECT DISTINCT t.id FROM tasks t
		 WHERE t.status = 'running'`)
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

	for _, taskID := range taskIDs {
		log.Printf("recovery: resuming task %s", taskID)

		// Mark orphaned running subtasks (no poll_job_id) as failed
		_, _ = e.DB.Exec(ctx,
			`UPDATE subtasks SET status = 'failed', error = 'server restarted before job submission completed'
			 WHERE task_id = $1 AND status = 'running' AND poll_job_id IS NULL`, taskID)

		// Resume the task's DAG loop in a goroutine
		go func(tid string) {
			if err := e.resumeTask(ctx, tid); err != nil {
				log.Printf("recovery: task %s failed: %v", tid, err)
			}
		}(taskID)
	}

	if len(taskIDs) > 0 {
		log.Printf("recovery: resumed %d tasks", len(taskIDs))
	}
}

func (e *DAGExecutor) resumeTask(ctx context.Context, taskID string) error {
	// Load task from DB
	var task models.Task
	// ... query task by ID
	// Create cancel context, store in cancels map
	// Load all subtasks
	// Re-enter DAG loop (same as Execute but skip planning step)
	// For running subtasks with poll_job_id, resume polling
	// For waiting_for_input subtasks, re-register signal channels
	return nil // TODO: implement full recovery
}
```

Note: Full implementation will be filled in by the implementing agent. The structure and logic are specified; the agent should complete the `resumeTask` method following the spec's crash recovery rules.

- [ ] **Step 2: Commit**

```bash
git add internal/executor/recovery.go
git commit -m "feat: add crash recovery — resume in-progress tasks on startup"
```

---

## Chunk 6: Task & Message Handlers

### Task 9: Task handlers

**Files:**
- Create: `internal/handlers/tasks.go`

- [ ] **Step 1: Write task handlers**

`TaskHandler` struct with `DB`, `Executor`, `EventStore`, `Audit` dependencies.

Handlers:
1. **Create** `POST /tasks` — validate title (required), create task record (status=pending), spawn `executor.Execute()` in goroutine, return 201
2. **List** `GET /tasks` — paginated list of tasks for org, support `status` filter
3. **Get** `GET /tasks/:id` — fetch task + subtasks in one response (`TaskWithSubtasks`)
4. **Cancel** `POST /tasks/:id/cancel` — call `executor.Cancel()`, return 200
5. **GetCost** `GET /tasks/:id/cost` — query audit_logs for cost summary
6. **ListSubtasks** `GET /tasks/:id/subtasks` — list subtasks for a task

- [ ] **Step 2: Commit**

```bash
git add internal/handlers/tasks.go
git commit -m "feat: add task handlers — create, list, get, cancel, cost, subtasks"
```

---

### Task 10: Message handlers

**Files:**
- Create: `internal/handlers/messages.go`

- [ ] **Step 1: Write message handlers**

`MessageHandler` struct with `DB`, `Executor`, `EventStore`, `Broker`.

Handlers:
1. **List** `GET /tasks/:id/messages` — fetch all messages for a task, ordered by created_at
2. **Send** `POST /tasks/:id/messages` — user sends message. Parse @mentions from content. If message @mentions an agent, find the waiting subtask for that agent and call `executor.Signal()`. Save message to DB. Publish message event.

@mention parsing: regex `@(\S+)` — extract agent names, look up agent IDs.

- [ ] **Step 2: Commit**

```bash
git add internal/handlers/messages.go
git commit -m "feat: add message handlers — list and send with @mention parsing"
```

---

### Task 11: SSE streaming handler

**Files:**
- Create: `internal/handlers/stream.go`

- [ ] **Step 1: Write SSE handler**

`StreamHandler` struct with `EventStore`, `Broker`.

`Stream` `GET /tasks/:id/events` — SSE endpoint:
1. Set SSE headers: `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`
2. Check `Last-Event-ID` header → if present, look up event by ID to get created_at, replay events after that point
3. If no Last-Event-ID → replay all events from DB (full catchup)
4. Subscribe to broker for live events
5. Loop: write events as SSE (`id: <event_id>\ndata: <json>\n\n`), flush after each
6. On client disconnect (context done) → unsubscribe

- [ ] **Step 2: Commit**

```bash
git add internal/handlers/stream.go
git commit -m "feat: add SSE streaming handler with DB catchup and Last-Event-ID support"
```

---

## Chunk 7: Wire & Verify

### Task 12: Wire task, message, and stream routes

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add new dependencies and routes**

Add to handler setup:
```go
eventStore := &events.Store{DB: pool}
broker := events.NewBroker()
auditLogger := &audit.Logger{DB: pool}
orch := &orchestrator.Orchestrator{}

adapters := map[string]adapter.AgentAdapter{
    "http_poll": adapter.NewHTTPPollAdapter(),
    "native":    adapter.NewNativeAdapter(),
}

executor := &executor.DAGExecutor{
    DB: pool, Broker: broker, EventStore: eventStore,
    Audit: auditLogger, Orchestrator: orch, Adapters: adapters,
}

// Crash recovery
executor.Recover(ctx)

taskH := &handlers.TaskHandler{DB: pool, Executor: executor, EventStore: eventStore, Audit: auditLogger}
msgH := &handlers.MessageHandler{DB: pool, Executor: executor, EventStore: eventStore, Broker: broker}
streamH := &handlers.StreamHandler{EventStore: eventStore, Broker: broker}
```

Add routes in org-scoped group:
```go
// Tasks
r.With(rbac.RequireRole("member")).Post("/tasks", taskH.Create)
r.Get("/tasks", taskH.List)
r.Get("/tasks/{id}", taskH.Get)
r.Post("/tasks/{id}/cancel", taskH.Cancel)
r.Get("/tasks/{id}/cost", taskH.GetCost)
r.Get("/tasks/{id}/subtasks", taskH.ListSubtasks)

// Messages
r.Get("/tasks/{id}/messages", msgH.List)
r.With(rbac.RequireRole("member")).Post("/tasks/{id}/messages", msgH.Send)

// SSE
r.Get("/tasks/{id}/events", streamH.Stream)
```

- [ ] **Step 2: Verify compilation**

```bash
go build ./cmd/server
```

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire task, message, SSE routes + executor with crash recovery"
```

---

### Task 13: Update TypeScript types

**Files:**
- Modify: `web/lib/types.ts`

- [ ] **Step 1: Add Task, SubTask, Event, Message interfaces**

```typescript
// Task System
export interface Task {
  id: string;
  org_id: string;
  title: string;
  description: string;
  status: "pending" | "planning" | "running" | "completed" | "failed" | "cancelled";
  created_by: string;
  metadata?: Record<string, unknown>;
  plan?: Record<string, unknown>;
  result?: Record<string, unknown>;
  error?: string;
  replan_count: number;
  created_at: string;
  completed_at?: string;
}

export interface SubTask {
  id: string;
  task_id: string;
  agent_id: string;
  instruction: string;
  depends_on: string[];
  status: "pending" | "running" | "completed" | "failed" | "waiting_for_input" | "cancelled" | "blocked";
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error?: string;
  poll_job_id?: string;
  attempt: number;
  max_attempts: number;
  created_at: string;
  started_at?: string;
  completed_at?: string;
}

export interface TaskWithSubtasks extends Task {
  subtasks: SubTask[];
}

// Events
export interface TaskEvent {
  id: string;
  task_id: string;
  subtask_id?: string;
  type: string;
  actor_type: "system" | "agent" | "user";
  actor_id?: string;
  data: Record<string, unknown>;
  created_at: string;
}

// Messages
export interface Message {
  id: string;
  task_id: string;
  sender_type: "agent" | "user" | "system";
  sender_id?: string;
  sender_name: string;
  content: string;
  mentions: string[];
  metadata?: Record<string, unknown>;
  created_at: string;
}
```

- [ ] **Step 2: Commit**

```bash
git add web/lib/types.ts
git commit -m "feat(web): add Task, SubTask, Event, Message TypeScript types"
```

---

### Task 14: End-to-end verification

- [ ] **Step 1: Run all tests**

```bash
go test ./... -v
```

- [ ] **Step 2: Run linter**

```bash
golangci-lint run ./...
```

- [ ] **Step 3: Build all binaries**

```bash
go build ./cmd/server && go build ./cmd/mockagent
```

- [ ] **Step 4: Clean up**

```bash
rm -f server mockagent
```
