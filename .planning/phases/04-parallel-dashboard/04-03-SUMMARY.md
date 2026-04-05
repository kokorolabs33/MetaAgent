---
phase: 04-parallel-dashboard
plan: 03
subsystem: ui
tags: [sse, zustand, react, next.js, suspense, dashboard, multiplexing, typescript]

# Dependency graph
requires:
  - phase: 04-parallel-dashboard/01
    provides: GET /api/tasks/stream?ids=a,b,c multiplexed SSE endpoint, Task.completed_subtasks and Task.total_subtasks fields
  - phase: 04-parallel-dashboard/02
    provides: SubtaskProgressBar, DashboardTaskCard, TaskFilterBar, DashboardEmptyState presentational components
  - phase: 02-agent-status
    provides: AgentStatusDot component, useAgentStore.connectStatusSSE/getAgentStatus
provides:
  - connectMultiTaskSSE SSE helper in web/lib/sse.ts (mirrors connectAgentStatusSSE pattern)
  - useDashboardStore Zustand slice with SSE lifecycle, progress map, and event dispatcher
  - TaskDashboard orchestration container composing all dashboard components
  - Suspense-wrapped home page at / replacing the old EmptyState
  - Task.agent_ids field end-to-end (Go model + SQL ARRAY_AGG + TypeScript interface)
affects: [future dashboard enhancements, task analytics, dashboard performance optimization]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Stable idsKey memo for SSE effect deps: tasks.map(t => t.id).join(',') prevents reconnect churn on reference changes"
    - "Dual SSE lifecycle in one container: multiplexed task stream + global agent status stream managed via separate store actions"
    - "Progress map delta dispatch: subtask.created increments total, subtask.completed/failed increments completed (capped at total)"
    - "Suspense boundary above useSearchParams for Next.js 16 static analysis compliance"

key-files:
  created:
    - web/components/dashboard/TaskDashboard.tsx
  modified:
    - web/lib/sse.ts
    - web/lib/store.ts
    - web/app/page.tsx
    - internal/models/task.go
    - internal/handlers/tasks.go
    - web/lib/types.ts
    - web/lib/conversationStore.ts

key-decisions:
  - "Individual Zustand selectors (one per field) to avoid object-reference re-renders in TaskDashboard"
  - "useMemo idsKey with join(',') as useEffect dep instead of array reference for SSE connection stability"
  - "per_page: 12 hardcoded in loadDashboard matching UI-SPEC 4 rows x 3 cols grid"
  - "subtask.failed counts toward progress completed denominator (finalized state) per research recommendation 4"
  - "ARRAY_REMOVE wrapping ARRAY_AGG to filter empty agent_id strings from the aggregated array"

patterns-established:
  - "Dashboard store pattern: load via API for snapshot, connect SSE for deltas, progress map merges both sources"
  - "Suspense-wrapped page pattern for Next.js 16 useSearchParams compliance"
  - "Stable string dep pattern for array-based useEffect dependencies"

requirements-completed: [INTR-02, INTR-03, INTR-04]

# Metrics
duration: 7min
completed: 2026-04-05
---

# Phase 4 Plan 3: Dashboard Integration Wiring Summary

**Multiplexed SSE wiring + Zustand dashboard store + TaskDashboard container composing filter bar, card grid, pagination, and empty states with dual SSE stream lifecycle and Suspense boundary at /**

## Performance

- **Duration:** ~7 min
- **Started:** 2026-04-05T23:13:13Z
- **Completed:** 2026-04-05T23:19:49Z
- **Tasks:** 3 completed (Task 4 is human-verify checkpoint)
- **Files created:** 1
- **Files modified:** 7

## Accomplishments

- `connectMultiTaskSSE` appended to `web/lib/sse.ts` mirroring `connectAgentStatusSSE` pattern -- connects to `/api/tasks/stream?ids=a,b,c` with URL-encoded comma-separated IDs, returns disconnect function, no-op for empty array
- `useDashboardStore` Zustand slice appended to `web/lib/store.ts` with: `loadDashboard` (API fetch with per_page: 12), `connectDashboardSSE`/`disconnectDashboardSSE` (SSE lifecycle), `handleDashboardEvent` (dispatches task status changes + subtask progress deltas)
- `TaskDashboard` orchestration container created composing TaskFilterBar + DashboardTaskCard grid + Pagination + DashboardEmptyState with loading/error states, dual SSE streams (task + agent), and stable idsKey memo
- `web/app/page.tsx` rewritten from EmptyState wrapper to Suspense-wrapped TaskDashboard, passing Next.js 16 build-time static analysis
- `agent_ids` field added end-to-end: Go `Task.AgentIDs []string`, SQL `ARRAY_AGG(DISTINCT s.agent_id) FILTER (WHERE s.agent_id IS NOT NULL)`, TypeScript `agent_ids: string[]`, enabling D-02 agent status dots on dashboard cards

## Task Commits

1. **Task 1: Add connectMultiTaskSSE helper + useDashboardStore slice** - `e8a6d76` (feat)
2. **Task 2: Create TaskDashboard container + rewrite page.tsx with Suspense** - `fdda7e4` (feat)
3. **Task 3: Full-stack quality gate** - no commit (verification-only)

## Files Created/Modified

### Created
- `web/components/dashboard/TaskDashboard.tsx` -- Orchestration container: reads URL params, loads dashboard store, manages dual SSE lifecycle, renders filter bar + responsive card grid + pagination + loading/error/empty states

### Modified
- `web/lib/sse.ts` -- Appended `connectMultiTaskSSE` function (existing functions untouched)
- `web/lib/store.ts` -- Appended `useDashboardStore` slice (existing useAgentStore and useTaskStore untouched); updated import to include `connectMultiTaskSSE`
- `web/app/page.tsx` -- Replaced EmptyState wrapper with Suspense-wrapped TaskDashboard
- `internal/models/task.go` -- Added `AgentIDs []string` field with `json:"agent_ids"` tag
- `internal/handlers/tasks.go` -- Added `ARRAY_AGG(DISTINCT s.agent_id)` to List query; scan `AgentIDs` in `scanTaskWithCounts`; nil-guard to ensure empty array not null
- `web/lib/types.ts` -- Added `agent_ids: string[]` to Task interface
- `web/lib/conversationStore.ts` -- Added `agent_ids: []` to optimistic task.created handler

## SSE Helper Signature

```typescript
export function connectMultiTaskSSE(
  taskIds: string[],
  onEvent: SSEEventHandler,
  onError?: (error: Event) => void,
): () => void
```

URL pattern: `${BASE}/api/tasks/stream?ids=${encodeURIComponent(taskIds.join(","))}`

## useDashboardStore State Shape

| Field | Type | Source |
|-------|------|--------|
| tasks | Task[] | loadDashboard API call |
| totalTasks | number | API pagination |
| currentPage | number | API pagination |
| totalPages | number | API pagination |
| taskProgress | Record<string, TaskProgress> | Initialized from API, updated by SSE deltas |
| isLoading | boolean | Loading state |
| error | string or null | User-facing error message (no stack traces) |
| multiTaskSSEDisconnect | (() => void) or null | SSE cleanup function |

## Event Dispatch Rules

| Event Type | Delta |
|------------|-------|
| task.planning/running/completed/failed/cancelled/approval_required | Patch task.status in list |
| subtask.created | Increment taskProgress[taskId].total by 1 |
| subtask.completed | Increment taskProgress[taskId].completed by 1 (capped at total) |
| subtask.failed | Increment taskProgress[taskId].completed by 1 (finalized state, capped at total) |

## Decisions Made

- **Individual field selectors for Zustand** -- Each `useDashboardStore((s) => s.field)` call selects a single primitive/reference to avoid triggering re-renders from unrelated state changes. No computed-object selectors.
- **useMemo idsKey with join(",")** -- The SSE connection effect depends on `idsKey` (a string) instead of the `tasks` array reference. This prevents teardown/reconnect when the tasks array reference changes but contains the same IDs (Pitfall 4 from research).
- **per_page: 12** -- Hardcoded in `loadDashboard` to match UI-SPEC grid layout (4 rows x 3 cols at lg breakpoint).
- **subtask.failed as finalized** -- Both `subtask.completed` and `subtask.failed` increment the progress bar's completed count, since both represent finalized subtask states per research recommendation 4.
- **ARRAY_REMOVE wrapping ARRAY_AGG** -- Filters out empty-string agent_ids from the aggregated array, since some subtasks may have been created before agent assignment.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added agent_ids field end-to-end (Go + SQL + TypeScript)**
- **Found during:** Task 1 (pre-implementation dependency check)
- **Issue:** Plan 03 references `task.agent_ids` as delivered by Plan 01 (line 123: "Task interface in types.ts has agent_ids: string[]"), but Plan 01 only delivered `completed_subtasks` and `total_subtasks`. The `agent_ids` field was missing from the Go Task model, the List SQL query, the TypeScript Task interface, and the conversationStore optimistic handler. Without it, the TaskDashboard's `agentIdsForTask` function would always return `[]`, breaking D-02 (agent status dots on cards).
- **Fix:** Added `AgentIDs []string` to `models.Task` with `json:"agent_ids"` tag; added `ARRAY_REMOVE(ARRAY_AGG(DISTINCT s.agent_id) FILTER (WHERE s.agent_id IS NOT NULL AND s.agent_id != ''), NULL)` to the List query; scanned it in `scanTaskWithCounts` with nil-guard; added `agent_ids: string[]` to TypeScript `Task` interface; added `agent_ids: []` to conversationStore optimistic handler.
- **Files modified:** `internal/models/task.go`, `internal/handlers/tasks.go`, `web/lib/types.ts`, `web/lib/conversationStore.ts`
- **Verification:** `go build ./...` passes; `go test ./...` all pass; `tsc --noEmit` passes; grep confirms `agent_ids` present in all layers.
- **Committed in:** `e8a6d76` (Task 1 commit)

**2. [Rule 3 - Blocking] Symlinked node_modules from main repo into worktree**
- **Found during:** Task 1 (pre-verification environment setup)
- **Issue:** The worktree had no `web/node_modules` directory, so TypeScript and Next.js commands would fail.
- **Fix:** Created symlink from worktree `web/node_modules` to main repo `web/node_modules`.
- **Files modified:** None tracked (environment fix).
- **Committed in:** N/A.

---

**Total deviations:** 2 auto-fixed (both Rule 3 - Blocking)
**Impact on plan:** The agent_ids fix was strictly necessary for D-02 agent status dots. Without it, dashboard cards would never show agent dots. The SQL ARRAY_AGG pattern follows the same LEFT JOIN + GROUP BY established by Plan 01. No scope creep.

## Issues Encountered

- **Pre-existing `make check` failure:** `make fmt-check-frontend` fails because `prettier` is not in `web/package.json` devDependencies. Same issue documented in 04-01-SUMMARY. All individual quality gates pass: `gofmt`, `go vet`, `go build`, `go test`, `tsc --noEmit`, `eslint`, `next build`. Out of scope per deviation scope boundary rule.

## User Setup Required

None -- no external service configuration required.

## Next Phase Readiness

- **Dashboard is fully wired:** Visiting `/` renders the task grid with live SSE updates, filter tabs, search, pagination, and agent status dots.
- **Awaiting human verification:** Task 4 checkpoint requires manual testing of all 10 interaction steps (SSE connection count in DevTools, filter URL sync, card navigation, empty states, accessibility).
- **No open blockers** for future phase work.

## Self-Check: PASSED

- All 8 created/modified source files exist on disk
- Both commits (`e8a6d76`, `fdda7e4`) exist in `git log --oneline --all`
- SUMMARY.md exists at `.planning/phases/04-parallel-dashboard/04-03-SUMMARY.md`

---
*Phase: 04-parallel-dashboard*
*Completed: 2026-04-05*
