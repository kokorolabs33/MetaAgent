<p align="center">
  <h1 align="center">TaskHub</h1>
  <p align="center">
    <strong>Open-source A2A multi-agent orchestration platform.</strong>
    <br />
    <em>Orchestrate AI agents via the A2A protocol — decompose tasks, manage execution, stream results.</em>
  </p>
  <p align="center">
    <a href="#quick-start">Quick Start</a> &bull;
    <a href="#features">Features</a> &bull;
    <a href="#architecture">Architecture</a> &bull;
    <a href="#agent-protocol">Agent Protocol</a> &bull;
    <a href="#contributing">Contributing</a>
  </p>
  <p align="center">
    <img src="https://img.shields.io/badge/license-Apache%202.0-blue.svg" alt="License" />
    <img src="https://img.shields.io/badge/go-1.26+-00ADD8.svg" alt="Go" />
    <img src="https://img.shields.io/badge/next.js-16-black.svg" alt="Next.js" />
    <img src="https://img.shields.io/badge/A2A-protocol-blueviolet.svg" alt="A2A Protocol" />
  </p>
</p>

---

TaskHub is an open-source platform for orchestrating AI agents using the [A2A (Agent-to-Agent) protocol](https://github.com/google/A2A). You describe what needs to be done — TaskHub decomposes it into a DAG of subtasks, routes them to the right agents via A2A, manages execution with retries and adaptive replanning, and streams results back in real-time.

**TaskHub does not run agents.** It orchestrates them. Your agents live wherever they are — any A2A-compatible service or HTTP endpoint can be plugged in via adapters, with zero code changes to your agent.

![TaskHub Dashboard](docs/images/hero-screenshot.png)

## Quick Start

### Prerequisites

- Docker and Docker Compose
- An Anthropic API key ([get one here](https://console.anthropic.com/))

### 1. Clone

```bash
git clone https://github.com/kokorolabs33/taskhub.git
cd taskhub
```

### 2. Start

```bash
ANTHROPIC_API_KEY=sk-your-key docker compose up
```

### 3. Open

Visit **http://localhost:3000** — no login required. Four demo agents are pre-registered.

Create a task and watch the DAG executor decompose it, route to agents, and stream results in real-time.

<details>
<summary>Development Setup (without Docker)</summary>

### Prerequisites

- Go 1.26+
- Node.js 22+ and pnpm
- PostgreSQL 15+

### Install and run

```bash
make install
createdb taskhub

# Terminal 1: Backend
make dev-backend

# Terminal 2: Frontend
make dev-frontend

# Terminal 3: OpenAI agents (requires OPENAI_API_KEY)
make agents
```

Open **http://localhost:3000** — no login required in local mode.

</details>

## Why TaskHub?

AI agents are everywhere, but orchestrating them in production is chaos:

- **Fragmented interfaces** — Every agent has a different API, input/output format, and lifecycle model
- **No control plane** — Which agents are running? Which one failed? Which one costs the most?
- **Manual orchestration** — Coordinating multi-agent workflows requires custom glue code
- **No standard protocol** — The A2A protocol exists, but few reference implementations demonstrate real multi-agent coordination

TaskHub solves this by providing a unified A2A orchestration layer that works with any agent.

## Features

### Core

- **A2A Protocol Server** — JSON-RPC 2.0 compliant A2A server with agent card aggregation and health monitoring
- **Agent Registry** — Register any external agent with a simple HTTP endpoint. No SDK required, no agent modification needed.
- **DAG Execution** — Tasks are automatically decomposed into subtask DAGs. Parallel when possible, sequential when dependent.
- **Adaptive Replanning** — Failed subtasks are automatically replanned via LLM, with the replanning process visible in the UI timeline.
- **Adapter System** — Plug in any agent via A2A protocol, HTTP polling adapters, or the native TaskHub protocol. JSONPath mapping for custom APIs.
- **Real-time Streaming** — SSE-based event stream with persistent event store. Browser auto-reconnect with zero event loss.

### Collaboration

- **Conversation System** — Every task gets a conversation. Agents and users communicate via @mentions with Slack-style `<@id|name>` format.
- **Human-in-the-Loop** — Agents can pause execution and ask for confirmation. Users @reply in chat to continue.
- **DAG Visualization** — React Flow pipeline view showing subtask status, dependencies, and progress in real-time.
- **Subtask Timeline** — Chronological trace view showing agent name, duration, and output per subtask with replanning event visibility.

### Governance

- **Policy Engine** — Define execution policies with approval thresholds. Tasks exceeding limits require human approval.
- **Audit Trail** — Every LLM call and agent invocation is logged with token counts, latency, and cost estimates.
- **Template System** — Save successful orchestration patterns as reusable templates with version tracking.
- **Credential Encryption** — Agent auth tokens encrypted at rest with AES-256-GCM.

## Architecture

```
User -> Frontend (Next.js) -> Backend API (Go/chi)
                                    |
                              +-----+------+
                              | Orchestrator |  <- Anthropic SDK decomposes tasks into DAG
                              +-----+------+
                                    |
                              +-----+------+
                              | DAG Executor |  <- Polls agents, manages lifecycle, replans on failure
                              +-----+------+
                                    |
                    +---------------+---------------+
                    |               |               |
              +-----+-----+  +-----+-----+  +-----+-----+
              |  Agent A   |  |  Agent B   |  |  Agent C   |
              | (A2A/HTTP) |  | (A2A/HTTP) |  | (A2A/HTTP) |
              +-----------+  +-----------+  +-----------+
```

**Design principles:**

| Principle | Description |
|-----------|-------------|
| **Agents are external** | TaskHub orchestrates, it doesn't run agent code. Agents are HTTP services you own. |
| **A2A-native** | First-class A2A protocol support with agent card aggregation and JSON-RPC 2.0 server. |
| **Adapter pattern** | Any HTTP API can be an agent. Configure JSON request/response mapping — zero code changes to your agent. |
| **Event-sourced** | Every state change is persisted. Full replay, audit trail, and real-time SSE streaming. |

## Agent Protocol

### Option 1: A2A Protocol (recommended)

TaskHub implements the [A2A protocol](https://github.com/google/A2A) for agent communication. Any A2A-compatible agent can be registered and will be orchestrated automatically.

### Option 2: Native Protocol

Implement three endpoints on your agent:

```
POST /tasks              -> Accept a task, return { "job_id": "..." }
GET  /tasks/{id}/status  -> Return current status
POST /tasks/{id}/input   -> Receive user input (optional)
```

Status values: `running` | `completed` | `failed` | `needs_input`

### Option 3: HTTP Poll Adapter (zero agent changes)

Already have an API? Configure a JSON mapping when registering:

```json
{
  "submit": {
    "method": "POST",
    "path": "/v1/analyze",
    "body_template": { "prompt": "{{instruction}}" },
    "job_id_path": "$.id"
  },
  "poll": {
    "method": "GET",
    "path": "/v1/jobs/{{job_id}}",
    "status_path": "$.state",
    "status_map": { "processing": "running", "done": "completed" },
    "result_path": "$.output"
  }
}
```

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.26, chi router, pgx (PostgreSQL), Anthropic SDK |
| Frontend | Next.js 16, React 19, Tailwind CSS 4, Zustand |
| Visualization | React Flow |
| UI Components | shadcn/ui |
| Database | PostgreSQL (event-sourced) |
| Real-time | Server-Sent Events (SSE) |
| Protocol | A2A (JSON-RPC 2.0) |

## Project Structure

```
taskhub/
├── cmd/
│   ├── server/         # API server
│   └── openaiagent/    # OpenAI-powered A2A agents
├── internal/
│   ├── a2a/            # A2A protocol server, aggregator, health checker
│   ├── adapter/        # Agent adapters (HTTP poll, native protocol)
│   ├── audit/          # Audit logging + cost tracking
│   ├── auth/           # Authentication (local mode + OAuth ready)
│   ├── config/         # Environment configuration
│   ├── db/             # PostgreSQL connection + migrations
│   ├── events/         # Event store + SSE broker
│   ├── executor/       # DAG execution engine + adaptive replanning
│   ├── handlers/       # HTTP request handlers
│   ├── models/         # Domain structs
│   ├── orchestrator/   # LLM-based task decomposition (Anthropic SDK)
│   ├── policy/         # Policy evaluation engine
│   └── seed/           # Data seeding (local mode + demo agents)
├── web/                # Next.js frontend
├── docker-compose.yml  # One-click startup
├── Makefile            # Build, lint, test, dev commands
├── Dockerfile          # Multi-stage container build
└── CLAUDE.md           # AI-assisted development guidelines
```

## Development

```bash
make check    # Full quality gate: format + lint + typecheck + build
make test     # Run all tests
make lint     # Run golangci-lint + eslint
make fmt      # Format all code
```

## Roadmap

- [x] A2A protocol server with agent card aggregation
- [x] Agent Registry with HTTP poll + native adapters
- [x] DAG-based task execution with parallel subtasks
- [x] Anthropic SDK integration for task decomposition
- [x] Adaptive replanning with UI visibility
- [x] Real-time SSE streaming with event persistence
- [x] Conversation system with @mention interaction
- [x] Policy engine with approval workflows
- [x] Template system with version tracking
- [x] Docker Compose one-click startup
- [ ] Agent status indicators (online/working/idle/offline)
- [ ] Task templates with experience accumulation
- [ ] Multi-task parallel dashboard
- [ ] Sub-agent chat intervention

## Contributing

Contributions are welcome! Please see the [PR template](.github/pull_request_template.md) for guidelines.

```bash
# Fork, clone, then:
git checkout -b feat/my-feature
# Make changes, ensure quality gate passes:
make check
# Submit PR
```

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
