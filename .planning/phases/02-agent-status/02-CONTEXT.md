# Phase 2: Agent Status - Context

**Gathered:** 2026-04-04
**Status:** Ready for planning

<domain>
## Phase Boundary

Real-time online/working/idle/offline agent status indicators across all UI surfaces, backed by a global SSE channel. This phase builds the shared infrastructure that Phases 4 and 5 depend on.

</domain>

<decisions>
## Implementation Decisions

### SSE Channel Architecture
- **D-01:** New global SSE endpoint at `/api/agents/stream` using a new `"agents"` topic key on the existing Broker
- **D-02:** Frontend connects via a new `connectAgentStatusSSE()` helper in `web/lib/sse.ts`
- **D-03:** On SSE connect, replay current agent statuses (subscribe-before-replay pattern from Phase 1 D-01/D-02) so new subscribers get an immediate snapshot

### Working/Idle Status Derivation
- **D-04:** In-memory per-agent running subtask counter in the executor — increment on `subtask.running`, decrement on `subtask.completed`/`subtask.failed`
- **D-05:** Publish `agent.status_changed` event to the `"agents"` Broker topic whenever the derived status transitions (online→working, working→idle, etc.)
- **D-06:** No new database column for activity_status — derive at runtime only

### Offline/Staleness Detection
- **D-07:** Keep existing HealthChecker 2-minute interval unchanged
- **D-08:** Extend HealthChecker to publish `agent.status_changed` event via Broker when `is_online` transitions (inject Broker as new dependency)
- **D-09:** Frontend renders agents with `last_health_check` older than threshold as "unknown" rather than falsely "online"

### Frontend Component Design
- **D-10:** Create `AgentStatusDot` component — color-coded dot (green=online/idle, amber+pulse=working, gray=offline, gray-dashed=unknown)
- **D-11:** Extend existing `useAgentStore` in `web/lib/store.ts` with status tracking + SSE connection (no separate store)
- **D-12:** Place `AgentStatusDot` on: AgentCard (list), agent detail pages, health overview, DAG nodes (inline with agent name)
- **D-13:** Use `animate-pulse` for working state (consistent with existing `SubtaskNode.tsx` pattern)

### Claude's Discretion
- Exact animation timing and CSS for pulse effect
- Whether to show "working on: [subtask name]" tooltip on hover
- SSE event payload structure for agent.status_changed
- How to handle the initial snapshot replay (REST call vs SSE replay)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### SSE and Broker Infrastructure
- `internal/events/broker.go` — Existing Broker with Subscribe/Publish, task_id keying, conv: prefix convention
- `internal/handlers/stream.go` — Phase 1 subscribe-before-replay pattern (reference for new endpoint)
- `web/lib/sse.ts` — Existing SSE helpers (connectSSE, connectConversationSSE)

### Agent Health and Status
- `internal/a2a/health.go` — HealthChecker, 2-min interval, updates is_online in DB
- `internal/models/agent.go` — Agent struct with is_online, last_health_check fields
- `cmd/server/main.go` — HealthChecker wiring (where to inject Broker dependency)

### Executor Events
- `internal/executor/executor.go` — Publishes subtask.running (line ~464), subtask.completed (~606), subtask.failed (~650) with agent_id

### Frontend Agent Components
- `web/components/agent/AgentCard.tsx` — Agent card with existing status badge
- `web/app/agents/[id]/page.tsx` — Agent detail page with is_online check
- `web/app/agents/health/page.tsx` — Health overview table
- `web/lib/store.ts` — useAgentStore (lines 16-48), extend with status tracking
- `web/components/task/SubtaskNode.tsx` — animate-pulse pattern (line 55) for reference

### Research
- `.planning/research/ARCHITECTURE.md` — Proposes global "agents" Broker topic and derive-don't-store approach

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `Broker.Subscribe(key)` / `Broker.Publish(key, event)` — extend with "agents" key for global topic
- `connectSSE()` pattern in sse.ts — template for new `connectAgentStatusSSE()`
- `animate-pulse` in SubtaskNode.tsx — CSS animation pattern for "working" state
- HealthChecker already probes agents and updates is_online — just needs to publish events

### Established Patterns
- SSE subscribe-before-replay (Phase 1 D-01/D-02) — apply same pattern to agent status endpoint
- Zustand store with handleEvent switch on event.type — extend for agent.status_changed
- Event store persistence — agent status events should also persist for replay on reconnect

### Integration Points
- Executor → Broker: publish agent status change when running subtask count transitions
- HealthChecker → Broker: publish on is_online state change
- New SSE handler → Broker.Subscribe("agents")
- Frontend agentStore → new SSE connection on app mount (not per-page)

</code_context>

<specifics>
## Specific Ideas

- Golutra reference: 4-status system (online/working/dnd/offline) with pulsing dots — TaskHub uses online/working/idle/offline (no dnd, add idle distinction)
- Status dot should be small enough to fit inline with agent name in DAG nodes without disrupting layout

</specifics>

<deferred>
## Deferred Ideas

- "Working on: [subtask name]" tooltip — nice to have, not required for Phase 2
- Agent status history/timeline — future observability enhancement
- Agent-initiated status changes (agent reports its own status) — would require A2A protocol extension

</deferred>

---

*Phase: 02-agent-status*
*Context gathered: 2026-04-04*
