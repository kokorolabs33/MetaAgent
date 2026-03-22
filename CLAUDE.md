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
│   ├── mockagent/main.go          # Mock Agent for testing
│   └── llmagent/main.go           # Real LLM Agent (manual testing)
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
