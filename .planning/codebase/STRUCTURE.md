# Codebase Structure

**Analysis Date:** 2026-04-04

## Directory Layout

```
taskhub/
├── cmd/                               # Entry points
│   ├── server/main.go                 # Backend API server (taskhub)
│   └── openaiagent/main.go            # Agent binary example (OpenAI-powered A2A agent)
├── internal/                          # Backend packages (not importable externally)
│   ├── a2a/                           # Agent-to-Agent protocol (JSON-RPC 2.0, A2A message types)
│   │   ├── protocol.go                # Shared JSON-RPC + A2A types
│   │   ├── server.go                  # A2A server (handles incoming agent calls)
│   │   ├── client.go                  # A2A client (invokes external agents)
│   │   ├── aggregator.go              # Multi-turn conversation state per contextId
│   │   ├── discovery.go               # Agent capability discovery
│   │   └── health.go                  # Background health checker
│   ├── audit/                         # Audit logging (LLM calls, token counts, costs)
│   │   └── audit.go                   # Logger type, SaveLLMCall, GetCost
│   ├── auth/                          # Authentication & session management
│   │   ├── middleware.go              # RequireAuth middleware (Google OAuth2 + simple login)
│   │   ├── handlers.go                # Login, callback, logout, GetMe
│   │   └── session.go                 # Session store (cookie-based)
│   ├── config/                        # Environment configuration
│   │   └── config.go                  # Config struct, Load() method (DATABASE_URL, ANTHROPIC_API_KEY, PORT, etc.)
│   ├── crypto/                        # Encryption utilities (for sensitive fields)
│   │   └── crypto.go                  # Encrypt/decrypt with AES-GCM
│   ├── ctxutil/                       # Context value helpers
│   │   └── ctxutil.go                 # UserFromCtx, OrgFromCtx, UserIntoCtx (request context injection)
│   ├── db/                            # Database connection + migrations
│   │   ├── db.go                      # Open (pgxpool), health checks
│   │   ├── migrate.go                 # RunMigrations (embedded SQL files)
│   │   └── migrations/                # SQL migration files (embedded via //go:embed)
│   │       ├── 001_foundation.sql     # Initial schema (users, tasks, subtasks, agents, etc.)
│   │       ├── 004_a2a_migration.sql  # A2A-related fields, agent_card storage
│   │       ├── 005_remove_org_add_templates.sql  # Templates, remove org isolation
│   │       ├── 006_webhooks.sql       # Webhook config + delivery log
│   │       └── 007_conversations.sql  # Conversations, messages, multi-turn UI
│   ├── events/                        # Real-time event system
│   │   ├── broker.go                  # In-memory pub/sub (task_id → channels)
│   │   └── store.go                   # Persistent event log (PostgreSQL)
│   ├── executor/                      # DAG execution engine
│   │   ├── executor.go                # DAGExecutor (planning, execution, replanning)
│   │   └── recovery.go                # Recover() for incomplete tasks on startup
│   ├── handlers/                      # HTTP request handlers (API routes)
│   │   ├── helpers.go                 # JSON response helpers (jsonOK, jsonError, jsonCreated, decodeJSON)
│   │   ├── conversations.go           # Conversation CRUD, messages, SSE stream, orchestrator integration
│   │   ├── tasks.go                   # Task CRUD, approval, cost, timeline, subtask list
│   │   ├── agents.go                  # Agent CRUD, health checks, discovery, adapter form
│   │   ├── messages.go                # Task-specific chat (legacy — superseded by conversations)
│   │   ├── stream.go                  # SSE endpoint (task events)
│   │   ├── policies.go                # Policy CRUD (constraints for task execution)
│   │   ├── templates.go               # Workflow template CRUD, create from task, rollback, executions
│   │   ├── webhooks.go                # Webhook config CRUD, test trigger
│   │   ├── evolution.go               # Template analysis/evolution (ML-based improvement suggestions)
│   │   ├── a2aconfig.go               # A2A server configuration, agent-card.json
│   │   ├── analytics.go               # Dashboard stats (task counts, costs, agent utilization)
│   │   ├── trace.go                   # Task timeline (subtask starts/completes for DAG visualization)
│   │   ├── agent_health.go            # Agent health overview + per-agent details
│   │   ├── auditlog.go                # Audit log query endpoint
│   │   └── response_contract_test.go  # Response shape validation (ensures Go/TS type sync)
│   ├── httputil/                      # HTTP response helpers
│   │   └── httputil.go                # CORS, error response formatting
│   ├── models/                        # Domain data structures (source of truth for API contracts)
│   │   ├── models.go                  # User, PageRequest, PageResponse
│   │   ├── task.go                    # Task, SubTask, TaskWithSubtasks, ExecutionPlan
│   │   ├── event.go                   # Event, Message
│   │   ├── agent.go                   # Agent, AgentCard
│   │   ├── conversation.go            # Conversation, ConversationListItem
│   │   ├── policy.go                  # Policy (JSON rules, priority)
│   │   ├── template.go                # WorkflowTemplate (steps, variables, versions)
│   │   └── models_test.go             # Model validation tests
│   ├── orchestrator/                  # Task decomposition via LLM
│   │   ├── orchestrator.go            # Plan(), Replan(), callLLM() (claude CLI)
│   │   └── orchestrator_test.go       # Unit tests (mocked LLM responses)
│   ├── policy/                        # Policy constraint engine
│   │   ├── engine.go                  # Evaluate() (matches task against policy rules)
│   │   └── engine_test.go             # Tests for policy matching
│   ├── rbac/                          # Role-based access control (future)
│   │   ├── roles.go                   # Role definitions
│   │   └── middleware.go              # Role-checking middleware
│   ├── seed/                          # Database seeding (dev data)
│   │   └── devseed.go                 # LocalSeedAndLog (creates default user, org, agents)
│   ├── testutil/                      # Test infrastructure
│   │   └── setup.go                   # SetupDB(), helpers for tests
│   └── webhook/                       # Webhook delivery
│       └── sender.go                  # Send() (HTTP POST to configured URLs with retries)
├── web/                               # Next.js frontend
│   ├── app/                           # App Router pages (Next.js 13+)
│   │   ├── layout.tsx                 # Root layout (nav, auth guard, sidebar)
│   │   ├── page.tsx                   # Dashboard (task list, create form)
│   │   ├── login/page.tsx             # Login page (email/Google)
│   │   ├── c/[id]/page.tsx            # Conversation detail (chat + DAG)
│   │   ├── tasks/[id]/page.tsx        # Task detail (DAG view, messages, timeline)
│   │   ├── agents/page.tsx            # Agent list
│   │   ├── agents/[id]/page.tsx       # Agent detail (health, adapter config)
│   │   ├── agents/register/page.tsx   # Agent registration wizard
│   │   ├── agents/health/page.tsx     # Agent health overview
│   │   ├── templates/page.tsx         # Workflow templates list
│   │   ├── templates/[id]/page.tsx    # Template detail, executions
│   │   ├── analytics/page.tsx         # Dashboard analytics
│   │   ├── audit/page.tsx             # Audit log viewer
│   │   ├── settings/                  # Settings pages
│   │   │   ├── policies/page.tsx      # Policy management
│   │   │   ├── webhooks/page.tsx      # Webhook config
│   │   │   └── a2a-server/page.tsx    # A2A server settings
│   │   └── manage/                    # Admin pages (same content, different layout)
│   │       ├── layout.tsx             # Admin nav sidebar
│   │       ├── tasks/[id]/page.tsx    # Admin task view
│   │       ├── agents/...             # Admin agent pages
│   │       ├── templates/...          # Admin template pages
│   │       ├── analytics/page.tsx     # Admin analytics
│   │       ├── audit/page.tsx         # Admin audit log
│   │       └── settings/...           # Admin settings
│   ├── components/                    # Reusable React components
│   │   ├── ui/                        # shadcn/ui base components
│   │   │   ├── button.tsx, input.tsx, card.tsx, etc.
│   │   ├── chat/                      # Chat-related components
│   │   │   ├── ChatMessage.tsx        # Message display (agent/user/system)
│   │   │   └── GroupChat.tsx          # Multi-agent chat view
│   │   ├── conversation/              # Conversation UI
│   │   │   ├── ConversationView.tsx   # Main chat + DAG panel
│   │   │   ├── ConversationSidebar.tsx # Sidebar (conversation list)
│   │   │   ├── TopBar.tsx             # Conversation title + settings
│   │   │   ├── DAGPanel.tsx           # DAG visualization (subtask nodes)
│   │   │   └── EmptyState.tsx         # No conversation selected
│   │   ├── task/                      # Task-specific components
│   │   │   ├── DAGView.tsx            # DAG visualization (subtask nodes, dependency arrows)
│   │   │   ├── SubtaskNode.tsx        # Individual subtask card (status, output)
│   │   │   ├── SubtaskDetailPanel.tsx # Subtask details (input, output, error, logs)
│   │   │   └── TraceTimeline.tsx      # Timeline view (subtask start/end times)
│   │   ├── agent/                     # Agent-related components
│   │   │   ├── AgentCard.tsx          # Agent summary card
│   │   │   ├── AdapterForm.tsx        # HTTP polling endpoint config form
│   │   ├── dashboard/                 # Dashboard components
│   │   │   ├── TaskCard.tsx           # Task summary card (title, status, agents)
│   │   │   ├── NewTaskDialog.tsx      # Create task modal (title, description, template select)
│   │   │   └── (other filters, empty states)
│   │   ├── template/                  # Workflow template components
│   │   │   ├── StepEditor.tsx         # Step editor (drag-drop workflow builder)
│   │   ├── OrgProvider.tsx            # Context provider for org/user (if multi-org)
│   │   ├── AuthGuard.tsx              # Wraps layout, checks auth + redirects to login
│   │   └── AppShell.tsx               # App layout shell (header, nav, content)
│   ├── lib/                           # Frontend utilities
│   │   ├── types.ts                   # TypeScript interfaces (mirrors Go models)
│   │   │   - Agent, Task, SubTask, Event, Message, Conversation, etc.
│   │   ├── api.ts                     # Typed API client (fetch wrapper)
│   │   │   - api.tasks.list(), api.tasks.create(), api.conversations.sendMessage(), etc.
│   │   ├── sse.ts                     # SSE connection manager
│   │   │   - connectSSE(taskId, onEvent) → EventSource, auto-reconnect
│   │   │   - connectConversationSSE(conversationId, onEvent)
│   │   ├── store.ts                   # Zustand store (agent list, UI state)
│   │   │   - useAgentStore, useTaskStore, etc.
│   │   ├── conversationStore.ts       # Zustand store for conversation (messages, tasks, SSE)
│   │   └── (other utilities)
│   ├── public/                        # Static assets
│   ├── .env.example                   # Example env vars (NEXT_PUBLIC_API_URL)
│   ├── package.json                   # Frontend deps (Next.js, React, Zustand, shadcn/ui, Tailwind)
│   ├── tailwind.config.ts             # Tailwind CSS configuration
│   ├── next.config.js                 # Next.js configuration
│   └── tsconfig.json                  # TypeScript configuration (strict mode enabled)
├── go.mod                             # Go module definition
├── go.sum                             # Go dependency lock
├── Makefile                           # Build targets (dev-backend, dev-frontend, build, lint, test, check)
├── Dockerfile                         # Docker build (Go + Next.js)
├── CLAUDE.md                          # Project guidelines & conventions
├── SECURITY.md                        # Security guidelines
├── .golangci.yml                      # Go linter config
├── .pre-commit-config.yaml            # Pre-commit hooks
├── .github/
│   ├── workflows/ci.yml               # GitHub Actions CI (lint, build, test)
│   ├── pull_request_template.md       # PR template
│   └── ISSUE_TEMPLATE/
└── .planning/
    └── codebase/                      # Analysis documents (this file lives here)
        ├── ARCHITECTURE.md
        ├── STRUCTURE.md
        ├── CONVENTIONS.md
        ├── TESTING.md
        ├── CONCERNS.md
        ├── STACK.md
        └── INTEGRATIONS.md
```

## Directory Purposes

**cmd/server/**
- Purpose: Backend API server entry point
- Key file: `main.go` — initializes DB, auth, handlers, routes, starts HTTP server
- Triggers: `make dev-backend` or `go run ./cmd/server`

**internal/a2a/**
- Purpose: Agent-to-Agent protocol implementation
- Key files: `protocol.go` (types), `server.go` (incoming), `client.go` (outgoing), `aggregator.go` (multi-turn)
- Exports: A2ATask, A2AMessage, Broker, Server, Client (importable by agent binaries)

**internal/executor/**
- Purpose: DAG execution engine (core orchestration)
- Key file: `executor.go` — Execute(), runDAGLoop(), tryReplan()
- Dependencies: Orchestrator, Broker, EventStore, Audit, A2AClient, PolicyEngine

**internal/handlers/**
- Purpose: HTTP request handlers (API boundary)
- Pattern: One handler type per resource (TaskHandler, AgentHandler, etc.)
- Response format: JSON with helper functions (jsonOK, jsonError)

**internal/models/**
- Purpose: Domain data structures (Go struct = source of truth for API contract)
- Critical: JSON tags must match TypeScript interfaces exactly
- Types: Task, SubTask, Agent, Event, Message, Conversation, Policy, Template

**internal/orchestrator/**
- Purpose: LLM-driven task decomposition
- Key function: Plan() calls claude CLI, returns ExecutionPlan
- Replan() generates replacement subtasks after failure

**internal/events/**
- Purpose: Real-time event system
- Broker: In-memory pub/sub (fanout to SSE subscribers)
- Store: Persistent log in PostgreSQL

**web/app/**
- Purpose: Next.js page components (App Router)
- Pattern: File = route (e.g., `app/tasks/[id]/page.tsx` → `/tasks/:id`)
- Nested: `manage/` subdirectory for admin pages

**web/components/**
- Purpose: Reusable React components (PascalCase files)
- Organized by domain: `ui/` (shadcn), `task/`, `agent/`, `chat/`, `conversation/`

**web/lib/**
- Purpose: Utilities, types, stores
- Critical: `types.ts` must mirror Go models, `api.ts` is typed fetch client
- Store: Zustand for shared state (agents, tasks, UI state)

## Key File Locations

**Entry Points:**
- Backend: `cmd/server/main.go` (HTTP server setup, route registration)
- Frontend: `web/app/layout.tsx` (root page, AuthGuard, nav)
- Agent: `cmd/openaiagent/main.go` (example A2A agent)

**Configuration:**
- Backend: `internal/config/config.go` (env vars: DATABASE_URL, ANTHROPIC_API_KEY, PORT)
- Frontend: `web/.env.local` (NEXT_PUBLIC_API_URL)
- Linting: `.golangci.yml` (Go), `web/tsconfig.json` (TS)

**Core Logic:**
- Task orchestration: `internal/executor/executor.go` (DAGExecutor.Execute, runDAGLoop)
- Task decomposition: `internal/orchestrator/orchestrator.go` (Plan, Replan)
- A2A protocol: `internal/a2a/server.go` (HandleJSONRPC), `internal/a2a/client.go` (Call)
- Real-time events: `internal/events/broker.go` (Publish, Subscribe)

**Data Models:**
- All: `internal/models/` (Task, SubTask, Agent, Event, Message, Conversation, Policy, Template)
- Frontend mirrors: `web/lib/types.ts` (TypeScript interfaces)

**Testing:**
- Go tests: Alongside source (e.g., `executor_test.go`, `orchestrator_test.go`)
- Frontend tests: `web/__tests__/` (Jest/Vitest)
- Integration: `internal/handlers/response_contract_test.go` (Go/TS sync validation)

**Database:**
- Migrations: `internal/db/migrations/` (SQL files, embedded via //go:embed)
- Schema: `001_foundation.sql`, `004_a2a_migration.sql`, `005_remove_org_add_templates.sql`, `006_webhooks.sql`, `007_conversations.sql`

## Naming Conventions

**Files:**

*Backend (Go):*
- Package files: lowercase, `_` for multiple words (e.g., `executor.go`, `response_contract_test.go`)
- Handlers: `{resource}_handler.go` pattern not used; each handler type in its own file (e.g., `tasks.go`, `agents.go`)
- Tests: `{name}_test.go` (same package)

*Frontend (TypeScript):*
- Page components: PascalCase (e.g., `page.tsx`)
- Components: PascalCase (e.g., `TaskCard.tsx`, `ChatMessage.tsx`)
- Utilities: camelCase (e.g., `api.ts`, `sse.ts`, `store.ts`)

**Directories:**
- Backend packages: lowercase, single word or `_` separator (e.g., `executor`, `auth`, `a2a`)
- Frontend components: PascalCase plural (e.g., `components/Chat/`, `components/Task/`)
- Feature grouping: By domain (agent, task, conversation, template)

**Go Code:**
- Functions: camelCase unexported, PascalCase exported
- Methods: Receiver on lines < 80 chars; receiver > 80 chars on separate line
- Interfaces: Descriptive (e.g., `EventStore`, `TaskExecutor`)
- Struct fields: JSON tags in `snake_case` (for TypeScript parity)

**TypeScript Code:**
- Interfaces: PascalCase, with description comment
- Types (unions): PascalCase
- Functions: camelCase
- Constants: UPPER_SNAKE_CASE
- Imports: `from "@/lib/..."` (path alias for `web/lib`)

## Where to Add New Code

**New Feature (e.g., "Task approval"):**
1. **Backend:**
   - Add handler: `internal/handlers/tasks.go` → Add `Approve()` method
   - Update orchestrator: `internal/orchestrator/orchestrator.go` if needs new decomposition logic
   - Update executor: `internal/executor/executor.go` if needs new execution step
   - Add event type: `internal/models/event.go`, publish in executor (e.g., `"approval.requested"`)
   - Add migration: `internal/db/migrations/NNN_feature.sql` (if schema changes)

2. **Frontend:**
   - Add type: `web/lib/types.ts` (mirrors Go model)
   - Add API method: `web/lib/api.ts` (typed fetch call)
   - Add page: `web/app/feature/page.tsx` (if new route)
   - Add component: `web/components/feature/FeatureName.tsx`
   - Add store: `web/lib/featureStore.ts` (if shared state)
   - Update SSE handler: `web/lib/conversationStore.ts` (if new event type)

3. **Tests:**
   - Go: `internal/handlers/tasks_test.go` (handler tests), `internal/executor/executor_test.go` (executor tests)
   - TS: `web/components/feature/FeatureName.test.tsx`

**New Endpoint (e.g., `POST /api/tasks/{id}/pause`):**
1. Define in `internal/models/` if response has new shape
2. Add handler method: `internal/handlers/tasks.go` → `Pause()`
3. Register route: `cmd/server/main.go` → `r.Post("/api/tasks/{id}/pause", taskH.Pause)`
4. Add API method: `web/lib/api.ts` → `tasks.pause(id)`
5. Call from frontend: Use `api.tasks.pause(taskId)` in component
6. Update handler tests: `internal/handlers/tasks_test.go`

**New Component/Module:**
- Feature components: `web/components/{feature}/ComponentName.tsx` (PascalCase, export as default or named)
- Shared utilities: `web/lib/{feature}Utils.ts` or `web/lib/{feature}Store.ts`
- Backend packages: `internal/{package}/` (single-letter receiver for small packages)

**Utilities:**
- Backend: `internal/{package}/` (e.g., `internal/audit/`, `internal/crypto/`)
- Frontend: `web/lib/{name}.ts` (e.g., `web/lib/api.ts`, `web/lib/sse.ts`)

**Shared/Middleware:**
- Auth: `internal/auth/middleware.go` (RequireAuth)
- Context injection: `internal/ctxutil/ctxutil.go` (UserFromCtx)
- Response helpers: `internal/handlers/helpers.go` (jsonOK, jsonError)
- HTTP utilities: `internal/httputil/httputil.go` (CORS, error formatting)

## Special Directories

**internal/db/migrations/:**
- Purpose: Version-controlled schema changes
- Generated: No (manually written SQL)
- Committed: Yes
- Naming: `NNN_description.sql` (sequential, zero-padded)
- Pattern: `IF NOT EXISTS` / `IF EXISTS` for idempotency
- Execution: Automatic on server startup via `RunMigrations()`

**internal/seed/:**
- Purpose: Database seeding (development data)
- Generated: No
- Committed: Yes
- Usage: Called in main.go if cfg.IsLocal() — auto-creates user, org, agents

**web/public/:**
- Purpose: Static assets (images, fonts, etc.)
- Generated: May contain build outputs
- Committed: Assets only (no build artifacts)

**web/.next/:**
- Purpose: Next.js build cache
- Generated: Yes (build output)
- Committed: No (in .gitignore)

**web/node_modules/:**
- Purpose: npm dependencies
- Generated: Yes (npm install)
- Committed: No (in .gitignore)

---

*Structure analysis: 2026-04-04*
