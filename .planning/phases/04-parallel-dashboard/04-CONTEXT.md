# Phase 4: Parallel Dashboard - Context

**Gathered:** 2026-04-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Multi-task parallel monitoring dashboard with live status updates, task filtering, and one-click navigation to task detail. Users can see all running tasks simultaneously with real-time progress from a single view.

</domain>

<decisions>
## Implementation Decisions

### Dashboard Layout
- **D-01:** Compact status cards in a responsive grid layout — no mini-DAG in cards
- **D-02:** Each card shows: task title, status badge, progress bar (N/M subtasks), active agent status dots (from Phase 2 AgentStatusDot), time ago
- **D-03:** Clicking a card navigates to `/tasks/{id}` for full DAG view and conversation
- **D-04:** Dashboard replaces the current EmptyState chat page at `/` or gets a dedicated route

### SSE Connection Strategy
- **D-05:** New multiplexed SSE endpoint at `/api/tasks/stream?ids=a,b,c` — one EventSource receives events for all visible tasks
- **D-06:** Backend subscribes to multiple Broker topics (one per task_id) and fans out through a single HTTP response writer
- **D-07:** Events include `task_id` field so frontend can dispatch to the correct card's state
- **D-08:** Combined with existing `/api/agents/stream` for agent status dots — total 2 SSE connections for the dashboard regardless of task count

### Task Filtering and Search
- **D-09:** Tab bar at top with status tabs: All / Running / Completed / Failed
- **D-10:** Search box (right-aligned in the tab bar) filters by title substring
- **D-11:** URL query params reflect active filter and search for shareable/bookmarkable state
- **D-12:** Default tab is "All" showing tasks sorted by most recently updated

### Claude's Discretion
- Grid column count and responsive breakpoints
- Progress bar visual style (Tailwind utility classes)
- Empty state for each filter tab (e.g., "No running tasks")
- Pagination strategy (infinite scroll vs page numbers) if task count grows large
- Whether to auto-refresh the task list or rely solely on SSE for updates
- Exact multiplexed SSE endpoint path and query param format

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### SSE and Broker Infrastructure
- `internal/events/broker.go` — Broker with Subscribe/Publish, task_id keying
- `internal/handlers/stream.go` — Existing per-task SSE handler (reference for multiplexed version)
- `web/lib/sse.ts` — Existing connectSSE, connectConversationSSE, connectAgentStatusSSE helpers

### Dashboard Components
- `web/components/dashboard/TaskCard.tsx` — Existing task card with status badges (extend or replace)
- `web/components/dashboard/NewTaskDialog.tsx` — Task creation dialog (keep accessible from dashboard)
- `web/app/page.tsx` — Current home page (EmptyState — will be replaced or augmented)

### Task Store and API
- `web/lib/store.ts` — useTaskStore with loadTasks({status, q, page}) already supports filtering
- `web/lib/api.ts` — api.tasks.list({status, q, page, per_page}) already implemented
- `web/lib/types.ts` — Task, SubTask, TaskWithSubtasks interfaces

### Agent Status (Phase 2)
- `web/components/agent/AgentStatusDot.tsx` — Reusable status dot component (use on dashboard cards)
- `web/lib/store.ts` — useAgentStore.agentStatuses for real-time agent status
- `web/lib/sse.ts` — connectAgentStatusSSE for global agent status stream

### Navigation
- `web/components/ui/nav.tsx` — Nav with Dashboard link (href="/")

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `TaskCard` component — existing card with status badges, time ago, click-to-navigate. Can extend with progress bar and agent dots
- `AgentStatusDot` — Phase 2 component for real-time agent status visualization
- `useTaskStore.loadTasks()` — already accepts `{status, q, page}` params for filtering
- `api.tasks.list()` — backend API already supports status/search/pagination filtering
- `Badge` component from shadcn/ui — consistent status badge styling
- `Card`, `CardHeader`, `CardContent` — shadcn/ui card primitives

### Established Patterns
- SSE subscribe-before-replay — apply same pattern to multiplexed endpoint
- Zustand store with handleEvent switch — extend for multi-task event dispatching
- `connectAgentStatusSSE()` pattern — template for new `connectMultiTaskSSE(ids[])`
- Tab-based navigation exists in agent health page — reusable pattern for status tabs

### Integration Points
- Home page (`/`) — replace EmptyState with dashboard grid (or add dashboard route)
- Broker — new multi-subscribe method for multiplexed SSE handler
- TaskStore — extend with multi-task tracking (map of task_id -> status/progress)
- AgentStore — already provides agent statuses, dashboard cards just read from it

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

- Phase 3 (Templates) skipped for now — can be revisited later
- Mini-DAG visualization in dashboard cards — decided against for v1, but could be added as enhancement
- Task grouping/categorization — not in scope for Phase 4

</deferred>

---

*Phase: 04-parallel-dashboard*
*Context gathered: 2026-04-05*
