# Phase 1: Foundation - Context

**Gathered:** 2026-04-04 (assumptions mode)
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix all interaction bugs, SSE race condition, and ship GitHub-ready repo artifacts. A developer can clone the repo, run `docker compose up`, and see a fully working A2A orchestration demo with no broken interactions and no frozen DAG nodes. Includes subtask timeline/trace view and adaptive replanning visibility.

</domain>

<decisions>
## Implementation Decisions

### SSE Subscribe/Replay Race Fix
- **D-01:** Reorder `stream.go` to subscribe to broker BEFORE replaying historical events, then dedup by event ID using the existing `ListByTaskAfter` cursor mechanism
- **D-02:** Use `Last-Event-ID` header (already parsed at stream.go:39) for reconnection dedup; broker buffer of 64 events provides adequate window

### Docker Compose One-Click Startup
- **D-03:** Create new top-level `docker-compose.yml` bundling platform server + PostgreSQL + demo agents
- **D-04:** Fix Next.js standalone output: add `output: 'standalone'` to `web/next.config.ts`
- **D-05:** Add agent seeding to `internal/seed/devseed.go` so demo agents exist on first startup without external setup

### LLM Dependency Strategy
- **D-06:** Replace `claude` CLI exec in orchestrator with Anthropic Go SDK (`github.com/anthropics/anthropic-sdk-go`) — enables Docker deployment with just `ANTHROPIC_API_KEY` env var, no CLI binary needed
- **D-07:** `ANTHROPIC_API_KEY` is required for task execution; if not set, show clear error message at task creation (not a silent failure)

### Replanning Visibility
- **D-08:** Add `task.replanned` case to TraceTimeline.tsx `describeEvent` switch — show failed subtask name, reason, and new subtask count
- **D-09:** Add `task.replanned` handler to Zustand store's `handleEvent` to update DAG view with new subtasks from replanning

### Subtask Timeline/Trace View
- **D-10:** `TraceTimeline.tsx` already exists (untracked) — review and integrate into task detail page
- **D-11:** Show chronological events with agent name, duration, and truncated output per subtask

### README and Demo Assets
- **D-12:** Write comprehensive README with hero screenshot/GIF, badge row (build, license, Go version), architecture diagram, and quickstart section
- **D-13:** Demo screenshot captured after all bugs fixed and Docker Compose working

### Claude's Discretion
- Exact TraceTimeline component styling and layout
- README architecture diagram format (mermaid vs ASCII vs image)
- Docker Compose volume and network configuration details
- Bug fix prioritization order within FOUND-01

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### SSE and Event System
- `internal/events/broker.go` — Event broker with Subscribe/Publish, 64-event channel buffer
- `internal/events/store.go` — Event store with ListByTask and ListByTaskAfter (cursor-based dedup)
- `internal/handlers/stream.go` — SSE stream handler with race condition (subscribe after replay)

### Executor and Replanning
- `internal/executor/executor.go` — DAG executor, publishes `task.replanned` at line ~986
- `internal/executor/recovery.go` — Replan logic with LLM-guided subtask replacement

### Orchestrator (LLM dependency)
- `internal/orchestrator/orchestrator.go` — Task decomposition, currently uses `claude` CLI via exec.CommandContext (lines 134-147)

### Docker
- `Dockerfile` — Multi-stage build, copies from `.next/standalone` (requires standalone output config)
- `docker-compose.agents.yml` — Existing agents-only compose (reference for agent container pattern)
- `web/next.config.ts` — Empty config, needs `output: 'standalone'`

### Frontend Components
- `web/components/task/TraceTimeline.tsx` — Untracked, existing trace timeline component
- `web/lib/store.ts` — Zustand store, handleEvent method (lines 151-280), no task.replanned case
- `web/lib/types.ts` — SSEEventType union, event type definitions

### Seeding
- `internal/seed/devseed.go` — Dev seed, currently only creates local user (no agent seeding)

### Research
- `.planning/research/PITFALLS.md` — Pitfall 1 (SSE race), Pitfall 9 (Claude CLI dependency)
- `.planning/codebase/CONCERNS.md` — Known issues and tech debt

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `TraceTimeline.tsx`: Already exists in working tree (untracked), needs review and integration
- `ListByTaskAfter` in event store: Cursor-based query ready for dedup after subscribe-first reorder
- `Last-Event-ID` parsing in stream handler: Already implemented, just needs correct usage order
- `task.replanned` event: Already published by executor with structured data (replan_count, failed_subtask, new_subtask_count)

### Established Patterns
- SSE events are JSON with `type` and `data` fields
- Event store uses `(created_at, id)` composite cursor for ordering
- Zustand store pattern: `handleEvent` switch on event type, update state immutably
- Go handlers follow thin-handler pattern, delegating to service packages

### Integration Points
- Stream handler → Broker.Subscribe() → Event Store replay (reorder needed)
- Orchestrator → Replace exec.CommandContext with Anthropic SDK client
- TraceTimeline → Zustand store events → SSE stream
- Docker Compose → Dockerfile → next.config.ts standalone output
- Dev seed → Agent registration on startup

</code_context>

<specifics>
## Specific Ideas

- Golutra reference: agent status pulsing dots pattern for showing working state (future phase, but foundation should not block it)
- The SSE fix is the most critical foundation piece — all future phases (agent status, multi-task view) depend on reliable event delivery

</specifics>

<deferred>
## Deferred Ideas

- Mock LLM mode (return pre-built plans without API key) — deferred to v2 as DX-01
- HTTP/2 for multi-task SSE connections — Phase 4 prerequisite, not Phase 1
- Agent status indicators — Phase 2

None — analysis stayed within phase scope

</deferred>

---

*Phase: 01-foundation*
*Context gathered: 2026-04-04*
