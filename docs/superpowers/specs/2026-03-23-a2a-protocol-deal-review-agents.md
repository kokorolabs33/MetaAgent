# A2A Protocol Integration & Deal Review Agents

## Goal

Migrate TaskHub from custom agent protocols (native + http_poll) to the Google A2A (Agent-to-Agent) standard protocol as the sole agent integration method. Build 4 real LLM-powered agents for the Deal Review demo scenario, deployed as Docker containers implementing A2A server.

## Design Principles

1. **Platform adapts to agents, not agents to platform** — minimize what external agents need to implement
2. **Single protocol** — A2A only, no protocol fragmentation
3. **Chat is the UI layer** — A2A messages map bidirectionally to the group chat; agents only know A2A
4. **Context isolation** — agents receive their instruction + upstream artifacts, never the full chat history

## Architecture

```
User → Frontend (Next.js) → Backend API (Go/chi) [A2A Client]
                                    │
                              ┌─────┴──────┐
                              │ Orchestrator │  ← LLM decomposes tasks into DAG
                              └─────┬──────┘
                                    │
                        A2A SendMessage (JSON-RPC over HTTP)
                                    │
                    ┌───────────────┼───────────────┐
                    │               │               │
              ┌─────┴─────┐  ┌─────┴─────┐  ┌─────┴─────┐
              │ A2A Agent  │  │ A2A Agent  │  │ A2A Agent  │
              │  (any)     │  │  (any)     │  │  (any)     │
              └───────────┘  └───────────┘  └───────────┘
```

**Key change:** TaskHub becomes an A2A Client. All agents are A2A Servers. The custom adapter layer (`internal/adapter/`) is removed entirely.

**Dependency:** `github.com/a2aproject/a2a-go/v2` — official A2A Go SDK v2 (server + client).

---

## A2A Protocol Mapping

### Agent States (simplified from current)

A2A uses lowercase-hyphenated state strings:

| Before | After (A2A wire format) |
|--------|------------------------|
| `running` | `working` |
| `completed` | `completed` |
| `failed` | `failed` |
| `waiting_for_input` (removed) | `input-required` (A2A standard) |
| `blocked` (DAG) | unchanged (TaskHub internal, not A2A) |

Additional A2A states TaskHub must handle: `submitted`, `auth-required`, `rejected`, `canceled`.

### TaskHub Concepts → A2A Concepts

| TaskHub | A2A | Notes |
|---------|-----|-------|
| task_id | contextId | All agents in one task share a contextId |
| subtask ↔ agent interaction | taskId | Each agent gets its own taskId (server-generated) |
| subtask.output | Artifact | Structured output from agent |
| upstream results passed to downstream | Message with DataPart | DAG dependency data injected into SendMessage |
| chat message from agent | Message (role: agent) | Displayed in group chat UI |
| user @mention agent | SendMessage with existing taskId | Continues the agent's conversation |

---

## Agent Registration & Discovery

### Current Flow (manual)
User fills in: name, endpoint, adapter_type, adapter_config JSON.

### A2A Flow (automatic)
1. User enters agent base URL (e.g., `https://legal-agent.example.com`)
2. TaskHub fetches `GET {url}/.well-known/agent-card.json`
3. AgentCard returns: name, description, skills, capabilities, auth requirements
4. TaskHub stores the card and displays agent info to user
5. User confirms registration

### AgentCard Example

Note: JSON shown is the A2A wire format. Go struct field names in the SDK may differ (e.g., `URL` vs `url`). Consult `a2a-go` SDK types during implementation.

```json
{
  "name": "Legal Review Agent",
  "description": "Analyzes contracts for legal risks, compliance issues, and liability exposure",
  "url": "https://legal-agent.example.com",
  "version": "1.0.0",
  "capabilities": {
    "streaming": true,
    "pushNotifications": false,
    "stateTransitionHistory": true
  },
  "skills": [{
    "id": "contract-review",
    "name": "Contract Risk Analysis",
    "description": "Reviews contracts and identifies legal risks, compliance issues, and liability exposure"
  }],
  "defaultInputModes": ["text/plain", "application/json"],
  "defaultOutputModes": ["text/plain", "application/json"]
}
```

### Database Changes (agents table)

Remove: `adapter_type`, `adapter_config`, `auth_config`
Add: `agent_card_url TEXT`, `agent_card JSONB` (cached card), `card_fetched_at TIMESTAMPTZ`

Health check = ability to fetch AgentCard. Refresh cached card every 24 hours or on manual refresh. If card changes (new skills, removed capabilities), update `agent_card` column and notify user.

### Authentication to A2A Agents

A2A supports security schemes declared in the AgentCard: API Key, HTTP Bearer, OAuth2, OpenID Connect, Mutual TLS. For Deal Review agents (local Docker), no auth needed. For production external agents, TaskHub stores credentials separately in a `agent_credentials` table (encrypted with AES-256-GCM, same as current `auth_config` approach). The A2A client attaches credentials based on the scheme declared in the agent's card.

---

## Task Execution Flow

### DAG Execution with A2A

1. User creates task: "Review this deal: Acme Corp, $2M contract, 35% discount"
2. Orchestrator decomposes into DAG:
   ```
   Legal Agent ────┐
   Finance Agent ──┼──→ Deal Review Agent
   Technical Agent ┘
   ```
3. Parallel execution of first layer (Legal, Finance, Technical):

   ```json
   // TaskHub → Legal Agent (A2A SendMessage)
   {
     "method": "SendMessage",
     "params": {
       "message": {
         "role": "user",
         "parts": [
           {"text": "Analyze the legal risks for this deal: Acme Corp, $2M enterprise license, 35% discount, 3-year term..."}
         ]
       },
       "contextId": "task-uuid-123"
     }
   }
   ```

   ```json
   // Legal Agent → TaskHub (response)
   {
     "task": {
       "taskId": "legal-task-001",
       "contextId": "task-uuid-123",
       "status": {"state": "completed"},
       "artifacts": [{
         "artifactId": "legal-output-001",
         "parts": [{
           "data": {
             "risk_level": "MEDIUM",
             "issues": [
               {"clause": "Section 5 - Limitation of Liability", "risk": "Cap set below industry standard", "recommendation": "Negotiate higher cap"}
             ],
             "summary": "Moderate risk. Key concern is liability cap in Section 5."
           }
         }]
       }]
     }
   }
   ```

4. When all 3 complete, send to Deal Review Agent with upstream artifacts:

   ```json
   {
     "method": "SendMessage",
     "params": {
       "message": {
         "role": "user",
         "parts": [
           {"text": "Synthesize a Go/No-Go recommendation for this deal based on the following analyses."},
           {"data": {
             "legal": {"risk_level": "MEDIUM", "issues": [...], "summary": "..."},
             "finance": {"margin_pct": 12, "recommendation": "Conditional approval", "...": "..."},
             "technical": {"feasibility": "HIGH", "timeline": "3 months", "...": "..."}
           }}
         ]
       },
       "contextId": "task-uuid-123"
     }
   }
   ```

5. Deal Review Agent returns final Go/No-Go → task completed.

### Executor Changes

| Before | After |
|--------|-------|
| `Submit()` → get JobHandle | `SendMessage()` → get Task or Message response |
| `pollSubtask()` with exponential backoff | `SendStreamingMessage()` for real-time progress; `SendMessage()` for simple agents |
| `poll_job_id` / `poll_endpoint` fields | `a2a_task_id` field on subtask |
| Signal mechanism for `waiting_for_input` | A2A `input-required` state + follow-up SendMessage |

**Execution mode:** Use `SendStreamingMessage` by default. This returns an SSE stream of events (`TaskStatusUpdateEvent`, `TaskArtifactUpdateEvent`, `Message`) that TaskHub relays to the frontend in real-time. For agents that don't support streaming (per AgentCard `capabilities.streaming`), fall back to `SendMessage` with a generous HTTP timeout (5 minutes).

**SendMessage response handling:** A2A `SendMessage` returns either `*a2a.Task` (for stateful interactions) or `*a2a.Message` (for simple stateless responses). The A2A client wrapper must handle both:
- `*a2a.Task` → store `taskId` as `a2a_task_id`, extract artifacts as subtask output
- `*a2a.Message` → treat as immediate completion, extract text/data parts as output

**Preserved:** DAG topological sort, parallel execution, concurrency limits, dependency checking, budget enforcement, crash recovery (via `a2a_task_id` + `GetTask`).

**Crash recovery:** On restart, for each subtask with `a2a_task_id` and status `running`, call `GetTask(taskId)` to check current state. If `completed` → store result. If `failed` → mark failed. If `working` → re-subscribe to streaming.

### Subtask Database Changes

Remove: `poll_job_id`, `poll_endpoint`
Add: `a2a_task_id TEXT` (agent-generated task ID for follow-up interactions)

### contextId Strategy

TaskHub generates `contextId` equal to the TaskHub `task.ID` (UUID). All A2A interactions for agents within one task share this contextId. This groups the conversation for each agent within the task context.

---

## Group Chat ↔ A2A Message Mapping

### Agent → Chat (agent output appears in group chat)

When TaskHub receives an A2A response:
- **Streaming** (`SendStreamingMessage`): SSE events map to chat messages in real-time
  - `statusUpdate` → system message: "[Agent] Working on analysis..."
  - `message` → agent message in chat
  - `artifactUpdate` → agent result message in chat
- **Synchronous** (`SendMessage`): artifact content posted as agent message in chat after completion

### User → Agent (user @mentions agent in chat)

```
User types: "@Legal What about the indemnification clause?"
      ↓
TaskHub parses @mention → finds Legal Agent's a2a_task_id for this task
      ↓
A2A SendMessage({
  taskId: "legal-task-001",      // continue existing conversation
  contextId: "task-uuid-123",
  message: { role: "user", parts: [{ text: "What about the indemnification clause?" }] }
})
      ↓
Legal Agent responds → posted to chat as agent message
```

This works for agents in any state:
- `working` — message queued, agent processes when ready
- `input-required` — message provides the requested input, agent resumes
- `completed` — starts a new interaction within the same context

### Context Rules (preventing context explosion)

| Scenario | What agent receives |
|----------|-------------------|
| First execution | Instruction + upstream artifacts (structured data via DAG) |
| User @mention follow-up | Own conversation history (within this a2a_task_id) + new message |
| DAG dependency | Upstream agent's artifact (structured data, not chat history) |

**Agents never receive the full group chat history.** The chat is a UI layer for humans. Agent-to-agent data flows through DAG artifacts.

---

## Human-in-the-Loop

Uses A2A's native `INPUT_REQUIRED` state. No custom protocol needed.

### Flow

1. Finance Agent analyzes deal, detects discount > 20%
2. Agent returns A2A task with `status.state = "INPUT_REQUIRED"` and `status.message = "35% discount exceeds 20% policy limit. Please approve or reject."`
3. TaskHub posts in group chat: **[Finance Agent]** 35% discount exceeds 20% policy limit. Please approve or reject.
4. User replies: `@Finance Approved, CEO signed off`
5. TaskHub sends A2A `SendMessage` with same `taskId` → Finance Agent resumes
6. Finance Agent completes with updated assessment

### Implementation in LLM Agent

```
LLM output JSON includes: { "needs_input": { "message": "...", "options": [...] } }
      ↓
Agent code detects needs_input field
      ↓
Returns A2A task with INPUT_REQUIRED state
      ↓
On follow-up SendMessage: append user input to conversation, call LLM again
      ↓
Returns COMPLETED with final artifact
```

The `needs_input` detection is internal to our agent implementation. External A2A agents use whatever internal logic they want to trigger `INPUT_REQUIRED`.

---

## Deal Review Agents

### Architecture

Single Go binary (`cmd/llmagent/`) with role-based configuration:

```
cmd/llmagent/
├── main.go       # A2A server (a2a-go SDK) + role routing
├── roles.go      # Role definitions: AgentCard + system prompt per role
├── executor.go   # AgentExecutor implementation: message → Claude API → response
└── claude.go     # Claude Messages API HTTP client (direct HTTP, no CLI dependency)
```

### Roles

| Role | Skill | Output Structure | Special Behavior |
|------|-------|-----------------|-----------------|
| `legal` | Contract Risk Analysis | `{ risk_level, issues[], summary }` | — |
| `finance` | Financial Deal Assessment | `{ margin_pct, pricing_assessment, discount_analysis, recommendation }` | `INPUT_REQUIRED` when discount > 20% |
| `technical` | Technical Feasibility Review | `{ feasibility, risks[], resource_estimate, timeline, recommendation }` | — |
| `deal-review` | Deal Go/No-Go Synthesis | `{ decision, confidence, rationale, conditions[], risk_summary }` | Receives upstream artifacts from all 3 agents |

### Execution Flow (all roles)

```
Receive A2A SendMessage
  → Extract instruction + any upstream data from message parts
  → Build Claude API prompt: system prompt (role-specific) + user message + conversation history
  → Call Claude Messages API (POST https://api.anthropic.com/v1/messages)
  → Parse LLM response as JSON
  → [Finance only] Check for needs_input field → return INPUT_REQUIRED if present
  → Return A2A task COMPLETED with artifact containing structured output
```

### System Prompts

**Legal Agent:**
```
You are an enterprise legal counsel specializing in contract risk assessment.

Analyze the provided deal for legal risks. Focus on:
- Limitation of liability and indemnification clauses
- Intellectual property ownership and licensing terms
- Termination conditions and exit clauses
- Regulatory compliance requirements
- Data protection and privacy obligations

Return your analysis as JSON:
{
  "risk_level": "LOW" | "MEDIUM" | "HIGH" | "CRITICAL",
  "issues": [{ "clause": "...", "risk": "...", "recommendation": "..." }],
  "summary": "2-3 sentence overall assessment"
}
```

**Finance Agent:**
```
You are a corporate finance analyst specializing in deal economics.

Evaluate the financial viability of the provided deal. Analyze:
- Revenue and margin projections
- Pricing relative to standard rates
- Discount justification and policy compliance
- Payment terms and cash flow impact
- Revenue recognition implications

If the discount exceeds 20%, you MUST include a "needs_input" field requesting approval.

Return your analysis as JSON:
{
  "margin_pct": <number>,
  "pricing_assessment": "...",
  "discount_analysis": { "discount_pct": <number>, "within_policy": <boolean>, "justification": "..." },
  "payment_terms": "...",
  "recommendation": "...",
  "needs_input": { "message": "...", "options": ["Approve", "Reject"] }  // only if discount > 20%
}
```

**Technical Agent:**
```
You are a technical architect evaluating implementation feasibility.

Assess the technical viability of delivering the described deal. Evaluate:
- Technology stack compatibility and integration complexity
- Security and compliance requirements
- Scalability and performance considerations
- Team capability and resource requirements
- Implementation timeline and milestones

Return your analysis as JSON:
{
  "feasibility": "HIGH" | "MEDIUM" | "LOW",
  "risks": [{ "area": "...", "severity": "...", "mitigation": "..." }],
  "resource_estimate": { "engineers": <number>, "months": <number> },
  "timeline": "...",
  "recommendation": "..."
}
```

**Deal Review Agent:**
```
You are the chair of the deal review committee. You synthesize analyses from Legal, Finance, and Technical teams into a final Go/No-Go recommendation.

You will receive structured data from three analysts. Weigh all factors:
- Legal risk severity and mitigation feasibility
- Financial viability and margin acceptability
- Technical implementation risk and timeline

Return your decision as JSON:
{
  "decision": "GO" | "NO-GO" | "CONDITIONAL",
  "confidence": <number 0-1>,
  "rationale": "3-5 sentence explanation",
  "conditions": ["condition 1", "..."],  // for CONDITIONAL decisions
  "risk_summary": "1-2 sentence overall risk posture"
}
```

### Deployment

```yaml
# docker-compose.agents.yml
services:
  legal-agent:
    build:
      context: .
      dockerfile: cmd/llmagent/Dockerfile
    command: ["--role=legal", "--port=9091"]
    ports: ["9091:9091"]
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}

  finance-agent:
    build:
      context: .
      dockerfile: cmd/llmagent/Dockerfile
    command: ["--role=finance", "--port=9092"]
    ports: ["9092:9092"]
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}

  technical-agent:
    build:
      context: .
      dockerfile: cmd/llmagent/Dockerfile
    command: ["--role=technical", "--port=9093"]
    ports: ["9093:9093"]
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}

  deal-review-agent:
    build:
      context: .
      dockerfile: cmd/llmagent/Dockerfile
    command: ["--role=deal-review", "--port=9094"]
    ports: ["9094:9094"]
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
```

### LLM Configuration

- Model: `claude-sonnet-4-6` (fast, cost-effective for structured analysis)
- Max tokens: 4096
- Temperature: 0.3 (structured output, low creativity needed)
- Direct HTTP to Anthropic Messages API (no CLI dependency)

---

## Error Handling

### Agent Registration Errors
- AgentCard URL unreachable → show "Cannot reach agent" error, do not register
- URL returns non-JSON / invalid AgentCard → show "Invalid AgentCard" with details
- URL returns valid JSON but not A2A AgentCard → show "Not an A2A agent"

### Execution Errors
- `SendMessage` network failure → retry up to 3 times with exponential backoff (1s, 3s, 9s), then mark subtask `failed`
- Agent returns A2A JSON-RPC error → map error code to subtask error message, mark `failed`
- Agent returns `auth-required` → mark subtask `failed` with "Authentication required — check agent credentials"
- Agent returns `rejected` → mark subtask `failed` with "Agent rejected the task"
- HTTP timeout (5 min) → mark subtask `failed` with "Agent timed out"
- LLM JSON parsing failure (inside our agents) → retry LLM call once, then return plain text result as fallback

### Subtask Timeout
Default 30 minutes per subtask (configurable per agent). If streaming connection is alive but no state transition for 30 minutes, cancel and mark `failed`.

---

## A2A Client API Surface

`internal/a2a/client.go` wraps the `a2a-go` SDK:

```go
// Client wraps the A2A SDK client for TaskHub use.
type Client struct {
    httpClient *http.Client
}

// Discover fetches and validates an AgentCard from the given base URL.
func (c *Client) Discover(ctx context.Context, baseURL string) (*a2a.AgentCard, error)

// Send sends a message to an agent and blocks until completion or interrupted state.
// Returns the A2A Task (with artifacts) or an error.
func (c *Client) Send(ctx context.Context, card *a2a.AgentCard, contextID string, taskID string, parts []a2a.Part) (*a2a.Task, error)

// SendStream sends a message and returns a channel of streaming events.
// Events include status updates, messages, and artifact chunks.
func (c *Client) SendStream(ctx context.Context, card *a2a.AgentCard, contextID string, taskID string, parts []a2a.Part) (<-chan StreamEvent, error)

// GetTask retrieves the current state of a task (for crash recovery).
func (c *Client) GetTask(ctx context.Context, card *a2a.AgentCard, taskID string) (*a2a.Task, error)

// Cancel requests cancellation of a task.
func (c *Client) Cancel(ctx context.Context, card *a2a.AgentCard, taskID string) error
```

`internal/a2a/discovery.go`:

```go
// Resolver handles AgentCard fetching, validation, and caching.
type Resolver struct {
    pool *pgxpool.Pool
}

// Resolve fetches the AgentCard from the agent's well-known URL.
func (r *Resolver) Resolve(ctx context.Context, baseURL string) (*a2a.AgentCard, error)

// Refresh re-fetches and updates the cached card for a registered agent.
func (r *Resolver) Refresh(ctx context.Context, agentID string) (*a2a.AgentCard, error)

// CachedCard returns the stored card for a registered agent.
func (r *Resolver) CachedCard(ctx context.Context, agentID string) (*a2a.AgentCard, error)
```

One `Client` instance is shared across the application. One SDK client is created per agent interaction (from the cached AgentCard).

---

## Platform Changes Summary

### Files to Delete

| Path | Reason |
|------|--------|
| `internal/adapter/adapter.go` | Replaced by A2A client |
| `internal/adapter/http_poll.go` | No more HTTP polling adapter |
| `internal/adapter/native.go` | No more native protocol adapter |
| `internal/adapter/template.go` | No more template variable substitution |
| `internal/adapter/jsonpath.go` | No more JSONPath extraction |

### Files to Create

| Path | Purpose |
|------|---------|
| `internal/a2a/client.go` | A2A client wrapping a2a-go SDK |
| `internal/a2a/discovery.go` | AgentCard fetching and caching |
| `cmd/llmagent/main.go` | A2A server entry point with role routing |
| `cmd/llmagent/roles.go` | Role definitions (AgentCard + system prompts) |
| `cmd/llmagent/executor.go` | AgentExecutor implementation |
| `cmd/llmagent/claude.go` | Claude Messages API HTTP client |
| `cmd/llmagent/Dockerfile` | Container image for LLM agents |
| `docker-compose.agents.yml` | Docker Compose for 4 Deal Review agents |
| `internal/db/migrations/NNN_a2a_migration.sql` | Schema changes for A2A |

### Files to Modify

| Path | Changes |
|------|---------|
| `internal/executor/executor.go` | Replace Submit+Poll with A2A SendMessage; remove poll logic |
| `internal/executor/recovery.go` | Crash recovery via A2A GetTask instead of poll resume |
| `internal/models/agent.go` | Remove adapter fields, add agent_card_url/agent_card |
| `internal/models/task.go` | SubTask: remove poll fields, add a2a_task_id |
| `internal/handlers/agents.go` | Registration = URL input → fetch AgentCard |
| `internal/handlers/messages.go` | @mention routing via A2A SendMessage |
| `cmd/mockagent/main.go` | Rewrite as A2A server using a2a-go SDK |
| `web/lib/types.ts` | Sync TypeScript interfaces with Go model changes |
| `web/components/agent/` | Simplify registration form (URL + Discover button) |
| `go.mod` | Add a2a-go/v2 dependency |

### Files Unchanged

| Path | Reason |
|------|--------|
| `internal/orchestrator/` | Task decomposition logic unchanged |
| `internal/events/` | Event store + SSE broker unchanged |
| `internal/auth/` | Authentication unaffected |
| `internal/rbac/` | RBAC unaffected |
| `internal/audit/` | Audit logging unaffected |
| `web/components/task/DAGView.tsx` | DAG visualization unchanged |
| `web/components/chat/GroupChat.tsx` | Chat UI unchanged (message format same) |

---

## Testing Strategy

### Mock Agent (A2A)
Rewrite `cmd/mockagent/` as an A2A server with keyword-based behaviors (preserving all existing test scenarios):
- Default: immediate `completed` with echo response
- `echo:msg`: immediate `completed` with msg as result
- `slow:N`: streams `working` status updates, completes after N polls/seconds
- `fail:msg`: returns `failed` with error message
- `fail-then-succeed:N`: fails N times, then succeeds (tests retry logic)
- `input:msg`: returns `input-required`, completes after receiving input
- `progress:`: streams progress updates (0.25 → 0.5 → 0.75 → 1.0)

### Docker Networking
When running all services in Docker Compose, agent URLs use Docker service names (e.g., `http://legal-agent:9091`), not `localhost`. The AgentCard's `url` field must match the network the TaskHub backend uses. For local development (backend outside Docker, agents in Docker), use `localhost` with exposed ports.

### End-to-End Test Scenario
1. Start TaskHub + 4 Deal Review agents (docker compose)
2. Register all 4 agents via AgentCard discovery
3. Create task: "Review deal: Acme Corp, $2M, 35% discount, 3-year term"
4. Verify: DAG created with 4 subtasks (3 parallel + 1 sequential)
5. Verify: Legal, Finance, Technical agents receive instructions
6. Verify: Finance agent returns INPUT_REQUIRED (35% > 20%)
7. User approves via @mention in chat
8. Verify: Finance completes, Deal Review receives all 3 outputs
9. Verify: Deal Review returns Go/No-Go recommendation
10. Verify: All results visible in chat + DAG view
