<p align="center">
  <h1 align="center">TaskHub</h1>
  <p align="center">
    <strong>Orchestrate, manage, and govern AI agents — Kubernetes for Agents.</strong>
  </p>
  <p align="center">
    <a href="#quick-start">Quick Start</a> &bull;
    <a href="#features">Features</a> &bull;
    <a href="#architecture">Architecture</a> &bull;
    <a href="#documentation">Docs</a> &bull;
    <a href="#contributing">Contributing</a>
  </p>
</p>

---

TaskHub is an open-source platform for unified orchestration, management, and governance of AI agents. Think of it as **Kubernetes, but for AI agents and tasks** — you describe what you want done, and TaskHub decomposes it into subtasks, routes them to the right agents, manages execution, and streams results back in real-time.

**TaskHub does not run agents.** It orchestrates them. Your agents live wherever they are — any HTTP endpoint that can accept a task and return results can be plugged into TaskHub via adapters.

<!-- TODO: Add hero screenshot
![TaskHub Dashboard](docs/images/dashboard.png)
-->

## Features

- **Agent Registry** — Register any external agent with a simple HTTP endpoint. No SDK required, no agent modification needed.
- **DAG Execution** — Tasks are automatically decomposed into subtask DAGs. Parallel when possible, sequential when dependent.
- **Adapter System** — Plug in any agent via configurable HTTP polling adapters or the native TaskHub protocol. JSONPath mapping for custom APIs.
- **Real-time Streaming** — SSE-based event stream with persistent event store. Browser auto-reconnect with zero event loss.
- **Group Chat** — Every task gets a chat room. Agents and users communicate via @mentions. Agents can request human input mid-execution.
- **Human-in-the-Loop** — Agents can pause and ask for confirmation. Users @reply in the chat to continue execution.
- **DAG Visualization** — React Flow pipeline view showing subtask status, dependencies, and progress in real-time.
- **Audit & Cost Tracking** — Every LLM call and agent invocation is logged with token counts and cost estimates.
- **Budget Control** — Set monthly spend limits per organization. Execution halts when budget is exceeded.
- **RBAC** — Organization-based multi-tenancy with four roles: owner, admin, member, viewer.

<!-- TODO: Add feature screenshots
![Task Detail - DAG + Chat](docs/images/task-detail.png)
![Agent Registration](docs/images/agent-register.png)
-->

## Architecture

```
User → Frontend (Next.js) → Backend API (Go/chi)
                                    │
                              ┌─────┴─────┐
                              │ Orchestrator│  ← LLM decomposes tasks
                              └─────┬─────┘
                                    │
                              ┌─────┴─────┐
                              │ DAG Executor│  ← Manages subtask lifecycle
                              └─────┬─────┘
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
              ┌─────┴─────┐  ┌─────┴─────┐  ┌─────┴─────┐
              │  Agent A   │  │  Agent B   │  │  Agent C   │
              │ (external) │  │ (external) │  │ (external) │
              └───────────┘  └───────────┘  └───────────┘
```

**Key design principles:**

- **Agents are external** — TaskHub orchestrates, it doesn't run agent code. Agents are HTTP services you register.
- **Adapter pattern** — Any HTTP API can be an agent. Configure JSON request/response mapping, no code changes to your agent.
- **Event-sourced** — Every state change is persisted as an event. Full replay, audit trail, and real-time SSE streaming.
- **Temporal-ready** — The executor interface is designed to swap in Temporal for durable execution when you outgrow the simple goroutine-based executor.

## Quick Start

### Prerequisites

- Go 1.22+
- Node.js 22+
- PostgreSQL 15+
- pnpm

### 1. Clone and install

```bash
git clone https://github.com/your-org/taskhub.git
cd taskhub
make install
```

### 2. Set up database

```bash
createdb taskhub
```

### 3. Start the backend

```bash
make dev-backend
```

The server starts on `http://localhost:8080` in **local mode** — no authentication required, a default workspace is created automatically.

### 4. Start the frontend

```bash
make dev-frontend
```

Open `http://localhost:3000` in your browser.

### 5. Start the mock agent (for testing)

```bash
go run ./cmd/mockagent
```

The mock agent listens on `http://localhost:9090` and simulates various agent behaviors for testing.

### 6. Register an agent and create a task

1. Go to **Agents** → **Register Agent**
2. Name: `mock-agent`, Endpoint: `http://localhost:9090`, Adapter Type: `Native`
3. Click **Test Connection** to verify, then **Register**
4. Go to **Dashboard** → **New Task**
5. Enter a title and description, and watch the DAG execute in real-time

## Agent Protocol

TaskHub supports two ways to connect agents:

### Native Protocol (simplest)

Your agent implements three endpoints:

```
POST /tasks              → Accept task, return { "job_id": "..." }
GET  /tasks/{id}/status  → Return { "status": "running|completed|failed|needs_input", ... }
POST /tasks/{id}/input   → Receive user input (optional)
```

### HTTP Poll Adapter (zero agent changes)

For existing APIs, configure a JSON mapping in the agent registration:

```json
{
  "submit": {
    "method": "POST",
    "path": "/v1/analyze",
    "body_template": { "prompt": "{{instruction}}" },
    "job_id_path": "$.id"
  },
  "poll": {
    "path": "/v1/jobs/{{job_id}}",
    "status_path": "$.state",
    "status_map": { "processing": "running", "done": "completed" },
    "result_path": "$.output"
  }
}
```

No changes to your agent required — TaskHub adapts to your API.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go, chi router, pgx (PostgreSQL) |
| Frontend | Next.js 15, React 19, Tailwind CSS 4, Zustand |
| Visualization | React Flow (DAG pipeline) |
| UI Components | shadcn/ui |
| Database | PostgreSQL |
| Real-time | Server-Sent Events (SSE) |
| Orchestration | Claude CLI (MVP), swappable LLM provider |

## Project Structure

```
taskhub/
├── cmd/
│   ├── server/         # API server
│   └── mockagent/      # Mock agent for testing
├── internal/
│   ├── adapter/        # Agent adapter layer (HTTP poll, native)
│   ├── audit/          # Audit logging + cost tracking
│   ├── auth/           # Authentication middleware + session store
│   ├── config/         # Environment configuration
│   ├── crypto/         # AES-256-GCM credential encryption
│   ├── ctxutil/        # Request context helpers
│   ├── db/             # Database connection + migrations
│   ├── events/         # Event store + SSE broker
│   ├── executor/       # DAG execution engine
│   ├── handlers/       # HTTP handlers
│   ├── httputil/       # HTTP response helpers
│   ├── models/         # Domain structs
│   ├── orchestrator/   # LLM-based task decomposition
│   ├── rbac/           # Role-based access control
│   └── seed/           # Local mode data seeding
├── web/                # Next.js frontend
│   ├── app/            # Pages (dashboard, task detail, agents)
│   ├── components/     # React components
│   └── lib/            # API client, SSE, Zustand stores, types
├── Makefile
├── Dockerfile
└── CLAUDE.md           # AI agent development guidelines
```

## Development

```bash
# Run all quality checks (format + lint + typecheck + build)
make check

# Run tests
make test

# Lint
make lint

# Format
make fmt
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TASKHUB_MODE` | `local` | `local` (no auth) or `cloud` (full auth) |
| `DATABASE_URL` | `postgres://localhost:5432/taskhub?sslmode=disable` | PostgreSQL connection |
| `PORT` | `8080` | Backend server port |
| `FRONTEND_URL` | `http://localhost:3000` | Frontend URL (for CORS) |
| `ANTHROPIC_API_KEY` | — | API key for LLM orchestration |

## Roadmap

- [x] Agent Registry with HTTP poll + native adapters
- [x] DAG-based task execution with parallel subtask support
- [x] Real-time SSE streaming with event persistence
- [x] Group Chat with @mention interaction
- [x] Human-in-the-loop (agent pauses for user input)
- [x] Audit logging + cost tracking
- [x] RBAC with organization-based multi-tenancy
- [x] Mock agent for end-to-end testing
- [ ] A2A protocol adapter
- [ ] WebSocket/streaming agent support
- [ ] Anthropic SDK integration (replace CLI)
- [ ] Google OAuth authentication
- [ ] Capability-based agent routing
- [ ] Global memory / vector store
- [ ] Agent marketplace

## Documentation

- [Design Spec](docs/superpowers/specs/2026-03-22-taskhub-v2-design.md) — Full architecture and data model specification
- [CLAUDE.md](CLAUDE.md) — Development guidelines for AI-assisted coding
- [SECURITY.md](SECURITY.md) — Security policy and trust model

## Contributing

Contributions are welcome! Please read the [contributing guidelines](.github/pull_request_template.md) before submitting a PR.

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/my-feature`)
3. Make your changes and ensure `make check` passes
4. Submit a pull request

## License

TaskHub is licensed under the [Apache License 2.0](LICENSE).
