# Architecture

**Analysis Date:** 2026-04-04

## Pattern Overview

**Overall:** Multi-agent task orchestration platform with real-time event streaming and adaptive execution

**Key Characteristics:**
- **Master Orchestrator Pattern**: LLM-driven task decomposition creates DAG (Directed Acyclic Graph) of subtasks
- **A2A Protocol**: Agent-to-Agent JSON-RPC 2.0 communication (supports HTTP polling and native adapters)
- **Event-Driven Real-Time Updates**: In-memory broker + persistent event store for SSE streaming to frontend
- **Policy-Driven Execution**: Constraint engine validates task plans before execution
- **Human-in-the-Loop**: Approval workflows and input-required states for non-autonomous execution
- **Adaptive Replanning**: Automatic failure recovery via LLM-guided subtask replanning

## Layers

**Presentation Layer (Frontend):**
- Location: `web/app/` (Next.js 13+ App Router), `web/components/`, `web/lib/`
- Contains: Page components, UI forms, real-time chat, DAG visualization, agent registry UI
- Depends on: API client (`web/lib/api.ts`), SSE connection manager (`web/lib/sse.ts`), Zustand stores
- Used by: End users (task creation, agent management, real-time monitoring)

**HTTP Handler Layer (API Boundary):**
- Location: `internal/handlers/`
- Contains: Request/response handlers for tasks, agents, conversations, webhooks, policies, analytics
- Handlers: `TaskHandler`, `AgentHandler`, `ConversationHandler`, `StreamHandler`, `MessageHandler`, `PolicyHandler`, `TemplateHandler`, `WebhookHandler`, `A2AConfigHandler`, `AnalyticsHandler`, `TraceHandler`, `AuditLogHandler`
- Depends on: Orchestrator, Executor, Event Store, Broker, Audit Logger, A2A resolver/aggregator
- Pattern: Thin handlers — business logic delegated to dedicated packages

**Orchestration Layer:**
- Location: `internal/orchestrator/orchestrator.go`
- Purpose: LLM-driven task decomposition into subtask DAGs
- Functions:
  - `Plan()`: Initial task decomposition via claude CLI (MVP — replace with Anthropic Go SDK)
  - `Replan()`: Failure recovery — generates replacement subtasks for failed work
  - `buildAgentDescription()`: Formats agent info for LLM prompts
  - `callLLM()`: Invokes LLM via `claude` CLI command with system/user prompts
- Output: `ExecutionPlan` with subtasks, dependencies, and agent assignments

**Execution Engine Layer:**
- Location: `internal/executor/executor.go`, `internal/executor/recovery.go`
- Core type: `DAGExecutor`
- Responsibilities:
  1. **Planning**: Call orchestrator, validate subtask count against policies
  2. **Approval Gate**: Pause for human approval if policy thresholds exceeded
  3. **DAG Creation**: Convert plan temp IDs to UUIDs, resolve dependencies, create DB records
  4. **DAG Loop** (`runDAGLoop()`): Core execution loop that:
     - Finds ready subtasks (dependencies satisfied)
     - Respects global (10 default) and per-agent (3 default) concurrency limits
     - Invokes agents via A2A protocol
     - Tracks subtask status (pending → running → completed/failed)
     - Detects failures and triggers replanning
  5. **Recovery** (`Recover()`): On startup, resumes incomplete tasks
- Key methods:
  - `Execute()`: Main entry point (planning → execution)
  - `ResumeApproved()`: Continues task after human approval
  - `executePlan()`: Subtask creation + DAG loop
  - `runDAGLoop()`: Core scheduling and execution loop
  - `tryReplan()`: Attempts recovery after subtask failure
  - `createSubtasks()`: Maps plan IDs to UUIDs, stores in DB
  - `findReadySubtasks()`: Identifies subtasks with satisfied dependencies
  - `executeSubtask()`: Invokes agent via A2A client
  - `aggregateResults()`: Combines subtask outputs into task result

**A2A Protocol Layer:**
- Location: `internal/a2a/`
- Components:
  - **Server** (`a2a/server.go`): TaskHub as an A2A agent — handles incoming JSON-RPC calls (tasks/send, tasks/get, tasks/cancel)
  - **Client** (`a2a/client.go`): Invokes external agents via JSON-RPC 2.0
  - **Aggregator** (`a2a/aggregator.go`): Tracks multi-agent conversations (multi-turn interactions per contextId)
  - **Protocol** (`a2a/protocol.go`): Shared JSON-RPC and A2A message types
  - **Discovery** (`a2a/discovery.go`): Agent capability discovery via agent-card.json
  - **Health** (`a2a/health.go`): Background health checking of agent endpoints
- Adapter types:
  - **HTTP Polling** (via client): Invokes agents at their endpoint, polls for completion
  - **Native** (future): Direct integration with agent binaries
- Communication: JSON-RPC 2.0 with A2A message envelope (role + text/artifact parts)

**Event System:**
- Location: `internal/events/`
- Components:
  - **Broker** (`events/broker.go`): In-memory pub/sub for real-time fanout
    - Channels per task_id and "conv:conversation_id" (dual routing)
    - Publish drops events to full channels (subscribers catch up from DB)
  - **Store** (`events/store.go`): Persistent event log in PostgreSQL
    - Event types: `task.planning`, `task.planned`, `task.running`, `subtask.created`, `agent.working`, `message`, `document_updated`, `approval.requested`, `policy.applied`, `template.matched`, `task.completed`, `task.failed`
- Flow: Event published → stored in DB → fanned out via broker → SSE streamed to frontend

**Data Model Layer:**
- Location: `internal/models/`
- Core entities:
  - `Task`: Title, description, status, plan (JSON), result (JSON), error, template reference, replan count
  - `SubTask`: Instruction, agent assignment, dependencies, input/output (JSON), attempt tracking
  - `ExecutionPlan`: Summary + PlanSubTask list (plan temp IDs for DAG layout)
  - `Agent`: Name, description, endpoint, capabilities, status, health
  - `Event`: Task ID, type, actor (system/agent/user), data (JSON), timestamp
  - `Message`: Conversation ID, sender (agent/user/system), content, mentions
  - `Conversation`: Title, created_by, timestamp (chat-first UI container)
  - `User`: Email, name, avatar, auth provider
  - `WorkflowTemplate`: Name, steps (JSON), version, source task, is_active
  - `Policy`: Name, rules (JSON), priority, is_active
  - `WebhookConfig`: URL, events to subscribe, bearer token

**Policy Engine:**
- Location: `internal/policy/engine.go`
- Purpose: Constraint evaluation before task execution
- Evaluates: Task title/description against defined policies
- Returns: Applied policies, format for LLM prompt, approval thresholds
- Used by: Executor to gate execution and provide policy guidance to orchestrator

**Database Layer:**
- Location: `internal/db/`
- Driver: PostgreSQL via pgx/v5 with connection pooling
- Migrations: Embedded SQL files in `internal/db/migrations/`
- Transaction handling: Row-level locking where needed (subtask status updates)
- Connection: Single `*pgxpool.Pool` injected across handlers/executor

**Supporting Layers:**

**Auth & RBAC:**
- Location: `internal/auth/`, `internal/rbac/`
- Middleware: `RequireAuth` checks session cookies
- Local mode: No auth required (auto-seed user)
- Cloud mode: Google OAuth2 or email login
- RBAC: Role-based access control (future expansion)

**Context Utilities:**
- Location: `internal/ctxutil/`
- Extracts: User, org, role from request context
- Used throughout: Handlers inject user context for audit/permission checks

**Audit Logging:**
- Location: `internal/audit/`
- Tracks: Every LLM call with model, input tokens, output tokens, cost estimate
- Used by: Executor, handler (cost endpoint)

**Webhooks:**
- Location: `internal/webhook/`
- Purpose: Event-triggered notifications to external URLs
- Sends: Task status changes, subtask completion, policy violations
- Retry logic: Built-in retry mechanism

## Data Flow

**Task Creation → Completion Flow:**

1. **Frontend sends POST /api/tasks**
   - User fills title + description + optional template_id
   - Handler validates, creates Task record (status: "pending")
   - Handler spawns executor in background goroutine
   - Response: 201 Created task

2. **Executor Planning Phase** (status: "planning")
   - Load all active agents
   - Evaluate policies against task
   - Load template skeleton if task.template_id set
   - Call orchestrator.Plan() with task, agents, constraints, template
   - Orchestrator calls LLM (claude CLI) for decomposition
   - LLM returns ExecutionPlan JSON
   - Publish "task.planning" event

3. **Policy Gate Check** (status: "approval_required" OR "running")
   - If policy.RequireApprovalAboveSubtasks > 0 AND subtask_count > threshold:
     - Store plan in task.plan (JSON)
     - Update task to "approval_required"
     - Publish "approval.requested" event
     - **Handler waits** for manual approval via POST /api/tasks/{id}/approve
   - Otherwise:
     - Proceed to DAG creation

4. **DAG Creation & Execution** (status: "running")
   - Create SubTask records from ExecutionPlan:
     - Map plan temp IDs (s1, s2) to real UUIDs
     - Resolve agent assignments (by ID or name)
     - Resolve dependencies (map depends_on IDs)
   - Publish "subtask.created" events
   - Enter runDAGLoop()

5. **DAG Loop (Concurrent Execution)**
   - **Status Check**: Reload subtask statuses from DB
   - **Ready Detection**: Find subtasks with satisfied dependencies
   - **Concurrency Gates**:
     - Global: max 10 running subtasks
     - Per-agent: max 3 running subtasks
   - **Invoke Agent**: For each ready subtask:
     - Call A2A client.Call(agent_endpoint, task_context, instruction)
     - Create A2A task, get back A2A task ID
     - Update subtask.a2a_task_id
     - Publish "agent.working" event
   - **Poll for Completion**: In separate goroutine:
     - Poll A2A task until state != "working" (completed/failed/input_required)
     - Update subtask status, output, error
     - Publish "subtask.completed" or "subtask.failed" event
   - **Loop Until Terminal State**:
     - All subtasks terminal (completed/failed/blocked) → task completed
     - Any failed + no pending/running → try replan
     - Otherwise → repeat (find more ready subtasks)

6. **Failure Path**
   - Subtask fails (max_attempts exceeded OR agent returns error)
   - DAG loop calls tryReplan()
   - Orchestrator.Replan() generates replacement subtasks
   - New subtasks inserted, old ones remain (for audit)
   - DAG loop resumes with new subtasks

7. **Task Completion** (status: "completed")
   - All subtasks in terminal state (no failures)
   - aggregateResults() combines all subtask outputs
   - Update task.result (JSON), completed_at, status
   - Publish "task.completed" event
   - Record template execution (if template_id set)

8. **SSE Stream to Frontend**
   - Event published → Event Store (DB)
   - Broker fanout to subscribers
   - StreamHandler sends via SSE
   - Frontend receives, updates React state (Zustand)
   - UI updates in real-time (DAG colors, message feed, etc.)

**Conversation/Chat Flow (Multi-Turn):**

1. Frontend sends POST /api/conversations/{id}/messages with content
2. Handler saves message to DB
3. If content contains agent mentions (@agent_id), route directly to agent
4. Otherwise, spawn orchestrator.DetectIntent() to decide action
5. If intent is "create_task":
   - Extract title/description from message
   - Create Task, execute (follows task flow above)
   - Link task to conversation_id
   - Post completion, message feed shows task link + result
6. All messages/events publish to "conv:conversation_id" broker topic
7. Conversation SSE streams updates to all subscribers

## Key Abstractions

**ExecutionPlan:**
- Purpose: Logical blueprint for task execution
- Location: `internal/models/task.go`
- Structure: Summary + list of PlanSubTask (with temp IDs and dependencies)
- Pattern: Temp IDs allow DAG layout before UUID assignment

**DAGExecutor:**
- Purpose: Manages full task lifecycle from plan to completion
- Location: `internal/executor/executor.go`
- Injected into: Handler, A2A Server (for recursive task creation)
- Key state: `cancels` map (task_id → context.CancelFunc)

**Event:**
- Purpose: Immutable record of state changes
- Location: `internal/models/event.go`
- Dual streaming: Broker (live) + Store (audit trail)

**A2ATask / A2AMessage:**
- Purpose: Standard wire format for agent communication
- Location: `internal/a2a/protocol.go`
- Parts: Text (instructions) + Artifacts (file/document references)

**Aggregator:**
- Purpose: Track multi-turn agent conversations
- Location: `internal/a2a/aggregator.go`
- State: contextId → conversation history
- Used by: A2A client when composing prompts for subsequent calls

## Entry Points

**Backend:**
- Location: `cmd/server/main.go`
- Triggers: `make dev-backend` or `./server`
- Responsibilities:
  - Initialize database + run migrations
  - Set up auth middleware + handlers
  - Create executor, broker, event store, auditor
  - Start health checker (background goroutine)
  - Register all HTTP routes (chi router)
  - Listen on :8080 (configurable)

**Frontend:**
- Location: `web/app/layout.tsx` (root), `web/app/page.tsx` (dashboard)
- Triggers: `make dev-frontend` or `npm run dev`
- Entry flow:
  - AuthGuard checks `/api/auth/me`
  - Dashboard lists conversations/tasks
  - Click task → `/app/tasks/[id]` → loads task + SSE stream
  - Click conversation → `/app/c/[id]` → loads messages + SSE stream

**Background Task Recovery:**
- Location: `internal/executor/recovery.go` (Recover method)
- Triggers: Server startup (called in main.go)
- Responsibilities:
  - Find incomplete tasks (status: planning, running, approval_required)
  - Requeue for execution
  - Ensures no work is lost on server restart

**A2A Server (Agent Entrypoint):**
- Location: `internal/a2a/server.go` (HandleJSONRPC)
- Triggers: POST /a2a with JSON-RPC request
- Methods handled: tasks/send, tasks/get, tasks/cancel
- Creates: Internal tasks from incoming A2A requests (enables agent-to-agent communication)

## Error Handling

**Strategy:** Propagate errors up the stack; executor logs and publishes failure events

**Patterns:**

1. **Handler Layer** (`internal/handlers/`):
   - Catch decoding/validation errors → 400 Bad Request
   - Catch DB errors → 500 Internal Server Error
   - Catch executor errors → logged, task marked failed

2. **Executor Layer** (`internal/executor/`):
   - Orchestration errors (LLM call fails) → task status="failed", publish event
   - Subtask failures → log, increment attempt, try replan
   - Max attempts exceeded → task status="failed"
   - Propagate context.Canceled on task cancellation

3. **A2A Client** (`internal/a2a/client.go`):
   - HTTP errors → retry logic (exponential backoff)
   - Agent returns error state → mark subtask failed
   - Timeout → fail with error message

4. **Event Publishing**:
   - Failed publishes don't halt execution (log only)
   - Broker drops to full channels (subscribers must read from DB)

5. **Database**:
   - Connection errors → Fatal in main, reconnect pool handles transients
   - Row locking on subtask updates prevents race conditions
   - Parameterized queries prevent SQL injection

## Cross-Cutting Concerns

**Logging:**
- Backend: Standard library `log` package
- Key events: Task lifecycle, executor decisions, LLM calls, agent invocations
- No structured logging (MVP — consider zerolog/slog upgrade)

**Validation:**
- Handler entry: Decode JSON, check required fields
- Subtask creation: Validate agent IDs exist, dependencies form DAG (no cycles)
- Task title: Required, trimmed
- Agent endpoint: HTTP URL validation on registration

**Authentication:**
- Middleware: `RequireAuth` wraps authenticated routes
- Local mode: Bypassed (auto-user)
- Cloud mode: Google OAuth2 + session cookies
- User injected: Via context (extractable in handlers)

**Authorization:**
- RBAC table exists but unused (future: per-task role checks)
- Currently: User can see/modify only their own tasks/conversations

**Concurrency:**
- DAG loop uses sync.WaitGroup for goroutine tracking
- Broker uses sync.RWMutex for subscriber map
- Subtask status updates: DB row-level locking prevents conflicts
- Task cancellation: context.CancelFunc stored per task

**Observability:**
- Events: Audit trail (all state changes)
- Audit Log: Every LLM call with token/cost
- Webhooks: Event-based notifications
- Timeline: Trace of subtask starts/ends (for DAG visualization)

---

*Architecture analysis: 2026-04-04*
