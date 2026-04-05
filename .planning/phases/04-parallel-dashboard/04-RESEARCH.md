# Phase 4: Parallel Dashboard - Research

**Researched:** 2026-04-04
**Domain:** Multi-task real-time dashboard, multiplexed SSE fan-out, Next.js App Router URL-synced filtering
**Confidence:** HIGH

## Summary

Phase 4 adds a parallel-task monitoring dashboard at `/` that replaces `EmptyState`. The work is cleanly bounded by CONTEXT.md decisions D-01..D-12 and the UI-SPEC. Backend adds **one new handler** — a multiplexed SSE endpoint `/api/tasks/stream?ids=a,b,c` — that reuses the existing `events.Broker` (`internal/events/broker.go:24`) by calling `Subscribe(taskID)` in a loop and fanning all channels out to a single `http.ResponseWriter`. Frontend adds a new dashboard container, an enhanced task card, a filter/tab bar, and a subtask progress bar; state lives in a new Zustand slice (or extension of `useTaskStore`) that tracks per-task progress and owns the multiplexed SSE lifecycle.

The two biggest research findings that shape the plan:

1. **Browser 6-connection cap is real and is exactly what INTR-03 is protecting against.** Chromium/WebKit limit concurrent `EventSource` connections to **6 per domain over HTTP/1.1** ([CITED: Chromium bug 275955](https://issues.chromium.org/issues/40329530), [CITED: MDN SSE guide](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)). Each SSE connection is permanently "busy" — so 4 tasks with per-task SSE + 1 agent-status stream + any other page load = instant starvation. The multiplexed endpoint is the correct fix; HTTP/2 was the alternative but requires TLS (ruled out per STATE.md blockers line: "plain HTTP Docker Compose deployment cannot use HTTP/2").
2. **Task-list API does not return subtask counts today.** `TaskHandler.List` (`internal/handlers/tasks.go:93`) selects only the tasks table. Progress bar (`{completed}/{total} subtasks` from UI-SPEC copy) requires either (a) a join-aggregation added to the SQL query, or (b) a separate bulk endpoint, or (c) lazy load per card after list. Recommendation: **option (a)** — extend the List query with a LEFT JOIN + GROUP BY, returning `completed_subtasks` and `total_subtasks` on each task row. Zero extra round-trips, matches existing pattern, keeps the frontend simple.

**Primary recommendation:** Build bottom-up per `.agents/skills/add-feature/SKILL.md` — (1) extend `TaskHandler.List` SQL to return subtask counts, (2) add `StreamHandler.MultiStream` handler wired to `GET /api/tasks/stream`, (3) update TS types and API client, (4) add `connectMultiTaskSSE(ids)` to `web/lib/sse.ts`, (5) build `useDashboardStore` slice, (6) build UI components in the order TaskFilterBar → SubtaskProgressBar → DashboardTaskCard → DashboardTaskGrid → TaskDashboard, (7) replace `web/app/page.tsx` content.

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Dashboard Layout**
- **D-01:** Compact status cards in a responsive grid layout — no mini-DAG in cards
- **D-02:** Each card shows: task title, status badge, progress bar (N/M subtasks), active agent status dots (from Phase 2 AgentStatusDot), time ago
- **D-03:** Clicking a card navigates to `/tasks/{id}` for full DAG view and conversation
- **D-04:** Dashboard replaces the current EmptyState chat page at `/` or gets a dedicated route

**SSE Connection Strategy**
- **D-05:** New multiplexed SSE endpoint at `/api/tasks/stream?ids=a,b,c` — one EventSource receives events for all visible tasks
- **D-06:** Backend subscribes to multiple Broker topics (one per task_id) and fans out through a single HTTP response writer
- **D-07:** Events include `task_id` field so frontend can dispatch to the correct card's state
- **D-08:** Combined with existing `/api/agents/stream` for agent status dots — total 2 SSE connections for the dashboard regardless of task count

**Task Filtering and Search**
- **D-09:** Tab bar at top with status tabs: All / Running / Completed / Failed
- **D-10:** Search box (right-aligned in the tab bar) filters by title substring
- **D-11:** URL query params reflect active filter and search for shareable/bookmarkable state
- **D-12:** Default tab is "All" showing tasks sorted by most recently updated

### Claude's Discretion

- Grid column count and responsive breakpoints (UI-SPEC pins this: `grid-cols-1 sm:grid-cols-2 lg:grid-cols-3`)
- Progress bar visual style (Tailwind utility classes) — UI-SPEC pins colors
- Empty state for each filter tab (e.g., "No running tasks") — UI-SPEC pins copy
- Pagination strategy (infinite scroll vs page numbers) if task count grows large — UI-SPEC pins: page-number, 12/page
- Whether to auto-refresh the task list or rely solely on SSE for updates — recommend SSE-only during filter stability; refresh list on filter change
- Exact multiplexed SSE endpoint path and query param format — recommend `GET /api/tasks/stream?ids=a,b,c` (comma-separated) to match D-05

### Deferred Ideas (OUT OF SCOPE)

- Phase 3 (Templates) skipped for now — can be revisited later
- Mini-DAG visualization in dashboard cards — decided against for v1, but could be added as enhancement
- Task grouping/categorization — not in scope for Phase 4
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| INTR-02 | Multi-task parallel view dashboard shows multiple running tasks simultaneously with status badges and active agent indicators | Extended TaskHandler.List returns subtask counts; DashboardTaskCard composes Badge + SubtaskProgressBar + AgentStatusDot. Existing `statusConfig` in `web/components/dashboard/TaskCard.tsx:8` reused exactly. |
| INTR-03 | Multi-task SSE connection strategy handles browser connection limits (HTTP/2 or multiplexed endpoint) | New `GET /api/tasks/stream?ids=a,b,c` handler subscribes to `broker.Subscribe(id)` per id and fans out on a single response writer → 1 task-stream + 1 agent-stream = 2 total SSE connections, well under the 6-connection browser cap ([CITED: MDN](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)). HTTP/2 alternative rejected because Compose deployment runs plain HTTP (STATE.md blocker). |
| INTR-04 | Dashboard supports task filtering by status (pending/running/completed/failed) and search by title | `api.tasks.list()` and `TaskHandler.List` already accept `?status=` and `?q=` + pagination (`internal/handlers/tasks.go:93`, `web/lib/api.ts:82`). New `TaskFilterBar` drives those params and syncs to URL via `useRouter`/`useSearchParams`. |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

**Backend (Go)**
- `gofmt` and `goimports` enforced; local prefix `taskhub`
- Always check errors; never `_ = err`. Exception list: `.golangci.yml:32-33`
- Use `pgx/v5` parameterized queries; never string-concat SQL
- Handlers stay thin — business logic in `internal/*` packages
- JSON tags snake_case to match frontend
- Migrations embedded via `//go:embed` in `internal/db/migrations/`
- No `panic()` in production code; `log.Fatalf` only in `main()`
- New routes registered in `cmd/server/main.go` under the authed group

**Frontend (TypeScript)**
- Strict mode; no `@ts-ignore`, no `any` (ESLint `no-explicit-any: error`)
- Zustand only for global state — no Context/Redux
- shadcn/ui + Tailwind only; components from `web/components/ui/`
- `web/lib/types.ts` must mirror Go JSON tags exactly (adds `completed_subtasks`, `total_subtasks` to `Task` interface)
- PascalCase for components, camelCase for utility files
- `console.log`/`console.debug` flagged as warning — use `console.error` for real errors only

**Hard gates**
- `make check` (fmt + lint + typecheck + build) MUST pass before pushing
- If API or shared types change, both backend and frontend must compile

## Standard Stack

### Core (already installed — no new dependencies required)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| chi v5 | v5.2.5 | HTTP routing for the new SSE endpoint | Existing router; `internal/handlers/stream.go` uses chi URL params — consistent pattern [VERIFIED: `web/package.json` + `go.mod` inspected this session] |
| pgx/v5 | v5.9.1 | DB driver for extended List query JOIN | Existing driver; all handlers use `h.DB.Query(ctx, ...)` [VERIFIED: codebase] |
| Next.js | 16.1.6 | App Router + `useSearchParams`/`useRouter` for URL-synced filters | Already the framework [VERIFIED: `web/package.json:17`] |
| Zustand | ^5.0.11 | New dashboard store slice | Only state manager allowed per CLAUDE.md [VERIFIED: `web/package.json:24`] |
| shadcn/ui | ^4.0.5 | Card, Badge, Button, Input, Pagination primitives | Declared design system in UI-SPEC [VERIFIED: `web/package.json:21`] |
| lucide-react | ^0.577.0 | `Search`, `Loader2`, `AlertCircle` icons | Icon library per UI-SPEC [VERIFIED: `web/package.json:16`] |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| EventSource (browser built-in) | n/a | Multiplexed SSE client | New `connectMultiTaskSSE(ids)` helper in `web/lib/sse.ts`, mirrors `connectAgentStatusSSE` pattern |
| `URLSearchParams` (web std) | n/a | Encode/decode filter state in URL | Existing pattern in `web/lib/api.ts:83-89`; no new library needed |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Multiplexed SSE endpoint | HTTP/2 to bypass 6-conn limit | **Rejected.** HTTP/2 requires TLS for browsers; Docker Compose local deployment is plain HTTP. Explicit STATE.md blocker: "plain HTTP Docker Compose deployment cannot use HTTP/2". |
| Hand-written debounce | `use-debounce` (npm) | **Hand-rolled is fine here.** 5-line `setTimeout`/`clearTimeout` in a `useCallback` is trivial; adding a dep for 5 lines violates the project's "new deps must be justified" principle. [CITED: [Next.js learn: Adding Search and Pagination](https://nextjs.org/learn/dashboard-app/adding-search-and-pagination)] |
| Zustand store extension vs new slice | Extend `useTaskStore` with dashboard fields | Recommend **new `useDashboardStore`** per UI-SPEC to avoid polluting task-detail state. Existing `useTaskStore.tasks` is used for single-task context; dashboard has distinct concerns (filter state, per-task progress map, multiplexed SSE lifecycle). |
| `nuqs` for URL state | Library-managed typed URL state | Nice-to-have but a new dependency. `useSearchParams`/`useRouter` is enough for 3 params (`status`, `q`, `page`). [CITED: [nuqs overview on DEV](https://dev.to/tphilus/stop-fighting-nextjs-search-params-use-nuqs-for-type-safe-url-state-2a0h)] |

**Installation:**
```bash
# No new dependencies required.
```

**Version verification:** No new packages — all versions verified against `web/package.json` and `go.mod` in this session (2026-04-04).

## Architecture Patterns

### Recommended File Layout (per UI-SPEC component inventory)
```
internal/
├── handlers/
│   └── stream.go                        # EXTEND: add MultiStream method
└── handlers/
    └── tasks.go                         # EXTEND: List query returns subtask counts

web/
├── app/
│   └── page.tsx                         # REPLACE: EmptyState → TaskDashboard
├── components/
│   └── dashboard/
│       ├── TaskDashboard.tsx            # NEW: top-level container (orchestrates lifecycle)
│       ├── TaskFilterBar.tsx            # NEW: tabs + search + New Task CTA
│       ├── DashboardTaskCard.tsx        # NEW: card w/ progress + agent dots
│       ├── DashboardTaskGrid.tsx        # NEW: responsive grid wrapper (optional split)
│       ├── SubtaskProgressBar.tsx       # NEW: N/M bar
│       └── DashboardEmptyState.tsx      # NEW: per-filter empty state
└── lib/
    ├── sse.ts                           # EXTEND: add connectMultiTaskSSE(ids[], onEvent)
    ├── store.ts                         # EXTEND: add useDashboardStore slice
    ├── api.ts                           # EXTEND: tasks.list already supports filters; no change expected
    └── types.ts                         # EXTEND: Task { completed_subtasks, total_subtasks }
```

### Pattern 1: Multiplexed SSE Fan-Out (Backend)
**What:** One HTTP response writer receives events from N broker subscriptions.
**When to use:** Whenever the frontend needs live updates for multiple task IDs and you want to stay under the 6-connection-per-domain browser cap.
**Why:** Mirrors the existing `AgentStatusStreamHandler.Stream` subscribe-then-loop pattern, but aggregates N channels via `reflect.Select` or a single fan-in goroutine.

**Example (pseudocode, verified against `internal/handlers/stream.go:24` and `agent_status_stream.go:27`):**
```go
// Source: Existing pattern in internal/handlers/stream.go + agent_status_stream.go

// MultiStream handles GET /api/tasks/stream?ids=a,b,c.
// Subscribes to each task's broker channel and fans events out on a single writer.
func (h *StreamHandler) MultiStream(w http.ResponseWriter, r *http.Request) {
    idsParam := r.URL.Query().Get("ids")
    if idsParam == "" {
        jsonError(w, "ids query param required", http.StatusBadRequest)
        return
    }
    ids := strings.Split(idsParam, ",")
    // Validate + cap (e.g., max 50 ids) to prevent DoS; return 400 on overflow.

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    flusher, ok := w.(http.Flusher)
    if !ok {
        jsonError(w, "streaming not supported", http.StatusInternalServerError)
        return
    }

    // Subscribe BEFORE replay (subscribe-before-replay pattern from Phase 1 D-01).
    type sub struct {
        taskID string
        ch     chan *models.Event
    }
    subs := make([]sub, 0, len(ids))
    for _, id := range ids {
        id = strings.TrimSpace(id)
        if id == "" { continue }
        subs = append(subs, sub{taskID: id, ch: h.Broker.Subscribe(id)})
    }
    defer func() {
        for _, s := range subs {
            h.Broker.Unsubscribe(s.taskID, s.ch)
        }
    }()

    // Optional replay: for each task, load historical events and write them.
    // (Decide whether the dashboard needs replay — for LIVE progress tracking,
    // replay may be skipped since the initial task list already carries counts.)

    // Fan-in loop: merge all channels into select via a single goroutine per channel
    // writing to a shared events chan, or use reflect.Select for arbitrary N.
    merged := make(chan *models.Event, 64)
    var wg sync.WaitGroup
    ctx := r.Context()
    for _, s := range subs {
        wg.Add(1)
        go func(s sub) {
            defer wg.Done()
            for {
                select {
                case <-ctx.Done():
                    return
                case evt, ok := <-s.ch:
                    if !ok { return }
                    select {
                    case merged <- evt:
                    case <-ctx.Done():
                        return
                    }
                }
            }
        }(s)
    }

    for {
        select {
        case <-ctx.Done():
            return
        case evt, ok := <-merged:
            if !ok { return }
            writeSSEEvent(w, evt)  // existing helper — already includes task_id in JSON
            flusher.Flush()
        }
    }
}
```

**Note on D-07 ("events include task_id field"):** This is already satisfied — `models.Event.TaskID` has `json:"task_id"` (`internal/models/event.go:10`). The existing `writeSSEEvent` serializes the full event, so multiplexed events already carry `task_id` out of the box. Do not add a new field.

### Pattern 2: Extended List Query with Subtask Counts (Backend)
**What:** Single SQL query returns each task plus its completed/total subtask counts via LEFT JOIN + GROUP BY.
**When to use:** To power the dashboard progress bar without N+1 queries.
**Example:**
```sql
-- Source: Extending internal/handlers/tasks.go:137 query
SELECT
    t.id, t.title, t.description, t.status, t.created_by,
    t.metadata, t.plan, t.result, t.error, t.replan_count,
    t.created_at, t.completed_at,
    COALESCE(COUNT(s.id), 0) AS total_subtasks,
    COALESCE(SUM(CASE WHEN s.status = 'completed' THEN 1 ELSE 0 END), 0) AS completed_subtasks
FROM tasks t
LEFT JOIN subtasks s ON s.task_id = t.id
WHERE {filters}
GROUP BY t.id
ORDER BY t.created_at DESC
LIMIT $N OFFSET $M;
```
Add two fields to `models.Task`: `CompletedSubtasks int `json:"completed_subtasks"`` and `TotalSubtasks int `json:"total_subtasks"``. Update `scanTask` accordingly — but note `scanTask` is shared between `List` and `Get`; either (a) create a parallel `scanTaskWithCounts` or (b) add the counts and default them to 0 on `Get`. Recommend **(a)** to keep the contract explicit.

### Pattern 3: URL-Synced Filter State (Frontend)
**What:** Filter state lives in the URL; React state reads from `useSearchParams`.
**Why:** D-11 requires shareable/bookmarkable URLs; Next.js App Router ships this pattern natively.
**Example (verified against [CITED: Next.js learn tutorial](https://nextjs.org/learn/dashboard-app/adding-search-and-pagination)):**
```typescript
// TaskFilterBar.tsx — top-level pattern
"use client";
import { useSearchParams, usePathname, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

export function TaskFilterBar() {
  const searchParams = useSearchParams();
  const pathname = usePathname();
  const router = useRouter();

  // Derive from URL — no separate state for filter values
  const activeTab = searchParams.get("status") ?? "all";
  const query = searchParams.get("q") ?? "";
  const page = Number(searchParams.get("page") ?? "1");

  // Local state for uncontrolled debounce
  const [searchInput, setSearchInput] = useState(query);

  // Push new URL when user changes a filter
  const updateParams = useCallback((updates: Record<string, string | null>) => {
    const params = new URLSearchParams(searchParams.toString());
    for (const [k, v] of Object.entries(updates)) {
      if (v === null || v === "" || v === "all") params.delete(k);
      else params.set(k, v);
    }
    // Reset page when filter changes
    if (!("page" in updates)) params.delete("page");
    router.replace(`${pathname}?${params.toString()}`);
  }, [searchParams, pathname, router]);

  // Debounced search — 300ms per UI-SPEC
  useEffect(() => {
    const t = setTimeout(() => {
      if (searchInput !== query) updateParams({ q: searchInput || null });
    }, 300);
    return () => clearTimeout(t);
  }, [searchInput, query, updateParams]);

  // ... render tabs + search input
}
```

**Suspense boundary:** Next.js 16 requires `useSearchParams` to be wrapped in `<Suspense>` at the page level to allow prerendering. Wrap `TaskDashboard` in `Suspense` inside `web/app/page.tsx`, or mark the dashboard as client-only. [CITED: [Next.js useSearchParams docs](https://nextjs.org/docs/app/api-reference/functions/use-search-params)]

### Pattern 4: Dashboard Store with SSE Lifecycle (Frontend)
**What:** A single Zustand store owns (a) filter-derived task list, (b) per-task progress map updated from SSE events, (c) SSE disconnect handle.
**When to use:** Any page that needs to coordinate filtered data + live updates + cleanup on unmount.
**Example (pattern verified against `web/lib/store.ts:90-326`):**
```typescript
// useDashboardStore — extracted pattern from useTaskStore + useAgentStore
interface DashboardState {
  tasks: Task[];
  totalPages: number;
  isLoading: boolean;
  taskProgress: Record<string, { completed: number; total: number }>;
  multiTaskSSEDisconnect: (() => void) | null;

  loadDashboard: (params: { status?: string; q?: string; page?: number }) => Promise<void>;
  connectMultiTaskSSE: (ids: string[]) => void;
  disconnectMultiTaskSSE: () => void;
  handleMultiTaskEvent: (event: TaskEvent) => void;
}

export const useDashboardStore = create<DashboardState>((set, get) => ({
  tasks: [],
  totalPages: 0,
  isLoading: false,
  taskProgress: {},
  multiTaskSSEDisconnect: null,

  loadDashboard: async (params) => {
    set({ isLoading: true });
    try {
      const result = await api.tasks.list(params);
      // Hydrate progress map from API-returned counts (no extra fetches)
      const progress: Record<string, { completed: number; total: number }> = {};
      for (const t of result.items) {
        progress[t.id] = {
          completed: t.completed_subtasks ?? 0,
          total: t.total_subtasks ?? 0,
        };
      }
      set({
        tasks: result.items,
        totalPages: result.pages,
        taskProgress: progress,
        isLoading: false,
      });
    } catch {
      set({ isLoading: false });
    }
  },

  connectMultiTaskSSE: (ids) => {
    get().disconnectMultiTaskSSE();
    if (ids.length === 0) return;
    const disconnect = connectMultiTaskSSE(ids, (evt) => get().handleMultiTaskEvent(evt as TaskEvent));
    set({ multiTaskSSEDisconnect: disconnect });
  },

  disconnectMultiTaskSSE: () => {
    const d = get().multiTaskSSEDisconnect;
    if (d) { d(); set({ multiTaskSSEDisconnect: null }); }
  },

  handleMultiTaskEvent: (event) => {
    const taskId = event.task_id;
    if (!taskId) return;

    // Update status from task.* events
    if (event.type.startsWith("task.")) {
      const statusMap: Record<string, Task["status"]> = { /* ... same as useTaskStore ... */ };
      const newStatus = statusMap[event.type];
      if (newStatus) {
        set((s) => ({
          tasks: s.tasks.map((t) => (t.id === taskId ? { ...t, status: newStatus } : t)),
        }));
      }
    }

    // Update per-task progress from subtask.* events
    if (event.type === "subtask.completed") {
      set((s) => {
        const cur = s.taskProgress[taskId];
        if (!cur) return s;
        return {
          taskProgress: {
            ...s.taskProgress,
            [taskId]: { ...cur, completed: cur.completed + 1 },
          },
        };
      });
    }
    if (event.type === "subtask.created") {
      set((s) => {
        const cur = s.taskProgress[taskId] ?? { completed: 0, total: 0 };
        return {
          taskProgress: {
            ...s.taskProgress,
            [taskId]: { ...cur, total: cur.total + 1 },
          },
        };
      });
    }
  },
}));
```

### Pattern 5: TaskDashboard Lifecycle Orchestration
```typescript
// TaskDashboard.tsx
"use client";
import { useEffect, useMemo } from "react";
import { useSearchParams } from "next/navigation";
import { useDashboardStore } from "@/lib/store";
import { useAgentStore } from "@/lib/store";

export function TaskDashboard() {
  const searchParams = useSearchParams();
  const status = searchParams.get("status") ?? undefined;
  const q = searchParams.get("q") ?? undefined;
  const page = Number(searchParams.get("page") ?? "1");

  const tasks = useDashboardStore((s) => s.tasks);
  const isLoading = useDashboardStore((s) => s.isLoading);
  const loadDashboard = useDashboardStore((s) => s.loadDashboard);
  const connectMultiTaskSSE = useDashboardStore((s) => s.connectMultiTaskSSE);
  const disconnectMultiTaskSSE = useDashboardStore((s) => s.disconnectMultiTaskSSE);

  const connectAgentSSE = useAgentStore((s) => s.connectStatusSSE);
  const disconnectAgentSSE = useAgentStore((s) => s.disconnectStatusSSE);

  // Reload when filter changes
  useEffect(() => {
    void loadDashboard({ status, q, page });
  }, [status, q, page, loadDashboard]);

  // Multiplexed SSE keyed to visible task IDs
  const ids = useMemo(() => tasks.map((t) => t.id), [tasks]);
  useEffect(() => {
    if (ids.length === 0) { disconnectMultiTaskSSE(); return; }
    connectMultiTaskSSE(ids);
    return () => disconnectMultiTaskSSE();
  }, [ids.join(","), connectMultiTaskSSE, disconnectMultiTaskSSE]); // stable dep via join

  // Global agent status stream (reused)
  useEffect(() => {
    connectAgentSSE();
    return () => disconnectAgentSSE();
  }, [connectAgentSSE, disconnectAgentSSE]);

  // ... render TaskFilterBar + grid + Pagination
}
```

### Anti-Patterns to Avoid

- **One EventSource per card.** Violates INTR-03 — 4 cards + agent stream = 5 concurrent SSE; add browser tabs and you hit the 6-connection wall immediately. Always use the multiplexed endpoint.
- **Polling the task list.** CONTEXT.md specifies SSE-only updates. `setInterval(loadTasks, N)` creates jitter, wastes CPU, and collides with SSE state updates.
- **N+1 fetches for subtask counts.** Do not loop over `tasks` and fetch `/api/tasks/{id}/subtasks` for each. Extend the list query once.
- **Storing filter state in Zustand.** URL is the source of truth per D-11. Zustand holds *derived data* (tasks, progress), not filter values.
- **Reusing `useTaskStore.connectSSE(taskId)` for dashboard.** That method points to the per-task stream (`/api/tasks/{id}/events`) and will break D-08's connection budget. The dashboard needs its own SSE helper.
- **Sending task-lifecycle updates to `useTaskStore.handleEvent`.** That store drives the task-detail page; the dashboard's events should update `useDashboardStore` only. Cross-contamination leads to stale `currentTask` and infinite-loop selects.
- **Forgetting to close broker channels on HTTP disconnect.** `defer h.Broker.Unsubscribe` inside a loop is required — miss this and you leak goroutines on every dashboard unload.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Per-task SSE in the dashboard | Custom `Map<taskId, EventSource>` | Multiplexed `/api/tasks/stream?ids=...` endpoint | Browser 6-connection cap, explicit INTR-03 requirement |
| Real-time fan-out | Raw goroutines writing to the same `http.ResponseWriter` from multiple broker subs | Existing `events.Broker` (`internal/events/broker.go`) with one goroutine per subscription feeding a merged channel | Broker already handles subscribe/publish/lifecycle; writing to the same `ResponseWriter` from multiple goroutines is a race (`net/http` requires serial writes). Fan-in via merged channel is the correct pattern. |
| Subtask progress aggregation | Frontend loop over N `/api/tasks/{id}/subtasks` calls | LEFT JOIN + GROUP BY in the List query | Single round-trip, no N+1, no client-side math races with SSE. |
| Status badge styling | New badge components | Reuse `statusConfig` from `web/components/dashboard/TaskCard.tsx:8` verbatim | UI-SPEC explicitly instructs reuse; avoids visual drift. |
| Agent status dots | New status component | Reuse `AgentStatusDot` (`web/components/agent/AgentStatusDot.tsx:42`) | Phase 2 deliverable; colors + pulse animation already match UI-SPEC. |
| URL state management | Custom history wrapper | `useSearchParams` + `useRouter().replace()` from `next/navigation` | Built into Next.js 16; zero deps [CITED: [Next.js docs](https://nextjs.org/docs/app/api-reference/functions/use-search-params)] |
| Time-ago formatting | New utility | Reuse `timeAgo` from `web/components/dashboard/TaskCard.tsx:42` | Already correct; avoid duplication. |
| Debounced search | `use-debounce` package | 5-line `setTimeout`/`clearTimeout` in `useEffect` | Trivial; no dependency justification. |
| Pagination UI | New component | Existing `web/components/ui/pagination.tsx` | UI-SPEC pins this. |

**Key insight:** Phase 4 is an *integration* phase, not a greenfield phase. Nearly every visual primitive and every backend building block already exists. The new code is: **one SSE handler (MultiStream), one SQL query extension (counts), one store slice, and five presentational components**. That's it.

## Runtime State Inventory

*This is a greenfield feature phase — no rename, refactor, or migration. Section omitted per research template instructions.*

## Common Pitfalls

### Pitfall 1: Race on `http.ResponseWriter` from multiple goroutines
**What goes wrong:** Naive implementation spawns one goroutine per broker channel, each writing directly to the response writer. `net/http` serializes writes but interleaved `writeSSEEvent` + `Flush` calls produce corrupted SSE frames (mixed `id:` / `data:` lines).
**Why it happens:** SSE frames are multi-line; each event needs 3 lines written atomically. Two goroutines calling `fmt.Fprintf(w, "id: ...\ndata: ...\n\n")` concurrently interleave lines.
**How to avoid:** Use a **single writer goroutine** that reads from a merged channel. All broker-subscriber goroutines write events into the merged channel; only the main handler loop writes to `w`. Pattern in Pattern 1 example above.
**Warning signs:** Frontend logs "failed to parse SSE event" or EventSource silently drops events.

### Pitfall 2: Forgetting subscribe-before-replay for multiplexed stream
**What goes wrong:** Same race Phase 1 D-01 fixed for per-task stream. If you replay historical events first, then `Subscribe`, events published between replay-end and subscribe-start are lost forever.
**Why it happens:** Copying the logical order "load history → subscribe" feels natural.
**How to avoid:** Call `h.Broker.Subscribe(id)` for every id **before** any DB replay query. The broker channel buffers up to 64 events. For the dashboard, replay may be skipped entirely — the initial List query already carries current counts, and only *future* events matter.
**Warning signs:** Progress bar lags behind task detail page; SSE events appear "missing" for events that happened during page load.

### Pitfall 3: EventSource reconnect replays with stale `ids` query
**What goes wrong:** User changes filter → frontend disconnects old EventSource and creates a new one with different ids. If the old connection wasn't cleanly closed, the browser's auto-reconnect might target the old URL (it shouldn't, since we called `.close()`, but CORS + proxy quirks exist).
**Why it happens:** Confusion between `EventSource.close()` (finalizes) and HTTP keepalive (may linger).
**How to avoid:** Always call `.close()` before creating a new `EventSource`; the `connectMultiTaskSSE` helper should return a disconnect function that calls `.close()`, matching existing pattern in `web/lib/sse.ts:47`.
**Warning signs:** Network tab shows duplicate `/api/tasks/stream` connections after filter change.

### Pitfall 4: `ids.join(",")` in `useEffect` deps triggers unnecessary reconnects
**What goes wrong:** `ids` array is a new array reference every render (from `tasks.map`). If you use `[ids]` as deps, the effect re-runs every render, tearing down and recreating the SSE connection.
**Why it happens:** React deps use `Object.is` comparison.
**How to avoid:** Use the **stable string** `ids.join(",")` as the dep. When the joined string is unchanged, the effect doesn't re-run.
**Warning signs:** Browser network tab shows SSE connection churn every second; dashboard updates feel laggy.

### Pitfall 5: `useSearchParams` requires Suspense boundary in Next.js 16
**What goes wrong:** Without `<Suspense>`, Next.js throws "useSearchParams() should be wrapped in a suspense boundary" during build.
**Why it happens:** App Router prerenders pages; `useSearchParams` is dynamic and opts out of prerendering for anything above it.
**How to avoid:** Wrap `TaskDashboard` in `<Suspense fallback={<Loader/>}>` inside `web/app/page.tsx`.
**Warning signs:** Build error during `make build`; dashboard works in dev but fails `next build`.
**Source:** [CITED: [Next.js useSearchParams docs](https://nextjs.org/docs/app/api-reference/functions/use-search-params)]

### Pitfall 6: Subtask counts diverge from SSE progress
**What goes wrong:** Initial counts come from LIST API; then SSE events update the progress map. If the API query and the SSE events use slightly different definitions of "completed" (e.g., API counts `status='completed'` but SSE fires on `subtask.completed` event which might not yet be reflected in DB), the user sees "3/5" jump to "4/5" and then back to "3/5".
**Why it happens:** Two sources of truth.
**How to avoid:** Single source of increment. API returns initial snapshot; SSE only *increments* from that snapshot. Never re-fetch the list when SSE fires — apply deltas only. On filter change, accept the new snapshot from API.
**Warning signs:** Progress numbers flicker during active task execution.

### Pitfall 7: Unsubscribing from broker leaks channels
**What goes wrong:** If the handler's `defer` loop calls `Unsubscribe` but a channel has 63 buffered events and the consumer goroutine already exited, the channel never drains.
**Why it happens:** `broker.Unsubscribe` closes the channel; but if the consumer goroutine is already exited on `ctx.Done()`, the close is fine — Go handles it. The actual leak is if you forget `Unsubscribe` entirely and just let the ctx cancel, because the broker still holds the channel in its map.
**How to avoid:** Every `Subscribe` call must be paired with an `Unsubscribe`. Use `defer` immediately after each `Subscribe`, not a batch defer that might run in wrong order.
**Warning signs:** Server memory growth over time under repeated dashboard page loads.

### Pitfall 8: List query ORDER BY breaks after GROUP BY
**What goes wrong:** `GROUP BY t.id` with `ORDER BY t.created_at DESC` works in PostgreSQL because `t.created_at` is functionally dependent on `t.id` (primary key) — but only when `t.id` is declared PK. Adding columns to SELECT that aren't in GROUP BY or an aggregate fails.
**Why it happens:** SQL spec requires all SELECT columns to be in GROUP BY or aggregated; PostgreSQL has a PK-dependency exception.
**How to avoid:** Verify `tasks.id` is primary key (it is — `internal/db/migrations/`). All task columns in SELECT are functionally dependent on `t.id` via PK, so `GROUP BY t.id` alone is legal. Test with `make test` locally.
**Warning signs:** SQL error `column "t.title" must appear in the GROUP BY clause` — means PK dependency isn't being picked up; fallback is `GROUP BY t.id, t.title, t.description, ...`.

## Code Examples

### Example 1: Extend Task model with subtask counts
```go
// internal/models/task.go — add two fields
type Task struct {
    // ...existing fields...
    CompletedSubtasks int `json:"completed_subtasks"`
    TotalSubtasks     int `json:"total_subtasks"`
}
```
```typescript
// web/lib/types.ts — mirror Go struct
export interface Task {
  // ...existing fields...
  completed_subtasks: number;
  total_subtasks: number;
}
```

### Example 2: Register new route
```go
// cmd/server/main.go — inside authed group, after existing SSE route at line 209
r.Get("/api/tasks/stream", streamH.MultiStream)  // Multiplexed multi-task SSE
```

### Example 3: Frontend SSE helper
```typescript
// web/lib/sse.ts — mirror existing connectAgentStatusSSE pattern
export function connectMultiTaskSSE(
  taskIds: string[],
  onEvent: SSEEventHandler,
  onError?: (error: Event) => void,
): () => void {
  if (taskIds.length === 0) return () => {};
  const url = `${BASE}/api/tasks/stream?ids=${encodeURIComponent(taskIds.join(","))}`;
  const eventSource = new EventSource(url, { withCredentials: true });

  eventSource.onmessage = (e: MessageEvent) => {
    try {
      const parsed = JSON.parse(e.data as string) as Record<string, unknown>;
      onEvent({
        id: e.lastEventId,
        type: parsed.type as string,
        data: (parsed.data as Record<string, unknown>) ?? {},
        ...(parsed.subtask_id ? { subtask_id: parsed.subtask_id as string } : {}),
        ...(parsed.task_id ? { task_id: parsed.task_id as string } : {}),
        ...(parsed.actor_type ? { actor_type: parsed.actor_type as string } : {}),
        ...(parsed.actor_id ? { actor_id: parsed.actor_id as string } : {}),
        ...(parsed.created_at ? { created_at: parsed.created_at as string } : {}),
      });
    } catch { /* ignore malformed */ }
  };

  eventSource.onerror = (e: Event) => { if (onError) onError(e); };
  return () => eventSource.close();
}
```

### Example 4: Subtask progress bar component
```typescript
// web/components/dashboard/SubtaskProgressBar.tsx
"use client";
import { cn } from "@/lib/utils";

interface SubtaskProgressBarProps {
  completed: number;
  total: number;
  failed?: boolean;
}

export function SubtaskProgressBar({ completed, total, failed }: SubtaskProgressBarProps) {
  const pct = total > 0 ? (completed / total) * 100 : 0;
  const barColor = failed
    ? "bg-red-500"
    : pct >= 100
      ? "bg-green-500"
      : pct >= 50
        ? "bg-blue-500"
        : "bg-amber-500";

  return (
    <div className="flex items-center gap-2">
      <div
        className="h-1.5 flex-1 overflow-hidden rounded-full bg-gray-800"
        role="progressbar"
        aria-valuenow={completed}
        aria-valuemin={0}
        aria-valuemax={total}
        aria-label="Subtask progress"
      >
        <div
          className={cn("h-full rounded-full transition-all", barColor)}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className="shrink-0 text-xs text-muted-foreground">
        {completed}/{total} subtasks
      </span>
    </div>
  );
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Multiple per-resource SSE connections | Single multiplexed SSE endpoint (or HTTP/2) | Chromium bug 275955 (2013, still unresolved) and general HTTP/2 adoption (~2018-2020) | The 6-connection cap over HTTP/1.1 has not moved; multiplexing is the only way to scale SSE without TLS. [CITED: [Chromium issue](https://issues.chromium.org/issues/40329530)] |
| Client-side search state with URL-out-of-sync | URL-first state with `useSearchParams` | Next.js App Router stable (13.4+, 2023) | Standard for Next.js 16. [CITED: [Next.js docs](https://nextjs.org/docs/app/api-reference/functions/use-search-params)] |
| N+1 fetches for denormalized dashboard views | Single join-aggregation query | Always best practice | Nothing new; just reinforcement. |

**Deprecated / not used here:**
- WebSockets for real-time dashboards — SSE is simpler, HTTP-native, auto-reconnects, and fits one-way server-to-client updates. [CITED: [SSE vs WebSockets 2025](https://dev.to/polliog/server-sent-events-beat-websockets-for-95-of-real-time-apps-heres-why-a4l)]

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Extending `TaskHandler.List` SQL with LEFT JOIN + GROUP BY is performant enough at expected task counts (v1 scale: <500 tasks total) | Pattern 2 | Low. At v1 scale the join is O(subtasks) per page; if a user has 10k tasks with 100 subtasks each, performance could degrade. Mitigation: index exists on `subtasks(task_id)`. Verify in Phase 4 execution by running `EXPLAIN` on a seeded dataset. |
| A2 | Existing subtask events use lifecycle types `subtask.created`, `subtask.completed`, `subtask.failed`, etc. matching `useTaskStore.handleEvent` (`web/lib/store.ts:217-269`) | Pattern 4 | Low — verified by reading the store this session. But the `handleEvent` logic in `useTaskStore` operates on `currentTask`; dashboard handler needs its own translation. |
| A3 | `writeSSEEvent` helper (`internal/handlers/stream.go:99`) is package-private within `handlers` — so `MultiStream` method on the same package can call it directly | Pattern 1 | None — verified in the same package. |
| A4 | Browsers still enforce the 6-connection HTTP/1.1 cap in 2026 | Summary + Pitfalls | Low. Chromium bug 275955 remains unresolved as of last search results. [CITED: [Chromium bug](https://issues.chromium.org/issues/40329530)]. If browsers ever increased the limit, the multiplexed endpoint is still correct — just not strictly required. |
| A5 | Phase 4 does not need to replay historical events on the multiplexed stream (LIST API is the snapshot) | Pattern 1 | Medium. If the user opens the dashboard exactly during a burst of subtask completions, the initial LIST counts may be slightly stale and SSE will correct them within milliseconds. Acceptable UX. |
| A6 | No replay → no need for Last-Event-ID support on the multiplexed endpoint | Pattern 1 | Medium. If reconnect is expected to be common, add Last-Event-ID + event store cursor. For dashboard's live-only use case, auto-reconnect without replay is acceptable because the next list refresh (on filter change) corrects drift. |
| A7 | Multiplexed endpoint will be rate-limited by max ids param (recommend cap at 50) to prevent DoS via unbounded subscriptions | Pattern 1 | Low. Matches pagination `per_page` cap of 100 in `internal/handlers/tasks.go:103`. |

**Follow-up for discuss-phase or planner:** A1 and A5 are the highest-risk assumptions. Both are low-probability at v1 scale and self-correcting in practice, but the planner should surface them as open risks in the PLAN's success criteria.

## Open Questions

1. **Dashboard replaces `/` vs dedicated `/dashboard` route** (D-04 is ambiguous)
   - What we know: D-04 says "replaces EmptyState at `/` OR gets a dedicated route". UI-SPEC layout diagram shows it rendering inside `ConversationSidebar + main` layout at `/`.
   - What's unclear: Does the sidebar still show conversations while the dashboard fills the main area? The UI-SPEC layout diagram confirms yes — AppShell routes `/` as `isChatRoute` so `ConversationSidebar` stays.
   - Recommendation: **Replace at `/`** — matches UI-SPEC's layout diagram. `web/app/page.tsx` becomes `<TaskDashboard />` wrapped in Suspense.

2. **Replay on multiplexed stream: yes or no?**
   - What we know: Per-task stream replays historical events (`internal/handlers/stream.go:41-64`).
   - What's unclear: Does dashboard need replay? Initial counts come from the LIST API, which is the effective snapshot.
   - Recommendation: **Skip replay for v1**. Simplifies the handler significantly. Revisit if users report progress-bar drift after reconnect.

3. **Max `ids` cap on `/api/tasks/stream?ids=...`**
   - What we know: No cap is defined.
   - What's unclear: What's a reasonable safety limit to prevent a client from subscribing to every task in the DB?
   - Recommendation: **Cap at 50**. Dashboard paginates at 12/page per UI-SPEC, so 50 gives 4x headroom for future larger grids. Return 400 with a clear error message if exceeded.

4. **What subtask event types increment progress?**
   - What we know: `subtask.completed` increments `completed`. `subtask.created` increments `total`.
   - What's unclear: Does `subtask.failed` count toward "completed" for progress-bar purposes? Does `subtask.cancelled` decrement `total`?
   - Recommendation: **Progress bar shows completed/total; failed counts as "done" for the bar (it's no longer running) but card status shows "failed" via the badge.** Planner should confirm with the user or in discuss-phase.

## Environment Availability

Phase 4 has no external tool dependencies beyond what Phase 1-2 already requires (Go 1.26, Node.js 22, PostgreSQL). Nothing new to probe. Skipping full audit.

## Validation Architecture

*Skipped — `workflow.nyquist_validation` is explicitly `false` in `.planning/config.json`.*

## Security Domain

*Skipped — no `security_enforcement` key in `.planning/config.json`; TaskHub is in local-mode development and this phase has no new auth/crypto/input-validation surface beyond what `RequireAuth` middleware already covers. The new `MultiStream` endpoint inherits `authMw.RequireAuth` by registration inside `r.Group(...)`.*

**One explicit check for the planner:** The multiplexed endpoint MUST be registered inside the authenticated route group (`cmd/server/main.go:165` onward), not as a public route. Unauthenticated access to task events would be an info leak.

## Sources

### Primary (HIGH confidence)
- **Codebase (verified this session):**
  - `internal/events/broker.go` — Broker Subscribe/Publish/Unsubscribe API
  - `internal/handlers/stream.go` — Per-task SSE handler reference
  - `internal/handlers/agent_status_stream.go` — Global SSE handler with subscribe-before-replay
  - `internal/handlers/tasks.go` — Existing List with status/search/pagination
  - `internal/models/event.go` — Event struct with `task_id` JSON tag
  - `cmd/server/main.go` — Route registration pattern
  - `web/lib/sse.ts` — Existing SSE helpers (connectSSE, connectAgentStatusSSE)
  - `web/lib/store.ts` — Zustand store patterns (useTaskStore, useAgentStore)
  - `web/lib/api.ts` — API client, `api.tasks.list` filter params already in place
  - `web/components/dashboard/TaskCard.tsx` — statusConfig, timeAgo helpers to reuse
  - `web/components/agent/AgentStatusDot.tsx` — Dot component from Phase 2
  - `web/components/dashboard/NewTaskDialog.tsx` — CTA dialog to mount in filter bar
  - `web/lib/types.ts` — Task/SubTask/TaskEvent interfaces
  - `web/package.json` — Next.js 16.1.6, React 19.2.3, Zustand 5.0.11, shadcn 4.0.5
  - `.planning/config.json` — Workflow flags (nyquist_validation false, ui_phase true)
  - `.planning/phases/04-parallel-dashboard/04-CONTEXT.md` — D-01..D-12 decisions
  - `.planning/phases/04-parallel-dashboard/04-UI-SPEC.md` — Visual + interaction contract

- **Next.js official docs:**
  - [useSearchParams API reference](https://nextjs.org/docs/app/api-reference/functions/use-search-params)
  - [Adding Search and Pagination tutorial](https://nextjs.org/learn/dashboard-app/adding-search-and-pagination)

- **MDN:**
  - [Using server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)

### Secondary (MEDIUM confidence — WebSearch corroborated)
- [Chromium issue: Limit of 6 concurrent EventSource connections is too low](https://issues.chromium.org/issues/40329530)
- [text/plain: The Pitfalls of EventSource over HTTP/1.1](https://textslashplain.com/2019/12/04/the-pitfalls-of-eventsource-over-http-1-1/)
- [Server-Sent Events Beat WebSockets for 95% of Real-Time Apps](https://dev.to/polliog/server-sent-events-beat-websockets-for-95-of-real-time-apps-heres-why-a4l)
- [SSE's Glorious Comeback: 2025](https://portalzine.de/sses-glorious-comeback-why-2025-is-the-year-of-server-sent-events/)

### Tertiary (LOW confidence — none required)
- No LOW-confidence claims in this research.

## Metadata

**Confidence breakdown:**
- Backend architecture (multiplexed SSE fan-out): **HIGH** — exact pattern exists in codebase (`agent_status_stream.go`), broker primitives are already built, only fan-in goroutine is new.
- Frontend architecture (store slice + URL filters): **HIGH** — all patterns verified against existing `useTaskStore` / `useAgentStore` and Next.js official docs.
- Subtask-count query extension: **HIGH** — straightforward SQL, PostgreSQL PK-dependency rule confirmed.
- Pitfalls (response writer race, Suspense boundary, ids.join dep): **HIGH** — each verified against codebase or Next.js docs this session.
- Browser connection cap: **HIGH** — [CITED: MDN + Chromium bug], consistent with every secondary source.
- Subtask event taxonomy used for progress increments: **MEDIUM** — event types verified in `useTaskStore.handleEvent`, but exact semantics of `failed`/`cancelled` vs progress bar need planner confirmation (open question 4).

**Research date:** 2026-04-04
**Valid until:** 2026-05-04 (30 days — stack is stable; only browser connection-cap behavior could theoretically change, and it hasn't in 13 years)
