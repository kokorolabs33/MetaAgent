# TaskHub MVP Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build TaskHub — an enterprise multi-agent collaboration platform with a Go backend and Next.js 15 frontend.

**Architecture:** A Master Agent decomposes tasks and coordinates Team Agents sequentially; agents communicate via a shared Channel document (blackboard pattern); real-time updates stream to the frontend via SSE.

**Tech Stack:** Go + chi + pgx + OpenAI SDK, Next.js 15 + TailwindCSS 4 + @xyflow/react + Zustand + shadcn/ui

---

## File Map

### Backend (`backend/`)

| File | Responsibility |
|------|----------------|
| `cmd/server/main.go` | Entry point: wire dependencies, run HTTP server |
| `internal/config/config.go` | Load env vars into typed Config struct |
| `internal/db/db.go` | Open pgx connection pool |
| `internal/db/migrate.go` | Run migrations on startup |
| `migrations/001_init.sql` | Full schema (agents, tasks, channels, messages, channel_agents) |
| `internal/models/models.go` | Domain structs: Agent, Task, Channel, Message, ChannelAgent |
| `internal/seed/seed.go` | Upsert 4 default agents on startup |
| `internal/openai/client.go` | Thin OpenAI chat completion client |
| `internal/sse/broker.go` | SSE broker: register streams, broadcast events per task |
| `internal/master/agent.go` | Decompose task via LLM, orchestrate team agents sequentially |
| `internal/handlers/agents.go` | GET/POST /api/agents |
| `internal/handlers/tasks.go` | GET /api/tasks, GET /api/tasks/:id |
| `internal/handlers/channels.go` | GET /api/channels/:id, GET /api/channels/:id/stream |
| `go.mod` | Go module definition |
| `.env.example` | Documented env vars |

### Frontend (`web/`)

| File | Responsibility |
|------|----------------|
| `lib/types.ts` | TypeScript interfaces matching Go models |
| `lib/api.ts` | Typed fetch functions for all backend endpoints |
| `lib/sse.ts` | SSE event source manager + typed event parser |
| `lib/store.ts` | Zustand store with all state + actions |
| `components/task/TaskBar.tsx` | Task input field + submit button |
| `components/topology/AgentTopology.tsx` | React Flow visualization of Master + Team Agents |
| `components/channel/DocumentViewer.tsx` | Markdown display of shared Channel.Document |
| `components/channel/MessageFeed.tsx` | Scrolling list of agent messages |
| `components/channel/ChannelPanel.tsx` | Container: Document above, Messages below |
| `app/layout.tsx` | Root layout: dark theme, global fonts |
| `app/page.tsx` | Main page: TaskBar top, Topology left, Channel right |
| `.env.example` | Documented env vars |

---

## Chunk 1: Backend Foundation

### Task 1: Go Module + Dependencies

**Files:**
- Create: `backend/go.mod`
- Create: `backend/.env.example`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/jasper/Documents/code/taskhub
mkdir -p backend/cmd/server
cd backend
go mod init taskhub
```

- [ ] **Step 2: Install dependencies**

```bash
cd /Users/jasper/Documents/code/taskhub/backend
go get github.com/go-chi/chi/v5
go get github.com/go-chi/cors
go get github.com/jackc/pgx/v5
go get github.com/google/uuid
go get github.com/joho/godotenv
go get github.com/sashabaranov/go-openai
```

- [ ] **Step 3: Create .env.example**

```
DATABASE_URL=postgres://localhost:5432/taskhub?sslmode=disable
OPENAI_API_KEY=sk-...
PORT=8080
```

- [ ] **Step 4: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add backend/
git commit -m "feat(backend): initialize Go module with dependencies"
```

---

### Task 2: Config + Database Connection

**Files:**
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/db/db.go`
- Create: `backend/internal/db/migrate.go`

- [ ] **Step 1: Create config package**

```go
// backend/internal/config/config.go
package config

import (
    "log"
    "os"

    "github.com/joho/godotenv"
)

type Config struct {
    DatabaseURL  string
    OpenAIAPIKey string
    Port         string
}

func Load() *Config {
    _ = godotenv.Load()
    cfg := &Config{
        DatabaseURL:  getEnv("DATABASE_URL", "postgres://localhost:5432/taskhub?sslmode=disable"),
        OpenAIAPIKey: getEnv("OPENAI_API_KEY", ""),
        Port:         getEnv("PORT", "8080"),
    }
    if cfg.OpenAIAPIKey == "" {
        log.Println("WARNING: OPENAI_API_KEY not set")
    }
    return cfg
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}
```

- [ ] **Step 2: Create db package**

```go
// backend/internal/db/db.go
package db

import (
    "context"
    "database/sql"
    "fmt"

    _ "github.com/jackc/pgx/v5/stdlib"
)

func Open(databaseURL string) (*sql.DB, error) {
    db, err := sql.Open("pgx", databaseURL)
    if err != nil {
        return nil, fmt.Errorf("open db: %w", err)
    }
    if err := db.PingContext(context.Background()); err != nil {
        return nil, fmt.Errorf("ping db: %w", err)
    }
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    return db, nil
}
```

- [ ] **Step 3: Create migration runner**

```go
// backend/internal/db/migrate.go
package db

import (
    "database/sql"
    "fmt"
    "os"
    "path/filepath"
    "runtime"
)

func RunMigrations(db *sql.DB) error {
    _, filename, _, _ := runtime.Caller(0)
    // migrations dir is relative to project root
    migrationsDir := filepath.Join(filepath.Dir(filename), "..", "..", "..", "migrations")

    sql, err := os.ReadFile(filepath.Join(migrationsDir, "001_init.sql"))
    if err != nil {
        return fmt.Errorf("read migration: %w", err)
    }
    if _, err := db.Exec(string(sql)); err != nil {
        return fmt.Errorf("run migration: %w", err)
    }
    return nil
}
```

- [ ] **Step 4: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add backend/
git commit -m "feat(backend): add config loading and db connection"
```

---

### Task 3: Database Schema

**Files:**
- Create: `backend/migrations/001_init.sql`

- [ ] **Step 1: Write schema**

```sql
-- backend/migrations/001_init.sql

CREATE TABLE IF NOT EXISTS agents (
    id           TEXT PRIMARY KEY,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    system_prompt TEXT NOT NULL DEFAULT '',
    capabilities TEXT NOT NULL DEFAULT '[]',
    color        TEXT NOT NULL DEFAULT '#6b7280',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tasks (
    id           TEXT PRIMARY KEY,
    title        TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'pending',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS channels (
    id         TEXT PRIMARY KEY,
    task_id    TEXT NOT NULL REFERENCES tasks(id),
    document   TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS messages (
    id          TEXT PRIMARY KEY,
    channel_id  TEXT NOT NULL REFERENCES channels(id),
    sender_id   TEXT NOT NULL,
    sender_name TEXT NOT NULL,
    content     TEXT NOT NULL,
    type        TEXT NOT NULL DEFAULT 'text',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS channel_agents (
    channel_id TEXT NOT NULL REFERENCES channels(id),
    agent_id   TEXT NOT NULL REFERENCES agents(id),
    status     TEXT NOT NULL DEFAULT 'idle',
    PRIMARY KEY (channel_id, agent_id)
);
```

- [ ] **Step 2: Create local postgres database**

```bash
createdb taskhub 2>/dev/null || echo "db may already exist"
```

- [ ] **Step 3: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add backend/migrations/
git commit -m "feat(backend): add database schema migration"
```

---

### Task 4: Domain Models

**Files:**
- Create: `backend/internal/models/models.go`

- [ ] **Step 1: Write models**

```go
// backend/internal/models/models.go
package models

import (
    "encoding/json"
    "time"
)

type Agent struct {
    ID           string    `json:"id"`
    Name         string    `json:"name"`
    Description  string    `json:"description"`
    SystemPrompt string    `json:"system_prompt"`
    Capabilities []string  `json:"capabilities"`
    Color        string    `json:"color"`
    CreatedAt    time.Time `json:"created_at"`
}

type Task struct {
    ID          string     `json:"id"`
    Title       string     `json:"title"`
    Description string     `json:"description"`
    Status      string     `json:"status"`
    CreatedAt   time.Time  `json:"created_at"`
    CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Channel struct {
    ID        string    `json:"id"`
    TaskID    string    `json:"task_id"`
    Document  string    `json:"document"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
}

type Message struct {
    ID         string    `json:"id"`
    ChannelID  string    `json:"channel_id"`
    SenderID   string    `json:"sender_id"`
    SenderName string    `json:"sender_name"`
    Content    string    `json:"content"`
    Type       string    `json:"type"`
    CreatedAt  time.Time `json:"created_at"`
}

type ChannelAgent struct {
    ChannelID string `json:"channel_id"`
    AgentID   string `json:"agent_id"`
    Status    string `json:"status"`
}

// CapabilitiesToJSON serializes []string to JSON string for DB storage.
func CapabilitiesToJSON(caps []string) string {
    b, _ := json.Marshal(caps)
    return string(b)
}

// JSONToCapabilities deserializes JSON string from DB to []string.
func JSONToCapabilities(s string) []string {
    var caps []string
    _ = json.Unmarshal([]byte(s), &caps)
    return caps
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add backend/internal/models/
git commit -m "feat(backend): add domain models"
```

---

### Task 5: Agent Seed Data

**Files:**
- Create: `backend/internal/seed/seed.go`

- [ ] **Step 1: Write seed logic**

```go
// backend/internal/seed/seed.go
package seed

import (
    "context"
    "database/sql"
    "log"

    "github.com/google/uuid"
    "taskhub/internal/models"
)

var defaultAgents = []models.Agent{
    {
        Name:         "SRE Agent",
        Description:  "Monitors and analyzes infrastructure issues.",
        SystemPrompt: "You are a Senior SRE engineer. You specialize in analyzing infrastructure problems, reading logs, and identifying root causes. Be concise and technical.",
        Capabilities: []string{"analyze_logs", "check_monitoring", "incident_response"},
        Color:        "#ef4444",
    },
    {
        Name:         "Engineering Agent",
        Description:  "Analyzes code and system changes.",
        SystemPrompt: "You are a Senior Software Engineer. You specialize in analyzing code changes, identifying bugs, and suggesting fixes. Be concise and technical.",
        Capabilities: []string{"code_review", "analyze_changes", "debug"},
        Color:        "#3b82f6",
    },
    {
        Name:         "Customer Success Agent",
        Description:  "Handles customer communication.",
        SystemPrompt: "You are a Customer Success Manager. You specialize in drafting clear customer communications about incidents and updates. Be empathetic and professional.",
        Capabilities: []string{"draft_communications", "customer_impact_analysis"},
        Color:        "#10b981",
    },
    {
        Name:         "Documentation Agent",
        Description:  "Creates technical documentation.",
        SystemPrompt: "You are a Technical Writer. You specialize in creating clear documentation, runbooks, and post-mortem reports. Be structured and thorough.",
        Capabilities: []string{"write_documentation", "create_runbooks", "post_mortem"},
        Color:        "#f59e0b",
    },
}

func Run(ctx context.Context, db *sql.DB) error {
    for _, a := range defaultAgents {
        var exists bool
        err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM agents WHERE name = $1)`, a.Name).Scan(&exists)
        if err != nil {
            return err
        }
        if exists {
            continue
        }
        a.ID = uuid.New().String()
        _, err = db.ExecContext(ctx,
            `INSERT INTO agents (id, name, description, system_prompt, capabilities, color) VALUES ($1, $2, $3, $4, $5, $6)`,
            a.ID, a.Name, a.Description, a.SystemPrompt, models.CapabilitiesToJSON(a.Capabilities), a.Color,
        )
        if err != nil {
            return err
        }
        log.Printf("seeded agent: %s", a.Name)
    }
    return nil
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add backend/internal/seed/
git commit -m "feat(backend): add agent seed data"
```

---

## Chunk 2: Backend API Handlers

### Task 6: Agent Handlers

**Files:**
- Create: `backend/internal/handlers/agents.go`

- [ ] **Step 1: Write handlers**

```go
// backend/internal/handlers/agents.go
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

// helpers
func jsonOK(w http.ResponseWriter, v any) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add backend/internal/handlers/
git commit -m "feat(backend): add agent handlers"
```

---

### Task 7: Task + Channel Handlers

**Files:**
- Create: `backend/internal/handlers/tasks.go`
- Create: `backend/internal/handlers/channels.go`

- [ ] **Step 1: Write task handlers**

```go
// backend/internal/handlers/tasks.go
package handlers

import (
    "database/sql"
    "net/http"

    "github.com/go-chi/chi/v5"
    "taskhub/internal/models"
)

type TaskHandler struct {
    DB *sql.DB
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
    rows, err := h.DB.QueryContext(r.Context(),
        `SELECT id, title, description, status, created_at, completed_at FROM tasks ORDER BY created_at DESC`)
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    tasks := []models.Task{}
    for rows.Next() {
        var t models.Task
        if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.CompletedAt); err != nil {
            jsonError(w, err.Error(), http.StatusInternalServerError)
            return
        }
        tasks = append(tasks, t)
    }
    jsonOK(w, tasks)
}

func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    var t models.Task
    err := h.DB.QueryRowContext(r.Context(),
        `SELECT id, title, description, status, created_at, completed_at FROM tasks WHERE id = $1`, id,
    ).Scan(&t.ID, &t.Title, &t.Description, &t.Status, &t.CreatedAt, &t.CompletedAt)
    if err == sql.ErrNoRows {
        jsonError(w, "not found", http.StatusNotFound)
        return
    }
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    jsonOK(w, t)
}
```

- [ ] **Step 2: Write channel handler**

```go
// backend/internal/handlers/channels.go
package handlers

import (
    "database/sql"
    "net/http"

    "github.com/go-chi/chi/v5"
    "taskhub/internal/models"
    "taskhub/internal/sse"
)

type ChannelHandler struct {
    DB     *sql.DB
    Broker *sse.Broker
}

type ChannelDetail struct {
    Channel  models.Channel       `json:"channel"`
    Messages []models.Message     `json:"messages"`
    Agents   []models.ChannelAgent `json:"agents"`
}

func (h *ChannelHandler) Get(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")

    var ch models.Channel
    err := h.DB.QueryRowContext(r.Context(),
        `SELECT id, task_id, document, status, created_at FROM channels WHERE id = $1`, id,
    ).Scan(&ch.ID, &ch.TaskID, &ch.Document, &ch.Status, &ch.CreatedAt)
    if err == sql.ErrNoRows {
        jsonError(w, "not found", http.StatusNotFound)
        return
    }
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    rows, err := h.DB.QueryContext(r.Context(),
        `SELECT id, channel_id, sender_id, sender_name, content, type, created_at FROM messages WHERE channel_id = $1 ORDER BY created_at`, id)
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    messages := []models.Message{}
    for rows.Next() {
        var m models.Message
        if err := rows.Scan(&m.ID, &m.ChannelID, &m.SenderID, &m.SenderName, &m.Content, &m.Type, &m.CreatedAt); err != nil {
            jsonError(w, err.Error(), http.StatusInternalServerError)
            return
        }
        messages = append(messages, m)
    }

    agentRows, err := h.DB.QueryContext(r.Context(),
        `SELECT channel_id, agent_id, status FROM channel_agents WHERE channel_id = $1`, id)
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    defer agentRows.Close()

    agents := []models.ChannelAgent{}
    for agentRows.Next() {
        var ca models.ChannelAgent
        if err := agentRows.Scan(&ca.ChannelID, &ca.AgentID, &ca.Status); err != nil {
            jsonError(w, err.Error(), http.StatusInternalServerError)
            return
        }
        agents = append(agents, ca)
    }

    jsonOK(w, ChannelDetail{Channel: ch, Messages: messages, Agents: agents})
}

func (h *ChannelHandler) Stream(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "SSE not supported", http.StatusInternalServerError)
        return
    }

    ch := h.Broker.Subscribe(id)
    defer h.Broker.Unsubscribe(id, ch)

    for {
        select {
        case event, ok := <-ch:
            if !ok {
                return
            }
            _, _ = w.Write([]byte("data: " + event + "\n\n"))
            flusher.Flush()
        case <-r.Context().Done():
            return
        }
    }
}
```

- [ ] **Step 3: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add backend/internal/handlers/
git commit -m "feat(backend): add task and channel handlers"
```

---

## Chunk 3: Backend Master Agent + SSE

### Task 8: OpenAI Client

**Files:**
- Create: `backend/internal/openai/client.go`

- [ ] **Step 1: Write OpenAI client**

```go
// backend/internal/openai/client.go
package openai

import (
    "context"
    "fmt"

    "github.com/sashabaranov/go-openai"
)

type Client struct {
    inner *openai.Client
}

func New(apiKey string) *Client {
    return &Client{inner: openai.NewClient(apiKey)}
}

// Chat sends messages and returns the assistant reply.
func (c *Client) Chat(ctx context.Context, systemPrompt string, userMessage string) (string, error) {
    resp, err := c.inner.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model: openai.GPT4oMini,
        Messages: []openai.ChatCompletionMessage{
            {Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
            {Role: openai.ChatMessageRoleUser, Content: userMessage},
        },
    })
    if err != nil {
        return "", fmt.Errorf("openai chat: %w", err)
    }
    if len(resp.Choices) == 0 {
        return "", fmt.Errorf("openai: no choices")
    }
    return resp.Choices[0].Message.Content, nil
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add backend/internal/openai/
git commit -m "feat(backend): add OpenAI client wrapper"
```

---

### Task 9: SSE Broker

**Files:**
- Create: `backend/internal/sse/broker.go`

- [ ] **Step 1: Write SSE broker**

```go
// backend/internal/sse/broker.go
package sse

import (
    "encoding/json"
    "sync"
)

// Broker manages SSE subscriptions keyed by channel ID.
type Broker struct {
    mu          sync.RWMutex
    subscribers map[string][]chan string
}

func NewBroker() *Broker {
    return &Broker{subscribers: make(map[string][]chan string)}
}

func (b *Broker) Subscribe(channelID string) chan string {
    ch := make(chan string, 64)
    b.mu.Lock()
    b.subscribers[channelID] = append(b.subscribers[channelID], ch)
    b.mu.Unlock()
    return ch
}

func (b *Broker) Unsubscribe(channelID string, sub chan string) {
    b.mu.Lock()
    defer b.mu.Unlock()
    subs := b.subscribers[channelID]
    for i, s := range subs {
        if s == sub {
            b.subscribers[channelID] = append(subs[:i], subs[i+1:]...)
            close(s)
            return
        }
    }
}

// Publish sends a typed SSE event to all subscribers of a channel.
func (b *Broker) Publish(channelID string, eventType string, data any) {
    payload := map[string]any{"type": eventType, "data": data}
    msg, _ := json.Marshal(payload)

    b.mu.RLock()
    subs := make([]chan string, len(b.subscribers[channelID]))
    copy(subs, b.subscribers[channelID])
    b.mu.RUnlock()

    for _, sub := range subs {
        select {
        case sub <- string(msg):
        default:
            // drop if buffer full
        }
    }
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add backend/internal/sse/
git commit -m "feat(backend): add SSE broker"
```

---

### Task 10: Master Agent Logic

**Files:**
- Create: `backend/internal/master/agent.go`

- [ ] **Step 1: Write master agent**

```go
// backend/internal/master/agent.go
package master

import (
    "context"
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "strings"
    "time"

    "github.com/google/uuid"
    "taskhub/internal/models"
    oai "taskhub/internal/openai"
    "taskhub/internal/sse"
)

const masterSystemPrompt = `You are the Master Agent for TaskHub, an enterprise AI coordination platform.
Your job is to:
1. Analyze the given task
2. Decompose it into subtasks
3. Assign each subtask to the most appropriate team agent based on their capabilities
4. Return a JSON plan

Always respond with valid JSON in this format:
{
  "summary": "brief task summary",
  "subtasks": [
    {
      "agent_id": "agent-id-here",
      "agent_name": "SRE Agent",
      "instruction": "specific instruction for this agent",
      "order": 1
    }
  ]
}`

type SubTask struct {
    AgentID     string `json:"agent_id"`
    AgentName   string `json:"agent_name"`
    Instruction string `json:"instruction"`
    Order       int    `json:"order"`
}

type Plan struct {
    Summary  string    `json:"summary"`
    SubTasks []SubTask `json:"subtasks"`
}

type Agent struct {
    DB     *sql.DB
    OpenAI *oai.Client
    Broker *sse.Broker
}

// Run executes the full master agent pipeline for a task.
// It is called in a goroutine after POST /api/tasks.
func (a *Agent) Run(taskID string, description string) {
    ctx := context.Background()

    // Update task to running
    if _, err := a.DB.ExecContext(ctx, `UPDATE tasks SET status='running' WHERE id=$1`, taskID); err != nil {
        log.Printf("master: update task status: %v", err)
        return
    }

    // Load available agents
    agents, err := a.loadAgents(ctx)
    if err != nil {
        log.Printf("master: load agents: %v", err)
        return
    }

    // Build agent capabilities summary for the LLM prompt
    var agentDesc strings.Builder
    for _, ag := range agents {
        agentDesc.WriteString(fmt.Sprintf("- ID: %s | Name: %s | Capabilities: %v\n",
            ag.ID, ag.Name, ag.Capabilities))
    }

    userMsg := fmt.Sprintf("Task: %s\n\nAvailable agents:\n%s", description, agentDesc.String())

    // Step 1: Ask master LLM to decompose
    planJSON, err := a.OpenAI.Chat(ctx, masterSystemPrompt, userMsg)
    if err != nil {
        log.Printf("master: llm decompose: %v", err)
        a.failTask(ctx, taskID)
        return
    }

    // Strip markdown code fences if present
    planJSON = strings.TrimSpace(planJSON)
    if strings.HasPrefix(planJSON, "```") {
        lines := strings.Split(planJSON, "\n")
        if len(lines) > 2 {
            planJSON = strings.Join(lines[1:len(lines)-1], "\n")
        }
    }

    var plan Plan
    if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
        log.Printf("master: parse plan: %v (raw: %s)", err, planJSON)
        a.failTask(ctx, taskID)
        return
    }

    // Step 2: Create channel
    channelID := uuid.New().String()
    initDoc := fmt.Sprintf("# Task: %s\n\n## Plan\n%s\n\n## Subtasks\n", description, plan.Summary)
    for _, st := range plan.SubTasks {
        initDoc += fmt.Sprintf("%d. **%s**: %s\n", st.Order, st.AgentName, st.Instruction)
    }

    if _, err := a.DB.ExecContext(ctx,
        `INSERT INTO channels (id, task_id, document) VALUES ($1,$2,$3)`,
        channelID, taskID, initDoc,
    ); err != nil {
        log.Printf("master: create channel: %v", err)
        a.failTask(ctx, taskID)
        return
    }

    a.Broker.Publish(channelID, "task_started", map[string]any{"task_id": taskID})
    a.Broker.Publish(channelID, "channel_created", map[string]any{"channel_id": channelID})
    a.Broker.Publish(channelID, "document_updated", map[string]any{"document": initDoc})

    // Step 3: Add selected agents to channel
    agentMap := make(map[string]models.Agent)
    for _, ag := range agents {
        agentMap[ag.ID] = ag
    }
    for _, st := range plan.SubTasks {
        if _, ok := agentMap[st.AgentID]; !ok {
            continue
        }
        if _, err := a.DB.ExecContext(ctx,
            `INSERT INTO channel_agents (channel_id, agent_id, status) VALUES ($1,$2,'idle') ON CONFLICT DO NOTHING`,
            channelID, st.AgentID,
        ); err != nil {
            log.Printf("master: add agent to channel: %v", err)
        }
        a.Broker.Publish(channelID, "agent_joined", map[string]any{
            "agent_id": st.AgentID, "agent_name": st.AgentName,
        })
    }

    // Post master system message
    a.postMessage(ctx, channelID, "master", "Master Agent", plan.Summary, "system")

    // Step 4: Execute subtasks sequentially
    currentDoc := initDoc
    for _, st := range plan.SubTasks {
        ag, ok := agentMap[st.AgentID]
        if !ok {
            log.Printf("master: agent %s not found, skipping", st.AgentID)
            continue
        }

        // Mark agent as working
        a.DB.ExecContext(ctx, `UPDATE channel_agents SET status='working' WHERE channel_id=$1 AND agent_id=$2`, channelID, ag.ID)
        a.Broker.Publish(channelID, "agent_working", map[string]any{
            "agent_id": ag.ID, "agent_name": ag.Name,
        })

        // Build prompt for this agent
        agentUserMsg := fmt.Sprintf("Current shared context:\n%s\n\nYour instruction: %s", currentDoc, st.Instruction)

        result, err := a.OpenAI.Chat(ctx, ag.SystemPrompt, agentUserMsg)
        if err != nil {
            log.Printf("master: agent %s error: %v", ag.Name, err)
            result = fmt.Sprintf("[Error: %v]", err)
        }

        // Append result to document
        currentDoc += fmt.Sprintf("\n\n---\n## %s Result\n%s", ag.Name, result)
        a.DB.ExecContext(ctx, `UPDATE channels SET document=$1 WHERE id=$2`, currentDoc, channelID)

        // Post message
        msg := a.postMessage(ctx, channelID, ag.ID, ag.Name, result, "result")
        if msg != nil {
            a.Broker.Publish(channelID, "message", map[string]any{"message": msg})
        }
        a.Broker.Publish(channelID, "document_updated", map[string]any{"document": currentDoc})

        // Mark agent done
        a.DB.ExecContext(ctx, `UPDATE channel_agents SET status='done' WHERE channel_id=$1 AND agent_id=$2`, channelID, ag.ID)
        a.Broker.Publish(channelID, "agent_done", map[string]any{"agent_id": ag.ID})
    }

    // Step 5: Complete task
    now := time.Now()
    a.DB.ExecContext(ctx, `UPDATE tasks SET status='completed', completed_at=$1 WHERE id=$2`, now, taskID)
    a.DB.ExecContext(ctx, `UPDATE channels SET status='archived' WHERE id=$1`, channelID)

    var task models.Task
    a.DB.QueryRowContext(ctx, `SELECT id, title, description, status, created_at, completed_at FROM tasks WHERE id=$1`, taskID).
        Scan(&task.ID, &task.Title, &task.Description, &task.Status, &task.CreatedAt, &task.CompletedAt)

    a.Broker.Publish(channelID, "task_completed", map[string]any{"task": task})
    log.Printf("master: task %s completed", taskID)
}

func (a *Agent) loadAgents(ctx context.Context) ([]models.Agent, error) {
    rows, err := a.DB.QueryContext(ctx,
        `SELECT id, name, description, system_prompt, capabilities, color, created_at FROM agents`)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var agents []models.Agent
    for rows.Next() {
        var ag models.Agent
        var caps string
        if err := rows.Scan(&ag.ID, &ag.Name, &ag.Description, &ag.SystemPrompt, &caps, &ag.Color, &ag.CreatedAt); err != nil {
            return nil, err
        }
        ag.Capabilities = models.JSONToCapabilities(caps)
        agents = append(agents, ag)
    }
    return agents, nil
}

func (a *Agent) postMessage(ctx context.Context, channelID, senderID, senderName, content, msgType string) *models.Message {
    msg := &models.Message{
        ID:         uuid.New().String(),
        ChannelID:  channelID,
        SenderID:   senderID,
        SenderName: senderName,
        Content:    content,
        Type:       msgType,
        CreatedAt:  time.Now(),
    }
    _, err := a.DB.ExecContext(ctx,
        `INSERT INTO messages (id, channel_id, sender_id, sender_name, content, type) VALUES ($1,$2,$3,$4,$5,$6)`,
        msg.ID, msg.ChannelID, msg.SenderID, msg.SenderName, msg.Content, msg.Type,
    )
    if err != nil {
        log.Printf("master: insert message: %v", err)
        return nil
    }
    return msg
}

func (a *Agent) failTask(ctx context.Context, taskID string) {
    a.DB.ExecContext(ctx, `UPDATE tasks SET status='failed' WHERE id=$1`, taskID)
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add backend/internal/master/
git commit -m "feat(backend): add master agent orchestration logic"
```

---

### Task 11: Task Create Handler + Main Entry Point

**Files:**
- Modify: `backend/internal/handlers/tasks.go` — add Create handler
- Create: `backend/cmd/server/main.go`

- [ ] **Step 1: Add Create handler to tasks.go** — append this to `backend/internal/handlers/tasks.go`:

```go
// Additional fields needed in TaskHandler struct — update the struct definition:
// type TaskHandler struct {
//     DB     *sql.DB
//     Master *master.Agent
// }
//
// Add this import: "taskhub/internal/master"

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Title       string `json:"title"`
        Description string `json:"description"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        jsonError(w, "invalid body", http.StatusBadRequest)
        return
    }
    if req.Description == "" {
        jsonError(w, "description required", http.StatusBadRequest)
        return
    }
    if req.Title == "" {
        // Truncate description to use as title
        req.Title = req.Description
        if len(req.Title) > 80 {
            req.Title = req.Title[:80] + "..."
        }
    }

    t := models.Task{
        ID:          uuid.New().String(),
        Title:       req.Title,
        Description: req.Description,
        Status:      "pending",
        CreatedAt:   time.Now(),
    }
    _, err := h.DB.ExecContext(r.Context(),
        `INSERT INTO tasks (id, title, description, status) VALUES ($1,$2,$3,$4)`,
        t.ID, t.Title, t.Description, t.Status,
    )
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Find the channel for SSE — created by master agent async
    // Return the task immediately; frontend subscribes to channel once channel_created event fires
    go h.Master.Run(t.ID, t.Description)

    w.WriteHeader(http.StatusCreated)
    jsonOK(w, t)
}
```

- [ ] **Step 2: Update TaskHandler struct** — edit the struct in `tasks.go` to include Master:

The full updated `tasks.go` should have:
```go
package handlers

import (
    "database/sql"
    "encoding/json"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "taskhub/internal/master"
    "taskhub/internal/models"
)

type TaskHandler struct {
    DB     *sql.DB
    Master *master.Agent
}
```

- [ ] **Step 3: Write main.go**

```go
// backend/cmd/server/main.go
package main

import (
    "log"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/go-chi/cors"
    "taskhub/internal/config"
    "taskhub/internal/db"
    "taskhub/internal/handlers"
    "taskhub/internal/master"
    oai "taskhub/internal/openai"
    "taskhub/internal/seed"
    "taskhub/internal/sse"
)

func main() {
    cfg := config.Load()

    database, err := db.Open(cfg.DatabaseURL)
    if err != nil {
        log.Fatalf("open db: %v", err)
    }
    defer database.Close()

    if err := db.RunMigrations(database); err != nil {
        log.Fatalf("run migrations: %v", err)
    }
    log.Println("migrations OK")

    if err := seed.Run(nil, database); err != nil {
        log.Fatalf("seed: %v", err)
    }

    openaiClient := oai.New(cfg.OpenAIAPIKey)
    broker := sse.NewBroker()

    masterAgent := &master.Agent{
        DB:     database,
        OpenAI: openaiClient,
        Broker: broker,
    }

    agentH := &handlers.AgentHandler{DB: database}
    taskH := &handlers.TaskHandler{DB: database, Master: masterAgent}
    channelH := &handlers.ChannelHandler{DB: database, Broker: broker}

    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)
    r.Use(cors.Handler(cors.Options{
        AllowedOrigins:   []string{"http://localhost:3000"},
        AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Accept", "Content-Type"},
        AllowCredentials: false,
        MaxAge:           300,
    }))

    r.Route("/api", func(r chi.Router) {
        r.Get("/agents", agentH.List)
        r.Post("/agents", agentH.Create)
        r.Get("/tasks", taskH.List)
        r.Post("/tasks", taskH.Create)
        r.Get("/tasks/{id}", taskH.Get)
        r.Get("/channels/{id}", channelH.Get)
        r.Get("/channels/{id}/stream", channelH.Stream)
    })

    log.Printf("server listening on :%s", cfg.Port)
    if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
        log.Fatalf("listen: %v", err)
    }
}
```

- [ ] **Step 4: Fix seed.Run to accept context**

Update `backend/internal/seed/seed.go` — the `Run` function signature uses `context.Context` but `main.go` passes `nil`. Update the call in main to:
```go
if err := seed.Run(context.Background(), database); err != nil {
```
And add `"context"` import to main.go.

- [ ] **Step 5: Build to verify compilation**

```bash
cd /Users/jasper/Documents/code/taskhub/backend
go build ./...
```
Expected: no errors.

- [ ] **Step 6: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add backend/
git commit -m "feat(backend): wire main server with all handlers and master agent"
```

---

## Chunk 4: Frontend Foundation

### Task 12: Next.js 15 Project Setup

**Files:**
- Create: `web/` (entire Next.js project)
- Create: `web/.env.example`

- [ ] **Step 1: Create Next.js 15 project**

```bash
cd /Users/jasper/Documents/code/taskhub
pnpm create next-app@latest web --typescript --tailwind --eslint --app --no-src-dir --import-alias "@/*"
```
When prompted, accept all defaults.

- [ ] **Step 2: Install additional dependencies**

```bash
cd /Users/jasper/Documents/code/taskhub/web
pnpm add @xyflow/react zustand lucide-react
pnpm add react-markdown
```

- [ ] **Step 3: Initialize shadcn/ui**

```bash
cd /Users/jasper/Documents/code/taskhub/web
pnpm dlx shadcn@latest init --defaults
```
Accept defaults (New York style, neutral color, CSS variables).

- [ ] **Step 4: Add shadcn components**

```bash
cd /Users/jasper/Documents/code/taskhub/web
pnpm dlx shadcn@latest add button input card badge separator scroll-area
```

- [ ] **Step 5: Create .env.example**

```
NEXT_PUBLIC_API_URL=http://localhost:8080
```

- [ ] **Step 6: Create .env.local**

```
NEXT_PUBLIC_API_URL=http://localhost:8080
```

- [ ] **Step 7: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add web/
git commit -m "feat(web): initialize Next.js 15 project with shadcn/ui and dependencies"
```

---

### Task 13: TypeScript Types + API Client

**Files:**
- Create: `web/lib/types.ts`
- Create: `web/lib/api.ts`

- [ ] **Step 1: Write types**

```typescript
// web/lib/types.ts
export interface Agent {
  id: string;
  name: string;
  description: string;
  system_prompt: string;
  capabilities: string[];
  color: string;
  created_at: string;
}

export interface Task {
  id: string;
  title: string;
  description: string;
  status: "pending" | "running" | "completed" | "failed";
  created_at: string;
  completed_at?: string;
}

export interface Channel {
  id: string;
  task_id: string;
  document: string;
  status: "active" | "archived";
  created_at: string;
}

export interface Message {
  id: string;
  channel_id: string;
  sender_id: string;
  sender_name: string;
  content: string;
  type: "text" | "result" | "system";
  created_at: string;
}

export interface ChannelAgent {
  channel_id: string;
  agent_id: string;
  status: "idle" | "working" | "done";
}

export interface ChannelDetail {
  channel: Channel;
  messages: Message[];
  agents: ChannelAgent[];
}

// SSE event types
export type SSEEventType =
  | "task_started"
  | "channel_created"
  | "agent_joined"
  | "agent_working"
  | "message"
  | "document_updated"
  | "agent_done"
  | "task_completed";

export interface SSEEvent<T = unknown> {
  type: SSEEventType;
  data: T;
}
```

- [ ] **Step 2: Write API client**

```typescript
// web/lib/api.ts
import type { Agent, ChannelDetail, Task } from "./types";

const BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) throw new Error(`GET ${path} → ${res.status}`);
  return res.json();
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`POST ${path} → ${res.status}`);
  return res.json();
}

export const api = {
  agents: {
    list: () => get<Agent[]>("/api/agents"),
    create: (data: Partial<Agent>) => post<Agent>("/api/agents", data),
  },
  tasks: {
    list: () => get<Task[]>("/api/tasks"),
    get: (id: string) => get<Task>(`/api/tasks/${id}`),
    create: (data: { title?: string; description: string }) =>
      post<Task>("/api/tasks", data),
  },
  channels: {
    get: (id: string) => get<ChannelDetail>(`/api/channels/${id}`),
    streamUrl: (id: string) => `${BASE}/api/channels/${id}/stream`,
  },
};
```

- [ ] **Step 3: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add web/lib/types.ts web/lib/api.ts
git commit -m "feat(web): add TypeScript types and API client"
```

---

### Task 14: Zustand Store + SSE Manager

**Files:**
- Create: `web/lib/sse.ts`
- Create: `web/lib/store.ts`

- [ ] **Step 1: Write SSE manager**

```typescript
// web/lib/sse.ts
import type { SSEEvent } from "./types";

type SSEHandler = (event: SSEEvent) => void;

export function connectSSE(url: string, onEvent: SSEHandler): () => void {
  const source = new EventSource(url);

  source.onmessage = (e) => {
    try {
      const event: SSEEvent = JSON.parse(e.data);
      onEvent(event);
    } catch {
      console.error("SSE parse error:", e.data);
    }
  };

  source.onerror = (e) => {
    console.error("SSE error:", e);
  };

  return () => source.close();
}
```

- [ ] **Step 2: Write Zustand store**

```typescript
// web/lib/store.ts
"use client";

import { create } from "zustand";
import { api } from "./api";
import { connectSSE } from "./sse";
import type { Agent, Channel, ChannelAgent, Message, SSEEvent, Task } from "./types";

interface TaskHubStore {
  // State
  agents: Agent[];
  currentTask: Task | null;
  currentChannel: Channel | null;
  messages: Message[];
  channelAgents: ChannelAgent[];
  channelId: string | null;
  isLoading: boolean;
  sseDisconnect: (() => void) | null;

  // Actions
  loadAgents: () => Promise<void>;
  submitTask: (description: string) => Promise<void>;
  connectToChannel: (channelId: string) => void;
  handleSSEEvent: (event: SSEEvent) => void;
  reset: () => void;
}

export const useTaskHubStore = create<TaskHubStore>((set, get) => ({
  agents: [],
  currentTask: null,
  currentChannel: null,
  messages: [],
  channelAgents: [],
  channelId: null,
  isLoading: false,
  sseDisconnect: null,

  loadAgents: async () => {
    try {
      const agents = await api.agents.list();
      set({ agents });
    } catch (e) {
      console.error("loadAgents:", e);
    }
  },

  submitTask: async (description: string) => {
    get().reset();
    set({ isLoading: true });
    try {
      const task = await api.tasks.create({ description });
      set({ currentTask: task });
      // SSE connect will happen once channel_created event fires via polling
      // We need to connect to SSE for this task — but we don't know the channel ID yet.
      // Solution: poll the task until we get a channel, or use a task-level stream.
      // For MVP: we subscribe to a "pending" SSE and wait for channel_created.
      // We'll create a task-level approach: frontend subscribes to channel once master creates it.
      // Workaround: the backend could expose /api/tasks/:id/stream
      // Simpler MVP: poll tasks/:id every 500ms until channel exists, then connect SSE.
      pollForChannel(task.id);
    } catch (e) {
      console.error("submitTask:", e);
      set({ isLoading: false });
    }
  },

  connectToChannel: (channelId: string) => {
    const { sseDisconnect } = get();
    if (sseDisconnect) sseDisconnect();

    const disconnect = connectSSE(
      api.channels.streamUrl(channelId),
      get().handleSSEEvent
    );
    set({ channelId, sseDisconnect: disconnect, isLoading: false });
  },

  handleSSEEvent: (event: SSEEvent) => {
    switch (event.type) {
      case "channel_created": {
        const data = event.data as { channel_id: string };
        // Load full channel detail
        api.channels.get(data.channel_id).then((detail) => {
          set({
            currentChannel: detail.channel,
            messages: detail.messages,
            channelAgents: detail.agents,
          });
        });
        break;
      }
      case "agent_joined": {
        const data = event.data as { agent_id: string; agent_name: string };
        set((state) => ({
          channelAgents: [
            ...state.channelAgents.filter((ca) => ca.agent_id !== data.agent_id),
            { channel_id: state.channelId ?? "", agent_id: data.agent_id, status: "idle" },
          ],
        }));
        break;
      }
      case "agent_working": {
        const data = event.data as { agent_id: string };
        set((state) => ({
          channelAgents: state.channelAgents.map((ca) =>
            ca.agent_id === data.agent_id ? { ...ca, status: "working" } : ca
          ),
        }));
        break;
      }
      case "agent_done": {
        const data = event.data as { agent_id: string };
        set((state) => ({
          channelAgents: state.channelAgents.map((ca) =>
            ca.agent_id === data.agent_id ? { ...ca, status: "done" } : ca
          ),
        }));
        break;
      }
      case "message": {
        const data = event.data as { message: Message };
        set((state) => ({
          messages: [...state.messages, data.message],
        }));
        break;
      }
      case "document_updated": {
        const data = event.data as { document: string };
        set((state) => ({
          currentChannel: state.currentChannel
            ? { ...state.currentChannel, document: data.document }
            : null,
        }));
        break;
      }
      case "task_completed": {
        const data = event.data as { task: Task };
        set({ currentTask: data.task });
        break;
      }
    }
  },

  reset: () => {
    const { sseDisconnect } = get();
    if (sseDisconnect) sseDisconnect();
    set({
      currentTask: null,
      currentChannel: null,
      messages: [],
      channelAgents: [],
      channelId: null,
      sseDisconnect: null,
      isLoading: false,
    });
  },
}));

// Poll tasks/:id until a channel exists for this task, then connect SSE
async function pollForChannel(taskId: string) {
  const store = useTaskHubStore.getState();
  let attempts = 0;
  const interval = setInterval(async () => {
    attempts++;
    if (attempts > 60) {
      clearInterval(interval);
      return;
    }
    try {
      // Fetch all tasks channels — workaround: check DB via task detail
      // Actually the master agent creates the channel and publishes channel_created.
      // We need a channel to subscribe to SSE first. Chicken-and-egg.
      //
      // Resolution: expose GET /api/tasks/:id/channel endpoint
      // OR: connect SSE to a task-level endpoint.
      //
      // Simpler: use GET /api/channels?task_id=:id endpoint.
      // For MVP: add GET /api/tasks/:id/channel to backend.
      //
      // Implementation shortcut: poll GET /api/tasks/:id,
      // and separately query channels by task.
      const res = await fetch(
        `${process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080"}/api/tasks/${taskId}/channel`
      );
      if (res.ok) {
        const { channel_id } = await res.json();
        clearInterval(interval);
        store.connectToChannel(channel_id);
      }
    } catch {
      // keep polling
    }
  }, 500);
}
```

**Note:** This requires an additional backend endpoint `GET /api/tasks/:id/channel`. Add it in Task 15.

- [ ] **Step 3: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add web/lib/sse.ts web/lib/store.ts
git commit -m "feat(web): add Zustand store and SSE manager"
```

---

### Task 15: Add Task-Channel Lookup Endpoint (Backend)

**Files:**
- Modify: `backend/internal/handlers/tasks.go` — add GetChannel handler
- Modify: `backend/cmd/server/main.go` — add route

- [ ] **Step 1: Add GetChannel to task handler**

Append to `backend/internal/handlers/tasks.go`:

```go
func (h *TaskHandler) GetChannel(w http.ResponseWriter, r *http.Request) {
    taskID := chi.URLParam(r, "id")
    var channelID string
    err := h.DB.QueryRowContext(r.Context(),
        `SELECT id FROM channels WHERE task_id = $1 ORDER BY created_at DESC LIMIT 1`, taskID,
    ).Scan(&channelID)
    if err == sql.ErrNoRows {
        jsonError(w, "no channel yet", http.StatusNotFound)
        return
    }
    if err != nil {
        jsonError(w, err.Error(), http.StatusInternalServerError)
        return
    }
    jsonOK(w, map[string]string{"channel_id": channelID})
}
```

- [ ] **Step 2: Register route in main.go**

Add to the `/api` route group:
```go
r.Get("/tasks/{id}/channel", taskH.GetChannel)
```

- [ ] **Step 3: Rebuild to verify**

```bash
cd /Users/jasper/Documents/code/taskhub/backend
go build ./...
```

- [ ] **Step 4: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add backend/
git commit -m "feat(backend): add task channel lookup endpoint"
```

---

## Chunk 5: Frontend UI

### Task 16: Root Layout + Global Styles

**Files:**
- Modify: `web/app/layout.tsx`
- Modify: `web/app/globals.css`

- [ ] **Step 1: Update layout.tsx for dark theme**

```tsx
// web/app/layout.tsx
import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "TaskHub",
  description: "Enterprise multi-agent collaboration platform",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className="dark">
      <body className={`${inter.className} bg-gray-950 text-gray-100 min-h-screen`}>
        {children}
      </body>
    </html>
  );
}
```

- [ ] **Step 2: Update globals.css** — keep shadcn CSS variables but ensure dark mode base is set. The shadcn init will have created appropriate CSS variables; just verify the `:root` has dark-friendly defaults or use `.dark` class overrides.

- [ ] **Step 3: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add web/app/layout.tsx web/app/globals.css
git commit -m "feat(web): set dark theme in root layout"
```

---

### Task 17: TaskBar Component

**Files:**
- Create: `web/components/task/TaskBar.tsx`

- [ ] **Step 1: Write TaskBar**

```tsx
// web/components/task/TaskBar.tsx
"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Loader2, Zap } from "lucide-react";
import { useTaskHubStore } from "@/lib/store";

export function TaskBar() {
  const [input, setInput] = useState("");
  const { currentTask, isLoading, submitTask } = useTaskHubStore();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim() || isLoading) return;
    await submitTask(input.trim());
    setInput("");
  };

  const statusColor: Record<string, string> = {
    pending: "bg-yellow-500/20 text-yellow-400 border-yellow-500/30",
    running: "bg-blue-500/20 text-blue-400 border-blue-500/30",
    completed: "bg-green-500/20 text-green-400 border-green-500/30",
    failed: "bg-red-500/20 text-red-400 border-red-500/30",
  };

  return (
    <div className="border-b border-gray-800 bg-gray-900/80 backdrop-blur px-6 py-4">
      <div className="flex items-center gap-4 max-w-screen-xl mx-auto">
        <div className="flex items-center gap-2 mr-4">
          <div className="w-7 h-7 rounded-lg bg-purple-600 flex items-center justify-center">
            <Zap className="w-4 h-4 text-white" />
          </div>
          <span className="font-semibold text-white text-sm">TaskHub</span>
        </div>

        <form onSubmit={handleSubmit} className="flex gap-3 flex-1">
          <Input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            placeholder="Describe a task for your agents... (e.g. 'Production API is down, investigate and respond')"
            className="flex-1 bg-gray-800/60 border-gray-700 text-gray-100 placeholder:text-gray-500 focus:border-purple-500"
            disabled={isLoading}
          />
          <Button
            type="submit"
            disabled={!input.trim() || isLoading}
            className="bg-purple-600 hover:bg-purple-700 text-white min-w-[100px]"
          >
            {isLoading ? (
              <>
                <Loader2 className="w-4 h-4 mr-2 animate-spin" />
                Running
              </>
            ) : (
              "Dispatch"
            )}
          </Button>
        </form>

        {currentTask && (
          <div className="flex items-center gap-2 ml-2 shrink-0">
            <span className="text-xs text-gray-400 max-w-[200px] truncate">
              {currentTask.title}
            </span>
            <Badge
              className={`text-xs border ${statusColor[currentTask.status] ?? ""}`}
            >
              {currentTask.status}
            </Badge>
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add web/components/task/
git commit -m "feat(web): add TaskBar component"
```

---

### Task 18: Agent Topology (React Flow)

**Files:**
- Create: `web/components/topology/AgentTopology.tsx`

- [ ] **Step 1: Write AgentTopology**

```tsx
// web/components/topology/AgentTopology.tsx
"use client";

import { useEffect, useMemo } from "react";
import {
  ReactFlow,
  Node,
  Edge,
  Background,
  BackgroundVariant,
  useNodesState,
  useEdgesState,
  Handle,
  Position,
  NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { CheckCircle2 } from "lucide-react";
import { useTaskHubStore } from "@/lib/store";

const MASTER_ID = "master";
const MASTER_COLOR = "#8b5cf6";

// Custom agent node
function AgentNode({ data }: NodeProps) {
  const { label, color, status, ismaster } = data as {
    label: string;
    color: string;
    status: string;
    ismaster: boolean;
  };

  const isWorking = status === "working";
  const isDone = status === "done";

  return (
    <div
      className="relative flex flex-col items-center gap-1"
      style={{ minWidth: 100 }}
    >
      <Handle type="target" position={Position.Top} className="opacity-0" />
      <div
        className={`w-16 h-16 rounded-full flex items-center justify-center border-2 transition-all duration-300 ${
          isWorking ? "animate-pulse shadow-lg" : ""
        }`}
        style={{
          borderColor: color,
          backgroundColor: `${color}22`,
          boxShadow: isWorking ? `0 0 20px ${color}88` : undefined,
        }}
      >
        {isDone ? (
          <CheckCircle2 className="w-7 h-7 text-green-400" />
        ) : (
          <span className="text-xl font-bold" style={{ color }}>
            {label.charAt(0)}
          </span>
        )}
      </div>
      <span className="text-xs text-gray-300 text-center leading-tight max-w-[90px]">
        {label}
      </span>
      {status && !ismaster && (
        <span
          className="text-[10px] px-1.5 py-0.5 rounded-full"
          style={{
            backgroundColor: isWorking
              ? "#3b82f622"
              : isDone
              ? "#10b98122"
              : "#6b728022",
            color: isWorking ? "#60a5fa" : isDone ? "#34d399" : "#9ca3af",
          }}
        >
          {status}
        </span>
      )}
      <Handle type="source" position={Position.Bottom} className="opacity-0" />
    </div>
  );
}

const nodeTypes = { agent: AgentNode };

export function AgentTopology() {
  const { agents, channelAgents } = useTaskHubStore();

  const statusMap = useMemo(() => {
    const m: Record<string, string> = {};
    for (const ca of channelAgents) {
      m[ca.agent_id] = ca.status;
    }
    return m;
  }, [channelAgents]);

  const { nodes: initialNodes, edges: initialEdges } = useMemo(() => {
    const nodes: Node[] = [
      {
        id: MASTER_ID,
        type: "agent",
        position: { x: 200, y: 20 },
        data: {
          label: "Master Agent",
          color: MASTER_COLOR,
          status: "",
          ismaster: true,
        },
      },
    ];

    const edges: Edge[] = [];
    const activeAgentIds = new Set(channelAgents.map((ca) => ca.agent_id));

    // Show all agents, highlight active ones
    const displayAgents =
      agents.length > 0
        ? agents
        : [
            { id: "sre", name: "SRE Agent", color: "#ef4444" },
            { id: "eng", name: "Engineering Agent", color: "#3b82f6" },
            { id: "cs", name: "Customer Success", color: "#10b981" },
            { id: "doc", name: "Documentation", color: "#f59e0b" },
          ];

    const cols = Math.min(displayAgents.length, 4);
    displayAgents.forEach((agent, i) => {
      const col = i % cols;
      const row = Math.floor(i / cols);
      const x = (col - (cols - 1) / 2) * 160 + 200;
      const y = 200 + row * 160;
      const status = statusMap[agent.id] ?? "idle";
      const isActive = activeAgentIds.has(agent.id);

      nodes.push({
        id: agent.id,
        type: "agent",
        position: { x, y },
        data: {
          label: agent.name,
          color: agent.color,
          status,
          ismaster: false,
        },
        style: {
          opacity: activeAgentIds.size === 0 || isActive ? 1 : 0.35,
          transition: "opacity 0.3s",
        },
      });

      if (isActive) {
        edges.push({
          id: `master-${agent.id}`,
          source: MASTER_ID,
          target: agent.id,
          style: {
            stroke: agent.color,
            strokeWidth: 2,
            opacity: 0.7,
          },
          animated: status === "working",
        });
      }
    });

    return { nodes, edges };
  }, [agents, channelAgents, statusMap]);

  const [nodes, setNodes, onNodesChange] = useNodesState(initialNodes);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges);

  useEffect(() => {
    setNodes(initialNodes);
    setEdges(initialEdges);
  }, [initialNodes, initialEdges, setNodes, setEdges]);

  return (
    <div className="w-full h-full bg-gray-950">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.3 }}
        minZoom={0.5}
        maxZoom={2}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        proOptions={{ hideAttribution: true }}
      >
        <Background
          variant={BackgroundVariant.Dots}
          gap={24}
          size={1}
          color="#374151"
        />
      </ReactFlow>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add web/components/topology/
git commit -m "feat(web): add React Flow agent topology visualization"
```

---

### Task 19: Channel Panel Components

**Files:**
- Create: `web/components/channel/DocumentViewer.tsx`
- Create: `web/components/channel/MessageFeed.tsx`
- Create: `web/components/channel/ChannelPanel.tsx`

- [ ] **Step 1: Write DocumentViewer**

```tsx
// web/components/channel/DocumentViewer.tsx
"use client";

import ReactMarkdown from "react-markdown";
import { useTaskHubStore } from "@/lib/store";
import { ScrollArea } from "@/components/ui/scroll-area";
import { FileText } from "lucide-react";

export function DocumentViewer() {
  const { currentChannel } = useTaskHubStore();

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center gap-2 px-4 py-2.5 border-b border-gray-800 bg-gray-900/50">
        <FileText className="w-4 h-4 text-gray-400" />
        <span className="text-xs font-medium text-gray-400 uppercase tracking-wider">
          Shared Context
        </span>
      </div>
      <ScrollArea className="flex-1 px-4 py-3">
        {currentChannel?.document ? (
          <div className="prose prose-invert prose-sm max-w-none text-gray-300">
            <ReactMarkdown>{currentChannel.document}</ReactMarkdown>
          </div>
        ) : (
          <div className="flex items-center justify-center h-full text-gray-600 text-sm">
            Awaiting task assignment...
          </div>
        )}
      </ScrollArea>
    </div>
  );
}
```

- [ ] **Step 2: Write MessageFeed**

```tsx
// web/components/channel/MessageFeed.tsx
"use client";

import { useEffect, useRef } from "react";
import { useTaskHubStore } from "@/lib/store";
import { ScrollArea } from "@/components/ui/scroll-area";
import { MessageCircle } from "lucide-react";

const SENDER_COLORS: Record<string, string> = {
  master: "#8b5cf6",
};

export function MessageFeed() {
  const { messages, agents } = useTaskHubStore();
  const bottomRef = useRef<HTMLDivElement>(null);

  const agentColorMap = Object.fromEntries(agents.map((a) => [a.id, a.color]));

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center gap-2 px-4 py-2.5 border-b border-gray-800 bg-gray-900/50">
        <MessageCircle className="w-4 h-4 text-gray-400" />
        <span className="text-xs font-medium text-gray-400 uppercase tracking-wider">
          Agent Messages
        </span>
        <span className="ml-auto text-xs text-gray-600">{messages.length}</span>
      </div>
      <ScrollArea className="flex-1">
        <div className="px-4 py-3 space-y-3">
          {messages.length === 0 && (
            <div className="text-center text-gray-600 text-sm py-8">
              No messages yet
            </div>
          )}
          {messages.map((msg) => {
            const color =
              agentColorMap[msg.sender_id] ??
              SENDER_COLORS[msg.sender_id] ??
              "#6b7280";
            return (
              <div
                key={msg.id}
                className={`rounded-lg p-3 border ${
                  msg.type === "system"
                    ? "border-purple-800/40 bg-purple-900/10"
                    : msg.type === "result"
                    ? "border-gray-700/50 bg-gray-800/40"
                    : "border-gray-800 bg-gray-900/30"
                }`}
              >
                <div className="flex items-center gap-2 mb-1.5">
                  <div
                    className="w-2 h-2 rounded-full"
                    style={{ backgroundColor: color }}
                  />
                  <span className="text-xs font-semibold" style={{ color }}>
                    {msg.sender_name}
                  </span>
                  <span className="text-xs text-gray-600 ml-auto">
                    {new Date(msg.created_at).toLocaleTimeString()}
                  </span>
                </div>
                <p className="text-sm text-gray-300 whitespace-pre-wrap leading-relaxed">
                  {msg.content}
                </p>
              </div>
            );
          })}
          <div ref={bottomRef} />
        </div>
      </ScrollArea>
    </div>
  );
}
```

- [ ] **Step 3: Write ChannelPanel**

```tsx
// web/components/channel/ChannelPanel.tsx
"use client";

import { DocumentViewer } from "./DocumentViewer";
import { MessageFeed } from "./MessageFeed";

export function ChannelPanel() {
  return (
    <div className="flex flex-col h-full">
      {/* Document: top 45% */}
      <div className="h-[45%] border-b border-gray-800 overflow-hidden">
        <DocumentViewer />
      </div>
      {/* Messages: bottom 55% */}
      <div className="flex-1 overflow-hidden">
        <MessageFeed />
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add web/components/channel/
git commit -m "feat(web): add DocumentViewer, MessageFeed, and ChannelPanel"
```

---

### Task 20: Main Page Assembly

**Files:**
- Modify: `web/app/page.tsx`

- [ ] **Step 1: Write main page**

```tsx
// web/app/page.tsx
"use client";

import { useEffect } from "react";
import { TaskBar } from "@/components/task/TaskBar";
import { AgentTopology } from "@/components/topology/AgentTopology";
import { ChannelPanel } from "@/components/channel/ChannelPanel";
import { useTaskHubStore } from "@/lib/store";

export default function Home() {
  const { loadAgents } = useTaskHubStore();

  useEffect(() => {
    loadAgents();
  }, [loadAgents]);

  return (
    <div className="flex flex-col h-screen overflow-hidden">
      <TaskBar />
      <div className="flex flex-1 overflow-hidden">
        {/* Left: Topology (40%) */}
        <div className="w-[40%] border-r border-gray-800 overflow-hidden">
          <div className="flex items-center gap-2 px-4 py-2.5 border-b border-gray-800 bg-gray-900/50">
            <span className="text-xs font-medium text-gray-400 uppercase tracking-wider">
              Agent Network
            </span>
          </div>
          <div className="h-[calc(100%-40px)]">
            <AgentTopology />
          </div>
        </div>

        {/* Right: Channel (60%) */}
        <div className="flex-1 overflow-hidden">
          <ChannelPanel />
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add web/app/page.tsx
git commit -m "feat(web): assemble main page with topology and channel panels"
```

---

## Chunk 6: Integration + Polish

### Task 21: Verify Backend Builds + Run

- [ ] **Step 1: Build backend**

```bash
cd /Users/jasper/Documents/code/taskhub/backend
go build ./...
```
Expected: no errors.

- [ ] **Step 2: Create local .env**

```bash
# Create backend/.env with real values
cat > /Users/jasper/Documents/code/taskhub/backend/.env << 'EOF'
DATABASE_URL=postgres://localhost:5432/taskhub?sslmode=disable
OPENAI_API_KEY=<your-key>
PORT=8080
EOF
```

- [ ] **Step 3: Run backend**

```bash
cd /Users/jasper/Documents/code/taskhub/backend
go run ./cmd/server
```
Expected output:
```
migrations OK
seeded agent: SRE Agent
seeded agent: Engineering Agent
seeded agent: Customer Success Agent
seeded agent: Documentation Agent
server listening on :8080
```

- [ ] **Step 4: Verify agents endpoint**

```bash
curl -s http://localhost:8080/api/agents | jq '.[].name'
```
Expected: 4 agent names.

---

### Task 22: Verify Frontend Builds + Runs

- [ ] **Step 1: Install dependencies**

```bash
cd /Users/jasper/Documents/code/taskhub/web
pnpm install
```

- [ ] **Step 2: Build check**

```bash
cd /Users/jasper/Documents/code/taskhub/web
pnpm build
```
Expected: successful build (may have lint warnings, no errors).

- [ ] **Step 3: Run dev server**

```bash
cd /Users/jasper/Documents/code/taskhub/web
pnpm dev
```
Expected: Next.js dev server at http://localhost:3000.

- [ ] **Step 4: Smoke test**
Open http://localhost:3000 and verify:
- Dark background loads
- TaskBar is visible at top
- Agent topology grid is visible on left
- Channel panel is visible on right
- No console errors about missing env vars

---

### Task 23: End-to-End Smoke Test

- [ ] **Step 1: Submit a production incident task**

In the TaskBar, enter:
```
Production API is returning 500 errors, investigate the root cause and prepare customer communication
```

- [ ] **Step 2: Verify SSE stream**

Expected behavior:
1. Task is created with status "pending" → "running"
2. Master Agent decomposes task
3. Channel is created, agents join
4. Agent nodes in topology become colored/animated as they work
5. Messages appear in the feed as agents complete subtasks
6. Document updates with agent results
7. Task status changes to "completed"

- [ ] **Step 3: Final commit**

```bash
cd /Users/jasper/Documents/code/taskhub
git add -A
git commit -m "feat: TaskHub MVP complete - backend + frontend ready"
```

- [ ] **Step 4: Notify completion**

```bash
openclaw system event --text "TaskHub MVP built - backend + frontend ready" --mode now
```

---

## Quick Reference: Start Commands

**Backend:**
```bash
cd /Users/jasper/Documents/code/taskhub/backend
go run ./cmd/server
```

**Frontend:**
```bash
cd /Users/jasper/Documents/code/taskhub/web
pnpm dev
```

**Prerequisites:**
- PostgreSQL running locally
- `createdb taskhub` executed
- `OPENAI_API_KEY` set in `backend/.env`
