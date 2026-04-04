# Architecture Patterns: A2A Meta-Agent Platform Milestone Features

**Domain:** Multi-agent orchestration platform with real-time observability
**Researched:** 2026-04-04
**Scope:** Agent status visualization, enhanced chat interaction, multi-task parallel view, template/experience systems — integrated into existing Go + Next.js + PostgreSQL + A2A stack

---

## Current Architecture Baseline

The existing system is already well-structured. This section maps what exists before describing what the milestone adds.

```
Frontend (Next.js App Router)
  └── Zustand stores  ──── SSE Manager ─────────┐
  └── API Client (api.ts)                        │
                                                 ↓
Backend (Go / chi router)                  In-Memory Broker
  └── HTTP Handlers                        (events/broker.go)
  └── DAGExecutor                               ↑
  └── Orchestrator                         Event Store
  └── A2A Client/Server                    (events/store.go → PostgreSQL)
  └── HealthChecker (background goroutine)
  └── Policy Engine
  └── Audit Logger
```

**Key existing capabilities used by this milestone:**
- `events/broker.go`: In-memory pub/sub keyed by `task_id` and `"conv:{id}"`. Buffer size 64. Drop-on-full.
- `a2a/health.go`: HealthChecker background goroutine. Polls every 60s. Updates `agents.is_online`, `agents.last_health_check`, `agents.skill_hash`.
- `handlers/agent_health.go`: REST endpoints for per-agent and overview health metrics (success rate, avg duration, subtask counts).
- `handlers/templates.go`: Full CRUD + `CreateFromTask` (extract DAG plan from completed task) + `ListExecutions` + `Rollback`.
- `models/agent.go`: Agent has `is_online bool`, `last_health_check *time.Time`, `skill_hash string` — status data already exists.
- `models/template.go`: `WorkflowTemplate`, `TemplateVersion`, `TemplateExecution` — versioning and execution tracking already exists.
- `conversationStore.ts`: Single Zustand store per conversation, single SSE connection, handles task + message events.

**What is missing for this milestone:**
- No derived `activity_status` (working/idle) beyond `is_online` — the DB only knows if the health check passed, not whether an agent is currently running a subtask.
- No global agent status SSE channel — status updates are only in task-scoped streams.
- Chat does not route user messages directly to sub-agents mid-execution.
- Dashboard shows tasks in a list; no parallel multi-task monitoring with live DAG per task.
- Templates exist but have no similarity matching for automatic suggestion at task creation time.

---

## Component Architecture for This Milestone

### Component 1: Agent Activity Status Layer

**What it does:** Derives a four-state presence signal — `online`, `working`, `idle`, `offline` — and broadcasts it to connected frontends.

**Why the current model is insufficient:**
- `is_online` (boolean) reflects only health-check reachability. An agent can be online but have 0 running subtasks (idle) or be in the middle of 3 concurrent subtasks (working).
- Activity must be derived from `subtasks` table state, not polled from the agent.

**Component boundary:**

```
HealthChecker (existing, 60s interval)
  → writes is_online to agents table
  → publishes agent.health_changed event to broker (NEW)

AgentActivityTracker (NEW — lightweight background goroutine in executor)
  → subscribes to subtask start/complete/fail events already published by DAGExecutor
  → derives working/idle from live running subtask count
  → publishes agent.status_changed event to global broker topic ("agents")

Broker (existing, extend to support global "agents" topic)
  → fanout to all SSE subscribers of /api/agents/stream (NEW endpoint)
```

**Data flow:**
```
DAGExecutor.executeSubtask() publishes "agent.working" event
  → Broker publishes to task topic (existing) + "agents" global topic (NEW)
  → SSE /api/agents/stream delivers to any connected frontend
  → Frontend AgentStatusDot component updates color/state

HealthChecker.checkOne() result
  → UPDATE agents SET is_online = $1 (existing)
  → Broker publishes "agent.health_changed" to "agents" topic (NEW)
```

**Derived status logic (backend):**

| is_online | running_subtasks | Displayed status |
|-----------|-----------------|-----------------|
| false | any | offline |
| true | 0 | idle |
| true | > 0 | working |

This derivation happens at query time (join agents with COUNT of running subtasks) and via SSE events — no separate status column needed.

**Frontend components needed:**
- `AgentStatusDot.tsx` — colored indicator (green=idle, blue=working, red=offline). Receives status from Zustand `agentStatusStore`.
- `agentStatusStore.ts` — Zustand store subscribing to `/api/agents/stream` SSE. Maps `agent_id → status`. Shared across all views.

**Communicates with:** DAGExecutor (event source), Broker (event transport), SSE stream handler, agentStatusStore (frontend consumer).

---

### Component 2: Enhanced Chat Interaction (Sub-Agent Direct Messaging)

**What it does:** Allows users to send messages directly to a running sub-agent during task execution, with the agent's response appearing inline in the conversation chat.

**Why the current architecture already supports most of this:**
- `conversations.go` already checks for `@mention` patterns and routes to agents.
- The A2A protocol supports `input_required` state — an agent can pause and wait for human input.
- The broker has dual routing: task events and conversation events already flow to the same SSE stream.

**What needs to extend:**

```
User types "@ResearchAgent clarify the scope"
  → POST /api/conversations/{id}/messages (existing endpoint)
  → ConversationHandler.SendMessage() (existing)
  → Mention parser extracts agent ID (existing)
  → Route to agent via A2A client with conversation context (mostly existing, needs improvement)
  → Agent response published as "message" event with sender_type="agent" (existing pattern)
  → SSE delivers to conversation stream (existing)
  → Frontend shows inline agent reply in chat thread
```

**The gap is in the A2A routing path:**
Current `@mention` routing in `conversations.go` dispatches to the agent but does not tie the reply back to the specific subtask context. The handler needs to:
1. Look up the agent's active subtask for this task (if any) to provide contextId.
2. Publish the result as a message with the correct `task_id` so the DAGPanel can correlate.

**Component boundary:**

```
ConversationHandler (existing)
  extends SendMessage():
    - if @mention found AND task is running:
        resolve agent_id from mention
        look up active subtask (agent_id + running status) for current task
        call A2AClient with subtask contextId (for memory continuity)
        await response (or fire-and-forget + SSE)
        publish message event with sender_type="agent", sender_name=agent.name

InterventionRouter (NEW — thin service in handlers/conversations.go)
  - resolveAgentFromMention(mentionText, conversationID) → agentID
  - findActiveSubtaskContext(agentID, taskID) → contextID, subtaskID
  - dispatchToAgent(agentID, contextID, userMessage) → response message
```

**State machine for agent interaction:**

```
Task running:
  user sends @mention → InterventionRouter → A2A dispatch → agent reply → message event

Task completed:
  user sends @mention → message stored → no agent dispatch (task is done)
  (future: allow post-hoc questions by recreating context)

Task input_required:
  agent published input_required A2A state → executor pauses subtask
  frontend shows "Waiting for input" state on subtask node
  user sends message (no mention needed — it's the pending input)
  → POST /api/tasks/{id}/subtasks/{subtask_id}/input (NEW endpoint)
  → executor resumes subtask with user input
```

**Communicates with:** ConversationHandler, A2A Client, DAGExecutor (for input_required state), Broker, SSE stream.

---

### Component 3: Multi-Task Parallel View

**What it does:** A dashboard panel showing multiple tasks executing concurrently, each with a live mini-DAG and status indicator.

**Architecture approach:**
This is primarily a frontend architecture problem. The backend already supports multiple SSE connections (broker handles arbitrary task subscriptions). The challenge is managing multiple concurrent SSE connections in the browser without excessive re-renders.

**Component boundary:**

```
Frontend:
  parallelTaskStore.ts (NEW Zustand store)
    - pinnedTaskIDs: string[]           ← tasks the user is "watching"
    - taskStates: Record<string, TaskViewState>
    - sseConnections: Record<string, () => void>  ← disconnect fns

    pinTask(id):
      connect SSE for task_id (reuse existing connectSSE from sse.ts)
      subscribe to task events, update taskStates[id]

    unpinTask(id):
      disconnect SSE for task_id
      remove from taskStates

  TaskMonitorGrid.tsx (NEW component)
    - renders N TaskMonitorCard components
    - layout: responsive grid (2 cols on wide screen)

  TaskMonitorCard.tsx (NEW component)
    - title, status badge, agent list with AgentStatusDot
    - miniDAG: React Flow subgraph (simplified, no edge labels)
    - latest message preview
    - "Open full view" link → /c/{conv_id}

  MiniDAG.tsx (NEW or extend SubtaskNode.tsx)
    - compact React Flow with smaller nodes
    - node color = subtask status
    - no interactivity needed (click opens full view)
```

**Data flow:**
```
User visits /dashboard (or a new /monitor route)
  → parallelTaskStore.loadActiveTasks() → GET /api/tasks?status=running
  → for each task: parallelTaskStore.pinTask(id)
    → connectSSE(taskId, onEvent)
    → onEvent updates taskStates[taskId]
  → TaskMonitorGrid renders from taskStates
  → each card live-updates as SSE events arrive

User manually pins/unpins tasks
  → parallelTaskStore.pinTask / unpinTask
```

**Performance constraint:** Each pinned task = one SSE connection (EventSource). Modern browsers allow 6 simultaneous HTTP/1.1 connections per domain; with HTTP/2 (which Go's net/http supports) this is multiplexed. Cap at 6 pinned tasks to stay safe, or use a single multiplexed SSE stream (see Pitfalls).

**Backend change required (minor):** Add `GET /api/tasks?status=running` filter if not already present. No new endpoints needed for SSE — existing `/api/channels/{id}/stream` or task-level stream suffices.

**Communicates with:** Existing SSE infrastructure, parallelTaskStore (new), existing Task + SubTask API endpoints.

---

### Component 4: Template and Experience Accumulation System

**What it does:** Allows successful orchestration patterns to be saved, discovered, and reused. The "experience accumulation" aspect is: each time a template is used, execution data (actual steps taken, replans, HITL interventions, duration) is recorded, enabling the template to improve over time.

**Existing foundation:** The template system already has `workflow_templates`, `template_versions`, `template_executions` tables and CRUD handlers. `CreateFromTask` already extracts the DAG structure from a completed task. `TemplateExecution` records HITL interventions and replan count.

**What is missing for "experience accumulation":**

1. **Template suggestion at task creation time** — currently users must manually select a template. The system should suggest matching templates based on task title/description similarity.

2. **Execution recording completeness** — `template_executions` exists but `duration_seconds` is not populated by the executor (only manual API calls create them). The executor needs to write a `template_execution` record on task completion when `task.template_id` is set.

3. **Evolution endpoint usability** — `handlers/evolution.go` exists but needs to surface concrete suggestions in the UI.

**Component boundary:**

```
Backend:

TemplateMatcher (NEW — package internal/templates/ or method on TemplateHandler)
  - SuggestForTask(title, description string) → []TemplateSuggestion
  - Strategy: keyword overlap scoring (no LLM needed for MVP)
    - tokenize title+description
    - compare against template name + step instruction_templates
    - return top-3 with overlap score
  - Endpoint: GET /api/templates/suggest?q=task+description

Executor (extend):
  - on task.completed with task.template_id set:
    - INSERT INTO template_executions (template_id, template_version, task_id, actual_steps, hitl_interventions, replan_count, outcome, duration_seconds)
    - actual_steps = final subtask list (serialized)
    - duration_seconds = completed_at - created_at
  - this closes the feedback loop: every template use generates a data point

EvolutionHandler (existing, extend):
  - GET /api/templates/{id}/evolution-suggestions
  - reads template_executions, identifies common divergences from template steps
  - returns structured suggestions (e.g., "step 3 is often replaced with X")

Frontend:

TemplateSuggestionBar.tsx (NEW — shown in NewTaskDialog)
  - when user types in description field (debounced 500ms)
  - calls GET /api/templates/suggest?q=...
  - shows top-3 matching templates as chips
  - clicking a chip pre-selects it as template_id

TemplateEvolutionPanel.tsx (NEW — in templates/[id]/page.tsx)
  - shows execution history chart (success rate over time)
  - shows evolution suggestions from /api/templates/{id}/evolution-suggestions
  - "Apply suggestion" button → creates new template version

```

**Data flow for suggestion:**
```
User types task description in NewTaskDialog
  → debounced GET /api/templates/suggest?q=...
  → TemplateMatcher scores all active templates
  → returns [{id, name, score, step_count}]
  → TemplateSuggestionBar renders chips

User selects template → templateId set in form state
  → POST /api/tasks {title, description, template_id}
  → Executor loads template skeleton for orchestrator context (existing)
  → On completion → INSERT template_execution (NEW)
```

**Data flow for experience accumulation:**
```
Task completes (with template_id)
  → executor.aggregateResults()
  → INSERT template_executions record
  → TemplateExecution.outcome = "success" | "failed_replanned" | "failed"
  → TemplateExecution.actual_steps = final subtask list

Template detail page loads
  → GET /api/templates/{id}/executions (existing)
  → GET /api/templates/{id}/evolution-suggestions (existing)
  → frontend renders execution trend + suggestions
```

**Communicates with:** TemplateHandler, Executor (for recording), NewTaskDialog (frontend), templates page (frontend).

---

## Full System Data Flow (Milestone Features Together)

```
┌─────────────────────────────────────────────────────────────────┐
│  FRONTEND                                                        │
│                                                                  │
│  Dashboard / Monitor View                                        │
│    parallelTaskStore ←── SSE /channels/{id}/stream (per task)   │
│    TaskMonitorGrid renders TaskMonitorCard × N                   │
│    agentStatusStore ←── SSE /api/agents/stream (global)         │
│                                                                  │
│  Conversation View (/c/{id})                                     │
│    ConversationView                                              │
│      ChatInput → POST /api/conversations/{id}/messages           │
│      DAGPanel ← real-time from conversationStore SSE            │
│      MessageFeed ← conversationStore.messages                    │
│      AgentStatusDot per agent ← agentStatusStore                │
│                                                                  │
│  Task Creation                                                   │
│    NewTaskDialog → GET /api/templates/suggest → chips           │
└─────────────────────────────────────────────────────────────────┘
           │ HTTP/SSE                │ HTTP/SSE
           ▼                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  BACKEND                                                         │
│                                                                  │
│  cmd/server/main.go                                              │
│    ↓ registers routes                                            │
│    GET  /api/agents/stream    ← AgentStatusHandler.Stream (NEW) │
│    GET  /api/templates/suggest ← TemplateHandler.Suggest (NEW)  │
│    POST /api/conversations/{id}/messages ← (extended)           │
│                                                                  │
│  DAGExecutor (executor.go)                                       │
│    executeSubtask() → Broker.Publish("agent.working")           │
│    subtask completes → Broker.Publish("subtask.completed")      │
│    task completes + template_id → INSERT template_execution     │
│                                                                  │
│  Events Broker (broker.go — extend)                             │
│    topic "agents" → all agent status changes (NEW topic)        │
│    topic "task:{id}" → existing task events                     │
│    topic "conv:{id}" → existing conversation events             │
│                                                                  │
│  HealthChecker (a2a/health.go — extend)                         │
│    on is_online change → Broker.Publish("agents" topic) (NEW)   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
           │
           ▼
┌─────────────────────────────────────────────────────────────────┐
│  POSTGRESQL                                                      │
│  agents: is_online, last_health_check (existing)                │
│  subtasks: status, agent_id, started_at, completed_at          │
│  template_executions: populated on task completion (extend)     │
└─────────────────────────────────────────────────────────────────┘
```

---

## Build Order: Component Dependencies

The four components have different dependency profiles. Build order matters.

### Phase 1: Agent Status Visualization

**Build this first.** Reasoning:
- No new data model — `agents.is_online` and subtask status already exist.
- Requires only: extend Broker with a global "agents" topic, add SSE endpoint, add `AgentStatusDot` component.
- Unblocks: all other components that show agent status (multi-task view, conversation view) can use `agentStatusStore` immediately.
- Complexity: LOW. Three files touched (broker.go, new handler, new TS store + component).

**Build order within component:**
1. Extend `broker.go`: add `Subscribe("agents")` support — already works, just need to use the "agents" key.
2. Extend `HealthChecker.checkAll()` and `DAGExecutor.executeSubtask()` / subtask completion: publish to "agents" topic.
3. Add `GET /api/agents/stream` SSE endpoint.
4. Add `agentStatusStore.ts` in frontend.
5. Add `AgentStatusDot.tsx`.
6. Wire into existing `AgentCard.tsx` and `ConversationView`.

### Phase 2: Template Suggestion and Experience Recording

**Build second.** Reasoning:
- Standalone backend feature (no SSE, no frontend real-time).
- Closes an existing gap (executor does not record `template_executions`).
- Template suggestion powers better task creation UX.
- No dependencies on Phase 1.

**Build order within component:**
1. Extend `executor.go`: INSERT `template_execution` on task completion with `template_id`.
2. Add `GET /api/templates/suggest` endpoint in `handlers/templates.go`.
3. Add `TemplateSuggestionBar.tsx` in `NewTaskDialog`.
4. Add `TemplateEvolutionPanel.tsx` in template detail page.

### Phase 3: Multi-Task Parallel View

**Build third.** Reasoning:
- Depends on Phase 1 (AgentStatusDot used in TaskMonitorCard).
- Purely frontend-heavy — no new backend endpoints needed (existing SSE + task API).
- The `MiniDAG` component can reuse `SubtaskNode.tsx` styling.

**Build order within component:**
1. Add `parallelTaskStore.ts`.
2. Add `MiniDAG.tsx` (simplified React Flow, no interactivity).
3. Add `TaskMonitorCard.tsx` (status badge, mini-dag, agent dots, latest message).
4. Add `TaskMonitorGrid.tsx` (responsive layout, pin/unpin controls).
5. Wire into dashboard page or add `/monitor` route.

### Phase 4: Enhanced Chat Interaction (Sub-Agent Intervention)

**Build last.** Reasoning:
- Most complex: requires understanding A2A contextId threading, active subtask lookup, and careful state management.
- Depends on Phase 1 for visual feedback (show agent as "working" when responding).
- Touches `ConversationHandler` which already has many responsibilities.
- The `input_required` path (formal A2A pause state) needs executor cooperation.

**Build order within component:**
1. Add `InterventionRouter` logic in `handlers/conversations.go` (resolve mention → agent, find active subtask context).
2. Extend A2A dispatch: when routing @mention during active task, use active subtask's contextId.
3. Add `POST /api/tasks/{id}/subtasks/{subtask_id}/input` endpoint for `input_required` state.
4. Frontend: update `ConversationView` to show input prompt when `input_required` subtask exists.
5. Frontend: ensure `MessageFeed` renders agent replies with correct sender attribution.

---

## Patterns to Follow

### Pattern: Derive Status, Don't Store It

**What:** Agent activity status (working/idle) is derived from live `subtasks` query, not persisted as a column.

**Why:** A stored `activity_status` column creates a dual-write problem — executor would need to update both subtask status and agent status atomically. Derivation at read-time or via events avoids this.

**How:** At SSE subscription time, send initial state from:
```sql
SELECT a.id,
  a.is_online,
  COUNT(s.id) FILTER (WHERE s.status = 'running') > 0 AS is_working
FROM agents a
LEFT JOIN subtasks s ON s.agent_id = a.id
GROUP BY a.id, a.is_online
```
Then update via SSE events as subtask status changes.

### Pattern: Single Global Agent Status Channel

**What:** One SSE endpoint (`/api/agents/stream`) delivers all agent status changes to all connected frontends.

**Why:** Individual agent status polling (one SSE per agent) does not scale and causes N connections per client. A single fanout channel is the standard presence platform pattern.

**How:** Broker key `"agents"` used by broker.Publish calls from HealthChecker and DAGExecutor. All agent status SSE connections subscribe to the same broker topic.

### Pattern: Template Suggestion via Keyword Overlap (No LLM for MVP)

**What:** Template matching uses TF-IDF-style overlap scoring between task description tokens and template step instruction templates.

**Why:** LLM-based semantic matching is more accurate but adds latency and cost to the task creation path. Keyword overlap is fast (< 5ms), free, and adequate for a small template library (< 100 templates typical for a developer tool).

**Upgrade path:** Replace scoring function with embedding similarity (pgvector) when template library grows beyond ~50 templates and users report poor suggestions.

### Pattern: Fan-Out Before Persisting for Real-Time UX

**What:** The broker publishes events to SSE subscribers AFTER event store persistence (existing pattern). Do not change this order.

**Why:** Persistence-first ensures no event is lost if a subscriber drops. The existing pattern is correct; new components must not short-circuit to broker-only.

### Pattern: Per-Conversation SSE, Not Per-Task SSE, for Chat

**What:** Chat interaction (including sub-agent replies) flows through the conversation SSE stream, not task-level streams.

**Why:** The conversation is the user's mental model for ongoing interaction. Task events are implementation detail. The `conversationStore` already handles both message and task events on a single SSE connection — maintain this unification.

---

## Anti-Patterns to Avoid

### Anti-Pattern: Polling Agent Status from Frontend

**What:** Front-end timer calling `GET /api/agents` every N seconds to refresh status.

**Why bad:** Creates thundering herd at 60s intervals (every connected client fires simultaneously). Produces stale data between polls. Already exists in some places in the codebase — do not extend this approach.

**Instead:** SSE push from `/api/agents/stream`. Frontend receives updates only when state changes.

### Anti-Pattern: Storing Derived State in Database

**What:** Adding `activity_status` column to agents table, written by executor on every subtask state change.

**Why bad:** Creates a write-heavy column requiring locks. Executor already fires SSE events on subtask state change — the derived state can be reconstructed from those events. Adds synchronization complexity for no benefit.

**Instead:** Derive from subtask counts. Broadcast via events.

### Anti-Pattern: Multiple SSE Connections per Component

**What:** Each component independently opens an SSE connection for its data.

**Why bad:** Browser HTTP/1.1 allows 6 connections per origin. Even with HTTP/2 multiplexing, unnecessary connections waste server-side goroutines and broker subscriber slots. The existing design wisely uses one SSE connection per conversation. Do not create per-agent SSE connections in AgentCard.

**Instead:** Single global `/api/agents/stream`. All agent status consumers read from `agentStatusStore`.

### Anti-Pattern: LLM for Template Matching at Creation Time

**What:** Calling Anthropic API to semantically match task description against templates during task creation.

**Why bad:** Adds 1-3s latency to the task creation critical path. Costs money per keystroke if called while typing. Templates are a developer tool feature — users can tolerate keyword matching quality.

**Instead:** Keyword overlap scoring in Go. Fast, free, good enough for the use case.

### Anti-Pattern: Blocking Sub-Agent Dispatch in HTTP Handler

**What:** `POST /api/conversations/{id}/messages` handler waits synchronously for the A2A agent response before returning.

**Why bad:** A2A agent invocations can take 5-30 seconds. The HTTP connection would hang. The existing pattern (fire-and-forget to goroutine, respond via SSE) must be preserved.

**Instead:** Handler returns 202 Accepted immediately. Agent processing happens in background. Response arrives via SSE as a message event.

---

## Scalability Considerations

| Concern | Current (1-5 users) | At 50 concurrent users |
|---------|---------------------|----------------------|
| Agent status SSE | One broker topic, all subscribers | Same — broker is O(subscribers), not O(agents) |
| Parallel task monitoring | 6 SSE connections per user | Cap pinned tasks at 6; HTTP/2 multiplexes |
| Template suggestion | Full table scan (< 100 templates) | Add GIN index on template name + pg_trgm if slow |
| Sub-agent dispatch | Sequential A2A call in goroutine | Goroutine per dispatch, existing per-agent concurrency limits apply |
| HealthChecker interval | 60s | Sufficient; increase to 120s if load grows |

---

## Sources

- A2A Protocol Specification: task states (WORKING, INPUT_REQUIRED, COMPLETED, FAILED, CANCELED), SSE streaming patterns — [a2a-protocol.org/latest/specification](https://a2a-protocol.org/latest/specification/) — MEDIUM confidence (spec is authoritative but task state enumeration was partial in retrieved content)
- Azure Architecture Center: AI Agent Orchestration Patterns (sequential, concurrent, handoff) — [learn.microsoft.com/azure/architecture/ai-ml/guide/ai-agent-design-patterns](https://learn.microsoft.com/en-us/azure/architecture/ai-ml/guide/ai-agent-design-patterns) — HIGH confidence (official Microsoft docs, updated 2026-03-07)
- Real-Time Presence Platform patterns (heartbeat, single fanout channel, derived status) — [systemdesign.one/real-time-presence-platform-system-design](https://systemdesign.one/real-time-presence-platform-system-design/) — MEDIUM confidence (community resource, aligns with broker pattern already implemented)
- Go health aggregation patterns — [oneuptime.com/blog/post/2026-02-01-go-service-health-aggregation](https://oneuptime.com/blog/post/2026-02-01-go-service-health-aggregation/view) — MEDIUM confidence
- LangGraph human-in-the-loop interrupt pattern — [blog.langchain.com/making-it-easier-to-build-human-in-the-loop-agents-with-interrupt](https://blog.langchain.com/making-it-easier-to-build-human-in-the-loop-agents-with-interrupt/) — MEDIUM confidence (Python-specific, but interrupt/pause/resume pattern applies to Go implementation)
- Workflow memory and template retrieval — [emergence.ai/blog/learning-from-stored-workflows-retrieval-for-better-orchestration](https://www.emergence.ai/blog/learning-from-stored-workflows-retrieval-for-better-orchestration) — LOW confidence (URL returned 404; finding based on search snippet only)
- Existing codebase analysis: `internal/a2a/health.go`, `internal/events/broker.go`, `internal/handlers/agent_health.go`, `internal/handlers/templates.go`, `internal/models/agent.go`, `internal/models/template.go`, `web/lib/conversationStore.ts` — HIGH confidence (direct code reading)

---

*Architecture research: 2026-04-04*
