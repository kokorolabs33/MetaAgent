# Repository Guidelines

- Repo: taskhub (enterprise multi-agent collaboration platform)
- File references must be repo-root relative (e.g. `internal/master/agent.go:35`), never absolute paths.

## Agent Skills

Use these skills for structured workflows. Read the SKILL.md before starting the workflow.

- **Code review:** Use `$code-review` at `.agents/skills/code-review/SKILL.md` for reviewing code changes and PRs.
- **PR management:** Use `$pr-maintainer` at `.agents/skills/pr-maintainer/SKILL.md` for triaging, reviewing, and landing PRs.
- **Add feature:** Use `$add-feature` at `.agents/skills/add-feature/SKILL.md` for full-stack feature implementation checklists.

## Project Structure

Go code lives at the project root (standard Go layout); frontend lives in `web/`.

```
taskhub/
├── cmd/
│   ├── server/main.go             # Platform API server
│   └── openaiagent/main.go        # OpenAI-powered A2A agents
├── internal/
│   ├── config/                    # Environment configuration
│   ├── db/                        # Database connection + migrations
│   │   └── migrations/            # Embedded SQL migration files
│   ├── models/                    # Domain structs
│   ├── handlers/                  # HTTP handlers
│   ├── orchestrator/              # Task decomposition via LLM
│   ├── executor/                  # DAG execution engine + poll manager
│   ├── adapter/                   # Agent adapter layer
│   │   ├── adapter.go             # Interface
│   │   ├── http_poll.go           # HTTP polling adapter
│   │   └── native.go             # Native protocol adapter
│   ├── auth/                      # Authentication middleware
│   ├── rbac/                      # Permission checking
│   ├── ctxutil/                   # Context value helpers (user, org, role)
│   ├── httputil/                  # HTTP response helpers (JSON, errors)
│   ├── events/                    # Event store + SSE broker
│   ├── audit/                     # Audit logging
│   └── testutil/                  # Test infrastructure
├── web/                           # Next.js frontend
│   ├── app/
│   │   ├── page.tsx               # Dashboard
│   │   ├── tasks/[id]/page.tsx    # Task detail (DAG + Chat)
│   │   ├── agents/page.tsx        # Agent list
│   │   ├── agents/[id]/page.tsx   # Agent detail
│   │   └── agents/register/page.tsx # Register agent wizard
│   ├── components/
│   │   ├── dashboard/             # Task cards, filters
│   │   ├── task/                  # DAG view, status nodes
│   │   ├── chat/                  # Group chat, message feed
│   │   ├── agent/                 # Agent cards, config forms
│   │   └── ui/                    # shadcn/ui base components
│   └── lib/
│       ├── types.ts               # TypeScript interfaces
│       ├── api.ts                 # Typed API client
│       ├── sse.ts                 # SSE connection manager
│       └── store.ts              # Zustand stores
├── go.mod
├── go.sum
├── Makefile
├── Dockerfile
├── CLAUDE.md
├── SECURITY.md
├── .golangci.yml
├── .pre-commit-config.yaml
└── .github/
    ├── workflows/ci.yml
    ├── pull_request_template.md
    └── ISSUE_TEMPLATE/
```

## Architecture

- **Master Agent** decomposes tasks via LLM, creates a Channel (shared blackboard), assigns Team Agents sequentially.
- **Agents** communicate through the shared Channel document; @mentions enable cross-agent collaboration (up to 3 rounds).
- **SSE** streams real-time updates to the frontend per channel.
- **Audit Logger** tracks every LLM call with token counts and cost estimates.

## Build, Test, and Development Commands

- Install all deps: `make install`
- Run backend: `make dev-backend`
- Run frontend: `make dev-frontend`
- Build: `make build`
- Lint: `make lint` (runs golangci-lint + eslint)
- Format: `make fmt` / `make fmt-check`
- Type-check frontend: `make typecheck`
- Test: `make test` (runs Go tests + frontend tests)
- Full quality gate (pre-push): `make check` (format + lint + typecheck + build)
- Database setup: `make db-create` / `make db-reset`
- Docker: `make docker-build` / `make docker-run`

### Hard gates

- `make check` MUST pass before pushing to `main`.
- If a change affects API endpoints, migrations, or shared types, both backend and frontend must build successfully.
- Do not commit or push with failing lint, format, type, build, or test checks caused by your change.

## Coding Style & Conventions

### Go (backend)

- Follow standard Go conventions: `gofmt`, short variable names, early returns.
- Error handling: always check and propagate errors; never silently ignore with `_ =` (except `godotenv.Load()`).
- Naming: `camelCase` for unexported, `PascalCase` for exported. Package names are lowercase, single-word.
- SQL migrations: sequential numbering (`001_init.sql`, `002_xxx.sql`). Migrations are embedded in `internal/db/migrations/` via `//go:embed`.
- JSON tags: use `snake_case` to match frontend interfaces.
- Do not use `panic()` in production code; use `log.Fatalf` only in `main()`.
- Keep handlers thin — business logic belongs in dedicated packages (e.g. `master/`, `audit/`).

### TypeScript (frontend)

- Strict mode enabled; do not add `@ts-ignore` or `@ts-nocheck`.
- Avoid `any` — use proper types or `unknown` with type guards. ESLint enforces `no-explicit-any` as error.
- Frontend types in `web/lib/types.ts` MUST mirror Go model JSON tags exactly.
- Use `interface` for data shapes, `type` for unions/intersections.
- Component files: PascalCase (e.g. `TaskBar.tsx`). Utility files: camelCase (e.g. `api.ts`).
- State management: Zustand only. No prop drilling for shared state.
- Styling: Tailwind CSS utility classes. Use shadcn/ui components from `components/ui/`.

### Frontend-Backend Type Contract

- Go models (`internal/models/`) are the source of truth for data shapes.
- TypeScript interfaces (`web/lib/types.ts`) must stay in sync with Go JSON serialization.
- When adding/modifying a model field:
  1. Update the Go struct with proper `json:"field_name"` tag
  2. Update the SQL migration if it's a new column
  3. Update the TypeScript interface to match
  4. Update API client functions if the endpoint signature changes

## API Endpoints

All endpoints are under `/api`:

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/agents` | AgentHandler.List | List all agents |
| POST | `/agents` | AgentHandler.Create | Create a new agent |
| GET | `/tasks` | TaskHandler.List | List all tasks |
| POST | `/tasks` | TaskHandler.Create | Create task + start Master Agent |
| GET | `/tasks/:id` | TaskHandler.Get | Get task by ID |
| GET | `/tasks/:id/channel` | TaskHandler.GetChannel | Get task's channel ID |
| GET | `/tasks/:id/audit` | TaskHandler.GetAudit | Get audit trail |
| GET | `/tasks/:id/cost` | TaskHandler.GetCost | Get token/cost summary |
| GET | `/channels/:id` | ChannelHandler.Get | Get channel detail (channel + messages + agents) |
| GET | `/channels/:id/stream` | ChannelHandler.Stream | SSE stream for channel |
| POST | `/chat` | ChatHandler.Chat | Chat with Master Agent |

### Adding a New Endpoint

1. Define the handler method in `internal/handlers/`
2. Register the route in `cmd/server/main.go`
3. Add the API client function in `web/lib/api.ts`
4. Add any new types to both `internal/models/models.go` and `web/lib/types.ts`

## SSE Event Types

Events are JSON objects with `type` and `data` fields:

- `task_started` — Master Agent begins processing
- `channel_created` — Channel established for task
- `agent_joined` — Agent added to channel
- `agent_working` — Agent begins its turn
- `message` — Agent sends a message
- `document_updated` — Shared document was modified
- `agent_done` — Agent finished its turn
- `task_completed` — All agents finished

When adding a new event type:
1. Add it to the backend SSE publish calls
2. Add it to `SSEEventType` union in `web/lib/types.ts`
3. Handle it in the Zustand store's `handleSSEEvent` method

## Database

- PostgreSQL with `pgx/v5` driver.
- Connection pool via `pgxpool`.
- Migrations run on startup via embedded SQL files in `internal/db/migrations/`.
- IDs: UUID strings (generated with `github.com/google/uuid`).
- Timestamps: `TIMESTAMPTZ` in SQL, `time.Time` in Go, ISO string in JSON.

### Adding a New Migration

1. Create `internal/db/migrations/NNN_description.sql` (next sequential number)
2. Add the filename to the migration list in `internal/db/migrate.go`
3. Use `IF NOT EXISTS` / `IF EXISTS` for idempotency
4. Migrations run automatically on server startup

## Configuration

Backend env vars (from `.env` or environment):

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | `postgres://localhost:5432/taskhub?sslmode=disable` | PostgreSQL connection string |
| `ANTHROPIC_API_KEY` | (none) | API key for LLM calls |
| `PORT` | `8080` | Backend server port |

Frontend env vars (from `web/.env.local`):

| Variable | Default | Description |
|----------|---------|-------------|
| `NEXT_PUBLIC_API_URL` | `http://localhost:8080` | Backend API base URL |

- Never commit `.env` or `.env.local` files.
- Use `.env.example` files to document required variables.
- CORS is configured to allow `http://localhost:3000` (Next.js dev server).

## Security

- Never commit secrets, API keys, or `.env` files.
- Validate all user input at handler boundaries.
- Use parameterized queries (pgx) — never construct SQL with string concatenation.
- CORS whitelist: only add origins that are actually needed.

## Commit & PR Guidelines

- Commit messages: concise, action-oriented (e.g. `feat(backend): add audit endpoint`, `fix(web): SSE reconnection race`).
- Use conventional commit prefixes: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`.
- Scope with `(backend)` or `(web)` when the change is single-stack.
- Group related changes; avoid bundling unrelated refactors.
- Run `make check` before pushing.

## Agent Capabilities

### Adding a New Agent

1. Add the agent definition in `internal/seed/seed.go`
2. Give it a unique color, name, description, and system prompt
3. The agent will be auto-created on next server startup (upsert)

### Modifying Agent Behavior

- Agent system prompts live in `seed.go` and are seeded on startup
- The Master Agent's decomposition prompt is in `internal/master/agent.go`
- @mention handling and round-robin logic is in the same file
- Each agent gets: its system prompt + mention instructions + shared channel context

### Adding New Agent Features

When extending agent capabilities:
1. Consider if it's a Master Agent feature (decomposition/orchestration) or Team Agent feature
2. Update the relevant system prompt
3. If adding a new tool/action, add the handler + SSE event type
4. Update frontend to render the new capability
5. Add audit logging for any new LLM calls

## Skill routing

When the user's request matches an available skill, ALWAYS invoke it using the Skill
tool as your FIRST action. Do NOT answer directly, do NOT use other tools first.
The skill has specialized workflows that produce better results than ad-hoc answers.

Key routing rules:
- Product ideas, "is this worth building", brainstorming → invoke office-hours
- Bugs, errors, "why is this broken", 500 errors → invoke investigate
- Ship, deploy, push, create PR → invoke ship
- QA, test the site, find bugs → invoke qa
- Code review, check my diff → invoke review
- Update docs after shipping → invoke document-release
- Weekly retro → invoke retro
- Design system, brand → invoke design-consultation
- Visual audit, design polish → invoke design-review
- Architecture review → invoke plan-eng-review

<!-- GSD:project-start source:PROJECT.md -->
## Project

**TaskHub**

TaskHub is an open-source A2A (Agent-to-Agent) protocol meta-agent platform that demonstrates how companies can use the A2A protocol for collaborative multi-agent task completion. It provides a Master Agent that decomposes user requests into subtask DAGs, orchestrates specialized sub-agents via A2A protocol, and offers real-time observability of the entire collaboration process. Built with Go + Next.js + PostgreSQL, targeting the developer community as a reference implementation and exploration of A2A-powered workflows.

**Core Value:** Developers can experience a complete A2A multi-agent collaboration flow — from chat-driven task creation through real-time observation of agent coordination to task completion — in a polished, open-source package they can clone and run.

### Constraints

- **Tech stack**: Go 1.26 + Next.js 16 + PostgreSQL + pgx — maintain existing stack, new dependencies allowed if justified
- **A2A compliance**: Must remain compatible with A2A protocol specification (JSON-RPC 2.0)
- **Database**: PostgreSQL with embedded migrations — no ORM, raw SQL via pgx
- **Frontend patterns**: shadcn/ui + Tailwind CSS + Zustand stores — maintain consistency
- **Quality gates**: `make check` must pass (format + lint + typecheck + build) before any merge
<!-- GSD:project-end -->

<!-- GSD:stack-start source:codebase/STACK.md -->
## Technology Stack

## Languages
- Go 1.26.1 - Backend API server (`cmd/server/main.go`), task orchestration, and execution engine
- TypeScript 5.x - Next.js frontend application (`web/`)
- JavaScript (via package.json scripts) - Build tooling and frontend automation
## Runtime
- Go 1.26.1 - Backend compilation and execution
- Node.js 22.x (Bookworm) - Frontend development and Next.js server
- Docker (multi-stage builds) - Container deployment
- Go modules (`go.mod`, `go.sum`) - Backend dependency management
- pnpm - Frontend package manager (`web/package.json`)
## Frameworks
- Chi v5.2.5 - HTTP router and middleware (`github.com/go-chi/chi/v5`)
- Chi CORS v1.2.2 - Cross-origin request handling (`github.com/go-chi/cors`)
- pgx v5.9.1 - PostgreSQL driver with connection pooling (`github.com/jackc/pgx/v5`)
- godotenv v1.5.1 - Environment variable loading (`github.com/joho/godotenv`)
- google/uuid v1.6.0 - UUID generation for IDs (`github.com/google/uuid`)
- Next.js 16.1.6 - React meta-framework for SSR and static generation
- React 19.2.3 - UI component library
- React DOM 19.2.3 - DOM rendering
- TypeScript 5.x - Type safety for all frontend code
- Tailwind CSS v4 - Utility-first CSS framework (`@tailwindcss/postcss`)
- shadcn/ui components - Pre-built accessible UI components
- Lucide React v0.577.0 - Icon library
- Class Variance Authority v0.7.1 - Component variant management
- clsx v2.1.1 - Conditional class name utility
- tailwind-merge v3.5.0 - Tailwind class merging
- React Flow (@xyflow/react) v12.10.1 - DAG visualization for task execution plans
- react-markdown v10.1.0 - Markdown rendering for rich content
- Zustand v5.0.11 - Lightweight client-side state management (`web/lib/store.ts`)
- tw-animate-css v1.4.0 - Animation utilities
- Base UI (@base-ui/react) v1.3.0 - Headless UI component library
## Key Dependencies
- jackc/pgx/v5 - PostgreSQL client with connection pooling; essential for database operations
- chi/v5 - HTTP routing foundation for all API endpoints
- google/uuid - ID generation for tasks, users, agents (used throughout the system)
- React 19.2.3 - Core frontend UI framework
- Next.js 16.1.6 - Server-side rendering and frontend routing
- chi/cors - CORS configuration for frontend requests from `http://localhost:3000` (dev) or configured `FRONTEND_URL`
- godotenv - Environment variable loading for configuration
- Tailwind CSS - CSS generation and optimization
- ESLint 9.x - Frontend linting (`web/package.json`)
## Configuration
- `DATABASE_URL` - PostgreSQL connection string (default: `postgres://localhost:5432/taskhub?sslmode=disable`)
- `ANTHROPIC_API_KEY` - Anthropic API key for LLM-based task orchestration
- `OPENAI_API_KEY` - OpenAI API key for team agents (`cmd/openaiagent`)
- `PORT` - Server port (default: `8080`)
- `TASKHUB_MODE` - `local` (no auth) or `cloud` (full auth) (default: `local`)
- `GOOGLE_CLIENT_ID` - Google OAuth client ID (not yet implemented)
- `GOOGLE_CLIENT_SECRET` - Google OAuth client secret (not yet implemented)
- `SESSION_SECRET` - Session encryption key (must be changed in production)
- `TASKHUB_SECRET_KEY` - Symmetric key for encrypting agent auth configs at rest
- `FRONTEND_URL` - Frontend origin for CORS (default: `http://localhost:3000`)
- `NEXT_PUBLIC_API_URL` - Backend API base URL (default: `http://localhost:8080`)
- `Dockerfile` - Multi-stage Docker build: Go binary in stage 1, Next.js in stage 2, runtime stage 3
- `Makefile` - Build and development commands
- `web/tsconfig.json` - TypeScript configuration with strict mode enabled, path aliases (`@/*` → `./`)
- `web/package.json` - Scripts: `dev`, `build`, `start`, `lint`
- Next.js uses `.next/` directory for build output (not committed)
- `.golangci.yml` - golangci-lint configuration
- `gofmt` - Go code formatter (via `make fmt`)
- ESLint 9.x with next config - Linting rules
- Prettier (via pnpm) - Code formatter (`make fmt-frontend`)
## Platform Requirements
- Go 1.26.1 (local build)
- Node.js 22.x (local build for frontend)
- PostgreSQL 12+ (local database)
- Make (build automation)
- docker/docker-compose (optional, for containerized development)
- pre-commit hooks (optional, configured in `.pre-commit-config.yaml`)
- Docker runtime with:
- PostgreSQL 12+ with pgx driver
- Connection pooling via pgxpool
- Migrations: SQL files in `internal/db/migrations/` (embedded via `//go:embed`)
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

## Naming Patterns
### Go Files
- Exported: `PascalCase` (e.g., `UserFromCtx`, `RequireAuth`, `HandleJSONRPC`)
- Unexported: `camelCase` (e.g., `mapStatusToA2AState`, `pollUntilTerminal`, `decodeJSON`)
- Test functions: `TestXxx` format (e.g., `TestRequireAuth_LocalMode_InjectsLocalUser`)
- Short variable names preferred: `ctx` for context, `t` for *testing.T, `w` for http.ResponseWriter, `r` for *http.Request, `rec` for httptest.Recorder
- Loop variables: `i`, `j`, `err`, `ok`, etc.
- Struct fields: `PascalCase` for exported, `camelCase` for unexported
- Exported: `PascalCase` (if needed)
- Unexported: `camelCase` (e.g., `ctxKeyUser`, `ctxKeyRole` in `internal/ctxutil/ctxutil.go:9-13`)
- Single word, lowercase: `ctxutil`, `models`, `handlers`, `executor`, `audit`
- Import paths use `taskhub` prefix (local prefixes configured in `.golangci.yml`)
- Use `snake_case` for JSON field names to match frontend interfaces
- Examples: `json:"id"`, `json:"avatar_url"`, `json:"created_at"`, `json:"auth_provider_id"`
- Omit field with `json:"field,omitempty"` for optional fields
- See `internal/models/models.go:7-26` for struct tag examples
### TypeScript Files
- File names: `PascalCase` (e.g., `EmptyState.tsx`, `TaskBar.tsx`)
- Exported component names: `PascalCase`
- React hooks with names starting with `use`: `useCallback`, `useRef`, `useState`
- File names: `camelCase` (e.g., `api.ts`, `types.ts`, `store.ts`, `sse.ts`)
- Helper functions: `camelCase`
- Interfaces: `PascalCase` (e.g., `User`, `Agent`, `Task`, `Message`)
- Type aliases: `PascalCase` (used for unions/intersections)
- Example interfaces in `web/lib/types.ts:1-149`
- Constants: `camelCase` or `UPPERCASE` based on context (e.g., `suggestions`, `BASE` in `web/lib/api.ts`)
- State variables: `camelCase` (e.g., `isLoading`, `isCreating`, `currentTask`)
- Event handlers: `handleXxx` pattern (e.g., `handleSubmit`, `handleKeyDown`)
## Code Style
### Go Style
- `gofmt` enforced (checked in pre-commit and CI)
- `goimports` enabled for import organization (`.golangci.yml:50-52`)
- No manual grouping of imports — tool handles it with local prefix `taskhub`
- Always check errors, never silently ignore with `_ =`
- Exception: `godotenv.Load()` is excluded in linter config (`.golangci.yml:32-33`)
- Use early returns for error cases (standard Go idiom)
- Example pattern in `cmd/server/main.go:36-44`:
- Early returns preferred over nested if/else
- Use `defer` for cleanup (e.g., `defer pool.Close()`)
- Never use `panic()` in production code
- Only `log.Fatalf()` in `main()` functions
- Keep handlers thin — business logic in dedicated packages
- Use dependency injection: pass dependencies to handler constructors
- Example in `cmd/server/main.go:62-80`: handlers receive DB, resolver, broker, etc.
### TypeScript Style
- TypeScript strict mode enabled in `web/tsconfig.json:7`
- Never add `@ts-ignore` or `@ts-nocheck`
- ESLint enforces `@typescript-eslint/no-explicit-any: "error"` in `web/eslint.config.mjs:11`
- Use `interface` for data shapes (e.g., `User`, `Agent`, `Task` in `web/lib/types.ts`)
- Use `type` for unions/intersections
- Avoid `any` — use `unknown` with type guards if needed
- Example from `web/lib/api.ts:23-26`:
- "use client" directive for client components
- `useCallback` for event handlers to prevent unnecessary re-renders
- `useRef` for uncontrolled DOM access
- Zustand for global state (no prop drilling) — see `web/lib/store.ts`
- Only Zustand stores allowed (not Context, Redux, etc.)
- Zustand stores defined with `create<StoreName>((set, get) => ({ ... }))`
- Example in `web/lib/store.ts:19-44`: agent store with `loadAgents`, `registerAgent`, `deleteAgent`
- Clear, action-oriented messages
- Include endpoint info: `GET /path -> 404`
- Example in `web/lib/api.ts:25`: `throw new Error(\`GET ${path} -> ${res.status}\`)`
- Tailwind CSS utility classes only
- shadcn/ui components from `web/components/ui/`
- Use Tailwind's responsive classes (e.g., `flex h-full flex-col items-center justify-center px-4`)
- Example in `web/components/conversation/EmptyState.tsx:56-72`
## Import Organization
### Go
- Configured in `.golangci.yml:51-52`: `taskhub`
- All internal imports use this prefix: `import "taskhub/internal/models"`
### TypeScript
- `@/*` maps to web root (e.g., `@/components/ui/button`)
- Use aliases to avoid relative paths (`../../../`)
## Error Handling
## Logging
### Go
- `log.Printf()` for informational messages (e.g., `cmd/server/main.go:33`: `log.Printf("TaskHub — mode: %s", cfg.Mode)`)
- `log.Fatalf()` only in `main()` for fatal errors
- Example: `cmd/server/main.go:38`: `log.Fatalf("db open: %v", err)`
### TypeScript
- `console.warn()` and `console.error()` allowed
- `console.log()` and `console.debug()` flagged as warnings by ESLint (`web/eslint.config.mjs:18`)
- Use `console.error()` for errors in state management catch blocks
## Comments
- Public functions/types: must have godoc comment starting with name
- Complex business logic: explain the why, not the what
- Non-obvious decisions: trade-offs, performance notes
- Not consistently used in codebase
- Functions are self-documenting via type signatures
- If adding JSDoc, use standard format:
## Function Design
### Go Functions
- Keep functions small and focused
- Test files show typical function patterns: `internal/handlers/handlers_test.go:11-31` shows a test for a small, focused decode function
- First parameter: `context.Context` if the function does I/O
- Use named return values when returning multiple values: Example in `internal/models/models.go:28-33`
- Receiver (method): `(receiver *ReceiverType)` not `(this *ReceiverType)`
- Standard Go pattern: `(result T, err error)`
- Named returns optional but clear
- Example: `func NewPageRequest(cursor string, limit int) PageRequest` (line 28 in models.go)
### TypeScript Functions
- Use `async/await`, not `.then()` chains
- Example in `web/lib/api.ts:23-26`: all API functions are async
- Return typed Promises: `Promise<T>`
- Wrap event handlers in `useCallback()` to prevent unnecessary re-renders
- Example in `web/components/conversation/EmptyState.tsx:24-43`:
- Use object destructuring for multiple parameters
- Example in `web/lib/api.ts:82-89`: `list(params?: { status?: string; q?: string; page?: number; per_page?: number })`
## Module Design
### Go Modules
- Exported types/functions: `PascalCase`
- Unexported: `camelCase`
- Example from `internal/ctxutil/ctxutil.go`: `UserFromCtx` (exported), `ctxKeyUser` (unexported constant)
- Not used — each package is small and focused
- Imports are direct: `import "taskhub/internal/models"`
- Handlers and services receive dependencies as struct fields
- Example in `cmd/server/main.go:62-80`: `AgentHandler{DB, Resolver}`, `DAGExecutor{DB, Broker, ...}`
### TypeScript Modules
- Named exports for utils, stores, types: `export const useAgentStore = ...`
- Default export for page components (Next.js requirement)
- API client is a barrel: `web/lib/api.ts` exports single `api` object with namespaced methods
- Example from `web/lib/api.ts:59-150`:
- Each store is its own file or grouped logically
- Example: `web/lib/store.ts` contains `useAgentStore`, `useTaskStore`
- Stores use closure pattern: `create<StoreName>((set, get) => ({ ... }))`
## Frontend-Backend Type Synchronization
- Go models in `internal/models/models.go` are source of truth
- TypeScript interfaces in `web/lib/types.ts` MUST match Go JSON tags exactly
- Example synchronization:
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

## Pattern Overview
- **Master Orchestrator Pattern**: LLM-driven task decomposition creates DAG (Directed Acyclic Graph) of subtasks
- **A2A Protocol**: Agent-to-Agent JSON-RPC 2.0 communication (supports HTTP polling and native adapters)
- **Event-Driven Real-Time Updates**: In-memory broker + persistent event store for SSE streaming to frontend
- **Policy-Driven Execution**: Constraint engine validates task plans before execution
- **Human-in-the-Loop**: Approval workflows and input-required states for non-autonomous execution
- **Adaptive Replanning**: Automatic failure recovery via LLM-guided subtask replanning
## Layers
- Location: `web/app/` (Next.js 13+ App Router), `web/components/`, `web/lib/`
- Contains: Page components, UI forms, real-time chat, DAG visualization, agent registry UI
- Depends on: API client (`web/lib/api.ts`), SSE connection manager (`web/lib/sse.ts`), Zustand stores
- Used by: End users (task creation, agent management, real-time monitoring)
- Location: `internal/handlers/`
- Contains: Request/response handlers for tasks, agents, conversations, webhooks, policies, analytics
- Handlers: `TaskHandler`, `AgentHandler`, `ConversationHandler`, `StreamHandler`, `MessageHandler`, `PolicyHandler`, `TemplateHandler`, `WebhookHandler`, `A2AConfigHandler`, `AnalyticsHandler`, `TraceHandler`, `AuditLogHandler`
- Depends on: Orchestrator, Executor, Event Store, Broker, Audit Logger, A2A resolver/aggregator
- Pattern: Thin handlers — business logic delegated to dedicated packages
- Location: `internal/orchestrator/orchestrator.go`
- Purpose: LLM-driven task decomposition into subtask DAGs
- Functions:
- Output: `ExecutionPlan` with subtasks, dependencies, and agent assignments
- Location: `internal/executor/executor.go`, `internal/executor/recovery.go`
- Core type: `DAGExecutor`
- Responsibilities:
- Key methods:
- Location: `internal/a2a/`
- Components:
- Adapter types:
- Communication: JSON-RPC 2.0 with A2A message envelope (role + text/artifact parts)
- Location: `internal/events/`
- Components:
- Flow: Event published → stored in DB → fanned out via broker → SSE streamed to frontend
- Location: `internal/models/`
- Core entities:
- Location: `internal/policy/engine.go`
- Purpose: Constraint evaluation before task execution
- Evaluates: Task title/description against defined policies
- Returns: Applied policies, format for LLM prompt, approval thresholds
- Used by: Executor to gate execution and provide policy guidance to orchestrator
- Location: `internal/db/`
- Driver: PostgreSQL via pgx/v5 with connection pooling
- Migrations: Embedded SQL files in `internal/db/migrations/`
- Transaction handling: Row-level locking where needed (subtask status updates)
- Connection: Single `*pgxpool.Pool` injected across handlers/executor
- Location: `internal/auth/`, `internal/rbac/`
- Middleware: `RequireAuth` checks session cookies
- Local mode: No auth required (auto-seed user)
- Cloud mode: Google OAuth2 or email login
- RBAC: Role-based access control (future expansion)
- Location: `internal/ctxutil/`
- Extracts: User, org, role from request context
- Used throughout: Handlers inject user context for audit/permission checks
- Location: `internal/audit/`
- Tracks: Every LLM call with model, input tokens, output tokens, cost estimate
- Used by: Executor, handler (cost endpoint)
- Location: `internal/webhook/`
- Purpose: Event-triggered notifications to external URLs
- Sends: Task status changes, subtask completion, policy violations
- Retry logic: Built-in retry mechanism
## Data Flow
## Key Abstractions
- Purpose: Logical blueprint for task execution
- Location: `internal/models/task.go`
- Structure: Summary + list of PlanSubTask (with temp IDs and dependencies)
- Pattern: Temp IDs allow DAG layout before UUID assignment
- Purpose: Manages full task lifecycle from plan to completion
- Location: `internal/executor/executor.go`
- Injected into: Handler, A2A Server (for recursive task creation)
- Key state: `cancels` map (task_id → context.CancelFunc)
- Purpose: Immutable record of state changes
- Location: `internal/models/event.go`
- Dual streaming: Broker (live) + Store (audit trail)
- Purpose: Standard wire format for agent communication
- Location: `internal/a2a/protocol.go`
- Parts: Text (instructions) + Artifacts (file/document references)
- Purpose: Track multi-turn agent conversations
- Location: `internal/a2a/aggregator.go`
- State: contextId → conversation history
- Used by: A2A client when composing prompts for subsequent calls
## Entry Points
- Location: `cmd/server/main.go`
- Triggers: `make dev-backend` or `./server`
- Responsibilities:
- Location: `web/app/layout.tsx` (root), `web/app/page.tsx` (dashboard)
- Triggers: `make dev-frontend` or `npm run dev`
- Entry flow:
- Location: `internal/executor/recovery.go` (Recover method)
- Triggers: Server startup (called in main.go)
- Responsibilities:
- Location: `internal/a2a/server.go` (HandleJSONRPC)
- Triggers: POST /a2a with JSON-RPC request
- Methods handled: tasks/send, tasks/get, tasks/cancel
- Creates: Internal tasks from incoming A2A requests (enables agent-to-agent communication)
## Error Handling
## Cross-Cutting Concerns
- Backend: Standard library `log` package
- Key events: Task lifecycle, executor decisions, LLM calls, agent invocations
- No structured logging (MVP — consider zerolog/slog upgrade)
- Handler entry: Decode JSON, check required fields
- Subtask creation: Validate agent IDs exist, dependencies form DAG (no cycles)
- Task title: Required, trimmed
- Agent endpoint: HTTP URL validation on registration
- Middleware: `RequireAuth` wraps authenticated routes
- Local mode: Bypassed (auto-user)
- Cloud mode: Google OAuth2 + session cookies
- User injected: Via context (extractable in handlers)
- RBAC table exists but unused (future: per-task role checks)
- Currently: User can see/modify only their own tasks/conversations
- DAG loop uses sync.WaitGroup for goroutine tracking
- Broker uses sync.RWMutex for subscriber map
- Subtask status updates: DB row-level locking prevents conflicts
- Task cancellation: context.CancelFunc stored per task
- Events: Audit trail (all state changes)
- Audit Log: Every LLM call with token/cost
- Webhooks: Event-based notifications
- Timeline: Trace of subtask starts/ends (for DAG visualization)
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

| Skill | Description | Path |
|-------|-------------|------|
| gstack | \| Fast headless browser for QA testing and site dogfooding. Navigate pages, interact with elements, verify state, diff before/after, take annotated screenshots, test responsive layouts, forms, uploads, dialogs, and capture bug evidence. Use when asked to open or test a site, verify a deployment, dogfood a user flow, or file a bug with screenshots. (gstack) | `.claude/skills/gstack/SKILL.md` |
| add-feature |  | `.agents/skills/add-feature/SKILL.md` |
| code-review |  | `.agents/skills/code-review/SKILL.md` |
| pr-maintainer |  | `.agents/skills/pr-maintainer/SKILL.md` |
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->

<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
