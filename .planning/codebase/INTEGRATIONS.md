# External Integrations

**Analysis Date:** 2026-04-04

## APIs & External Services

**LLM Providers:**
- Anthropic Claude API - Task orchestration and decomposition via CLI (`internal/orchestrator/orchestrator.go`)
  - SDK/Client: Direct CLI invocation (`claude` binary required at runtime)
  - Auth: `ANTHROPIC_API_KEY` environment variable
  - Usage: Master Agent decomposes tasks into DAG subtasks using `callLLM()` function

- OpenAI Chat Completions API - Team agent implementations (`cmd/openaiagent/openai.go`)
  - SDK/Client: Direct HTTP requests (native implementation, no SDK)
  - Auth: `OPENAI_API_KEY` environment variable
  - Model: Configurable via `OPENAI_MODEL` (defaults to `gpt-4o-mini`)
  - Usage: Agents process messages and respond via JSON-RPC A2A protocol

**Google OAuth (Not Yet Implemented):**
- Google OAuth 2.0 - User authentication in cloud mode
  - SDK/Client: Pending implementation
  - Auth: `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` environment variables
  - Handlers present but return `NotImplemented` status in `internal/auth/handlers.go`

## Data Storage

**Databases:**
- PostgreSQL 12+ (required)
  - Connection: `DATABASE_URL` environment variable (default: `postgres://localhost:5432/taskhub?sslmode=disable`)
  - Client: pgx v5.9.1 with connection pooling (`internal/db/db.go`)
  - Pool: `pgxpool.Pool` created in `main()` and injected into handlers
  - Migrations: Embedded SQL in `internal/db/migrations/` directory, auto-run on startup via `db.RunMigrations()`

**File Storage:**
- Local filesystem only - No external file storage service configured

**Caching:**
- None detected - No Redis or memcached integration

## Authentication & Identity

**Auth Provider:**
- Custom implementation with dual modes:
  - `local` mode: No auth required, single default user (`internal/seed/seed.go`)
  - `cloud` mode: Email-only login + Google OAuth (partially implemented)

**Session Management:**
- SessionStore backed by database (`internal/auth/session.go`)
  - Stores sessions in `sessions` table
  - Cookie-based: `sessionCookie` (HttpOnly, Secure, SameSite=Lax)
  - Location: `internal/auth/middleware.go` handles middleware injection

**Implementation Details:**
- SimpleLogin endpoint: Email-only (MVP), creates user if not exists
- GoogleLogin/GoogleCallback: Stubs, return 501 Not Implemented
- Frontend stores user context via Zustand store (`web/lib/store.ts`)

## Monitoring & Observability

**Error Tracking:**
- Not detected - No Sentry, Datadog, or similar integration

**Logs:**
- Standard Go `log` package for backend
- Server entry point uses `log.Printf()` for info/debug, `log.Fatalf()` for fatal errors
- Audit logging available via `internal/audit/audit.go` - Tracks LLM calls with token counts and cost estimates
- Frontend uses `console.*` methods for debugging

**Audit & Compliance:**
- Audit Logger in `internal/audit/audit.go` - Records:
  - LLM provider calls
  - Token usage and cost estimates
  - User actions (available but usage depends on handler implementation)
  - Audit logs queryable via `/api/audit-logs` endpoint (`internal/handlers/auditlog.go`)

## CI/CD & Deployment

**Hosting:**
- Docker (containerized) - Multi-stage Dockerfile in project root
  - Stage 1: Go compilation
  - Stage 2: Next.js build
  - Stage 3: Runtime with Node.js 22 base, runs both services
  - Entry: `/app/start.sh` runs backend and frontend in parallel

**CI Pipeline:**
- GitHub Actions (not configured, placeholder in `.github/workflows/`)
  - Configuration file present: `.github/workflows/ci.yml`
  - Pull request template: `.github/pull_request_template.md`
  - Issue templates: `.github/ISSUE_TEMPLATE/`

**Build Commands:**
- Local: `make build` (Go + Next.js)
- Docker: `make docker-build` and `make docker-run`

## Environment Configuration

**Required env vars (Backend):**
- `DATABASE_URL` - PostgreSQL connection (CRITICAL for startup)
- `ANTHROPIC_API_KEY` - LLM provider for task orchestration

**Required env vars (Frontend):**
- None - All critical frontend URLs default safely to `http://localhost:8080`

**Secrets location:**
- Backend: `.env` file at project root (not committed)
- Frontend: `web/.env.local` file (not committed)
- Both have `.example` variants for reference

**Recommended production changes:**
- `SESSION_SECRET` - Change from default `change-me-in-production`
- `TASKHUB_SECRET_KEY` - Generate a strong symmetric key for agent config encryption
- `FRONTEND_URL` - Set to actual frontend origin for CORS
- Consider enabling Google OAuth by providing `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET`

## Webhooks & Callbacks

**Incoming Webhooks:**
- Not detected - No webhook receiver endpoints

**Outgoing Webhooks:**
- User-configurable webhook notifications available in `internal/webhook/sender.go`
  - Endpoint: `/api/webhooks` (create, list, update, delete, test)
  - Handler: `internal/handlers/webhook.go` (`WebhookHandler`)
  - Payload structure: `WebhookPayload` with event type, task/subtask ID, data, timestamp
  - HMAC-SHA256 signing with `X-TaskHub-Signature` header (uses optional `secret` from config)
  - HTTP timeout: 10 seconds per delivery
  - Async delivery with fire-and-forget pattern (background goroutines)
  - Test endpoint: `/api/webhooks/{id}/test` for synchronous test delivery

**Webhook Events:**
- Event subscriptions stored in `webhook_configs` table
- Event types include (non-exhaustive):
  - Task lifecycle events (started, completed, failed, canceled, approved)
  - Agent lifecycle events (joined, working, done)
  - Message events (new messages sent)
  - Document updates

## Agent-to-Agent Protocol

**A2A Protocol Implementation:**
- Custom JSON-RPC 2.0 protocol for agent communication
- Location: `internal/a2a/` package

**Server Side (Platform):**
- A2A Server: `internal/a2a/server.go`
  - Endpoint: POST `/a2a` (public, no auth)
  - Agent Card Discovery: GET `/.well-known/agent-card.json` (public)
  - Aggregator: Collects agent responses across channels
  - Health Checker: `internal/a2a/health.go` - Polls agent health every 2 minutes

**Client Side (Executor):**
- A2A Client: `internal/a2a/client.go`
  - Sends JSON-RPC requests to agent endpoints
  - HTTP timeout: 5 minutes (generous for LLM agents)
  - Methods: `message/send`, `tasks/send`, `tasks/get`, `tasks/cancel`
  - Returns normalized `SendResult` with task state, artifacts, error messages

**Agent Implementation:**
- Reference implementation: `cmd/openaiagent/main.go`
- Role-based agents: engineering, finance, legal, marketing (configurable via `--role` flag)
- Port selection: Each role has default port (configurable via `--port` flag)
- Protocol handling: Full JSON-RPC 2.0 compliance with error codes
- Conversation history: Per-contextID, max 20 messages (prevent context overflow)
- Async task processing: Tasks return immediately with "working" state, process in background

**A2A Message Format:**
- Text parts in messages (`MessagePart` with `Text` field)
- Artifacts for complex responses (JSON-serializable data)
- Status states: `working`, `completed`, `failed`, `canceled`, `input-required`

## Special Integrations

**Policy Engine:**
- Location: `internal/policy/` package
- Used during task decomposition to constrain which agents can be assigned
- Endpoints: `/api/policies` (CRUD operations)
- Handler: `internal/handlers/policy.go` (`PolicyHandler`)

**Task Templates & Workflow Automation:**
- Location: `internal/models/` (models), `internal/handlers/template.go` (handler)
- Endpoints: `/api/templates` (CRUD), `/api/templates/{id}/analyze` (evolution analysis)
- Template versioning: `template_id` and `template_version` fields in tasks
- Evolution analysis: Learns from execution history to suggest improvements

**Conversation/Channel System:**
- Shared blackboard pattern via `Channel` document
- Endpoints: `/api/conversations`, `/api/conversations/{id}/messages`
- Real-time updates: SSE stream at `/api/conversations/{id}/events`
- Message types: Agent messages, system messages, @mentions for cross-agent collaboration

**Health Monitoring:**
- Agent health tracking: `/api/agents/{id}/health`
- Overview health: `/api/agents/health/overview`
- Handler: `internal/handlers/agent_health.go`
- Health checker runs background goroutines polling agent endpoints

---

*Integration audit: 2026-04-04*
